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
		q1, err := r.Client.QueryWorkflow(ctx, run.GetID(), run.GetRunID(), "counterQ")
		r.Assert.Nil(err)
		r.Assert.Equal(0, q1)
		err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "counterInc", nil)
		r.Assert.Nil(err)
		q2, err := r.Client.QueryWorkflow(ctx, run.GetID(), run.GetRunID(), "counterQ")
		r.Assert.Nil(err)
		r.Assert.Equal(1, q2)
		err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "counterInc", nil)
		r.Assert.Nil(err)
		err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "counterInc", nil)
		r.Assert.Nil(err)
		err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "counterInc", nil)
		r.Assert.Nil(err)
		q3, err := r.Client.QueryWorkflow(ctx, run.GetID(), run.GetRunID(), "counterQ")
		r.Assert.Nil(err)
		// TODO: Something isn't right with go runner. This should fail and it does not
		r.Assert.Equal(50, q3)
		err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "counterInc", nil)
		r.Assert.Nil(err)
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
