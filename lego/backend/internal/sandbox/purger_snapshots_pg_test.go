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

package sandbox

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w22/017: a workspace deleted while one of its sandboxes was suspended left the
// sandbox's image in the registry, because `sandbox_storage_objects` cascades
// away with the tenant row and no reap_pending row ever asked the reaper. These
// tests run against the real control-plane Postgres, whose rows the purge reads.

type purgeFixture struct {
	st       *store.PGStore
	registry *memoryRegistry
	purger   *WorkspacePurger
	ws       string
	other    string
}

func newPurgeFixture(t *testing.T) *purgeFixture {
	t.Helper()
	st := openGroupsPG(t)
	mk := func(label string) string {
		t.Helper()
		stamp := time.Now().UnixNano()
		tenant, err := st.CreateWorkspace(context.Background(), fmt.Sprintf("purge-%s-%d", label, stamp), store.PlanHobby, fmt.Sprintf("purge-owner-%s-%d", label, stamp))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
		return tenant.ID
	}
	registry := newMemoryRegistry(t)
	return &purgeFixture{
		st: st, registry: registry, ws: mk("a"), other: mk("b"),
		purger: &WorkspacePurger{Reaper: registry.reaper(), Storage: st},
	}
}

// retain records a rootfs object of workspace ws for the sandbox whose substrate
// id is ref, and puts its image under the controller's repository layout.
func (f *purgeFixture) retain(t *testing.T, ws, object, ref string, tags ...string) string {
	t.Helper()
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if err := f.st.StartSandboxStorage(context.Background(), ws, object, store.StorageKindRootfs, ref, at); err != nil {
		t.Fatal(err)
	}
	repo := fmt.Sprintf("snapshots/%s-sandbox/%s-sandbox", ws, ref)
	f.registry.add(repo, tags...)
	return repo
}

func (f *purgeFixture) objects(t *testing.T, ws string) int {
	t.Helper()
	var n int
	if err := f.st.Pool.QueryRow(context.Background(), `SELECT count(*) FROM sandbox_storage_objects WHERE workspace_id = $1`, ws).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// A workspace with a suspended sandbox, one that was resumed and is running with
// its older tags still in the registry, and one whose image already waits for the
// reaper leaves no snap-gen tag after the purge; another workspace's repository,
// and a tag the controller did not write, are untouched. The tenant-row cascade
// that follows takes the objects with it, which is the leak the purge closes.
func TestPurgeWorkspaceDeletesSuspendedSandboxImages(t *testing.T) {
	f := newPurgeFixture(t)
	suspended := f.retain(t, f.ws, "sbx-suspended", "os-susp", "snap-gen1", "snap-gen2", "keep-me")
	resumed := f.retain(t, f.ws, "sbx-resumed", "os-resumed", "snap-gen3")
	queued := f.retain(t, f.ws, "sbx-queued", "os-queued", "snap-gen9")
	if _, err := f.st.Pool.Exec(context.Background(), `UPDATE sandbox_storage_objects SET ended_at = now(), reap_pending = true WHERE workspace_id = $1 AND object_id = 'sbx-queued'`, f.ws); err != nil {
		t.Fatal(err)
	}
	// A ref that no repository can carry is skipped, not a purge that never ends.
	f.retain(t, f.ws, "sbx-unsafe", "../not/a ref")
	neighbour := f.retain(t, f.other, "sbx-neighbour", "os-neighbour", "snap-gen1", "snap-gen2")

	if err := f.purger.PurgeWorkspace(context.Background(), f.ws); err != nil {
		t.Fatalf("purge: %v", err)
	}
	for _, repo := range []string{suspended, resumed, queued} {
		for _, tag := range f.registry.tags(repo) {
			if tag != "keep-me" {
				t.Errorf("%s still has %s after the purge", repo, tag)
			}
		}
	}
	if got := f.registry.tags(suspended); !slices.Equal(got, []string{"keep-me"}) {
		t.Errorf("%s = %v, want only the tag the controller did not write", suspended, got)
	}
	if got := f.registry.tags(neighbour); !slices.Equal(got, []string{"snap-gen1", "snap-gen2"}) {
		t.Errorf("another workspace's repository = %v, want it untouched", got)
	}

	// The delete that follows the purge drops the objects; the registry is clean.
	if f.objects(t, f.ws) == 0 {
		t.Fatal("the fixture recorded no objects, so the test proves nothing")
	}
	if err := f.st.DeleteTenant(context.Background(), f.ws); err != nil {
		t.Fatal(err)
	}
	if f.objects(t, f.ws) != 0 {
		t.Fatal("the tenant delete left the objects")
	}
	for repo, tags := range f.registry.everyTag() {
		for _, tag := range tags {
			if snapshotTag.MatchString(tag) && repo != neighbour {
				t.Errorf("%s:%s is left in the registry with its workspace deleted", repo, tag)
			}
		}
	}
}

// A registry failure fails the purge, so the workspace delete stops before the
// tenant row goes and is retried; the retry finishes, deleting what the failed
// attempt did not. Nothing is skipped silently.
func TestPurgeWorkspaceRetriesAfterARegistryFailure(t *testing.T) {
	f := newPurgeFixture(t)
	first := f.retain(t, f.ws, "sbx-1", "os-1", "snap-gen1", "snap-gen2")
	second := f.retain(t, f.ws, "sbx-2", "os-2", "snap-gen1")

	f.registry.setFailDeletes(true)
	if err := f.purger.PurgeWorkspace(context.Background(), f.ws); err == nil {
		t.Fatal("a refused registry delete must fail the purge, or the workspace delete would go on and leak the image")
	}
	if len(f.registry.tags(first))+len(f.registry.tags(second)) != 3 {
		t.Fatalf("tags changed while every delete failed: %v %v", f.registry.tags(first), f.registry.tags(second))
	}

	f.registry.setFailDeletes(false)
	if err := f.purger.PurgeWorkspace(context.Background(), f.ws); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if left := append(f.registry.tags(first), f.registry.tags(second)...); len(left) != 0 {
		t.Errorf("the retry left %v", left)
	}
	// Both sandboxes are tried on every attempt: one failing repository does not
	// hide the others' images from a later one.
	f.retain(t, f.ws, "sbx-3", "os-3", "snap-gen1")
	f.retain(t, f.ws, "sbx-4", "os-4", "snap-gen1")
	f.registry.deny(fmt.Sprintf("snapshots/%s-sandbox/os-3-sandbox", f.ws))
	if err := f.purger.PurgeWorkspace(context.Background(), f.ws); err == nil {
		t.Fatal("a refused delete in one repository must fail the purge")
	}
	if got := f.registry.tags(fmt.Sprintf("snapshots/%s-sandbox/os-4-sandbox", f.ws)); len(got) != 0 {
		t.Errorf("one refused repository stopped the purge from reaching the next: %v", got)
	}
}

// Without a reaper (no registry credential with the delete action) the purge is
// the no-op it was: it reads nothing and calls nothing, and the workspace
// deletes. Without a store to list from it is the same.
func TestPurgeWorkspaceWithoutAReaperLeavesTheRegistryAlone(t *testing.T) {
	f := newPurgeFixture(t)
	repo := f.retain(t, f.ws, "sbx-1", "os-1", "snap-gen1")
	for name, p := range map[string]*WorkspacePurger{
		"no reaper":  {Storage: f.st},
		"no storage": {Reaper: f.registry.reaper()},
	} {
		if err := p.PurgeWorkspace(context.Background(), f.ws); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if got := f.registry.tags(repo); !slices.Equal(got, []string{"snap-gen1"}) {
		t.Errorf("tags = %v, want the registry untouched", got)
	}
	if len(f.registry.deletes()) != 0 {
		t.Errorf("deletes = %v", f.registry.deletes())
	}
}
