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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestRegistryCleanupDeletesThenProvesRepositoryEmpty(t *testing.T) {
	var deleted atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/tags/list"):
			w.Header().Set("Content-Type", "application/json")
			if deleted.Load() {
				_, _ = w.Write([]byte(`{"tags":[]}`))
			} else {
				_, _ = w.Write([]byte(`{"tags":["gen-1","gen-2"]}`))
			}
		case r.Method == http.MethodHead:
			w.Header().Set("Docker-Content-Digest", "sha256:abc")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete:
			deleted.Store(true)
			w.WriteHeader(http.StatusAccepted)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	r := &AppReconciler{Registry: server.URL, HTTPClient: server.Client()}
	app := &appv1alpha1.App{}
	app.Name = "web"
	if done, err := r.deleteRegistryRepo(context.Background(), app); err != nil || done {
		t.Fatalf("delete pass = done %v err %v", done, err)
	}
	if done, err := r.deleteRegistryRepo(context.Background(), app); err != nil || !done {
		t.Fatalf("absence pass = done %v err %v", done, err)
	}
}

func TestRegistryCleanupHonorsCallerCancellationDuringBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	r := &AppReconciler{Registry: server.URL, HTTPClient: server.Client()}
	app := &appv1alpha1.App{}
	app.Name = "web"
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := r.deleteRegistryRepo(ctx, app)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > time.Second {
		t.Fatalf("stalled registry body err=%v elapsed=%s", err, time.Since(started))
	}
}
