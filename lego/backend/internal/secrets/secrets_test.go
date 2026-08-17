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

package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

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

// --- test harness -------------------------------------------------------------

func testScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	return scheme
}

func fakeClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(objs...).Build()
}

func sampleApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: name + ":v1", Replicas: 2},
	}
}

func fixedNow() time.Time { return time.Unix(1_000_000, 0).UTC() }

func newService(store core.SecretKV, objs ...client.Object) *Service {
	return &Service{Base: &core.Base{Client: fakeClient(objs...), Namespace: "default", Clock: fixedNow}, Store: store}
}

func getSecret(t *testing.T, cl client.Client, name string) *corev1.Secret {
	t.Helper()
	var s corev1.Secret
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &s); err != nil {
		t.Fatalf("get secret %s: %v", name, err)
	}
	return &s
}

func getApp(t *testing.T, cl client.Client, name string) *appv1alpha1.App {
	t.Helper()
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &a); err != nil {
		t.Fatalf("get app %s: %v", name, err)
	}
	return &a
}

// fakeSecretStore is an in-memory core.SecretKV, keyed by logical path (e.g.
// "services/web/env"). failGet/failPut inject store outages so the error paths are
// exercised too.
type fakeSecretStore struct {
	m       map[string]map[string]string
	deletes int
	failGet error
	failPut error
}

func newFakeSecretStore() *fakeSecretStore {
	return &fakeSecretStore{m: map[string]map[string]string{}}
}

func (f *fakeSecretStore) Get(_ context.Context, path string) (map[string]string, error) {
	if f.failGet != nil {
		return nil, f.failGet
	}
	out := map[string]string{}
	for k, v := range f.m[path] {
		out[k] = v
	}
	return out, nil
}

func (f *fakeSecretStore) Put(_ context.Context, path string, data map[string]string) error {
	if f.failPut != nil {
		return f.failPut
	}
	cp := map[string]string{}
	for k, v := range data {
		cp[k] = v
	}
	f.m[path] = cp
	return nil
}

func (f *fakeSecretStore) Delete(_ context.Context, path string) error {
	f.deletes++
	delete(f.m, path)
	return nil
}

func (f *fakeSecretStore) List(_ context.Context, path string) ([]string, error) {
	prefix := path + "/"
	seen := map[string]bool{}
	var out []string
	for k := range f.m {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		rest := strings.TrimPrefix(k, prefix)
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			rest = rest[:i]
		}
		if !seen[rest] {
			seen[rest] = true
			out = append(out, rest)
		}
	}
	return out, nil
}

// fakeChecker answers authorization uniformly and records the last relation.
type fakeChecker struct {
	allow        bool
	lastRelation string
}

func (c *fakeChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	c.lastRelation = relation
	return c.allow, nil
}

// --- Service verbs ------------------------------------------------------------

func TestEnvVars_RoundTripAndMaterialize(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	ctx := context.Background()

	got, err := svc.SetEnvVars(ctx, "web", []EnvVarView{{Key: "FOO", Value: "bar"}, {Key: "APP_KEY", Value: "s3cret"}})
	if err != nil {
		t.Fatalf("SetEnvVars: %v", err)
	}
	if len(got) != 2 || got[0].Key != "APP_KEY" || got[1].Key != "FOO" {
		t.Fatalf("SetEnvVars should return key-sorted set, got %+v", got)
	}
	if store.m[envPath("web")]["APP_KEY"] != "s3cret" || store.m[envPath("web")]["FOO"] != "bar" {
		t.Fatalf("store not written: %+v", store.m[envPath("web")])
	}

	// Materialized into <app>-env, owned by the App, and the App consumes it + rolls.
	sec := getSecret(t, svc.Client, "web-env")
	if string(sec.Data["APP_KEY"]) != "s3cret" || string(sec.Data["FOO"]) != "bar" {
		t.Fatalf("secret not materialized: %+v", sec.Data)
	}
	if len(sec.OwnerReferences) != 1 || sec.OwnerReferences[0].Name != "web" {
		t.Fatalf("secret should be owned by the App: %+v", sec.OwnerReferences)
	}
	app := getApp(t, svc.Client, "web")
	if app.Spec.EnvFromSecret != "web-env" || app.Spec.RestartedAt == "" {
		t.Fatalf("app should consume the secret and roll: %+v", app.Spec)
	}

	// DeleteEnvVar removes one; the stale key must not linger in the Secret.
	if err := svc.DeleteEnvVar(ctx, "web", "FOO"); err != nil {
		t.Fatalf("DeleteEnvVar: %v", err)
	}
	if _, ok := getSecret(t, svc.Client, "web-env").Data["FOO"]; ok {
		t.Fatal("FOO lingered in materialized Secret")
	}
	// Removing the last key deletes the OpenBao path outright (storeEnv).
	if err := svc.DeleteEnvVar(ctx, "web", "APP_KEY"); err != nil {
		t.Fatalf("DeleteEnvVar last: %v", err)
	}
	if store.deletes != 1 || len(store.m[envPath("web")]) != 0 {
		t.Errorf("emptying the set should DeleteEnv once: deletes=%d m=%+v", store.deletes, store.m[envPath("web")])
	}
}

func TestEnvVar_SingleKey(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	ctx := context.Background()

	if _, err := svc.SetEnvVars(ctx, "web", []EnvVarView{{Key: "KEEP", Value: "1"}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := svc.SetEnvVar(ctx, "web", "ADDED", EnvVarWrite{Value: "2"})
	if err != nil || got.Key != "ADDED" || got.Value != "2" {
		t.Fatalf("SetEnvVar: %+v err=%v", got, err)
	}
	if store.m[envPath("web")]["KEEP"] != "1" || store.m[envPath("web")]["ADDED"] != "2" {
		t.Fatalf("SetEnvVar should merge, not replace: %+v", store.m[envPath("web")])
	}
	one, err := svc.GetEnvVar(ctx, "web", "ADDED")
	if err != nil || one.Value != "2" {
		t.Fatalf("GetEnvVar: %+v err=%v", one, err)
	}
	if _, err := svc.GetEnvVar(ctx, "web", "MISSING"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GetEnvVar unknown key: %v", err)
	}

	// core.EnvVarReader: EnvVarKeys is keys-only (value empty), EnvVarValue reads one.
	keys, err := svc.EnvVarKeys(ctx, "web")
	if err != nil || len(keys) != 2 {
		t.Fatalf("EnvVarKeys: %+v err=%v", keys, err)
	}
	for _, k := range keys {
		if k.Value != "" || k.ID != k.Key {
			t.Fatalf("EnvVarKeys must be keys-only with id==key: %+v", k)
		}
	}
	if v, err := svc.EnvVarValue(ctx, "web", "ADDED"); err != nil || v.Value != "2" {
		t.Fatalf("EnvVarValue: %+v err=%v", v, err)
	}
}

func TestEnvVars_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("nil store => unavailable", func(t *testing.T) {
		svc := newService(nil, sampleApp("web"))
		if _, err := svc.ListEnvVars(ctx, "web"); !errors.Is(err, core.ErrSecretsUnavailable) {
			t.Errorf("List: %v", err)
		}
		if _, err := svc.SetEnvVars(ctx, "web", nil); !errors.Is(err, core.ErrSecretsUnavailable) {
			t.Errorf("Set: %v", err)
		}
		if err := svc.DeleteEnvVar(ctx, "web", "X"); !errors.Is(err, core.ErrSecretsUnavailable) {
			t.Errorf("Delete: %v", err)
		}
	})

	t.Run("unknown service => not found", func(t *testing.T) {
		svc := newService(newFakeSecretStore(), sampleApp("web"))
		if _, err := svc.ListEnvVars(ctx, "nope"); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("List: %v", err)
		}
		if _, err := svc.SetEnvVars(ctx, "nope", []EnvVarView{{Key: "A", Value: "b"}}); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Set: %v", err)
		}
	})

	t.Run("invalid key => bad request, value never leaks", func(t *testing.T) {
		store := newFakeSecretStore()
		svc := newService(store, sampleApp("web"))
		_, err := svc.SetEnvVars(ctx, "web", []EnvVarView{{Key: "BAD KEY", Value: "topsecret"}})
		if !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("want ErrBadRequest, got %v", err)
		}
		if !strings.Contains(err.Error(), "BAD KEY") || strings.Contains(err.Error(), "topsecret") {
			t.Errorf("error should name the key, never the value: %v", err)
		}
		if _, ok := store.m[envPath("web")]; ok {
			t.Error("invalid write should not have stored anything")
		}
	})

	t.Run("store outage surfaces without the value", func(t *testing.T) {
		store := newFakeSecretStore()
		store.failPut = errors.New("openbao sealed")
		svc := newService(store, sampleApp("web"))
		_, err := svc.SetEnvVars(ctx, "web", []EnvVarView{{Key: "A", Value: "topsecret"}})
		if err == nil || strings.Contains(err.Error(), "topsecret") {
			t.Fatalf("store error should surface without the value: %v", err)
		}
	})
}

func TestEnvVars_Authorization(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-1", Method: "oauth2"})
	chk := &fakeChecker{allow: false}
	svc := &Service{Base: &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: chk}, Store: newFakeSecretStore()}

	if _, err := svc.ListEnvVars(ctx, "web"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("List deny: %v", err)
	}
	if chk.lastRelation != core.RelCanViewSensitive {
		t.Errorf("read checked %s, want can_view_sensitive", chk.lastRelation)
	}
	if _, err := svc.SetEnvVars(ctx, "web", nil); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("Set deny: %v", err)
	}
	if chk.lastRelation != core.RelCanCreate {
		t.Errorf("write checked %s, want can_create", chk.lastRelation)
	}
}

func TestListEnvVarsPageVisitsEveryKeyExactlyOnce(t *testing.T) {
	svc := newService(newFakeSecretStore(), sampleApp("web"))
	ctx := context.Background()
	input := []EnvVarView{
		{Key: "E", Value: "5"}, {Key: "A", Value: "1"}, {Key: "D", Value: "4"},
		{Key: "B", Value: "2"}, {Key: "C", Value: "3"},
	}
	if _, err := svc.SetEnvVars(ctx, "web", input); err != nil {
		t.Fatal(err)
	}

	var got []string
	cursor := ""
	for {
		page, err := svc.ListEnvVarsPage(ctx, "web", cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for _, v := range page {
			got = append(got, v.Key)
		}
		cursor = page[len(page)-1].Key
	}
	if want := []string{"A", "B", "C", "D", "E"}; !slices.Equal(got, want) {
		t.Fatalf("paged keys = %v, want %v", got, want)
	}
	all, err := svc.ListEnvVarsPage(ctx, "web", "", 0)
	if err != nil || len(all) != len(input) {
		t.Fatalf("omitted pagination = %d vars, %v; want all %d", len(all), err, len(input))
	}
}

// --- REST fragment ------------------------------------------------------------

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

func TestREST_EnvVars(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))

	// PUT replace-all => Render envVarWithCursor envelope, key-sorted.
	w := serveREST(svc, "PUT", "/v1/services/web/env-vars", `[{"key":"FOO","value":"bar"},{"key":"BAZ","value":"qux"}]`)
	if w.Code != 200 {
		t.Fatalf("PUT => 200, got %d: %s", w.Code, w.Body.String())
	}
	var put []envVarWithCursor
	_ = json.Unmarshal(w.Body.Bytes(), &put)
	if len(put) != 2 || put[0].EnvVar.Key != "BAZ" || put[0].Cursor == "" {
		t.Fatalf("PUT response not Render-shaped/sorted: %+v", put)
	}
	if string(getSecret(t, svc.Client, "web-env").Data["FOO"]) != "bar" {
		t.Fatal("PUT did not materialize")
	}

	// single-key GET => bare {key,value}; 404 unknown.
	var one EnvVarView
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/services/web/env-vars/FOO", "").Body.Bytes(), &one)
	if one.Key != "FOO" || one.Value != "bar" {
		t.Fatalf("single GET bare shape: %+v", one)
	}
	if serveREST(svc, "GET", "/v1/services/web/env-vars/NOPE", "").Code != 404 {
		t.Error("single GET unknown key => 404")
	}

	// single-key PUT upserts (merge), body {value}.
	_ = json.Unmarshal(serveREST(svc, "PUT", "/v1/services/web/env-vars/NEW", `{"value":"z"}`).Body.Bytes(), &one)
	if one.Key != "NEW" || one.Value != "z" || store.m[envPath("web")]["FOO"] != "bar" {
		t.Fatalf("single PUT should merge: %+v store=%+v", one, store.m[envPath("web")])
	}

	// DELETE one => 204; unknown key => 404.
	if serveREST(svc, "DELETE", "/v1/services/web/env-vars/NEW", "").Code != 204 {
		t.Error("delete => 204")
	}
	if serveREST(svc, "DELETE", "/v1/services/web/env-vars/NOPE", "").Code != 404 {
		t.Error("delete unknown key => 404")
	}
	if serveREST(svc, "GET", "/v1/services/web/env-vars", "").Code != 200 {
		t.Error("canonical service route should work")
	}
	if serveREST(svc, "GET", "/v1/services/ghost/env-vars", "").Code != 404 {
		t.Error("unknown service => 404")
	}

	// Requested pagination is cursor-exclusive; omitting both params remains
	// the pre-pagination full-list behavior.
	var firstPage, secondPage []envVarWithCursor
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/services/web/env-vars?limit=1", "").Body.Bytes(), &firstPage)
	if len(firstPage) != 1 {
		t.Fatalf("first page = %+v, want one item", firstPage)
	}
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/services/web/env-vars?limit=1&cursor="+firstPage[0].Cursor, "").Body.Bytes(), &secondPage)
	if len(secondPage) != 1 || secondPage[0].EnvVar.Key == firstPage[0].EnvVar.Key {
		t.Fatalf("second page = %+v after %+v", secondPage, firstPage)
	}
	var unpaged []envVarWithCursor
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/services/web/env-vars", "").Body.Bytes(), &unpaged)
	if len(unpaged) != 2 {
		t.Fatalf("unpaged list = %+v, want the complete two-item set", unpaged)
	}
}

func TestRESTSecretPathsResolveTypedServiceIDToPublicName(t *testing.T) {
	store := newFakeSecretStore()
	app := sampleApp("tea-example-web")
	app.Labels = map[string]string{
		core.LabelAppID:       "srv-example",
		core.LabelServiceName: "web",
	}
	store.m[envPath("web")] = map[string]string{"FOO": "bar"}
	store.m[filesPath("web")] = map[string]string{"config.txt": "contents"}
	svc := newService(store, app)

	env := serveREST(svc, "GET", "/v1/services/srv-example/env-vars/FOO", "")
	if env.Code != http.StatusOK || !strings.Contains(env.Body.String(), `"value":"bar"`) {
		t.Fatalf("typed-id env read = %d: %s", env.Code, env.Body.String())
	}
	file := serveREST(svc, "GET", "/v1/services/srv-example/secret-files/config.txt", "")
	if file.Code != http.StatusOK || !strings.Contains(file.Body.String(), `"content":"contents"`) {
		t.Fatalf("typed-id secret-file read = %d: %s", file.Code, file.Body.String())
	}
}

func TestREST_UnconfiguredIs503(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	if serveREST(svc, "GET", "/v1/services/web/env-vars", "").Code != 503 {
		t.Error("GET without a store => 503")
	}
	if serveREST(svc, "PUT", "/v1/services/web/env-vars", `[]`).Code != 503 {
		t.Error("PUT without a store => 503")
	}
}

func TestREST_GenerateValue(t *testing.T) {
	svc := newService(newFakeSecretStore(), sampleApp("web"))
	var bulk []envVarWithCursor
	rec := serveREST(svc, "PUT", "/v1/services/web/env-vars", `[{"key":"BULK_TOKEN","generateValue":true}]`)
	if rec.Code != http.StatusOK {
		t.Fatalf("generated bulk PUT => %d: %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &bulk)
	if len(bulk) != 1 || len(bulk[0].EnvVar.Value) != 44 {
		t.Fatalf("generated bulk response = %+v", bulk)
	}
	var generated EnvVarView
	rec = serveREST(svc, "PUT", "/v1/services/web/env-vars/TOKEN", `{"generateValue":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("generated PUT => %d: %s", rec.Code, rec.Body.String())
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &generated)
	if generated.Key != "TOKEN" || len(generated.Value) != 44 {
		t.Fatalf("generated response = %+v", generated)
	}
	var revealed EnvVarView
	_ = json.Unmarshal(serveREST(svc, "GET", "/v1/services/web/env-vars/TOKEN", "").Body.Bytes(), &revealed)
	if revealed.Value != generated.Value {
		t.Fatalf("revealed value = %q, generated = %q", revealed.Value, generated.Value)
	}
	rec = serveREST(svc, "PUT", "/v1/services/web/env-vars/TOKEN", `{"value":"literal","generateValue":true}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("conflicting PUT => %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// --- MCP fragment -------------------------------------------------------------

func mcpSession(t *testing.T, svc *Service) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	srv := mcp.NewServer(&mcp.Implementation{Name: "bex", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestMCP_EnvVars(t *testing.T) {
	store := newFakeSecretStore()
	cs := mcpSession(t, newService(store, sampleApp("web")))
	call := func(name string, args map[string]any, out any) {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("call %s: err=%v isErr=%v", name, err, res != nil && res.IsError)
		}
		if out != nil {
			b, _ := json.Marshal(res.StructuredContent)
			_ = json.Unmarshal(b, out)
		}
	}

	var res envVarsResult
	call("update_env_vars", map[string]any{"serviceId": "web", "envVars": []map[string]any{
		{"key": "FOO", "value": "bar"},
		{"key": "BULK_RANDOM", "generateValue": true},
	}}, &res)
	if len(res.EnvVars) != 2 || store.m[envPath("web")]["FOO"] != "bar" || len(store.m[envPath("web")]["BULK_RANDOM"]) != 44 {
		t.Fatalf("update_env_vars: %+v store=%+v", res.EnvVars, store.m[envPath("web")])
	}
	var single EnvVarView
	call("set_env_var", map[string]any{"serviceId": "web", "key": "NEW", "value": "z"}, &single)
	if single.Key != "NEW" || store.m[envPath("web")]["FOO"] != "bar" {
		t.Fatalf("set_env_var should merge: %+v", store.m[envPath("web")])
	}
	call("get_env_var", map[string]any{"serviceId": "web", "key": "FOO"}, &single)
	if single.Value != "bar" {
		t.Fatalf("get_env_var: %+v", single)
	}
	call("list_env_vars", map[string]any{"serviceId": "web"}, &res)
	if len(res.EnvVars) != 3 {
		t.Fatalf("list_env_vars: %+v", res.EnvVars)
	}
	call("list_env_vars", map[string]any{"serviceId": "web", "limit": 1}, &res)
	if len(res.EnvVars) != 1 || res.Cursor == "" {
		t.Fatalf("paged list_env_vars: %+v", res)
	}
	var next envVarsResult
	call("list_env_vars", map[string]any{"serviceId": "web", "limit": 1, "cursor": res.Cursor}, &next)
	if len(next.EnvVars) != 1 || next.EnvVars[0].Key == res.EnvVars[0].Key {
		t.Fatalf("next list_env_vars page: first=%+v next=%+v", res, next)
	}
	call("set_env_var", map[string]any{"serviceId": "web", "key": "RANDOM", "generateValue": true}, &single)
	if len(single.Value) != 44 || store.m[envPath("web")]["RANDOM"] != single.Value {
		t.Fatalf("generated set_env_var: %+v", single)
	}
	var del deleteEnvVarResult
	call("delete_env_var", map[string]any{"serviceId": "web", "key": "FOO"}, &del)
	if !del.Deleted || store.m[envPath("web")]["FOO"] != "" || len(store.m[envPath("web")]) != 3 {
		t.Fatalf("delete_env_var: %+v store=%+v", del, store.m[envPath("web")])
	}
}

func TestMCP_SecretFiles(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	cs := mcpSession(t, svc)
	call := func(name string, args map[string]any, out any) {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("call %s: err=%v isErr=%v", name, err, res != nil && res.IsError)
		}
		if out != nil {
			b, _ := json.Marshal(res.StructuredContent)
			_ = json.Unmarshal(b, out)
		}
	}

	// Seed three files, then exercise the same list/page/cursor contract as env vars.
	var file SecretFileView
	for _, name := range []string{"c.pem", "a.pem", "b.pem"} {
		call("set_secret_file", map[string]any{"serviceId": "web", "name": name, "content": "x"}, &file)
	}
	var res secretFilesResult
	call("list_secret_files", map[string]any{"serviceId": "web"}, &res)
	if len(res.SecretFiles) != 3 || res.Cursor != "" {
		t.Fatalf("unpaged list_secret_files: %+v", res)
	}
	call("list_secret_files", map[string]any{"serviceId": "web", "limit": 1}, &res)
	if len(res.SecretFiles) != 1 || res.Cursor == "" {
		t.Fatalf("paged list_secret_files: %+v", res)
	}
	var next secretFilesResult
	call("list_secret_files", map[string]any{"serviceId": "web", "limit": 1, "cursor": res.Cursor}, &next)
	if len(next.SecretFiles) != 1 || next.SecretFiles[0].Name == res.SecretFiles[0].Name {
		t.Fatalf("next list_secret_files page: first=%+v next=%+v", res, next)
	}
	// The list never carries content — that stays a per-name get_secret_file read.
	if res.SecretFiles[0].Content != "" {
		t.Fatalf("list must be names-only: %+v", res.SecretFiles[0])
	}
	call("get_secret_file", map[string]any{"serviceId": "web", "name": "a.pem"}, &file)
	if file.Content != "x" {
		t.Fatalf("get_secret_file content: %+v", file)
	}
}

// --- OpenBao KV v2 client (httptest stub) --------------------------------------

// baoStub emulates OpenBao's k8s login + KV v2 data read/write + metadata delete,
// requiring the login flow first and checking X-Vault-Token thereafter.
type baoStub struct {
	t       *testing.T
	data    map[string]map[string]string
	token   string
	logins  int
	rejectN int // reject the first N KV calls with 403 (stale-token retry test)
}

func (b *baoStub) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/auth/kubernetes/login", func(w http.ResponseWriter, _ *http.Request) {
		b.logins++
		b.token = "bao-token-" + string(rune('0'+b.logins))
		core.WriteJSON(w, 200, map[string]any{"auth": map[string]any{"client_token": b.token, "lease_duration": 3600}})
	})
	kv := func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != b.token {
			http.Error(w, "missing token", http.StatusForbidden)
			return
		}
		if b.rejectN > 0 {
			b.rejectN--
			http.Error(w, "permission denied", http.StatusForbidden)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/v1/tenants/data/")
		path = strings.TrimPrefix(path, "/v1/tenants/metadata/")
		switch r.Method {
		case http.MethodGet:
			env, ok := b.data[path]
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			core.WriteJSON(w, 200, map[string]any{"data": map[string]any{"data": env}})
		case http.MethodPost:
			var in struct {
				Data map[string]string `json:"data"`
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				b.t.Fatalf("decode KV body: %v", err)
			}
			b.data[path] = in.Data
			core.WriteJSON(w, 200, map[string]any{})
		case http.MethodDelete:
			delete(b.data, path)
			w.WriteHeader(http.StatusNoContent)
		}
	}
	mux.HandleFunc("/v1/tenants/data/", kv)
	mux.HandleFunc("/v1/tenants/metadata/", kv)
	return mux
}

func newOpenBaoStoreForTest(t *testing.T, addr string) *openBaoStore {
	t.Helper()
	jwt := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(jwt, []byte("fake-sa-jwt"), 0o600); err != nil {
		t.Fatal(err)
	}
	return &openBaoStore{
		addr: addr, role: baoRole, mount: baoMount,
		jwtPath: jwt, client: &http.Client{Timeout: 5 * time.Second},
	}
}

func TestOpenBaoStore_RoundTrip(t *testing.T) {
	stub := &baoStub{t: t, data: map[string]map[string]string{}}
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)
	s := newOpenBaoStoreForTest(t, srv.URL)
	ctx := context.Background()

	if got, err := s.Get(ctx, "services/web/env"); err != nil || len(got) != 0 {
		t.Fatalf("empty GET: %+v err=%v", got, err)
	}
	if stub.logins != 1 {
		t.Fatalf("first call logs in once, did %d", stub.logins)
	}
	if err := s.Put(ctx, "services/web/env", map[string]string{"A": "1", "B": "2"}); err != nil {
		t.Fatalf("PutEnv: %v", err)
	}
	got, err := s.Get(ctx, "services/web/env")
	if err != nil || got["A"] != "1" || got["B"] != "2" {
		t.Fatalf("round-trip: %+v err=%v", got, err)
	}
	if stub.logins != 1 {
		t.Fatalf("token should be cached, logins=%d", stub.logins)
	}
	if _, ok := stub.data["default/services/web/env"]; !ok {
		t.Fatalf("wrong KV path, have %v", stub.data)
	}
	if err := s.Delete(ctx, "services/web/env"); err != nil {
		t.Fatalf("DeleteEnv: %v", err)
	}
	if got, _ := s.Get(ctx, "services/web/env"); len(got) != 0 {
		t.Fatalf("after delete, GET empty: %+v", got)
	}
}

func TestOpenBaoStore_ReloginOn403(t *testing.T) {
	stub := &baoStub{t: t, data: map[string]map[string]string{}, rejectN: 1}
	srv := httptest.NewServer(stub.handler())
	t.Cleanup(srv.Close)
	s := newOpenBaoStoreForTest(t, srv.URL)
	if err := s.Put(context.Background(), "services/web/env", map[string]string{"A": "1"}); err != nil {
		t.Fatalf("PutEnv should recover from a 403 via re-login: %v", err)
	}
	if stub.logins != 2 {
		t.Fatalf("expected a re-login after 403, logins=%d", stub.logins)
	}
	if stub.data["default/services/web/env"]["A"] != "1" {
		t.Fatalf("retry did not persist: %+v", stub.data)
	}
}

func TestOpenBaoStore_LoginFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad role", http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)
	s := newOpenBaoStoreForTest(t, srv.URL)
	if _, err := s.Get(context.Background(), "services/web/env"); err == nil {
		t.Fatal("login failure should surface as an error")
	}
}

// TestSecretMapAggregateQuotas (round-11 #6): per-key writes cannot grow a
// service's env map or file map without bound — the count and aggregate-byte
// quotas reject before any OpenBao (or Kubernetes projection) write, and a
// rejected write leaves the store unchanged.
func TestSecretMapAggregateQuotas(t *testing.T) {
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("web"))
	ctx := context.Background()

	// Fill to exactly the env key cap.
	for i := 0; i < maxEnvKeys; i++ {
		key := "V" + strconv.Itoa(i)
		if _, err := svc.SetEnvVar(ctx, "web", key, EnvVarWrite{Value: "x"}); err != nil {
			t.Fatalf("SetEnvVar %d within cap: %v", i, err)
		}
	}
	if _, err := svc.SetEnvVar(ctx, "web", "OVERFLOW", EnvVarWrite{Value: "x"}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("key %d beyond the cap = %v, want ErrBadRequest", maxEnvKeys+1, err)
	}
	if _, ok := store.m[envPath("web")]["OVERFLOW"]; ok {
		t.Fatal("rejected key must not reach the store")
	}
	// Deleting one key frees room again.
	if err := svc.DeleteEnvVar(ctx, "web", "V0"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetEnvVar(ctx, "web", "AFTER_DELETE", EnvVarWrite{Value: "x"}); err != nil {
		t.Fatalf("write after freeing a slot: %v", err)
	}

	// The aggregate-byte cap rejects a single oversized value outright.
	big := strings.Repeat("y", maxSecretMapBytes+1)
	if _, err := svc.SetEnvVar(ctx, "web", "BIG", EnvVarWrite{Value: big}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("oversized value = %v, want ErrBadRequest", err)
	}
	// A batch replacement larger than the quota is rejected before the store write.
	bigBatch := []EnvVarView{{Key: "B1", Value: big}, {Key: "B2", Value: big}}
	if _, err := svc.SetEnvVars(ctx, "web", bigBatch); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("oversized batch = %v, want ErrBadRequest", err)
	}
	if _, ok := store.m[envPath("web")]["B1"]; ok {
		t.Fatal("rejected batch must not reach the store")
	}

	// Secret files carry the same two caps.
	for i := 0; i < maxSecretFiles; i++ {
		name := "f" + strconv.Itoa(i)
		if _, err := svc.SetSecretFile(ctx, "web", name, "x"); err != nil {
			t.Fatalf("SetSecretFile %d within cap: %v", i, err)
		}
	}
	if _, err := svc.SetSecretFile(ctx, "web", "overflow.txt", "x"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("file %d beyond the cap = %v, want ErrBadRequest", maxSecretFiles+1, err)
	}
	if _, err := svc.SetSecretFile(ctx, "web", "f0", big); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("oversized file = %v, want ErrBadRequest", err)
	}
	if _, ok := store.m[filesPath("web")]["overflow.txt"]; ok {
		t.Fatal("rejected file must not reach the store")
	}
}
