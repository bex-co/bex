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

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/operator/api/v1alpha1"
)

const testToken = "secret-token"

// fakeAppClient builds the scheme + fake client every server test uses.
func fakeAppClient(t *testing.T, apps ...*appv1alpha1.App) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	objs := make([]client.Object, len(apps))
	for i, a := range apps {
		objs[i] = a
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func newServer(t *testing.T, apps ...*appv1alpha1.App) (http.Handler, client.Client) {
	t.Helper()
	cl := fakeAppClient(t, apps...)
	srv := &Server{
		Core:          &Core{Client: cl, Namespace: "default", Now: func() time.Time { return time.Unix(1_000_000, 0).UTC() }},
		HydraAdminURL: fakeHydraURL(t), // introspects testToken as active
	}
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	return h, cl
}

func sampleApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: name + ":v1", Replicas: 2},
		Status:     appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, URL: "https://" + name + ".onbex.co"},
	}
}

func do(t *testing.T, h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestAuth(t *testing.T) {
	h, _ := newServer(t, sampleApp("web"))
	if got := do(t, h, "GET", "/healthz", "", "").Code; got != 200 {
		t.Errorf("healthz should be open, got %d", got)
	}
	if got := do(t, h, "GET", "/v1/services", "", "").Code; got != 401 {
		t.Errorf("no token => 401, got %d", got)
	}
	if got := do(t, h, "GET", "/v1/services", "wrong", "").Code; got != 401 {
		t.Errorf("wrong token => 401, got %d", got)
	}
	if got := do(t, h, "GET", "/v1/services", testToken, "").Code; got != 200 {
		t.Errorf("valid token => 200, got %d", got)
	}
}

// --- REST adapter (Render-public-API compatible) ---

func TestREST_ListIsRenderWrapped(t *testing.T) {
	h, _ := newServer(t, sampleApp("web"), sampleApp("api"))
	// Render's GET /v1/services returns [{service, cursor}], not a bare array.
	var list []serviceWithCursor
	decode(t, do(t, h, "GET", "/v1/services", testToken, ""), &list)
	if len(list) != 2 {
		t.Fatalf("want 2 items, got %d", len(list))
	}
	if list[0].Service.ID == "" || list[0].Cursor == "" {
		t.Fatalf("each item must carry a service + cursor: %+v", list[0])
	}
	// /v1/apps is a bex-native alias for the same handler.
	var viaApps []serviceWithCursor
	decode(t, do(t, h, "GET", "/v1/apps", testToken, ""), &viaApps)
	if len(viaApps) != 2 {
		t.Errorf("/v1/apps alias should return the same list, got %d", len(viaApps))
	}
}

func TestREST_GetRenderShape(t *testing.T) {
	h, _ := newServer(t, sampleApp("web"))
	var svc renderService
	decode(t, do(t, h, "GET", "/v1/services/web", testToken, ""), &svc)
	if svc.ID != "web" || svc.Name != "web" {
		t.Errorf("id/name should be the app name: %+v", svc)
	}
	if svc.Type != renderWebService {
		t.Errorf("type should be web_service, got %q", svc.Type)
	}
	// Render's suspended is a STRING enum, not a bool.
	if svc.Suspended != "not_suspended" {
		t.Errorf("suspended should be not_suspended, got %q", svc.Suspended)
	}
	if code := do(t, h, "GET", "/v1/services/nope", testToken, "").Code; code != 404 {
		t.Errorf("unknown => 404, got %d", code)
	}
}

func TestREST_VerbsAndStatusCodes(t *testing.T) {
	h, cl := newServer(t, sampleApp("web"))

	// Render status codes: suspend/resume => 202, restart => 200.
	if code := do(t, h, "POST", "/v1/services/web/suspend", testToken, "").Code; code != 202 {
		t.Errorf("suspend => 202, got %d", code)
	}
	if got := getApp(t, cl, "web"); !got.Spec.Suspended || got.Spec.Replicas != 2 {
		t.Errorf("suspend set suspended without keeping replicas: %+v", got.Spec)
	}
	// The verb response body is the Render service with suspended="suspended".
	var svc renderService
	decode(t, do(t, h, "GET", "/v1/services/web", testToken, ""), &svc)
	if svc.Suspended != "suspended" {
		t.Errorf("after suspend, suspended should be %q, got %q", "suspended", svc.Suspended)
	}

	if code := do(t, h, "POST", "/v1/services/web/resume", testToken, "").Code; code != 202 {
		t.Errorf("resume => 202, got %d", code)
	}
	if getApp(t, cl, "web").Spec.Suspended {
		t.Error("resume should clear suspended")
	}

	if code := do(t, h, "POST", "/v1/services/web/restart", testToken, "").Code; code != 200 {
		t.Errorf("restart => 200 (Render), got %d", code)
	}
	if _, err := time.Parse(time.RFC3339, getApp(t, cl, "web").Spec.RestartedAt); err != nil {
		t.Errorf("restart should set an RFC3339 restartedAt: %v", err)
	}

	if code := do(t, h, "POST", "/v1/services/nope/restart", testToken, "").Code; code != 404 {
		t.Errorf("verb on unknown => 404, got %d", code)
	}
}

// --- GraphQL adapter (same Core, so same effects) ---

func gql(t *testing.T, h http.Handler, query string) map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"query": query})
	w := do(t, h, "POST", "/graphql", testToken, string(body))
	if w.Code != 200 {
		t.Fatalf("graphql http %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Data   map[string]any `json:"data"`
		Errors []any          `json:"errors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Errors) > 0 {
		t.Fatalf("graphql errors: %v", out.Errors)
	}
	return out.Data
}

func TestGraphQL_RenderOperationsAndShape(t *testing.T) {
	h, cl := newServer(t, sampleApp("web"))

	// Render dashboard query names: services, server(id). suspended is a string enum.
	data := gql(t, h, `{ services { id type suspended replicas } }`)
	svcs := data["services"].([]any)
	if len(svcs) != 1 {
		t.Fatalf("want 1 service, got %d", len(svcs))
	}
	first := svcs[0].(map[string]any)
	if first["type"] != renderWebService || first["suspended"] != renderNotSuspended {
		t.Fatalf("unexpected service shape: %+v", first)
	}

	gql(t, h, `{ server(id:"web") { id name } }`)

	// Render dashboard mutation names: suspendService/resumeService/restartServer.
	gql(t, h, `mutation { suspendService(id:"web") { id suspended } }`)
	if got := getApp(t, cl, "web"); !got.Spec.Suspended || got.Spec.Replicas != 2 {
		t.Error("graphql suspendService must suspend and keep replicas")
	}

	gql(t, h, `mutation { resumeService(id:"web") { id suspended } }`)
	if getApp(t, cl, "web").Spec.Suspended {
		t.Error("graphql resumeService did not resume")
	}

	gql(t, h, `mutation { restartServer(id:"web") { id } }`)
	if _, err := time.Parse(time.RFC3339, getApp(t, cl, "web").Spec.RestartedAt); err != nil {
		t.Errorf("graphql restartServer should set restartedAt: %v", err)
	}
}

func TestGraphQL_RequiresAuth(t *testing.T) {
	h, _ := newServer(t, sampleApp("web"))
	body, _ := json.Marshal(map[string]string{"query": `{ services { id } }`})
	if code := do(t, h, "POST", "/graphql", "", string(body)).Code; code != 401 {
		t.Errorf("graphql without token => 401, got %d", code)
	}
}

// --- managed Postgres (Render-shaped /v1/postgres) ---

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

func TestREST_PostgresCRUD(t *testing.T) {
	h, cl := newServer(t)

	// create — Render: POST /v1/postgres => 201; name with a hyphen normalizes.
	body := `{"name":"pg-test","plan":"free","public":true}`
	w := do(t, h, "POST", "/v1/postgres", testToken, body)
	if w.Code != 201 {
		t.Fatalf("create => 201, got %d: %s", w.Code, w.Body.String())
	}
	var pg PostgresView
	decode(t, w, &pg)
	if pg.ID != "pg-test" || pg.DatabaseName != "pg_test" || pg.DatabaseUser != "pg_test_user" {
		t.Fatalf("normalized names wrong: %+v", pg)
	}
	if pg.Plan != "free" || !pg.Public {
		t.Fatalf("spec not applied: %+v", pg)
	}
	// the CR really exists
	var got appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "pg-test"}, &got); err != nil {
		t.Fatalf("db CR not created: %v", err)
	}

	// list + get
	var list []PostgresView
	decode(t, do(t, h, "GET", "/v1/postgres", testToken, ""), &list)
	if len(list) != 1 {
		t.Fatalf("list => 1, got %d", len(list))
	}
	if do(t, h, "GET", "/v1/postgres/pg-test", testToken, "").Code != 200 {
		t.Fatalf("get failed")
	}
	if code := do(t, h, "GET", "/v1/postgres/nope", testToken, "").Code; code != 404 {
		t.Errorf("unknown => 404, got %d", code)
	}

	// delete — Render: 204
	if code := do(t, h, "DELETE", "/v1/postgres/pg-test", testToken, "").Code; code != 204 {
		t.Errorf("delete => 204, got %d", code)
	}
	if do(t, h, "GET", "/v1/postgres/pg-test", testToken, "").Code != 404 {
		t.Errorf("deleted db should be gone")
	}
}

func TestREST_PostgresConnectionInfo(t *testing.T) {
	h, cl := newServer(t)
	seedDatabase(t, cl, "conn-db")

	var ci PostgresConnectionInfo
	decode(t, do(t, h, "GET", "/v1/postgres/conn-db/connection-info", testToken, ""), &ci)

	if ci.Password != "s3cret" {
		t.Errorf("password = %q", ci.Password)
	}
	if ci.InternalConnectionString != "postgresql://conn_db_user:s3cret@conn-db-rw.default:5432/conn_db" {
		t.Errorf("internal = %q", ci.InternalConnectionString)
	}
	wantExt := "postgresql://conn_db_user:s3cret@conn-db.db.bex.co:5432/conn_db?sslmode=require&sslnegotiation=direct"
	if ci.ExternalConnectionString != wantExt {
		t.Errorf("external = %q", ci.ExternalConnectionString)
	}
	if ci.PSQLCommand == "" {
		t.Error("psqlCommand empty")
	}
}

func TestGraphQL_Postgres(t *testing.T) {
	h, cl := newServer(t)
	seedDatabase(t, cl, "gql-db")

	// Render dashboard GraphQL noun is "database" (verified via Playwright).
	data := gql(t, h, `{ databases { id databaseName status } }`)
	if len(data["databases"].([]any)) != 1 {
		t.Fatalf("databases want 1")
	}
	ci := gql(t, h, `{ databaseConnectionInfo(id:"gql-db") { externalConnectionString password } }`)["databaseConnectionInfo"].(map[string]any)
	if ci["password"] != "s3cret" || ci["externalConnectionString"] == "" {
		t.Fatalf("connection info: %+v", ci)
	}
	// create via mutation
	gql(t, h, `mutation { createDatabase(name:"gql-new", plan:"basic-1gb") { id databaseName } }`)
	var made appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "gql-new"}, &made); err != nil {
		t.Fatalf("createPostgres did not create the CR: %v", err)
	}
	if made.Spec.Plan != "basic-1gb" {
		t.Errorf("plan not applied: %s", made.Spec.Plan)
	}
}

func decode(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(bytes.NewReader(w.Body.Bytes())).Decode(v); err != nil {
		t.Fatalf("decode (%d): %v", w.Code, err)
	}
}

func getApp(t *testing.T, cl client.Client, name string) *appv1alpha1.App {
	t.Helper()
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &a); err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	return &a
}
