package s3setup

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.temporal.io/sdk/contrib/aws/s3driver"
	"go.temporal.io/sdk/contrib/aws/s3driver/awssdkv2"
	"go.temporal.io/sdk/converter"
)

func CreateS3Driver() converter.StorageDriver {
	// @@@SNIPSTART go-s3-driver-create
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-2"),
	)
	if err != nil {
		log.Fatalf("load AWS config: %v", err)
	}

	driver, err := s3driver.NewDriver(s3driver.Options{
		Client: awssdkv2.NewClient(s3.NewFromConfig(cfg)),
		Bucket: s3driver.StaticBucket("my-temporal-payloads"),
	})
	if err != nil {
		log.Fatalf("create S3 driver: %v", err)
	}
	// @@@SNIPEND
	return driver
}
