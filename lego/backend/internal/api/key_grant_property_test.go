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

package api

import (
	"context"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
)

// subjectRoles is a FreshChecker answering each subject's role in tea-a with
// roleGrants, the table TestRoleGrantsPinnedToModel holds to model.fga.
type subjectRoles map[string]string

func (r subjectRoles) Check(_ context.Context, subject, relation, object string) (bool, error) {
	return object == core.WorkspaceObject("tea-a") && roleGrants[r[strings.TrimPrefix(subject, "user:")]][relation], nil
}

func (r subjectRoles) CheckFresh(ctx context.Context, subject, relation, object string) (bool, error) {
	return r.Check(ctx, subject, relation, object)
}

// I1 and I6 over random cases, through the real dispatch gate and the real
// creator ceiling: a key is allowed an operation's relation exactly when its
// grant covers every action the operation needs, its own tuple holds the
// relation, and its creator holds it now. The expectation is computed from the
// catalog's tables, not from KeyGrant.Allows.
func TestKeyDecisionIsItsGrantAndItsCreator(t *testing.T) {
	rng := rand.New(rand.NewPCG(496, 2026))
	var ops []string
	for op, need := range keyPermissionOps {
		if !slices.Contains(need, core.KeyNoGrant) {
			ops = append(ops, op)
		}
	}
	slices.Sort(ops)
	var permissions []core.KeyPermission
	for _, area := range core.KeyPermissionCatalog {
		permissions = append(permissions, area.Permissions...)
	}
	roleNames := append([]string{""}, workspaceRoles...) // "": a creator who left
	relations := []string{core.RelCanView, core.RelCanViewLogs, core.RelCanOperate, core.RelCanCreate,
		core.RelCanViewSensitive, core.RelCanManage, core.RelCanManageBilling, core.RelCanViewBilling}

	roles := subjectRoles{}
	grants := &grantStore{grants: map[string]core.KeyGrant{}}
	base := &core.Base{
		Client: fakeClient(), Namespace: "default",
		Authz: roles, Workspace: coretest.Workspaces{"tea-a"},
		KeyCreators: coretest.KeyCreators{"key": "creator"}, KeyGrants: grants,
	}
	srv := NewServer(base, Deps{})
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "key", ClientID: "key", Method: core.MethodOAuth2})

	allowedCases := 0
	for i := range 500 {
		var grant core.KeyGrant
		covered := map[core.KeyAction]bool{}
		if rng.IntN(10) == 0 {
			grant = core.AllKeyGrant()
		} else {
			grant = core.KeyGrant{Grants: []core.KeyGrantEntry{}}
			for _, p := range permissions {
				if rng.IntN(3) != 0 {
					continue
				}
				l := p.Levels[rng.IntN(len(p.Levels))]
				grant.Grants = append(grant.Grants, core.KeyGrantEntry{Permission: p.ID, Level: l.Level})
				for _, a := range l.Actions {
					covered[a] = true
				}
			}
		}
		roles["creator"] = roleNames[rng.IntN(len(roleNames))]
		roles["key"] = roleNames[rng.IntN(len(roleNames))] // the key's tuple may lag its creator either way
		grants.set("key", grant)
		op := ops[rng.IntN(len(ops))]
		relation := relations[rng.IntN(len(relations))]

		grantCovers := grant.All
		if !grant.All {
			grantCovers = true
			for _, a := range keyPermissionOps[op] {
				if a != core.KeyAnyGrant && !covered[a] {
					grantCovers = false
				}
			}
		}
		want := grantCovers && roleGrants[roles["key"]][relation] && roleGrants[roles["creator"]][relation]

		gateErr := srv.requireKeyGrant(ctx, op)
		held, err := base.HoldsFresh(ctx, relation, core.WorkspaceObject("tea-a"))
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if got := gateErr == nil && held; got != want {
			t.Fatalf("case %d: %s with %s as a %s key of a %s creator, grant %v: allowed=%v, want %v (gate %v, relation %v)",
				i, op, relation, roles["key"], roles["creator"], grant.Entries(), got, want, gateErr, held)
		}
		if want {
			allowedCases++
		}
	}
	if allowedCases < 25 {
		t.Fatalf("only %d of 500 cases were allowed; the generator is not exercising the allow side", allowedCases)
	}
}
