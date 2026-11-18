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

// public_url_test.go is w4/124 — the fourth member of the disk (w1/m86) /
// ipAllowList (w6/m106) / renderSubdomainPolicy (w6/m130) family: a field
// emitted for a service that cannot have it.
//
// `url` means the PUBLIC url. The operator fills status.URL with the
// cluster-internal address whenever an App has no public host, so GraphQL —
// which reads AppView rather than renderServiceDetails' emission gate — handed
// back `http://<slug>:<port>` there, the same value the payload already carried
// correctly in internalAddress. REST omitted it for private_service only, which
// left the exposed-but-unrouted web service leaking on every surface.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// internalStatusApp is a converged App whose status.URL is the internal
// address — what the operator writes when there is no public host.
func internalStatusApp(name, svcType string) *appv1alpha1.App {
	a := typedApp(name, svcType)
	a.Spec.Port = 8080
	a.Status.Phase = appv1alpha1.PhaseRunning
	a.Status.URL = "http://" + name + ":8080"
	a.Status.URLs = nil
	return a
}

func TestM124_InternalAddressIsNeverReportedAsThePublicURL(t *testing.T) {
	for _, tc := range []struct {
		name    string
		svcType string
		// addressable says whether the type has an internal address at all —
		// a static site is served by the platform, not by a sibling-reachable
		// Service, so it has none (AppSpec.InternallyAddressable).
		addressable bool
	}{
		// The reported case: a private service, whose address is the one thing
		// status.URL can ever be.
		{name: "private_service", svcType: appv1alpha1.TypePrivateService, addressable: true},
		// The case the type gate alone would miss: an ingress type whose
		// platform subdomain is off with no custom domain, so the operator
		// falls back to the same internal URL. REST leaked this one too.
		{name: "web_service with no public host", svcType: appv1alpha1.TypeWebService, addressable: true},
		{name: "static_site with no public host", svcType: appv1alpha1.TypeStaticSite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := internalStatusApp("svc", tc.svcType)
			// No platform host to fall back to, or pendingPublicURL would
			// legitimately supply the deterministic https intent instead.
			a.Spec.SubdomainPolicy = appv1alpha1.SubdomainPolicyDisabled
			svc, _ := newService(nil, a)
			ctx := context.Background()

			view, err := svc.Get(ctx, "svc")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if view.URL != "" {
				t.Errorf("AppView.URL = %q, want empty — that is the internal address", view.URL)
			}
			// The address still travels, in the field that says what it is.
			if tc.addressable && view.InternalAddress == "" {
				t.Error("internalAddress must still carry the address")
			}

			// GraphQL reads AppView directly — the surface the bug was on.
			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			})
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: `{ service(id: "svc") { url internalAddress } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("gql: %v", res.Errors)
			}
			gqlSvc := res.Data.(map[string]any)["service"].(map[string]any)
			if got := gqlSvc["url"]; got != nil && got != "" {
				t.Errorf("GraphQL url = %v, want null/empty (GraphQL's url is the public URL only)", got)
			}
			if got := gqlSvc["internalAddress"]; tc.addressable && (got == nil || got == "") {
				t.Errorf("GraphQL internalAddress = %v, want the address", got)
			}

			// REST
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			validated(t, mux).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/svc", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("REST GET: %d %s", rec.Code, rec.Body)
			}
			if strings.Contains(rec.Body.String(), `"url":"http://`) {
				t.Errorf("REST leaked an http:// url: %s", rec.Body)
			}
			var restBody struct {
				ServiceDetails map[string]any `json:"serviceDetails"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &restBody); err != nil {
				t.Fatalf("decode REST body: %v", err)
			}
			// Render's private service url is its scheme-less internal address;
			// every other type with no public host has no url (nil = absent).
			var wantURL any
			if tc.svcType == appv1alpha1.TypePrivateService {
				wantURL = "svc:8080"
			}
			if got := restBody.ServiceDetails["url"]; got != wantURL {
				t.Errorf("REST serviceDetails.url = %v, want %v", got, wantURL)
			}

			// MCP shares REST's rendering.
			if got := toRenderService(view).ServiceDetails["url"]; got != wantURL {
				t.Errorf("MCP serviceDetails.url = %v, want %v", got, wantURL)
			}
		})
	}
}

// TestM124_APublicServiceStillReportsItsURL keeps the guard above from being
// satisfied by never reporting a url at all.
func TestM124_APublicServiceStillReportsItsURL(t *testing.T) {
	a := typedApp("web", appv1alpha1.TypeWebService)
	a.Status.Phase = appv1alpha1.PhaseRunning
	a.Status.URL = "https://web.onbex.co"
	a.Status.URLs = []string{"https://web.onbex.co"}
	svc, _ := newService(nil, a)

	view, err := svc.Get(context.Background(), "web")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if view.URL != "https://web.onbex.co" {
		t.Fatalf("AppView.URL = %q, want the public URL", view.URL)
	}
}

// TestM125_MaintenanceModeIsNullOffAWebService is w4/125, the sibling this
// sweep was meant to find. REST and MCP omit maintenanceMode for every type but
// web_service; GraphQL answered {enabled:false, uri:""} for all of them, which
// is not "unset" — it is a claim about a mode the service does not have.
//
// The w6/m130 remedy could not reach it: emptying the VALUE in view() still
// yields the zero object. Only the schema can say "not applicable", so the
// field is nullable now, which is why the web-service control below matters.
func TestM125_MaintenanceModeIsNullOffAWebService(t *testing.T) {
	for _, tc := range []struct {
		name     string
		svcType  string
		wantNull bool
	}{
		{name: "web_service", svcType: appv1alpha1.TypeWebService},
		{name: "private_service", svcType: appv1alpha1.TypePrivateService, wantNull: true},
		{name: "background_worker", svcType: appv1alpha1.TypeBackgroundWorker, wantNull: true},
		{name: "cron_job", svcType: appv1alpha1.TypeCronJob, wantNull: true},
		{name: "static_site", svcType: appv1alpha1.TypeStaticSite, wantNull: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newService(nil, typedApp("svc", tc.svcType))
			ctx := context.Background()

			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			})
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: `{ service(id: "svc") { maintenanceMode { enabled uri } } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("gql: %v", res.Errors)
			}
			got := res.Data.(map[string]any)["service"].(map[string]any)["maintenanceMode"]
			if tc.wantNull {
				if got != nil {
					t.Errorf("GraphQL maintenanceMode = %v, want null (agrees with REST's omission)", got)
				}
			} else if got == nil {
				t.Error("GraphQL maintenanceMode = null for a web_service, want the object")
			}

			// REST and MCP are the reference: present for web_service only.
			view, err := svc.Get(ctx, "svc")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			_, present := toRenderService(view).ServiceDetails["maintenanceMode"]
			if present == tc.wantNull {
				t.Errorf("REST/MCP serviceDetails.maintenanceMode present = %v, want %v", present, !tc.wantNull)
			}
		})
	}
}

// TestM131_LatestDeployIDIsNullOnReads is w4/131, the third member of this same
// family. REST puts the id in Render's serviceAndDeploy CREATE envelope
// (w2/m47), so GET /v1/services/{id} has no deploy key at all — the service
// object never claims to know its latest deploy. GraphQL hung it on Service,
// where only Create ever assigns it, so every ordinary read answered "" — live,
// seven services for seven, including five long-lived production services with
// real deploy history and the two whose create call had just returned the id.
//
// "" is not "unset": it reads as "there is no deploy". Null is.
func TestM131_LatestDeployIDIsNullOnReads(t *testing.T) {
	svc, _ := newService(nil, typedApp("svc", appv1alpha1.TypeWebService))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	for _, q := range []struct{ name, query, root string }{
		{"service", `{ service(id: "svc") { latestDeployId } }`, "service"},
		{"services", `{ services { latestDeployId } }`, "services"},
	} {
		t.Run(q.name, func(t *testing.T) {
			res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: q.query})
			if len(res.Errors) > 0 {
				t.Fatalf("gql: %v", res.Errors)
			}
			data := res.Data.(map[string]any)[q.root]
			entry, ok := data.(map[string]any)
			if !ok {
				list, isList := data.([]any)
				if !isList || len(list) == 0 {
					t.Fatalf("%s returned %v", q.root, data)
				}
				entry = list[0].(map[string]any)
			}
			if got := entry["latestDeployId"]; got != nil {
				t.Errorf("latestDeployId = %#v on a read, want null — \"\" claims the service has no deploy", got)
			}
		})
	}

	// The control that keeps the fix honest: the create path, the one place the
	// id is knowable, still answers it. Only Create assigns LatestDeployID, so
	// this asserts the resolver passes a populated value straight through.
	view := AppView{LatestDeployID: "dep-1"}
	if got := latestDeployIDOf(view); got != "dep-1" {
		t.Errorf("populated latestDeployId = %#v, want dep-1", got)
	}
}

// w9/m473: a private service's serviceDetails.url is its scheme-less
// <slug>:<port> on every operation that answers the service, as every
// private-service response in Render's Terraform provider recordings carries
// it. rendertest.Allowlist still excuses a missing url on these operations for
// a web service with no public host, so the schema check alone would not catch
// a private service losing it.
func TestPrivateServiceURLIsItsInternalAddressOnEveryOperation(t *testing.T) {
	svc, cl := newService(nil)
	svc.BaseDomain = "onbex.co"
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	serve := func(method, path, body string, want int) []byte {
		t.Helper()
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		if rec.Code != want {
			t.Fatalf("%s %s = %d, want %d: %s", method, path, rec.Code, want, rec.Body)
		}
		return rec.Body.Bytes()
	}
	type read struct {
		ID             string         `json:"id"`
		Slug           string         `json:"slug"`
		ServiceDetails map[string]any `json:"serviceDetails"`
	}
	check := func(op string, got read) {
		t.Helper()
		want := got.Slug + ":8080"
		if got.Slug == "" || got.ServiceDetails["url"] != want || got.ServiceDetails["internalAddress"] != want {
			t.Errorf("%s: url = %v, internalAddress = %v, want both %q", op, got.ServiceDetails["url"], got.ServiceDetails["internalAddress"], want)
		}
	}

	var created struct {
		Service read `json:"service"`
	}
	if err := json.Unmarshal(serve(http.MethodPost, "/v1/services",
		`{"type":"private_service","name":"pserv","port":8080,"image":{"imagePath":"nginx:1"}}`, http.StatusCreated), &created); err != nil {
		t.Fatal(err)
	}
	// Before the operator reads its image, the configured port is already
	// the primary that read keeps (w9/104).
	check("create", created.Service)
	id := created.Service.ID

	// The operator's image read publishes the configured port.
	app := getApp(t, cl, "pserv")
	app.Status.ServingNetwork = &appv1alpha1.ImageNetworkStatus{Image: "nginx:1", Revision: "r1", PrimaryPort: 8080, Ports: []int32{8080}}
	if err := cl.Update(context.Background(), app); err != nil {
		t.Fatal(err)
	}

	var got read
	if err := json.Unmarshal(serve(http.MethodGet, "/v1/services/"+id, "", http.StatusOK), &got); err != nil {
		t.Fatal(err)
	}
	check("retrieve", got)

	var listed []struct {
		Service read `json:"service"`
	}
	if err := json.Unmarshal(serve(http.MethodGet, "/v1/services", "", http.StatusOK), &listed); err != nil || len(listed) != 1 {
		t.Fatalf("list: %v (%d rows)", err, len(listed))
	}
	check("list", listed[0].Service)

	var updated read
	if err := json.Unmarshal(serve(http.MethodPatch, "/v1/services/"+id, `{"name":"pserv-renamed"}`, http.StatusOK), &updated); err != nil {
		t.Fatal(err)
	}
	check("update", updated)
}
