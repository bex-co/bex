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
	"github.com/bex-co/bex/lego/backend/internal/store"
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
// refused by their default workspace — and a non-member's id answers exactly
// like a missing one.
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
	if !errors.Is(foreign, store.ErrNotFound) && !errors.Is(foreign, core.ErrNotFound) {
		t.Fatalf("non-member Get = %v, want not found", foreign)
	}
	if foreign.Error() != missing.Error() {
		t.Fatalf("non-member %v vs missing %v, want identical", foreign, missing)
	}
	if err := outsider.Delete(ctx, env.ID); err == nil || foreign.Error() != err.Error() {
		t.Fatalf("non-member Delete = %v, want the same not-found", err)
	}
}
