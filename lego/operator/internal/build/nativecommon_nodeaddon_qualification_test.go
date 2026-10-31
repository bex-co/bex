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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type legacyNodeAddonCase struct {
	name, runtime, node string
	common              bool
}

func legacyNodeAddonCases() []legacyNodeAddonCase {
	return []legacyNodeAddonCase{
		{name: "common-node14-primary", runtime: nativeNodeRuntime, node: "14", common: true},
		{name: "common-node16-primary", runtime: nativeNodeRuntime, node: "16", common: true},
		{name: "common-python313-node16", runtime: nativePythonRuntime, node: "16", common: true},
		{name: "historical-python313-node16", runtime: nativePythonRuntime, node: "16"},
	}
}

func legacyNodeAddonFixture(t *testing.T, tc legacyNodeAddonCase) (Options, map[string]string, map[string]string) {
	t.Helper()
	files := map[string]string{".node-version": tc.node + "\n"}
	for target, source := range map[string]string{
		"addon-check.py":         "native-node-addon-qualification/check.py",
		"addon-check.js":         "native-node-addon-qualification/check.js",
		"binding.gyp":            "native-node14-addon/binding.gyp",
		"package-template.json":  "native-node14-addon/package.json",
		"addon.c":                "native-node-qualification/addon.c",
		"node-family-oracles.py": "native-node-qualification/check.py",
	} {
		data, err := os.ReadFile(filepath.Join("testdata", source))
		must(t, err)
		files[target] = string(data)
	}
	o := nativeOptions()
	o.Runtime, o.BuildEnv, o.RuntimeEnvSecret = tc.runtime, nil, ""
	interpreter := "/usr/bin/python3"
	var env map[string]string
	if tc.runtime == nativePythonRuntime {
		python, found := nativeToolchains[nativePythonRuntime].find("3.13")
		if !found || !fullVersion.MatchString(python.version) {
			t.Fatal("addon companion requires the reviewed full Python3.13 patch")
		}
		env = map[string]string{"PYTHON_VERSION": python.version}
		interpreter = "/usr/local/bin/python3"
	}
	o.BuildCommand, o.StartCommand = interpreter+" addon-check.py build", interpreter+" addon-check.py runtime"
	return o, env, files
}

func legacyNodeAddonExpected(tc legacyNodeAddonCase, choices []NativeChoice, platform string) ([]byte, NativeChoice, error) {
	line, found := nativeToolchains[nativeNodeRuntime].find(tc.node)
	if !found || !nativeLegacyPnpmLine(tc.node) || !fullVersion.MatchString(line.version) || len(choices) == 0 {
		return nil, NativeChoice{}, fmt.Errorf("addon fixture requires a reviewed Node14/16 selection")
	}
	var node NativeChoice
	count := 0
	for _, choice := range choices {
		if choice.Label == nativeToolchains[nativeNodeRuntime].label {
			node, count = choice, count+1
		}
	}
	if count != 1 || node.Line != tc.node || node.Image != line.image || node.declined != "" {
		return nil, NativeChoice{}, fmt.Errorf("addon fixture did not select exactly its reviewed Node donor")
	}
	if len(node.imageEnv) != 0 {
		version, err := commonImageVariable(node.imageEnv, "NODE_VERSION")
		if err != nil || version != line.version {
			return nil, NativeChoice{}, fmt.Errorf("selected Node metadata changed its reviewed patch")
		}
	}
	var nodeEnv, appPython *string
	if tc.runtime == nativeNodeRuntime {
		nodeEnv = &line.version
	} else {
		python, ok := nativeToolchains[nativePythonRuntime].find("3.13")
		if !ok || choices[0].Label != nativeToolchains[nativePythonRuntime].label || choices[0].Line != python.line || choices[0].Image != python.image {
			return nil, NativeChoice{}, fmt.Errorf("addon companion changed the application Python donor")
		}
		appPython = &python.version
	}
	if platform != "linux/amd64" && platform != "linux/arm64" {
		return nil, NativeChoice{}, fmt.Errorf("addon fixture requires an explicit target platform")
	}
	private, _, err := commonNodeBuildPython()
	if err != nil {
		return nil, NativeChoice{}, err
	}
	raw, err := json.MarshalIndent(map[string]any{"case": tc.name, "common": tc.common,
		"node_contract": map[string]any{"node": line.version, "node_env": nodeEnv}, "app_python": appPython,
		"private_python": private.version, "arch": map[string]string{"linux/amd64": "x64", "linux/arm64": "arm64"}[platform], "yarn": strings.TrimPrefix(nativeYarnDir, "/opt/yarn-v"), "pnpm": nativePnpm.version,
		"scope": "implicit npm/Yarn/pnpm gypfile installs and explicit Python controls; no hosted/adoption claim"}, "", "  ")
	return raw, node, err
}

// This additional profile is never enabled by generic integration, the first
// cohort, or the existing nine positive Node projects. No runtime TREE is needed.
func TestLegacyNodeOfflineAddons(t *testing.T) {
	if os.Getenv("BEX_NATIVE_TEST_NODE_ADDON_PROFILE") != "1" {
		t.Skip("set BEX_NATIVE_TEST_NODE_ADDON_PROFILE=1 for the separate implicit-gyp profile")
	}
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") != "1" {
		t.Fatal("addon profile also requires BEX_BUILDKIT_INTEGRATION=1")
	}
	common, _, _ := nativeUtilityTestImages(t)
	docker := nativeUtilityDockerConfig(t)
	if common == "" || docker.builder == "" {
		t.Fatal("addon profile requires an immutable common and explicit owned builder/platform")
	}
	original, supplied, artifacts := nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = original, supplied, artifacts })
	output := t.TempDir()
	if requested := os.Getenv("BEX_NATIVE_TEST_OUTPUT"); requested != "" {
		must(t, os.MkdirAll(requested, 0o700))
		var err error
		output, err = os.MkdirTemp(requested, "native-node-addon-")
		must(t, err)
	}
	t.Logf("retained implicit-gyp evidence: %s", output)
	for _, tc := range legacyNodeAddonCases() {
		if !t.Run(tc.name, func(t *testing.T) { legacyNodeAddonGeneratedCase(t, docker, common, output, tc) }) {
			break
		}
	}
}

func legacyNodeAddonGeneratedCase(t *testing.T, docker nativeUtilityDocker, common, output string, tc legacyNodeAddonCase) {
	t.Helper()
	nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = "", nil, nil
	if tc.common {
		nativeCommonBase = common
	}
	o, env, files := legacyNodeAddonFixture(t, tc)
	bundle, path, source := writeResolveFixture(t, o, env, files)
	choices, err := ResolveNativeBuildAt(time.Time{}, tc.runtime, bundle, path, source)
	must(t, err)
	expected, node, err := legacyNodeAddonExpected(tc, choices, docker.platform)
	must(t, err)
	must(t, os.WriteFile(filepath.Join(source, "expected-addon.json"), expected, 0o600))
	evidence := filepath.Join(output, tc.name)
	must(t, os.Mkdir(evidence, 0o700))
	canonical, err := os.ReadFile(path)
	must(t, err)
	must(t, os.WriteFile(filepath.Join(evidence, "Dockerfile.canonical"), canonical, 0o600))
	must(t, os.WriteFile(filepath.Join(evidence, "expected-addon.json"), expected, 0o600))
	executed := string(canonical)
	if tc.common {
		executed, _, err = commonNodeQualificationRecipe(executed, node)
		must(t, err)
	}
	if tc.runtime == nativePythonRuntime {
		base := choices[0].Image
		if tc.common {
			base = common
		}
		// The original3.13 closure (including its original Django/pyvips solve)
		// is reused unchanged; the modern family helper intentionally excludes3.13.
		wheels := nativeUtilityPrepareWheels(t, docker, []byte(executed), base, bundle, source, evidence)
		copyNativeUtilityWheels(t, wheels, source, evidence)
		executed, err = afterNativeBase(executed, base, "COPY .bex-test-wheels/ /opt/bex-test-wheels/\nENV PIP_NO_INDEX=1 PIP_FIND_LINKS=/opt/bex-test-wheels\n")
		must(t, err)
	}
	must(t, os.WriteFile(filepath.Join(source, "Dockerfile"), []byte(executed), 0o600))
	must(t, os.WriteFile(filepath.Join(evidence, "Dockerfile"), []byte(executed), 0o600))
	hashes := map[string]string{}
	for name, data := range files {
		hashes[name] = fmt.Sprintf("%x", sha256.Sum256([]byte(data)))
	}
	receipt, err := json.MarshalIndent(map[string]any{"common_input": common, "uses_common": tc.common, "choices": choices,
		"owner": docker.owner, "builder": docker.builder, "platform": docker.platform, "fixture_sha256": hashes,
		"canonical_sha256": fmt.Sprintf("%x", sha256.Sum256(canonical)), "executed_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(executed))),
		"scope": "one explicit addon route, including original historical companion; authored build/runtime offline; no hosted claim"}, "", "  ")
	must(t, err)
	must(t, os.WriteFile(filepath.Join(evidence, "source-custody.json"), receipt, 0o600))
	tag := fmt.Sprintf("bex-node-addon:%s-%d-%d", tc.name, os.Getpid(), time.Now().UnixNano())
	t.Cleanup(func() { nativeUtilityCleanup("image", "rm", "-f", tag) })
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Minute)
	defer cancel()
	log, buildErr := exec.CommandContext(ctx, "docker", docker.buildArgs([]string{"--secret", "id=render-env,src=" + bundle, "-t", tag}, source, true)...).CombinedOutput()
	must(t, os.WriteFile(filepath.Join(evidence, "build.log"), log, 0o600))
	if buildErr != nil {
		t.Fatalf("actual implicit-gyp build: %v\n%s", buildErr, log)
	}
	docker.runImage(t, nativeUtilityCase{name: tc.name}, tag, evidence, tc.runtime == nativePythonRuntime || tc.common)
}

func TestLegacyNodeAddonCanonicalRoutes(t *testing.T) {
	original, supplied, artifacts := nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = original, supplied, artifacts })
	if len(legacyNodeAddonCases()) != 4 {
		t.Fatal("all three common routes and original historical companion are required")
	}
	for _, tc := range legacyNodeAddonCases() {
		t.Run(tc.name, func(t *testing.T) {
			nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = "", nil, nil
			if tc.common {
				nativeCommonBase = commonFixtureImage
			}
			o, env, files := legacyNodeAddonFixture(t, tc)
			bundle, path, source := writeResolveFixture(t, o, env, files)
			choices, err := ResolveNativeBuildExact(t.Context(), commonNodeQualificationFake(), time.Time{}, tc.runtime, bundle, path, source)
			must(t, err)
			raw, node, err := legacyNodeAddonExpected(tc, choices, "linux/arm64")
			must(t, err)
			var expected map[string]any
			must(t, json.Unmarshal(raw, &expected))
			if tc.runtime == nativePythonRuntime {
				python, _ := nativeToolchains[nativePythonRuntime].find("3.13")
				if expected["app_python"] != python.version || env["PYTHON_VERSION"] != python.version {
					t.Fatal("Python expectation did not follow the reviewed current full patch")
				}
			}
			canonical, err := os.ReadFile(path)
			must(t, err)
			text := string(canonical)
			privateAt, helperAt, authoredAt := strings.Index(text, " /opt/bex/native-node-python/\n"), strings.Index(text, " -I /usr/libexec/bex-native/prepare-node14-tools.py "), strings.Index(text, "\nCOPY . .\n")
			if privateAt < 0 || helperAt < privateAt || authoredAt < helperAt {
				t.Fatal("private Python closure/helper must precede authored source")
			}
			if tc.runtime == nativePythonRuntime {
				npmAt := strings.Index(text, " /usr/local/lib/node_modules /usr/local/lib/node_modules\n")
				if npmAt < 0 || npmAt > privateAt {
					t.Fatal("helper must follow the companion's selected npm tree")
				}
			}
			executed := text
			if tc.common {
				executed, _, err = commonNodeQualificationRecipe(text, node)
				must(t, err)
				if strings.Count(executed, "RUN --network=default ") != 1 {
					t.Fatal("only the exact Node source prerequisite may acquire network")
				}
			} else if strings.Contains(text, "--common-bookworm") {
				t.Fatal("historical companion unexpectedly uses common source assembly")
			}
			if tc.runtime == nativePythonRuntime {
				base := choices[0].Image
				if tc.common {
					base = commonFixtureImage
				}
				prepared, err := nativeUtilityWheelRecipe(executed, base)
				must(t, err)
				if strings.Contains(prepared, "addon-check.py build") || strings.Index(prepared, "pip wheel") > strings.Index(prepared, " -m venv /opt/bex/poetry") {
					t.Fatal("wheel preparation ran authored source or followed Poetry bootstrap")
				}
			}
		})
	}
}
