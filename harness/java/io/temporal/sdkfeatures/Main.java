package io.temporal.sdkfeatures;

import static picocli.CommandLine.*;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.base.Verify;
import io.grpc.netty.shaded.io.netty.handler.ssl.SslContext;
import io.micrometer.core.instrument.util.StringUtils;
import io.temporal.serviceclient.SimpleSslContextBuilder;
import java.io.*;
import java.net.Socket;
import java.net.URI;
import java.util.Arrays;
import java.util.List;
import java.util.NoSuchElementException;
import javax.net.ssl.SSLException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import picocli.CommandLine;

@Command(name = "features", description = "Runs Java features")
public class Main implements Runnable {

  enum Outcome {
    PASSED,
    FAILED,
    SKIPPED;
  }

  private static class SummaryEntry {
    String name;
    String outcome;
    String message;

    public SummaryEntry(String name, String outcome, String message) {
      this.name = name;
      this.outcome = outcome;
      this.message = message;
    }

    public String getName() {
      return name;
    }

    public String getOutcome() {
      return outcome;
    }

    public String getMessage() {
      return message;
    }
  }

  BufferedWriter createSummaryServerWriter() {
    try {
      URI uri = new URI(summaryUri);
      switch (uri.getScheme()) {
        case "tcp":
          Socket socket = new Socket(uri.getHost(), uri.getPort());
          return new BufferedWriter(new OutputStreamWriter(socket.getOutputStream(), "UTF-8"));
        case "file":
          FileWriter fileWriter = new FileWriter(uri.getPath(), true);
          return new BufferedWriter(fileWriter);
        default:
          throw new IllegalArgumentException("unsupported summary scheme: " + uri.getScheme());
      }
    } catch (Exception e) {
      throw new RuntimeException(e);
    }
  }

  private static final Logger log = LoggerFactory.getLogger(Main.class);

  @Option(names = "--summary-uri", description = "The URL of the summary server", required = true)
  private String summaryUri;

  @Option(names = "--server", description = "The host:port of the server", required = true)
  private String server;

  @Option(names = "--namespace", description = "The namespace to use", required = true)
  private String namespace;

  @Option(names = "--client-cert-path", description = "Path to a client cert for TLS")
  private String clientCertPath;

  @Option(names = "--client-key-path", description = "Path to a client key for TLS")
  private String clientKeyPath;

  @Option(names = "--http-proxy-url", description = "URL for an HTTP CONNECT proxy to the server")
  private String httpProxyUrl;

  @Option(names = "--tls-server-name", description = "TLS server name to use for verification (optional)")
  private String tlsServerName;

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

    try (BufferedWriter writer = createSummaryServerWriter()) {
      ObjectMapper mapper = new ObjectMapper();

      // Run each
      // TODO(cretz): Concurrent with log capturing
      var failureCount = 0;
      var failedFeatures = new StringBuilder();
      for (var featureWithTaskQueue : features) {
        var pieces = featureWithTaskQueue.split(":", 2);
        // Find feature
        var feature =
            Arrays.stream(PreparedFeature.ALL)
                .filter(p -> p.dir.equals(pieces[0]))
                .findAny()
                .orElseThrow(
                    () ->
                        new NoSuchElementException(
                            "feature "
                                + pieces[0]
                                + " not found. Make sure you add it to PreparedFeature.ALL"));

        log.info("Running feature {}", feature.dir);
        var config = new Runner.Config();
        config.serverHostPort = server;
        config.namespace = namespace;
        config.httpProxyUrl = httpProxyUrl;
        config.sslContext = sslContext;
        config.tlsServerName = tlsServerName;
        config.taskQueue = pieces[1];
        Outcome outcome = Outcome.PASSED;
        String message = "";
        try {
          try (var runner = new Runner(config, feature)) {
            runner.run();
          } catch (TestSkippedException e) {
            outcome = Outcome.SKIPPED;
            message = e.getMessage();
            log.info("Skipping feature {} because {}", feature.dir, e.getMessage());
          }
        } catch (Exception e) {
          outcome = Outcome.FAILED;
          message = e.getMessage();
          failureCount++;
          log.error("Feature {} failed", feature.dir, e);
          failedFeatures.append("\n").append(feature.dir).append(": ").append(e.getMessage());
        }
        try {
          String jsonInString =
              mapper.writeValueAsString(new SummaryEntry(feature.dir, outcome.toString(), message));
          writer.write(jsonInString + "\n");
        } catch (IOException e) {
          throw new RuntimeException(e);
        }
      }
      Verify.verify(
          failureCount == 0, "%s feature(s) failed: %s", failureCount, failedFeatures.toString());
    } catch (IOException e) {
      throw new RuntimeException(e);
    }
  }

  public static void main(String... args) {
    System.exit(new CommandLine(new Main()).execute(args));
  }
}
