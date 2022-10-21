package child_workflow.result;

import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;


@WorkflowInterface
public interface ChildWorkflow {
    @WorkflowMethod
    public String executeChild(String input );
}

class ChildWorkflowImpl implements ChildWorkflow {
    @Override
    public String executeChild(String input) {
        return input;
    }
}
