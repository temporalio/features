using NexusRpc;
using NexusRpc.Handlers;
using Temporalio.Activities;
using Temporalio.Api.Common.V1;
using Temporalio.Client.Interceptors;
using Temporalio.Common;
using Temporalio.Converters;
using Temporalio.Nexus;
using Temporalio.Worker.Interceptors;
using Temporalio.Workflows;

class PluginsSnippet
{
    // @@@SNIPSTART dotnet-plugins-activity
    [Activity]
    static void SomeActivity() => throw new NotImplementedException();

    SimplePlugin activityPlugin = new SimplePlugin(
        "organization.PluginName",
        new SimplePluginOptions() { }.AddActivity(SomeActivity));
    // @@@SNIPEND

    // @@@SNIPSTART dotnet-plugins-workflow
    [Workflow]
    class SimpleWorkflow
    {
        [WorkflowRun]
        public Task<string> RunAsync(string name) => Task.FromResult($"Hello, {name}!");
    }

    SimplePlugin workflowPlugin = new SimplePlugin(
        "organization.PluginName",
        new SimplePluginOptions() { }.AddWorkflow<SimpleWorkflow>());
    // @@@SNIPEND

    // @@@SNIPSTART dotnet-plugins-nexus
    [NexusService]
    public interface IStringService
    {
        [NexusOperation]
        string DoSomething(string name);
    }

    [NexusServiceHandler(typeof(IStringService))]
    public class HandlerFactoryStringService
    {
        private readonly Func<IOperationHandler<string, string>> handlerFactory;

        public HandlerFactoryStringService(Func<IOperationHandler<string, string>> handlerFactory) =>
            this.handlerFactory = handlerFactory;

        [NexusOperationHandler]
        public IOperationHandler<string, string> DoSomething() => handlerFactory();
    }

    SimplePlugin nexusPlugin = new SimplePlugin(
        "organization.PluginName",
        new SimplePluginOptions() { }.AddNexusService(new HandlerFactoryStringService(() =>
            OperationHandler.Sync<string, string>((ctx, name) => $"Hello, {name}")))
    );
    // @@@SNIPEND

    // @@@SNIPSTART dotnet-plugins-converter
    private class Codec : IPayloadCodec
    {
        public Task<IReadOnlyCollection<Payload>> EncodeAsync(IReadOnlyCollection<Payload> payloads) => throw new NotImplementedException();
        public Task<IReadOnlyCollection<Payload>> DecodeAsync(IReadOnlyCollection<Payload> payloads) => throw new NotImplementedException();
    }

    SimplePlugin converterPlugin = new SimplePlugin(
        "organization.PluginName",
        new SimplePluginOptions()
        {
            DataConverterOption = new SimplePluginOptions.SimplePluginOption<DataConverter>(
                (converter) => converter with { PayloadCodec = new Codec() }
            ),
        });
    // @@@SNIPEND

    // @@@SNIPSTART dotnet-plugins-interceptors
    private class SomeClientInterceptor : IClientInterceptor
    {
        public ClientOutboundInterceptor InterceptClient(
            ClientOutboundInterceptor nextInterceptor) =>
            throw new NotImplementedException();
    }

    private class SomeWorkerInterceptor : IWorkerInterceptor
    {
        public WorkflowInboundInterceptor InterceptWorkflow(
            WorkflowInboundInterceptor nextInterceptor) =>
            throw new NotImplementedException();

        public ActivityInboundInterceptor InterceptActivity(
            ActivityInboundInterceptor nextInterceptor) =>
            throw new NotImplementedException();
    }

    SimplePlugin interceptorPlugin = new SimplePlugin(
        "organization.PluginName",
        new SimplePluginOptions()
        {
            ClientInterceptors = new List<IClientInterceptor>() { new SomeClientInterceptor() },
            WorkerInterceptors = new List<IWorkerInterceptor>() { new SomeWorkerInterceptor() },
        });
    // @@@SNIPEND
}
