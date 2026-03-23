# frozen_string_literal: true

require 'securerandom'

require 'temporalio/workflow'

require 'harness'

class BasicSignalWorkflow < Temporalio::Workflow::Definition
  def execute
    Temporalio::Workflow.wait_condition { @state }
  end

  workflow_signal
  def my_signal(arg)
    @state = arg
  end
end

start = proc do |client, task_queue, _feature|
  handle = client.start_workflow(
    BasicSignalWorkflow,
    id: "signal-basic-#{SecureRandom.uuid}",
    task_queue: task_queue,
    execution_timeout: 60
  )
  handle.signal(BasicSignalWorkflow.my_signal, 'arg')
  handle
end

Harness.register_feature(
  workflows: [BasicSignalWorkflow],
  expect_run_result: 'arg',
  start: start
)
