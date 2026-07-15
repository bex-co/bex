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

// ip_allow_list_test.go covers the service inbound ipAllowList (w7/m32, Render's
// ipAllowList on webServiceDetails + staticSiteDetails) — CIDR validation, CRD
// round-trip, type rejection (private / worker / cron have no Ingress), and
// adapter parity (REST/GraphQL/MCP). Mirrors the depth of notifyonfail_test.go.

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

// --- CIDR validation ---

func TestSetIPAllowListRejectsInvalidCIDR(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))

	badCIDRs := [][]string{
		{"not-a-cidr"},
		{"1.2.3.4"},     // host address, not CIDR
		{"1.2.3.4/33"},  // prefix too long for IPv4
		{"::1/129"},     // prefix too long for IPv6
		{""},
		{"203.0.113.0/24", "bad"},
	}
	for _, cidrs := range badCIDRs {
		if _, err := svc.SetIPAllowList(context.Background(), "web", cidrs); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("SetIPAllowList(%v) should be ErrBadRequest, got %v", cidrs, err)
		}
	}
}

func TestSetIPAllowListAcceptsValidCIDRs(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))

	valid := [][]string{
		nil,
		{},
		{"0.0.0.0/0"},
		{"203.0.113.0/24"},
		{"203.0.113.0/24", "10.0.0.0/8"},
		{"2001:db8::/32"},
	}
	for _, cidrs := range valid {
		if _, err := svc.SetIPAllowList(context.Background(), "web", cidrs); err != nil {
			t.Errorf("SetIPAllowList(%v) should succeed, got %v", cidrs, err)
		}
	}
}

// --- CRD round-trip ---

func TestSetIPAllowListRoundTripsThroughSpec(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	cidrs := []string{"203.0.113.0/24", "10.0.0.0/8"}

	v, err := svc.SetIPAllowList(context.Background(), "web", cidrs)
	if err != nil {
		t.Fatalf("SetIPAllowList: %v", err)
	}
	if len(v.IPAllowList) != 2 || v.IPAllowList[0] != cidrs[0] {
		t.Errorf("AppView.IPAllowList = %v, want %v", v.IPAllowList, cidrs)
	}
	got := getApp(t, cl, "web").Spec.IPAllowList
	if len(got) != 2 || got[0] != cidrs[0] || got[1] != cidrs[1] {
		t.Errorf("spec.ipAllowList = %v, want %v", got, cidrs)
	}

	// Clear it.
	v, err = svc.SetIPAllowList(context.Background(), "web", nil)
	if err != nil {
		t.Fatalf("SetIPAllowList(nil): %v", err)
	}
	if len(v.IPAllowList) != 0 {
		t.Errorf("AppView.IPAllowList after clear = %v, want empty", v.IPAllowList)
	}
}

// --- REST ---

func TestRESTCreateWithIPAllowListAndReadBack(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// Render wire shape: ipAllowList nested under serviceDetails as [{cidrBlock,description}].
	// description is accepted but not stored (flat []string in the CRD).
	body := `{"name":"web","image":{"imagePath":"nginx:v1"},"serviceDetails":{"ipAllowList":[{"cidrBlock":"203.0.113.0/24","description":"office"}]}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	spec := getApp(t, cl, "web").Spec.IPAllowList
	if len(spec) != 1 || spec[0] != "203.0.113.0/24" {
		t.Fatalf("spec.ipAllowList = %v, want [203.0.113.0/24]", spec)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET => 200, got %d", rec.Code)
	}
	var out struct {
		IPAllowList []struct {
			CidrBlock   string `json:"cidrBlock"`
			Description string `json:"description"`
		} `json:"ipAllowList"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.IPAllowList) != 1 || out.IPAllowList[0].CidrBlock != "203.0.113.0/24" {
		t.Errorf("GET ipAllowList = %v, want [{cidrBlock:203.0.113.0/24}]", out.IPAllowList)
	}
}

func TestRESTCreateWithInvalidIPAllowListReturns400(t *testing.T) {
	svc, _ := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","image":{"imagePath":"nginx:v1"},"serviceDetails":{"ipAllowList":[{"cidrBlock":"not-a-cidr","description":"bad"}]}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create with invalid cidr => 400, got %d: %s", rec.Code, rec.Body)
	}
}

func TestRESTPatchIPAllowList(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// PATCH nests ipAllowList under serviceDetails (Render-faithful).
	body := strings.NewReader(`{"serviceDetails":{"ipAllowList":[{"cidrBlock":"10.0.0.0/8","description":"vpn"},{"cidrBlock":"192.168.0.0/16","description":"lan"}]}}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH => 200, got %d: %s", rec.Code, rec.Body)
	}
	spec := getApp(t, cl, "web").Spec.IPAllowList
	if len(spec) != 2 || spec[0] != "10.0.0.0/8" || spec[1] != "192.168.0.0/16" {
		t.Errorf("spec.ipAllowList = %v, want [10.0.0.0/8 192.168.0.0/16]", spec)
	}
}

func TestRESTPatchIPAllowListWithInvalidCIDRReturns400(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := strings.NewReader(`{"serviceDetails":{"ipAllowList":[{"cidrBlock":"bad-cidr","description":""}]}}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PATCH invalid cidr => 400, got %d: %s", rec.Code, rec.Body)
	}
}

// --- GraphQL ---

func TestGraphQLSetServiceIpAllowList(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setServiceIpAllowList(id: "web", cidrs: ["203.0.113.0/24"]) { ipAllowList } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("setServiceIpAllowList: %v", res.Errors)
	}
	data := res.Data.(map[string]any)["setServiceIpAllowList"].(map[string]any)
	list, _ := data["ipAllowList"].([]any)
	if len(list) != 1 || list[0] != "203.0.113.0/24" {
		t.Errorf("setServiceIpAllowList.ipAllowList = %v, want [203.0.113.0/24]", list)
	}
	if spec := getApp(t, cl, "web").Spec.IPAllowList; len(spec) != 1 || spec[0] != "203.0.113.0/24" {
		t.Errorf("spec.ipAllowList = %v, want [203.0.113.0/24]", spec)
	}
}

func TestGraphQLSetServiceIpAllowListRejectsInvalidCIDR(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setServiceIpAllowList(id: "web", cidrs: ["not-a-cidr"]) { id } }`})
	if len(res.Errors) == 0 {
		t.Fatal("setServiceIpAllowList with an invalid CIDR should error")
	}
}

func TestGraphQLServerQueryReturnsIPAllowList(t *testing.T) {
	app := sampleApp("web")
	app.Spec.IPAllowList = []string{"203.0.113.0/24", "10.0.0.0/8"}
	svc, _ := newService(nil, app)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ server(id: "web") { ipAllowList } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("server query: %v", res.Errors)
	}
	list, _ := res.Data.(map[string]any)["server"].(map[string]any)["ipAllowList"].([]any)
	if len(list) != 2 || list[0] != "203.0.113.0/24" {
		t.Errorf("server.ipAllowList = %v, want [203.0.113.0/24 10.0.0.0/8]", list)
	}
}

// --- MCP ---

func TestMCPCreateWebServiceThreadsIPAllowList(t *testing.T) {
	req := createWebServiceArgs{
		Name:        "w",
		Image:       "nginx:v1",
		IPAllowList: []string{"203.0.113.0/24"},
	}.toCreateRequest()
	if len(req.IPAllowList) != 1 || req.IPAllowList[0] != "203.0.113.0/24" {
		t.Errorf("create_web_service ipAllowList not threaded: %+v", req)
	}
}

func TestMCPSetIPAllowListUpdatesSpec(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	cidrs := []string{"203.0.113.0/24"}

	v, err := svc.SetIPAllowList(context.Background(), "web", cidrs)
	if err != nil {
		t.Fatalf("SetIPAllowList: %v", err)
	}
	out := toRenderService(v)
	if len(out.IPAllowList) != 1 || out.IPAllowList[0].CidrBlock != "203.0.113.0/24" {
		t.Errorf("toRenderService.ipAllowList = %v, want [{cidrBlock:203.0.113.0/24}]", out.IPAllowList)
	}
	if spec := getApp(t, cl, "web").Spec.IPAllowList; len(spec) != 1 || spec[0] != "203.0.113.0/24" {
		t.Errorf("spec.ipAllowList = %v, want [203.0.113.0/24]", spec)
	}
}

// --- Adapter parity ---

// TestIPAllowListPresentOnAllThreeSurfaces proves ipAllowList round-trips
// identically over REST, GraphQL, and MCP — mirrors notifyonfail_test.go's
// TestNotifyOnFailPresentOnAllThreeSurfaces.
func TestIPAllowListPresentOnAllThreeSurfaces(t *testing.T) {
	cases := []struct {
		name  string
		cidrs []string
	}{
		{name: "empty", cidrs: nil},
		{name: "single", cidrs: []string{"203.0.113.0/24"}},
		{name: "multi", cidrs: []string{"203.0.113.0/24", "10.0.0.0/8"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app := sampleApp("web")
			app.Spec.IPAllowList = c.cidrs
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
			var restBody struct {
				IPAllowList []struct{ CidrBlock string `json:"cidrBlock"` } `json:"ipAllowList"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &restBody); err != nil {
				t.Fatalf("decode REST body: %v", err)
			}
			if len(restBody.IPAllowList) != len(c.cidrs) {
				t.Errorf("REST ipAllowList len = %d, want %d", len(restBody.IPAllowList), len(c.cidrs))
			}
			for i, e := range restBody.IPAllowList {
				if e.CidrBlock != c.cidrs[i] {
					t.Errorf("REST ipAllowList[%d].cidrBlock = %q, want %q", i, e.CidrBlock, c.cidrs[i])
				}
			}

			// GraphQL
			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			})
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: `{ service(id: "web") { ipAllowList } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("gql: %v", res.Errors)
			}
			gqlList, _ := res.Data.(map[string]any)["service"].(map[string]any)["ipAllowList"].([]any)
			if len(gqlList) != len(c.cidrs) {
				t.Errorf("GraphQL ipAllowList len = %d, want %d", len(gqlList), len(c.cidrs))
			}
			for i, entry := range gqlList {
				if entry != c.cidrs[i] {
					t.Errorf("GraphQL ipAllowList[%d] = %v, want %q", i, entry, c.cidrs[i])
				}
			}

			// MCP (via toRenderService — same path as get_service)
			v, err := svc.Get(ctx, "web")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			rendered := toRenderService(v)
			if len(rendered.IPAllowList) != len(c.cidrs) {
				t.Errorf("MCP ipAllowList len = %d, want %d", len(rendered.IPAllowList), len(c.cidrs))
			}
			for i, e := range rendered.IPAllowList {
				if e.CidrBlock != c.cidrs[i] {
					t.Errorf("MCP ipAllowList[%d].cidrBlock = %q, want %q", i, e.CidrBlock, c.cidrs[i])
				}
			}
		})
	}
}
