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

// subdomain_policy_test.go covers Render's renderSubdomainPolicy at its
// documented nesting (w6/m130) — the third field in the disk (w1/m86) /
// ipAllowList (w6/m106) family:
//
//   - request side: create + PATCH accept serviceDetails.renderSubdomainPolicy
//     (Render declares it there, never on the top-level service), with the
//     top-level form still honored (top level wins on create).
//   - response side: it is emitted ONLY inside serviceDetails and ONLY for the
//     two ingress types (web_service, static_site); private/worker/cron report
//     nothing (they have no platform subdomain), and REST/GraphQL/MCP agree.
//   - the setSubdomainPolicy verb refuses a non-routable type with a
//     type-specific reason that does not send the caller after a custom domain.
//
// Mirrors the depth of ip_allow_list_test.go, the sibling milestone.

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
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// typedApp is sampleApp with an explicit service type — the non-routable types
// need one to exercise the gate.
func typedApp(name, svcType string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Type = svcType
	return a
}

var nonRoutableTypes = []string{
	appv1alpha1.TypePrivateService,
	appv1alpha1.TypeBackgroundWorker,
	appv1alpha1.TypeCronJob,
}

// --- request side: create ---

// TestRESTCreateAcceptsNestedRenderSubdomainPolicy is the core t001 fix: the
// field decodes at Render's documented location instead of the 400
// `unknown field "renderSubdomainPolicy"` bex used to return there.
func TestRESTCreateAcceptsNestedRenderSubdomainPolicy(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// enabled needs no custom domain.
	body := `{"type":"web_service","name":"web","image":{"imagePath":"nginx:v1"},"serviceDetails":{"renderSubdomainPolicy":"enabled"}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create nested enabled => 201, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.SubdomainPolicy; got != appv1alpha1.SubdomainPolicyEnabled {
		t.Fatalf("spec.subdomainPolicy = %q, want enabled", got)
	}

	// disabled decodes too, and reaches the same custom-domain guard the
	// top-level form always did (here satisfied by a domain in the body).
	body = `{"type":"web_service","name":"web2","image":{"imagePath":"nginx:v1"},"domains":["ex.example.com"],"serviceDetails":{"renderSubdomainPolicy":"disabled"}}`
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create nested disabled+domain => 201, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web2").Spec.SubdomainPolicy; got != appv1alpha1.SubdomainPolicyDisabled {
		t.Fatalf("spec.subdomainPolicy = %q, want disabled", got)
	}
}

// TestRESTCreateNestedRenderSubdomainPolicyNoLongerUnknownField reproduces the
// milestone's exact probe: a deliberately incomplete body (name:"") so that a
// surviving unknown-field rejection — which happens during decoding, before
// validation — is distinguishable from the later name validation it should now
// reach, exactly as the ipAllowList control already does.
func TestRESTCreateNestedRenderSubdomainPolicyNoLongerUnknownField(t *testing.T) {
	svc, _ := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"type":"web_service","name":"","serviceDetails":{"env":"go","renderSubdomainPolicy":"disabled"}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("incomplete body => 400, got %d: %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "unknown field") {
		t.Fatalf("nested renderSubdomainPolicy still rejected as an unknown field: %s", rec.Body)
	}
	// It decoded and proceeded to name validation, like the ipAllowList control.
	if !strings.Contains(rec.Body.String(), "name") {
		t.Fatalf("want a name-validation 400 after decode, got: %s", rec.Body)
	}
}

// TestRESTCreateTopLevelRenderSubdomainPolicyWins asserts the documented
// precedence: with both spellings present, the top-level value is used. Sending
// nested "disabled" with no custom domain would 400 at the guard if it won, so
// a clean 201+enabled proves top level took effect.
func TestRESTCreateTopLevelRenderSubdomainPolicyWins(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"type":"web_service","name":"web","image":{"imagePath":"nginx:v1"},"renderSubdomainPolicy":"enabled","serviceDetails":{"renderSubdomainPolicy":"disabled"}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("top-level enabled must win (no domain needed) => 201, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.SubdomainPolicy; got != appv1alpha1.SubdomainPolicyEnabled {
		t.Fatalf("spec.subdomainPolicy = %q, want enabled (top level wins)", got)
	}
}

// --- request side: PATCH ---

func TestRESTPatchAcceptsNestedRenderSubdomainPolicy(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Hosts = []string{"ex.example.com"} // lets disabled validate
	svc, cl := newService(nil, app)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := strings.NewReader(`{"serviceDetails":{"renderSubdomainPolicy":"disabled"}}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH nested => 200, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.SubdomainPolicy; got != appv1alpha1.SubdomainPolicyDisabled {
		t.Fatalf("spec.subdomainPolicy = %q, want disabled", got)
	}
}

// TestRESTPatchNestedRenderSubdomainPolicyWins mirrors healthCheckPath/
// preDeployCommand/schedule: on PATCH the nested spelling wins when a body
// carries both. Nested "disabled" (with a host present) over top-level
// "enabled" must land disabled.
func TestRESTPatchNestedRenderSubdomainPolicyWins(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Hosts = []string{"ex.example.com"}
	svc, cl := newService(nil, app)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := strings.NewReader(`{"renderSubdomainPolicy":"enabled","serviceDetails":{"renderSubdomainPolicy":"disabled"}}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH => 200, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.SubdomainPolicy; got != appv1alpha1.SubdomainPolicyDisabled {
		t.Fatalf("spec.subdomainPolicy = %q, want disabled (nested wins on PATCH)", got)
	}
}

// --- response side: emission nesting + type gate ---

// TestRenderServiceDetailsGatesRenderSubdomainPolicyToIngressTypes is the
// belt-and-suspenders render gate (mirrors the ipAllowList sibling): the field
// only ever appears in serviceDetails for the two types Render declares it on;
// private/worker/cron omit it entirely even if a view carries a value.
func TestRenderServiceDetailsGatesRenderSubdomainPolicyToIngressTypes(t *testing.T) {
	view := AppView{RenderSubdomainPolicy: appv1alpha1.SubdomainPolicyEnabled}

	for _, svcType := range nonRoutableTypes {
		if _, present := renderServiceDetails(view, svcType, "")["renderSubdomainPolicy"]; present {
			t.Errorf("%s serviceDetails carries renderSubdomainPolicy, want it omitted (no such Render property)", svcType)
		}
	}
	for _, svcType := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypeStaticSite} {
		if got := renderServiceDetails(view, svcType, "")["renderSubdomainPolicy"]; got != appv1alpha1.SubdomainPolicyEnabled {
			t.Errorf("%s serviceDetails.renderSubdomainPolicy = %v, want enabled", svcType, got)
		}
	}
	// An empty policy on an ingress type omits the key, like every other
	// optional serviceDetails field.
	if _, present := renderServiceDetails(AppView{}, appv1alpha1.TypeWebService, "")["renderSubdomainPolicy"]; present {
		t.Error("empty policy should omit serviceDetails.renderSubdomainPolicy entirely")
	}
}

// TestViewEmptiesRenderSubdomainPolicyForNonIngressTypes proves the single seam
// (view()): a type with no platform subdomain reads back "", so the same
// AppView drives REST omission and GraphQL null. Today this field reported
// "enabled" for a worker while the same payload had no url and the host 404'd.
func TestViewEmptiesRenderSubdomainPolicyForNonIngressTypes(t *testing.T) {
	for _, svcType := range nonRoutableTypes {
		app := typedApp("svc", svcType)
		app.Spec.SubdomainPolicy = appv1alpha1.SubdomainPolicyEnabled // stored but not applicable
		svc, _ := newService(nil, app)
		v, err := svc.Get(context.Background(), "svc")
		if err != nil {
			t.Fatalf("Get(%s): %v", svcType, err)
		}
		if v.RenderSubdomainPolicy != "" {
			t.Errorf("%s AppView.RenderSubdomainPolicy = %q, want \"\" (no platform subdomain)", svcType, v.RenderSubdomainPolicy)
		}
	}

	// A web service keeps its real, applicable policy.
	svc, _ := newService(nil, sampleApp("web"))
	v, err := svc.Get(context.Background(), "web")
	if err != nil {
		t.Fatalf("Get(web): %v", err)
	}
	if v.RenderSubdomainPolicy != appv1alpha1.SubdomainPolicyEnabled {
		t.Errorf("web AppView.RenderSubdomainPolicy = %q, want enabled", v.RenderSubdomainPolicy)
	}
}

// TestRESTResponseRenderSubdomainPolicyNestedNotTopLevel is the read-side
// contract: a Render-parity client finds the field at
// serviceDetails.renderSubdomainPolicy, never at the JSON root, and never at
// all for a non-routable type.
func TestRESTResponseRenderSubdomainPolicyNestedNotTopLevel(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"), typedApp("worker", appv1alpha1.TypeBackgroundWorker))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	get := func(name string) struct {
		Root           json.RawMessage `json:"renderSubdomainPolicy"`
		ServiceDetails struct {
			Policy *string `json:"renderSubdomainPolicy"`
		} `json:"serviceDetails"`
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/"+name, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s => 200, got %d: %s", name, rec.Code, rec.Body)
		}
		var out struct {
			Root           json.RawMessage `json:"renderSubdomainPolicy"`
			ServiceDetails struct {
				Policy *string `json:"renderSubdomainPolicy"`
			} `json:"serviceDetails"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("unmarshal %s: %v", name, err)
		}
		return out
	}

	web := get("web")
	if web.Root != nil {
		t.Errorf("web: renderSubdomainPolicy present at JSON root (%s), want only under serviceDetails", web.Root)
	}
	if web.ServiceDetails.Policy == nil || *web.ServiceDetails.Policy != appv1alpha1.SubdomainPolicyEnabled {
		t.Errorf("web: serviceDetails.renderSubdomainPolicy = %v, want enabled", web.ServiceDetails.Policy)
	}

	worker := get("worker")
	if worker.Root != nil {
		t.Errorf("worker: renderSubdomainPolicy present at JSON root (%s), want absent", worker.Root)
	}
	if worker.ServiceDetails.Policy != nil {
		t.Errorf("worker: serviceDetails.renderSubdomainPolicy = %q, want absent (no platform subdomain)", *worker.ServiceDetails.Policy)
	}
}

// TestRenderSubdomainPolicyAgreesAcrossSurfaces proves REST, GraphQL and MCP
// tell one story per type: enabled+nested for a web service, absent/null for a
// worker. Mirrors TestIPAllowListPresentOnAllThreeSurfaces.
func TestRenderSubdomainPolicyAgreesAcrossSurfaces(t *testing.T) {
	for _, tc := range []struct {
		name    string
		svcType string
		want    string // "" => absent on REST/MCP, null on GraphQL
	}{
		{name: "web_service", svcType: appv1alpha1.TypeWebService, want: appv1alpha1.SubdomainPolicyEnabled},
		{name: "static_site", svcType: appv1alpha1.TypeStaticSite, want: appv1alpha1.SubdomainPolicyEnabled},
		{name: "private_service", svcType: appv1alpha1.TypePrivateService, want: ""},
		{name: "background_worker", svcType: appv1alpha1.TypeBackgroundWorker, want: ""},
		{name: "cron_job", svcType: appv1alpha1.TypeCronJob, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newService(nil, typedApp("svc", tc.svcType))
			ctx := context.Background()

			// REST
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/svc", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("REST GET: %d %s", rec.Code, rec.Body)
			}
			var restBody struct {
				Root           json.RawMessage `json:"renderSubdomainPolicy"`
				ServiceDetails struct {
					Policy *string `json:"renderSubdomainPolicy"`
				} `json:"serviceDetails"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &restBody); err != nil {
				t.Fatalf("decode REST body: %v", err)
			}
			if restBody.Root != nil {
				t.Errorf("REST: renderSubdomainPolicy at JSON root (%s), want never top-level", restBody.Root)
			}
			if tc.want == "" {
				if restBody.ServiceDetails.Policy != nil {
					t.Errorf("REST serviceDetails.renderSubdomainPolicy = %q, want absent", *restBody.ServiceDetails.Policy)
				}
			} else if restBody.ServiceDetails.Policy == nil || *restBody.ServiceDetails.Policy != tc.want {
				t.Errorf("REST serviceDetails.renderSubdomainPolicy = %v, want %q", restBody.ServiceDetails.Policy, tc.want)
			}

			// GraphQL
			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			})
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: `{ service(id: "svc") { renderSubdomainPolicy } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("gql: %v", res.Errors)
			}
			gql := res.Data.(map[string]any)["service"].(map[string]any)["renderSubdomainPolicy"]
			if tc.want == "" {
				if gql != nil {
					t.Errorf("GraphQL renderSubdomainPolicy = %v, want null (agrees with REST omission)", gql)
				}
			} else if gql != tc.want {
				t.Errorf("GraphQL renderSubdomainPolicy = %v, want %q", gql, tc.want)
			}

			// MCP shares REST's rendering (toRenderService).
			v, err := svc.Get(ctx, "svc")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			rendered := toRenderService(v)
			mcpPolicy, present := rendered.ServiceDetails["renderSubdomainPolicy"]
			if tc.want == "" {
				if present {
					t.Errorf("MCP serviceDetails.renderSubdomainPolicy = %v, want absent", mcpPolicy)
				}
			} else if mcpPolicy != tc.want {
				t.Errorf("MCP serviceDetails.renderSubdomainPolicy = %v, want %q", mcpPolicy, tc.want)
			}
		})
	}
}

// --- setSubdomainPolicy verb: type gate + retained routable behavior ---

// TestSetSubdomainPolicyRefusesNonRoutableTypes is t003: a private service,
// worker or cron job is refused with a reason it can act on — it names the type
// and does NOT tell the caller to add a custom domain (which would then 400
// because the type has no ingress). Both enabled and disabled are refused.
func TestSetSubdomainPolicyRefusesNonRoutableTypes(t *testing.T) {
	for _, svcType := range nonRoutableTypes {
		app := typedApp("svc", svcType)
		app.Spec.Hosts = []string{"ex.example.com"} // present, to prove the type gate fires first
		svc, _ := newService(nil, app)
		for _, policy := range []string{appv1alpha1.SubdomainPolicyEnabled, appv1alpha1.SubdomainPolicyDisabled} {
			_, err := svc.SetSubdomainPolicy(context.Background(), "svc", policy)
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("SetSubdomainPolicy(%s, %s) => ErrBadRequest, got %v", svcType, policy, err)
			}
			msg := err.Error()
			if !strings.Contains(msg, "web services and static sites") {
				t.Errorf("SetSubdomainPolicy(%s): message %q should name the applicable types", svcType, msg)
			}
			if strings.Contains(msg, "custom domain") {
				t.Errorf("SetSubdomainPolicy(%s): message %q must not send the caller after a custom domain", svcType, msg)
			}
			if !strings.Contains(msg, svcType) {
				t.Errorf("SetSubdomainPolicy(%s): message %q should name the actual type", svcType, msg)
			}
		}
	}
}

// TestSetSubdomainPolicyRoutableTypesStillWork is the t004 regression: the two
// types that behave correctly today keep accepting the field and keep gating
// disabled behind having at least one custom host.
func TestSetSubdomainPolicyRoutableTypesStillWork(t *testing.T) {
	for _, svcType := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypeStaticSite} {
		// enabled always succeeds.
		svc, cl := newService(nil, typedApp("svc", svcType))
		if _, err := svc.SetSubdomainPolicy(context.Background(), "svc", appv1alpha1.SubdomainPolicyEnabled); err != nil {
			t.Fatalf("SetSubdomainPolicy(%s, enabled): %v", svcType, err)
		}

		// disabled without a custom host keeps its existing guard.
		_, err := svc.SetSubdomainPolicy(context.Background(), "svc", appv1alpha1.SubdomainPolicyDisabled)
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "custom domain") {
			t.Fatalf("SetSubdomainPolicy(%s, disabled) with no host => custom-domain 400, got %v", svcType, err)
		}

		// disabled WITH a custom host is permitted and lands in the spec.
		withHost := typedApp("host", svcType)
		withHost.Spec.Hosts = []string{"ex.example.com"}
		svc, cl = newService(nil, withHost)
		if _, err := svc.SetSubdomainPolicy(context.Background(), "host", appv1alpha1.SubdomainPolicyDisabled); err != nil {
			t.Fatalf("SetSubdomainPolicy(%s, disabled) with host: %v", svcType, err)
		}
		if got := getApp(t, cl, "host").Spec.SubdomainPolicy; got != appv1alpha1.SubdomainPolicyDisabled {
			t.Errorf("%s spec.subdomainPolicy = %q, want disabled", svcType, got)
		}
	}
}
