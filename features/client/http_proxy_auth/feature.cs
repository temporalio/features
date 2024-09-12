namespace client.http_proxy_auth;

using Temporalio.Client;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;

class Feature : IFeature
{
    public void ConfigureClient(Runner runner, TemporalClientConnectOptions options) =>
        http_proxy.Feature.ConfigureClient(runner, options, useAuth: true);

    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options) =>
        options.AddWorkflow<MyWorkflow>();

    [Workflow]
    class MyWorkflow
    {
        [WorkflowRun]
        public async Task<string> RunAsync() => "done";
    }
}