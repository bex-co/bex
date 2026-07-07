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

	"github.com/graphql-go/graphql"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func newService(objs ...client.Object) (*Service, client.Client) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Service{Base: &core.Base{Client: cl, Namespace: "default"}}, cl
}

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

// seedDatabase adds a Ready public Database + its CNPG-style "<name>-app" Secret.
func seedDatabase(t *testing.T, cl client.Client, name string) {
	t.Helper()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Plan: "free", Public: true},
		Status: appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseReady, Host: name + "-rw.default.svc", Port: 5432,
			SecretName: name + "-app", ExternalHost: name + ".db.bex.co",
		},
	}
	dbn := pgIdent(name)
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-app", Namespace: "default"},
		Data: map[string][]byte{
			"username": []byte(dbn + "_user"),
			"password": []byte("s3cret"),
			"dbname":   []byte(dbn),
			"uri":      []byte("postgresql://" + dbn + "_user:s3cret@" + name + "-rw.default:5432/" + dbn),
		},
	}
	if err := cl.Create(context.Background(), db); err != nil {
		t.Fatalf("seed db: %v", err)
	}
	if err := cl.Create(context.Background(), sec); err != nil {
		t.Fatalf("seed secret: %v", err)
	}
}

func TestRESTPostgresCRUD(t *testing.T) {
	svc, cl := newService()

	// create — Render: 201; hyphenated name normalizes.
	w := serveREST(svc, "POST", "/v1/postgres", `{"name":"pg-test","plan":"free","public":true}`)
	if w.Code != 201 {
		t.Fatalf("create => 201, got %d: %s", w.Code, w.Body.String())
	}
	var pg PostgresView
	_ = json.Unmarshal(w.Body.Bytes(), &pg)
	if pg.ID != "pg-test" || pg.DatabaseName != "pg_test" || pg.DatabaseUser != "pg_test_user" || pg.Plan != "free" || !pg.Public {
		t.Fatalf("normalized names/spec wrong: %+v", pg)
	}
	var got appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "pg-test"}, &got); err != nil {
		t.Fatalf("db CR not created: %v", err)
	}

	var list []PostgresView
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/postgres", "").Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list => 1, got %d", len(list))
	}
	if serveREST(svc, "GET", "/v1/postgres/pg-test", "").Code != 200 {
		t.Fatal("get failed")
	}
	if serveREST(svc, "GET", "/v1/postgres/nope", "").Code != 404 {
		t.Error("unknown => 404")
	}
	if serveREST(svc, "DELETE", "/v1/postgres/pg-test", "").Code != 204 {
		t.Error("delete => 204")
	}
	if serveREST(svc, "GET", "/v1/postgres/pg-test", "").Code != 404 {
		t.Error("deleted db should be gone")
	}
}

func TestRESTPostgresConnectionInfo(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "conn-db")

	var ci PostgresConnectionInfo
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/postgres/conn-db/connection-info", "").Body.Bytes(), &ci)
	if ci.Password != "s3cret" {
		t.Errorf("password = %q", ci.Password)
	}
	if ci.InternalConnectionString != "postgresql://conn_db_user:s3cret@conn-db-rw.default:5432/conn_db" {
		t.Errorf("internal = %q", ci.InternalConnectionString)
	}
	want := "postgresql://conn_db_user:s3cret@conn-db.db.bex.co:5432/conn_db?sslmode=require&sslnegotiation=direct"
	if ci.ExternalConnectionString != want {
		t.Errorf("external = %q", ci.ExternalConnectionString)
	}
	if ci.PSQLCommand == "" {
		t.Error("psqlCommand empty")
	}
}

func TestGraphQLPostgres(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "gql-db")
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	run := func(q string) map[string]any {
		res := graphql.Do(graphql.Params{Schema: schema, RequestString: q, Context: context.Background()})
		if len(res.Errors) > 0 {
			t.Fatalf("gql %q: %v", q, res.Errors)
		}
		return res.Data.(map[string]any)
	}

	if len(run(`{ databases { id databaseName status } }`)["databases"].([]any)) != 1 {
		t.Fatal("databases want 1")
	}
	ci := run(`{ databaseConnectionInfo(id:"gql-db") { externalConnectionString password } }`)["databaseConnectionInfo"].(map[string]any)
	if ci["password"] != "s3cret" || ci["externalConnectionString"] == "" {
		t.Fatalf("connection info: %+v", ci)
	}
	run(`mutation { createDatabase(name:"gql-new", plan:"basic-1gb") { id databaseName } }`)
	var made appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "gql-new"}, &made); err != nil || made.Spec.Plan != "basic-1gb" {
		t.Fatalf("createDatabase did not create the CR with plan: %v %+v", err, made.Spec)
	}
}
