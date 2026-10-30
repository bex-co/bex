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
	"fmt"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w3/m246: a workspace's freeform display name is set once for a signup
// workspace, a rename sets it without touching the slug, a later sign-in never
// overwrites a rename, and every tenant read returns it; the webhook feed's
// public service id is the App's CR name (tenant id), not the slug.
func TestPGTenantDisplayName(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	defer pool.Close()
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())

	owner := "dn-owner-" + stamp
	ws, err := st.CreateTenantWithMember(ctx, owner, PlanHobby)
	if err != nil {
		t.Fatalf("create signup workspace: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	if ws.Name != ws.ID || ws.DisplayName != "" {
		t.Fatalf("signup workspace = %+v, want slug = id and no display name yet", ws)
	}

	if err := st.SetTenantDisplayNameIfUnset(ctx, ws.ID, "Tian Pan's Workspace"); err != nil {
		t.Fatalf("first name: %v", err)
	}
	if err := st.SetTenantDisplayNameIfUnset(ctx, ws.ID, "Someone Else's Workspace"); err != nil {
		t.Fatalf("second name: %v", err)
	}
	if got, err := st.GetTenant(ctx, ws.ID); err != nil || got.DisplayName != "Tian Pan's Workspace" {
		t.Fatalf("after a second name = (%+v, %v), want the first kept", got, err)
	}
	if err := st.SetTenantDisplayNameIfUnset(ctx, "tea-missing-"+stamp, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("naming a missing workspace = %v, want ErrNotFound", err)
	}

	app, err := st.CreateApp(ctx, App{
		TenantID: ws.ID, Name: "web-" + stamp, Image: "traefik/whoami",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}
	renamed, err := st.SetTenantDisplayName(ctx, ws.ID, "Acme")
	if err != nil || renamed.DisplayName != "Acme" || renamed.Name != ws.ID {
		t.Fatalf("rename = (%+v, %v), want the display name set and the slug kept", renamed, err)
	}
	if err := st.SetTenantDisplayNameIfUnset(ctx, ws.ID, "Tian Pan's Workspace"); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.GetTenant(ctx, ws.ID); got.DisplayName != "Acme" || got.Label() != "Acme" {
		t.Fatalf("a later sign-in overwrote the rename: %+v", got)
	}

	list, err := st.ListTenantsForSubject(ctx, owner)
	if err != nil || len(list) != 1 || list[0].DisplayName != "Acme" {
		t.Fatalf("list for subject = (%+v, %v)", list, err)
	}

	rows, err := st.ListWebhookEvents(ctx, time.Time{}, "", time.Now().UTC().Add(time.Hour), nil, []string{ws.ID}, 100)
	if err != nil {
		t.Fatalf("list webhook events: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("no webhook rows for the app's create deploy")
	}
	for _, r := range rows {
		if r.AppID == app.ID && r.ServiceID != core.CRName(ws.ID, app.Name) {
			t.Errorf("%s row service id = %q, want the CR name %q", r.Source, r.ServiceID, core.CRName(ws.ID, app.Name))
		}
	}
}
