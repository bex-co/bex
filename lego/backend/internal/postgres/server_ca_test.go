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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The w4/m95 delivery gap: the external connection string pins
// sslmode=verify-full against a private CNPG server CA no client could obtain
// through the product. Connection info must now carry exactly that CA's
// certificate PEM — and never the private key stored beside it.
func TestConnectionInfoServesServerCACertificateOnly(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "ca-db")
	var caSec corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "ca-db-ca"}, &caSec); err != nil {
		t.Fatal(err)
	}
	wantPEM := string(caSec.Data["ca.crt"])

	var ci PostgresConnectionInfo
	rec := serveREST(svc, "GET", "/v1/postgres/ca-db/connection-info", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("connection-info => %d: %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ci)
	if ci.ServerCACertificate != wantPEM {
		t.Fatalf("serverCaCertificate = %q, want the CNPG CA certificate PEM", ci.ServerCACertificate)
	}
	if strings.Contains(ci.ServerCACertificate, "PRIVATE KEY") {
		t.Fatal("server CA delivery must never include key material")
	}
	if strings.Contains(rec.Body.String(), "PRIVATE KEY") {
		t.Fatal("response body carries key material")
	}

	// GraphQL serves the identical bundle.
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ databaseConnectionInfo(id:"ca-db") { serverCaCertificate } }`,
		Context:       ctxAs("user-a"),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	info := res.Data.(map[string]any)["databaseConnectionInfo"].(map[string]any)
	if info["serverCaCertificate"] != wantPEM {
		t.Fatalf("GraphQL serverCaCertificate diverges from REST")
	}
}

// A public database whose CA Secret is not readable must fail actionably —
// handing out a verify-full URL that cannot connect is the filed bug, and
// downgrading TLS to make it "work" is forbidden.
func TestConnectionInfoUnavailableWhenServerCAMissing(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "noca-db")
	var caSec corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "noca-db-ca"}, &caSec); err != nil {
		t.Fatal(err)
	}
	if err := cl.Delete(context.Background(), &caSec); err != nil {
		t.Fatal(err)
	}
	rec := serveREST(svc, "GET", "/v1/postgres/noca-db/connection-info", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing CA => %d, want 503: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "server CA") {
		t.Fatalf("error is not actionable: %s", rec.Body.String())
	}
}

func TestConnectionInfoRejectsMalformedServerCA(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "badca-db")
	var caSec corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "badca-db-ca"}, &caSec); err != nil {
		t.Fatal(err)
	}
	// A key where the certificate should be must be refused, not served.
	caSec.Data["ca.crt"] = caSec.Data["ca.key"]
	if err := cl.Update(context.Background(), &caSec); err != nil {
		t.Fatal(err)
	}
	rec := serveREST(svc, "GET", "/v1/postgres/badca-db/connection-info", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("malformed CA => %d, want 503: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "PRIVATE KEY") {
		t.Fatal("refusal must not echo the material")
	}
}

// An internal-only database has no verify-full external edge: no CA is read,
// none is required to exist, and the panel keeps its coherent shape.
func TestConnectionInfoPrivateDatabaseOmitsServerCA(t *testing.T) {
	svc, cl := newService()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "internal-db", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "free"},
		Status: appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseReady, Host: "internal-db-rw.default.svc",
			SecretName: "internal-db-app",
		},
	}
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "internal-db-app", Namespace: "default"},
		Data: map[string][]byte{
			"username": []byte("u"), "password": []byte("p"), "dbname": []byte("d"),
			"uri": []byte("postgresql://u:p@internal-db-rw.default:5432/d"),
		},
	}
	if err := cl.Create(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if err := cl.Create(context.Background(), sec); err != nil {
		t.Fatal(err)
	}
	rec := serveREST(svc, "GET", "/v1/postgres/internal-db/connection-info", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("internal-only connection-info => %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "serverCaCertificate") {
		t.Fatal("internal-only database must omit the CA field")
	}
}

func TestCertificateOnlyPEM(t *testing.T) {
	for name, raw := range map[string]string{
		"empty":     "",
		"garbage":   "not pem at all",
		"key-block": "-----BEGIN EC PRIVATE KEY-----\nAAAA\n-----END EC PRIVATE KEY-----\n",
	} {
		if _, err := certificateOnlyPEM([]byte(raw)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}

	// A valid multi-certificate bundle round-trips whole and key-free.
	_, cl := newService()
	certPEM := seedDatabaseCA(t, cl, "pemcheck")
	bundle, err := certificateOnlyPEM([]byte(certPEM + certPEM))
	if err != nil {
		t.Fatalf("multi-block bundle: %v", err)
	}
	if bundle != certPEM+certPEM {
		t.Fatalf("multi-block bundle not preserved")
	}
	// One key smuggled between certificate blocks fails the whole bundle.
	var caSec corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "pemcheck-ca"}, &caSec); err != nil {
		t.Fatal(err)
	}
	mixed := certPEM + string(caSec.Data["ca.key"]) + certPEM
	if _, err := certificateOnlyPEM([]byte(mixed)); err == nil {
		t.Fatal("certificate+key bundle must be refused whole")
	}
}
