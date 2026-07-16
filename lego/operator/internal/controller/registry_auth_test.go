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

func TestPrepareBuildRegistrySecretMergesPullAndPushAuth(t *testing.T) {
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
	var merged corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: buildNS, Name: name}, &merged); err != nil {
		t.Fatal(err)
	}
	var config struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(merged.Data[buildRegistryConfigKey], &config); err != nil {
		t.Fatal(err)
	}
	if config.Auths["private.example"].Auth != "external" || config.Auths["zot.example"].Auth != "push" {
		t.Fatalf("merged auths = %+v", config.Auths)
	}
	if got := string(merged.Data[buildkitRegistryConfigKey]); got != "[registry.\"zot.example\"]\n  http = true\n" {
		t.Fatalf("buildkit registry config = %q", got)
	}

	// The derived Secret is the Job's only registry credential. It is mounted
	// into buildkitd's container filesystem, never declared as a Dockerfile
	// build arg/BuildKit secret or mounted into another container.
	o := build.Options{
		Repo: "https://github.com/octo/app", Name: app.Name, Registry: "zot.example",
		Revision: "gen-1", Namespace: buildNS, Builder: build.BuilderDockerfile, PushSecret: name, RegistryConfig: true,
	}
	spec := build.BuildJob(o, o.ImageRef()).Spec.Template.Spec
	if len(spec.Volumes) != 1 || spec.Volumes[0].Secret == nil || spec.Volumes[0].Secret.SecretName != name {
		t.Fatalf("Job registry volumes = %+v", spec.Volumes)
	}
	if len(spec.Containers) != 1 || spec.Containers[0].Name != "buildkit" || len(spec.Containers[0].VolumeMounts) != 1 {
		t.Fatalf("Job containers = %+v", spec.Containers)
	}
	for _, arg := range spec.Containers[0].Args {
		if strings.Contains(arg, name) || strings.Contains(arg, "config.json") || strings.Contains(arg, "build-arg") || strings.Contains(arg, "registry-auth") || strings.Contains(arg, "--secret") {
			t.Fatalf("registry credential leaked into buildctl arg %q", arg)
		}
	}
	if got := envValue(spec.Containers[0].Env, "BUILDKITD_FLAGS"); !strings.Contains(got, "/docker-config/buildkitd.toml") {
		t.Fatalf("BUILDKITD_FLAGS = %q; want derived registry config", got)
	}
	for _, container := range spec.InitContainers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == "registry-cred" {
				t.Fatalf("registry credential mounted into non-buildkit container %q", container.Name)
			}
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
	if err != nil || name != r.RegistryPushSecret {
		t.Fatalf("unset binding = name %q err %v", name, err)
	}
	var derived corev1.Secret
	err = cl.Get(context.Background(), client.ObjectKey{Namespace: "builds", Name: build.JobName(app.Name, "registry-auth")}, &derived)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("unset binding created derived Secret: %v", err)
	}
}
