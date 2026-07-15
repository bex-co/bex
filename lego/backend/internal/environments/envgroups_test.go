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
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeEnvGroupIndex struct {
	groups []envgroups.EnvGroupView
}

func (f *fakeEnvGroupIndex) ListEnvironmentMemberships(_ context.Context, ownerID string) ([]envgroups.EnvironmentMembership, error) {
	out := make([]envgroups.EnvironmentMembership, 0, len(f.groups))
	for _, g := range f.groups {
		if g.OwnerID == ownerID {
			out = append(out, envgroups.EnvironmentMembership{ID: g.ID, EnvironmentID: g.EnvironmentID})
		}
	}
	return out, nil
}

func (f *fakeEnvGroupIndex) SetEnvironmentID(_ context.Context, id, environmentID string) error {
	for i := range f.groups {
		if f.groups[i].ID == id {
			f.groups[i].EnvironmentID = environmentID
			return nil
		}
	}
	return core.ErrNotFound
}

func envGroupFixture(t *testing.T) (*Service, *fakeEnvGroupIndex, EnvironmentView) {
	t.Helper()
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-alpha", TenantID: "tea-alpha", Name: "alpha"})
	idx := &fakeEnvGroupIndex{groups: []envgroups.EnvGroupView{
		{ID: "evg-alpha", OwnerID: "tea-alpha"},
		{ID: "evg-alpha-old", OwnerID: "tea-alpha"},
		{ID: "evg-bravo", OwnerID: "tea-bravo"},
	}}
	svc := newService(st)
	svc.EnvGroups = idx
	e, err := svc.Create(ctxAs("user-a"), "prj-alpha", "production")
	if err != nil {
		t.Fatalf("Create environment: %v", err)
	}
	idx.groups[1].EnvironmentID = e.ID
	return svc, idx, e
}

func TestSetEnvGroups_ReplacesMembershipAndReadsBack(t *testing.T) {
	svc, idx, e := envGroupFixture(t)

	got, err := svc.Get(ctxAs("user-a"), e.ID)
	if err != nil || !slices.Equal(got.EnvGroupIDs, []string{"evg-alpha-old"}) {
		t.Fatalf("initial membership = %+v err=%v", got.EnvGroupIDs, err)
	}
	got, err = svc.SetEnvGroups(ctxAs("user-a"), e.ID, []string{"evg-alpha"})
	if err != nil {
		t.Fatalf("SetEnvGroups: %v", err)
	}
	if !slices.Equal(got.EnvGroupIDs, []string{"evg-alpha"}) {
		t.Fatalf("membership after replace = %+v, want [evg-alpha]", got.EnvGroupIDs)
	}
	if idx.groups[0].EnvironmentID != e.ID || idx.groups[1].EnvironmentID != "" {
		t.Fatalf("stored membership after replace: %+v", idx.groups)
	}

	if _, err := svc.SetEnvGroups(ctxAs("user-a"), e.ID, []string{"evg-bravo"}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("cross-workspace group: want ErrForbidden, got %v", err)
	}
	if idx.groups[0].EnvironmentID != e.ID || idx.groups[2].EnvironmentID != "" {
		t.Fatalf("refused link mutated membership: %+v", idx.groups)
	}

	got, err = svc.SetEnvGroups(ctxAs("user-a"), e.ID, nil)
	if err != nil || len(got.EnvGroupIDs) != 0 {
		t.Fatalf("clear membership = %+v err=%v", got.EnvGroupIDs, err)
	}
}

func TestDeleteEnvironmentClearsEnvGroupMembership(t *testing.T) {
	svc, idx, e := envGroupFixture(t)
	if err := svc.Delete(ctxAs("user-a"), e.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if idx.groups[1].EnvironmentID != "" {
		t.Fatalf("deleted environment left dangling membership %q", idx.groups[1].EnvironmentID)
	}
}

func TestEnvGroupMembershipSurfaces(t *testing.T) {
	t.Run("REST", func(t *testing.T) {
		svc, _, e := envGroupFixture(t)
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/v1/environments/"+e.ID+"/env-group-links", strings.NewReader(`{"envGroupIds":["evg-alpha"]}`))
		mux.ServeHTTP(rec, req.WithContext(ctxAs("user-a")))
		if rec.Code != http.StatusOK {
			t.Fatalf("PUT env-group-links: got %d: %s", rec.Code, rec.Body.String())
		}
		var got EnvironmentView
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got.EnvGroupIDs, []string{"evg-alpha"}) {
			t.Fatalf("REST envGroupIds = %+v", got.EnvGroupIDs)
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		svc, _, e := envGroupFixture(t)
		field := svc.GraphQLMutation()["setEnvironmentEnvGroups"]
		out, err := field.Resolve(graphql.ResolveParams{
			Context: ctxAs("user-a"),
			Args:    map[string]any{"id": e.ID, "envGroupIds": []any{"evg-alpha"}},
		})
		if err != nil {
			t.Fatalf("setEnvironmentEnvGroups: %v", err)
		}
		got := out.(EnvironmentView)
		if !slices.Equal(got.EnvGroupIDs, []string{"evg-alpha"}) {
			t.Fatalf("GraphQL envGroupIds = %+v", got.EnvGroupIDs)
		}
		if svc.GraphQLQuery()["environment"].Type == nil {
			t.Fatal("environment query type missing")
		}
	})

	t.Run("MCP", func(t *testing.T) {
		svc, _, e := envGroupFixture(t)
		// The in-memory MCP transport does not propagate caller context values;
		// disable the fake checker here so this subtest exercises the adapter and
		// shared service method rather than session-auth middleware.
		svc.Authz = nil
		srv := mcp.NewServer(&mcp.Implementation{Name: "bex", Version: "0"}, nil)
		svc.RegisterMCP(srv)
		serverT, clientT := mcp.NewInMemoryTransports()
		if _, err := srv.Connect(context.Background(), serverT, nil); err != nil {
			t.Fatalf("server connect: %v", err)
		}
		cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(context.Background(), clientT, nil)
		if err != nil {
			t.Fatalf("client connect: %v", err)
		}
		t.Cleanup(func() { _ = cs.Close() })
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "set_environment_env_groups",
			Arguments: map[string]any{"id": e.ID, "envGroupIds": []string{"evg-alpha"}},
		})
		if err != nil || res.IsError {
			t.Fatalf("set_environment_env_groups: err=%v result=%+v", err, res)
		}
		var got EnvironmentView
		b, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got.EnvGroupIDs, []string{"evg-alpha"}) {
			t.Fatalf("MCP envGroupIds = %+v", got.EnvGroupIDs)
		}
	})
}
