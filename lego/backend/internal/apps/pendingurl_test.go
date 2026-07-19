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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// pendingurl_test.go covers the derived platform URL (w5/m48/t003): Render
// shows a service's URL from the moment it's created, so when status.url is
// still empty (no successful deploy yet) the API projects the deterministic
// intent — mirroring the operator's effectiveHosts ordering — identically
// across REST, GraphQL, and MCP. The CR's status stays the operator's observed
// truth; only the API view derives.

// pendingApp is a never-successfully-deployed exposed App (no status.url).
func pendingApp(name, svcType string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: appv1alpha1.AppSpec{
			Type:   svcType,
			Repo:   "https://github.com/x/mono",
			Branch: "main",
			Expose: svcType == appv1alpha1.TypeWebService || svcType == appv1alpha1.TypeStaticSite,
		},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseBuilding},
	}
}

func TestPendingURLDerivedForStaticSiteBeforeFirstDeploy(t *testing.T) {
	svc, _ := newBaseDomainService("onbex.co", "", pendingApp("site", appv1alpha1.TypeStaticSite))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/site", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		ServiceDetails struct {
			URL string `json:"url"`
		} `json:"serviceDetails"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ServiceDetails.URL != "https://site.onbex.co" {
		t.Errorf("serviceDetails.url = %q, want https://site.onbex.co (derived before first deploy)", out.ServiceDetails.URL)
	}
}

func TestPendingURLDerivedOnGraphQLAndMCPProjections(t *testing.T) {
	svc, _ := newBaseDomainService("onbex.co", "", pendingApp("site", appv1alpha1.TypeStaticSite))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ server(id: "site") { url } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["server"].(map[string]any)
	if got["url"] != "https://site.onbex.co" {
		t.Errorf("GraphQL url = %v, want https://site.onbex.co", got["url"])
	}

	// The MCP surface shares the same view + renderService projection.
	v, err := svc.Get(context.Background(), "site")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if out := toRenderService(v); out.ServiceDetails["url"] != "https://site.onbex.co" {
		t.Errorf("MCP renderService url = %v, want https://site.onbex.co", out.ServiceDetails["url"])
	}
}

func TestPendingURLRespectsTypeHostPolicyAndStatus(t *testing.T) {
	cases := []struct {
		name string
		app  *appv1alpha1.App
		base string
		want string
	}{
		{
			name: "web service derives its platform URL too",
			app:  pendingApp("web", appv1alpha1.TypeWebService),
			base: "onbex.co",
			want: "https://web.onbex.co",
		},
		{
			name: "spec.host wins over the platform subdomain (operator hosts[0])",
			app: func() *appv1alpha1.App {
				a := pendingApp("site", appv1alpha1.TypeStaticSite)
				a.Spec.Host = "www.example.com"
				return a
			}(),
			base: "onbex.co",
			want: "https://www.example.com",
		},
		{
			name: "disabled subdomain policy derives nothing",
			app: func() *appv1alpha1.App {
				a := pendingApp("site", appv1alpha1.TypeStaticSite)
				a.Spec.SubdomainPolicy = appv1alpha1.SubdomainPolicyDisabled
				return a
			}(),
			base: "onbex.co",
			want: "",
		},
		{
			name: "no base domain derives nothing",
			app:  pendingApp("site", appv1alpha1.TypeStaticSite),
			base: "",
			want: "",
		},
		{
			name: "a background worker never gets a URL",
			app:  pendingApp("job", appv1alpha1.TypeBackgroundWorker),
			base: "onbex.co",
			want: "",
		},
		{
			name: "a cron job never gets a URL",
			app: func() *appv1alpha1.App {
				a := pendingApp("tick", appv1alpha1.TypeCronJob)
				a.Spec.Schedule = "* * * * *"
				return a
			}(),
			base: "onbex.co",
			want: "",
		},
		{
			name: "status.url stays authoritative once set",
			app: func() *appv1alpha1.App {
				a := pendingApp("site", appv1alpha1.TypeStaticSite)
				a.Status.URL = "https://custom-slug.onbex.co"
				return a
			}(),
			base: "onbex.co",
			want: "https://custom-slug.onbex.co",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newBaseDomainService(tc.base, "", tc.app)
			v, err := svc.Get(context.Background(), tc.app.Name)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if v.URL != tc.want {
				t.Errorf("url = %q, want %q", v.URL, tc.want)
			}
		})
	}
}
