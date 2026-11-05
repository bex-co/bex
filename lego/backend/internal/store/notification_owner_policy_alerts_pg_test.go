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

package store_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/notifications"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestAlertEmailWorkerFollowsTheStoredWorkspacePolicy (w3/m271) on real
// Postgres: the policy an admin saves is the one the failure-alert worker
// reads. A workspace on "none" gets no failure mail for a service on
// "default", while a service whose own override is "failure" still mails.
func TestAlertEmailWorkerFollowsTheStoredWorkspacePolicy(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)
	if _, err := pool.Exec(ctx, `DELETE FROM alert_email_watermarks`); err != nil {
		t.Fatal(err)
	}

	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	alice := "policy-alice-" + stamp
	ws, err := st.CreateWorkspace(ctx, "policy-"+stamp, store.PlanHobby, alice)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), ws.ID) })
	none := appv1alpha1.NotificationsToSendNone
	if _, err := st.UpsertNotificationOwnerPolicy(ctx, ws.ID, store.NotificationOwnerPolicyPatch{NotificationsToSend: &none}); err != nil {
		t.Fatal(err)
	}
	followsWorkspace, err := st.CreateApp(ctx, store.App{TenantID: ws.ID, Name: "follows-" + stamp, Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	overridden, err := st.CreateApp(ctx, store.App{TenantID: ws.ID, Name: "override-" + stamp, Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}

	mailer := &alertMailer{}
	clock := time.Now()
	worker := &notifications.AlertEmailWorker{
		Store: st, Mailer: mailer, Owners: notifications.StoredOwnerPolicy(st),
		Identities:   alertEmails{alice: "alice@example.com"},
		Clock:        func() time.Time { return clock },
		Window:       10 * time.Minute,
		BlueprintLag: store.FeedCommitLag,
		Policy: func(_ context.Context, _, serviceID string) (string, error) {
			if serviceID == overridden.ID {
				return appv1alpha1.NotificationsToSendFailure, nil
			}
			return appv1alpha1.NotificationsToSendDefault, nil
		},
	}
	if err := worker.RunOnce(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}
	for _, app := range []store.App{followsWorkspace, overridden} {
		if _, err := st.InsertServiceEventFact(ctx, store.ServiceEventFact{
			SourceKey: "policy-fail-" + app.ID, AppID: app.ID, Type: store.EventFactServerFailed, At: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	clock = time.Now().Add(store.FeedCommitLag + time.Second)
	if err := worker.RunOnce(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}
	sent := mailer.take()
	if len(sent) != 1 || sent[0].subject != "Service unavailable: "+overridden.Name {
		t.Fatalf("sent %v, want only the overridden service's failure (the workspace is on none)", sent)
	}
}
