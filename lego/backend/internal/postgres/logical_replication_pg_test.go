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
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// lrFixture is a tenant database on the test PostgreSQL: its own database
// owned by a non-superuser owner, a dedicated reader credential, and a
// Database CR whose Secrets point at them, as CNPG's would.
type lrFixture struct {
	t                        *testing.T
	superuser, owner, reader string // connection URIs into the fixture database
	subscriber               string // a superuser URI into the subscriber's database
	ownerRole, readerRole    string
	dbname, n                string
	db                       *appv1alpha1.Database
	svc                      *Service
	sink                     *webhookAuditSink
}

func withRole(t *testing.T, base, database, role, password string) string {
	t.Helper()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal("parse test database URI")
	}
	u.Path = "/" + database
	if role != "" {
		u.User = url.UserPassword(role, password)
	}
	return u.String()
}

func execAs(t *testing.T, uri string, statements ...string) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	for _, s := range statements {
		if _, err := conn.Exec(ctx, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
}

func queryBool(t *testing.T, uri, sql string, args ...any) bool {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	var v bool
	if err := conn.QueryRow(ctx, sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

func newLRFixture(t *testing.T) *lrFixture {
	t.Helper()
	base := testDBURI(t)
	ctx := context.Background()
	if !queryBool(t, base, `SELECT current_setting('wal_level') = 'logical'`) {
		testenv.Skip(t, "the test PostgreSQL does not run wal_level=logical (backend-test.yml and scripts/backend-test-deps.sh start it so)")
	}
	n := fmt.Sprintf("%d_%d", os.Getpid(), time.Now().UnixNano()%1_000_000)
	f := &lrFixture{t: t, ownerRole: "lr_owner_" + n, readerRole: "lr_reader_" + n}
	dbname, subname := "lr_db_"+n, "lr_sub_"+n
	f.dbname, f.n = dbname, n
	execAs(t, base,
		fmt.Sprintf(`CREATE ROLE %s LOGIN PASSWORD 'owner-pw' CREATEDB`, f.ownerRole),
		fmt.Sprintf(`CREATE ROLE %s LOGIN PASSWORD 'reader-pw'`, f.readerRole),
		fmt.Sprintf(`CREATE DATABASE %s OWNER %s`, dbname, f.ownerRole),
		fmt.Sprintf(`CREATE DATABASE %s`, subname),
	)
	f.superuser = withRole(t, base, dbname, "", "")
	f.subscriber = withRole(t, base, subname, "", "")
	f.owner = withRole(t, base, dbname, f.ownerRole, "owner-pw")
	f.reader = withRole(t, base, dbname, f.readerRole, "reader-pw")
	t.Cleanup(func() {
		conn, err := pgx.Connect(ctx, base)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close(ctx) }()
		if sub, err := pgx.Connect(ctx, f.subscriber); err == nil {
			for _, s := range []string{"DISABLE", "SET (slot_name = NONE)"} {
				_, _ = sub.Exec(ctx, fmt.Sprintf(`ALTER SUBSCRIPTION lr_s_%s %s`, n, s))
			}
			_, _ = sub.Exec(ctx, fmt.Sprintf(`DROP SUBSCRIPTION IF EXISTS lr_s_%s`, n))
			_ = sub.Close(ctx)
		}
		_, _ = conn.Exec(ctx, `SELECT pg_terminate_backend(active_pid) FROM pg_replication_slots WHERE database = $1 AND active_pid IS NOT NULL`, dbname)
		_, _ = conn.Exec(ctx, `SELECT pg_drop_replication_slot(slot_name) FROM pg_replication_slots WHERE database = $1 AND NOT active`, dbname)
		_, _ = conn.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %s WITH (FORCE)`, subname))
		_, _ = conn.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS %s WITH (FORCE)`, dbname))
		_, _ = conn.Exec(ctx, fmt.Sprintf(`DROP ROLE IF EXISTS %s`, f.readerRole))
		_, _ = conn.Exec(ctx, fmt.Sprintf(`DROP ROLE IF EXISTS %s`, f.ownerRole))
	})
	execAs(t, f.owner,
		`CREATE SCHEMA sales`,
		`CREATE TABLE public.orders (id int PRIMARY KEY, note text)`,
		`CREATE TABLE sales.items (id int PRIMARY KEY)`,
	)
	f.db = publisherDatabase("dpg-lr-pg")
	f.db.Spec.DatabaseName, f.db.Spec.DatabaseUser = dbname, f.ownerRole
	f.db.Spec.Users = []appv1alpha1.DatabaseUser{{Name: f.readerRole, SecretName: "dpg-lr-pg-reader"}}
	f.db.Spec.DeletedUsers = nil
	secrets := []client.Object{
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "dpg-lr-pg-app", Namespace: "default"},
			Data: map[string][]byte{"uri": []byte(f.owner), "password": []byte("owner-pw")}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "dpg-lr-pg-reader", Namespace: "default"},
			Data: map[string][]byte{"password": []byte("reader-pw")}},
	}
	f.svc, f.sink = newLogicalReplicationService(t, "pro", append(secrets, f.db)...)
	return f
}

// operate plays the operator and CNPG until the test ends: it projects
// spec.logicalReplication onto the database as a superuser would, the role's
// REPLICATION attribute and the all-tables publication.
func (f *lrFixture) operate() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	f.t.Cleanup(func() { cancel(); <-done })
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
				attribute := "NOREPLICATION"
				if lr != nil && lr.PublisherRole == f.readerRole {
					attribute = "REPLICATION"
				}
				_, _ = conn.Exec(ctx, fmt.Sprintf(`ALTER ROLE %s %s`, f.readerRole, attribute))
				if lr != nil && lr.AllTables {
					_, _ = conn.Exec(ctx, `DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = 'bex_all_tables') THEN CREATE PUBLICATION bex_all_tables FOR ALL TABLES; END IF; END $$`)
				} else {
					_, _ = conn.Exec(ctx, `DROP PUBLICATION IF EXISTS bex_all_tables`)
				}
			}
			select {
			case <-ctx.Done():
			case <-time.After(50 * time.Millisecond):
			}
		}
	}()
}

func (f *lrFixture) spec() *appv1alpha1.DatabaseLogicalReplication {
	f.t.Helper()
	var d appv1alpha1.Database
	if err := f.svc.Client.Get(context.Background(), client.ObjectKeyFromObject(f.db), &d); err != nil {
		f.t.Fatal(err)
	}
	return d.Spec.LogicalReplication
}

// walsenderRefused reports PostgreSQL's refusal of a walsender to a role
// without REPLICATION: SQLSTATE 42501, worded "permission denied to start WAL
// sender" since PostgreSQL 16 and "must be superuser or replication role to
// start walsender" before.
func walsenderRefused(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.InsufficientPrivilege &&
		(strings.Contains(pgErr.Message, "WAL sender") || strings.Contains(pgErr.Message, "walsender"))
}

// walsender opens a logical replication connection as the reader, as a
// subscriber's apply worker does, and returns PostgreSQL's refusal if any.
func walsender(t *testing.T, uri string) error {
	t.Helper()
	cfg, err := pgconn.ParseConfig(uri)
	if err != nil {
		t.Fatal(err)
	}
	cfg.RuntimeParams["replication"] = "database"
	conn, err := pgconn.ConnectConfig(context.Background(), cfg)
	if err != nil {
		return err
	}
	_ = conn.Close(context.Background())
	return nil
}

func shortenLogicalReplicationWait(t *testing.T, timeout time.Duration) {
	t.Helper()
	oldTimeout, oldPoll := logicalReplicationConvergeTimeout, logicalReplicationPollInterval
	logicalReplicationConvergeTimeout, logicalReplicationPollInterval = timeout, 50*time.Millisecond
	t.Cleanup(func() { logicalReplicationConvergeTimeout, logicalReplicationPollInterval = oldTimeout, oldPoll })
}

// The verb end to end on a real PostgreSQL, with the operator played by a
// superuser loop: a missing schema and a user's publication of the reserved
// name refuse with nothing written; enable grants the schemas (and tables
// created later), lets the reader start a walsender and decode the
// all-tables publication; disable drops the reader's slots and the
// publication, and the walsender is refused again (DoD 1-4, at the SQL level).
func TestLogicalReplicationIntegration(t *testing.T) {
	f := newLRFixture(t)
	shortenLogicalReplicationWait(t, 20*time.Second)
	ctx := context.Background()
	if err := walsender(t, f.reader); !walsenderRefused(err) {
		t.Fatalf("before enabling, the reader's walsender = %v, want PostgreSQL's refusal", err)
	}
	readerSelects := func(table string) bool {
		return queryBool(t, f.superuser, `SELECT has_table_privilege($1, $2, 'SELECT')`, f.readerRole, table)
	}
	enable := LogicalReplicationRequest{Database: f.db.Name, Enable: true, PublisherRole: f.readerRole, Schemas: []string{"public", "sales"}, AllTables: true}

	// Refusals that need the database read it, and still write nothing.
	missing := enable
	missing.Schemas = []string{"public", "analytics"}
	if _, err := f.svc.SetLogicalReplication(ctx, missing); codeOf(err) != LogicalReplicationSchemaUnknownCode || !strings.Contains(err.Error(), "analytics") {
		t.Fatalf("a missing schema = %v, want %s naming it", err, LogicalReplicationSchemaUnknownCode)
	}
	execAs(t, f.owner, `CREATE PUBLICATION bex_all_tables FOR TABLE public.orders`)
	if _, err := f.svc.SetLogicalReplication(ctx, enable); codeOf(err) != LogicalReplicationPublicationInUseCode {
		t.Fatalf("a user's publication of the name = %v, want %s", err, LogicalReplicationPublicationInUseCode)
	}
	if f.spec() != nil || len(f.sink.events) != 0 || readerSelects("public.orders") {
		t.Fatalf("a refusal wrote: spec %+v, %d audit rows, reader selects %v", f.spec(), len(f.sink.events), readerSelects("public.orders"))
	}
	execAs(t, f.owner, `DROP PUBLICATION bex_all_tables`)

	// Enable.
	f.operate()
	res, err := f.svc.SetLogicalReplication(ctx, enable)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Converged || !res.RoleReplication || !res.PublicationLive || res.Publication != appv1alpha1.LogicalReplicationAllTablesPublication ||
		res.ExternalHost != f.db.Status.ExternalHost || strings.Join(res.Schemas, ",") != "public,sales" {
		t.Fatalf("enable = %+v", res)
	}
	if lr := f.spec(); lr == nil || lr.PublisherRole != f.readerRole || !lr.AllTables {
		t.Fatalf("spec = %+v", lr)
	}
	execAs(t, f.owner, `CREATE TABLE sales.later (id int PRIMARY KEY)`)
	for _, table := range []string{"public.orders", "sales.items", "sales.later"} {
		if !readerSelects(table) {
			t.Errorf("the reader cannot SELECT %s", table)
		}
	}
	if err := walsender(t, f.reader); err != nil {
		t.Fatalf("enabled, the reader's walsender = %v", err)
	}
	// A real subscription to the all-tables publication, connected as the
	// reader, copies the existing rows and then receives every insert, update
	// and delete. (Within one server the slot is created beforehand, as
	// PostgreSQL requires.) The owner's own FOR TABLE publication decodes for
	// the reader too, through a second slot.
	execAs(t, f.owner, `INSERT INTO public.orders VALUES (1, 'existing')`, `INSERT INTO sales.items VALUES (1)`)
	slot := "lr_s_" + f.n
	execAs(t, f.reader, fmt.Sprintf(`SELECT pg_create_logical_replication_slot('%s', 'pgoutput')`, slot))
	var port string
	if conn, err := pgx.Connect(ctx, f.superuser); err == nil {
		_ = conn.QueryRow(ctx, `SHOW port`).Scan(&port)
		_ = conn.Close(ctx)
	}
	execAs(t, f.subscriber,
		`CREATE SCHEMA sales`, `CREATE TABLE public.orders (id int PRIMARY KEY, note text)`, `CREATE TABLE sales.items (id int PRIMARY KEY)`, `CREATE TABLE sales.later (id int PRIMARY KEY)`,
		fmt.Sprintf(`CREATE SUBSCRIPTION %s CONNECTION 'host=127.0.0.1 port=%s dbname=%s user=%s password=reader-pw' PUBLICATION bex_all_tables WITH (create_slot = false, slot_name = '%s')`,
			slot, port, f.dbname, f.readerRole, slot))
	execAs(t, f.owner, `CREATE PUBLICATION mine FOR TABLE public.orders`)
	execAs(t, f.reader, `SELECT pg_create_logical_replication_slot('lr_peek', 'pgoutput')`)
	execAs(t, f.owner, `INSERT INTO public.orders VALUES (2, 'new')`, `UPDATE public.orders SET note = 'changed' WHERE id = 1`,
		`INSERT INTO sales.items VALUES (2)`, `DELETE FROM sales.items WHERE id = 1`)
	const contents = `SELECT (SELECT string_agg(id || ':' || note, ',' ORDER BY id) FROM public.orders) || '|' || (SELECT string_agg(id::text, ',' ORDER BY id) FROM sales.items)`
	want := "1:changed,2:new|2"
	waitForSQL(t, 30*time.Second, f.subscriber, contents, want)
	if !queryBool(t, f.reader, `SELECT count(*) >= 2 FROM pg_logical_slot_peek_binary_changes('lr_peek', NULL, NULL, 'proto_version', '1', 'publication_names', 'mine')`) {
		t.Error("the reader decoded nothing through the owner's FOR TABLE publication")
	}
	// Asking again changes and records nothing.
	if res, err := f.svc.SetLogicalReplication(ctx, enable); err != nil || !res.Converged {
		t.Fatalf("enable again = %+v, %v", res, err)
	}
	if verbs := auditVerbs(f.sink); strings.Join(verbs, ",") != AuditVerbEnableLogicalReplication {
		t.Fatalf("audit rows = %v, want one %s", verbs, AuditVerbEnableLogicalReplication)
	}
	if ev := f.sink.events[0]; ev.Caller != logicalReplicationAuditCaller || ev.CallerMethod != "internal" {
		t.Errorf("audit row caller = %q/%q", ev.Caller, ev.CallerMethod)
	}

	// Disable.
	res, err = f.svc.SetLogicalReplication(ctx, LogicalReplicationRequest{Database: f.db.Name})
	if err != nil {
		t.Fatal(err)
	}
	// The subscription's apply worker reconnects as soon as its walsender
	// ends; the slots still go.
	if !res.Converged || res.Enabled || res.RoleReplication || res.PublicationLive ||
		!slices.Equal(res.DroppedSlots, []string{"lr_peek", slot}) || len(res.RemainingSlots) != 0 {
		t.Fatalf("disable = %+v", res)
	}
	if f.spec() != nil {
		t.Fatalf("spec after disable = %+v", f.spec())
	}
	if err := walsender(t, f.reader); !walsenderRefused(err) {
		t.Fatalf("after disabling, the reader's walsender = %v, want PostgreSQL's refusal", err)
	}
	if verbs := auditVerbs(f.sink); strings.Join(verbs, ",") != AuditVerbEnableLogicalReplication+","+AuditVerbDisableLogicalReplication {
		t.Fatalf("audit rows = %v", verbs)
	}
	// The grants were the user's to keep: they stay. Nothing streams on.
	if !readerSelects("sales.items") {
		t.Error("disable revoked the reader's grants")
	}
	execAs(t, f.owner, `INSERT INTO public.orders VALUES (3, 'after')`)
	time.Sleep(2 * time.Second)
	if got := sqlString(t, f.subscriber, contents); got != want {
		t.Errorf("the subscriber received a change after the disable: %s", got)
	}
}

func sqlString(t *testing.T, uri, sql string) string {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, uri)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	var v *string
	if err := conn.QueryRow(ctx, sql).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	if v == nil {
		return ""
	}
	return *v
}

func waitForSQL(t *testing.T, timeout time.Duration, uri, sql, want string) {
	t.Helper()
	got := ""
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if got = sqlString(t, uri, sql); got == want {
			return
		}
	}
	t.Fatalf("%s = %q after %s, want %q", sql, got, timeout, want)
}

// A database the operator never converges answers with what it read and the
// operator's word, not an error: the spec is written and recorded.
func TestLogicalReplicationReportsAnUnconvergedPublisher(t *testing.T) {
	f := newLRFixture(t)
	shortenLogicalReplicationWait(t, 300*time.Millisecond)
	res, err := f.svc.SetLogicalReplication(context.Background(),
		LogicalReplicationRequest{Database: f.db.Name, Enable: true, PublisherRole: f.readerRole, Schemas: []string{"public"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Converged || res.RoleReplication || res.Message == "" {
		t.Fatalf("result = %+v, want not converged with the operator's report", res)
	}
	if f.spec() == nil || len(f.sink.events) != 1 {
		t.Fatalf("spec %+v, %d audit rows; want the request written and recorded", f.spec(), len(f.sink.events))
	}
}

func auditVerbs(sink *webhookAuditSink) []string {
	verbs := make([]string, len(sink.events))
	for i, ev := range sink.events {
		verbs[i] = ev.Verb
	}
	return verbs
}
