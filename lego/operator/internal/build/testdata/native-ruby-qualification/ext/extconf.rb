require 'mkmf'

abort 'missing selected Ruby headers' unless have_header('ruby.h') && have_header('ruby/version.h')
create_makefile('bex_ruby_probe')
