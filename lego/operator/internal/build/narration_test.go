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
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/types/renderlog"
)

// renderCodes are the escapes Render's forms use (w32/m576): SGR and the
// `ESC ( B` charset selection `tput sgr0` writes.
var renderCodes = regexp.MustCompile("\x1b(?:\\[[0-9;]*m|\\(B)")

// shown is s as a terminal shows it: Render's codes dropped.
func shown(s string) string { return renderCodes.ReplaceAllString(s, "") }

// Render's build lines as Render's log API returns them, byte for byte from
// ivanseibel/tretta's capture (04b-render-logs.json at b0f216c985), for the
// texts bex writes.
func rendersShellLine(text string) string {
	return "\x1b[34m\x1b[1m==>\x1b(B\x1b[m \x1b[1m" + text + "\x1b(B\x1b[m"
}

func rendersRuntimeLine(text string) string {
	return "\x1b[34;1m==>\x1b[0;22m \x1b[1m" + text + "\x1b[22m"
}

const rendersBuildSuccessful = "\x1b[32;1m==>\x1b[0;22m \x1b[1mBuild successful 🎉\x1b[22m"

// A script's printf of shellLine writes Render's bytes, the variables and
// commands in its text expanded as the echo before it did.
func TestShellLinePrintsRendersBytes(t *testing.T) {
	script := cacheSizeFunc + `bex_t0=$(( $(date +%s) - 7 ))
bex_bytes=160000000
` + shellLine(renderlog.BuildShell, `Transferred $(bex_size "$bex_bytes") in $(( $(date +%s) - bex_t0 ))s. Extraction took 0s.`)
	out, err := exec.Command("sh", "-c", script).Output()
	if err != nil {
		t.Fatal(err)
	}
	if want := rendersShellLine("Transferred 160MB in 7s. Extraction took 0s.") + "\n"; string(out) != want {
		t.Fatalf("printed %q, want %q", out, want)
	}
}

// Every awk the build pod may run reads awkString's spelling as Render's
// bytes.
func TestAwkStringPrintsRendersBytes(t *testing.T) {
	for _, name := range []string{"busybox", "mawk", "gawk", "awk"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		args := []string{`BEGIN { printf "` + awkString(renderlog.BuildShell, "Running build command %c%s%c...") + `\n", 39, "npm ci", 39; print "` +
			awkString(renderlog.BuildRuntime, "Uploading build...") + `" }`}
		if name == "busybox" {
			args = append([]string{"awk"}, args...)
		}
		out, err := exec.Command(path, args...).Output()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want := rendersShellLine("Running build command 'npm ci'...") + "\n" + rendersRuntimeLine("Uploading build...") + "\n"; string(out) != want {
			t.Errorf("%s printed %q, want %q", name, out, want)
		}
	}
}

// Node.js's version and docs lines are in Render's blue runtime form, as
// tretta's capture shows them; every other runtime's, the Node request line
// and bex's own lines stay plain, since no capture shows their codes.
func TestOnlyNodesVersionLinesCarryRendersForm(t *testing.T) {
	node, err := ResolveNativeVersion("node", map[string]string{"NODE_VERSION": "22"}, files(nil))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(node.Narration(), "\n")
	version, ok := strings.CutPrefix(shown(lines[0]), "==> Using Node.js version ")
	if len(lines) != 2 || !ok || lines[0] != rendersRuntimeLine("Using Node.js version "+version) ||
		!strings.HasSuffix(version, " via environment variable NODE_VERSION") ||
		lines[1] != rendersRuntimeLine("Docs on specifying a Node.js version: https://render.com/docs/node-version") {
		t.Fatalf("Node's narration = %q", lines)
	}
	ranged, err := ResolveNativeVersion("node", map[string]string{"NODE_VERSION": ">=20"}, files(nil))
	if err != nil {
		t.Fatal(err)
	}
	if first, _, _ := strings.Cut(ranged.Narration(), "\n"); first != "==> Requesting Node.js version >=20" {
		t.Errorf("the request line = %q, want it plain", first)
	}
	for _, runtime := range []string{"python", "ruby", "elixir", "go", "rust"} {
		c, err := ResolveNativeVersion(runtime, nil, files(nil))
		if err != nil {
			t.Fatal(err)
		}
		if n := c.Narration(); strings.Contains(n, "\x1b") {
			t.Errorf("%s's narration %q carries codes no Render capture shows", runtime, n)
		}
	}
}
