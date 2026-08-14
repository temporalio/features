package gcssetup

import (
	"context"
	"log"

	"cloud.google.com/go/storage"
	"go.temporal.io/sdk/contrib/gcp/gcsdriver"
	"go.temporal.io/sdk/contrib/gcp/gcsdriver/gcssdk"
	"go.temporal.io/sdk/converter"
)

func CreateGCSDriver() converter.StorageDriver {
	// @@@SNIPSTART go-gcs-driver-create
	gcsClient, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("create GCS client: %v", err)
	}

	driver, err := gcsdriver.NewDriver(gcsdriver.Options{
		Client: gcssdk.NewClient(gcsClient),
		Bucket: gcsdriver.StaticBucket("my-temporal-payloads"),
	})
	if err != nil {
		log.Fatalf("create GCS driver: %v", err)
	}
	// @@@SNIPEND
	return driver
}
