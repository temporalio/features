package continue_as_new.continue_as_same;

import io.temporal.client.WorkflowOptions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.Workflow;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;

import java.time.Duration;
import java.util.HashMap;

@WorkflowInterface
public interface feature extends Feature {

  @WorkflowMethod
  public String workflow(String input);

  class Impl implements feature {
    private static final String INPUT_DATA  = "InputData";
    private static final String MEMO_KEY    = "MemoKey";
    private static final String MEMO_VALUE  = "MemoValue";
    private static final String WORKFLOW_ID = "TestID";

    @Override
    public String workflow(String input) {
      if (Workflow.getInfo().getContinuedExecutionRunId().isPresent()) {
        return input;
      }
      Workflow.continueAsNew(input);
      return "";
    }

    @Override
    public Run execute(Runner runner) throws Exception {
      var options = WorkflowOptions.newBuilder()
        .setWorkflowId(WORKFLOW_ID)
        .setMemo(new HashMap<String, Object>(){
          {
              put(MEMO_KEY, MEMO_VALUE);
          }
        })
        .setTaskQueue(runner.config.taskQueue)
        .setWorkflowExecutionTimeout(Duration.ofMinutes(1))
        .build();
      return runner.executeSingleWorkflow(options, INPUT_DATA);
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      var resultStr = runner.waitForRunResult(run, String.class);
      assertEquals(resultStr, INPUT_DATA);
      // Workflow ID does not change after continue as new
      assertEquals(run.execution.getWorkflowId(), WORKFLOW_ID);
      // Memos do not change after continue as new
      var payload = runner.getWorkflowExecutionInfo(run).getMemo().getFieldsMap().get(MEMO_KEY);
      assertNotNull(payload);
      var testMemo = payload.getData().toStringUtf8();
      testMemo = testMemo.substring(1, testMemo.length() - 1);
      assertEquals(testMemo, MEMO_VALUE);
    }
  }
}
