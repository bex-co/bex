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

package controller

import (
	"testing"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestBuildpackEnvFiltersRuntimeAndSecretValues(t *testing.T) {
	env := buildEnv(build.BuilderBuildpack, []appv1alpha1.EnvVar{
		{Name: "BP_GO_TARGETS", Value: "./cmd/api"},
		{Name: "BPE_DEFAULT_BEX", Value: "1"},
		{Name: "RUNTIME_ONLY", Value: "not-in-build"},
		{Name: "BP_SECRET", ValueFrom: &appv1alpha1.EnvVarSource{SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "env", Key: "BP_SECRET"}}},
	})
	if len(env) != 2 || env[0].Name != "BP_GO_TARGETS" || env[1].Name != "BPE_DEFAULT_BEX" {
		t.Fatalf("buildpackEnv = %#v", env)
	}
}

func TestNativeBuildEnvKeepsAllLiteralValues(t *testing.T) {
	env := buildEnv(build.BuilderNative, []appv1alpha1.EnvVar{
		{Name: "NODE_ENV", Value: "production"},
		{Name: "BP_NODE_VERSION", Value: "24"},
		{Name: "SECRET", ValueFrom: &appv1alpha1.EnvVarSource{SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "env", Key: "SECRET"}}},
	})
	if len(env) != 2 || env[0].Name != "NODE_ENV" || env[1].Name != "BP_NODE_VERSION" {
		t.Fatalf("buildEnv(native) = %#v", env)
	}
}

func TestEffectiveBuilderMapsRenderRuntime(t *testing.T) {
	for runtime, want := range map[string]string{
		"node":   build.BuilderNative,
		"rust":   build.BuilderNative,
		"docker": build.BuilderDockerfile,
		"":       build.BuilderBuildpack,
	} {
		spec := appv1alpha1.AppSpec{Runtime: runtime, Builder: build.BuilderBuildpack}
		if got := effectiveBuilder(spec); got != want {
			t.Errorf("effectiveBuilder(runtime=%q) = %q, want %q", runtime, got, want)
		}
	}
}
