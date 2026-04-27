package threshold

import (
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
)

func ThresholdConfig(driver converter.StorageDriver) {
	// @@@SNIPSTART go-external-storage-threshold
	c, err := client.Dial(client.Options{
		ExternalStorage: converter.ExternalStorage{
			Drivers:              []converter.StorageDriver{driver},
			PayloadSizeThreshold: 1,
		},
	})
	// @@@SNIPEND
	if err != nil {
		log.Fatalf("connect to Temporal: %v", err)
	}
	defer c.Close()
}
