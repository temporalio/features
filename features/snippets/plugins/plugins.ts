import * as nexus from 'nexus-rpc';
import { Context } from '@temporalio/activity';
import { SimplePlugin } from '@temporalio/plugin';
import { DataConverter, PayloadCodec, Payload } from '@temporalio/common';
import { WorkflowClientInterceptor } from '@temporalio/client';
import { ActivityInboundCallsInterceptor, ActivityOutboundCallsInterceptor } from '@temporalio/worker';
/* eslint-disable @typescript-eslint/no-unused-vars */

{
  // @@@SNIPSTART typescript-plugins-activity
  const activity = async () => 'activity';
  const plugin = new SimplePlugin({
    name: 'organization.PluginName',
    activities: {
      pluginActivity: activity,
    },
  });
  // @@@SNIPEND
}
{
  // @@@SNIPSTART typescript-plugins-nexus
  const testServiceHandler = nexus.serviceHandler(
    nexus.service('testService', {
      testSyncOp: nexus.operation<string, string>(),
    }),
    {
      async testSyncOp(_, input) {
        return input;
      },
    }
  );
  const plugin = new SimplePlugin({
    name: 'organization.PluginName',
    nexusServices: [testServiceHandler],
  });
  // @@@SNIPEND
}

{
  // @@@SNIPSTART typescript-plugins-converter
  const codec: PayloadCodec = {
    encode(_payloads: Payload[]): Promise<Payload[]> {
      throw new Error();
    },
    decode(_payloads: Payload[]): Promise<Payload[]> {
      throw new Error();
    },
  };
  const plugin = new SimplePlugin({
    name: 'organization.PluginName',
    dataConverter: (converter: DataConverter | undefined) => ({
      ...converter,
      payloadCodecs: [...(converter?.payloadCodecs ?? []), codec],
    }),
  });
  // @@@SNIPEND
}

{
  // @@@SNIPSTART typescript-plugins-interceptors
  class MyWorkflowClientInterceptor implements WorkflowClientInterceptor {}

  class MyActivityInboundInterceptor implements ActivityInboundCallsInterceptor {}

  class MyActivityOutboundInterceptor implements ActivityOutboundCallsInterceptor {}

  const workflowInterceptorsPath = '';

  const plugin = new SimplePlugin({
    name: 'organization.PluginName',
    clientInterceptors: {
      workflow: [new MyWorkflowClientInterceptor()],
    },
    workerInterceptors: {
      client: {
        workflow: [new MyWorkflowClientInterceptor()],
      },
      workflowModules: [workflowInterceptorsPath],
      activity: [
        (_: Context) => ({
          inbound: new MyActivityInboundInterceptor(),
          outbound: new MyActivityOutboundInterceptor(),
        }),
      ],
    },
  });
  // @@@SNIPEND
}
