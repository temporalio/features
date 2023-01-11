package unexpected_arguments

import (
	"context"
	"fmt"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows: Workflow,
	CheckResult: func(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
		// Go does not reject anything

		var qRes string

		q, err := r.Client.QueryWorkflow(ctx, run.GetID(), run.GetRunID(), "theQuery", 123)
		r.Require.NoError(err)
		r.Require.NoError(q.Get(&qRes))
		r.Require.Equal("got 123", qRes)

		// Drops extra arg
		q, err = r.Client.QueryWorkflow(ctx, run.GetID(), run.GetRunID(), "theQuery", 123, true)
		r.Require.NoError(err)
		r.Require.NoError(q.Get(&qRes))
		r.Require.Equal("got 123", qRes)

		// Assumes default value
		q, err = r.Client.QueryWorkflow(ctx, run.GetID(), run.GetRunID(), "theQuery")
		r.Require.NoError(err)
		r.Require.NoError(q.Get(&qRes))
		r.Require.Equal("got 0", qRes)

		err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "finish", nil)
		r.Require.NoError(err)
		return r.CheckResultDefault(ctx, run)
	},
}

func Workflow(ctx workflow.Context) error {
	err := workflow.SetQueryHandler(ctx, "theQuery", func(a int) (string, error) {
		return fmt.Sprintf("got %d", a), nil
	})
	if err != nil {
		return err
	}

	sigChan := workflow.GetSignalChannel(ctx, "finish")
	sigChan.Receive(ctx, nil)

	return nil
}
