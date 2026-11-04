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
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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
		// Common routes qualify the published Node14/16 trees production
		// adopts: compiling Node from source under AMD64 emulation cannot
		// finish inside the 45-minute build context (w8/m91, 2026-11-04).
		donors, err := commonNodeQualificationArtifacts("1")
		must(t, err)
		nativeCommonBase, nativeRuntimeArtifacts = common, donors
	}
	o, env, files := legacyNodeAddonFixture(t, tc)
	bundle, path, source := writeResolveFixture(t, o, env, files)
	resolver := NewDockerHubResolver()
	choices, err := ResolveNativeBuildExact(t.Context(), resolver, time.Time{}, tc.runtime, bundle, path, source)
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
	prerequisites := false
	if tc.common {
		executed, prerequisites, err = commonNodeQualificationRecipe(executed, node)
		must(t, err)
	}
	if tc.runtime == nativePythonRuntime {
		base := choices[0].Image
		if tc.common {
			base = common
		}
		// The original3.13 closure (including its original Django/pyvips solve)
		// is reused unchanged; the modern family helper intentionally excludes3.13.
		// The wheel solve needs the exact application Python, not its Node16
		// companion's source compiler. Resolve an auxiliary canonical fixture;
		// the original files, Node16 recipe and final45m build stay unchanged.
		wheelFiles := maps.Clone(files)
		delete(wheelFiles, ".node-version")
		wheelBundle, wheelPath, wheelSource := writeResolveFixture(t, o, env, wheelFiles)
		wheelChoices, err := ResolveNativeBuildExact(t.Context(), resolver, time.Time{}, tc.runtime, wheelBundle, wheelPath, wheelSource)
		must(t, err)
		wheelCanonical, err := os.ReadFile(wheelPath)
		must(t, err)
		prepared, err := legacyNodeAddonWheelRecipe(string(wheelCanonical), base, choices, wheelChoices)
		must(t, err)
		must(t, os.WriteFile(filepath.Join(evidence, "Dockerfile.wheels.canonical"), wheelCanonical, 0o600))
		wheelCustody, err := json.MarshalIndent(map[string]any{"common": base, "main_choices": choices, "auxiliary_choices": wheelChoices,
			"primary_all_fields_equal": reflect.DeepEqual(choices[0], wheelChoices[0]), "poetry_all_fields_equal": reflect.DeepEqual(choices[1], wheelChoices[1]),
			"primary_image_env": wheelChoices[0].imageEnv, "primary_image_suite": wheelChoices[0].imageSuite,
			"primary_runtime_artifact": wheelChoices[0].primaryImage, "primary_stage": wheelChoices[0].primaryStage,
			"primary_runtime_check": wheelChoices[0].runtimeCheck, "primary_legacy_source": wheelChoices[0].legacySource,
			"auxiliary_canonical_sha256": fmt.Sprintf("%x", sha256.Sum256(wheelCanonical)), "prerequisite_recipe_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(prepared))),
			"scope": "Python-only wheel acquisition; canonical default Node companion copied without a Node16 compiler; original main Node16 source/addon/runtime checks unchanged"}, "", "  ")
		must(t, err)
		must(t, os.WriteFile(filepath.Join(evidence, "wheel-inputs.json"), wheelCustody, 0o600))
		wheels := nativeUtilityExportWheels(t, docker, prepared, wheelBundle, wheelSource, evidence)
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
	extra := []string{"--secret", "id=render-env,src=" + bundle, "-t", tag}
	args := docker.buildArgs(extra, source, true)
	if prerequisites {
		args = docker.prerequisiteBuildArgs(extra, source, true)
	}
	log, buildErr := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	must(t, os.WriteFile(filepath.Join(evidence, "build.log"), log, 0o600))
	if buildErr != nil {
		t.Fatalf("actual implicit-gyp build: %v\n%s", buildErr, log)
	}
	docker.runImage(t, nativeUtilityCase{name: tc.name}, tag, evidence, tc.runtime == nativePythonRuntime || tc.common)
}

// Preserve the generic wheel helper's full-prefix contract. Only the addon's
// auxiliary input omits the irrelevant legacy companion compiler.
func legacyNodeAddonWheelRecipe(canonical, base string, main, auxiliary []NativeChoice) (string, error) {
	if len(main) < 2 || len(auxiliary) != 3 || main[0].Label != nativeToolchains[nativePythonRuntime].label || main[1].Label != "Poetry" ||
		!reflect.DeepEqual(main[:2], auxiliary[:2]) {
		return "", fmt.Errorf("auxiliary wheel input changed the full application Python or Poetry choice")
	}
	node := auxiliary[2]
	if node.Label != nativeToolchains[nativeNodeRuntime].label || node.Source != "" || node.Image != nativeToolchains[nativeNodeRuntime].defaultImage() ||
		strings.Contains(canonical, "/usr/libexec/bex-native/build-node") || strings.Contains(canonical, " AS bex-node-build\n") {
		return "", fmt.Errorf("auxiliary wheel input must use the canonical default Node without a legacy compiler dependency")
	}
	return nativeUtilityWheelRecipe(canonical, base)
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

func TestLegacyNodeAddonWheelAcquisitionDoesNotCompileCompanion(t *testing.T) {
	original, supplied, artifacts := nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = original, supplied, artifacts })
	for _, tc := range legacyNodeAddonCases() {
		if tc.runtime != nativePythonRuntime {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = "", nil, nil
			base := ""
			if tc.common {
				nativeCommonBase, base = commonFixtureImage, commonFixtureImage
			}
			o, env, files := legacyNodeAddonFixture(t, tc)
			originalFiles := maps.Clone(files)
			fake := commonNodeQualificationFake()
			bundle, path, source := writeResolveFixture(t, o, env, files)
			main, err := ResolveNativeBuildExact(t.Context(), fake, time.Time{}, tc.runtime, bundle, path, source)
			must(t, err)
			canonical, err := os.ReadFile(path)
			must(t, err)
			if base == "" {
				base = main[0].Image
			}
			reviewedNode, found := nativeToolchains[nativeNodeRuntime].find("16")
			if !found {
				t.Fatal("reviewed Node16 line missing")
			}
			sourceCompiler := "/usr/libexec/bex-native/build-node --common-bookworm " + reviewedNode.version + " "
			oldWheels, err := nativeUtilityWheelRecipe(string(canonical), base)
			must(t, err)
			if tc.common && !strings.Contains(oldWheels, sourceCompiler) {
				t.Fatal("old full-prefix export no longer reproduces the observed Node16 compiler dependency")
			}
			wheelFiles := maps.Clone(files)
			delete(wheelFiles, ".node-version")
			wheelBundle, wheelPath, wheelSource := writeResolveFixture(t, o, env, wheelFiles)
			auxiliary, err := ResolveNativeBuildExact(t.Context(), fake, time.Time{}, tc.runtime, wheelBundle, wheelPath, wheelSource)
			must(t, err)
			auxCanonical, err := os.ReadFile(wheelPath)
			must(t, err)
			prepared, err := legacyNodeAddonWheelRecipe(string(auxCanonical), base, main, auxiliary)
			must(t, err)
			if strings.Contains(prepared, "/usr/libexec/bex-native/build-node") || strings.Contains(prepared, main[2].Image) ||
				!strings.Contains(prepared, "pyvips==3.2.0 Django==5.2.18 poetry==2.1.3") || strings.Contains(prepared, "addon-check.py build") {
				t.Fatal("wheel export retained Node16 or changed the original solve/source boundary")
			}
			after, err := os.ReadFile(path)
			must(t, err)
			if string(after) != string(canonical) || !maps.Equal(files, originalFiles) || files[".node-version"] != "16\n" || !reflect.DeepEqual(main[0], auxiliary[0]) {
				t.Fatal("auxiliary preparation mutated the main Node16 recipe/files or selected app Python")
			}
			if tc.common {
				_, selectedNode, err := legacyNodeAddonExpected(tc, main, "linux/arm64")
				must(t, err)
				executed, overlay, err := commonNodeQualificationRecipe(string(after), selectedNode)
				must(t, err)
				if !overlay || !strings.Contains(executed, sourceCompiler) {
					t.Fatal("main generated build lost its mandatory Node16 source compiler")
				}
			}
			for name, change := range map[string]func(*NativeChoice){
				"image":         func(c *NativeChoice) { c.Image += "wrong" },
				"version":       func(c *NativeChoice) { c.Line = "3.14" },
				"env":           func(c *NativeChoice) { c.imageEnv = append([]string{"PYTHON_VERSION=wrong"}, c.imageEnv...) },
				"artifact":      func(c *NativeChoice) { c.primaryImage = "wrong" },
				"stage":         func(c *NativeChoice) { c.primaryStage = "wrong" },
				"runtime-check": func(c *NativeChoice) { c.runtimeCheck = "wrong" },
				"suite":         func(c *NativeChoice) { c.imageSuite = "wrong" },
				"legacy":        func(c *NativeChoice) { c.legacySource = !c.legacySource },
			} {
				t.Run(name, func(t *testing.T) {
					wrong := append([]NativeChoice(nil), auxiliary...)
					change(&wrong[0])
					if _, err := legacyNodeAddonWheelRecipe(string(auxCanonical), base, main, wrong); err == nil {
						t.Fatal("changed full primary identity accepted for wheel acquisition")
					}
				})
			}
			wrongPoetry := append([]NativeChoice(nil), auxiliary...)
			wrongPoetry[1].Line = "2.2.0"
			if _, err := legacyNodeAddonWheelRecipe(string(auxCanonical), base, main, wrongPoetry); err == nil {
				t.Fatal("different Poetry closure accepted")
			}
			if _, err := legacyNodeAddonWheelRecipe(string(canonical), base, main, main); err == nil {
				t.Fatal("original selected Node16 compiler input still accepted for wheel acquisition")
			}
		})
	}
}
