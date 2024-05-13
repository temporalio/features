import dataclasses
from urllib.parse import urlparse

from temporalio import workflow
from temporalio.client import Client, WorkflowHandle
from temporalio.service import HttpConnectProxyConfig, ServiceClient

from harness.python.feature import Runner, register_feature


@workflow.defn
class Workflow:
    @workflow.run
    async def run(self) -> str:
        return "done"


async def start(runner: Runner) -> WorkflowHandle:
    # Make sure proxy URL exists, and parse it to get parts
    assert runner.http_proxy_url
    url = urlparse(runner.http_proxy_url)

    # Create a new client with a different service client connected to the proxy
    config = runner.client.config()
    config["service_client"] = await ServiceClient.connect(
        dataclasses.replace(
            runner.client.service_client.config,
            http_connect_proxy_config=HttpConnectProxyConfig(
                target_host=f"{url.hostname}:{url.port}"
            ),
        )
    )
    client = Client(**config)

    # Now can do the normal
    return await runner.start_single_parameterless_workflow(override_client=client)


register_feature(workflows=[Workflow], expect_run_result="done", start=start)
