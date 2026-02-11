# frozen_string_literal: true

module Harness
  Feature = Struct.new(
    :workflows,
    :activities,
    :expect_run_result,
    :expect_activity_error,
    :start_callback,
    :check_result_callback,
    keyword_init: true
  )

  @features = {}

  class << self
    attr_reader :features
  end

  def self.register_feature(
    workflows:,
    activities: [],
    expect_run_result: nil,
    expect_activity_error: nil,
    start: nil,
    check_result: nil
  )
    rel_dir = caller_feature_dir(caller_locations(1, 1).first.path)
    @features[rel_dir] = Feature.new(
      workflows: workflows,
      activities: activities,
      expect_run_result: expect_run_result,
      expect_activity_error: expect_activity_error,
      start_callback: start,
      check_result_callback: check_result
    )
  end

  def self.caller_feature_dir(file_path)
    parts = file_path.gsub('\\', '/').split('/')
    features_idx = parts.rindex('features')
    raise "Cannot determine feature dir from path: #{file_path}" unless features_idx

    parts[(features_idx + 1)...-1].join('/')
  end

  class SkipFeature < StandardError; end
end
