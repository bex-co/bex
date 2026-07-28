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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestImagePullSecretsGatedOnRegistryHostedImage pins w7/m8: the internal-registry
// imagePullSecret is attached ONLY to registry-hosted images (so a build-from-git
// deploy authenticates its pulls), never to an external/prebuilt image (so the
// registry credential is not sent to a foreign registry), and omitted entirely
// when no pull secret is configured (byte-identical default).
func TestImagePullSecretsGatedOnRegistryHostedImage(t *testing.T) {
	r := &AppReconciler{Registry: "zot.bex-registry.svc:5000", RegistryPullSecret: "bex-registry-pull"}
	app := &appv1alpha1.App{}

	// Registry-hosted image (a build-from-git deploy) → pull secret attached.
	got := r.imagePullSecrets(app, "zot.bex-registry.svc:5000/hello:gen-1")
	if len(got) != 1 || got[0].Name != "bex-registry-pull" {
		t.Errorf("registry-hosted image = %+v; want [bex-registry-pull]", got)
	}

	// External/prebuilt image → no pull secret (cred never sent to a foreign registry).
	if got := r.imagePullSecrets(app, "docker.io/nginx:1.25"); got != nil {
		t.Errorf("external image = %+v; want nil", got)
	}
	// A same-name-but-different-host image is also external (prefix match is exact).
	if got := r.imagePullSecrets(app, "notzot.bex-registry.svc:5000/hello:gen-1"); got != nil {
		t.Errorf("foreign host masquerading = %+v; want nil", got)
	}

	// Unset pull secret → nil (byte-identical default; no imagePullSecret on the pod).
	r2 := &AppReconciler{Registry: "zot.bex-registry.svc:5000"}
	if got := r2.imagePullSecrets(app, "zot.bex-registry.svc:5000/hello:gen-1"); got != nil {
		t.Errorf("unset pull secret = %+v; want nil", got)
	}
}

// TestImagePullSecretsIncludesExternalRegistryCredential pins w2/m14: an App
// whose spec.externalRegistryPullSecret is set gets that Secret referenced
// too — additive to, never replacing, the internal-registry path, and present
// even when no internal pull secret is configured at all (an external/prebuilt
// image with a stored credential, the common case).
func TestImagePullSecretsIncludesExternalRegistryCredential(t *testing.T) {
	r := &AppReconciler{Registry: "zot.bex-registry.svc:5000", RegistryPullSecret: "bex-registry-pull"}
	app := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{ExternalRegistryPullSecret: "web-registry-pull"}}

	// Registry-hosted image + an external credential on the App → both attached.
	got := r.imagePullSecrets(app, "zot.bex-registry.svc:5000/hello:gen-1")
	if len(got) != 2 || got[0].Name != "bex-registry-pull" || got[1].Name != "web-registry-pull" {
		t.Errorf("both sources = %+v; want [bex-registry-pull web-registry-pull]", got)
	}

	// External/prebuilt image (no internal pull secret attaches) + a credential on
	// the App → only the external one, the common case this milestone adds.
	got = r.imagePullSecrets(app, "ghcr.io/acme/private-app:1.0")
	if len(got) != 1 || got[0].Name != "web-registry-pull" {
		t.Errorf("external-only image = %+v; want [web-registry-pull]", got)
	}

	// No credential on the App at all → nil, byte-identical to before this milestone.
	if got := r.imagePullSecrets(&appv1alpha1.App{}, "ghcr.io/acme/private-app:1.0"); got != nil {
		t.Errorf("no external credential = %+v; want nil", got)
	}
}

func TestPrepareBuildRegistrySecretSeparatesPullAndPushAuth(t *testing.T) {
	const (
		appNS    = "apps"
		buildNS  = "builds"
		external = "web-registry-pull"
		push     = "bex-registry-push"
	)
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	externalSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: external, Namespace: appNS},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(
			`{"auths":{"private.example":{"auth":"external"},"zot.example":{"auth":"wrong"}}}`,
		)},
	}
	pushSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: push, Namespace: buildNS},
		Data: map[string][]byte{buildRegistryConfigKey: []byte(
			`{"auths":{"zot.example":{"auth":"push"}}}`,
		)},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(externalSecret, pushSecret).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl, Registry: "zot.example", RegistryPushSecret: push}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: appNS},
		Spec:       appv1alpha1.AppSpec{ExternalRegistryPullSecret: external},
	}

	name, err := r.prepareBuildRegistrySecret(context.Background(), app, buildNS, build.BuilderDockerfile)
	if err != nil {
		t.Fatal(err)
	}
	if name != build.JobName(app.Name, "registry-auth") {
		t.Fatalf("merged Secret name = %q", name)
	}
	var pull corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: buildNS, Name: name}, &pull); err != nil {
		t.Fatal(err)
	}
	var config struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(pull.Data[buildRegistryConfigKey], &config); err != nil {
		t.Fatal(err)
	}
	if config.Auths["private.example"].Auth != "external" || config.Auths["zot.example"].Auth != "wrong" {
		t.Fatalf("pull-only auths = %+v", config.Auths)
	}
	if got := string(pull.Data[buildkitRegistryConfigKey]); got != "[registry.\"zot.example\"]\n  http = true\n" {
		t.Fatalf("buildkit registry config = %q", got)
	}

	// The derived pull Secret enters BuildKit; the platform push Secret enters
	// only the serial pusher. Neither phase gets the other's credential.
	o := build.Options{
		Repo: "https://github.com/octo/app", Name: app.Name, Registry: "zot.example",
		Revision: "gen-1", Namespace: buildNS, Builder: build.BuilderDockerfile,
		PullSecret: name, PushSecret: push, RegistryConfig: true,
	}
	spec := build.BuildJob(o, o.ImageRef()).Spec.Template.Spec
	bk := findContainer(spec.InitContainers, "buildkit")
	pusher := findContainer(spec.Containers, "push")
	if bk == nil || pusher == nil {
		t.Fatalf("Job phases = init %+v main %+v", spec.InitContainers, spec.Containers)
	}
	for _, arg := range bk.Args {
		if strings.Contains(arg, name) || strings.Contains(arg, "config.json") || strings.Contains(arg, "build-arg") || strings.Contains(arg, "registry-auth") || strings.Contains(arg, "--secret") {
			t.Fatalf("registry credential leaked into buildctl arg %q", arg)
		}
	}
	if got := envValue(bk.Env, "BUILDKITD_FLAGS"); !strings.Contains(got, "/docker-config/buildkitd.toml") {
		t.Fatalf("BUILDKITD_FLAGS = %q; want derived registry config", got)
	}
	for _, mount := range bk.VolumeMounts {
		if mount.Name == "push-registry-cred" {
			t.Fatal("push credential leaked into BuildKit")
		}
	}
	for _, mount := range pusher.VolumeMounts {
		if mount.Name == "pull-registry-cred" {
			t.Fatal("private-base pull credential leaked into pusher")
		}
	}
}

func TestPrepareBuildRegistrySecretUnsetIsNoOp(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl, RegistryPushSecret: "bex-registry-push"}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "apps"}}

	name, err := r.prepareBuildRegistrySecret(context.Background(), app, "builds", build.BuilderDockerfile)
	if err != nil || name != "" {
		t.Fatalf("unset binding = name %q err %v", name, err)
	}
	var derived corev1.Secret
	err = cl.Get(context.Background(), client.ObjectKey{Namespace: "builds", Name: build.JobName(app.Name, "registry-auth")}, &derived)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("unset binding created derived Secret: %v", err)
	}
}

func TestCopyBuildRegistryCredentialAddsSkopeoFilename(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	const config = `{"auths":{"zot.example":{"username":"app-web","password":"redacted"}}}`
	src := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "reg-pull-web", Namespace: "apps"},
		Type:       corev1.SecretTypeDockerConfigJson,
		Data:       map[string][]byte{corev1.DockerConfigJsonKey: []byte(config)},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(src).Build()
	r := &AppReconciler{Client: cl, BuildClient: cl}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "apps", UID: "uid-web"}}
	if err := r.copyBuildRegistryCredential(context.Background(), app, "apps", "bex-build", src.Name); err != nil {
		t.Fatal(err)
	}
	var got corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "bex-build", Name: src.Name}, &got); err != nil {
		t.Fatal(err)
	}
	if string(got.Data[corev1.DockerConfigJsonKey]) != config || string(got.Data[buildRegistryConfigKey]) != config {
		t.Fatal("mirror must retain .dockerconfigjson and add byte-identical config.json")
	}
	if got.Type != corev1.SecretTypeDockerConfigJson || got.Labels[labelApp] != "web" || got.Labels["app.bex.co/component"] != buildRegistryComponent || got.Labels["app.bex.co/app-uid"] != "uid-web" {
		t.Fatalf("mirrored metadata = type %s labels %v", got.Type, got.Labels)
	}
}

func findContainer(containers []corev1.Container, name string) *corev1.Container {
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i]
		}
	}
	return nil
}
