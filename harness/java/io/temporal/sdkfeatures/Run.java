package io.temporal.sdkfeatures;

import io.temporal.api.common.v1.WorkflowExecution;
import io.temporal.common.metadata.POJOWorkflowMethodMetadata;

public class Run {
  public final POJOWorkflowMethodMetadata method;
  public final WorkflowExecution execution;

  public Run(POJOWorkflowMethodMetadata method, WorkflowExecution execution) {
    this.method = method;
    this.execution = execution;
  }
}
