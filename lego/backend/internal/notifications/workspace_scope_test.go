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

package notifications

import (
	"context"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// workspace_scope_test.go is w4/m128: notification_settings is UNIQUE (tenant_id,
// subject) and the mail fan-out joins on that pair, but both verbs keyed on the
// caller's DEFAULT workspace and no adapter offered an argument to name another.
// For an account in three workspaces the product could read and write one of
// three rows while all three decided whether it got mail — so turning deploy
// email off did not stop it, and nothing could show why.

func multiWorkspaceService() (*Service, *fakeStore) {
	st := newFakeStore()
	// multiWorkspace (subscriptions_test.go) is the caller-in-several-
	// workspaces fake; its first entry is the default. fakeWorkspace cannot
	// express this — it maps a subject to exactly one tenant, which is the
	// shape that hid this defect for as long as it did.
	ws := multiWorkspace{"alice": {"tea-bex", "tea-personal", "tea-canary"}}
	return newTestService(st, ws, nil, nil), st
}

func aliceCtx() context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: "alice"})
}

func TestM128_EveryWorkspacesRowIsReachable(t *testing.T) {
	svc, st := multiWorkspaceService()
	ctx := aliceCtx()

	// Turn failure mail off in the workspace that is NOT the default. Before
	// w4/m128 this row could not be addressed at all.
	off := SettingsView{DeployStarted: false, DeploySucceeded: false, DeployFailed: false}
	if got, err := svc.UpdateSettings(ctx, "tea-canary", off.DeployStarted, off.DeploySucceeded, off.DeployFailed); err != nil || got != off {
		t.Fatalf("UpdateSettings in tea-canary = %+v (%v), want %+v", got, err, off)
	}

	// It landed on the row the fan-out reads for that workspace, and only there.
	if row, ok := st.rows[[2]string{"tea-canary", "alice"}]; !ok || row.DeployFailed {
		t.Fatalf("tea-canary row = %+v, want deploy_failed false", row)
	}
	if _, ok := st.rows[[2]string{"tea-bex", "alice"}]; ok {
		t.Fatal("writing one workspace's preferences must not touch another's row")
	}

	// And it reads back per workspace: the named one is off, the default and
	// the untouched third still answer with the failure-only default.
	if got, err := svc.GetSettings(ctx, "tea-canary"); err != nil || got != off {
		t.Fatalf("GetSettings tea-canary = %+v (%v), want %+v", got, err, off)
	}
	for _, other := range []string{"tea-bex", "tea-personal"} {
		if got, err := svc.GetSettings(ctx, other); err != nil || got != defaultSettings {
			t.Fatalf("GetSettings %s = %+v (%v), want the untouched default %+v", other, got, err, defaultSettings)
		}
	}
	// Omitting the workspace keeps the old behavior — the default workspace.
	if got, err := svc.GetSettings(ctx, ""); err != nil || got != defaultSettings {
		t.Fatalf("GetSettings with no workspace = %+v (%v), want the default workspace's %+v", got, err, defaultSettings)
	}
}

// TestM128_PreferencesGovernTheirOwnWorkspacesMail closes the loop the
// milestone is actually about: the preference the API writes is the one
// ListNotifyRecipients consults for that workspace, and no other.
func TestM128_PreferencesGovernTheirOwnWorkspacesMail(t *testing.T) {
	svc, st := multiWorkspaceService()
	ctx := aliceCtx()
	if _, err := svc.UpdateSettings(ctx, "tea-canary", false, false, false); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	for tenant, wantFailed := range map[string]bool{"tea-canary": false, "tea-bex": true} {
		row, err := st.GetNotificationSettings(ctx, tenant, "alice")
		if tenant == "tea-bex" {
			// No explicit row: the store's COALESCE default is what the
			// fan-out resolves, which is failure-on.
			if err == nil {
				t.Fatalf("tea-bex should have no explicit row, got %+v", row)
			}
			continue
		}
		if err != nil {
			t.Fatalf("GetNotificationSettings %s: %v", tenant, err)
		}
		if row.DeployFailed != wantFailed {
			t.Fatalf("%s deploy_failed = %v, want %v", tenant, row.DeployFailed, wantFailed)
		}
	}
}

func TestM128_AWorkspaceTheCallerIsNotInIsRefused(t *testing.T) {
	svc, _ := multiWorkspaceService()
	ctx := aliceCtx()

	if _, err := svc.GetSettings(ctx, "tea-someone-else"); err == nil {
		t.Fatal("reading a foreign workspace's preferences must be refused")
	}
	if _, err := svc.UpdateSettings(ctx, "tea-someone-else", true, true, true); err == nil {
		t.Fatal("writing a foreign workspace's preferences must be refused")
	}
}
