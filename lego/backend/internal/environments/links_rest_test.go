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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// linkHarness is restHarness with all four member seams wired, so every link
// route can be driven end to end. The shared restHarness leaves
// Databases/KeyValues/EnvGroups nil (those verbs then answer
// ErrEnvironmentsUnavailable), which is fine for the routes it exercises but
// would make three of the four link routes untestable here.
func linkHarness(t *testing.T) (*Service, *http.ServeMux, string) {
	t.Helper()
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	svc, _ := newServiceWithClient(st)

	dbs := newDatabaseIndex()
	dbs.add(postgres.PostgresView{ID: "db-a", OwnerID: "tea-a"})
	svc.Databases = dbs
	kvs := newKeyValueIndex()
	kvs.add(keyvalue.KeyValueView{ID: "kv-a", OwnerID: "tea-a"})
	svc.KeyValues = kvs
	svc.EnvGroups = &fakeEnvGroupIndex{groups: []envgroups.EnvGroupView{{ID: "evg-a", OwnerID: "tea-a"}}}

	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := doREST(t, mux, http.MethodPost, "/v1/environments", `{"name":"staging","projectId":"prj-1"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create environment = %d (body: %s)", rec.Code, rec.Body.String())
	}
	var created renderEnvironment
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created environment: %v", err)
	}
	return svc, mux, created.ID
}

// TestLinkRoutesWireTheRightFieldAndSetter is the wiring guard for the four
// link routes, which now share one handler (core.HandleSetLinks) instead of
// four hand-copied closures. Sharing removes the copy-paste drift risk but
// introduces a new one — a route naming the wrong body type or the wrong
// setter still compiles, because all four have the same shape. This table
// walks every route end to end and asserts the value lands in ITS OWN list
// and no other, which is exactly what a swapped pair would violate.
func TestLinkRoutesWireTheRightFieldAndSetter(t *testing.T) {
	for _, tc := range []struct {
		name  string
		path  string
		body  string
		field func(renderEnvironment) []string
	}{
		{
			name:  "service-links",
			path:  "service-links",
			body:  `{"serviceIds":["srv-a"]}`,
			field: func(e renderEnvironment) []string { return e.ServiceIDs },
		},
		{
			name:  "database-links",
			path:  "database-links",
			body:  `{"databaseIds":["db-a"]}`,
			field: func(e renderEnvironment) []string { return e.DatabaseIDs },
		},
		{
			name:  "keyvalue-links",
			path:  "keyvalue-links",
			body:  `{"keyValueIds":["kv-a"]}`,
			field: func(e renderEnvironment) []string { return e.KeyValueIDs },
		},
		{
			name:  "env-group-links",
			path:  "env-group-links",
			body:  `{"envGroupIds":["evg-a"]}`,
			field: func(e renderEnvironment) []string { return e.EnvGroupIDs },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, mux, id := linkHarness(t)

			rec := doREST(t, mux, http.MethodPut, "/v1/environments/"+id+"/"+tc.path, tc.body)
			if rec.Code != http.StatusOK {
				t.Fatalf("PUT %s = %d (body: %s)", tc.path, rec.Code, rec.Body.String())
			}
			var got renderEnvironment
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if list := tc.field(got); len(list) != 1 {
				t.Fatalf("%s did not populate its own list: %+v", tc.path, got)
			}
			// Every OTHER list must stay empty — a route wired to the wrong
			// setter would fill a neighbour's list instead of its own.
			others := map[string][]string{
				"serviceIds":  got.ServiceIDs,
				"databaseIds": got.DatabaseIDs,
				"keyValueIds": got.KeyValueIDs,
				"envGroupIds": got.EnvGroupIDs,
			}
			total := 0
			for _, list := range others {
				total += len(list)
			}
			if total != 1 {
				t.Fatalf("PUT %s populated %d list entries across all kinds, want exactly 1: %+v", tc.path, total, got)
			}
		})
	}
}

// TestLinkRoutesTreatAnAbsentListAsAReplaceWithNothing pins the nil-to-empty
// rule the shared handler now owns for all four routes. It matters because the
// setters take []string: were nil to reach them, "clear this list" and "the
// caller sent no list" would be indistinguishable downstream.
func TestLinkRoutesTreatAnAbsentListAsAReplaceWithNothing(t *testing.T) {
	for _, body := range []string{`{}`, `{"serviceIds":null}`, `{"serviceIds":[]}`} {
		t.Run(body, func(t *testing.T) {
			_, mux, id := linkHarness(t)

			// Seed a service so the clear is observable.
			if rec := doREST(t, mux, http.MethodPut, "/v1/environments/"+id+"/service-links",
				`{"serviceIds":["srv-a"]}`); rec.Code != http.StatusOK {
				t.Fatalf("seed = %d (body: %s)", rec.Code, rec.Body.String())
			}

			rec := doREST(t, mux, http.MethodPut, "/v1/environments/"+id+"/service-links", body)
			if rec.Code != http.StatusOK {
				t.Fatalf("PUT %s = %d (body: %s)", body, rec.Code, rec.Body.String())
			}
			var got renderEnvironment
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(got.ServiceIDs) != 0 {
				t.Fatalf("PUT %s left serviceIds = %v, want the list replaced with nothing", body, got.ServiceIDs)
			}
			// The wire shape stays [] and never null — the official CLI decodes
			// the field as a list.
			if raw, err := json.Marshal(got); err == nil {
				var fields map[string]json.RawMessage
				_ = json.Unmarshal(raw, &fields)
				if string(fields["serviceIds"]) != "[]" {
					t.Fatalf("serviceIds serialized as %s, want []", fields["serviceIds"])
				}
			}
		})
	}
}

// TestLinkRoutesRejectAMalformedBody pins the 400 the shared handler maps a
// decode failure to — the same status the four hand-written closures produced.
func TestLinkRoutesRejectAMalformedBody(t *testing.T) {
	_, mux, id := linkHarness(t)

	rec := doREST(t, mux, http.MethodPut, "/v1/environments/"+id+"/service-links", `{"serviceIds":`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed body = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}
