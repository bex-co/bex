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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The fixture database is the subscriber here, and its sibling database on
// the same server (f.subscriber in the publisher tests) is the publisher:
// it gets the same tables and a publication.
func (f *lrFixture) asSubscriber() (publisher string) {
	f.t.Helper()
	publisher = f.subscriber
	execAs(f.t, publisher,
		fmt.Sprintf(`CREATE ROLE lr_pub_%s LOGIN REPLICATION PASSWORD 'pub-pw'`, f.n),
		`CREATE TABLE public.orders (id int PRIMARY KEY, note text)`,
		`INSERT INTO public.orders VALUES (1, 'existing')`,
		fmt.Sprintf(`GRANT SELECT ON public.orders TO lr_pub_%s`, f.n),
		`CREATE PUBLICATION src_orders FOR TABLE public.orders`,
	)
	// A subscription must go before its database can; one within one server
	// keeps its slot apart (slot_name = NONE) so dropping it needs no
	// connection.
	f.t.Cleanup(func() {
		ctx := context.Background()
		conn, err := pgx.Connect(ctx, f.superuser)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close(ctx) }()
		rows, _ := conn.Query(ctx, `SELECT subname::text FROM pg_subscription WHERE subdbid = (SELECT oid FROM pg_database WHERE datname = current_database())`)
		names, _ := pgx.CollectRows(rows, pgx.RowTo[string])
		for _, name := range names {
			id := pgx.Identifier{name}.Sanitize()
			for _, s := range []string{"ALTER SUBSCRIPTION " + id + " DISABLE", "ALTER SUBSCRIPTION " + id + " SET (slot_name = NONE)", "DROP SUBSCRIPTION " + id} {
				_, _ = conn.Exec(ctx, s)
			}
		}
		_, _ = conn.Exec(ctx, `SELECT pg_drop_replication_slot(slot_name) FROM pg_replication_slots WHERE database = $1 AND NOT active`, "lr_sub_"+f.n)
		// The role owns grants in the publisher's database, which goes first.
		f.t.Cleanup(func() {
			if conn, err := pgx.Connect(ctx, testDBURI(f.t)); err == nil {
				_, _ = conn.Exec(ctx, "DROP ROLE IF EXISTS lr_pub_"+f.n)
				_ = conn.Close(ctx)
			}
		})
	})
	return publisher
}

// operateSubscriber plays the operator and CNPG for the subscriber half
// until the test ends: on 16 and later it makes the spec's subscriber role,
// and no other, a member of pg_create_subscription; on 13-15 it creates the
// spec's subscriptions as the superuser with the password from their
// Secret (without connecting, as the publisher shares the server) and drops
// the others.
func (f *lrFixture) operateSubscriber(major int) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	f.t.Cleanup(func() { cancel(); <-done })
	port := sqlString(f.t, f.superuser, `SHOW port`)
	go func() {
		defer close(done)
		conn, err := pgx.Connect(ctx, f.superuser)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close(context.Background()) }()
		for ctx.Err() == nil {
			var d appv1alpha1.Database
			if err := f.svc.Client.Get(ctx, client.ObjectKeyFromObject(f.db), &d); err == nil {
				lr := d.Spec.LogicalReplication
				if major >= appv1alpha1.PgCreateSubscriptionMajor {
					for _, role := range []string{f.ownerRole, f.readerRole} {
						statement := "REVOKE pg_create_subscription FROM " + role
						if lr != nil && lr.SubscriberRole == role {
							statement = "GRANT pg_create_subscription TO " + role
						}
						_, _ = conn.Exec(ctx, statement)
					}
				} else {
					f.convergeSubscriptions(ctx, conn, lr, port)
				}
			}
			select {
			case <-ctx.Done():
			case <-time.After(50 * time.Millisecond):
			}
		}
	}()
}

func (f *lrFixture) convergeSubscriptions(ctx context.Context, conn *pgx.Conn, lr *appv1alpha1.DatabaseLogicalReplication, port string) {
	wanted := map[string]bool{}
	if lr != nil {
		for _, sub := range lr.Subscriptions {
			wanted[sub.Name] = true
			var sec corev1.Secret
			if err := f.svc.Client.Get(ctx, client.ObjectKey{Namespace: f.db.Namespace, Name: sub.PasswordSecret}, &sec); err != nil {
				continue
			}
			_, _ = conn.Exec(ctx, fmt.Sprintf(`CREATE SUBSCRIPTION %s CONNECTION 'host=127.0.0.1 port=%s dbname=%s user=%s password=%s' PUBLICATION %s WITH (connect = false)`,
				sub.Name, port, sub.DBName, sub.User, sec.Data["password"], sub.Publication))
		}
	}
	rows, _ := conn.Query(ctx, `SELECT subname::text FROM pg_subscription WHERE subdbid = (SELECT oid FROM pg_database WHERE datname = current_database())`)
	names, _ := pgx.CollectRows(rows, pgx.RowTo[string])
	for _, name := range names {
		if !wanted[name] {
			_, _ = conn.Exec(ctx, "ALTER SUBSCRIPTION "+name+" SET (slot_name = NONE)")
			_, _ = conn.Exec(ctx, "DROP SUBSCRIPTION "+name)
		}
	}
}

// createSubscriptionRefused reports PostgreSQL's refusal of CREATE
// SUBSCRIPTION to a role without the privilege: SQLSTATE 42501, "permission
// denied to create subscription" from 16, "must be superuser to create
// subscriptions" before.
func createSubscriptionRefused(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.InsufficientPrivilege &&
		(strings.Contains(pgErr.Message, "permission denied to create subscription") || strings.Contains(pgErr.Message, "must be superuser to create subscriptions"))
}

func execErr(t *testing.T, uri, sql string) error {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	_, err = conn.Exec(ctx, sql)
	return err
}

// The subscriber verb end to end on a real PostgreSQL (m542 DoD 1-3 at the
// SQL level), whichever form the server's major takes:
//   - 16 and later: the database's role cannot create a subscription until
//     it is enabled; enabled, it subscribes and receives the copy and every
//     change; disable is refused by name while it owns the subscription and
//     goes through once it is dropped; the refusal is PostgreSQL's again.
//     The 13-15 form is refused, writing nothing.
//   - 13-15: the role form is refused; a subscription bex creates is
//     written with its password in a Secret, created by the operator, and
//     removed with its Secret.
func TestLogicalReplicationSubscriberIntegration(t *testing.T) {
	f := newLRFixture(t)
	shortenLogicalReplicationWait(t, 20*time.Second)
	ctx := context.Background()
	publisher := f.asSubscriber()
	var major int
	if conn, err := pgx.Connect(ctx, f.superuser); err == nil {
		_ = conn.QueryRow(ctx, `SELECT current_setting('server_version_num')::int / 10000`).Scan(&major)
		_ = conn.Close(ctx)
	}
	port := sqlString(t, f.superuser, `SHOW port`)
	// A non-superuser's subscription must authenticate with its password
	// (password_required), so it connects to the address the test reached
	// the server on, which pg_hba does not trust as it trusts loopback.
	host := sqlString(t, f.superuser, `SELECT host(inet_server_addr())`)
	if host == "" || host == "127.0.0.1" || host == "::1" {
		t.Skip("the test PostgreSQL is reached over loopback, where pg_hba trusts and a non-superuser subscription cannot use its password")
	}
	slot := "lr_s_" + f.n
	subscribe := fmt.Sprintf(`CREATE SUBSCRIPTION %s CONNECTION 'host=%s port=%s dbname=lr_sub_%s user=lr_pub_%s password=pub-pw' PUBLICATION src_orders WITH (create_slot = false, slot_name = '%s')`,
		slot, host, port, f.n, f.n, slot)
	if err := execErr(t, f.owner, subscribe); !createSubscriptionRefused(err) {
		t.Fatalf("before enabling, the owner's CREATE SUBSCRIPTION = %v, want PostgreSQL's refusal", err)
	}
	f.operateSubscriber(major)
	roleForm := LogicalReplicationRequest{Database: f.db.Name, Enable: true, SubscriberRole: f.ownerRole}
	subscriptionForm := LogicalReplicationRequest{Database: f.db.Name, Enable: true, Subscription: &LogicalSubscriptionRequest{
		Name: "from_src", Publication: "src_orders", PublisherURL: fmt.Sprintf("postgresql://lr_pub_%s:pub-pw@127.0.0.1:%s/lr_sub_%s?sslmode=disable", f.n, port, f.n)}}
	if major < appv1alpha1.PgCreateSubscriptionMajor {
		subscriberIntegration1315(t, f, roleForm, subscriptionForm)
		return
	}

	// The 13-15 form is refused on 16 and later, writing nothing.
	if _, err := f.svc.SetLogicalReplication(ctx, subscriptionForm); codeOf(err) != LogicalReplicationSubscriberFormCode {
		t.Fatalf("the 13-15 form on %d = %v, want %s", major, err, LogicalReplicationSubscriberFormCode)
	}
	if f.spec() != nil || len(f.sink.events) != 0 {
		t.Fatalf("a refusal wrote: spec %+v, %d audit rows", f.spec(), len(f.sink.events))
	}

	// Enable the owner, which owns the tables the subscription writes.
	res, err := f.svc.SetLogicalReplication(ctx, roleForm)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Converged || !res.RoleCreatesSubscriptions || res.SubscriberRole != f.ownerRole || res.RoleReplication || res.PublisherRole != "" {
		t.Fatalf("enable = %+v", res)
	}
	// Within one server the slot is made beforehand, in the publisher's
	// database, as PostgreSQL requires.
	execAs(t, publisher, fmt.Sprintf(`SELECT pg_create_logical_replication_slot('%s', 'pgoutput')`, slot))
	execAs(t, f.owner, subscribe)
	execAs(t, publisher, `INSERT INTO public.orders VALUES (2, 'new')`, `UPDATE public.orders SET note = 'changed' WHERE id = 1`)
	execAs(t, publisher, `INSERT INTO public.orders VALUES (3, 'gone')`, `DELETE FROM public.orders WHERE id = 3`)
	const contents = `SELECT string_agg(id || ':' || note, ',' ORDER BY id) FROM public.orders`
	waitForSQL(t, 30*time.Second, f.owner, contents, "1:changed,2:new")

	// Disable is refused while the role owns the subscription, by name, and
	// writes nothing; asking again for the same enable writes nothing either.
	if res, err := f.svc.SetLogicalReplication(ctx, roleForm); err != nil || !res.Converged || strings.Join(res.LiveSubscriptions, ",") != slot {
		t.Fatalf("enable again = %+v, %v", res, err)
	}
	_, err = f.svc.SetLogicalReplication(ctx, LogicalReplicationRequest{Database: f.db.Name, Subscriber: true})
	if codeOf(err) != LogicalReplicationSubscriptionsExistCode || !strings.Contains(err.Error(), f.dbname+"."+slot) {
		t.Fatalf("disable with a live subscription = %v, want %s naming %s.%s", err, LogicalReplicationSubscriptionsExistCode, f.dbname, slot)
	}
	if lr := f.spec(); lr == nil || lr.SubscriberRole != f.ownerRole || strings.Join(auditVerbs(f.sink), ",") != AuditVerbEnableLogicalReplication {
		t.Fatalf("after the refusal: spec %+v, audit %v", lr, auditVerbs(f.sink))
	}
	// Naming another subscriber role is the same loss for the owner.
	other := roleForm
	other.SubscriberRole = f.readerRole
	if _, err := f.svc.SetLogicalReplication(ctx, other); codeOf(err) != LogicalReplicationSubscriptionsExistCode {
		t.Fatalf("replacing the subscriber role = %v, want %s", err, LogicalReplicationSubscriptionsExistCode)
	}

	// The user drops it; disable goes through and the refusal is
	// PostgreSQL's again.
	execAs(t, f.owner, "ALTER SUBSCRIPTION "+slot+" DISABLE", "ALTER SUBSCRIPTION "+slot+" SET (slot_name = NONE)", "DROP SUBSCRIPTION "+slot)
	res, err = f.svc.SetLogicalReplication(ctx, LogicalReplicationRequest{Database: f.db.Name, Subscriber: true})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Converged || res.Enabled || res.RoleCreatesSubscriptions || f.spec() != nil {
		t.Fatalf("disable = %+v, spec %+v", res, f.spec())
	}
	if err := execErr(t, f.owner, subscribe); !createSubscriptionRefused(err) {
		t.Fatalf("after disabling, the owner's CREATE SUBSCRIPTION = %v, want PostgreSQL's refusal", err)
	}
	if verbs := strings.Join(auditVerbs(f.sink), ","); verbs != AuditVerbEnableLogicalReplication+","+AuditVerbDisableLogicalReplication {
		t.Fatalf("audit rows = %s", verbs)
	}

	// A dedicated credential is granted CREATE on the database, which
	// CREATE SUBSCRIPTION needs beside the membership.
	readerCreates := func() bool {
		return queryBool(t, f.superuser, `SELECT has_database_privilege($1, current_database(), 'CREATE')`, f.readerRole)
	}
	if readerCreates() {
		t.Fatal("the reader had CREATE on the database before it was enabled")
	}
	if res, err := f.svc.SetLogicalReplication(ctx, other); err != nil || !res.Converged || !readerCreates() {
		t.Fatalf("enable the reader = %+v, %v, CREATE %v", res, err, readerCreates())
	}
}

func subscriberIntegration1315(t *testing.T, f *lrFixture, roleForm, subscriptionForm LogicalReplicationRequest) {
	ctx := context.Background()
	if _, err := f.svc.SetLogicalReplication(ctx, roleForm); codeOf(err) != LogicalReplicationSubscriberFormCode {
		t.Fatalf("a subscriber role on 13-15 = %v, want %s", err, LogicalReplicationSubscriberFormCode)
	}
	if f.spec() != nil || len(f.sink.events) != 0 {
		t.Fatalf("a refusal wrote: spec %+v, %d audit rows", f.spec(), len(f.sink.events))
	}
	res, err := f.svc.SetLogicalReplication(ctx, subscriptionForm)
	if err != nil {
		t.Fatal(err)
	}
	lr := f.spec()
	if !res.Converged || strings.Join(res.LiveSubscriptions, ",") != "from_src" || lr == nil || len(lr.Subscriptions) != 1 {
		t.Fatalf("enable = %+v, spec %+v", res, lr)
	}
	sub := lr.Subscriptions[0]
	if sub.Host != "127.0.0.1" || sub.User != "lr_pub_"+f.n || sub.SSLMode != "disable" || sub.Publication != "src_orders" || strings.Contains(fmt.Sprint(lr), "pub-pw") {
		t.Fatalf("spec entry = %+v", sub)
	}
	var sec corev1.Secret
	if err := f.svc.Client.Get(ctx, client.ObjectKey{Namespace: f.db.Namespace, Name: sub.PasswordSecret}, &sec); err != nil || string(sec.Data["password"]) != "pub-pw" {
		t.Fatalf("password Secret = %v, %v", sec.Data, err)
	}
	// The same request again writes and records nothing new.
	if res, err := f.svc.SetLogicalReplication(ctx, subscriptionForm); err != nil || !res.Converged || f.spec().Subscriptions[0].PasswordSecret != sub.PasswordSecret || len(f.sink.events) != 1 {
		t.Fatalf("enable again = %+v, %v, %d audit rows", res, err, len(f.sink.events))
	}
	// Removing it drops it, and its Secret goes.
	res, err = f.svc.SetLogicalReplication(ctx, LogicalReplicationRequest{Database: f.db.Name, Subscription: &LogicalSubscriptionRequest{Name: "from_src"}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Converged || res.Enabled || len(res.LiveSubscriptions) != 0 || f.spec() != nil {
		t.Fatalf("remove = %+v, spec %+v", res, f.spec())
	}
	if err := f.svc.Client.Get(ctx, client.ObjectKey{Namespace: f.db.Namespace, Name: sub.PasswordSecret}, &sec); err == nil {
		t.Error("the removed subscription's password Secret survived")
	}
}
