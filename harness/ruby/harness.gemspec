# frozen_string_literal: true

Gem::Specification.new do |spec|
  spec.name = 'harness'
  spec.version = '0.1.0'
  spec.authors = ['Temporal Technologies']
  spec.summary = 'Temporal features test harness for Ruby'
  spec.files = Dir['lib/**/*.rb']
  spec.require_paths = ['lib']
  spec.required_ruby_version = '>= 4.0.0'
  spec.metadata['rubygems_mfa_required'] = 'true'
end
