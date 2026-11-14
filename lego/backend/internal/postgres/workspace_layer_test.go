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

package postgres

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// workspaceRules is a core.WorkspaceIPRulesReader over fixed rows.
type workspaceRules map[string][]core.IPAllowListEntry

func (w workspaceRules) WorkspaceIPAllowList(_ context.Context, id string) ([]core.IPAllowListEntry, bool, error) {
	entries, ok := w[id]
	return entries, ok, nil
}

// w9/m388: the workspace fan-out's Postgres write projects only the
// workspace layer — each database's own rules and environment layer stay as
// they were — onto every database of the workspace and no other, patching
// from the listed object and re-reading only when a concurrent update wins
// the race. A database already carrying the layer is not written.
func TestSetWorkspaceIPAllowListProjectsOnlyTheWorkspaceLayer(t *testing.T) {
	other := sampleDatabase("other-ws")
	other.Labels = map[string]string{core.LabelTenant: "tea-b"}
	svc, cl := newService(placementResource(), other)
	before := readPlacement(t, cl)
	conflicts := 1
	tracking := &placementPatchClient{Client: cl}
	tracking.beforeEach = func() {
		if conflicts > 0 {
			conflicts--
			// An unrelated write lands between the list and the patch.
			bumped := readPlacement(t, cl)
			bumped.Annotations = map[string]string{"unrelated": "write"}
			if err := cl.Update(context.Background(), bumped); err != nil {
				t.Fatal(err)
			}
		}
	}
	svc.Client = tracking
	layer := []string{"192.0.2.0/24"}
	if err := svc.SetWorkspaceIPAllowList(ctxAs("user-a"), "tea-a", layer); err != nil {
		t.Fatalf("SetWorkspaceIPAllowList: %v", err)
	}
	got := readPlacement(t, cl)
	if tracking.patches != 2 || !slices.Equal(got.Spec.WorkspaceIPAllowList, layer) {
		t.Fatalf("patches=%d layer=%v; want the conflicted patch retried once", tracking.patches, got.Spec.WorkspaceIPAllowList)
	}
	if got.Annotations["unrelated"] != "write" {
		t.Fatal("the retried patch lost the concurrent write")
	}
	got.Spec.WorkspaceIPAllowList = nil
	if !reflect.DeepEqual(got.Spec, before.Spec) || !reflect.DeepEqual(got.Labels, before.Labels) {
		t.Fatalf("the workspace layer write changed the database's own configuration: %+v", got.Spec)
	}
	var foreign appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "other-ws"}, &foreign); err != nil || foreign.Spec.WorkspaceIPAllowList != nil {
		t.Fatalf("another workspace's database got %v (err %v)", foreign.Spec.WorkspaceIPAllowList, err)
	}
	// Re-sending the same layer writes nothing; nil clears it.
	tracking.patches = 0
	if err := svc.SetWorkspaceIPAllowList(ctxAs("user-a"), "tea-a", layer); err != nil || tracking.patches != 0 {
		t.Fatalf("unchanged layer: err=%v patches=%d", err, tracking.patches)
	}
	if err := svc.SetWorkspaceIPAllowList(ctxAs("user-a"), "tea-a", nil); err != nil || readPlacement(t, cl).Spec.WorkspaceIPAllowList != nil {
		t.Fatalf("clearing: err=%v layer=%v", err, readPlacement(t, cl).Spec.WorkspaceIPAllowList)
	}
	// A persistent failure surfaces; a caller without the relation is refused
	// before anything is written.
	tracking.fail = apierrors.NewInternalError(errors.New("apiserver down"))
	if err := svc.SetWorkspaceIPAllowList(ctxAs("user-a"), "tea-a", layer); err == nil {
		t.Fatal("a failed patch reported success")
	}
	tracking.fail = nil
	svc.Authz = &fakeChecker{allow: false}
	if err := svc.SetWorkspaceIPAllowList(ctxAs("user-a"), "tea-a", layer); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("denied caller: %v", err)
	}
	if readPlacement(t, cl).Spec.WorkspaceIPAllowList != nil {
		t.Fatal("a refused write changed the layer")
	}
}

// Every Postgres create is born inside its workspace's rules (w9/m388): the
// saved CIDRs, the deny-all placeholder for a saved empty list, nothing for a
// workspace that never saved rules. A recovery is a create in the source's
// workspace.
func TestCreatePostgresStampsTheWorkspaceLayer(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rules workspaceRules
		want  []string
	}{
		{"no rules", workspaceRules{}, nil},
		{"saved rules", workspaceRules{"tea-a": {{CIDRBlock: "203.0.113.0/24", Description: "office"}}}, []string{"203.0.113.0/24"}},
		{"saved deny-all", workspaceRules{"tea-a": {}}, []string{core.DenyAllCIDR}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService()
			svc.Authz = &fakeChecker{allow: true}
			svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
			svc.WorkspaceIPRules = tc.rules
			view, err := svc.CreatePostgres(ctxAs("user-a"), CreatePostgresRequest{Name: "db1"})
			if err != nil {
				t.Fatalf("CreatePostgres: %v", err)
			}
			var d appv1alpha1.Database
			if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "tea-a", Name: view.ID}, &d); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(d.Spec.WorkspaceIPAllowList, tc.want) || (tc.want == nil) != (d.Spec.WorkspaceIPAllowList == nil) {
				t.Fatalf("created layer = %#v, want %#v", d.Spec.WorkspaceIPAllowList, tc.want)
			}
		})
	}
}

func TestRecoverStampsTheWorkspaceLayer(t *testing.T) {
	ctx := context.Background()
	svc, cl := newServicePostgresResources()
	svc.WorkspaceIPRules = workspaceRules{"tea-a": {{CIDRBlock: "198.51.100.0/24"}}}
	src := seedDatabaseSpec(t, cl, "src-db", appv1alpha1.DatabaseSpec{Plan: "basic-1gb", Public: true}, true)
	src.Labels = map[string]string{core.LabelTenant: "tea-a"}
	if err := cl.Update(ctx, src); err != nil {
		t.Fatal(err)
	}
	seedBarmanRecoveryWindow(t, cl, "src-db", "2026-07-01T00:00:00Z", "2026-07-10T00:00:00Z")
	v, err := svc.Recover(ctx, "src-db", RecoverRequest{Name: "restored-db", TargetTime: "2026-07-09T10:00:00Z"})
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	var made appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: v.ID}, &made); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(made.Spec.WorkspaceIPAllowList, []string{"198.51.100.0/24"}) {
		t.Fatalf("recovered layer = %v, want the source workspace's", made.Spec.WorkspaceIPAllowList)
	}
}
