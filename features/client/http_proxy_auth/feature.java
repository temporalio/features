package client.http_proxy_auth;

import io.temporal.sdkfeatures.Assertions;
import io.temporal.sdkfeatures.Feature;
import io.temporal.sdkfeatures.Run;
import io.temporal.sdkfeatures.Runner;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;

@WorkflowInterface
public interface feature extends Feature {
  @WorkflowMethod
  String workflow();

  class Impl implements feature {
    @Override
    public Run execute(Runner runner) throws Exception {
      return client.http_proxy.feature.Impl.execute(runner, true);
    }

    @Override
    public void checkResult(Runner runner, Run run) throws Exception {
      Assertions.assertEquals("done", runner.waitForRunResult(run, String.class));
    }

    @Override
    public String workflow() {
      return "done";
    }
  }
}
