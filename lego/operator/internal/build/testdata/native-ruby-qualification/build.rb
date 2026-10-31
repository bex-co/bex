require 'rbconfig'
require 'open3'

ruby_version, node_version, mode = ARGV
abort 'usage: build.rb RUBY_VERSION NODE_VERSION primary|static' unless ARGV.length == 3 && %w[primary static].include?(mode)

def run!(*argv)
  output, status = Open3.capture2e(*argv)
  puts output
  abort "command failed: #{argv.inspect}" unless status.success?
end

Dir.chdir('ext') do
  run!(RbConfig.ruby, 'extconf.rb')
  run!('make', '-j2')
end

# Use the supplied Bundler and an existing dependency-free lockfile, including
# the primary Ruby recipe's deployment defaults. No gem source is configured.
run!('bundle', 'install', '--local')
run!('bundle', 'check')
run!('bundle', 'exec', 'ruby', '-e', 'abort "wrong bundled Ruby" unless RUBY_VERSION == ARGV.fetch(0); puts "offline-bundler-ruby=" + RUBY_VERSION', ruby_version)
run!(RbConfig.ruby, 'check.rb', ruby_version, node_version, mode, 'build')
