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
	"testing"
	"time"
)

// The legacy and modern profiles share the application compiler/project
// consumer and retained image runner. Their tool solves, interpreter checks
// and wheel exports remain explicit in each profile.
type nativePythonQualification struct {
	family, version, evidence, bundle, source string
	files                                     map[string]string
	canonical                                 []byte
	choices                                   []NativeChoice
}

func prepareNativePythonQualification(t *testing.T, family, version, output string, files map[string]string) nativePythonQualification {
	t.Helper()
	q := nativePythonQualification{family: family, version: version, evidence: filepath.Join(output, version), files: files}
	must(t, os.Mkdir(q.evidence, 0o700))
	o := nativeOptions()
	o.Runtime = nativePythonRuntime
	o.BuildCommand, o.StartCommand = "/usr/local/bin/python3 check.py build "+version, "/usr/local/bin/python3 check.py runtime "+version
	bundle, path, source := writeResolveFixture(t, o, map[string]string{"PYTHON_VERSION": version}, files)
	q.bundle, q.source = bundle, source
	var err error
	q.choices, err = ResolveNativeBuildAt(time.Time{}, o.Runtime, bundle, path, source)
	must(t, err)
	q.canonical, err = os.ReadFile(path)
	must(t, err)
	must(t, os.WriteFile(filepath.Join(q.evidence, "Dockerfile.canonical"), q.canonical, 0o600))
	return q
}

func (q nativePythonQualification) run(t *testing.T, docker nativeUtilityDocker, common, prepared, wheelExport, solve string, metadata []byte) {
	t.Helper()
	actualPrerequisite, err := os.ReadFile(filepath.Join(filepath.Dir(wheelExport), "Dockerfile.prerequisites"))
	must(t, err)
	if string(actualPrerequisite) != prepared {
		t.Fatal("Python wheel closure was prepared with another common/runtime/tool solve")
	}
	must(t, os.WriteFile(filepath.Join(q.evidence, "tool-python.json"), metadata, 0o600))
	copyNativeUtilityWheels(t, filepath.Join(wheelExport, "wheels"), q.source, q.evidence)
	wheelManifest, err := os.ReadFile(filepath.Join(q.evidence, "test-wheelhouse.json"))
	must(t, err)
	const overlay = "COPY .bex-test-wheels/ /opt/bex-test-wheels/\nENV PIP_NO_INDEX=1 PIP_FIND_LINKS=/opt/bex-test-wheels\n"
	recipe, err := afterNativeBase(string(q.canonical), common, overlay)
	must(t, err)
	must(t, os.WriteFile(filepath.Join(q.source, "Dockerfile"), []byte(recipe), 0o600))
	fileHashes := map[string]string{}
	for name, content := range q.files {
		fileHashes[name] = fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	}
	receipt, err := json.MarshalIndent(map[string]any{"common": common, "choices": q.choices, "platform": docker.platform, "builder": docker.builder, "owner": docker.owner,
		"canonical_sha256": fmt.Sprintf("%x", sha256.Sum256(q.canonical)), "executed_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(recipe))),
		"overlay": overlay, "prerequisite_recipe_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(prepared))),
		"actual_shared_wheel_export": wheelExport, "wheel_solve": solve, "fixture_sha256": fileHashes,
		"tool_metadata_sha256": fmt.Sprintf("%x", sha256.Sum256(metadata)), "wheel_manifest_sha256": fmt.Sprintf("%x", sha256.Sum256(wheelManifest)),
		"scope": "separate common " + q.family + " Python family gate; original13/28 unchanged; no native-init/Kubernetes/hosted claim"}, "", "  ")
	must(t, err)
	must(t, os.WriteFile(filepath.Join(q.evidence, "source-custody.json"), receipt, 0o600))
	must(t, os.WriteFile(filepath.Join(q.evidence, "Dockerfile"), []byte(recipe), 0o600))
	tag := fmt.Sprintf("bex-native-%s-python:%s-%d-%d", q.family, q.version, os.Getpid(), time.Now().UnixNano())
	t.Cleanup(func() { nativeUtilityCleanup("image", "rm", "-f", tag) })
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "docker", docker.buildArgs([]string{"--secret", "id=render-env,src=" + q.bundle, "-t", tag}, q.source, true)...)
	log, buildErr := command.CombinedOutput()
	must(t, os.WriteFile(filepath.Join(q.evidence, "build.log"), log, 0o600))
	if buildErr != nil {
		t.Fatalf("actual generated %s Python build: %v\n%s", q.family, buildErr, log)
	}
	docker.runImage(t, nativeUtilityCase{name: q.family + "-python-" + q.version}, tag, q.evidence, true)
}
