package versions_added_while_worker_polling

import (
	"context"
	"errors"
	"time"

	"go.temporal.io/api/common/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/features/harness/go/harness"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var Feature = harness.Feature{
	Workflows:     Workflow,
	Execute:       Execute,
	WorkerOptions: worker.Options{BuildIDForVersioning: "1.0"},
}

func Execute(ctx context.Context, r *harness.Runner) (client.WorkflowRun, error) {
	// Add 1.0 to the queue
	err := r.Client.UpdateWorkerBuildIdCompatibility(ctx, &client.UpdateWorkerBuildIdCompatibilityOptions{
		TaskQueue:     r.TaskQueue,
		WorkerBuildID: "1.0",
		BecomeDefault: true,
	})
	if err != nil {
		return nil, err
	}

	// Start workflow & process a task
	run, err := r.ExecuteDefault(ctx)
	if err != nil {
		return nil, err
	}
	err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "add1", nil)
	if err != nil {
		return nil, err
	}
	err = r.QueryUntilEventually(ctx, run, "counter", 1, time.Millisecond*200, time.Second*2)
	if err != nil {
		return nil, err
	}

	// Stop worker & start 1.1 worker
	r.StopWorker()
	_, err = r.Client.WorkflowService().ResetStickyTaskQueue(ctx, &workflowservice.ResetStickyTaskQueueRequest{
		Namespace: r.Namespace,
		Execution: &common.WorkflowExecution{
			WorkflowId: run.GetID(),
			RunId:      run.GetRunID(),
		},
	})
	if err != nil {
		return nil, err
	}
	r.Feature.WorkerOptions.BuildIDForVersioning = "1.1"
	err = r.StartWorker()
	if err != nil {
		return nil, err
	}

	// Send another signal, see that worker gets it *after* we add 1.1 to compat set
	err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "add1", nil)
	if err != nil {
		return nil, err
	}
	// Signal should not have been seen yet
	err = r.QueryUntilEventually(ctx, run, "counter", 2, time.Millisecond*200, time.Second*2)
	if err == nil {
		return nil, errors.New("1.1 worker should not have seen task since 1.1 is not yet in sets")
	}

	err = r.Client.UpdateWorkerBuildIdCompatibility(ctx, &client.UpdateWorkerBuildIdCompatibilityOptions{
		TaskQueue:         r.TaskQueue,
		WorkerBuildID:     "1.1",
		CompatibleBuildID: "1.0",
		BecomeDefault:     true,
	})
	if err != nil {
		return nil, err
	}
	err = r.QueryUntilEventually(ctx, run, "counter", 2, time.Millisecond*200, time.Second*2)
	if err != nil {
		return nil, err
	}

	// Add 1.2 and see that no new tasks aren't received
	err = r.Client.UpdateWorkerBuildIdCompatibility(ctx, &client.UpdateWorkerBuildIdCompatibilityOptions{
		TaskQueue:     r.TaskQueue,
		WorkerBuildID: "1.2",
		BecomeDefault: true,
	})
	if err != nil {
		return nil, err
	}
	err = r.Client.SignalWorkflow(ctx, run.GetID(), run.GetRunID(), "add1", nil)
	if err != nil {
		return nil, err
	}
	err = r.QueryUntilEventually(ctx, run, "counter", 2, time.Millisecond*200, time.Second*2)
	if err == nil {
		return nil, errors.New("1.1 worker should not have seen task since 1.2 is now default")
	}
	// Stop and start at 1.2
	r.StopWorker()
	_, err = r.Client.WorkflowService().ResetStickyTaskQueue(ctx, &workflowservice.ResetStickyTaskQueueRequest{
		Namespace: r.Namespace,
		Execution: &common.WorkflowExecution{
			WorkflowId: run.GetID(),
			RunId:      run.GetRunID(),
		},
	})
	if err != nil {
		return nil, err
	}
	r.Feature.WorkerOptions.BuildIDForVersioning = "1.2"
	err = r.StartWorker()

	// Cancel workflow to end it
	err = r.Client.CancelWorkflow(ctx, run.GetID(), run.GetRunID())
	if err != nil {
		return nil, err
	}

	return run, nil
}

func Workflow(ctx workflow.Context) error {
	counter := 0
	addChan := workflow.GetSignalChannel(ctx, "add1")
	err := workflow.SetQueryHandler(ctx, "counter", func() int { return counter })
	if err != nil {
		return err
	}

	go func() {
		for addChan.Receive(ctx, nil) {
			counter += 1
		}
	}()

	ctx.Done().Receive(ctx, nil)

	return nil
}
