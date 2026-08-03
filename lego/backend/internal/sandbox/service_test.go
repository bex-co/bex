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

// adminChecker allows ordinary workspace relations for every fixture identity,
// but reserves can_manage for id-admin. This models two members and one explicit
// administrator in the same workspace.
type adminChecker struct{}

func (adminChecker) Check(_ context.Context, subject, relation, _ string) (bool, error) {
	if relation == core.RelCanManage {
		return subject == "user:id-admin", nil
	}
	return true, nil
}

type egressCall struct {
	op, namespace, session, modelEndpoint string
	extra                                 []string
}

type fakeSessionEgress struct {
	calls []egressCall
	err   error
}

func (f *fakeSessionEgress) PrepareSetup(_ context.Context, namespace, session, modelEndpoint string, extra []string) error {
	f.calls = append(f.calls, egressCall{op: "setup", namespace: namespace, session: session, modelEndpoint: modelEndpoint, extra: append([]string(nil), extra...)})
	return f.err
}

func (f *fakeSessionEgress) TransitionToAgent(_ context.Context, namespace, session, modelEndpoint string, extra []string) error {
	f.calls = append(f.calls, egressCall{op: "agent", namespace: namespace, session: session, modelEndpoint: modelEndpoint, extra: append([]string(nil), extra...)})
	return f.err
}

func (f *fakeSessionEgress) Delete(_ context.Context, namespace, session string) error {
	f.calls = append(f.calls, egressCall{op: "delete", namespace: namespace, session: session})
	return f.err
}

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
	return identityCtx("id-a")
}

func identityCtx(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
}

func TestCreateUsesTemplateImageAndEchoesPlan(t *testing.T) {
	var got createRequest
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode create body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		// A partial create echo must not replace the exact server-bound metadata in
		// the immediate public response.
		_, _ = w.Write([]byte(`{"id":"os-1","metadata":{"bex.co/owner":"wrong"},"status":{"state":"Creating"}}`))
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
	if got.Image.URI != "node:20" {
		t.Errorf("create image = %q, want node:20", got.Image.URI)
	}
	if got.Metadata[metadataOwner] != "id-a" || got.Metadata[metadataWorkspace] != "tea-a" ||
		got.Metadata[metadataNetworkPolicy] != string(NetworkPolicyDenyAll) || got.Metadata[metadataTemplate] != "node" ||
		got.Metadata[metadataRegime] != metadataSandboxRegime {
		t.Errorf("security metadata = %#v", got.Metadata)
	}
	if sb.NetworkPolicy == nil || sb.NetworkPolicy.Default != NetworkPolicyDenyAll {
		t.Errorf("effective network policy = %#v, want deny-all", sb.NetworkPolicy)
	}
}

func TestAgentSessionPolicyPrecedesSandboxCreateAndTransitionUsesDurableAllowlist(t *testing.T) {
	var got createRequest
	eg := &fakeSessionEgress{}
	created := false
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			if len(eg.calls) != 1 || eg.calls[0].op != "setup" {
				t.Errorf("OpenSandbox create happened before setup policy: %#v", eg.calls)
			}
			created = true
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Fatalf("decode create: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"os-agent","status":{"state":"Running"}}`))
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"id":"os-agent","metadata":{"bex.co/owner":"id-a","bex.co/workspace":"tea-a","bex.co/network-policy":"deny-all","app.bex.co/regime":"sandbox","bex.co/agent-session":"ags-one","bex.co/agent-session-model-endpoint":"https://models.example.com/v1","bex.co/agent-session-egress-allowlist":"[\"docs.example.com\"]"},"status":{"state":"Running"}}`))
		default:
			http.NotFound(w, r)
		}
	})
	svc.SessionEgress = eg
	lifecycle := NewAgentSessionLifecycle(svc)
	_, err := lifecycle.CreateAgentSessionSandbox(callerCtx(), "tea-a", "node", "ags-one", "bex-co/example", "bex-agent/session-test", "https://models.example.com/v1", "", []string{"docs.example.com"}, nil)
	if err != nil {
		t.Fatalf("CreateAgentSessionSandbox: %v", err)
	}
	if !created || got.Metadata[metadataAgentSession] != "ags-one" || got.Metadata[metadataModelEndpoint] != "https://models.example.com/v1" || got.Metadata[metadataEgressAllow] != `["docs.example.com"]` {
		t.Fatalf("agent session metadata = %#v", got.Metadata)
	}
	if err := lifecycle.EnterAgentSessionPhase(callerCtx(), "tea-a", "ags-one", "os-agent"); err != nil {
		t.Fatalf("EnterAgentSessionPhase: %v", err)
	}
	if len(eg.calls) != 2 || eg.calls[1].op != "agent" || eg.calls[1].modelEndpoint != "https://models.example.com/v1" || len(eg.calls[1].extra) != 1 || eg.calls[1].extra[0] != "docs.example.com" {
		t.Fatalf("egress calls = %#v", eg.calls)
	}
}

func TestAgentSessionCreateRollsPolicyBackOnSandboxFailure(t *testing.T) {
	eg := &fakeSessionEgress{}
	svc := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	})
	svc.SessionEgress = eg
	_, err := NewAgentSessionLifecycle(svc).CreateAgentSessionSandbox(callerCtx(), "tea-a", "node", "ags-one", "bex-co/example", "bex-agent/session-test", "https://models.example.com/v1", "", nil, nil)
	if err == nil {
		t.Fatal("CreateAgentSessionSandbox succeeded against failed upstream")
	}
	if len(eg.calls) != 2 || eg.calls[0].op != "setup" || eg.calls[1].op != "delete" {
		t.Fatalf("egress calls = %#v, want setup then rollback delete", eg.calls)
	}
}

func TestCreateRejectsUnenforcedNetworkPolicyBeforeUpstream(t *testing.T) {
	calls := 0
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) { calls++ })
	_, err := svc.Create(callerCtx(), CreateRequest{
		Template:      "node",
		NetworkPolicy: &NetworkPolicy{Default: NetworkPolicyAllowAll},
	})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("allow-all error = %v, want ErrBadRequest", err)
	}
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "SANDBOX_NETWORK_POLICY_UNSUPPORTED" {
		t.Fatalf("allow-all error = %#v, want named policy refusal", err)
	}
	if calls != 0 {
		t.Fatalf("unsupported policy reached OpenSandbox %d time(s)", calls)
	}
}

func TestCreateRejectsUnknownTemplateAndPlan(t *testing.T) {
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) { t.Error("must not reach server") })
	if _, err := svc.Create(callerCtx(), CreateRequest{Template: "ghost"}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("unknown template err = %v, want ErrBadRequest", err)
	}
	if _, err := svc.Create(callerCtx(), CreateRequest{Template: "node", Plan: "mega"}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("unknown plan err = %v, want ErrBadRequest", err)
	}
}

func TestCreateRejectsUnpersistableRegionAndTimeout(t *testing.T) {
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) { t.Error("must not reach server") })
	for _, tc := range []struct {
		name string
		req  CreateRequest
		code string
	}{
		{name: "region", req: CreateRequest{Template: "node", Region: "not/a/label"}, code: "SANDBOX_REGION_INVALID"},
		{name: "negative timeout", req: CreateRequest{Template: "node", TimeoutSeconds: -1}, code: "SANDBOX_TIMEOUT_INVALID"},
		{name: "below upstream minimum timeout", req: CreateRequest{Template: "node", TimeoutSeconds: minSandboxTimeout - 1}, code: "SANDBOX_TIMEOUT_INVALID"},
		{name: "excess timeout", req: CreateRequest{Template: "node", TimeoutSeconds: maxSandboxTimeout + 1}, code: "SANDBOX_TIMEOUT_INVALID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(callerCtx(), tc.req)
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != tc.code {
				t.Fatalf("error = %v, want %s", err, tc.code)
			}
		})
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
		t.Error("authz-refused verb must not reach the server")
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
		_, _ = w.Write([]byte(`[{"id":"os-1","metadata":{"bex.co/owner":"id-a","bex.co/workspace":"tea-a","bex.co/network-policy":"deny-all","app.bex.co/regime":"sandbox"},"status":{"state":"Running"}}]`))
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

func TestOwnerBoundaryAndWorkspaceAdminOverride(t *testing.T) {
	var deletes int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes":
			_, _ = w.Write([]byte(`[
				{"id":"os-a","metadata":{"bex.co/owner":"id-a","bex.co/workspace":"tea-a","bex.co/network-policy":"deny-all","app.bex.co/regime":"sandbox"},"status":{"state":"Running"}},
				{"id":"os-b","metadata":{"bex.co/owner":"id-b","bex.co/workspace":"tea-a","bex.co/network-policy":"deny-all","app.bex.co/regime":"sandbox"},"status":{"state":"Running"}},
				{"id":"os-legacy","metadata":{"bex.co/workspace":"tea-a"},"status":{"state":"Running"}},
				{"id":"os-unhardened","metadata":{"bex.co/owner":"id-a","bex.co/workspace":"tea-a","app.bex.co/regime":"sandbox"},"status":{"state":"Running"}},
				{"id":"os-foreign","metadata":{"bex.co/owner":"id-a","bex.co/workspace":"tea-other","bex.co/network-policy":"deny-all","app.bex.co/regime":"sandbox"},"status":{"state":"Running"}}
			]`))
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes/os-b":
			_, _ = w.Write([]byte(`{"id":"os-b","metadata":{"bex.co/owner":"id-b","bex.co/workspace":"tea-a","bex.co/network-policy":"deny-all","app.bex.co/regime":"sandbox"},"status":{"state":"Running"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes/os-legacy":
			_, _ = w.Write([]byte(`{"id":"os-legacy","metadata":{"bex.co/workspace":"tea-a"},"status":{"state":"Running"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes/os-unhardened":
			_, _ = w.Write([]byte(`{"id":"os-unhardened","metadata":{"bex.co/owner":"id-a","bex.co/workspace":"tea-a","app.bex.co/regime":"sandbox"},"status":{"state":"Running"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/sandboxes/os-foreign":
			_, _ = w.Write([]byte(`{"id":"os-foreign","metadata":{"bex.co/owner":"id-a","bex.co/workspace":"tea-other","bex.co/network-policy":"deny-all","app.bex.co/regime":"sandbox"},"status":{"state":"Running"}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/sandboxes/os-b":
			deletes++
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	svc := &Service{
		Base: &core.Base{
			Namespace: "default",
			Workspace: fakeWorkspace{"id-a": "tea-a", "id-b": "tea-a", "id-admin": "tea-a"},
			Authz:     adminChecker{},
		},
		Client: NewClient(srv.URL),
	}

	for _, tc := range []struct {
		subject string
		wantID  string
	}{
		{subject: "id-a", wantID: "os-a"},
		{subject: "id-b", wantID: "os-b"},
	} {
		got, err := svc.List(identityCtx(tc.subject))
		if err != nil || len(got) != 1 || got[0].ID != tc.wantID {
			t.Fatalf("list as %s = %+v, %v; want only %s", tc.subject, got, err, tc.wantID)
		}
	}

	if _, err := svc.Get(identityCtx("id-a"), "os-b"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("cross-owner get = %v, want non-enumerating not found", err)
	}
	for _, id := range []string{"os-legacy", "os-unhardened", "os-foreign"} {
		if _, err := svc.Get(identityCtx("id-a"), id); !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("%s get = %v, want non-enumerating not found", id, err)
		}
	}
	if err := svc.Terminate(identityCtx("id-a"), "os-b"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("cross-owner terminate = %v, want non-enumerating not found", err)
	}
	if deletes != 0 {
		t.Fatalf("cross-owner terminate issued %d delete(s)", deletes)
	}

	adminList, err := svc.List(identityCtx("id-admin"))
	if err != nil || len(adminList) != 2 {
		t.Fatalf("admin list = %+v, %v; want both owned resources only", adminList, err)
	}
	if _, err := svc.Get(identityCtx("id-admin"), "os-b"); err != nil {
		t.Fatalf("admin get: %v", err)
	}
	if err := svc.Terminate(identityCtx("id-admin"), "os-b"); err != nil {
		t.Fatalf("admin terminate: %v", err)
	}
	if deletes != 1 {
		t.Fatalf("admin terminate issued %d delete(s), want 1", deletes)
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

func TestCreateUsesDefaultTemplateWhenEmpty(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"os-1","status":{"state":"Running"}}`))
	}))
	t.Cleanup(srv.Close)
	svc := &Service{
		Base:            &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client:          NewClient(srv.URL),
		Templates:       map[string]Template{"base": {Image: "alpine:3", Entrypoint: []string{"sh"}, CPU: "500m", Memory: "512Mi"}},
		DefaultTemplate: "base",
	}
	// The Render CLI sends no template — an empty Template must resolve to the default.
	sb, err := svc.Create(callerCtx(), CreateRequest{Plan: PlanStarter, Region: "oregon", TimeoutSeconds: 3600})
	if err != nil {
		t.Fatalf("create with empty template should use default: %v", err)
	}
	if sb.Image != "alpine:3" {
		t.Errorf("image = %q, want alpine:3 (default template)", sb.Image)
	}
	if sb.Region != "oregon" || sb.TimeoutSeconds != 3600 {
		t.Errorf("region/timeout not echoed: %+v", sb)
	}
	if !contains(gotBody, `"alpine:3"`) {
		t.Errorf("create body %q missing default-template image", gotBody)
	}
}

func TestCreateStillRejectsWhenNoDefaultAndNoTemplate(t *testing.T) {
	svc := &Service{
		Base:      &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Client:    NewClient(httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).URL),
		Templates: map[string]Template{"base": {Image: "alpine:3", Entrypoint: []string{"sh"}}},
		// DefaultTemplate unset.
	}
	if _, err := svc.Create(callerCtx(), CreateRequest{}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("empty template + no default should be ErrBadRequest, got %v", err)
	}
}
