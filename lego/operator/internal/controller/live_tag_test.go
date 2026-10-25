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
	"sync"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakeLiveRegistry serves one manifest by digest and records every tag PUT.
type fakeLiveRegistry struct {
	mu       sync.Mutex
	repo     string
	digest   string
	failPuts bool
	puts     []string // "<tag> <content-type> <body>"
}

func (f *fakeLiveRegistry) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	prefix := "/v2/" + f.repo + "/manifests/"
	ref := strings.TrimPrefix(r.URL.Path, prefix)
	switch {
	case r.Method == http.MethodGet && r.URL.Path == prefix+f.digest:
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
		_, _ = w.Write([]byte(`{"schemaVersion":2}`))
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, prefix):
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.failPuts {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		body := make([]byte, 64)
		n, _ := r.Body.Read(body)
		f.puts = append(f.puts, ref+" "+r.Header.Get("Content-Type")+" "+string(body[:n]))
		w.WriteHeader(http.StatusCreated)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func liveTagApp(image string) *appv1alpha1.App {
	app := &appv1alpha1.App{}
	app.Name, app.Namespace = "web", "default"
	app.Status.Image = image
	app.Status.ImageDigest = &appv1alpha1.ImageDigestStatus{Generation: 7, Digest: "sha256:" + strings.Repeat("a", 64)}
	return app
}

// The serving release's manifest is pushed as live-<generation> exactly once:
// the steady-state reconciles that follow change nothing.
func TestProtectServingImageTagsTheLiveDigestOnce(t *testing.T) {
	reg := &fakeLiveRegistry{digest: "sha256:" + strings.Repeat("a", 64)}
	srv := httptest.NewServer(reg)
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	app := liveTagApp(host + "/web:gen-7@" + reg.digest)
	reg.repo = appIdentity(app).Repo()
	r := &AppReconciler{Registry: host}

	for range 3 {
		r.protectServingImage(context.Background(), app)
	}
	if len(reg.puts) != 1 || reg.puts[0] != `live-7 application/vnd.oci.image.manifest.v1+json {"schemaVersion":2}` {
		t.Fatalf("tag pushes = %q, want one live-7 PUT of the unchanged manifest", reg.puts)
	}
	if app.Status.ProtectedImageTag != "live-7" {
		t.Errorf("protectedImageTag = %q, want live-7", app.Status.ProtectedImageTag)
	}
}

// A failed push is retried on the next reconcile instead of being recorded.
func TestProtectServingImageRetriesAFailedTag(t *testing.T) {
	reg := &fakeLiveRegistry{digest: "sha256:" + strings.Repeat("a", 64), failPuts: true}
	srv := httptest.NewServer(reg)
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	app := liveTagApp(host + "/web:gen-7@" + reg.digest)
	reg.repo = appIdentity(app).Repo()
	r := &AppReconciler{Registry: host}

	r.protectServingImage(context.Background(), app)
	if app.Status.ProtectedImageTag != "" {
		t.Fatalf("a failed tag was recorded as protected")
	}
	reg.failPuts = false
	r.protectServingImage(context.Background(), app)
	if app.Status.ProtectedImageTag != "live-7" || len(reg.puts) != 1 {
		t.Errorf("retry: protected=%q puts=%q, want the tag on the second reconcile", app.Status.ProtectedImageTag, reg.puts)
	}
}

// An image outside the operator's registry is not subject to its retention
// and is never sent the operator's credential.
func TestProtectServingImageLeavesExternalImagesAlone(t *testing.T) {
	reg := &fakeLiveRegistry{digest: "sha256:" + strings.Repeat("a", 64)}
	srv := httptest.NewServer(reg)
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	app := liveTagApp("docker.io/library/nginx@" + reg.digest)
	r := &AppReconciler{Registry: host}

	r.protectServingImage(context.Background(), app)
	if len(reg.puts) != 0 || app.Status.ProtectedImageTag != "" {
		t.Errorf("external image was tagged: puts=%q protected=%q", reg.puts, app.Status.ProtectedImageTag)
	}
}

// A rollback re-promotes an older image under a new generation; it gets a live
// tag of its own rather than relying on the old one staying in the window.
func TestProtectServingImageRetagsARolledBackRelease(t *testing.T) {
	reg := &fakeLiveRegistry{digest: "sha256:" + strings.Repeat("a", 64)}
	srv := httptest.NewServer(reg)
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	app := liveTagApp(host + "/web:gen-7@" + reg.digest)
	reg.repo = appIdentity(app).Repo()
	r := &AppReconciler{Registry: host}

	r.protectServingImage(context.Background(), app)
	app.Status.ImageDigest.Generation = 9 // the same digest, rolled back to
	r.protectServingImage(context.Background(), app)
	if len(reg.puts) != 2 || !strings.HasPrefix(reg.puts[1], "live-9 ") || app.Status.ProtectedImageTag != "live-9" {
		t.Errorf("puts=%q protected=%q, want a fresh live-9 tag for the re-promoted release", reg.puts, app.Status.ProtectedImageTag)
	}
}

// Another repository on the same registry is not this App's artifact: tagging
// it in the App's repository would 404 and retry forever.
func TestProtectServingImageLeavesOtherRepositoriesAlone(t *testing.T) {
	reg := &fakeLiveRegistry{digest: "sha256:" + strings.Repeat("a", 64)}
	srv := httptest.NewServer(reg)
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "http://")
	app := liveTagApp(host + "/someone-else:gen-1@" + reg.digest)
	reg.repo = appIdentity(app).Repo()
	r := &AppReconciler{Registry: host}

	r.protectServingImage(context.Background(), app)
	if len(reg.puts) != 0 || app.Status.ProtectedImageTag != "" {
		t.Errorf("another repository's image was tagged: puts=%q", reg.puts)
	}
}
