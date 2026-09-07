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

package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type enforcementCapture struct {
	entries   map[string]store.BillingEnforcement
	rowWrites []bool
}

func enforcementKey(kind, name string) string { return kind + "/" + name }

func (s *enforcementCapture) EnsureBillingEnforcement(_ context.Context, e store.BillingEnforcement) (store.BillingEnforcement, error) {
	if prior, ok := s.entries[enforcementKey(e.ResourceKind, e.ResourceName)]; ok {
		return prior, nil
	}
	e.EnforcedAt = time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	s.entries[enforcementKey(e.ResourceKind, e.ResourceName)] = e
	return e, nil
}

func (s *enforcementCapture) ListActiveBillingEnforcements(_ context.Context, workspaceID string) ([]store.BillingEnforcement, error) {
	var out []store.BillingEnforcement
	for _, e := range s.entries {
		if e.WorkspaceID == workspaceID && e.RecoveredAt == nil {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *enforcementCapture) MarkBillingEnforcementRecovered(_ context.Context, workspaceID, kind, name string, at time.Time) error {
	e := s.entries[enforcementKey(kind, name)]
	if e.WorkspaceID == workspaceID {
		e.RecoveredAt = &at
		s.entries[enforcementKey(kind, name)] = e
	}
	return nil
}

func (s *enforcementCapture) SetAppSuspended(_ context.Context, _ string, suspended bool) error {
	s.rowWrites = append(s.rowWrites, suspended)
	return nil
}

func TestKubernetesEnforcerPreservesDataAndOnlyRecoversOwnedIntent(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	labels := map[string]string{core.LabelTenant: "tea-a"}
	managedLabels := map[string]string{core.LabelTenant: "tea-a", store.LabelManagedBy: store.ManagedByValue, store.LabelAppID: "srv-running"}
	objects := []client.Object{
		// App CRs live in their own per-tenant namespace (ADR043); Database/KeyValue stay in the shared "apps" namespace.
		&appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "running", Namespace: "tea-a", Labels: managedLabels}},
		&appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "user-suspended", Namespace: "tea-a", Labels: labels}, Spec: appv1alpha1.AppSpec{Suspended: true}},
		&appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "static", Namespace: "tea-a", Labels: labels}, Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeStaticSite}},
		&appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "apps", Labels: labels}},
		&appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "apps", Labels: labels}, Spec: appv1alpha1.KeyValueSpec{Suspended: true}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
	capture := &enforcementCapture{entries: map[string]store.BillingEnforcement{}}
	enforcer := &KubernetesEnforcer{Client: cl, Store: capture, Namespace: "apps", Clock: func() time.Time {
		return time.Date(2026, 7, 28, 1, 0, 0, 0, time.UTC)
	}}
	state := store.BillingLifecycle{WorkspaceID: "tea-a", TransitionVersion: 4}
	if err := enforcer.Enforce(ctx, state); err != nil {
		t.Fatalf("enforce: %v", err)
	}

	assertApp := func(name string, suspended bool, marker string) *appv1alpha1.App {
		t.Helper()
		var got appv1alpha1.App
		if err := cl.Get(ctx, client.ObjectKey{Namespace: "tea-a", Name: name}, &got); err != nil {
			t.Fatal(err)
		}
		if got.Spec.Suspended != suspended || got.Annotations[BillingEnforcementAnnotation] != marker {
			t.Fatalf("App %s suspended=%v marker=%q", name, got.Spec.Suspended, got.Annotations[BillingEnforcementAnnotation])
		}
		return &got
	}
	running := assertApp("running", true, "tea-a:4")
	assertApp("user-suspended", true, "")
	assertApp("static", false, "")
	var db appv1alpha1.Database
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "apps", Name: "db"}, &db); err != nil {
		t.Fatal(err)
	}
	if !db.Spec.Suspended || db.Annotations[BillingEnforcementAnnotation] != "tea-a:4" {
		t.Fatalf("Database not reversibly suspended: %+v", db.Spec)
	}
	var kv appv1alpha1.KeyValue
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "apps", Name: "cache"}, &kv); err != nil {
		t.Fatal(err)
	}
	if kv.Annotations[BillingEnforcementAnnotation] != "" {
		t.Fatalf("pre-suspended KeyValue acquired marker %q", kv.Annotations[BillingEnforcementAnnotation])
	}
	if len(capture.entries) != 2 {
		t.Fatalf("markers = %d, want running App + Database", len(capture.entries))
	}

	// An independent operator replaces the App marker. Recovery must release its
	// database row but preserve the operator's suspended intent.
	base := running.DeepCopy()
	running.Annotations[BillingEnforcementAnnotation] = "operator-owned"
	if err := cl.Patch(ctx, running, client.MergeFrom(base)); err != nil {
		t.Fatal(err)
	}
	if err := enforcer.Recover(ctx, store.BillingLifecycle{WorkspaceID: "tea-a"}); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertApp("running", true, "operator-owned")
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "apps", Name: "db"}, &db); err != nil {
		t.Fatal(err)
	}
	if db.Spec.Suspended || db.Annotations[BillingEnforcementAnnotation] != "" {
		t.Fatalf("owned Database recovery = suspended %v marker %q", db.Spec.Suspended, db.Annotations[BillingEnforcementAnnotation])
	}
	if len(capture.rowWrites) != 1 || !capture.rowWrites[0] {
		t.Fatalf("managed App row writes = %v; replaced marker must not resume row", capture.rowWrites)
	}
}

// TestEnforceRefusesOpsWorkspace is the ADR087 §4 suspension guard: dunning
// enforcement against the pinned ops workspace fails with the stable
// OPS_WORKSPACE_PROTECTED code (409-class conflict) BEFORE touching any CR,
// while an ordinary workspace — and an unset pin — enforce exactly as before.
// Recover stays allowed for the ops workspace (un-suspending is always safe).
func TestEnforceRefusesOpsWorkspace(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	build := func(opsID string) (*KubernetesEnforcer, client.Client, *enforcementCapture) {
		labels := map[string]string{core.LabelTenant: "tea-ops"}
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			&appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "grafana-adjacent", Namespace: "tea-ops", Labels: labels}},
		).Build()
		capture := &enforcementCapture{entries: map[string]store.BillingEnforcement{}}
		return &KubernetesEnforcer{Client: cl, Store: capture, Namespace: "apps", OpsWorkspaceID: opsID}, cl, capture
	}
	state := store.BillingLifecycle{WorkspaceID: "tea-ops", TransitionVersion: 1}

	// Pinned: refused with the coded error, no CR mutated, no marker written.
	enforcer, cl, capture := build("tea-ops")
	err := enforcer.Enforce(ctx, state)
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.CodeOpsWorkspaceProtected {
		t.Fatalf("want code %s, got %v", core.CodeOpsWorkspaceProtected, err)
	}
	var app appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "tea-ops", Name: "grafana-adjacent"}, &app); err != nil {
		t.Fatal(err)
	}
	if app.Spec.Suspended || len(capture.entries) != 0 {
		t.Fatalf("refused enforcement still acted: suspended=%v markers=%d", app.Spec.Suspended, len(capture.entries))
	}
	// Recovery of the pinned workspace stays allowed.
	if err := enforcer.Recover(ctx, state); err != nil {
		t.Fatalf("recover of the ops workspace must stay allowed: %v", err)
	}

	// Pin naming another workspace, and unset pin: enforcement proceeds.
	for _, opsID := range []string{"tea-other", ""} {
		enforcer, cl, _ := build(opsID)
		if err := enforcer.Enforce(ctx, state); err != nil {
			t.Fatalf("pin %q must not block enforcement of tea-ops: %v", opsID, err)
		}
		var got appv1alpha1.App
		if err := cl.Get(ctx, client.ObjectKey{Namespace: "tea-ops", Name: "grafana-adjacent"}, &got); err != nil {
			t.Fatal(err)
		}
		if !got.Spec.Suspended {
			t.Fatalf("pin %q: enforcement did not suspend", opsID)
		}
	}
}
