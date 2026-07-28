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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

type fakeWorkspace map[string]string

func (f fakeWorkspace) Tenant(_ context.Context, id core.Identity) (string, bool) {
	tid, ok := f[id.Subject]
	return tid, ok
}

func (f fakeWorkspace) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	return f[id.Subject] == tenantID, nil
}

// denyChecker refuses every relation (for the authz-refusal test).
type denyChecker struct{}

func (denyChecker) Check(context.Context, string, string, string) (bool, error) { return false, nil }

func stubServer(t *testing.T, handler http.HandlerFunc) *Service {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Service{
		Base:      &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client:    NewClient(srv.URL),
		Templates: map[string]Template{"node": {Image: "node:20", Entrypoint: []string{"sh"}, CPU: "500m", Memory: "512Mi"}},
	}
}

func callerCtx() context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})
}

func TestCreateUsesTemplateImageAndEchoesPlan(t *testing.T) {
	var gotBody string
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"os-1","status":{"state":"Creating"}}`))
	})
	sb, err := svc.Create(callerCtx(), CreateRequest{Template: "node", Plan: PlanStandard})
	if err != nil {
		t.Fatal(err)
	}
	if sb.ID != "os-1" || sb.Status != StatusCreating || sb.Plan != PlanStandard {
		t.Errorf("sandbox = %+v", sb)
	}
	if sb.Image != "node:20" {
		t.Errorf("image = %q, want node:20 (from template)", sb.Image)
	}
	if sb.Owner != "id-a" {
		t.Errorf("owner = %q, want id-a", sb.Owner)
	}
	if want := `"node:20"`; !contains(gotBody, want) {
		t.Errorf("create body %q missing template image %s", gotBody, want)
	}
}

func TestCreateRejectsUnknownTemplateAndPlan(t *testing.T) {
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) { t.Fatal("must not reach server") })
	if _, err := svc.Create(callerCtx(), CreateRequest{Template: "ghost"}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("unknown template err = %v, want ErrBadRequest", err)
	}
	if _, err := svc.Create(callerCtx(), CreateRequest{Template: "node", Plan: "mega"}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("unknown plan err = %v, want ErrBadRequest", err)
	}
}

func TestVerbsReturnUnavailableWhenClientNil(t *testing.T) {
	svc := &Service{Base: &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}}}
	ctx := callerCtx()
	if _, err := svc.Create(ctx, CreateRequest{Template: "node"}); !errors.Is(err, core.ErrSandboxesUnavailable) {
		t.Errorf("Create err = %v, want ErrSandboxesUnavailable", err)
	}
	if _, err := svc.List(ctx); !errors.Is(err, core.ErrSandboxesUnavailable) {
		t.Errorf("List err = %v, want ErrSandboxesUnavailable", err)
	}
	if err := svc.Terminate(ctx, "x"); !errors.Is(err, core.ErrSandboxesUnavailable) {
		t.Errorf("Terminate err = %v, want ErrSandboxesUnavailable", err)
	}
}

func TestAuthzRefusalBlocksBeforeServerCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("authz-refused verb must not reach the server")
	}))
	t.Cleanup(srv.Close)
	svc := &Service{
		Base:      &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}, Authz: denyChecker{}},
		Client:    NewClient(srv.URL),
		Templates: map[string]Template{"node": {Image: "node:20", Entrypoint: []string{"sh"}, CPU: "500m", Memory: "512Mi"}},
	}
	if _, err := svc.Create(callerCtx(), CreateRequest{Template: "node"}); err == nil {
		t.Fatal("expected authz refusal")
	}
}

func TestListAndLifecycleScopedByWorkspaceKey(t *testing.T) {
	var keys []string
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		keys = append(keys, r.Header.Get(tenantKeyHeader))
		_, _ = w.Write([]byte(`[{"id":"os-1","status":{"state":"Running"}}]`))
	})
	svc.Keys = staticKey("ws-key-tea-a")
	got, err := svc.List(callerCtx())
	if err != nil || len(got) != 1 || got[0].Status != StatusRunning {
		t.Fatalf("list: got %+v err %v", got, err)
	}
	if len(keys) == 0 || keys[0] != "ws-key-tea-a" {
		t.Errorf("workspace key not sent: %v", keys)
	}
}

type staticKey string

func (k staticKey) WorkspaceKey(context.Context, string) (string, error) { return string(k), nil }

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
