package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows: Workflow,
	StartWorkflowOptionsMutator: func(o *client.StartWorkflowOptions) {
		o.CronSchedule = "@every 2s"
	},
	CheckResult: CheckResult,
	// Disable history check because we can't guarantee cron execution times
	CheckHistory: harness.NoHistoryCheck,
}

func Workflow(ctx workflow.Context) (string, error) {
	if workflow.GetInfo(ctx).CronSchedule != "@every 2s" {
		return "", fmt.Errorf("invalid cron schedule")
	}
	return "", nil
}

func CheckResult(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
	defer r.Client.TerminateWorkflow(ctx, run.GetID(), "", "feature complete")
	// Try 10 times every 1s to get at least two workflow executions
	var lastResp *workflowservice.ListWorkflowExecutionsResponse
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		var err error
		lastResp, err = r.Client.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
			Query: fmt.Sprintf("WorkflowId = '%v'", run.GetID()),
		})
		r.Require.NoError(err)
		// Need at least two completed executions and no failures
		completed := 0
		for _, exec := range lastResp.GetExecutions() {
			if exec.Status == enums.WORKFLOW_EXECUTION_STATUS_COMPLETED {
				completed++
			} else {
				r.Require.Equal(enums.WORKFLOW_EXECUTION_STATUS_RUNNING, exec.Status)
			}
		}
		if completed >= 2 {
			lastResp = nil
			break
		}
	}
	r.Require.Nil(lastResp, "expected >= 2 completed executions")
	return nil
}
