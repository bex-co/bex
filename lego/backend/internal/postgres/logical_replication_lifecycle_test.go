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
	"maps"
	"slices"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Logical replication through a database's lifecycle (w34/m543): what a
// recovery carries, the upgrade gate without a database to read, and the
// verb's failover warnings. The real-PostgreSQL halves are in
// logical_slots_pg_test.go.

// A copy recovered from an enabled database starts not enabled (DoD 5):
// no spec.logicalReplication, the source's publisher and subscriber
// credentials retired (the backup restores their REPLICATION and
// pg_create_subscription), the owner left as the copy's login, and none of
// the settings the operator owns while a database publishes or is recovered.
func TestRecoveredCopyIsNotEnabled(t *testing.T) {
	for _, tc := range []struct {
		name string
		lr   *appv1alpha1.DatabaseLogicalReplication
		want []string
	}{
		{name: "never enabled", want: []string{"leaked_login"}},
		{name: "a dedicated reader and a subscriber credential",
			lr:   &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "cdc_reader", Schemas: []string{"public"}, AllTables: true, SubscriberRole: "app_rotated"},
			want: []string{"leaked_login", "cdc_reader", "app_rotated"}},
		{name: "the owner publishes and subscribes",
			lr:   &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "orders_owner", Schemas: []string{"public"}, SubscriberRole: "orders_owner"},
			want: []string{"leaked_login"}},
		{name: "a retired login is named once",
			lr:   &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "leaked_login", Schemas: []string{"public"}},
			want: []string{"leaked_login"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newServicePostgresResources()
			seedDatabaseSpec(t, cl, "src-db", appv1alpha1.DatabaseSpec{
				Plan: "basic-1gb", Version: "17", DatabaseName: "orders_data", DatabaseUser: "orders_owner", StorageGB: 10,
				Users:              []appv1alpha1.DatabaseUser{{Name: "cdc_reader"}, {Name: "app_rotated", OwnerPrivileges: true}},
				DeletedUsers:       []string{"leaked_login"},
				LogicalReplication: tc.lr,
				Parameters: map[string]string{
					"work_mem": "8MB", "max_slot_wal_keep_size": "4GB", "Hot_Standby_Feedback": "off",
					"sync_replication_slots": "on", "max_logical_replication_workers": "8",
				},
			}, true)
			seedBarmanRecoveryWindow(t, cl, "src-db", "2026-07-01T00:00:00Z", "2026-07-10T00:00:00Z")
			ctx := context.Background()
			v, err := svc.Recover(ctx, "src-db", RecoverRequest{Name: "restored-db", TargetTime: "2026-07-09T10:00:00Z"})
			if err != nil {
				t.Fatalf("Recover => %v", err)
			}
			var made appv1alpha1.Database
			if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: v.ID}, &made); err != nil {
				t.Fatal(err)
			}
			if made.Spec.LogicalReplication != nil {
				t.Errorf("the copy carries spec.logicalReplication %+v", made.Spec.LogicalReplication)
			}
			if !slices.Equal(made.Spec.DeletedUsers, tc.want) {
				t.Errorf("the copy retires %v, want %v", made.Spec.DeletedUsers, tc.want)
			}
			if !maps.Equal(made.Spec.Parameters, map[string]string{"work_mem": "8MB"}) {
				t.Errorf("the copy carries parameters %v, want only the tenant's own", made.Spec.Parameters)
			}
		})
	}
}

// A publisher's upgrade reads its slots first; one that cannot be read is
// refused, writing nothing, rather than started blind. A database that does
// not publish is never read, so the same unreachable database upgrades as
// before (DoD 4's "unchanged" half).
func TestUpgradeOfAnUnreadablePublisherIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lr       *appv1alpha1.DatabaseLogicalReplication
		wantCode string
		wantSpec string
	}{
		{name: "a publisher", lr: &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "cdc_reader", Schemas: []string{"public"}},
			wantCode: PostgresUpgradeLogicalSlotsUnreadableCode, wantSpec: "17"},
		{name: "a subscriber only", lr: &appv1alpha1.DatabaseLogicalReplication{SubscriberRole: "cdc_reader"}, wantSpec: "18"},
		{name: "not enabled", wantSpec: "18"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := versionDatabase("unreadable-db", "free", "17", false)
			db.Spec.LogicalReplication = tc.lr
			svc, cl := newService(db) // no -app Secret: the database cannot be dialed
			view, err := svc.SetVersion(context.Background(), "unreadable-db", "18")
			if got := codedErrorCode(err); got != tc.wantCode {
				t.Fatalf("SetVersion = %v (code %q), want code %q", err, got, tc.wantCode)
			}
			if err == nil && view.UpgradeNotice != "" {
				t.Errorf("an upgrade with nothing to drop answers notice %q", view.UpgradeNotice)
			}
			var got appv1alpha1.Database
			_ = cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "unreadable-db"}, &got)
			if got.Spec.Version != tc.wantSpec {
				t.Errorf("spec.version = %q, want %q", got.Spec.Version, tc.wantSpec)
			}
		})
	}
}

// The notice an accepted upgrade from before 17 answers names every slot it
// drops and says what the user does next; none without slots.
func TestDroppedSlotsNotice(t *testing.T) {
	if got := droppedSlotsNotice("16", nil); got != "" {
		t.Fatalf("no slots: notice %q, want none", got)
	}
	got := droppedSlotsNotice("16", []string{"cdc", "debezium"})
	for _, want := range []string{"PostgreSQL 16", "drops cdc, debezium", "re-create each of their subscriptions", "17 or later"} {
		if !strings.Contains(got, want) {
			t.Errorf("notice %q lacks %q", got, want)
		}
	}
}

// The verb tells the operator how a failover treats a database with a
// standby. As a publisher (t002): before 17 every slot is lost, pending
// pg_failover_slots in bex's image (w8/m92); from 17 a subscription keeps its
// place only with failover = true. As a subscriber, on every major (w34/003):
// the asynchronous standby can miss what the subscriptions applied last, which
// their publishers never resend. A database with no standby, or not enabled,
// has nothing to warn about.
func TestFailoverWarnings(t *testing.T) {
	publisher := &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "cdc_reader", Schemas: []string{"public"}}
	subscriber := &appv1alpha1.DatabaseLogicalReplication{SubscriberRole: "cdc_reader"}
	created := &appv1alpha1.DatabaseLogicalReplication{Subscriptions: []appv1alpha1.DatabaseLogicalSubscription{{Name: "from_render"}}}
	both := &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "cdc_reader", Schemas: []string{"public"}, SubscriberRole: "cdc_reader"}
	const subscriberLoss = "an automatic failover can lose the changes its subscriptions applied"
	for _, tc := range []struct {
		name  string
		ha    bool
		lr    *appv1alpha1.DatabaseLogicalReplication
		major int
		want  []string
	}{
		{name: "HA publisher on 16", ha: true, lr: publisher, major: 16, want: []string{"w8/m92"}},
		{name: "HA publisher on 13", ha: true, lr: publisher, major: 13, want: []string{"PostgreSQL 13, whose failover or switchover does not carry"}},
		{name: "HA publisher on 17", ha: true, lr: publisher, major: 17, want: []string{"WITH (failover = true)"}},
		{name: "HA publisher on 18", ha: true, lr: publisher, major: 18, want: []string{"WITH (failover = true)"}},
		{name: "publisher without a standby", lr: publisher, major: 16},
		{name: "HA subscriber role on 18", ha: true, lr: subscriber, major: 18, want: []string{subscriberLoss}},
		{name: "HA subscriber role on 16", ha: true, lr: subscriber, major: 16, want: []string{subscriberLoss}},
		{name: "HA with subscriptions bex creates on 15", ha: true, lr: created, major: 15, want: []string{subscriberLoss}},
		{name: "HA publisher and subscriber on 18", ha: true, lr: both, major: 18, want: []string{"WITH (failover = true)", subscriberLoss}},
		{name: "HA publisher and subscriber on 16", ha: true, lr: both, major: 16, want: []string{"w8/m92", subscriberLoss}},
		{name: "subscriber without a standby", lr: subscriber, major: 18},
		{name: "HA, not enabled", ha: true, major: 16},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := publisherDatabase("dpg-fo")
			d.Spec.HighAvailability = tc.ha
			got := failoverWarnings(d, tc.lr, tc.major)
			if len(got) != len(tc.want) {
				t.Fatalf("warnings = %q, want %d naming %q", got, len(tc.want), tc.want)
			}
			for i, want := range tc.want {
				if !strings.Contains(got[i], want) {
					t.Fatalf("warning %d = %q, want it to name %q", i, got[i], want)
				}
			}
		})
	}
}
