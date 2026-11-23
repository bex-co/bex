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

package deploys

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
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func buildSourceScopeFixture(t *testing.T) (*Service, *fakeStore, string, string, string, string, string) {
	t.Helper()
	const tenant = "tea-scope"
	cr := func(name string) string { return store.CRName(tenant, name) }
	ds := newFakeStore()
	ds.mintIDs = true
	bdsID := ids.New(ids.BuildSource)
	autoID := ids.New(ids.Service)
	manualID := ids.New(ids.Service)
	suspID := ids.New(ids.Service)
	unlinkedID := ids.New(ids.Service)

	carrier := sampleBuildSource(bdsID, "https://github.com/acme/mono")
	auto := linkedService(cr("auto"), autoID, bdsID, "registry.local/mono:old", true)
	manual := linkedService(cr("manual"), manualID, bdsID, "registry.local/mono:old", false)
	susp := linkedService(cr("susp"), suspID, bdsID, "registry.local/mono:old", true)
	susp.Spec.Suspended = true
	unlinked := sampleApp(cr("unlinked"), unlinkedID)

	ds.links = []store.App{
		{ID: autoID, TenantID: tenant, Name: "auto", BuildSourceID: bdsID, BuildSourceImage: "registry.local/mono:old"},
		{ID: manualID, TenantID: tenant, Name: "manual", BuildSourceID: bdsID, BuildSourceImage: "registry.local/mono:old"},
		{ID: suspID, TenantID: tenant, Name: "susp", BuildSourceID: bdsID, BuildSourceImage: "registry.local/mono:old"},
	}

	svc, _ := newService(ds, carrier, auto, manual, susp, unlinked)
	return svc, ds, bdsID, autoID, manualID, suspID, unlinkedID
}

func TestTriggerBuildDeployScopesRESTGraphQLAndMCP(t *testing.T) {
	svc, ds, bdsID, autoID, manualID, suspID, unlinkedID := buildSourceScopeFixture(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	post := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/build-sources/"+bdsID+"/builds", strings.NewReader(body))
		mux.ServeHTTP(rec, req)
		return rec
	}

	// 1. Omitted deploy -> autodeploy_only default: deploys autoID, silently skips suspended and manual.
	rec := post(`{}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("default trigger = %d %s", rec.Code, rec.Body.String())
	}
	var res TriggerBuildResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Deploys) != 1 || res.Deploys[0].ServiceID != autoID || !strings.HasPrefix(res.Deploys[0].DeployID, "dep-") {
		t.Fatalf("autodeploy_only deploys = %+v, want [%s]", res.Deploys, autoID)
	}
	if len(res.SkippedServices) != 0 {
		t.Fatalf("autodeploy_only skippedServices = %+v, want []", res.SkippedServices)
	}
	// Awaiting-build row exists in store and promotes in place when BuildSucceeded runs.
	awaiting, err := ds.ListBuildAwaitingDeploys(context.Background(), res.ID)
	if err != nil || len(awaiting) != 1 || awaiting[0].ID != res.Deploys[0].DeployID || !awaiting[0].AwaitingBuild {
		t.Fatalf("awaiting rows = %+v, %v", awaiting, err)
	}
	svc.BuildSucceeded(context.Background(), store.BuiltSource{
		TenantID:         "tea-scope",
		CarrierID:        bdsID,
		DeployID:         res.BuildRun.ID,
		Namespace:        "default",
		Carrier:          bdsID,
		Image:            "registry.local/mono:sha1",
		BuildDeployScope: store.BuildDeployScopeAutoDeployOnly,
	})
	if d, err := ds.GetDeploy(context.Background(), autoID, res.Deploys[0].DeployID); err != nil || d.AwaitingBuild || d.BuildID != res.ID {
		t.Fatalf("promoted deploy = %+v, %v", d, err)
	}

	// 2. serviceScope=none: no awaiting deploys and BuildSucceeded deploys nothing.
	rec = post(`{"deploy":{"serviceScope":"none"}}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("none trigger = %d %s", rec.Code, rec.Body.String())
	}
	res = TriggerBuildResult{}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Deploys) != 0 || len(res.SkippedServices) != 0 {
		t.Fatalf("none result = %+v, want empty deploys and skippedServices", res)
	}
	beforeManual := len(ds.byApp[manualID])
	svc.BuildSucceeded(context.Background(), store.BuiltSource{
		TenantID:         "tea-scope",
		CarrierID:        bdsID,
		DeployID:         res.BuildRun.ID,
		Namespace:        "default",
		Carrier:          bdsID,
		Image:            "registry.local/mono:sha2",
		BuildDeployScope: store.BuildDeployScopeNone,
	})
	if len(ds.byApp[manualID]) != beforeManual {
		t.Fatalf("none scope unexpectedly deployed manual service")
	}

	// 3. serviceScope=custom: deploys manualID (even with autoDeploy=false) and reports suspID as skipped(suspended).
	rec = post(`{"deploy":{"serviceScope":"custom","serviceIds":["` + manualID + `","` + suspID + `"]}}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("custom trigger = %d %s", rec.Code, rec.Body.String())
	}
	res = TriggerBuildResult{}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Deploys) != 1 || res.Deploys[0].ServiceID != manualID {
		t.Fatalf("custom deploys = %+v, want [%s]", res.Deploys, manualID)
	}
	if len(res.SkippedServices) != 1 || res.SkippedServices[0].ServiceID != suspID || res.SkippedServices[0].Reason != SkipReasonSuspended {
		t.Fatalf("custom skippedServices = %+v, want [{%s suspended}]", res.SkippedServices, suspID)
	}

	// 4. Canceling a pre-created awaiting-build deploy closes it cleanly.
	if _, err := svc.Cancel(context.Background(), manualID, res.Deploys[0].DeployID); err != nil {
		t.Fatalf("Cancel awaiting deploy: %v", err)
	}
	if d, err := ds.GetDeploy(context.Background(), manualID, res.Deploys[0].DeployID); err != nil || d.Status != store.DeployCanceled || d.AwaitingBuild {
		t.Fatalf("after cancel, planned deploy = %+v, %v; want canceled and not awaiting", d, err)
	}

	// 5. Invalid deploy shapes -> 400 (and nonexistent service -> 404).
	for _, body := range []string{
		`{"deploy":{"serviceScope":"custom"}}`,
		`{"deploy":{"serviceScope":"custom","serviceIds":[]}}`,
		`{"deploy":{"serviceScope":"none","serviceIds":["` + autoID + `"]}}`,
		`{"deploy":{"serviceScope":"autodeploy_only","serviceIds":["` + autoID + `"]}}`,
		`{"deploy":{"serviceScope":"bogus"}}`,
		`{"deploy":{"serviceScope":"custom","serviceIds":["` + unlinkedID + `"]}}`,
	} {
		if r := post(body); r.Code != http.StatusBadRequest {
			t.Errorf("POST %s = %d %s, want 400", body, r.Code, r.Body.String())
		}
	}
	if r := post(`{"deploy":{"serviceScope":"custom","serviceIds":["srv-missing"]}}`); r.Code != http.StatusNotFound {
		t.Errorf("POST nonexistent custom service = %d %s, want 404", r.Code, r.Body.String())
	}

	// 6. GraphQL triggerBuild with deploy scope returns deploys & skippedServices.
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	gqlRes := graphql.Do(graphql.Params{
		Schema:  schema,
		Context: context.Background(),
		RequestString: `mutation { triggerBuild(buildSourceId: "` + bdsID + `", deploy: { serviceScope: "custom", serviceIds: ["` + manualID + `", "` + suspID + `"] }) {
			id deploys { serviceId deployId } skippedServices { serviceId reason }
		} }`,
	})
	if len(gqlRes.Errors) > 0 {
		t.Fatalf("GraphQL triggerBuild errors = %v", gqlRes.Errors)
	}
	raw, _ := json.Marshal(gqlRes.Data)
	if !strings.Contains(string(raw), manualID) || !strings.Contains(string(raw), `"reason":"suspended"`) {
		t.Fatalf("GraphQL triggerBuild = %s", raw)
	}

	// 7. MCP trigger_build_source_build with deploy scope returns deploys & skippedServices.
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	c1, c2 := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(context.Background(), c1, nil); err != nil {
		t.Fatal(err)
	}
	cli := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := cli.Connect(context.Background(), c2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	out, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "trigger_build_source_build",
		Arguments: map[string]any{
			"buildSourceId": bdsID,
			"deploy": map[string]any{
				"serviceScope": "custom",
				"serviceIds":   []string{manualID, suspID},
			},
		},
	})
	if err != nil || out.IsError {
		t.Fatalf("MCP trigger_build_source_build = err %v, out %+v", err, out)
	}
	var mcpText string
	for _, c := range out.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			mcpText += tc.Text
		}
	}
	if !strings.Contains(mcpText, manualID) || !strings.Contains(mcpText, `"reason":"suspended"`) {
		t.Fatalf("MCP trigger_build_source_build text = %s", mcpText)
	}
}

type denyOneAppChecker struct {
	deny string
}

func (c denyOneAppChecker) Check(_ context.Context, _, _, obj string) (bool, error) {
	return !strings.Contains(obj, c.deny), nil
}

func TestTriggerBuildCustomScopeRefusesUnauthorizedService(t *testing.T) {
	svc, _, bdsID, autoID, _, _, _ := buildSourceScopeFixture(t)
	svc.Base.Authz = denyOneAppChecker{deny: autoID}
	ids := []string{autoID}
	_, err := svc.TriggerBuild(context.Background(), bdsID, "", &BuildDeployScope{
		ServiceScope: store.BuildDeployScopeCustom,
		ServiceIDs:   &ids,
	})
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("unauthorized custom target = %v, want ErrForbidden", err)
	}
}
