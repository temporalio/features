import dataclasses
from dataclasses import dataclass

import nexusrpc
import temporalio.client
import temporalio.worker
from temporalio import activity, workflow
from temporalio.contrib.pydantic import pydantic_data_converter
from temporalio.converter import DataConverter
from temporalio.plugin import SimplePlugin
from temporalio.worker import WorkflowRunner
from temporalio.worker.workflow_sandbox import SandboxedWorkflowRunner


# @@@SNIPSTART python-plugin-activity
@activity.defn
async def some_activity() -> None:
    return None


plugin = SimplePlugin("organization.PluginName", activities=[some_activity])
# @@@SNIPEND


# @@@SNIPSTART python-plugin-workflow
@workflow.defn
class HelloWorkflow:
    @workflow.run
    async def run(self, name: str) -> str:
        return f"Hello, {name}!"


plugin = SimplePlugin("organization.PluginName", workflows=[HelloWorkflow])
# @@@SNIPEND


@dataclass
class Weather:
    city: str
    temperature_range: str
    conditions: str


@dataclass
class WeatherInput:
    city: str


# @@@SNIPSTART python-plugin-nexus
@nexusrpc.service
class WeatherService:
    get_weather_nexus_operation: nexusrpc.Operation[WeatherInput, Weather]


@nexusrpc.handler.service_handler(service=WeatherService)
class WeatherServiceHandler:
    @nexusrpc.handler.sync_operation
    async def get_weather_nexus_operation(
        self, ctx: nexusrpc.handler.StartOperationContext, input: WeatherInput
    ) -> Weather:
        return Weather(
            city=input.city,
            temperature_range="14-20C",
            conditions="Sunny with wind.",
        )


plugin = SimplePlugin(
    "organization.PluginName", nexus_service_handlers=[WeatherServiceHandler()]
)
# @@@SNIPEND


# @@@SNIPSTART python-plugin-converter
def set_converter(converter: DataConverter | None) -> DataConverter:
    if converter is None or converter == DataConverter.default:
        return pydantic_data_converter
    # Should consider interactions with other plugins,
    # as this will override the data converter.
    # This may mean failing, warning, or something else
    return converter


plugin = SimplePlugin("organization.PluginName", data_converter=set_converter)
# @@@SNIPEND


# @@@SNIPSTART python-plugin-interceptors
class SomeWorkerInterceptor(temporalio.worker.Interceptor):
    pass  # Your implementation


class SomeClientInterceptor(temporalio.client.Interceptor):
    pass  # Your implementation


plugin = SimplePlugin(
    "organization.PluginName",
    interceptors=[SomeWorkerInterceptor(), SomeClientInterceptor()],
)
# @@@SNIPEND


# @@@SNIPSTART python-plugin-sandbox
def workflow_runner(runner: WorkflowRunner | None) -> WorkflowRunner:
    if not runner:
        raise ValueError("No WorkflowRunner provided to the plugin.")

    # If in sandbox, add additional passthrough
    if isinstance(runner, SandboxedWorkflowRunner):
        return dataclasses.replace(
            runner,
            restrictions=runner.restrictions.with_passthrough_modules("module"),
        )
    return runner


plugin = SimplePlugin("organization.PluginName", workflow_runner=workflow_runner)
# @@@SNIPEND
