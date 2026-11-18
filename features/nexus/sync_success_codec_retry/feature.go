package sync_success_codec_retry

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/features/data_converter/codec"
	"github.com/temporalio/features/harness/go/harness"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/workflow"
)

// These counters are used only by the codec, handler, and result check, never the workflow.
var handlerCalls, encodeAttempts atomic.Int32

type failOnceCodec struct {
	converter.PayloadCodec
}

func (c failOnceCodec) Encode(payloads []*commonpb.Payload) ([]*commonpb.Payload, error) {
	for _, p := range payloads {
		if string(p.Data) == `"result"` && encodeAttempts.Add(1) == 1 {
			return nil, errors.New("one-shot result encoding failure")
		}
	}
	return c.PayloadCodec.Encode(payloads)
}

var operation = nexus.NewSyncOperation(
	"run",
	func(ctx context.Context, input string, options nexus.StartOperationOptions) (string, error) {
		handlerCalls.Add(1)
		return "result", nil
	},
)

var service = func() *nexus.Service {
	s := nexus.NewService("codec-service")
	s.MustRegister(operation)
	return s
}()

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	future := workflow.NewNexusClient(endpoint, "codec-service").ExecuteOperation(
		ctx, operation, "input", workflow.NexusOperationOptions{ScheduleToCloseTimeout: time.Minute})
	var result string
	if err := future.Get(ctx, &result); err != nil {
		return "", err
	}
	if result != "result" {
		return "", harness.AppErrorf("unexpected result: %q", result)
	}
	return "ok", nil
}

var Feature = harness.Feature{
	Workflows:       Workflow,
	NexusServices:   service,
	ExpectRunResult: "ok",
	ClientOptions: client.Options{
		DataConverter: converter.NewCodecDataConverter(
			converter.GetDefaultDataConverter(), failOnceCodec{codec.NewBase64Codec()}),
	},
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		handlerCalls.Store(0)
		encodeAttempts.Store(0)
		return runner.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}, Workflow, runner.NexusEndpoint)
	},
	CheckResult: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		if err := runner.CheckResultDefault(ctx, run); err != nil {
			return err
		}
		runner.Require.Equal(int32(2), handlerCalls.Load(), "handler calls")
		runner.Require.Equal(int32(2), encodeAttempts.Load(), "result encoding attempts")
		return nil
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		hist := runner.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
		for hist.HasNext() {
			event, err := hist.Next()
			if err != nil {
				return err
			}
			if a := event.GetNexusOperationScheduledEventAttributes(); a != nil {
				runner.Require.Equal("my-encoding", string(a.Input.GetMetadata()[converter.MetadataEncoding]))
			}
			if a := event.GetNexusOperationCompletedEventAttributes(); a != nil {
				runner.Require.Equal("my-encoding", string(a.Result.GetMetadata()[converter.MetadataEncoding]))
			}
		}
		return harness.CheckNexusHistory(ctx, runner, run,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_SCHEDULED,
			enumspb.EVENT_TYPE_NEXUS_OPERATION_COMPLETED)
	},
}
