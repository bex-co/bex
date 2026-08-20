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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
)

// tenant_test.go exercises the w2/m80 workspace-prefixed OpenBao layout
// directly: writes land under the owning workspace's own tenant, a legacy
// locator always remains discoverable by bare gid, and a still-unmigrated
// legacy-resident group keeps working through the dual-read fallback.

func TestCreateEnvGroupLandsUnderWorkspaceTenant(t *testing.T) {
	store := newFakeStore()
	resolver := multiWorkspace{"dana": {"tea-a"}}
	svc := &Service{
		Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: resolver},
		Store: store,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})

	g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{
		Name:    "shared",
		EnvVars: []CreateEnvVarInput{{Key: "K", Value: "v"}},
	})
	if err != nil {
		t.Fatalf("CreateEnvGroup: %v", err)
	}

	workspaceCtx := secrets.WithTenant(context.Background(), "tea-a")
	if _, ok := store.m[store.key(workspaceCtx, metaPath(g.ID))]; !ok {
		t.Errorf("group meta should live under the tea-a tenant")
	}
	if _, ok := store.m[store.key(workspaceCtx, envPath(g.ID))]; !ok {
		t.Errorf("group env should live under the tea-a tenant")
	}

	legacyCtxBg := secrets.WithTenant(context.Background(), secrets.LegacyTenant)
	legacyMeta := store.m[store.key(legacyCtxBg, metaPath(g.ID))]
	if legacyMeta["locator"] != "1" || legacyMeta["workspace"] != "tea-a" {
		t.Errorf("legacy tenant should hold a thin locator pointing at tea-a, got %+v", legacyMeta)
	}
	// The locator must never carry the group's own content.
	if _, ok := store.m[store.key(legacyCtxBg, envPath(g.ID))]; ok {
		t.Errorf("legacy tenant should not hold the group's env content")
	}
}

func TestWriteMetaSkipsLocatorForLegacyWorkspace(t *testing.T) {
	svc := newService(newFakeStore())
	ctx := context.Background()
	gid := "evg-d7a1g900000000000099"

	if err := svc.writeMeta(ctx, gid, meta{name: "n", workspace: ""}); err != nil {
		t.Fatalf("writeMeta with empty workspace: %v", err)
	}
	got, err := svc.readMeta(ctx, gid)
	if err != nil || got.name != "n" {
		t.Fatalf("readMeta after empty-workspace write: %+v err=%v", got, err)
	}
}

// TestGetGroupMapDualReadsLegacyFallback proves getGroupMap falls back to the
// legacy tenant for a group whose content was never written under a
// workspace prefix (the pre-w2/m80 shape, or a not-yet-migrated group), and
// prefers the workspace copy once one exists.
func TestGetGroupMapDualReadsLegacyFallback(t *testing.T) {
	svc := newService(newFakeStore())
	ctx := context.Background()
	gid := "evg-d7a1g900000000000098"

	if err := svc.Store.Put(legacyCtx(ctx), envPath(gid), map[string]string{"K": "legacy-value"}); err != nil {
		t.Fatalf("seed legacy content: %v", err)
	}
	got, err := svc.getGroupMap(ctx, "tea-a", envPath(gid))
	if err != nil || got["K"] != "legacy-value" {
		t.Fatalf("dual-read fallback = %+v err=%v, want legacy-value", got, err)
	}

	if err := svc.putGroupMap(ctx, "tea-a", envPath(gid), map[string]string{"K": "workspace-value"}); err != nil {
		t.Fatalf("write workspace content: %v", err)
	}
	got, err = svc.getGroupMap(ctx, "tea-a", envPath(gid))
	if err != nil || got["K"] != "workspace-value" {
		t.Fatalf("workspace copy should win once present: %+v err=%v", got, err)
	}
}

// TestLegacySeededGroupStillReadableViaGetEnvGroup seeds a group's FULL
// metadata + content directly under the legacy tenant — bypassing
// CreateEnvGroup entirely, the shape every group had before this milestone
// (or one MigratePaths has not yet visited) — and proves an ordinary caller
// in the group's owning workspace can still read it end to end through the
// dual-read path, with no locator involved.
func TestLegacySeededGroupStillReadableViaGetEnvGroup(t *testing.T) {
	store := newFakeStore()
	resolver := multiWorkspace{"dana": {"tea-a"}}
	svc := &Service{
		Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: resolver},
		Store: store,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	gid := "evg-d7a1g900000000000097"

	if err := store.Put(legacyCtx(context.Background()), metaPath(gid), map[string]string{
		"name": "legacy-shared", "workspace": "tea-a", "createdAt": "t0", "updatedAt": "t0",
	}); err != nil {
		t.Fatalf("seed legacy meta: %v", err)
	}
	if err := store.Put(legacyCtx(context.Background()), envPath(gid), map[string]string{"K": "v"}); err != nil {
		t.Fatalf("seed legacy env: %v", err)
	}

	got, err := svc.GetEnvGroup(ctx, gid)
	if err != nil {
		t.Fatalf("GetEnvGroup(legacy-resident group): %v", err)
	}
	if got.Name != "legacy-shared" || got.OwnerID != "tea-a" {
		t.Fatalf("unmigrated legacy group view = %+v", got)
	}
	value, err := svc.GetEnvGroupVar(ctx, gid, "K")
	if err != nil || value.Value != "v" {
		t.Fatalf("unmigrated legacy group content reveal: %+v err=%v", value, err)
	}
}
