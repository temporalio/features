package io.temporal.sdkfeatures;

import io.temporal.client.WorkflowFailedException;
import io.temporal.failure.ActivityFailure;
import io.temporal.failure.ApplicationFailure;

public class Assertions extends org.junit.jupiter.api.Assertions {
  public static void assertActivityErrorMessage(String expected, Throwable error) {
    assertNotNull(error, "Did not get error");
    // Check that it is our expected error
    if (error instanceof WorkflowFailedException && error.getCause() instanceof ActivityFailure) {
      var appErr = error.getCause().getCause();
      if (appErr instanceof ApplicationFailure) {
        assertEquals("activity attempt 5 failed", ((ApplicationFailure) appErr).getOriginalMessage());
        return;
      }
    }
    // Otherwise fail
    fail(error);
  }

  private Assertions() {}
}
