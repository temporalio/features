import { context, trace } from '@opentelemetry/api';

export async function workflow(message: string): Promise<{ message: string; traceId: string }> {
  const span = trace.getSpan(context.active());
  if (!span) throw new Error('No active workflow tracing span');
  return { message, traceId: span.spanContext().traceId };
}
