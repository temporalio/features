package worker

import (
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

func HelloWorkflow(ctx workflow.Context, name string) (string, error) {
	return "Hello, " + name + "!", nil
}

func Run() error {
	c, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
	})
	if err != nil {
		return err
	}
	defer c.Close()

	// @@@SNIPSTART go-worker-max-cached-workflows
	worker.SetStickyWorkflowCacheSize(0)
	w := worker.New(c, "task-queue", worker.Options{})
	// @@@SNIPEND

	return w.Run(worker.InterruptCh())
}

func RunVersioned() error {
	c, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
	})
	if err != nil {
		return err
	}
	defer c.Close()

	// @@@SNIPSTART go-versioned-worker
	w := worker.New(c, "my-task-queue", worker.Options{
		DeploymentOptions: worker.DeploymentOptions{
			UseVersioning: true,
			Version: worker.WorkerDeploymentVersion{
				DeploymentName: "my-app",
				BuildID:        "1.0",
			},
		},
	})

	w.RegisterWorkflowWithOptions(HelloWorkflow, workflow.RegisterOptions{
		VersioningBehavior: workflow.VersioningBehaviorPinned,
	})
	// @@@SNIPEND

	return w.Run(worker.InterruptCh())
}
