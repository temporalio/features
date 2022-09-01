import * as activity from '@temporalio/activity';
import { Connection, WorkflowClient } from '@temporalio/client';
import { ActivityExecuteInput, ActivityInboundCallsInterceptor, Next } from '@temporalio/worker';

export class ConnectionInjectorInterceptor implements ActivityInboundCallsInterceptor {
  constructor(public readonly connection: Connection, public readonly workflowClient: WorkflowClient) {}
  async execute(input: ActivityExecuteInput, next: Next<ActivityInboundCallsInterceptor, 'execute'>): Promise<unknown> {
    Object.assign(activity.Context.current(), {
      connection: this.connection,
      workflowClient: this.workflowClient,
    });
    return next(input);
  }
}

/**
 * Extend the basic activity Context
 */
export interface Context extends activity.Context {
  connection: Connection;
  workflowClient: WorkflowClient;
}

/**
 * Get the workflowClient object associated with the current activity context
 */
export function getWorkflowClient(): WorkflowClient {
  return (activity.Context.current() as unknown as Context).workflowClient;
}

/**
 * Get the connection object associated with the current activity context
 */
export function getConnection(): Connection {
  return (activity.Context.current() as unknown as Context).connection;
}
