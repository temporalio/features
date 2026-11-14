package signal_with_start

import (
	"context"
	"fmt"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

const signalName = "add"

func CallerWorkflow(ctx workflow.Context, targetID string) (workflow.Execution, error) {
	options := workflow.SignalWithStartWorkflowOptions{
		TaskQueue:                workflow.GetInfo(ctx).TaskQueueName,
		WorkflowExecutionTimeout: time.Minute,
		WorkflowIDConflictPolicy: enumspb.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING,
	}

	var first workflow.Execution
	if err := workflow.SignalWithStartWorkflow(
		ctx, options, targetID, signalName, "signal-one", TargetWorkflow, "start-value",
	).Get(ctx, &first); err != nil {
		return workflow.Execution{}, err
	}
	if first.RunID == "" {
		return workflow.Execution{}, fmt.Errorf("first SignalWithStart returned no run ID")
	}

	var second workflow.Execution
	if err := workflow.SignalWithStartWorkflow(
		ctx, options, targetID, signalName, "signal-two", TargetWorkflow, "unused-start-value",
	).Get(ctx, &second); err != nil {
		return workflow.Execution{}, err
	}
	if second.RunID != first.RunID {
		return workflow.Execution{}, fmt.Errorf(
			"SignalWithStart calls targeted different runs: first=%q second=%q",
			first.RunID, second.RunID,
		)
	}
	return first, nil
}

func TargetWorkflow(ctx workflow.Context, value string) ([]string, error) {
	signals := workflow.GetSignalChannel(ctx, signalName)
	var first, second string
	signals.Receive(ctx, &first)
	signals.Receive(ctx, &second)
	return []string{"started: " + value, "signal: " + first, "signal: " + second}, nil
}

var Feature = harness.Feature{
	Workflows: []any{CallerWorkflow, TargetWorkflow},
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		targetID := "system-nexus-signal-with-start-" + runner.TaskQueue
		return runner.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: time.Minute,
		}, CallerWorkflow, targetID)
	},
	CheckResult: func(ctx context.Context, runner *harness.Runner, run client.WorkflowRun) error {
		var target workflow.Execution
		if err := run.Get(ctx, &target); err != nil {
			return err
		}
		var result []string
		if err := runner.Client.GetWorkflow(ctx, target.ID, target.RunID).Get(ctx, &result); err != nil {
			return err
		}
		runner.Require.Equal(
			[]string{"started: start-value", "signal: signal-one", "signal: signal-two"},
			result,
		)
		return nil
	},
}
