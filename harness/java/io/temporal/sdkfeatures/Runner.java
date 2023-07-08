package io.temporal.sdkfeatures;

import com.google.common.base.Preconditions;
import com.google.common.io.Resources;
import com.google.gson.Gson;
import com.google.gson.JsonElement;
import com.uber.m3.tally.NoopScope;
import com.uber.m3.tally.Scope;
import io.grpc.StatusRuntimeException;
import io.grpc.netty.shaded.io.netty.handler.ssl.SslContext;
import io.temporal.activity.ActivityInterface;
import io.temporal.api.common.v1.Payload;
import io.temporal.api.common.v1.WorkflowExecution;
import io.temporal.api.history.v1.History;
import io.temporal.api.workflow.v1.WorkflowExecutionInfo;
import io.temporal.api.workflowservice.v1.DescribeWorkflowExecutionRequest;
import io.temporal.client.*;
import io.temporal.internal.client.WorkflowClientHelper;
import io.temporal.internal.common.WorkflowExecutionHistory;
import io.temporal.serviceclient.WorkflowServiceStubs;
import io.temporal.serviceclient.WorkflowServiceStubsOptions;
import io.temporal.worker.Worker;
import io.temporal.worker.WorkerFactory;
import io.temporal.worker.WorkerFactoryOptions;
import io.temporal.worker.WorkerOptions;
import java.io.Closeable;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.*;
import java.util.function.Supplier;
import org.reflections.Reflections;
import org.reflections.scanners.Scanners;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class Runner implements Closeable {
  private static final Logger log = LoggerFactory.getLogger(Main.class);

  public static class Config {
    public String serverHostPort;
    public String namespace;
    public String taskQueue;
    public Scope metricsScope = new NoopScope();
    public SslContext sslContext;
  }

  public final Config config;
  public final PreparedFeature featureInfo;
  public final Feature feature;
  public final WorkflowServiceStubs service;
  public final WorkflowClient client;
  private WorkerFactory workerFactory;
  private Worker worker;

  Runner(Config config, PreparedFeature featureInfo) {
    Objects.requireNonNull(config.serverHostPort);
    Objects.requireNonNull(config.namespace);
    Objects.requireNonNull(config.taskQueue);
    this.config = config;
    this.featureInfo = featureInfo;
    feature = featureInfo.newInstance();

    // Build service
    var serviceBuild =
        WorkflowServiceStubsOptions.newBuilder()
            .setTarget(config.serverHostPort)
            .setSslContext(config.sslContext)
            .setMetricsScope(config.metricsScope);
    feature.workflowServiceOptions(serviceBuild);
    service = WorkflowServiceStubs.newServiceStubs(serviceBuild.build());
    // Shutdown service on failure
    try {
      // Build client
      var clientBuild = WorkflowClientOptions.newBuilder().setNamespace(config.namespace);
      feature.workflowClientOptions(clientBuild);
      client = WorkflowClient.newInstance(service, clientBuild.build());

      // Build worker
      restartWorker();
    } catch (Throwable e) {
      service.shutdownNow();
      throw e;
    }
  }

  /**
   * Instantiates a new worker, replacing the existing worker and workerFactory. You should shut
   * down the worker factory before calling this.
   */
  public void restartWorker() {
    var factoryBuild = WorkerFactoryOptions.newBuilder();
    feature.workerFactoryOptions(factoryBuild);
    this.workerFactory = WorkerFactory.newInstance(client, factoryBuild.build());
    var workerBuild = WorkerOptions.newBuilder();
    feature.workerOptions(workerBuild);
    this.worker = workerFactory.newWorker(config.taskQueue, workerBuild.build());

    // Register workflow class
    worker.registerWorkflowImplementationTypes(featureInfo.factoryClass);

    // Register activity impl if any direct interfaces have the annotation
    if (Arrays.stream(feature.getClass().getInterfaces())
        .anyMatch(i -> i.isAnnotationPresent(ActivityInterface.class))) {
      worker.registerActivitiesImplementations(feature);
    }

    // Start the worker factory
    workerFactory.start();
  }

  void run() throws Exception {
    log.info("Executing feature {}", featureInfo.dir);
    var run = feature.execute(this);
    if (run == null) {
      log.info("Feature {} returned null", featureInfo.dir);
      return;
    }
    log.info("Checking result of feature {}", featureInfo.dir);
    feature.checkResult(this, run);

    feature.checkHistory(this, run);
  }

  public Run executeSingleParameterlessWorkflow() {
    // Find single workflow method or fail if multiple
    var methods = featureInfo.metadata.getWorkflowMethods();
    Preconditions.checkState(
        methods.size() == 1, "expected only one workflow method, got %s", methods.size());

    // Expect no parameters
    var reflectMethod = methods.get(0).getWorkflowMethod();
    Preconditions.checkState(
        reflectMethod.getParameterCount() == 0,
        "expected no parameters, got %s",
        reflectMethod.getParameterCount());

    // Call
    return new Run(methods.get(0), executeWorkflow(methods.get(0).getName()));
  }

  public Run executeSingleWorkflow(WorkflowOptions options, Object... args) {
    // Find single workflow method or fail if multiple
    var methods = featureInfo.metadata.getWorkflowMethods();
    Preconditions.checkState(
        methods.size() == 1, "expected only one workflow method, got %s", methods.size());

    // Use default options if not provided
    if (options == null) {
      var builder =
          WorkflowOptions.newBuilder()
              .setTaskQueue(config.taskQueue)
              .setWorkflowExecutionTimeout(Duration.ofMinutes(1));
      feature.workflowOptions(builder);
      options = builder.build();
    }

    var stub = client.newUntypedWorkflowStub(methods.get(0).getName(), options);

    // Call workflow with args
    return new Run(methods.get(0), stub.start(args));
  }

  public Object waitForRunResult(Run run) {
    if (run == null) {
      return null;
    }
    return waitForRunResult(run, run.method.getWorkflowMethod().getReturnType());
  }

  public <T> T waitForRunResult(Run run, Class<T> type) {
    var stub = client.newUntypedWorkflowStub(run.execution, Optional.empty());
    return stub.getResult(type);
  }

  public WorkflowExecution executeWorkflow(String workflowType, Object... args) {
    var builder =
        WorkflowOptions.newBuilder()
            .setTaskQueue(config.taskQueue)
            .setWorkflowExecutionTimeout(Duration.ofMinutes(1));
    feature.workflowOptions(builder);
    var stub = client.newUntypedWorkflowStub(workflowType, builder.build());
    return stub.start(args);
  }

  public History getWorkflowHistory(Run run) throws Exception {
    var eventIter =
        WorkflowClientHelper.getHistory(
            service, config.namespace, run.execution, config.metricsScope);
    return History.newBuilder().addAllEvents(() -> eventIter).build();
  }

  public Payload getWorkflowResultPayload(Run run) throws Exception {
    var history = getWorkflowHistory(run);
    var event =
        history.getEventsList().stream()
            .filter(e -> e.hasWorkflowExecutionCompletedEventAttributes())
            .findFirst();
    return event.get().getWorkflowExecutionCompletedEventAttributes().getResult().getPayloads(0);
  }

  public Payload getWorkflowArgumentPayload(Run run) throws Exception {
    var history = getWorkflowHistory(run);
    var event =
        history.getEventsList().stream()
            .filter(e -> e.hasWorkflowExecutionStartedEventAttributes())
            .findFirst();
    return event.get().getWorkflowExecutionStartedEventAttributes().getInput().getPayloads(0);
  }

  public WorkflowExecutionInfo getWorkflowExecutionInfo(Run run) throws Exception {
    var describeRequest =
        DescribeWorkflowExecutionRequest.newBuilder()
            .setNamespace(this.config.namespace)
            .setExecution(run.execution)
            .build();
    var exec =
        this.client
            .getWorkflowServiceStubs()
            .blockingStub()
            .describeWorkflowExecution(describeRequest);
    return exec.getWorkflowExecutionInfo();
  }

  public void checkCurrentAndPastHistories(Run run) throws Exception {
    // Obtain the current history and run it through replay
    log.info("Checking current history");
    var currentHistory = getWorkflowHistory(run);
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
        version =
            new Version(
                jsonFile.substring("history.java.".length(), jsonFile.length() - ".json".length()));
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

  public WorkerFactory getWorkerFactory() {
    return workerFactory;
  }

  public Worker getWorker() {
    return worker;
  }

  public void requireNoUpdateRejectedEvents(Run run) throws Exception {
    var history = getWorkflowHistory(run);
    var event =
        history.getEventsList().stream()
            .filter(e -> e.hasWorkflowExecutionUpdateRejectedEventAttributes())
            .findFirst();
    Assertions.assertFalse(event.isPresent());
  }

  public void skipIfUpdateNotSupported() {
    try {
      client.newUntypedWorkflowStub("fake").update("also_fake", Void.class);
    } catch (WorkflowNotFoundException exception) {
      return;
    } catch (WorkflowServiceException exception) {
      StatusRuntimeException e = (StatusRuntimeException) exception.getCause();
      switch (e.getStatus().getCode()) {
        case PERMISSION_DENIED:
          skip(
              "server support for update is disabled; set frontend.enableUpdateWorkflowExecution=true in dynamic config to enable");
        case UNIMPLEMENTED:
          skip("server version too old to support update");
      }
    }
    skip("unknown");
  }

  public void skipIfAsyncAcceptedUpdateNotSupported() {
    try {
      client.newUntypedWorkflowStub("fake").startUpdate("also_fake", Void.class);
    } catch (WorkflowNotFoundException exception) {
      return;
    } catch (WorkflowServiceException exception) {
      StatusRuntimeException e = (StatusRuntimeException) exception.getCause();
      switch (e.getStatus().getCode()) {
        case PERMISSION_DENIED:
          skip(
              "server support for async accepted update is disabled; set frontend.enableUpdateWorkflowExecutionAsyncAccepted=true in dynamic config to enable");
        case UNIMPLEMENTED:
          skip("server version too old to support update");
      }
    }
    skip("unknown");
  }

  public void skip(String message) {
    throw new TestSkippedException(message);
  }

  public void retry(Supplier<Boolean> fn, int retries, Duration sleepBetweenRetries) {
    for (int i = 0; i < retries; i++) {
      if (fn.get()) {
        return;
      }
      try {
        Thread.sleep(sleepBetweenRetries.toMillis());
      } catch (InterruptedException e) {
        throw new RuntimeException(e);
      }
    }
    Assertions.fail("retry limit exceeded");
  }
}
