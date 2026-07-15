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

package apps

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// notifyonfail_test.go covers spec.notifyOnFail (Render's per-service
// deploy-failure notification override, w4/m21, docs/render-artifacts/
// notify-on-fail.md) — enum validation, CRD round-trip (an App with no
// notifyOnFail set reads back "default"), and settable+readable identically
// across REST, GraphQL, and MCP — mirrors healthcheckpath_test.go depth. The
// notifier decision table itself (ignore/notify/default × member opt-in/out)
// lives in internal/notifications, the package that owns the fan-out.

// --- Enum validation ---

func TestNormalizeNotifyOnFail(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "", want: "default"},
		{in: "default", want: "default"},
		{in: "notify", want: "notify"},
		{in: "ignore", want: "ignore"},
		{in: "always", wantErr: true},
		{in: "Notify", wantErr: true}, // case-sensitive, matches Render's own enum
	}
	for _, c := range cases {
		got, err := normalizeNotifyOnFail(c.in)
		if c.wantErr {
			if !errors.Is(err, core.ErrBadRequest) {
				t.Errorf("normalizeNotifyOnFail(%q) = (%q, %v), want ErrBadRequest", c.in, got, err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("normalizeNotifyOnFail(%q) = (%q, %v), want (%q, nil)", c.in, got, err, c.want)
		}
	}
}

func TestSetNotifyOnFailRejectsUnknownValue(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))

	if _, err := svc.SetNotifyOnFail(context.Background(), "web", "sometimes"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("SetNotifyOnFail('sometimes') should be ErrBadRequest, got %v", err)
	}
}

// --- CRD round-trip / default ---

// TestViewDefaultsEmptyNotifyOnFailToDefault (CRD round-trip) is the DoD's
// "default" case: an App projected before this field existed (spec.notifyOnFail
// == "", the CRD's zero value) reads back the string "default", never a bare
// empty string a client would have to special-case.
func TestViewDefaultsEmptyNotifyOnFailToDefault(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web")) // sampleApp leaves NotifyOnFail unset

	v, err := svc.Get(context.Background(), "web")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v.NotifyOnFail != "default" {
		t.Errorf("AppView.NotifyOnFail = %q, want %q for an unset spec field", v.NotifyOnFail, "default")
	}
}

func TestSetNotifyOnFailRoundTripsThroughSpec(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))

	for _, value := range []string{"ignore", "notify", "default"} {
		v, err := svc.SetNotifyOnFail(context.Background(), "web", value)
		if err != nil {
			t.Fatalf("SetNotifyOnFail(%q): %v", value, err)
		}
		if v.NotifyOnFail != value {
			t.Errorf("view NotifyOnFail = %q, want %q", v.NotifyOnFail, value)
		}
		if got := getApp(t, cl, "web").Spec.NotifyOnFail; got != value {
			t.Errorf("spec.notifyOnFail = %q, want %q", got, value)
		}
	}
}

// --- REST ---

func TestRESTCreateThreadsNotifyOnFailAndReadsItBack(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","image":{"imagePath":"nginx:v1"},"notifyOnFail":"ignore"}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.NotifyOnFail; got != "ignore" {
		t.Fatalf("spec.notifyOnFail = %q, want ignore", got)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		NotifyOnFail string `json:"notifyOnFail"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.NotifyOnFail != "ignore" {
		t.Errorf("GET notifyOnFail = %q, want ignore", out.NotifyOnFail)
	}
}

func TestRESTCreateRejectsUnknownNotifyOnFail(t *testing.T) {
	svc, _ := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","image":{"imagePath":"nginx:v1"},"notifyOnFail":"loud"}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create with notifyOnFail='loud' => 400, got %d: %s", rec.Code, rec.Body)
	}
}

func TestRESTPatchServiceNotifyOnFail(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := strings.NewReader(`{"notifyOnFail":"notify"}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH notifyOnFail => 200, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.NotifyOnFail; got != "notify" {
		t.Errorf("spec.notifyOnFail = %q, want notify", got)
	}
	var out struct {
		NotifyOnFail string `json:"notifyOnFail"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.NotifyOnFail != "notify" {
		t.Errorf("response notifyOnFail = %q, want notify", out.NotifyOnFail)
	}
}

func TestRESTPatchServiceRejectsUnknownNotifyOnFail(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := strings.NewReader(`{"notifyOnFail":"loud"}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PATCH notifyOnFail='loud' => 400, got %d: %s", rec.Code, rec.Body)
	}
}

// --- GraphQL ---

func TestGraphQLCreateServiceThreadsNotifyOnFail(t *testing.T) {
	svc, cl := newService(nil)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { createService(name: "web", image: "nginx:v1", notifyOnFail: "ignore") { notifyOnFail } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("createService: %v", res.Errors)
	}
	if got := getApp(t, cl, "web").Spec.NotifyOnFail; got != "ignore" {
		t.Errorf("createService did not thread notifyOnFail onto the spec: got %q", got)
	}
	srv := res.Data.(map[string]any)["createService"].(map[string]any)
	if srv["notifyOnFail"] != "ignore" {
		t.Errorf("createService response notifyOnFail = %v, want ignore", srv["notifyOnFail"])
	}
}

func TestGraphQLSetNotifyOnFail(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setNotifyOnFail(id: "web", value: "notify") { notifyOnFail } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["setNotifyOnFail"].(map[string]any)
	if got["notifyOnFail"] != "notify" {
		t.Errorf("notifyOnFail = %v, want notify", got["notifyOnFail"])
	}
}

func TestGraphQLSetNotifyOnFailRejectsUnknownValue(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setNotifyOnFail(id: "web", value: "loud") { notifyOnFail } }`})
	if len(res.Errors) == 0 {
		t.Fatal("setNotifyOnFail with an unknown value should error")
	}
}

func TestGraphQLServerQueryReturnsNotifyOnFail(t *testing.T) {
	app := sampleApp("web")
	app.Spec.NotifyOnFail = "notify"
	svc, _ := newService(nil, app)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ server(id: "web") { notifyOnFail } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("server query: %v", res.Errors)
	}
	srv := res.Data.(map[string]any)["server"].(map[string]any)
	if srv["notifyOnFail"] != "notify" {
		t.Errorf("server.notifyOnFail = %v, want notify", srv["notifyOnFail"])
	}
}

// --- MCP ---

func TestMCPCreateWebServiceThreadsNotifyOnFail(t *testing.T) {
	req := createWebServiceArgs{Name: "w", Image: "nginx:v1", NotifyOnFail: "ignore"}.toCreateRequest()
	if req.NotifyOnFail != "ignore" {
		t.Errorf("create_web_service notifyOnFail not threaded: %+v", req)
	}
}

func TestMCPCreateCronJobThreadsNotifyOnFail(t *testing.T) {
	req := createCronJobArgs{Name: "c", Schedule: "0 0 * * *", Image: "nginx:v1", NotifyOnFail: "notify"}.toCreateRequest()
	if req.NotifyOnFail != "notify" {
		t.Errorf("create_cron_job notifyOnFail not threaded: %+v", req)
	}
}

func TestMCPSetNotifyOnFailUpdatesSpec(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))

	v, err := svc.SetNotifyOnFail(context.Background(), "web", "ignore")
	if err != nil {
		t.Fatalf("SetNotifyOnFail: %v", err)
	}
	out := toRenderService(v)
	if out.NotifyOnFail != "ignore" {
		t.Errorf("set_notify_on_fail notifyOnFail = %q, want ignore", out.NotifyOnFail)
	}
	if got := getApp(t, cl, "web").Spec.NotifyOnFail; got != "ignore" {
		t.Errorf("spec.notifyOnFail = %q, want ignore", got)
	}
}

// --- Adapter parity ---

// TestNotifyOnFailPresentOnAllThreeSurfaces proves the field round-trips
// identically over REST, GraphQL and MCP for every valid enum value —
// mirrors TestSlugPresentOnAllThreeSurfaces's shape (w4/m20/t002). REST's GET
// and MCP's get_service both delegate to the same toRenderService(AppView)
// the assertions below share, so this also proves MCP without standing up a
// full mcp.Server transport.
func TestNotifyOnFailPresentOnAllThreeSurfaces(t *testing.T) {
	for _, value := range []string{"default", "notify", "ignore"} {
		t.Run(value, func(t *testing.T) {
			app := sampleApp("web")
			app.Spec.NotifyOnFail = value
			svc, _ := newService(nil, app)
			ctx := context.Background()

			// REST
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("REST GET: %d %s", rec.Code, rec.Body)
			}
			var restBody map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &restBody); err != nil {
				t.Fatalf("decode REST body: %v", err)
			}
			if got := restBody["notifyOnFail"]; got != value {
				t.Errorf("REST notifyOnFail = %v, want %q", got, value)
			}

			// GraphQL
			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			})
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: `{ service(id: "web") { notifyOnFail } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("gql: %v", res.Errors)
			}
			if got := res.Data.(map[string]any)["service"].(map[string]any)["notifyOnFail"]; got != value {
				t.Errorf("GraphQL notifyOnFail = %v, want %q", got, value)
			}

			// MCP (get_service's handler, the same toRenderService REST's GET uses)
			v, err := svc.Get(ctx, "web")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got := toRenderService(v).NotifyOnFail; got != value {
				t.Errorf("MCP get_service notifyOnFail = %v, want %q", got, value)
			}
		})
	}
}
