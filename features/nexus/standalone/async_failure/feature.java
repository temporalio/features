package nexus.standalone.async_failure;

import static org.junit.jupiter.api.Assertions.assertThrows;

import io.nexusrpc.Operation;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
import io.temporal.client.NexusOperationFailedException;
import io.temporal.client.WorkflowOptions;
import io.temporal.failure.ApplicationFailure;
import io.temporal.nexus.Nexus;
import io.temporal.nexus.WorkflowRunOperation;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import nexus.standalone.StandaloneNexus;

@WorkflowInterface
public interface feature extends Feature {
  String FAILURE_MESSAGE = "deliberate backing workflow failure";

  @WorkflowMethod(name = "StandaloneNexusAsyncFailure")
  String fail(String name);

  @Service(name = StandaloneNexus.SERVICE)
  interface TestService {
    @Operation(name = "async-failure")
    String fail(String name);
  }

  class Impl implements feature {
    @Override
    public String fail(String name) {
      throw ApplicationFailure.newFailure(FAILURE_MESSAGE, StandaloneNexus.FAILURE_TYPE);
    }

    @Override
    public Object[] nexusServiceImplementations() {
      return new Object[] {new TestServiceImpl()};
    }

    @Override
    public Run execute(Runner runner) {
      var handle = StandaloneNexus.start(runner, TestService.class, TestService::fail);
      var failure = assertThrows(NexusOperationFailedException.class, handle::getResult);
      StandaloneNexus.assertApplicationFailure(failure.getCause(), FAILURE_MESSAGE);
      return null;
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> fail() {
      return WorkflowRunOperation.fromWorkflowMethod(
          (context, details, name) ->
              Nexus.getOperationContext()
                      .getWorkflowClient()
                      .newWorkflowStub(
                          feature.class,
                          WorkflowOptions.newBuilder()
                              .setWorkflowId(details.getRequestId())
                              .build())
                  ::fail);
    }
  }
}
