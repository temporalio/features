package unexpected_query_type_name

import (
	"context"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk-features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows: Workflow,
	CheckResult: func(ctx context.Context, r *harness.Runner, run client.WorkflowRun) error {
		_, err := r.Client.QueryWorkflow(ctx, run.GetID(), run.GetRunID(), "nonexistent")
		r.Require.Error(err)
		r.Require.IsType(err, &serviceerror.QueryFailed{})

		err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "finish", nil)
		r.Require.NoError(err)
		return r.CheckResultDefault(ctx, run)
	},
}

func Workflow(ctx workflow.Context) error {
	sigChan := workflow.GetSignalChannel(ctx, "finish")
	sigChan.Receive(ctx, nil)

	return nil
}
