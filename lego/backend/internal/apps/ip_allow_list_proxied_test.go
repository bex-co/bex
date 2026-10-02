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

// ip_allow_list_proxied_test.go pins w1/m171: an inbound IP allowlist on a
// service with a Cloudflare-proxied custom domain is SAVED (m150 Decision 2
// warns rather than refuses) and every surface — the service verb, REST
// serviceDetails, GraphQL, and MCP text — names the proxied host on which the
// allowlist sees Cloudflare's edge instead of the client.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// mapResolver answers from a fixed table; an unknown host is an NXDOMAIN.
type mapResolver map[string][]string

func (m mapResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	raw, ok := m[host]
	if !ok {
		return nil, errors.New("no such host")
	}
	out := make([]netip.Addr, len(raw))
	for i, a := range raw {
		out[i] = netip.MustParseAddr(a)
	}
	return out, nil
}

// proxiedFixture is a web service with one orange-cloud custom domain
// (104.16.0.0/13 is Cloudflare's), one grey-cloud domain pointing at the bex
// edge, and its platform host.
func proxiedFixture(t *testing.T, allowList ...string) (*Service, *appv1alpha1.App) {
	t.Helper()
	app := sampleApp("web")
	app.Spec.Type = appv1alpha1.TypeWebService
	app.Spec.Expose = true
	app.Spec.Hosts = []string{"shop.example.com", "grey.example.com"}
	app.Spec.IPAllowList = allowList
	svc, _ := newService(nil, app)
	svc.ProxiedHosts = &core.ProxiedHostDetector{Resolver: mapResolver{
		"shop.example.com": {"104.21.32.1", "2606:4700:3030::6815:2001"},
		"grey.example.com": {"49.12.20.236"},
		"web.onbex.co":     {"49.12.20.236"},
	}}
	return svc, app
}

var officeAllowList = []core.IPAllowListEntry{{CIDRBlock: "203.0.113.0/24", Description: "office"}}

func TestSetIPAllowListAcceptsAndNamesCloudflareProxiedDomains(t *testing.T) {
	svc, _ := proxiedFixture(t)
	v, err := svc.SetIPAllowList(context.Background(), "web", officeAllowList)
	if err != nil {
		t.Fatalf("SetIPAllowList must accept the save (warn, not refuse): %v", err)
	}
	if len(v.IPAllowList) != 1 {
		t.Fatalf("saved allowlist = %v, want the office entry", v.IPAllowList)
	}
	if want := []string{"shop.example.com"}; !slices.Equal(v.IPAllowListProxiedDomains, want) {
		t.Errorf("IPAllowListProxiedDomains = %v, want %v (only the orange-cloud host)", v.IPAllowListProxiedDomains, want)
	}

	got, err := svc.Get(context.Background(), "web")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !slices.Equal(got.IPAllowListProxiedDomains, []string{"shop.example.com"}) {
		t.Errorf("Get IPAllowListProxiedDomains = %v, want [shop.example.com]", got.IPAllowListProxiedDomains)
	}
}

func TestProxiedDomainsOnlyWarnWhileTheAllowListRestricts(t *testing.T) {
	for name, entries := range map[string][]core.IPAllowListEntry{
		"cleared":  nil,
		"open-v4":  {{CIDRBlock: "0.0.0.0/0"}},
		"open-mix": {{CIDRBlock: "203.0.113.0/24"}, {CIDRBlock: "::/0"}},
	} {
		t.Run(name, func(t *testing.T) {
			svc, _ := proxiedFixture(t, "203.0.113.0/24")
			v, err := svc.SetIPAllowList(context.Background(), "web", entries)
			if err != nil {
				t.Fatalf("SetIPAllowList: %v", err)
			}
			if v.IPAllowListProxiedDomains != nil {
				t.Errorf("a non-restricting allowlist warned about %v", v.IPAllowListProxiedDomains)
			}
		})
	}
}

func TestProxiedDomainsNeedAWiredDetector(t *testing.T) {
	svc, _ := proxiedFixture(t, "203.0.113.0/24")
	svc.ProxiedHosts = nil
	v, err := svc.Get(context.Background(), "web")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v.IPAllowListProxiedDomains != nil {
		t.Errorf("no detector wired, got %v", v.IPAllowListProxiedDomains)
	}
}

// TestApplyServicePatchKeepsTheAllowListWarning: a patch answers with its last
// row's view, and maintenanceMode runs after ipAllowList — the warning must
// survive that.
func TestApplyServicePatchKeepsTheAllowListWarning(t *testing.T) {
	svc, app := proxiedFixture(t)
	app.Spec.Tier = "starter" // maintenance mode needs a paid plan
	if err := svc.Client.Update(context.Background(), app); err != nil {
		t.Fatalf("set tier: %v", err)
	}
	entries := officeAllowList
	v, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
		IPAllowList:     &entries,
		MaintenanceMode: &MaintenanceModeView{Enabled: true},
	})
	if err != nil {
		t.Fatalf("ApplyServicePatch: %v", err)
	}
	if !v.MaintenanceMode.Enabled {
		t.Fatalf("the later maintenance row did not run: %+v", v.MaintenanceMode)
	}
	if !slices.Equal(v.IPAllowListProxiedDomains, []string{"shop.example.com"}) {
		t.Errorf("patch response IPAllowListProxiedDomains = %v, want [shop.example.com]", v.IPAllowListProxiedDomains)
	}
}

func TestRESTServiceDetailsNameProxiedDomains(t *testing.T) {
	svc, _ := proxiedFixture(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	decode := func(t *testing.T, rec *httptest.ResponseRecorder) []string {
		t.Helper()
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		var body struct {
			ProxiedAtRoot  json.RawMessage `json:"ipAllowListProxiedDomains"`
			ServiceDetails struct {
				Proxied []string `json:"ipAllowListProxiedDomains"`
			} `json:"serviceDetails"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.ProxiedAtRoot != nil {
			t.Errorf("ipAllowListProxiedDomains at the JSON root; it belongs beside serviceDetails.ipAllowList")
		}
		return body.ServiceDetails.Proxied
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web",
		strings.NewReader(`{"serviceDetails":{"ipAllowList":[{"cidrBlock":"203.0.113.0/24","description":"office"}]}}`)))
	if got := decode(t, rec); !slices.Equal(got, []string{"shop.example.com"}) {
		t.Errorf("PATCH serviceDetails.ipAllowListProxiedDomains = %v", got)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
	if got := decode(t, rec); !slices.Equal(got, []string{"shop.example.com"}) {
		t.Errorf("GET serviceDetails.ipAllowListProxiedDomains = %v", got)
	}

	// Cleared: the key disappears again, byte-compatible with Render.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web",
		strings.NewReader(`{"serviceDetails":{"ipAllowList":[]}}`)))
	if strings.Contains(rec.Body.String(), "ipAllowListProxiedDomains") {
		t.Errorf("cleared allowlist still carries the warning field: %s", rec.Body)
	}
}

func TestGraphQLNamesProxiedDomains(t *testing.T) {
	svc, _ := proxiedFixture(t)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	for _, q := range []string{
		`mutation { r: setServiceIpAllowList(id: "web", entries: [{cidrBlock:"203.0.113.0/24"}]) { ipAllowListProxiedDomains } }`,
		`{ r: service(id: "web") { ipAllowListProxiedDomains } }`,
	} {
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: q})
		if len(res.Errors) > 0 {
			t.Fatalf("%s: %v", q, res.Errors)
		}
		got, _ := res.Data.(map[string]any)["r"].(map[string]any)["ipAllowListProxiedDomains"].([]any)
		if len(got) != 1 || got[0] != "shop.example.com" {
			t.Errorf("%s => %v, want [shop.example.com]", q, got)
		}
	}
}

func TestMCPTextWarnsAboutProxiedDomains(t *testing.T) {
	svc, _ := proxiedFixture(t)
	ctx := context.Background()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
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

	texts := func(name string, args map[string]any) []string {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("%s: err=%v result=%#v", name, err, res)
		}
		var out []string
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				out = append(out, tc.Text)
			}
		}
		return out
	}

	for _, call := range []struct {
		name string
		args map[string]any
	}{
		{"update_service", map[string]any{"serviceId": "web", "ipAllowListCidrs": []string{"203.0.113.0/24"}}},
		{"get_service", map[string]any{"serviceId": "web"}},
	} {
		got := texts(call.name, call.args)
		if len(got) != 2 {
			t.Fatalf("%s: %d text blocks, want the JSON body then the warning: %q", call.name, len(got), got)
		}
		if !strings.Contains(got[0], `"ipAllowListProxiedDomains":["shop.example.com"]`) {
			t.Errorf("%s: JSON block lacks the field: %s", call.name, got[0])
		}
		if !strings.Contains(got[1], "Cloudflare") || !strings.Contains(got[1], "shop.example.com") {
			t.Errorf("%s: warning block = %q", call.name, got[1])
		}
	}

	// No restriction, no warning: the result is the SDK's single JSON block.
	if got := texts("update_service", map[string]any{"serviceId": "web", "ipAllowListCidrs": []string{}}); len(got) != 1 {
		t.Errorf("cleared allowlist: %d text blocks, want 1: %q", len(got), got)
	}
}
