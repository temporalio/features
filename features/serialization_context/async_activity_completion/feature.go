package async_activity_completion

import (
	"context"
	"sync"
	"time"

	"github.com/temporalio/features/features/serialization_context/sercontext"
	"github.com/temporalio/features/harness/go/harness"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const (
	activityResult = "completed-out-of-band"
	heartbeatData  = "beat"
)

// scheduledActivity is what the activity worker saw, used by the completing
// client to reconstruct the same activity serialization context.
type scheduledActivity struct {
	workflowID   string
	runID        string
	activityID   string
	activityType string
	workflowType string
	taskQueue    string
}

var (
	scheduledLock sync.Mutex
	scheduled     *scheduledActivity
)

var Feature = harness.Feature{
	Workflows:     Workflow,
	Activities:    Activity,
	ClientOptions: sercontext.ClientOptions(),
	Execute:       Execute,
	CheckResult:   CheckResult,
	CheckHistory:  harness.NoHistoryCheck,
}

func Workflow(ctx workflow.Context) (string, error) {
	opts := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		HeartbeatTimeout:    30 * time.Second,
	}
	var result string
	err := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, opts), Activity).Get(ctx, &result)
	return result, err
}

// Activity hands its identity to the test and lets the client complete it.
func Activity(ctx context.Context) (string, error) {
	info := activity.GetInfo(ctx)
	scheduledLock.Lock()
	scheduled = &scheduledActivity{
		workflowID:   info.WorkflowExecution.ID,
		runID:        info.WorkflowExecution.RunID,
		activityID:   info.ActivityID,
		activityType: info.ActivityType.Name,
		workflowType: info.WorkflowType.Name,
		taskQueue:    info.TaskQueue,
	}
	scheduledLock.Unlock()
	return "", activity.ErrResultPending
}

func Execute(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
	run, err := runner.ExecuteDefault(ctx)
	if err != nil {
		return nil, err
	}

	var pending *scheduledActivity
	err = runner.DoUntilEventually(ctx, 100*time.Millisecond, 30*time.Second, func() bool {
		scheduledLock.Lock()
		defer scheduledLock.Unlock()
		pending = scheduled
		return pending != nil
	})
	if err != nil {
		return nil, err
	}

	// Without the *WithOptions variants the client has no workflow ID or activity
	// type to build an activity serialization context from.
	err = runner.Client.RecordActivityHeartbeatByIDWithOptions(ctx, client.RecordActivityHeartbeatByIDOptions{
		Namespace:    runner.Namespace,
		WorkflowID:   pending.workflowID,
		RunID:        pending.runID,
		ActivityID:   pending.activityID,
		ActivityType: pending.activityType,
		WorkflowType: pending.workflowType,
		TaskQueue:    pending.taskQueue,
		Details:      []interface{}{heartbeatData},
	})
	if err != nil {
		return nil, err
	}

	err = runner.Client.CompleteActivityByIDWithOptions(ctx, client.CompleteActivityByIDOptions{
		Namespace:    runner.Namespace,
		WorkflowID:   pending.workflowID,
		RunID:        pending.runID,
		ActivityID:   pending.activityID,
		ActivityType: pending.activityType,
		WorkflowType: pending.workflowType,
		TaskQueue:    pending.taskQueue,
		Result:       activityResult,
	})
	if err != nil {
		return nil, err
	}
	return run, nil
}

func CheckResult(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
	var result string
	if err := run.Get(ctx, &result); err != nil {
		return err
	}
	runner.Require.Equal(activityResult, result)

	scheduledLock.Lock()
	pending := scheduled
	scheduledLock.Unlock()

	events, err := sercontext.Events(ctx, runner.Client, run.GetID(), run.GetRunID())
	if err != nil {
		return err
	}
	completed, err := sercontext.FindEvent(events, "ActivityTaskCompleted", func(e *historypb.HistoryEvent) bool {
		return e.GetActivityTaskCompletedEventAttributes() != nil
	})
	if err != nil {
		return err
	}
	runner.Require.Equal(
		sercontext.ActivitySignature(
			runner.Namespace,
			pending.workflowID,
			pending.workflowType,
			pending.activityType,
			pending.taskQueue,
			false,
		),
		sercontext.FirstSignature(completed.GetActivityTaskCompletedEventAttributes().GetResult()),
	)

	return nil
}
