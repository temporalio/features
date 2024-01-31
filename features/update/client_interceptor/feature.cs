namespace update.client_interceptor;

using Temporalio.Client.Interceptors;
using Temporalio.Client;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;

class MyOutboundClientInterceptor : IClientInterceptor
{
    public ClientOutboundInterceptor InterceptClient(
        ClientOutboundInterceptor nextInterceptor) =>
        new ClientOutbound(nextInterceptor);

    private sealed class ClientOutbound : ClientOutboundInterceptor
    {
        public ClientOutbound(ClientOutboundInterceptor next) : base(next)
        {
        }

        public override Task<WorkflowUpdateHandle<TResult>> StartWorkflowUpdateAsync<TResult>(
            StartWorkflowUpdateInput input)
        {
            var newInput = input with { Args = new object[] { "intercepted" } };
            return base.StartWorkflowUpdateAsync<TResult>(newInput);
        }
    }
}

class Feature : IFeature
{
    [Workflow]
    class MyWorkflow
    {
        private bool shutdown;

        [WorkflowRun]
        public Task RunAsync() => Workflow.WaitConditionAsync(() => shutdown);

        [WorkflowUpdate]
        public async Task<string> MyUpdate(string arg)
        {
            shutdown = true;
            return arg;
        }
    }

    public void ConfigureClient(Runner runner, TemporalClientConnectOptions options)
    {
        options.Interceptors = new[] { new MyOutboundClientInterceptor() };
    }

    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options)
    {
        options.AddWorkflow<MyWorkflow>();
    }

    public async Task<WorkflowHandle?> ExecuteAsync(Runner runner)
    {
        await runner.SkipIfUpdateNotSupportedAsync();

        var handle = await runner.Client.StartWorkflowAsync(
            (MyWorkflow wf) => wf.RunAsync(),
            runner.NewWorkflowOptions());

        Assert.Equal("intercepted", await handle.ExecuteUpdateAsync(wf => wf.MyUpdate("Enchicat")));

        return handle;
    }
}