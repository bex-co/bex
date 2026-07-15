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

package projects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeProjectStore struct {
	projects map[string]store.Project
	services map[string][]string
}

func newFakeProjectStore(projects ...store.Project) *fakeProjectStore {
	f := &fakeProjectStore{projects: map[string]store.Project{}, services: map[string][]string{}}
	for _, p := range projects {
		f.projects[p.ID] = p
	}
	return f
}

func (f *fakeProjectStore) CreateProject(_ context.Context, tenantID, name string) (store.Project, error) {
	p := store.Project{ID: "prj-created", TenantID: tenantID, Name: name}
	f.projects[p.ID] = p
	return p, nil
}

func (f *fakeProjectStore) GetProject(_ context.Context, projectID string) (store.Project, error) {
	p, ok := f.projects[projectID]
	if !ok {
		return store.Project{}, fmt.Errorf("project: %w", store.ErrNotFound)
	}
	return p, nil
}

func (f *fakeProjectStore) ListProjects(_ context.Context, tenantID string) ([]store.Project, error) {
	var out []store.Project
	for _, p := range f.projects {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeProjectStore) RenameProject(_ context.Context, projectID, name string) error {
	p, ok := f.projects[projectID]
	if !ok {
		return fmt.Errorf("project: %w", store.ErrNotFound)
	}
	p.Name = name
	f.projects[projectID] = p
	return nil
}

func (f *fakeProjectStore) DeleteProject(_ context.Context, projectID string) error {
	if _, ok := f.projects[projectID]; !ok {
		return fmt.Errorf("project: %w", store.ErrNotFound)
	}
	delete(f.projects, projectID)
	delete(f.services, projectID)
	return nil
}

func (f *fakeProjectStore) SetProjectServices(_ context.Context, projectID, _ string, serviceNames []string) error {
	f.services[projectID] = append([]string(nil), serviceNames...)
	return nil
}

func (f *fakeProjectStore) ListProjectServices(_ context.Context, projectID string) ([]string, error) {
	return append([]string(nil), f.services[projectID]...), nil
}

type allowChecker struct{}

func (allowChecker) Check(context.Context, string, string, string) (bool, error) { return true, nil }

type denyObjectChecker string

func (d denyObjectChecker) Check(_ context.Context, _, _, object string) (bool, error) {
	return object != string(d), nil
}

func ctxAs(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: subject, Method: "session"})
}

func newMCPClient(t *testing.T, ctx context.Context, svc *Service) *mcp.ClientSession {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestGetCrossTenantIsForbiddenNotNotFound(t *testing.T) {
	st := newFakeProjectStore(store.Project{ID: "prj-other", TenantID: "tea-other", Name: "other"})
	svc := &Service{
		Base:  &core.Base{Authz: denyObjectChecker(core.WorkspaceObject("tea-other"))},
		Store: st,
	}

	_, err := svc.Get(ctxAs("user-a"), "prj-other")
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("cross-tenant Get: got %v, want ErrForbidden", err)
	}
	if errors.Is(err, core.ErrNotFound) {
		t.Fatalf("cross-tenant Get leaked nonexistence semantics: %v", err)
	}
}

func TestGetNonexistentIsNotFound(t *testing.T) {
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}, Store: newFakeProjectStore()}
	_, err := svc.Get(ctxAs("user-a"), "prj-missing")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("nonexistent Get: got %v, want ErrNotFound", err)
	}
}

func TestCrossTenantGetIsForbiddenAcrossAdapters(t *testing.T) {
	st := newFakeProjectStore(store.Project{ID: "prj-other", TenantID: "tea-other", Name: "other"})
	svc := &Service{
		Base:  &core.Base{Authz: denyObjectChecker(core.WorkspaceObject("tea-other"))},
		Store: st,
	}
	ctx := ctxAs("user-a")

	t.Run("REST", func(t *testing.T) {
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/projects/prj-other", nil)
		mux.ServeHTTP(rec, req.WithContext(ctx))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403 (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		})
		if err != nil {
			t.Fatalf("schema: %v", err)
		}
		result := graphql.Do(graphql.Params{
			Schema:        schema,
			Context:       ctx,
			RequestString: `{ project(id: "prj-other") { id } }`,
		})
		if len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Message, core.ErrForbidden.Error()) {
			t.Fatalf("GraphQL errors = %#v, want forbidden resolver error", result.Errors)
		}
	})

	t.Run("MCP", func(t *testing.T) {
		client := newMCPClient(t, ctx, svc)
		result, err := client.CallTool(ctx, &mcp.CallToolParams{
			Name:      "get_project",
			Arguments: map[string]any{"id": "prj-other"},
		})
		if err != nil {
			t.Fatalf("transport error: %v", err)
		}
		raw, _ := json.Marshal(result.Content)
		if !result.IsError || !strings.Contains(string(raw), core.ErrForbidden.Error()) {
			t.Fatalf("MCP result = %#v content=%s, want forbidden tool error", result, raw)
		}
	})
}

func TestUnavailableStoreAcrossAdapters(t *testing.T) {
	svc := &Service{Base: &core.Base{Authz: allowChecker{}}}

	t.Run("REST is 503 with Render error envelope", func(t *testing.T) {
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/projects/prj-missing", nil)
		mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503 (body: %s)", rec.Code, rec.Body.String())
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode REST body: %v", err)
		}
		if body["id"] != "unavailable" || body["message"] != ErrProjectsUnavailable.Error() {
			t.Fatalf("REST body = %#v, want Render unavailable envelope", body)
		}
	})

	t.Run("GraphQL is a resolver error", func(t *testing.T) {
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		})
		if err != nil {
			t.Fatalf("schema: %v", err)
		}
		result := graphql.Do(graphql.Params{
			Schema:        schema,
			Context:       ctxAs("user-a"),
			RequestString: `{ project(id: "prj-missing") { id } }`,
		})
		if len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Message, ErrProjectsUnavailable.Error()) {
			t.Fatalf("GraphQL errors = %#v, want projects-unavailable resolver error", result.Errors)
		}
	})

	t.Run("MCP is a tool error", func(t *testing.T) {
		ctx := ctxAs("user-a")
		client := newMCPClient(t, ctx, svc)

		result, err := client.CallTool(ctx, &mcp.CallToolParams{
			Name:      "get_project",
			Arguments: map[string]any{"id": "prj-missing"},
		})
		if err != nil {
			t.Fatalf("transport error: %v", err)
		}
		if !result.IsError {
			t.Fatalf("MCP result = %#v, want tool error", result)
		}
		raw, _ := json.Marshal(result.Content)
		if !strings.Contains(string(raw), ErrProjectsUnavailable.Error()) {
			t.Fatalf("MCP error content = %s, want projects-unavailable message", raw)
		}
	})
}
