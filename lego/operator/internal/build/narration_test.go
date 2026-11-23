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
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

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
` + shellLine(renderlog.BuildShell, `Downloaded $(bex_size "$bex_bytes") in $(( $(date +%s) - bex_t0 ))s. Extraction took 0s.`)
	out, err := exec.Command("sh", "-c", script).Output()
	if err != nil {
		t.Fatal(err)
	}
	if want := rendersShellLine("Downloaded 160MB in 7s. Extraction took 0s.") + "\n"; string(out) != want {
		t.Fatalf("printed %q, want %q", out, want)
	}

	uploadScript := `bex_summary="Uploaded in 9.9s. Compression took 2.4s"
` + shellLine(renderlog.BuildRuntime, "$bex_summary")
	uploadOut, err := exec.Command("sh", "-c", uploadScript).Output()
	if err != nil {
		t.Fatal(err)
	}
	if want := rendersRuntimeLine("Uploaded in 9.9s. Compression took 2.4s") + "\n"; string(uploadOut) != want {
		t.Fatalf("printed %q, want %q", uploadOut, want)
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

// w32/098: bex assembles every primary language runtime and Bun from prebuilt
// OCI images copied onto nativeCommonBase (never running an in-build runtime
// installer), while Poetry and uv install inside BuildKit after native-resolve
// has already narrated every runtime and companion. Neither path emits Render's
// `==> Installing <Runtime> version <v>...` line.
func TestNativeResolveOmitsInstallingLineAndUsesPrebuiltOCIImages(t *testing.T) {
	exact := &fakeExact{digests: map[string]string{
		"node:22.23.1-bookworm":  exactDigest,
		"node:25.8.1-bookworm":   exactDigest,
		"python:3.11.9-bookworm": exactDigest,
	}}
	for _, tc := range []struct {
		name           string
		runtime        string
		env            map[string]string
		repoFiles      map[string]string
		wantCopies     []string
		wantPipInstall []string
	}{
		{
			name:       "node default",
			runtime:    nativeNodeRuntime,
			wantCopies: []string{"COPY --link --from=node:24-bookworm@"},
		},
		{
			name:       "node exact env",
			runtime:    nativeNodeRuntime,
			env:        map[string]string{"NODE_VERSION": "22.23.1"},
			wantCopies: []string{"COPY --link --from=node:22.23.1-bookworm@" + exactDigest},
		},
		{
			name:       "node package.json range",
			runtime:    nativeNodeRuntime,
			repoFiles:  map[string]string{"package.json": `{"engines":{"node":">=20"}}`},
			wantCopies: []string{"COPY --link --from=node:26-bookworm@"},
		},
		{
			name:       "node unreviewed exact release",
			runtime:    nativeNodeRuntime,
			env:        map[string]string{"NODE_VERSION": "25.8.1"},
			wantCopies: []string{"COPY --link --from=node:25.8.1-bookworm@" + exactDigest},
		},
		{
			name:       "node with bun lockfile",
			runtime:    nativeNodeRuntime,
			repoFiles:  map[string]string{"bun.lock": "{}"},
			wantCopies: []string{"COPY --link --from=node:24-bookworm@", "COPY --link --from=oven/bun:"},
		},
		{
			name:           "python exact with poetry, uv and node companion",
			runtime:        nativePythonRuntime,
			env:            map[string]string{"PYTHON_VERSION": "3.11.9", "POETRY_VERSION": "1.8.3", "UV_VERSION": "0.13.7"},
			repoFiles:      map[string]string{"uv.lock": ""},
			wantCopies:     []string{"COPY --link --from=python:3.11.9-bookworm@" + exactDigest, "COPY --link --from=node:24-bookworm@"},
			wantPipInstall: []string{"poetry==1.8.3", "uv==0.13.7"},
		},
		{
			name:       "ruby file with node companion",
			runtime:    nativeRubyRuntime,
			repoFiles:  map[string]string{".ruby-version": "3.2\n"},
			wantCopies: []string{"COPY --link --from=ruby:3.2-bookworm@", "COPY --link --from=node:24-bookworm@"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			srcDir := filepath.Join(dir, "src")
			if err := os.MkdirAll(srcDir, 0o700); err != nil {
				t.Fatal(err)
			}
			for name, content := range tc.repoFiles {
				if err := os.WriteFile(filepath.Join(srcDir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var bundle strings.Builder
			for k, v := range tc.env {
				bundle.WriteString(k + "=" + base64.StdEncoding.EncodeToString([]byte(v)) + "\n")
			}
			bundlePath := filepath.Join(dir, "render-env")
			if err := os.WriteFile(bundlePath, []byte(bundle.String()), 0o600); err != nil {
				t.Fatal(err)
			}
			o := nativeOptions()
			o.Runtime = tc.runtime
			o.BuildCommand = "true"
			o.StartCommand = "true"
			dockerfilePath := filepath.Join(dir, "Dockerfile")
			if err := os.WriteFile(dockerfilePath, []byte(nativeDockerfile(o)), 0o600); err != nil {
				t.Fatal(err)
			}

			choices, err := ResolveNativeBuildExact(context.Background(), exact, time.Time{}, tc.runtime, bundlePath, dockerfilePath, srcDir)
			if err != nil {
				t.Fatalf("ResolveNativeBuildExact: %v", err)
			}
			for _, c := range choices {
				narration := c.NarrationIn(NativeWorkdir)
				if strings.Contains(shown(narration), "Installing ") {
					t.Errorf("%s narration unexpectedly emitted an Installing line:\n%s", c.Label, narration)
				}
			}
			raw, err := os.ReadFile(dockerfilePath)
			if err != nil {
				t.Fatal(err)
			}
			dockerfile := string(raw)
			if !strings.Contains(dockerfile, "\nFROM "+nativeCommonBase+"\n") {
				t.Errorf("Dockerfile does not build on nativeCommonBase:\n%s", dockerfile)
			}
			for _, want := range tc.wantCopies {
				if !strings.Contains(dockerfile, want) {
					t.Errorf("Dockerfile missing prebuilt OCI copy %q:\n%s", want, dockerfile)
				}
			}
			for _, want := range tc.wantPipInstall {
				if !strings.Contains(dockerfile, "pip install --no-cache-dir --disable-pip-version-check --no-input "+want) {
					t.Errorf("Dockerfile missing BuildKit pip install for %q:\n%s", want, dockerfile)
				}
			}
		})
	}
}
