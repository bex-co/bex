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
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// These controls resolve the complete recipe, not a runnable common image.
// Actual old-Python/Poetry project and dependency compatibility remains a
// separate image gate, including the tool venv's system-interpreter supply.
func TestCommonLegacyPythonDefaultPoetryBootstrap(t *testing.T) {
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	for _, tc := range []struct {
		name, line, exact, poetry, interpreter string
		historical, static, uv                 bool
	}{
		{name: "python37-default", line: "3.7", interpreter: "/usr/bin/python3"},
		{name: "python38-default", line: "3.8", interpreter: "/usr/bin/python3"},
		{name: "python37-exact", line: "3.7", exact: "3.7.16", interpreter: "/usr/bin/python3"},
		{name: "python38-exact", line: "3.8", exact: "3.8.19", interpreter: "/usr/bin/python3"},
		{name: "python37-empty-default", line: "3.7", poetry: " ", interpreter: "/usr/bin/python3"},
		{name: "python38-static", line: "3.8", static: true, interpreter: "/usr/bin/python3"},
		{name: "python38-uv-unchanged", line: "3.8", uv: true, interpreter: "/usr/bin/python3"},
		{name: "python37-compatible-explicit", line: "3.7", poetry: "1.5.1", interpreter: "/usr/local/bin/python"},
		{name: "python38-compatible-explicit", line: "3.8", poetry: "1.8.5", interpreter: "/usr/local/bin/python"},
		{name: "explicit-compatibility-contract", line: "3.7", poetry: "2.1.3", interpreter: "/usr/local/bin/python"},
		{name: "python39-unchanged", line: "3.9", interpreter: "/usr/local/bin/python"},
		{name: "python313-unchanged", line: "3.13", interpreter: "/usr/local/bin/python"},
		{name: "historical-route-unchanged", line: "3.7", historical: true, interpreter: "/usr/local/bin/python"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nativeCommonBase = commonFixtureImage
			if tc.historical {
				nativeCommonBase = ""
			}
			line, _ := nativeToolchains[nativePythonRuntime].find(tc.line)
			primary, version, selected := line.image, line.version, tc.line
			fake := &fakeExact{digests: map[string]string{}, envByImage: map[string]fakeExactMetadata{}}
			if tc.exact != "" {
				suite := "bookworm"
				if tc.line == "3.7" {
					suite = "bullseye"
				}
				tag := "python:" + tc.exact + "-" + suite
				fake.digests[tag] = exactDigest
				primary, version, selected = tag+"@"+exactDigest, tc.exact, tc.exact
			}
			fake.envByImage[primary] = fakeExactMetadata{env: []string{
				"PYTHON_VERSION=" + version, "PATH=/usr/local/bin:/usr/bin", "LANG=C.UTF-8", `LITERAL=$HOME\path"quoted`,
			}}
			node, _ := nativeToolchains[nativeNodeRuntime].find("24")
			fake.envByImage[node.image] = commonReviewedFixtureMetadata(nativeNodeRuntime, node)
			o := nativeOptions()
			o.Runtime, o.StaticSite = nativePythonRuntime, tc.static
			o.BuildCommand = `python -m pip --version && printf '%s' "build $PYTHON_VERSION"` + "\n"
			o.StartCommand = `python -c 'import sys; print(sys.version)'` + " "
			if tc.static {
				o.StartCommand = ""
			}
			env := map[string]string{"PYTHON_VERSION": selected, "POETRY_VERSION": tc.poetry, "BUN_VERSION": "1.4"}
			sourceFiles := map[string]string{}
			if tc.uv {
				sourceFiles["uv.lock"] = ""
			}
			bundle, path, source := writeResolveFixture(t, o, env, sourceFiles)
			choices, err := ResolveNativeBuildExact(context.Background(), fake, time.Time{}, o.Runtime, bundle, path, source)
			must(t, err)
			raw, err := os.ReadFile(path)
			must(t, err)
			recipe := string(raw)
			if choices[0].Image != primary || choices[0].Line != selected {
				t.Fatalf("Poetry bootstrap changed the selected application Python: %+v", choices[0])
			}
			poetry := defaultPoetryVersion
			if strings.TrimSpace(tc.poetry) != "" {
				poetry = tc.poetry
			}
			if choices[1].Label != "Poetry" || choices[1].Line != poetry ||
				(choices[1].Source == "") != (strings.TrimSpace(tc.poetry) == "") {
				t.Fatalf("Poetry version or default/explicit meaning changed: %+v", choices[1])
			}
			layer := resolvedPythonToolLayer(t, recipe, "poetry")
			if !strings.HasPrefix(layer, "RUN "+tc.interpreter+" -m venv /opt/bex/poetry && ") ||
				!strings.Contains(layer, "--no-input poetry=="+poetry+" && ln -s /opt/bex/poetry/bin/poetry /usr/local/bin/poetry") {
				t.Fatalf("wrong private tool interpreter or exact Poetry release: %s", layer)
			}
			if !tc.historical {
				assertCommonPrimaryOrder(t, recipe, primary, "PYTHON_VERSION", version, true)
			}
			for _, reject := range []string{"ENV VIRTUAL_ENV", "ENV PYTHONHOME", "ENV PYTHONPATH", "POETRY_VIRTUALENVS_USE_POETRY_PYTHON", "ENV PATH=\"/usr/bin"} {
				if strings.Contains(recipe, reject) {
					t.Fatalf("tool bootstrap changed application environment with %q", reject)
				}
			}
			for _, retained := range []string{" /usr/local/bin/node /usr/local/bin/node\n", " /usr/local/lib/node_modules /usr/local/lib/node_modules\n", " /usr/local/bin/bun /usr/local/bin/bun\n", "/usr/local/bin/yarn", "/usr/local/bin/pnpm", " /ffmpeg /ffprobe /\n"} {
				if !strings.Contains(recipe, retained) {
					t.Fatalf("Poetry bootstrap lost a companion: %q", retained)
				}
			}
			if tc.uv && !strings.HasPrefix(resolvedPythonToolLayer(t, recipe, "uv"), "RUN /usr/local/bin/python -m venv /opt/bex/uv && ") {
				t.Fatal("Poetry tool interpreter leaked into uv")
			}
			assertPoetryRecipeCommands(t, recipe, o)
		})
	}
}

func resolvedPythonToolLayer(t *testing.T, recipe, pkg string) string {
	t.Helper()
	var found []string
	for line := range strings.SplitSeq(recipe, "\n") {
		if strings.HasPrefix(line, "RUN ") && strings.Contains(line, " -m venv /opt/bex/"+pkg+" && ") {
			found = append(found, line)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected one private %s install, got %v", pkg, found)
	}
	return found[0]
}

func assertPoetryRecipeCommands(t *testing.T, recipe string, o Options) {
	t.Helper()
	var build, start []string
	for line := range strings.SplitSeq(recipe, "\n") {
		if strings.HasPrefix(line, "RUN --mount=type=secret,id=render-env,") {
			at := strings.Index(line, " [")
			if at < 0 {
				t.Fatal("authored build lost its shell argv")
			}
			must(t, json.Unmarshal([]byte(line[at+1:]), &build))
		}
		if command, ok := strings.CutPrefix(line, "CMD "); ok {
			must(t, json.Unmarshal([]byte(command), &start))
		}
	}
	if !slices.Equal(build, []string{"/bin/bash", "-c", ". /opt/bex/load-env\n" + o.BuildCommand}) {
		t.Fatalf("authored build command bytes changed: %q", build)
	}
	if o.StaticSite {
		if len(start) != 0 {
			t.Fatalf("static recipe gained a serving command: %q", start)
		}
	} else if !slices.Equal(start, []string{"/bin/bash", "-c", o.StartCommand}) {
		t.Fatalf("authored start command bytes changed: %q", start)
	}
}
