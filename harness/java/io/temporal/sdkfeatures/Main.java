package io.temporal.sdkfeatures;

import com.google.common.base.Verify;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import picocli.CommandLine;

import java.util.Arrays;
import java.util.List;
import java.util.NoSuchElementException;

import static picocli.CommandLine.*;

@Command(name = "sdk-features", description = "Runs Java features")
public class Main implements Runnable {

  private static final Logger log = LoggerFactory.getLogger(Main.class);

  @Option(names = "--server", description = "The host:port of the server", required = true)
  private String server;

  @Option(names = "--namespace", description = "The namespace to use", required = true)
  private String namespace;

  @Parameters(description = "Features as dir + ':' + task queue")
  private List<String> features;

  @Override
  public void run() {
    // Run each
    // TODO(cretz): Concurrent with log capturing
    var failureCount = 0;
    for (var featureWithTaskQueue : features) {
      var pieces = featureWithTaskQueue.split(":", 2);
      // Find feature
      var feature = Arrays.stream(PreparedFeature.ALL)
              .filter(p -> p.dir.equals(pieces[0]))
              .findAny()
              .orElseThrow(() -> new NoSuchElementException("feature " + pieces[0] + " not found"));

      log.info("Running feature {}", feature.dir);
      var config = new Runner.Config();
      config.serverHostPort = server;
      config.namespace = namespace;
      config.taskQueue = pieces[1];
      try {
        try (var runner = new Runner(config, feature)) {
          runner.run();
        }
      } catch (Exception e) {
        failureCount++;
        log.error("Feature {} failed", feature.dir, e);
      }
    }
    Verify.verify(failureCount == 0, "%s feature(s) failed", failureCount);
  }

  public static void main(String... args) {
    System.exit(new CommandLine(new Main()).execute(args));
  }
}
