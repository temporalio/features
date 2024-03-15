package server_frozen_between_start_worker_and_start_workflow

import (
	"context"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows: Workflow,
	BeforeWorkerStart: func(runner *harness.Runner) error {
		ctx := context.Background()

		if err := runner.ProxyFreeze(ctx); err != nil {
			return err
		}

		go func() {
			time.Sleep(10 * time.Second)
			_ = runner.ProxyThaw(ctx)
		}()

		return nil
	},
}

func Workflow(ctx workflow.Context) (string, error) {
	return "OK", nil
}
