namespace Temporalio.Features.Harness;

using Temporalio.Client;
using Temporalio.Worker;

/// <summary>
/// Interface that must be implemented by all features.
/// </summary>
public interface IFeature
{
    /// <summary>
    /// Configure the worker options. This is where workflows and activities
    /// should be added.
    /// </summary>
    /// <param name="runner">Current runner.</param>
    /// <param name="options">Options to mutate.</param>
    void ConfigureWorker(Runner runner, TemporalWorkerOptions options);

    /// <summary>
    /// Execute the feature and optionally return a handle. If no handle is
    /// returned, <see cref="CheckResultAsync" /> and
    /// <see cref="CheckHistoryAsync" /> will not be called. The default
    /// implementation expects a single parameterless workflow to be on the
    /// worker and then starts it.
    /// </summary>
    /// <param name="runner">Current runner.</param>
    /// <returns>Task with handle or null.</returns>
    async Task<WorkflowHandle?> ExecuteAsync(Runner runner) => await runner.StartSingleParameterlessWorkflowAsync();

    /// <summary>
    /// Check result for the given workflow handle. The default implementation
    /// just gets the result to make sure it didn't fail.
    /// </summary>
    /// <param name="runner">Current runner.</param>
    /// <param name="handle">Workflow handle.</param>
    /// <returns>Task for completion.</returns>
    Task CheckResultAsync(Runner runner, WorkflowHandle handle) => handle.GetResultAsync();

    /// <summary>
    /// Check history for the given workflow handle. The default implementation
    /// just checks current history via replay.
    /// </summary>
    /// <param name="runner">Current runner.</param>
    /// <param name="handle">Workflow handle.</param>
    /// <returns>Task for completion.</returns>
    Task CheckHistoryAsync(Runner runner, WorkflowHandle handle) => runner.CheckCurrentHistoryAsync(handle);
}