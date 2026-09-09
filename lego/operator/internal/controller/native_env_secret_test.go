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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func nativeEnvScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func envSecret(namespace, name string, data map[string]string) *corev1.Secret {
	byteData := make(map[string][]byte, len(data))
	for k, v := range data {
		byteData[k] = []byte(v)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Type:       corev1.SecretTypeOpaque,
		Data:       byteData,
	}
}

func nativeEnvApp(name string, groups []string, own string) *appv1alpha1.App {
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "default", UID: types.UID("uid-" + name),
	}}
	app.Spec.EnvFromSecrets = groups
	app.Spec.EnvFromSecret = own
	return app
}

// The w4/m93 regression: a linked environment group's value reached runtime but
// never the native build. The merged projection must carry group-only keys,
// keep later groups over earlier ones, and keep the service's own Secret over
// every group — exactly the runtime envFrom order Kubernetes applies.
func TestProjectNativeBuildEnvMergesGroupsAndOwn(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(
		envSecret("default", "evg-a-env", map[string]string{
			"MESSAGE": "qa-group-value", "SHARED": "from-a", "LAYERED": "from-a",
		}),
		envSecret("default", "evg-b-env", map[string]string{"LAYERED": "from-b"}),
		envSecret("default", "web-env", map[string]string{"SHARED": "from-own"}),
	).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	app := nativeEnvApp("web", []string{"evg-a-env", "evg-b-env"}, "web-env")

	name, _, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil)
	if err != nil {
		t.Fatal(err)
	}
	if name != "web-native-env" {
		t.Fatalf("merged secret name = %q", name)
	}
	var merged corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "bex-build", Name: name}, &merged); err != nil {
		t.Fatalf("merged secret not created: %v", err)
	}
	for key, want := range map[string]string{
		"MESSAGE": "qa-group-value", // group-only key must reach the build
		"LAYERED": "from-b",         // later group wins
		"SHARED":  "from-own",       // own service wins over groups
	} {
		if got := string(merged.Data[key]); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if merged.Labels["app.bex.co/app-uid"] != "uid-web" || merged.Labels["app.bex.co/component"] != "native-env-secret" {
		t.Fatalf("merged Secret missing artifact ownership labels: %v", merged.Labels)
	}
}

func TestProjectNativeBuildEnvGroupOnlyWithoutOwnSecret(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(
		envSecret("default", "evg-a-env", map[string]string{"MESSAGE": "qa-group-value"}),
	).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	app := nativeEnvApp("web", []string{"evg-a-env"}, "")

	name, _, err := r.projectNativeBuildEnv(context.Background(), app, "default", nil)
	if err != nil {
		t.Fatal(err)
	}
	var merged corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &merged); err != nil {
		t.Fatal(err)
	}
	if got := string(merged.Data["MESSAGE"]); got != "qa-group-value" {
		t.Fatalf("group-only MESSAGE = %q, want qa-group-value", got)
	}
}

// A briefly-absent group Secret is optional (mirroring the runtime envFrom
// projection); the service's own Secret is required — a partial environment
// must fail the build, never silently build without saved values.
func TestProjectNativeBuildEnvSourcePresenceContract(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(
		envSecret("default", "evg-a-env", map[string]string{"MESSAGE": "qa-group-value"}),
	).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}

	app := nativeEnvApp("web", []string{"evg-a-env", "evg-gone-env"}, "")
	name, _, err := r.projectNativeBuildEnv(context.Background(), app, "default", nil)
	if err != nil {
		t.Fatalf("absent group must be optional: %v", err)
	}
	var merged corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &merged); err != nil {
		t.Fatal(err)
	}
	if got := string(merged.Data["MESSAGE"]); got != "qa-group-value" {
		t.Fatalf("MESSAGE = %q after skipping the absent group", got)
	}

	missingOwn := nativeEnvApp("api", nil, "api-env")
	if _, _, err := r.projectNativeBuildEnv(context.Background(), missingOwn, "default", nil); err == nil {
		t.Fatal("missing own Secret must fail the projection")
	}
}

func TestProjectNativeBuildEnvNoSources(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	app := nativeEnvApp("web", nil, "")

	name, rev, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil)
	if err != nil {
		t.Fatal(err)
	}
	if name != "" {
		t.Fatalf("no sources must produce no mount, got %q", name)
	}
	if rev != build.NativeEnvNoneRevision {
		t.Fatalf("empty environment revision = %q, want %q", rev, build.NativeEnvNoneRevision)
	}
	var list corev1.SecretList
	if err := cl.List(context.Background(), &list, client.InNamespace("bex-build")); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("no sources must create no Secret, found %d", len(list.Items))
	}
}

func TestProjectNativeBuildEnvRefusesProtectedSource(t *testing.T) {
	protected := envSecret("default", "bex-tenant-postgres", map[string]string{"AWS_SECRET_ACCESS_KEY": "shared"})
	protected.Labels = map[string]string{execution.LabelProtectedFromTenantMount: execution.ProtectedFromTenantMount}
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(protected).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	app := nativeEnvApp("web", []string{"bex-tenant-postgres"}, "")

	if _, _, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil); err == nil ||
		!strings.Contains(err.Error(), "protected") {
		t.Fatalf("protected source must be refused, got %v", err)
	}
	var list corev1.SecretList
	if err := cl.List(context.Background(), &list, client.InNamespace("bex-build")); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("refused projection must write nothing, found %d Secrets", len(list.Items))
	}
}

// The deterministic destination name derives from the workspace-local App name,
// so a same-named foreign App's merged Secret in the shared build namespace
// must be refused, not clobbered — the copyCloneSecret ownership contract.
func TestProjectNativeBuildEnvRefusesForeignOwner(t *testing.T) {
	foreign := envSecret("bex-build", "web-native-env", map[string]string{"THEIRS": "bytes"})
	foreign.Labels = map[string]string{"app.bex.co/app": "web", "app.bex.co/app-uid": "uid-other"}
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(
		foreign,
		envSecret("default", "web-env", map[string]string{"SHARED": "mine"}),
	).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	app := nativeEnvApp("web", nil, "web-env")

	if _, _, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil); err == nil {
		t.Fatal("foreign-owned destination must be refused")
	}
	var kept corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "bex-build", Name: "web-native-env"}, &kept); err != nil {
		t.Fatal(err)
	}
	if got := string(kept.Data["THEIRS"]); got != "bytes" {
		t.Fatalf("foreign Secret clobbered: %q", got)
	}
}

// Two Apps linking one group each get their own merged destination; deleting or
// rebuilding one never rewrites the other's input or the shared group source.
func TestProjectNativeBuildEnvSharedGroupIsolation(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(
		envSecret("default", "evg-a-env", map[string]string{"MESSAGE": "qa-group-value"}),
		envSecret("default", "web-env", map[string]string{"OWN": "web"}),
		envSecret("default", "api-env", map[string]string{"OWN": "api"}),
	).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	web := nativeEnvApp("web", []string{"evg-a-env"}, "web-env")
	api := nativeEnvApp("api", []string{"evg-a-env"}, "api-env")

	webName, _, err := r.projectNativeBuildEnv(context.Background(), web, "bex-build", nil)
	if err != nil {
		t.Fatal(err)
	}
	apiName, _, err := r.projectNativeBuildEnv(context.Background(), api, "bex-build", nil)
	if err != nil {
		t.Fatal(err)
	}
	if webName == apiName {
		t.Fatalf("both Apps merged into %q", webName)
	}
	for name, own := range map[string]string{webName: "web", apiName: "api"} {
		var merged corev1.Secret
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "bex-build", Name: name}, &merged); err != nil {
			t.Fatal(err)
		}
		if string(merged.Data["MESSAGE"]) != "qa-group-value" || string(merged.Data["OWN"]) != own {
			t.Fatalf("%s data = %v", name, merged.Data)
		}
	}
	var source corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "evg-a-env"}, &source); err != nil {
		t.Fatal(err)
	}
	if len(source.Labels) != 0 {
		t.Fatalf("shared group source must not be adopted: %v", source.Labels)
	}
}

// Unlinking a group replaces the merged bundle wholesale on the next build, so
// a removed key cannot linger from a prior projection.
func TestProjectNativeBuildEnvUnlinkDropsGroupKeys(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(
		envSecret("default", "evg-a-env", map[string]string{"MESSAGE": "qa-group-value"}),
		envSecret("default", "web-env", map[string]string{"OWN": "web"}),
	).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	app := nativeEnvApp("web", []string{"evg-a-env"}, "web-env")

	if _, _, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil); err != nil {
		t.Fatal(err)
	}
	app.Spec.EnvFromSecrets = nil
	name, _, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil)
	if err != nil {
		t.Fatal(err)
	}
	var merged corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "bex-build", Name: name}, &merged); err != nil {
		t.Fatal(err)
	}
	if _, stale := merged.Data["MESSAGE"]; stale {
		t.Fatal("unlinked group key must not linger in the merged bundle")
	}
	if got := string(merged.Data["OWN"]); got != "web" {
		t.Fatalf("OWN = %q", got)
	}
}

// A direct App edit to the linked-group list must produce a fresh native
// artifact — the build bakes those values in — while a Dockerfile service's
// artifact (whose build never reads them) stays put.
func TestNativeArtifactIdentityIncludesEnvFromSecrets(t *testing.T) {
	spec := appv1alpha1.AppSpec{
		Repo: "https://github.com/bex-co/bex.git", Branch: "main",
		Runtime: "go", Builder: "native",
		BuildCommand: "go build -o app .", StartCommand: "./app", Port: 3000,
	}
	linked := spec
	linked.EnvFromSecrets = []string{"evg-a-env"}
	if desiredAppReleaseIdentity(spec).artifact == desiredAppReleaseIdentity(linked).artifact {
		t.Fatal("linking a group must change the native artifact identity")
	}

	docker := spec
	docker.Runtime = ""
	docker.Builder = "dockerfile"
	dockerLinked := docker
	dockerLinked.EnvFromSecrets = []string{"evg-a-env"}
	if desiredAppReleaseIdentity(docker).artifact != desiredAppReleaseIdentity(dockerLinked).artifact {
		t.Fatal("a Dockerfile build does not consume group env; its artifact must not change")
	}
}

// The opaque revision must stay put across reconcile when the effective
// environment is unchanged, and bump when Secret bytes or literals change —
// without putting secret values into annotations that BuildKit will see.
func TestProjectNativeBuildEnvRevisionStabilityAndInvalidation(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(
		envSecret("default", "web-env", map[string]string{"MESSAGE": "A"}),
	).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	app := nativeEnvApp("web", nil, "web-env")

	name, rev1, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil)
	if err != nil {
		t.Fatal(err)
	}
	if rev1 != "1" {
		t.Fatalf("first revision = %q, want 1", rev1)
	}
	_, rev2, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil)
	if err != nil {
		t.Fatal(err)
	}
	if rev2 != rev1 {
		t.Fatalf("unchanged input reminted revision %q → %q", rev1, rev2)
	}

	var own corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web-env"}, &own); err != nil {
		t.Fatal(err)
	}
	own.Data["MESSAGE"] = []byte("B")
	if err := cl.Update(context.Background(), &own); err != nil {
		t.Fatal(err)
	}
	_, rev3, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil)
	if err != nil {
		t.Fatal(err)
	}
	if rev3 != "2" {
		t.Fatalf("Secret value change revision = %q, want 2", rev3)
	}

	litA := []corev1.EnvVar{{Name: "MESSAGE", Value: "literal-A"}}
	_, rev4, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", litA)
	if err != nil {
		t.Fatal(err)
	}
	if rev4 != "3" {
		t.Fatalf("literal overlay revision = %q, want 3", rev4)
	}
	_, rev5, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", litA)
	if err != nil {
		t.Fatal(err)
	}
	if rev5 != rev4 {
		t.Fatalf("unchanged literals reminted revision %q → %q", rev4, rev5)
	}
	_, rev6, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build",
		[]corev1.EnvVar{{Name: "MESSAGE", Value: "literal-B"}})
	if err != nil {
		t.Fatal(err)
	}
	if rev6 != "4" {
		t.Fatalf("literal value change revision = %q, want 4", rev6)
	}

	var merged corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "bex-build", Name: name}, &merged); err != nil {
		t.Fatal(err)
	}
	for _, v := range merged.Annotations {
		if strings.Contains(v, "literal-A") || strings.Contains(v, "literal-B") ||
			strings.Contains(v, "MESSAGE=A") || v == "A" || v == "B" {
			t.Fatalf("annotations must not carry raw env values: %v", merged.Annotations)
		}
	}
	if merged.Annotations[annotNativeEnvRevision] != rev6 {
		t.Fatalf("persisted revision = %q, want %q", merged.Annotations[annotNativeEnvRevision], rev6)
	}
	if len(merged.Annotations[annotNativeEnvInput]) != 64 {
		t.Fatalf("input token should be hex sha256, got %q", merged.Annotations[annotNativeEnvInput])
	}
}

func TestProjectNativeBuildEnvLiteralsAlonePersistRevision(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	app := nativeEnvApp("web", nil, "")
	literals := []corev1.EnvVar{{Name: "MESSAGE", Value: "only-literal"}}

	name, rev, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", literals)
	if err != nil {
		t.Fatal(err)
	}
	if name != "web-native-env" || rev != "1" {
		t.Fatalf("literals-only got name=%q rev=%q", name, rev)
	}
	var merged corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "bex-build", Name: name}, &merged); err != nil {
		t.Fatal(err)
	}
	if len(merged.Data) != 0 {
		t.Fatalf("literals-only Secret must carry no Data keys, got %v", merged.Data)
	}
	_, rev2, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", literals)
	if err != nil {
		t.Fatal(err)
	}
	if rev2 != rev {
		t.Fatalf("literals-only restart reminted %q → %q", rev, rev2)
	}
}

func TestProjectNativeBuildEnvUnlinkBumpsRevision(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(
		envSecret("default", "evg-a-env", map[string]string{"MESSAGE": "qa-group-value"}),
		envSecret("default", "web-env", map[string]string{"OWN": "web"}),
	).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	app := nativeEnvApp("web", []string{"evg-a-env"}, "web-env")

	_, rev1, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil)
	if err != nil {
		t.Fatal(err)
	}
	app.Spec.EnvFromSecrets = nil
	_, rev2, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil)
	if err != nil {
		t.Fatal(err)
	}
	if rev2 == rev1 {
		t.Fatal("unlinking a group must invalidate the native env revision")
	}
}
