package update.worker_restart;

import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;

@WorkflowInterface
public interface IntWorkflow {
    @WorkflowMethod
    int workflow();
}
