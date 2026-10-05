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
	"errors"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// memberOf is a caller whose default is the first workspace and who belongs
// to every listed one.
type memberOf []string

func (m memberOf) Tenant(context.Context, core.Identity) (string, bool) { return m[0], true }
func (m memberOf) IsMember(_ context.Context, _ core.Identity, tenantID string) (bool, error) {
	return slices.Contains(m, tenantID), nil
}

// w4/m172: an environment id resolves its own workspace, so the verb's leading
// check runs there — a member whose role lives only in that workspace is not
// refused by their default workspace — and a non-member's typed id is a 403
// while a missing one is a 404 (ADR072 #8, w4/199).
func TestEnvironmentByIDVerbsResolveTheOwningWorkspace(t *testing.T) {
	st := newFakeStore()
	env, err := st.CreateEnvironment(context.Background(), "prj-b", "tea-b", "staging")
	if err != nil {
		t.Fatal(err)
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	onlyB := denyObjectChecker(core.WorkspaceObject("tea-a")) // dana holds no role in her default
	member := &Service{Base: &core.Base{Authz: onlyB, Workspace: memberOf{"tea-a", "tea-b"}}, Store: st}
	if got, err := member.Get(ctx, env.ID); err != nil || got.ID == "" {
		t.Fatalf("Get by a tea-b member = %+v, %v", got, err)
	}

	outsider := &Service{Base: &core.Base{Authz: allowChecker{}, Workspace: memberOf{"tea-c"}}, Store: st}
	_, foreign := outsider.Get(ctx, env.ID)
	_, missing := outsider.Get(ctx, "evm-d0000000000000000000")
	if !errors.Is(foreign, core.ErrForbidden) || !errors.Is(missing, core.ErrNotFound) {
		t.Fatalf("non-member Get = %v, missing = %v; want forbidden vs not found (ADR072 #8)", foreign, missing)
	}
	if err := outsider.Delete(ctx, env.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("non-member Delete = %v, want forbidden", err)
	}
}
