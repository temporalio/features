package parallel_sync_operations

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-rpc/sdk-go/nexus"
	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const ServiceName = "test-service"

var SyncOperation = nexus.NewSyncOperation(
	"say-hello",
	func(ctx context.Context, name string, options nexus.StartOperationOptions) (string, error) {
		return "Hello, " + name + "!", nil
	},
)

var Service = func() *nexus.Service {
	s := nexus.NewService(ServiceName)
	s.MustRegister(SyncOperation)
	return s
}()

var names = []string{"one", "two", "three"}

func Workflow(ctx workflow.Context, endpoint string) (string, error) {
	nc := workflow.NewNexusClient(endpoint, ServiceName)
	futures := make([]workflow.NexusOperationFuture, len(names))
	for i, name := range names {
		futures[i] = nc.ExecuteOperation(ctx, SyncOperation, name, workflow.NexusOperationOptions{
			ScheduleToCloseTimeout: time.Minute,
		})
	}
	results := make([]string, len(futures))
	for i, fut := range futures {
		if err := fut.Get(ctx, &results[i]); err != nil {
			return "", err
		}
	}
	return strings.Join(results, " "), nil
}

func scheduledWorkflowTaskIDs(hist client.HistoryEventIterator) ([]int64, error) {
	var taskIDs []int64
	for hist.HasNext() {
		ev, err := hist.Next()
		if err != nil {
			return nil, err
		}
		if attrs := ev.GetNexusOperationScheduledEventAttributes(); attrs != nil {
			taskIDs = append(taskIDs, attrs.WorkflowTaskCompletedEventId)
		}
	}
	return taskIDs, nil
}

var Feature = harness.Feature{
	Workflows:       Workflow,
	NexusServices:   Service,
	ExpectRunResult: "Hello, one! Hello, two! Hello, three!",
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		opts := client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}
		return runner.Client.ExecuteWorkflow(ctx, opts, Workflow, runner.NexusEndpoint)
	},
	CheckHistory: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		countEvents := func(t enumspb.EventType) (int, error) {
			hist := runner.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
			return harness.CountEvents(hist, func(ev *historypb.HistoryEvent) bool { return ev.EventType == t })
		}
		expected := map[enumspb.EventType]int{
			enumspb.EVENT_TYPE_NEXUS_OPERATION_COMPLETED: len(names),
			enumspb.EVENT_TYPE_NEXUS_OPERATION_STARTED:   0,
		}
		for t, want := range expected {
			got, err := countEvents(t)
			if err != nil {
				return err
			}
			if got != want {
				return fmt.Errorf("expected %v %v events, got %v", want, t, got)
			}
		}
		hist := runner.Client.GetWorkflowHistory(ctx, run.GetID(), run.GetRunID(), false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
		scheduledBy, err := scheduledWorkflowTaskIDs(hist)
		if err != nil {
			return err
		}
		if len(scheduledBy) != len(names) {
			return fmt.Errorf("expected %v scheduled operations, got %v", len(names), len(scheduledBy))
		}
		for _, id := range scheduledBy {
			if id != scheduledBy[0] {
				return fmt.Errorf("expected all operations to be scheduled by a single workflow task, got tasks %v", scheduledBy)
			}
		}
		return runner.CheckHistoryDefault(ctx, run)
	},
}
