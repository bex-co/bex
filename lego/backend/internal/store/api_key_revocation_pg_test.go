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
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestPGUnbindClientRevokesCredential(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	run := uniqueMachineRun()
	human, client, other := "human-"+run, "client-"+run, "other-client-"+run
	tenant, err := st.CreateWorkspace(ctx, "key-revocation-"+run, PlanPro, human)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{client, other} {
		if err := st.BindClient(ctx, key, tenant.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.UnbindClient(ctx, client); err != nil {
		t.Fatalf("unbind: %v", err)
	}
	if _, err := st.GetTenantMember(ctx, tenant.ID, client); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked key binding: got %v, want ErrNotFound", err)
	}

	// The marker is shared state, readable by another replica after the binding
	// disappeared. Hydra deletion alone does not invalidate cached tokens.
	replica := NewPGStore(st.Pool)
	at, revoked, err := replica.OAuthRevokedAt(ctx, client, client)
	if err != nil || !revoked || at.IsZero() {
		t.Fatalf("durable revocation: at=%v revoked=%v err=%v", at, revoked, err)
	}
	if err := st.UnbindClient(ctx, client); err != nil {
		t.Fatalf("repeat unbind: %v", err)
	}
	again, revoked, err := replica.OAuthRevokedAt(ctx, client, client)
	if err != nil || !revoked || !again.Equal(at) {
		t.Fatalf("repeat changed permanent marker: at=%v revoked=%v err=%v, want %v", again, revoked, err, at)
	}

	// A lost or already-removed binding still permits fail-closed cleanup.
	unbound := "never-bound-" + run
	if err := st.UnbindClient(ctx, unbound); err != nil {
		t.Fatalf("unbind unknown key: %v", err)
	}
	if _, revoked, err := replica.OAuthRevokedAt(ctx, unbound, unbound); err != nil || !revoked {
		t.Fatalf("unknown key revocation: revoked=%v err=%v", revoked, err)
	}

	// Revocation is scoped to this machine identity; human consent chains and
	// other keys retain both their membership and authentication state.
	for _, subject := range []string{human, other} {
		if member, err := st.IsMember(ctx, subject, tenant.ID); err != nil || !member {
			t.Errorf("unrelated binding %s: member=%v err=%v", subject, member, err)
		}
	}
	for _, identity := range [][2]string{{human, client}, {client, other}, {other, other}} {
		if _, revoked, err := replica.OAuthRevokedAt(ctx, identity[0], identity[1]); err != nil || revoked {
			t.Errorf("unrelated identity %v: revoked=%v err=%v", identity, revoked, err)
		}
	}
}

func TestPGUnbindClientFailureIsAtomic(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	for _, tc := range []struct {
		table, operation, row string
	}{
		{"oauth_revocations", "INSERT", "NEW"},
		{"tenant_members", "DELETE", "OLD"},
	} {
		t.Run(tc.table, func(t *testing.T) {
			run := uniqueMachineRun()
			client := "unbind-failure-" + run
			tenant, err := st.CreateTenant(ctx, "unbind-failure-"+run, PlanHobby)
			if err != nil {
				t.Fatal(err)
			}
			if err := st.BindClient(ctx, client, tenant.ID); err != nil {
				t.Fatal(err)
			}

			// The trigger faults only this test's credential, so other package
			// tests can continue using the shared CI database while it exists.
			function := pgx.Identifier{"fail_unbind_" + run}.Sanitize()
			trigger := pgx.Identifier{"fail_unbind_" + run}.Sanitize()
			_, err = st.Pool.Exec(ctx, fmt.Sprintf(`
				CREATE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
				BEGIN
					IF %s.subject = TG_ARGV[0] THEN
						RAISE EXCEPTION 'injected API key unbind failure';
					END IF;
					RETURN %s;
				END $$;
				CREATE TRIGGER %s BEFORE %s ON %s
				FOR EACH ROW EXECUTE FUNCTION %s('%s')`,
				function, tc.row, tc.row, trigger, tc.operation,
				pgx.Identifier{tc.table}.Sanitize(), function, strings.ReplaceAll(client, "'", "''")))
			if err != nil {
				t.Fatalf("install database fault: %v", err)
			}
			t.Cleanup(func() {
				if _, err := st.Pool.Exec(ctx, "DROP FUNCTION IF EXISTS "+function+"() CASCADE"); err != nil {
					t.Errorf("remove database fault: %v", err)
				}
			})

			err = st.UnbindClient(ctx, client)
			if err == nil || !strings.Contains(err.Error(), "injected API key unbind failure") {
				t.Fatalf("unbind error = %v, want injected database failure", err)
			}
			if member, err := st.IsMember(ctx, client, tenant.ID); err != nil || !member {
				t.Fatalf("failed unbind lost retry binding: member=%v err=%v", member, err)
			}
			if _, revoked, err := st.OAuthRevokedAt(ctx, client, client); err != nil || revoked {
				t.Fatalf("failed unbind committed marker: revoked=%v err=%v", revoked, err)
			}
			if _, err := st.Pool.Exec(ctx, "DROP FUNCTION "+function+"() CASCADE"); err != nil {
				t.Fatal(err)
			}
			if err := st.UnbindClient(ctx, client); err != nil {
				t.Fatalf("retry unbind: %v", err)
			}
			if member, err := st.IsMember(ctx, client, tenant.ID); err != nil || member {
				t.Errorf("retried unbind kept binding: member=%v err=%v", member, err)
			}
			if _, revoked, err := st.OAuthRevokedAt(ctx, client, client); err != nil || !revoked {
				t.Errorf("retried unbind missing marker: revoked=%v err=%v", revoked, err)
			}
		})
	}
}
