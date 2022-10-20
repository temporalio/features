package unexpected_return_type

import (
	"context"
	"go.temporal.io/sdk-features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows: Workflow,
	CheckResult: func(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {

		var qRes int

		q, err := r.Client.QueryWorkflow(ctx, run.GetID(), run.GetRunID(), "theQuery")
		r.Require.NoError(err)
		r.Require.Error(q.Get(&qRes))

		err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "finish", nil)
		r.Require.NoError(err)
		return r.CheckResultDefault(ctx, run)
	},
}

func Workflow(ctx workflow.Context) error {
	err := workflow.SetQueryHandler(ctx, "theQuery", func() (string, error) {
		return "hi bob", nil
	})
	if err != nil {
		return err
	}

	sigChan := workflow.GetSignalChannel(ctx, "finish")
	sigChan.Receive(ctx, nil)

	return nil
}
