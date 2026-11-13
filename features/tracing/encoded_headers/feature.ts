import * as assert from 'node:assert/strict';
import { Resource } from '@opentelemetry/resources';
import { InMemorySpanExporter, SimpleSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { NodeTracerProvider } from '@opentelemetry/sdk-trace-node';
import { Feature } from '@temporalio/harness';
import { defaultPayloadConverter, Payload, PayloadCodec } from '@temporalio/common';
import { temporal } from '@temporalio/proto';
import { OpenTelemetryPlugin } from '@temporalio/interceptors-opentelemetry';
import { workflow } from './workflow';

const encoding = 'binary/encrypted';
const payloadProto = temporal.api.common.v1.Payload;
const xor = (data: Uint8Array): Uint8Array => data.map((byte) => byte ^ 0x5a);

class WrappingCodec implements PayloadCodec {
  async encode(payloads: Payload[]): Promise<Payload[]> {
    return payloads.map((payload) => {
      if (Buffer.from(payload.metadata?.encoding ?? []).toString() === encoding) {
        throw new Error('Payload encoded twice');
      }
      return {
        metadata: { encoding: Buffer.from(encoding) },
        data: xor(payloadProto.encode(payload).finish()),
      };
    });
  }

  async decode(payloads: Payload[]): Promise<Payload[]> {
    return payloads.map((payload) => {
      if (Buffer.from(payload.metadata?.encoding ?? []).toString() !== encoding) {
        throw new Error('Expected an encoded payload');
      }
      return payloadProto.decode(xor(payload.data!));
    });
  }
}

const resource = new Resource({ 'service.name': 'encoded-headers-feature' });
const spanProcessor = new SimpleSpanProcessor(new InMemorySpanExporter());
const provider = new NodeTracerProvider({ resource, spanProcessors: [spanProcessor] });
provider.register();
const tracer = provider.getTracer('encoded-headers-feature');
const tracingOptions = { resource, spanProcessor, tracer, encodeClientHeaders: true };
const tracingPlugin = new OpenTelemetryPlugin(tracingOptions);

const workerOptions = {
  codecHeaders: true,
  plugins: [tracingPlugin],
};

export const feature = new Feature({
  workflow,
  workflowsPath: require.resolve('./workflow'),
  dataConverter: { payloadCodecs: [new WrappingCodec()] },
  clientOptions: { plugins: [tracingPlugin] },
  workerOptions,
  execute: async (runner) =>
    tracer.startActiveSpan('ts-caller', async (parent) => {
      try {
        const handle = await runner.client.workflow.start(workflow, {
          taskQueue: runner.options.taskQueue,
          workflowId: `${runner.options.taskQueue}-workflow`,
          workflowExecutionTimeout: '15s',
          args: ['hello'],
        });
        try {
          const result = await handle.result();
          const events = (await handle.fetchHistory()).events!;
          const header = events[0]!.workflowExecutionStartedEventAttributes!.header!.fields!['_tracer-data']!;
          const [decoded] = await new WrappingCodec().decode([header]);
          const carrier = defaultPayloadConverter.fromPayload<{ traceparent: string }>(decoded!);
          assert.equal(carrier.traceparent.split('-')[1], parent.spanContext().traceId);
          assert.deepEqual(result, { message: 'hello', traceId: parent.spanContext().traceId });
          assert.ok(events.every((event) => !event.workflowTaskFailedEventAttributes));
          return handle;
        } catch (error) {
          const events = (await handle.fetchHistory()).events!;
          for (const event of events) {
            if (event.workflowTaskFailedEventAttributes) {
              console.error('Workflow Task failed:', event.workflowTaskFailedEventAttributes.failure);
            }
          }
          throw error;
        }
      } finally {
        parent.end();
      }
    }),
});
