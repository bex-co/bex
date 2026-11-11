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
	"slices"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// service_ip_block_test.go is w8/m157. Render: "If you delete all rules,
// Render blocks all inbound traffic to the affected service(s)", and a
// service nobody changed has the single rule 0.0.0.0/0. On bex an empty list
// on a web service removed the restriction instead, and the service answered
// anyone. These pin the three states (untouched, rules, blocked) on every
// writer and reader, and that a write which omits the list changes nothing.

// restAllowList is serviceDetails.ipAllowList of a REST service body, and
// whether the key was there at all.
func restAllowList(t *testing.T, body []byte) (cidrs []string, present bool) {
	t.Helper()
	var svc struct {
		ServiceDetails map[string]json.RawMessage `json:"serviceDetails"`
	}
	if err := json.Unmarshal(body, &svc); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	raw, present := svc.ServiceDetails["ipAllowList"]
	if !present {
		return nil, false
	}
	var entries []core.IPAllowListEntry
	if err := json.Unmarshal(raw, &entries); err != nil || entries == nil {
		t.Fatalf("ipAllowList = %s, want an array (err %v)", raw, err)
	}
	return core.AllowListCIDRs(entries), true
}

func restOK(t *testing.T, mux http.Handler, method, path, body string) []byte {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	if rec.Code/100 != 2 {
		t.Fatalf("%s %s %s => %d: %s", method, path, body, rec.Code, rec.Body)
	}
	return rec.Body.Bytes()
}

var allowAllCIDRs = []string{"0.0.0.0/0", "::/0"}

func TestRESTEmptyIPAllowListBlocksAServiceAndAnOmittedOneChangesNothing(t *testing.T) {
	office := sampleApp("web")
	office.Spec.IPAllowListEntries = []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.0/24", Description: "office"}}
	svc, cl := newService(nil, office)
	raw := http.NewServeMux()
	svc.RegisterREST(raw)
	mux := validated(t, raw)
	read := func() ([]string, bool) { return restAllowList(t, restOK(t, mux, "GET", "/v1/services/web", "")) }

	// The finding: this write removed the restriction.
	restOK(t, mux, "PATCH", "/v1/services/web", `{"serviceDetails":{"ipAllowList":[]}}`)
	if spec := getApp(t, cl, "web").Spec; !core.ServiceBlocksAll(spec.IPAllowListEntries) || !slices.Equal(spec.EffectiveIPAllowListCIDRs(), []string{core.DenyAllCIDR}) {
		t.Fatalf("after ipAllowList: [] the App enforces %v, want the deny-all range", spec.EffectiveIPAllowListCIDRs())
	}
	if got, present := read(); !present || len(got) != 0 {
		t.Errorf("a blocked service reads ipAllowList %v (present %v), want []", got, present)
	}

	// A write that does not name the list leaves the service blocked.
	restOK(t, mux, "PATCH", "/v1/services/web", `{"serviceDetails":{"healthCheckPath":"/healthz"}}`)
	if !core.ServiceBlocksAll(getApp(t, cl, "web").Spec.IPAllowListEntries) {
		t.Fatal("a PATCH without ipAllowList opened the blocked service")
	}

	// Render's default rule opens it. No rule is saved, so no middleware
	// refuses IPv6 sources, and the read is the allow-all pair.
	restOK(t, mux, "PATCH", "/v1/services/web", `{"serviceDetails":{"ipAllowList":[{"cidrBlock":"0.0.0.0/0","description":"everywhere"}]}}`)
	if spec := getApp(t, cl, "web").Spec; len(spec.EffectiveIPAllowListEntries()) != 0 {
		t.Fatalf("the allow-all rule saved %v, want no rule", spec.EffectiveIPAllowListEntries())
	}
	if got, present := read(); !present || !slices.Equal(got, allowAllCIDRs) {
		t.Errorf("an open service reads ipAllowList %v (present %v), want %v", got, present, allowAllCIDRs)
	}
}

func TestRESTCreateReadsTheThreeIPAllowListStates(t *testing.T) {
	for _, tc := range []struct {
		name, details string
		enforced      []string // what the operator is given
		reads         []string
	}{
		{"omitted", `{}`, nil, allowAllCIDRs},
		{"empty", `{"ipAllowList":[]}`, []string{core.DenyAllCIDR}, []string{}},
		{"allow-all", `{"ipAllowList":[{"cidrBlock":"0.0.0.0/0","description":"everywhere"}]}`, nil, allowAllCIDRs},
		{"a rule", `{"ipAllowList":[{"cidrBlock":"203.0.113.0/24","description":"office"}]}`, []string{"203.0.113.0/24"}, []string{"203.0.113.0/24"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(nil)
			raw := http.NewServeMux()
			svc.RegisterREST(raw)
			mux := validated(t, raw)
			restOK(t, mux, "POST", "/v1/services", `{"name":"web","image":{"imagePath":"nginx:v1"},"serviceDetails":`+tc.details+`}`)
			if got := getApp(t, cl, "web").Spec.EffectiveIPAllowListCIDRs(); !slices.Equal(got, tc.enforced) {
				t.Errorf("enforced CIDRs = %v, want %v", got, tc.enforced)
			}
			got, present := restAllowList(t, restOK(t, mux, "GET", "/v1/services/web", ""))
			if !present || !slices.Equal(got, tc.reads) {
				t.Errorf("GET reads %v (present %v), want %v", got, present, tc.reads)
			}
		})
	}
}

// A type with no public endpoint has no rule: an empty list is accepted, saves
// nothing and reads nothing, on any plan.
func TestEmptyIPAllowListOnAPrivateServiceSavesNothing(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})
	cl := fakeClient()
	svc := &Service{
		Base:  &core.Base{Client: cl, Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
		Store: &recordingStore{},
	}
	withWorkspacePlan(svc, "hobby")
	v, err := svc.Create(ctx, CreateRequest{Name: "internal", Type: "private_service", Image: "nginx:1.27", Port: 80, IPAllowList: []core.IPAllowListEntry{}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if v.IPAllowList != nil {
		t.Errorf("a private service reads ipAllowList %#v, want none", v.IPAllowList)
	}
	var list appv1alpha1.AppList
	if err := cl.List(ctx, &list); err != nil || len(list.Items) != 1 || len(list.Items[0].Spec.EffectiveIPAllowListEntries()) != 0 {
		t.Fatalf("apps=%d err=%v; want one App with no rule", len(list.Items), err)
	}
}

// Blocking is a restriction, so it needs the plan that rules need; leaving the
// list out, or sending the allow-all rule, never does.
func TestEmptyIPAllowListNeedsTheScalePlanOnCreate(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})
	for _, tc := range []struct {
		name    string
		plan    string
		list    []core.IPAllowListEntry
		refused bool
	}{
		{"hobby, omitted", "hobby", nil, false},
		{"hobby, allow-all", "hobby", core.UnrestrictedAllowList(), false},
		{"hobby, empty", "hobby", []core.IPAllowListEntry{}, true},
		{"scale, empty", "scale", []core.IPAllowListEntry{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &Service{
				Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}},
				Store: &recordingStore{},
			}
			withWorkspacePlan(svc, tc.plan)
			_, err := svc.Create(ctx, CreateRequest{Name: "web", Type: "web_service", Image: "nginx:1.27", Port: 80, IPAllowList: tc.list})
			if tc.refused {
				if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), core.InboundIPRulesMessage) {
					t.Fatalf("want the Scale-plan refusal, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("unexpected refusal: %v", err)
			}
		})
	}
}

func TestGraphQLEmptyIPAllowListBlocksAService(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	entries := func(query, field string) []any {
		t.Helper()
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: query})
		if len(res.Errors) > 0 {
			t.Fatalf("%s: %v", query, res.Errors)
		}
		list, ok := res.Data.(map[string]any)[field].(map[string]any)["ipAllowListEntries"].([]any)
		if !ok {
			t.Fatalf("%s: ipAllowListEntries is not a list: %v", query, res.Data)
		}
		return list
	}

	// Untouched: the allow-all pair.
	if got := entries(`{ service(id: "web") { ipAllowListEntries { cidrBlock } } }`, "service"); len(got) != 2 {
		t.Errorf("an untouched service reads %v, want the allow-all pair", got)
	}
	// A call that names neither list is refused: it is not an empty list, and
	// it must neither block the service nor open it.
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setServiceIpAllowList(id: "web") { id } }`})
	if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Message, "needs entries or cidrs") {
		t.Errorf("setServiceIpAllowList without a list = %v, want a refusal", res.Errors)
	}
	if len(getApp(t, cl, "web").Spec.EffectiveIPAllowListEntries()) != 0 {
		t.Error("a call without a list changed the service")
	}
	// Both spellings of an empty list block.
	for _, arg := range []string{"entries: []", "cidrs: []"} {
		if got := entries(`mutation { setServiceIpAllowList(id: "web", `+arg+`) { ipAllowListEntries { cidrBlock } } }`, "setServiceIpAllowList"); len(got) != 0 {
			t.Errorf("%s reads %v, want []", arg, got)
		}
		if !core.ServiceBlocksAll(getApp(t, cl, "web").Spec.IPAllowListEntries) {
			t.Errorf("%s did not block the service", arg)
		}
	}
}

// The CIDR-only GraphQL field has no list for a type with no public endpoint.
// It used to read [] there, which now means blocked.
func TestGraphQLIPAllowListIsNullWithoutAPublicEndpoint(t *testing.T) {
	worker := sampleApp("worker")
	worker.Spec.Type = appv1alpha1.TypeBackgroundWorker
	svc, _ := newService(nil, worker)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ service(id: "worker") { ipAllowList ipAllowListEntries { cidrBlock } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("query: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["service"].(map[string]any)
	if got["ipAllowList"] != nil || got["ipAllowListEntries"] != nil {
		t.Errorf("a worker reads ipAllowList %v and entries %v, want null for both", got["ipAllowList"], got["ipAllowListEntries"])
	}
}

func TestBlueprintEmptyIPAllowListBlocksAServiceAndExportsAsEmpty(t *testing.T) {
	ctx := context.Background()
	svc, cl := newService(nil)
	manifest := func(list string) string {
		return "services:\n  - type: web\n    name: web\n    runtime: image\n    image:\n      url: docker.io/library/nginx:1.27\n" + list
	}
	enforced := func() []string {
		t.Helper()
		return getApp(t, cl, "web").Spec.EffectiveIPAllowListCIDRs()
	}
	apply := func(list string) {
		t.Helper()
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest(list)}); err != nil {
			t.Fatalf("apply %q: %v", list, err)
		}
	}

	apply("    ipAllowList:\n      - source: 203.0.113.0/24\n        description: office\n")
	if got := enforced(); !slices.Equal(got, []string{"203.0.113.0/24"}) {
		t.Fatalf("rules = %v", got)
	}
	// "To block all external connections, set ipAllowList to an empty list".
	apply("    ipAllowList: []\n")
	if got := enforced(); !slices.Equal(got, []string{core.DenyAllCIDR}) {
		t.Fatalf("ipAllowList: [] enforces %v, want the deny-all range", got)
	}
	// "Render retains the current IP ranges for an existing service."
	apply("")
	if got := enforced(); !slices.Equal(got, []string{core.DenyAllCIDR}) {
		t.Fatalf("a manifest without ipAllowList changed the blocked service: %v", got)
	}

	// The export of a blocked service says so, and applying it blocks again.
	out, err := svc.GenerateBlueprint(ctx, GenerateBlueprintRequest{ServiceIDs: []string{"web"}})
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	if !strings.Contains(out.Manifest, "ipAllowList: []") {
		t.Errorf("the export of a blocked service has no `ipAllowList: []`:\n%s", out.Manifest)
	}
	if strings.Contains(out.Manifest, core.DenyAllCIDR) {
		t.Errorf("the export leaks the deny-all placeholder:\n%s", out.Manifest)
	}
}
