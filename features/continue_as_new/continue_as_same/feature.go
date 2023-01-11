package continue_as_same

import (
	"context"
	"time"

	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/workflow"
)

const (
	InputData  = "InputData"
	MemoKey    = "MemoKey"
	MemoValue  = "MemoValue"
	WorkflowID = "TestID"
)

var Feature = harness.Feature{
	Workflows: Workflow,
	CheckResult: func(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
		var result string
		if err := run.Get(ctx, &result); err != nil {
			return err
		}
		r.Require.Equal(InputData, result)

		execution, err := r.Client.DescribeWorkflowExecution(ctx, run.GetID(), run.GetRunID())
		if err != nil {
			return err
		}
		// Workflow ID does not change after continue as new
		r.Require.Equal(WorkflowID, run.GetID())
		// Memos do not change after continue as new
		memo := execution.GetWorkflowExecutionInfo().GetMemo().GetFields()
		r.Require.NotNil(memo)
		memoPayload, ok := memo[MemoKey]
		r.Require.True(ok)
		var testMemo string
		err = converter.GetDefaultDataConverter().FromPayload(memoPayload, &testMemo)
		if err != nil {
			return err
		}
		r.Require.Equal(MemoValue, testMemo)
		return nil
	},
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		opts := client.StartWorkflowOptions{
			TaskQueue: runner.TaskQueue,
			ID:        WorkflowID,
			Memo: map[string]interface{}{
				MemoKey: MemoValue,
			},
			WorkflowExecutionTimeout: 1 * time.Minute,
		}
		return runner.Client.ExecuteWorkflow(ctx, opts, runner.Feature.Workflows[0], InputData)
	},
}

// Workflow waits for a single signal and returns the data contained therein
func Workflow(ctx workflow.Context, input string) (string, error) {
	// check if the workflow execution was started by continue as new
	if workflow.GetInfo(ctx).ContinuedExecutionRunID != "" {
		return input, nil
	}
	return "", workflow.NewContinueAsNewError(ctx, Workflow, input)
}
