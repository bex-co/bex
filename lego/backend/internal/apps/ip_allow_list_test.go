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
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// --- CIDR validation ---

func TestSetIPAllowListRejectsInvalidCIDR(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))

	badCIDRs := [][]string{
		{"not-a-cidr"},
		{"1.2.3.4"},    // host address, not CIDR
		{"1.2.3.4/33"}, // prefix too long for IPv4
		{"::1/129"},    // prefix too long for IPv6
		{""},
		{"203.0.113.0/24", "bad"},
	}
	for _, cidrs := range badCIDRs {
		if _, err := svc.SetIPAllowList(context.Background(), "web", core.AllowListFromCIDRs(cidrs)); !errors.Is(err, core.ErrBadRequest) {
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
		if _, err := svc.SetIPAllowList(context.Background(), "web", core.AllowListFromCIDRs(cidrs)); err != nil {
			t.Errorf("SetIPAllowList(%v) should succeed, got %v", cidrs, err)
		}
	}
}

// --- CRD round-trip ---

func TestSetIPAllowListRoundTripsThroughSpec(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	entries := []core.IPAllowListEntry{
		{CIDRBlock: "203.0.113.0/24", Description: "office"},
		{CIDRBlock: "10.0.0.0/8", Description: "vpn"},
	}

	v, err := svc.SetIPAllowList(context.Background(), "web", entries)
	if err != nil {
		t.Fatalf("SetIPAllowList: %v", err)
	}
	if len(v.IPAllowList) != 2 || v.IPAllowList[0] != entries[0] {
		t.Errorf("AppView.IPAllowList = %v, want %v", v.IPAllowList, entries)
	}
	app := getApp(t, cl, "web")
	if app.Spec.IPAllowList != nil {
		t.Errorf("legacy spec.ipAllowList = %v, want nil", app.Spec.IPAllowList)
	}
	wantSpec := core.AllowListToSpec(entries)
	if got := app.Spec.IPAllowListEntries; len(got) != 2 || got[0] != wantSpec[0] || got[1] != wantSpec[1] {
		t.Errorf("spec.ipAllowListEntries = %v, want %v", got, wantSpec)
	}

	// Clear it.
	v, err = svc.SetIPAllowList(context.Background(), "web", nil)
	if err != nil {
		t.Fatalf("SetIPAllowList(nil): %v", err)
	}
	if len(v.IPAllowList) != 0 {
		t.Errorf("AppView.IPAllowList after clear = %v, want empty", v.IPAllowList)
	}
	app = getApp(t, cl, "web")
	if app.Spec.IPAllowList != nil || app.Spec.IPAllowListEntries != nil {
		t.Errorf("clear left legacy=%v structured=%v", app.Spec.IPAllowList, app.Spec.IPAllowListEntries)
	}
}

// --- serviceDetails nesting + type gate ---

// TestRenderServiceDetailsGatesIPAllowListToIngressTypes proves ipAllowList
// only ever appears in serviceDetails for the two types Render's schema
// declares it on (web_service, static_site). private/worker/cron omit the
// property entirely — matching privateServiceDetails/backgroundWorkerDetails/
// cronJobDetails, which have no ipAllowList — even when a view somehow carries
// one. The create/PATCH decode already rejects it on those types
// (deploy.go); this is the belt-and-suspenders render gate (w6/m106).
func TestRenderServiceDetailsGatesIPAllowListToIngressTypes(t *testing.T) {
	view := AppView{IPAllowList: []core.IPAllowListEntry{{CIDRBlock: "203.0.113.0/24", Description: "office"}}}

	for _, svcType := range []string{
		appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker,
		appv1alpha1.TypeCronJob,
	} {
		if _, present := renderServiceDetails(view, svcType, "")["ipAllowList"]; present {
			t.Errorf("%s serviceDetails carries ipAllowList, want it omitted (no such Render property)", svcType)
		}
	}

	for _, svcType := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypeStaticSite} {
		list, ok := renderServiceDetails(view, svcType, "")["ipAllowList"].([]core.IPAllowListEntry)
		if !ok || len(list) != 1 || list[0].CIDRBlock != "203.0.113.0/24" || list[0].Description != "office" {
			t.Errorf("%s serviceDetails.ipAllowList = %#v, want the single {cidrBlock,description} entry", svcType, list)
		}
	}

	// Empty allowlist on an ingress type omits the key (open to all = Render's
	// default), like every other optional serviceDetails field.
	if _, present := renderServiceDetails(AppView{}, appv1alpha1.TypeWebService, "")["ipAllowList"]; present {
		t.Error("empty allowlist should omit serviceDetails.ipAllowList entirely")
	}
}

// --- REST ---

func TestRESTCreateWithIPAllowListAndReadBack(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// Render wire shape: ipAllowList nested under serviceDetails as
	// [{cidrBlock,description}], with both fields persisted — on the create
	// response AND a subsequent GET (no create-vs-get divergence), and NEVER at
	// the JSON root (webServiceDetails owns ipAllowList; the top-level service
	// schema has no such property — w6/m106).
	body := `{"name":"web","image":{"imagePath":"nginx:v1"},"serviceDetails":{"ipAllowList":[{"cidrBlock":"203.0.113.0/24","description":"office"}]}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	spec := getApp(t, cl, "web").Spec
	if spec.IPAllowList != nil {
		t.Fatalf("legacy spec.ipAllowList = %v, want nil", spec.IPAllowList)
	}
	if len(spec.IPAllowListEntries) != 1 || spec.IPAllowListEntries[0] != (appv1alpha1.IPAllowEntry{CIDR: "203.0.113.0/24", Description: "office"}) {
		t.Fatalf("spec.ipAllowListEntries = %v", spec.IPAllowListEntries)
	}
	// The create response (serviceAndDeploy: {service, deployId}) already carries
	// the nested shape, identical to the GET below.
	var created struct {
		Service json.RawMessage `json:"service"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create envelope: %v", err)
	}
	assertNestedIPAllowList(t, "create", created.Service, "203.0.113.0/24", "office")

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET => 200, got %d", rec.Code)
	}
	assertNestedIPAllowList(t, "get", rec.Body.Bytes(), "203.0.113.0/24", "office")
}

// assertNestedIPAllowList fails unless the service JSON carries its inbound
// allowlist at serviceDetails.ipAllowList as [{cidrBlock,description}] and NOT
// at the JSON root — Render's exact nesting (webServiceDetails owns the field;
// the top-level service schema has no ipAllowList property).
func assertNestedIPAllowList(t *testing.T, label string, serviceJSON []byte, wantCIDR, wantDesc string) {
	t.Helper()
	var out struct {
		IPAllowList    json.RawMessage `json:"ipAllowList"` // must be absent at the root
		ServiceDetails struct {
			IPAllowList []struct {
				CidrBlock   string `json:"cidrBlock"`
				Description string `json:"description"`
			} `json:"ipAllowList"`
		} `json:"serviceDetails"`
	}
	if err := json.Unmarshal(serviceJSON, &out); err != nil {
		t.Fatalf("%s: unmarshal: %v", label, err)
	}
	if out.IPAllowList != nil {
		t.Errorf("%s: ipAllowList present at JSON root (%s), want it only under serviceDetails", label, out.IPAllowList)
	}
	got := out.ServiceDetails.IPAllowList
	if len(got) != 1 || got[0].CidrBlock != wantCIDR || got[0].Description != wantDesc {
		t.Errorf("%s: serviceDetails.ipAllowList = %v, want [{%s %s}]", label, got, wantCIDR, wantDesc)
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
	spec := getApp(t, cl, "web").Spec
	want := []appv1alpha1.IPAllowEntry{{CIDR: "10.0.0.0/8", Description: "vpn"}, {CIDR: "192.168.0.0/16", Description: "lan"}}
	if len(spec.IPAllowListEntries) != 2 || spec.IPAllowListEntries[0] != want[0] || spec.IPAllowListEntries[1] != want[1] {
		t.Errorf("spec.ipAllowListEntries = %v, want %v", spec.IPAllowListEntries, want)
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
		RequestString: `mutation { setServiceIpAllowList(id: "web", entries: [{cidrBlock:"203.0.113.0/24", description:"office"}]) { ipAllowList ipAllowListEntries { cidrBlock description } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("setServiceIpAllowList: %v", res.Errors)
	}
	data := res.Data.(map[string]any)["setServiceIpAllowList"].(map[string]any)
	list, _ := data["ipAllowList"].([]any)
	if len(list) != 1 || list[0] != "203.0.113.0/24" {
		t.Errorf("setServiceIpAllowList.ipAllowList = %v, want [203.0.113.0/24]", list)
	}
	structured := data["ipAllowListEntries"].([]any)
	if got := structured[0].(map[string]any)["description"]; got != "office" {
		t.Errorf("GraphQL description = %v, want office", got)
	}
	if spec := getApp(t, cl, "web").Spec.IPAllowListEntries; len(spec) != 1 || spec[0].CIDR != "203.0.113.0/24" || spec[0].Description != "office" {
		t.Errorf("spec.ipAllowListEntries = %v", spec)
	}
}

func TestGraphQLSetServiceIpAllowListRejectsConflictingForms(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setServiceIpAllowList(id:"web", cidrs:["203.0.113.0/24"], entries:[{cidrBlock:"10.0.0.0/8"}]) { id } }`})
	if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Message, "conflicting") {
		t.Fatalf("conflicting forms should error, got %v", res.Errors)
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
		RequestString: `{ server(id: "web") { ipAllowList ipAllowListEntries { cidrBlock description } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("server query: %v", res.Errors)
	}
	list, _ := res.Data.(map[string]any)["server"].(map[string]any)["ipAllowList"].([]any)
	if len(list) != 2 || list[0] != "203.0.113.0/24" {
		t.Errorf("server.ipAllowList = %v, want [203.0.113.0/24 10.0.0.0/8]", list)
	}
	entries := res.Data.(map[string]any)["server"].(map[string]any)["ipAllowListEntries"].([]any)
	if len(entries) != 2 || entries[0].(map[string]any)["description"] != "" {
		t.Errorf("legacy App structured projection = %v", entries)
	}
}

// --- MCP ---

// mcpNestedAllowList extracts serviceDetails.ipAllowList from an MCP service
// result. get_service, create_*, and update_service all return the shared
// renderService shape, so — like REST — the allowlist nests under
// serviceDetails, not at the result root (w6/m106).
func mcpNestedAllowList(t *testing.T, got map[string]any) []map[string]any {
	t.Helper()
	if _, atRoot := got["ipAllowList"]; atRoot {
		t.Errorf("MCP service result carries ipAllowList at the root: %#v", got["ipAllowList"])
	}
	details, ok := got["serviceDetails"].(map[string]any)
	if !ok {
		t.Fatalf("MCP result missing serviceDetails object: %#v", got)
	}
	raw, _ := details["ipAllowList"].([]any)
	out := make([]map[string]any, len(raw))
	for i, e := range raw {
		out[i], _ = e.(map[string]any)
	}
	return out
}

func TestMCPCreateWebServiceThreadsStructuredIPAllowList(t *testing.T) {
	svc, cl := newService(nil)
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()

	got := call("create_web_service", map[string]any{
		"name":         "mcp-allowlist",
		"image":        "nginx:v1",
		"runtime":      "image",
		"buildCommand": "",
		"startCommand": "",
		"ipAllowListEntries": []map[string]any{{
			"cidrBlock":   "203.0.113.0/24",
			"description": "office",
		}},
	})
	entries := mcpNestedAllowList(t, got)
	if len(entries) != 1 || entries[0]["cidrBlock"] != "203.0.113.0/24" || entries[0]["description"] != "office" {
		t.Fatalf("create_web_service serviceDetails.ipAllowList = %#v", entries)
	}
	if spec := getApp(t, cl, "mcp-allowlist").Spec; len(spec.IPAllowListEntries) != 1 || spec.IPAllowListEntries[0].Description != "office" {
		t.Fatalf("spec.ipAllowListEntries = %#v", spec.IPAllowListEntries)
	}
}

func TestMCPSetIPAllowListUpdatesSpec(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()

	got := call("update_service", map[string]any{
		"serviceId": "web",
		"ipAllowList": []map[string]any{{
			"cidrBlock":   "203.0.113.0/24",
			"description": "office",
		}},
	})
	entries := mcpNestedAllowList(t, got)
	if len(entries) != 1 || entries[0]["cidrBlock"] != "203.0.113.0/24" || entries[0]["description"] != "office" {
		t.Errorf("update_service serviceDetails.ipAllowList = %#v", entries)
	}
	if spec := getApp(t, cl, "web").Spec.IPAllowListEntries; len(spec) != 1 || spec[0].CIDR != "203.0.113.0/24" || spec[0].Description != "office" {
		t.Errorf("spec.ipAllowListEntries = %v", spec)
	}

	// A present list REPLACES; an empty one clears. Both were the retired
	// setter's contract and both must survive the fold.
	cleared := call("update_service", map[string]any{"serviceId": "web", "ipAllowList": []map[string]any{}})
	if list := mcpNestedAllowList(t, cleared); len(list) != 0 {
		t.Errorf("update_service ipAllowList=[] = %#v", list)
	}
	if spec := getApp(t, cl, "web").Spec.IPAllowListEntries; len(spec) != 0 {
		t.Errorf("spec.ipAllowListEntries after clear = %v", spec)
	}

	// The plain-string form reaches the same field.
	viaCIDRs := call("update_service", map[string]any{"serviceId": "web", "ipAllowListCidrs": []string{"198.51.100.0/24"}})
	if list := mcpNestedAllowList(t, viaCIDRs); len(list) != 1 {
		t.Errorf("update_service ipAllowListCidrs = %#v", list)
	}
	if spec := getApp(t, cl, "web").Spec.IPAllowListEntries; len(spec) != 1 || spec[0].CIDR != "198.51.100.0/24" {
		t.Errorf("spec.ipAllowListEntries via cidrs = %v", spec)
	}
}

func TestMCPSetIPAllowListRejectsConflictingForms(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	ctx := context.Background()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	result, err := client.CallTool(ctx, &mcp.CallToolParams{
		Name: "update_service",
		Arguments: map[string]any{
			"serviceId":        "web",
			"ipAllowListCidrs": []string{"203.0.113.0/24"},
			"ipAllowList":      []map[string]any{{"cidrBlock": "10.0.0.0/8"}},
		},
	})
	if err != nil {
		t.Fatalf("update_service transport error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("conflicting MCP forms should return a tool error: %#v", result)
	}
}

func TestBlueprintServicePreservesIPAllowListDescription(t *testing.T) {
	req, _, err := parseService(blueprintParseOverrides{}, bexService{
		Name: "blueprint-web",
		Type: "web",
		IPAllowList: []bexIPEntry{{
			Source:      "203.0.113.0/24",
			Description: "office",
		}},
	})
	if err != nil {
		t.Fatalf("parseService: %v", err)
	}
	if len(req.IPAllowList) != 1 || req.IPAllowList[0] != (core.IPAllowListEntry{CIDRBlock: "203.0.113.0/24", Description: "office"}) {
		t.Fatalf("Blueprint ipAllowList = %#v", req.IPAllowList)
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
				ServiceDetails struct {
					IPAllowList []struct {
						CidrBlock string `json:"cidrBlock"`
					} `json:"ipAllowList"`
				} `json:"serviceDetails"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &restBody); err != nil {
				t.Fatalf("decode REST body: %v", err)
			}
			restList := restBody.ServiceDetails.IPAllowList
			if len(restList) != len(c.cidrs) {
				t.Errorf("REST serviceDetails.ipAllowList len = %d, want %d", len(restList), len(c.cidrs))
			}
			for i, e := range restList {
				if e.CidrBlock != c.cidrs[i] {
					t.Errorf("REST serviceDetails.ipAllowList[%d].cidrBlock = %q, want %q", i, e.CidrBlock, c.cidrs[i])
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

			// MCP shares REST's rendering (get_service / create_* / list_services
			// all route through toRenderService), so it carries ipAllowList at the
			// same serviceDetails.ipAllowList location, not the JSON root.
			v, err := svc.Get(ctx, "web")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			rendered := toRenderService(v)
			mcpList, _ := rendered.ServiceDetails["ipAllowList"].([]core.IPAllowListEntry)
			if len(mcpList) != len(c.cidrs) {
				t.Errorf("MCP serviceDetails.ipAllowList len = %d, want %d", len(mcpList), len(c.cidrs))
			}
			for i, e := range mcpList {
				if e.CIDRBlock != c.cidrs[i] {
					t.Errorf("MCP serviceDetails.ipAllowList[%d].cidrBlock = %q, want %q", i, e.CIDRBlock, c.cidrs[i])
				}
			}
		})
	}
}
