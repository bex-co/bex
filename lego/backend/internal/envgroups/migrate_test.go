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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/secrets"
)

// seedLegacyGroup writes a full, unmigrated group directly at the legacy
// tenant — meta + env + files + a revision row — the exact shape MigratePaths
// is meant to find and move.
func seedLegacyGroup(t *testing.T, store *fakeStore, gid, name, workspace string) {
	t.Helper()
	ctx := context.Background()
	if err := store.Put(legacyCtx(ctx), metaPath(gid), map[string]string{
		"name": name, "workspace": workspace, "createdAt": "t0", "updatedAt": "t0",
	}); err != nil {
		t.Fatalf("seed legacy meta: %v", err)
	}
	if err := store.Put(legacyCtx(ctx), envPath(gid), map[string]string{"K": "v"}); err != nil {
		t.Fatalf("seed legacy env: %v", err)
	}
	if err := store.Put(legacyCtx(ctx), filesPath(gid), map[string]string{"ca.pem": "CERT"}); err != nil {
		t.Fatalf("seed legacy files: %v", err)
	}
	if _, err := store.PutCAS(legacyCtx(ctx), revisionPath(gid), map[string]string{"state": "idle", "generation": "3"}, 0); err != nil {
		t.Fatalf("seed legacy revision: %v", err)
	}
}

func TestMigratePathsDryRunReportsWithoutWriting(t *testing.T) {
	store := newFakeStore()
	seedLegacyGroup(t, store, "evg-d7a1g900000000000010", "alpha", "tea-a")
	seedLegacyGroup(t, store, "evg-d7a1g900000000000011", "bravo", "tea-b")

	report, err := MigratePaths(context.Background(), store, true)
	if err != nil {
		t.Fatalf("MigratePaths dry-run: %v", err)
	}
	if report.Scanned != 2 || report.Migrated != 2 || report.AlreadyMigrated != 0 || report.SkippedNoWorkspace != 0 {
		t.Fatalf("dry-run report = %+v", report)
	}
	// Nothing written: the legacy metas are still full, non-locator entries.
	raw, _ := store.Get(legacyCtx(context.Background()), metaPath("evg-d7a1g900000000000010"))
	if isLocator(raw) || raw["name"] != "alpha" {
		t.Fatalf("dry-run must not mutate legacy meta, got %+v", raw)
	}
	if _, ok := store.m[store.key(secrets.WithTenant(context.Background(), "tea-a"), metaPath("evg-d7a1g900000000000010"))]; ok {
		t.Fatalf("dry-run must not write to the workspace tenant")
	}
}

func TestMigratePathsApplyMovesContentAndTombstonesLegacy(t *testing.T) {
	store := newFakeStore()
	gid := "evg-d7a1g900000000000012"
	seedLegacyGroup(t, store, gid, "alpha", "tea-a")

	report, err := MigratePaths(context.Background(), store, false)
	if err != nil {
		t.Fatalf("MigratePaths apply: %v", err)
	}
	if report.Scanned != 1 || report.Migrated != 1 || report.AlreadyMigrated != 0 || report.SkippedNoWorkspace != 0 || len(report.Failed) != 0 {
		t.Fatalf("apply report = %+v", report)
	}

	workspaceCtx := secrets.WithTenant(context.Background(), "tea-a")
	meta, err := store.Get(workspaceCtx, metaPath(gid))
	if err != nil || meta["name"] != "alpha" || meta["workspace"] != "tea-a" {
		t.Fatalf("workspace tenant meta after migration = %+v err=%v", meta, err)
	}
	env, err := store.Get(workspaceCtx, envPath(gid))
	if err != nil || env["K"] != "v" {
		t.Fatalf("workspace tenant env after migration = %+v err=%v", env, err)
	}
	files, err := store.Get(workspaceCtx, filesPath(gid))
	if err != nil || files["ca.pem"] != "CERT" {
		t.Fatalf("workspace tenant files after migration = %+v err=%v", files, err)
	}
	rev, err := store.GetVersioned(workspaceCtx, revisionPath(gid))
	if err != nil || rev.Data["generation"] != "3" {
		t.Fatalf("workspace tenant revision after migration = %+v err=%v", rev, err)
	}

	legacyMeta, err := store.Get(legacyCtx(context.Background()), metaPath(gid))
	if err != nil || legacyMeta["locator"] != "1" || legacyMeta["tombstoned"] != "1" || legacyMeta["workspace"] != "tea-a" {
		t.Fatalf("legacy meta after migration should be a tombstoned locator, got %+v err=%v", legacyMeta, err)
	}
	for _, path := range []string{envPath(gid), filesPath(gid), revisionPath(gid)} {
		if leftover, _ := store.Get(legacyCtx(context.Background()), path); len(leftover) != 0 {
			t.Errorf("legacy leftover at %s after migration: %+v", path, leftover)
		}
	}

	// End-to-end: an ordinary read still works after the move.
	svc := &Service{Base: nil, Store: store}
	got, err := svc.readMeta(context.Background(), gid)
	if err != nil || got.name != "alpha" || got.workspace != "tea-a" {
		t.Fatalf("readMeta after migration = %+v err=%v", got, err)
	}
}

func TestMigratePathsSkipsGroupsWithNoWorkspace(t *testing.T) {
	store := newFakeStore()
	gid := "evg-d7a1g900000000000013"
	seedLegacyGroup(t, store, gid, "storeoff", "")

	report, err := MigratePaths(context.Background(), store, false)
	if err != nil {
		t.Fatalf("MigratePaths: %v", err)
	}
	if report.Scanned != 1 || report.Migrated != 0 || report.SkippedNoWorkspace != 1 {
		t.Fatalf("no-workspace report = %+v", report)
	}
	// Left exactly as it was: still full legacy meta, no tombstone.
	raw, err := store.Get(legacyCtx(context.Background()), metaPath(gid))
	if err != nil || isLocator(raw) || raw["name"] != "storeoff" {
		t.Fatalf("no-workspace group must be left untouched, got %+v err=%v", raw, err)
	}
}

// TestMigratePathsIsIdempotent proves a second pass over an already-migrated
// store does no further work and reports every entry as already-migrated
// rather than re-scanning or re-copying anything.
func TestMigratePathsIsIdempotent(t *testing.T) {
	store := newFakeStore()
	gidA := "evg-d7a1g900000000000014"
	gidB := "evg-d7a1g900000000000015"
	seedLegacyGroup(t, store, gidA, "alpha", "tea-a")
	seedLegacyGroup(t, store, gidB, "bravo", "tea-b")

	if _, err := MigratePaths(context.Background(), store, false); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	second, err := MigratePaths(context.Background(), store, false)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	// Locators are skipped before Scanned is even incremented (they are not
	// "still full legacy" entries), so a fully-migrated store has nothing
	// left to scan at all.
	if second.Scanned != 0 || second.Migrated != 0 || second.AlreadyMigrated != 0 {
		t.Fatalf("idempotent re-run report = %+v, want an all-zero no-op", second)
	}

	workspaceCtx := secrets.WithTenant(context.Background(), "tea-a")
	meta, err := store.Get(workspaceCtx, metaPath(gidA))
	if err != nil || meta["name"] != "alpha" {
		t.Fatalf("re-run must not disturb the migrated group: %+v err=%v", meta, err)
	}
}

// TestMigratePathsCompletesPartialPriorRun exercises the "workspace already
// has full content, legacy still has full content too" branch — as if a
// prior apply crashed after copying but before tombstoning the legacy entry.
// A re-run must finish the tombstone without re-copying (which could clobber
// newer workspace-side edits).
func TestMigratePathsCompletesPartialPriorRun(t *testing.T) {
	store := newFakeStore()
	gid := "evg-d7a1g900000000000016"
	seedLegacyGroup(t, store, gid, "alpha", "tea-a")
	// Simulate the crash: copy to the workspace tenant, but leave the legacy
	// entry as full (uncommitted-looking) content rather than a locator.
	workspaceCtx := secrets.WithTenant(context.Background(), "tea-a")
	if err := store.Put(workspaceCtx, metaPath(gid), map[string]string{
		"name": "alpha", "workspace": "tea-a", "createdAt": "t0", "updatedAt": "t0",
	}); err != nil {
		t.Fatalf("simulate partial copy: %v", err)
	}

	report, err := MigratePaths(context.Background(), store, false)
	if err != nil {
		t.Fatalf("MigratePaths: %v", err)
	}
	if report.Scanned != 1 || report.Migrated != 0 || report.AlreadyMigrated != 1 {
		t.Fatalf("partial-prior-run report = %+v, want AlreadyMigrated=1", report)
	}
	legacyMeta, err := store.Get(legacyCtx(context.Background()), metaPath(gid))
	if err != nil || legacyMeta["locator"] != "1" {
		t.Fatalf("partial prior run should still finish the tombstone: %+v err=%v", legacyMeta, err)
	}
}
