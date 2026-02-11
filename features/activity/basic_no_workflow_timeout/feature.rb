# frozen_string_literal: true

require 'temporalio/activity'
require 'temporalio/workflow'

require 'harness'

class EchoActivity < Temporalio::Activity::Definition
  def execute
    'echo'
  end
end

class BasicNoWorkflowTimeoutWorkflow < Temporalio::Workflow::Definition
  def execute
    Temporalio::Workflow.execute_activity(EchoActivity, schedule_to_close_timeout: 60)
    Temporalio::Workflow.execute_activity(EchoActivity, start_to_close_timeout: 60)
  end
end

Harness.register_feature(
  workflows: [BasicNoWorkflowTimeoutWorkflow],
  activities: [EchoActivity],
  expect_run_result: 'echo'
)
