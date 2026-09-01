package oversized_result_external_storage

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/temporalio/features/harness/go/harness"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/workflow"
	"google.golang.org/protobuf/proto"
)

const (
	queryName    = "oversized-result"
	finishSignal = "finish"
	driverName   = "query-result-memory"
	// Exceed the server limit so the SDK must offload the result.
	resultSize       = 3 * 1024 * 1024
	storageThreshold = 1024
)

var storage = newMemoryDriver()

var Feature = harness.Feature{
	Workflows: Workflow,
	ClientOptions: client.Options{
		ExternalStorage: converter.ExternalStorage{
			Drivers:              []converter.StorageDriver{storage},
			PayloadSizeThreshold: storageThreshold,
		},
	},
	CheckResult: checkResult,
}

func Workflow(ctx workflow.Context) error {
	result := strings.Repeat("a", resultSize)
	if err := workflow.SetQueryHandler(ctx, queryName, func() (string, error) {
		return result, nil
	}); err != nil {
		return err
	}

	workflow.GetSignalChannel(ctx, finishSignal).Receive(ctx, nil)
	return nil
}

func checkResult(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	value, err := r.Client.QueryWorkflow(ctx, run.GetID(), run.GetRunID(), queryName)
	if err != nil {
		return err
	}

	var result string
	if err := value.Get(&result); err != nil {
		return err
	}
	if result != strings.Repeat("a", resultSize) {
		return fmt.Errorf("unexpected query result")
	}
	if storage.storeCalls() == 0 || storage.retrieveCalls() == 0 {
		return fmt.Errorf("query result did not use external storage")
	}

	if err := r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), finishSignal, nil); err != nil {
		return err
	}
	return r.CheckResultDefault(ctx, run)
}

type memoryDriver struct {
	mu        sync.Mutex
	payloads  map[string]*commonpb.Payload
	nextID    int
	stores    int
	retrieves int
}

func newMemoryDriver() *memoryDriver {
	return &memoryDriver{payloads: make(map[string]*commonpb.Payload)}
}

func (*memoryDriver) Name() string {
	return driverName
}

func (*memoryDriver) Type() string {
	return driverName
}

func (d *memoryDriver) Store(
	_ converter.StorageDriverStoreContext,
	payloads []*commonpb.Payload,
) ([]converter.StorageDriverClaim, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stores++
	claims := make([]converter.StorageDriverClaim, len(payloads))
	for i, payload := range payloads {
		key := fmt.Sprintf("payload-%d", d.nextID)
		d.nextID++
		d.payloads[key] = proto.Clone(payload).(*commonpb.Payload)
		claims[i] = converter.StorageDriverClaim{ClaimData: map[string]string{"key": key}}
	}
	return claims, nil
}

func (d *memoryDriver) Retrieve(
	_ converter.StorageDriverRetrieveContext,
	claims []converter.StorageDriverClaim,
) ([]*commonpb.Payload, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.retrieves++
	payloads := make([]*commonpb.Payload, len(claims))
	for i, claim := range claims {
		key := claim.ClaimData["key"]
		payload, ok := d.payloads[key]
		if !ok {
			return nil, fmt.Errorf("payload %q not found", key)
		}
		payloads[i] = proto.Clone(payload).(*commonpb.Payload)
	}
	return payloads, nil
}

func (d *memoryDriver) storeCalls() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.stores
}

func (d *memoryDriver) retrieveCalls() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.retrieves
}
