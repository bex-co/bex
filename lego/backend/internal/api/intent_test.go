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
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// recordingStore fakes IntentStore, capturing SetAppSuspended calls.
type recordingStore struct {
	calls []struct {
		id        string
		suspended bool
	}
	err error
}

func (r *recordingStore) SetAppSuspended(_ context.Context, id string, suspended bool) error {
	if r.err != nil {
		return r.err
	}
	r.calls = append(r.calls, struct {
		id        string
		suspended bool
	}{id, suspended})
	return nil
}

func intentCore(t *testing.T, st IntentStore, apps ...*appv1alpha1.App) (*Core, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	objs := make([]client.Object, len(apps))
	for i, a := range apps {
		objs[i] = a
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &Core{Client: cl, Namespace: "default", Store: st}, cl
}

func managedApp(name, appID string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Labels = map[string]string{
		store.LabelManagedBy: store.ManagedByValue,
		store.LabelAppID:     appID,
	}
	return a
}

// Suspend/Resume on a store-managed App must write the row first (the
// projection loop owns spec.suspended and would revert a bare CR patch),
// then patch the CR.
func TestSuspendManagedAppWritesRowThenCR(t *testing.T) {
	rec := &recordingStore{}
	core, cl := intentCore(t, rec, managedApp("web", "srv-1"))

	if _, err := core.Suspend(context.Background(), "web"); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if _, err := core.Resume(context.Background(), "web"); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	if len(rec.calls) != 2 || rec.calls[0].id != "srv-1" || !rec.calls[0].suspended ||
		rec.calls[1].id != "srv-1" || rec.calls[1].suspended {
		t.Fatalf("want row writes [srv-1 true, srv-1 false], got %v", rec.calls)
	}
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &a); err != nil {
		t.Fatalf("get: %v", err)
	}
	if a.Spec.Suspended {
		t.Fatal("CR should be resumed after the fast-path patch")
	}
}

// A hand-applied App (no app-id label) has no row — the CR patch is the only
// write even when a store is wired.
func TestSuspendUnmanagedAppSkipsStore(t *testing.T) {
	rec := &recordingStore{}
	core, cl := intentCore(t, rec, sampleApp("hand"))

	if _, err := core.Suspend(context.Background(), "hand"); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	if len(rec.calls) != 0 {
		t.Fatalf("unmanaged app must not touch the store, got %v", rec.calls)
	}
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "hand"}, &a); err != nil {
		t.Fatalf("get: %v", err)
	}
	if !a.Spec.Suspended {
		t.Fatal("CR should be suspended")
	}
}

// Row write failing means no CR patch: the row is the truth, and patching the
// CR anyway would hand the projection loop a revert to perform.
func TestSuspendRowWriteFailureLeavesCRUntouched(t *testing.T) {
	rec := &recordingStore{err: errors.New("db down")}
	core, cl := intentCore(t, rec, managedApp("web", "srv-1"))

	if _, err := core.Suspend(context.Background(), "web"); err == nil {
		t.Fatal("want error when the row write fails")
	}
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &a); err != nil {
		t.Fatalf("get: %v", err)
	}
	if a.Spec.Suspended {
		t.Fatal("CR must not be patched when the row write failed")
	}
}

// Nil store (tests, mcp-stdio mode) keeps the legacy CR-only behavior.
func TestSuspendWithoutStorePatchesCR(t *testing.T) {
	core, cl := intentCore(t, nil, managedApp("web", "srv-1"))

	if _, err := core.Suspend(context.Background(), "web"); err != nil {
		t.Fatalf("Suspend: %v", err)
	}
	var a appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &a); err != nil {
		t.Fatalf("get: %v", err)
	}
	if !a.Spec.Suspended {
		t.Fatal("CR should be suspended")
	}
}
