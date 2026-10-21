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
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

// dockerOpts is a Dockerfile build with Secret-backed env, literals and two
// secret files, as the controller hands it over for a Render-shaped service.
func dockerOpts() Options {
	o := opts()
	o.Builder = BuilderDockerfile
	o.RuntimeEnvSecret = "hello-native-env"
	o.BuildEnv = []corev1.EnvVar{
		{Name: "PATH", Value: "/literal/bin"},
		{Name: "GREETING", Value: "from-literal"},
		{Name: "PORT", Value: "9999"},
	}
	o.NativeFilesSecret = "bld-hello-native-files"
	o.NativeFiles = []string{".env", "credentials.json"}
	return o
}

func buildkitContainer(t *testing.T, job *batchv1.Job) corev1.Container {
	t.Helper()
	spec := job.Spec.Template.Spec
	c := containerByName(append(spec.InitContainers, spec.Containers...), "buildkit")
	if c == nil {
		t.Fatal("no buildkit container in the build Job")
	}
	return *c
}

func TestRenderSecretFileID(t *testing.T) {
	for name, want := range map[string]string{
		".env":             "_env",
		"credentials.json": "credentials_json",
		"my-key_1":         "my-key_1",
		"a.b.c":            "a_b_c",
	} {
		if got := renderSecretFileID(name); got != want {
			t.Errorf("renderSecretFileID(%q) = %q, want %q", name, got, want)
		}
	}
}

// The Job carries the plumbing — mounts, Render-id secret flags, indexed
// literals — but never a Secret-backed value, which only the running container
// reads from its mount.
func TestDockerfileBuildJobMountsEnvAndSecretFiles(t *testing.T) {
	o := dockerOpts()
	job := BuildJob(o, o.ImageRef())
	c := buildkitContainer(t, job)

	for _, want := range []string{
		"id=_env,src=" + dockerBuildInputsMount + "/files/.env",
		"id=credentials_json,src=" + dockerBuildInputsMount + "/files/credentials.json",
	} {
		i := slices.Index(c.Args, want)
		if i < 1 || c.Args[i-1] != "--secret" {
			t.Errorf("buildkit args lack --secret %s: %q", want, c.Args)
		}
	}

	mounts := map[string]corev1.VolumeMount{}
	for _, m := range c.VolumeMounts {
		mounts[m.Name] = m
	}
	for _, name := range []string{runtimeEnvVolume, secretFilesVolume, buildInputsVolume} {
		if _, ok := mounts[name]; !ok {
			t.Errorf("buildkit container does not mount %s: %+v", name, c.VolumeMounts)
		}
	}
	if !mounts[runtimeEnvVolume].ReadOnly || !mounts[secretFilesVolume].ReadOnly || mounts[buildInputsVolume].ReadOnly {
		t.Errorf("Secret mounts must be read-only and the snapshot writable: %+v", mounts)
	}

	volumes := map[string]corev1.Volume{}
	for _, v := range job.Spec.Template.Spec.Volumes {
		volumes[v.Name] = v
	}
	if v := volumes[runtimeEnvVolume]; v.Secret == nil || v.Secret.SecretName != o.RuntimeEnvSecret {
		t.Errorf("env volume = %+v, want Secret %s", v, o.RuntimeEnvSecret)
	}
	if v := volumes[secretFilesVolume]; v.Secret == nil || v.Secret.SecretName != o.NativeFilesSecret {
		t.Errorf("secret-files volume = %+v, want Secret %s", v, o.NativeFilesSecret)
	}
	if v := volumes[buildInputsVolume]; v.EmptyDir == nil {
		t.Errorf("snapshot volume = %+v, want an EmptyDir", v)
	}

	env := map[string]string{}
	for _, e := range c.Env {
		env[e.Name] = e.Value
	}
	if got := env[dockerBuildArgKeysEnv]; got != "GREETING\nPATH" {
		t.Errorf("literal keys = %q, want GREETING and PATH sorted, PORT dropped", got)
	}
	if env[dockerBuildArgEnv+"0"] != "from-literal" || env[dockerBuildArgEnv+"1"] != "/literal/bin" {
		t.Errorf("indexed literals = %v", env)
	}
	if _, ok := env["PATH"]; ok {
		t.Error("a literal PATH must not become the BuildKit container's own PATH")
	}
	for _, ic := range job.Spec.Template.Spec.InitContainers {
		if ic.Name == "prepare-native-build" {
			t.Error("a Dockerfile build must not run the native preparer")
		}
	}
}

// Runs the container's input script locally against stand-in mounts and reads
// back the positional parameters it hands to buildctl.
func TestDockerfileBuildInputsScriptAssemblesBuildArgs(t *testing.T) {
	o := dockerOpts()
	dir := t.TempDir()
	envDir, filesDir, inputsDir := filepath.Join(dir, "env"), filepath.Join(dir, "files"), filepath.Join(dir, "inputs")
	for _, d := range []string{envDir, filepath.Join(envDir, "..data"), filesDir, inputsDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, value string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(envDir, "GREETING"), "from-secret")
	write(filepath.Join(envDir, "MULTI"), "a\nb\n\n")
	write(filepath.Join(envDir, "EQ"), "x=y=z")
	write(filepath.Join(filesDir, ".env"), "TOKEN=1\n")
	write(filepath.Join(filesDir, "credentials.json"), `{"k":"v"}`)

	out := filepath.Join(dir, "args")
	script := strings.NewReplacer(dockerBuildEnvMount, envDir, dockerBuildFilesMount, filesDir, dockerBuildInputsMount, inputsDir).
		Replace(dockerBuildInputsScript(o, BuildLiterals(o.BuildEnv))) + `printf '%s\0' "$@" > "` + out + `"`
	cmd := exec.Command("sh", "-eu", "-c", script, "bex-buildkit", "--frontend", "dockerfile.v0")
	cmd.Env = os.Environ()
	for _, e := range dockerBuildInputsEnv(o, BuildLiterals(o.BuildEnv)) {
		cmd.Env = append(cmd.Env, e.Name+"="+e.Value)
	}
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("input script: %v: %s", err, b)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
	want := []string{
		"--frontend", "dockerfile.v0",
		"--opt", "build-arg:EQ=x=y=z",
		"--opt", "build-arg:GREETING=from-secret",
		"--opt", "build-arg:MULTI=a\nb\n\n",
		"--opt", "build-arg:GREETING=from-literal",
		"--opt", "build-arg:PATH=/literal/bin",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("buildctl args =\n%q\nwant\n%q", got, want)
	}

	// buildctl keeps the last --opt per key (cmd/buildctl/build attrMap), so a
	// literal overrides a Secret key exactly as appEnv lets it at runtime.
	effective := map[string]string{}
	for i := 0; i+1 < len(got); i++ {
		if got[i] == "--opt" {
			k, v, _ := strings.Cut(got[i+1], "=")
			effective[k] = v
		}
	}
	if effective["build-arg:GREETING"] != "from-literal" {
		t.Errorf("effective GREETING = %q, want the literal", effective["build-arg:GREETING"])
	}

	for name, want := range map[string]string{".env": "TOKEN=1\n", "credentials.json": `{"k":"v"}`} {
		b, err := os.ReadFile(filepath.Join(inputsDir, "files", name))
		if err != nil || !bytes.Equal(b, []byte(want)) {
			t.Errorf("snapshot of %s = %q (%v), want %q", name, b, err, want)
		}
	}
}

func TestDockerfileSecretFileIDCollisionFailsTheBuild(t *testing.T) {
	o := dockerOpts()
	o.NativeFiles = []string{"a.b", "a_b"}
	o.Client = fakeClient()
	_, err := EnsureBuild(context.Background(), o)
	if err == nil || !strings.Contains(err.Error(), `"a.b"`) || !strings.Contains(err.Error(), `"a_b"`) {
		t.Fatalf("EnsureBuild = %v, want a refusal naming both files", err)
	}
	var jobs batchv1.JobList
	if err := o.Client.List(context.Background(), &jobs); err != nil {
		t.Fatal(err)
	}
	if len(jobs.Items) != 0 {
		t.Fatalf("a refused build created %d Job(s)", len(jobs.Items))
	}
}

// A Dockerfile service with no env and no secret files adds no input script,
// carrier env or volume, so its Job keeps the pre-w1/m178 shape; one with only
// literal env gets the script and carriers but no Secret volume.
func TestDockerfileBuildInputsOnlyWhenPresent(t *testing.T) {
	bare := opts()
	bare.Builder = BuilderDockerfile
	literalOnly := bare
	literalOnly.BuildEnv = []corev1.EnvVar{{Name: "GREETING", Value: "hi"}}

	for _, tc := range []struct {
		name       string
		o          Options
		wantScript bool
	}{{"no inputs", bare, false}, {"literals only", literalOnly, true}} {
		job := BuildJob(tc.o, tc.o.ImageRef())
		c := buildkitContainer(t, job)
		// The revision stamp's RENDER_GIT_COMMIT build-arg is on every build.
		script := strings.Replace(strings.Join(c.Command, " "), revisionStamp, "", 1)
		if got := strings.Contains(script, "build-arg:"); got != tc.wantScript {
			t.Errorf("%s: input script present = %v, want %v", tc.name, got, tc.wantScript)
		}
		var carriers []string
		for _, e := range c.Env {
			if strings.HasPrefix(e.Name, "BEX_BUILD_") {
				carriers = append(carriers, e.Name+"="+e.Value)
			}
		}
		if want := tc.wantScript; (len(carriers) > 0) != want {
			t.Errorf("%s: carrier env = %q", tc.name, carriers)
		}
		for _, v := range job.Spec.Template.Spec.Volumes {
			switch v.Name {
			case runtimeEnvVolume, secretFilesVolume, buildInputsVolume:
				t.Errorf("%s: unexpected volume %s", tc.name, v.Name)
			}
		}
	}
}
