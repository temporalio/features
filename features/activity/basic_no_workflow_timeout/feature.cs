namespace activity.basic_no_workflow_timeout;

using Temporalio.Activities;
using Temporalio.Client;
using Temporalio.Exceptions;
using Temporalio.Features.Harness;
using Temporalio.Worker;
using Temporalio.Workflows;

class Feature : IFeature
{
    public void ConfigureWorker(Runner runner, TemporalWorkerOptions options) =>
        options.AddWorkflow<MyWorkflow>().AddAllActivities(new MyActivities(runner.Client));

    [Workflow]
    class MyWorkflow
    {
        private string? activityResult;

        [WorkflowRun]
        public async Task RunAsync()
        {
            await Workflow.ExecuteActivityAsync(
                (MyActivities act) => act.Echo(),
                new()
                {
                    ScheduleToCloseTimeout = TimeSpan.FromMinutes(1)
                });

            await Workflow.ExecuteActivityAsync(
                (MyActivities act) => act.Echo(),
                new()
                {
                    StartToCloseTimeout = TimeSpan.FromMinutes(1)
                });
        }

        [WorkflowSignal]
        public async Task SetActivityResultAsync(string res) => activityResult = res;
    }

    class MyActivities
    {
        private readonly ITemporalClient client;

        public MyActivities(ITemporalClient client) => this.client = client;

        [Activity]
        public async Task<string> Echo()
        {
            return "hi";
        }
    }
}