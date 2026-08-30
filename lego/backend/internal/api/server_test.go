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
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/accounts"
	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/cliauth"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/deploys"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/environments"
	"github.com/bex-co/bex/lego/backend/internal/events"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/jobs"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/notifications"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/projects"
	"github.com/bex-co/bex/lego/backend/internal/registrycreds"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/sshkeys"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/usage"
	"github.com/bex-co/bex/lego/backend/internal/webhooks"
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

// deployHookStore is the smallest control-plane store needed to prove the
// composed public Deploy Hook route reaches the shared deploy trigger. The
// other methods satisfy deploys.DeployStore but are not part of this test.
type deployHookStore struct{}

func (deployHookStore) CreateDeploy(_ context.Context, appID, trigger, image string, generation int64, _ store.CommitInfo) (store.Deploy, error) {
	return store.Deploy{
		ID:         "dep-public-hook",
		AppID:      appID,
		Trigger:    trigger,
		Image:      image,
		Generation: generation,
		Status:     store.DeployCreated,
	}, nil
}

func (deployHookStore) CreateRollbackDeploy(context.Context, string, string, string, int64, store.CommitInfo) (store.Deploy, error) {
	return store.Deploy{}, errors.New("unexpected CreateRollbackDeploy")
}

func (deployHookStore) ListDeploys(context.Context, string, store.DeployFilter) ([]store.Deploy, error) {
	return nil, errors.New("unexpected ListDeploys")
}

func (deployHookStore) GetDeploy(context.Context, string, string) (store.Deploy, error) {
	return store.Deploy{}, errors.New("unexpected GetDeploy")
}

func (deployHookStore) CloseDeploy(context.Context, string, string, string) (bool, error) {
	return false, errors.New("unexpected CloseDeploy")
}

func (deployHookStore) SetAppImage(context.Context, string, string) error {
	return errors.New("unexpected SetAppImage")
}

func (deployHookStore) InsertServiceEventFact(context.Context, store.ServiceEventFact) (bool, error) {
	return false, errors.New("unexpected InsertServiceEventFact")
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

func TestRenderCLIProtocolRoutesAreTheOnlyPublicV1AuthRoutes(t *testing.T) {
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/device/auth" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code": "device", "user_code": "ABCDEF",
			"verification_uri":          "https://dashboard.bex.co/auth/device",
			"verification_uri_complete": "https://dashboard.bex.co/auth/device?user_code=ABCDEF",
			"expires_in":                600, "interval": 5,
		})
	}))
	defer public.Close()

	srv := NewServer(&core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.OAuthIssuer = public.URL
	h := buildHandler(t, srv)
	body := `{"client_id":"` + cliauth.RenderCLIClientID + `"}`
	if got := do(t, h, http.MethodPost, "/v1/device-grant", "", body); got.Code != http.StatusOK {
		t.Fatalf("device grant = %d %s", got.Code, got.Body.String())
	}
	for _, path := range []string{"/v1/services", "/v1/oauth/revoke"} {
		if got := do(t, h, http.MethodPost, path, "", ""); got.Code != http.StatusUnauthorized {
			t.Fatalf("%s without auth = %d, want 401", path, got.Code)
		}
	}
}

func TestRenderCLIProtocolRoutesEnforceBodyLimit(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	srv := NewServer(&core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.OAuthIssuer = upstream.URL
	srv.MaxBodyBytes = 64
	h := buildHandler(t, srv)

	for _, path := range []string{"/v1/device-grant", "/v1/device-token", "/v1/token/refresh/"} {
		t.Run(path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(strings.Repeat("x", 65)))
			r.ContentLength = -1 // Exercise the chunked/unknown-length path too.
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("oversized request = %d %s, want 413", w.Code, w.Body.String())
			}
		})
	}
	if upstreamCalls != 0 {
		t.Fatalf("oversized requests reached Hydra %d times, want 0", upstreamCalls)
	}
}

func TestRenderCLIDeviceRateLimitPrecedesBodyLimit(t *testing.T) {
	srv := NewServer(&core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.MaxBodyBytes = 1
	srv.DeviceRateLimiter = cliauth.NewDeviceRateLimiter(60, 1)
	h := buildHandler(t, srv)

	request := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/v1/device-grant", strings.NewReader("xx"))
		r.RemoteAddr = "203.0.113.10:54321"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if got := request(); got.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("first admitted oversized request = %d, want 413", got.Code)
	}
	if got := request(); got.Code != http.StatusTooManyRequests {
		t.Fatalf("second oversized request = %d, want 429 before body read", got.Code)
	}
}

func TestRenderCLILogoutImmediatelyInvalidatesCachedAccessToken(t *testing.T) {
	revoked := false
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/admin/oauth2/introspect":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active": !revoked,
				"sub":    "human-a", "client_id": cliauth.RenderCLIClientID,
				"scope": "openid offline_access bex.read bex.write bex.sensitive",
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/admin/clients/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"client_id": cliauth.RenderCLIClientID,
				"metadata":  map[string]any{platformClientMarker: true},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/admin/oauth2/auth/sessions/consent":
			if r.URL.Query().Get("subject") != "human-a" || r.URL.Query().Get("client") != cliauth.RenderCLIClientID {
				t.Fatalf("wrong revoke scope: %s", r.URL.RawQuery)
			}
			revoked = true
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer admin.Close()

	srv := NewServer(&core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})
	srv.HydraAdminURL = admin.URL
	srv.OAuthIssuer = "https://oauth.bex.co"
	h := buildHandler(t, srv)
	// The CLI refreshes in SetupCommands before executing logout. Seed the
	// immediately previous access token in bex's positive cache; consent-chain
	// revocation must evict it as well as the bearer used for logout.
	if got := do(t, h, http.MethodGet, "/v1/services", "previous-token", ""); got.Code != http.StatusOK {
		t.Fatalf("pre-cache previous token = %d %s", got.Code, got.Body.String())
	}
	if got := do(t, h, http.MethodPost, "/v1/oauth/revoke", testToken, ""); got.Code != http.StatusNoContent {
		t.Fatalf("logout = %d %s", got.Code, got.Body.String())
	}
	for _, token := range []string{testToken, "previous-token"} {
		if got := do(t, h, http.MethodGet, "/v1/services", token, ""); got.Code != http.StatusUnauthorized {
			t.Fatalf("revoked cached token %q = %d, want 401; body=%s", token, got.Code, got.Body.String())
		}
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

	// list is the {service, cursor} envelope.
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
	if do(t, h, "GET", "/v1/apps", testToken, "").Code != http.StatusNotFound {
		t.Error("retired public /v1/apps alias should return 404")
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

func TestRetiredPublicRESTAliasesAreAbsent(t *testing.T) {
	h, _ := serverWith(t, &core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/v1/apps"},
		{http.MethodGet, "/v1/databases"},
		{http.MethodGet, "/v1/registry-credentials"},
		{http.MethodGet, "/v1/webhooks/endpoints"},
		{http.MethodGet, "/v1/postgres/dpg-old/exports"},
		{http.MethodGet, "/v1/postgres/dpg-old/recovery-info"},
	} {
		if got := do(t, h, tc.method, tc.path, testToken, "").Code; got != http.StatusNotFound {
			t.Errorf("%s %s = %d, want 404", tc.method, tc.path, got)
		}
	}

	legacyPayload := `{"name":"legacy","url":"https://example.com/hook","eventTypes":["deploy_started"]}`
	if got := do(t, h, http.MethodPost, "/v1/webhooks", testToken, legacyPayload).Code; got != http.StatusBadRequest {
		t.Errorf("POST /v1/webhooks with eventTypes = %d, want 400", got)
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

func TestGraphQLWebhookUpdateRejectsRetiredEventFilterAlias(t *testing.T) {
	h, _ := serverWith(t, &core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})
	body, _ := json.Marshal(map[string]string{
		"query": `mutation { updateWebhookEndpoint(id:"whk-old", eventFilter:["deploy_started"]) { id } }`,
	})
	w := do(t, h, http.MethodPost, "/graphql", testToken, string(body))
	if w.Code != http.StatusOK {
		t.Fatalf("graphql http %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode GraphQL response: %v", err)
	}
	if len(out.Errors) != 1 || !strings.Contains(out.Errors[0].Message, `Unknown argument "eventFilter"`) {
		t.Fatalf("GraphQL eventFilter alias was not rejected: %s", w.Body.String())
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

// TestStripeWebhookBypassesAuthGate proves the Stripe-Signature-verified route
// is mounted before OAuth. Signature behavior itself is covered in billing;
// this test protects the composition-root trust boundary.
func TestStripeWebhookBypassesAuthGate(t *testing.T) {
	called := false
	srv := NewServer(&core.Base{Client: fakeClient(), Namespace: "default"}, Deps{
		StripeWebhook: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusNoContent)
		}),
	})
	srv.HydraAdminURL = fakeHydraURL(t)
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	if code := do(t, h, http.MethodPost, "/v1/webhooks/stripe", "", "{}").Code; code != http.StatusNoContent {
		t.Fatalf("Stripe webhook without OAuth = %d, want 204", code)
	}
	if !called {
		t.Fatal("Stripe webhook handler was not reached")
	}
}

// TestDeployHookBypassesAuthGate proves the hook's secret URL is its complete
// credential at the composed server boundary. It must reach the deploy handler
// without OAuth while ordinary /v1 routes remain protected by Hydra.
func TestDeployHookBypassesAuthGate(t *testing.T) {
	a := sampleApp("web")
	a.Labels = map[string]string{store.LabelAppID: "srv-web"}
	srv := NewServer(&core.Base{Client: fakeClient(a), Namespace: "default"}, Deps{
		DeployStore: deployHookStore{},
	})
	// One request exhausts the authenticated caller limiter. Both hook calls
	// below must still pass through their separate two-request hook bucket.
	srv.RateLimiter = NewRateLimiter(0.01, 1)
	// No bearer request in this test needs introspection; a non-empty URL is
	// enough to build the auth gate without opening an unnecessary test socket.
	srv.HydraAdminURL = "http://hydra.invalid"

	hook, err := srv.Deploys.GetDeployHook(context.Background(), "web")
	if err != nil {
		t.Fatalf("GetDeployHook: %v", err)
	}
	h := buildHandler(t, srv)

	for i := range 2 {
		if w := do(t, h, http.MethodPost, hook.URL, "", ""); w.Code != http.StatusOK {
			t.Fatalf("public hook call %d without OAuth = %d %s, want 200", i+1, w.Code, w.Body.String())
		}
	}
	for _, path := range []string{
		"/v1/deploy-hooks/not-a-token",
		"/v1/deploy-hooks/not/even/one/segment",
		"/v1/deploy-hooks",
	} {
		if w := do(t, h, http.MethodPost, path, "", ""); w.Code != http.StatusNotFound {
			t.Fatalf("invalid hook %q without OAuth = %d %s, want handler 404", path, w.Code, w.Body.String())
		}
	}
	if w := do(t, h, http.MethodGet, "/v1/services", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("ordinary /v1 route without OAuth = %d, want 401", w.Code)
	}
}

func TestDeployHookLookupLimiterShedsRandomTokensBeforeLookup(t *testing.T) {
	srv := NewServer(&core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}, Deps{
		DeployStore: deployHookStore{},
	})
	srv.HydraAdminURL = "http://hydra.invalid"
	srv.DeployHookLookupRateLimiter = NewRateLimiter(0.01, 1)
	h := buildHandler(t, srv)

	// Both are well-formed 256-bit credentials and use different per-token
	// buckets. The second 429 can therefore only come from the outer IP budget.
	firstToken := "dhk-" + strings.Repeat("A", 43)
	secondToken := "dhk-" + strings.Repeat("B", 43)
	if w := do(t, h, http.MethodPost, "/v1/deploy-hooks?key="+firstToken, "", ""); w.Code != http.StatusNotFound {
		t.Fatalf("first random token = %d %s, want 404", w.Code, w.Body.String())
	}
	if w := do(t, h, http.MethodPost, "/v1/deploy-hooks?key="+secondToken, "", ""); w.Code != http.StatusTooManyRequests {
		t.Fatalf("second random token = %d %s, want pre-lookup 429", w.Code, w.Body.String())
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

func TestMCP_ExposesRenderConsistentAndBexExtensionTools(t *testing.T) {
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
		"restart_service", "suspend_service", "resume_service", "scale_service", "delete_service",
		// w1/m71 folded set_registry_credential (and 17 other service setters)
		// into the patch-shaped update_service.
		"update_service",
		"run_cron_job", "list_cron_job_runs", "get_cron_job_run", "cancel_cron_job_run",
		"create_api_key", "list_api_keys", "revoke_api_key",
		"list_postgres_instances", "get_postgres", "create_postgres",
		"list_key_value", "get_key_value", "create_key_value",
	} {
		if !got[want] {
			t.Errorf("missing Render-consistent tool %q (have %v)", want, got)
		}
	}
	for _, want := range []string{"list_webhook_deliveries", "resend_webhook_delivery", "get_service_outbound_ips"} {
		if !got[want] {
			t.Errorf("missing bex webhook extension tool %q (have %v)", want, got)
		}
	}
}

func TestWebhookResendRESTAndGraphQLRoutesAreComposed(t *testing.T) {
	h, _ := serverWith(t, &core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})

	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/whk-one/events/whd-one/resend", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Idempotency-Key", "server-resend-0001")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "webhook store not configured") {
		t.Fatalf("REST resend route = %d %s", w.Code, w.Body.String())
	}

	body, _ := json.Marshal(map[string]string{
		"query": `mutation { resendWebhookDelivery(endpointId:"whk-one", attemptId:"whd-one", idempotencyKey:"server-resend-0001") { id status } }`,
	})
	w = do(t, h, http.MethodPost, "/graphql", testToken, string(body))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "webhook store not configured") || strings.Contains(w.Body.String(), "Unknown field") {
		t.Fatalf("GraphQL resend route = %d %s", w.Code, w.Body.String())
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
	app := sampleApp("web") // starts at 2
	app.Spec.Image = ""
	app.Spec.Repo = "https://github.com/bex-co/private.git"
	app.Spec.CloneSecret = "deliberately-unusable-clone-secret"
	cl := fakeClient(app, podFor("web", "web-1"))
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

// TestMCP_CreateThenDeleteRoundTrip proves the lifecycle loop over MCP: an agent
// creates a service from a prebuilt image and gets it back, then delete_service
// removes it through the same Core.Delete verb REST/GraphQL use (the CR is gone).
func TestMCP_CreateThenDeleteRoundTrip(t *testing.T) {
	cl := fakeClient() // empty cluster — create makes the App
	srv := NewServer(&core.Base{Client: cl, Namespace: "default"}, Deps{})
	cs := mcpSession(t, srv)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "create_web_service",
		Arguments: map[string]any{"name": "web", "image": "nginx:1", "runtime": "image", "buildCommand": "", "startCommand": "", "plan": "starter"},
	})
	if err != nil || res.IsError {
		t.Fatalf("create_web_service: %v isErr=%v %+v", err, res.IsError, res)
	}
	var a appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "web"}, &a); err != nil {
		t.Fatalf("create must make the App CR: %v", err)
	}
	if a.Spec.Image != "nginx:1" {
		t.Errorf("spec.image = %q, want nginx:1", a.Spec.Image)
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "delete_service", Arguments: map[string]any{"serviceId": "web"},
	})
	if err != nil || res.IsError {
		t.Fatalf("delete_service: %v isErr=%v %+v", err, res.IsError, res)
	}
	err = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "web"}, &a)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("delete_service must remove the App CR, got err=%v", err)
	}

	// Deleting the now-unknown service surfaces the error to the agent.
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "delete_service", Arguments: map[string]any{"serviceId": "web"},
	})
	if err == nil && !res.IsError {
		t.Errorf("delete of an unknown service must be an error result")
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
		// Round-7 F3: the bearer is a machine token, so the mint verb is
		// credential-class-gated to 403 even with an allow-all checker; the
		// read above proves the pass-through.
		if do(t, h, "POST", "/v1/api-keys", testToken, `{"name":"x"}`).Code != 403 {
			t.Fatal("machine bearer mint: want 403")
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
// shape a bearer subject gets (docs/ADR012-auth.md's Authorization section).
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

	t.Run("machine bearer cannot mint — round-7 F3 credential-class gate", func(t *testing.T) {
		// The bearer here is testToken: sub == client_id, i.e. a
		// client_credentials API key. Before round-7 F3 it could mint another
		// key (self-replication of a compromised credential); the class gate
		// now refuses it before any relation check, while every non-mint verb
		// keeps working for the same bearer.
		h := newH(&fakeChecker{allow: true})
		w := do(t, h, http.MethodPost, "/graphql", testToken,
			`{"query":"mutation { createApiKey(name: \"bearer-key\") { id name secret } }"}`)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "forbidden") {
			t.Fatalf("machine bearer mint: code %d body %s", w.Code, w.Body.String())
		}
		if resp := do(t, h, http.MethodGet, "/v1/api-keys", testToken, ""); resp.Code != http.StatusOK {
			t.Fatalf("non-mint verb refused for the same bearer: code %d body %s", resp.Code, resp.Body.String())
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
// sweepableServices lists every feature service TestAuthzGuardsEveryVerb (and
// w4/m10's audit-coverage sweep, which reuses it so the two inventories can't
// drift) walks by reflection. One list, shared, so a new feature added here
// automatically joins both sweeps.
type sweepProjectStore struct{}

type sweepProjectResources struct{}

func (sweepProjectResources) ListPostgres(context.Context, string) ([]postgres.PostgresView, error) {
	return nil, nil
}

func (sweepProjectResources) ListKeyValues(context.Context, string) ([]keyvalue.KeyValueView, error) {
	return nil, nil
}

func (sweepProjectResources) SetProjectID(context.Context, string, string) error { return nil }

func (sweepProjectStore) CreateProject(_ context.Context, tenantID, name string) (store.Project, error) {
	return store.Project{ID: "prj-sweep", TenantID: tenantID, Name: name}, nil
}

func (sweepProjectStore) GetProject(context.Context, string) (store.Project, error) {
	return store.Project{ID: "prj-sweep", TenantID: "tea-b", Name: "sweep"}, nil
}

func (sweepProjectStore) ListProjects(context.Context, string) ([]store.Project, error) {
	return nil, nil
}

func (sweepProjectStore) RenameProject(context.Context, string, string) error { return nil }
func (sweepProjectStore) DeleteProject(context.Context, string) error         { return nil }
func (sweepProjectStore) SetProjectServices(context.Context, string, string, []string) ([]core.ServicePlacementChange, error) {
	return nil, nil
}
func (sweepProjectStore) ListProjectServices(context.Context, string) ([]string, error) {
	return nil, nil
}

func sweepableServices(base *core.Base) []any {
	return []any{
		&apps.Service{Base: base},
		&logs.Service{Base: base},
		&metrics.Service{Base: base},
		&apikeys.Service{Base: base, APIKeys: newFakeKeyStore()},
		&accounts.Service{Base: base},
		&sshkeys.Service{Base: base},
		&sandbox.Service{Base: base},
		&agentsessions.Service{Base: base},
		&postgres.Service{Base: base},
		&secrets.Service{Base: base},
		&envgroups.Service{Base: base},
		&workspaces.Service{Base: base},
		&members.Service{Base: base},
		&billing.Service{Base: base},
		&deploys.Service{Base: base},
		&audit.Service{Base: base},
		&github.Service{Base: base},
		&registrycreds.Service{Base: base},
		&environments.Service{Base: base},
		&projects.Service{
			Base:      base,
			Store:     sweepProjectStore{},
			Databases: sweepProjectResources{},
			KeyValues: sweepProjectResources{},
		},
		&jobs.Service{Base: base},
		// w1/m53: these five were wired into features() but omitted from the
		// sweep, so TestAuthzGuardsEveryVerb silently skipped them. Added; the
		// TestSweepCoversEveryWiredService guard below now fails if the list ever
		// diverges from features() again.
		&keyvalue.Service{Base: base},
		&usage.Service{Base: base},
		&events.Service{Base: base},
		&notifications.Service{Base: base},
		&webhooks.Service{Base: base},
	}
}

// TestSweepCoversEveryWiredService is the w1/m53 anti-drift guard: it fails when
// a feature service wired into the live surface (Server.features()) is absent
// from sweepableServices — the hand-maintained inventory the authz/audit/relation
// sweeps walk. Without it, five wired services (keyvalue/usage/events/
// notifications/webhooks) had silently escaped TestAuthzGuardsEveryVerb. It
// discovers the wired set by reflection (every *core.Base-embedding pointer field
// of Server), so no second list can drift.
func TestSweepCoversEveryWiredService(t *testing.T) {
	baseType := reflect.TypeOf(&core.Base{})
	embedsBase := func(structType reflect.Type) bool {
		for i := 0; i < structType.NumField(); i++ {
			f := structType.Field(i)
			if f.Anonymous && f.Type == baseType {
				return true
			}
		}
		return false
	}

	// Populate every feature-service field of Server so features() surfaces all
	// of them; the inner *core.Base staying nil is fine (features() only nil-checks
	// the pointer).
	srv := &Server{}
	v := reflect.ValueOf(srv).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() != reflect.Ptr || f.Type().Elem().Kind() != reflect.Struct || !f.CanSet() {
			continue
		}
		if embedsBase(f.Type().Elem()) {
			f.Set(reflect.New(f.Type().Elem()))
		}
	}

	wired := srv.features()
	if len(wired) == 0 {
		t.Fatal("no feature services discovered via reflection — the guard would be vacuous")
	}
	covered := map[reflect.Type]bool{}
	for _, s := range sweepableServices(&core.Base{}) {
		covered[reflect.TypeOf(s)] = true
	}
	for _, s := range wired {
		if !covered[reflect.TypeOf(s)] {
			t.Errorf("%T is wired by Server.features() but missing from sweepableServices — "+
				"TestAuthzGuardsEveryVerb will not check its verbs; add it to sweepableServices", s)
		}
	}
}

// baseMethodNames returns the promoted core.Base helpers (Authorize/GetApp/
// AppPods/Now) — kernel primitives, not verbs, excluded from every sweep by
// name.
func baseMethodNames() map[string]bool {
	names := map[string]bool{}
	bt := reflect.TypeOf(&core.Base{})
	for i := 0; i < bt.NumMethod(); i++ {
		names[bt.Method(i).Name] = true
	}
	return names
}

// isVerbMethod reports whether m is a verb the sweeps below walk: exported,
// (ctx, ...) -> (..., error), and not one of the promoted core.Base helpers.
func isVerbMethod(baseMethods map[string]bool, m reflect.Method) bool {
	if baseMethods[m.Name] {
		return false
	}
	mt := m.Func.Type()
	if mt.NumIn() < 2 || mt.In(1) != reflect.TypeFor[context.Context]() {
		return false
	}
	return mt.NumOut() > 0 && mt.Out(mt.NumOut()-1) == reflect.TypeFor[error]()
}

// callVerb invokes m on cv with ctx plus zero-valued remaining args (e.g.
// FollowLogs' emit callback becomes a no-op func), returning its error result.
func callVerb(cv reflect.Value, m reflect.Method, ctx context.Context) error {
	mt := m.Func.Type()
	args := []reflect.Value{cv, reflect.ValueOf(ctx)}
	for a := 2; a < mt.NumIn(); a++ {
		at := mt.In(a)
		if at.Kind() == reflect.Func {
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
	return err
}

// sweepEveryVerb walks every service's verb methods (isVerbMethod) and calls
// fn once per verb with the zero-valued call's error result.
func sweepEveryVerb(t *testing.T, ctx context.Context, services []any, fn func(serviceName, method string, err error)) int {
	t.Helper()
	baseMethods := baseMethodNames()
	swept := 0
	for _, svc := range services {
		cv := reflect.ValueOf(svc)
		ct := cv.Type()
		for i := 0; i < ct.NumMethod(); i++ {
			m := ct.Method(i)
			if !isVerbMethod(baseMethods, m) {
				continue
			}
			swept++
			fn(ct.Elem().Name(), m.Name, callVerb(cv, m, ctx))
		}
	}
	return swept
}

// wantMinSweptVerbs is the sanity floor every reflection sweep below checks
// its walk against — shared so the two sweeps' thresholds can't drift apart.
const wantMinSweptVerbs = 19

// sweepVerbPairs walks two parallel service inventories (same types, same
// method order — sweepableServices called twice, once per core.Base) verb by
// verb, calling each verb on BOTH before moving to the next and handing fn
// both outcomes — the seam TestAuditCoversEveryWriteVerbExactlyOnce uses to
// compare an allow-all run against a deny-all run of the exact same call.
//
// This deliberately does NOT build on sweepEveryVerb by running it twice
// (once per inventory): TestAuditCoversEveryWriteVerbExactlyOnce identifies
// each verb's own event by the *delta* in cumulative sink length since the
// previous verb, which only holds if the allow and deny calls for one verb
// happen back-to-back. Two fully separate passes would front-load the entire
// deny sink's growth onto the first verb of the second pass — a real bug, not
// a style choice, so the interleaving here is load-bearing.
func sweepVerbPairs(t *testing.T, ctx context.Context, allowServices, denyServices []any, fn func(serviceName, method string, allowErr, denyErr error)) int {
	t.Helper()
	baseMethods := baseMethodNames()
	swept := 0
	for i := range allowServices {
		av, dv := reflect.ValueOf(allowServices[i]), reflect.ValueOf(denyServices[i])
		ct := av.Type()
		for j := 0; j < ct.NumMethod(); j++ {
			m := ct.Method(j)
			if !isVerbMethod(baseMethods, m) {
				continue
			}
			swept++
			allowErr := callVerb(av, m, ctx)
			denyErr := callVerb(dv, m, ctx)
			fn(ct.Elem().Name(), m.Name, allowErr, denyErr)
		}
	}
	return swept
}

func TestAuthzGuardsEveryVerb(t *testing.T) {
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: &fakeChecker{allow: false}}
	// A direct session identity (round-7 F3): the durable-credential mint verbs
	// are credential-class-gated before their Authorize, and this sweep exists
	// to prove every verb's relation guard exists — see mint_class_test.go for
	// the class gate's own coverage.
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-1", Method: "session"})

	swept := sweepEveryVerb(t, ctx, sweepableServices(base), func(serviceName, method string, err error) {
		if !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s.%s: unguarded — returned %v, want ErrForbidden", serviceName, method, err)
		}
	})
	if swept < wantMinSweptVerbs {
		t.Fatalf("sweep found only %d verbs — reflection filter broke?", swept)
	}
}

// fakeAuditSink records every event handed to it (w4/m10, t004's acceptance
// bar: "unit-verifiable with a fake sink").
type fakeAuditSink struct {
	events []core.AuditEvent
}

func (f *fakeAuditSink) Record(_ context.Context, ev core.AuditEvent) error {
	f.events = append(f.events, ev)
	return nil
}

// TestAuditCoversEveryWriteVerbExactlyOnce sweeps the SAME service inventory
// TestAuthzGuardsEveryVerb does (sweepableServices — one list, so a verb that
// joins the authz sweep automatically joins this one, per t001/t004's design
// goal) once with an allow-all checker and once with a deny-all checker, each
// wired to its own fake sink, and compares event counts per verb. No
// hand-maintained "these are the write verbs" list: every relation now
// records on denial (w4/m20/t001 — writeRelations ∪ readRelations covers
// every Rel… constant, so TestAuthzGuardsEveryVerb's own ErrForbidden proof
// means the deny run must record exactly once, whichever map the verb's
// relation falls in); a write-relation verb ADDITIONALLY records on success
// (one event each outcome), a read-relation verb does not (zero on success —
// volume) — any other pattern (no denial event, more than one event per
// call, or a success event for a read verb) is the bug this test exists to
// catch.
func TestAuditCoversEveryWriteVerbExactlyOnce(t *testing.T) {
	allowSink := &fakeAuditSink{}
	allowBase := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: &fakeChecker{allow: true}, Audit: allowSink}
	denySink := &fakeAuditSink{}
	denyBase := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: &fakeChecker{allow: false}, Audit: denySink}
	// A direct session identity (round-7 F3): the durable-credential mint verbs
	// (apikeys/sshkeys Create) are class-gated BEFORE their Authorize, so a
	// machine-class fixture would refuse there without ever reaching — or
	// recording — the relation decision this sweep exists to count. The class
	// gate itself has dedicated coverage (mint_class_test.go).
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-1", Method: "session"})

	var allowSeen, denySeen, writeVerbs, readVerbs int
	swept := sweepVerbPairs(t, ctx, sweepableServices(allowBase), sweepableServices(denyBase),
		func(serviceName, method string, allowErr, denyErr error) {
			allowDelta := len(allowSink.events) - allowSeen
			denyDelta := len(denySink.events) - denySeen
			allowSeen, denySeen = len(allowSink.events), len(denySink.events)

			if denyDelta != 1 {
				t.Errorf("%s.%s: deny run recorded %d event(s), want exactly 1 — every relation records its denial now, write or read",
					serviceName, method, denyDelta)
				return
			}
			if allowDelta > 1 {
				t.Errorf("%s.%s: recorded %d success events for one call, want at most 1", serviceName, method, allowDelta)
				return
			}
			denyEv := denySink.events[len(denySink.events)-1]
			if denyEv.Outcome != core.AuditDenied {
				t.Errorf("%s.%s: deny-run event outcome = %q, want %q", serviceName, method, denyEv.Outcome, core.AuditDenied)
			}
			if !errors.Is(denyErr, core.ErrForbidden) {
				t.Errorf("%s.%s: deny run returned %v, want ErrForbidden alongside the denial event", serviceName, method, denyErr)
			}
			if allowDelta == 0 {
				readVerbs++
				return // a read-relation verb — correctly unaudited on success
			}
			writeVerbs++
			allowEv := allowSink.events[len(allowSink.events)-1]
			if allowEv.Outcome != core.AuditAllowed {
				t.Errorf("%s.%s: allow-run event outcome = %q, want %q", serviceName, method, allowEv.Outcome, core.AuditAllowed)
			}
			if allowEv.Verb == "" || allowEv.Resource == "" {
				t.Errorf("%s.%s: event has an empty Verb/Resource: %+v", serviceName, method, allowEv)
			}
			if allowEv.Caller != "client-1" {
				t.Errorf("%s.%s: event Caller = %q, want the calling identity", serviceName, method, allowEv.Caller)
			}
		})
	if swept < wantMinSweptVerbs {
		t.Fatalf("sweep found only %d verbs — reflection filter broke?", swept)
	}
	if writeVerbs == 0 {
		t.Fatal("no write verbs recorded an audit event — the hook or the sweep is broken")
	}
	if readVerbs == 0 {
		t.Fatal("no read verbs skipped the success event — the hook or the sweep is broken")
	}
	t.Logf("%d/%d swept verbs are audited write verbs, %d are audited-on-denial-only read verbs", writeVerbs, swept, readVerbs)
}

// fakeAuditKV is a minimal in-memory core.SecretKV for TestAuditNeverCarriesSecretValues.
type fakeAuditKV struct{ m map[string]map[string]string }

func (f *fakeAuditKV) Get(_ context.Context, path string) (map[string]string, error) {
	if f.m[path] == nil {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(f.m[path]))
	for k, v := range f.m[path] {
		out[k] = v
	}
	return out, nil
}
func (f *fakeAuditKV) Put(_ context.Context, path string, data map[string]string) error {
	if f.m == nil {
		f.m = map[string]map[string]string{}
	}
	f.m[path] = data
	return nil
}
func (f *fakeAuditKV) Delete(_ context.Context, path string) error    { delete(f.m, path); return nil }
func (f *fakeAuditKV) List(context.Context, string) ([]string, error) { return nil, nil }

// TestAuditNeverCarriesSecretValues proves the hygiene half of t004's
// acceptance bar directly against the highest-risk write verb: setting an env
// var. The audit hook fires from Base.Authorize, called before SetEnvVar ever
// reads its key/value arguments (internal/secrets/service.go) — a structural
// guarantee, not a scrub step — so no event field can ever contain the value.
// This test pins that guarantee so a future refactor that moved the Authorize
// call after the value was touched would be caught here.
func TestAuditNeverCarriesSecretValues(t *testing.T) {
	sink := &fakeAuditSink{}
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: &fakeChecker{allow: true}, Audit: sink}
	svc := &secrets.Service{Base: base, Store: &fakeAuditKV{}}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-1", Method: "oauth2"})

	const secretValue = "sk_live_do_not_leak_this_9f8e7d6c"
	if _, err := svc.SetEnvVar(ctx, "web", "API_KEY", secrets.EnvVarWrite{Value: secretValue}); err != nil {
		t.Fatalf("SetEnvVar: %v", err)
	}
	if len(sink.events) != 1 {
		t.Fatalf("got %d audit events, want exactly 1", len(sink.events))
	}
	ev := sink.events[0]
	for _, field := range []string{ev.Verb, ev.Resource, ev.Caller, ev.CallerMethod, string(ev.Outcome)} {
		if strings.Contains(field, secretValue) {
			t.Fatalf("audit event field %q contains the secret value — event: %+v", field, ev)
		}
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
	for _, path := range []string{"/v1/services", "/v1/postgres", "/v1/api-keys", "/v1/logs?resource=web", "/v1/metrics/instance-count?resource=web", "/v1/owners", "/v1/env-groups", "/v1/services/web/secret-files", "/v1/services/web/deploys", "/v1/services/web/events", "/v1/events/evt-00000000000000000000"} {
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
	for _, f := range []string{"services", "cronJobRuns", "cronJobRun", "databases", "apiKeys", "logs", "metrics", "workspaces", "envGroups", "envGroup", "envGroupVar", "envGroupSecretFile", "deploys", "serviceEvents", "serviceEvent", "workspaceMembers", "workspaceInvites"} {
		if qFields[f] == nil {
			t.Errorf("Query.%s not wired into the single schema", f)
		}
	}
	mFields := schema.MutationType().Fields()
	for _, f := range []string{"suspendService", "runCronJob", "cancelCronJobRun", "createDatabase", "createApiKey", "createWorkspace", "renameWorkspace", "changeWorkspacePlan", "deleteWorkspace", "createEnvGroup", "renameEnvGroup", "setEnvGroupVar", "deleteEnvGroupVar", "linkEnvGroup", "setSecretFile", "inviteWorkspaceMember", "changeWorkspaceMemberRole", "removeWorkspaceMember", "revokeWorkspaceInvite"} {
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
	for _, name := range []string{"list_services", "list_cron_job_runs", "get_cron_job_run", "cancel_cron_job_run", "list_logs", "list_log_label_values", "get_metrics", "create_api_key", "list_workspaces", "list_env_groups", "rename_env_group", "get_env_group_var", "set_env_group_var", "delete_env_group_var", "get_env_group_secret_file", "list_secret_files", "list_deploys", "get_deploy", "list_service_events", "get_service_event", "list_workspace_members", "invite_workspace_member"} {
		if !have[name] {
			t.Errorf("MCP tool %q not registered into the single registry", name)
		}
	}
}

// fakeKeyStore is the in-memory APIKeyStore for the root integration tests.
type fakeKeyStore struct {
	mu   sync.Mutex
	keys map[string]apikeys.APIKey
	n    int
}

func newFakeKeyStore() *fakeKeyStore { return &fakeKeyStore{keys: map[string]apikeys.APIKey{}} }

func (f *fakeKeyStore) Create(_ context.Context, name, createdBy string) (apikeys.APIKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	k := apikeys.APIKey{ID: fmt.Sprintf("key-%d", f.n), Name: name, Secret: "s3cret", CreatedBy: createdBy}
	f.keys[k.ID] = apikeys.APIKey{ID: k.ID, Name: k.Name, CreatedBy: createdBy}
	return k, nil
}

func (f *fakeKeyStore) List(context.Context) ([]apikeys.APIKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]apikeys.APIKey, 0, len(f.keys))
	for _, k := range f.keys {
		out = append(out, k)
	}
	return out, nil
}

func (f *fakeKeyStore) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.keys[id]; !ok {
		return core.ErrNotFound
	}
	delete(f.keys, id)
	return nil
}

func (f *fakeKeyStore) Touch(_ context.Context, id string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if k, ok := f.keys[id]; ok {
		k.LastUsedAt = at.UTC().Format(time.RFC3339)
		f.keys[id] = k
	}
	return nil
}

func (f *fakeKeyStore) KeyOwner(_ context.Context, id string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k, ok := f.keys[id]
	return k.CreatedBy, ok && k.CreatedBy != ""
}
