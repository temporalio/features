package io.temporal.sdkfeatures;

import com.google.common.base.Verify;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import io.grpc.netty.shaded.io.netty.handler.ssl.SslContext;
import io.micrometer.core.instrument.util.StringUtils;
import io.temporal.serviceclient.SimpleSslContextBuilder;
import picocli.CommandLine;

import java.io.FileInputStream;
import java.io.FileNotFoundException;
import java.io.InputStream;
import java.util.Arrays;
import java.util.List;
import java.util.NoSuchElementException;

import static picocli.CommandLine.*;

import javax.net.ssl.SSLException;

@Command(name = "sdk-features", description = "Runs Java features")
public class Main implements Runnable {

  private static final Logger log = LoggerFactory.getLogger(Main.class);

  @Option(names = "--server", description = "The host:port of the server", required = true)
  private String server;

  @Option(names = "--namespace", description = "The namespace to use", required = true)
  private String namespace;

  @Option(names = "--client-cert-path", description = "Path to a client cert for TLS")
  private String clientCertPath;

  @Option(names = "--client-key-path", description = "Path to a client key for TLS")
  private String clientKeyPath;

  @Parameters(description = "Features as dir + ':' + task queue")
  private List<String> features;

  @Override
  public void run() {
    // Load TLS certs if specified
    SslContext sslContext = null;
    if (StringUtils.isNotEmpty(clientCertPath)) {
      if (StringUtils.isEmpty(clientKeyPath)) {
        throw new RuntimeException("Client key path must be specified since cert path is");
      }

      try {
        InputStream clientCert = new FileInputStream(clientCertPath);
        InputStream clientKey = new FileInputStream(clientKeyPath);
        sslContext = SimpleSslContextBuilder.forPKCS8(clientCert, clientKey).build();
      } catch (FileNotFoundException | SSLException e) {
        throw new RuntimeException("Error loading certs", e);
      }

    } else if (StringUtils.isNotEmpty(clientKeyPath) && StringUtils.isEmpty(clientCertPath)) {
      throw new RuntimeException("Client cert path must be specified since key path is");
    }


    // Run each
    // TODO(cretz): Concurrent with log capturing
    var failureCount = 0;
    var failedFeatures = new StringBuilder();
    for (var featureWithTaskQueue : features) {
      var pieces = featureWithTaskQueue.split(":", 2);
      // Find feature
      var feature = Arrays.stream(PreparedFeature.ALL)
          .filter(p -> p.dir.equals(pieces[0]))
          .findAny()
          .orElseThrow(() -> new NoSuchElementException(
              "feature " + pieces[0] + " not found. Make sure you add it to PreparedFeature.ALL"));

      log.info("Running feature {}", feature.dir);
      var config = new Runner.Config();
      config.serverHostPort = server;
      config.namespace = namespace;
      config.sslContext = sslContext;
      config.taskQueue = pieces[1];
      try {
        try (var runner = new Runner(config, feature)) {
          runner.run();
        }
      } catch (Exception e) {
        failureCount++;
        log.error("Feature {} failed", feature.dir, e);
        failedFeatures.append("\n").append(feature.dir).append(": ").append(e.getMessage());
      }
    }
    Verify.verify(failureCount == 0, "%s feature(s) failed: %s",
        failureCount, failedFeatures.toString());
  }

  public static void main(String... args) {
    System.exit(new CommandLine(new Main()).execute(args));
  }
}
