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

package environments

import (
	"context"
	"errors"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func datastorePlacementService(t *testing.T, hooks interceptor.Funcs) (*Service, client.Client, *fakeStore) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	var objects []client.Object
	for _, member := range []struct{ name, tenant, env string }{
		{"member", "tea-a", "evm-delete"},
		{"other", "tea-a", "evm-other"},
		{"foreign", "tea-b", "evm-delete"},
	} {
		meta := metav1.ObjectMeta{Name: member.name, Namespace: "default", Labels: map[string]string{
			core.LabelTenant: member.tenant, core.LabelProject: "prj-1", core.LabelEnvironment: member.env,
		}}
		objects = append(objects,
			&appv1alpha1.Database{ObjectMeta: meta, Spec: appv1alpha1.DatabaseSpec{
				IPAllowList:            []appv1alpha1.IPAllowEntry{{CIDR: "192.0.2.0/24", Description: "resource rule"}},
				EnvironmentIPAllowList: []string{"10.0.0.0/8"},
			}},
			&appv1alpha1.KeyValue{ObjectMeta: *meta.DeepCopy(), Spec: appv1alpha1.KeyValueSpec{
				IPAllowList:            []appv1alpha1.IPAllowEntry{{CIDR: "192.0.2.0/24", Description: "resource rule"}},
				EnvironmentIPAllowList: []string{"10.0.0.0/8"},
			}},
		)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).WithInterceptorFuncs(hooks).Build()
	base := &core.Base{Authz: allowChecker{}, Client: cl, Namespace: "default"}
	st := newFakeStore()
	st.addProject(store.Project{ID: "prj-1", TenantID: "tea-a"})
	st.envs["evm-delete"] = store.Environment{ID: "evm-delete", ProjectID: "prj-1", TenantID: "tea-a"}
	st.envs["evm-other"] = store.Environment{ID: "evm-other", ProjectID: "prj-1", TenantID: "tea-a"}
	return &Service{Base: base, Store: st, Databases: &postgres.Service{Base: base}, KeyValues: &keyvalue.Service{Base: base}}, cl, st
}

func assertDatastorePlacement(t *testing.T, cl client.Client, obj client.Object, name, projectID, environmentID string, inherited []string) {
	t.Helper()
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "default", Name: name}, obj); err != nil {
		t.Fatal(err)
	}
	if labels := obj.GetLabels(); labels[core.LabelProject] != projectID || labels[core.LabelEnvironment] != environmentID {
		t.Errorf("%T %s placement = %v, want project %q environment %q", obj, name, labels, projectID, environmentID)
	}
	var own []appv1alpha1.IPAllowEntry
	var layer []string
	switch v := obj.(type) {
	case *appv1alpha1.Database:
		own, layer = v.Spec.IPAllowList, v.Spec.EnvironmentIPAllowList
	case *appv1alpha1.KeyValue:
		own, layer = v.Spec.IPAllowList, v.Spec.EnvironmentIPAllowList
	}
	if !reflect.DeepEqual(layer, inherited) {
		t.Errorf("%T %s inherited rules = %v, want %v", obj, name, layer, inherited)
	}
	wantOwn := []appv1alpha1.IPAllowEntry{{CIDR: "192.0.2.0/24", Description: "resource rule"}}
	if !reflect.DeepEqual(own, wantOwn) {
		t.Errorf("%T %s resource rules changed: %v", obj, name, own)
	}
}

func TestDeleteClearsDatastoreEnvironmentPreservingProjectAndOwnRules(t *testing.T) {
	svc, cl, st := datastorePlacementService(t, interceptor.Funcs{})
	if err := svc.Delete(ctxAs("user-a"), "evm-delete"); err != nil {
		t.Fatal(err)
	}
	for _, obj := range []client.Object{&appv1alpha1.Database{}, &appv1alpha1.KeyValue{}} {
		assertDatastorePlacement(t, cl, obj, "member", "prj-1", "", nil)
		assertDatastorePlacement(t, cl, obj, "other", "prj-1", "evm-other", []string{"10.0.0.0/8"})
		assertDatastorePlacement(t, cl, obj, "foreign", "prj-1", "evm-delete", []string{"10.0.0.0/8"})
	}
	if _, err := st.GetEnvironment(t.Context(), "evm-delete"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted environment still exists: %v", err)
	}
}

func TestDeleteDatastoreCleanupRetriesAfterWriteFailure(t *testing.T) {
	boom := errors.New("injected key-value write failure")
	fail := true
	svc, cl, st := datastorePlacementService(t, interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if _, ok := obj.(*appv1alpha1.KeyValue); ok && obj.GetName() == "member" && fail {
				fail = false
				return boom
			}
			return c.Patch(ctx, obj, patch, opts...)
		},
	})
	if err := svc.Delete(ctxAs("user-a"), "evm-delete"); !errors.Is(err, boom) {
		t.Fatalf("Delete = %v, want write failure", err)
	}
	if _, err := st.GetEnvironment(t.Context(), "evm-delete"); err != nil {
		t.Fatalf("failed cleanup removed the environment: %v", err)
	}
	assertDatastorePlacement(t, cl, &appv1alpha1.Database{}, "member", "prj-1", "", nil)
	assertDatastorePlacement(t, cl, &appv1alpha1.KeyValue{}, "member", "prj-1", "evm-delete", []string{"10.0.0.0/8"})
	var db appv1alpha1.Database
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "default", Name: "member"}, &db); err != nil {
		t.Fatal(err)
	}
	db.Labels[core.LabelEnvironment] = "evm-other"
	db.Spec.EnvironmentIPAllowList = []string{"203.0.113.0/24"}
	if err := cl.Update(t.Context(), &db); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctxAs("user-a"), "evm-delete"); err != nil {
		t.Fatalf("retry Delete: %v", err)
	}
	assertDatastorePlacement(t, cl, &appv1alpha1.Database{}, "member", "prj-1", "evm-other", []string{"203.0.113.0/24"})
	assertDatastorePlacement(t, cl, &appv1alpha1.KeyValue{}, "member", "prj-1", "", nil)
}

func reassignDatastoreAfterList(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
	if err := c.List(ctx, list, opts...); err != nil {
		return err
	}
	var moved client.Object
	switch list.(type) {
	case *appv1alpha1.DatabaseList:
		moved = &appv1alpha1.Database{}
	case *appv1alpha1.KeyValueList:
		moved = &appv1alpha1.KeyValue{}
	default:
		return nil
	}
	if err := c.Get(ctx, client.ObjectKey{Namespace: "default", Name: "member"}, moved); err != nil {
		return err
	}
	// Move once. Subsequent response reads must not hide a stale fan-out write.
	if moved.GetLabels()[core.LabelEnvironment] != "evm-delete" {
		return nil
	}
	moved.GetLabels()[core.LabelEnvironment] = "evm-other"
	switch v := moved.(type) {
	case *appv1alpha1.Database:
		v.Spec.EnvironmentIPAllowList = []string{"203.0.113.0/24"}
	case *appv1alpha1.KeyValue:
		v.Spec.EnvironmentIPAllowList = []string{"203.0.113.0/24"}
	}
	return c.Update(ctx, moved)
}

func TestDeleteDatastoreCleanupPreservesReassignmentAfterList(t *testing.T) {
	svc, cl, _ := datastorePlacementService(t, interceptor.Funcs{List: reassignDatastoreAfterList})
	if err := svc.Delete(ctxAs("user-a"), "evm-delete"); err != nil {
		t.Fatal(err)
	}
	for _, obj := range []client.Object{&appv1alpha1.Database{}, &appv1alpha1.KeyValue{}} {
		assertDatastorePlacement(t, cl, obj, "member", "prj-1", "evm-other", []string{"203.0.113.0/24"})
	}
}

func TestSetACLPreservesDatastoreReassignmentAfterList(t *testing.T) {
	svc, cl, _ := datastorePlacementService(t, interceptor.Funcs{List: reassignDatastoreAfterList})
	if _, err := svc.SetACL(ctxAs("user-a"), "evm-delete", ProtectedStatusUnprotected, false, []core.IPAllowListEntry{{CIDRBlock: "172.16.0.0/12"}}); err != nil {
		t.Fatal(err)
	}
	for _, obj := range []client.Object{&appv1alpha1.Database{}, &appv1alpha1.KeyValue{}} {
		assertDatastorePlacement(t, cl, obj, "member", "prj-1", "evm-other", []string{"203.0.113.0/24"})
	}
}
