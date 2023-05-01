package io.temporal.sdkfeatures;

public class TestSkippedException extends RuntimeException {
    final String message;
    public TestSkippedException(String message) {
        this.message = message;
    }

    @Override
    public String getMessage() {
        return message;
    }
}
