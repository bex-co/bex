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

package keyvalue

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type workspaceRules map[string][]core.IPAllowListEntry

func (w workspaceRules) WorkspaceIPAllowList(_ context.Context, id string) ([]core.IPAllowListEntry, bool, error) {
	entries, ok := w[id]
	return entries, ok, nil
}

// w9/m388: the workspace fan-out's Key Value write projects only the
// workspace layer onto the workspace's stores, retries a patch lost to a
// concurrent update, restarts nothing, and refuses a caller without the
// relation.
func TestSetWorkspaceIPAllowListProjectsOnlyTheWorkspaceLayer(t *testing.T) {
	svc, cl := newService(placementResource())
	before := readPlacement(t, cl)
	conflicts := 1
	tracking := &placementPatchClient{Client: cl}
	tracking.beforeEach = func() {
		if conflicts > 0 {
			conflicts--
			bumped := readPlacement(t, cl)
			bumped.Annotations = map[string]string{"unrelated": "write"}
			if err := cl.Update(context.Background(), bumped); err != nil {
				t.Fatal(err)
			}
		}
	}
	svc.Client = tracking
	layer := []string{"192.0.2.0/24"}
	ws := before.Labels[core.LabelTenant]
	if err := svc.SetWorkspaceIPAllowList(ctxAs("user-a"), ws, layer); err != nil {
		t.Fatalf("SetWorkspaceIPAllowList: %v", err)
	}
	got := readPlacement(t, cl)
	if tracking.patches != 2 || !slices.Equal(got.Spec.WorkspaceIPAllowList, layer) {
		t.Fatalf("patches=%d layer=%v; want the conflicted patch retried once", tracking.patches, got.Spec.WorkspaceIPAllowList)
	}
	if _, restartFree := got.Annotations[annotationRestartFreeGeneration]; !restartFree {
		t.Fatalf("a workspace-layer change was not marked restart-free: %v", got.Annotations)
	}
	got.Spec.WorkspaceIPAllowList = nil
	if !reflect.DeepEqual(got.Spec, before.Spec) || !reflect.DeepEqual(got.Labels, before.Labels) {
		t.Fatalf("the workspace layer write changed the store's own configuration: %+v", got.Spec)
	}
	if err := svc.SetWorkspaceIPAllowList(ctxAs("user-a"), "tea-other", nil); err != nil || !slices.Equal(readPlacement(t, cl).Spec.WorkspaceIPAllowList, layer) {
		t.Fatalf("another workspace's change reached this store: err=%v layer=%v", err, readPlacement(t, cl).Spec.WorkspaceIPAllowList)
	}
	svc.Authz = &fakeChecker{allow: false}
	if err := svc.SetWorkspaceIPAllowList(ctxAs("user-a"), ws, nil); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("denied caller: %v", err)
	}
	if !slices.Equal(readPlacement(t, cl).Spec.WorkspaceIPAllowList, layer) {
		t.Fatal("a refused write changed the layer")
	}
}

// Every Key Value create is born inside its workspace's rules.
func TestCreateKeyValueStampsTheWorkspaceLayer(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rules workspaceRules
		want  []string
	}{
		{"no rules", workspaceRules{}, nil},
		{"saved rules", workspaceRules{"tea-a": {{CIDRBlock: "203.0.113.0/24"}}}, []string{"203.0.113.0/24"}},
		{"saved deny-all", workspaceRules{"tea-a": {}}, []string{core.DenyAllCIDR}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService()
			svc.Authz = &fakeChecker{allow: true}
			svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
			svc.WorkspaceIPRules = tc.rules
			view, err := svc.CreateKeyValue(ctxAs("user-a"), CreateKeyValueRequest{Name: "kv1"})
			if err != nil {
				t.Fatalf("CreateKeyValue: %v", err)
			}
			var kv appv1alpha1.KeyValue
			if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "tea-a", Name: view.ID}, &kv); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(kv.Spec.WorkspaceIPAllowList, tc.want) || (tc.want == nil) != (kv.Spec.WorkspaceIPAllowList == nil) {
				t.Fatalf("created layer = %#v, want %#v", kv.Spec.WorkspaceIPAllowList, tc.want)
			}
		})
	}
}
