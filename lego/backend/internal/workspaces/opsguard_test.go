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

package workspaces

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// assertOpsProtected pins the coded-error contract (ADR088 §4): a stable
// OPS_WORKSPACE_PROTECTED code wrapping ErrConflict (409), never prose-matched.
func assertOpsProtected(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("want ErrConflict (409), got %v", err)
	}
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.CodeOpsWorkspaceProtected {
		t.Fatalf("want code %s, got %v", core.CodeOpsWorkspaceProtected, err)
	}
}

// TestDelete_OpsWorkspaceRefused: deleting the pinned ops workspace is refused
// with the stable coded 409 even for an admin presenting the exact confirmation
// phrase, and no teardown side effect happens — the workspace's membership is
// the observability-UI ACL (ADR088 §4).
func TestDelete_OpsWorkspaceRefused(t *testing.T) {
	st := newFakeStore()
	rev := &fakeRevoker{}
	purge := &fakePurger{}
	var kicks int
	svc := allowSvc(st, &fakeGranter{}, rev, func() { kicks++ }, purge)
	w, _ := svc.Create(ctxAs("user-a"), "ops", "hobby")
	svc.OpsWorkspaceID = w.ID

	err := svc.Delete(ctxAs("user-a"), w.ID, "sudo delete workspace ops")
	assertOpsProtected(t, err)
	if _, gerr := st.GetTenant(context.Background(), w.ID); gerr != nil {
		t.Fatalf("refused delete still removed the row: %v", gerr)
	}
	if len(rev.revoked) != 0 || len(purge.purged) != 0 || kicks != 0 {
		t.Fatalf("refused delete had side effects: revoked=%v purged=%v kicks=%d", rev.revoked, purge.purged, kicks)
	}
}

// TestDelete_OrdinaryWorkspaceUnaffectedByPin: with the pin naming a DIFFERENT
// workspace, every other workspace deletes exactly as before.
func TestDelete_OrdinaryWorkspaceUnaffectedByPin(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, &fakeRevoker{}, nil)
	svc.OpsWorkspaceID = "tea-elsewhere"
	w, _ := svc.Create(ctxAs("user-a"), "acme", "hobby")

	if err := svc.Delete(ctxAs("user-a"), w.ID, "sudo delete workspace acme"); err != nil {
		t.Fatalf("pin on another workspace must not block this delete: %v", err)
	}
}

// TestDelete_UnsetPinIsInert: BEX_OPS_WORKSPACE unset (empty OpsWorkspaceID)
// means zero behavior change — every workspace, whatever its name, deletes.
func TestDelete_UnsetPinIsInert(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, &fakeRevoker{}, nil)
	w, _ := svc.Create(ctxAs("user-a"), "ops", "hobby")

	if err := svc.Delete(ctxAs("user-a"), w.ID, "sudo delete workspace ops"); err != nil {
		t.Fatalf("unset pin must leave delete unchanged: %v", err)
	}
}

// TestAccountTeardown_OpsWorkspaceRefused: the trusted account-deletion
// teardown path shares the same backstop — a deletion that somehow carries a
// `delete` disposition for the ops workspace (e.g. begun before the pin was
// configured) refuses instead of tearing it down.
func TestAccountTeardown_OpsWorkspaceRefused(t *testing.T) {
	st := newFakeStore()
	svc := allowSvc(st, &fakeGranter{}, &fakeRevoker{}, nil)
	w, _ := svc.Create(ctxAs("user-a"), "ops", "hobby")
	svc.OpsWorkspaceID = w.ID

	err := AccountTeardown{Service: svc}.Delete(context.Background(), w.ID)
	assertOpsProtected(t, err)
	if _, gerr := st.GetTenant(context.Background(), w.ID); gerr != nil {
		t.Fatalf("refused teardown still removed the row: %v", gerr)
	}
	// An ordinary workspace still tears down through the same path.
	other, _ := svc.Create(ctxAs("user-a"), "acme", "hobby")
	if err := (AccountTeardown{Service: svc}).Delete(context.Background(), other.ID); err != nil {
		t.Fatalf("ordinary teardown blocked: %v", err)
	}
}
