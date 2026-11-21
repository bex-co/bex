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
	"errors"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestMoveEnvGroupChecksTheEnvironmentItLeaves is w4/m232: MoveEnvGroup
// authorized only can_create on the group, so a developer could take a group
// out of a protected environment. The environment it leaves now goes through
// the exit check, against the group's current row; moving in stays open.
func TestMoveEnvGroupChecksTheEnvironmentItLeaves(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	svc.EnvironmentWorkspace = environmentResolver("env-prod", "env-qa", "env-dev")
	var exits []string
	svc.ExitGuards = func(context.Context, string) (core.EnvironmentExitGuard, error) {
		return func(ids ...string) error {
			exits = append(exits, ids...)
			if slices.Contains(ids, "env-prod") {
				return core.ErrForbidden
			}
			return nil
		}, nil
	}
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared", EnvironmentID: "env-dev"})
	if err != nil {
		t.Fatal(err)
	}

	// Into the protected environment: only the bare source is checked.
	if _, err := svc.MoveEnvGroup(ctx, group.ID, "env-prod"); err != nil {
		t.Fatalf("move into prod: %v", err)
	}
	if !slices.Equal(exits, []string{"env-dev"}) {
		t.Fatalf("exits checked = %v, want only env-dev", exits)
	}
	// Out of it: refused, and the group stays put.
	if _, err := svc.MoveEnvGroup(ctx, group.ID, "env-qa"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("move out of prod = %v, want ErrForbidden", err)
	}
	if got, _ := svc.GetEnvGroup(ctx, group.ID); got.EnvironmentID != "env-prod" {
		t.Fatalf("a refused move changed the group's environment to %q", got.EnvironmentID)
	}
	// Unscoping is leaving too.
	if _, err := svc.MoveEnvGroup(ctx, group.ID, ""); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("unscope out of prod = %v, want ErrForbidden", err)
	}
}

// SetEnvironmentID moves a group only while it is still in the environment the
// caller observed it in, as a datastore's SetEnvironmentID does: a group that
// another request moved before the caller's read, or between that read and the
// metadata write, stays where that request put it and the move is refused. The
// environments feature's add, remove and the reversal of a failed add go
// through it, so none of them takes a group from a concurrent request.
func TestSetEnvironmentIDLeavesAGroupAnotherRequestMoved(t *testing.T) {
	ctx := testCtx()
	svc := newService(newFakeStore())
	resolve := environmentResolver("env-prod", "env-qa", "env-dev")
	svc.EnvironmentWorkspace = resolve
	group, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "shared", EnvironmentID: "env-qa"})
	if err != nil {
		t.Fatal(err)
	}
	groups := groupsOf(svc.Store)
	placedIn := func() string {
		row, _ := groups.Row(group.ID)
		return row.EnvironmentID
	}
	refused := func(err error) bool {
		var coded *core.CodedError
		return errors.As(err, &coded) && coded.Code == "RESOURCE_PLACEMENT_CHANGED"
	}

	// The caller saw it somewhere it no longer is.
	for _, to := range []string{"env-prod", ""} {
		if err := svc.SetEnvironmentID(ctx, group.ID, "env-dev", to); !refused(err) {
			t.Fatalf("a move to %q from a stale environment = %v, want RESOURCE_PLACEMENT_CHANGED", to, err)
		}
		if got := placedIn(); got != "env-qa" {
			t.Fatalf("the refused move to %q left the group in %q, want env-qa", to, got)
		}
	}

	// Another request moves it after the caller read it, before the write.
	svc.EnvironmentWorkspace = func(ctx context.Context, environmentID string) (string, error) {
		groups.Commit(group.ID, func(g *store.EnvGroup) { g.EnvironmentID = "env-dev" })
		return resolve(ctx, environmentID)
	}
	if err := svc.SetEnvironmentID(ctx, group.ID, "env-qa", "env-prod"); !refused(err) {
		t.Fatalf("a move racing a concurrent one = %v, want RESOURCE_PLACEMENT_CHANGED", err)
	}
	if got := placedIn(); got != "env-dev" {
		t.Fatalf("the group is in %q, want env-dev, where the concurrent request put it", got)
	}

	// From where it is, the move lands; the public move needs no observation.
	svc.EnvironmentWorkspace = resolve
	if err := svc.SetEnvironmentID(ctx, group.ID, "env-dev", "env-prod"); err != nil || placedIn() != "env-prod" {
		t.Fatalf("a move from where the group is = %v, group in %q; want env-prod", err, placedIn())
	}
	if _, err := svc.MoveEnvGroup(ctx, group.ID, "env-qa"); err != nil || placedIn() != "env-qa" {
		t.Fatalf("MoveEnvGroup = %v, group in %q; want env-qa", err, placedIn())
	}
}
