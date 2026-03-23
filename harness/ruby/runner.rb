# frozen_string_literal: true

require 'json'
require 'optparse'
require 'securerandom'
require 'socket'
require 'uri'

require 'temporalio/client'
require 'temporalio/worker'

require 'harness'

module Harness
  class Runner
    def initialize(argv)
      @features_arg = []
      parse_args(argv)
    end

    def run
      summary_io = open_summary
      failed_features = []

      @features_arg.each do |feature_and_queue|
        rel_dir, task_queue = feature_and_queue.split(':', 2)
        entry = { name: rel_dir, outcome: 'PASSED', message: '' }

        begin
          run_feature(rel_dir, task_queue)
        rescue Harness::SkipFeature => e
          entry[:outcome] = 'SKIPPED'
          entry[:message] = e.message
          warn "Feature #{rel_dir} skipped: #{e.message}"
        rescue StandardError => e
          entry[:outcome] = 'FAILED'
          entry[:message] = e.message
          warn "Feature #{rel_dir} failed: #{e.class}: #{e.message}"
          warn e.backtrace.first(10).join("\n") if e.backtrace
          failed_features << rel_dir
        end

        write_summary_entry(summary_io, entry)
      end

      summary_io&.close

      if failed_features.any?
        warn "#{failed_features.size} feature(s) failed: #{failed_features.join(', ')}"
        exit 1
      end

      warn 'All features passed'
    end

    private

    def parse_args(argv)
      parser = OptionParser.new do |opts|
        opts.banner = 'Usage: runner.rb [options] feature:taskqueue ...'

        opts.on('--server HOST', 'The host:port of the server') { |v| @server = v }
        opts.on('--namespace NS', 'The namespace to use') { |v| @namespace = v }
        opts.on('--client-cert-path PATH', 'Path to a client certificate for TLS') { |v| @client_cert_path = v }
        opts.on('--client-key-path PATH', 'Path to a client key for TLS') { |v| @client_key_path = v }
        opts.on('--ca-cert-path PATH', 'Path to a CA certificate') { |v| @ca_cert_path = v }
        opts.on('--tls-server-name NAME', 'TLS server name override') { |v| @tls_server_name = v }
        opts.on('--http-proxy-url URL', 'HTTP proxy URL') { |v| @http_proxy_url = v }
        opts.on('--summary-uri URI', 'Where to stream the test summary JSONL') { |v| @summary_uri = v }
      end

      @features_arg = parser.parse(argv)

      raise ArgumentError, 'Missing --server' unless @server
      raise ArgumentError, 'Missing --namespace' unless @namespace
      raise ArgumentError, 'No features specified' if @features_arg.empty?
    end

    def open_summary
      return nil unless @summary_uri

      uri = URI.parse(@summary_uri)
      case uri.scheme
      when 'tcp'
        TCPSocket.new(uri.host, uri.port)
      when 'file'
        File.open(uri.path, 'w')
      else
        raise "Unsupported summary scheme: #{uri.scheme}"
      end
    end

    def write_summary_entry(summary_io, entry)
      return unless summary_io

      summary_io.puts(JSON.generate(entry))
      summary_io.flush
    end

    def connect_client
      connect_options = {}

      if @client_cert_path
        raise ArgumentError, 'Client cert specified, but not client key!' unless @client_key_path

        connect_options[:tls] = build_tls_options
      end

      if @http_proxy_url
        connect_options[:http_connect_proxy] =
          Temporalio::Client::Connection::HTTPConnectProxyOptions.new(target_host: @http_proxy_url)
      end

      Temporalio::Client.connect(@server, @namespace, **connect_options)
    end

    def build_tls_options
      tls_opts = {
        client_cert: File.binread(@client_cert_path),
        client_private_key: File.binread(@client_key_path)
      }
      tls_opts[:server_root_ca_cert] = File.binread(@ca_cert_path) if @ca_cert_path
      tls_opts[:domain] = @tls_server_name if @tls_server_name
      Temporalio::Client::Connection::TLSOptions.new(**tls_opts)
    end

    def run_feature(rel_dir, task_queue)
      warn "Running feature #{rel_dir}"

      load_feature_file(rel_dir)
      feature = Harness.features[rel_dir]
      raise "Feature #{rel_dir} not registered after loading" unless feature

      client = connect_client
      execute_feature(client, feature, rel_dir, task_queue)
    end

    def load_feature_file(rel_dir)
      feature_file = File.join(features_root, rel_dir, 'feature.rb')
      raise "Feature file not found: #{feature_file}" unless File.exist?(feature_file)

      load feature_file
    end

    def execute_feature(client, feature, rel_dir, task_queue)
      worker = Temporalio::Worker.new(
        client: client,
        task_queue: task_queue,
        activities: feature.activities,
        workflows: feature.workflows
      )

      worker.run do
        handle = start_workflow(client, feature, rel_dir, task_queue)
        check_result(client, handle, feature)
      end
    end

    def start_workflow(client, feature, rel_dir, task_queue)
      if feature.start_callback
        feature.start_callback.call(client, task_queue, feature)
      else
        start_default_workflow(client, feature, rel_dir, task_queue)
      end
    end

    def start_default_workflow(client, feature, rel_dir, task_queue)
      raise 'Must have exactly one workflow for default start' unless feature.workflows.size == 1

      client.start_workflow(
        feature.workflows.first,
        id: "#{rel_dir}-#{SecureRandom.uuid}",
        task_queue: task_queue,
        execution_timeout: 60
      )
    end

    def check_result(_client, handle, feature)
      if feature.check_result_callback
        feature.check_result_callback.call(handle, feature)
      else
        check_default_result(handle, feature)
      end
    end

    def check_default_result(handle, feature)
      result = handle.result
      return if feature.expect_run_result.nil?
      return if result == feature.expect_run_result

      raise "Expected result #{feature.expect_run_result.inspect}, got #{result.inspect}"
    end

    def features_root
      File.expand_path('../../features', __dir__)
    end
  end
end

Harness::Runner.new(ARGV).run
