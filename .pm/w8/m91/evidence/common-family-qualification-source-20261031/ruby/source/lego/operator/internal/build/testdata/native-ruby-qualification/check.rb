require 'rbconfig'
require 'json'
require 'openssl'
require 'digest'
require 'fiddle'
require 'yaml'
require 'zlib'
require 'date'
require 'stringio'
require 'socket'
require 'open3'
require 'fileutils'
require_relative 'ext/bex_ruby_probe'

expected_ruby, expected_node, mode, phase = ARGV
abort 'invalid check arguments' unless ARGV.length == 4 && %w[primary static].include?(mode) && %w[build runtime].include?(phase)
raise 'wrong selected Ruby' unless RUBY_VERSION == expected_ruby
raise 'wrong Ruby executable' unless File.realpath(RbConfig.ruby) == '/usr/local/bin/ruby'
raise 'wrong RbConfig release' unless %w[MAJOR MINOR TEENY].map { |key| RbConfig::CONFIG.fetch(key) }.join('.') == expected_ruby
raise 'wrong selected headers' unless BexRubyProbe.compiled_api == expected_ruby.split('.')[0, 2].map(&:to_i)
raise 'wrong linked Ruby' unless BexRubyProbe.runtime_version == expected_ruby
text = "Ruby 雪 ' $ \\\""
raise 'native Unicode roundtrip failed' unless BexRubyProbe.echo(text) == text

expected_env = {
  'LANG' => 'C.UTF-8',
  'BUNDLE_PATH' => '/opt/render/project/.gems',
  'BUNDLE_BIN' => '/opt/render/project/.gems/bin'
}
if mode == 'primary'
  expected_env.merge!('RUBY_VERSION' => expected_ruby, 'RAILS_ENV' => 'production',
                      'RAILS_SERVE_STATIC_FILES' => 'true', 'RAILS_LOG_TO_STDOUT' => 'true',
                      'BUNDLE_DEPLOYMENT' => 'true', 'BUNDLE_APP_CONFIG' => '/opt/render/project/.gems')
else
  # A Ruby companion does not replace the Node primary's version ENV. Its
  # selected Ruby identity is checked from the interpreter and native module.
  expected_env.merge!('NODE_VERSION' => expected_node, 'BUNDLE_SILENCE_ROOT_WARNING' => '1')
  raise 'companion introduced Ruby primary defaults' if ENV.key?('BUNDLE_DEPLOYMENT') || ENV.key?('RUBY_VERSION')
end
expected_env.each { |key, value| raise "wrong #{key}: #{ENV[key].inspect}" unless ENV[key] == value }
raise 'missing Bundler bin PATH' unless ENV.fetch('PATH').split(':').first == expected_env.fetch('BUNDLE_BIN')
raise 'wrong working directory' unless Dir.pwd == '/opt/render/project/src'
raise 'wrong source locale' unless Encoding.default_external == Encoding::UTF_8

node, status = Open3.capture2('node', '-p', 'process.versions.node')
raise 'selected Node companion/primary changed' unless status.success? && node.strip == expected_node
raise 'OpenSSL digest failed' unless OpenSSL::Digest.hexdigest('SHA256', text) == Digest::SHA256.hexdigest(text)
strlen = Fiddle::Function.new(Fiddle.dlopen(nil)['strlen'], [Fiddle::TYPE_VOIDP], Fiddle::TYPE_SIZE_T)
raise 'Fiddle/libffi call failed' unless strlen.call(text) == text.bytesize
raise 'YAML failed' unless YAML.safe_load("value: #{text.to_json}\n") == { 'value' => text }
raise 'zlib failed' unless Zlib::Inflate.inflate(Zlib::Deflate.deflate(text)) == text
raise 'date failed' unless Date.iso8601('2026-10-31').day == 31
raise 'StringIO failed' unless StringIO.new(text).read == text
raise 'socket stdlib failed' unless Addrinfo.ip('127.0.0.1').ip_address == '127.0.0.1'
bundle, status = Open3.capture2('bundle', '--version')
# Bundler 2/3 print "Bundler version x.y.z"; Bundler 4 prints just x.y.z.
bundle_version = bundle.strip.delete_prefix('Bundler version ')
raise 'Bundler unavailable' unless status.success? && /\A\d+\.\d+\.\d+\z/.match?(bundle_version)

proof = {
  pass: true, ruby: RUBY_VERSION, node: node.strip, mode: mode, phase: phase,
  compiled_api: BexRubyProbe.compiled_api, openssl: OpenSSL::OPENSSL_VERSION,
  openssl_library: OpenSSL::OPENSSL_LIBRARY_VERSION,
  bundler: bundle_version, env: expected_env.keys.to_h { |key| [key, ENV.fetch(key)] },
  arch: RbConfig::CONFIG.fetch('arch'), native_extension: true, stdlib: true,
  source_root: Dir.pwd
}
FileUtils.mkdir_p('qa-output')
File.write("qa-output/#{phase}.json", JSON.pretty_generate(proof))
puts JSON.pretty_generate(proof)
