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

type commonNodeCase struct {
	name, request, runtime string
}

func commonNodeQualificationCases() []commonNodeCase {
	var cases []commonNodeCase
	for _, line := range []string{"14", "16", "18", "20", "22", "26"} {
		cases = append(cases, commonNodeCase{"node-" + line, line, nativeNodeRuntime})
	}
	return append(cases,
		commonNodeCase{"node-exact14212", "14.21.2", nativeNodeRuntime},
		commonNodeCase{"ruby-node-exact14212", "14.21.2", nativeRubyRuntime})
}

func commonNodeQualificationFiles(t *testing.T, tc commonNodeCase) map[string]string {
	t.Helper()
	files := map[string]string{".node-version": tc.request + "\n"}
	for _, name := range []string{"package.json", "addon.c", "check.js", "check.py"} {
		raw, err := os.ReadFile(filepath.Join("testdata", "native-node-qualification", name))
		must(t, err)
		files[name] = string(raw)
	}
	if tc.runtime == nativeRubyRuntime {
		files[".ruby-version"] = "3.4\n"
	}
	return files
}

func commonNodeQualificationExpectations(tc commonNodeCase, choices []NativeChoice) ([]byte, NativeChoice, error) {
	index := 0
	if tc.runtime == nativeRubyRuntime {
		index = 1
	}
	if len(choices) != index+1 {
		return nil, NativeChoice{}, fmt.Errorf("Node profile requires its canonical primary and optional Ruby/Node companion")
	}
	node := choices[index]
	version, err := commonImageVariable(node.imageEnv, "NODE_VERSION")
	if err != nil || !fullVersion.MatchString(version) || node.declined != "" {
		return nil, NativeChoice{}, fmt.Errorf("selected Node lacks exact metadata")
	}
	if fullVersion.MatchString(tc.request) {
		if node.Line != tc.request || version != tc.request {
			return nil, NativeChoice{}, fmt.Errorf("exact Node representative fell back to a different patch")
		}
	} else {
		line, found := nativeToolchains[nativeNodeRuntime].find(tc.request)
		if !found || node.Line != tc.request || version != line.version {
			return nil, NativeChoice{}, fmt.Errorf("Node profile changed its reviewed line/patch")
		}
	}
	if err := validateNativeBase(node.Image); err != nil {
		return nil, NativeChoice{}, err
	}
	var nodeEnv *string
	if index == 0 {
		nodeEnv = &version
	}
	// Companions supply the selected executable/headers, not the donor's
	// entire ENV. The version file leaves NODE_VERSION absent in Ruby.
	raw, err := json.MarshalIndent(map[string]any{
		"node": version, "node_env": nodeEnv, "node_donor": node.Image,
		"yarn": strings.TrimPrefix(nativeYarnDir, "/opt/yarn-v"), "pnpm": nativePnpm.version,
		"npm_expectation": "actual selected npm package.json version must equal actual npm CLI; no fixed npm6 guess",
	}, "", "  ")
	return raw, node, err
}

func commonNodeQualificationRecipe(canonical string, node NativeChoice) (string, bool, error) {
	version, err := commonImageVariable(node.imageEnv, "NODE_VERSION")
	if err != nil {
		return "", false, err
	}
	const command = "/usr/libexec/bex-native/build-node --common-bookworm "
	if !commonNodeBackport(version) {
		if strings.Contains(canonical, command) || node.primaryStage != "" {
			return "", false, fmt.Errorf("modern Node acquired an unreviewed source builder")
		}
		return canonical, false, nil
	}
	// Empty artifact maps are intentional: qualify the same-release source
	// fallback. Only this exact trusted platform RUN gets network access.
	const prefix = "RUN /sbin/ldconfig && /usr/local/bin/python3.9 -c 'import sys; assert sys.version_info[:2] == (3, 9)' && "
	var selected string
	for line := range strings.SplitSeq(node.primaryStage, "\n") {
		if checksum, found := strings.CutPrefix(line, prefix+command+version+" "); found {
			if !digestPattern.MatchString("sha256:" + checksum) {
				return "", false, fmt.Errorf("Node source builder checksum changed")
			}
			selected = line
		}
	}
	if selected == "" || strings.Count(node.primaryStage, command) != 1 ||
		strings.Count(canonical, command) != 1 || strings.Count(canonical, "\n"+selected+"\n") != 1 ||
		!strings.Contains(canonical, "\nCOPY . .\n") {
		return "", false, fmt.Errorf("Node source prerequisite requires one exact canonical builder RUN")
	}
	executed := strings.Replace(canonical, "\n"+selected+"\n",
		"\nRUN --network=default "+strings.TrimPrefix(selected, "RUN ")+"\n", 1)
	return executed, true, nil
}

// This is an additional profile, not the Node24 original13/28 cohort. In
// particular, current pnpm14/16 startup incompatibility is an actual failure,
// never skipped, downgraded, or replaced with a private-host Node launcher.
func TestCommonNodeGeneratedImages(t *testing.T) {
	if os.Getenv("BEX_NATIVE_TEST_NODE_PROFILE") != "1" {
		t.Skip("set BEX_NATIVE_TEST_NODE_PROFILE=1 for the separate common Node profile")
	}
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") != "1" {
		t.Fatal("Node profile also requires BEX_BUILDKIT_INTEGRATION=1")
	}
	common, _, _ := nativeUtilityTestImages(t)
	docker := nativeUtilityDockerConfig(t)
	if common == "" || docker.builder == "" {
		t.Fatal("Node profile requires an immutable common image and explicitly owned builder/platform")
	}
	original, supplied, artifacts := nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = original, supplied, artifacts })
	nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = common, nil, nil
	output := t.TempDir()
	if requested := os.Getenv("BEX_NATIVE_TEST_OUTPUT"); requested != "" {
		must(t, os.MkdirAll(requested, 0o700))
		var err error
		output, err = os.MkdirTemp(requested, "native-common-node-")
		must(t, err)
	}
	t.Logf("retained Node profile evidence: %s", output)
	for _, tc := range commonNodeQualificationCases() {
		t.Run(tc.name, func(t *testing.T) { commonNodeQualificationCase(t, docker, common, tc, output) })
	}
}

func commonNodeQualificationCase(t *testing.T, docker nativeUtilityDocker, common string, tc commonNodeCase, output string) {
	t.Helper()
	evidence := filepath.Join(output, tc.name)
	must(t, os.Mkdir(evidence, 0o700))
	o := nativeOptions()
	o.Runtime, o.BuildEnv, o.RuntimeEnvSecret = tc.runtime, nil, ""
	o.BuildCommand, o.StartCommand = "/usr/bin/python3 check.py build", "/usr/bin/python3 check.py runtime"
	files := commonNodeQualificationFiles(t, tc)
	bundle, path, source := writeResolveFixture(t, o, nil, files)
	choices, err := ResolveNativeBuildAt(time.Time{}, tc.runtime, bundle, path, source)
	must(t, err)
	expected, node, err := commonNodeQualificationExpectations(tc, choices)
	must(t, err)
	files["expected-node.json"] = string(expected) + "\n"
	must(t, os.WriteFile(filepath.Join(source, "expected-node.json"), []byte(files["expected-node.json"]), 0o600))
	canonical, err := os.ReadFile(path)
	must(t, err)
	executed, overlay, err := commonNodeQualificationRecipe(string(canonical), node)
	must(t, err)
	must(t, os.WriteFile(filepath.Join(evidence, "Dockerfile.canonical"), canonical, 0o600))
	must(t, os.WriteFile(filepath.Join(evidence, "Dockerfile"), []byte(executed), 0o600))
	must(t, os.WriteFile(filepath.Join(source, "Dockerfile"), []byte(executed), 0o600))
	must(t, os.WriteFile(filepath.Join(evidence, "expected-node.json"), expected, 0o600))
	hashes := map[string]string{}
	for name, content := range files {
		hashes[name] = fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	}
	custody, err := json.MarshalIndent(map[string]any{
		"common": common, "choices": choices, "node_source": node.Image,
		"builder": docker.builder, "platform": docker.platform, "owner": docker.owner,
		"canonical_sha256":          fmt.Sprintf("%x", sha256.Sum256(canonical)),
		"executed_sha256":           fmt.Sprintf("%x", sha256.Sum256([]byte(executed))),
		"test_only_network_overlay": overlay, "empty_runtime_artifact_map": true, "fixture_sha256": hashes,
		"known_unresolved": "pnpm10.34.7 JS rejects Node14/16; strict tool/lifecycle assertions remain enabled",
		"scope":            "additional Node family generated-image gate; no original13/28, native-init, Kubernetes, hosted or common publication claim",
	}, "", "  ")
	must(t, err)
	must(t, os.WriteFile(filepath.Join(evidence, "source-custody.json"), custody, 0o600))
	tag := fmt.Sprintf("bex-common-%s:%d-%d", tc.name, os.Getpid(), time.Now().UnixNano())
	t.Cleanup(func() { nativeUtilityCleanup("image", "rm", "-f", tag) })
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "docker", docker.buildArgs([]string{"--secret", "id=render-env,src=" + bundle, "-t", tag}, source, true)...)
	log, buildErr := command.CombinedOutput()
	must(t, os.WriteFile(filepath.Join(evidence, "build.log"), log, 0o600))
	if buildErr != nil {
		t.Fatalf("actual common Node offline build (pnpm14/16 remains unresolved): %v\n%s", buildErr, log)
	}
	docker.runImage(t, nativeUtilityCase{name: tc.name}, tag, evidence, overlay)
}
