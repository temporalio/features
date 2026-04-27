package s3setup

import (
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/worker"
)

func SetupExternalStorage(driver converter.StorageDriver) {
	// @@@SNIPSTART go-s3-external-storage-setup
	c, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
		ExternalStorage: converter.ExternalStorage{
			Drivers: []converter.StorageDriver{driver},
		},
	})
	if err != nil {
		log.Fatalf("connect to Temporal: %v", err)
	}
	defer c.Close()

	w := worker.New(c, "my-task-queue", worker.Options{})
	// @@@SNIPEND
	_ = w
}
