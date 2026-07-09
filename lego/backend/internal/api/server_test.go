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

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const testToken = "secret-token"

func testScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	return scheme
}

func fakeClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(objs...).Build()
}

func sampleApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: name + ":v1", Replicas: 2},
		Status:     appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, URL: "https://" + name + ".onbex.co"},
	}
}

func podFor(app, name string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "default", Labels: map[string]string{core.PodLabelApp: app},
	}}
}

func staticLogs(lines map[string][]string) logs.PodLogSource {
	return func(_ context.Context, _, pod, _ string, _ int64) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(strings.Join(lines[pod], "\n"))), nil
	}
}

// serverWith builds a fully wired, auth-gated handler over the given base + deps.
func serverWith(t *testing.T, base *core.Base, d Deps) (http.Handler, *Server) {
	t.Helper()
	srv := NewServer(base, d)
	srv.HydraAdminURL = fakeHydraURL(t)
	return buildHandler(t, srv), srv
}

// serverWithKratos is serverWith plus an explicit Kratos URL, for tests that
// also need the session (cookie) auth path alongside the usual bearer one.
// hydraURL is a parameter (not built here) so callers can share one fakeHydra
// across subtests instead of spinning up a new httptest.Server each time.
func serverWithKratos(t *testing.T, base *core.Base, d Deps, hydraURL, kratosURL string) http.Handler {
	t.Helper()
	srv := NewServer(base, d)
	srv.HydraAdminURL = hydraURL
	srv.KratosURL = kratosURL
	return buildHandler(t, srv)
}

func buildHandler(t *testing.T, srv *Server) http.Handler {
	t.Helper()
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	return h
}

func do(t *testing.T, h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func gql(t *testing.T, h http.Handler, query string) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"query": query})
	w := do(t, h, "POST", "/graphql", testToken, string(body))
	if w.Code != 200 {
		t.Fatalf("graphql http %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Data   map[string]any `json:"data"`
		Errors []any          `json:"errors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Errors) > 0 {
		t.Fatalf("graphql errors: %v", out.Errors)
	}
	return out.Data
}

// --- Auth gate ---

func TestAuth(t *testing.T) {
	h, _ := serverWith(t, &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}, Deps{})
	if got := do(t, h, "GET", "/healthz", "", "").Code; got != 200 {
		t.Errorf("healthz should be open, got %d", got)
	}
	if got := do(t, h, "GET", "/v1/services", "", "").Code; got != 401 {
		t.Errorf("no token => 401, got %d", got)
	}
	if got := do(t, h, "GET", "/v1/services", "wrong", "").Code; got != 401 {
		t.Errorf("wrong token => 401, got %d", got)
	}
	if got := do(t, h, "GET", "/v1/services", testToken, "").Code; got != 200 {
		t.Errorf("valid token => 200, got %d", got)
	}
}

func TestMissingHydraURLRefusesToServe(t *testing.T) {
	srv := NewServer(&core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})
	if _, err := srv.Handler(); err == nil || !strings.Contains(err.Error(), "BEX_HYDRA_ADMIN_URL") {
		t.Fatalf("Handler without a Hydra URL must refuse to build, got err=%v", err)
	}
}

// --- REST + GraphQL + MCP surface behavior (one Core, three adapters) ---

func TestREST_ServiceShapesAndVerbs(t *testing.T) {
	h, _ := serverWith(t, &core.Base{Client: fakeClient(sampleApp("web"), sampleApp("api")), Namespace: "default",
		Clock: func() time.Time { return time.Unix(1_000_000, 0).UTC() }}, Deps{})

	// list is the {service, cursor} envelope; /v1/apps aliases it.
	var list []struct {
		Service struct{ ID, Type, Suspended string } `json:"service"`
		Cursor  string                               `json:"cursor"`
	}
	if err := json.Unmarshal(do(t, h, "GET", "/v1/services", testToken, "").Body.Bytes(), &list); err != nil || len(list) != 2 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
	if list[0].Service.ID == "" || list[0].Cursor == "" || list[0].Service.Type != "web_service" {
		t.Fatalf("render envelope wrong: %+v", list[0])
	}
	if do(t, h, "GET", "/v1/apps", testToken, "").Code != 200 {
		t.Error("/v1/apps alias should work")
	}
	// verbs: suspend/resume 202, restart 200; unknown 404.
	if do(t, h, "POST", "/v1/services/web/suspend", testToken, "").Code != 202 {
		t.Error("suspend => 202")
	}
	if do(t, h, "POST", "/v1/services/web/restart", testToken, "").Code != 200 {
		t.Error("restart => 200")
	}
	if do(t, h, "POST", "/v1/services/nope/restart", testToken, "").Code != 404 {
		t.Error("verb on unknown => 404")
	}
}

func TestGraphQL_RenderOperations(t *testing.T) {
	cl := fakeClient(sampleApp("web"))
	h, _ := serverWith(t, &core.Base{Client: cl, Namespace: "default"}, Deps{})

	data := gql(t, h, `{ services { id type suspended } }`)
	if len(data["services"].([]any)) != 1 {
		t.Fatal("want 1 service")
	}
	gql(t, h, `{ server(id:"web") { id name } }`)
	gql(t, h, `mutation { suspendService(id:"web") { id suspended } }`)
	var a appv1alpha1.App
	_ = cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &a)
	if !a.Spec.Suspended {
		t.Error("graphql suspendService must suspend")
	}
}

func TestGraphQL_RequiresAuth(t *testing.T) {
	h, _ := serverWith(t, &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}, Deps{})
	body, _ := json.Marshal(map[string]string{"query": `{ services { id } }`})
	if code := do(t, h, "POST", "/graphql", "", string(body)).Code; code != 401 {
		t.Errorf("graphql without token => 401, got %d", code)
	}
}

// TestGitWebhookBypassesAuthGate proves the push webhook is reachable WITHOUT an
// OAuth token (its HMAC signature is its authentication) — an unsigned call is
// rejected by the webhook itself (401), not by the auth gate, and reaches it at
// all only because it's mounted ahead of the /v1/ auth wildcard.
func TestGitWebhookBypassesAuthGate(t *testing.T) {
	srv := NewServer(&core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}, Deps{})
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.WebhookSecret = "" // unset => the webhook 503s (reached, not gate-blocked)
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	// No token: the auth gate would answer 401 for a gated /v1/ route, but the
	// webhook is not gated, so it answers its own 503 (secret unset).
	if code := do(t, h, "POST", "/v1/webhooks/git", "", "{}").Code; code != 503 {
		t.Errorf("webhook with no secret => 503 (reached, ungated), got %d", code)
	}
}

func mcpSession(t *testing.T, srv *Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.MCPServer().Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestMCP_ExposesRenderConsistentTools(t *testing.T) {
	srv := NewServer(&core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}, Deps{})
	cs := mcpSession(t, srv)

	got := map[string]bool{}
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tl := range tools.Tools {
		got[tl.Name] = true
	}
	for _, want := range []string{
		"list_services", "get_service", "create_web_service", "deploy", "list_logs", "get_metrics",
		"restart_service", "suspend_service", "resume_service", "scale_service",
		"create_api_key", "list_api_keys", "revoke_api_key",
		"list_postgres_instances", "get_postgres", "create_postgres",
	} {
		if !got[want] {
			t.Errorf("missing Render-consistent tool %q (have %v)", want, got)
		}
	}
}

func TestMCP_SuspendDelegatesToCore(t *testing.T) {
	cl := fakeClient(sampleApp("web"), podFor("web", "web-1"))
	srv := NewServer(&core.Base{Client: cl, Namespace: "default"}, Deps{})
	cs := mcpSession(t, srv)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "suspend_service", Arguments: map[string]any{"serviceId": "web"},
	})
	if err != nil || res.IsError {
		t.Fatalf("suspend_service: %v isErr=%v", err, res.IsError)
	}
	var a appv1alpha1.App
	_ = cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &a)
	if !a.Spec.Suspended || a.Spec.Replicas != 2 {
		t.Errorf("suspend_service must suspend and keep replicas: %+v", a.Spec)
	}
}

func TestMCP_ScaleDelegatesToCore(t *testing.T) {
	cl := fakeClient(sampleApp("web"), podFor("web", "web-1")) // sampleApp starts at 2
	srv := NewServer(&core.Base{Client: cl, Namespace: "default"}, Deps{})
	cs := mcpSession(t, srv)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "scale_service", Arguments: map[string]any{"serviceId": "web", "numInstances": 3},
	})
	if err != nil || res.IsError {
		t.Fatalf("scale_service: %v isErr=%v", err, res.IsError)
	}
	var a appv1alpha1.App
	_ = cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &a)
	if a.Spec.Replicas != 3 {
		t.Errorf("scale_service must set spec.replicas to 3, got %d", a.Spec.Replicas)
	}
}

// --- Authorization enforcement (identical across surfaces) ---

type fakeChecker struct {
	allow        bool
	err          error
	lastRelation string
	lastSubject  string
}

func (f *fakeChecker) Check(_ context.Context, subject, relation, _ string) (bool, error) {
	f.lastSubject = subject
	f.lastRelation = relation
	return f.allow, f.err
}

func TestAuthzEnforcement(t *testing.T) {
	newH := func(chk core.Checker) http.Handler {
		base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: chk}
		h, _ := serverWith(t, base, Deps{APIKeys: newFakeKeyStore()})
		return h
	}

	t.Run("deny is 403 REST / errors GraphQL", func(t *testing.T) {
		chk := &fakeChecker{allow: false}
		h := newH(chk)
		if code := do(t, h, "GET", "/v1/services", testToken, "").Code; code != 403 {
			t.Fatalf("REST read: got %d, want 403", code)
		}
		if chk.lastRelation != core.RelCanView {
			t.Errorf("read checked %s, want can_view", chk.lastRelation)
		}
		if do(t, h, "POST", "/v1/services/web/suspend", testToken, "").Code != 403 {
			t.Fatal("REST manage: want 403")
		}
		if chk.lastRelation != core.RelCanOperate {
			t.Errorf("suspend checked %s, want can_operate", chk.lastRelation)
		}
		w := do(t, h, "POST", "/graphql", testToken, `{"query":"{ services { id } }"}`)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "forbidden") {
			t.Fatalf("graphql deny: code %d body %s", w.Code, w.Body.String())
		}
	})
	t.Run("allow passes through", func(t *testing.T) {
		h := newH(&fakeChecker{allow: true})
		if do(t, h, "GET", "/v1/services", testToken, "").Code != 200 {
			t.Fatal("allowed read: want 200")
		}
		if do(t, h, "POST", "/v1/api-keys", testToken, `{"name":"x"}`).Code != 201 {
			t.Fatal("allowed mint: want 201")
		}
	})
	t.Run("checker error fails closed 503", func(t *testing.T) {
		if do(t, newH(&fakeChecker{err: errors.New("fga down")}), "GET", "/v1/services", testToken, "").Code != 503 {
			t.Fatal("checker outage: want 503")
		}
	})
}

// gqlSession posts a GraphQL request over a Kratos session cookie (or none, for
// cookie == "") — the dashboard's auth path, as opposed to gql/do's bearer
// token. Decodes `data` like gql does on a 200, but — unlike gql — doesn't
// fatal on a GraphQL error, since TestAPIKeys_SessionCaller asserts both the
// allow and the deny path; the deny case is checked against the raw body
// (matching this file's existing convention, e.g. TestAuthzEnforcement).
func gqlSession(t *testing.T, h http.Handler, cookie, query string) (code int, data map[string]any, body string) {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"query": query})
	r := httptest.NewRequest("POST", "/graphql", strings.NewReader(string(b)))
	if cookie != "" {
		r.Header.Set("Cookie", cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code == http.StatusOK {
		var out struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		data = out.Data
	}
	return w.Code, data, w.Body.String()
}

// TestAPIKeys_SessionCaller proves the dashboard's auth path (a Kratos session
// cookie, not a bearer token) reaches the api-key GraphQL verbs through the same
// Authorize gate as any other caller — no bearer-only special-casing — and that
// the resolved session identity is checked as "user:<kratos-id>", the same tuple
// shape a bearer subject gets (docs/auth.md's Authorization section).
func TestAPIKeys_SessionCaller(t *testing.T) {
	const sessionCookie = "ory_kratos_session=live" // fakeKratos => identity "identity-1"
	kratos := fakeKratos(t)
	hydraURL := fakeHydraURL(t) // shared across subtests, including the one bearer-path check below
	newH := func(chk core.Checker) http.Handler {
		base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: chk}
		return serverWithKratos(t, base, Deps{APIKeys: newFakeKeyStore()}, hydraURL, kratos.URL)
	}

	t.Run("authorized session mints, lists, and revokes", func(t *testing.T) {
		chk := &fakeChecker{allow: true}
		h := newH(chk)

		code, data, body := gqlSession(t, h, sessionCookie, `mutation { createApiKey(name: "agent-key") { id name secret } }`)
		created, _ := data["createApiKey"].(map[string]any)
		if code != 200 || created["secret"] != "s3cret" {
			t.Fatalf("session mint: code %d body %s", code, body)
		}
		if chk.lastSubject != "user:identity-1" || chk.lastRelation != core.RelCanManageKeys {
			t.Errorf("mint checked subject=%q relation=%q, want user:identity-1 / can_manage_keys", chk.lastSubject, chk.lastRelation)
		}

		code, data, body = gqlSession(t, h, sessionCookie, `{ apiKeys { id name secret } }`)
		if code != 200 || strings.Contains(body, "s3cret") {
			t.Fatalf("session list: code %d body %s (secret must never appear in list)", code, body)
		}
		found := false
		for _, k := range data["apiKeys"].([]any) {
			if key, _ := k.(map[string]any); key["name"] == "agent-key" {
				found = true
			}
		}
		if !found {
			t.Fatalf("session list: agent-key missing, got %v", data)
		}

		code, data, body = gqlSession(t, h, sessionCookie, `mutation { revokeApiKey(id: "key-1") }`)
		if code != 200 || data["revokeApiKey"] != true {
			t.Fatalf("session revoke: code %d body %s", code, body)
		}
	})

	t.Run("bearer path is unchanged — still mints via GraphQL alongside the new session path", func(t *testing.T) {
		h := newH(&fakeChecker{allow: true})
		data := gql(t, h, `mutation { createApiKey(name: "bearer-key") { id name secret } }`)
		created, _ := data["createApiKey"].(map[string]any)
		if created["secret"] != "s3cret" {
			t.Fatalf("bearer mint: got %v", data)
		}
	})

	t.Run("session without the workspace tuple is forbidden, not silently allowed", func(t *testing.T) {
		h := newH(&fakeChecker{allow: false})
		code, _, body := gqlSession(t, h, sessionCookie, `mutation { createApiKey(name: "agent-key") { id } }`)
		if code != 200 || !strings.Contains(body, "forbidden") {
			t.Fatalf("session deny: code %d body %s", code, body)
		}
	})

	t.Run("no session and no bearer is 401 before reaching the resolver", func(t *testing.T) {
		h := newH(&fakeChecker{allow: true})
		code, _, _ := gqlSession(t, h, "", `{ apiKeys { id } }`)
		if code != 401 {
			t.Fatalf("anonymous graphql: got %d, want 401", code)
		}
	})
}

// TestAuthzGuardsEveryVerb sweeps every feature service's verbs (exported methods
// that take a context and return an error) with a deny-all checker: each must
// return ErrForbidden before doing anything else. A new verb that forgets its
// Authorize guard fails this sweep automatically — the CLAUDE.md rule, enforced.
func TestAuthzGuardsEveryVerb(t *testing.T) {
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: &fakeChecker{allow: false}}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-1", Method: "oauth2"})

	// The promoted core.Base helpers (Authorize/GetApp/AppPods/Now) are kernel
	// primitives, not verbs — exclude them from the sweep by name.
	baseMethods := map[string]bool{}
	bt := reflect.TypeOf(&core.Base{})
	for i := 0; i < bt.NumMethod(); i++ {
		baseMethods[bt.Method(i).Name] = true
	}

	services := []any{
		&apps.Service{Base: base},
		&logs.Service{Base: base},
		&metrics.Service{Base: base},
		&apikeys.Service{Base: base, APIKeys: newFakeKeyStore()},
		&postgres.Service{Base: base},
		&secrets.Service{Base: base},
		&envgroups.Service{Base: base},
		&workspaces.Service{Base: base},
	}
	swept := 0
	for _, svc := range services {
		cv := reflect.ValueOf(svc)
		ct := cv.Type()
		for i := 0; i < ct.NumMethod(); i++ {
			m := ct.Method(i)
			if baseMethods[m.Name] {
				continue
			}
			mt := m.Func.Type()
			if mt.NumIn() < 2 || mt.In(1) != reflect.TypeFor[context.Context]() {
				continue
			}
			if mt.NumOut() == 0 || mt.Out(mt.NumOut()-1) != reflect.TypeFor[error]() {
				continue
			}
			swept++
			args := []reflect.Value{cv, reflect.ValueOf(ctx)}
			for a := 2; a < mt.NumIn(); a++ {
				at := mt.In(a)
				if at.Kind() == reflect.Func { // e.g. FollowLogs' emit callback
					args = append(args, reflect.MakeFunc(at, func(_ []reflect.Value) []reflect.Value {
						outs := make([]reflect.Value, at.NumOut())
						for o := range outs {
							outs[o] = reflect.Zero(at.Out(o))
						}
						return outs
					}))
					continue
				}
				args = append(args, reflect.Zero(at))
			}
			out := m.Func.Call(args)
			err, _ := out[len(out)-1].Interface().(error)
			if !errors.Is(err, core.ErrForbidden) {
				t.Errorf("%s.%s: unguarded — returned %v, want ErrForbidden", ct.Elem().Name(), m.Name, err)
			}
		}
	}
	if swept < 19 {
		t.Fatalf("sweep found only %d verbs — reflection filter broke?", swept)
	}
}

// --- Parity / wiring: every feature registers into the single roots ---

func TestSurfaceParityAndWiring(t *testing.T) {
	base := &core.Base{Client: fakeClient(sampleApp("web"), podFor("web", "web-1")), Namespace: "default",
		Clock: func() time.Time { return time.Unix(1_000_000, 0).UTC() }}
	deps := Deps{
		PodLogs:         staticLogs(map[string][]string{"web-1": {"2026-07-05T00:00:01Z hi"}}),
		ResourceMetrics: func(context.Context, string, string) ([]metrics.PodResourceUsage, error) { return nil, nil },
		APIKeys:         newFakeKeyStore(),
	}
	h, srv := serverWith(t, base, deps)

	// REST: every feature's noun answers (2xx/empty, not 404-route-missing).
	for _, path := range []string{"/v1/services", "/v1/postgres", "/v1/api-keys", "/v1/logs?resource=web", "/v1/metrics/instance-count?resource=web", "/v1/owners", "/v1/env-groups", "/v1/services/web/secret-files"} {
		if code := do(t, h, "GET", path, testToken, "").Code; code == 404 {
			t.Errorf("REST route %q not registered (404)", path)
		}
	}

	// GraphQL: the single schema carries every feature's fields.
	schema, err := srv.newSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	qFields := schema.QueryType().Fields()
	for _, f := range []string{"services", "databases", "apiKeys", "logs", "metrics", "workspaces", "envGroups"} {
		if qFields[f] == nil {
			t.Errorf("Query.%s not wired into the single schema", f)
		}
	}
	mFields := schema.MutationType().Fields()
	for _, f := range []string{"suspendService", "createDatabase", "createApiKey", "createWorkspace", "renameWorkspace", "deleteWorkspace", "createEnvGroup", "linkEnvGroup", "setSecretFile"} {
		if mFields[f] == nil {
			t.Errorf("Mutation.%s not wired into the single schema", f)
		}
	}

	// MCP: the single registry carries every feature's tools.
	cs := mcpSession(t, srv)
	tools, _ := cs.ListTools(context.Background(), nil)
	have := map[string]bool{}
	for _, tl := range tools.Tools {
		have[tl.Name] = true
	}
	for _, name := range []string{"list_services", "list_logs", "get_metrics", "create_api_key", "list_workspaces", "select_workspace", "get_selected_workspace", "list_env_groups", "list_secret_files"} {
		if !have[name] {
			t.Errorf("MCP tool %q not registered into the single registry", name)
		}
	}
}

// fakeKeyStore is the in-memory APIKeyStore for the root integration tests.
type fakeKeyStore struct {
	keys map[string]apikeys.APIKey
	n    int
}

func newFakeKeyStore() *fakeKeyStore { return &fakeKeyStore{keys: map[string]apikeys.APIKey{}} }

func (f *fakeKeyStore) Create(_ context.Context, name, createdBy string) (apikeys.APIKey, error) {
	f.n++
	k := apikeys.APIKey{ID: fmt.Sprintf("key-%d", f.n), Name: name, Secret: "s3cret", CreatedBy: createdBy}
	f.keys[k.ID] = apikeys.APIKey{ID: k.ID, Name: k.Name, CreatedBy: createdBy}
	return k, nil
}

func (f *fakeKeyStore) List(context.Context) ([]apikeys.APIKey, error) {
	out := make([]apikeys.APIKey, 0, len(f.keys))
	for _, k := range f.keys {
		out = append(out, k)
	}
	return out, nil
}

func (f *fakeKeyStore) Delete(_ context.Context, id string) error {
	if _, ok := f.keys[id]; !ok {
		return core.ErrNotFound
	}
	delete(f.keys, id)
	return nil
}

func (f *fakeKeyStore) Touch(_ context.Context, id string, at time.Time) error {
	if k, ok := f.keys[id]; ok {
		k.LastUsedAt = at.UTC().Format(time.RFC3339)
		f.keys[id] = k
	}
	return nil
}
