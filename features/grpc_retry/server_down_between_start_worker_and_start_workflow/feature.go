package server_down_between_start_worker_and_start_workflow

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

		if err := runner.ProxyStop(ctx); err != nil {
			return err
		}

		if err := runner.ProxyKillAll(ctx); err != nil {
			return err
		}

		go func() {
			time.Sleep(10 * time.Second)
			_ = runner.ProxyStart(ctx)
		}()

		return nil
	},
}

func Workflow(ctx workflow.Context) (string, error) {
	return "OK", nil
}
