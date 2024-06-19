import * as activity from '@temporalio/activity';
import { Connection, Client } from '@temporalio/client';
import { ActivityExecuteInput, ActivityInboundCallsInterceptor, Next } from '@temporalio/worker';

export class ConnectionInjectorInterceptor implements ActivityInboundCallsInterceptor {
  constructor(public readonly connection: Connection, public readonly client: Client) {}
  async execute(input: ActivityExecuteInput, next: Next<ActivityInboundCallsInterceptor, 'execute'>): Promise<unknown> {
    Object.assign(activity.Context.current(), {
      connection: this.connection,
      client: this.client,
    });
    return next(input);
  }
}

/**
 * Extend the basic activity Context
 */
export interface Context extends activity.Context {
  connection: Connection;
  client: Client;
}

/**
 * Get the client object associated with the current activity context
 */
export function getClient(): Client {
  return (activity.Context.current() as unknown as Context).client;
}

/**
 * Get the connection object associated with the current activity context
 */
export function getConnection(): Connection {
  return (activity.Context.current() as unknown as Context).connection;
}
