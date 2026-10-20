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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// staticRulesREST is a static_site behind the REST mux.
func staticRulesREST(t *testing.T) func(method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	svc, _ := newService(nil, sampleStaticApp("site"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return func(method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, "/v1/services/site"+path, strings.NewReader(body)))
		return rec
	}
}

func listedRoutes(t *testing.T, do func(method, path, body string) *httptest.ResponseRecorder) []renderRoute {
	t.Helper()
	var rows []routeWithCursor
	if err := json.Unmarshal(do(http.MethodGet, "/routes", "").Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	out := make([]renderRoute, len(rows))
	for i, row := range rows {
		out[i] = row.Route
	}
	return out
}

// Render's single-rule route operations (w8/137): add-route appends, or
// inserts at its priority; patch-route moves a rule and shifts the others;
// delete-route removes one rule. Before, only the bulk GET/PUT were served,
// so these answered 405 or a bare 404.
func TestStaticRouteSingleRuleOperations(t *testing.T) {
	do := staticRulesREST(t)
	for _, src := range []string{"/a", "/b"} {
		if rec := do(http.MethodPost, "/routes", `{"type":"redirect","source":"`+src+`","destination":"/x"}`); rec.Code != http.StatusCreated {
			t.Fatalf("add-route %s = %d %s, want 201", src, rec.Code, rec.Body.String())
		}
	}
	rec := do(http.MethodPost, "/routes", `{"type":"rewrite","source":"/first","destination":"/y","priority":0}`)
	var added renderRoute
	if err := json.Unmarshal(rec.Body.Bytes(), &added); rec.Code != http.StatusCreated || err != nil || added.Source != "/first" || added.Priority != 0 || added.ID == "" {
		t.Fatalf("add-route at priority 0 = %d %s, want the bare route at 0", rec.Code, rec.Body.String())
	}
	sources := func() []string {
		var out []string
		for _, r := range listedRoutes(t, do) {
			out = append(out, r.Source)
		}
		return out
	}
	// sampleStaticApp starts with its own "/*" rule.
	if got := sources(); !slices.Equal(got, []string{"/first", "/*", "/a", "/b"}) {
		t.Fatalf("routes = %v, want /first inserted at 0", got)
	}

	rec = do(http.MethodPatch, "/routes/"+added.ID, `{"priority":3}`)
	var moved map[string]renderRoute
	if err := json.Unmarshal(rec.Body.Bytes(), &moved); rec.Code != http.StatusOK || err != nil || moved["headers"].Source != "/first" || moved["headers"].Priority != 3 {
		t.Fatalf("patch-route = %d %s, want /first at priority 3", rec.Code, rec.Body.String())
	}
	if got := sources(); !slices.Equal(got, []string{"/*", "/a", "/b", "/first"}) {
		t.Fatalf("routes = %v, want /first moved last", got)
	}
	if rec := do(http.MethodPatch, "/routes/"+added.ID, `{"priority":4}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("patch-route past the end = %d %s, want 400", rec.Code, rec.Body.String())
	}

	if rec := do(http.MethodDelete, "/routes/"+added.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete-route = %d %s, want 204", rec.Code, rec.Body.String())
	}
	if got := sources(); !slices.Equal(got, []string{"/*", "/a", "/b"}) {
		t.Fatalf("routes = %v, want /first gone", got)
	}
	for _, rec := range []*httptest.ResponseRecorder{do(http.MethodDelete, "/routes/rte-nope", ""), do(http.MethodPatch, "/routes/rte-nope", `{"priority":0}`)} {
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "route not found") {
			t.Fatalf("unknown route = %d %s, want a named 404", rec.Code, rec.Body.String())
		}
	}
}

// Render's add-headers and delete-header (w8/137).
func TestStaticHeaderSingleRuleOperations(t *testing.T) {
	do := staticRulesREST(t)
	rec := do(http.MethodPost, "/headers", `{"path":"/*","name":"X-Content-Type-Options","value":"nosniff"}`)
	var added map[string]renderHeader
	if err := json.Unmarshal(rec.Body.Bytes(), &added); rec.Code != http.StatusCreated || err != nil || added["headers"].Name != "X-Content-Type-Options" || added["headers"].ID == "" {
		t.Fatalf("add-headers = %d %s, want 201 wrapping the header", rec.Code, rec.Body.String())
	}
	// sampleStaticApp starts with its own X-Frame-Options rule; the new one
	// lists after it.
	var rows []headerWithCursor
	_ = json.Unmarshal(do(http.MethodGet, "/headers", "").Body.Bytes(), &rows)
	if len(rows) != 2 || rows[1].Header.ID != added["headers"].ID {
		t.Fatalf("headers = %+v, want the added rule last", rows)
	}
	if rec := do(http.MethodDelete, "/headers/"+added["headers"].ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete-header = %d %s, want 204", rec.Code, rec.Body.String())
	}
	rows = nil
	_ = json.Unmarshal(do(http.MethodGet, "/headers", "").Body.Bytes(), &rows)
	if len(rows) != 1 || rows[0].Header.Name != "X-Frame-Options" {
		t.Fatalf("headers = %+v, want only the fixture's rule", rows)
	}
	if rec := do(http.MethodDelete, "/headers/hdr-nope", ""); rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "header not found") {
		t.Fatalf("unknown header = %d %s, want a named 404", rec.Code, rec.Body.String())
	}
}
