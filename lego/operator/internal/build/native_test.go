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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func nativeOptions() Options {
	return Options{
		Repo: "https://github.com/example/app", Ref: "main", Name: "web",
		Registry: "zot:5000", Revision: "gen-1", Namespace: "bex-system",
		Builder: BuilderNative, Runtime: "node",
		BuildCommand: "npm ci && npm run build",
		StartCommand: "node dist/server.js --flag='quoted'",
		BuildEnv: []corev1.EnvVar{
			{Name: "NODE_ENV", Value: "production"},
			{Name: "PORT", Value: "9999"},
		},
		RuntimeEnvSecret: "web-env",
	}
}

func TestNativeDockerfilePreservesRenderCommands(t *testing.T) {
	o := nativeOptions()
	dockerfile := nativeDockerfile(o)
	for _, want := range []string{
		"FROM node:24-bookworm@sha256:",
		"npm ci && npm run build",
		`CMD ["/bin/bash","-lc","node dist/server.js --flag='quoted'"]`,
		"--mount=type=secret,id=render-env",
	} {
		if !strings.Contains(dockerfile, want) {
			t.Errorf("Dockerfile missing %q:\n%s", want, dockerfile)
		}
	}
}

func TestNativeRuntimeImagesAreDigestPinned(t *testing.T) {
	for runtime, image := range nativeRuntimeImages {
		parts := strings.Split(image, "@sha256:")
		if len(parts) != 2 || len(parts[1]) != 64 {
			t.Errorf("native runtime %s image is not digest-pinned: %q", runtime, image)
		}
	}
}

func TestNativeBuildJobPreparesDockerfileAndSecretEnv(t *testing.T) {
	o := nativeOptions()
	job := BuildJob(o, o.ImageRef())
	if len(job.Spec.Template.Spec.InitContainers) != 3 {
		t.Fatalf("initContainers = %d, want clone, preparer, buildkit", len(job.Spec.Template.Spec.InitContainers))
	}
	prep := job.Spec.Template.Spec.InitContainers[1]
	if prep.Name != "prepare-native-build" {
		t.Fatalf("preparer name = %q", prep.Name)
	}
	if hasEnv(prep.Env, "PORT") {
		t.Fatal("operator-owned PORT must not enter the native build env bundle")
	}
	if !hasEnv(prep.Env, "NODE_ENV") {
		t.Fatal("literal service env missing from native build preparer")
	}
	prepScript := prep.Args[0]
	if strings.Index(prepScript, "for path in /runtime-env/*") > strings.Index(prepScript, "for key in $BEX_NATIVE_LITERAL_KEYS") {
		t.Fatal("literal env must be appended after envFrom Secret so it wins like Kubernetes container env")
	}
	if !hasVolume(job.Spec.Template.Spec.Volumes, "runtime-env") {
		t.Fatal("OpenBao-backed runtime env Secret not mounted for native build")
	}
	buildkit := containerByName(job.Spec.Template.Spec.InitContainers, "buildkit")
	if !containsPair(buildkit.Args, "--secret", "id=render-env,src=/native/render-env") {
		t.Fatalf("buildkit args missing render env secret: %v", buildkit.Args)
	}
}

func TestValidateNativeOptions(t *testing.T) {
	o := nativeOptions()
	if err := validateNativeOptions(o); err != nil {
		t.Fatal(err)
	}
	o.Runtime = "java"
	if err := validateNativeOptions(o); err == nil {
		t.Fatal("unsupported runtime accepted")
	}
}

func hasEnv(env []corev1.EnvVar, name string) bool {
	for _, item := range env {
		if item.Name == name {
			return true
		}
	}
	return false
}

func hasVolume(volumes []corev1.Volume, name string) bool {
	for _, volume := range volumes {
		if volume.Name == name {
			return true
		}
	}
	return false
}

func containsPair(items []string, first, second string) bool {
	for i := 0; i+1 < len(items); i++ {
		if items[i] == first && items[i+1] == second {
			return true
		}
	}
	return false
}
