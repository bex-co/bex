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

// w6/m458: Render pools every read replica of a pooled database, so a
// replica's connection info carries pooled strings beside its direct ones, on
// REST and GraphQL connection info and, host-only, on the REST, GraphQL and
// MCP database view. An unpooled database's replicas carry none.
func TestPooledDatabasesReplicasHavePooledConnections(t *testing.T) {
	svc, cl := newService()
	seedDatabaseSpec(t, cl, "dpg-pooled", appv1alpha1.DatabaseSpec{
		Plan: "basic-1gb", Public: true, Pooler: true,
		ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "analytics"}},
	}, false)
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "dpg-pooled"}, &db); err != nil {
		t.Fatal(err)
	}
	db.Status.ExternalCertificate = appv1alpha1.DBExternalCertificatePubliclyTrusted
	db.Status.ExternalHost = "dpg-pooled.db.bex.co"
	db.Status.PoolerHost = "dpg-pooled-pooler.default.svc"
	db.Status.PoolerExternalHost = "dpg-pooled-pool.db.bex.co"
	db.Status.ReadReplicaStatuses = []appv1alpha1.DatabaseReadReplicaStatus{{
		Name: "analytics", InternalHost: "dpg-pooled-ro.default.svc", ExternalHost: "dpg-pooled-ro-analytics.db.bex.co",
		PoolerHost: "dpg-pooled-pooler-ro.default.svc", PoolerExternalHost: "dpg-pooled-pool-ro.db.bex.co",
	}}
	if err := cl.Update(context.Background(), &db); err != nil {
		t.Fatal(err)
	}
	user := db.Spec.EffectiveDatabaseUser(db.Name)
	dbn := db.Spec.EffectiveDatabaseName(db.Name)
	want := ReplicaConnectionStrings{
		Name:                         "analytics",
		InternalConnectionString:     "postgresql://" + user + ":s3cret@dpg-pooled-ro.default.svc:5432/" + dbn,
		ExternalConnectionString:     "postgresql://" + user + ":s3cret@dpg-pooled-ro-analytics.db.bex.co:5432/" + dbn,
		InternalConnectionPoolString: "postgresql://" + user + ":s3cret@dpg-pooled-pooler-ro.default.svc:6432/" + dbn,
		ExternalConnectionPoolString: "postgresql://" + user + ":s3cret@dpg-pooled-pool-ro.db.bex.co:5432/" + dbn,
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
	if len(ci.ReadReplicaConnectionStrings) != 1 || ci.ReadReplicaConnectionStrings[0] != want {
		t.Fatalf("REST replica strings = %+v, want %+v", ci.ReadReplicaConnectionStrings, want)
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
		database(id:"dpg-pooled") { readReplicas { connectionInfo { internalPoolHost externalPoolHost } } }
	}`})
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	data := res.Data.(map[string]any)
	gqlStrings := data["databaseConnectionInfo"].(map[string]any)["readReplicaConnectionStrings"].([]any)[0].(map[string]any)
	for field, value := range map[string]string{
		"name": want.Name, "internalConnectionString": want.InternalConnectionString, "externalConnectionString": want.ExternalConnectionString,
		"internalConnectionPoolString": want.InternalConnectionPoolString, "externalConnectionPoolString": want.ExternalConnectionPoolString,
	} {
		if gqlStrings[field] != value {
			t.Errorf("GraphQL replica %s = %v, want %q", field, gqlStrings[field], value)
		}
	}
	wantHosts := map[string]any{"internalPoolHost": "dpg-pooled-pooler-ro.default.svc", "externalPoolHost": "dpg-pooled-pool-ro.db.bex.co"}
	gqlHosts := data["database"].(map[string]any)["readReplicas"].([]any)[0].(map[string]any)["connectionInfo"].(map[string]any)
	assertPoolHosts(t, "GraphQL database", gqlHosts, wantHosts)

	// REST and MCP views carry the replica pooler's hosts, without a password.
	var view map[string]any
	decodeREST(t, serveREST(svc, "GET", "/v1/postgres/dpg-pooled", ""), &view)
	assertPoolHosts(t, "REST get", replicaConnectionInfo(t, view), wantHosts)
	call, cleanup := pgMCPClient(t, svc)
	defer cleanup()
	assertPoolHosts(t, "MCP get_postgres", replicaConnectionInfo(t, call("get_postgres", map[string]any{"postgresId": "dpg-pooled"})), wantHosts)

	// Pooling off: the operator withdraws the replicas' pooler hosts, and no
	// surface offers a pooled replica string.
	db.Spec.Pooler = false
	db.Status.PoolerHost, db.Status.PoolerExternalHost = "", ""
	db.Status.ReadReplicaStatuses[0].PoolerHost, db.Status.ReadReplicaStatuses[0].PoolerExternalHost = "", ""
	if err := cl.Update(context.Background(), &db); err != nil {
		t.Fatal(err)
	}
	w = serveREST(svc, "GET", "/v1/postgres/dpg-pooled/connection-info", "")
	ci = PostgresConnectionInfo{}
	if err := json.Unmarshal(w.Body.Bytes(), &ci); err != nil {
		t.Fatal(err)
	}
	if got := ci.ReadReplicaConnectionStrings[0]; got.InternalConnectionPoolString != "" || got.ExternalConnectionPoolString != "" || got.InternalConnectionString == "" {
		t.Errorf("unpooled replica strings = %+v, want direct strings only", got)
	}
	decodeREST(t, serveREST(svc, "GET", "/v1/postgres/dpg-pooled", ""), &view)
	assertPoolHosts(t, "unpooled REST get", replicaConnectionInfo(t, view), map[string]any{"internalPoolHost": nil, "externalPoolHost": nil})
}

func replicaConnectionInfo(t *testing.T, view map[string]any) map[string]any {
	t.Helper()
	replicas, _ := view["readReplicas"].([]any)
	if len(replicas) != 1 {
		t.Fatalf("readReplicas = %v", view["readReplicas"])
	}
	info, _ := replicas[0].(map[string]any)["connectionInfo"].(map[string]any)
	if info == nil {
		t.Fatalf("replica has no connectionInfo: %v", replicas[0])
	}
	return info
}

func assertPoolHosts(t *testing.T, surface string, got, want map[string]any) {
	t.Helper()
	for field, value := range want {
		if got[field] != value {
			t.Errorf("%s replica %s = %v, want %v", surface, field, got[field], value)
		}
	}
}
