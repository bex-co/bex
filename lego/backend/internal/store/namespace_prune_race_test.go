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
	"strings"
	"testing"
)

// bex-api ensures a new workspace's namespace synchronously (w8/071), so a
// workspace created between the two reads must not be pruned.
func TestNamespaceCreatedBetweenSnapshotsIsNotPruned(t *testing.T) {
	ctx := context.Background()
	r, base, cl := newTestNamespaceReconciler(t)
	hooked := &snapshotRaceStore{memStore: base}
	r.Store = hooked
	var tenant Tenant
	hooked.after = func() {
		var err error
		if tenant, err = base.CreateTenant(ctx, "fresh", "free"); err != nil {
			t.Fatal(err)
		}
		if err := r.ensureNamespace(ctx, tenant, RegimeHosting); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if !namespaceExists(t, cl, WorkspaceNamespace(tenant.ID)) {
		t.Fatal("a just-created workspace's namespace was pruned on a stale snapshot")
	}
	// The next pass sees the workspace and converges both regimes.
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if !namespaceExists(t, cl, SandboxNamespace(tenant.ID)) {
		t.Error("next pass did not converge the new workspace")
	}
}

func TestNamespacePruneKeepsNamespaceWhenWorkspaceReadFails(t *testing.T) {
	ctx := context.Background()
	r, base, cl := newTestNamespaceReconciler(t)
	gone, _ := base.CreateTenant(ctx, "gone", "free")
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	base.mu.Lock()
	delete(base.tenants, gone.ID)
	base.mu.Unlock()

	r.Store = &snapshotRaceStore{memStore: base, getErr: errors.New("connection reset")}
	err := r.ReconcileOnce(ctx)
	if err == nil || !strings.Contains(err.Error(), "confirm namespace") {
		t.Fatalf("want a retryable confirm error, got %v", err)
	}
	if !namespaceExists(t, cl, WorkspaceNamespace(gone.ID)) {
		t.Fatal("namespace deleted on an inconclusive workspace read")
	}

	r.Store = base
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if namespaceExists(t, cl, WorkspaceNamespace(gone.ID)) {
		t.Error("true orphan namespace not reclaimed once the read answers")
	}
}
