import io.temporal.payload.storage.s3driver.S3StorageDriver;
import io.temporal.payload.storage.s3driver.awssdkv2.S3AsyncClientAdapter;
import software.amazon.awssdk.regions.Region;
import software.amazon.awssdk.services.s3.S3AsyncClient;
import software.amazon.awssdk.services.s3.S3Configuration;

class MrapDriverCreateSnippet {
  static S3StorageDriver createDriver() {
    // @@@SNIPSTART java-s3-mrap-driver-create
    S3AsyncClient s3Client =
        S3AsyncClient.builder()
            .region(Region.US_EAST_2)
            .serviceConfiguration(S3Configuration.builder().useArnRegionEnabled(true).build())
            .build();

    return S3StorageDriver.newBuilder()
        .setClient(new S3AsyncClientAdapter(s3Client))
        .setBucket("arn:aws:s3::123456789012:accesspoint/example.mrap")
        .build();
    // @@@SNIPEND
  }
}
