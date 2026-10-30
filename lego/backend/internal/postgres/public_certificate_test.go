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
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// seedPublicDatabase seeds a public database with a pooler and a replica whose
// endpoints present the given certificate.
func seedPublicDatabase(t *testing.T, cl client.Client, name string, cert appv1alpha1.DatabaseExternalCertificate) {
	t.Helper()
	seedDatabase(t, cl, name)
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &db); err != nil {
		t.Fatal(err)
	}
	db.Status.ExternalCertificate = cert
	db.Status.PoolerExternalHost = name + "-pool.db.bex.co"
	db.Status.ReadReplicaStatuses = []appv1alpha1.DatabaseReadReplicaStatus{{Name: "reader", ExternalHost: name + "-ro-reader.db.bex.co"}}
	if err := cl.Update(context.Background(), &db); err != nil {
		t.Fatal(err)
	}
}

func connectionInfoGraphQL(t *testing.T, svc *Service, id string) map[string]any {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ databaseConnectionInfo(id:"` + id + `") { externalConnectionString externalConnectionPoolString psqlCommand serverCaCertificate } }`,
		Context:       ctxAs("user-a"),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	return res.Data.(map[string]any)["databaseConnectionInfo"].(map[string]any)
}

// w2/m245: once the endpoints present a publicly trusted certificate, the
// copied external URL is Render's exact shape — no sslmode, so libpq never
// demands ~/.postgresql/root.crt — and no CA is handed out (system roots
// verify the chain), on REST and GraphQL alike.
func TestPubliclyTrustedDatabaseServesRenderShapedURLsWithoutCA(t *testing.T) {
	svc, cl := newService()
	seedPublicDatabase(t, cl, "lets-db", appv1alpha1.DBExternalCertificatePubliclyTrusted)
	// The CNPG CA is irrelevant now: its absence must not 503 the read.
	var ca corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "lets-db-ca"}, &ca); err != nil {
		t.Fatal(err)
	}
	if err := cl.Delete(context.Background(), &ca); err != nil {
		t.Fatal(err)
	}

	rec := serveREST(svc, http.MethodGet, "/v1/postgres/lets-db/connection-info", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("connection-info => %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "serverCaCertificate") {
		t.Fatalf("publicly trusted database still hands out a CA: %s", rec.Body.String())
	}
	var ci PostgresConnectionInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &ci); err != nil {
		t.Fatal(err)
	}
	user, db := "lets_db_user", "lets_db"
	want := map[string]string{
		"external":         "postgresql://" + user + ":s3cret@lets-db.db.bex.co:5432/" + db,
		"external pool":    "postgresql://" + user + ":s3cret@lets-db-pool.db.bex.co:5432/" + db,
		"external replica": "postgresql://" + user + ":s3cret@lets-db-ro-reader.db.bex.co:5432/" + db,
		"psql":             "PGPASSWORD=s3cret psql -h lets-db.db.bex.co -U " + user + " " + db,
	}
	got := map[string]string{
		"external":         ci.ExternalConnectionString,
		"external pool":    ci.ExternalConnectionPoolString,
		"external replica": ci.ReadReplicaConnectionStrings[0].ExternalConnectionString,
		"psql":             ci.PSQLCommand,
	}
	for k := range want {
		if got[k] != want[k] {
			t.Errorf("%s = %q, want %q", k, got[k], want[k])
		}
	}

	gql := connectionInfoGraphQL(t, svc, "lets-db")
	if gql["externalConnectionString"] != ci.ExternalConnectionString || gql["externalConnectionPoolString"] != ci.ExternalConnectionPoolString ||
		gql["psqlCommand"] != ci.PSQLCommand {
		t.Fatalf("GraphQL diverges from REST: %v", gql)
	}
	if v, _ := gql["serverCaCertificate"].(string); v != "" {
		t.Fatalf("GraphQL serverCaCertificate = %q, want empty", v)
	}
}

// Still issuing (or withdrawing): the endpoints present CNPG's certificate, so
// the private-CA contract (verify-full + serverCaCertificate) stays in force.
func TestPendingPublicCertificateKeepsPrivateCAContract(t *testing.T) {
	svc, cl := newService()
	seedPublicDatabase(t, cl, "issuing-db", appv1alpha1.DBExternalCertificatePending)
	rec := serveREST(svc, http.MethodGet, "/v1/postgres/issuing-db/connection-info", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("connection-info => %d: %s", rec.Code, rec.Body.String())
	}
	var ci PostgresConnectionInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &ci); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{ci.ExternalConnectionString, ci.ExternalConnectionPoolString, ci.ReadReplicaConnectionStrings[0].ExternalConnectionString} {
		if !strings.HasSuffix(s, "?sslmode=verify-full") {
			t.Errorf("external string %q dropped verify-full against a private CA", s)
		}
	}
	if !strings.Contains(ci.PSQLCommand, "sslmode=verify-full") || !strings.Contains(ci.ServerCACertificate, "BEGIN CERTIFICATE") {
		t.Fatalf("private-CA contract incomplete: psql=%q ca=%q", ci.PSQLCommand, ci.ServerCACertificate)
	}
}
