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

package envgroups

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func serveREST(svc *Service, method, path, body string) *httptest.ResponseRecorder {
	return serveRESTContext(context.Background(), svc, method, path, body)
}

func serveRESTContext(ctx context.Context, svc *Service, method, path, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	r = r.WithContext(ctx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, r)
	return rec
}

func TestREST_EnvGroupsLifecycle(t *testing.T) {
	svc := newService(newFakeStore(), sampleApp("web"))

	// Create => 201 with an evg- id.
	w := serveREST(svc, "POST", "/v1/env-groups", `{"name":"shared"}`)
	if w.Code != 201 {
		t.Fatalf("POST => 201, got %d: %s", w.Code, w.Body.String())
	}
	var g EnvGroupView
	_ = json.Unmarshal(w.Body.Bytes(), &g)
	if !strings.HasPrefix(g.ID, "evg-") || g.Name != "shared" {
		t.Fatalf("created group shape: %+v", g)
	}

	// Rename keeps the id and returns the updated group.
	w = serveREST(svc, "PATCH", "/v1/env-groups/"+g.ID, `{"name":"shared-prod"}`)
	if w.Code != 200 {
		t.Fatalf("PATCH => 200, got %d: %s", w.Code, w.Body.String())
	}
	var renamed EnvGroupView
	_ = json.Unmarshal(w.Body.Bytes(), &renamed)
	if renamed.ID != g.ID || renamed.Name != "shared-prod" {
		t.Fatalf("renamed group shape: %+v", renamed)
	}

	// Replace-all vars.
	if c := serveREST(svc, "PUT", "/v1/env-groups/"+g.ID+"/env-vars", `[{"key":"K","value":"v"}]`).Code; c != 200 {
		t.Fatalf("PUT env-vars => 200, got %d", c)
	}
	// Reveal one var.
	var one EnvVarView
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/env-groups/"+g.ID+"/env-vars/K", "").Body.Bytes(), &one)
	if one.Value != "v" {
		t.Fatalf("reveal var: %+v", one)
	}
	// Per-key update/delete preserves the collection endpoint's other variables.
	if c := serveREST(svc, "PUT", "/v1/env-groups/"+g.ID+"/env-vars/OTHER", `{"value":"two"}`).Code; c != 200 {
		t.Fatalf("PUT one env var => 200, got %d", c)
	}
	if c := serveREST(svc, "DELETE", "/v1/env-groups/"+g.ID+"/env-vars/K", "").Code; c != 204 {
		t.Fatalf("DELETE one env var => 204, got %d", c)
	}
	if c := serveREST(svc, "GET", "/v1/env-groups/"+g.ID+"/env-vars/OTHER", "").Code; c != 200 {
		t.Fatalf("sibling env var should remain, got %d", c)
	}

	// Link a service => 204, and the App picks up the group refs.
	if c := serveREST(svc, "POST", "/v1/env-groups/"+g.ID+"/services/web", "").Code; c != 204 {
		t.Fatalf("link => 204, got %d", c)
	}
	web := getApp(t, svc.Client, "web")
	if !slices.Contains(web.Spec.EnvFromSecrets, envSecretName(g.ID)) {
		t.Fatalf("link should project onto the service: %+v", web.Spec.EnvFromSecrets)
	}

	// List and get.
	var list []envGroupWithCursor
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/env-groups", "").Body.Bytes(), &list)
	if len(list) != 1 || list[0].EnvGroup.ID != g.ID || list[0].Cursor != g.ID {
		t.Fatalf("list: %+v", list)
	}

	// Unlink => 204; delete group => 204.
	if c := serveREST(svc, "DELETE", "/v1/env-groups/"+g.ID+"/services/web", "").Code; c != 204 {
		t.Fatalf("unlink => 204, got %d", c)
	}
	if c := serveREST(svc, "DELETE", "/v1/env-groups/"+g.ID, "").Code; c != 204 {
		t.Fatalf("delete => 204, got %d", c)
	}
	if c := serveREST(svc, "GET", "/v1/env-groups/"+g.ID, "").Code; c != 404 {
		t.Fatalf("deleted group GET => 404, got %d", c)
	}
}

func TestREST_EnvGroupsPaginationWalkUsesRenderEnvelope(t *testing.T) {
	svc := newService(newFakeStore())
	for _, name := range []string{"echo", "alpha", "delta", "bravo", "charlie"} {
		if _, err := svc.CreateEnvGroup(context.Background(), CreateEnvGroupRequest{Name: name}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}

	var got []string
	cursor := ""
	for pageNo := 0; pageNo < 10; pageNo++ {
		path := "/v1/env-groups?limit=2"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		w := serveREST(svc, http.MethodGet, path, "")
		if w.Code != http.StatusOK {
			t.Fatalf("page %d status = %d: %s", pageNo, w.Code, w.Body.String())
		}
		var page []envGroupWithCursor
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatalf("page %d decode Render envelope: %v (%s)", pageNo, err, w.Body.String())
		}
		if len(page) == 0 {
			break
		}
		if len(page) > 2 {
			t.Fatalf("page %d length = %d, want <= 2", pageNo, len(page))
		}
		for _, item := range page {
			if item.EnvGroup.ID == "" || item.Cursor != item.EnvGroup.ID {
				t.Fatalf("page %d item is not {envGroup,cursor}: %+v", pageNo, item)
			}
			if slices.Contains(got, item.EnvGroup.ID) {
				t.Fatalf("pagination duplicated %s across pages: %v", item.EnvGroup.ID, got)
			}
			got = append(got, item.EnvGroup.ID)
		}
		cursor = page[len(page)-1].Cursor
	}
	if len(got) != 5 {
		t.Fatalf("pagination walk returned %d groups, want 5: %v", len(got), got)
	}
}

func TestREST_EnvGroupsAbsentLimitUsesRenderDefault(t *testing.T) {
	svc := newService(newFakeStore())
	for i := 0; i < core.DefaultPageLimit+1; i++ {
		if _, err := svc.CreateEnvGroup(context.Background(), CreateEnvGroupRequest{Name: fmt.Sprintf("group-%02d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	w := serveREST(svc, http.MethodGet, "/v1/env-groups", "")
	var page []envGroupWithCursor
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page) != core.DefaultPageLimit {
		t.Fatalf("absent limit returned %d groups, want Render default %d", len(page), core.DefaultPageLimit)
	}
}

func filterTestService(t *testing.T) *Service {
	t.Helper()
	store := newFakeStore()
	for _, group := range []struct {
		id, name, owner, environment, created, updated string
	}{
		{"evg-01", "alpha", "tea-a", "env-one", "2026-07-15T10:00:00Z", "2026-07-15T11:00:00Z"},
		{"evg-02", "bravo", "tea-a", "env-two", "2026-07-15T12:00:00Z", "2026-07-15T13:00:00Z"},
		{"evg-03", "charlie", "tea-b", "env-one", "2026-07-15T14:00:00Z", "2026-07-15T15:00:00Z"},
	} {
		if err := store.Put(context.Background(), metaPath(group.id), map[string]string{
			"name": group.name, "workspace": group.owner, "environment": group.environment,
			"createdAt": group.created, "updatedAt": group.updated,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return &Service{
		Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: multiWorkspace{"filter-user": {"tea-a", "tea-b"}}},
		Store: store,
	}
}

func serveFilterREST(svc *Service, path string) *httptest.ResponseRecorder {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "filter-user", Method: "session"})
	return serveRESTContext(ctx, svc, http.MethodGet, path, "")
}

func envGroupListNames(t *testing.T, response *httptest.ResponseRecorder) []string {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("GET = %d: %s", response.Code, response.Body.String())
	}
	var page []envGroupWithCursor
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(page))
	for _, item := range page {
		names = append(names, item.EnvGroup.Name)
	}
	return names
}

func TestREST_EnvGroupListFilters(t *testing.T) {
	tests := []struct {
		name, query string
		want        []string
	}{
		{"name repeated and comma OR", "name=alpha,missing&name=bravo", []string{"alpha", "bravo"}},
		{"ownerId narrows", "ownerId=tea-b", []string{"charlie"}},
		{"ownerId repeated OR", "ownerId=tea-a&ownerId=tea-b", []string{"alpha", "bravo", "charlie"}},
		{"environmentId repeated and comma OR", "ownerId=tea-a,tea-b&environmentId=env-one,missing&environmentId=also-missing", []string{"alpha", "charlie"}},
		{"createdBefore", "createdBefore=2026-07-15T11:00:00Z", []string{"alpha"}},
		{"createdAfter", "createdAfter=2026-07-15T11:00:00Z", []string{"bravo"}},
		{"updatedBefore", "updatedBefore=2026-07-15T12:00:00Z", []string{"alpha"}},
		{"updatedAfter", "updatedAfter=2026-07-15T12:00:00Z", []string{"bravo"}},
		{"different params compose with AND", "ownerId=tea-a,tea-b&name=alpha,charlie&environmentId=env-one&createdAfter=2026-07-15T09:00:00Z", []string{"alpha", "charlie"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := envGroupListNames(t, serveFilterREST(filterTestService(t), "/v1/env-groups?"+tc.query))
			if !slices.Equal(got, tc.want) {
				t.Fatalf("names = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestREST_EnvGroupListRejectsInvalidTimestampsByName(t *testing.T) {
	for _, key := range []string{"createdBefore", "createdAfter", "updatedBefore", "updatedAfter"} {
		t.Run(key, func(t *testing.T) {
			response := serveFilterREST(filterTestService(t), "/v1/env-groups?"+key+"=yesterday")
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			message, _ := body["message"].(string)
			if response.Code != http.StatusBadRequest || body["id"] != "bad_request" || !strings.Contains(message, key) {
				t.Fatalf("%s = %d: %s; want named 400", key, response.Code, response.Body.String())
			}
		})
	}
}

func TestREST_EnvGroupFiltersBeforePagination(t *testing.T) {
	svc := filterTestService(t)
	store := svc.Store.(*fakeStore)
	for i, name := range []string{"delta", "echo", "foxtrot"} {
		id := fmt.Sprintf("evg-%02d", i+4)
		if err := store.Put(context.Background(), metaPath(id), map[string]string{
			"name": name, "workspace": "tea-a", "createdAt": "2026-07-15T16:00:00Z", "updatedAt": "2026-07-15T16:00:00Z",
		}); err != nil {
			t.Fatal(err)
		}
	}
	got := envGroupListNames(t, serveFilterREST(svc, "/v1/env-groups?name=delta,echo,foxtrot&limit=2"))
	if !slices.Equal(got, []string{"delta", "echo"}) {
		t.Fatalf("filtered page = %v, want a full two-item page", got)
	}
}

func TestREST_EnvGroupLegacyEmptyTimestampPassesTimeFilter(t *testing.T) {
	// Legacy env groups pre-dating w6/m24's timestamp stamping have empty
	// createdAt/updatedAt. Before w2/m51 matchesTimeWindow returned false on a
	// parse error, silently excluding them from any time-filtered list.
	// After the fix: an empty timestamp passes any time window (it can't be
	// placed in one, so it is not excluded — omitted data ≠ excluded).
	store := newFakeStore()
	// Legacy group: no timestamps at all.
	if err := store.Put(context.Background(), metaPath("evg-legacy"), map[string]string{
		"name": "legacy", "workspace": "tea-a",
	}); err != nil {
		t.Fatal(err)
	}
	svc := &Service{
		Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: multiWorkspace{"filter-user": {"tea-a"}}},
		Store: store,
	}
	// A createdBefore filter that excludes all known-created groups must still
	// return the legacy group (empty timestamp → pass).
	got := envGroupListNames(t, serveFilterREST(svc, "/v1/env-groups?createdBefore=2020-01-01T00:00:00Z"))
	if !slices.Equal(got, []string{"legacy"}) {
		t.Fatalf("legacy group with empty createdAt should pass time filter, got %v", got)
	}
	// Same for updatedBefore.
	got = envGroupListNames(t, serveFilterREST(svc, "/v1/env-groups?updatedBefore=2020-01-01T00:00:00Z"))
	if !slices.Equal(got, []string{"legacy"}) {
		t.Fatalf("legacy group with empty updatedAt should pass time filter, got %v", got)
	}
}

func TestREST_EnvGroupsUnconfigured503(t *testing.T) {
	svc := newService(nil)
	if c := serveREST(svc, "GET", "/v1/env-groups", "").Code; c != 503 {
		t.Errorf("GET without a store => 503, got %d", c)
	}
}

func TestREST_CreateEnvGroupAcceptsEnvironmentID(t *testing.T) {
	svc := newService(newFakeStore())
	svc.EnvironmentWorkspace = func(_ context.Context, environmentID string) (string, error) {
		if environmentID != "env-alpha" {
			return "", core.ErrNotFound
		}
		return "", nil // store-off group owner is also empty
	}
	w := serveREST(svc, "POST", "/v1/env-groups", `{"name":"shared","environmentId":"env-alpha"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST with environmentId: got %d: %s", w.Code, w.Body.String())
	}
	var g EnvGroupView
	if err := json.Unmarshal(w.Body.Bytes(), &g); err != nil {
		t.Fatal(err)
	}
	if g.EnvironmentID != "env-alpha" {
		t.Fatalf("environmentId = %q, want env-alpha", g.EnvironmentID)
	}
}

func TestREST_CreateEnvGroupAcceptsInitialContentsAndLinks(t *testing.T) {
	svc := newService(newFakeStore(), sampleApp("web"))
	w := serveREST(svc, "POST", "/v1/env-groups", `{
		"name":"shared",
		"envVars":[{"key":"LITERAL","value":"rest"},{"key":"GENERATED","generateValue":true}],
		"secretFiles":[{"name":"rest.txt","content":"body"}],
		"serviceIds":["web"]
	}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST populated create: got %d: %s", w.Code, w.Body.String())
	}
	var g EnvGroupView
	if err := json.Unmarshal(w.Body.Bytes(), &g); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(g.ServiceLinks, []string{"web"}) ||
		!slices.Equal(g.EnvVars, []EnvVarView{{Key: "GENERATED"}, {Key: "LITERAL"}}) ||
		!slices.Equal(g.SecretFiles, []SecretFileView{{Name: "rest.txt"}}) {
		t.Fatalf("populated create response: %+v", g)
	}
	if got, err := svc.GetEnvGroupVar(context.Background(), g.ID, "LITERAL"); err != nil || got.Value != "rest" {
		t.Fatalf("literal round-trip: %+v err=%v", got, err)
	}
	if got, err := svc.GetEnvGroupVar(context.Background(), g.ID, "GENERATED"); err != nil || len(got.Value) != 44 {
		t.Fatalf("generated round-trip: len=%d err=%v", len(got.Value), err)
	}
}

func TestREST_CreateEnvGroupInvalidServiceIsNamed404WithNoOrphan(t *testing.T) {
	store := newFakeStore()
	svc := newService(store)
	w := serveREST(svc, "POST", "/v1/env-groups", `{
		"name":"shared","envVars":[{"key":"TOKEN","value":"secret"}],"serviceIds":["srv-missing"]
	}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("POST invalid serviceId: got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	message, _ := body["message"].(string)
	if body["id"] != "not_found" || !strings.Contains(message, `serviceId "srv-missing"`) {
		t.Fatalf("named Render error envelope: %v", body)
	}
	if ids, _ := store.List(context.Background(), "env-groups"); len(ids) != 0 {
		t.Fatalf("invalid serviceId left orphan groups: %v", ids)
	}
}
