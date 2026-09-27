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
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/149: the host a user pastes becomes the host images are matched against.
func TestCanonicalRegistryHost(t *testing.T) {
	ok := map[string]string{
		"ghcr.io":                      "ghcr.io",
		"https://ghcr.io/":             "ghcr.io",
		"HTTP://GHCR.IO//":             "ghcr.io",
		"  ghcr.io  ":                  "ghcr.io",
		"registry.example.com:5000":    "registry.example.com:5000",
		"https://localhost:5000/":      "localhost:5000",
		"index.docker.io":              "docker.io",
		"https://registry-1.docker.io": "docker.io",
		"registry.hub.docker.com":      "docker.io",
		"docker.io":                    "docker.io",
	}
	for in, want := range ok {
		got, err := canonicalRegistryHost(in)
		if err != nil || got != want {
			t.Errorf("canonicalRegistryHost(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"ghcr.io/acme", "https://ghcr.io/v2/", "ghcr.io?x=1", "ftp://ghcr.io", "has space.io", ""} {
		if _, err := canonicalRegistryHost(bad); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("canonicalRegistryHost(%q) err = %v, want a 400", bad, err)
		}
	}
}

func TestCreateStoresTheCanonicalHost(t *testing.T) {
	s, _, _ := newTestService()
	v, err := s.Create(context.Background(), CreateRequest{Host: "https://ghcr.io/", Username: "alice", Secret: "hunter2"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if v.Host != "ghcr.io" || v.Name != "ghcr.io" {
		t.Fatalf("stored host/name = %q/%q, want ghcr.io", v.Host, v.Name)
	}
	if _, err := s.Create(context.Background(), CreateRequest{Host: "ghcr.io/acme", Username: "alice", Secret: "x"}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("a host with a path: err = %v, want a 400", err)
	}
}

// Rows stored before canonicalization ("https://ghcr.io/", "index.docker.io")
// match their images too, by explicit binding and by host auto-match.
func TestLegacyUncanonicalHostsStillMatchTheirImages(t *testing.T) {
	ctx := context.Background()
	s, st, kv := newTestService()
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}}
	for _, host := range []string{"https://ghcr.io/", "index.docker.io"} {
		c, err := st.CreateRegistryCredential(ctx, core.DefaultTenant, "", host, "alice", "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := kv.Put(ctx, secretPath(core.DefaultTenant, c.ID), map[string]string{"password": "hunter2"}); err != nil {
			t.Fatal(err)
		}
	}

	for _, image := range []string{"ghcr.io/acme/private:1", "docker.io/library/nginx:latest", "index.docker.io/library/nginx:latest", "nginx:latest"} {
		if name, ok, err := s.materializePullSecret(ctx, core.DefaultTenant, app, image, nil); err != nil || !ok || name == "" {
			t.Errorf("auto-match %s = name %q ok %v err %v", image, name, ok, err)
		}
	}
	rows, _ := st.ListRegistryCredentials(ctx, core.DefaultTenant)
	for _, c := range rows {
		image := "ghcr.io/acme/private:1"
		if c.Host == "index.docker.io" {
			image = "docker.io/library/nginx:latest"
		}
		id := c.ID
		if _, ok, err := s.materializePullSecret(ctx, core.DefaultTenant, app, image, &id); err != nil || !ok {
			t.Errorf("explicit %s → %s: ok %v err %v", c.Host, image, ok, err)
		}
	}
}
