package io.temporal.sdkfeatures;

import com.google.common.base.Preconditions;
import com.google.common.io.Resources;
import com.google.gson.Gson;
import com.google.gson.JsonElement;
import com.uber.m3.tally.NoopScope;
import com.uber.m3.tally.Scope;
import io.temporal.api.common.v1.WorkflowExecution;
import io.temporal.api.history.v1.History;
import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.client.WorkflowOptions;
import io.temporal.internal.client.WorkflowClientHelper;
import io.temporal.internal.common.WorkflowExecutionHistory;
import io.temporal.serviceclient.WorkflowServiceStubs;
import io.temporal.serviceclient.WorkflowServiceStubsOptions;
import io.temporal.worker.Worker;
import io.temporal.worker.WorkerFactory;
import io.temporal.worker.WorkerFactoryOptions;
import io.temporal.worker.WorkerOptions;
import org.reflections.Reflections;
import org.reflections.scanners.Scanners;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.Closeable;
import java.nio.charset.StandardCharsets;
import java.util.HashMap;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

public class Runner implements Closeable {
  private static final Logger log = LoggerFactory.getLogger(Main.class);

  public static class Config {
    public String serverHostPort;
    public String namespace;
    public String taskQueue;
    public Scope metricsScope = new NoopScope();
  }

  public final Config config;
  public final PreparedFeature featureInfo;
  public final Feature feature;
  public final WorkflowServiceStubs service;
  public final WorkflowClient client;
  public final WorkerFactory workerFactory;
  public final Worker worker;

  Runner(Config config, PreparedFeature featureInfo) {
    Objects.requireNonNull(config.serverHostPort);
    Objects.requireNonNull(config.namespace);
    Objects.requireNonNull(config.taskQueue);
    this.config = config;
    this.featureInfo = featureInfo;
    feature = featureInfo.newInstance();

    // Build service
    var serviceBuild = WorkflowServiceStubsOptions.newBuilder()
            .setTarget(config.serverHostPort).setMetricsScope(config.metricsScope);
    feature.workflowServiceOptions(serviceBuild);
    service = WorkflowServiceStubs.newInstance(serviceBuild.build());
    // Shutdown service on failure
    try {
      // Build client
      var clientBuild = WorkflowClientOptions.newBuilder()
              .setNamespace(config.namespace);
      feature.workflowClientOptions(clientBuild);
      client = WorkflowClient.newInstance(service, clientBuild.build());

      // Build worker
      var factoryBuild = WorkerFactoryOptions.newBuilder();
      feature.workerFactoryOptions(factoryBuild);
      workerFactory = WorkerFactory.newInstance(client, factoryBuild.build());
      var workerBuild = WorkerOptions.newBuilder();
      feature.workerOptions(workerBuild);
      worker = workerFactory.newWorker(config.taskQueue, workerBuild.build());

      // Register workflow class and activity impl
      worker.registerWorkflowImplementationTypes(featureInfo.factoryClass);
      worker.registerActivitiesImplementations(feature);

      // Start the worker factory
      workerFactory.start();
    } catch (Throwable e) {
      service.shutdownNow();
      throw e;
    }
  }

  void run() throws Exception {
    log.info("Executing feature {}", featureInfo.dir);
    var run = feature.execute(this);

    log.info("Checking result of feature {}", featureInfo.dir);
    feature.checkResult(this, run);

    feature.checkHistory(this, run);
  }

  public Run executeSingleParameterlessWorkflow() {
    // Find single workflow method or fail if multiple
    var methods = featureInfo.metadata.getWorkflowMethods();
    Preconditions.checkState(methods.size() == 1,
            "expected only one workflow method, got %s", methods.size());

    // Expect no parameters
    var reflectMethod = methods.get(0).getWorkflowMethod();
    Preconditions.checkState(reflectMethod.getParameterCount() == 0,
            "expected no parameters, got %s", reflectMethod.getParameterCount());

    // Call
    return new Run(methods.get(0), executeWorkflow(methods.get(0).getName()));
  }

  public Object waitForRunResult(Run run) {
    return waitForRunResult(run, run.method.getWorkflowMethod().getReturnType());
  }

  public <T> T waitForRunResult(Run run, Class<T> type) {
    var stub = client.newUntypedWorkflowStub(run.execution, Optional.empty());
    return stub.getResult(type);
  }

  public WorkflowExecution executeWorkflow(String workflowType, Object... args) {
    var stub = client.newUntypedWorkflowStub(workflowType,
            WorkflowOptions.newBuilder().setTaskQueue(config.taskQueue).build());
    return stub.start(args);
  }

  public void checkCurrentAndPastHistories(Run run) throws Exception {
    // Obtain the current history and run it through replay
    log.info("Checking current history");
    var eventIter = WorkflowClientHelper.getHistory(service, config.namespace, run.execution, config.metricsScope);
    var currentHistory = History.newBuilder().addAllEvents(() -> eventIter).build();
    worker.replayWorkflowExecution(new WorkflowExecutionHistory(currentHistory));

    // Replay each history
    for (var entry : loadPastHistories().entrySet()) {
      log.info("Checking history for version {}", entry.getKey());
      for (var history : entry.getValue()) {
        try {
          worker.replayWorkflowExecution(history);
        } catch (Exception e) {
          throw new RuntimeException("history for version " + entry.getKey() + " failed", e);
        }
      }
    }
  }

  @SuppressWarnings("UnstableApiUsage")
  public Map<Version, WorkflowExecutionHistory[]> loadPastHistories() throws Exception {
    var pkg = featureInfo.dir.replace('/', '.') + ".history";
    var jsonPaths = new Reflections(pkg, Scanners.Resources).getResources(".*\\.json");
    var pastHistories = new HashMap<Version, WorkflowExecutionHistory[]>();
    var gson = new Gson();
    for (var jsonPath : jsonPaths) {
      // Get filename
      var jsonFile = jsonPath.substring(jsonPath.lastIndexOf('/') + 1);

      // We only care about Java ones
      if (!jsonFile.startsWith("history.java.") || !jsonFile.endsWith(".json")) {
        continue;
      }

      // Get version
      Version version;
      try {
        version = new Version(jsonFile.substring("history.java.".length(), jsonFile.length() - ".json".length()));
      } catch (Exception e) {
        throw new RuntimeException("file " + jsonPath + " has invalid version", e);
      }

      // We only care about versions that are <= this one
      if (version.compareTo(Version.SDK) > 0) {
        continue;
      }

      // Read file
      try {
        var str = Resources.toString(Resources.getResource(jsonPath), StandardCharsets.UTF_8);
        // Read into list of elements
        var raw = gson.fromJson(str, JsonElement[].class);
        // Read each element
        var histories = new WorkflowExecutionHistory[raw.length];
        for (int i = 0; i < raw.length; i++) {
          histories[i] = WorkflowExecutionHistory.fromJson(raw[i].toString());
        }
        pastHistories.put(version, histories);
      } catch (Exception e) {
        throw new RuntimeException("file " + jsonPath + " has invalid JSON", e);
      }
    }
    return pastHistories;
  }

  public void close() {
    try {
      workerFactory.shutdownNow();
    } catch (Throwable e) {
      try {
        service.shutdownNow();
      } catch (Throwable ignored) {
      }
      throw e;
    }
    service.shutdownNow();
  }
}
