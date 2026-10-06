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

package envgroups

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// mcpSession connects an in-memory MCP client to svc's tools.
func mcpSession(t *testing.T, svc *Service) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	srv := mcp.NewServer(&mcp.Implementation{Name: "bex", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// callTool calls a tool that must succeed, decoding its structured result
// into out.
func callTool(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any, out any) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil || res.IsError {
		t.Fatalf("call %s: err=%v isErr=%v", name, err, res != nil && res.IsError)
	}
	if out != nil {
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, out); err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
	}
}

// failingAppLists fails every App list while fail is set: the API server
// refusing the list that names a group's linked services.
type failingAppLists struct {
	client.Client
	fail bool
}

func (c *failingAppLists) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, apps := list.(*appv1alpha1.AppList); apps && c.fail {
		return errors.New("the API server is unavailable")
	}
	return c.Client.List(ctx, list, opts...)
}

// linkedFixture is a group linked to a service of each type, one renamed to a
// display name, and to two services deleted outside the API: one gone, one
// held by a finalizer. want is the serviceLinks Render's envGroup carries.
type linkedFixture struct {
	svc    *Service
	cl     *failingAppLists
	group  EnvGroupView
	want   []serviceLink
	stored []string
}

func newLinkedFixture(t *testing.T) linkedFixture {
	t.Helper()
	ctx := context.Background()
	services := []struct{ name, displayName, specType, short string }{
		{"site", "", appv1alpha1.TypeStaticSite, "static"},
		{"api", "Public API", appv1alpha1.TypeWebService, "web"},
		{"legacy", "", "", "web"},
		{"internal", "", appv1alpha1.TypePrivateService, "pserv"},
		{"jobs", "", appv1alpha1.TypeBackgroundWorker, "worker"},
		{"nightly", "", appv1alpha1.TypeCronJob, "cron"},
	}
	var f linkedFixture
	var objs []client.Object
	for _, s := range services {
		serviceID := id.New(id.Service)
		a := apiApp(s.name, serviceID)
		a.Name = "app-" + s.name // the object name is not the service's name
		a.Spec.DisplayName, a.Spec.Type = s.displayName, s.specType
		objs = append(objs, a)
		name := s.name
		if s.displayName != "" {
			name = s.displayName
		}
		f.want = append(f.want, serviceLink{ID: serviceID, Name: name, Type: s.short})
		f.stored = append(f.stored, serviceID)
	}
	gone, deleting := apiApp("gone", id.New(id.Service)), apiApp("deleting", id.New(id.Service))
	deleting.Finalizers = []string{"app.bex.co/finalizer"}
	objs = append(objs, gone, deleting)
	f.stored = append(f.stored, core.AppPublicID(gone), core.AppPublicID(deleting))

	f.cl = &failingAppLists{Client: fakeClient(objs...)}
	f.svc = newService(newFakeStore())
	f.svc.Client = f.cl
	var err error
	if f.group, err = f.svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared"}); err != nil {
		t.Fatal(err)
	}
	for _, serviceID := range f.stored {
		if err := f.svc.LinkService(ctx, f.group.ID, serviceID); err != nil {
			t.Fatalf("link %s: %v", serviceID, err)
		}
	}
	for _, a := range []*appv1alpha1.App{gone, deleting} {
		if err := f.cl.Delete(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

var renderSpec = sync.OnceValues(func() (*openapi3.T, error) {
	return openapi3.NewLoader().LoadFromFile("../api/openapi/render-public-api-1.json")
})

// expectRenderLinks checks raw is serviceLinks as Render's envGroup carries
// them: envGroupLink objects, equal to want.
func expectRenderLinks(t *testing.T, raw any, want []serviceLink) {
	t.Helper()
	spec, err := renderSpec()
	if err != nil {
		t.Fatal(err)
	}
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("serviceLinks = %#v, want an array", raw)
	}
	for _, item := range items {
		if err := spec.Components.Schemas["envGroupLink"].Value.VisitJSON(item); err != nil {
			t.Fatalf("link %v is not Render's envGroupLink: %v", item, err)
		}
	}
	var got []serviceLink
	encoded, _ := json.Marshal(items)
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("serviceLinks = %+v, want %+v", got, want)
	}
}

// TestServiceLinksAreRenderEnvGroupLinks (w5/092): REST and MCP answer an env
// group's serviceLinks as Render's envGroupLink objects, which Render's own
// CLI decodes: each linked service's id, its displayed name, and its short
// type. A link whose service is gone or being deleted is left out. GraphQL
// keeps the stored ids the dashboard reads.
func TestServiceLinksAreRenderEnvGroupLinks(t *testing.T) {
	f := newLinkedFixture(t)
	path := "/v1/env-groups/" + f.group.ID
	for _, tc := range []struct {
		method, path, body string
		status             int
		envelope           bool // a list of {envGroup, cursor}
		want               []serviceLink
	}{
		{method: http.MethodGet, path: path, status: http.StatusOK, want: f.want},
		{method: http.MethodGet, path: "/v1/env-groups", status: http.StatusOK, envelope: true, want: f.want},
		{method: http.MethodPatch, path: path, body: `{"name":"renamed"}`, status: http.StatusOK, want: f.want},
		{method: http.MethodPatch, path: path + "/environment", body: `{"environmentId":""}`, status: http.StatusOK, want: f.want},
		{method: http.MethodPost, path: path + "/clone", body: `{"name":"copy"}`, status: http.StatusCreated, want: []serviceLink{}},
		{method: http.MethodPost, path: "/v1/env-groups", body: `{"name":"fresh","serviceIds":["` + f.stored[0] + `"]}`, status: http.StatusCreated, want: f.want[:1]},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			res := serveREST(f.svc, tc.method, tc.path, tc.body)
			if res.Code != tc.status {
				t.Fatalf("%s %s = %d %s", tc.method, tc.path, res.Code, res.Body)
			}
			var group map[string]any
			if tc.envelope {
				var page []struct {
					EnvGroup map[string]any `json:"envGroup"`
				}
				if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil || len(page) == 0 {
					t.Fatalf("list = %s (%v)", res.Body, err)
				}
				group = page[0].EnvGroup
			} else if err := json.Unmarshal(res.Body.Bytes(), &group); err != nil {
				t.Fatal(err)
			}
			expectRenderLinks(t, group["serviceLinks"], tc.want)
		})
	}

	t.Run("MCP", func(t *testing.T) {
		cs := mcpSession(t, f.svc)
		var got map[string]any
		callTool(t, cs, "get_env_group", map[string]any{"id": f.group.ID}, &got)
		expectRenderLinks(t, got["serviceLinks"], f.want)
		var listed struct {
			EnvGroups []map[string]any `json:"envGroups"`
		}
		callTool(t, cs, "list_env_groups", map[string]any{}, &listed)
		i := slices.IndexFunc(listed.EnvGroups, func(g map[string]any) bool { return g["id"] == f.group.ID })
		if i < 0 {
			t.Fatalf("list_env_groups lost the group: %+v", listed)
		}
		expectRenderLinks(t, listed.EnvGroups[i]["serviceLinks"], f.want)
	})

	t.Run("GraphQL keeps the stored ids", func(t *testing.T) {
		res := graphql.Do(graphql.Params{Schema: *envGroupSchema(t, f.svc), Context: context.Background(),
			RequestString:  `query($id: String!) { envGroup(id: $id) { serviceLinks } }`,
			VariableValues: map[string]any{"id": f.group.ID}})
		if len(res.Errors) > 0 {
			t.Fatalf("graphql: %v", res.Errors)
		}
		var got struct {
			EnvGroup struct {
				ServiceLinks []string `json:"serviceLinks"`
			} `json:"envGroup"`
		}
		encoded, _ := json.Marshal(res.Data)
		if err := json.Unmarshal(encoded, &got); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got.EnvGroup.ServiceLinks, f.stored) {
			t.Fatalf("GraphQL serviceLinks = %v, want the stored ids %v", got.EnvGroup.ServiceLinks, f.stored)
		}
	})
}

// TestNamingServiceLinksFailsOnlyAReadAfterAFailedList (w5/092): naming a
// group's links lists the workspace's services. A read whose list fails
// fails, and can be retried. A write has already committed by then, so it
// answers its links empty rather than report a failure a retry cannot repeat.
func TestNamingServiceLinksFailsOnlyAReadAfterAFailedList(t *testing.T) {
	f := newLinkedFixture(t)
	f.cl.fail = true
	path := "/v1/env-groups/" + f.group.ID

	if res := serveREST(f.svc, http.MethodGet, path, ""); res.Code < http.StatusInternalServerError {
		t.Fatalf("GET with the list failing = %d %s, want a server error", res.Code, res.Body)
	}
	res := serveREST(f.svc, http.MethodPatch, path, `{"name":"renamed"}`)
	if res.Code != http.StatusOK {
		t.Fatalf("rename with the list failing = %d %s, want 200", res.Code, res.Body)
	}
	var group map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &group); err != nil {
		t.Fatal(err)
	}
	expectRenderLinks(t, group["serviceLinks"], []serviceLink{})
	if got, err := f.svc.GetEnvGroup(context.Background(), f.group.ID); err != nil || got.Name != "renamed" {
		t.Fatalf("the rename did not commit: %+v, %v", got, err)
	}
}

// TestAnUnlinkedGroupRendersWithoutListingServices (w5/092): only a linked
// group names services, so a group with no links is answered without the list
// that names them.
func TestAnUnlinkedGroupRendersWithoutListingServices(t *testing.T) {
	cl := &failingAppLists{Client: fakeClient()}
	svc := newService(newFakeStore())
	svc.Client = cl
	group, err := svc.CreateEnvGroup(context.Background(), CreateEnvGroupRequest{Name: "alone"})
	if err != nil {
		t.Fatal(err)
	}
	cl.fail = true

	res := serveREST(svc, http.MethodGet, "/v1/env-groups/"+group.ID, "")
	if res.Code != http.StatusOK {
		t.Fatalf("GET of an unlinked group with the list failing = %d %s", res.Code, res.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	expectRenderLinks(t, body["serviceLinks"], []serviceLink{})
}
