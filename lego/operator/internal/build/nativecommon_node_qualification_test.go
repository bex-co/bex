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
	"cmp"
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

func commonNodeQualificationArtifacts(mode string) (map[string]string, error) {
	if mode == "" || mode == "0" {
		return nil, nil
	}
	if mode != "1" {
		return nil, fmt.Errorf("BEX_NATIVE_TEST_NODE_DONORS must be 0 or 1")
	}
	// Exact reviewed upstream identities, not mutable line lookups. These
	// published runtime trees still require generated common-image qualification.
	return map[string]string{
		"node:14-bullseye@sha256:c0bff0d29a742f40650d5f0305dd581351c10954e6cb6676fc96f47590b9666e": "ghcr.io/bex-co/bex-cnb-run@sha256:4fec31cbb186e340cd7ce25ac1d0705f78735631f793b75629d653c6601f83fc",
		"node:16-bookworm@sha256:73e677f85fbc56d22c817b4e5d7f72086c5343c0859f3bf8aaadab22aab29ce6": "ghcr.io/bex-co/bex-cnb-run@sha256:2802078e18496838c15159ec99c01c9b260128cef8e1d1ba29cd0a1836165113",
		"node:18-bookworm@sha256:c6ae79e38498325db67193d391e6ec1d224d96c693a8a4d943498556716d3783": "ghcr.io/bex-co/bex-cnb-run@sha256:6c40aaf4af55979e9b923e5949704f562d8e334e7370022f0c3584341742ddd1",
		"node:20-bookworm@sha256:8f693eaa7e0a8e71560c9a82b55fd54c2ae920a2ba5d2cde28bac7d1c01c9ba5": "ghcr.io/bex-co/bex-cnb-run@sha256:2c69d3e2913703a634016cc24338f9532f24a750ddb831351e1374d274846a4d",
	}, nil
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

func commonNodeDonorQualificationCases() []commonNodeCase {
	return []commonNodeCase{
		{"node-14", "14", nativeNodeRuntime},
		{"node-16", "16", nativeNodeRuntime},
		{"node-18", "18", nativeNodeRuntime},
		{"node-20", "20", nativeNodeRuntime},
		{"node-exact14213", "14.21.3", nativeNodeRuntime},
		{"ruby-node-exact14213", "14.21.3", nativeRubyRuntime},
	}
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
		line, reviewed := nativeToolchains[nativeNodeRuntime].find(node.Line)
		if version != tc.request || (node.Line != tc.request && (!reviewed || line.version != tc.request || node.Image != line.image)) {
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
	if len(nativeRuntimeArtifacts) != 0 && commonNodeBackport(version) {
		artifact, found := nativeRuntimeArtifacts[node.Image]
		if !found || node.primaryImage != artifact {
			return "", false, fmt.Errorf("donor qualification requires an exact published artifact for %s", node.Image)
		}
		binary := "COPY --link --from=" + artifact + " /usr/local/bin/node /usr/local/bin/node\n"
		tree := "COPY --link --from=" + artifact + " /usr/local/ /usr/local/\n"
		provenance := "COPY --link --from=" + artifact + " /usr/share/bex-node-brotli /usr/share/bex-node-brotli\n"
		if (!strings.Contains(canonical, binary) && !strings.Contains(canonical, tree)) || !strings.Contains(canonical, provenance) ||
			strings.Contains(canonical, "/usr/libexec/bex-native/build-node") || strings.Contains(canonical, "RUN --network=default ") {
			return "", false, fmt.Errorf("donor qualification requires its unchanged offline runtime tree without a Node patch compiler")
		}
		return canonical, false, nil
	}
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
	executed, err := nativeUtilityPrerequisiteRecipe(canonical, selected)
	return executed, true, err
}

// This is an additional profile, not the Node24 original13/28 cohort. In
// particular, pnpm on Node 14/16 is the standalone build of the same release
// (w8/226), never skipped or downgraded, and lifecycle scripts still run the
// app's Node.
func TestCommonNodeGeneratedImages(t *testing.T) {
	if os.Getenv("BEX_NATIVE_TEST_NODE_PROFILE") != "1" && os.Getenv("BEX_NATIVE_TEST_NODE_DONORS") != "1" {
		t.Skip("set BEX_NATIVE_TEST_NODE_PROFILE=1 or BEX_NATIVE_TEST_NODE_DONORS=1 for the separate common Node profile")
	}
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") != "1" {
		t.Fatal("Node profile also requires BEX_BUILDKIT_INTEGRATION=1")
	}
	runtimeArtifacts, err := commonNodeQualificationArtifacts(os.Getenv("BEX_NATIVE_TEST_NODE_DONORS"))
	must(t, err)
	common, _, _ := nativeUtilityTestImages(t)
	docker := nativeUtilityDockerConfig(t)
	if common == "" || docker.builder == "" {
		t.Fatal("Node profile requires an immutable common image and explicitly owned builder/platform")
	}
	original, supplied, artifacts := nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = original, supplied, artifacts })
	nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = common, nil, runtimeArtifacts
	output := t.TempDir()
	if requested := os.Getenv("BEX_NATIVE_TEST_OUTPUT"); requested != "" {
		must(t, os.MkdirAll(requested, 0o700))
		var err error
		output, err = os.MkdirTemp(requested, "native-common-node-")
		must(t, err)
	}
	t.Logf("retained Node profile evidence: %s", output)
	cases := commonNodeQualificationCases()
	if len(runtimeArtifacts) != 0 {
		cases = commonNodeDonorQualificationCases()
	}
	for _, tc := range cases {
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
	artifactMode := "same-version-source-fallback"
	if len(nativeRuntimeArtifacts) != 0 {
		artifactMode = "published-runtime-donors"
	}
	custody, err := json.MarshalIndent(map[string]any{
		"common": common, "choices": choices, "node_source": node.Image,
		"node_runtime_artifact": cmp.Or(node.primaryImage, node.Image),
		"runtime_artifact_mode": artifactMode, "runtime_artifact_map": nativeRuntimeArtifacts,
		"builder": docker.builder, "platform": docker.platform, "owner": docker.owner,
		"canonical_sha256":          fmt.Sprintf("%x", sha256.Sum256(canonical)),
		"executed_sha256":           fmt.Sprintf("%x", sha256.Sum256([]byte(executed))),
		"test_only_network_overlay": overlay, "empty_runtime_artifact_map": len(nativeRuntimeArtifacts) == 0, "fixture_sha256": hashes,
		"legacy_pnpm": "Node14/16 run the pnpm10.34.7 standalone (w8/226); strict tool/lifecycle assertions remain enabled",
		"scope":       "additional Node family generated-image gate; no original13/28, native-init, Kubernetes, hosted or common publication claim",
	}, "", "  ")
	must(t, err)
	must(t, os.WriteFile(filepath.Join(evidence, "source-custody.json"), custody, 0o600))
	tag := fmt.Sprintf("bex-common-%s:%d-%d", tc.name, os.Getpid(), time.Now().UnixNano())
	t.Cleanup(func() { nativeUtilityCleanup("image", "rm", "-f", tag) })
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Minute)
	defer cancel()
	extra := []string{"--secret", "id=render-env,src=" + bundle, "-t", tag}
	args := docker.buildArgs(extra, source, true)
	if overlay {
		args = docker.prerequisiteBuildArgs(extra, source, true)
	}
	command := exec.CommandContext(ctx, "docker", args...)
	log, buildErr := command.CombinedOutput()
	must(t, os.WriteFile(filepath.Join(evidence, "build.log"), log, 0o600))
	if buildErr != nil {
		t.Fatalf("actual common Node offline build: %v\n%s", buildErr, log)
	}
	docker.runImage(t, nativeUtilityCase{name: tc.name}, tag, evidence, overlay)
}
