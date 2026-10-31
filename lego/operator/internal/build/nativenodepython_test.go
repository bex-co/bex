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
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHistoricalLegacyNodeAddonPythonIsPrivateForPrimaryAndCompanion(t *testing.T) {
	common, supplied := nativeCommonBase, nativeSuppliedBases
	t.Cleanup(func() { nativeCommonBase, nativeSuppliedBases = common, supplied })
	nativeCommonBase, nativeSuppliedBases = "", nil
	for _, tc := range []struct {
		runtime, node, python string
		private               bool
	}{
		{nativeNodeRuntime, "16", "", true},
		{nativePythonRuntime, "16", "3.13", true},
		{nativePythonRuntime, "14", "3.13", true},
		{nativeRubyRuntime, "16", "", true},
		// Do not introduce a newer-libc Python donor on a raw Bullseye
		// foundation. That historical combination's full ABI proof is separate.
		{nativePythonRuntime, "16", "3.7", false},
		{nativeNodeRuntime, "14", "", false},
		{nativeNodeRuntime, "24", "", false},
		{nativePythonRuntime, "24", "3.13", false},
	} {
		t.Run(tc.runtime+"/"+tc.node+"/"+tc.python, func(t *testing.T) {
			o := nativeOptions()
			o.Runtime = tc.runtime
			env := map[string]string{"NODE_VERSION": tc.node}
			if tc.python != "" {
				env["PYTHON_VERSION"] = tc.python
			}
			bundle, path, source := writeResolveFixture(t, o, env, nil)
			choices, err := ResolveNativeBuildExact(context.Background(), &fakeExact{}, time.Time{}, tc.runtime, bundle, path, source)
			must(t, err)
			raw, err := os.ReadFile(path)
			must(t, err)
			recipe := string(raw)
			if !strings.Contains(recipe, "FROM "+choices[0].Image+"\n") {
				t.Fatal("private tool interpreter replaced the selected primary")
			}
			if !tc.private {
				if strings.Contains(recipe, "native-node-python") || strings.Contains(recipe, "BEX_NODE_PYTHON") {
					t.Fatal("unrelated runtime acquired a private legacy tool interpreter")
				}
				return
			}
			python, donor, err := commonNodeBuildPython()
			must(t, err)
			copyLine := "COPY --link --from=" + donor + " /usr/local/ /opt/bex/native-node-python/\n"
			runLine := "RUN --network=none /opt/bex/native-node-python/bin/python3.9 -I /usr/libexec/bex-native/prepare-node14-tools.py " + python.version + "\n"
			copyAt, runAt, authoredAt := strings.Index(recipe, copyLine), strings.Index(recipe, runLine), strings.Index(recipe, "\nCOPY . .\n")
			if copyAt < 0 || runAt < copyAt || authoredAt < runAt || strings.Count(recipe, copyLine) != 1 || strings.Count(recipe, runLine) != 1 {
				t.Fatalf("private default missing or out of order:\n%s", recipe)
			}
			if tc.runtime != nativeNodeRuntime {
				npmAt := strings.Index(recipe, " /usr/local/lib/node_modules /usr/local/lib/node_modules\n")
				if npmAt < 0 || npmAt > copyAt {
					t.Fatal("private helper preceded the selected companion npm tree")
				}
			}
			if !strings.Contains(recipe, "COPY <<'BEX_NODE_PYTHON' /usr/libexec/bex-native/prepare-node14-tools.py\n"+nativeNodePythonHelper+"BEX_NODE_PYTHON\n") {
				t.Fatal("historical image did not receive the shared exact helper")
			}
			for _, forbidden := range []string{"ENV PYTHON=", "ENV npm_config_python=", "ENV PYTHONHOME=", "ENV PYTHONPATH=", "ENV LD_LIBRARY_PATH=", "--from=" + donor + " /usr/local/ /usr/local/"} {
				if strings.Contains(recipe, forbidden) {
					t.Fatalf("private interpreter changed app environment/prefix: %s", forbidden)
				}
			}
			actualEnv, err := readEnvBundle(bundle)
			must(t, err)
			if actualEnv["NODE_VERSION"] != tc.node || actualEnv["PYTHON_VERSION"] != tc.python {
				t.Fatal("tool default changed authored runtime selections")
			}
		})
	}
}

func TestHistoricalNodePythonUsesOnlyCompatibleHostAndLegacyLine(t *testing.T) {
	common := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = common })
	nativeCommonBase = ""
	for _, tc := range []struct {
		host, line string
		private    bool
	}{
		{"python:3.13-bookworm@" + exactDigest, "16", true},
		{"node:16-bookworm@" + exactDigest, "16.20.2", true},
		{"python:3.7-bullseye@" + exactDigest, "16", false},
		{"python:3.13-bookworm@" + exactDigest, "24", false},
		{"python:3.13-bookworm@" + exactDigest, "14", true},
	} {
		lines, err := nativeHistoricalNodePythonLines(tc.host, tc.line)
		must(t, err)
		if (lines != "") != tc.private {
			t.Fatalf("private Node Python selection for %s/%s = %t, want %t", tc.host, tc.line, lines != "", tc.private)
		}
		if strings.Contains(lines, "\nENV ") || strings.Contains(lines, "export PYTHON") {
			t.Fatal("private tool default changed the global app environment")
		}
	}
	nativeCommonBase = commonFixtureImage
	lines, err := nativeHistoricalNodePythonLines("python:3.13-bookworm@"+exactDigest, "16")
	must(t, err)
	if lines != "" {
		t.Fatal("common recipe acquired the historical helper a second time")
	}
}

func TestNativeNodePythonHelperIsInProductionDockerContext(t *testing.T) {
	helper, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "deploy", "native-common", "prepare-node14-tools.py"))
	must(t, err)
	if string(helper) != nativeNodePythonHelper {
		t.Fatal("historical and common Node tool helpers drifted")
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", ".dockerignore"))
	must(t, err)
	// These are the glob forms used by this actual context: **, recursive
	// basename globs, directory prefixes and ordinary slash-separated paths.
	included := func(file string) bool {
		selected := true
		for line := range strings.SplitSeq(string(raw), "\n") {
			pattern := strings.TrimSpace(line)
			if pattern == "" || strings.HasPrefix(pattern, "#") {
				continue
			}
			pattern, exception := strings.CutPrefix(pattern, "!")
			matches := pattern == "**"
			if recursive, ok := strings.CutPrefix(pattern, "**/"); ok {
				if strings.Contains(recursive, "/") {
					t.Fatal("review new recursive Docker context pattern")
				}
				matches, err = path.Match(recursive, path.Base(file))
			} else if strings.HasSuffix(pattern, "/") {
				matches = strings.HasPrefix(file, pattern)
			} else if pattern != "**" {
				matches, err = path.Match(pattern, file)
			}
			must(t, err)
			if matches {
				selected = exception
			}
		}
		return selected
	}
	for file, want := range map[string]bool{
		"operator/internal/build/nativenodepython.py":                         true,
		"operator/internal/build/nativenodepython.go":                         true,
		"operator/internal/build/nativenodepython_test.go":                    false,
		"operator/internal/build/testdata/native-node14-addon/check-build.py": false,
		"operator/internal/build/testdata/native-node-qualification/check.py": false,
	} {
		if included(file) != want {
			t.Fatalf("production context includes %s = %t, want %t", file, included(file), want)
		}
	}
	dockerfile, err := os.ReadFile(filepath.Join("..", "..", "..", "Dockerfile"))
	must(t, err)
	copyAt := strings.Index(string(dockerfile), "COPY operator/ ./operator/")
	compileAt := strings.Index(string(dockerfile), "go build -o /workspace/native-resolve")
	if copyAt < 0 || compileAt < copyAt {
		t.Fatal("production resolver compilation precedes its embedded helper context")
	}
}
