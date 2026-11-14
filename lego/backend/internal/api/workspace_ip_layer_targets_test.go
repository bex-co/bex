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

package api

import (
	"context"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w9/m388: the datastore services the composition root hands the workspace
// fan-out as its targets each write the layer onto every one of their
// resources in the workspace — and only that workspace — leaving each
// resource's own rules and its environment layer alone.
func TestWorkspaceIPLayerTargetsReachEveryDatastoreOfTheWorkspace(t *testing.T) {
	tenant := func(ws string) map[string]string { return core.TenantLabels(ws) }
	own := []appv1alpha1.IPAllowEntry{{CIDR: "10.0.0.0/8", Description: "own"}}
	cl := fakeClient(
		&appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-a", Namespace: "tea-a", Labels: tenant("tea-a")},
			Spec: appv1alpha1.DatabaseSpec{Name: "a", Plan: "free", IPAllowList: own, EnvironmentIPAllowList: []string{"192.168.0.0/16"}}},
		&appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-b", Namespace: "tea-b", Labels: tenant("tea-b")},
			Spec: appv1alpha1.DatabaseSpec{Name: "b", Plan: "free"}},
		&appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-a", Namespace: "tea-a", Labels: tenant("tea-a")},
			Spec: appv1alpha1.KeyValueSpec{Name: "a", Plan: "free", IPAllowList: own}},
	)
	base := &core.Base{Client: cl, Namespace: "default", Workspace: coretest.Workspaces{"tea-a", "tea-b"}}
	targets := []workspaces.IPLayerTarget{&postgres.Service{Base: base}, &keyvalue.Service{Base: base}}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "admin", Method: "session"})
	layer := []string{"203.0.113.0/24"}
	for _, target := range targets {
		if err := target.SetWorkspaceIPAllowList(ctx, "tea-a", layer); err != nil {
			t.Fatalf("%T: %v", target, err)
		}
	}
	var db appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "tea-a", Name: "dpg-a"}, &db); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(db.Spec.WorkspaceIPAllowList, layer) || !slices.Equal(db.Spec.IPAllowList, own) || !slices.Equal(db.Spec.EnvironmentIPAllowList, []string{"192.168.0.0/16"}) {
		t.Fatalf("Postgres spec = %+v", db.Spec)
	}
	var other appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "tea-b", Name: "dpg-b"}, &other); err != nil || other.Spec.WorkspaceIPAllowList != nil {
		t.Fatalf("another workspace's Postgres got %v (err %v)", other.Spec.WorkspaceIPAllowList, err)
	}
	var kv appv1alpha1.KeyValue
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "tea-a", Name: "red-a"}, &kv); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(kv.Spec.WorkspaceIPAllowList, layer) || !slices.Equal(kv.Spec.IPAllowList, own) {
		t.Fatalf("Key Value spec = %+v", kv.Spec)
	}
}
