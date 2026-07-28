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

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type billingAdminCapture struct {
	workspace, action, actor, reason string
	extension                        time.Duration
}

func (a *billingAdminCapture) OverrideBilling(_ context.Context, workspace, action, actor, reason string, extension time.Duration) (BillingLifecycle, error) {
	a.workspace, a.action, a.actor, a.reason, a.extension = workspace, action, actor, reason, extension
	return BillingLifecycle{WorkspaceID: workspace, Status: BillingGrace, Reason: reason}, nil
}

func newTestAPI(t *testing.T) (*API, *memStore, *int) {
	t.Helper()
	store := newMemStore()
	kicks := 0
	return &API{Store: store, Kick: func() { kicks++ }}, store, &kicks
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestServiceCap_HobbyRefused26th(t *testing.T) {
	api, mem, _ := newTestAPI(t)
	ctx := context.Background()
	ten, _ := mem.CreateTenant(ctx, "acme", PlanHobby)

	// 25 apps of any kind (some suspended — the row exists regardless, so it
	// counts toward the cap, matching Render's "including suspended").
	for i := 0; i < 25; i++ {
		if _, err := mem.CreateApp(ctx, App{TenantID: ten.ID, Name: fmt.Sprintf("a%d", i), Image: "x", Suspended: i%2 == 0}); err != nil {
			t.Fatalf("seed app %d: %v", i, err)
		}
	}
	if err := api.enforceServiceCap(ctx, ten.ID); err == nil {
		t.Fatal("26th service on Hobby should be refused")
	}

	// The same workspace on a paid plan is unlimited.
	ten.Plan = PlanPro
	mem.tenants[ten.ID] = ten
	if err := api.enforceServiceCap(ctx, ten.ID); err != nil {
		t.Fatalf("pro plan should not cap: %v", err)
	}
}

func TestCreateTenantAndApp(t *testing.T) {
	api, _, kicks := newTestAPI(t)
	h := api.Handler()

	rr := do(t, h, "POST", "/v1/tenants", `{"name":"acme"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create tenant: %d %s", rr.Code, rr.Body)
	}
	var ten Tenant
	if err := json.Unmarshal(rr.Body.Bytes(), &ten); err != nil {
		t.Fatal(err)
	}
	// A plan-less create defaults to the Hobby workspace plan (w6/m1) — distinct
	// from the app's compute-tier default ("free"), asserted below.
	if ten.Plan != PlanHobby || ten.ID == "" {
		t.Errorf("tenant = %+v", ten)
	}

	rr = do(t, h, "POST", "/v1/apps", fmt.Sprintf(`{"tenantId":%q,"name":"web","image":"traefik/whoami"}`, ten.ID))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create app: %d %s", rr.Code, rr.Body)
	}
	var app App
	if err := json.Unmarshal(rr.Body.Bytes(), &app); err != nil {
		t.Fatal(err)
	}
	// Zero values fall back to platform defaults.
	if app.Port != 3000 || app.Replicas != 1 || app.Branch != "main" || app.Tier != "free" {
		t.Errorf("defaults not applied: %+v", app)
	}
	if *kicks != 1 {
		t.Errorf("kicks = %d, want 1 (writes must nudge the reconciler)", *kicks)
	}

	rr = do(t, h, "GET", "/v1/apps/"+app.ID, "")
	if rr.Code != http.StatusOK {
		t.Errorf("get app: %d %s", rr.Code, rr.Body)
	}
	rr = do(t, h, "DELETE", "/v1/apps/"+app.ID, "")
	if rr.Code != http.StatusNoContent {
		t.Errorf("delete app: %d %s", rr.Code, rr.Body)
	}
	if *kicks != 2 {
		t.Errorf("kicks = %d, want 2", *kicks)
	}
}

func TestSetBillingExcludedRoute(t *testing.T) {
	api, mem, _ := newTestAPI(t)
	h := api.Handler()
	ten, _ := mem.CreateTenant(t.Context(), "acme", PlanHobby)

	rr := do(t, h, "PATCH", "/v1/tenants/"+ten.ID+"/billing-excluded", `{"excluded":true,"actor":"admin@bex.co"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("exclude: %d %s", rr.Code, rr.Body)
	}
	var resp struct {
		TenantID        string `json:"tenantId"`
		BillingExcluded bool   `json:"billingExcluded"`
		Changed         bool   `json:"changed"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.TenantID != ten.ID || !resp.BillingExcluded || !resp.Changed {
		t.Fatalf("response = %+v, want {tenantId=%s excluded=true changed=true}", resp, ten.ID)
	}
	if !mem.billingExcluded[ten.ID] {
		t.Fatal("flag not set in store")
	}

	// A no-op toggle reports changed=false.
	rr = do(t, h, "PATCH", "/v1/tenants/"+ten.ID+"/billing-excluded", `{"excluded":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("re-exclude: %d %s", rr.Code, rr.Body)
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Changed {
		t.Error("no-op toggle reported changed=true")
	}

	// Unknown workspace → 404.
	rr = do(t, h, "PATCH", "/v1/tenants/tea-nope/billing-excluded", `{"excluded":true}`)
	if rr.Code != http.StatusNotFound {
		t.Errorf("unknown tenant = %d, want 404", rr.Code)
	}
}

func TestValidationAndErrorCodes(t *testing.T) {
	api, store, _ := newTestAPI(t)
	h := api.Handler()
	ten, _ := store.CreateTenant(t.Context(), "acme", "free")
	app, _ := store.CreateApp(t.Context(), App{TenantID: ten.ID, Name: "web", Image: "img"})

	cases := []struct {
		name         string
		method, path string
		body         string
		want         int
	}{
		{"tenant bad name", "POST", "/v1/tenants", `{"name":"Not_Valid!"}`, 400},
		{"tenant name too long", "POST", "/v1/tenants", `{"name":"` + strings.Repeat("a", 31) + `"}`, 400},
		{"tenant bad plan", "POST", "/v1/tenants", `{"name":"ok","plan":"gold"}`, 400},
		{"tenant admin with whitespace", "POST", "/v1/tenants", `{"name":"ok","admin":"has space"}`, 400},
		{"tenant admin with tuple char", "POST", "/v1/tenants", `{"name":"ok","admin":"evil:injected#relation"}`, 400},
		{"tenant admin valid uuid", "POST", "/v1/tenants", `{"name":"okadmin","admin":"a1b2c3d4-0000-1111-2222-333344445555"}`, 201},
		{"tenant duplicate", "POST", "/v1/tenants", `{"name":"acme"}`, 409},
		{"tenant bad json", "POST", "/v1/tenants", `{`, 400},
		{"app no tenant", "POST", "/v1/apps", `{"name":"web","image":"img"}`, 400},
		{"app unknown tenant", "POST", "/v1/apps", `{"tenantId":"ten-999","name":"web","image":"img"}`, 404},
		{"app no repo or image", "POST", "/v1/apps", fmt.Sprintf(`{"tenantId":%q,"name":"api"}`, ten.ID), 400},
		{"app bad port", "POST", "/v1/apps", fmt.Sprintf(`{"tenantId":%q,"name":"api","image":"img","port":70000}`, ten.ID), 400},
		{"app bad tier", "POST", "/v1/apps", fmt.Sprintf(`{"tenantId":%q,"name":"api","image":"img","tier":"gold"}`, ten.ID), 400},
		{"app negative replicas", "POST", "/v1/apps", fmt.Sprintf(`{"tenantId":%q,"name":"api","image":"img","replicas":-1}`, ten.ID), 400},
		{"app duplicate", "POST", "/v1/apps", fmt.Sprintf(`{"tenantId":%q,"name":"web","image":"img"}`, ten.ID), 409},
		{"app get missing", "GET", "/v1/apps/nope", "", 404},
		{"app delete missing", "DELETE", "/v1/apps/nope", "", 404},
		{"domain no app", "POST", "/v1/domains", `{"host":"a.example.com"}`, 400},
		{"domain unknown app", "POST", "/v1/domains", `{"appId":"app-999","host":"a.example.com"}`, 404},
		{"domain bare label host", "POST", "/v1/domains", fmt.Sprintf(`{"appId":%q,"host":"nodots"}`, app.ID), 400},
		{"domain uppercase host", "POST", "/v1/domains", fmt.Sprintf(`{"appId":%q,"host":"WWW.example.com"}`, app.ID), 400},
		{"domain ok", "POST", "/v1/domains", fmt.Sprintf(`{"appId":%q,"host":"web.example.com","primary":true}`, app.ID), 201},
		{"domain duplicate", "POST", "/v1/domains", fmt.Sprintf(`{"appId":%q,"host":"web.example.com"}`, app.ID), 409},
	}
	for _, tc := range cases {
		if rr := do(t, h, tc.method, tc.path, tc.body); rr.Code != tc.want {
			t.Errorf("%s: got %d want %d (%s)", tc.name, rr.Code, tc.want, rr.Body)
		}
	}
}

func TestBearerToken(t *testing.T) {
	api, _, _ := newTestAPI(t)
	api.Token = "sekret"
	h := api.Handler()

	if rr := do(t, h, "GET", "/healthz", ""); rr.Code != http.StatusOK {
		t.Errorf("healthz must stay open: %d", rr.Code)
	}
	if rr := do(t, h, "GET", "/v1/apps", ""); rr.Code != http.StatusUnauthorized {
		t.Errorf("no token: got %d want 401", rr.Code)
	}
	req := httptest.NewRequest("GET", "/v1/apps", nil)
	req.Header.Set("Authorization", "Bearer sekret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("with token: got %d want 200 (%s)", rr.Code, rr.Body)
	}
}

func TestBillingOverrideIsControlPlaneBearerOnlyAndReasoned(t *testing.T) {
	api, _, _ := newTestAPI(t)
	admin := &billingAdminCapture{}
	api.Token = "control-plane-secret"
	api.Billing = admin
	h := api.Handler()
	body := `{"action":"extend_grace","actor":"ops@example.com","reason":"support incident","extension":"2h"}`

	if rr := do(t, h, http.MethodPost, "/v1/tenants/tea-a/billing-override", body); rr.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized override = %d %s", rr.Code, rr.Body.String())
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/tenants/tea-a/billing-override", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer control-plane-secret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("authorized override = %d %s", rr.Code, rr.Body.String())
	}
	if admin.workspace != "tea-a" || admin.action != "extend_grace" || admin.actor != "ops@example.com" || admin.reason != "support incident" || admin.extension != 2*time.Hour {
		t.Fatalf("override args = %+v", admin)
	}
}

func TestHealthzReportsDBState(t *testing.T) {
	api, _, _ := newTestAPI(t)
	api.Health = func(context.Context) error { return fmt.Errorf("db down") }
	if rr := do(t, api.Handler(), "GET", "/healthz", ""); rr.Code != http.StatusServiceUnavailable {
		t.Errorf("healthz with failing ping: got %d want 503", rr.Code)
	}
}

// fakeSandboxTenants is a stub SandboxTenantResolver: it maps known keys to
// workspaces and errors on the rest (the 401 path).
type fakeSandboxTenants map[string]string

func (f fakeSandboxTenants) WorkspaceForSandboxKey(_ context.Context, key string) (string, error) {
	if ws, ok := f[key]; ok {
		return ws, nil
	}
	return "", ErrNotFound
}

func TestSandboxTenantLookup(t *testing.T) {
	api, _, _ := newTestAPI(t)
	api.Token = "cp-secret"
	api.SandboxTenants = fakeSandboxTenants{"osk-good": "tea-a"}
	h := api.Handler()

	get := func(auth, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/v1/sandbox-tenants", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		if key != "" {
			req.Header.Set("OPEN-SANDBOX-API-KEY", key)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	// Wrong/absent CP bearer is rejected by a.bearer before the handler runs.
	if rr := get("", "osk-good"); rr.Code != http.StatusUnauthorized {
		t.Errorf("no bearer: got %d want 401", rr.Code)
	}

	// Valid bearer + known key → 200 with the `<ws>-sandbox` namespace and a ttl.
	rr := get("Bearer cp-secret", "osk-good")
	if rr.Code != http.StatusOK {
		t.Fatalf("known key: got %d %s", rr.Code, rr.Body.String())
	}
	var ok struct {
		Namespace string `json:"namespace"`
		TTL       int    `json:"ttl"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &ok); err != nil {
		t.Fatal(err)
	}
	if ok.Namespace != "tea-a-sandbox" || ok.TTL <= 0 {
		t.Errorf("lookup body = %+v, want namespace tea-a-sandbox + positive ttl", ok)
	}

	// Valid bearer + unknown key → 401 with the provider's UNAUTHORIZED shape.
	rr = get("Bearer cp-secret", "osk-bogus")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("unknown key: got %d want 401", rr.Code)
	}
	var bad struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &bad)
	if bad.Code != "UNAUTHORIZED" {
		t.Errorf("unknown-key code = %q, want UNAUTHORIZED", bad.Code)
	}
}

func TestSandboxTenantUnconfiguredIs503(t *testing.T) {
	api, _, _ := newTestAPI(t)
	api.Token = "cp-secret"
	// SandboxTenants left nil.
	req := httptest.NewRequest(http.MethodGet, "/v1/sandbox-tenants", nil)
	req.Header.Set("Authorization", "Bearer cp-secret")
	req.Header.Set("OPEN-SANDBOX-API-KEY", "osk-x")
	rr := httptest.NewRecorder()
	api.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("nil resolver: got %d want 503", rr.Code)
	}
}
