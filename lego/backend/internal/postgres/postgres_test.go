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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

	// databaseInstanceTypes surfaces the shared Postgres catalog (w5/m8) — the
	// create dialog's plan picker source, never a hardcoded ladder.
	tt := run(`{ databaseInstanceTypes { id name cpu memory storageGB } }`)["databaseInstanceTypes"].([]any)
	if len(tt) == 0 {
		t.Fatal("databaseInstanceTypes want >=1")
	}
	first := tt[0].(map[string]any)
	if first["id"] != "free" || first["name"] != "Free" {
		t.Fatalf("first tier = %+v, want free/Free", first)
	}
}

func TestMCPPostgres(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "mcp-db")

	// Register the postgres tools into an MCP server, connect an in-memory client.
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	call := func(name string, args map[string]any) map[string]any {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("%s: err=%v isErr=%v", name, err, res.IsError)
		}
		out := map[string]any{}
		if res.StructuredContent != nil {
			b, _ := json.Marshal(res.StructuredContent)
			_ = json.Unmarshal(b, &out)
		}
		return out
	}

	// Render's tool names + arg (postgresId), delegating to the same Core verbs.
	if list, ok := call("list_postgres_instances", nil)["postgres"].([]any); !ok || len(list) != 1 {
		t.Fatalf("list_postgres_instances want 1, got %v", call("list_postgres_instances", nil))
	}
	if got := call("get_postgres", map[string]any{"postgresId": "mcp-db"}); got["id"] != "mcp-db" {
		t.Fatalf("get_postgres id = %v", got["id"])
	}
	// create_postgres delegates to CreatePostgres — verify the CR lands.
	if got := call("create_postgres", map[string]any{"name": "mcp-new", "plan": "basic-1gb"}); got["id"] != "mcp-new" {
		t.Fatalf("create_postgres id = %v", got["id"])
	}
	var made appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "mcp-new"}, &made); err != nil || made.Spec.Plan != "basic-1gb" {
		t.Fatalf("create_postgres did not create the CR: %v %+v", err, made.Spec)
	}
}

func TestInstanceTypesCatalog(t *testing.T) {
	svc, _ := newService()
	tt, err := svc.InstanceTypes(context.Background())
	if err != nil {
		t.Fatalf("InstanceTypes: %v", err)
	}
	// Sourced from lego/types/tiers.Postgres (basic-256mb, basic-1gb, …) — the
	// id is the spec.plan spelling createDatabase accepts, so it must round-trip
	// verbatim, and each tier carries its resource specs for the picker card.
	byID := map[string]DatabaseInstanceType{}
	for _, it := range tt {
		byID[it.ID] = it
	}
	b, ok := byID["basic-1gb"]
	if !ok {
		t.Fatalf("basic-1gb missing from catalog: %+v", tt)
	}
	if b.Name != "Basic 1GB" || b.CPU == "" || b.Memory == "" || b.StorageGB <= 0 {
		t.Fatalf("basic-1gb projection wrong: %+v", b)
	}
}

func TestPGTierDisplayName(t *testing.T) {
	for _, c := range []struct{ id, want string }{
		{"free", "Free"},
		{"basic-256mb", "Basic 256MB"},
		{"basic-1gb", "Basic 1GB"},
	} {
		if got := pgTierDisplayName(c.id); got != c.want {
			t.Errorf("pgTierDisplayName(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}

func newTenantService(ws core.WorkspaceResolver, dbs ...*appv1alpha1.Database) (*Service, client.Client) {
	objs := make([]client.Object, len(dbs))
	for i, d := range dbs {
		objs[i] = d
	}
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Service{Base: &core.Base{Client: cl, Namespace: "default", Workspace: ws}}, cl
}

func tenantDB(name, tenantID string) *appv1alpha1.Database {
	return &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    map[string]string{core.LabelTenant: tenantID},
		},
	}
}

// TestPostgresCapEnforcement verifies that the (N+1)th Postgres create is
// refused with ErrBadRequest while a second workspace can still create.
func TestPostgresCapEnforcement(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a", "user-b": "tea-b"}
	ctx := func(subject string) context.Context {
		return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
	}

	// tea-a has one instance (at cap=1); tea-b has none.
	svc, _ := newTenantService(ws, tenantDB("pg-1", "tea-a"))
	svc.MaxPostgres = 1

	// tea-a is at cap.
	if _, err := svc.CreatePostgres(ctx("user-a"), CreatePostgresRequest{Name: "pg-new", Plan: "free"}); err == nil {
		t.Fatal("create at cap: want ErrBadRequest, got nil")
	} else if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("create at cap: got %v, want ErrBadRequest", err)
	}

	// tea-b has zero instances — can still create despite tea-a being at cap.
	if _, err := svc.CreatePostgres(ctx("user-b"), CreatePostgresRequest{Name: "pg-b1", Plan: "free"}); err != nil {
		t.Errorf("second workspace create: %v, want success", err)
	}

	// MaxPostgres=0: unlimited.
	svc2, _ := newTenantService(ws, tenantDB("pg-1", "tea-a"))
	svc2.MaxPostgres = 0
	if _, err := svc2.CreatePostgres(ctx("user-a"), CreatePostgresRequest{Name: "pg-2", Plan: "free"}); err != nil {
		t.Errorf("unlimited cap: %v, want success", err)
	}

	// Store off (no Workspace resolver): cap is skipped.
	svc3, _ := newService(tenantDB("pg-1", "tea-a"))
	svc3.MaxPostgres = 1
	if _, err := svc3.CreatePostgres(context.Background(), CreatePostgresRequest{Name: "pg-2", Plan: "free"}); err != nil {
		t.Errorf("store-off cap: %v, want success (no workspace to count against)", err)
	}
}
