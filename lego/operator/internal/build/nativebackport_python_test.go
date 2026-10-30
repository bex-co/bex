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

func TestNativePythonBackportBuildAndStart(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the reviewed Python backport")
	}
	line, ok := nativeToolchains[nativePythonRuntime].find("3.7")
	if !ok {
		t.Fatal("missing reviewed Python 3.7 line")
	}
	node20, ok := nativeToolchains[nativeNodeRuntime].find("20")
	if !ok {
		t.Fatal("missing reviewed Node 20 companion")
	}
	probe, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "deploy", "native-base", "check-brotli.py"))
	if err != nil {
		t.Fatal(err)
	}
	o := nativeOptions()
	o.Runtime = nativePythonRuntime
	o.BuildCommand = "/usr/bin/python3 system-probe.py && python runtime_info.py > built.json"
	o.StartCommand = "/usr/bin/python3 system-probe.py && python app.py"
	files := map[string]string{
		"system-probe.py": string(probe),
		"runtime_info.py": `import json, platform, ssl, subprocess
def current():
    node = json.loads(subprocess.check_output(["node", "-p", "JSON.stringify({node:process.versions.node,brotli:process.versions.brotli})"], universal_newlines=True))
    return dict(node, python=platform.python_version(), poetry=subprocess.check_output(["poetry", "--version"], universal_newlines=True).strip(), ssl=ssl.OPENSSL_VERSION)
if __name__ == "__main__":
    print(json.dumps(current()))
`,
		"app.py": `import json
from http.server import BaseHTTPRequestHandler, HTTPServer
from runtime_info import current
running = current()
with open("built.json") as built:
    assert running == json.load(built), "build and runtime versions differ"
class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(json.dumps(running).encode())
HTTPServer(("127.0.0.1", 10000), Handler).serve_forever()
`,
	}
	tag := buildResolvedImage(t, o, map[string]string{"PYTHON_VERSION": line.version, "POETRY_VERSION": "1.5.1", "NODE_VERSION": node20.version}, files,
		fmt.Sprintf("bex-m188-python37-start:%d", os.Getpid()))
	body, _ := runServing(t, tag, fmt.Sprintf("bex-m188-python37-http-%d", os.Getpid()), "http://127.0.0.1:10000/", "--network", "none")
	var got struct{ Python, Poetry, SSL, Node, Brotli string }
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("runtime response: %v: %s", err, body)
	}
	if got.Python != line.version || got.Poetry != "Poetry (version 1.5.1)" || got.SSL == "" || got.Node != node20.version || got.Brotli == "" {
		t.Fatalf("runtime = %+v; want Python %s, Poetry 1.5.1, working SSL and Node %s with Brotli", got, line.version, node20.version)
	}
	t.Logf("build and HTTP start preserved Python %s, %s and Node %s with Brotli %s; system and provided Node decoder probes passed", got.Python, got.Poetry, got.Node, got.Brotli)
}
