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
