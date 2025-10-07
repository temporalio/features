import * as activity from '@temporalio/activity';
import { Connection, Client } from '@temporalio/client';
import { ActivityExecuteInput, ActivityInboundCallsInterceptor, Next } from '@temporalio/worker';

// FIXME: Remove this interceptor once 1.12.4 has been released
//        https://github.com/temporalio/sdk-typescript/pull/1769
export class ConnectionInjectorInterceptor implements ActivityInboundCallsInterceptor {
  constructor(public readonly connection: Connection, public readonly client: Client) {}
  async execute(input: ActivityExecuteInput, next: Next<ActivityInboundCallsInterceptor, 'execute'>): Promise<unknown> {
    Object.assign(activity.Context.current(), {
      injectedConnection: this.connection,
      injectedClient: this.client,
    });
    return next(input);
  }
}

/**
 * Extend the basic activity Context
 */
export interface Context extends activity.Context {
  injectedConnection: Connection;
  injectedClient: Client;
}

/**
 * Get the client object associated with the current activity context
 */
export function getClient(): Client {
  return (activity.Context.current() as unknown as Context).injectedClient;
}

/**
 * Get the connection object associated with the current activity context
 */
export function getConnection(): Connection {
  return (activity.Context.current() as unknown as Context).injectedConnection;
}
