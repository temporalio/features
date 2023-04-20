package versioned_worker_polls_unversioned

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var encounteredError error

var Feature = harness.Feature{
	Workflows: Workflow,
	Execute:   Execute,
	WorkerOptions: worker.Options{
		OnFatalError: func(err error) {
			encounteredError = err
		},
		BuildIDForVersioning: "wahoo",
	},
}

func Execute(_ context.Context, _ *harness.Runner) (client.WorkflowRun, error) {
	// wait a beat to ensure we've had a chance to poll
	time.Sleep(100 * time.Millisecond)

	if encounteredError == nil {
		return nil, errors.New("Worker is expected to fail polling")
	}

	return nil, nil
}

func Workflow(_ workflow.Context) error {
	return nil
}
