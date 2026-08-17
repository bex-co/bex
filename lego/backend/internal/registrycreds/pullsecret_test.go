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

package registrycreds

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func fakeK8sClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func TestRegistryHost(t *testing.T) {
	cases := map[string]string{
		"ghcr.io/acme/private-app:1.0":     "ghcr.io",
		"docker.io/library/nginx:1.25":     "docker.io",
		"localhost:5000/hello:gen-1":       "localhost:5000",
		"registry.gitlab.com/acme/app:1.0": "registry.gitlab.com",
		"nginx:latest":                     "docker.io", // Docker Hub, no "/" at all
		"acme/private-app:1.0":             "docker.io", // Docker Hub, implicit host
		"library/nginx":                    "docker.io",
	}
	for image, want := range cases {
		if got := registryHost(image); got != want {
			t.Errorf("registryHost(%q) = %q, want %q", image, got, want)
		}
	}
}

func TestMaterializePullSecretNoCredentialIsOkFalseNoError(t *testing.T) {
	s := &Service{
		Base:   &core.Base{Client: fakeK8sClient(), Namespace: "default"},
		Store:  newFakeStore(),
		Secret: newFakeSecretKV(),
	}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}}

	name, ok, err := s.materializePullSecret(context.Background(), core.DefaultTenant, app, "ghcr.io/acme/private-app:1.0", nil)
	if err != nil || ok || name != "" {
		t.Fatalf("no credential = name=%q ok=%v err=%v, want ok=false nil-err", name, ok, err)
	}
}

func TestMaterializePullSecretUpsertsDockerConfigJSON(t *testing.T) {
	cl := fakeK8sClient()
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}}
	s := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, Store: newFakeStore(), Secret: newFakeSecretKV()}
	ctx := context.Background()

	if _, err := s.Create(ctx, CreateRequest{Host: "ghcr.io", Username: "alice", Secret: "hunter2"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	name, ok, err := s.materializePullSecret(ctx, core.DefaultTenant, app, "ghcr.io/acme/private-app:1.0", nil)
	if err != nil || !ok || name != "web-registry-pull" {
		t.Fatalf("materializePullSecret = name=%q ok=%v err=%v", name, ok, err)
	}

	var sec corev1.Secret
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: name}, &sec); err != nil {
		t.Fatalf("secret not created: %v", err)
	}
	if sec.Type != corev1.SecretTypeDockerConfigJson {
		t.Errorf("secret type = %s, want %s", sec.Type, corev1.SecretTypeDockerConfigJson)
	}
	// Round-11 #1: the workspace's external-registry credential is operational
	// — the label makes the operator's App reconcile refuse envFrom/volume/env
	// references to it (imagePullSecret resolution ignores labels).
	if sec.Labels[appv1alpha1.LabelProtectedFromTenantMount] != appv1alpha1.ProtectedFromTenantMount {
		t.Errorf("pull secret missing protected-from-tenant-mount label: %+v", sec.Labels)
	}
	var doc struct {
		Auths map[string]struct {
			Username, Password, Auth string
		} `json:"auths"`
	}
	if err := json.Unmarshal(sec.Data[corev1.DockerConfigJsonKey], &doc); err != nil {
		t.Fatalf("dockerconfigjson not valid JSON: %v", err)
	}
	entry, ok := doc.Auths["ghcr.io"]
	if !ok || entry.Username != "alice" || entry.Password != "hunter2" {
		t.Fatalf("dockerconfigjson auths[ghcr.io] = %+v, want alice/hunter2", entry)
	}
	wantAuth := base64.StdEncoding.EncodeToString([]byte("alice:hunter2"))
	if entry.Auth != wantAuth {
		t.Errorf("auth field = %q, want base64(alice:hunter2) = %q", entry.Auth, wantAuth)
	}

	// Deliberately no owner reference (this can run before the App has a UID,
	// during its own Create call) — apps.Service.Delete cleans it up explicitly
	// by name instead, mirroring the clone-secret Secret's identical constraint.
	if owners := sec.GetOwnerReferences(); len(owners) != 0 {
		t.Errorf("owner references = %+v, want none", owners)
	}

	// A second call with the same credential is idempotent (upsert, not a
	// duplicate object), and re-materializing after a username/secret rotation
	// picks up the new value.
	if _, _, err := s.materializePullSecret(ctx, core.DefaultTenant, app, "ghcr.io/acme/private-app:1.0", nil); err != nil {
		t.Fatalf("second materialize: %v", err)
	}
}

func TestMaterializePullSecretMissingOpenBaoValueErrors(t *testing.T) {
	st := newFakeStore()
	kv := newFakeSecretKV()
	s := &Service{Base: &core.Base{Client: fakeK8sClient(), Namespace: "default"}, Store: st, Secret: kv}
	ctx := context.Background()

	// A metadata row with no corresponding OpenBao secret (e.g. deleted
	// out-of-band) must refuse rather than materialize an empty-password Secret.
	if _, err := st.CreateRegistryCredential(ctx, core.DefaultTenant, "", "ghcr.io", "alice", "", nil); err != nil {
		t.Fatalf("seed row: %v", err)
	}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}}

	if _, _, err := s.materializePullSecret(ctx, core.DefaultTenant, app, "ghcr.io/acme/private-app:1.0", nil); err == nil {
		t.Fatal("want an error when the credential's OpenBao secret is missing")
	}
}

func TestMaterializePullSecretUnconfiguredIsOkFalseNoError(t *testing.T) {
	s := &Service{Base: &core.Base{Client: fakeK8sClient(), Namespace: "default"}} // Store/Secret nil
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}}

	name, ok, err := s.materializePullSecret(context.Background(), core.DefaultTenant, app, "ghcr.io/acme/private-app:1.0", nil)
	if err != nil || ok || name != "" {
		t.Fatalf("unconfigured = name=%q ok=%v err=%v, want ok=false nil-err", name, ok, err)
	}
}

func TestMaterializePullSecretExplicitIDClassifiesAndValidates(t *testing.T) {
	ctx := context.Background()
	st := newFakeStore()
	kv := newFakeSecretKV()
	s := &Service{Base: &core.Base{Client: fakeK8sClient(), Namespace: "default"}, Store: st, Secret: kv}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}}

	valid, err := st.CreateRegistryCredential(ctx, core.DefaultTenant, "", "ghcr.io", "alice", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := kv.Put(ctx, secretPath(core.DefaultTenant, valid.ID), map[string]string{"password": "hunter2"}); err != nil {
		t.Fatal(err)
	}
	if name, ok, err := s.materializePullSecret(ctx, core.DefaultTenant, app, "ghcr.io/acme/private:1", &valid.ID); err != nil || !ok || name == "" {
		t.Fatalf("valid explicit id = name %q ok %v err %v", name, ok, err)
	}
	if name, ok, err := s.materializePullSecret(ctx, core.DefaultTenant, app, "", &valid.ID); err != nil || !ok || name == "" {
		t.Fatalf("Docker-build explicit id = name %q ok %v err %v", name, ok, err)
	}

	foreign, err := st.CreateRegistryCredential(ctx, "tea-other", "", "ghcr.io", "mallory", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.materializePullSecret(ctx, core.DefaultTenant, app, "ghcr.io/acme/private:1", &foreign.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("foreign id error = %v, want forbidden", err)
	}
	missing := "rgc-missing"
	if _, _, err := s.materializePullSecret(ctx, core.DefaultTenant, app, "ghcr.io/acme/private:1", &missing); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("missing id error = %v, want not found", err)
	}
	if _, _, err := s.materializePullSecret(ctx, core.DefaultTenant, app, "docker.io/library/nginx:1", &valid.ID); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("host mismatch error = %v, want bad request", err)
	}
	empty := ""
	if name, ok, err := s.materializePullSecret(ctx, core.DefaultTenant, app, "ghcr.io/acme/private:1", &empty); err != nil || ok || name != "" {
		t.Fatalf("explicit clear = name %q ok %v err %v", name, ok, err)
	}
}
