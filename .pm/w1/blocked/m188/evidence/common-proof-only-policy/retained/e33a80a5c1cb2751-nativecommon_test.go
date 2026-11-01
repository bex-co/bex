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
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// Synthetic identity is restricted to source-only tests; no production pin.
const commonFixtureImage = "ghcr.io/bex-co/bex-cnb-run:common-fixture@" + otherDigest

func TestCommonFoundationPreservesDefaultAndExactSelections(t *testing.T) {
	original, originalSupplied := nativeCommonBase, nativeSuppliedBases
	t.Cleanup(func() { nativeCommonBase, nativeSuppliedBases = original, originalSupplied })
	nativeCommonBase = commonFixtureImage
	// The common route cannot silently adopt an unrelated C derivative.
	nativeSuppliedBases = map[string]string{"unrelated": "invalid-C-candidate"}
	for _, tc := range []struct {
		name, runtime, line, version, tag string
		env                               map[string]string
		created                           time.Time
		missing                           bool
	}{
		{name: "node-default", runtime: nativeNodeRuntime, line: "24", version: "24.22.1"},
		{name: "python-explicit-line", runtime: nativePythonRuntime, line: "3.13", version: "3.13.18", env: map[string]string{"PYTHON_VERSION": "3.13"}},
		{name: "python-creation-default", runtime: nativePythonRuntime, line: "3.14.8", version: "3.14.8", tag: "python:3.14.8-bookworm", created: pythonCreationCutoff.Add(time.Second)},
		{name: "node-exact", runtime: nativeNodeRuntime, line: "24.10.0", version: "24.10.0", tag: "node:24.10.0-bookworm", env: map[string]string{"NODE_VERSION": "24.10.0", "BUN_VERSION": "1.3"}},
		{name: "python-exact", runtime: nativePythonRuntime, line: "3.13.1", version: "3.13.1", tag: "python:3.13.1-bookworm", env: map[string]string{"PYTHON_VERSION": "3.13.1"}},
		{name: "existing-exact-fallback", runtime: nativeNodeRuntime, line: "24", version: "24.22.1", tag: "node:24.10.0-bookworm", env: map[string]string{"NODE_VERSION": "24.10.0"}, missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := nativeOptions()
			o.Runtime = tc.runtime
			bundle, path, source := writeResolveFixture(t, o, tc.env, nil)
			fake := &fakeExact{digests: map[string]string{}}
			if tc.tag != "" && !tc.missing {
				fake.digests[tc.tag] = exactDigest
			}
			versionKey := "NODE_VERSION"
			if tc.runtime == nativePythonRuntime {
				versionKey = "PYTHON_VERSION"
			}
			fake.env = []string{versionKey + "=" + tc.version, "PATH=/usr/local/bin:/usr/bin", "LANG=C.UTF-8", `LITERAL=$HOME\path"quoted`}
			choices, err := ResolveNativeBuildExact(context.Background(), fake, tc.created, tc.runtime, bundle, path, source)
			must(t, err)
			data, err := os.ReadFile(path)
			must(t, err)
			recipe := string(data)
			reviewed, err := ResolveNativeVersionAt(tc.created, tc.runtime, tc.env, files(nil))
			must(t, err)
			primary := reviewed.Image
			if tc.tag != "" && !tc.missing {
				primary = tc.tag + "@" + exactDigest
			}
			if choices[0].Image != primary || choices[0].Line != tc.line {
				t.Fatalf("common base changed selected primary: %+v", choices[0])
			}
			if !strings.Contains(recipe, "\nFROM "+commonFixtureImage+"\n") {
				t.Fatalf("not using the one common FROM:\n%s", recipe)
			}
			assertCommonPrimaryOrder(t, recipe, primary, versionKey, tc.version, tc.runtime == nativePythonRuntime)
			if len(fake.asked) == 0 || fake.asked[len(fake.asked)-1] != "env "+primary {
				t.Fatalf("default/exact source metadata not read: %v", fake.asked)
			}
			if tc.missing && !strings.Contains(choices[0].Narration(), "No official image "+tc.tag) {
				t.Fatalf("existing fallback explanation lost: %s", choices[0].Narration())
			}
			if strings.Contains(recipe, "apt-get") {
				t.Fatal("common tools are being installed during a tenant build")
			}
			for _, command := range []string{o.BuildCommand, o.StartCommand} {
				if !strings.Contains(recipe, command) {
					t.Fatalf("authored command changed: %s", command)
				}
			}
		})
	}
}

func assertCommonPrimaryOrder(t *testing.T, recipe, primary, key, version string, python bool) {
	t.Helper()
	markers := []string{"rm -rf /usr/local", "COPY --link --from=" + primary + " /usr/local/ /usr/local/\n", "RUN --network=none /sbin/ldconfig\n", "ENV " + key + "=\"" + version + "\"\n"}
	if python {
		markers = append(markers, "RUN --network=none /usr/bin/bex-native-python-tools\n")
	}
	previous := -1
	for _, marker := range markers {
		at := strings.Index(recipe, marker)
		if at < 0 || at <= previous {
			t.Fatalf("primary/loader/environment/packaging order lost %q:\n%s", marker, recipe)
		}
		previous = at
	}
	for _, marker := range []string{" /usr/local/bin/bun /usr/local/bin/bun", "/opt/bex/poetry", " /usr/local/bin/node /usr/local/bin/node", "\nCOPY . .\n"} {
		if at := strings.Index(recipe, marker); at >= 0 && at < previous {
			t.Fatalf("primary replacement erased a later layer %q:\n%s", marker, recipe)
		}
	}
	for _, marker := range []string{`primary_os="$VERSION_CODENAME"`, `ENV LITERAL="\$HOME\\path\"quoted"`, `ENV PATH="/usr/local/bin:/usr/bin"`, `ENV LANG="C.UTF-8"`} {
		if !strings.Contains(recipe, marker) {
			t.Fatalf("suite guard or exact ENV escaping lost %q:\n%s", marker, recipe)
		}
	}
}

func TestCommonMetadataFailureNeverSubstitutesAnotherRuntime(t *testing.T) {
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = commonFixtureImage
	o := nativeOptions()
	o.Runtime = nativePythonRuntime
	bundle, path, source := writeResolveFixture(t, o, nil, nil)
	before, err := os.ReadFile(path)
	must(t, err)
	fake := &fakeExact{digests: map[string]string{"python:3.14.8-bookworm": exactDigest}, envErr: errors.New("metadata transport unavailable")}
	_, err = ResolveNativeBuildExact(context.Background(), fake, pythonCreationCutoff.Add(time.Second), o.Runtime, bundle, path, source)
	if err == nil || !strings.Contains(err.Error(), "read selected native primary environment") {
		t.Fatalf("metadata failure was hidden: %v", err)
	}
	after, readErr := os.ReadFile(path)
	must(t, readErr)
	if string(after) != string(before) {
		t.Fatal("failed metadata resolution wrote a downgraded or partial recipe")
	}
}
