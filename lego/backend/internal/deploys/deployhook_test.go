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

package deploys

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type staleDeployHookChecker struct {
	freshCalls int
}

func deployHookTokenFromURL(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get("key")
}

func (*staleDeployHookChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func (c *staleDeployHookChecker) CheckFresh(context.Context, string, string, string) (bool, error) {
	c.freshCalls++
	return false, nil
}

func TestDeployHookRevealAndRotationFailClosedOnFreshRevocation(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*Service, context.Context) error
	}{
		{name: "reveal", call: func(s *Service, ctx context.Context) error { _, err := s.GetDeployHook(ctx, "web"); return err }},
		{name: "rotate", call: func(s *Service, ctx context.Context) error { _, err := s.RegenerateDeployHook(ctx, "web"); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(newFakeStore(), sampleApp("web", "srv-1"))
			checker := &staleDeployHookChecker{}
			svc.Authz = checker
			// Method "session" so the round-7 F3 credential-class gate (which now
			// fronts both verbs) passes and this test still exercises what it is
			// about: the FRESH relation re-check. TestDeployHookVerbsRequireMintCredentialClass
			// covers the class seam itself.
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "revoked", Method: "session"})
			if err := tc.call(svc, ctx); !errors.Is(err, core.ErrForbidden) {
				t.Fatalf("call error = %v, want ErrForbidden", err)
			}
			if checker.freshCalls != 1 {
				t.Fatalf("fresh checks = %d, want 1", checker.freshCalls)
			}
			var app appv1alpha1.App
			if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &app); err != nil {
				t.Fatal(err)
			}
			if app.Annotations[DeployHookTokenAnnotation] != "" {
				t.Fatal("freshly revoked caller wrote a deploy-hook token")
			}
		})
	}
}

func TestDeployHookTokenIsStableOpaqueAndRotatable(t *testing.T) {
	svc, cl := newService(newFakeStore(), sampleApp("web", "srv-1"))
	svc.DeployHookBaseURL = "https://api.bex.co/"

	first, err := svc.GetDeployHook(context.Background(), "web")
	if err != nil {
		t.Fatalf("GetDeployHook: %v", err)
	}
	second, err := svc.GetDeployHook(context.Background(), "web")
	if err != nil {
		t.Fatalf("GetDeployHook again: %v", err)
	}
	if first.URL != second.URL {
		t.Fatalf("lazy read changed stable URL: %q != %q", first.URL, second.URL)
	}
	token := deployHookTokenFromURL(t, first.URL)
	if !validDeployHookToken(token) {
		t.Fatalf("token is not a 256-bit deploy-hook credential: %q", token)
	}
	var app appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &app); err != nil {
		t.Fatal(err)
	}
	if app.Annotations[DeployHookTokenAnnotation] != token {
		t.Fatalf("App annotation token = %q, want URL token", app.Annotations[DeployHookTokenAnnotation])
	}
	if got, want := app.Labels[DeployHookTokenDigestLabel], deployHookTokenDigest(token); got != want {
		t.Fatalf("App token digest label = %q, want %q", got, want)
	}

	rotated, err := svc.RegenerateDeployHook(context.Background(), "web")
	if err != nil {
		t.Fatalf("RegenerateDeployHook: %v", err)
	}
	if rotated.URL == first.URL {
		t.Fatal("rotation returned the old URL")
	}
	if _, err := svc.appForDeployHookToken(context.Background(), token); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("old token after rotation = %v, want ErrNotFound", err)
	}
	newToken := deployHookTokenFromURL(t, rotated.URL)
	if _, err := svc.appForDeployHookToken(context.Background(), newToken); err != nil {
		t.Fatalf("new token after rotation: %v", err)
	}
}

func TestDeployHookDigestBackfillIndexesExistingTokens(t *testing.T) {
	token, err := newDeployHookToken()
	if err != nil {
		t.Fatal(err)
	}
	a := sampleApp("web", "srv-1")
	a.Annotations = map[string]string{DeployHookTokenAnnotation: token}
	svc, cl := newService(newFakeStore(), a)

	if _, err := svc.appForDeployHookToken(context.Background(), token); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("unindexed token lookup = %v, want ErrNotFound", err)
	}
	if err := BackfillDeployHookTokenDigests(context.Background(), svc.Client); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if _, err := svc.appForDeployHookToken(context.Background(), token); err != nil {
		t.Fatalf("indexed token lookup: %v", err)
	}

	var got appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &got); err != nil {
		t.Fatal(err)
	}
	if got.Labels[DeployHookTokenDigestLabel] != deployHookTokenDigest(token) {
		t.Fatalf("backfilled digest label = %q", got.Labels[DeployHookTokenDigestLabel])
	}
}

func TestDeployHookTokenDigestIsKubernetesLabelSafe(t *testing.T) {
	token := deployHookTokenPrefix + strings.Repeat("A", 43)
	digest := deployHookTokenDigest(token)
	if len(digest) != 52 {
		t.Fatalf("digest length = %d, want full SHA-256 base32 length 52", len(digest))
	}
	if problems := validation.IsValidLabelValue(digest); len(problems) != 0 {
		t.Fatalf("digest %q is not a Kubernetes label value: %v", digest, problems)
	}
}

func TestDeployHookHandlerTriggersGETAndPOSTWithoutAuth(t *testing.T) {
	ds := newFakeStore()
	svc, cl := newService(ds, sampleApp("web", "srv-1"))
	svc.DeployHookLimiter = NewDeployHookRateLimiter(6000, 10)
	hook, err := svc.GetDeployHook(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	token := deployHookTokenFromURL(t, hook.URL)
	h := svc.DeployHookHandler()

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		r := httptest.NewRequest(method, "/v1/deploy-hooks?key="+url.QueryEscape(token)+"&ref=deadbeef", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("%s trigger: code=%d body=%s", method, w.Code, w.Body.String())
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s trigger Cache-Control = %q, want no-store", method, got)
		}
		var body struct {
			Deploy struct {
				ID string `json:"id"`
			} `json:"deploy"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Deploy.ID == "" {
			t.Fatalf("%s response = %s (err %v)", method, w.Body.String(), err)
		}
	}
	app := getApp(t, cl, "web")
	if app.Spec.RestartedAt == "" || app.Spec.BuildCommit != "deadbeef" {
		t.Fatalf("hook did not reuse trigger path: %+v", app.Spec)
	}
	list, err := ds.ListDeploys(context.Background(), "srv-1", store.DeployFilter{})
	if err != nil || len(list) != 2 {
		t.Fatalf("hook deploy rows = %+v (err %v)", list, err)
	}
	for _, d := range list {
		if d.Trigger != store.TriggerDeployHook {
			t.Errorf("deploy trigger = %q, want %q", d.Trigger, store.TriggerDeployHook)
		}
	}
}

func TestDeployHookHandlerCollapsesInvalidAndRotatedTokensTo404(t *testing.T) {
	svc, _ := newService(newFakeStore(), sampleApp("web", "srv-1"))
	old, err := svc.GetDeployHook(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RegenerateDeployHook(context.Background(), "web"); err != nil {
		t.Fatal(err)
	}
	h := svc.DeployHookHandler()
	for _, path := range []string{
		"/v1/deploy-hooks/not-even-close",
		old.URL,
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("%s: code=%d body=%s, want 404", path, w.Code, w.Body.String())
		}
	}
}

func TestDeployHookCredentialNeverOccupiesRequestPath(t *testing.T) {
	svc, _ := newService(newFakeStore(), sampleApp("web", "srv-1"))
	hook, err := svc.GetDeployHook(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(hook.URL)
	if err != nil {
		t.Fatal(err)
	}
	token := u.Query().Get("key")
	if token == "" || strings.Contains(u.Path, token) || u.Path != "/v1/deploy-hooks" {
		t.Fatalf("deploy hook URL retains credential in request path: %q", hook.URL)
	}
}

// TestDeployHookHandlerMethodNotAllowedSpeaksTheOneErrorDialect pins w9/m38:
// the 405 rejection is JSON with Content-Type application/json and Render's
// `message` key, not a text/plain bare-`{"error"}` body.
func TestDeployHookHandlerMethodNotAllowedSpeaksTheOneErrorDialect(t *testing.T) {
	svc, _ := newService(newFakeStore(), sampleApp("web", "srv-1"))
	h := svc.DeployHookHandler()

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/v1/deploy-hooks?key=whatever", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d, want 405; body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if allow := w.Header().Get("Allow"); allow != "GET, POST" {
		t.Errorf("Allow = %q, want %q", allow, "GET, POST")
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v (%s)", err, w.Body.String())
	}
	if body["message"] != "method not allowed" {
		t.Errorf("message = %v, want %q", body["message"], "method not allowed")
	}
}

func TestDeployHookHandlerHasIndependentPerTokenRateLimit(t *testing.T) {
	svc, _ := newService(newFakeStore(), sampleApp("web", "srv-1"))
	svc.DeployHookLimiter = NewDeployHookRateLimiter(0.01, 1)
	hook, err := svc.GetDeployHook(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	h := svc.DeployHookHandler()

	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest(http.MethodPost, hook.URL, nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first trigger = %d, want 200", first.Code)
	}
	second := httptest.NewRecorder()
	h.ServeHTTP(second, httptest.NewRequest(http.MethodPost, hook.URL, nil))
	if second.Code != http.StatusTooManyRequests || second.Header().Get("Retry-After") == "" {
		t.Fatalf("second trigger = %d Retry-After=%q body=%s, want 429", second.Code, second.Header().Get("Retry-After"), second.Body.String())
	}
}

// TestDeployHookRateLimiterIsPerReplica pins the w1/m58 decision: the deploy-hook
// limiter is intentionally replica-local (see DeployHookRateLimiter's doc), so
// two bex-api replicas each grant the same token its own bucket — an effective
// per-token ceiling of up to 2× DefaultDeployHookRPM, accepted as a bounded
// over-provision for a credential-gated, newest-wins-idempotent endpoint.
func TestDeployHookRateLimiterIsPerReplica(t *testing.T) {
	const token = "dhk-sametoken0000000"
	replicaA := NewDeployHookRateLimiter(0.01, 1) // burst 1, effectively no refill
	replicaB := NewDeployHookRateLimiter(0.01, 1)

	// Within a replica the per-token guarantee holds: first allowed, second denied.
	if ok, _ := replicaA.reserve(token); !ok {
		t.Fatal("replica A must allow the token's first request")
	}
	if ok, _ := replicaA.reserve(token); ok {
		t.Fatal("replica A must deny the token's second request (burst exhausted)")
	}
	// The second replica's bucket is independent: the same token is allowed once
	// more there. Two replicas ⇒ up to 2× the per-token budget, by design.
	if ok, _ := replicaB.reserve(token); !ok {
		t.Fatal("replica B must independently allow the same token (per-replica by design)")
	}
}

func TestDeployHookManagementSurfaceParity(t *testing.T) {
	svc, _ := newService(newFakeStore(), sampleApp("web", "srv-1"))
	svc.DeployHookBaseURL = "https://api.bex.co"

	// REST read establishes the stable URL.
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/services/web/deploy-hook", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("REST deploy hook = %d %s", w.Code, w.Body.String())
	}
	var rest DeployHookView
	if err := json.Unmarshal(w.Body.Bytes(), &rest); err != nil {
		t.Fatal(err)
	}

	// GraphQL returns the exact same {url} view.
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	result := graphql.Do(graphql.Params{Context: context.Background(), Schema: schema, RequestString: `{ deployHook(serviceId:"web") { url } }`})
	if len(result.Errors) > 0 {
		t.Fatalf("GraphQL deployHook: %v", result.Errors)
	}
	gqlURL := result.Data.(map[string]any)["deployHook"].(map[string]any)["url"].(string)
	if gqlURL != rest.URL {
		t.Fatalf("GraphQL URL = %q, REST = %q", gqlURL, rest.URL)
	}

	// MCP returns the same initial view.
	cs := newMCPSession(t, svc)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "get_deploy_hook", Arguments: map[string]any{"serviceId": "web"},
	})
	if err != nil || res.IsError {
		t.Fatalf("get_deploy_hook: err=%v result=%+v", err, res)
	}
	var mcpHook DeployHookView
	if err := decodeStructured(res.StructuredContent, &mcpHook); err != nil || mcpHook.URL != rest.URL {
		t.Fatalf("MCP hook = %+v err=%v, REST=%+v", mcpHook, err, rest)
	}

	// Each adapter exposes rotation, and every subsequent adapter observes the
	// same one credential rather than maintaining adapter-local state.
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/services/web/deploy-hook/regenerate", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("REST rotate = %d %s", w.Code, w.Body.String())
	}
	var restRotated DeployHookView
	if err := json.Unmarshal(w.Body.Bytes(), &restRotated); err != nil || restRotated.URL == rest.URL {
		t.Fatalf("REST rotated hook = %+v err=%v, old=%+v", restRotated, err, rest)
	}

	result = graphql.Do(graphql.Params{Context: context.Background(), Schema: schema, RequestString: `mutation { regenerateDeployHook(serviceId:"web") { url } }`})
	if len(result.Errors) > 0 {
		t.Fatalf("GraphQL rotate: %v", result.Errors)
	}
	gqlRotatedURL := result.Data.(map[string]any)["regenerateDeployHook"].(map[string]any)["url"].(string)
	if gqlRotatedURL == restRotated.URL {
		t.Fatal("GraphQL rotation returned the prior REST credential")
	}

	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "get_deploy_hook", Arguments: map[string]any{"serviceId": "web"},
	})
	if err != nil || res.IsError {
		t.Fatalf("get_deploy_hook after GraphQL rotation: err=%v result=%+v", err, res)
	}
	if err := decodeStructured(res.StructuredContent, &mcpHook); err != nil || mcpHook.URL != gqlRotatedURL {
		t.Fatalf("MCP hook after GraphQL rotation = %+v err=%v, GraphQL=%q", mcpHook, err, gqlRotatedURL)
	}

	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "regenerate_deploy_hook", Arguments: map[string]any{"serviceId": "web"},
	})
	if err != nil || res.IsError {
		t.Fatalf("regenerate_deploy_hook: err=%v result=%+v", err, res)
	}
	var rotated DeployHookView
	if err := decodeStructured(res.StructuredContent, &rotated); err != nil || rotated.URL == gqlRotatedURL {
		t.Fatalf("rotated MCP hook = %+v err=%v, prior GraphQL URL=%q", rotated, err, gqlRotatedURL)
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/services/web/deploy-hook", nil))
	if err := json.Unmarshal(w.Body.Bytes(), &rest); w.Code != http.StatusOK || err != nil || rest.URL != rotated.URL {
		t.Fatalf("REST after MCP rotation = code %d hook %+v err=%v, MCP=%+v", w.Code, rest, err, rotated)
	}
}
