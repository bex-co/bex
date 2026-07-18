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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestPinBuiltImage pins the w9/013 digest-pinning seam: a freshly built
// mutable-tag reference gains the digest the registry serves for it, an
// already-pinned (kpack) or foreign-registry reference passes through
// untouched, and a failed resolution falls back to the tag (pre-w9/013
// behavior) instead of failing the deploy.
func TestPinBuiltImage(t *testing.T) {
	const digest = "sha256:341e43e9d1e4a1d8ad2b8a29bd28e6be3e0d33a9a26e6bd1e1f7a5b2c3d4e5f6"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/web/manifests/gen-1" {
			w.Header().Set("Docker-Content-Digest", digest)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")

	app := &appv1alpha1.App{}
	app.Name = "web"
	app.Namespace = "default"
	r := &AppReconciler{Registry: host}

	if got := r.pinBuiltImage(context.Background(), app, host+"/web:gen-1"); got != host+"/web:gen-1@"+digest {
		t.Errorf("pinned = %q, want tag@digest", got)
	}
	// kpack references arrive digest-pinned already — pass through.
	pinned := host + "/web:gen-1@" + digest
	if got := r.pinBuiltImage(context.Background(), app, pinned); got != pinned {
		t.Errorf("already-pinned = %q, want unchanged", got)
	}
	// A reference outside the platform registry is never touched.
	if got := r.pinBuiltImage(context.Background(), app, "docker.io/library/nginx:latest"); got != "docker.io/library/nginx:latest" {
		t.Errorf("foreign ref = %q, want unchanged", got)
	}
	// Resolution failure (unknown tag) falls back to the mutable tag.
	if got := r.pinBuiltImage(context.Background(), app, host+"/web:gen-9"); got != host+"/web:gen-9" {
		t.Errorf("failed resolve = %q, want the tag unchanged", got)
	}
}
