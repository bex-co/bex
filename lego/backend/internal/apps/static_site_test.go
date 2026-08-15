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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func sampleStaticApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: appv1alpha1.AppSpec{
			Type:        appv1alpha1.TypeStaticSite,
			Repo:        "https://github.com/acme/site",
			PublishPath: "dist",
			Expose:      true,
			Routes:      []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/*", Destination: "/index.html"}},
			Headers:     []appv1alpha1.StaticHeader{{Path: "/*", Name: "X-Frame-Options", Value: "DENY"}},
		},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, URL: "https://" + name + ".onbex.co"},
	}
}

func TestCreateStaticSite(t *testing.T) {
	svc, cl := newService(nil)
	v, err := svc.Create(context.Background(), CreateRequest{
		Name:        "site",
		Type:        appv1alpha1.TypeStaticSite,
		Repo:        "https://github.com/acme/site",
		PublishPath: "dist",
		Routes:      []StaticRouteView{{Type: "redirect", Source: "/old", Destination: "/new"}},
		Headers:     []StaticHeaderView{{Path: "/*", Name: "X-Foo", Value: "bar"}},
	})
	if err != nil {
		t.Fatalf("Create static_site: %v", err)
	}
	if v.Type != appv1alpha1.TypeStaticSite || v.PublishPath != "dist" {
		t.Errorf("view = %+v, want static_site with publishPath dist", v)
	}
	if len(v.Routes) != 1 || v.Routes[0].Destination != "/new" {
		t.Errorf("routes = %+v", v.Routes)
	}
	a := getApp(t, cl, "site")
	if a.Spec.Type != appv1alpha1.TypeStaticSite || a.Spec.PublishPath != "dist" {
		t.Errorf("spec = %+v", a.Spec)
	}
	// A static site is exposed at the platform hostname like a web service.
	if !a.Spec.Expose {
		t.Errorf("static site should set spec.expose")
	}
	if len(a.Spec.Headers) != 1 || a.Spec.Headers[0].Name != "X-Foo" {
		t.Errorf("headers = %+v", a.Spec.Headers)
	}
}

func TestCreateStaticSiteRequiresPublishPath(t *testing.T) {
	svc, _ := newService(nil)
	_, err := svc.Create(context.Background(), CreateRequest{
		Name: "site", Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/acme/site",
	})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("missing publishPath => ErrBadRequest, got %v", err)
	}
}

func TestPublishPathRejectedForNonStatic(t *testing.T) {
	svc, _ := newService(nil)
	_, err := svc.Create(context.Background(), CreateRequest{
		Name: "web", Type: appv1alpha1.TypeWebService, Image: "nginx:1", PublishPath: "dist",
	})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("publishPath on web_service => ErrBadRequest, got %v", err)
	}
}

func TestCreateStaticSiteValidatesRoutes(t *testing.T) {
	svc, _ := newService(nil)
	_, err := svc.Create(context.Background(), CreateRequest{
		Name: "site", Type: appv1alpha1.TypeStaticSite, Repo: "r", PublishPath: "dist",
		Routes: []StaticRouteView{{Type: "bogus", Source: "/a", Destination: "/b"}},
	})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("bad route type => ErrBadRequest, got %v", err)
	}
}

func TestSetRoutesAndHeaders(t *testing.T) {
	svc, cl := newService(nil, sampleStaticApp("site"))
	ctx := context.Background()

	v, err := svc.SetRoutes(ctx, "site", []StaticRouteView{
		{Type: "redirect", Source: "/a", Destination: "/b"},
		{Type: "rewrite", Source: "/*", Destination: "/index.html"},
	})
	if err != nil {
		t.Fatalf("SetRoutes: %v", err)
	}
	if len(v.Routes) != 2 || v.Routes[0].Type != "redirect" {
		t.Errorf("routes = %+v", v.Routes)
	}
	if got := getApp(t, cl, "site").Spec.Routes; len(got) != 2 {
		t.Errorf("spec.routes len = %d, want 2", len(got))
	}

	if _, err := svc.SetHeaders(ctx, "site", []StaticHeaderView{{Path: "/*", Name: "X-A", Value: "1"}}); err != nil {
		t.Fatalf("SetHeaders: %v", err)
	}
	hs, err := svc.ListHeaders(ctx, "site")
	if err != nil || len(hs) != 1 || hs[0].Name != "X-A" {
		t.Fatalf("ListHeaders: %v %+v", err, hs)
	}
}

func TestSetRoutesRejectedForNonStatic(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	_, err := svc.SetRoutes(context.Background(), "web", []StaticRouteView{{Type: "redirect", Source: "/a", Destination: "/b"}})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("SetRoutes on web_service => ErrBadRequest, got %v", err)
	}
}

func TestSetPublishPath(t *testing.T) {
	svc, cl := newService(nil, sampleStaticApp("site"))
	if _, err := svc.SetPublishPath(context.Background(), "site", "build"); err != nil {
		t.Fatalf("SetPublishPath: %v", err)
	}
	a := getApp(t, cl, "site")
	if a.Spec.PublishPath != "build" {
		t.Errorf("publishPath = %q, want build", a.Spec.PublishPath)
	}
	// Republish is forced by bumping restartedAt.
	if a.Spec.RestartedAt == "" {
		t.Errorf("SetPublishPath should bump restartedAt to republish")
	}
}

// round-5 finding 11: the content/security setters must reject open-redirect and
// header-injection payloads a bare "/" prefix would otherwise let through, while
// still accepting legitimate values.
func TestStaticSetterValidationHardening(t *testing.T) {
	for _, dest := range []string{"//evil.com", `/\evil.com`, "/ok\r\nSet-Cookie: x"} {
		if err := validateRoutes([]StaticRouteView{{Type: "redirect", Source: "/a", Destination: dest}}); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("redirect dest %q => ErrBadRequest, got %v", dest, err)
		}
	}
	if err := validateRoutes([]StaticRouteView{{Type: "redirect", Source: "/a", Destination: "/b/c"}}); err != nil {
		t.Errorf("clean local redirect must pass, got %v", err)
	}
	if err := validateHeaders([]StaticHeaderView{{Path: "/*", Name: "X Bad", Value: "1"}}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("malformed header name => ErrBadRequest, got %v", err)
	}
	if err := validateHeaders([]StaticHeaderView{{Path: "/*", Name: "X-A", Value: "a\r\nX-Injected: 1"}}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("CRLF header value => ErrBadRequest, got %v", err)
	}
	if err := validateHeaders([]StaticHeaderView{{Path: "/*", Name: "Content-Security-Policy", Value: "default-src 'self'"}}); err != nil {
		t.Errorf("valid security header must pass, got %v", err)
	}
	if err := validatePublishPath("dist\nrm -rf /"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("control-char publishPath => ErrBadRequest, got %v", err)
	}
	// Absolute paths stay valid — an image-backed site serves a known in-image dir.
	if err := validatePublishPath("/usr/share/nginx/html"); err != nil {
		t.Errorf("absolute image publishPath must pass, got %v", err)
	}
}

func TestRESTCreateStaticSite(t *testing.T) {
	svc, _ := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"site","type":"static_site","repo":"https://github.com/acme/site",
		"serviceDetails":{"publishPath":"dist"},
		"routes":[{"type":"rewrite","source":"/*","destination":"/index.html"}]}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create static_site => 201, got %d: %s", rec.Code, rec.Body)
	}
	var got serviceAndDeploy
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// publishPath is nested under serviceDetails, matching Render.
	if got.Service.ServiceDetails["publishPath"] != "dist" {
		t.Errorf("serviceDetails.publishPath = %v, want dist", got.Service.ServiceDetails["publishPath"])
	}
}

func TestRESTPutRoutesAndHeaders(t *testing.T) {
	svc, _ := newService(nil, sampleStaticApp("site"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// PUT /routes replaces the whole list.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PUT", "/v1/services/site/routes",
		strings.NewReader(`[{"type":"redirect","source":"/x","destination":"/y"}]`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT routes => 200, got %d: %s", rec.Code, rec.Body)
	}
	var routes []StaticRouteView
	if err := json.Unmarshal(rec.Body.Bytes(), &routes); err != nil {
		t.Fatalf("decode routes: %v", err)
	}
	if len(routes) != 1 || routes[0].Source != "/x" {
		t.Errorf("routes = %+v", routes)
	}

	// GET /headers reads them back.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/site/headers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET headers => 200, got %d", rec.Code)
	}
}

func TestGraphQLCreateStaticSiteAndSetRoutes(t *testing.T) {
	svc, cl := newService(nil, sampleStaticApp("site"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	// The service query exposes publishPath/routes/headers.
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `{
		service(id:"site"){ type publishPath routes{ type source destination } headers{ name value } }
	}`})
	if len(res.Errors) > 0 {
		t.Fatalf("query errors: %v", res.Errors)
	}

	// setStaticRoutes replaces the routes.
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation {
		setStaticRoutes(id:"site", routes:[{type:"redirect",source:"/a",destination:"/b"}]) {
			routes { source }
		}
	}`})
	if len(res.Errors) > 0 {
		t.Fatalf("mutation errors: %v", res.Errors)
	}
	if got := getApp(t, cl, "site").Spec.Routes; len(got) != 1 || got[0].Source != "/a" {
		t.Errorf("spec.routes = %+v after setStaticRoutes", got)
	}
}

func TestToRenderServiceStaticDetails(t *testing.T) {
	rs := toRenderService(view(sampleStaticApp("site")))
	if rs.Type != appv1alpha1.TypeStaticSite {
		t.Errorf("type = %q, want static_site", rs.Type)
	}
	if rs.ServiceDetails["publishPath"] != "dist" {
		t.Errorf("serviceDetails.publishPath = %v, want dist", rs.ServiceDetails["publishPath"])
	}
}
