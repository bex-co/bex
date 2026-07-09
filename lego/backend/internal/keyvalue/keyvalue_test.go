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
	"context"
	"encoding/json"
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
			"uri":         []byte("redis://:s3cret@" + name + ".default.svc:6379"),
			"externalUri": []byte("rediss://:s3cret@" + name + ".kv.bex.co:6379"),
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

	// create — Render: 201.
	w := serveREST(svc, "POST", "/v1/key-value", `{"name":"cache-1","plan":"starter","public":true}`)
	if w.Code != 201 {
		t.Fatalf("create => 201, got %d: %s", w.Code, w.Body.String())
	}
	var kv KeyValueView
	_ = json.Unmarshal(w.Body.Bytes(), &kv)
	if kv.ID != "cache-1" || kv.Name != "cache-1" || kv.Plan != "starter" || !kv.Public {
		t.Fatalf("view wrong: %+v", kv)
	}
	if kv.Suspended != core.RenderNotSuspended {
		t.Errorf("fresh store should be not_suspended, got %q", kv.Suspended)
	}
	var got appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "cache-1"}, &got); err != nil {
		t.Fatalf("kv CR not created: %v", err)
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

func TestRESTKeyValueConnectionInfo(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "conn-kv")

	body := serveREST(svc, "GET", "/v1/key-value/conn-kv/connection-info", "").Body.Bytes()
	var ci KeyValueConnectionInfo
	_ = json.Unmarshal(body, &ci)
	if ci.InternalConnectionString != "redis://:s3cret@conn-kv.default.svc:6379" {
		t.Errorf("internal = %q", ci.InternalConnectionString)
	}
	if ci.ExternalConnectionString != "rediss://:s3cret@conn-kv.kv.bex.co:6379" {
		t.Errorf("external = %q", ci.ExternalConnectionString)
	}
	// cliCommand connects over the external (TLS) endpoint when public.
	if ci.CLICommand != "redis-cli -u rediss://:s3cret@conn-kv.kv.bex.co:6379" {
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
		Data:       map[string][]byte{"uri": []byte("redis://:pw@priv.default.svc:6379")},
	}
	_ = cl.Create(context.Background(), kv)
	_ = cl.Create(context.Background(), sec)

	var ci KeyValueConnectionInfo
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/key-value/priv/connection-info", "").Body.Bytes(), &ci)
	if ci.ExternalConnectionString != "" {
		t.Errorf("private store must have no external string, got %q", ci.ExternalConnectionString)
	}
	if ci.CLICommand != "redis-cli -u redis://:pw@priv.default.svc:6379" {
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
		res := graphql.Do(graphql.Params{Schema: schema, RequestString: q, Context: context.Background()})
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
	run(`mutation { createKeyValue(name:"gql-new", plan:"standard") { id } }`)
	var made appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "gql-new"}, &made); err != nil || made.Spec.Plan != "standard" {
		t.Fatalf("createKeyValue did not create the CR with plan: %v %+v", err, made.Spec)
	}
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

	// Render's tool names + arg (keyValueId), delegating to the same Core verbs.
	if list, ok := call("list_key_value_instances", nil)["keyValues"].([]any); !ok || len(list) != 1 {
		t.Fatalf("list_key_value_instances want 1, got %v", call("list_key_value_instances", nil))
	}
	if got := call("get_key_value", map[string]any{"keyValueId": "mcp-kv"}); got["id"] != "mcp-kv" {
		t.Fatalf("get_key_value id = %v", got["id"])
	}
	if got := call("create_key_value", map[string]any{"name": "mcp-new", "plan": "standard"}); got["id"] != "mcp-new" {
		t.Fatalf("create_key_value id = %v", got["id"])
	}
	var made appv1alpha1.KeyValue
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "mcp-new"}, &made); err != nil || made.Spec.Plan != "standard" {
		t.Fatalf("create_key_value did not create the CR: %v %+v", err, made.Spec)
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
