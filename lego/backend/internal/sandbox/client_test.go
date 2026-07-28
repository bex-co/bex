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

package sandbox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCreateSendsTenantKeyAndMapsStatus(t *testing.T) {
	var gotKey, gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get(tenantKeyHeader)
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"os-abc","status":{"state":"Creating"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	id, status, err := c.Create(context.Background(), "wskey-123", "img:1", []string{"sh"}, "500m", "512Mi", map[string]string{"A": "b"})
	if err != nil {
		t.Fatal(err)
	}
	if id != "os-abc" {
		t.Errorf("id = %q, want os-abc", id)
	}
	if status != StatusCreating {
		t.Errorf("status = %q, want creating", status)
	}
	if gotKey != "wskey-123" {
		t.Errorf("tenant key header = %q, want wskey-123", gotKey)
	}
	if gotMethod != http.MethodPost || gotPath != "/sandboxes" {
		t.Errorf("request = %s %s, want POST /sandboxes", gotMethod, gotPath)
	}
}

func TestClientGetMapsRunningAndNotFound(t *testing.T) {
	var notFound bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if notFound {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"id":"os-abc","status":{"state":"Running"}}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)

	st, err := c.Get(context.Background(), "k", "os-abc")
	if err != nil || st != StatusRunning {
		t.Fatalf("get running: st=%q err=%v", st, err)
	}
	notFound = true
	st, err = c.Get(context.Background(), "k", "os-abc")
	if err != nil || st != StatusTerminated {
		t.Fatalf("get 404 should map to terminated: st=%q err=%v", st, err)
	}
}

func TestClientListHandlesArrayAndWrapped(t *testing.T) {
	bodies := []string{`[{"id":"a","status":{"state":"Running"}}]`, `{"items":[{"id":"b","status":{"state":"Paused"}}]}`}
	idx := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(bodies[idx]))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)

	for i := range bodies {
		idx = i
		got, err := c.List(context.Background(), "k")
		if err != nil {
			t.Fatalf("list body %d: %v", i, err)
		}
		if len(got) != 1 {
			t.Fatalf("list body %d: got %d sandboxes, want 1", i, len(got))
		}
	}
}

func TestClientSuspendResumeTerminate(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	ctx := context.Background()

	if err := c.Suspend(ctx, "k", "id1"); err != nil {
		t.Fatal(err)
	}
	if err := c.Resume(ctx, "k", "id1"); err != nil {
		t.Fatal(err)
	}
	if err := c.Terminate(ctx, "k", "id1"); err != nil {
		t.Fatal(err)
	}
	want := []string{"POST /sandboxes/id1/pause", "POST /sandboxes/id1/resume", "DELETE /sandboxes/id1"}
	for i, w := range want {
		if paths[i] != w {
			t.Errorf("call %d = %q, want %q", i, paths[i], w)
		}
	}
}

func TestClientCreateErrorsOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`boom`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	if _, _, err := c.Create(context.Background(), "k", "img", []string{"sh"}, "500m", "512Mi", nil); err == nil {
		t.Fatal("expected error on 500")
	}
}
