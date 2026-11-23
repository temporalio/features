package nexus.standalone.sync_success;

import static org.junit.jupiter.api.Assertions.assertEquals;

import io.nexusrpc.Operation;
import io.nexusrpc.Service;
import io.nexusrpc.handler.OperationHandler;
import io.nexusrpc.handler.OperationImpl;
import io.nexusrpc.handler.ServiceImpl;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.sdkfeatures.SimpleWorkflow;
import nexus.standalone.StandaloneNexus;

public interface feature extends Feature, SimpleWorkflow {
  @Service(name = StandaloneNexus.SERVICE)
  interface TestService {
    @Operation(name = "sync-success")
    String greet(String name);
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
      var handle = StandaloneNexus.start(runner, TestService.class, TestService::greet);
      assertEquals("Hello from the handler, world!", handle.getResult());
      return null;
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> greet() {
      return OperationHandler.sync(
          (context, details, name) -> "Hello from the handler, " + name + "!");
    }
  }
}
