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
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestWorkspacePurger_DeletesOnlyTheGivenTenantsApps(t *testing.T) {
	mine := sampleApp("mine")
	mine.Labels = map[string]string{core.LabelTenant: "tea-a"}
	other := sampleApp("other")
	other.Labels = map[string]string{core.LabelTenant: "tea-b"}
	svc, cl := newService(nil, mine, other)
	purger := &WorkspacePurger{Service: svc}

	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("PurgeWorkspace: %v", err)
	}

	var list appv1alpha1.AppList
	if err := cl.List(context.Background(), &list); err != nil {
		t.Fatalf("list Apps: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].Name != "other" {
		t.Fatalf("Apps after purge = %+v, want only tea-b's \"other\" left", list.Items)
	}
}

// TestWorkspacePurger_CatchesAppsWithNoManagedByLabel is the regression case
// for the w6/m11 live-verification finding: an App created via the public
// REST/GraphQL/MCP surface carries core.LabelTenant only (no
// store.LabelManagedBy), so it must still be purged on workspace delete even
// though the row-backed reconciler's own prune sweep would never see it.
func TestWorkspacePurger_CatchesAppsWithNoManagedByLabel(t *testing.T) {
	orphanProne := sampleApp("hand-applied")
	orphanProne.Labels = map[string]string{core.LabelTenant: "tea-a"} // no managed-by label, unlike a row-backed App
	svc, cl := newService(nil, orphanProne)
	purger := &WorkspacePurger{Service: svc}

	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("PurgeWorkspace: %v", err)
	}

	var list appv1alpha1.AppList
	if err := cl.List(context.Background(), &list, client.MatchingLabels{core.LabelTenant: "tea-a"}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("want the unmanaged App gone, got %+v", list.Items)
	}
}

func TestWorkspacePurger_NoMatchingAppsIsANoOp(t *testing.T) {
	svc, _ := newService(nil, sampleApp("unrelated"))
	purger := &WorkspacePurger{Service: svc}

	if err := purger.PurgeWorkspace(context.Background(), "tea-never-existed"); err != nil {
		t.Fatalf("PurgeWorkspace on an unowned tenant should be a no-op, got: %v", err)
	}
}

func TestWorkspacePurger_SecondPurgeIsANoOp(t *testing.T) {
	a := sampleApp("mine")
	a.Labels = map[string]string{core.LabelTenant: "tea-a"}
	svc, cl := newService(nil, a)
	purger := &WorkspacePurger{Service: svc}

	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("first purge: %v", err)
	}
	if err := purger.PurgeWorkspace(context.Background(), "tea-a"); err != nil {
		t.Fatalf("second purge (already-gone) should be a no-op, got: %v", err)
	}
	var list appv1alpha1.AppList
	if err := cl.List(context.Background(), &list, client.MatchingLabels{core.LabelTenant: "tea-a"}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("want no tea-a Apps left, got %+v", list.Items)
	}
}
