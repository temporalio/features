import io.temporal.payload.storage.s3driver.S3StorageDriver;
import io.temporal.payload.storage.s3driver.awssdkv2.S3AsyncClientAdapter;
import software.amazon.awssdk.regions.Region;
import software.amazon.awssdk.services.s3.S3AsyncClient;

class S3DriverCreateSnippet {
  static S3StorageDriver createDriver() {
    // @@@SNIPSTART java-s3-driver-create
    S3AsyncClient s3Client = S3AsyncClient.builder().region(Region.US_EAST_2).build();

    S3StorageDriver driver =
        S3StorageDriver.newBuilder()
            .setClient(new S3AsyncClientAdapter(s3Client))
            .setBucket("my-temporal-payloads")
            .build();
    // @@@SNIPEND
    return driver;
  }
}
