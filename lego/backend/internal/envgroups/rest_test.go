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
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
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
