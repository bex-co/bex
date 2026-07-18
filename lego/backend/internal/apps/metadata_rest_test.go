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
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
)

// metadata_rest_test.go is w9/m41/t005's REST-boundary metadata-parity test —
// the Service sibling of postgres/metadata_rest_test.go and
// keyvalue/metadata_rest_test.go. Every response body a Service REST handler
// can write (list/get/create/update/suspend/resume) must carry the same
// `owner`/`updatedAt`/`dashboardUrl` shape (nested `owner`, top-level
// `dashboardUrl`/`updatedAt`) as its siblings, and `region` at Render's own
// Service location — `serviceDetails.region`, not top-level (documented,
// docs/ADR006-bex-api.md § Resource metadata contract) — with identical
// omission behavior when unresolved/unconfigured. Decodes each response into
// a bare map[string]any and checks specific keys/shape (not a snapshot), so a
// handler that regresses to a bare AppView (no owner/dashboardUrl keys at
// all) fails here.

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

// serviceDetailsRegion reads Render's serviceDetails.region location (a
// Service divergence from Postgres/KeyValue's top-level `region` — both
// documented in docs/ADR006-bex-api.md § Resource metadata contract).
func serviceDetailsRegion(body map[string]any) (string, bool) {
	sd, ok := body["serviceDetails"].(map[string]any)
	if !ok {
		return "", false
	}
	region, ok := sd["region"].(string)
	return region, ok
}

func assertNonEmptyString(t *testing.T, step string, body map[string]any, key string) {
	t.Helper()
	got, _ := body[key].(string)
	if got == "" {
		t.Fatalf("%s: %s missing/empty, want a stamped value (body=%#v)", step, key, body)
	}
}

func assertFieldAbsent(t *testing.T, step string, body map[string]any, key string) {
	t.Helper()
	if v, ok := body[key]; ok {
		t.Fatalf("%s: %s should be omitted (unconfigured), got %#v", step, key, v)
	}
}

// TestRESTServiceMetadataParity_Present exercises create/list/get/update/
// suspend/resume with a resolvable owner and a configured region + dashboard
// base URL, and checks owner/region/dashboardUrl/updatedAt on every one.
func TestRESTServiceMetadataParity_Present(t *testing.T) {
	svc := danaService()
	wantOwner := resourcemeta.Owner{ID: "tea-2", Name: "Acme", Email: "a@acme.test", Type: "team"}
	svc.Owners = fixedOwnerResolver{"tea-2": wantOwner}
	svc.Metadata = resourcemeta.Config{Region: "fsn1", DashboardBaseURL: "https://dashboard.bex.co"}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	check := func(step string, body map[string]any) {
		t.Helper()
		assertOwnerPresent(t, step, body, wantOwner)
		if region, ok := serviceDetailsRegion(body); !ok || region != "fsn1" {
			t.Fatalf("%s: serviceDetails.region = %q (present=%v), want fsn1 (body=%#v)", step, region, ok, body)
		}
		assertNonEmptyString(t, step, body, "dashboardUrl")
		assertNonEmptyString(t, step, body, "updatedAt")
	}

	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(
		`{"name":"web-meta","image":{"imagePath":"nginx"},"ownerId":"tea-2"}`))
	mux.ServeHTTP(createRec, createReq.WithContext(ctxAs("dana")))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", createRec.Code, createRec.Body.String())
	}
	createEnvelope := decodeMap(t, createRec.Body.Bytes())
	createBody, ok := createEnvelope["service"].(map[string]any)
	if !ok {
		t.Fatalf("create response has no nested service object: %#v", createEnvelope)
	}
	check("create", createBody)
	id, _ := createBody["id"].(string)
	if id == "" {
		t.Fatalf("create response has no id: %#v", createBody)
	}

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/v1/services?ownerId=tea-2", nil)
	mux.ServeHTTP(listRec, listReq.WithContext(ctxAs("dana")))
	var list []map[string]any
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("list decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list = %d items, want 1: %#v", len(list), list)
	}
	item, ok := list[0]["service"].(map[string]any)
	if !ok {
		t.Fatalf("list[0] has no nested service object: %#v", list[0])
	}
	check("list", item)

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/v1/services/"+id, nil)
	mux.ServeHTTP(getRec, getReq.WithContext(ctxAs("dana")))
	check("get", decodeMap(t, getRec.Body.Bytes()))

	patchRec := httptest.NewRecorder()
	patchReq := httptest.NewRequest(http.MethodPatch, "/v1/services/"+id, strings.NewReader(`{"name":"web-meta-renamed"}`))
	mux.ServeHTTP(patchRec, patchReq.WithContext(ctxAs("dana")))
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch => %d: %s", patchRec.Code, patchRec.Body.String())
	}
	check("update", decodeMap(t, patchRec.Body.Bytes()))

	suspendRec := httptest.NewRecorder()
	suspendReq := httptest.NewRequest(http.MethodPost, "/v1/services/"+id+"/suspend", nil)
	mux.ServeHTTP(suspendRec, suspendReq.WithContext(ctxAs("dana")))
	if suspendRec.Code != http.StatusAccepted {
		t.Fatalf("suspend => %d: %s", suspendRec.Code, suspendRec.Body.String())
	}
	check("suspend", decodeMap(t, suspendRec.Body.Bytes()))

	resumeRec := httptest.NewRecorder()
	resumeReq := httptest.NewRequest(http.MethodPost, "/v1/services/"+id+"/resume", nil)
	mux.ServeHTTP(resumeRec, resumeReq.WithContext(ctxAs("dana")))
	if resumeRec.Code != http.StatusAccepted {
		t.Fatalf("resume => %d: %s", resumeRec.Code, resumeRec.Body.String())
	}
	check("resume", decodeMap(t, resumeRec.Body.Bytes()))
}

// TestRESTServiceMetadataParity_OmittedWhenUnresolved is the omission half:
// no owner resolver, no region, no dashboard base URL configured. owner/
// region/dashboardUrl must be OMITTED (absent keys), never faked as zero
// values — while updatedAt still stamps on every mutation.
func TestRESTServiceMetadataParity_OmittedWhenUnresolved(t *testing.T) {
	svc := danaService() // Owners/Metadata left zero-valued
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	check := func(step string, body map[string]any) {
		t.Helper()
		assertOwnerAbsent(t, step, body)
		if region, ok := serviceDetailsRegion(body); ok {
			t.Fatalf("%s: serviceDetails.region should be omitted (unconfigured), got %q", step, region)
		}
		assertFieldAbsent(t, step, body, "dashboardUrl")
		assertNonEmptyString(t, step, body, "updatedAt")
	}

	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(
		`{"name":"web-bare","image":{"imagePath":"nginx"}}`))
	mux.ServeHTTP(createRec, createReq.WithContext(ctxAs("dana")))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", createRec.Code, createRec.Body.String())
	}
	createEnvelope := decodeMap(t, createRec.Body.Bytes())
	createBody, ok := createEnvelope["service"].(map[string]any)
	if !ok {
		t.Fatalf("create response has no nested service object: %#v", createEnvelope)
	}
	check("create", createBody)
	id, _ := createBody["id"].(string)

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/v1/services", nil)
	mux.ServeHTTP(listRec, listReq.WithContext(ctxAs("dana")))
	var list []map[string]any
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("list decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list = %d items, want 1: %#v", len(list), list)
	}
	item, ok := list[0]["service"].(map[string]any)
	if !ok {
		t.Fatalf("list[0] has no nested service object: %#v", list[0])
	}
	check("list", item)

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/v1/services/"+id, nil)
	mux.ServeHTTP(getRec, getReq.WithContext(ctxAs("dana")))
	check("get", decodeMap(t, getRec.Body.Bytes()))

	patchRec := httptest.NewRecorder()
	patchReq := httptest.NewRequest(http.MethodPatch, "/v1/services/"+id, strings.NewReader(`{"name":"web-bare-renamed"}`))
	mux.ServeHTTP(patchRec, patchReq.WithContext(ctxAs("dana")))
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch => %d: %s", patchRec.Code, patchRec.Body.String())
	}
	check("update", decodeMap(t, patchRec.Body.Bytes()))

	suspendRec := httptest.NewRecorder()
	suspendReq := httptest.NewRequest(http.MethodPost, "/v1/services/"+id+"/suspend", nil)
	mux.ServeHTTP(suspendRec, suspendReq.WithContext(ctxAs("dana")))
	if suspendRec.Code != http.StatusAccepted {
		t.Fatalf("suspend => %d: %s", suspendRec.Code, suspendRec.Body.String())
	}
	check("suspend", decodeMap(t, suspendRec.Body.Bytes()))

	resumeRec := httptest.NewRecorder()
	resumeReq := httptest.NewRequest(http.MethodPost, "/v1/services/"+id+"/resume", nil)
	mux.ServeHTTP(resumeRec, resumeReq.WithContext(ctxAs("dana")))
	if resumeRec.Code != http.StatusAccepted {
		t.Fatalf("resume => %d: %s", resumeRec.Code, resumeRec.Body.String())
	}
	check("resume", decodeMap(t, resumeRec.Body.Bytes()))
}

// TestGraphQLServiceListMetadata exposes the same authoritative placement and
// update-time facts the REST Service shape carries. Region is nullable when the
// installation does not configure BEX_REGION; updatedAt and runtime remain
// independent server facts rather than dashboard derivations.
func TestGraphQLServiceListMetadata(t *testing.T) {
	for _, tc := range []struct {
		name       string
		region     string
		wantRegion any
	}{
		{name: "configured", region: "fsn1", wantRegion: "fsn1"},
		{name: "unconfigured", wantRegion: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := danaService()
			svc.Metadata = resourcemeta.Config{Region: tc.region}

			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			createRec := httptest.NewRecorder()
			createReq := httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader(
				`{"name":"web-meta","repo":"https://github.com/acme/web","serviceDetails":{"runtime":"node","envSpecificDetails":{"buildCommand":"npm install","startCommand":"npm start"}}}`))
			mux.ServeHTTP(createRec, createReq.WithContext(ctxAs("dana")))
			if createRec.Code != http.StatusCreated {
				t.Fatalf("create => %d: %s", createRec.Code, createRec.Body.String())
			}

			res := graphql.Do(graphql.Params{
				Schema:        mustSchema(t, svc),
				Context:       ctxAs("dana"),
				RequestString: `{ services { runtime region createdAt updatedAt } }`,
			})
			if len(res.Errors) > 0 {
				t.Fatalf("graphql errors: %v", res.Errors)
			}
			data := res.Data.(map[string]any)
			rows := data["services"].([]any)
			if len(rows) != 1 {
				t.Fatalf("services = %d rows, want 1", len(rows))
			}
			row := rows[0].(map[string]any)
			if row["runtime"] != "node" || row["region"] != tc.wantRegion {
				t.Fatalf("metadata = runtime:%v region:%v, want node/%v", row["runtime"], row["region"], tc.wantRegion)
			}
			updated, _ := row["updatedAt"].(string)
			if updated == "" {
				t.Fatalf("updatedAt = %q, want authoritative mutation timestamp", updated)
			}
		})
	}
}
