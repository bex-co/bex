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

package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// w22/m311: the first group of a workspace is its default, names are unique per
// workspace, the cap is enforced in the create transaction, and the database
// itself refuses a second default.
func TestSandboxGroupsPG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ws := lifecycleTenant(t, st, "sandbox-groups").ID
	other := lifecycleTenant(t, st, "sandbox-groups-other").ID

	first, err := st.CreateSandboxGroup(ctx, ws, "CI agents", SandboxGroupRegion, 10, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !first.IsDefault || first.ConcurrencyLimit != 10 || first.Region != "oregon" || first.WorkspaceID != ws || !strings.HasPrefix(first.ID, "sbg-") {
		t.Fatalf("first group = %+v, want the workspace's default", first)
	}
	if kind, ok := ids.KindOf(first.ID); !ok || kind != ids.SandboxGroup {
		t.Fatalf("id %q is not a minted sandbox group id", first.ID)
	}
	second, err := st.CreateSandboxGroup(ctx, ws, "Nightly", SandboxGroupRegion, 5, 3)
	if err != nil || second.IsDefault {
		t.Fatalf("second group = %+v, %v, want a non-default group", second, err)
	}

	// Names are unique within a workspace, and only there.
	if _, err := st.CreateSandboxGroup(ctx, ws, "CI agents", SandboxGroupRegion, 1, 3); !errors.Is(err, ErrSandboxGroupNameTaken) {
		t.Fatalf("duplicate name = %v, want ErrSandboxGroupNameTaken", err)
	}
	if g, err := st.CreateSandboxGroup(ctx, other, "CI agents", SandboxGroupRegion, 1, 3); err != nil || !g.IsDefault {
		t.Fatalf("same name in another workspace = %+v, %v, want that workspace's own default", g, err)
	}
	if _, err := st.CreateSandboxGroup(ctx, ws, "", SandboxGroupRegion, 1, 3); err == nil {
		t.Error("an empty name was stored")
	}
	if _, err := st.CreateSandboxGroup(ctx, ws, strings.Repeat("a", 101), SandboxGroupRegion, 1, 3); err == nil {
		t.Error("a 101-character name was stored")
	}
	if _, err := st.CreateSandboxGroup(ctx, ws, "zero", SandboxGroupRegion, 0, 3); err == nil {
		t.Error("a concurrency limit of 0 was stored")
	}

	// The cap counts the workspace's groups and refuses the one past it; a
	// refused create leaves nothing behind.
	if _, err := st.CreateSandboxGroup(ctx, ws, "Third", SandboxGroupRegion, 1, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateSandboxGroup(ctx, ws, "Fourth", SandboxGroupRegion, 1, 3); !errors.Is(err, ErrSandboxGroupLimit) {
		t.Fatalf("create past the cap = %v, want ErrSandboxGroupLimit", err)
	}
	list, err := st.SandboxGroups(ctx, ws)
	if err != nil || len(list) != 3 {
		t.Fatalf("list = %d groups (%v), want 3", len(list), err)
	}
	if list[0].Name != "Third" || list[2].ID != first.ID {
		t.Fatalf("list order = %v, want newest first", []string{list[0].Name, list[1].Name, list[2].Name})
	}
	defaults := 0
	for _, g := range list {
		if g.IsDefault {
			defaults++
		}
	}
	if defaults != 1 {
		t.Fatalf("%d default groups, want exactly 1", defaults)
	}

	// A group is only ever read inside its own workspace.
	if _, err := st.SandboxGroup(ctx, other, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign get = %v, want ErrNotFound", err)
	}
	if owner, err := st.SandboxGroupWorkspace(ctx, second.ID); err != nil || owner != ws {
		t.Fatalf("routing read = %q, %v, want %q", owner, err, ws)
	}
	if _, err := st.SandboxGroupWorkspace(ctx, ids.New(ids.SandboxGroup)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("routing read of an unknown id = %v, want ErrNotFound", err)
	}

	// The limit changes in place and stops nothing; a foreign update finds no group.
	updated, err := st.SetSandboxGroupLimit(ctx, ws, second.ID, 2)
	if err != nil || updated.ConcurrencyLimit != 2 || !updated.UpdatedAt.After(second.UpdatedAt) {
		t.Fatalf("update = %+v, %v", updated, err)
	}
	if _, err := st.SetSandboxGroupLimit(ctx, other, second.ID, 9); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign update = %v, want ErrNotFound", err)
	}
	if got, _ := st.SandboxGroup(ctx, ws, second.ID); got.ConcurrencyLimit != 2 {
		t.Fatalf("a foreign update changed the group to %d", got.ConcurrencyLimit)
	}

	// The database, not just the transaction above, refuses a second default.
	if _, err := st.Pool.Exec(ctx, `UPDATE sandbox_groups SET is_default = true WHERE id = $1`, second.ID); err == nil {
		t.Fatal("two defaults in one workspace were accepted")
	}
}

// A workspace has exactly one default however many first requests race to make it.
func TestEnsureDefaultSandboxGroupIsExactlyOnePG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ws := lifecycleTenant(t, st, "sandbox-default").ID

	var wg sync.WaitGroup
	got := make([]SandboxGroup, 12)
	errs := make([]error, len(got))
	for i := range got {
		wg.Go(func() { got[i], errs[i] = st.EnsureDefaultSandboxGroup(ctx, ws, 1000) })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("ensure %d: %v", i, err)
		}
		if got[i].ID != got[0].ID {
			t.Fatalf("ensure %d returned %s, want the one default %s", i, got[i].ID, got[0].ID)
		}
	}
	if !got[0].IsDefault || got[0].Name != DefaultSandboxGroupName || got[0].ConcurrencyLimit != 1000 || got[0].Region != "oregon" {
		t.Fatalf("default group = %+v", got[0])
	}
	list, err := st.SandboxGroups(ctx, ws)
	if err != nil || len(list) != 1 {
		t.Fatalf("workspace has %d groups (%v), want 1", len(list), err)
	}

	// A group created after the default is not the default.
	next, err := st.CreateSandboxGroup(ctx, ws, "Later", SandboxGroupRegion, 5, 10)
	if err != nil || next.IsDefault {
		t.Fatalf("group after the default = %+v, %v", next, err)
	}
	if again, err := st.EnsureDefaultSandboxGroup(ctx, ws, 1000); err != nil || again.ID != got[0].ID {
		t.Fatalf("ensure after a second group = %+v, %v, want the original default", again, err)
	}
}

// A workspace whose first group is created by hand has that group as its default
// (Render: "the workspace's first group becomes its default"), and ensure then
// returns it instead of adding another.
func TestFirstExplicitGroupIsTheDefaultPG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ws := lifecycleTenant(t, st, "sandbox-first").ID
	first, err := st.CreateSandboxGroup(ctx, ws, "Mine", SandboxGroupRegion, 7, 10)
	if err != nil || !first.IsDefault {
		t.Fatalf("first group = %+v, %v", first, err)
	}
	got, err := st.EnsureDefaultSandboxGroup(ctx, ws, 1000)
	if err != nil || got.ID != first.ID || got.Name != "Mine" {
		t.Fatalf("ensure = %+v, %v, want %s", got, err, first.ID)
	}
}

// A workspace that has groups but no default (which these writes cannot produce,
// but a manual repair or a future delete could) gets its oldest group as the
// default, instead of a second "Default" group or an error.
func TestEnsureDefaultPromotesTheOldestGroupWhenNoneIsDefaultPG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ws := lifecycleTenant(t, st, "sandbox-nodefault").ID
	oldest, err := st.CreateSandboxGroup(ctx, ws, "Oldest", SandboxGroupRegion, 7, 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateSandboxGroup(ctx, ws, "Newer", SandboxGroupRegion, 7, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE sandbox_groups SET is_default = false WHERE workspace_id = $1`, ws); err != nil {
		t.Fatal(err)
	}
	got, err := st.EnsureDefaultSandboxGroup(ctx, ws, 1000)
	if err != nil || got.ID != oldest.ID || !got.IsDefault {
		t.Fatalf("ensure with no default = %+v, %v, want the oldest group %s promoted", got, err, oldest.ID)
	}
	if list, err := st.SandboxGroups(ctx, ws); err != nil || len(list) != 2 {
		t.Fatalf("workspace has %d groups (%v), want its 2 and no extra Default", len(list), err)
	}
}

// Bindings carry the lease, refuse a group of another workspace, are pruned only
// once old and unleased and absent from the live list, and die with the workspace.
func TestSandboxGroupMembersPG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "sandbox-members")
	ws := tenant.ID
	other := lifecycleTenant(t, st, "sandbox-members-other").ID
	g, err := st.CreateSandboxGroup(ctx, ws, "g", SandboxGroupRegion, 5, 10)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := st.CreateSandboxGroup(ctx, other, "g", SandboxGroupRegion, 5, 10)
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().UnixNano()
	a, b := fmt.Sprintf("sbx-a%d", stamp), fmt.Sprintf("sbx-b%d", stamp)
	lease := time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond)
	bind := func(sandbox, group string, until time.Time, workspace string) error {
		return st.WithSandboxAdmission(ctx, workspace, func(adm SandboxAdmission) error { return adm.Bind(ctx, sandbox, group, until) })
	}
	if err := bind(a, g.ID, lease, ws); err != nil {
		t.Fatal(err)
	}
	if err := bind(b, g.ID, lease, ws); err != nil {
		t.Fatal(err)
	}
	// A sandbox of this workspace cannot be bound to another workspace's group.
	if err := bind("sbx-x"+fmt.Sprint(stamp), foreign.ID, lease, ws); err == nil {
		t.Fatal("a sandbox was bound to another workspace's group")
	}
	members, err := st.SandboxGroupMembers(ctx, ws)
	if err != nil || len(members) != 2 || members[a].GroupID != g.ID || members[a].LeaseUntil == nil || !members[a].LeaseUntil.Equal(lease) {
		t.Fatalf("members = %+v (%v), want a and b in %s with the lease", members, err, g.ID)
	}

	// A re-bind renews the lease and keeps the group.
	later := lease.Add(time.Hour)
	if err := bind(a, foreign.ID, later, ws); err != nil {
		t.Fatal(err) // an existing binding only renews its lease; the group argument is not applied
	}
	if members, _ = st.SandboxGroupMembers(ctx, ws); members[a].GroupID != g.ID {
		t.Fatalf("a re-bind moved the sandbox to %s", members[a].GroupID)
	}
	if err := bind(a, g.ID, later, ws); err != nil {
		t.Fatal(err)
	}
	if members, _ = st.SandboxGroupMembers(ctx, ws); !members[a].LeaseUntil.Equal(later) || members[a].GroupID != g.ID {
		t.Fatalf("re-bound member = %+v, want the renewed lease in the same group", members[a])
	}
	if err := st.SetSandboxSlotLease(ctx, ws, a, nil); err != nil {
		t.Fatal(err)
	}
	if members, _ = st.SandboxGroupMembers(ctx, ws); members[a].LeaseUntil != nil {
		t.Fatalf("lease after clear = %v, want none", members[a].LeaseUntil)
	}

	// Prune: young rows survive, and so does a row with a live lease; an old,
	// unleased row that is not in the live list goes.
	if err := st.PruneSandboxGroupMembers(ctx, ws, nil); err != nil {
		t.Fatal(err)
	}
	if members, _ = st.SandboxGroupMembers(ctx, ws); len(members) != 2 {
		t.Fatalf("a young row was pruned: %v", members)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE sandbox_group_members SET created_at = now() - interval '1 hour' WHERE workspace_id = $1`, ws); err != nil {
		t.Fatal(err)
	}
	if err := st.PruneSandboxGroupMembers(ctx, ws, []string{b}); err != nil {
		t.Fatal(err)
	}
	members, _ = st.SandboxGroupMembers(ctx, ws)
	if _, kept := members[a]; kept || len(members) != 1 {
		t.Fatalf("after prune = %v, want only the leased b (a is old, unleased and not live)", members)
	}
	if err := st.PruneSandboxGroupMembers(ctx, ws, []string{a}); err != nil {
		t.Fatal(err)
	}
	if members, _ = st.SandboxGroupMembers(ctx, ws); len(members) != 1 {
		t.Fatalf("a leased row was pruned: %v", members)
	}
	if err := st.ReleaseSandboxSlot(ctx, ws, b); err != nil {
		t.Fatal(err)
	}
	if members, _ = st.SandboxGroupMembers(ctx, ws); len(members) != 0 {
		t.Fatalf("after release = %v", members)
	}

	// Deleting the workspace removes its groups and their bindings.
	if err := bind(a, g.ID, lease, ws); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteTenant(ctx, ws); err != nil {
		t.Fatal(err)
	}
	var groups, bound int
	if err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM sandbox_groups WHERE workspace_id = $1`, ws).Scan(&groups); err != nil {
		t.Fatal(err)
	}
	if err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM sandbox_group_members WHERE workspace_id = $1`, ws).Scan(&bound); err != nil {
		t.Fatal(err)
	}
	if groups != 0 || bound != 0 {
		t.Fatalf("a deleted workspace left %d groups and %d bindings", groups, bound)
	}
	if got, err := st.SandboxGroups(ctx, other); err != nil || len(got) != 1 {
		t.Fatalf("deleting one workspace changed another's groups: %v %v", got, err)
	}
}

// The admission lock makes count-then-bind one critical section per workspace:
// however many creates race, no more than the limit are admitted. The sleep
// widens the window a missing lock would need to overshoot.
func TestSandboxAdmissionNeverExceedsTheLimitPG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ws := lifecycleTenant(t, st, "sandbox-admit").ID
	const limit, racers = 3, 24
	g, err := st.CreateSandboxGroup(ctx, ws, "g", SandboxGroupRegion, limit, 10)
	if err != nil {
		t.Fatal(err)
	}
	var admitted, refused, inside, overlapped atomic.Int32
	var wg sync.WaitGroup
	for i := range racers {
		wg.Go(func() {
			err := st.WithSandboxAdmission(ctx, ws, func(adm SandboxAdmission) error {
				if inside.Add(1) != 1 {
					overlapped.Add(1)
				}
				defer inside.Add(-1)
				members, err := adm.Members(ctx)
				if err != nil {
					return err
				}
				if len(members) >= limit {
					refused.Add(1)
					return nil
				}
				time.Sleep(5 * time.Millisecond)
				if err := adm.Bind(ctx, fmt.Sprintf("sbx-race%02d", i), g.ID, time.Now().Add(time.Hour)); err != nil {
					return err
				}
				admitted.Add(1)
				return nil
			})
			if err != nil {
				t.Errorf("racer %d: %v", i, err)
			}
		})
	}
	wg.Wait()
	if overlapped.Load() != 0 {
		t.Fatalf("%d admissions ran inside another's critical section", overlapped.Load())
	}
	members, err := st.SandboxGroupMembers(ctx, ws)
	if err != nil {
		t.Fatal(err)
	}
	if admitted.Load() != limit || len(members) != limit || refused.Load() != racers-limit {
		t.Fatalf("admitted %d, refused %d, bound %d of %d racers; want exactly %d admitted", admitted.Load(), refused.Load(), len(members), racers, limit)
	}
}

// Admission locks are per workspace: one workspace's slow admission does not
// hold another's.
func TestSandboxAdmissionLockIsPerWorkspacePG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	a := lifecycleTenant(t, st, "sandbox-lock-a").ID
	b := lifecycleTenant(t, st, "sandbox-lock-b").ID
	held, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- st.WithSandboxAdmission(ctx, a, func(SandboxAdmission) error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	other := make(chan error, 1)
	go func() { other <- st.WithSandboxAdmission(ctx, b, func(SandboxAdmission) error { return nil }) }()
	select {
	case err := <-other:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("another workspace's admission waited on this workspace's lock")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// A waiter that cannot get the workspace's lock in time gives up with a named
// error instead of holding a pooled connection for ever.
func TestSandboxAdmissionWaitIsBoundedPG(t *testing.T) {
	ctx := context.Background()
	st := openLifecyclePG(t)
	ws := lifecycleTenant(t, st, "sandbox-busy").ID
	defer func(old time.Duration) { sandboxAdmissionLockTimeout = old }(sandboxAdmissionLockTimeout)
	sandboxAdmissionLockTimeout = 200 * time.Millisecond

	held, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- st.WithSandboxAdmission(ctx, ws, func(SandboxAdmission) error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	started := time.Now()
	err := st.WithSandboxAdmission(ctx, ws, func(SandboxAdmission) error {
		t.Error("an admission ran while another held the workspace's lock")
		return nil
	})
	if !errors.Is(err, ErrSandboxAdmissionBusy) || time.Since(started) > 5*time.Second {
		t.Fatalf("waiter = %v after %v, want ErrSandboxAdmissionBusy within the bound", err, time.Since(started))
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// The lock is free again.
	if err := st.WithSandboxAdmission(ctx, ws, func(SandboxAdmission) error { return nil }); err != nil {
		t.Fatalf("admission after the holder finished = %v", err)
	}
}
