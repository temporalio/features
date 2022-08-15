package query.timeout_due_to_no_active_workers;

import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.client.WorkflowServiceException;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;
import io.temporal.serviceclient.RpcRetryOptions;
import io.temporal.serviceclient.WorkflowServiceStubs;
import io.temporal.serviceclient.WorkflowServiceStubsOptions;
import io.temporal.workflow.QueryMethod;
import io.temporal.workflow.SignalMethod;
import io.temporal.workflow.Workflow;
import java.time.Duration;
import java.util.Optional;
import org.junit.jupiter.api.Assertions;

public interface feature extends Feature, SimpleWorkflow {
  @QueryMethod
  boolean someQuery();

  @SignalMethod
  void finish();

  class Impl implements feature {
    private boolean doFinish = false;

    @Override
    public void workflow() {
      Workflow.await(() -> this.doFinish);
    }

    @Override
    public boolean someQuery() {
      return true;
    }

    @Override
    public void finish() {
      this.doFinish = true;
    }

    @Override
    public void checkResult(Runner runner, Run run) {
      var serviceStubs = runner.client.getWorkflowServiceStubs();
      var newStubOpts = WorkflowServiceStubsOptions.newBuilder(serviceStubs.getOptions());
      newStubOpts.setRpcRetryOptions(RpcRetryOptions.newBuilder().setMaximumAttempts(1).build());
      newStubOpts.setRpcQueryTimeout(Duration.ofSeconds(1));
      var newStubs = WorkflowServiceStubs.newServiceStubs(newStubOpts.build());
      var noRetryClient = WorkflowClient.newInstance(newStubs,
          WorkflowClientOptions.newBuilder().setNamespace(runner.config.namespace).build());
      var stub = noRetryClient.newWorkflowStub(feature.class,
          run.execution.getWorkflowId(), Optional.of(run.execution.getRunId()));
      // Shutdown the worker
      runner.getWorkerFactory().shutdownNow();
      try {
        stub.someQuery();
      } catch (Exception e) {
        Assertions.assertInstanceOf(WorkflowServiceException.class, e);
        Assertions.assertInstanceOf(StatusRuntimeException.class, e.getCause());
        StatusRuntimeException s = (StatusRuntimeException) e.getCause();
        Assertions.assertEquals(Status.DEADLINE_EXCEEDED.getCode(), s.getStatus().getCode());
      }
      runner.restartWorker();
      stub.finish();
      runner.waitForRunResult(run);
    }
  }
}
