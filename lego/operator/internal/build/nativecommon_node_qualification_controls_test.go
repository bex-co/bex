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
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func commonNodeQualificationFake() *fakeExact {
	fake := &fakeExact{digests: map[string]string{"node:14.21.2-bullseye": exactDigest}, envByImage: map[string]fakeExactMetadata{}}
	for runtime, chain := range nativeToolchains {
		for _, line := range chain.lines {
			fake.envByImage[line.image] = commonReviewedFixtureMetadata(runtime, line)
		}
	}
	fake.envByImage["node:14.21.2-bullseye@"+exactDigest] = fakeExactMetadata{
		env: []string{"NODE_VERSION=14.21.2", "PATH=/usr/local/bin:/usr/bin"},
	}
	return fake
}

func TestCommonNodeQualificationCanonicalCells(t *testing.T) {
	original, supplied, artifacts := nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = original, supplied, artifacts })
	nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = commonFixtureImage, nil, nil
	cases := commonNodeQualificationCases()
	if len(cases) != 8 {
		t.Fatal("six reviewed cells and two exact representatives are required")
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := nativeOptions()
			o.Runtime, o.BuildEnv, o.RuntimeEnvSecret = tc.runtime, nil, ""
			o.BuildCommand, o.StartCommand = "/usr/bin/python3 check.py build", "/usr/bin/python3 check.py runtime"
			bundle, path, source := writeResolveFixture(t, o, nil, commonNodeQualificationFiles(t, tc))
			fake := commonNodeQualificationFake()
			choices, err := ResolveNativeBuildExact(context.Background(), fake, time.Time{}, tc.runtime, bundle, path, source)
			must(t, err)
			expected, node, err := commonNodeQualificationExpectations(tc, choices)
			must(t, err)
			var contract map[string]any
			must(t, json.Unmarshal(expected, &contract))
			if (contract["node_env"] != nil) != (tc.runtime == nativeNodeRuntime) {
				t.Fatal("companion incorrectly imported donor NODE_VERSION")
			}
			nodeVersion := contract["node"].(string)
			canonical, err := os.ReadFile(path)
			must(t, err)
			executed, overlay, err := commonNodeQualificationRecipe(string(canonical), node)
			must(t, err)
			needsSource := commonNodeBackport(nodeVersion)
			if overlay != needsSource || strings.Count(executed, "RUN --network=default ") != map[bool]int{true: 1, false: 0}[needsSource] {
				t.Fatal("source fallback network boundary changed")
			}
			reversed := strings.Replace(executed, "\nRUN --network=default /sbin/ldconfig", "\nRUN /sbin/ldconfig", 1)
			if reversed != string(canonical) {
				t.Fatal("test overlay changed canonical payload, authored command or runtime")
			}
			for _, required := range []string{"\nFROM " + commonFixtureImage + "\n", o.BuildCommand, o.StartCommand, nativePnpmInstall()} {
				if !strings.Contains(string(canonical), required) {
					t.Fatalf("canonical profile lost %q", required)
				}
			}
			if needsSource && !strings.Contains(node.primaryStage, "--common-bookworm "+nodeVersion+" "+strings.Repeat("b", 64)) {
				t.Fatal("empty artifact map did not select checksum-bound same-patch source fallback")
			}
			if tc.runtime == nativeRubyRuntime && !strings.Contains(string(canonical), "/usr/local/include/node /usr/local/include/node\n") {
				t.Fatal("exact Node companion lost its matching headers")
			}
			if strings.Contains(executed, "pnpm/exe") || strings.Contains(executed, "npm_config_node_gyp=") {
				t.Fatal("unresolved legacy pnpm was masked by a test-only workaround")
			}
		})
	}
}

func TestCommonNodeQualificationRejectsChangedPrerequisite(t *testing.T) {
	node := NativeChoice{imageEnv: []string{"NODE_VERSION=14.21.3"}}
	stage, err := commonNodeBuildStage("node:14-bullseye@"+exactDigest, "14.21.3", strings.Repeat("b", 64))
	must(t, err)
	node.primaryStage = stage
	canonical := "# syntax=docker/dockerfile:1.7\n" + stage + "FROM " + commonFixtureImage + "\nCOPY . .\nRUN unchanged-authored\n"
	_, _, err = commonNodeQualificationRecipe(canonical, node)
	must(t, err)
	for name, changed := range map[string]string{
		"missing":                 strings.ReplaceAll(canonical, "build-node --common-bookworm", "different-builder --common-bookworm"),
		"duplicate":               canonical + stage,
		"wrong-patch":             strings.ReplaceAll(canonical, "--common-bookworm 14.21.3 ", "--common-bookworm 14.21.2 "),
		"missing-source-boundary": strings.ReplaceAll(canonical, "\nCOPY . .\n", "\n"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := commonNodeQualificationRecipe(changed, node); err == nil {
				t.Fatal("changed canonical prerequisite accepted")
			}
		})
	}
	node.primaryStage = strings.ReplaceAll(stage, strings.Repeat("b", 64), "untrusted")
	if _, _, err := commonNodeQualificationRecipe(canonical, node); err == nil {
		t.Fatal("non-checksum source input accepted")
	}
	node.imageEnv = []string{"NODE_VERSION=22.23.4"}
	if _, _, err := commonNodeQualificationRecipe(canonical, node); err == nil {
		t.Fatal("modern Node source stage accepted")
	}
}

func TestCommonNodeQualificationRejectsExactFallback(t *testing.T) {
	tc := commonNodeCase{"exact", "14.21.2", nativeNodeRuntime}
	for _, choice := range []NativeChoice{
		{Line: "14", Image: "node:14-bullseye@" + exactDigest, imageEnv: []string{"NODE_VERSION=14.21.3"}},
		{Line: "14.21.2", Image: "node:14.21.2-bullseye@" + exactDigest},
	} {
		if _, _, err := commonNodeQualificationExpectations(tc, []NativeChoice{choice}); err == nil {
			t.Fatal("exact representative accepted a fallback or missing exact metadata")
		}
	}
}

func TestCommonNodeQualificationGenericOptInSkipsExtraProfile(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestCommonNodeGeneratedImages$", "-test.v")
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "BEX_NATIVE_TEST_NODE_PROFILE=") && !strings.HasPrefix(item, "BEX_BUILDKIT_INTEGRATION=") {
			command.Env = append(command.Env, item)
		}
	}
	command.Env = append(command.Env, "BEX_BUILDKIT_INTEGRATION=1")
	output, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "--- SKIP: TestCommonNodeGeneratedImages") {
		t.Fatalf("generic integration acquired an extra profile/input requirement: %v\n%s", err, output)
	}
}

func TestCommonNodeQualificationOutputControls(t *testing.T) {
	python, err := exec.LookPath("python3")
	must(t, err)
	command := exec.Command(python, "-B", "check_test.py")
	command.Dir = filepath.Join("testdata", "native-node-qualification")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Node output-oracle controls (no Node/image execution): %v\n%s", err, output)
	}
}
