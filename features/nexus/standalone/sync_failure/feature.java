package nexus.standalone.sync_failure;

import static org.junit.jupiter.api.Assertions.assertThrows;

import io.nexusrpc.Operation;
import io.nexusrpc.OperationException;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
import io.temporal.client.NexusOperationFailedException;
import io.temporal.failure.ApplicationFailure;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;
import nexus.standalone.StandaloneNexus;

public interface feature extends Feature, SimpleWorkflow {
  String FAILURE_MESSAGE = "deliberate sync handler failure";

  @Service(name = StandaloneNexus.SERVICE)
  interface TestService {
    @Operation(name = "sync-failure")
    String fail(String name);
  }

  class Impl implements feature {
    @Override
    public void workflow() {}

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
      return OperationHandler.sync(
          (context, details, name) -> {
            throw OperationException.failed(
                "sync operation failed",
                ApplicationFailure.newFailure(FAILURE_MESSAGE, StandaloneNexus.FAILURE_TYPE));
          });
    }
  }
}
