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

package workspaces

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// policyStore is fakeStore plus tenants.overlapping_deploy_policy.
type policyStore struct {
	*fakeStore
	mu       sync.Mutex
	policies map[string]string
}

func (s *policyStore) OverlappingDeployPolicy(_ context.Context, id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.policies[id]; ok {
		return p, nil
	}
	return store.OverlapWait, nil
}

func (s *policyStore) SetOverlappingDeployPolicy(_ context.Context, id, policy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies[id] = policy
	return nil
}

// w4/653: GraphQL reads Render's workspace Overlapping Deploy Policy, Wait by
// default, and an Admin sets it to Override and back; anything else is refused
// before it is written, and a member who cannot manage the workspace is denied.
func TestGraphQLWorkspaceOverlappingDeployPolicy(t *testing.T) {
	st := &policyStore{fakeStore: newFakeStore(), policies: map[string]string{}}
	checker := &fakeChecker{allow: true}
	base := &core.Base{Authz: checker, Audit: &recordingAudit{}}
	svc := &Service{Base: base, Store: st, Granter: &fakeGranter{}}
	base.Workspace = &fakeResolver{store: st}
	w, err := svc.Create(ctxAs("admin"), "acme", "pro")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	run := func(q string, vars map[string]any) (string, []error) {
		res := graphql.Do(graphql.Params{Schema: schema, RequestString: q, VariableValues: vars, Context: ctxAs("admin")})
		b, _ := json.Marshal(res.Data)
		var errs []error
		for _, e := range res.Errors {
			errs = append(errs, e)
		}
		return string(b), errs
	}
	read := `query($id: String!) { workspace(id: $id) { id overlappingDeployPolicy } }`
	set := `mutation($id: String!, $p: String!) { setWorkspaceOverlappingDeployPolicy(id: $id, policy: $p) { id overlappingDeployPolicy } }`

	if got, errs := run(read, map[string]any{"id": w.ID}); len(errs) > 0 || got != `{"workspace":{"id":"`+w.ID+`","overlappingDeployPolicy":"wait"}}` {
		t.Fatalf("default read = %s (errors %v), want wait", got, errs)
	}
	if got, errs := run(set, map[string]any{"id": w.ID, "p": "override"}); len(errs) > 0 ||
		got != `{"setWorkspaceOverlappingDeployPolicy":{"id":"`+w.ID+`","overlappingDeployPolicy":"override"}}` {
		t.Fatalf("set override = %s (errors %v)", got, errs)
	}
	if got, _ := run(read, map[string]any{"id": w.ID}); got != `{"workspace":{"id":"`+w.ID+`","overlappingDeployPolicy":"override"}}` {
		t.Fatalf("read after set = %s, want override", got)
	}
	if _, errs := run(set, map[string]any{"id": w.ID, "p": "newest"}); len(errs) != 1 || st.policies[w.ID] != store.OverlapOverride {
		t.Fatalf("an unknown policy: errors %v, stored %q; want refused and nothing written", errs, st.policies[w.ID])
	}

	checker.allow = false
	if _, err := svc.SetOverlappingDeployPolicy(ctxAs("member"), w.ID, store.OverlapWait); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("a member's set: err = %v, want forbidden", err)
	}
	if st.policies[w.ID] != store.OverlapOverride {
		t.Fatalf("a denied set wrote %q", st.policies[w.ID])
	}
}
