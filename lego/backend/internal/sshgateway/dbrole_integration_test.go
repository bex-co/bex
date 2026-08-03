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

package sshgateway

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// dbrole_integration_test.go is w7/m56/t005: the durable proof of the gateway's
// least-privilege database boundary (docs/ADR035-ssh.md §116). It provisions the
// scoped role from the SAME dbrole.sql the production script applies, runs the
// gateway's whole Store surface under it (every operation must succeed), and
// asserts that reads on billing/usage/credential/app tables are refused BY
// POSTGRES (SQLSTATE 42501), not merely unexercised by Go. A future migration or
// store change that silently widens the gateway's needs, or a grant that silently
// widens its reach, fails here.
//
// Gated on BEX_TEST_DB_URI (a superuser connection, used to migrate + create the
// role) — run unconditionally in CI against the ephemeral Postgres.
//
// The role name is fixed and torn down/recreated each run; the suite is
// serialized package-by-package in CI (go test -p 1), so there is no cross-run
// role collision.
const gatewayTestRole = "bex_ssh_gw_test"

// sensitiveTables are control-plane tables the gateway has no business reading;
// a SELECT on each must be refused for the scoped role. Chosen to span the risk
// classes the milestone names: billing + usage (money), credentials, and
// workspace/app data. (There is no api_keys table — OAuth2 clients live in Hydra,
// not this store — so the DoD's "API-key tables" maps to these instead.)
var sensitiveTables = []string{
	"stripe_billing_events", // billing / money
	"usage_hourly",          // metering / money
	"sandbox_meter_states",  // metering cursor / tenant sandbox inventory
	"registry_credentials",  // tenant registry secrets
	"git_connections",       // tenant git credentials
	"apps",                  // workspace resource data
}

func TestGatewayScopedRoleAllowsOwnSurfaceDeniesTheRest(t *testing.T) {
	dbURI := os.Getenv("BEX_TEST_DB_URI")
	if dbURI == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()

	if err := store.Migrate(dbURI); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	admin, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		t.Fatalf("admin pool: %v", err)
	}
	defer admin.Close()

	// (Re)create the role fresh and apply the shipped grant DDL. DROP OWNED BY
	// clears the prior run's privileges so DROP ROLE cannot fail on a dependency.
	const pw = "gw_test_pw"
	recreate := `DO $$ BEGIN
		IF EXISTS (SELECT FROM pg_roles WHERE rolname = '` + gatewayTestRole + `') THEN
			EXECUTE 'DROP OWNED BY ` + gatewayTestRole + `';
			EXECUTE 'DROP ROLE ` + gatewayTestRole + `';
		END IF;
	END $$;`
	if _, err := admin.Exec(ctx, recreate); err != nil {
		t.Fatalf("drop role: %v", err)
	}
	if _, err := admin.Exec(ctx, `CREATE ROLE `+gatewayTestRole+
		` LOGIN PASSWORD '`+pw+`' NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT`); err != nil {
		t.Fatalf("create role: %v", err)
	}
	if _, err := admin.Exec(ctx, RoleGrantsSQL(gatewayTestRole)); err != nil {
		t.Fatalf("apply grants: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DROP OWNED BY `+gatewayTestRole)
		_, _ = admin.Exec(context.Background(), `DROP ROLE IF EXISTS `+gatewayTestRole)
	})

	// Connect AS the scoped role.
	scopedURI, err := swapUserInfo(dbURI, gatewayTestRole, pw)
	if err != nil {
		t.Fatalf("build scoped uri: %v", err)
	}
	scoped, err := pgxpool.New(ctx, scopedURI)
	if err != nil {
		t.Fatalf("scoped pool: %v", err)
	}
	defer scoped.Close()
	st := store.NewPGStore(scoped)

	// --- ALLOW: the whole gateway surface works under the role ---------------
	exercised := map[string]bool{}

	// SSHKeyByFingerprint (SELECT ssh_keys). A non-existent fingerprint returns
	// ErrNotFound — NOT a permission error, which is all this asserts: the SELECT
	// privilege is present.
	if _, err := st.SSHKeyByFingerprint(ctx, "SHA256:none"); permDenied(err) {
		t.Errorf("SSHKeyByFingerprint denied under scoped role: %v", err)
	}
	exercised["SSHKeyByFingerprint"] = true

	// Unique per run — the CI Postgres is shared and additive across runs, and the
	// scoped role deliberately cannot DELETE ssh_sessions, so a fixed id would
	// collide on re-run. Admin cleans the seeded rows up afterward.
	runID := fmt.Sprintf("%d", time.Now().UnixNano())
	sessID, nonce := "sess-m56-"+runID, "nonce-m56-"+runID
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DELETE FROM ssh_sessions WHERE id = $1`, sessID)
		_, _ = admin.Exec(context.Background(), `DELETE FROM shell_ticket_nonces WHERE nonce = $1`, nonce)
	})

	// StartSSHSession (INSERT ssh_sessions) + EndSSHSession (SELECT+UPDATE).
	sess := store.SSHSessionAudit{
		ID: sessID, Subject: "user:test", WorkspaceID: "tea-test",
		ServiceID: "srv-test", InstanceID: "inst-test", RemoteAddress: "10.0.0.1", StartedAt: time.Now().UTC(),
	}
	if err := st.StartSSHSession(ctx, sess); err != nil {
		t.Errorf("StartSSHSession under scoped role: %v", err)
	}
	exercised["StartSSHSession"] = true
	if err := st.EndSSHSession(ctx, sess.ID, "ok", time.Now().UTC()); err != nil {
		t.Errorf("EndSSHSession under scoped role: %v", err)
	}
	exercised["EndSSHSession"] = true

	// ClaimShellNonce (DELETE expired + INSERT shell_ticket_nonces).
	if claimed, err := st.ClaimShellNonce(ctx, nonce, time.Now().Add(time.Minute)); err != nil {
		t.Errorf("ClaimShellNonce under scoped role: %v", err)
	} else if !claimed {
		t.Errorf("ClaimShellNonce should claim a fresh nonce")
	}
	exercised["ClaimShellNonce"] = true

	// The Audit sink (base.Audit -> PGStore.Record, INSERT audit_events). Not part
	// of the Store interface but part of the gateway's write surface, so the role
	// must permit it.
	auditEv := core.AuditEvent{
		Caller: "user:test", CallerMethod: "session", Verb: "apps.ResolveSSHSession",
		Resource: core.WorkspaceObject("tea-test"), Outcome: core.AuditAllowed, At: time.Now().UTC(),
	}
	if err := st.Record(ctx, auditEv); permDenied(err) {
		t.Errorf("audit Record denied under scoped role: %v", err)
	}

	// The agent-session transcript surface (ADR047 D9, w3/m43): the attach
	// listener SELECTs (replay + max-seq) and INSERTs (tee) agent_session_transcripts.
	// Both must be permitted for the scoped role, or the stream returns
	// "transcript unavailable" (caught live on prod).
	if _, _, err := st.AgentSessionTranscriptMaxSeq(ctx, "ags-nope000000000000000"); permDenied(err) {
		t.Errorf("transcript SELECT denied under scoped role: %v", err)
	}

	// --- DENY: sensitive tables are refused by Postgres ----------------------
	for _, table := range sensitiveTables {
		_, err := scoped.Exec(ctx, "SELECT 1 FROM "+table+" LIMIT 1")
		if !permDenied(err) {
			t.Errorf("SELECT on %s was NOT denied for the gateway role (err=%v) — the role reaches a table it must not", table, err)
		}
	}

	// --- Completeness: every Store interface method is exercised above -------
	storeType := reflect.TypeOf((*Store)(nil)).Elem()
	for i := 0; i < storeType.NumMethod(); i++ {
		name := storeType.Method(i).Name
		if !exercised[name] {
			t.Errorf("Store method %q is not exercised under the scoped role — add it (and any grant it needs to dbrole.sql), "+
				"so a new gateway DB dependency can't silently escape the least-privilege proof", name)
		}
	}
}

// permDenied reports whether err is a Postgres insufficient_privilege (42501).
func permDenied(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "42501"
}

// swapUserInfo returns dbURI with its userinfo replaced by user:password — the
// scoped-role connection string derived from the admin one.
func swapUserInfo(dbURI, user, password string) (string, error) {
	u, err := url.Parse(dbURI)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword(user, password)
	return u.String(), nil
}
