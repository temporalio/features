import { Feature } from '@temporalio/harness';
import * as wf from '@temporalio/workflow';

const activities = wf.proxyActivities<typeof activitiesImpl>({
  startToCloseTimeout: '1 minute',
});
const activitiesSched2Close = wf.proxyActivities<typeof activitiesImpl>({
   scheduleToCloseTimeout: '1 minute',
});

export async function workflow(): Promise<string> {
  await activitiesSched2Close.echo('hello');
  return await activities.echo('hello');
}

const activitiesImpl = {
  async echo(input: string): Promise<string> {
    return input;
  },
};

export const feature = new Feature({
  workflow,
  workflowStartOptions: { workflowExecutionTimeout: undefined },
  activities: activitiesImpl,
});
