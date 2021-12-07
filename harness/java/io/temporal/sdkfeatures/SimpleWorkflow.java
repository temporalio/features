package io.temporal.sdkfeatures;

import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;

@WorkflowInterface
public interface SimpleWorkflow {
  @WorkflowMethod
  void workflow();
}
