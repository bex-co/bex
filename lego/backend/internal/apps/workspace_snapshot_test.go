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
	"strings"
	"sync/atomic"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// preWriteLists is workspaceListCounter until the first cluster write: it
// counts only the lists made before that write.
type preWriteLists struct {
	workspaceListCounter
	wrote atomic.Bool
}

func (c *preWriteLists) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if c.wrote.Load() {
		return c.Client.List(ctx, list, opts...)
	}
	return c.workspaceListCounter.List(ctx, list, opts...)
}

func (c *preWriteLists) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	c.wrote.Store(true)
	return c.Client.Create(ctx, obj, opts...)
}

func (c *preWriteLists) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	c.wrote.Store(true)
	return c.Client.Update(ctx, obj, opts...)
}

func (c *preWriteLists) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	c.wrote.Store(true)
	return c.Client.Patch(ctx, obj, patch, opts...)
}

// TestABlueprintRequestListsEachResourceKindOnceBeforeItWrites (w5/124): the
// action plan, the ownership check, reference resolution and the apply's
// lookups read one snapshot, so a sync and a preview each list the
// workspace's Databases and Key Values once before the first write; they
// listed them two or three times. A create lists them twice: its apply reads
// them again after admission, which serializes the Blueprint's applies. The
// plan, the detachment check and the ownership check read their Apps from it
// too (w5/132), so Apps are listed as often as each datastore kind.
func TestABlueprintRequestListsEachResourceKindOnceBeforeItWrites(t *testing.T) {
	// web reads an existing Postgres the manifest does not declare, so
	// reference resolution reads the Databases too.
	const manifest = `services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1}
    envVars:
      - key: LEGACY_URL
        fromDatabase: {name: legacy, property: connectionString}
  - type: keyvalue
    name: cache
    plan: free
    ipAllowList: []
databases:
  - name: orders
    plan: free
`
	svc, _ := ownershipService(t)
	svc.GitFetcher = fakeBlueprintFetcher{contents: manifest, sha: "abc1234"}
	ctx := ownershipCtx()
	legacy := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-legacy", Namespace: "default", Labels: map[string]string{core.LabelTenant: "tea-a"}},
		Spec:       appv1alpha1.DatabaseSpec{Name: "legacy"},
	}
	if err := svc.Client.Create(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	lists := &preWriteLists{workspaceListCounter: workspaceListCounter{Client: svc.Client}}
	svc.Client = lists
	listed := func(t *testing.T, request string, want int32) {
		t.Helper()
		if apps, databases, keyValues := lists.apps.Load(), lists.databases.Load(), lists.keyValues.Load(); apps != want || databases != want || keyValues != want {
			t.Errorf("%s listed Apps %d, Databases %d and Key Values %d times before its first write, want %d each", request, apps, databases, keyValues, want)
		}
		lists.wrote.Store(false)
		lists.apps.Store(0)
		lists.databases.Store(0)
		lists.keyValues.Store(0)
	}

	bp, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/acme/a", Branch: "main"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	listed(t, "the create", 2)
	preview, err := svc.PreviewBlueprint(ctx, "tea-a", "https://github.com/acme/a", "main", "", bp.ID)
	if err != nil || preview.Validation == nil || !preview.Validation.Valid {
		t.Fatalf("preview = %+v, %v; want a valid preview", preview, err)
	}
	listed(t, "the preview", 1)
	// The sync adds a Key Value, whose create is its first write, ahead of the
	// apply's own per-service lookups (w5/141) and the post-write ownership
	// stamp. It drops the Postgres, so it resolves a detachment.
	const added = `  - type: keyvalue
    name: queue
    plan: free
    ipAllowList: []
`
	synced := strings.Replace(manifest, "  - type: keyvalue", added+"  - type: keyvalue", 1)
	synced = synced[:strings.Index(synced, "databases:")]
	result, err := svc.SyncBlueprint(ctx, bp.ID, "tea-a", synced, "", nil)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(result.DetachedResources) != 1 || result.DetachedResources[0].Name != "orders" {
		t.Fatalf("detached %+v, want the Postgres orders", result.DetachedResources)
	}
	listed(t, "the sync", 1)
}
