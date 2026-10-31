/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package build

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeRubyBackportBuildAndStart(t *testing.T) {
	requireNativeBackportCommonFoundation(t)
	line, ok := nativeToolchains[nativeRubyRuntime].find("3.0")
	if !ok {
		t.Fatal("missing reviewed Ruby 3.0 line")
	}
	node20, ok := nativeToolchains[nativeNodeRuntime].find("20")
	if !ok {
		t.Fatal("missing reviewed Node 20 companion")
	}
	t.Logf("selected Ruby %s image: %s; Node %s companion image: %s", line.version, line.image, node20.version, node20.image)
	probe, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "deploy", "native-base", "check-brotli.py"))
	if err != nil {
		t.Fatal(err)
	}
	o := nativeOptions()
	o.Runtime = nativeRubyRuntime
	o.BuildCommand = "/usr/bin/python3 system-probe.py && ruby runtime_info.rb > built.json"
	o.StartCommand = "/usr/bin/python3 system-probe.py && ruby app.rb"
	files := map[string]string{
		".ruby-version":   "3.0\n",
		"system-probe.py": string(probe),
		"runtime_info.rb": `require "json"
require "openssl"
require "zlib"
def current_runtime
  node = JSON.parse(IO.popen(["node", "-p", "JSON.stringify({node:process.versions.node,brotli:process.versions.brotli})"], &:read))
  {"ruby" => RUBY_VERSION, "openssl" => OpenSSL::OPENSSL_VERSION, "zlib" => Zlib.zlib_version}.merge(node)
end
puts JSON.generate(current_runtime) if $PROGRAM_NAME == __FILE__
`,
		"app.rb": `require "socket"
require_relative "runtime_info"
running = current_runtime
raise "build and runtime versions differ" unless running == JSON.parse(File.read("built.json"))
body = JSON.generate(running)
TCPServer.open("127.0.0.1", 10000) do |server|
  loop do
    client = server.accept
    begin
      while (header = client.gets)
        break if header == "\r\n" || header == "\n"
      end
      client.write("HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: #{body.bytesize}\r\nConnection: close\r\n\r\n#{body}")
    ensure
      client.close
    end
  end
end
`,
	}
	tag := buildResolvedImage(t, o, map[string]string{"NODE_VERSION": node20.version}, files, fmt.Sprintf("bex-m188-ruby30-start:%d", os.Getpid()))
	body, _ := runServing(t, tag, fmt.Sprintf("bex-m188-ruby30-http-%d", os.Getpid()), "http://127.0.0.1:10000/", "--network", "none")
	var got struct{ Ruby, OpenSSL, Zlib, Node, Brotli string }
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("runtime response: %v: %s", err, body)
	}
	if got.Ruby != line.version || got.OpenSSL == "" || got.Zlib == "" || got.Node != node20.version || got.Brotli == "" {
		t.Fatalf("runtime = %+v; want Ruby %s with working OpenSSL, Zlib and Node %s with Brotli", got, line.version, node20.version)
	}
	t.Logf("build and HTTP start preserved Ruby %s, %s, Zlib %s and Node %s with Brotli %s; system and provided Node decoder probes passed", got.Ruby, got.OpenSSL, got.Zlib, got.Node, got.Brotli)
}
