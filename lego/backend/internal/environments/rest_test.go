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

package environments

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestToRenderEnvironmentUsesOfficialCLIFields(t *testing.T) {
	got := toRenderEnvironment(EnvironmentView{
		ID: "env-1", ProjectID: "prj-1", Name: "staging", ServiceIDs: []string{"web"},
		DatabaseIDs: []string{"db"}, KeyValueIDs: []string{"kv"}, IPAllowList: []string{"10.0.0.0/8"},
	})
	if got.ID != "env-1" || len(got.ServiceIDs) != 1 || len(got.DatabasesIDs) != 1 || len(got.RedisIDs) != 1 {
		t.Fatalf("Render environment = %+v", got)
	}
	if len(got.IPAllowList) != 1 || got.IPAllowList[0].CIDRBlock != "10.0.0.0/8" {
		t.Fatalf("Render ipAllowList = %+v", got.IPAllowList)
	}
}

// restHarness stands up a Service (fake store, fake k8s client) and its REST
// mux, pre-seeded with project prj-1 — the w4/017 create/update tests below
// all start from here.
func restHarness(t *testing.T) (*Service, *http.ServeMux) {
	t.Helper()
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, _ := newServiceWithClient(st)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return svc, mux
}

func doREST(t *testing.T, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
	return rec
}

// TestREST_CreateAcceptsRenderACLObjects is w4/017: Render's create body
// carries the ACL triple with ipAllowList as [{cidrBlock, description}]
// objects — bex accepts the object form on the standard POST (description
// discarded — the apps/postgres/keyvalue convention), not just string CIDRs
// through the bex-only /acl route.
func TestREST_CreateAcceptsRenderACLObjects(t *testing.T) {
	_, mux := restHarness(t)
	rec := doREST(t, mux, "POST", "/v1/environments", `{
		"name": "staging", "projectId": "prj-1",
		"protectedStatus": "protected",
		"networkIsolationEnabled": true,
		"ipAllowList": [{"cidrBlock": "10.0.0.0/8", "description": "office"}]
	}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var got renderEnvironment
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ProtectedStatus != "protected" || !got.NetworkIsolationEnabled {
		t.Errorf("created ACL = %q/%v, want protected/true", got.ProtectedStatus, got.NetworkIsolationEnabled)
	}
	if len(got.IPAllowList) != 1 || got.IPAllowList[0].CIDRBlock != "10.0.0.0/8" {
		t.Errorf("created ipAllowList = %+v, want the object-form CIDR echoed", got.IPAllowList)
	}
}

// TestREST_CreateRejectsBadACLWithoutOrphan: a bad CIDR (or protectedStatus)
// in the create body is a clean 400 and the environment must NOT have been
// created — the ACL is validated before the row exists.
func TestREST_CreateRejectsBadACLWithoutOrphan(t *testing.T) {
	svc, mux := restHarness(t)
	rec := doREST(t, mux, "POST", "/v1/environments", `{
		"name": "staging", "projectId": "prj-1",
		"ipAllowList": [{"cidrBlock": "not-a-cidr", "description": ""}]
	}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST with bad CIDR = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	list, err := svc.List(ctxAs("user-a"), "prj-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("environment created despite the 400: %+v", list)
	}
}

// TestREST_PatchMergesRenderACLFields: Render's PATCH is per-field partial —
// naming only ipAllowList must keep the current protectedStatus/
// networkIsolationEnabled (SetACL itself is full-replace, so the handler
// merges); naming only name must not touch the ACL; an empty body is 400.
func TestREST_PatchMergesRenderACLFields(t *testing.T) {
	svc, mux := restHarness(t)
	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusProtected, true, []string{"10.0.0.0/8"}); err != nil {
		t.Fatalf("SetACL: %v", err)
	}

	// Only ipAllowList: protectedStatus/networkIsolationEnabled survive.
	rec := doREST(t, mux, "PATCH", "/v1/environments/"+e.ID,
		`{"ipAllowList": [{"cidrBlock": "192.168.0.0/16", "description": "vpn"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH ipAllowList = %d (body: %s)", rec.Code, rec.Body.String())
	}
	var got renderEnvironment
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ProtectedStatus != ProtectedStatusProtected || !got.NetworkIsolationEnabled {
		t.Errorf("ACL siblings not preserved: %q/%v, want protected/true", got.ProtectedStatus, got.NetworkIsolationEnabled)
	}
	if len(got.IPAllowList) != 1 || got.IPAllowList[0].CIDRBlock != "192.168.0.0/16" {
		t.Errorf("ipAllowList = %+v, want the replaced CIDR", got.IPAllowList)
	}

	// networkIsolationEnabled:false must be appliable (absent != false).
	rec = doREST(t, mux, "PATCH", "/v1/environments/"+e.ID, `{"networkIsolationEnabled": false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH networkIsolationEnabled=false = %d (body: %s)", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.NetworkIsolationEnabled || got.ProtectedStatus != ProtectedStatusProtected || len(got.IPAllowList) != 1 {
		t.Errorf("after isolation-off PATCH: %+v, want only that field changed", got)
	}

	// Only name: rename, ACL untouched.
	rec = doREST(t, mux, "PATCH", "/v1/environments/"+e.ID, `{"name": "production"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH name = %d (body: %s)", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "production" || got.ProtectedStatus != ProtectedStatusProtected {
		t.Errorf("rename-only PATCH: %+v, want name changed and ACL untouched", got)
	}

	// Empty body names nothing: 400, same as before w4/017.
	if rec := doREST(t, mux, "PATCH", "/v1/environments/"+e.ID, `{}`); rec.Code != http.StatusBadRequest {
		t.Errorf("PATCH {} = %d, want 400", rec.Code)
	}
}
