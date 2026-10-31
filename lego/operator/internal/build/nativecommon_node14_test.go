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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommonNode14AddonPythonIsPrivateForPrimaryAndCompanion(t *testing.T) {
	original, artifacts := nativeCommonBase, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeRuntimeArtifacts = original, artifacts })
	nativeCommonBase, nativeRuntimeArtifacts = commonFixtureImage, nil
	python, _ := nativeToolchains[nativePythonRuntime].find("3.9")
	for _, tc := range []struct{ runtime, version string }{
		{nativeNodeRuntime, "14"}, {nativeNodeRuntime, "14.21.2"},
		{nativePythonRuntime, "14"}, {nativePythonRuntime, "14.21.2"},
		{nativeNodeRuntime, "24"}, {nativePythonRuntime, "24"},
	} {
		t.Run(tc.runtime+"/"+tc.version, func(t *testing.T) {
			o := nativeOptions()
			o.Runtime = tc.runtime
			env := map[string]string{"NODE_VERSION": tc.version, "PYTHON_VERSION": "3.13"}
			fake := &fakeExact{digests: map[string]string{"node:14.21.2-bullseye": exactDigest}, envByImage: map[string]fakeExactMetadata{}}
			for runtime, chain := range nativeToolchains {
				for _, line := range chain.lines {
					fake.envByImage[line.image] = commonReviewedFixtureMetadata(runtime, line)
				}
			}
			fake.envByImage["node:14.21.2-bullseye@"+exactDigest] = fakeExactMetadata{env: []string{"NODE_VERSION=14.21.2", "PATH=/usr/local/bin:/usr/bin"}}
			bundle, path, source := writeResolveFixture(t, o, env, nil)
			_, err := ResolveNativeBuildExact(context.Background(), fake, time.Time{}, tc.runtime, bundle, path, source)
			must(t, err)
			raw, err := os.ReadFile(path)
			must(t, err)
			recipe := string(raw)
			pythonSource := python.image
			if python.upstream != "" {
				pythonSource = python.upstream
			}
			private := "COPY --link --from=" + pythonSource + " /usr/local/ /opt/bex/native-node-python/\n"
			if tc.version == "24" {
				if strings.Contains(recipe, "native-node-python") || strings.Contains(recipe, "prepare-node14-tools") {
					t.Fatal("modern Node acquired a legacy interpreter")
				}
				return
			}
			helper := "RUN --network=none /usr/bin/python3 -I /usr/libexec/bex-native/prepare-node14-tools.py " + python.version + "\n"
			copyAt, helperAt, authoredAt := strings.Index(recipe, private), strings.Index(recipe, helper), strings.Index(recipe, "\nCOPY . .\n")
			if copyAt < 0 || helperAt < copyAt || authoredAt < helperAt || strings.Count(recipe, private) != 1 {
				t.Fatalf("private exact tool interpreter/order missing:\n%s", recipe)
			}
			final := recipe[strings.Index(recipe, "\nFROM "+commonFixtureImage+"\n"):]
			for _, forbidden := range []string{"ENV PYTHON=", "ENV npm_config_python=", "ENV LD_LIBRARY_PATH=", "ENV PYTHONHOME=", "ENV PYTHONPATH=", "--from=" + pythonSource + " /usr/local/ /usr/local/"} {
				if strings.Contains(final, forbidden) {
					t.Fatalf("addon fallback changed app environment/prefix: %s", forbidden)
				}
			}
			if tc.runtime == nativePythonRuntime && !strings.Contains(final, `ENV PYTHON_VERSION="3.13.18"`) {
				t.Fatal("Node addon fallback changed the actual primary Python")
			}
		})
	}
}

func TestCommonNodeAddonPythonRequiresReviewedBookwormInput(t *testing.T) {
	original := nativeToolchains[nativePythonRuntime]
	t.Cleanup(func() { nativeToolchains[nativePythonRuntime] = original })
	for _, source := range []string{"python:3.9-bullseye@" + exactDigest, "python:3.9-bookworm:mutable"} {
		changed := original
		changed.lines = append([]nativeLine(nil), original.lines...)
		for i := range changed.lines {
			if changed.lines[i].line == "3.9" {
				changed.lines[i].image, changed.lines[i].upstream = source, ""
			}
		}
		nativeToolchains[nativePythonRuntime] = changed
		if _, _, err := commonNodeBuildPython(); err == nil {
			t.Fatalf("unreviewed addon interpreter source accepted: %s", source)
		}
	}
}

// This separate fixture does not change the original 13 tool replays or 28
// hosted programs. It qualifies the selected Node14 runtime TREE above the
// common base; the same-release Node source-rebuild path needs its own proof.
func TestCommonNode14OfflineAddon(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("opt-in actual common Node14 offline addon build")
	}
	common, _, _ := nativeUtilityTestImages(t)
	tree := os.Getenv("BEX_NATIVE_TEST_NODE14_TREE")
	if common == "" || tree == "" {
		t.Fatal("requires immutable BEX_NATIVE_TEST_COMMON_IMAGE and reviewed BEX_NATIVE_TEST_NODE14_TREE")
	}
	must(t, validateNativeBase(tree))
	docker := nativeUtilityDockerConfig(t)
	original, artifacts := nativeCommonBase, nativeRuntimeArtifacts
	t.Cleanup(func() { nativeCommonBase, nativeRuntimeArtifacts = original, artifacts })
	line, _ := nativeToolchains[nativeNodeRuntime].find("14")
	nativeCommonBase, nativeRuntimeArtifacts = common, map[string]string{line.image: tree}
	files := map[string]string{}
	for _, name := range []string{"package.json", "binding.gyp", "addon.c", "check.js", "check-build.py"} {
		data, err := os.ReadFile(filepath.Join("testdata", "native-node14-addon", name))
		must(t, err)
		files[name] = string(data)
	}
	o := nativeOptions()
	o.BuildCommand, o.StartCommand = "/usr/bin/python3 check-build.py "+line.version, "node check.js "+line.version
	bundle, path, source := writeResolveFixture(t, o, map[string]string{"NODE_VERSION": "14"}, files)
	_, err := ResolveNativeBuildAt(time.Time{}, o.Runtime, bundle, path, source)
	must(t, err)
	recipe, err := os.ReadFile(path)
	must(t, err)
	output := t.TempDir()
	if requested := os.Getenv("BEX_NATIVE_TEST_OUTPUT"); requested != "" {
		output, err = os.MkdirTemp(requested, "native-node14-addon-")
		must(t, err)
	}
	t.Logf("retained Node14 addon evidence: %s", output)
	must(t, os.WriteFile(filepath.Join(output, "Dockerfile.canonical"), recipe, 0o600))
	must(t, os.Rename(path, filepath.Join(source, "Dockerfile")))
	tag := fmt.Sprintf("bex-native-node14-addon:%d-%d", os.Getpid(), time.Now().UnixNano())
	t.Cleanup(func() { nativeUtilityCleanup("image", "rm", "-f", tag) })
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "docker", docker.buildArgs([]string{"--secret", "id=render-env,src=" + bundle, "-t", tag}, source, true)...)
	log, buildErr := command.CombinedOutput()
	must(t, os.WriteFile(filepath.Join(output, "build.log"), log, 0o600))
	if buildErr != nil {
		t.Fatalf("actual offline Node14 addon build: %v\n%s", buildErr, log)
	}
	docker.runImage(t, nativeUtilityCase{name: "node14-addon"}, tag, output, false)
}
