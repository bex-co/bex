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
)

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
	if ten.Plan != "free" || ten.ID == "" {
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

func TestHealthzReportsDBState(t *testing.T) {
	api, _, _ := newTestAPI(t)
	api.Health = func(context.Context) error { return fmt.Errorf("db down") }
	if rr := do(t, api.Handler(), "GET", "/healthz", ""); rr.Code != http.StatusServiceUnavailable {
		t.Errorf("healthz with failing ping: got %d want 503", rr.Code)
	}
}
