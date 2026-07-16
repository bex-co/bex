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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeHydra serves POST /admin/oauth2/introspect: testToken is active (sub
// "client-1", with the given issuer and aud list), everything else inactive.
// hits counts real introspections.
func fakeHydra(t *testing.T, hits *atomic.Int32, iss string, aud ...string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/oauth2/introspect" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		hits.Add(1)
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		if r.PostFormValue("token") == testToken {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active": true, "sub": "client-1", "client_id": "client-1", "iss": iss, "aud": aud,
			})
			return
		}
		_, _ = fmt.Fprint(w, `{"active":false}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fakeHydraURL is the shorthand for tests that only need an introspection backend
// accepting testToken.
func fakeHydraURL(t *testing.T) string {
	var hits atomic.Int32
	return fakeHydra(t, &hits, "").URL
}

// fakeKratos serves GET /sessions/whoami: session token "live-session" or a
// cookie containing ory_kratos_session=live is a valid session for identity
// "identity-1"; anything else is 401.
func fakeKratos(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sessions/whoami" {
			http.NotFound(w, r)
			return
		}
		ok := r.Header.Get("X-Session-Token") == "live-session" ||
			strings.Contains(r.Header.Get("Cookie"), "ory_kratos_session=live")
		if !ok {
			http.Error(w, `{"error":{"code":401}}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"identity":{"id":"identity-1","traits":{"email":"Ada@Example.com","name":"Ada Lovelace"}}}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestAuthGateSessionTraits: the whoami traits land on core.Identity — email
// lowercased (the invite-redemption key), name verbatim (w4/m25's display name).
func TestAuthGateSessionTraits(t *testing.T) {
	kratos := fakeKratos(t)
	s := &Server{HydraAdminURL: fakeHydraURL(t), KratosURL: kratos.URL}
	gate, err := s.newAuthGate()
	if err != nil {
		t.Fatalf("newAuthGate: %v", err)
	}
	auth := gate.middleware
	r := httptest.NewRequest(http.MethodGet, "/probe", nil)
	r.Header.Set("X-Session-Token", "live-session")
	w := httptest.NewRecorder()
	auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := core.IdentityFrom(r.Context())
		_, _ = fmt.Fprintf(w, "%s/%s", id.Email, id.Name)
	})).ServeHTTP(w, r)
	if w.Code != http.StatusOK || w.Body.String() != "ada@example.com/Ada Lovelace" {
		t.Fatalf("status %d body %q", w.Code, w.Body.String())
	}
}

func brokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func deadServer(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return srv.URL
}

// echoIdentity is the probe behind the middleware: it writes what IdentityFrom
// sees so tests assert both the status and the resolved caller.
var echoIdentity = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if id, ok := core.IdentityFrom(r.Context()); ok {
		_, _ = fmt.Fprintf(w, "%s/%s", id.Method, id.Subject)
		return
	}
	_, _ = fmt.Fprint(w, "anonymous")
})

type callbackGitHubClient struct{}

func (callbackGitHubClient) InstallURL() string {
	return "https://github.com/apps/bex/installations/new"
}

func (callbackGitHubClient) GetInstallation(_ context.Context, id int64) (github.Installation, error) {
	return github.Installation{ID: id, AccountLogin: "octo"}, nil
}

func (callbackGitHubClient) ListRepos(context.Context, int64) ([]github.Repo, error) {
	return nil, nil
}

func (callbackGitHubClient) MintInstallationToken(context.Context, int64) (github.InstallationToken, error) {
	return github.InstallationToken{}, nil
}

func (callbackGitHubClient) RepoAccessible(context.Context, string, string, string) (bool, error) {
	return false, nil
}

func (callbackGitHubClient) GetCommit(context.Context, string, string, string, string) (github.Commit, error) {
	return github.Commit{}, nil
}

type callbackGitHubStore struct {
	connections map[string]store.GitConnection
}

func (s *callbackGitHubStore) UpsertGitConnection(_ context.Context, conn store.GitConnection) (store.GitConnection, error) {
	s.connections[conn.WorkspaceID] = conn
	return conn, nil
}

func (s *callbackGitHubStore) GetGitConnection(_ context.Context, workspaceID string) (store.GitConnection, error) {
	conn, ok := s.connections[workspaceID]
	if !ok {
		return store.GitConnection{}, store.ErrNotFound
	}
	return conn, nil
}

func (s *callbackGitHubStore) DeleteGitConnection(_ context.Context, workspaceID string) error {
	delete(s.connections, workspaceID)
	return nil
}

func TestAuthGate(t *testing.T) {
	var hits atomic.Int32
	hydra := fakeHydra(t, &hits, "")
	kratos := fakeKratos(t)

	cases := []struct {
		name       string
		hydraURL   string
		kratosURL  string
		bearer     string
		rawAuth    string
		sessionTok string
		cookie     string
		wantStatus int
		wantBody   string
	}{
		{name: "bearer-active", bearer: testToken, wantStatus: 200, wantBody: "oauth2/client-1"},
		{name: "bearer-inactive", bearer: "revoked", wantStatus: 401},
		{name: "no-credentials", wantStatus: 401},
		{name: "prefixless-authorization-header", rawAuth: testToken, wantStatus: 401},
		{name: "session-token", sessionTok: "live-session", wantStatus: 200, wantBody: "session/identity-1"},
		{name: "session-cookie", cookie: "ory_kratos_session=live", wantStatus: 200, wantBody: "session/identity-1"},
		{name: "session-expired", cookie: "ory_kratos_session=stale", wantStatus: 401},
		{name: "sessions-disabled", kratosURL: "off", cookie: "ory_kratos_session=live", wantStatus: 401},
		{name: "inactive-bearer-beats-valid-session", bearer: "revoked", cookie: "ory_kratos_session=live", wantStatus: 401},
		{name: "hydra-500", hydraURL: "broken", bearer: testToken, wantStatus: 503},
		{name: "hydra-down", hydraURL: "dead", bearer: testToken, wantStatus: 503},
		{name: "kratos-500", kratosURL: "broken", cookie: "ory_kratos_session=live", wantStatus: 503},
		{name: "kratos-down", kratosURL: "dead", cookie: "ory_kratos_session=live", wantStatus: 503},
	}

	upstream := func(t *testing.T, spec, healthy string) string {
		switch spec {
		case "off":
			return ""
		case "broken":
			return brokenServer(t).URL
		case "dead":
			return deadServer(t)
		default:
			return healthy
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{
				HydraAdminURL: upstream(t, tc.hydraURL, hydra.URL),
				KratosURL:     upstream(t, tc.kratosURL, kratos.URL),
			}
			gate, err := s.newAuthGate()
			if err != nil {
				t.Fatalf("newAuthGate: %v", err)
			}
			auth := gate.middleware
			r := httptest.NewRequest(http.MethodGet, "/probe", nil)
			if tc.bearer != "" {
				r.Header.Set("Authorization", "Bearer "+tc.bearer)
			}
			if tc.rawAuth != "" {
				r.Header.Set("Authorization", tc.rawAuth)
			}
			if tc.sessionTok != "" {
				r.Header.Set("X-Session-Token", tc.sessionTok)
			}
			if tc.cookie != "" {
				r.Header.Set("Cookie", tc.cookie)
			}
			w := httptest.NewRecorder()
			auth(echoIdentity).ServeHTTP(w, r)

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", w.Code, tc.wantStatus, w.Body.String())
			}
			if tc.wantBody != "" && w.Body.String() != tc.wantBody {
				t.Fatalf("body = %q, want %q", w.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestAuthGateGitHubCallbackExceptionIsExact(t *testing.T) {
	var hits atomic.Int32
	hydra := fakeHydra(t, &hits, "")
	mw := newOryAuth(hydra.URL, "", "", "", "", nil, nil).middleware(echoIdentity)

	for _, tc := range []struct {
		name, method, path, bearer string
		wantStatus                 int
		wantBody                   string
	}{
		{name: "anonymous exact callback", method: http.MethodGet, path: "/v1/git/callback?state=signed", wantStatus: http.StatusOK, wantBody: "anonymous"},
		{name: "post callback remains gated", method: http.MethodPost, path: "/v1/git/callback", wantStatus: http.StatusUnauthorized},
		{name: "callback child remains gated", method: http.MethodGet, path: "/v1/git/callback/extra", wantStatus: http.StatusUnauthorized},
		{name: "other git route remains gated", method: http.MethodGet, path: "/v1/git/connection", wantStatus: http.StatusUnauthorized},
		{name: "active bearer keeps identity", method: http.MethodGet, path: "/v1/git/callback", bearer: testToken, wantStatus: http.StatusOK, wantBody: "oauth2/client-1"},
		{name: "inactive bearer is not bypassed", method: http.MethodGet, path: "/v1/git/callback", bearer: "revoked", wantStatus: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.bearer != "" {
				r.Header.Set("Authorization", "Bearer "+tc.bearer)
			}
			w := httptest.NewRecorder()
			mw.ServeHTTP(w, r)
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%q", w.Code, tc.wantStatus, w.Body.String())
			}
			if tc.wantBody != "" && w.Body.String() != tc.wantBody {
				t.Fatalf("body = %q, want %q", w.Body.String(), tc.wantBody)
			}
		})
	}
}

func TestGitHubBrowserCallbackThroughFullAuthStack(t *testing.T) {
	st := &callbackGitHubStore{connections: map[string]store.GitConnection{}}
	srv := NewServer(&core.Base{Namespace: "default"}, Deps{
		GitHubClient:      callbackGitHubClient{},
		GitHubStore:       st,
		GitHubStateSecret: []byte("test-only-high-entropy-state-secret"),
		DashboardURL:      "https://dash.bex.co",
	})
	srv.HydraAdminURL = fakeHydraURL(t)
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}

	// The authenticated start call authorizes the workspace and returns the
	// stateful GitHub install URL.
	start := httptest.NewRequest(http.MethodPost, "/v1/git/connect", nil)
	start.Header.Set("Authorization", "Bearer "+testToken)
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, start)
	if startRec.Code != http.StatusOK {
		t.Fatalf("start status = %d, want 200; body=%s", startRec.Code, startRec.Body.String())
	}
	var conn github.Connection
	if err := json.Unmarshal(startRec.Body.Bytes(), &conn); err != nil {
		t.Fatal(err)
	}
	installURL, err := url.Parse(conn.InstallURL)
	if err != nil {
		t.Fatal(err)
	}
	state := installURL.Query().Get("state")
	if state == "" {
		t.Fatal("start response installUrl has no state")
	}

	// GitHub's redirect carries no Ory credential. It passes the exact auth-gate
	// exception, verifies state in the feature, records the connection, and
	// redirects the browser when the production dashboard URL is configured.
	callback := httptest.NewRequest(http.MethodGet, "/v1/git/callback?installation_id=42&state="+url.QueryEscape(state), nil)
	callbackRec := httptest.NewRecorder()
	h.ServeHTTP(callbackRec, callback)
	if callbackRec.Code != http.StatusFound {
		t.Fatalf("callback status = %d, want 302; body=%s", callbackRec.Code, callbackRec.Body.String())
	}
	if got := callbackRec.Header().Get("Location"); got != "https://dash.bex.co/settings" {
		t.Fatalf("callback Location = %q", got)
	}
	got := st.connections[core.DefaultTenant]
	if got.InstallationID != 42 || got.AccountLogin != "octo" {
		t.Fatalf("recorded connection = %+v", got)
	}

	// The legacy API/agent callback uses its ordinary Bearer identity and omits
	// state. DashboardURL must not turn that JSON API response into a redirect.
	apiCallback := httptest.NewRequest(http.MethodGet, "/v1/git/callback?installation_id=42", nil)
	apiCallback.Header.Set("Authorization", "Bearer "+testToken)
	apiCallbackRec := httptest.NewRecorder()
	h.ServeHTTP(apiCallbackRec, apiCallback)
	if apiCallbackRec.Code != http.StatusOK {
		t.Fatalf("authenticated callback status = %d, want 200; body=%s", apiCallbackRec.Code, apiCallbackRec.Body.String())
	}
	var apiBody map[string]string
	if err := json.Unmarshal(apiCallbackRec.Body.Bytes(), &apiBody); err != nil {
		t.Fatal(err)
	}
	if apiBody["status"] != "connected" {
		t.Fatalf("authenticated callback body = %v", apiBody)
	}
	if got := apiCallbackRec.Header().Get("Location"); got != "" {
		t.Fatalf("authenticated callback redirected to %q", got)
	}
}

func TestIntrospectionCache(t *testing.T) {
	var hits atomic.Int32
	hydra := fakeHydra(t, &hits, "")
	mw := newOryAuth(hydra.URL, "", "", "", "", nil, nil).middleware(echoIdentity)

	req := func(token string) int {
		r := httptest.NewRequest(http.MethodGet, "/probe", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, r)
		return w.Code
	}

	for i := range 2 {
		if code := req(testToken); code != 200 {
			t.Fatalf("live token request %d: status %d, want 200", i, code)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("introspections = %d, want 1 (positive cached)", got)
	}
	for i := range 2 {
		if code := req("revoked"); code != 401 {
			t.Fatalf("revoked token request %d: status %d, want 401", i, code)
		}
	}
	if got := hits.Load(); got != 3 {
		t.Fatalf("introspections = %d, want 3 (negatives not cached)", got)
	}
}

func TestLogoutCannotBeUndoneByInflightIntrospection(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	hydra := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
			_ = json.NewEncoder(w).Encode(map[string]any{
				"active": true, "sub": "human-a", "client_id": "shared-client",
			})
			return
		}
		_, _ = fmt.Fprint(w, `{"active":false}`)
	}))
	defer hydra.Close()

	auth := newOryAuth(hydra.URL, "", "", "", "", nil, nil)
	introspected := make(chan struct{})
	go func() {
		_, _ = auth.introspect(context.Background(), "old-access")
		close(introspected)
	}()
	<-started

	invalidated := make(chan struct{})
	go func() {
		auth.invalidate("logout-access", core.Identity{
			Subject: "human-a", Method: "oauth2", ClientID: "shared-client", Human: true,
		})
		close(invalidated)
	}()
	close(release)
	<-introspected
	<-invalidated

	if _, ok := auth.cache.Get("old-access"); ok {
		t.Fatal("in-flight introspection repopulated the cache after logout")
	}
	id, err := auth.introspect(context.Background(), "old-access")
	if err != nil || id != (core.Identity{}) {
		t.Fatalf("old access after logout = %+v, %v", id, err)
	}
	if calls.Load() != 2 {
		t.Fatalf("introspection calls = %d, want 2", calls.Load())
	}
}

func TestInvalidateEvictsOnlyTheRevokedOAuthCredential(t *testing.T) {
	auth := newOryAuth("http://unused", "", "", "", "", nil, nil)
	expires := time.Now().Add(time.Minute)
	values := map[string]core.Identity{
		"human-a-1": {Subject: "human-a", Method: "oauth2", ClientID: "shared", Human: true},
		"human-a-2": {Subject: "human-a", Method: "oauth2", ClientID: "shared", Human: true},
		"human-b":   {Subject: "human-b", Method: "oauth2", ClientID: "shared", Human: true},
		"machine-1": {Subject: "key-1", Method: "oauth2", ClientID: "key-1"},
		"machine-2": {Subject: "key-1", Method: "oauth2", ClientID: "key-1"},
	}
	for token, id := range values {
		auth.cache.Put(token, id, expires)
	}

	auth.invalidate("human-a-1", values["human-a-1"])
	for _, token := range []string{"human-a-1", "human-a-2"} {
		if _, ok := auth.cache.Get(token); ok {
			t.Fatalf("human token %q survived subject-scoped logout", token)
		}
	}
	for _, token := range []string{"human-b", "machine-1", "machine-2"} {
		if _, ok := auth.cache.Get(token); !ok {
			t.Fatalf("unrelated token %q was evicted", token)
		}
	}

	auth.invalidate("machine-1", values["machine-1"])
	for _, token := range []string{"machine-1", "machine-2"} {
		if _, ok := auth.cache.Get(token); ok {
			t.Fatalf("machine token %q survived client self-revocation", token)
		}
	}
	if _, ok := auth.cache.Get("human-b"); !ok {
		t.Fatal("shared human client was evicted by an unrelated API key")
	}
}

type recordingOnboard struct {
	subject string
	email   string
	calls   int
}

func (o *recordingOnboard) EnsureTenant(_ context.Context, subject, email string) (string, error) {
	o.subject, o.email = subject, email
	o.calls++
	return "tea-human", nil
}

func TestHumanOAuthTokenRunsTenantOnboarding(t *testing.T) {
	for _, tc := range []struct {
		name      string
		sub       string
		clientID  string
		wantCalls int
	}{
		{name: "device or authorization token", sub: "kratos-human", clientID: "oauth-client", wantCalls: 1},
		{name: "client credentials without subject", clientID: "oauth-client", wantCalls: 0},
		{name: "client credentials with client subject", sub: "oauth-client", clientID: "oauth-client", wantCalls: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hydra := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"active": true, "sub": tc.sub, "client_id": tc.clientID,
				})
			}))
			defer hydra.Close()
			onboard := &recordingOnboard{}
			mw := newOryAuth(hydra.URL, "", "", "", "", onboard, nil).middleware(echoIdentity)
			r := httptest.NewRequest(http.MethodGet, "/probe", nil)
			r.Header.Set("Authorization", "Bearer token")
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, r)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			if onboard.calls != tc.wantCalls {
				t.Fatalf("onboarding calls = %d, want %d", onboard.calls, tc.wantCalls)
			}
			if tc.wantCalls == 1 && onboard.subject != tc.sub {
				t.Fatalf("onboarded subject = %q", onboard.subject)
			}
		})
	}
}

// TestIntrospectionTouchesKey asserts the gate calls its last-used recorder with
// the token's client_id after a successful API-key introspection, and never for
// an inactive token (w4/m13). The recorder itself is fire-and-forget; here it's a
// synchronous capture so the assertion is deterministic.
func TestIntrospectionTouchesKey(t *testing.T) {
	var hits atomic.Int32
	hydra := fakeHydra(t, &hits, "")
	touched := make(chan string, 4)
	mw := newOryAuth(hydra.URL, "", "", "", "", nil, func(clientID string) { touched <- clientID }).middleware(echoIdentity)

	do := func(token string) {
		r := httptest.NewRequest(http.MethodGet, "/probe", nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		mw.ServeHTTP(httptest.NewRecorder(), r)
	}

	do(testToken)
	select {
	case got := <-touched:
		if got != "client-1" {
			t.Fatalf("touched client = %q, want client-1", got)
		}
	case <-time.After(time.Second):
		t.Fatal("active introspection did not touch the key")
	}

	do("revoked") // inactive token → no touch
	select {
	case got := <-touched:
		t.Fatalf("inactive token must not touch a key, got %q", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestWithCORS(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	do := func(origins, reqOrigin, method string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/graphql", nil)
		if reqOrigin != "" {
			req.Header.Set("Origin", reqOrigin)
		}
		rec := httptest.NewRecorder()
		withCORS(origins, ok).ServeHTTP(rec, req)
		return rec
	}
	const list = "https://dashboard.bex.co, http://localhost:5173"

	for _, origin := range []string{"https://dashboard.bex.co", "http://localhost:5173"} {
		rec := do(list, origin, http.MethodGet)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Errorf("Allow-Origin for %s = %q, want the origin echoed", origin, got)
		}
		if rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Errorf("Allow-Credentials missing for %s", origin)
		}
	}
	// Unlisted origin: Vary only, no allow headers.
	rec := do(list, "https://evil.example", http.MethodGet)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin for unlisted origin = %q, want empty", got)
	}
	// Empty config: pure pass-through.
	rec = do("", "http://localhost:5173", http.MethodGet)
	for _, hd := range []string{"Access-Control-Allow-Origin", "Vary"} {
		if got := rec.Header().Get(hd); got != "" {
			t.Errorf("empty config set %s = %q, want unset", hd, got)
		}
	}
	// Preflight short-circuits with 204 and the echoed origin.
	rec = do(list, "http://localhost:5173", http.MethodOptions)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("preflight: code %d origin %q", rec.Code, rec.Header().Get("Access-Control-Allow-Origin"))
	}
}
