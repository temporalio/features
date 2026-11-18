package nexus.standalone.sync_handler_error_non_retryable;

import static io.temporal.sdkfeatures.NexusAssertions.assertApplicationCause;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertThrows;

import io.nexusrpc.Operation;
import io.nexusrpc.Service;
import io.nexusrpc.handler.HandlerException;
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
  String HANDLER_MESSAGE = "standalone handler rejected the request";
  String CAUSE_TYPE = "StandaloneHandlerCause";
  String CAUSE_MESSAGE = "rejection from the handler";
  String CAUSE_DETAIL = "standalone handler error detail";

  @Service(name = StandaloneNexus.SERVICE)
  interface TestService {
    @Operation(name = "sync-handler-error")
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
      var handlerError = assertInstanceOf(HandlerException.class, failure.getCause());
      assertEquals(HandlerException.ErrorType.UNAVAILABLE, handlerError.getErrorType());
      assertEquals(HANDLER_MESSAGE, handlerError.getMessage());
      assertEquals(HandlerException.RetryBehavior.NON_RETRYABLE, handlerError.getRetryBehavior());
      assertFalse(handlerError.isRetryable());
      assertApplicationCause(handlerError.getCause(), CAUSE_TYPE, CAUSE_MESSAGE, CAUSE_DETAIL);
      return null;
    }
  }

  @ServiceImpl(service = TestService.class)
  class TestServiceImpl {
    @OperationImpl
    public OperationHandler<String, String> fail() {
      return OperationHandler.sync(
          (context, details, name) -> {
            throw new HandlerException(
                HandlerException.ErrorType.UNAVAILABLE,
                HANDLER_MESSAGE,
                ApplicationFailure.newFailure(CAUSE_MESSAGE, CAUSE_TYPE, CAUSE_DETAIL),
                HandlerException.RetryBehavior.NON_RETRYABLE);
          });
    }
  }
}
