package update.self;

import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;

@WorkflowInterface
public interface SelfUpdateWorkflow {

    @WorkflowMethod
    String workflow();
}
