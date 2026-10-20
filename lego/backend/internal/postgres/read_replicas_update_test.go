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
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestReadReplicaSetChangesAfterCreate is w5/m138: Render's
// postgresPATCHInput.readReplicas replaces the replica set after create, and
// REST, GraphQL and MCP apply it with create's refusals, by the same code, and
// without writing on a refusal or a dry run.
func TestReadReplicaSetChangesAfterCreate(t *testing.T) {
	eligible := appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: 10, ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "r1"}}}
	six := replicaNames(admissionReplicas(6))
	for _, tc := range []struct {
		name string
		seed appv1alpha1.DatabaseSpec
		set  []string
		code string
	}{
		{name: "add one", seed: eligible, set: []string{"r1", "r2"}},
		{name: "replace", seed: eligible, set: []string{"r3"}},
		{name: "clear", seed: eligible, set: []string{}},
		{name: "invalid name", seed: eligible, set: []string{"r1", "Reader_2"}, code: "POSTGRES_READ_REPLICA_NAME_INVALID"},
		{name: "duplicate name", seed: eligible, set: []string{"r1", "r1"}, code: "POSTGRES_READ_REPLICA_NAME_DUPLICATE"},
		{name: "over the limit", seed: eligible, set: six, code: "POSTGRES_READ_REPLICA_LIMIT_EXCEEDED"},
		{name: "plan without replicas", seed: appv1alpha1.DatabaseSpec{Plan: "free"}, set: []string{"r1"}, code: "POSTGRES_READ_REPLICA_PLAN_UNSUPPORTED"},
		{name: "storage below the floor", seed: appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: 5}, set: []string{"r1"}, code: "POSTGRES_READ_REPLICA_STORAGE_UNSUPPORTED"},
		{
			// A name created before the name rule is kept through an edit that
			// adds a well-formed sibling.
			name: "legacy name kept",
			seed: appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: 10, ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "Legacy_R"}}},
			set:  []string{"Legacy_R", "r2"},
		},
	} {
		for _, surface := range []string{"REST", "GraphQL", "MCP"} {
			for _, dryRun := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/dry=%v", tc.name, surface, dryRun), func(t *testing.T) {
					svc, cl := newService()
					seedDatabaseSpec(t, cl, "dpg-reps", tc.seed, false)
					before := getDatabase(t, cl, "dpg-reps")
					sink := &webhookAuditSink{}
					svc.Audit = sink

					names, code := replicaSetCall(t, svc, surface, "dpg-reps", tc.set, dryRun)
					if code != tc.code {
						t.Fatalf("error code = %q, want %q", code, tc.code)
					}
					after := getDatabase(t, cl, "dpg-reps")
					if tc.code != "" || dryRun {
						if !reflect.DeepEqual(before.Spec, after.Spec) || len(sink.events) != 0 {
							t.Fatalf("refused or preview update wrote: spec %+v, events %+v", after.Spec, sink.events)
						}
						if tc.code != "" {
							return
						}
					} else {
						if got := replicaNames(after.Spec.ReadReplicas); !slices.Equal(got, tc.set) {
							t.Fatalf("spec.readReplicas = %v, want %v", got, tc.set)
						}
						if len(sink.events) != 1 || sink.events[0].Verb != core.AuditVerbPostgresReadReplicasChanged {
							t.Fatalf("effects = %+v, want one %s", sink.events, core.AuditVerbPostgresReadReplicasChanged)
						}
					}
					// The answer is the set asked for, preview included.
					if !slices.Equal(names, tc.set) {
						t.Fatalf("response readReplicas = %v, want %v", names, tc.set)
					}
				})
			}
		}
	}
}

// An update that does not name readReplicas leaves the set alone, and a
// combined patch is judged against its own plan and storage, not the stored
// ones: a free database upgraded in the same call may gain a replica.
func TestReadReplicaPatchPresenceAndCombinedPatch(t *testing.T) {
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "dpg-keep", appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: 10, ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "r1"}}}, false)
	if w := serveREST(svc, http.MethodPatch, "/v1/postgres/dpg-keep", `{"enableDiskAutoscaling":true}`); w.Code != http.StatusOK {
		t.Fatalf("patch => %d: %s", w.Code, w.Body)
	}
	if got := replicaNames(getDatabase(t, cl, "dpg-keep").Spec.ReadReplicas); !slices.Equal(got, []string{"r1"}) {
		t.Fatalf("an update without readReplicas changed the set to %v", got)
	}

	seedDatabaseSpec(t, cl, "dpg-up", appv1alpha1.DatabaseSpec{Plan: "free"}, false)
	w := serveREST(svc, http.MethodPatch, "/v1/postgres/dpg-up", `{"plan":"basic-1gb","diskSizeGB":10,"readReplicas":[{"name":"r1"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("combined patch => %d: %s", w.Code, w.Body)
	}
	got := getDatabase(t, cl, "dpg-up").Spec
	if got.Plan != "basic-1gb" || got.StorageGB != 10 || !slices.Equal(replicaNames(got.ReadReplicas), []string{"r1"}) {
		t.Fatalf("combined patch spec = %+v", got)
	}
}

// The update response keeps the connection info of a replica the operator
// already reports and lists a new one by id and name until it does.
func TestReadReplicaPatchResponseKeepsObservedHosts(t *testing.T) {
	svc, _ := newService(&appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-hosts", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Name: "hosts", Plan: "basic-1gb", StorageGB: 10, ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "r1"}}},
		Status: appv1alpha1.DatabaseStatus{
			Phase:               appv1alpha1.DBPhaseReady,
			ReadReplicaStatuses: []appv1alpha1.DatabaseReadReplicaStatus{{Name: "r1", InternalHost: "dpg-hosts-ro-r1.default.svc"}},
		},
	})
	w := serveREST(svc, http.MethodPatch, "/v1/postgres/dpg-hosts", `{"readReplicas":[{"name":"r1"},{"name":"r2"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("patch => %d: %s", w.Code, w.Body)
	}
	var view PostgresView
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.ReadReplicas) != 2 ||
		view.ReadReplicas[0].ConnectionInfo == nil || view.ReadReplicas[0].ConnectionInfo.InternalHost != "dpg-hosts-ro-r1.default.svc" ||
		view.ReadReplicas[1].ID != ReadReplicaID("dpg-hosts", "r2") || view.ReadReplicas[1].ConnectionInfo != nil {
		t.Fatalf("response readReplicas = %+v", view.ReadReplicas)
	}
}

func replicaNames(replicas []appv1alpha1.DatabaseReadReplica) []string {
	out := make([]string, 0, len(replicas))
	for _, r := range replicas {
		out = append(out, r.Name)
	}
	return out
}

// replicaSetCall replaces dbID's replica set on one surface and answers the
// response's replica names, or the refusal's code.
func replicaSetCall(t *testing.T, svc *Service, surface, dbID string, set []string, dryRun bool) ([]string, string) {
	t.Helper()
	items := make([]map[string]any, 0, len(set))
	for _, name := range set {
		items = append(items, map[string]any{"name": name})
	}
	var view struct {
		ReadReplicas []struct{ Name string } `json:"readReplicas"`
	}
	names := func() []string {
		out := make([]string, 0, len(view.ReadReplicas))
		for _, r := range view.ReadReplicas {
			out = append(out, r.Name)
		}
		return out
	}
	switch surface {
	case "REST":
		body, _ := json.Marshal(map[string]any{"readReplicas": items, "dryRun": dryRun})
		res := serveREST(svc, http.MethodPatch, "/v1/postgres/"+dbID, string(body))
		if res.Code == http.StatusBadRequest {
			var out struct{ Code string }
			_ = json.Unmarshal(res.Body.Bytes(), &out)
			return nil, out.Code
		}
		if res.Code != http.StatusOK {
			t.Fatalf("patch status = %d: %s", res.Code, res.Body)
		}
		if err := json.Unmarshal(res.Body.Bytes(), &view); err != nil {
			t.Fatal(err)
		}
		return names(), ""
	case "GraphQL":
		schema, err := pgGQLSchema(svc)
		if err != nil {
			t.Fatal(err)
		}
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
			VariableValues: map[string]any{"id": dbID, "readReplicas": items, "dryRun": dryRun},
			RequestString: `mutation($id:String!, $readReplicas:[DatabaseReadReplicaInput!]!, $dryRun:Boolean) {
				setDatabaseReadReplicas(id:$id, readReplicas:$readReplicas, dryRun:$dryRun) { readReplicas { name } }
			}`})
		if len(res.Errors) != 0 {
			if len(res.Errors) != 1 || res.Errors[0].Extensions["code"] == nil {
				t.Fatalf("GraphQL errors = %+v", res.Errors)
			}
			return nil, res.Errors[0].Extensions["code"].(string)
		}
		body, _ := json.Marshal(res.Data.(map[string]any)["setDatabaseReadReplicas"])
		if err := json.Unmarshal(body, &view); err != nil {
			t.Fatal(err)
		}
		return names(), ""
	case "MCP":
		session, cleanup := pgMCPSession(t, svc)
		defer cleanup()
		res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "update_postgres",
			Arguments: map[string]any{"postgresId": dbID, "readReplicas": items, "dryRun": dryRun}})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			return nil, admissionMCPErrorCode(t, res)
		}
		body, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(body, &view); err != nil {
			t.Fatal(err)
		}
		return names(), ""
	default:
		t.Fatalf("unknown surface %q", surface)
		return nil, ""
	}
}

// GraphQL reports a replica's hosts as REST and MCP do. The connectionInfo
// resolver handed gqlutil.Field a pointer, so both hosts read null on GraphQL —
// the dashboard HA panel's source — while the other surfaces returned them.
func TestReadReplicaHostsReadTheSameOnGraphQL(t *testing.T) {
	svc, _ := newService(&appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-gqlhosts", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Name: "gqlhosts", Plan: "basic-1gb", StorageGB: 10, ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "r1"}}},
		Status: appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseReady,
			ReadReplicaStatuses: []appv1alpha1.DatabaseReadReplicaStatus{{
				Name: "r1", InternalHost: "dpg-gqlhosts-ro.default.svc", ExternalHost: "dpg-gqlhosts-ro-r1.db.example",
			}},
		},
	})
	schema, err := pgGQLSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ database(id: "dpg-gqlhosts") { readReplicas { name connectionInfo { internalHost externalHost } } } }`})
	if len(res.Errors) != 0 {
		t.Fatalf("GraphQL errors = %+v", res.Errors)
	}
	body, _ := json.Marshal(res.Data)
	want := `{"database":{"readReplicas":[{"connectionInfo":{"externalHost":"dpg-gqlhosts-ro-r1.db.example","internalHost":"dpg-gqlhosts-ro.default.svc"},"name":"r1"}]}}`
	if string(body) != want {
		t.Fatalf("GraphQL replica = %s, want %s", body, want)
	}
}
