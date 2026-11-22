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

package apps

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render: "Only one Free Render Postgres database can be active for any
// given workspace" (render.com/docs/free). A Blueprint sync holds a paid
// database moving onto free to that rule exactly as it holds a free create,
// and as the API's plan change does, judging the workspace the whole file
// leaves (w4/m241).

// databasesBlueprint declares one database per name/plan pair, in order.
func databasesBlueprint(pairs ...string) string {
	var out strings.Builder
	out.WriteString("databases:\n")
	for i := 0; i+1 < len(pairs); i += 2 {
		out.WriteString("  - name: " + pairs[i] + "\n    plan: " + pairs[i+1] + "\n")
	}
	return out.String()
}

// databasePlans maps each database's display name to its stored plan.
func databasePlans(t *testing.T, svc *Service) map[string]string {
	t.Helper()
	var list appv1alpha1.DatabaseList
	if err := svc.Client.List(context.Background(), &list); err != nil {
		t.Fatal(err)
	}
	plans := map[string]string{}
	for _, d := range list.Items {
		plans[d.Spec.Name] = d.Spec.Plan
	}
	return plans
}

func wantPlans(t *testing.T, svc *Service, want map[string]string) {
	t.Helper()
	got := databasePlans(t, svc)
	if len(got) != len(want) {
		t.Fatalf("databases = %v, want %v", got, want)
	}
	for name, plan := range want {
		if got[name] != plan {
			t.Fatalf("databases = %v, want %v", got, want)
		}
	}
}

func wantFreeInstanceLimit(t *testing.T, what string, err error, holder string) {
	t.Helper()
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.FreeInstanceLimitCode || !strings.Contains(err.Error(), "already has a free Postgres database ("+holder+")") {
		t.Fatalf("%s = %v, want %s naming %s", what, err, core.FreeInstanceLimitCode, holder)
	}
}

func wantFreeInstanceLimitEntry(t *testing.T, what string, v BlueprintValidation, err error, path, holder string) {
	t.Helper()
	if err != nil || v.Valid || len(v.Errors) != 1 {
		t.Fatalf("%s = %+v, %v; want one refusal", what, v, err)
	}
	e := v.Errors[0]
	if e.Code != core.FreeInstanceLimitCode || e.Path == nil || *e.Path != path || !strings.Contains(e.Error, "("+holder+")") {
		t.Fatalf("%s refusal = %s at %v: %s; want %s at %s naming %s", what, e.Code, ptrValue(e.Path), e.Error, core.FreeInstanceLimitCode, path, holder)
	}
}

func TestABlueprintCannotMoveAPaidPostgresToFreeBesideAFreeOne(t *testing.T) {
	svc, fs := ownershipService(t)
	ctx := ownershipCtx()
	svc.GitFetcher = fakeBlueprintFetcher{contents: databasesBlueprint("db1", "free", "db2", "basic-256mb")}
	bp, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/free-slot", Branch: "main"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	moved := databasesBlueprint("db1", "free", "db2", "free")
	svc.GitFetcher = fakeBlueprintFetcher{contents: moved}

	v, err := svc.ValidateBlueprint(ctx, "", moved, bp.ID)
	wantFreeInstanceLimitEntry(t, "validate", v, err, "databases[1].plan", "db1")

	planned, err := svc.PlanBlueprint(ctx, bp.ID, "tea-a")
	if err != nil || planned.PlanningOutcome != PlanningFailed || len(planned.Notices) != 1 ||
		!strings.Contains(planned.Notices[0].Message, "already has a free Postgres database (db1)") {
		t.Fatalf("plan = %+v, %v; want failed with the free-instance refusal", planned, err)
	}

	_, err = svc.SyncBlueprint(ctx, bp.ID, "tea-a", "", "", nil)
	wantFreeInstanceLimit(t, "sync", err, "db1")

	_, err = svc.DeployStack(ctx, DeployRequest{Manifest: moved})
	wantFreeInstanceLimit(t, "deploy", err, "db1")

	// Auto-sync runs the same sync, unattended.
	storageID := ids.BlueprintStorageID(bp.ID)
	fs.mu.Lock()
	row := fs.blueprints[storageID]
	row.AutoSync = true
	fs.blueprints[storageID] = row
	fs.mu.Unlock()
	if _, err := fs.EnqueueBlueprintAutoSyncIntent(ctx, store.BlueprintAutoSyncIntent{BlueprintID: row.ID, TenantID: row.TenantID, DeliveryDigest: "push-1", CommitSHA: testCommitSHA, Path: row.Path}); err != nil {
		t.Fatal(err)
	}
	intents, err := fs.ClaimBlueprintAutoSyncIntents(ctx, 1, 0)
	if err != nil || len(intents) != 1 {
		t.Fatalf("claim = %+v, %v", intents, err)
	}
	(&BlueprintAutoSyncWorker{Svc: svc}).process(context.Background(), intents[0])
	if !autoSyncFailedWith(fs, row.ID, "already has a free Postgres database (db1)") {
		t.Fatalf("auto-sync history = %+v, want a failed run with the free-instance refusal", fs.syncs)
	}

	wantPlans(t, svc, map[string]string{"db1": "free", "db2": "basic-256mb"})
}

// autoSyncFailedWith reports a failed sync of blueprintID whose error says
// message.
func autoSyncFailedWith(fs *fakeBlueprintStore, blueprintID, message string) bool {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for _, run := range fs.syncs {
		if run.BlueprintID == blueprintID && run.State == store.BlueprintSyncStateError && run.ErrorMessage != nil && strings.Contains(*run.ErrorMessage, message) {
			return true
		}
	}
	return false
}

// The workspace's only Postgres moves to free, as SetPlan admits it.
func TestABlueprintMovesTheOnlyPostgresToFree(t *testing.T) {
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	ctx := paidGateContext()
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: databasesBlueprint("db2", "basic-256mb")}); err != nil {
		t.Fatal(err)
	}
	moved := databasesBlueprint("db2", "free")
	if v, err := svc.ValidateBlueprint(ctx, "", moved, ""); err != nil || !v.Valid {
		t.Fatalf("validate = %+v, %v; want valid", v, err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: moved}); err != nil {
		t.Fatalf("sync: %v", err)
	}
	wantPlans(t, svc, map[string]string{"db2": "free"})
}

// The file is judged as a whole: two databases moving onto free, or one moving
// onto it beside a free create, are one too many, whatever the order, and the
// sync is refused before it writes either.
func TestABlueprintSyncClaimingTheFreeSlotTwiceIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name, manifest, path, holder string
	}{
		{"two moves onto free", databasesBlueprint("db1", "free", "db2", "free"), "databases[1].plan", "db1"},
		{"a move then a free create", databasesBlueprint("db1", "free", "db2", "basic-256mb", "db3", "free"), "databases[2].plan", "db1"},
		{"a free create then a move", databasesBlueprint("db3", "free", "db1", "free", "db2", "basic-256mb"), "databases[1].plan", "db3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
			ctx := paidGateContext()
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: databasesBlueprint("db1", "basic-256mb", "db2", "basic-256mb")}); err != nil {
				t.Fatal(err)
			}
			v, err := svc.ValidateBlueprint(ctx, "", tc.manifest, "")
			wantFreeInstanceLimitEntry(t, "validate", v, err, tc.path, tc.holder)
			_, err = svc.DeployStack(ctx, DeployRequest{Manifest: tc.manifest})
			wantFreeInstanceLimit(t, "sync", err, tc.holder)
			wantPlans(t, svc, map[string]string{"db1": "basic-256mb", "db2": "basic-256mb"})
		})
	}
}

// A sync that hands the free slot from one database to another is admitted
// whichever the file lists first: the apply moves the holder off free before
// it moves the other onto it.
func TestABlueprintSyncSwapsTheFreeSlotInEitherFileOrder(t *testing.T) {
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	locks := &tenantLockSpy{}
	svc.Workspace = &lockingFakeWorkspace{fakeWorkspace: fakeWorkspace{"identity-a": "tea-a"}, spy: locks}
	ctx := paidGateContext()
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: databasesBlueprint("db1", "free", "db2", "basic-256mb")}); err != nil {
		t.Fatal(err)
	}
	for _, swap := range []struct {
		name     string
		manifest string
		want     map[string]string
	}{
		{"the holder listed first", databasesBlueprint("db1", "basic-256mb", "db2", "free"), map[string]string{"db1": "basic-256mb", "db2": "free"}},
		{"the claim listed first", databasesBlueprint("db1", "free", "db2", "basic-256mb"), map[string]string{"db1": "free", "db2": "basic-256mb"}},
	} {
		if v, err := svc.ValidateBlueprint(ctx, "", swap.manifest, ""); err != nil || !v.Valid {
			t.Fatalf("%s: validate = %+v, %v; want valid", swap.name, v, err)
		}
		res, err := svc.DeployStack(ctx, DeployRequest{Manifest: swap.manifest})
		if err != nil {
			t.Fatalf("%s: sync: %v", swap.name, err)
		}
		wantPlans(t, svc, swap.want)
		if len(res.Databases) != 2 || res.Databases[0].Name != "db1" || res.Databases[1].Name != "db2" {
			t.Fatalf("%s: applied views = %+v, want db1 then db2 as the file lists them", swap.name, res.Databases)
		}
	}
	// The free create and each move onto free took the tenant lock, once each.
	if got := locks.taken(); got != 3 {
		t.Fatalf("tenant lock taken %d times, want 3 (one free create, two moves onto free)", got)
	}
}

// tenantLockSpy is an in-process tenant lock that counts its holds and fails a
// second acquisition while it is held: the advisory lock is not re-entrant, so
// production would deadlock there.
type tenantLockSpy struct {
	mu    sync.Mutex
	holds int
}

func (s *tenantLockSpy) taken() int {
	if !s.mu.TryLock() {
		return -1
	}
	defer s.mu.Unlock()
	return s.holds
}

type lockingFakeWorkspace struct {
	fakeWorkspace
	spy *tenantLockSpy
}

func (w *lockingFakeWorkspace) WithTenantAdvisoryLock(_ context.Context, _ string, fn func() error) error {
	if !w.spy.mu.TryLock() {
		return errors.New("tenant lock taken again while held")
	}
	defer w.spy.mu.Unlock()
	w.spy.holds++
	return fn()
}
