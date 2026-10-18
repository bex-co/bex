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
	"slices"
	"sync"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func envGroupFixture(t *testing.T, st *PGStore) (tenantID string, environmentID string) {
	t.Helper()
	ctx := context.Background()
	tenant, err := st.CreateTenant(ctx, "env-groups-"+time.Now().Format("150405.000000000"), PlanHobby)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	project, err := st.CreateProject(ctx, tenant.ID, "p")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	env, err := st.CreateEnvironment(ctx, project.ID, tenant.ID, "staging")
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}
	return tenant.ID, env.ID
}

func newEnvGroupRow(tenantID, name string) EnvGroup {
	now := time.Date(2026, 10, 18, 4, 0, 0, 0, time.UTC)
	return EnvGroup{
		ID: ids.New(ids.EnvGroup), TenantID: tenantID, Name: name,
		ServiceLinks: []string{"srv-a"}, EnvKeys: []string{"A"}, FileNames: []string{"f.txt"},
		CreatedAt: now, UpdatedAt: now,
	}
}

func TestEnvGroupInsertGetListAndNameUniqueness(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	tenant, _ := envGroupFixture(t, st)
	g := newEnvGroupRow(tenant, "shared")
	if _, err := st.InsertEnvGroup(ctx, g); err != nil {
		t.Fatalf("insert: %v", err)
	}
	got, err := st.GetEnvGroup(ctx, g.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "shared" || got.TenantID != tenant || !slices.Equal(got.ServiceLinks, []string{"srv-a"}) ||
		!slices.Equal(got.EnvKeys, []string{"A"}) || !slices.Equal(got.FileNames, []string{"f.txt"}) {
		t.Fatalf("round trip = %+v", got)
	}
	if got.Content.State != EnvGroupIdle || got.Content.Generation != 1 || got.Content.Op != nil {
		t.Fatalf("new group content = %+v, want idle generation 1 with no op", got.Content)
	}
	if !got.CreatedAt.Equal(g.CreatedAt) {
		t.Fatalf("createdAt = %v, want %v", got.CreatedAt, g.CreatedAt)
	}

	dup := newEnvGroupRow(tenant, "shared")
	if _, err := st.InsertEnvGroup(ctx, dup); !errors.Is(err, ErrEnvGroupNameTaken) {
		t.Fatalf("duplicate name insert = %v, want ErrEnvGroupNameTaken", err)
	}
	other, _ := envGroupFixture(t, st)
	if _, err := st.InsertEnvGroup(ctx, newEnvGroupRow(other, "shared")); err != nil {
		t.Fatalf("same name in another workspace: %v", err)
	}
	if taken, err := st.EnvGroupNameTaken(ctx, tenant, "shared"); err != nil || !taken {
		t.Fatalf("name taken = %v, %v; want true", taken, err)
	}
	if taken, _ := st.EnvGroupNameTaken(ctx, tenant, "unused"); taken {
		t.Fatal("an unused name reads as taken")
	}
	if byName, err := st.GetEnvGroupByName(ctx, tenant, "shared"); err != nil || byName.ID != g.ID {
		t.Fatalf("by name = %+v, %v; want the group", byName, err)
	}
	if theirs, err := st.GetEnvGroupByName(ctx, other, "shared"); err != nil || theirs.ID == g.ID {
		t.Fatalf("by name in another workspace = %+v, %v; want that workspace's own group", theirs, err)
	}
	if _, err := st.GetEnvGroupByName(ctx, tenant, "unused"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unused name = %v, want ErrNotFound", err)
	}
	list, err := st.ListEnvGroups(ctx, tenant)
	if err != nil || len(list) != 1 || list[0].ID != g.ID {
		t.Fatalf("list = %+v, %v; want only the workspace's own group", list, err)
	}
	if n, err := st.CountEnvGroups(ctx, tenant); err != nil || n != 1 {
		t.Fatalf("count = %d, %v; want 1", n, err)
	}
}

// Two creates of one name race; the unique constraint admits exactly one.
func TestEnvGroupConcurrentSameNameInsertAdmitsOne(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	tenant, _ := envGroupFixture(t, st)
	const racers = 8
	errs := make([]error, racers)
	var wg sync.WaitGroup
	for i := range racers {
		wg.Go(func() { _, errs[i] = st.InsertEnvGroup(ctx, newEnvGroupRow(tenant, "raced")) })
	}
	wg.Wait()
	won := 0
	for _, err := range errs {
		switch {
		case err == nil:
			won++
		case !errors.Is(err, ErrEnvGroupNameTaken):
			t.Fatalf("loser error = %v, want ErrEnvGroupNameTaken", err)
		}
	}
	if won != 1 {
		t.Fatalf("%d creates won, want exactly 1", won)
	}
}

func TestEnvGroupMetaCompareAndSet(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	tenant, envID := envGroupFixture(t, st)
	g := newEnvGroupRow(tenant, "one")
	if _, err := st.InsertEnvGroup(ctx, g); err != nil {
		t.Fatal(err)
	}
	cur, _ := st.GetEnvGroup(ctx, g.ID)

	next := cur
	next.Name, next.EnvironmentID, next.ServiceLinks = "renamed", envID, []string{"srv-a", "srv-b"}
	next.UpdatedAt = cur.UpdatedAt.Add(time.Minute)
	updated, err := st.UpdateEnvGroupMeta(ctx, next)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.MetaVersion != cur.MetaVersion+1 || updated.Name != "renamed" || updated.EnvironmentID != envID ||
		!slices.Equal(updated.ServiceLinks, []string{"srv-a", "srv-b"}) {
		t.Fatalf("updated = %+v", updated)
	}
	// The same stale version loses.
	if _, err := st.UpdateEnvGroupMeta(ctx, next); !errors.Is(err, ErrEnvGroupChanged) {
		t.Fatalf("stale update = %v, want ErrEnvGroupChanged", err)
	}
	// A rename onto a taken name is the name error, not a generic conflict.
	if _, err := st.InsertEnvGroup(ctx, newEnvGroupRow(tenant, "taken")); err != nil {
		t.Fatal(err)
	}
	clash := updated
	clash.Name = "taken"
	if _, err := st.UpdateEnvGroupMeta(ctx, clash); !errors.Is(err, ErrEnvGroupNameTaken) {
		t.Fatalf("rename onto a taken name = %v, want ErrEnvGroupNameTaken", err)
	}
	// Deleting the row makes every later write report not found.
	deleted, err := st.DeleteEnvGroup(ctx, g.ID)
	if err != nil || deleted.Name != "renamed" {
		t.Fatalf("delete = %+v, %v", deleted, err)
	}
	if _, err := st.UpdateEnvGroupMeta(ctx, updated); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update after delete = %v, want ErrNotFound", err)
	}
	if _, err := st.GetEnvGroup(ctx, g.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete = %v, want ErrNotFound", err)
	}
}

func TestEnvGroupContentClaimAdvanceRelease(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	tenant, _ := envGroupFixture(t, st)
	g := newEnvGroupRow(tenant, "content")
	if _, err := st.InsertEnvGroup(ctx, g); err != nil {
		t.Fatal(err)
	}
	cur, _ := st.GetEnvGroup(ctx, g.ID)
	lease := time.Date(2026, 10, 18, 4, 2, 0, 0, time.UTC)
	op := EnvGroupOp{ID: "ego-1", Kind: "patch", Phase: "admitted", LeaseUntil: lease, EnvChanged: true}
	claim, err := st.ClaimEnvGroupContent(ctx, g.ID, cur.Content.Version, op)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := st.ClaimEnvGroupContent(ctx, g.ID, cur.Content.Version, EnvGroupOp{ID: "ego-2", Kind: "patch", Phase: "admitted", LeaseUntil: lease}); !errors.Is(err, ErrEnvGroupChanged) {
		t.Fatalf("second claim at the old version = %v, want ErrEnvGroupChanged", err)
	}
	busy, _ := st.GetEnvGroup(ctx, g.ID)
	if busy.Content.State != EnvGroupBusy || busy.Content.Op == nil || busy.Content.Op.ID != "ego-1" || !busy.Content.Op.EnvChanged {
		t.Fatalf("claimed content = %+v op=%+v", busy.Content, busy.Content.Op)
	}
	if err := st.AdvanceEnvGroupOp(ctx, g.ID, "ego-1", "env_written", lease.Add(time.Minute)); err != nil {
		t.Fatalf("advance: %v", err)
	}
	if err := st.AdvanceEnvGroupOp(ctx, g.ID, "ego-other", "env_written", lease); !errors.Is(err, ErrEnvGroupChanged) {
		t.Fatalf("advance for another op = %v, want ErrEnvGroupChanged", err)
	}
	advanced, _ := st.GetEnvGroup(ctx, g.ID)
	if advanced.Content.Op.Phase != "env_written" || advanced.Content.Version != claim {
		t.Fatalf("advance moved the claim: %+v", advanced.Content)
	}

	// repair_required keeps the operation as recovery evidence.
	if err := st.ReleaseEnvGroupContent(ctx, g.ID, claim, EnvGroupRelease{RepairRequired: true}); err != nil {
		t.Fatalf("release repair_required: %v", err)
	}
	stranded, _ := st.GetEnvGroup(ctx, g.ID)
	if stranded.Content.State != EnvGroupRepairRequired || stranded.Content.Op == nil || stranded.Content.Op.ID != "ego-1" {
		t.Fatalf("repair_required content = %+v", stranded.Content)
	}
	// A commit releases idle at the next generation and clears the operation.
	at := time.Date(2026, 10, 18, 4, 3, 0, 0, time.UTC)
	if err := st.ReleaseEnvGroupContent(ctx, g.ID, stranded.Content.Version, EnvGroupRelease{
		Commit: &EnvGroupCommit{EnvKeys: []string{"B"}, UpdatedAt: at},
	}); err != nil {
		t.Fatalf("release commit: %v", err)
	}
	idle, _ := st.GetEnvGroup(ctx, g.ID)
	if idle.Content.State != EnvGroupIdle || idle.Content.Generation != 2 || idle.Content.Op != nil ||
		!slices.Equal(idle.EnvKeys, []string{"B"}) || len(idle.FileNames) != 0 || !idle.UpdatedAt.Equal(at) {
		t.Fatalf("committed = %+v keys=%v files=%v updated=%v", idle.Content, idle.EnvKeys, idle.FileNames, idle.UpdatedAt)
	}
	// A release without a commit leaves generation, names and time alone.
	if err := st.ReleaseEnvGroupContent(ctx, g.ID, idle.Content.Version, EnvGroupRelease{}); err != nil {
		t.Fatal(err)
	}
	kept, _ := st.GetEnvGroup(ctx, g.ID)
	if kept.Content.Generation != 2 || !slices.Equal(kept.EnvKeys, []string{"B"}) || !kept.UpdatedAt.Equal(at) {
		t.Fatalf("plain release changed content: gen=%d %v %v", kept.Content.Generation, kept.EnvKeys, kept.UpdatedAt)
	}
}

func TestEnvGroupCascadesFromTenantAndEnvironment(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	tenant, envID := envGroupFixture(t, st)
	g := newEnvGroupRow(tenant, "scoped")
	g.EnvironmentID = envID
	if _, err := st.InsertEnvGroup(ctx, g); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteEnvironment(ctx, envID); err != nil {
		t.Fatalf("delete environment: %v", err)
	}
	got, err := st.GetEnvGroup(ctx, g.ID)
	if err != nil || got.EnvironmentID != "" {
		t.Fatalf("after environment delete = %+v, %v; want workspace scope", got, err)
	}
	if _, err := st.Pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant); err != nil {
		t.Fatalf("delete tenant: %v", err)
	}
	if _, err := st.GetEnvGroup(ctx, g.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("group after tenant delete = %v, want ErrNotFound", err)
	}
}

func TestEnvGroupCutoverMarker(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	if _, err := st.Pool.Exec(ctx, `DELETE FROM env_group_cutover`); err != nil {
		t.Fatal(err)
	}
	if done, err := st.EnvGroupCutoverComplete(ctx); err != nil || done {
		t.Fatalf("fresh marker = %v, %v; want false", done, err)
	}
	at := time.Now().UTC()
	for range 2 {
		if err := st.CompleteEnvGroupCutover(ctx, at); err != nil {
			t.Fatalf("complete: %v", err)
		}
	}
	if done, err := st.EnvGroupCutoverComplete(ctx); err != nil || !done {
		t.Fatalf("after complete = %v, %v; want true", done, err)
	}
}
