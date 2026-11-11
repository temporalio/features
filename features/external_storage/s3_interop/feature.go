package s3_interop

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/temporalio/features/harness/go/harness"
	commonpb "go.temporal.io/api/common/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/contrib/aws/s3driver"
	"go.temporal.io/sdk/contrib/aws/s3driver/awssdkv2"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	envBucket      = "TEMPORAL_FEATURES_S3_BUCKET"
	envEndpoint    = "TEMPORAL_FEATURES_S3_ENDPOINT"
	driverName     = "aws.s3driver"
	referenceType  = "temporal.api.sdk.v1.ExternalStorageReference"
	signalName     = "foreign"
	foreignValue   = "hello"
	foreignPayload = "ChYKCGVuY29kaW5nEgpqc29uL3BsYWluEgciaGVsbG8i"
	foreignHash    = "1b8435174847de88db3a8c5a764295d62add152743b3a9a730796a4c090caf8c"
	inputHash      = "8016abfa3587d601e521edf45b84c3fca4ef12f10d6b470d4c4aa0eab0f82f1a"
)

var (
	input    = strings.Repeat("x", 300*1024)
	bucket   = os.Getenv(envBucket)
	s3Client = newS3Client()
)

var Feature = harness.Feature{
	Workflows:     Workflow,
	Execute:       execute,
	CheckResult:   checkResult,
	CheckHistory:  checkHistory,
	ClientOptions: client.Options{ExternalStorage: externalStorage()},
}

func Workflow(ctx workflow.Context, value string) (string, error) {
	var foreign string
	workflow.GetSignalChannel(ctx, signalName).Receive(ctx, &foreign)
	if foreign != foreignValue {
		return "", fmt.Errorf("foreign payload is %q, expected %q", foreign, foreignValue)
	}
	return value, nil
}

func execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	if bucket == "" {
		return nil, r.Skip(envBucket + " is not set")
	}

	data, _ := base64.StdEncoding.DecodeString(foreignPayload)
	if err := putObject(ctx, foreignKey(), data); err != nil {
		return nil, err
	}

	run, err := r.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		TaskQueue:                r.TaskQueue,
		WorkflowExecutionTimeout: time.Minute,
	}, Workflow, input)
	if err != nil {
		return nil, err
	}

	reference, err := json.Marshal(storageReference{DriverName: driverName, ClaimData: map[string]string{
		"bucket":         bucket,
		"key":            foreignKey(),
		"hash_algorithm": "sha256",
		"hash_value":     foreignHash,
	}})
	if err != nil {
		return nil, err
	}

	_, err = r.Client.WorkflowService().SignalWorkflowExecution(ctx, &workflowservice.SignalWorkflowExecutionRequest{
		Namespace:         r.Namespace,
		WorkflowExecution: &commonpb.WorkflowExecution{WorkflowId: run.GetID(), RunId: run.GetRunID()},
		SignalName:        signalName,
		Input: &commonpb.Payloads{Payloads: []*commonpb.Payload{{
			Metadata: map[string][]byte{"encoding": []byte("json/protobuf"), "messageType": []byte(referenceType)},
			Data:     reference,
		}}},
	})
	return run, err
}

func checkResult(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	var result string
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	r.Require.Equal(input, result)

	response, err := r.Client.WorkflowService().GetWorkflowExecutionHistory(ctx, &workflowservice.GetWorkflowExecutionHistoryRequest{
		Namespace: r.Namespace,
		Execution: &commonpb.WorkflowExecution{WorkflowId: run.GetID(), RunId: run.GetRunID()},
	})
	if err != nil {
		return err
	}

	var started *historypb.WorkflowExecutionStartedEventAttributes
	var completed *historypb.WorkflowExecutionCompletedEventAttributes
	for _, event := range response.GetHistory().GetEvents() {
		if attrs := event.GetWorkflowExecutionStartedEventAttributes(); attrs != nil {
			started = attrs
		}
		if attrs := event.GetWorkflowExecutionCompletedEventAttributes(); attrs != nil {
			completed = attrs
		}
	}
	r.Require.NotNil(started)
	r.Require.NotNil(completed)

	prefix := "v0/ns/" + r.Namespace + "/wt/" + started.GetWorkflowType().GetName() + "/wi/" + run.GetID()
	inputKey := checkReference(r, started.GetInput().GetPayloads()[0], prefix+"/ri/null/d/sha256/"+inputHash)
	checkReference(r, completed.GetResult().GetPayloads()[0], prefix+"/ri/"+run.GetRunID()+"/d/sha256/"+inputHash)

	object, err := getObject(ctx, inputKey)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(object)
	r.Require.Equal(inputHash, hex.EncodeToString(digest[:]))
	return nil
}

func checkHistory(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	replayer, err := worker.NewWorkflowReplayerWithOptions(worker.WorkflowReplayerOptions{ExternalStorage: externalStorage()})
	if err != nil {
		return err
	}
	replayer.RegisterWorkflow(Workflow)
	return replayer.ReplayWorkflowExecution(ctx, r.Client.WorkflowService(), nil, r.Namespace, workflow.Execution{
		ID:    run.GetID(),
		RunID: run.GetRunID(),
	})
}

type storageReference struct {
	DriverName string            `json:"driverName"`
	ClaimData  map[string]string `json:"claimData"`
}

func checkReference(r *harness.Runner, payload *commonpb.Payload, key string) string {
	r.Require.Equal("json/protobuf", string(payload.GetMetadata()["encoding"]))
	r.Require.Equal(referenceType, string(payload.GetMetadata()["messageType"]))

	var reference storageReference
	r.Require.NoError(json.Unmarshal(payload.GetData(), &reference))
	r.Require.Equal(storageReference{DriverName: driverName, ClaimData: map[string]string{
		"bucket":         bucket,
		"key":            key,
		"hash_algorithm": "sha256",
		"hash_value":     inputHash,
	}}, reference)
	return key
}

func foreignKey() string {
	return "v0/d/sha256/" + foreignHash
}

func newS3Client() *s3.Client {
	if bucket == "" {
		return nil
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(err)
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint := os.Getenv(envEndpoint); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}
	})
}

func externalStorage() converter.ExternalStorage {
	if s3Client == nil {
		return converter.ExternalStorage{}
	}
	driver, err := s3driver.NewDriver(s3driver.Options{
		Client: awssdkv2.NewClient(s3Client),
		Bucket: s3driver.StaticBucket(bucket),
	})
	if err != nil {
		panic(err)
	}
	return converter.ExternalStorage{Drivers: []converter.StorageDriver{driver}}
}

func putObject(ctx context.Context, key string, data []byte) error {
	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(bucket), Key: aws.String(key), Body: strings.NewReader(string(data))})
	return err
}

func getObject(ctx context.Context, key string) ([]byte, error) {
	output, err := s3Client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()
	return io.ReadAll(output.Body)
}
