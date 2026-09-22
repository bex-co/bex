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
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func TestWebhookAttemptsAccountCleanupPreservesEvidencePG(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	run := uniqueMachineRun()
	subject, other := "webhook-departing-"+run, "webhook-remaining-"+run
	unrelated := "webhook-unrelated-" + run
	tenant, err := st.CreateWorkspace(ctx, "webhook-account-"+run, PlanPro, other)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = st.Pool.Exec(ctx, `DELETE FROM tenants WHERE id=$1`, tenant.ID)
		_, _ = st.Pool.Exec(ctx, `DELETE FROM account_deletions WHERE subject=ANY($1)`, []string{subject, unrelated})
	})
	if err := st.AddMember(ctx, subject, tenant.ID, "developer"); err != nil {
		t.Fatal(err)
	}
	endpoint, err := st.CreateWebhookEndpoint(ctx, tenant.ID, "cleanup", "https://hooks.example.test/events", "whsec_test", []string{"deploy_started"}, true, other)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Truncate(time.Second)
	type fixture struct {
		request WebhookResendRequest
		id      string
		state   string
	}
	var fixtures []fixture
	for _, owner := range []string{subject, other} {
		for _, state := range []string{WebhookAttemptPending, WebhookAttemptFailed, WebhookAttemptDelivered} {
			notification := WebhookDelivery{
				ID: ids.New(ids.WebhookDelivery), EndpointID: endpoint.ID,
				EventID: ids.Derive(ids.Event, owner+":"+state), EventType: "deploy_started", ServiceID: "cleanup-api",
				Payload: `{"type":"deploy_started","data":{"serviceId":"cleanup-api"}}`, NextAttemptAt: base,
			}
			if _, err := st.EnqueueWebhookDeliveries(ctx, []WebhookDelivery{notification}, base, "cleanup-"+run, 0); err != nil {
				t.Fatal(err)
			}
			if ok, err := st.CompleteWebhookAttempt(ctx, WebhookAttemptCompletion{
				AttemptID: notification.ID, StatusCode: 204, ResponseBody: "initial exchange",
				CompletedAt: base.Add(time.Minute), Delivered: true,
			}); err != nil || !ok {
				t.Fatalf("initial completion: %v, %v", ok, err)
			}
			request := WebhookResendRequest{
				TenantID: tenant.ID, EndpointID: endpoint.ID, SourceAttemptID: notification.ID,
				RequestedBy: owner, IdempotencyKey: owner + "-" + state, RequestedAt: base.Add(2 * time.Minute),
			}
			attempt, err := st.QueueWebhookResend(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if state != WebhookAttemptPending {
				completion := WebhookAttemptCompletion{
					AttemptID: attempt.ID, StatusCode: 503, TransportError: "receiver unavailable",
					ResponseBody: "retained exchange evidence", CompletedAt: base.Add(3 * time.Minute),
				}
				if state == WebhookAttemptDelivered {
					completion.StatusCode, completion.TransportError, completion.Delivered = 202, "", true
				}
				if ok, err := st.CompleteWebhookAttempt(ctx, completion); err != nil || !ok {
					t.Fatalf("manual completion: %v, %v", ok, err)
				}
			}
			fixtures = append(fixtures, fixture{request: request, id: attempt.ID, state: state})
		}
	}

	snapshot := func() string {
		t.Helper()
		var evidence string
		if err := st.Pool.QueryRow(ctx, `
			SELECT jsonb_agg(jsonb_build_object(
			    'attempt', to_jsonb(a)-'requested_by', 'notification', to_jsonb(d)
			) ORDER BY a.id)::text
			FROM webhook_delivery_attempts a
			JOIN webhook_deliveries d ON d.id=a.notification_id
			WHERE a.endpoint_id=$1`, endpoint.ID).Scan(&evidence); err != nil {
			t.Fatal(err)
		}
		return evidence
	}
	before := snapshot()
	deletion, err := st.BeginAccountDeletion(ctx, subject, "webhook-"+run+"@example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	foreignDeletion, err := st.BeginAccountDeletion(ctx, unrelated, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	var pendingID string
	for _, f := range fixtures {
		if f.request.RequestedBy != subject {
			continue
		}
		if f.state == WebhookAttemptPending {
			pendingID = f.id
		}
		for _, change := range []struct {
			name, query string
			value       string
		}{
			{"different creator", `UPDATE webhook_delivery_attempts SET requested_by=$2 WHERE id=$1`, other},
			{"unrecorded marker", `UPDATE webhook_delivery_attempts SET requested_by=$2 WHERE id=$1`, "deleted:unrecorded"},
			{"another deletion's marker", `UPDATE webhook_delivery_attempts SET requested_by=$2 WHERE id=$1`, foreignDeletion.DeletedMarker},
			{"marker plus lease", `UPDATE webhook_delivery_attempts SET requested_by=$2, lease_until=now() WHERE id=$1`, deletion.DeletedMarker},
		} {
			_, err := st.Pool.Exec(ctx, change.query, f.id, change.value)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
				t.Fatalf("%s %s: got %v, want immutable constraint", f.state, change.name, err)
			}
		}
	}
	if err := st.RemoveAccountMember(ctx, tenant.ID, subject); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := st.CleanupAccountSubject(ctx, subject, deletion.DeletedMarker); err != nil {
			t.Fatalf("cleanup: %v", err)
		}
	}
	if after := snapshot(); after != before {
		t.Fatalf("cleanup changed delivery evidence or replay metadata:\nbefore %s\nafter  %s", before, after)
	}
	for _, f := range fixtures {
		attempt, err := st.QueueWebhookResend(ctx, f.request)
		wantOwner := f.request.RequestedBy
		if wantOwner == subject {
			wantOwner = deletion.DeletedMarker
		}
		if err != nil || attempt.ID != f.id || attempt.Status != f.state || attempt.RequestedBy != wantOwner {
			t.Errorf("replay after cleanup: %+v, %v; want %s/%s/%s", attempt, err, f.id, f.state, wantOwner)
		}
		if f.state != WebhookAttemptPending {
			_, err := st.Pool.Exec(ctx, `UPDATE webhook_delivery_attempts SET response_body='rewritten' WHERE id=$1`, f.id)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
				t.Errorf("terminal evidence rewrite: got %v, want immutable constraint", err)
			}
		}
	}
	// Anonymizing an unsent reservation must not prevent its ordinary worker
	// lease and one permitted terminal transition.
	if pendingID == "" {
		t.Fatal("missing departing requester pending fixture")
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE webhook_delivery_attempts SET lease_until=now()+interval '2 minutes' WHERE id=$1`, pendingID); err != nil {
		t.Fatalf("lease anonymized reservation: %v", err)
	}
	if ok, err := st.CompleteWebhookAttempt(ctx, WebhookAttemptCompletion{
		AttemptID: pendingID, StatusCode: 204, CompletedAt: base.Add(4 * time.Minute), Delivered: true,
	}); err != nil || !ok {
		t.Fatalf("complete anonymized reservation: %v, %v", ok, err)
	}
}

func TestWebhookRequesterAnonymizationMigrationRoundTripPG(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	up, err := migrationsFS.ReadFile("migrations/0134_webhook_requester_anonymization.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := migrationsFS.ReadFile("migrations/0134_webhook_requester_anonymization.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := st.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		CREATE SCHEMA migration_0134_webhook_requester;
		SET LOCAL search_path TO migration_0134_webhook_requester;
		CREATE TABLE account_deletions (subject text PRIMARY KEY, deleted_marker text NOT NULL);
		CREATE TABLE deploys (LIKE public.deploys INCLUDING DEFAULTS INCLUDING CONSTRAINTS);
		CREATE TABLE webhook_delivery_attempts (
			LIKE public.webhook_delivery_attempts INCLUDING DEFAULTS INCLUDING CONSTRAINTS,
			future_evidence text NOT NULL DEFAULT 'preserved'
		);
		INSERT INTO account_deletions VALUES ('departing', 'deleted:recorded');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("apply 0134: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		CREATE TRIGGER webhook_delivery_attempts_immutable
		BEFORE UPDATE ON webhook_delivery_attempts
		FOR EACH ROW EXECUTE FUNCTION bex_guard_webhook_attempt_update();
	`); err != nil {
		t.Fatal(err)
	}
	first, second := ids.New(ids.WebhookDelivery), ids.New(ids.WebhookDelivery)
	for _, id := range []string{first, second} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO webhook_delivery_attempts
			    (id, notification_id, endpoint_id, attempt_number, status, origin,
			     requested_by, idempotency_key, available_at, sent_at, status_code, response_body)
			VALUES ($1,$1,'endpoint',1,'delivered','manual','departing',$1,now(),now(),202,'evidence')`, id); err != nil {
			t.Fatal(err)
		}
	}
	reject := func(query string, args ...any) {
		t.Helper()
		savepoint, err := tx.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, mutationErr := savepoint.Exec(ctx, query, args...)
		var pgErr *pgconn.PgError
		if !errors.As(mutationErr, &pgErr) || pgErr.Code != "23514" {
			t.Errorf("mutation: got %v, want immutable constraint", mutationErr)
		}
		if err := savepoint.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
	reject(`UPDATE webhook_delivery_attempts SET requested_by='deleted:recorded', future_evidence='rewritten' WHERE id=$1`, first)
	if _, err := tx.Exec(ctx, `UPDATE webhook_delivery_attempts SET requested_by='deleted:recorded' WHERE id=$1`, first); err != nil {
		t.Fatalf("anonymize after up: %v", err)
	}
	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("rollback 0134: %v", err)
	}
	reject(`UPDATE webhook_delivery_attempts SET requested_by='deleted:recorded' WHERE id=$1`, second)
	reject(`UPDATE webhook_delivery_attempts SET response_body='rewritten' WHERE id=$1`, first)
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("reapply 0134: %v", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE webhook_delivery_attempts SET requested_by='deleted:recorded' WHERE id=$1`, second); err != nil {
		t.Fatalf("anonymize after reapply: %v", err)
	}
	var preserved int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM webhook_delivery_attempts
		WHERE requested_by='deleted:recorded' AND response_body='evidence' AND future_evidence='preserved'`).Scan(&preserved); err != nil || preserved != 2 {
		t.Fatalf("round trip evidence: rows=%d err=%v", preserved, err)
	}
}
