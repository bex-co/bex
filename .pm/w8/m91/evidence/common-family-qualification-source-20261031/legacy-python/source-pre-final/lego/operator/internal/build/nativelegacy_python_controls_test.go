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
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestNativeLegacyPythonProfileRequiresSeparateOptIn(t *testing.T) {
	for _, tc := range []struct {
		generic, legacy string
		want            bool
	}{{"", "", false}, {"1", "", false}, {"", "1", false}, {"1", "1", true}} {
		t.Setenv("BEX_BUILDKIT_INTEGRATION", tc.generic)
		t.Setenv("BEX_NATIVE_LEGACY_PYTHON_INTEGRATION", tc.legacy)
		if nativeLegacyPythonProfileEnabled() != tc.want {
			t.Fatalf("generic=%q legacy=%q profile enablement changed", tc.generic, tc.legacy)
		}
	}
}

func TestNativeLegacyWheelRecipesUseActualCanonicalSystemToolSolve(t *testing.T) {
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = commonFixtureImage
	previous := ""
	for _, line := range []string{"3.7", "3.8", "3.13"} {
		t.Run(line, func(t *testing.T) {
			selected, _ := nativeToolchains[nativePythonRuntime].find(line)
			fake := &fakeExact{envByImage: map[string]fakeExactMetadata{}}
			for runtime, chain := range nativeToolchains {
				for _, item := range chain.lines {
					fake.envByImage[item.image] = commonReviewedFixtureMetadata(runtime, item)
				}
			}
			o := nativeOptions()
			o.Runtime, o.BuildCommand = nativePythonRuntime, "unchanged-legacy-authored-command"
			files := nativeLegacyPythonFiles(t, selected.version)
			bundle, path, source := writeResolveFixture(t, o, map[string]string{"PYTHON_VERSION": selected.version}, files)
			choices, err := ResolveNativeBuildExact(t.Context(), fake, time.Time{}, o.Runtime, bundle, path, source)
			must(t, err)
			raw, err := os.ReadFile(path)
			must(t, err)
			prepared, err := nativeLegacyPythonWheelRecipe(string(raw), nativeCommonBase)
			if line == "3.13" {
				if err == nil {
					t.Fatal("modern app-Python wheel recipe accepted as a legacy tool closure")
				}
				return
			}
			must(t, err)
			if choices[0].Image != selected.image || !strings.Contains(string(raw), `ENV PYTHON_VERSION="`+selected.version+`"`) || !strings.Contains(files["pyproject.toml"], `python = "==`+selected.version+`"`) {
				t.Fatal("canonical exact application identity/project constraint changed")
			}
			for _, required := range []string{"FROM " + nativeCommonBase, "--network=default /usr/bin/python3 -m venv", "--only-binary=:all:", legacyPoetrySolve,
				"RUN --network=none /usr/bin/python3 -m venv /opt/bex/poetry", "RUN --network=none /opt/bex/poetry/bin/python -I", " /tool-python.json"} {
				if !strings.Contains(prepared, required) {
					t.Fatalf("legacy closure lost %s", required)
				}
			}
			if strings.Contains(prepared, selected.image) || strings.Contains(prepared, "unchanged-legacy-authored-command") || strings.Contains(prepared, "COPY . .") || strings.Contains(prepared, "/usr/local/bin/python") {
				t.Fatal("legacy tool wheels were prepared with the app interpreter or authored command")
			}
			if previous != "" && prepared != previous {
				t.Fatal("legacy cases no longer share the same actual system-Python prerequisite recipe")
			}
			previous = prepared
			for _, invalid := range []string{strings.ReplaceAll(string(raw), "virtualenv<20.33.0", "virtualenv"), string(raw) + "\nCOPY . .\n", strings.ReplaceAll(string(raw), "poetry==2.1.3", "poetry==1.8.5")} {
				if _, err := nativeLegacyPythonWheelRecipe(invalid, nativeCommonBase); err == nil {
					t.Fatal("another tool solve/source boundary accepted")
				}
			}
		})
	}
}

func TestNativeLegacyWheelMetadataRejectsModernOrWrongArchitecture(t *testing.T) {
	actual := map[string]any{"python": "3.11.15", "base_prefix": "/usr", "prefix": "/opt/bex/poetry", "poetry": "2.1.3", "virtualenv_cap_satisfied": true, "os": "Linux", "machine": "aarch64"}
	data, err := json.Marshal(actual)
	must(t, err)
	must(t, nativeLegacyWheelPlatform(data, "linux/arm64"))
	if nativeLegacyWheelPlatform(data, "linux/amd64") == nil {
		t.Fatal("arm64 tool closure accepted for amd64")
	}
	for key, wrong := range map[string]any{"python": "3.13.18", "base_prefix": "/usr/local", "prefix": "/usr/local", "poetry": "2.2.0", "virtualenv_cap_satisfied": false, "os": "Darwin"} {
		previous := actual[key]
		actual[key] = wrong
		data, err := json.Marshal(actual)
		must(t, err)
		if nativeLegacyWheelPlatform(data, "linux/arm64") == nil {
			t.Fatalf("wrong legacy tool identity accepted: %s", key)
		}
		actual[key] = previous
	}
}

func TestNativeLegacyPythonFixtureOracleControls(t *testing.T) {
	command := exec.CommandContext(t.Context(), "python3", "-B", "-m", "unittest", "discover", "-s", "testdata/native-legacy-python", "-p", "*_test.py", "-v")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("legacy fixture-oracle controls (not runtime qualification): %v\n%s", err, output)
	}
}
