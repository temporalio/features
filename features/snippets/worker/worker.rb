# frozen_string_literal: true

require 'temporalio/activity'
require 'temporalio/client'
require 'temporalio/common_enums'
require 'temporalio/worker'
require 'temporalio/worker_deployment_version'
require 'temporalio/workflow'

class SayHello < Temporalio::Activity::Definition
  def execute(name)
    "Hello, #{name}!"
  end
end

class GreetingWorkflow < Temporalio::Workflow::Definition
  def execute(name)
    Temporalio::Workflow.execute_activity(SayHello, name, start_to_close_timeout: 10)
  end
end

# A versioning behavior is only valid on a Worker that has versioning enabled.
class VersionedGreetingWorkflow < Temporalio::Workflow::Definition
  workflow_versioning_behavior Temporalio::VersioningBehavior::PINNED

  def execute(name)
    Temporalio::Workflow.execute_activity(SayHello, name, start_to_close_timeout: 10)
  end
end

def run
  client = Temporalio::Client.connect(
    'localhost:7233',
    'default'
  )

  # @@@SNIPSTART ruby-worker-max-cached-workflows
  worker = Temporalio::Worker.new(
    client: client,
    task_queue: 'task-queue',
    max_cached_workflows: 0
  )
  # @@@SNIPEND

  worker.run
end

def run_worker
  client = Temporalio::Client.connect('localhost:7233', 'default')

  # @@@SNIPSTART ruby-create-worker
  worker = Temporalio::Worker.new(
    client: client,
    task_queue: 'my-task-queue',
    workflows: [GreetingWorkflow],
    activities: [SayHello]
  )

  worker.run
  # @@@SNIPEND
end

def run_versioned_worker
  client = Temporalio::Client.connect('localhost:7233', 'default')

  # @@@SNIPSTART ruby-versioned-worker
  worker = Temporalio::Worker.new(
    client: client,
    task_queue: 'my-task-queue',
    workflows: [VersionedGreetingWorkflow],
    activities: [SayHello],
    deployment_options: Temporalio::Worker::DeploymentOptions.new(
      version: Temporalio::WorkerDeploymentVersion.new(
        deployment_name: 'my-app',
        build_id: '1.0'
      ),
      use_worker_versioning: true
    )
  )
  # @@@SNIPEND

  worker.run
end

def run_worker_until_interrupted
  client = Temporalio::Client.connect('localhost:7233', 'default')

  worker = Temporalio::Worker.new(
    client: client,
    task_queue: 'my-task-queue',
    workflows: [GreetingWorkflow],
    activities: [SayHello]
  )

  # @@@SNIPSTART ruby-worker-graceful-shutdown
  worker.run(shutdown_signals: %w[SIGINT SIGTERM])
  # @@@SNIPEND
end
