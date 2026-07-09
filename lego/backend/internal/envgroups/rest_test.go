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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
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

	// Link a service => 204, and the App picks up the group refs.
	if c := serveREST(svc, "POST", "/v1/env-groups/"+g.ID+"/services/web", "").Code; c != 204 {
		t.Fatalf("link => 204, got %d", c)
	}
	web := getApp(t, svc.Client, "web")
	if !slices.Contains(web.Spec.EnvFromSecrets, envSecretName(g.ID)) {
		t.Fatalf("link should project onto the service: %+v", web.Spec.EnvFromSecrets)
	}

	// List and get.
	var list []EnvGroupView
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/env-groups", "").Body.Bytes(), &list)
	if len(list) != 1 || list[0].ID != g.ID {
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

func TestREST_EnvGroupsUnconfigured503(t *testing.T) {
	svc := newService(nil)
	if c := serveREST(svc, "GET", "/v1/env-groups", "").Code; c != 503 {
		t.Errorf("GET without a store => 503, got %d", c)
	}
}
