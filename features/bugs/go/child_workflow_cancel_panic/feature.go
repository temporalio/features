package child_workflow_cancel_panic

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk-features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"golang.org/x/mod/semver"
)

var Feature = harness.Feature{
	Workflows:    []interface{}{Workflow, workflow.Sleep},
	Activities:   DoNothing,
	Execute:      Execute,
	CheckResult:  CheckResult,
	CheckHistory: CheckHistory,
}

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	// Start workflow
	run, err := r.ExecuteDefault(ctx)
	if err != nil {
		return nil, err
	}

	// Wait a few seconds for query to say it's ready for cancel
	err = r.QueryUntilEventually(ctx, run, "waiting-for-cancel", true, 50*time.Millisecond, 5*time.Second)
	if err != nil {
		return nil, err
	}

	// Send a cancel
	err = r.Client.CancelWorkflow(ctx, run.GetID(), run.GetRunID())
	if err != nil {
		return nil, err
	}
	return run, nil
}

func CheckResult(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	// Through 1.11.1 this errors
	if semver.Compare(harness.SDKVersion, "v1.11.1") <= 0 {
		// TODO(cretz): On some versions this is slow due to some internal retry (is
		// it gRPC? Consider overwriting when
		// https://github.com/temporalio/sdk-go/pull/651 merged)
		err := run.Get(ctx, nil)
		if !temporal.IsPanicError(err) {
			return fmt.Errorf("expected panic error, got %w", err)
		}
		return nil
	}
	return r.CheckResultDefault(ctx, run)
}

func CheckHistory(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	// We do not check history on <= 1.11.1 because it panics
	if semver.Compare(harness.SDKVersion, "v1.11.1") <= 0 {
		return nil
	}
	return r.CheckHistoryDefault(ctx, run)
}

func Workflow(ctx workflow.Context) error {
	// Setup query to tell caller we're waiting for cancel
	waitingForCancel := false
	err := workflow.SetQueryHandler(ctx, "waiting-for-cancel", func() (bool, error) { return waitingForCancel, nil })
	if err != nil {
		return fmt.Errorf("failed setting query handler: %w", err)
	}

	// Start child but ignore future result
	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1},
	})
	workflow.ExecuteChildWorkflow(ctx, workflow.Sleep, 5*time.Hour)

	// Mark as waiting for cancel
	waitingForCancel = true

	// Receive from done channel
	// XXX: This is important, waiting for child future or anything else does not
	// trigger this
	ctx.Done().Receive(ctx, nil)

	// Run after-cancel activity
	ctx, _ = workflow.NewDisconnectedContext(ctx)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 5 * time.Minute})
	return workflow.ExecuteActivity(ctx, DoNothing).Get(ctx, nil)
}

func DoNothing(context.Context) error { return nil }
