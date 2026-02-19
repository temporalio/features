import { SimplePlugin } from '@temporalio/plugin';
import { PayloadCodec, Payload } from '@temporalio/common';
import { WorkflowClientInterceptor } from '@temporalio/client';
import { ActivityInboundCallsInterceptor, ActivityOutboundCallsInterceptor } from '@temporalio/worker';
import * as nexus from 'nexus-rpc';

// @@@SNIPSTART typescript-plugins-activity
const activity = async () => 'activity';
const plugin = new SimplePlugin({
  name: 'plugin-name',
  activities: {
    pluginActivity: activity,
  },
});
// @@@SNIPEND

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
    name: 'plugin-name',
    nexusServices: [testServiceHandler],
  });
  // @@@SNIPEND
}

{
  // @@@SNIPSTART typescript-plugins-converter
  const codec: PayloadCodec = {
    encode(payloads: Payload[]): Promise<Payload[]> {
      throw new Error();
    },
    decode(payloads: Payload[]): Promise<Payload[]> {
      throw new Error();
    },
  };
  const plugin = new SimplePlugin({
    name: 'plugin-name',
    dataConverter: (converter) => ({
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
    name: 'plugin-name',
    clientInterceptors: {
      workflow: [new MyWorkflowClientInterceptor()],
    },
    workerInterceptors: {
      client: {
        workflow: [new MyWorkflowClientInterceptor()],
      },
      workflowModules: [workflowInterceptorsPath],
      activity: [
        (_) => ({
          inbound: new MyActivityInboundInterceptor(),
          outbound: new MyActivityOutboundInterceptor(),
        }),
      ],
    },
  });
  // @@@SNIPEND
}
