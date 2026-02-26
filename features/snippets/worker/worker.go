package worker

import (
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

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