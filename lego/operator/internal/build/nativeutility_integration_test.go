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
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This opt-in gate runs the actual generator/resolver and independent utility
// consumers. It never changes the production supplier catalog or publishes.
// BEX_NATIVE_TEST_NODE_IMAGE must be a real immutable Node24 supplier. Optional
// BEX_NATIVE_TEST_PYTHON313_IMAGE supplies genuine Python3.13, with an explicitly
// prepared offline wheel cache for pyvips3.2.0, Django5.2.18 and Poetry2.1.3.
// BEX_NATIVE_TEST_OUTPUT retains recipes, command logs and actual output proofs.
func TestNativeUtilityGeneratedImages(t *testing.T) {
	if os.Getenv("BEX_BUILDKIT_INTEGRATION") == "" {
		t.Skip("set BEX_BUILDKIT_INTEGRATION=1 and BEX_NATIVE_TEST_NODE_IMAGE to an immutable Node24 supplier")
	}
	nodeImage, pythonImage := os.Getenv("BEX_NATIVE_TEST_NODE_IMAGE"), os.Getenv("BEX_NATIVE_TEST_PYTHON313_IMAGE")
	for name, image := range map[string]string{"node": nodeImage, "python": pythonImage} {
		if name == "python" && image == "" {
			continue
		}
		ref, digest, pinned := strings.Cut(image, "@")
		if !pinned || ref == "" || !digestPattern.MatchString(digest) {
			t.Fatalf("%s test image must name a real immutable supplier", name)
		}
	}
	output := t.TempDir()
	if requested := os.Getenv("BEX_NATIVE_TEST_OUTPUT"); requested != "" {
		must(t, os.MkdirAll(requested, 0o700))
		var err error
		output, err = os.MkdirTemp(requested, "native-utilities-")
		must(t, err)
		t.Logf("retained proof directory: %s", output)
	}
	original := nativeSuppliedBases
	t.Cleanup(func() { nativeSuppliedBases = original })
	node, _ := nativeToolchains[nativeNodeRuntime].find("24")
	python, _ := nativeToolchains[nativePythonRuntime].find("3.13")
	files := nativeUtilityFixtureFiles(t)
	type utilityCase struct {
		name, runtime, interpreter, build, start string
		static, supplied                         bool
	}
	cases := []utilityCase{
		{"supplier-removed", nativeNodeRuntime, "/usr/bin/python3", "utility-check.py build", "", false, false},
		{"node-compute", nativeNodeRuntime, "/usr/bin/python3", "utility-check.py build && /usr/bin/python3 original-cases.py build node", "utility-check.py runtime && /usr/bin/python3 original-cases.py runtime node", false, true},
		{"node-static", nativeNodeRuntime, "/usr/bin/python3", "utility-check.py build", "", true, true},
	}
	if pythonImage != "" {
		cases = append(cases, utilityCase{"python313-original", nativePythonRuntime, "/usr/local/bin/python3", "original-cases.py build python", "original-cases.py runtime python", false, true})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nativeSuppliedBases = nil
			if tc.supplied {
				nativeSuppliedBases = map[string]string{node.image: nodeImage, python.image: pythonImage}
			}
			o := nativeOptions()
			o.Runtime, o.StaticSite, o.StaticPublishPath = tc.runtime, tc.static, "public"
			o.BuildCommand, o.StartCommand = tc.interpreter+" "+tc.build, tc.interpreter+" "+tc.start
			if tc.static || !tc.supplied {
				o.StartCommand = ""
			}
			env := map[string]string{"NODE_VERSION": "24"}
			if tc.runtime == nativePythonRuntime {
				env = map[string]string{"PYTHON_VERSION": "3.13"}
			}
			bundle, dockerfile, source := writeResolveFixture(t, o, env, files)
			_, err := ResolveNativeBuild(tc.runtime, bundle, dockerfile, source)
			must(t, err)
			recipe, err := os.ReadFile(dockerfile)
			must(t, err)
			evidence := filepath.Join(output, tc.name)
			must(t, os.Mkdir(evidence, 0o700))
			must(t, os.WriteFile(filepath.Join(evidence, "Dockerfile"), recipe, 0o600))
			must(t, os.Rename(dockerfile, filepath.Join(source, "Dockerfile")))
			tag := fmt.Sprintf("bex-native-utilities-%s:%d-%d", tc.name, os.Getpid(), time.Now().UnixNano())
			t.Cleanup(func() { _ = exec.Command("docker", "image", "rm", "-f", tag).Run() })
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
			defer cancel()
			build := exec.CommandContext(ctx, "docker", "build", "--network", "none", "--progress", "plain", "--secret", "id=render-env,src="+bundle, "-t", tag, source)
			build.Env = append(os.Environ(), "DOCKER_BUILDKIT=1")
			log, buildErr := build.CombinedOutput()
			must(t, os.WriteFile(filepath.Join(evidence, "build.log"), log, 0o600))
			if !tc.supplied {
				if buildErr == nil || !strings.Contains(string(log), "pg_restore") || !strings.Contains(string(log), "FileNotFoundError") {
					t.Fatalf("supplier-removed command must fail on pg_restore: %v\n%s", buildErr, log)
				}
				return
			}
			if buildErr != nil {
				t.Fatalf("generated offline build failed: %v\n%s", buildErr, log)
			}
			create := exec.CommandContext(ctx, "docker", "create", "--network", "none", tag)
			if tc.static {
				create = exec.CommandContext(ctx, "docker", "create", "--network", "none", "--entrypoint", "/bin/true", tag)
			}
			id, err := create.Output()
			must(t, err)
			container := strings.TrimSpace(string(id))
			t.Cleanup(func() { _ = exec.Command("docker", "rm", "-f", container).Run() })
			if !tc.static {
				start, runErr := exec.CommandContext(ctx, "docker", "start", "--attach", container).CombinedOutput()
				must(t, os.WriteFile(filepath.Join(evidence, "runtime.log"), start, 0o600))
				if runErr != nil || !strings.Contains(string(start), `"pass": true`) {
					t.Fatalf("generated network-off runtime: %v\n%s", runErr, start)
				}
				if copied, err := exec.CommandContext(ctx, "docker", "cp", container+":/opt/render/project/src/qa-output", filepath.Join(evidence, "qa-output")).CombinedOutput(); err != nil {
					t.Fatalf("export actual original-program proofs: %v %s", err, copied)
				}
			} else if copied, err := exec.CommandContext(ctx, "docker", "cp", container+":/opt/render/project/src/public", filepath.Join(evidence, "public")).CombinedOutput(); err != nil {
				t.Fatalf("export static artifacts: %v %s", err, copied)
			}
			inspected, err := exec.CommandContext(ctx, "docker", "inspect", container).Output()
			must(t, err)
			var states []struct {
				Image      string
				State      struct{ ExitCode int }
				HostConfig struct{ NetworkMode string }
			}
			must(t, json.Unmarshal(inspected, &states))
			if len(states) != 1 || states[0].HostConfig.NetworkMode != "none" || states[0].State.ExitCode != 0 {
				t.Fatalf("unexpected owned container state: %s", inspected)
			}
			receipt, err := json.MarshalIndent(map[string]any{"case": tc.name, "container_id": container, "image_config_id": states[0].Image, "network": "none", "runtime_command_executed": !tc.static, "static_artifacts_extracted": tc.static, "scope": "local generated image; no hosted, Kubernetes pre-deploy or publication claim"}, "", "  ")
			must(t, err)
			must(t, os.WriteFile(filepath.Join(evidence, "receipt.json"), receipt, 0o600))
			t.Logf("%s passed actual generated network-off commands; image %s", tc.name, states[0].Image)
		})
	}
}

func nativeUtilityFixtureFiles(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{}
	must(t, filepath.WalkDir("testdata/native-utilities", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("fixture must not contain a symlink: %s", path)
		}
		relative, err := filepath.Rel("testdata/native-utilities", path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err == nil {
			files[relative] = string(content)
		}
		return err
	}))
	for name, source := range map[string]string{"pg18.dump.b64": "archive.dump.b64", "pg18-rows.txt": "rows.txt"} {
		content, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "deploy", "native-base", "testdata", source))
		must(t, err)
		files[name] = string(content)
	}
	return files
}
