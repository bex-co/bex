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

package postgres

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
)

// metadata_rest_test.go is w9/m41/t005's REST-boundary metadata-parity test:
// every response body a Postgres REST handler can write (list/get/create/
// update/suspend/resume) must carry the same nested `owner`/`region`/
// `dashboardUrl`/`updatedAt` shape internal/apps and internal/keyvalue also
// promise (resourcemeta.Config, docs/ADR006-bex-api.md § Resource metadata
// contract). It is deliberately NOT a snapshot: it decodes each response into
// a bare map[string]any and checks the specific keys/shape, so a future
// handler that regresses to a bare PostgresView (no owner/region/dashboardUrl
// keys at all) fails every assertion below, not just an id/name check.

// fixedOwnerResolver is a minimal resourcemeta.OwnerResolver a test can seed
// directly, without pulling in the real workspaces.Service.
type fixedOwnerResolver map[string]resourcemeta.Owner

func (f fixedOwnerResolver) ResolveResourceOwners(_ context.Context, ids []string) map[string]resourcemeta.Owner {
	out := map[string]resourcemeta.Owner{}
	for _, id := range ids {
		if o, ok := f[id]; ok {
			out[id] = o
		}
	}
	return out
}

func decodeMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return m
}

func assertOwnerPresent(t *testing.T, step string, body map[string]any, want resourcemeta.Owner) {
	t.Helper()
	owner, ok := body["owner"].(map[string]any)
	if !ok {
		t.Fatalf("%s: owner missing or wrong shape in %#v", step, body)
	}
	if owner["id"] != want.ID || owner["name"] != want.Name || owner["email"] != want.Email || owner["type"] != want.Type {
		t.Fatalf("%s: owner = %#v, want {id:%s name:%s email:%s type:%s}", step, owner, want.ID, want.Name, want.Email, want.Type)
	}
}

func assertOwnerAbsent(t *testing.T, step string, body map[string]any) {
	t.Helper()
	if v, ok := body["owner"]; ok {
		t.Fatalf("%s: owner should be omitted (unresolved), got %#v", step, v)
	}
}

func assertStringField(t *testing.T, step string, body map[string]any, key, want string) {
	t.Helper()
	got, _ := body[key].(string)
	if got != want {
		t.Fatalf("%s: %s = %q, want %q (body=%#v)", step, key, got, want, body)
	}
}

func assertFieldAbsent(t *testing.T, step string, body map[string]any, key string) {
	t.Helper()
	if v, ok := body[key]; ok {
		t.Fatalf("%s: %s should be omitted (unconfigured), got %#v", step, key, v)
	}
}

func assertNonEmptyString(t *testing.T, step string, body map[string]any, key string) {
	t.Helper()
	got, _ := body[key].(string)
	if got == "" {
		t.Fatalf("%s: %s missing/empty, want a stamped value (body=%#v)", step, key, body)
	}
}

// TestRESTPostgresMetadataParity_Present exercises create/list/get/update/
// suspend/resume with a resolvable owner and a configured region + dashboard
// base URL, and checks owner/region/dashboardUrl/updatedAt on every one.
func TestRESTPostgresMetadataParity_Present(t *testing.T) {
	svc, _ := newService()
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	wantOwner := resourcemeta.Owner{ID: "tea-a", Name: "Acme", Email: "a@acme.test", Type: "team"}
	svc.Owners = fixedOwnerResolver{"tea-a": wantOwner}
	svc.Metadata = resourcemeta.Config{Region: "fsn1", DashboardBaseURL: "https://dashboard.bex.co"}

	check := func(step string, body map[string]any) {
		t.Helper()
		assertOwnerPresent(t, step, body, wantOwner)
		assertStringField(t, step, body, "region", "fsn1")
		assertNonEmptyString(t, step, body, "dashboardUrl")
		assertNonEmptyString(t, step, body, "updatedAt")
	}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/v1/postgres", strings.NewReader(`{"name":"pg-meta","plan":"free"}`))
	mux.ServeHTTP(createRec, createReq.WithContext(ctxAs("user-a")))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", createRec.Code, createRec.Body.String())
	}
	createBody := decodeMap(t, createRec.Body.Bytes())
	check("create", createBody)
	id, _ := createBody["id"].(string)
	if id == "" {
		t.Fatalf("create response has no id: %#v", createBody)
	}

	var list []map[string]any
	if err := json.Unmarshal(serveREST(svc, "GET", "/v1/postgres", "").Body.Bytes(), &list); err != nil {
		t.Fatalf("list decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list = %d items, want 1: %#v", len(list), list)
	}
	item, ok := list[0]["postgres"].(map[string]any)
	if !ok {
		t.Fatalf("list[0] has no nested postgres object: %#v", list[0])
	}
	check("list", item)

	check("get", decodeMap(t, serveREST(svc, "GET", "/v1/postgres/"+id, "").Body.Bytes()))

	patchRec := serveREST(svc, "PATCH", "/v1/postgres/"+id, `{"name":"pg-meta-renamed"}`)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch => %d: %s", patchRec.Code, patchRec.Body.String())
	}
	check("update", decodeMap(t, patchRec.Body.Bytes()))

	suspendRec := serveREST(svc, "POST", "/v1/postgres/"+id+"/suspend", "")
	if suspendRec.Code != http.StatusAccepted {
		t.Fatalf("suspend => %d: %s", suspendRec.Code, suspendRec.Body.String())
	}
	check("suspend", decodeMap(t, suspendRec.Body.Bytes()))

	resumeRec := serveREST(svc, "POST", "/v1/postgres/"+id+"/resume", "")
	if resumeRec.Code != http.StatusAccepted {
		t.Fatalf("resume => %d: %s", resumeRec.Code, resumeRec.Body.String())
	}
	check("resume", decodeMap(t, resumeRec.Body.Bytes()))
}

// TestRESTPostgresMetadataParity_OmittedWhenUnresolved is the omission half:
// no owner resolver, no region, no dashboard base URL configured. owner/
// region/dashboardUrl must be OMITTED (absent keys), never faked as zero
// values — while updatedAt still stamps on every mutation.
func TestRESTPostgresMetadataParity_OmittedWhenUnresolved(t *testing.T) {
	svc, _ := newService() // no Workspace, no Owners, no Metadata

	check := func(step string, body map[string]any) {
		t.Helper()
		assertOwnerAbsent(t, step, body)
		assertFieldAbsent(t, step, body, "region")
		assertFieldAbsent(t, step, body, "dashboardUrl")
		assertNonEmptyString(t, step, body, "updatedAt")
	}

	createRec := serveREST(svc, "POST", "/v1/postgres", `{"name":"pg-bare","plan":"free"}`)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", createRec.Code, createRec.Body.String())
	}
	createBody := decodeMap(t, createRec.Body.Bytes())
	check("create", createBody)
	id, _ := createBody["id"].(string)

	var list []map[string]any
	if err := json.Unmarshal(serveREST(svc, "GET", "/v1/postgres", "").Body.Bytes(), &list); err != nil {
		t.Fatalf("list decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list = %d items, want 1: %#v", len(list), list)
	}
	item, ok := list[0]["postgres"].(map[string]any)
	if !ok {
		t.Fatalf("list[0] has no nested postgres object: %#v", list[0])
	}
	check("list", item)

	check("get", decodeMap(t, serveREST(svc, "GET", "/v1/postgres/"+id, "").Body.Bytes()))

	patchRec := serveREST(svc, "PATCH", "/v1/postgres/"+id, `{"name":"pg-bare-renamed"}`)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch => %d: %s", patchRec.Code, patchRec.Body.String())
	}
	check("update", decodeMap(t, patchRec.Body.Bytes()))

	suspendRec := serveREST(svc, "POST", "/v1/postgres/"+id+"/suspend", "")
	if suspendRec.Code != http.StatusAccepted {
		t.Fatalf("suspend => %d: %s", suspendRec.Code, suspendRec.Body.String())
	}
	check("suspend", decodeMap(t, suspendRec.Body.Bytes()))

	resumeRec := serveREST(svc, "POST", "/v1/postgres/"+id+"/resume", "")
	if resumeRec.Code != http.StatusAccepted {
		t.Fatalf("resume => %d: %s", resumeRec.Code, resumeRec.Body.String())
	}
	check("resume", decodeMap(t, resumeRec.Body.Bytes()))
}
