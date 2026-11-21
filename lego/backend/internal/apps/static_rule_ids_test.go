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

package apps

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/380: bex accepts a service's slug and display name where Render takes
// only its srv- id (ADR018 "Address a service by name"). A static rule's id
// derived from the path string, so one route had a different id under each
// form, and an id read under one form answered 404 under another. Every form
// now gives the ids the srv- path always did.

const ruleIDsSrvID = "srv-w8380rules00000000a"

type ruleIDsHarness struct {
	t   *testing.T
	mux http.Handler
	app *appv1alpha1.App
	svc *Service
}

func newRuleIDsHarness(t *testing.T) *ruleIDsHarness {
	t.Helper()
	a := sampleStaticApp("tea-a-site")
	a.Namespace = "tea-a"
	a.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelServiceName: "site", core.LabelAppID: ruleIDsSrvID}
	a.Spec.DisplayName = "Marketing"
	svc, _ := newTenantService(fakeWorkspace{"id-a": "tea-a"}, a)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return &ruleIDsHarness{t: t, mux: validated(t, mux), app: a, svc: svc}
}

func (h *ruleIDsHarness) do(method, service, path, body string) *httptest.ResponseRecorder {
	h.t.Helper()
	req := httptest.NewRequest(method, "/v1/services/"+service+path, strings.NewReader(body))
	req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "id-a", Method: "session"}))
	rec := httptest.NewRecorder()
	h.mux.ServeHTTP(rec, req)
	return rec
}

func (h *ruleIDsHarness) routeIDs(service string) []string {
	h.t.Helper()
	rec := h.do(http.MethodGet, service, "/routes", "")
	var rows []routeWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); rec.Code != http.StatusOK || err != nil {
		h.t.Fatalf("list routes under %q = %d %s", service, rec.Code, rec.Body.String())
	}
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.Route.ID
	}
	return out
}

func (h *ruleIDsHarness) headerIDs(service string) []string {
	h.t.Helper()
	rec := h.do(http.MethodGet, service, "/headers", "")
	var rows []headerWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); rec.Code != http.StatusOK || err != nil {
		h.t.Fatalf("list headers under %q = %d %s", service, rec.Code, rec.Body.String())
	}
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.Header.ID
	}
	return out
}

var ruleIDsForms = []string{ruleIDsSrvID, "site", "Marketing"}

func TestStaticRuleIDsAgreeAcrossServiceForms(t *testing.T) {
	h := newRuleIDsHarness(t)
	if rec := h.do(http.MethodPost, "site", "/routes", `{"type":"redirect","source":"/old","destination":"/new"}`); rec.Code != http.StatusCreated {
		t.Fatalf("add-route under the slug = %d %s", rec.Code, rec.Body.String())
	}
	// The srv- form's ids are the ones a Render client sees: derived from the
	// srv- id, as they always were.
	wantRoutes := make([]string, 0, 2)
	for _, r := range toRenderRouteObjects(ruleIDsSrvID, staticRouteViews(append(h.app.Spec.Routes, appv1alpha1.StaticRoute{Type: "redirect", Source: "/old", Destination: "/new"}))) {
		wantRoutes = append(wantRoutes, r.ID)
	}
	wantHeaders := []string{toRenderHeaderObjects(ruleIDsSrvID, staticHeaderViews(h.app.Spec.Headers))[0].ID}
	for _, form := range ruleIDsForms {
		if got := h.routeIDs(form); !slices.Equal(got, wantRoutes) {
			t.Errorf("route ids under %q = %v, want %v", form, got, wantRoutes)
		}
		if got := h.headerIDs(form); !slices.Equal(got, wantHeaders) {
			t.Errorf("header ids under %q = %v, want %v", form, got, wantHeaders)
		}
	}

	// Every write answers the same ids, whichever form it was sent under.
	rec := h.do(http.MethodPatch, "Marketing", "/routes/"+wantRoutes[1], `{"priority":0}`)
	var moved map[string]renderRoute
	if err := json.Unmarshal(rec.Body.Bytes(), &moved); rec.Code != http.StatusOK || err != nil || moved["headers"].ID != wantRoutes[1] {
		t.Fatalf("patch-route by the srv- form's id under the display name = %d %s, want 200 with that id", rec.Code, rec.Body.String())
	}
	if got := h.routeIDs(ruleIDsSrvID); !slices.Equal(got, []string{wantRoutes[1], wantRoutes[0]}) {
		t.Fatalf("route ids after the move = %v, want %v reordered", got, wantRoutes)
	}
	rec = h.do(http.MethodPost, "Marketing", "/headers", `{"path":"/*","name":"X-Robots-Tag","value":"none"}`)
	var added map[string]renderHeader
	if err := json.Unmarshal(rec.Body.Bytes(), &added); rec.Code != http.StatusCreated || err != nil {
		t.Fatalf("add-headers under the display name = %d %s", rec.Code, rec.Body.String())
	}
	if got := h.headerIDs(ruleIDsSrvID); len(got) != 2 || got[1] != added["headers"].ID {
		t.Fatalf("header ids under the srv- id = %v, want the added %q last", got, added["headers"].ID)
	}
	rec = h.do(http.MethodPut, "site", "/routes", `[{"type":"rewrite","source":"/*","destination":"/index.html"}]`)
	var put []renderRoute
	if err := json.Unmarshal(rec.Body.Bytes(), &put); rec.Code != http.StatusOK || err != nil || len(put) != 1 || put[0].ID != wantRoutes[0] {
		t.Fatalf("put-routes under the slug = %d %s, want the srv- form's id %s", rec.Code, rec.Body.String(), wantRoutes[0])
	}

	// An id read under one form edits the rule under every other.
	if rec := h.do(http.MethodDelete, "site", "/headers/"+added["headers"].ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete-header under the slug = %d %s, want 204", rec.Code, rec.Body.String())
	}
	if rec := h.do(http.MethodDelete, "Marketing", "/routes/"+wantRoutes[0], ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete-route under the display name = %d %s, want 204", rec.Code, rec.Body.String())
	}
	for _, form := range ruleIDsForms {
		if got := h.routeIDs(form); len(got) != 0 {
			t.Errorf("route ids under %q after the delete = %v, want none", form, got)
		}
		if got := h.headerIDs(form); !slices.Equal(got, wantHeaders) {
			t.Errorf("header ids under %q after the delete = %v, want %v", form, got, wantHeaders)
		}
	}
}

// A rename changes neither the rule ids nor which ones a path form finds.
func TestStaticRuleIDsSurviveARename(t *testing.T) {
	h := newRuleIDsHarness(t)
	before := h.routeIDs("Marketing")
	if _, err := h.svc.SetDisplayName(core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"}), ruleIDsSrvID, "Landing"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	for _, form := range []string{ruleIDsSrvID, "site", "Landing"} {
		if got := h.routeIDs(form); !slices.Equal(got, before) {
			t.Errorf("route ids under %q after the rename = %v, want %v", form, got, before)
		}
	}
}
