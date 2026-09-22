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
				t.Errorf("GraphQL url = %v, want null/empty (agrees with REST's omission)", got)
			}
			if got := gqlSvc["internalAddress"]; tc.addressable && (got == nil || got == "") {
				t.Errorf("GraphQL internalAddress = %v, want the address", got)
			}

			// REST
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/svc", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("REST GET: %d %s", rec.Code, rec.Body)
			}
			if strings.Contains(rec.Body.String(), `"url":"http://`) {
				t.Errorf("REST leaked an http:// url: %s", rec.Body)
			}
			var restBody struct {
				ServiceDetails map[string]json.RawMessage `json:"serviceDetails"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &restBody); err != nil {
				t.Fatalf("decode REST body: %v", err)
			}
			if raw, ok := restBody.ServiceDetails["url"]; ok {
				t.Errorf("REST serviceDetails.url = %s, want absent", raw)
			}

			// MCP shares REST's rendering.
			if rendered := toRenderService(view); rendered.ServiceDetails["url"] != nil {
				t.Errorf("MCP serviceDetails.url = %v, want absent", rendered.ServiceDetails["url"])
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
