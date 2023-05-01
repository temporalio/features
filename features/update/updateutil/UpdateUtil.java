package update.updateutil;

import io.grpc.StatusRuntimeException;
import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowNotFoundException;
import io.temporal.client.WorkflowServiceException;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import org.junit.jupiter.api.Assertions;

public class UpdateUtil {

    public static void RequireNoUpdateRejectedEvents(Runner runner, Run run) {
        try {
            var history = runner.getWorkflowHistory(run);
            var event = history.getEventsList().stream().filter(e -> e.hasWorkflowExecutionUpdateRejectedEventAttributes()).findFirst();
            Assertions.assertFalse(event.isPresent());
        } catch(Exception e) {
            Assertions.fail();
        }
    }

    public static String CheckServerSupportsUpdate(WorkflowClient client){
        try {
            client.newUntypedWorkflowStub("fake").update("also_fake", Void.class);
        } catch (WorkflowNotFoundException exception) {
            return "";
        } catch (WorkflowServiceException exception) {
            StatusRuntimeException e = (StatusRuntimeException) exception.getCause();
            switch (e.getStatus().getCode()) {
                case PERMISSION_DENIED:
                    return "server support for update is disabled; set frontend.enableUpdateWorkflowExecution=true in dynamic config to enable";
                case UNIMPLEMENTED:
                    return "server version too old to support update";
            }
        }
        return "unknown";
    }
}
