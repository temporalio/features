# frozen_string_literal: true

require 'json'
require 'securerandom'

require 'temporalio/activity'
require 'temporalio/workflow'

require 'harness'

class PollCompleteWorkflow < Temporalio::Workflow::Definition
  def execute
    loop do
      Temporalio::Workflow.sleep(0.02)
      Temporalio::Workflow.execute_activity(
        NoopActivity,
        schedule_to_close_timeout: 10,
        start_to_close_timeout: 5
      )
    end
  end
end

class NoopActivity < Temporalio::Activity::Definition
  def execute
    nil
  end
end

Harness.register_feature(
  workflows: [PollCompleteWorkflow],
  activities: [NoopActivity],
  start: lambda do |client, task_queue, _feature|
    expect_worker_poll_complete_on_shutdown
    client.start_workflow(
      PollCompleteWorkflow,
      id: "worker_shutdown/poll_complete_on_shutdown-#{SecureRandom.uuid}",
      task_queue: task_queue,
      execution_timeout: 60
    )
  end,
  check_result: lambda do |handle, _feature|
    handle.terminate(reason: 'feature cleanup')
  rescue StandardError
    nil
  end
)

def expect_worker_poll_complete_on_shutdown
  capabilities_json = ENV['FEATURE_NAMESPACE_CAPABILITIES']
  raise 'FEATURE_NAMESPACE_CAPABILITIES is required' if capabilities_json.nil? || capabilities_json.empty?

  capabilities = JSON.parse(capabilities_json)
  unless capabilities.key?('workerPollCompleteOnShutdown')
    raise 'FEATURE_NAMESPACE_CAPABILITIES missing workerPollCompleteOnShutdown'
  end

  capabilities['workerPollCompleteOnShutdown']
end
