# frozen_string_literal: true

require 'temporalio/simple_plugin'
require 'temporalio/activity'
require 'temporalio/workflow'

# @@@SNIPSTART ruby-plugin-activity
def some_activity
  # Activity implementation
end

plugin = Temporalio::SimplePlugin.new(
  name: 'organization.PluginName',
  activities: [method(:some_activity)]
)
# @@@SNIPEND

# @@@SNIPSTART ruby-plugin-workflow
class HelloWorkflow < Temporalio::Workflow::Definition
  def execute(name)
    "Hello, #{name}!"
  end
end

plugin = Temporalio::SimplePlugin.new(
  name: 'organization.PluginName',
  workflows: [HelloWorkflow]
)
# @@@SNIPEND

# @@@SNIPSTART ruby-plugin-converter
custom_converter = Temporalio::Converters::DataConverter.new(
  payload_converter: Temporalio::Converters::PayloadConverter.default
)

plugin = Temporalio::SimplePlugin.new(
  name: 'organization.PluginName',
  data_converter: custom_converter
)
# @@@SNIPEND

# @@@SNIPSTART ruby-plugin-interceptors
class SomeWorkerInterceptor
  include Temporalio::Worker::Interceptor::Workflow

  def intercept_workflow(next_interceptor)
    # Your interceptor implementation
    next_interceptor
  end
end

class SomeClientInterceptor
  include Temporalio::Client::Interceptor

  def intercept_client(next_interceptor)
    # Your interceptor implementation
    next_interceptor
  end
end

plugin = Temporalio::SimplePlugin.new(
  name: 'organization.PluginName',
  client_interceptors: [SomeClientInterceptor.new],
  worker_interceptors: [SomeWorkerInterceptor.new]
)
# @@@SNIPEND
