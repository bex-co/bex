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
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/bex-co/bex/lego/operator/internal/execution/executiontest"
)

// expandedBuildValues models ordered literal Env.Value resolution. Secret
// values never pass this expansion stage; those use ValueFrom in Kubernetes.
func expandedBuildValues(env []corev1.EnvVar) map[string]string {
	values := map[string]string{"HOME": "/platform/home", "PORT": "10000", "SHELL": "/bin/platform-shell"}
	for _, item := range env {
		if item.ValueFrom == nil {
			values[item.Name] = executiontest.Expand(item.Value, values)
		}
	}
	return values
}

func TestNativeRecipeAndBuildLiteralsSurviveKubeletExpansion(t *testing.T) {
	o := nativeOptions()
	o.BuildCommand = `printf 'build=%s\n' "a$$b"; printf '%s\n' '$(HOME)' "$MARK"`
	o.StartCommand = `printf 'start=%s\n' "a$$b"; printf '%s\n' '$(PORT)' "$MARK"`
	o.BuildEnv = []corev1.EnvVar{{Name: "MARK", Value: "pa$$wo$(HOME)rd\n$$(PORT)\n"}, {Name: "PATH", Value: os.Getenv("PATH") + ":/literal/$$/$(HOME)"}}
	prep := nativeBuildPreparer(o)
	resolved := expandedBuildValues(prep.Env)
	if got := resolved["BEX_NATIVE_DOCKERFILE"]; got != nativeDockerfile(o) {
		t.Fatalf("generated recipe changed through kubelet:\n%s", got)
	}
	if got := resolved[nativeLiteralEnvPrefix+"0"]; got != o.BuildEnv[0].Value {
		t.Fatalf("MARK changed: %q", got)
	}
	if got := resolved[nativeLiteralEnvPrefix+"1"]; got != o.BuildEnv[1].Value {
		t.Fatalf("PATH changed: %q", got)
	}
	// Exercise the real preparer shell, which writes the post-kubelet value to
	// disk. Do not reduce this to checking doubled-dollar text in the pod spec.
	dir := t.TempDir()
	script := strings.NewReplacer("/native", dir, "/runtime-env", filepath.Join(dir, "missing-env")).Replace(prep.Args[0])
	cmd := exec.Command("sh", "-eu", "-c", executiontest.Expand(script, resolved))
	cmd.Env = os.Environ()
	for _, item := range prep.Env {
		cmd.Env = append(cmd.Env, item.Name+"="+resolved[item.Name])
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("preparer failed: %v: %s", err, out)
	}
	recipe, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if string(recipe) != nativeDockerfile(o) {
		t.Fatal("preparer did not write the authored recipe byte-for-byte")
	}
	var command []string
	for line := range strings.SplitSeq(string(recipe), "\n") {
		if value, ok := strings.CutPrefix(line, "CMD "); ok {
			if err := json.Unmarshal([]byte(value), &command); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(command) != 3 || command[2] != o.StartCommand {
		t.Fatalf("image CMD lost authored command: %q", command)
	}
	output, err := runNativeBuildShell(t, o, filepath.Join(dir, "render-env"))
	if err != nil || !regexp.MustCompile(`(?m)^build=a[0-9]+b$`).Match(output) || !strings.Contains(string(output), o.BuildEnv[0].Value) {
		t.Fatalf("authored build execution=%q err=%v", output, err)
	}
	start := exec.Command(command[0], command[1:]...)
	start.Env = append(os.Environ(), "MARK="+o.BuildEnv[0].Value)
	output, err = start.CombinedOutput()
	if err != nil || !regexp.MustCompile(`(?m)^start=a[0-9]+b$`).Match(output) || !strings.Contains(string(output), o.BuildEnv[0].Value) {
		t.Fatalf("authored start execution=%q err=%v", output, err)
	}

}

func TestDockerBuildLiteralArgsSurviveKubeletAndInputShell(t *testing.T) {
	o := dockerOpts()
	o.NativeFiles = nil
	o.NativeFilesSecret = ""
	o.RuntimeEnvSecret = ""
	literal := "a$$b $(HOME) $$(PORT)\ntrailing\n\n"
	o.BuildEnv = []corev1.EnvVar{{Name: "MARK", Value: literal}}
	env := dockerBuildInputsEnv(o, BuildLiterals(o.BuildEnv))
	resolved := expandedBuildValues(env)
	if resolved[dockerBuildArgEnv+"0"] != literal {
		t.Fatalf("indexed literal changed: %q", resolved)
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "args")
	script := strings.NewReplacer(dockerBuildEnvMount, filepath.Join(dir, "missing-env"), dockerBuildFilesMount, filepath.Join(dir, "missing-files"), dockerBuildInputsMount, filepath.Join(dir, "inputs")).Replace(dockerBuildInputsScript(o, BuildLiterals(o.BuildEnv))) + `printf '%s\0' "$@" > ` + out
	cmd := exec.Command("sh", "-eu", "-c", executiontest.Expand(script, resolved), "bex-buildkit")
	cmd.Env = os.Environ()
	for _, item := range env {
		cmd.Env = append(cmd.Env, item.Name+"="+resolved[item.Name])
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("input script: %v: %s", err, output)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--opt", "build-arg:MARK=" + literal}
	got := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
	if !slices.Equal(got, want) {
		t.Fatalf("buildctl args=%q want=%q", got, want)
	}
}

func TestKpackBuildEnvSurvivesItsPreparePodsExpansion(t *testing.T) {
	literal := "a$$b $(HOME) $$(PLATFORM_ENV_MARK)\n"
	o := opts()
	o.BuildEnv = []corev1.EnvVar{{Name: "BP_MARK", Value: literal}}
	image := KpackImage(o)
	env, found, err := unstructured.NestedSlice(image.Object, "spec", "build", "env")
	if err != nil || !found || len(env) != 1 {
		t.Fatalf("kpack env=%v found=%v err=%v", env, found, err)
	}
	// kpack v0.18.0 Build.BuildPod copies build env values into PLATFORM_ENV_
	// prepare-container Env.Value unchanged before writing platform files:
	// github.com/buildpacks-community/kpack/blob/v0.18.0/pkg/apis/build/v1alpha2/build_pod.go
	entry := env[0].(map[string]any)
	prepared := executiontest.Expand(entry["value"].(string), map[string]string{"HOME": "/builder/home", "PLATFORM_ENV_MARK": "collision"})
	if prepared != literal {
		t.Fatalf("kpack literal after kubelet=%q want=%q", prepared, literal)
	}
}

func TestCloneAndBuildPathsSurviveKubeletExpansion(t *testing.T) {
	o := dockerOpts()
	o.Repo = "https://github.com/acme/a$$b$(HOME)"
	o.Ref = "branch$$$(HOME)"
	o.RootDir = "src/a$$b/$(HOME)"
	o.DockerContext = "context/a$$b/$(HOME)"
	o.DockerfilePath = "Docker$$file$(HOME)"
	clone := buildCloneContainer(o, defaultGitImage)
	resolved := expandedBuildValues(clone.Env)
	for key, want := range map[string]string{
		"REPO": o.Repo, "REF": o.Ref, "BEX_ROOT_DIR": repoRelative(o.RootDir), "BEX_ROOT_DIR_PATH": boundedSourceDir(o.RootDir),
		"BEX_DOCKER_CONTEXT": repoRelative(o.DockerContext), "BEX_DOCKER_CONTEXT_PATH": boundedSourceDir(o.DockerContext),
	} {
		if got := resolved[key]; got != want {
			t.Fatalf("%s=%q want=%q", key, got, want)
		}
	}
	job := BuildJob(o, o.ImageRef())
	buildkit := buildkitContainer(t, job)
	args := make([]string, 0, len(buildkit.Args))
	for _, arg := range buildkit.Args {
		args = append(args, executiontest.Expand(arg, resolved))
	}
	for _, want := range []string{"context=" + boundedSourceDir(o.DockerContext), "dockerfile=" + boundedSourceDir(o.RootDir), "filename=" + o.DockerfilePath} {
		if !slices.Contains(args, want) {
			t.Fatalf("buildctl args=%q missing literal %q", args, want)
		}
	}
	resolver := nativeVersionResolver(o)
	if got := expandedBuildValues(resolver.Env)["BEX_NATIVE_SOURCE_DIR"]; got != boundedSourceDir(o.RootDir) {
		t.Fatalf("native resolver source=%q", got)
	}
	// Raw Options remain reusable; escapes are added only at Kubernetes fields.
	if !reflect.DeepEqual(o.BuildEnv, dockerOpts().BuildEnv) {
		t.Fatal("projection mutated build intent")
	}
}
