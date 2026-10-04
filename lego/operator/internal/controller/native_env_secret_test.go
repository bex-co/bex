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
	"slices"
	"strconv"
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

	name, _, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil, nil)
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

	name, _, err := r.projectNativeBuildEnv(context.Background(), app, "default", nil, nil)
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
	name, _, err := r.projectNativeBuildEnv(context.Background(), app, "default", nil, nil)
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
	if _, _, err := r.projectNativeBuildEnv(context.Background(), missingOwn, "default", nil, nil); err == nil {
		t.Fatal("missing own Secret must fail the projection")
	}
}

func TestProjectNativeBuildEnvNoSources(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	app := nativeEnvApp("web", nil, "")

	name, rev, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil, nil)
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

	if _, _, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil, nil); err == nil ||
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

	if _, _, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil, nil); err == nil {
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

	webName, _, err := r.projectNativeBuildEnv(context.Background(), web, "bex-build", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	apiName, _, err := r.projectNativeBuildEnv(context.Background(), api, "bex-build", nil, nil)
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

	if _, _, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil, nil); err != nil {
		t.Fatal(err)
	}
	app.Spec.EnvFromSecrets = nil
	name, _, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil, nil)
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

	name, rev1, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rev1 != "1" {
		t.Fatalf("first revision = %q, want 1", rev1)
	}
	_, rev2, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil, nil)
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
	_, rev3, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rev3 != "2" {
		t.Fatalf("Secret value change revision = %q, want 2", rev3)
	}

	litA := []corev1.EnvVar{{Name: "MESSAGE", Value: "literal-A"}}
	_, rev4, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", litA, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rev4 != "3" {
		t.Fatalf("literal overlay revision = %q, want 3", rev4)
	}
	_, rev5, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", litA, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rev5 != rev4 {
		t.Fatalf("unchanged literals reminted revision %q → %q", rev4, rev5)
	}
	_, rev6, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build",
		[]corev1.EnvVar{{Name: "MESSAGE", Value: "literal-B"}}, nil)
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

	name, rev, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", literals, nil)
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
	_, rev2, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", literals, nil)
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

	_, rev1, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	app.Spec.EnvFromSecrets = nil
	_, rev2, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rev2 == rev1 {
		t.Fatal("unlinking a group must invalidate the native env revision")
	}
}

// A build runs for the release being built, before that release is snapshotted,
// while the App still projects the served release's copies. It must read the
// saved sources — a changed build-time value otherwise never reached the build
// that was meant to ship it (w1/m152 t004).
func TestProjectNativeBuildEnvReadsTheSavedSourcesNotTheServedSnapshot(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(
		envSecret("default", "web-env", map[string]string{"NODE_ENV": "saved"}),
		envSecret("default", "web-env-r1", map[string]string{"NODE_ENV": "served"}),
	).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	app := nativeEnvApp("web", nil, "web-env")
	app.Status.ReleaseGeneration, app.Status.ConfigSnapshotGeneration = 2, 1

	name, _, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var merged corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "bex-build", Name: name}, &merged); err != nil {
		t.Fatal(err)
	}
	if got := string(merged.Data["NODE_ENV"]); got != "saved" {
		t.Fatalf("the build reads NODE_ENV=%q from the served release's copy, want the saved value", got)
	}
}

func nativeFilesApp(name string, files ...string) *appv1alpha1.App {
	app := nativeEnvApp(name, nil, "")
	app.Spec.FilesFromSecrets = files
	return app
}

func readSecret(t *testing.T, cl client.Client, namespace, name string) *corev1.Secret {
	t.Helper()
	var s corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, &s); err != nil {
		t.Fatalf("Secret %s/%s: %v", namespace, name, err)
	}
	return &s
}

// The w4/m163 regression: saved secret files reached runtime but never the
// native build. The build's merged projection must carry the runtime's exact
// precedence — later groups over earlier, the service's own files over every
// group even when listed first — with bytes (empty, trailing newlines) intact,
// and an absent source contributes nothing, like the optional runtime volume.
func TestProjectNativeBuildFilesMergesRuntimeOrder(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(
		envSecret("default", "evg-a-files", map[string]string{"shared.pem": "from-a", "layered": "from-a", "group.txt": "g"}),
		envSecret("default", "evg-b-files", map[string]string{"layered": "from-b"}),
		envSecret("default", "web-files", map[string]string{"shared.pem": "from-own", "empty": "", "nl": "two\n\n"}),
	).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	// Own Secret first in the spec: the backend's create-time order. It must
	// still win, exactly as secretFileMounts resolves it at runtime.
	app := nativeFilesApp("web", "web-files", "evg-a-files", "evg-gone-files", "evg-b-files")

	name, files, err := r.projectNativeBuildFiles(context.Background(), app, "bex-build")
	if err != nil {
		t.Fatal(err)
	}
	if name != "bld-web-native-files" {
		t.Fatalf("files Secret name = %q", name)
	}
	merged := readSecret(t, cl, "bex-build", name)
	want := map[string]string{"shared.pem": "from-own", "layered": "from-b", "group.txt": "g", "empty": "", "nl": "two\n\n"}
	if len(merged.Data) != len(want) || len(files) != len(want) {
		t.Fatalf("merged keys = %v, want %v", merged.Data, want)
	}
	for key, value := range want {
		got, ok := merged.Data[key]
		if !ok || string(got) != value || string(files[key]) != value {
			t.Errorf("%s = %q (present %v), want %q", key, got, ok, value)
		}
	}
	if merged.Labels["app.bex.co/app-uid"] != "uid-web" || merged.Labels["app.bex.co/component"] != "native-files-secret" {
		t.Fatalf("files Secret missing artifact ownership labels: %v", merged.Labels)
	}
	if len(merged.Annotations) != 0 {
		t.Fatalf("files Secret must carry no derived annotations: %v", merged.Annotations)
	}
	for _, source := range []string{"evg-a-files", "evg-b-files", "web-files"} {
		if labels := readSecret(t, cl, "default", source).Labels; len(labels) != 0 {
			t.Fatalf("source %s adopted: %v", source, labels)
		}
	}
}

// A save-only pending file Secret joins the build exactly where it joins the
// runtime volume, and the build reads the SAVED sources rather than the served
// release's snapshot (w1/m152 t004).
func TestProjectNativeBuildFilesReadsSavedAndPendingSources(t *testing.T) {
	app := nativeFilesApp("web", "web-files")
	app.Annotations = map[string]string{appv1alpha1.PendingFilesSecretAnnotation: "web-pending-files"}
	app.Status.ReleaseGeneration, app.Status.ConfigSnapshotGeneration = 2, 1
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(
		envSecret("default", "web-files", map[string]string{"cert.pem": "saved"}),
		envSecret("default", snapshotOrSource(app, "web-files"), map[string]string{"cert.pem": "served"}),
		envSecret("default", "web-pending-files", map[string]string{"extra": "pending"}),
	).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}

	_, files, err := r.projectNativeBuildFiles(context.Background(), app, "bex-build")
	if err != nil {
		t.Fatal(err)
	}
	if string(files["cert.pem"]) != "saved" || string(files["extra"]) != "pending" {
		t.Fatalf("files = %q, want saved + pending sources", files)
	}
}

// Removing the last file deletes this App's projection so removed bytes do
// not linger next to the build Job; a same-named Secret another App lifetime
// owns is neither overwritten nor deleted.
func TestProjectNativeBuildFilesRemovalAndOwnership(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(
		envSecret("default", "web-files", map[string]string{"cert.pem": "mine"}),
	).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	app := nativeFilesApp("web", "web-files")
	if _, _, err := r.projectNativeBuildFiles(context.Background(), app, "bex-build"); err != nil {
		t.Fatal(err)
	}
	app.Spec.FilesFromSecrets = nil
	name, files, err := r.projectNativeBuildFiles(context.Background(), app, "bex-build")
	if err != nil || name != "" || files != nil {
		t.Fatalf("no files = (%q, %v, %v), want no projection", name, files, err)
	}
	var gone corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "bex-build", Name: "bld-web-native-files"}, &gone); err == nil {
		t.Fatal("removed files must delete the App's build projection")
	}

	foreign := envSecret("bex-build", "bld-web-native-files", map[string]string{"theirs": "bytes"})
	foreign.Labels = map[string]string{"app.bex.co/app": "web", "app.bex.co/app-uid": "uid-other"}
	if err := cl.Create(context.Background(), foreign); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.projectNativeBuildFiles(context.Background(), app, "bex-build"); err != nil {
		t.Fatal(err)
	}
	app.Spec.FilesFromSecrets = []string{"web-files"}
	if _, _, err := r.projectNativeBuildFiles(context.Background(), app, "bex-build"); err == nil {
		t.Fatal("foreign-owned destination must be refused")
	}
	if got := string(readSecret(t, cl, "bex-build", "bld-web-native-files").Data["theirs"]); got != "bytes" {
		t.Fatalf("foreign Secret clobbered: %q", got)
	}
}

func TestProjectNativeBuildFilesRefusesProtectedSource(t *testing.T) {
	protected := envSecret("default", "bex-tenant-postgres", map[string]string{"AWS_SECRET_ACCESS_KEY": "shared"})
	protected.Labels = map[string]string{execution.LabelProtectedFromTenantMount: execution.ProtectedFromTenantMount}
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(protected).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}

	if _, _, err := r.projectNativeBuildFiles(context.Background(), nativeFilesApp("web", "bex-tenant-postgres"), "bex-build"); err == nil ||
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

// Two Apps linking one group file get isolated build projections, and App
// deletion reclaims the projection through the artifact inventory.
func TestProjectNativeBuildFilesIsolationAndReclaim(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).WithObjects(
		envSecret("default", "evg-a-files", map[string]string{"ca.pem": "group"}),
		envSecret("default", "web-files", map[string]string{"own": "web"}),
		envSecret("default", "api-files", map[string]string{"own": "api"}),
	).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	web := nativeFilesApp("web", "evg-a-files", "web-files")
	api := nativeFilesApp("api", "evg-a-files", "api-files")
	for _, app := range []*appv1alpha1.App{web, api} {
		if _, _, err := r.projectNativeBuildFiles(context.Background(), app, "bex-build"); err != nil {
			t.Fatal(err)
		}
	}
	for name, own := range map[string]string{"bld-web-native-files": "web", "bld-api-native-files": "api"} {
		merged := readSecret(t, cl, "bex-build", name)
		if string(merged.Data["ca.pem"]) != "group" || string(merged.Data["own"]) != own {
			t.Fatalf("%s data = %v", name, merged.Data)
		}
	}
	if !slices.Contains(r.knownBuildSecretNames(web), "bld-web-native-files") {
		t.Fatal("finalizer cleanup must know the files projection by name")
	}

	owned := execution.ArtifactIdentity{Name: "web", UID: "uid-web", Namespace: "default"}
	if _, _, err := build.ReclaimAppArtifacts(context.Background(), owned, "bex-build", cl); err != nil {
		t.Fatal(err)
	}
	var gone corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "bex-build", Name: "bld-web-native-files"}, &gone); err == nil {
		t.Fatal("App reclaim must delete its files projection")
	}
	readSecret(t, cl, "bex-build", "bld-api-native-files") // the other App's survives
}

// File bytes ride secret mounts BuildKit's cache key ignores, so they share the
// opaque native revision: changed, emptied, added or removed files bump it at
// an unchanged source commit, identical bytes keep it, files alone persist it,
// and an App without files keeps the token it had before files joined.
func TestNativeBuildRevisionCoversFiles(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(nativeEnvScheme(t)).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	app := nativeEnvApp("web", nil, "")
	project := func(files map[string][]byte) string {
		t.Helper()
		_, rev, err := r.projectNativeBuildEnv(context.Background(), app, "bex-build", nil, files)
		if err != nil {
			t.Fatal(err)
		}
		return rev
	}
	if rev := project(map[string][]byte{"qa-r59.txt": []byte("qa-r59-build-file-marker")}); rev != "1" {
		t.Fatalf("files-only revision = %q, want 1", rev)
	}
	if rev := project(map[string][]byte{"qa-r59.txt": []byte("qa-r59-build-file-marker")}); rev != "1" {
		t.Fatalf("identical bytes reminted %q", rev)
	}
	for i, files := range []map[string][]byte{
		{"qa-r59.txt": []byte("qa-r59-build-file-marker\n")}, // trailing newline
		{"qa-r59.txt": {}},              // emptied
		{"qa-r59.txt": {}, "b.pem": {}}, // empty file added
		{"qa-r59.txt": {}},              // file removed, one kept
		{"renamed.txt": {}},             // renamed
	} {
		if rev, want := project(files), strconv.Itoa(i+2); rev != want {
			t.Fatalf("files %q revision = %q, want %s", files, rev, want)
		}
	}
	if rev := project(nil); rev != build.NativeEnvNoneRevision {
		t.Fatalf("no inputs at all = %q, want none", rev)
	}
	merged := readSecret(t, cl, "bex-build", "web-native-env")
	if len(merged.Data) != 0 {
		t.Fatalf("file bytes must not enter the env bundle: %v", merged.Data)
	}
	for _, v := range merged.Annotations {
		if strings.Contains(v, "qa-r59-build-file-marker") {
			t.Fatalf("annotations must not carry file contents: %v", merged.Annotations)
		}
	}
	data := map[string][]byte{"MESSAGE": []byte("A")}
	if nativeEnvInputToken("uid", data, nil, nil) != nativeEnvInputToken("uid", data, nil, map[string][]byte{}) {
		t.Fatal("an App without files must keep its pre-files token")
	}
}

// A direct App edit to the file-source list must produce a fresh native
// artifact — the build reads those files — while Dockerfile and buildpack
// artifacts, whose builds never mount them, stay put.
func TestNativeArtifactIdentityIncludesFilesFromSecrets(t *testing.T) {
	spec := appv1alpha1.AppSpec{
		Repo: "https://github.com/bex-co/bex.git", Branch: "main",
		Type: appv1alpha1.TypeStaticSite, BuildCommand: "cp /etc/secrets/qa-r59.txt index.html",
	}
	linked := spec
	linked.FilesFromSecrets = []string{"evg-a-files"}
	if desiredAppReleaseIdentity(spec).artifact == desiredAppReleaseIdentity(linked).artifact {
		t.Fatal("linking a file group must change the native static artifact identity")
	}
	for _, builder := range []string{"dockerfile", "buildpack"} {
		other := spec
		other.Type, other.BuildCommand, other.Builder = "", "", builder
		otherLinked := other
		otherLinked.FilesFromSecrets = []string{"evg-a-files"}
		if desiredAppReleaseIdentity(other).artifact != desiredAppReleaseIdentity(otherLinked).artifact {
			t.Fatalf("a %s build does not read secret files; its artifact must not change", builder)
		}
	}
}
