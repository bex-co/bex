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

func TestCommonNodeQualificationDonorMode(t *testing.T) {
	for _, mode := range []string{"", "0"} {
		artifacts, err := commonNodeQualificationArtifacts(mode)
		must(t, err)
		if len(artifacts) != 0 {
			t.Fatal("source mode acquired a runtime donor")
		}
	}
	if _, err := commonNodeQualificationArtifacts("unreviewed"); err == nil {
		t.Fatal("unknown donor mode accepted")
	}
	artifacts, err := commonNodeQualificationArtifacts("1")
	must(t, err)
	if len(artifacts) != 5 || len(commonNodeDonorQualificationCases()) != 8 {
		t.Fatal("five published runtime donors and eight separate qualification cells are required")
	}
	for source, artifact := range artifacts {
		must(t, validateNativeBase(source))
		must(t, validateNativeBase(artifact))
		if !strings.HasPrefix(source, "node:") || !strings.HasPrefix(artifact, "ghcr.io/bex-co/bex-cnb-run@") {
			t.Fatal("donor is not bound to an immutable official Node identity and reviewed public package")
		}
	}
}

func TestCommonNodeQualificationDonorCells(t *testing.T) {
	original, supplied, artifacts := nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = original, supplied, artifacts })
	var err error
	nativeCommonBase, nativeSuppliedBases = commonFixtureImage, nil
	nativeRuntimeArtifacts, err = commonNodeQualificationArtifacts("1")
	must(t, err)
	for _, tc := range commonNodeDonorQualificationCases() {
		t.Run(tc.name, func(t *testing.T) {
			o := nativeOptions()
			o.Runtime, o.BuildEnv, o.RuntimeEnvSecret = tc.runtime, nil, ""
			o.BuildCommand, o.StartCommand = "/usr/bin/python3 check.py build", "/usr/bin/python3 check.py runtime"
			bundle, path, source := writeResolveFixture(t, o, nil, commonNodeQualificationFiles(t, tc))
			fake := commonNodeQualificationFake()
			if tc.request == "14.21.2" {
				// This control binds the real approved upstream key; the ENV is
				// synthetic metadata only, never a new supplier execution claim.
				fake.digests["node:14.21.2-bullseye"] = "sha256:cc748803fe49871926c433ae6aafbfd59b765eed6005dc0843b916eb74f67b12"
				fake.envByImage["node:14.21.2-bullseye@sha256:cc748803fe49871926c433ae6aafbfd59b765eed6005dc0843b916eb74f67b12"] = fake.envByImage["node:14.21.2-bullseye@"+exactDigest]
			}
			choices, err := ResolveNativeBuildExact(context.Background(), fake, time.Time{}, tc.runtime, bundle, path, source)
			must(t, err)
			expected, node, err := commonNodeQualificationExpectations(tc, choices)
			must(t, err)
			artifact, found := nativeRuntimeArtifacts[node.Image]
			if !found || node.primaryImage != artifact {
				t.Fatal("selected official identity did not choose its exact reviewed donor")
			}
			if tc.request == "14.21.2" && artifact != "ghcr.io/bex-co/bex-cnb-run@sha256:332869e644cdd45bf1f9e7c42ed3df91fedb95c4d0cee6bf2abf771c9eb760c7" {
				t.Fatal("exact14.21.2 did not select its strictly reviewed supplier subject")
			}
			for _, asked := range fake.asked {
				if strings.HasPrefix(asked, "node source ") {
					t.Fatal("published donor unexpectedly requested compiler inputs")
				}
			}
			canonical, err := os.ReadFile(path)
			must(t, err)
			executed, overlay, err := commonNodeQualificationRecipe(string(canonical), node)
			must(t, err)
			if overlay || executed != string(canonical) || strings.Contains(executed, "build-node") || strings.Contains(executed, "\nFROM "+artifact+"\n") {
				t.Fatal("donor mode changed the common base or acquired a security compiler/network overlay")
			}
			var contract map[string]any
			must(t, json.Unmarshal(expected, &contract))
			if (contract["node_env"] != nil) != (tc.runtime == nativeNodeRuntime) {
				t.Fatal("companion imported donor NODE_VERSION into its primary environment")
			}
			if tc.runtime == nativeRubyRuntime && !strings.Contains(executed, "COPY --link --from="+artifact+" /usr/local/include/node /usr/local/include/node\n") {
				t.Fatal("companion headers did not come from its selected runtime donor")
			}
			for _, changed := range []string{
				executed + "RUN /usr/libexec/bex-native/build-node --common-bookworm 14.21.3 unreviewed\n",
				executed + "RUN --network=default unreviewed\n",
				strings.ReplaceAll(executed, "--from="+artifact+" /usr/share/bex-node-brotli", "--from="+node.Image+" /usr/share/bex-node-brotli"),
			} {
				if _, _, err := commonNodeQualificationRecipe(changed, node); err == nil {
					t.Fatal("donor recipe accepted a changed compiler, network or provenance boundary")
				}
			}
		})
	}
}

func TestCommonNodeQualificationDonorRequiresExactArtifact(t *testing.T) {
	original, supplied, artifacts := nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = original, supplied, artifacts })
	nativeCommonBase, nativeSuppliedBases = commonFixtureImage, nil
	for _, runtime := range []string{nativeNodeRuntime, nativeRubyRuntime} {
		for _, exactDonor := range []bool{false, true} {
			t.Run(runtime+map[bool]string{false: "-missing", true: "-exact"}[exactDonor], func(t *testing.T) {
				var err error
				nativeRuntimeArtifacts, err = commonNodeQualificationArtifacts("1")
				must(t, err)
				exactImage := "node:14.21.2-bullseye@" + exactDigest
				if exactDonor {
					// Synthetic same-patch artifact for generator behavior only.
					nativeRuntimeArtifacts[exactImage] = "ghcr.io/bex-co/bex-cnb-run@" + otherDigest
				}
				tc := commonNodeCase{"exact", "14.21.2", runtime}
				o := nativeOptions()
				o.Runtime = runtime
				bundle, path, source := writeResolveFixture(t, o, nil, commonNodeQualificationFiles(t, tc))
				choices, err := ResolveNativeBuildExact(context.Background(), commonNodeQualificationFake(), time.Time{}, runtime, bundle, path, source)
				must(t, err)
				_, node, err := commonNodeQualificationExpectations(tc, choices)
				must(t, err)
				if node.Image != exactImage || node.Line != "14.21.2" {
					t.Fatal("different exact request was replaced by the reviewed14.21.3 donor")
				}
				canonical, err := os.ReadFile(path)
				must(t, err)
				executed, overlay, err := commonNodeQualificationRecipe(string(canonical), node)
				if !exactDonor {
					if err == nil || !strings.Contains(err.Error(), "requires an exact published artifact") {
						t.Fatal("missing exact donor silently entered the source qualification path")
					}
					return
				}
				must(t, err)
				if overlay || executed != string(canonical) || strings.Contains(executed, "build-node") {
					t.Fatal("exact donor required a compiler or changed its canonical recipe")
				}
			})
		}
	}
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
			if nativeUtilityWithoutRunNetwork(executed) != nativeUtilityWithoutRunNetwork(string(canonical)) {
				t.Fatal("test overlay changed canonical payload, authored command or runtime")
			}
			for _, required := range []string{"\nFROM " + commonFixtureImage + "\n", o.BuildCommand, o.StartCommand, nativePnpmFor(nodeVersion)} {
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
		if !strings.HasPrefix(item, "BEX_NATIVE_TEST_NODE_PROFILE=") && !strings.HasPrefix(item, "BEX_NATIVE_TEST_NODE_DONORS=") && !strings.HasPrefix(item, "BEX_BUILDKIT_INTEGRATION=") {
			command.Env = append(command.Env, item)
		}
	}
	command.Env = append(command.Env, "BEX_BUILDKIT_INTEGRATION=1")
	output, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "--- SKIP: TestCommonNodeGeneratedImages") {
		t.Fatalf("generic integration acquired an extra profile/input requirement: %v\n%s", err, output)
	}
}

func TestCommonNodeQualificationDonorOptInRequiresIntegration(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestCommonNodeGeneratedImages$", "-test.v")
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "BEX_NATIVE_TEST_NODE_PROFILE=") && !strings.HasPrefix(item, "BEX_NATIVE_TEST_NODE_DONORS=") && !strings.HasPrefix(item, "BEX_BUILDKIT_INTEGRATION=") {
			command.Env = append(command.Env, item)
		}
	}
	command.Env = append(command.Env, "BEX_NATIVE_TEST_NODE_DONORS=1")
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "Node profile also requires BEX_BUILDKIT_INTEGRATION=1") {
		t.Fatalf("standalone donor opt-in did not require the actual integration boundary: %v\n%s", err, output)
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

// Regression uses existing APIs so the same test compiles against the failing source.
func TestNativeSourcePrerequisiteNetworkRegression(t *testing.T) {
	original := nativeRuntimeArtifacts
	t.Cleanup(func() { nativeRuntimeArtifacts = original })
	nativeRuntimeArtifacts = nil
	node := NativeChoice{imageEnv: []string{"NODE_VERSION=14.21.3"}}
	stage, err := commonNodeBuildStage("node:14-bullseye@"+exactDigest, "14.21.3", strings.Repeat("b", 64))
	must(t, err)
	node.primaryStage = stage
	primary := NativeChoice{Image: "erlang:28.0@" + exactDigest, commonOTP: "28.0"}
	otp := "RUN --mount=type=bind,from=" + primary.Image + ",source=/usr/local/lib/erlang,target=/opt/bex-otp-reference,readonly /usr/libexec/bex-native/build-otp 28.0 " + strings.Repeat("a", 64) + "\n"
	primary.primaryStage = otp
	for _, name := range []string{"Node", "OTP"} {
		t.Run(name, func(t *testing.T) {
			prefix := stage
			if name == "OTP" {
				prefix = "FROM platform-source\n" + otp
			}
			canonical := prefix + "FROM " + commonFixtureImage + "\nRUN preserve-tool-setup\nCOPY . .\nRUN --mount=type=secret,id=render-env [\"/bin/bash\",\"-c\",\". /opt/bex/load-env\\noriginal-authored-build\"]\nCMD [\"original-runtime\"]\n"
			var executed string
			if name == "Node" {
				executed, _, err = commonNodeQualificationRecipe(canonical, node)
			} else {
				executed, _, err = commonElixirQualificationRecipe(canonical, primary)
			}
			must(t, err)
			if strings.Count(executed, "RUN --network=default ") != 1 {
				t.Fatal("exact platform source prerequisite must be the only online RUN")
			}
			for line := range strings.SplitSeq(executed, "\n") {
				if strings.HasPrefix(line, "RUN ") && !strings.HasPrefix(line, "RUN --network=none ") && !strings.HasPrefix(line, "RUN --network=default ") {
					t.Fatalf("non-prerequisite command inherits the open build network: %s", line)
				}
			}
			if !strings.Contains(executed, "original-authored-build") || !strings.Contains(executed, "CMD [\"original-runtime\"]") {
				t.Fatal("original authored commands changed")
			}
		})
	}
}

// The dated AMD primary was independently admitted as e6/e4e evidence. Replay
// its exact verified metadata inputs without a registry read; this is a recipe
// equivalence control, never a new image execution or current-source CI claim.
func TestCommonNodeDonorMatchesDatedPrimaryRecipe(t *testing.T) {
	original, supplied, artifacts := nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeSuppliedBases, nativeRuntimeArtifacts = original, supplied, artifacts })
	nativeCommonBase = "ghcr.io/bex-co/bex-cnb-run@sha256:0941c204f87643c2faad308e72acdf584976e67d8fb980a0646c5ab662dcc5d0"
	nativeSuppliedBases = nil
	var err error
	nativeRuntimeArtifacts, err = commonNodeQualificationArtifacts("1")
	must(t, err)
	const selected = "node:14.21.2-bullseye@sha256:cc748803fe49871926c433ae6aafbfd59b765eed6005dc0843b916eb74f67b12"
	fake := commonNodeQualificationFake()
	fake.digests["node:14.21.2-bullseye"] = "sha256:cc748803fe49871926c433ae6aafbfd59b765eed6005dc0843b916eb74f67b12"
	fake.envByImage[selected] = fakeExactMetadata{env: []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"NODE_VERSION=14.21.2", "YARN_VERSION=1.22.19",
	}}
	o := nativeOptions()
	o.Runtime, o.BuildEnv, o.RuntimeEnvSecret = nativeNodeRuntime, nil, ""
	o.BuildCommand, o.StartCommand = "/usr/bin/python3 check.py build", "/usr/bin/python3 check.py runtime"
	bundle, path, source := writeResolveFixture(t, o, nil, commonNodeQualificationFiles(t, commonNodeCase{"node-exact14212", "14.21.2", nativeNodeRuntime}))
	choices, err := ResolveNativeBuildExact(t.Context(), fake, time.Time{}, nativeNodeRuntime, bundle, path, source)
	must(t, err)
	_, node, err := commonNodeQualificationExpectations(commonNodeCase{"node-exact14212", "14.21.2", nativeNodeRuntime}, choices)
	must(t, err)
	canonical, err := os.ReadFile(path)
	must(t, err)
	executed, overlay, err := commonNodeQualificationRecipe(string(canonical), node)
	must(t, err)
	const admitted = "6dddfdf0cb7dcde88eae806ac54fdd3ed125ea0cfbbe1cfa0cd2be5a43fc11f6"
	if overlay || executed != string(canonical) || fmt.Sprintf("%x", sha256.Sum256(canonical)) != admitted {
		t.Fatal("donor branch changed the admitted primary canonical recipe or acquired an overlay")
	}
	for _, asked := range fake.asked {
		if strings.HasPrefix(asked, "node source ") {
			t.Fatal("dated donor replay acquired a compiler input")
		}
	}
}
