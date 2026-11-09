# frozen_string_literal: true

require 'securerandom'

require 'temporalio/activity'
require 'temporalio/client'

require 'harness'

class StandaloneEchoActivity < Temporalio::Activity::Definition
  def execute(value)
    value
  end
end

start = proc do |client, task_queue, _feature|
  client.start_activity(
    StandaloneEchoActivity,
    'hello',
    id: SecureRandom.uuid,
    task_queue: task_queue,
    start_to_close_timeout: 60
  )
end

check_result = proc do |handle, _feature|
  result = handle.result
  raise "Expected result \"hello\", got #{result.inspect}" unless result == 'hello'

  status = handle.describe.status
  completed = Temporalio::Client::ActivityExecutionStatus::COMPLETED
  raise "Expected status #{completed}, got #{status}" unless status == completed
end

Harness.register_feature(
  workflows: [],
  activities: [StandaloneEchoActivity],
  start: start,
  check_result: check_result
)
