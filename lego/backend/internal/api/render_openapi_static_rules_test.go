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
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/062: the static-site rule endpoints answer Render's id-bearing objects —
// list-routes/list-headers as {route|header, cursor} envelopes, put-routes/
// update-headers as bare route/header objects — validated against the pinned
// spec through the composed server.
func TestStaticRuleEndpointsMatchRenderContract(t *testing.T) {
	site := conformApp("site", "srv-site")
	site.Spec.Type = appv1alpha1.TypeStaticSite
	site.Spec.Image = ""
	site.Spec.Repo = "https://github.com/acme/site"
	site.Spec.PublishPath = "dist"
	h, _ := serverWith(t, &core.Base{Client: fakeClient(site), Namespace: "default"}, Deps{APIKeys: newFakeKeyStore()})
	spec := loadRenderSpec(t)

	routes := `[{"type":"redirect","source":"/old","destination":"/"},` +
		`{"type":"rewrite","source":"/app/*","destination":"/index.html"},` +
		`{"type":"redirect","source":"/old","destination":"/"}]`
	headers := `[{"path":"/*","name":"X-Frame-Options","value":"DENY"},{"path":"/*.css","name":"Cache-Control","value":"max-age=60"}]`

	type route struct {
		ID, Type, Source, Destination string
		Priority                      int
	}
	putRoutes := func() []route {
		t.Helper()
		w := do(t, h, http.MethodPut, "/v1/services/site/routes", testToken, routes)
		if w.Code != http.StatusOK {
			t.Fatalf("put-routes = %d: %s", w.Code, w.Body.String())
		}
		if errs := spec.validate("put-routes", w.Body.Bytes()); len(errs) > 0 {
			t.Fatalf("put-routes diverges from [route]: %v", errs)
		}
		var out []route
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	first := putRoutes()
	if len(first) != 3 || first[0].Source != "/old" || first[1].Type != "rewrite" {
		t.Fatalf("put-routes = %+v, want the three rules in order", first)
	}
	for i, r := range first {
		if r.Priority != i {
			t.Errorf("route %d priority = %d, want its list index", i, r.Priority)
		}
	}
	if first[0].ID == first[2].ID {
		t.Errorf("identical rules share id %s; duplicates must stay distinct", first[0].ID)
	}
	if again := putRoutes(); again[0].ID != first[0].ID || again[1].ID != first[1].ID || again[2].ID != first[2].ID {
		t.Errorf("ids changed across a no-op re-PUT: %+v vs %+v", first, again)
	}
	// A reorder keeps each rule's id: the rewrite moves to priority 0.
	routes = `[{"type":"rewrite","source":"/app/*","destination":"/index.html"},` +
		`{"type":"redirect","source":"/old","destination":"/"},` +
		`{"type":"redirect","source":"/old","destination":"/"}]`
	reordered := putRoutes()
	if reordered[0].ID != first[1].ID || reordered[0].Priority != 0 || reordered[1].ID != first[0].ID || reordered[2].ID != first[2].ID {
		t.Errorf("ids did not follow their rules across a reorder: %+v vs %+v", first, reordered)
	}
	first = reordered

	w := do(t, h, http.MethodGet, "/v1/services/site/routes", testToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list-routes = %d: %s", w.Code, w.Body.String())
	}
	if errs := spec.validate("list-routes", w.Body.Bytes()); len(errs) > 0 {
		t.Fatalf("list-routes diverges from [routeWithCursor]: %v", errs)
	}
	type routeItem struct {
		Route  route
		Cursor string
	}
	listRoutes := func(query string) []routeItem {
		t.Helper()
		w := do(t, h, http.MethodGet, "/v1/services/site/routes"+query, testToken, "")
		if w.Code != http.StatusOK {
			t.Fatalf("list-routes%s = %d: %s", query, w.Code, w.Body.String())
		}
		var items []routeItem
		if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil {
			t.Fatal(err)
		}
		return items
	}
	if listed := listRoutes(""); len(listed) != 3 || listed[1].Route.ID != first[1].ID || listed[1].Cursor != first[1].ID {
		t.Fatalf("list-routes = %+v, want the PUT's ids as route ids and cursors", listed)
	}

	// Filters and paging keep priority order: a page resumes after its cursor.
	if listed := listRoutes("?type=redirect"); len(listed) != 2 || listed[0].Route.Priority != 1 || listed[1].Route.Priority != 2 {
		t.Fatalf("type=redirect = %+v, want priorities 1 and 2", listed)
	}
	if listed := listRoutes("?source=/app/*&destination=/index.html"); len(listed) != 1 || listed[0].Route.ID != first[0].ID {
		t.Fatalf("source+destination filter = %+v, want only the rewrite", listed)
	}
	if listed := listRoutes("?limit=1&cursor=" + first[0].ID); len(listed) != 1 || listed[0].Route.ID != first[1].ID {
		t.Fatalf("page after %s = %+v, want the priority-1 rule", first[0].ID, listed)
	}

	w = do(t, h, http.MethodPut, "/v1/services/site/headers", testToken, headers)
	if w.Code != http.StatusOK {
		t.Fatalf("update-headers = %d: %s", w.Code, w.Body.String())
	}
	if errs := spec.validate("update-headers", w.Body.Bytes()); len(errs) > 0 {
		t.Fatalf("update-headers diverges from [header]: %v", errs)
	}
	w = do(t, h, http.MethodGet, "/v1/services/site/headers?name=Cache-Control", testToken, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list-headers = %d: %s", w.Code, w.Body.String())
	}
	if errs := spec.validate("list-headers", w.Body.Bytes()); len(errs) > 0 {
		t.Fatalf("list-headers diverges from [headerWithCursor]: %v", errs)
	}
	var hdrs []struct {
		Header struct{ ID, Path, Name, Value string }
		Cursor string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &hdrs); err != nil || len(hdrs) != 1 || hdrs[0].Header.Path != "/*.css" || hdrs[0].Cursor != hdrs[0].Header.ID || hdrs[0].Header.ID == "" {
		t.Fatalf("list-headers?name=Cache-Control = %s, want the one css rule with cursor = id", w.Body.String())
	}
}
