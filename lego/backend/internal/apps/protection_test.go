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
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// protection_test.go exercises w6/m19's protected-environment guard
// (protection.go): a member App of a protectedStatus=protected Environment
// refuses Delete/Suspend/an existing-service DeployStack override without a
// matching ProtectedConfirmation phrase — enforcement, not just the storage
// round-trip environments' own tests cover.

func TestDelete_BlockedWhenProtectedWithoutConfirm(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if err := svc.Delete(context.Background(), "web"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("Delete on a protected member: got %v, want ErrBadRequest", err)
	}
	// The App must survive an unconfirmed, blocked delete.
	getApp(t, cl, "web")
}

func TestDelete_SucceedsWithCorrectConfirm(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	want := ProtectedConfirmation("delete", "web")
	ctx := withConfirm(context.Background(), want)
	if err := svc.Delete(ctx, "web"); err != nil {
		t.Fatalf("Delete with correct confirm: %v", err)
	}
	var got appv1alpha1.AppList
	if err := cl.List(context.Background(), &got); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got.Items) != 0 {
		t.Fatalf("App should be deleted, got %d remaining", len(got.Items))
	}
}

func TestDelete_WrongConfirmStillBlocked(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, _ := newService(rec, managedApp("web", "srv-1"))

	ctx := withConfirm(context.Background(), "sudo delete service someone-else")
	if err := svc.Delete(ctx, "web"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("Delete with wrong confirm: got %v, want ErrBadRequest", err)
	}
}

func TestDelete_UnprotectedNeedsNoConfirm(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "unprotected"}}
	svc, _ := newService(rec, managedApp("web", "srv-1"))

	if err := svc.Delete(context.Background(), "web"); err != nil {
		t.Fatalf("Delete on an unprotected member: %v", err)
	}
}

func TestDelete_RetryRemovesCRAfterSourceRowIsGone(t *testing.T) {
	rec := &recordingStore{err: store.ErrNotFound}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if err := svc.Delete(context.Background(), "web"); err != nil {
		t.Fatalf("Delete after row-first interruption: %v", err)
	}
	gone(t, cl, "web")
}

func TestSuspend_BlockedWhenProtectedWithoutConfirm(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.Suspend(context.Background(), "web"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("Suspend on a protected member: got %v, want ErrBadRequest", err)
	}
	if getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("a blocked suspend must not patch the CR")
	}
}

func TestSuspend_SucceedsWithCorrectConfirm(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	want := ProtectedConfirmation("suspend", "web")
	ctx := withConfirm(context.Background(), want)
	if _, err := svc.Suspend(ctx, "web"); err != nil {
		t.Fatalf("Suspend with correct confirm: %v", err)
	}
	if !getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("App should be suspended")
	}
}

// TestResume_NeverBlockedEvenWhenProtected proves Resume is exempt: a
// protected environment blocks taking availability AWAY, not restoring it.
func TestResume_NeverBlockedEvenWhenProtected(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	app := managedApp("web", "srv-1")
	app.Spec.Suspended = true
	svc, cl := newService(rec, app)

	if _, err := svc.Resume(context.Background(), "web"); err != nil {
		t.Fatalf("Resume on a protected member must never be blocked: %v", err)
	}
	if getApp(t, cl, "web").Spec.Suspended {
		t.Fatal("App should be resumed")
	}
}

func TestDeployStack_DirectOverrideBlockedWhenProtected(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	existing := managedApp("web", "srv-1")
	existing.Spec.Image = "old:1"
	svc, cl := newService(rec, existing)

	changed := "services:\n  - {name: web, type: web, runtime: image, image: {url: new:1}}\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: changed}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("DeployStack override on a protected member: got %v, want ErrBadRequest", err)
	}
	if got := getApp(t, cl, "web").Spec.Image; got != "old:1" {
		t.Fatalf("image must not change on a blocked override, got %q", got)
	}
}

func TestDeployStack_DirectOverrideSucceedsWithConfirm(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	existing := managedApp("web", "srv-1")
	existing.Spec.Image = "old:1"
	svc, cl := newService(rec, existing)

	changed := "services:\n  - {name: web, type: web, runtime: image, image: {url: new:1}}\n"
	confirm := ProtectedConfirmation("deploy", "web")
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: changed, Confirm: confirm}); err != nil {
		t.Fatalf("DeployStack override with correct confirm: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Image; got != "new:1" {
		t.Fatalf("image = %q, want new:1", got)
	}
}

func TestSyncBlueprint_ProtectedOverrideSucceedsWithConfirm(t *testing.T) {
	rec := &recordingStore{protectedStatus: map[string]string{"srv-1": "protected"}}
	existing := managedApp("web", "srv-1")
	existing.Spec.Image = "old:1"
	svc, cl := newService(rec, existing)
	svc.Blueprints = newFakeBlueprintStore(store.Blueprint{
		ID:       "blp-1",
		Manifest: "services:\n  - {name: web, type: web, runtime: image, image: {url: new:1}}\n",
		Status:   "active",
	})

	if _, err := svc.SyncBlueprint(context.Background(), "blp-1", "", "", ""); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("SyncBlueprint override on a protected member: got %v, want ErrBadRequest", err)
	}
	if got := getApp(t, cl, "web").Spec.Image; got != "old:1" {
		t.Fatalf("blocked sync changed image to %q", got)
	}

	confirm := ProtectedConfirmation("deploy", "web")
	if _, err := svc.SyncBlueprint(context.Background(), "blp-1", "", "", confirm); err != nil {
		t.Fatalf("SyncBlueprint with correct confirm: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Image; got != "new:1" {
		t.Fatalf("image = %q, want new:1", got)
	}
}

// TestDeployStack_NewServiceNeverBlocked proves a brand-new service (no
// existing App to override) is exempt — only an override of something
// already running is guarded.
func TestDeployStack_NewServiceNeverBlocked(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec)

	manifest := "services:\n  - {name: web, type: web, runtime: image, image: {url: new:1}}\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack for a brand-new service: %v", err)
	}
	getApp(t, cl, "web")
}
