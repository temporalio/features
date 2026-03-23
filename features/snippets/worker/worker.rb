# frozen_string_literal: true

require 'temporalio/client'
require 'temporalio/worker'

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