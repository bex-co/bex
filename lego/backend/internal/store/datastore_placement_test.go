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
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type placementStore struct {
	*memStore
	projects      map[string]Project
	environments  map[string]Environment
	lookupErr     error
	onProjectRead func(string)
}

func (s *placementStore) GetProject(_ context.Context, id string) (Project, error) {
	if s.onProjectRead != nil {
		s.onProjectRead(id)
	}
	if s.lookupErr != nil {
		return Project{}, s.lookupErr
	}
	p, ok := s.projects[id]
	if !ok {
		return Project{}, ErrNotFound
	}
	return p, nil
}

func TestReconcilePlacementResumesAfterInterruptedBudget(t *testing.T) {
	r, st, cl := placementReconciler(t)
	pg := placementFixture("postgres", "tea-a", "prj-a", "env-deleted")
	kv := placementFixture("keyvalue", "tea-a", "prj-b", "env-deleted")
	for _, obj := range []client.Object{pg, kv} {
		if err := cl.Create(t.Context(), obj); err != nil {
			t.Fatal(err)
		}
	}
	for pass := 0; pass < 2; pass++ {
		ctx, cancel := context.WithCancel(t.Context())
		st.onProjectRead = func(id string) {
			if id == "prj-a" {
				cancel()
			}
		}
		if err := r.ReconcileOnce(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("interrupted pass = %v", err)
		}
		cancel()
	}
	if err := cl.Get(t.Context(), client.ObjectKeyFromObject(kv), kv); err != nil {
		t.Fatal(err)
	}
	if kv.GetLabels()[core.LabelEnvironment] != "" {
		t.Fatal("early interrupted resource starved the later datastore")
	}
	if err := cl.Get(t.Context(), client.ObjectKeyFromObject(pg), pg); err != nil {
		t.Fatal(err)
	}
	if pg.GetLabels()[core.LabelEnvironment] != "env-deleted" {
		t.Fatal("canceled repair changed the datastore")
	}
}

func (s *placementStore) GetEnvironment(_ context.Context, id string) (Environment, error) {
	if s.lookupErr != nil {
		return Environment{}, s.lookupErr
	}
	e, ok := s.environments[id]
	if !ok {
		return Environment{}, ErrNotFound
	}
	return e, nil
}

func placementFixture(kind, namespace, project, environment string) client.Object {
	meta := metav1.ObjectMeta{Name: "resource", Namespace: namespace, Labels: core.TenantLabels("tea-a")}
	meta.Labels[core.LabelProject], meta.Labels[core.LabelEnvironment] = project, environment
	own := []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.0/24", Description: "resource-owned"}}
	inherited := []string{"198.51.100.0/24"}
	if kind == "postgres" {
		return &appv1alpha1.Database{ObjectMeta: meta, Spec: appv1alpha1.DatabaseSpec{IPAllowList: own, EnvironmentIPAllowList: inherited}}
	}
	return &appv1alpha1.KeyValue{ObjectMeta: meta, Spec: appv1alpha1.KeyValueSpec{IPAllowList: own, EnvironmentIPAllowList: inherited}}
}

func placementLayers(obj client.Object) ([]appv1alpha1.IPAllowEntry, []string) {
	switch obj := obj.(type) {
	case *appv1alpha1.Database:
		return obj.Spec.IPAllowList, obj.Spec.EnvironmentIPAllowList
	case *appv1alpha1.KeyValue:
		return obj.Spec.IPAllowList, obj.Spec.EnvironmentIPAllowList
	default:
		panic("unexpected test resource")
	}
}

func placementReconciler(t *testing.T) (*Reconciler, *placementStore, client.Client) {
	t.Helper()
	r, st, cl := newTestReconciler(t)
	st.tenants["tea-a"] = Tenant{ID: "tea-a"}
	s := &placementStore{memStore: st, projects: map[string]Project{
		"prj-a": {ID: "prj-a", TenantID: "tea-a"}, "prj-b": {ID: "prj-b", TenantID: "tea-a"}, "prj-foreign": {ID: "prj-foreign", TenantID: "tea-b"},
	}, environments: map[string]Environment{
		"env-a": {ID: "env-a", ProjectID: "prj-a", TenantID: "tea-a"}, "env-b": {ID: "env-b", ProjectID: "prj-b", TenantID: "tea-a"}, "env-foreign": {ID: "env-foreign", ProjectID: "prj-foreign", TenantID: "tea-b"},
	}}
	r.Store, r.Identity = s, "dev-7"
	ns := (&NamespaceReconciler{Identity: "dev-7"}).workspaceNamespaceObject("tea-a", Tenant{ID: "tea-a"}, RegimeHosting)
	if err := cl.Create(t.Context(), ns); err != nil {
		t.Fatal(err)
	}
	return r, s, cl
}

func TestReconcileDatastorePlacement(t *testing.T) {
	for _, kind := range []string{"postgres", "keyvalue"} {
		for _, tc := range []struct {
			name, project, environment, wantProject, wantEnvironment string
			clearLayer, wantErr                                      bool
		}{
			{"deleted environment", "prj-a", "env-deleted", "prj-a", "", true, false},
			{"deleted project", "prj-deleted", "env-deleted", "", "", true, false},
			{"incompatible live environment", "prj-b", "env-a", "prj-b", "", true, false},
			{"unassigned project", "", "env-a", "", "", true, false},
			{"orphaned inherited layer", "prj-a", "", "prj-a", "", true, false},
			{"valid placement", "prj-a", "env-a", "prj-a", "env-a", false, false},
			{"foreign environment", "prj-a", "env-foreign", "prj-a", "env-foreign", false, true},
			{"foreign project", "prj-foreign", "env-a", "prj-foreign", "env-a", false, true},
		} {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				r, _, cl := placementReconciler(t)
				obj := placementFixture(kind, "tea-a", tc.project, tc.environment)
				if err := cl.Create(t.Context(), obj); err != nil {
					t.Fatal(err)
				}
				err := r.ReconcileOnce(t.Context())
				if (err != nil) != tc.wantErr {
					t.Fatalf("reconcile error = %v", err)
				}
				if err := cl.Get(t.Context(), client.ObjectKeyFromObject(obj), obj); err != nil {
					t.Fatal(err)
				}
				own, layer := placementLayers(obj)
				if obj.GetLabels()[core.LabelProject] != tc.wantProject || obj.GetLabels()[core.LabelEnvironment] != tc.wantEnvironment || (len(layer) == 0) != tc.clearLayer {
					t.Fatalf("placement = %v, inherited=%v", obj.GetLabels(), layer)
				}
				if len(own) != 1 || own[0].CIDR != "203.0.113.0/24" || own[0].Description != "resource-owned" {
					t.Fatalf("own layer changed: %v", own)
				}
				rv := obj.GetResourceVersion()
				_ = r.ReconcileOnce(t.Context())
				if err := cl.Get(t.Context(), client.ObjectKeyFromObject(obj), obj); err != nil {
					t.Fatal(err)
				}
				if obj.GetResourceVersion() != rv {
					t.Fatal("repair was not idempotent")
				}
			})
		}
	}
}

func TestReconcilePlacementPreservesUncertainOwnership(t *testing.T) {
	for _, scenario := range []string{"lookup outage", "foreign namespace", "legacy shared namespace", "missing tenant"} {
		t.Run(scenario, func(t *testing.T) {
			r, st, cl := placementReconciler(t)
			obj := placementFixture("postgres", "tea-a", "prj-deleted", "env-deleted")
			switch scenario {
			case "lookup outage":
				st.lookupErr = errors.New("database unavailable")
			case "foreign namespace":
				var ns corev1.Namespace
				if err := cl.Get(t.Context(), client.ObjectKey{Name: "tea-a"}, &ns); err != nil {
					t.Fatal(err)
				}
				ns.Labels[ControlPlaneLabel] = "dev-8"
				if err := cl.Update(t.Context(), &ns); err != nil {
					t.Fatal(err)
				}
			case "legacy shared namespace":
				obj.SetNamespace("default")
			case "missing tenant":
				delete(st.tenants, "tea-a")
			}
			if err := cl.Create(t.Context(), obj); err != nil {
				t.Fatal(err)
			}
			rv := obj.GetResourceVersion()
			if err := r.ReconcileOnce(t.Context()); err == nil {
				t.Fatal("ambiguous placement was not reported")
			}
			if err := cl.Get(t.Context(), client.ObjectKeyFromObject(obj), obj); err != nil {
				t.Fatal(err)
			}
			if obj.GetResourceVersion() != rv {
				t.Fatal("uncertain ownership mutated")
			}
		})
	}
}

type reassignBeforePlacementPatch struct {
	client.Client
	reassigned bool
}

func (c *reassignBeforePlacementPatch) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if !c.reassigned {
		c.reassigned = true
		current := obj.DeepCopyObject().(client.Object)
		if err := c.Client.Get(ctx, client.ObjectKeyFromObject(obj), current); err != nil {
			return err
		}
		current.GetLabels()[core.LabelProject] = "prj-b"
		current.GetLabels()[core.LabelEnvironment] = "env-b"
		switch current := current.(type) {
		case *appv1alpha1.Database:
			current.Spec.EnvironmentIPAllowList = []string{"192.0.2.0/24"}
		case *appv1alpha1.KeyValue:
			current.Spec.EnvironmentIPAllowList = []string{"192.0.2.0/24"}
		}
		if err := c.Client.Update(ctx, current); err != nil {
			return err
		}
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func TestReconcilePlacementDoesNotClearConcurrentReassignment(t *testing.T) {
	for _, kind := range []string{"postgres", "keyvalue"} {
		t.Run(kind, func(t *testing.T) {
			r, _, cl := placementReconciler(t)
			obj := placementFixture(kind, "tea-a", "prj-a", "env-deleted")
			if err := cl.Create(t.Context(), obj); err != nil {
				t.Fatal(err)
			}
			r.Client = &reassignBeforePlacementPatch{Client: cl}
			if err := r.ReconcileOnce(t.Context()); err == nil {
				t.Fatal("expected optimistic conflict")
			}
			if err := r.ReconcileOnce(t.Context()); err != nil {
				t.Fatal(err)
			}
			if err := cl.Get(t.Context(), client.ObjectKeyFromObject(obj), obj); err != nil {
				t.Fatal(err)
			}
			_, layer := placementLayers(obj)
			if obj.GetLabels()[core.LabelProject] != "prj-b" || obj.GetLabels()[core.LabelEnvironment] != "env-b" || !slices.Equal(layer, []string{"192.0.2.0/24"}) {
				t.Fatalf("concurrent placement lost: %v %v", obj.GetLabels(), layer)
			}
		})
	}
}
