package server_down_between_dial_and_start_workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/temporalio/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows: Workflow,
	Execute: func(ctx context.Context, runner *harness.Runner) (client.WorkflowRun, error) {
		c, err := client.Dial(runner.Feature.ClientOptions)
		if err != nil {
			return nil, fmt.Errorf("failed creating client: %w", err)
		}
		defer c.Close()

		if err := runner.ProxyStop(ctx); err != nil {
			return nil, err
		}

		var wg sync.WaitGroup
		wg.Add(1)
		defer wg.Wait()
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Second)
			_ = runner.ProxyStart(ctx)
		}()

		opts := client.StartWorkflowOptions{
			TaskQueue:                runner.TaskQueue,
			WorkflowExecutionTimeout: 1 * time.Minute,
		}
		return c.ExecuteWorkflow(ctx, opts, Workflow)
	},
}

func Workflow(ctx workflow.Context) (string, error) {
	return "OK", nil
}
