import { Payload, PayloadCodec, SerializationContext } from '@temporalio/common';
import { decode, encode } from '@temporalio/common/lib/encoding';

export const METADATA_KEY = 'ctx-signature';
export const NO_CONTEXT = 'none';

/**
 * Every signature the codec has been asked to encode or decode with. Worker and
 * client share a process here, so this is how a feature asserts on a context
 * whose payload never shows up in history.
 */
export const observedSignatures = new Set<string>();

export function workflowSignature(namespace: string, workflowId: string): string {
  return `wf|${namespace}|${workflowId}`;
}

export function activitySignature(
  namespace: string,
  workflowId: string | undefined,
  activityId: string | undefined,
  isLocal: boolean,
): string {
  return `act|${namespace}|${workflowId}|${activityId}|${isLocal}`;
}

export function signatureOf(context?: SerializationContext): string {
  switch (context?.type) {
    case 'workflow':
      return workflowSignature(context.namespace, context.workflowId);
    case 'activity':
      return activitySignature(context.namespace, context.workflowId, context.activityId, context.isLocal);
    default:
      return NO_CONTEXT;
  }
}

/**
 * Stamps the signature of its serialization context onto every payload it
 * encodes and refuses to decode a payload encoded under a different context.
 */
export class SigningCodec implements PayloadCodec {
  async encode(payloads: Payload[], context?: SerializationContext): Promise<Payload[]> {
    const signature = signatureOf(context);
    observedSignatures.add(signature);
    return payloads.map((payload) => ({
      ...payload,
      metadata: { ...payload.metadata, [METADATA_KEY]: encode(signature) },
    }));
  }

  async decode(payloads: Payload[], context?: SerializationContext): Promise<Payload[]> {
    const signature = signatureOf(context);
    observedSignatures.add(signature);
    return payloads.map((payload) => {
      const encoded = payloadSignature(payload);
      if (encoded !== signature) {
        throw new Error(`serialization context mismatch: payload encoded as '${encoded}', decoded as '${signature}'`);
      }
      const metadata = { ...payload.metadata };
      delete metadata[METADATA_KEY];
      return { ...payload, metadata };
    });
  }
}

export function payloadSignature(payload?: { metadata?: Record<string, Uint8Array> | null } | null): string {
  const raw = payload?.metadata?.[METADATA_KEY];
  return raw ? decode(raw) : '';
}

export function firstSignature(
  payloads?: { payloads?: { metadata?: Record<string, Uint8Array> | null }[] | null } | null,
): string {
  return payloadSignature(payloads?.payloads?.[0]);
}

export function findEvent<T>(events: T[], name: string, predicate: (event: T) => boolean): T {
  const event = events.find(predicate);
  if (!event) {
    throw new Error(`no ${name} event in history`);
  }
  return event;
}
