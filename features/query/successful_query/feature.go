package successful_query

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
		q1, err := r.Client.QueryWorkflow(ctx, run.GetID(), run.GetRunID(), "counterQ")
		r.Assert.NoError(err)
		r.Assert.NoError(q1.Get(&qRes))
		r.Require.Equal(0, qRes)
		err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "counterInc", nil)
		r.Assert.NoError(err)
		q2, err := r.Client.QueryWorkflow(ctx, run.GetID(), run.GetRunID(), "counterQ")
		r.Assert.NoError(err)
		r.Assert.NoError(q2.Get(&qRes))
		r.Require.Equal(1, qRes)
		err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "counterInc", nil)
		r.Assert.NoError(err)
		err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "counterInc", nil)
		r.Assert.NoError(err)
		err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "counterInc", nil)
		r.Assert.NoError(err)
		q3, err := r.Client.QueryWorkflow(ctx, run.GetID(), run.GetRunID(), "counterQ")
		r.Assert.NoError(err)
		r.Assert.NoError(q3.Get(&qRes))
		r.Require.Equal(4, qRes)
		err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "counterInc", nil)
		r.Assert.NoError(err)
		return r.CheckResultDefault(ctx, run)
	},
}

func Workflow(ctx workflow.Context) error {
	counter := 0
	err := workflow.SetQueryHandler(ctx, "counterQ", func() (int, error) { return counter, nil })
	if err != nil {
		return err
	}

	sigChan := workflow.GetSignalChannel(ctx, "counterInc")
	for i := 0; i < 5; i++ {
		sigChan.Receive(ctx, nil)
		counter += 1
	}

	return nil
}
