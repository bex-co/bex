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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
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
	dbn := appv1alpha1.DefaultPostgresDatabaseName(name)
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
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	svc.Environments = &fixedCreateEnvironment{assignment: core.EnvironmentAssignment{ID: "env-staging", ProjectID: "prj-platform", WorkspaceID: "tea-a"}}

	// create — Render: 201; hyphenated name normalizes.
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/postgres", strings.NewReader(`{"name":"pg-test","plan":"free","public":true,"environmentId":"env-staging"}`))
	mux.ServeHTTP(w, req.WithContext(ctxAs("user-a")))
	if w.Code != 201 {
		t.Fatalf("create => 201, got %d: %s", w.Code, w.Body.String())
	}
	var pg PostgresView
	_ = json.Unmarshal(w.Body.Bytes(), &pg)
	if !strings.HasPrefix(pg.ID, "dpg-") || pg.Name != "pg-test" || pg.DatabaseName != appv1alpha1.DefaultPostgresDatabaseName(pg.ID) || pg.DatabaseUser != appv1alpha1.DefaultPostgresDatabaseName(pg.ID)+"_user" || pg.Plan != "free" || !pg.Public || pg.ProjectID != "prj-platform" || pg.EnvironmentID != "env-staging" {
		t.Fatalf("normalized names/spec wrong: %+v", pg)
	}
	var got appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: pg.ID}, &got); err != nil {
		t.Fatalf("db CR not created: %v", err)
	}
	if got.Spec.Name != "pg-test" {
		t.Fatalf("spec.name = %q, want pg-test", got.Spec.Name)
	}
	if got.Labels[core.LabelProject] != "prj-platform" || got.Labels[core.LabelEnvironment] != "env-staging" {
		t.Fatalf("db environment labels = %v", got.Labels)
	}

	// list — Render's cursor envelope ({postgres, cursor} per item, RC3):
	// decode into the wrapper shape and check the nested record's real
	// fields, not just the array length (a bare-array/wrong-shape regression
	// would still produce a length-1 slice of zero-value structs and pass a
	// length-only check).
	var list []postgresWithCursor
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/postgres", "").Body.Bytes(), &list)
	if len(list) != 1 || list[0].Postgres.ID != pg.ID || list[0].Cursor != pg.ID {
		t.Fatalf("list => one {postgres,cursor} entry for pg-test, got %+v", list)
	}
	if serveREST(svc, "GET", "/v1/postgres/"+pg.ID, "").Code != 200 {
		t.Fatal("get failed")
	}
	if serveREST(svc, "GET", "/v1/postgres/nope", "").Code != 404 {
		t.Error("unknown => 404")
	}
	if serveREST(svc, "DELETE", "/v1/postgres/"+pg.ID, "").Code != 204 {
		t.Error("delete => 204")
	}
	if serveREST(svc, "GET", "/v1/postgres/"+pg.ID, "").Code != 404 {
		t.Error("deleted db should be gone")
	}
}

func TestPostgresListPaginationAcrossRESTAndGraphQL(t *testing.T) {
	svc, cl := newService()
	for i := 22; i >= 0; i-- { // deliberately seed opposite cursor order
		seedDatabase(t, cl, fmt.Sprintf("pg-%02d", i))
	}

	ctx := context.Background()
	all, err := svc.ListPostgres(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	wantBody, _ := json.Marshal(svc.toPostgresList(ctx, all))
	wantBody = append(wantBody, '\n')
	omitted := serveREST(svc, http.MethodGet, "/v1/postgres", "")
	if !bytes.Equal(omitted.Body.Bytes(), wantBody) {
		t.Fatalf("omitted params changed full-list body\ngot:  %s\nwant: %s", omitted.Body.Bytes(), wantBody)
	}
	if alias := serveREST(svc, http.MethodGet, "/v1/databases", ""); !bytes.Equal(alias.Body.Bytes(), omitted.Body.Bytes()) {
		t.Fatalf("/v1/databases alias drifted from /v1/postgres\npostgres: %s\nalias:    %s", omitted.Body.Bytes(), alias.Body.Bytes())
	}

	var walked []string
	cursor := ""
	var firstREST []string
	for {
		path := "/v1/postgres?limit=7"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		var page []postgresWithCursor
		if err := json.Unmarshal(serveREST(svc, http.MethodGet, path, "").Body.Bytes(), &page); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		if len(page) == 0 {
			break
		}
		ids := make([]string, 0, len(page))
		for _, item := range page {
			if item.Cursor != item.Postgres.ID {
				t.Fatalf("cursor %q != postgres id %q", item.Cursor, item.Postgres.ID)
			}
			ids = append(ids, item.Postgres.ID)
		}
		if firstREST == nil {
			firstREST = slices.Clone(ids)
		}
		walked = append(walked, ids...)
		cursor = page[len(page)-1].Cursor
	}
	wantIDs := make([]string, 23)
	for i := range wantIDs {
		wantIDs[i] = fmt.Sprintf("pg-%02d", i)
	}
	if !slices.Equal(walked, wantIDs) {
		t.Fatalf("REST page walk = %v, want every id once in %v", walked, wantIDs)
	}
	var pastEnd []postgresWithCursor
	_ = json.Unmarshal(serveREST(svc, http.MethodGet, "/v1/postgres?limit=7&cursor=does-not-exist", "").Body.Bytes(), &pastEnd)
	if len(pastEnd) != 0 {
		t.Fatalf("unknown REST cursor = %v, want empty page", pastEnd)
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	gqlIDs := func(query string) []string {
		t.Helper()
		res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: query})
		if len(res.Errors) > 0 {
			t.Fatalf("GraphQL %s: %v", query, res.Errors)
		}
		raw := res.Data.(map[string]any)["databases"].([]any)
		ids := make([]string, 0, len(raw))
		for _, item := range raw {
			ids = append(ids, item.(map[string]any)["id"].(string))
		}
		return ids
	}
	if got := gqlIDs(`{ databases { id } }`); len(got) != len(wantIDs) {
		t.Fatalf("omitted GraphQL args returned %d items, want full %d", len(got), len(wantIDs))
	}
	firstGQL := gqlIDs(`{ databases(limit: 7) { id } }`)
	if !slices.Equal(firstGQL, firstREST) {
		t.Fatalf("first REST/GQL pages drift: REST=%v GQL=%v", firstREST, firstGQL)
	}
	if got := gqlIDs(`{ databases(cursor: "pg-06", limit: 7) { id } }`); !slices.Equal(got, wantIDs[7:14]) {
		t.Fatalf("second GraphQL page = %v, want %v", got, wantIDs[7:14])
	}
	if got := gqlIDs(`{ databases(cursor: "does-not-exist", limit: 7) { id } }`); len(got) != 0 {
		t.Fatalf("unknown GraphQL cursor = %v, want empty page", got)
	}
}

// TestRESTCreatePostgresIPAllowListWireShape covers the other bug found live
// against the real render-oss/cli: `postgres create --ip-allow-list ...`
// sends Render's {cidrBlock,description} object array
// (cli/pkg/client/types_gen.go's CidrBlockAndDescription), but a prior
// version of CreatePostgresRequest.IPAllowList was a bare []string — decoding
// an object array into []string failed the whole request body decode
// ("bad request body"), so the flag was unusable end to end.
func TestRESTCreatePostgresIPAllowListWireShape(t *testing.T) {
	svc, _ := newService()
	w := serveREST(svc, "POST", "/v1/postgres",
		`{"name":"pg-ipal","plan":"free","ipAllowList":[{"cidrBlock":"10.0.0.0/8","description":"internal"}]}`)
	if w.Code != 201 {
		t.Fatalf("create with object-array ipAllowList => 201, got %d: %s", w.Code, w.Body.String())
	}
	var pg PostgresView
	_ = json.Unmarshal(w.Body.Bytes(), &pg)
	// Both fields round-trip: the description persists on the CR (w4/m24), no
	// longer accepted-but-dropped.
	if len(pg.IPAllowList) != 1 || pg.IPAllowList[0].CIDRBlock != "10.0.0.0/8" || pg.IPAllowList[0].Description != "internal" {
		t.Errorf("ipAllowList in create response = %+v, want one {10.0.0.0/8, internal} entry", pg.IPAllowList)
	}

	// a bad CIDR inside a well-formed entry is still rejected.
	w = serveREST(svc, "POST", "/v1/postgres",
		`{"name":"pg-ipal-bad","plan":"free","ipAllowList":[{"cidrBlock":"nonsense"}]}`)
	if w.Code != 400 {
		t.Errorf("create with a bad CIDR => 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreatePostgresPhysicalIdentifierAdapterParity(t *testing.T) {
	const databaseName = "orders_data"
	const databaseUser = "orders_owner"

	restService, restClient := newService()
	rest := serveREST(restService, http.MethodPost, "/v1/postgres",
		`{"name":"orders-rest","databaseName":"orders_data","databaseUser":"orders_owner"}`)
	if rest.Code != http.StatusCreated {
		t.Fatalf("REST create = %d: %s", rest.Code, rest.Body.String())
	}
	var restView PostgresView
	if err := json.Unmarshal(rest.Body.Bytes(), &restView); err != nil {
		t.Fatal(err)
	}
	var restDB appv1alpha1.Database
	if err := restClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: restView.ID}, &restDB); err != nil {
		t.Fatal(err)
	}

	gqlService, gqlClient := newService()
	gqlSchema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: gqlService.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: gqlService.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	gqlResult := graphql.Do(graphql.Params{Schema: gqlSchema, Context: context.Background(), RequestString: `
		mutation { createDatabase(name:"orders-gql", databaseName:"orders_data", databaseUser:"orders_owner") {
			id databaseName databaseUser
		} }`})
	if len(gqlResult.Errors) != 0 {
		t.Fatalf("GraphQL create: %v", gqlResult.Errors)
	}
	gqlView := gqlResult.Data.(map[string]any)["createDatabase"].(map[string]any)
	var gqlDB appv1alpha1.Database
	if err := gqlClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: gqlView["id"].(string)}, &gqlDB); err != nil {
		t.Fatal(err)
	}

	mcpService, mcpKubeClient := newService()
	mcpCall, cleanup := pgMCPClient(t, mcpService)
	defer cleanup()
	mcpView := mcpCall("create_postgres", map[string]any{
		"name": "orders-mcp", "databaseName": databaseName, "databaseUser": databaseUser,
	})
	var mcpDB appv1alpha1.Database
	if err := mcpKubeClient.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: mcpView["id"].(string)}, &mcpDB); err != nil {
		t.Fatal(err)
	}

	if restView.DatabaseName != databaseName || restView.DatabaseUser != databaseUser ||
		gqlView["databaseName"] != databaseName || gqlView["databaseUser"] != databaseUser ||
		mcpView["databaseName"] != databaseName || mcpView["databaseUser"] != databaseUser {
		t.Fatalf("adapter readback drift: REST=%+v GraphQL=%v MCP=%v", restView, gqlView, mcpView)
	}
	for surface, db := range map[string]appv1alpha1.Database{"REST": restDB, "GraphQL": gqlDB, "MCP": mcpDB} {
		if db.Spec.DatabaseName != databaseName || db.Spec.DatabaseUser != databaseUser {
			t.Errorf("%s CR physical identifiers = %q/%q", surface, db.Spec.DatabaseName, db.Spec.DatabaseUser)
		}
	}
}

func TestCreatePostgresPhysicalIdentifierDefaultsAreIndependent(t *testing.T) {
	svc, _ := newService()
	onlyDB, err := svc.CreatePostgres(context.Background(), CreatePostgresRequest{Name: "only-db", DatabaseName: "orders"})
	if err != nil {
		t.Fatal(err)
	}
	if onlyDB.DatabaseName != "orders" || onlyDB.DatabaseUser != appv1alpha1.DefaultPostgresDatabaseName(onlyDB.ID)+"_user" {
		t.Fatalf("database-only defaults = %q/%q", onlyDB.DatabaseName, onlyDB.DatabaseUser)
	}

	onlyUser, err := svc.CreatePostgres(context.Background(), CreatePostgresRequest{Name: "only-user", DatabaseUser: "reporter"})
	if err != nil {
		t.Fatal(err)
	}
	if onlyUser.DatabaseName != appv1alpha1.DefaultPostgresDatabaseName(onlyUser.ID) || onlyUser.DatabaseUser != "reporter" {
		t.Fatalf("user-only defaults = %q/%q", onlyUser.DatabaseName, onlyUser.DatabaseUser)
	}

	bad := serveREST(svc, http.MethodPost, "/v1/postgres", `{"name":"bad-ident","databaseName":"Bad-Name"}`)
	if bad.Code != http.StatusBadRequest || !strings.Contains(bad.Body.String(), "POSTGRES_IDENTIFIER_INVALID") {
		t.Fatalf("invalid physical identifier = %d: %s", bad.Code, bad.Body.String())
	}
}

func TestPostgresDatadogFieldsFailClosedWithoutLeakingCredential(t *testing.T) {
	svc, cl := newService()
	const secret = "super-secret-datadog-key"

	created := serveREST(svc, http.MethodPost, "/v1/postgres",
		`{"name":"datadog-create","datadogAPIKey":"`+secret+`","datadogSite":"US1"}`)
	if created.Code != http.StatusBadRequest || !strings.Contains(created.Body.String(), "POSTGRES_DATADOG_UNSUPPORTED") {
		t.Fatalf("Datadog create = %d: %s", created.Code, created.Body.String())
	}
	if strings.Contains(created.Body.String(), secret) {
		t.Fatal("Datadog create error leaked the API key")
	}
	var list appv1alpha1.DatabaseList
	if err := cl.List(context.Background(), &list); err != nil || len(list.Items) != 0 {
		t.Fatalf("Datadog create had side effects: err=%v databases=%d", err, len(list.Items))
	}

	seedDatabase(t, cl, "datadog-update")
	before := getDatabase(t, cl, "datadog-update")
	updated := serveREST(svc, http.MethodPatch, "/v1/postgres/datadog-update", `{"datadogAPIKey":"`+secret+`"}`)
	if updated.Code != http.StatusBadRequest || !strings.Contains(updated.Body.String(), "POSTGRES_DATADOG_UNSUPPORTED") {
		t.Fatalf("Datadog update = %d: %s", updated.Code, updated.Body.String())
	}
	if strings.Contains(updated.Body.String(), secret) {
		t.Fatal("Datadog update error leaked the API key")
	}
	after := getDatabase(t, cl, "datadog-update")
	if !reflect.DeepEqual(before.Spec, after.Spec) || before.ResourceVersion != after.ResourceVersion {
		t.Fatalf("Datadog rejection mutated Database: before=%+v after=%+v", before.Spec, after.Spec)
	}

	emptySite := serveREST(svc, http.MethodPatch, "/v1/postgres/datadog-update", `{"datadogSite":""}`)
	if emptySite.Code != http.StatusBadRequest || !strings.Contains(emptySite.Body.String(), "POSTGRES_DATADOG_UNSUPPORTED") {
		t.Fatalf("empty Datadog site update = %d: %s", emptySite.Code, emptySite.Body.String())
	}
}

func TestCreatePostgresIPAllowListAdapterParity(t *testing.T) {
	want := core.IPAllowListEntry{CIDRBlock: "203.0.113.0/24", Description: "office"}

	restService, _ := newService()
	rest := serveREST(restService, "POST", "/v1/postgres",
		`{"name":"acl-rest","ipAllowList":[{"cidrBlock":"203.0.113.0/24","description":"office"}]}`)
	if rest.Code != http.StatusCreated {
		t.Fatalf("REST create = %d: %s", rest.Code, rest.Body.String())
	}
	var restView PostgresView
	if err := json.Unmarshal(rest.Body.Bytes(), &restView); err != nil {
		t.Fatal(err)
	}

	gqlService, _ := newService()
	gqlSchema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: gqlService.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: gqlService.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	gqlResult := graphql.Do(graphql.Params{Schema: gqlSchema, Context: context.Background(), RequestString: `
		mutation { createDatabase(name:"acl-gql", ipAllowListEntries:[{cidrBlock:"203.0.113.0/24", description:"office"}]) {
			ipAllowListEntries { cidrBlock description }
		} }`})
	if len(gqlResult.Errors) != 0 {
		t.Fatalf("GraphQL create: %v", gqlResult.Errors)
	}
	gqlEntries := gqlResult.Data.(map[string]any)["createDatabase"].(map[string]any)["ipAllowListEntries"].([]any)
	gqlEntry := gqlEntries[0].(map[string]any)

	mcpService, _ := newService()
	mcpCall, cleanup := pgMCPClient(t, mcpService)
	defer cleanup()
	mcpView := mcpCall("create_postgres", map[string]any{
		"name":               "acl-mcp",
		"ipAllowListEntries": []any{map[string]any{"cidrBlock": "203.0.113.0/24", "description": "office"}},
	})
	mcpEntries := mcpView["ipAllowList"].([]any)
	mcpEntry := mcpEntries[0].(map[string]any)

	if len(restView.IPAllowList) != 1 || restView.IPAllowList[0] != want ||
		gqlEntry["cidrBlock"] != want.CIDRBlock || gqlEntry["description"] != want.Description ||
		mcpEntry["cidrBlock"] != want.CIDRBlock || mcpEntry["description"] != want.Description {
		t.Fatalf("create allowlist drift: REST=%+v GraphQL=%v MCP=%v", restView.IPAllowList, gqlEntry, mcpEntry)
	}

	badREST := serveREST(restService, "POST", "/v1/postgres", `{"name":"bad-rest","ipAllowList":[{"cidrBlock":"not-a-cidr"}]}`)
	badGQL := graphql.Do(graphql.Params{Schema: gqlSchema, Context: context.Background(), RequestString: `mutation { createDatabase(name:"bad-gql", ipAllowList:["not-a-cidr"]) { id } }`})
	badMCPService, _ := newService()
	badServer := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	badMCPService.RegisterMCP(badServer)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := badServer.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	badClient, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer badClient.Close()
	badMCP, err := badClient.CallTool(ctx, &mcp.CallToolParams{Name: "create_postgres", Arguments: map[string]any{"name": "bad-mcp", "ipAllowList": []string{"not-a-cidr"}}})
	if err != nil {
		t.Fatal(err)
	}
	mcpErrorText := ""
	if len(badMCP.Content) > 0 {
		if text, ok := badMCP.Content[0].(*mcp.TextContent); ok {
			mcpErrorText = text.Text
		}
	}
	if badREST.Code != http.StatusBadRequest || len(badGQL.Errors) != 1 || !badMCP.IsError ||
		!strings.Contains(badREST.Body.String(), "CIDR") ||
		!strings.Contains(badGQL.Errors[0].Message, "CIDR") || !strings.Contains(mcpErrorText, "CIDR") {
		t.Fatalf("invalid CIDR mismatch: REST=%d %s GraphQL=%v MCP=%+v", badREST.Code, badREST.Body.String(), badGQL.Errors, badMCP)
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
	want := "postgresql://conn_db_user:s3cret@conn-db.db.bex.co:5432/conn_db?sslmode=require"
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
		res := graphql.Do(graphql.Params{Schema: schema, RequestString: q, Context: ctxAs("user-a")})
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
	renamed := run(`mutation { renameDatabase(id:"gql-db", name:"gql-renamed") { id name databaseName } }`)["renameDatabase"].(map[string]any)
	if renamed["id"] != "gql-db" || renamed["name"] != "gql-renamed" || renamed["databaseName"] != "gql_db" {
		t.Fatalf("renameDatabase changed identity/physical name: %+v", renamed)
	}
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	svc.Environments = &fixedCreateEnvironment{assignment: core.EnvironmentAssignment{ID: "env-staging", ProjectID: "prj-platform", WorkspaceID: "tea-a"}}
	created := run(`mutation { createDatabase(name:"gql-new", plan:"basic-1gb", environmentId:"env-staging") { id name databaseName projectId environmentId } }`)["createDatabase"].(map[string]any)
	if !strings.HasPrefix(created["id"].(string), "dpg-") || created["name"] != "gql-new" || created["projectId"] != "prj-platform" || created["environmentId"] != "env-staging" {
		t.Fatalf("createDatabase association = %+v", created)
	}
	var made appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: created["id"].(string)}, &made); err != nil || made.Spec.Plan != "basic-1gb" || made.Spec.Name != "gql-new" {
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
	ctx := ctxAs("user-a")
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name != "list_postgres_instances" {
			continue
		}
		b, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Properties map[string]any `json:"properties"`
		}
		_ = json.Unmarshal(b, &schema)
		if len(schema.Properties) != 0 {
			t.Fatalf("list_postgres_instances args = %v, Render accepts none", schema.Properties)
		}
	}

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
	if got := call("rename_postgres", map[string]any{"postgresId": "mcp-db", "name": "mcp-renamed"}); got["id"] != "mcp-db" || got["name"] != "mcp-renamed" || got["databaseName"] != "mcp_db" {
		t.Fatalf("rename_postgres changed identity/physical name: %+v", got)
	}
	// create_postgres delegates to CreatePostgres — verify the CR lands.
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	svc.Environments = &fixedCreateEnvironment{assignment: core.EnvironmentAssignment{ID: "env-staging", ProjectID: "prj-platform", WorkspaceID: "tea-a"}}
	created := call("create_postgres", map[string]any{"name": "mcp-new", "plan": "basic-1gb", "environmentId": "env-staging"})
	createdID, _ := created["id"].(string)
	if !strings.HasPrefix(createdID, "dpg-") || created["name"] != "mcp-new" || created["projectId"] != "prj-platform" || created["environmentId"] != "env-staging" {
		t.Fatalf("create_postgres = %+v", created)
	}
	var made appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: createdID}, &made); err != nil || made.Spec.Plan != "basic-1gb" || made.Spec.Name != "mcp-new" {
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

func TestRESTSetPostgresPlan(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "plan-db")

	// valid plan upgrade.
	w := serveREST(svc, "PATCH", "/v1/postgres/plan-db", `{"plan":"basic-1gb"}`)
	if w.Code != 200 {
		t.Fatalf("PATCH plan => 200, got %d: %s", w.Code, w.Body.String())
	}
	var pg PostgresView
	_ = json.Unmarshal(w.Body.Bytes(), &pg)
	if pg.Plan != "basic-1gb" {
		t.Errorf("plan in view = %q, want basic-1gb", pg.Plan)
	}
	var got appv1alpha1.Database
	_ = cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "plan-db"}, &got)
	if got.Spec.Plan != "basic-1gb" {
		t.Errorf("spec.plan on CR = %q, want basic-1gb", got.Spec.Plan)
	}

	// unknown plan => 400.
	if w := serveREST(svc, "PATCH", "/v1/postgres/plan-db", `{"plan":"nope"}`); w.Code != 400 {
		t.Errorf("unknown plan => 400, got %d: %s", w.Code, w.Body.String())
	}
	// unknown instance => 404.
	if w := serveREST(svc, "PATCH", "/v1/postgres/missing", `{"plan":"free"}`); w.Code != 404 {
		t.Errorf("missing db => 404, got %d", w.Code)
	}
	// /v1/databases alias works too.
	w = serveREST(svc, "PATCH", "/v1/databases/plan-db", `{"plan":"basic-256mb"}`)
	if w.Code != 200 {
		t.Errorf("/v1/databases alias => 200, got %d", w.Code)
	}
}

// TestRESTUpdatePostgresPartial covers the bug found live against the real
// render-oss/cli: a prior version of the PATCH handler decoded plan as a
// plain (non-pointer) string and called SetPlan unconditionally, so ANY
// update that didn't include plan — a disk resize, an HA toggle, an
// ip-allow-list replace — 400'd with "plan must be one of ...". Render's
// PostgresPATCHInput (cli/pkg/client/types_gen.go) makes every field
// optional; omitting one must leave it untouched, not fail the whole request.
func TestRESTUpdatePostgresPartial(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "upd-db")
	ctx := context.Background()
	var got appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "upd-db"}, &got); err != nil {
		t.Fatal(err)
	}
	got.Labels = map[string]string{
		core.LabelProject:     "prj-platform",
		core.LabelEnvironment: "env-production",
	}
	if err := cl.Update(ctx, &got); err != nil {
		t.Fatal(err)
	}

	// disk resize alone, no plan — must succeed (previously 400'd).
	w := serveREST(svc, "PATCH", "/v1/postgres/upd-db", `{"diskSizeGB":20}`)
	if w.Code != 200 {
		t.Fatalf("disk-only PATCH => 200, got %d: %s", w.Code, w.Body.String())
	}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "upd-db"}, &got); err != nil {
		t.Fatal(err)
	}
	if got.Spec.StorageGB != 20 {
		t.Errorf("spec.storageGB = %d, want 20", got.Spec.StorageGB)
	}
	if got.Spec.Plan != "free" {
		t.Errorf("plan should be untouched by a disk-only update, got %q", got.Spec.Plan)
	}

	// HA toggle alone, no plan — must succeed.
	w = serveREST(svc, "PATCH", "/v1/postgres/upd-db", `{"enableHighAvailability":true}`)
	if w.Code != 200 {
		t.Fatalf("HA-only PATCH => 200, got %d: %s", w.Code, w.Body.String())
	}
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "upd-db"}, &got)
	if !got.Spec.HighAvailability {
		t.Error("spec.highAvailability should be true after an HA-only update")
	}

	// Render accepts parameterOverrides inline on PATCH. It reaches the same
	// replace-style spec.parameters seam as PUT /parameter-overrides.
	w = serveREST(svc, "PATCH", "/v1/postgres/upd-db", `{"parameterOverrides":{"work_mem":"16MB","shared_preload_libraries":"badlib"}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("inline parameterOverrides PATCH => 200, got %d: %s", w.Code, w.Body.String())
	}
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "upd-db"}, &got)
	if !reflect.DeepEqual(got.Spec.Parameters, map[string]string{"work_mem": "16MB"}) {
		t.Fatalf("inline parameterOverrides = %v, want guarded replace map", got.Spec.Parameters)
	}
	// Omission leaves overrides untouched.
	w = serveREST(svc, "PATCH", "/v1/postgres/upd-db", `{"diskSizeGB":21}`)
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "upd-db"}, &got)
	if w.Code != http.StatusOK || !reflect.DeepEqual(got.Spec.Parameters, map[string]string{"work_mem": "16MB"}) {
		t.Fatalf("PATCH without parameterOverrides changed them: status=%d params=%v", w.Code, got.Spec.Parameters)
	}

	// Storage is grow-only across every REST/CLI call: a smaller explicit disk
	// request is a named 400 and leaves the accepted intent untouched.
	w = serveREST(svc, "PATCH", "/v1/postgres/upd-db", `{"diskSizeGB":20}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "storage is grow-only") {
		t.Fatalf("disk shrink => named 400, got %d: %s", w.Code, w.Body.String())
	}
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "upd-db"}, &got)
	if got.Spec.StorageGB != 21 {
		t.Fatalf("rejected shrink changed spec.storageGB to %d", got.Spec.StorageGB)
	}

	// ip-allow-list alone, Render's {cidrBlock,description} wire shape, no
	// plan — must succeed and round-trip through the same shape on read.
	w = serveREST(svc, "PATCH", "/v1/postgres/upd-db",
		`{"ipAllowList":[{"cidrBlock":"10.0.0.0/8","description":"internal"}]}`)
	if w.Code != 200 {
		t.Fatalf("ip-allow-list-only PATCH => 200, got %d: %s", w.Code, w.Body.String())
	}
	var pg PostgresView
	_ = json.Unmarshal(w.Body.Bytes(), &pg)
	if len(pg.IPAllowList) != 1 || pg.IPAllowList[0].CIDRBlock != "10.0.0.0/8" || pg.IPAllowList[0].Description != "internal" {
		t.Errorf("ipAllowList in response = %+v, want one {10.0.0.0/8, internal} entry", pg.IPAllowList)
	}
	if w2 := serveREST(svc, "GET", "/v1/postgres/upd-db", ""); w2.Code == 200 {
		var reGet PostgresView
		_ = json.Unmarshal(w2.Body.Bytes(), &reGet)
		// The description persists on the CR (w4/m24) — a re-read returns it,
		// never an empty stand-in.
		if len(reGet.IPAllowList) != 1 || reGet.IPAllowList[0].CIDRBlock != "10.0.0.0/8" || reGet.IPAllowList[0].Description != "internal" {
			t.Errorf("GET after PATCH ipAllowList = %+v, want the full entry to persist", reGet.IPAllowList)
		}
	}

	// clearing the allow-list with an explicit empty array — must clear, not
	// be treated as "field omitted".
	w = serveREST(svc, "PATCH", "/v1/postgres/upd-db", `{"ipAllowList":[]}`)
	if w.Code != 200 {
		t.Fatalf("ip-allow-list clear PATCH => 200, got %d: %s", w.Code, w.Body.String())
	}
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "upd-db"}, &got)
	if len(got.Spec.IPAllowList) != 0 {
		t.Errorf("spec.ipAllowList should be cleared, got %v", got.Spec.IPAllowList)
	}

	// Rename changes only spec.name: the API id and every physical identity
	// derived from metadata.name remain stable.
	beforeID, beforeDatabaseName, beforeUser, beforeHost := pg.ID, pg.DatabaseName, pg.DatabaseUser, pg.ExternalHost
	w = serveREST(svc, "PATCH", "/v1/postgres/upd-db", `{"name":"renamed-db"}`)
	if w.Code != 200 {
		t.Fatalf("rename => 200, got %d: %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &pg)
	if pg.ID != beforeID || pg.Name != "renamed-db" || pg.DatabaseName != beforeDatabaseName || pg.DatabaseUser != beforeUser || pg.ExternalHost != beforeHost {
		t.Errorf("rename changed stable identity/connection fields: before=%q/%q/%q/%q after=%+v", beforeID, beforeDatabaseName, beforeUser, beforeHost, pg)
	}
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "upd-db"}, &got)
	if got.Spec.Name != "renamed-db" || got.Name != "upd-db" {
		t.Fatalf("rename should patch only spec.name, got metadata.name=%q spec.name=%q", got.Name, got.Spec.Name)
	}
	if got.Labels[core.LabelProject] != "prj-platform" || got.Labels[core.LabelEnvironment] != "env-production" {
		t.Fatalf("rename changed project/environment membership labels: %+v", got.Labels)
	}
	if pg.ProjectID != "prj-platform" || pg.EnvironmentID != "env-production" {
		t.Fatalf("rename response changed project/environment membership: project=%q environment=%q", pg.ProjectID, pg.EnvironmentID)
	}

	// The official CLI resolves a name through GET /postgres?name=...; the new
	// display name resolves. This fixture is a grandfathered Database whose old
	// display name is also its immutable id, so that string intentionally remains
	// resolvable as an id after rename.
	var filtered []postgresWithCursor
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/postgres?name=renamed-db", "").Body.Bytes(), &filtered)
	if len(filtered) != 1 || filtered[0].Postgres.ID != "upd-db" {
		t.Fatalf("new name filter did not resolve stable id: %+v", filtered)
	}
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/postgres?name=upd-db", "").Body.Bytes(), &filtered)
	if len(filtered) != 1 || filtered[0].Postgres.ID != "upd-db" {
		t.Fatalf("stable id must remain resolvable through the name/id filter: %+v", filtered)
	}

	// A same-display-name rename is an idempotent success.
	w = serveREST(svc, "PATCH", "/v1/postgres/upd-db", `{"name":"renamed-db","diskSizeGB":25}`)
	if w.Code != 200 {
		t.Errorf("same-name PATCH => 200, got %d: %s", w.Code, w.Body.String())
	}

	// Invalid and colliding names fail specifically and leave the object alone.
	if w = serveREST(svc, "PATCH", "/v1/postgres/upd-db", `{"name":"Bad Name"}`); w.Code != 400 || !strings.Contains(w.Body.String(), "lowercase") {
		t.Errorf("invalid rename => descriptive 400, got %d: %s", w.Code, w.Body.String())
	}
	other := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "other-db", Namespace: "default"}, Spec: appv1alpha1.DatabaseSpec{Name: "taken", Plan: "free"}}
	if err := cl.Create(ctx, other); err != nil {
		t.Fatal(err)
	}
	if w = serveREST(svc, "PATCH", "/v1/postgres/upd-db", `{"name":"taken"}`); w.Code != 409 || !strings.Contains(w.Body.String(), "already exists") {
		t.Errorf("colliding rename => descriptive 409, got %d: %s", w.Code, w.Body.String())
	}

	// Rename dry-run previews the new name without persisting it.
	w = serveREST(svc, "PATCH", "/v1/postgres/upd-db?dryRun=true", `{"name":"preview-name"}`)
	if w.Code != 200 {
		t.Fatalf("rename dry-run => 200, got %d: %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &pg)
	if pg.Name != "preview-name" || pg.ID != "upd-db" {
		t.Errorf("rename dry-run preview = %+v", pg)
	}
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "upd-db"}, &got)
	if got.Spec.Name != "renamed-db" {
		t.Errorf("rename dry-run wrote spec.name = %q", got.Spec.Name)
	}

	// dry-run: previews the disk change without writing it.
	w = serveREST(svc, "PATCH", "/v1/postgres/upd-db?dryRun=true", `{"diskSizeGB":99}`)
	if w.Code != 200 {
		t.Fatalf("dry-run PATCH => 200, got %d: %s", w.Code, w.Body.String())
	}
	_ = json.Unmarshal(w.Body.Bytes(), &pg)
	if pg.DiskSizeGB != 99 {
		t.Errorf("dry-run response diskSizeGB = %d, want 99 (preview)", pg.DiskSizeGB)
	}
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "upd-db"}, &got)
	if got.Spec.StorageGB == 99 {
		t.Error("dry-run must not write to the CR")
	}
}

func TestRESTRenameOpaqueIDStopsResolvingOldDisplayName(t *testing.T) {
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-stable", Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Name: "old-name", Plan: "free"},
	}
	svc, _ := newService(db)

	w := serveREST(svc, "PATCH", "/v1/postgres/dpg-stable", `{"name":"new-name"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("rename => 200, got %d: %s", w.Code, w.Body.String())
	}

	assertIDs := func(filter string, want ...string) {
		t.Helper()
		var got []postgresWithCursor
		w := serveREST(svc, "GET", "/v1/postgres?name="+filter, "")
		if w.Code != http.StatusOK {
			t.Fatalf("filter %q => 200, got %d: %s", filter, w.Code, w.Body.String())
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode filter %q: %v", filter, err)
		}
		ids := make([]string, 0, len(got))
		for _, item := range got {
			ids = append(ids, item.Postgres.ID)
		}
		if !slices.Equal(ids, want) {
			t.Fatalf("filter %q ids = %v, want %v", filter, ids, want)
		}
	}

	assertIDs("new-name", "dpg-stable")
	assertIDs("dpg-stable", "dpg-stable")
	assertIDs("old-name")
}

func TestGraphQLSetPostgresPlan(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "gql-plan-db")
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	// valid plan change.
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { updateDatabasePlan(id:"gql-plan-db", plan:"basic-1gb") { id plan } }`,
		Context:       context.Background(),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("updateDatabasePlan: %v", res.Errors)
	}
	var got appv1alpha1.Database
	_ = cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "gql-plan-db"}, &got)
	if got.Spec.Plan != "basic-1gb" {
		t.Errorf("spec.plan after updateDatabasePlan = %q, want basic-1gb", got.Spec.Plan)
	}

	// invalid plan => graphql error.
	res = graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { updateDatabasePlan(id:"gql-plan-db", plan:"invalid") { id } }`,
		Context:       context.Background(),
	})
	if len(res.Errors) == 0 {
		t.Error("invalid plan must yield a GraphQL error")
	}
}

func TestMCPSetPostgresPlan(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "mcp-plan-db")

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

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "update_postgres_plan",
		Arguments: map[string]any{"postgresId": "mcp-plan-db", "plan": "basic-1gb"},
	})
	if err != nil || res.IsError {
		t.Fatalf("update_postgres_plan: err=%v isErr=%v", err, res.IsError)
	}
	var got appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "mcp-plan-db"}, &got); err != nil || got.Spec.Plan != "basic-1gb" {
		t.Fatalf("update_postgres_plan did not update spec.plan: %v %+v", err, got.Spec)
	}

	// invalid plan => tool error.
	bad, _ := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "update_postgres_plan",
		Arguments: map[string]any{"postgresId": "mcp-plan-db", "plan": "invalid"},
	})
	if !bad.IsError {
		t.Error("invalid plan must yield tool error")
	}
}
