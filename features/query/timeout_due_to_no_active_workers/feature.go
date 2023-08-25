package timeout_due_to_no_active_workers

import (
	"context"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows: Workflow,
	CheckResult: func(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
		// Shut off the worker
		r.StopWorker()

		oneSecCtx, cf := context.WithDeadline(ctx, time.Now().Add(time.Second))
		defer cf()
		_, err := r.Client.QueryWorkflow(oneSecCtx, run.GetID(), run.GetRunID(), "somequery")
		r.Require.Error(err)
		_, ok := err.(*serviceerror.DeadlineExceeded)
		r.Require.True(ok)

		err = r.StartWorker()
		if err != nil {
			return err
		}

		// Finish the wf
		err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "finish", nil)
		r.Require.NoError(err)
		return r.CheckResultDefault(ctx, run)
	},
}

func Workflow(ctx workflow.Context) error {
	err := workflow.SetQueryHandler(ctx, "somequery", func() (bool, error) { return true, nil })
	if err != nil {
		return err
	}

	sigChan := workflow.GetSignalChannel(ctx, "finish")
	sigChan.Receive(ctx, nil)

	return nil
}
