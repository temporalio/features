# frozen_string_literal: true

require 'json'
require 'securerandom'
require 'opentelemetry/sdk'
require 'temporalio/contrib/open_telemetry'
require 'temporalio/converters/data_converter'
require 'temporalio/converters/payload_codec'
require 'temporalio/workflow'

module EncodedHeaders
  class WrappingCodec < Temporalio::Converters::PayloadCodec
    def encode(payloads)
      payloads.map do |payload|
        raise 'Payload encoded twice' if payload.metadata['encoding'] == 'binary/encrypted'

        Temporalio::Api::Common::V1::Payload.new(
          metadata: { 'encoding' => 'binary/encrypted' },
          data: Temporalio::Api::Common::V1::Payload.encode(payload).bytes.map { |b| b ^ 0x5a }.pack('C*')
        )
      end
    end

    def decode(payloads)
      payloads.map do |payload|
        raise 'Expected encoded payload' unless payload.metadata['encoding'] == 'binary/encrypted'

        Temporalio::Api::Common::V1::Payload.decode(payload.data.bytes.map { |b| b ^ 0x5a }.pack('C*'))
      end
    end
  end

  class Workflow < Temporalio::Workflow::Definition
    workflow_name 'workflow'

    def execute(message)
      span = Temporalio::Contrib::OpenTelemetry::Workflow.completed_span('feature-result')
      raise 'No active workflow tracing span' unless span&.context&.valid?

      { 'message' => message, 'traceId' => span.context.hex_trace_id }
    end
  end

  PROVIDER = OpenTelemetry::SDK::Trace::TracerProvider.new
  TRACER = PROVIDER.tracer('encoded-headers-feature')

  def self.start(client, task_queue, _feature)
    TRACER.in_span('ruby-caller') do |parent|
      trace_id = parent.context.hex_trace_id
      handle = client.start_workflow(
        'workflow', 'hello', id: SecureRandom.uuid, task_queue: task_queue, execution_timeout: 15
      )
      begin
        result = handle.result
      ensure
        history = handle.fetch_history
        history.events.each do |event|
          raise "Workflow task failed: #{event.workflow_task_failed_event_attributes}" if event.workflow_task_failed_event_attributes
        end
      end
      header = history.events.first.workflow_execution_started_event_attributes.header.fields['_tracer-data']
      carrier = JSON.parse(WrappingCodec.new.decode([header]).first.data)
      raise "History trace identity mismatch: #{carrier}" unless carrier['traceparent'].split('-')[1] == trace_id
      raise "Input/trace round-trip mismatch: #{result}" unless result == { 'message' => 'hello', 'traceId' => trace_id }

      handle
    end
  end
end

Harness.register_feature(
  workflows: [EncodedHeaders::Workflow],
  start: EncodedHeaders.method(:start),
  worker_options: { workflow_payload_codec_thread_pool: Temporalio::Worker::ThreadPool.default },
  client_options: {
    data_converter: Temporalio::Converters::DataConverter.new(payload_codec: EncodedHeaders::WrappingCodec.new),
    interceptors: [Temporalio::Contrib::OpenTelemetry::TracingInterceptor.new(EncodedHeaders::TRACER)]
  }
)
