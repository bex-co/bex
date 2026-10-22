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
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func serveREST(svc *Service, method, path, body string) *httptest.ResponseRecorder {
	return serveRESTContext(testCtx(), svc, method, path, body)
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

	// Link a service => 200 with the updated group (Render's envGroup), and
	// the App picks up the group refs.
	w = serveREST(svc, "POST", "/v1/env-groups/"+g.ID+"/services/web", "")
	var linked struct {
		ID           string
		ServiceLinks []struct{ ID, Name, Type string }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &linked); w.Code != 200 || err != nil || linked.ID != g.ID || len(linked.ServiceLinks) != 1 || linked.ServiceLinks[0].Name != "web" {
		t.Fatalf("link => %d %s, want 200 with the group and its one link object", w.Code, w.Body.String())
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
		if _, err := svc.CreateEnvGroup(testCtx(), CreateEnvGroupRequest{Name: name}); err != nil {
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
		if _, err := svc.CreateEnvGroup(testCtx(), CreateEnvGroupRequest{Name: fmt.Sprintf("group-%02d", i)}); err != nil {
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
		// Well-formed ids: the REST list reveals each group (w8/182), which
		// authorizes it by id.
		groupsOf(store).Seed(storeRow(id.New(id.EnvGroup), group.name, group.owner, group.environment, group.created, group.updated))
	}
	return withGroups(&Service{
		Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: coretest.Members{"filter-user": {"tea-a", "tea-b"}}},
		Store: store,
	})
}

func storeRow(gid, name, owner, environment, created, updated string) store.EnvGroup {
	return store.EnvGroup{
		ID: gid, Name: name, TenantID: owner, EnvironmentID: environment,
		CreatedAt: mustTime(created), UpdatedAt: mustTime(updated),
	}
}

func serveFilterREST(svc *Service, path string) *httptest.ResponseRecorder {
	ctx := core.WithIdentity(testCtx(), core.Identity{Subject: "filter-user", Method: "session"})
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
	for _, name := range []string{"delta", "echo", "foxtrot"} {
		groupsOf(store).Seed(storeRow(id.New(id.EnvGroup), name, "tea-a", "", "2026-07-15T16:00:00Z", "2026-07-15T16:00:00Z"))
	}
	got := envGroupListNames(t, serveFilterREST(svc, "/v1/env-groups?name=delta,echo,foxtrot&limit=2"))
	if !slices.Equal(got, []string{"delta", "echo"}) {
		t.Fatalf("filtered page = %v, want a full two-item page", got)
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
		return core.DefaultTenant, nil
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
	var g renderEnvGroup
	if err := json.Unmarshal(w.Body.Bytes(), &g); err != nil {
		t.Fatal(err)
	}
	// The create response is Render's envGroup: every variable carries its value
	// and every file its content (w8/182).
	if !slices.Equal(g.ServiceLinks, []serviceLink{{ID: "web", Name: "web", Type: "web"}}) ||
		len(g.EnvVars) != 2 || g.EnvVars[0].Key != "GENERATED" || len(g.EnvVars[0].Value) != 44 ||
		g.EnvVars[1] != (EnvVarView{Key: "LITERAL", Value: "rest", ValueSet: true}) ||
		!slices.Equal(g.SecretFiles, []SecretFileView{{Name: "rest.txt", Content: "body"}}) {
		t.Fatalf("populated create response: %+v", g)
	}
	if got, err := svc.GetEnvGroupVar(testCtx(), g.ID, "LITERAL"); err != nil || got.Value != "rest" {
		t.Fatalf("literal round-trip: %+v err=%v", got, err)
	}
	if got, err := svc.GetEnvGroupVar(testCtx(), g.ID, "GENERATED"); err != nil || len(got.Value) != 44 {
		t.Fatalf("generated round-trip: len=%d err=%v", len(got.Value), err)
	}
}

func TestREST_SetEnvGroupVarAcceptsGenerateValue(t *testing.T) {
	svc := newService(newFakeStore())
	g, err := svc.CreateEnvGroup(testCtx(), CreateEnvGroupRequest{Name: "shared"})
	if err != nil {
		t.Fatal(err)
	}
	w := serveREST(svc, http.MethodPut, "/v1/env-groups/"+g.ID+"/env-vars/TOKEN", `{"generateValue":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("generated PUT: got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"value":`) {
		t.Fatalf("generated PUT leaked a value: %s", w.Body.String())
	}
	got, err := svc.GetEnvGroupVar(testCtx(), g.ID, "TOKEN")
	if err != nil || len(got.Value) != 44 {
		t.Fatalf("generated reveal: len=%d err=%v", len(got.Value), err)
	}
	w = serveREST(svc, http.MethodPut, "/v1/env-groups/"+g.ID+"/env-vars/BAD", `{"value":"literal","generateValue":true}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("conflicting generated PUT: got %d: %s", w.Code, w.Body.String())
	}
	var coded map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &coded); err != nil || coded["code"] != "ENVIRONMENT_VALUE_INPUT_INVALID" {
		t.Fatalf("conflicting generated PUT envelope: %v err=%v", coded, err)
	}
	w = serveREST(svc, http.MethodPut, "/v1/env-groups/"+g.ID+"/env-vars/MISSING", `{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing value intent PUT: got %d: %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &coded); err != nil || coded["code"] != "ENVIRONMENT_VALUE_INPUT_INVALID" {
		t.Fatalf("missing value intent PUT envelope: %v err=%v", coded, err)
	}
	w = serveREST(svc, http.MethodPut, "/v1/env-groups/"+g.ID+"/env-vars/EMPTY", `{"value":""}`)
	if w.Code != http.StatusOK {
		t.Fatalf("empty literal PUT: got %d: %s", w.Code, w.Body.String())
	}
	if got, err := svc.GetEnvGroupVar(testCtx(), g.ID, "EMPTY"); err != nil || got.Value != "" {
		t.Fatalf("empty literal reveal: %+v err=%v", got, err)
	}
	revealed := serveREST(svc, http.MethodGet, "/v1/env-groups/"+g.ID+"/env-vars/EMPTY", "")
	if revealed.Code != http.StatusOK || !strings.Contains(revealed.Body.String(), `"value":""`) {
		t.Fatalf("empty literal REST reveal: %d %s", revealed.Code, revealed.Body.String())
	}
}

func TestREST_DuplicateNameIs409AndPatchEnvironmentIsNamesOnly(t *testing.T) {
	svc := newService(newFakeStore())
	first := serveREST(svc, http.MethodPost, "/v1/env-groups", `{"name":"shared","envVars":[{"key":"A","value":"secret"}]}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", first.Code, first.Body.String())
	}
	var group EnvGroupView
	if err := json.Unmarshal(first.Body.Bytes(), &group); err != nil {
		t.Fatal(err)
	}
	duplicate := serveREST(svc, http.MethodPost, "/v1/env-groups", `{"name":"shared"}`)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status=%d body=%s", duplicate.Code, duplicate.Body.String())
	}
	patch := serveREST(svc, http.MethodPatch, "/v1/env-groups/"+group.ID+"/contents", `{
		"saveMode":"save_only",
		"expectedRevision":"`+group.Revision+`",
		"envVars":[{"key":"B","fromKey":"A"},{"key":"GENERATED","generateValue":true}],
		"secretFiles":[{"name":"token.txt","content":"file-secret"}]
	}`)
	if patch.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%s", patch.Code, patch.Body.String())
	}
	if body := patch.Body.String(); strings.Contains(body, "file-secret") || strings.Contains(body, `"value"`) || strings.Contains(body, `"content"`) {
		t.Fatalf("patch response leaked secret material: %s", body)
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
	if ids, _ := store.List(testCtx(), "env-groups"); len(ids) != 0 {
		t.Fatalf("invalid serviceId left orphan groups: %v", ids)
	}
}

// TestRevealedEmptyGroupFileCarriesItsContent (w4/326, a725): reading a present
// zero-byte group file answered {"name":…} with no content field over REST and
// MCP — indistinguishable from a names-only metadata row — while GraphQL said
// "". A content read now always carries content; metadata and the masked write
// acknowledgement stay names-only, and a missing file stays 404.
func TestRevealedEmptyGroupFileCarriesItsContent(t *testing.T) {
	svc := newService(newFakeStore(), sampleApp("web"))
	w := serveREST(svc, "POST", "/v1/env-groups", `{"name":"a725"}`)
	var g EnvGroupView
	if err := json.Unmarshal(w.Body.Bytes(), &g); w.Code != 201 || err != nil {
		t.Fatalf("create => %d %s", w.Code, w.Body.String())
	}
	for name, content := range map[string]string{"qa-a725-empty.txt": "", "qa-a725.txt": "雪\n"} {
		ack := serveREST(svc, "PUT", "/v1/env-groups/"+g.ID+"/secret-files/"+name, `{"content":`+strconv.Quote(content)+`}`)
		if ack.Code != 200 || strings.Contains(ack.Body.String(), `"content"`) {
			t.Fatalf("PUT %s => %d %s, want a names-only acknowledgement", name, ack.Code, ack.Body.String())
		}
	}
	for name, want := range map[string]string{"qa-a725-empty.txt": `{"name":"qa-a725-empty.txt","content":""}`, "qa-a725.txt": `{"name":"qa-a725.txt","content":"雪\n"}`} {
		read := serveREST(svc, "GET", "/v1/env-groups/"+g.ID+"/secret-files/"+name, "")
		var got, wantObj map[string]any
		_ = json.Unmarshal(read.Body.Bytes(), &got)
		_ = json.Unmarshal([]byte(want), &wantObj)
		if read.Code != 200 || !reflect.DeepEqual(got, wantObj) {
			t.Fatalf("GET %s => %d %s, want %s", name, read.Code, read.Body.String(), want)
		}
	}
	if c := serveREST(svc, "GET", "/v1/env-groups/"+g.ID+"/secret-files/missing.txt", "").Code; c != 404 {
		t.Fatalf("missing file => %d, want 404", c)
	}
	// The group read is Render's envGroup: each file carries its content, an
	// empty file's "" included (w8/182).
	detail := serveREST(svc, "GET", "/v1/env-groups/"+g.ID, "")
	if detail.Code != 200 || !strings.Contains(detail.Body.String(), `{"name":"qa-a725-empty.txt","content":""}`) ||
		!strings.Contains(detail.Body.String(), `"content":"雪\n"`) {
		t.Fatalf("group detail => %d %s, want files with their contents", detail.Code, detail.Body.String())
	}
}
