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

// Exercise the generated tenant build and start path, not only the publishing
// image's probes. Run after adopting verified backports, with Docker available.
func TestNativeBackportsBuildAndStart(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 to build and run the reviewed backports")
	}
	probe, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "deploy", "native-base", "node-brotli", "check.js"))
	if err != nil {
		t.Fatal(err)
	}
	systemProbe, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "deploy", "native-base", "check-brotli.py"))
	if err != nil {
		t.Fatal(err)
	}
	for _, major := range []string{"14", "16", "18", "20"} {
		t.Run("node"+major, func(t *testing.T) {
			line, ok := nativeToolchains[nativeNodeRuntime].find(major)
			if !ok {
				t.Fatal("missing reviewed Node line")
			}
			o := nativeOptions()
			o.BuildCommand = `/usr/bin/python3 system-probe.py && node probe.js && node -e 'require("fs").writeFileSync("built.json", JSON.stringify({node:process.versions.node,brotli:process.versions.brotli}))'`
			o.StartCommand = "/usr/bin/python3 system-probe.py && node probe.js && node app.js"
			files := map[string]string{
				"probe.js": string(probe), "system-probe.py": string(systemProbe),
				"app.js": `const assert = require("assert");
const http = require("http");
const built = require("./built.json");
const running = {node:process.versions.node,brotli:process.versions.brotli};
assert.deepStrictEqual(running, built);
const server = http.createServer((req,res) => res.end(JSON.stringify(running)));
server.listen(10000, "127.0.0.1");
`,
			}
			tag := buildResolvedImage(t, o, map[string]string{"NODE_VERSION": major}, files,
				fmt.Sprintf("bex-m188-native-start-%s:%d", major, os.Getpid()))
			body, _ := runServing(t, tag, fmt.Sprintf("bex-m188-native-http-%s-%d", major, os.Getpid()),
				"http://127.0.0.1:10000/", "--network", "none")
			var got struct {
				Node   string `json:"node"`
				Brotli string `json:"brotli"`
			}
			if err := json.Unmarshal([]byte(body), &got); err != nil {
				t.Fatalf("runtime response: %v: %s", err, body)
			}
			// A security backport can retain the original library version string;
			// the malformed-stream probes above establish the fix itself.
			if got.Node != line.version || got.Brotli == "" {
				t.Fatalf("runtime = %+v; want Node %s with Brotli", got, line.version)
			}
			t.Logf("build and HTTP start preserved Node %s; system and bundled decoder probes passed (%s)", got.Node, got.Brotli)
		})
	}
}
