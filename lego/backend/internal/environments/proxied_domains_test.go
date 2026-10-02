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

package environments

// proxied_domains_test.go pins w1/m171 for the environment allowlist layer:
// the save is accepted, and REST, GraphQL and MCP name the member services'
// Cloudflare-proxied custom domains on which the layer sees Cloudflare.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

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

// proxiedEnvFixture: env "staging" holds web (one orange-cloud custom domain,
// one grey-cloud) and api (grey-cloud only); worker has an orange-cloud host
// but is NOT a member.
func proxiedEnvFixture(t *testing.T) (*Service, EnvironmentView) {
	t.Helper()
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a", Name: "web-stack"})
	app := func(name string, hosts ...string) *appv1alpha1.App {
		a := sampleApp(name)
		a.Labels = map[string]string{core.LabelTenant: "tea-a"}
		a.Spec.Hosts = hosts
		return a
	}
	svc, _ := newServiceWithClient(st,
		app("web", "shop.example.com", "grey.example.com"),
		app("api", "api.example.com"),
		app("worker", "other.example.com"),
	)
	svc.ProxiedHosts = &core.ProxiedHostDetector{Resolver: mapResolver{
		"shop.example.com":  {"172.67.1.1"},
		"grey.example.com":  {"49.12.20.236"},
		"api.example.com":   {"49.12.20.236"},
		"other.example.com": {"104.21.0.9"},
	}}
	e, err := svc.Create(ctxAs("user-a"), "prj-1", "staging")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e, err = svc.SetServices(ctxAs("user-a"), e.ID, []string{"web", "api"}); err != nil {
		t.Fatalf("SetServices: %v", err)
	}
	return svc, e
}

var officeOnly = []core.IPAllowListEntry{{CIDRBlock: "203.0.113.0/24", Description: "office"}}

func TestEnvironmentAllowListNamesProxiedMemberDomains(t *testing.T) {
	svc, e := proxiedEnvFixture(t)
	if e.IPAllowListProxiedDomains != nil {
		t.Fatalf("the seeded allow-all environment warned about %v", e.IPAllowListProxiedDomains)
	}

	saved, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, false, officeOnly)
	if err != nil {
		t.Fatalf("SetACL must accept the save (warn, not refuse): %v", err)
	}
	want := []string{"shop.example.com"}
	if !slices.Equal(saved.IPAllowListProxiedDomains, want) {
		t.Errorf("SetACL IPAllowListProxiedDomains = %v, want %v", saved.IPAllowListProxiedDomains, want)
	}
	got, err := svc.Get(ctxAs("user-a"), e.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !slices.Equal(got.IPAllowListProxiedDomains, want) {
		t.Errorf("Get IPAllowListProxiedDomains = %v, want %v", got.IPAllowListProxiedDomains, want)
	}
	// The dashboard reads environments through the project list.
	list, err := svc.List(ctxAs("user-a"), "prj-1")
	if err != nil || len(list) != 1 {
		t.Fatalf("List: %v %v", list, err)
	}
	if !slices.Equal(list[0].IPAllowListProxiedDomains, want) {
		t.Errorf("List IPAllowListProxiedDomains = %v, want %v", list[0].IPAllowListProxiedDomains, want)
	}

	// Explicit deny-all refuses Cloudflare and clients alike: nothing to warn.
	denyAll, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, false, []core.IPAllowListEntry{})
	if err != nil {
		t.Fatalf("SetACL deny-all: %v", err)
	}
	if denyAll.IPAllowListProxiedDomains != nil {
		t.Errorf("deny-all warned about %v", denyAll.IPAllowListProxiedDomains)
	}
}

func TestEnvironmentProxiedDomainsOnEverySurface(t *testing.T) {
	svc, e := proxiedEnvFixture(t)
	if _, err := svc.SetACL(ctxAs("user-a"), e.ID, ProtectedStatusUnprotected, false, officeOnly); err != nil {
		t.Fatalf("SetACL: %v", err)
	}

	// REST: PATCH and GET carry the bex extension field.
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	for _, rec := range []struct{ method, body string }{
		{"PATCH", `{"ipAllowList":[{"cidrBlock":"203.0.113.0/24","description":"office"}]}`},
		{"GET", ""},
	} {
		res := doREST(t, mux, rec.method, "/v1/environments/"+e.ID, rec.body)
		if res.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", rec.method, res.Code, res.Body)
		}
		var body struct {
			Proxied []string `json:"ipAllowListProxiedDomains"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s decode: %v", rec.method, err)
		}
		if !slices.Equal(body.Proxied, []string{"shop.example.com"}) {
			t.Errorf("REST %s ipAllowListProxiedDomains = %v", rec.method, body.Proxied)
		}
	}

	// GraphQL: the field on the Environment type.
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctxAs("user-a"),
		RequestString: `{ environments(projectId: "prj-1") { ipAllowListProxiedDomains } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("graphql: %v", res.Errors)
	}
	envs := res.Data.(map[string]any)["environments"].([]any)
	if got, _ := envs[0].(map[string]any)["ipAllowListProxiedDomains"].([]any); len(got) != 1 || got[0] != "shop.example.com" {
		t.Errorf("GraphQL ipAllowListProxiedDomains = %v", got)
	}

	// MCP: a second text block spells the warning out.
	client := newMCPClient(t, ctxAs("user-a"), svc)
	result, err := client.CallTool(ctxAs("user-a"), &mcp.CallToolParams{Name: "get_environment", Arguments: map[string]any{"id": e.ID}})
	if err != nil || result.IsError {
		t.Fatalf("get_environment: %v %#v", err, result)
	}
	if len(result.Content) != 2 {
		t.Fatalf("get_environment: %d content blocks, want JSON + warning", len(result.Content))
	}
	warning, _ := result.Content[1].(*mcp.TextContent)
	if warning == nil || !strings.Contains(warning.Text, "Cloudflare") || !strings.Contains(warning.Text, "shop.example.com") {
		t.Errorf("get_environment warning = %#v", result.Content[1])
	}
}
