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
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestClientCreateSendsTenantKeyAndMapsStatus(t *testing.T) {
	var gotKey, gotPath, gotMethod string
	var gotBody createRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get(tenantKeyHeader)
		gotPath = r.URL.Path
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode create body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"os-abc","status":{"state":"Creating"}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	sandbox, err := c.Create(context.Background(), "wskey-123", "img:1", []string{"sh"}, osResourceLimits{CPU: "500m", Memory: "512Mi"}, 600, map[string]string{"A": "b"}, map[string]string{metadataOwner: "id-a"})
	if err != nil {
		t.Fatal(err)
	}
	if sandbox.ID != "os-abc" {
		t.Errorf("id = %q, want os-abc", sandbox.ID)
	}
	if mapOpenSandboxStatus(sandbox.Status.State) != StatusCreating {
		t.Errorf("status = %q, want creating", sandbox.Status.State)
	}
	if gotKey != "wskey-123" {
		t.Errorf("tenant key header = %q, want wskey-123", gotKey)
	}
	if gotMethod != http.MethodPost || gotPath != "/sandboxes" {
		t.Errorf("request = %s %s, want POST /sandboxes", gotMethod, gotPath)
	}
	if gotBody.Timeout != 600 || gotBody.Env["A"] != "b" || gotBody.Metadata[metadataOwner] != "id-a" {
		t.Errorf("create body = %+v, want timeout/env/security metadata", gotBody)
	}
	// The limit is sent verbatim; the request is the overcommit fraction so the
	// pod schedules on a fraction of its cap (see requestFor).
	if gotBody.ResourceLimits.CPU != "500m" || gotBody.ResourceLimits.Memory != "512Mi" {
		t.Errorf("limits = %+v, want 500m/512Mi", gotBody.ResourceLimits)
	}
	if gotBody.ResourceLimits.EphemeralStorage != "" || gotBody.ResourceRequests.EphemeralStorage != "" {
		t.Errorf("ephemeral storage = %q/%q, want none (the LimitRange default)", gotBody.ResourceLimits.EphemeralStorage, gotBody.ResourceRequests.EphemeralStorage)
	}
	if gotBody.ResourceRequests.CPU != "125m" || gotBody.ResourceRequests.Memory != "128Mi" {
		t.Errorf("requests = %+v, want overcommit 125m/128Mi", gotBody.ResourceRequests)
	}
}

func TestRequestForQuartersLimitWithFloors(t *testing.T) {
	cases := []struct {
		cpu, mem      string
		wantCPU, wMem string
	}{
		// Agent template: a quarter lets ~4x as many pods schedule on a node.
		{"2", "4Gi", "500m", "1Gi"},
		// Base template: a quarter of a small limit, above the floors.
		{"500m", "512Mi", "125m", "128Mi"},
		// Floors keep a tiny limit's request non-trivial (never below 50m/128Mi).
		{"100m", "256Mi", "50m", "128Mi"},
		// An unparseable value falls through to the limit (pre-overcommit behaviour).
		{"garbage", "4Gi", "garbage", "1Gi"},
	}
	for _, tc := range cases {
		gotCPU, gotMem := requestFor(tc.cpu, tc.mem)
		if gotCPU != tc.wantCPU || gotMem != tc.wMem {
			t.Errorf("requestFor(%q,%q) = %q/%q, want %q/%q", tc.cpu, tc.mem, gotCPU, gotMem, tc.wantCPU, tc.wMem)
		}
	}
}

// An ephemeral storage limit requests a quarter, never below the
// LimitRange's 1Gi default request; no limit asks for nothing.
func TestEphemeralRequestForQuartersLimitWithFloor(t *testing.T) {
	for limit, want := range map[string]string{"30Gi": "7680Mi", "2Gi": "1Gi", "": "", "garbage": ""} {
		if got := ephemeralRequestFor(limit); got != want {
			t.Errorf("ephemeralRequestFor(%q) = %q, want %q", limit, got, want)
		}
	}
}

// The ephemeral storage limit and its request travel in the create body.
func TestClientCreateSendsEphemeralStorage(t *testing.T) {
	var gotBody createRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"os-abc","status":{"state":"Creating"}}`))
	}))
	defer srv.Close()
	limits := osResourceLimits{CPU: "4", Memory: "8Gi", EphemeralStorage: "30Gi"}
	if _, err := NewClient(srv.URL).Create(context.Background(), "k", "img", []string{"sh"}, limits, 0, nil, nil); err != nil {
		t.Fatal(err)
	}
	if gotBody.ResourceLimits.EphemeralStorage != "30Gi" || gotBody.ResourceRequests.EphemeralStorage != "7680Mi" {
		t.Errorf("ephemeral storage limit/request = %q/%q, want 30Gi/7680Mi", gotBody.ResourceLimits.EphemeralStorage, gotBody.ResourceRequests.EphemeralStorage)
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

	sandbox, err := c.Get(context.Background(), "k", "os-abc")
	if err != nil || mapOpenSandboxStatus(sandbox.Status.State) != StatusRunning {
		t.Fatalf("get running: sandbox=%+v err=%v", sandbox, err)
	}
	notFound = true
	_, err = c.Get(context.Background(), "k", "os-abc")
	if !errors.Is(err, errOpenSandboxNotFound) {
		t.Fatalf("get 404 error = %v, want errOpenSandboxNotFound", err)
	}
}

func TestClientEscapesSandboxIDAndNeverForwardsTenantKeyOnRedirect(t *testing.T) {
	var redirected bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected = true
	}))
	defer target.Close()

	var escapedPath, rawQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		escapedPath, rawQuery = r.URL.EscapedPath(), r.URL.RawQuery
		http.Redirect(w, r, target.URL+"/capture", http.StatusFound)
	}))
	defer upstream.Close()

	_, err := NewClient(upstream.URL).Get(context.Background(), "workspace-capability", "../other?admin=1")
	if err == nil {
		t.Fatal("redirect response must not be treated as a sandbox")
	}
	if redirected {
		t.Fatal("OpenSandbox client followed a redirect carrying the tenant key")
	}
	if rawQuery != "" || escapedPath != "/sandboxes/..%2Fother%3Fadmin=1" {
		t.Fatalf("sandbox id contributed URL syntax: escapedPath=%q rawQuery=%q", escapedPath, rawQuery)
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

// TestClientLifetimeCalls pins the OpenSandbox 0.2.2 wire shape of the
// metadata merge patch and renew-expiration (api/lifecycle.py,
// RenewSandboxExpirationRequest.expiresAt), and the 409 a sandbox without an
// expiry answers.
func TestClientLifetimeCalls(t *testing.T) {
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		calls = append(calls, r.Method+" "+r.URL.Path+" "+r.Header.Get(tenantKeyHeader)+" "+string(body))
		if strings.HasPrefix(r.URL.Path, "/sandboxes/no-expiry/") {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	ctx := context.Background()
	at := time.Date(2026, 12, 7, 12, 0, 0, 0, time.FixedZone("x", 3600))
	if err := c.PatchMetadata(ctx, "k", "id1", map[string]string{metadataResumedAt: "1"}); err != nil {
		t.Fatal(err)
	}
	if err := c.RenewExpiration(ctx, "k", "id1", at); err != nil {
		t.Fatal(err)
	}
	if err := c.RenewExpiration(ctx, "k", "no-expiry", at); !errors.Is(err, errOpenSandboxConflict) {
		t.Fatalf("renew without an expiry = %v, want errOpenSandboxConflict", err)
	}
	want := []string{
		`PATCH /sandboxes/id1/metadata k {"bex.co/resumed-at":"1"}`,
		`POST /sandboxes/id1/renew-expiration k {"expiresAt":"2026-12-07T11:00:00Z"}`,
	}
	if !slices.Equal(calls[:2], want) {
		t.Fatalf("calls = %q, want %q", calls[:2], want)
	}
}

func TestClientCreateErrorsOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`boom`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)
	if _, err := c.Create(context.Background(), "k", "img", []string{"sh"}, osResourceLimits{CPU: "500m", Memory: "512Mi"}, 0, nil, nil); err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestClientCreateMapsPodReadyTimeoutSignature(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGatewayTimeout)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	c := NewClient(srv.URL)

	// The server's own error code for an elapsed synchronous pod-ready wait.
	body = `{"code":"KUBERNETES::POD_READY_TIMEOUT","message":"sandbox pod not ready within 300s"}`
	if _, err := c.Create(context.Background(), "k", "img", []string{"sh"}, osResourceLimits{CPU: "500m", Memory: "512Mi"}, 0, nil, nil); !errors.Is(err, errPodReadyTimeout) {
		t.Fatalf("timeout-signature create error = %v, want errPodReadyTimeout", err)
	}

	// Narrow mapping: any other failure — even the same status — keeps the
	// generic mapping and is never reported as a capacity refusal.
	body = `{"code":"KUBERNETES::INTERNAL","message":"boom"}`
	if _, err := c.Create(context.Background(), "k", "img", []string{"sh"}, osResourceLimits{CPU: "500m", Memory: "512Mi"}, 0, nil, nil); err == nil || errors.Is(err, errPodReadyTimeout) {
		t.Fatalf("non-timeout create error = %v, want a generic error without errPodReadyTimeout", err)
	}
}
