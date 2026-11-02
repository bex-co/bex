require 'rbconfig'
require 'json'
require 'zlib'
require 'digest'
text = "Ruby 雪 ' $ \\\""
roundtrip = Zlib::Inflate.inflate(Zlib::Deflate.deflate(text))
puts JSON.generate({ruby:RUBY_VERSION,ruby_executable:File.realpath(RbConfig.ruby),zlib:Zlib::ZLIB_VERSION,zlib_runtime:Zlib.zlib_version,text_sha256:Digest::SHA256.hexdigest(text),text_hex:text.unpack1('H*'),expected_encoding:text.encoding.name,actual_encoding:roundtrip.encoding.name,literal_equality:roundtrip==text,binary_equality:roundtrip.b==text.b})
raise 'zlib failed' unless Zlib::Inflate.inflate(Zlib::Deflate.deflate(text)) == text
puts 'PASS'
