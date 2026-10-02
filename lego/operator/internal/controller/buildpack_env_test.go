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
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w1/121: a service's saved environment lives in its Secret sources, not in
// spec.env, so BP_NODE_VERSION set through the API never reached kpack. The
// projection must carry BP_/BPE_ keys from every source in runtime precedence
// (groups, then the service's own Secret, then literals) and nothing else —
// kpack has no secretKeyRef, so a non-BP value would land in the Image in clear.
func TestProjectBuildpackBuildEnvCarriesServiceBPKeysOnly(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(
		envSecret("default", "evg-a-env", map[string]string{
			"BP_NODE_VERSION": "22.*", "BPE_FROM_GROUP": "g", "GROUP_TOKEN": "group-secret",
		}),
		envSecret("default", "web-env", map[string]string{
			"BP_NODE_VERSION": "24.*", "BP_GO_TARGETS": "./cmd/from-secret", "DATABASE_URL": "postgres://secret",
		}),
	).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	app := nativeEnvApp("web", []string{"evg-a-env"}, "web-env")
	literals := buildEnv(build.BuilderBuildpack, []appv1alpha1.EnvVar{
		{Name: "BP_GO_TARGETS", Value: "./cmd/literal"},
		{Name: "RUNTIME_ONLY", Value: "dropped"},
	})

	got, err := r.projectBuildpackBuildEnv(context.Background(), app, literals)
	if err != nil {
		t.Fatal(err)
	}
	want := []corev1.EnvVar{
		{Name: "BPE_FROM_GROUP", Value: "g"},
		{Name: "BP_GO_TARGETS", Value: "./cmd/literal"}, // literal wins over the Secret
		{Name: "BP_NODE_VERSION", Value: "24.*"},        // own Secret wins over the group
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projected buildpack env = %#v, want %#v", got, want)
	}

	// The kpack Image the build plane authors from it carries the projected
	// value and no non-buildpack variable anywhere.
	image := build.KpackImage(build.Options{
		Name: "web", AppUID: "uid-web", Namespace: "bex-build", Revision: "gen-1",
		Repo: "https://github.com/bex-co/bex", Registry: "zot.bex-registry.svc:5000", BuildEnv: got,
	})
	env, _, _ := unstructured.NestedSlice(image.Object, "spec", "build", "env")
	if !slices.ContainsFunc(env, func(item any) bool {
		m, _ := item.(map[string]any)
		return m["name"] == "BP_NODE_VERSION" && m["value"] == "24.*"
	}) {
		t.Fatalf("kpack Image spec.build.env = %#v, want BP_NODE_VERSION=24.*", env)
	}
	raw, err := json.Marshal(image.Object)
	if err != nil {
		t.Fatal(err)
	}
	for _, leaked := range []string{"postgres://secret", "group-secret", "DATABASE_URL", "GROUP_TOKEN", "RUNTIME_ONLY"} {
		if strings.Contains(string(raw), leaked) {
			t.Fatalf("kpack Image carries %q — a non-buildpack variable leaked into the CR", leaked)
		}
	}
}

// Same source contract as the native build: an absent group is optional, a
// missing own Secret fails, a protected operator Secret is refused, and the
// in-flight release reads the saved sources rather than the served snapshot.
func TestProjectBuildpackBuildEnvSourceContract(t *testing.T) {
	protected := envSecret("default", "bex-tenant-postgres", map[string]string{"BP_X": "y"})
	protected.Labels = map[string]string{execution.LabelProtectedFromTenantMount: execution.ProtectedFromTenantMount}
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(
		protected,
		envSecret("default", "web-env", map[string]string{"BP_NODE_VERSION": "saved"}),
		envSecret("default", "web-env-r1", map[string]string{"BP_NODE_VERSION": "served"}),
	).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}

	app := nativeEnvApp("web", []string{"evg-gone-env"}, "web-env")
	app.Status.ReleaseGeneration, app.Status.ConfigSnapshotGeneration = 2, 1
	got, err := r.projectBuildpackBuildEnv(context.Background(), app, nil)
	if err != nil {
		t.Fatalf("absent group must be optional: %v", err)
	}
	if want := []corev1.EnvVar{{Name: "BP_NODE_VERSION", Value: "saved"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("projected env = %#v, want the saved (not served) value %#v", got, want)
	}

	if _, err := r.projectBuildpackBuildEnv(context.Background(), nativeEnvApp("api", nil, "api-env"), nil); err == nil {
		t.Fatal("missing own Secret must fail the projection")
	}
	if _, err := r.projectBuildpackBuildEnv(context.Background(), nativeEnvApp("web", []string{"bex-tenant-postgres"}, ""), nil); err == nil ||
		!strings.Contains(err.Error(), "protected") {
		t.Fatalf("protected source must be refused, got %v", err)
	}

	literals := []corev1.EnvVar{{Name: "BP_GO_TARGETS", Value: "./cmd/api"}}
	got, err = r.projectBuildpackBuildEnv(context.Background(), nativeEnvApp("web", nil, ""), literals)
	if err != nil || !reflect.DeepEqual(got, literals) {
		t.Fatalf("no Secret sources must pass the literals through, got %#v, %v", got, err)
	}
}

// Editing which Secrets a buildpack service reads changes what its build sees,
// so it must demand a fresh artifact, as it already does for native builds.
func TestBuildpackEnvSourcesAreArtifactIdentity(t *testing.T) {
	base := appv1alpha1.AppSpec{Repo: "https://github.com/bex-co/bex.git", Builder: build.BuilderBuildpack}
	baseline := desiredAppReleaseIdentity(base)
	for name, edit := range map[string]func(*appv1alpha1.AppSpec){
		"own secret":    func(s *appv1alpha1.AppSpec) { s.EnvFromSecret = "web-env" },
		"group secrets": func(s *appv1alpha1.AppSpec) { s.EnvFromSecrets = []string{"evg-a-env"} },
	} {
		changed := base
		edit(&changed)
		if desiredAppReleaseIdentity(changed).artifact == baseline.artifact {
			t.Errorf("%s edit left the buildpack artifact identity unchanged", name)
		}
	}
}

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
