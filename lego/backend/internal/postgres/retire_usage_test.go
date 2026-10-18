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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeRetirements struct {
	recorded []store.RetiredUsageResource
	err      error
}

func (f *fakeRetirements) RecordRetiredUsageResource(_ context.Context, r store.RetiredUsageResource) error {
	if f.err != nil {
		return f.err
	}
	f.recorded = append(f.recorded, r)
	return nil
}

// w4/m173 t007: usage metering lists live CRs, so deleting one first records
// what the meters need for its final hour — and refuses to delete, as
// unavailable, if that record cannot be written.
func TestDeletePostgresRetiresUsageFirst(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 10, 5, 7, 21, 27, 0, time.UTC))
	fixture := func() *appv1alpha1.Database {
		obj := databaseForProtection("dpg-short", "orders", false)
		obj.Labels[core.LabelTenant] = "tea-short"
		obj.CreationTimestamp = created
		obj.Spec.Public = true
		return obj
	}

	t.Run("recorded, then deleted", func(t *testing.T) {
		obj := fixture()
		svc, cl := newService(obj)
		rec := &fakeRetirements{}
		svc.Retirements = rec
		if err := svc.DeletePostgres(context.Background(), obj.Name); err != nil {
			t.Fatalf("DeletePostgres: %v", err)
		}
		want := store.RetiredUsageResource{ResourceKind: store.ResourceKindPostgres, ResourceID: obj.Name, WorkspaceID: "tea-short",
			Name: obj.Name, Namespace: obj.Namespace, Display: obj.Spec.Name, Public: true, Tier: obj.Spec.Plan, CreatedAt: created.Time}
		if len(rec.recorded) != 1 || rec.recorded[0] != want {
			t.Fatalf("recorded = %+v, want %+v", rec.recorded, want)
		}
		if err := cl.Get(context.Background(), client.ObjectKeyFromObject(obj), &appv1alpha1.Database{}); err == nil {
			t.Fatal("the CR survived its delete")
		}
	})

	t.Run("an unrecordable retirement keeps the resource", func(t *testing.T) {
		obj := fixture()
		svc, cl := newService(obj)
		svc.Retirements = &fakeRetirements{err: errors.New("store down")}
		if err := svc.DeletePostgres(context.Background(), obj.Name); !errors.Is(err, core.ErrUnavailable) {
			t.Fatalf("DeletePostgres without a retirement record = %v, want unavailable", err)
		}
		if err := cl.Get(context.Background(), client.ObjectKeyFromObject(obj), &appv1alpha1.Database{}); err != nil {
			t.Fatalf("the CR was deleted although its usage could not be retained: %v", err)
		}
	})
}
