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

package keyvalue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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

// seedKeyValue adds a Ready public KeyValue + its operator-style credentials
// Secret (the keys the reconciler writes: username/password/host/port/uri, plus
// externalUri when public).
func seedKeyValue(t *testing.T, cl client.Client, name string) {
	t.Helper()
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "free", Public: true},
		Status: appv1alpha1.KeyValueStatus{
			Phase: appv1alpha1.KVPhaseReady, Host: name + ".default.svc", Port: 6379,
			SecretName: name, ExternalHost: name + ".kv.bex.co",
		},
	}
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Data: map[string][]byte{
			"username":    []byte("default"),
			"password":    []byte("s3cret"),
			"host":        []byte(name + ".default.svc"),
			"port":        []byte("6379"),
			"uri":         []byte("redis://default:s3cret@" + name + ".default.svc:6379"),
			"externalUri": []byte("rediss://default:s3cret@" + name + ".kv.bex.co:6379"),
		},
	}
	if err := cl.Create(context.Background(), kv); err != nil {
		t.Fatalf("seed kv: %v", err)
	}
	if err := cl.Create(context.Background(), sec); err != nil {
		t.Fatalf("seed secret: %v", err)
	}
}

func TestRESTKeyValueCRUD(t *testing.T) {
	svc, cl := newService()
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	svc.Environments = &fixedCreateEnvironment{assignment: core.EnvironmentAssignment{ID: "env-staging", ProjectID: "prj-platform", WorkspaceID: "tea-a"}}

	// create — Render: 201.
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/key-value", strings.NewReader(`{"name":"cache-1","plan":"starter","public":true,"environmentId":"env-staging"}`))
	mux.ServeHTTP(w, req.WithContext(ctxAs("user-a")))
	if w.Code != 201 {
		t.Fatalf("create => 201, got %d: %s", w.Code, w.Body.String())
	}
	var kv KeyValueView
	_ = json.Unmarshal(w.Body.Bytes(), &kv)
	if kv.ID != "cache-1" || kv.Name != "cache-1" || kv.Plan != "starter" || !kv.Public || kv.ProjectID != "prj-platform" || kv.EnvironmentID != "env-staging" {
		t.Fatalf("view wrong: %+v", kv)
	}
	if kv.Suspended != core.RenderNotSuspended {
		t.Errorf("fresh store should be not_suspended, got %q", kv.Suspended)
	}
	var got appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "cache-1"}, &got); err != nil {
		t.Fatalf("kv CR not created: %v", err)
	}
	if got.Labels[core.LabelProject] != "prj-platform" || got.Labels[core.LabelEnvironment] != "env-staging" {
		t.Fatalf("kv environment labels = %v", got.Labels)
	}

	var list []KeyValueView
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/key-value", "").Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list => 1, got %d", len(list))
	}
	if serveREST(svc, "GET", "/v1/key-value/cache-1", "").Code != 200 {
		t.Fatal("get failed")
	}
	if serveREST(svc, "GET", "/v1/key-value/nope", "").Code != 404 {
		t.Error("unknown => 404")
	}
	if serveREST(svc, "DELETE", "/v1/key-value/cache-1", "").Code != 204 {
		t.Error("delete => 204")
	}
	if serveREST(svc, "GET", "/v1/key-value/cache-1", "").Code != 404 {
		t.Error("deleted store should be gone")
	}
}

func TestKeyValueListPaginationAcrossRESTAndGraphQL(t *testing.T) {
	svc, cl := newService()
	for i := 22; i >= 0; i-- { // deliberately seed opposite cursor order
		seedKeyValue(t, cl, fmt.Sprintf("kv-%02d", i))
	}

	ctx := context.Background()
	all, err := svc.ListKeyValues(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	wantBody, _ := json.Marshal(svc.toKeyValueList(ctx, all))
	wantBody = append(wantBody, '\n')
	omitted := serveREST(svc, http.MethodGet, "/v1/key-value", "")
	if !bytes.Equal(omitted.Body.Bytes(), wantBody) {
		t.Fatalf("omitted params changed full-list body\ngot:  %s\nwant: %s", omitted.Body.Bytes(), wantBody)
	}

	var walked []string
	cursor := ""
	var firstREST []string
	for {
		path := "/v1/key-value?limit=7"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		var page []keyValueWithCursor
		if err := json.Unmarshal(serveREST(svc, http.MethodGet, path, "").Body.Bytes(), &page); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		if len(page) == 0 {
			break
		}
		ids := make([]string, 0, len(page))
		for _, item := range page {
			if item.Cursor != item.KeyValue.ID {
				t.Fatalf("cursor %q != key-value id %q", item.Cursor, item.KeyValue.ID)
			}
			ids = append(ids, item.KeyValue.ID)
		}
		if firstREST == nil {
			firstREST = slices.Clone(ids)
		}
		walked = append(walked, ids...)
		cursor = page[len(page)-1].Cursor
	}
	wantIDs := make([]string, 23)
	for i := range wantIDs {
		wantIDs[i] = fmt.Sprintf("kv-%02d", i)
	}
	if !slices.Equal(walked, wantIDs) {
		t.Fatalf("REST page walk = %v, want every id once in %v", walked, wantIDs)
	}
	var pastEnd []keyValueWithCursor
	_ = json.Unmarshal(serveREST(svc, http.MethodGet, "/v1/key-value?limit=7&cursor=does-not-exist", "").Body.Bytes(), &pastEnd)
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
		raw := res.Data.(map[string]any)["keyValues"].([]any)
		ids := make([]string, 0, len(raw))
		for _, item := range raw {
			ids = append(ids, item.(map[string]any)["id"].(string))
		}
		return ids
	}
	if got := gqlIDs(`{ keyValues { id } }`); len(got) != len(wantIDs) {
		t.Fatalf("omitted GraphQL args returned %d items, want full %d", len(got), len(wantIDs))
	}
	firstGQL := gqlIDs(`{ keyValues(limit: 7) { id } }`)
	if !slices.Equal(firstGQL, firstREST) {
		t.Fatalf("first REST/GQL pages drift: REST=%v GQL=%v", firstREST, firstGQL)
	}
	if got := gqlIDs(`{ keyValues(cursor: "kv-06", limit: 7) { id } }`); !slices.Equal(got, wantIDs[7:14]) {
		t.Fatalf("second GraphQL page = %v, want %v", got, wantIDs[7:14])
	}
	if got := gqlIDs(`{ keyValues(cursor: "does-not-exist", limit: 7) { id } }`); len(got) != 0 {
		t.Fatalf("unknown GraphQL cursor = %v, want empty page", got)
	}
}

func TestRESTKeyValueCreateValidation(t *testing.T) {
	svc, _ := newService()
	// unknown plan => 400 (not silently accepted).
	if w := serveREST(svc, "POST", "/v1/key-value", `{"name":"x","plan":"basic-1gb"}`); w.Code != 400 {
		t.Errorf("unknown plan => 400, got %d: %s", w.Code, w.Body.String())
	}
	// empty name => 400.
	if w := serveREST(svc, "POST", "/v1/key-value", `{"plan":"free"}`); w.Code != 400 {
		t.Errorf("empty name => 400, got %d", w.Code)
	}
	// a valid plan still creates.
	if w := serveREST(svc, "POST", "/v1/key-value", `{"name":"ok","plan":"standard"}`); w.Code != 201 {
		t.Errorf("valid plan => 201, got %d: %s", w.Code, w.Body.String())
	}
}

// TestKeyValueMaxmemoryPersistence pins that create carries the maxmemoryPolicy
// / persistenceMode settings (w5/011) onto spec, the view reads them back, and
// bad values are refused (400) rather than silently dropped — across REST +
// GraphQL.
func TestKeyValueMaxmemoryPersistence(t *testing.T) {
	svc, cl := newService()

	// REST create with both settings => spec seeded + view echoes them.
	w := serveREST(svc, "POST", "/v1/key-value",
		`{"name":"cache-mm","plan":"starter","maxmemoryPolicy":"volatile-ttl","persistenceMode":"off"}`)
	if w.Code != 201 {
		t.Fatalf("create => 201, got %d: %s", w.Code, w.Body.String())
	}
	var view renderKeyValue
	_ = json.Unmarshal(w.Body.Bytes(), &view)
	// The view echoes Render's underscore-separated wire format regardless of
	// which separator the create request used (renderToCRD/crdToRender), nested
	// under "options" (Render's real KeyValueDetail shape, not top-level).
	if view.Options.MaxmemoryPolicy != "volatile_ttl" || view.Options.PersistenceMode != "off" {
		t.Fatalf("view settings = %q/%q", view.Options.MaxmemoryPolicy, view.Options.PersistenceMode)
	}
	var made appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "cache-mm"}, &made); err != nil {
		t.Fatalf("get CR: %v", err)
	}
	if made.Spec.MaxmemoryPolicy != "volatile-ttl" || made.Spec.PersistenceMode != "off" {
		t.Fatalf("spec settings = %q/%q", made.Spec.MaxmemoryPolicy, made.Spec.PersistenceMode)
	}

	// Bad values => 400 (named), not a create.
	if w := serveREST(svc, "POST", "/v1/key-value", `{"name":"bad-mm","maxmemoryPolicy":"evict-everything"}`); w.Code != 400 {
		t.Errorf("bad maxmemoryPolicy => 400, got %d: %s", w.Code, w.Body.String())
	}
	if w := serveREST(svc, "POST", "/v1/key-value", `{"name":"bad-pm","persistenceMode":"maybe"}`); w.Code != 400 {
		t.Errorf("bad persistenceMode => 400, got %d: %s", w.Code, w.Body.String())
	}

	// GraphQL create carries the same settings onto spec.
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       context.Background(),
		RequestString: `mutation { createKeyValue(name:"cache-gql-mm", maxmemoryPolicy:"allkeys-lfu", persistenceMode:"snapshot") { maxmemoryPolicy persistenceMode } }`,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql create: %v", res.Errors)
	}
	obj := res.Data.(map[string]any)["createKeyValue"].(map[string]any)
	if obj["maxmemoryPolicy"] != "allkeys_lfu" || obj["persistenceMode"] != "snapshot" {
		t.Fatalf("gql view = %v", obj)
	}
	var gqlMade appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "cache-gql-mm"}, &gqlMade); err != nil {
		t.Fatalf("get gql CR: %v", err)
	}
	if gqlMade.Spec.MaxmemoryPolicy != "allkeys-lfu" || gqlMade.Spec.PersistenceMode != "snapshot" {
		t.Fatalf("gql spec = %q/%q", gqlMade.Spec.MaxmemoryPolicy, gqlMade.Spec.PersistenceMode)
	}
}

func TestRESTKeyValueConnectionInfo(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "conn-kv")

	body := serveREST(svc, "GET", "/v1/key-value/conn-kv/connection-info", "").Body.Bytes()
	var ci KeyValueConnectionInfo
	_ = json.Unmarshal(body, &ci)
	if ci.InternalConnectionString != "redis://default:s3cret@conn-kv.default.svc:6379" {
		t.Errorf("internal = %q", ci.InternalConnectionString)
	}
	if ci.ExternalConnectionString != "rediss://default:s3cret@conn-kv.kv.bex.co:6379" {
		t.Errorf("external = %q", ci.ExternalConnectionString)
	}
	// cliCommand connects over the external (TLS) endpoint when public.
	if ci.CLICommand != "redis-cli -u rediss://default:s3cret@conn-kv.kv.bex.co:6379" {
		t.Errorf("cliCommand = %q", ci.CLICommand)
	}
	// Render's keyValueConnectionInfo has no standalone password field — the
	// password lives inside the strings only.
	if strings.Contains(string(body), `"password"`) {
		t.Errorf("connection-info must not expose a standalone password field: %s", body)
	}
}

func TestRESTKeyValueConnectionInfoInternalOnly(t *testing.T) {
	svc, cl := newService()
	// a private store: no externalUri key, so no external string and the CLI uses internal.
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "priv", Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "free"},
		Status:     appv1alpha1.KeyValueStatus{Phase: appv1alpha1.KVPhaseReady, SecretName: "priv"},
	}
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "priv", Namespace: "default"},
		Data:       map[string][]byte{"uri": []byte("redis://default:pw@priv.default.svc:6379")},
	}
	_ = cl.Create(context.Background(), kv)
	_ = cl.Create(context.Background(), sec)

	var ci KeyValueConnectionInfo
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/key-value/priv/connection-info", "").Body.Bytes(), &ci)
	if ci.ExternalConnectionString != "" {
		t.Errorf("private store must have no external string, got %q", ci.ExternalConnectionString)
	}
	if ci.CLICommand != "redis-cli -u redis://default:pw@priv.default.svc:6379" {
		t.Errorf("cliCommand should use internal for a private store, got %q", ci.CLICommand)
	}
}

func TestRESTKeyValueSuspendResume(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "life")

	w := serveREST(svc, "POST", "/v1/key-value/life/suspend", "")
	if w.Code != 202 {
		t.Fatalf("suspend => 202, got %d", w.Code)
	}
	var v KeyValueView
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	if v.Suspended != core.RenderSuspended {
		t.Errorf("suspended enum = %q, want %q", v.Suspended, core.RenderSuspended)
	}
	var got appv1alpha1.KeyValue
	_ = cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "life"}, &got)
	if !got.Spec.Suspended {
		t.Error("suspend must set spec.suspended on the CR")
	}

	w = serveREST(svc, "POST", "/v1/key-value/life/resume", "")
	if w.Code != 202 {
		t.Fatalf("resume => 202, got %d", w.Code)
	}
	_ = json.Unmarshal(w.Body.Bytes(), &v)
	if v.Suspended != core.RenderNotSuspended {
		t.Errorf("resumed enum = %q, want %q", v.Suspended, core.RenderNotSuspended)
	}
	// unknown store => 404.
	if serveREST(svc, "POST", "/v1/key-value/nope/suspend", "").Code != 404 {
		t.Error("suspend unknown => 404")
	}
}

func TestGraphQLKeyValue(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "gql-kv")
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

	if len(run(`{ keyValues { id status } }`)["keyValues"].([]any)) != 1 {
		t.Fatal("keyValues want 1")
	}
	ci := run(`{ keyValueConnectionInfo(id:"gql-kv") { internalConnectionString externalConnectionString cliCommand } }`)["keyValueConnectionInfo"].(map[string]any)
	if ci["internalConnectionString"] == "" || ci["cliCommand"] == "" {
		t.Fatalf("connection info: %+v", ci)
	}
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	svc.Environments = &fixedCreateEnvironment{assignment: core.EnvironmentAssignment{ID: "env-staging", ProjectID: "prj-platform", WorkspaceID: "tea-a"}}
	created := run(`mutation { createKeyValue(name:"gql-new", plan:"standard", environmentId:"env-staging") { id projectId environmentId } }`)["createKeyValue"].(map[string]any)
	if created["projectId"] != "prj-platform" || created["environmentId"] != "env-staging" {
		t.Fatalf("createKeyValue association = %+v", created)
	}
	var made appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "gql-new"}, &made); err != nil || made.Spec.Plan != "standard" {
		t.Fatalf("createKeyValue did not create the CR with plan: %v %+v", err, made.Spec)
	}
	svc.Workspace = nil
	// suspend/resume mutations flip spec.suspended.
	run(`mutation { suspendKeyValue(id:"gql-kv") { suspended } }`)
	_ = cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "gql-kv"}, &made)
	if !made.Spec.Suspended {
		t.Error("suspendKeyValue must set spec.suspended")
	}
	run(`mutation { resumeKeyValue(id:"gql-kv") { suspended } }`)
	_ = cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "gql-kv"}, &made)
	if made.Spec.Suspended {
		t.Error("resumeKeyValue must clear spec.suspended")
	}

	tt := run(`{ keyValueInstanceTypes { id name cpu memory storageGB } }`)["keyValueInstanceTypes"].([]any)
	if len(tt) == 0 {
		t.Fatal("keyValueInstanceTypes want >=1")
	}
	first := tt[0].(map[string]any)
	if first["id"] != "free" || first["name"] != "Free" {
		t.Fatalf("first tier = %+v, want free/Free", first)
	}
}

func TestMCPKeyValue(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "mcp-kv")

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
	wantedLists := map[string]bool{"list_key_value": false, "list_key_value_instances": false}
	for _, tool := range tools.Tools {
		if _, ok := wantedLists[tool.Name]; !ok {
			continue
		}
		wantedLists[tool.Name] = true
		b, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Properties map[string]any `json:"properties"`
		}
		_ = json.Unmarshal(b, &schema)
		if len(schema.Properties) != 0 {
			t.Fatalf("%s args = %v, Render accepts none", tool.Name, schema.Properties)
		}
	}
	for name, found := range wantedLists {
		if !found {
			t.Fatalf("missing key-value list tool %q", name)
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

	// Render's tool names + arg (keyValueId), delegating to the same Core verbs.
	if list, ok := call("list_key_value", nil)["keyValues"].([]any); !ok || len(list) != 1 {
		t.Fatalf("list_key_value want 1, got %v", call("list_key_value", nil))
	}
	if list, ok := call("list_key_value_instances", nil)["keyValues"].([]any); !ok || len(list) != 1 {
		t.Fatalf("deprecated list alias want 1, got %v", call("list_key_value_instances", nil))
	}
	if got := call("get_key_value", map[string]any{"keyValueId": "mcp-kv"}); got["id"] != "mcp-kv" {
		t.Fatalf("get_key_value id = %v", got["id"])
	}
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	svc.Environments = &fixedCreateEnvironment{assignment: core.EnvironmentAssignment{ID: "env-staging", ProjectID: "prj-platform", WorkspaceID: "tea-a"}}
	if got := call("create_key_value", map[string]any{"name": "mcp-new", "plan": "standard", "environmentId": "env-staging"}); got["id"] != "mcp-new" || got["projectId"] != "prj-platform" || got["environmentId"] != "env-staging" {
		t.Fatalf("create_key_value id = %v", got["id"])
	}
	var made appv1alpha1.KeyValue
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "mcp-new"}, &made); err != nil || made.Spec.Plan != "standard" {
		t.Fatalf("create_key_value did not create the CR: %v %+v", err, made.Spec)
	}
}

// TestIPAllowList pins the service-level allowlist contract, mirroring the
// postgres sibling (advanced_test.go): a bad CIDR is rejected before any write,
// a valid list round-trips, and an empty list clears the spec field.
func TestIPAllowList(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "acl-kv")
	ctx := context.Background()

	// invalid CIDR rejected before any write
	if _, err := svc.SetIPAllowList(ctx, "acl-kv", []core.IPAllowListEntry{{CIDRBlock: "nonsense"}}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("bad CIDR should be ErrBadRequest, got %v", err)
	}
	var kv appv1alpha1.KeyValue
	_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "acl-kv"}, &kv)
	if len(kv.Spec.IPAllowList) != 0 {
		t.Fatal("a rejected allowlist must not be written")
	}

	if _, err := svc.SetIPAllowList(ctx, "acl-kv", []core.IPAllowListEntry{{CIDRBlock: "203.0.113.0/24", Description: "office"}, {CIDRBlock: "10.0.0.0/8"}}); err != nil {
		t.Fatalf("SetIPAllowList => %v", err)
	}
	got, err := svc.GetIPAllowList(ctx, "acl-kv")
	if err != nil || len(got) != 2 || got[0].CIDRBlock != "203.0.113.0/24" || got[0].Description != "office" {
		t.Fatalf("GetIPAllowList = %v (err %v)", got, err)
	}
	// empty clears it
	if _, err := svc.SetIPAllowList(ctx, "acl-kv", nil); err != nil {
		t.Fatalf("clear allowlist => %v", err)
	}
	if got, _ := svc.GetIPAllowList(ctx, "acl-kv"); len(got) != 0 {
		t.Fatalf("cleared allowlist should be empty, got %v", got)
	}
}

// TestRESTIPAllowList pins the REST wire shape: create seeds spec.ipAllowList,
// PUT/GET /ip-allow-list use the {"cidrs": [...]} envelope (byte-compatible with
// the postgres endpoints), and the view carries ipAllowList back.
func TestRESTIPAllowList(t *testing.T) {
	svc, cl := newService()

	// create with an allowlist seed — Render's real wire shape is
	// {cidrBlock,description} objects, not bare CIDR strings.
	w := serveREST(svc, "POST", "/v1/key-value", `{"name":"acl-rest","public":true,"ipAllowList":[{"cidrBlock":"203.0.113.0/24","description":"office"}]}`)
	if w.Code != 201 {
		t.Fatalf("create => 201, got %d: %s", w.Code, w.Body.String())
	}
	var made appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "acl-rest"}, &made); err != nil || len(made.Spec.IPAllowList) != 1 {
		t.Fatalf("create must seed spec.ipAllowList: %v %+v", err, made.Spec)
	}

	// PUT replaces; the response is the updated view carrying ipAllowList,
	// wrapped as Render's {cidrBlock,description} entries (not bare strings).
	w = serveREST(svc, "PUT", "/v1/key-value/acl-rest/ip-allow-list", `{"cidrs":["10.0.0.0/8","192.0.2.0/24"]}`)
	if w.Code != 200 {
		t.Fatalf("put => 200, got %d: %s", w.Code, w.Body.String())
	}
	var view renderKeyValue
	_ = json.Unmarshal(w.Body.Bytes(), &view)
	if len(view.IPAllowList) != 2 || view.IPAllowList[0].CIDRBlock != "10.0.0.0/8" {
		t.Fatalf("put view ipAllowList = %+v", view.IPAllowList)
	}

	// GET returns the {"cidrs": ...} envelope
	w = serveREST(svc, "GET", "/v1/key-value/acl-rest/ip-allow-list", "")
	if w.Code != 200 {
		t.Fatalf("get => 200, got %d: %s", w.Code, w.Body.String())
	}
	var env struct {
		CIDRs []string `json:"cidrs"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if len(env.CIDRs) != 2 || env.CIDRs[1] != "192.0.2.0/24" {
		t.Fatalf("get cidrs = %v", env.CIDRs)
	}

	// a bad CIDR is a 400
	w = serveREST(svc, "PUT", "/v1/key-value/acl-rest/ip-allow-list", `{"cidrs":["not-a-cidr"]}`)
	if w.Code != 400 {
		t.Fatalf("bad CIDR => 400, got %d: %s", w.Code, w.Body.String())
	}

	// create validates the seed too — same gate, no CR written
	w = serveREST(svc, "POST", "/v1/key-value", `{"name":"acl-bad","ipAllowList":[{"cidrBlock":"not-a-cidr"}]}`)
	if w.Code != 400 {
		t.Fatalf("create with bad CIDR => 400, got %d: %s", w.Code, w.Body.String())
	}
	var rejected appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "acl-bad"}, &rejected); err == nil {
		t.Fatal("a rejected create must not write the CR")
	}
}

// TestGraphQLIPAllowList pins the GraphQL surface: the keyValueIpAllowList
// query, the setKeyValueIpAllowList mutation, the ipAllowList view field, and
// the createKeyValue seed argument.
func TestGraphQLIPAllowList(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "acl-gql")
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

	// set, and read the field off the returned object
	set := run(`mutation { setKeyValueIpAllowList(id:"acl-gql", cidrs:["203.0.113.0/24"]) { ipAllowList } }`)["setKeyValueIpAllowList"].(map[string]any)
	if l := set["ipAllowList"].([]any); len(l) != 1 || l[0] != "203.0.113.0/24" {
		t.Fatalf("setKeyValueIpAllowList => %v", set)
	}
	// dedicated read query
	if l := run(`{ keyValueIpAllowList(id:"acl-gql") }`)["keyValueIpAllowList"].([]any); len(l) != 1 || l[0] != "203.0.113.0/24" {
		t.Fatalf("keyValueIpAllowList => %v", l)
	}
	// create with a seed
	run(`mutation { createKeyValue(name:"acl-gql-new", ipAllowList:["10.0.0.0/8"]) { id } }`)
	var made appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "acl-gql-new"}, &made); err != nil || len(made.Spec.IPAllowList) != 1 || made.Spec.IPAllowList[0].CIDR != "10.0.0.0/8" {
		t.Fatalf("createKeyValue must seed spec.ipAllowList: %v %+v", err, made.Spec)
	}

	// The description-carrying extension (w4/m24): the entries argument wins,
	// the description persists on the CR, and ipAllowListEntries returns it.
	set = run(`mutation { setKeyValueIpAllowList(id:"acl-gql", entries:[{cidrBlock:"192.0.2.0/24", description:"office"}]) { ipAllowListEntries { cidrBlock description } } }`)["setKeyValueIpAllowList"].(map[string]any)
	if l := set["ipAllowListEntries"].([]any); len(l) != 1 {
		t.Fatalf("setKeyValueIpAllowList entries => %v", set)
	} else if e := l[0].(map[string]any); e["cidrBlock"] != "192.0.2.0/24" || e["description"] != "office" {
		t.Fatalf("entries round-trip => %v", e)
	}
	var relabeled appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "acl-gql"}, &relabeled); err != nil ||
		len(relabeled.Spec.IPAllowList) != 1 || relabeled.Spec.IPAllowList[0] != (appv1alpha1.IPAllowEntry{CIDR: "192.0.2.0/24", Description: "office"}) {
		t.Fatalf("entries must persist on the CR spec: %v %+v", err, relabeled.Spec.IPAllowList)
	}
}

// TestMCPCreateIPAllowList pins that create_key_value (MCP's only KV write,
// Render's 3-tool set) carries the ipAllowList seed and the view returns it.
func TestMCPCreateIPAllowList(t *testing.T) {
	svc, cl := newService()

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
		Name: "create_key_value",
		Arguments: map[string]any{
			"name": "acl-mcp", "public": true,
			"ipAllowList": []string{"203.0.113.0/24"},
		},
	})
	if err != nil || res.IsError {
		t.Fatalf("create_key_value: err=%v isErr=%v", err, res.IsError)
	}
	out := map[string]any{}
	b, _ := json.Marshal(res.StructuredContent)
	_ = json.Unmarshal(b, &out)
	// The MCP view carries {cidrBlock, description} entries since w4/m24
	// (bare-string ipAllowList input still accepted, lifted with an empty
	// description).
	if l, ok := out["ipAllowList"].([]any); !ok || len(l) != 1 {
		t.Fatalf("create_key_value view ipAllowList = %v", out["ipAllowList"])
	} else if e, ok := l[0].(map[string]any); !ok || e["cidrBlock"] != "203.0.113.0/24" || e["description"] != "" {
		t.Fatalf("create_key_value view ipAllowList entry = %v", l[0])
	}
	var made appv1alpha1.KeyValue
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "acl-mcp"}, &made); err != nil || len(made.Spec.IPAllowList) != 1 {
		t.Fatalf("create_key_value must seed spec.ipAllowList: %v %+v", err, made.Spec)
	}
}

func TestKVInstanceTypesCatalog(t *testing.T) {
	svc, _ := newService()
	tt, err := svc.InstanceTypes(context.Background())
	if err != nil {
		t.Fatalf("InstanceTypes: %v", err)
	}
	// Sourced from lego/types/tiers.Valkey (free, starter, standard) — the id is
	// the spec.plan spelling createKeyValue accepts, so it must round-trip verbatim.
	byID := map[string]KeyValueInstanceType{}
	for _, it := range tt {
		byID[it.ID] = it
	}
	for _, want := range []string{"free", "starter", "standard"} {
		it, ok := byID[want]
		if !ok {
			t.Fatalf("%q missing from catalog: %+v", want, tt)
		}
		if it.CPU == "" || it.Memory == "" || it.StorageGB <= 0 {
			t.Fatalf("%q projection incomplete: %+v", want, it)
		}
	}
}

func TestKVTierDisplayName(t *testing.T) {
	for _, c := range []struct{ id, want string }{
		{"free", "Free"},
		{"starter", "Starter"},
		{"standard", "Standard"},
		{"", ""},
	} {
		if got := kvTierDisplayName(c.id); got != c.want {
			t.Errorf("kvTierDisplayName(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}

func newTenantService(ws core.WorkspaceResolver, kvs ...*appv1alpha1.KeyValue) (*Service, client.Client) {
	objs := make([]client.Object, len(kvs))
	for i, k := range kvs {
		objs[i] = k
	}
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Service{Base: &core.Base{Client: cl, Namespace: "default", Workspace: ws}}, cl
}

func tenantKV(name, tenantID string) *appv1alpha1.KeyValue {
	return &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    map[string]string{core.LabelTenant: tenantID},
		},
	}
}

// TestKeyValueCapEnforcement verifies that the (N+1)th key-value create is
// refused with ErrBadRequest while a second workspace can still create.
func TestKeyValueCapEnforcement(t *testing.T) {
	ws := fakeWorkspace{"user-a": "tea-a", "user-b": "tea-b"}
	ctx := func(subject string) context.Context {
		return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
	}

	// tea-a has one instance (at cap=1); tea-b has none.
	svc, _ := newTenantService(ws, tenantKV("kv-1", "tea-a"))
	svc.MaxKeyValues = 1

	// tea-a is at cap.
	if _, err := svc.CreateKeyValue(ctx("user-a"), CreateKeyValueRequest{Name: "kv-new", Plan: "free"}); err == nil {
		t.Fatal("create at cap: want ErrBadRequest, got nil")
	} else if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("create at cap: got %v, want ErrBadRequest", err)
	}

	// tea-b has zero instances — can still create despite tea-a being at cap.
	if _, err := svc.CreateKeyValue(ctx("user-b"), CreateKeyValueRequest{Name: "kv-b1", Plan: "free"}); err != nil {
		t.Errorf("second workspace create: %v, want success", err)
	}

	// MaxKeyValues=0: unlimited.
	svc2, _ := newTenantService(ws, tenantKV("kv-1", "tea-a"))
	svc2.MaxKeyValues = 0
	if _, err := svc2.CreateKeyValue(ctx("user-a"), CreateKeyValueRequest{Name: "kv-2", Plan: "free"}); err != nil {
		t.Errorf("unlimited cap: %v, want success", err)
	}

	// Store off (no Workspace resolver): cap is skipped.
	svc3, _ := newService(tenantKV("kv-1", "tea-a"))
	svc3.MaxKeyValues = 1
	if _, err := svc3.CreateKeyValue(context.Background(), CreateKeyValueRequest{Name: "kv-2", Plan: "free"}); err != nil {
		t.Errorf("store-off cap: %v, want success (no workspace to count against)", err)
	}
}

func TestRESTSetKeyValuePlan(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "plan-kv")

	// valid plan upgrade.
	w := serveREST(svc, "PATCH", "/v1/key-value/plan-kv", `{"plan":"standard"}`)
	if w.Code != 200 {
		t.Fatalf("PATCH plan => 200, got %d: %s", w.Code, w.Body.String())
	}
	var kv KeyValueView
	_ = json.Unmarshal(w.Body.Bytes(), &kv)
	if kv.Plan != "standard" {
		t.Errorf("plan in view = %q, want standard", kv.Plan)
	}
	var got appv1alpha1.KeyValue
	_ = cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "plan-kv"}, &got)
	if got.Spec.Plan != "standard" {
		t.Errorf("spec.plan on CR = %q, want standard", got.Spec.Plan)
	}

	// unknown plan => 400.
	if w := serveREST(svc, "PATCH", "/v1/key-value/plan-kv", `{"plan":"basic-1gb"}`); w.Code != 400 {
		t.Errorf("unknown plan => 400, got %d: %s", w.Code, w.Body.String())
	}
	// unknown instance => 404.
	if w := serveREST(svc, "PATCH", "/v1/key-value/missing", `{"plan":"free"}`); w.Code != 404 {
		t.Errorf("missing kv => 404, got %d", w.Code)
	}
}

func TestGraphQLSetKeyValuePlan(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "gql-plan-kv")
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
		RequestString: `mutation { updateKeyValuePlan(id:"gql-plan-kv", plan:"standard") { id plan } }`,
		Context:       context.Background(),
	})
	if len(res.Errors) > 0 {
		t.Fatalf("updateKeyValuePlan: %v", res.Errors)
	}
	var got appv1alpha1.KeyValue
	_ = cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "gql-plan-kv"}, &got)
	if got.Spec.Plan != "standard" {
		t.Errorf("spec.plan after updateKeyValuePlan = %q, want standard", got.Spec.Plan)
	}

	// invalid plan => graphql error.
	res = graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `mutation { updateKeyValuePlan(id:"gql-plan-kv", plan:"invalid") { id } }`,
		Context:       context.Background(),
	})
	if len(res.Errors) == 0 {
		t.Error("invalid plan must yield a GraphQL error")
	}
}

func TestMCPSetKeyValuePlan(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "mcp-plan-kv")

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
		Name:      "update_key_value_plan",
		Arguments: map[string]any{"keyValueId": "mcp-plan-kv", "plan": "standard"},
	})
	if err != nil || res.IsError {
		t.Fatalf("update_key_value_plan: err=%v isErr=%v", err, res.IsError)
	}
	var got appv1alpha1.KeyValue
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "mcp-plan-kv"}, &got); err != nil || got.Spec.Plan != "standard" {
		t.Fatalf("update_key_value_plan did not update spec.plan: %v %+v", err, got.Spec)
	}

	// invalid plan => tool error.
	bad, _ := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "update_key_value_plan",
		Arguments: map[string]any{"keyValueId": "mcp-plan-kv", "plan": "invalid"},
	})
	if !bad.IsError {
		t.Error("invalid plan must yield tool error")
	}
}
