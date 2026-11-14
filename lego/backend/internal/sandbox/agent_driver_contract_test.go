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

package sandbox

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"testing"
)

// The two sides of a stop request are written apart: bex-api reads the status
// file and writes the interrupt sentinel inside the sandbox (this package), and
// the TypeScript driver writes the status file and watches the sentinel. A path
// that drifts on one side leaves a Stop turn that is accepted and does nothing,
// or a turn that never settles, so the driver's own defaults are read from its
// source and held to the constants used here.
func TestSessionFilePathsMatchTheDriversDefaults(t *testing.T) {
	driverSource := func(name string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "agent-image", "driver", "src", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	match := func(source, name, pattern string) string {
		t.Helper()
		found := regexp.MustCompile(pattern).FindStringSubmatch(source)
		if found == nil {
			t.Fatalf("%s no longer matches %s: update this test with the driver", name, pattern)
		}
		return found[1]
	}

	// statusPath = env.BEX_AGENT_STATUS_FILE || "/var/run/bex-agent/status.json"
	statusPath := match(driverSource("config.ts"), "config.ts", `env\.BEX_AGENT_STATUS_FILE\s*\|\|\s*"([^"]+)"`)
	if statusPath != agentSessionStatusPath {
		t.Errorf("the driver writes its status to %q, bex-api reads %q", statusPath, agentSessionStatusPath)
	}
	// path.join(path.dirname(config.statusPath), "interrupt")
	sentinel := match(driverSource("interrupt.ts"), "interrupt.ts", `path\.join\(path\.dirname\(config\.statusPath\),\s*"([^"]+)"\)`)
	if got := path.Join(path.Dir(statusPath), sentinel); got != agentSessionInterruptPath {
		t.Errorf("the driver watches %q for a stop, bex-api writes %q", got, agentSessionInterruptPath)
	}
}
