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
	"testing"

	"github.com/graphql-go/graphql"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m458, w6/m467: Render pools every read replica of a pooled database, and
// each replica has its own instance and its own pool. So each replica's
// connection info carries its own direct and pooled strings, on REST and
// GraphQL connection info and, host-only, on the REST, GraphQL and MCP
// database view. An unpooled database's replicas carry no pooled string.
func TestPooledDatabasesReplicasHavePooledConnections(t *testing.T) {
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "dpg-pooled", appv1alpha1.DatabaseSpec{
		Plan: "basic-1gb", Public: true, Pooler: true,
		ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "analytics"}, {Name: "reports"}},
	}, false)
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "dpg-pooled"}, &db); err != nil {
		t.Fatal(err)
	}
	db.Status.ExternalCertificate = appv1alpha1.DBExternalCertificatePubliclyTrusted
	db.Status.ExternalHost = "dpg-pooled.db.bex.co"
	db.Status.PoolerHost = "dpg-pooled-pooler.default.svc"
	db.Status.PoolerExternalHost = "dpg-pooled-pool.db.bex.co"
	// The operator's hosts (updateExternalAddressStatus): each replica's own
	// Service and PgBouncer.
	names := []string{"analytics", "reports"}
	for _, name := range names {
		db.Status.ReadReplicaStatuses = append(db.Status.ReadReplicaStatuses, appv1alpha1.DatabaseReadReplicaStatus{
			Name:         name,
			InternalHost: "dpg-pooled-ro-" + name + ".default.svc", ExternalHost: "dpg-pooled-ro-" + name + ".db.bex.co",
			PoolerHost: "dpg-pooled-rp-" + name + ".default.svc", PoolerExternalHost: "dpg-pooled-rp-" + name + ".db.bex.co",
		})
	}
	if err := cl.Update(context.Background(), &db); err != nil {
		t.Fatal(err)
	}
	user := db.Spec.EffectiveDatabaseUser(db.Name)
	dbn := db.Spec.EffectiveDatabaseName(db.Name)
	want := make([]ReplicaConnectionStrings, 0, len(names))
	wantHosts := make([]map[string]any, 0, len(names))
	for _, name := range names {
		want = append(want, ReplicaConnectionStrings{
			Name:                         name,
			InternalConnectionString:     "postgresql://" + user + ":s3cret@dpg-pooled-ro-" + name + ".default.svc:5432/" + dbn,
			ExternalConnectionString:     "postgresql://" + user + ":s3cret@dpg-pooled-ro-" + name + ".db.bex.co:5432/" + dbn,
			InternalConnectionPoolString: "postgresql://" + user + ":s3cret@dpg-pooled-rp-" + name + ".default.svc:6432/" + dbn,
			ExternalConnectionPoolString: "postgresql://" + user + ":s3cret@dpg-pooled-rp-" + name + ".db.bex.co:5432/" + dbn,
		})
		wantHosts = append(wantHosts, map[string]any{
			"internalHost": "dpg-pooled-ro-" + name + ".default.svc", "externalHost": "dpg-pooled-ro-" + name + ".db.bex.co",
			"internalPoolHost": "dpg-pooled-rp-" + name + ".default.svc", "externalPoolHost": "dpg-pooled-rp-" + name + ".db.bex.co",
		})
	}

	// REST connection info.
	w := serveREST(svc, "GET", "/v1/postgres/dpg-pooled/connection-info", "")
	if w.Code != 200 {
		t.Fatalf("connection-info => %d: %s", w.Code, w.Body.String())
	}
	var ci PostgresConnectionInfo
	if err := json.Unmarshal(w.Body.Bytes(), &ci); err != nil {
		t.Fatal(err)
	}
	if len(ci.ReadReplicaConnectionStrings) != len(want) {
		t.Fatalf("REST replica strings = %+v, want %+v", ci.ReadReplicaConnectionStrings, want)
	}
	for i := range want {
		if ci.ReadReplicaConnectionStrings[i] != want[i] {
			t.Errorf("REST replica strings[%d] = %+v, want %+v", i, ci.ReadReplicaConnectionStrings[i], want[i])
		}
	}

	// GraphQL connection info and view.
	schema, err := pgGQLSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctxAs("user-a"), RequestString: `{
		databaseConnectionInfo(id:"dpg-pooled") { readReplicaConnectionStrings {
			name internalConnectionString externalConnectionString internalConnectionPoolString externalConnectionPoolString
		} }
		database(id:"dpg-pooled") { readReplicas { connectionInfo { internalHost externalHost internalPoolHost externalPoolHost } } }
	}`})
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	data := res.Data.(map[string]any)
	gqlStrings := data["databaseConnectionInfo"].(map[string]any)["readReplicaConnectionStrings"].([]any)
	for i, w := range want {
		got := gqlStrings[i].(map[string]any)
		for field, value := range map[string]string{
			"name": w.Name, "internalConnectionString": w.InternalConnectionString, "externalConnectionString": w.ExternalConnectionString,
			"internalConnectionPoolString": w.InternalConnectionPoolString, "externalConnectionPoolString": w.ExternalConnectionPoolString,
		} {
			if got[field] != value {
				t.Errorf("GraphQL replica %d %s = %v, want %q", i, field, got[field], value)
			}
		}
	}
	assertReplicaHosts(t, "GraphQL database", data["database"].(map[string]any), wantHosts)

	// REST and MCP views carry each replica's own hosts, without a password.
	var view map[string]any
	decodeREST(t, serveREST(svc, "GET", "/v1/postgres/dpg-pooled", ""), &view)
	assertReplicaHosts(t, "REST get", view, wantHosts)
	call, cleanup := pgMCPClient(t, svc)
	defer cleanup()
	assertReplicaHosts(t, "MCP get_postgres", call("get_postgres", map[string]any{"postgresId": "dpg-pooled"}), wantHosts)

	// Pooling off: the operator withdraws the replicas' pooler hosts, and no
	// surface offers a pooled replica string.
	db.Spec.Pooler = false
	db.Status.PoolerHost, db.Status.PoolerExternalHost = "", ""
	for i := range db.Status.ReadReplicaStatuses {
		db.Status.ReadReplicaStatuses[i].PoolerHost, db.Status.ReadReplicaStatuses[i].PoolerExternalHost = "", ""
		wantHosts[i]["internalPoolHost"], wantHosts[i]["externalPoolHost"] = nil, nil
	}
	if err := cl.Update(context.Background(), &db); err != nil {
		t.Fatal(err)
	}
	w = serveREST(svc, "GET", "/v1/postgres/dpg-pooled/connection-info", "")
	ci = PostgresConnectionInfo{}
	if err := json.Unmarshal(w.Body.Bytes(), &ci); err != nil {
		t.Fatal(err)
	}
	for _, got := range ci.ReadReplicaConnectionStrings {
		if got.InternalConnectionPoolString != "" || got.ExternalConnectionPoolString != "" || got.InternalConnectionString == "" {
			t.Errorf("unpooled replica strings = %+v, want direct strings only", got)
		}
	}
	decodeREST(t, serveREST(svc, "GET", "/v1/postgres/dpg-pooled", ""), &view)
	assertReplicaHosts(t, "unpooled REST get", view, wantHosts)
}

// assertReplicaHosts checks each replica's host-only connectionInfo in a
// database view, in order.
func assertReplicaHosts(t *testing.T, surface string, view map[string]any, want []map[string]any) {
	t.Helper()
	replicas, _ := view["readReplicas"].([]any)
	if len(replicas) != len(want) {
		t.Fatalf("%s readReplicas = %v, want %d", surface, view["readReplicas"], len(want))
	}
	for i, replica := range replicas {
		info, _ := replica.(map[string]any)["connectionInfo"].(map[string]any)
		if info == nil {
			t.Fatalf("%s replica %d has no connectionInfo: %v", surface, i, replica)
		}
		for field, value := range want[i] {
			if info[field] != value {
				t.Errorf("%s replica %d %s = %v, want %v", surface, i, field, info[field], value)
			}
		}
	}
}
