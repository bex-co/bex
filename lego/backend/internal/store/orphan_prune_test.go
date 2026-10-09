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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// afterDesiredStore runs a hook once, right after ListDesiredApps took its
// snapshot — the window between the desired-row read and the CR list.
type afterDesiredStore struct {
	*memStore
	after  func()
	getErr error
}

func (s *afterDesiredStore) ListDesiredApps(ctx context.Context) ([]DesiredApp, error) {
	snapshot, err := s.memStore.ListDesiredApps(ctx)
	if err == nil && s.after != nil {
		after := s.after
		s.after = nil
		after()
	}
	return snapshot, err
}

func (s *afterDesiredStore) GetApp(ctx context.Context, id string) (App, error) {
	if s.getErr != nil {
		return App{}, s.getErr
	}
	return s.memStore.GetApp(ctx, id)
}

// publishBetweenSnapshots creates a row plus its complete App (the CR-only
// settings writeNewApp seeds) after the reconciler's desired snapshot.
func publishBetweenSnapshots(t *testing.T, r *Reconciler, base *memStore, cl client.Client) *client.ObjectKey {
	t.Helper()
	ctx := context.Background()
	hooked := &afterDesiredStore{memStore: base}
	r.Store = hooked
	tenant, err := base.CreateTenant(ctx, "acme", "free")
	if err != nil {
		t.Fatal(err)
	}
	key := &client.ObjectKey{}
	hooked.after = func() {
		row, err := base.CreateApp(ctx, App{TenantID: tenant.ID, Name: "new-web", Image: "img", Port: 3000, Replicas: 1, Tier: "free"})
		if err != nil {
			t.Fatal(err)
		}
		app := r.projectApp(ctx, DesiredApp{App: row, TenantName: tenant.Name})
		app.UID = types.UID("uid-original")
		app.Spec.HealthCheckPath = "/"
		app.Spec.PreDeployCommand = "printf ok"
		app.Spec.EnvFromSecret = "owned-env"
		*key = client.ObjectKeyFromObject(app)
		if err := cl.Create(ctx, app); err != nil {
			t.Fatal(err)
		}
	}
	return key
}

func TestNewAppCreatedBetweenSnapshotsKeepsItsCompleteSpec(t *testing.T) {
	ctx := context.Background()
	r, base, cl := newTestReconciler(t)
	key := publishBetweenSnapshots(t, r, base, cl)

	for pass := 1; pass <= 2; pass++ {
		if err := r.ReconcileOnce(ctx); err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
		var got appv1alpha1.App
		if err := cl.Get(ctx, *key, &got); err != nil {
			t.Fatalf("pass %d: the live row's App was deleted: %v", pass, err)
		}
		if got.UID != "uid-original" {
			t.Errorf("pass %d: App was recreated (uid %q)", pass, got.UID)
		}
		if got.Spec.PreDeployCommand != "printf ok" || got.Spec.HealthCheckPath != "/" || got.Spec.EnvFromSecret != "owned-env" {
			t.Errorf("pass %d: accepted settings lost: predeploy=%q health=%q envFromSecret=%q",
				pass, got.Spec.PreDeployCommand, got.Spec.HealthCheckPath, got.Spec.EnvFromSecret)
		}
	}
}

// A row read that fails for any reason other than not-found proves nothing
// about orphanhood: the App stays and the pass reports a retryable error.
func TestOrphanPruneKeepsAppWhenRowReadFails(t *testing.T) {
	ctx := context.Background()
	r, base, cl := newTestReconciler(t)
	tenant, _ := base.CreateTenant(ctx, "acme", "free")
	row, _ := base.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web", Image: "img", Port: 80, Replicas: 1, Tier: "free"})
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := base.DeleteApp(ctx, row.ID); err != nil {
		t.Fatal(err)
	}
	r.Store = &afterDesiredStore{memStore: base, getErr: errors.New("connection reset")}

	err := r.ReconcileOnce(ctx)
	if err == nil || !strings.Contains(err.Error(), "confirm App") {
		t.Fatalf("want a retryable confirm error, got %v", err)
	}
	var apps appv1alpha1.AppList
	if err := cl.List(ctx, &apps, client.MatchingLabels{LabelManagedBy: ManagedByValue}); err != nil {
		t.Fatal(err)
	}
	if len(apps.Items) != 1 {
		t.Fatalf("App deleted on an inconclusive row read: %d left", len(apps.Items))
	}

	// Once the read answers, the genuine orphan converges.
	r.Store = base
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := cl.List(ctx, &apps, client.MatchingLabels{LabelManagedBy: ManagedByValue}); err != nil {
		t.Fatal(err)
	}
	if len(apps.Items) != 0 {
		t.Errorf("true orphan not reaped: %d left", len(apps.Items))
	}
}

// replacingClient swaps the listed App for a same-named replacement right
// after the list, so the prune holds a stale UID. The fake client ignores UID
// preconditions, so Delete enforces them the way the apiserver does.
type replacingClient struct {
	client.Client
	replace func()
}

func (c *replacingClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	err := c.Client.List(ctx, list, opts...)
	if err == nil && c.replace != nil {
		replace := c.replace
		c.replace = nil
		replace()
	}
	return err
}

func (c *replacingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	var del client.DeleteOptions
	del.ApplyOptions(opts)
	if del.Preconditions != nil && del.Preconditions.UID != nil {
		live := obj.DeepCopyObject().(client.Object)
		if err := c.Get(ctx, client.ObjectKeyFromObject(obj), live); err != nil {
			return err
		}
		if live.GetUID() != *del.Preconditions.UID {
			return apierrors.NewConflict(appv1alpha1.SchemeGroupVersion.WithResource("apps").GroupResource(), obj.GetName(), errors.New("uid precondition failed"))
		}
	}
	return c.Client.Delete(ctx, obj, opts...)
}

func TestOrphanPruneDoesNotDeleteAReplacementApp(t *testing.T) {
	ctx := context.Background()
	r, base, cl := newTestReconciler(t)
	tenant, _ := base.CreateTenant(ctx, "acme", "free")
	row, _ := base.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web", Image: "img", Port: 80, Replicas: 1, Tier: "free"})
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var apps appv1alpha1.AppList
	if err := cl.List(ctx, &apps, client.MatchingLabels{LabelManagedBy: ManagedByValue}); err != nil || len(apps.Items) != 1 {
		t.Fatalf("setup: %v %d", err, len(apps.Items))
	}
	old := apps.Items[0]
	if err := base.DeleteApp(ctx, row.ID); err != nil {
		t.Fatal(err)
	}
	r.Client = &replacingClient{Client: cl, replace: func() {
		if err := cl.Delete(ctx, &old); err != nil {
			t.Fatal(err)
		}
		replacement := old.DeepCopy()
		replacement.ResourceVersion = ""
		replacement.UID = types.UID("uid-replacement")
		if err := cl.Create(ctx, replacement); err != nil {
			t.Fatal(err)
		}
	}}

	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var got appv1alpha1.App
	err := cl.Get(ctx, client.ObjectKeyFromObject(&old), &got)
	if apierrors.IsNotFound(err) {
		t.Fatal("stale orphan state deleted a replacement App")
	} else if err != nil {
		t.Fatal(err)
	}
	if got.UID != "uid-replacement" {
		t.Errorf("uid = %q", got.UID)
	}
}
