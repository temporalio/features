package io.temporal.sdkfeatures;

import static org.junit.jupiter.api.Assertions.*;

import io.temporal.api.enums.v1.EventType;
import io.temporal.api.enums.v1.TimeoutType;
import io.temporal.api.history.v1.HistoryEvent;
import io.temporal.failure.ApplicationFailure;
import io.temporal.failure.NexusOperationFailure;
import io.temporal.failure.TimeoutFailure;
import java.util.Arrays;
import java.util.stream.Collectors;

/** Shared assertions for the workflow-facing Nexus contract. */
public final class NexusAssertions {
  public static NexusOperationFailure assertOperationFailure(
      Throwable failure, String endpoint, String service, String operation, String token) {
    assertInstanceOf(NexusOperationFailure.class, failure);
    var operationFailure = (NexusOperationFailure) failure;
    assertEquals(endpoint, operationFailure.getEndpoint());
    assertEquals(service, operationFailure.getService());
    assertEquals(operation, operationFailure.getOperation());
    assertEquals(token, operationFailure.getOperationToken());
    assertTrue(operationFailure.getScheduledEventId() > 0);
    return operationFailure;
  }

  public static void assertApplicationCause(
      Throwable failure, String type, String message, String detail) {
    assertInstanceOf(ApplicationFailure.class, failure);
    var applicationFailure = (ApplicationFailure) failure;
    assertEquals(type, applicationFailure.getType());
    assertEquals(message, applicationFailure.getOriginalMessage());
    assertFalse(applicationFailure.isNonRetryable());
    assertEquals(1, applicationFailure.getDetails().getSize());
    assertEquals(detail, applicationFailure.getDetails().get(0, String.class));
    assertNull(applicationFailure.getCause());
  }

  public static void assertTimeoutCause(Throwable failure, TimeoutType timeoutType) {
    assertInstanceOf(TimeoutFailure.class, failure);
    var timeoutFailure = (TimeoutFailure) failure;
    assertEquals(timeoutType, timeoutFailure.getTimeoutType());
    assertEquals(0, timeoutFailure.getLastHeartbeatDetails().getSize());
    assertNull(timeoutFailure.getCause());
  }

  /** Assert the entire single-operation lifecycle, then replay the workflow history. */
  public static void assertHistory(Runner runner, Run run, EventType... expected) throws Exception {
    var actual =
        runner.getWorkflowHistory(run).getEventsList().stream()
            .map(HistoryEvent::getEventType)
            .filter(type -> type.name().startsWith("EVENT_TYPE_NEXUS_OPERATION_"))
            // Successful cancellation acknowledgements can race with operation completion.
            .filter(type -> type != EventType.EVENT_TYPE_NEXUS_OPERATION_CANCEL_REQUEST_COMPLETED)
            .collect(Collectors.toList());
    assertEquals(Arrays.asList(expected), actual);
    runner.checkCurrentAndPastHistories(run);
  }

  private NexusAssertions() {}
}
