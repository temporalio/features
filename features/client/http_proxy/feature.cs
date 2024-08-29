namespace client.http_proxy;

using Temporalio.Client;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;

class Feature : IFeature
{
    public static void ConfigureClient(
        Runner runner, TemporalClientConnectOptions options, bool useAuth)
    {
        var uri = new Uri(runner.HttpProxyUrl ?? throw new InvalidOperationException("Missing proxy URL"));
        options.HttpConnectProxy = new() { TargetHost = $"{uri.Host}:{uri.Port}" };
        if (useAuth)
        {
            options.HttpConnectProxy!.BasicAuth = ("proxy-user", "proxy-pass");
        }
    }

    public void ConfigureClient(Runner runner, TemporalClientConnectOptions options) =>
        ConfigureClient(runner, options, useAuth: false);

    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options) =>
        options.AddWorkflow<MyWorkflow>();

    [Workflow]
    class MyWorkflow
    {
        [WorkflowRun]
        public async Task<string> RunAsync() => "done";
    }
}