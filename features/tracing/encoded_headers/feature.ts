import { Resource } from '@opentelemetry/resources';
import { InMemorySpanExporter, SimpleSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { Feature } from '@temporalio/harness';
import { Payload, PayloadCodec } from '@temporalio/common';
import { temporal } from '@temporalio/proto';
import { OpenTelemetryPlugin } from '@temporalio/interceptors-opentelemetry';

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

// Keep the codec on the worker boundary; an interceptor workaround would mask the regression.
const dataConverter = { payloadCodecs: [new WrappingCodec()], encodeHeaders: true };

export const feature = new Feature({
  workflowsPath: require.resolve('./workflow'),
  dataConverter,
  workerOptions: {
    plugins: [
      new OpenTelemetryPlugin({
        resource: new Resource({ 'service.name': 'encoded-headers-feature' }),
        spanProcessor: new SimpleSpanProcessor(new InMemorySpanExporter()),
      }),
    ],
  },
});
