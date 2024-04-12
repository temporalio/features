package client.http_proxy;

import io.grpc.HttpConnectProxiedSocketAddress;
import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowClientOptions;
import io.temporal.sdkfeatures.*;
import io.temporal.serviceclient.WorkflowServiceStubs;
import io.temporal.serviceclient.WorkflowServiceStubsOptions;
import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import java.net.InetSocketAddress;
import java.net.URL;

@WorkflowInterface
public interface feature extends Feature {
  @WorkflowMethod
  String workflow();

  class Impl implements feature {
    @Override
    public Run execute(Runner runner) throws Exception {
      return execute(runner, false);
    }

    public static Run execute(Runner runner, boolean useAuth) throws Exception {
      // Make sure proxy URL is present
      Assertions.assertNotNull(runner.config.httpProxyUrl);

      // Build proxied addr
      var proxyUrl = new URL(runner.config.httpProxyUrl);
      var targetParts = runner.config.serverHostPort.split(":");
      var proxyAddrBuilder =
          HttpConnectProxiedSocketAddress.newBuilder()
              .setProxyAddress(new InetSocketAddress(proxyUrl.getHost(), proxyUrl.getPort()))
              .setTargetAddress(
                  new InetSocketAddress(targetParts[0], Integer.parseInt(targetParts[1])));
      if (useAuth) {
        proxyAddrBuilder.setUsername("proxy-user").setPassword("proxy-pass");
      }
      var proxyAddr = proxyAddrBuilder.build();

      // Build a client that uses the HTTP proxy
      var service =
          WorkflowServiceStubs.newServiceStubs(
              WorkflowServiceStubsOptions.newBuilder()
                  .setTarget(runner.config.serverHostPort)
                  .setSslContext(runner.config.sslContext)
                  .setMetricsScope(runner.config.metricsScope)
                  .setChannelInitializer(builder -> builder.proxyDetector(addr -> proxyAddr))
                  .build());
      var client =
          WorkflowClient.newInstance(
              service,
              WorkflowClientOptions.newBuilder().setNamespace(runner.config.namespace).build());

      // Run the workflow
      return runner.executeSingleParameterlessWorkflow(client);
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
