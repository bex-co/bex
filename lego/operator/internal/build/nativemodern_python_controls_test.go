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
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNativeModernPythonMatrixAndUnchangedApplicationFixture(t *testing.T) {
	if !slices.Equal(nativeModernPythonLines, []string{"3.9", "3.10", "3.11", "3.12", "3.14"}) {
		t.Fatal("modern profile must cover the five remaining reviewed Python lines")
	}
	for name, version := range map[string]string{"3.9": "3.9.25", "3.10": "3.10.22", "3.11": "3.11.18", "3.12": "3.12.16", "3.14": "3.14.10"} {
		line, found := nativeToolchains[nativePythonRuntime].find(name)
		if !found || line.version != version || !nativeModernPythonVersion(version) {
			t.Fatalf("missing reviewed %s %s", name, version)
		}
		legacy, modern := nativeLegacyPythonFiles(t, version), nativeModernPythonFiles(t, version)
		for _, file := range []string{"check.py", "setup.py", "legacy_probe.c", "pyproject.toml"} {
			if legacy[file] != modern[file] {
				t.Fatalf("shared selected-header/loader/project fixture changed: %s", file)
			}
		}
		if !strings.Contains(modern["check-tool.py"], `EXPECTED_PYTHON = "`+version+`"`) || modern[".node-version"] != "24\n" {
			t.Fatal("modern exact tool identity/Node companion fixture lost")
		}
	}
	for _, excluded := range []string{"3.7.17", "3.8.20", "3.13.18", "3.14.8"} {
		if nativeModernPythonVersion(excluded) {
			t.Fatalf("legacy/original/default cohort joined modern matrix: %s", excluded)
		}
	}
}

func TestNativeModernPythonWheelRecipesPreserveActualSelectedPrefix(t *testing.T) {
	original := nativeCommonBase
	t.Cleanup(func() { nativeCommonBase = original })
	nativeCommonBase = commonFixtureImage
	for _, name := range append(slices.Clone(nativeModernPythonLines), "3.7", "3.8") {
		t.Run(name, func(t *testing.T) {
			selected, _ := nativeToolchains[nativePythonRuntime].find(name)
			fake := &fakeExact{envByImage: map[string]fakeExactMetadata{}}
			for runtime, chain := range nativeToolchains {
				for _, line := range chain.lines {
					fake.envByImage[line.image] = commonReviewedFixtureMetadata(runtime, line)
				}
			}
			o := nativeOptions()
			o.Runtime, o.BuildCommand = nativePythonRuntime, "unchanged-modern-authored-command"
			files := nativeModernPythonFiles(t, selected.version)
			bundle, path, source := writeResolveFixture(t, o, map[string]string{"PYTHON_VERSION": selected.version}, files)
			choices, err := ResolveNativeBuildExact(t.Context(), fake, time.Time{}, o.Runtime, bundle, path, source)
			must(t, err)
			canonical, err := os.ReadFile(path)
			must(t, err)
			prepared, err := nativeModernPythonWheelRecipe(string(canonical), nativeCommonBase, selected.version)
			if name == "3.7" || name == "3.8" {
				if err == nil {
					t.Fatal("legacy system-Python default accepted as a selected-modern tool closure")
				}
				return
			}
			must(t, err)
			env, err := readEnvBundle(bundle)
			must(t, err)
			if len(choices) != 3 || choices[0].Image != selected.image || choices[1].Line != "2.1.3" || choices[1].Source != "" || env["PYTHON_VERSION"] != selected.version ||
				!strings.Contains(files["pyproject.toml"], `python = "==`+selected.version+`"`) {
				t.Fatal("canonical app/default Poetry/exact authored version contract changed")
			}
			assertModernPythonWheelPrefix(t, prepared, selected)
			for label, invalid := range map[string]string{
				"system bootstrap": strings.ReplaceAll(string(canonical), "RUN /usr/local/bin/python -m venv", "RUN /usr/bin/python3 -m venv"),
				"legacy cap":       strings.ReplaceAll(string(canonical), "poetry==2.1.3 &&", "poetry==2.1.3 'virtualenv<20.33.0' &&"),
				"another tool":     strings.ReplaceAll(string(canonical), "poetry==2.1.3", "poetry==1.8.5"),
				"wrong version":    strings.ReplaceAll(string(canonical), "ENV PYTHON_VERSION=\""+selected.version+"\"", "ENV PYTHON_VERSION=\"0.0.0\""),
				"second source":    string(canonical) + "\nCOPY . .\n",
				"second bootstrap": strings.Replace(string(canonical), "\nCOPY . .\n", "\nRUN /usr/local/bin/python -m venv /opt/bex/poetry && duplicate\nCOPY . .\n", 1),
			} {
				if _, err := nativeModernPythonWheelRecipe(invalid, nativeCommonBase, selected.version); err == nil {
					t.Fatalf("incorrect modern prerequisite accepted: %s", label)
				}
			}
			if _, err := nativeModernPythonWheelRecipe(string(canonical), nativeCommonBase, "3.14.8"); err == nil {
				t.Fatal("table patch replaced by distinct creation default")
			}
		})
	}
}

func assertModernPythonWheelPrefix(t *testing.T, prepared string, selected nativeLine) {
	t.Helper()
	previous := -1
	for _, marker := range []string{
		"FROM " + nativeCommonBase + " AS bex-test-prerequisites\n",
		"COPY --link --from=" + selected.image + " /usr/local/ /usr/local/\n",
		"RUN --network=none /sbin/ldconfig\n", "ENV PYTHON_VERSION=\"" + selected.version + "\"\n",
		"RUN --network=none /usr/bin/bex-native-python-tools\n",
		"RUN --network=default /usr/local/bin/python -m pip wheel", "--wheel-dir /opt/bex-test-wheels " + modernPoetrySolve,
		"ENV PIP_NO_INDEX=1 PIP_FIND_LINKS=/opt/bex-test-wheels\n",
		"RUN --network=none /usr/local/bin/python -m venv /opt/bex/poetry && ",
		"--no-input " + modernPoetrySolve + " && ", "COPY check-tool.py /opt/bex-test-check-tool.py\n",
		"RUN --network=none /opt/bex/poetry/bin/python -I /opt/bex-test-check-tool.py", "FROM scratch\n",
	} {
		at := strings.Index(prepared, marker)
		if at < 0 || at <= previous {
			t.Fatalf("selected runtime/loader/ENV/hook/wheels/private bootstrap order lost %s:\n%s", marker, prepared)
		}
		previous = at
	}
	if strings.Count(prepared, "RUN --network=default ") != 1 {
		t.Fatal("modern prerequisite changed another network boundary")
	}
	for _, excluded := range []string{"Django", "pyvips", "virtualenv<", "--only-binary", "/usr/bin/python3 -m venv", "unchanged-modern-authored-command", "COPY . ."} {
		if strings.Contains(prepared, excluded) {
			t.Fatalf("modern prerequisite widened its closure/command scope: %s", excluded)
		}
	}
}

func TestNativeModernWheelMetadataRejectsWrongRuntimeOrArchitecture(t *testing.T) {
	actual := map[string]any{"python": "3.9.25", "python_env": "3.9.25", "base_prefix": "/usr/local", "prefix": "/opt/bex/poetry",
		"executable": "/opt/bex/poetry/bin/python", "poetry": "2.1.3", "virtualenv": "20.35.0", "dependency_check_succeeded": true, "os": "Linux", "machine": "aarch64"}
	data, err := json.Marshal(actual)
	must(t, err)
	must(t, nativeModernWheelPlatform(data, "linux/arm64", "3.9.25"))
	for _, target := range []string{"linux/amd64", "darwin/arm64", ""} {
		if nativeModernWheelPlatform(data, target, "3.9.25") == nil {
			t.Fatalf("wrong architecture/platform accepted: %s", target)
		}
	}
	for key, wrong := range map[string]any{"python": "3.11.15", "python_env": "3.9", "base_prefix": "/usr", "prefix": "/usr/local",
		"executable": "/usr/local/bin/python", "poetry": "2.2.0", "virtualenv": "", "dependency_check_succeeded": false, "os": "Darwin"} {
		previous := actual[key]
		actual[key] = wrong
		data, err := json.Marshal(actual)
		must(t, err)
		if nativeModernWheelPlatform(data, "linux/arm64", "3.9.25") == nil {
			t.Fatalf("wrong selected modern tool identity accepted: %s", key)
		}
		actual[key] = previous
	}
	data, err = json.Marshal(actual)
	must(t, err)
	if nativeModernWheelPlatform(data, "linux/arm64", "3.10.22") == nil {
		t.Fatal("one selected Python's wheel closure accepted for another Python")
	}
}

func TestNativeModernPythonFixtureOracleControls(t *testing.T) {
	command := exec.CommandContext(t.Context(), "python3", "-B", "-m", "unittest", "discover", "-s", "testdata/native-modern-python", "-p", "*_test.py", "-v")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("modern tool-oracle controls (not runtime qualification): %v\n%s", err, output)
	}
}

func TestNativeModernPythonProfileSkipsGenericIntegration(t *testing.T) {
	binary, err := os.Executable()
	must(t, err)
	for _, tc := range []struct{ name, profile, integration, want string }{
		{"generic integration skips", "", "1", "--- SKIP: TestNativeModernPythonGeneratedImages"},
		{"profile needs integration opt-in", "1", "", "modern Python profile also requires BEX_BUILDKIT_INTEGRATION=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary, "-test.run=^TestNativeModernPythonGeneratedImages$", "-test.v")
			for _, env := range os.Environ() {
				if !strings.HasPrefix(env, "BEX_NATIVE_TEST_MODERN_PYTHON_PROFILE=") && !strings.HasPrefix(env, "BEX_BUILDKIT_INTEGRATION=") {
					command.Env = append(command.Env, env)
				}
			}
			command.Env = append(command.Env, "BEX_NATIVE_TEST_MODERN_PYTHON_PROFILE="+tc.profile, "BEX_BUILDKIT_INTEGRATION="+tc.integration)
			output, err := command.CombinedOutput()
			if (err == nil) != (tc.profile == "") || !strings.Contains(string(output), tc.want) {
				t.Fatalf("modern profile guard result %v: %s", err, output)
			}
		})
	}
}
