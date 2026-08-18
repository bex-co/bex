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
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func TestWebhookAttemptLedgerAndResend(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `DELETE FROM tenants WHERE name='webhook-attempt-ledger'`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	tenant, err := s.CreateTenant(ctx, "webhook-attempt-ledger", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id=$1`, tenant.ID) }()
	endpoint, err := s.CreateWebhookEndpoint(ctx, tenant.ID, "ledger", "https://hooks.example.test/events", "whsec_ledger", []string{"deploy_started"}, true, "user-owner")
	if err != nil {
		t.Fatal(err)
	}

	base := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	notification := WebhookDelivery{
		ID: ids.New(ids.WebhookDelivery), EndpointID: endpoint.ID,
		EventID: "evt-ledger00000000000", EventType: "deploy_started", ServiceID: "acme-api",
		Payload: `{"type":"deploy_started","data":{"id":"evt-ledger00000000000"}}`, NextAttemptAt: base,
	}
	if err := s.EnqueueWebhookDeliveries(ctx, []WebhookDelivery{notification}, base, "ledger-watermark"); err != nil {
		t.Fatal(err)
	}

	// A reservation with no network evidence cannot be converted into a manual
	// replay by guessing its whd.
	if _, err := s.QueueWebhookResend(ctx, WebhookResendRequest{
		TenantID: tenant.ID, EndpointID: endpoint.ID, SourceAttemptID: notification.ID,
		RequestedBy: "user-owner", IdempotencyKey: "pending-source-key", RequestedAt: base,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("resend pending source = %v, want ErrNotFound", err)
	}
	claimed, err := s.ClaimDueWebhookAttempts(ctx, base, base.Add(time.Minute), 10)
	if err != nil || len(claimed) != 1 || claimed[0].ID != notification.ID {
		t.Fatalf("initial claim = %+v, %v", claimed, err)
	}
	retryAt := base.Add(time.Hour)
	retryID := ids.New(ids.WebhookDelivery)
	// A legal unusual 7xx response is failed evidence, not a constraint error
	// that leaves the reservation stuck forever.
	if completed, err := s.CompleteWebhookAttempt(ctx, WebhookAttemptCompletion{
		AttemptID: notification.ID, NextAttemptID: retryID, StatusCode: 700,
		TransportError: "receiver returned an unusual status", ResponseBody: "odd",
		CompletedAt: base.Add(time.Minute), NextAttemptAt: retryAt,
	}); err != nil || !completed {
		t.Fatalf("complete unusual response = %v, %v", completed, err)
	}
	failed, err := s.ListWebhookAttempts(ctx, WebhookAttemptFilter{EndpointID: endpoint.ID, Status: WebhookAttemptFailed, Limit: 10})
	if err != nil || len(failed) != 1 {
		t.Fatalf("failed history = %+v, %v", failed, err)
	}
	if got := failed[0]; got.ID != notification.ID || got.StatusCode != 700 || got.ParentStatus != WebhookAttemptPending || got.NextAttemptAt == nil || !got.NextAttemptAt.Equal(retryAt) || got.Payload != notification.Payload {
		t.Fatalf("failed attempt/parent diagnostics = %+v", got)
	}

	// Same-key concurrent Resend requests converge on one pending reservation.
	const callers = 8
	var wg sync.WaitGroup
	idsSeen := make(chan string, callers)
	errs := make(chan error, callers)
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			attempt, queueErr := s.QueueWebhookResend(ctx, WebhookResendRequest{
				TenantID: tenant.ID, EndpointID: endpoint.ID, SourceAttemptID: notification.ID,
				RequestedBy: "user-owner", IdempotencyKey: "resend-key-0001", RequestedAt: base.Add(2 * time.Minute),
			})
			idsSeen <- attempt.ID
			errs <- queueErr
		}()
	}
	wg.Wait()
	close(idsSeen)
	close(errs)
	var manualID string
	for err := range errs {
		if err != nil {
			t.Fatalf("idempotent concurrent resend: %v", err)
		}
	}
	for id := range idsSeen {
		if manualID == "" {
			manualID = id
		}
		if id != manualID {
			t.Fatalf("same idempotency key returned %q and %q", manualID, id)
		}
	}
	manual, err := getWebhookAttempt(ctx, pool, manualID)
	if err != nil || manual.Status != WebhookAttemptPending || manual.Origin != WebhookAttemptManual || manual.RequestedBy != "user-owner" || manual.ResumeAutomaticAt == nil || !manual.ResumeAutomaticAt.Equal(retryAt) {
		t.Fatalf("manual reservation = %+v, %v", manual, err)
	}
	if _, err := s.QueueWebhookResend(ctx, WebhookResendRequest{
		TenantID: tenant.ID, EndpointID: endpoint.ID, SourceAttemptID: notification.ID,
		RequestedBy: "user-owner", IdempotencyKey: "resend-key-0002", RequestedAt: base.Add(3 * time.Minute),
	}); !errors.Is(err, ErrWebhookAttemptPending) {
		t.Fatalf("distinct concurrent resend = %v, want pending conflict", err)
	}
	if _, err := s.QueueWebhookResend(ctx, WebhookResendRequest{
		TenantID: "tea-foreign00000000", EndpointID: endpoint.ID, SourceAttemptID: notification.ID,
		RequestedBy: "user-owner", IdempotencyKey: "resend-key-foreign", RequestedAt: base,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign resend = %v, want ErrNotFound", err)
	}

	manualDue, err := s.ClaimDueWebhookAttempts(ctx, base.Add(4*time.Minute), base.Add(5*time.Minute), 10)
	if err != nil || len(manualDue) != 1 || manualDue[0].ID != manualID || manualDue[0].Origin != WebhookAttemptManual {
		t.Fatalf("manual claim = %+v, %v", manualDue, err)
	}
	resumedID := ids.New(ids.WebhookDelivery)
	if completed, err := s.CompleteWebhookAttempt(ctx, WebhookAttemptCompletion{
		AttemptID: manualID, NextAttemptID: resumedID, StatusCode: 500,
		TransportError: "manual receiver failure", ResponseBody: "failed",
		CompletedAt: base.Add(4 * time.Minute), NextAttemptAt: base.Add(4 * time.Minute),
	}); err != nil || !completed {
		t.Fatalf("complete failed manual = %v, %v", completed, err)
	}
	// A second completion cannot rewrite evidence or create another retry.
	if completed, err := s.CompleteWebhookAttempt(ctx, WebhookAttemptCompletion{
		AttemptID: manualID, NextAttemptID: ids.New(ids.WebhookDelivery), StatusCode: 204,
		CompletedAt: base.Add(5 * time.Minute), NextAttemptAt: base.Add(5 * time.Minute), Delivered: true,
	}); err != nil || completed {
		t.Fatalf("repeat completion = %v, %v, want false,nil", completed, err)
	}
	var automaticCount int
	if err := pool.QueryRow(ctx, `SELECT attempt_count FROM webhook_deliveries WHERE id=$1`, notification.ID).Scan(&automaticCount); err != nil || automaticCount != 1 {
		t.Fatalf("automatic attempt budget after manual = %d, %v, want 1", automaticCount, err)
	}
	// The parked retry was restored at its original due time, not shifted by the
	// manual exchange.
	if due, err := s.ClaimDueWebhookAttempts(ctx, retryAt.Add(-time.Second), retryAt.Add(time.Minute), 10); err != nil || len(due) != 0 {
		t.Fatalf("automatic retry claimed early = %+v, %v", due, err)
	}

	// A second manual Resend supersedes the restored auto; success drops it and
	// closes the parent. Manual sends still do not consume automatic attempt_count.
	manualSuccess, err := s.QueueWebhookResend(ctx, WebhookResendRequest{
		TenantID: tenant.ID, EndpointID: endpoint.ID, SourceAttemptID: manualID,
		RequestedBy: "user-owner", IdempotencyKey: "resend-key-0003", RequestedAt: base.Add(6 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if completed, err := s.CompleteWebhookAttempt(ctx, WebhookAttemptCompletion{
		AttemptID: manualSuccess.ID, StatusCode: 204, ResponseBody: "accepted",
		CompletedAt: base.Add(7 * time.Minute), NextAttemptAt: base.Add(7 * time.Minute), Delivered: true,
	}); err != nil || !completed {
		t.Fatalf("complete successful manual = %v, %v", completed, err)
	}
	if due, err := s.ClaimDueWebhookAttempts(ctx, retryAt.Add(time.Hour), retryAt.Add(2*time.Hour), 10); err != nil || len(due) != 0 {
		t.Fatalf("automatic retry survived manual success = %+v, %v", due, err)
	}

	history, err := s.ListWebhookAttempts(ctx, WebhookAttemptFilter{EndpointID: endpoint.ID, Limit: 10})
	if err != nil || len(history) != 3 {
		t.Fatalf("attempt history = %+v, %v", history, err)
	}
	if history[0].ID != manualSuccess.ID || history[0].Status != WebhookAttemptDelivered || history[1].ID != manualID || history[1].Status != WebhookAttemptFailed || history[2].ID != notification.ID {
		t.Fatalf("attempt ordering/evidence = %+v", history)
	}
	if err := pool.QueryRow(ctx, `SELECT attempt_count FROM webhook_deliveries WHERE id=$1`, notification.ID).Scan(&automaticCount); err != nil || automaticCount != 1 {
		t.Fatalf("manual success consumed automatic budget = %d, %v", automaticCount, err)
	}

	if _, err := s.SetWebhookEndpointEnabled(ctx, tenant.ID, endpoint.ID, false, "manual"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.QueueWebhookResend(ctx, WebhookResendRequest{
		TenantID: tenant.ID, EndpointID: endpoint.ID, SourceAttemptID: manualSuccess.ID,
		RequestedBy: "user-owner", IdempotencyKey: "resend-key-disabled", RequestedAt: base.Add(8 * time.Minute),
	}); !errors.Is(err, ErrWebhookEndpointDisabled) {
		t.Fatalf("disabled endpoint resend = %v", err)
	}
	// Exact idempotent replay remains readable even after later disable.
	repeated, err := s.QueueWebhookResend(ctx, WebhookResendRequest{
		TenantID: tenant.ID, EndpointID: endpoint.ID, SourceAttemptID: manualID,
		RequestedBy: "user-owner", IdempotencyKey: "resend-key-0001", RequestedAt: base.Add(8 * time.Minute),
	})
	if err != nil || repeated.ID != manualID || repeated.Status != WebhookAttemptFailed {
		t.Fatalf("late idempotent repeat = %+v, %v", repeated, err)
	}

	// keep=2 counts three immutable attempts and deletes the whole oldest
	// notification, cascading payload parent + evidence instead of orphaning or
	// retaining a partial sequence.
	if n, err := s.SweepWebhookDeliveries(ctx, time.Time{}, 2, 100); err != nil || n != 1 {
		t.Fatalf("attempt-count sweep = %d, %v", n, err)
	}
	var parents, attempts int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries WHERE id=$1`, notification.ID).Scan(&parents); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_delivery_attempts WHERE notification_id=$1`, notification.ID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if parents != 0 || attempts != 0 {
		t.Fatalf("retention left parent=%d attempts=%d", parents, attempts)
	}
	if _, err := s.SetWebhookEndpointEnabled(ctx, tenant.ID, endpoint.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.QueueWebhookResend(ctx, WebhookResendRequest{
		TenantID: tenant.ID, EndpointID: endpoint.ID, SourceAttemptID: manualSuccess.ID,
		RequestedBy: "user-owner", IdempotencyKey: "resend-key-deleted", RequestedAt: base,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted notification resend = %v, want ErrNotFound", err)
	}
	if err := s.DeleteWebhookEndpoint(ctx, tenant.ID, endpoint.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.QueueWebhookResend(ctx, WebhookResendRequest{
		TenantID: tenant.ID, EndpointID: endpoint.ID, SourceAttemptID: manualSuccess.ID,
		RequestedBy: "user-owner", IdempotencyKey: "resend-key-deleted-endpoint", RequestedAt: base,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted endpoint resend = %v, want ErrNotFound", err)
	}
}

func TestWebhookAttemptResendClaimAndCompletionRaces(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `DELETE FROM tenants WHERE name='webhook-attempt-races'`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	tenant, err := s.CreateTenant(ctx, "webhook-attempt-races", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, tenant.ID) }()
	endpoint, err := s.CreateWebhookEndpoint(ctx, tenant.ID, "races", "https://hooks.example.test/races", "whsec_races", []string{"deploy_started"}, true, "user-races")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 17, 14, 0, 0, 0, time.UTC)

	enqueueAndFail := func(eventID string, at, retryAt time.Time) (WebhookDelivery, string) {
		t.Helper()
		notification := WebhookDelivery{
			ID: ids.New(ids.WebhookDelivery), EndpointID: endpoint.ID,
			EventID: eventID, EventType: "deploy_started", ServiceID: "race-service",
			Payload: `{"type":"deploy_started"}`, NextAttemptAt: at,
		}
		if err := s.EnqueueWebhookDeliveries(ctx, []WebhookDelivery{notification}, at, eventID); err != nil {
			t.Fatal(err)
		}
		claimed, err := s.ClaimDueWebhookAttempts(ctx, at, at.Add(time.Minute), 10)
		if err != nil || len(claimed) != 1 || claimed[0].ID != notification.ID {
			t.Fatalf("claim initial %s = %+v, %v", eventID, claimed, err)
		}
		retryID := ids.New(ids.WebhookDelivery)
		if completed, err := s.CompleteWebhookAttempt(ctx, WebhookAttemptCompletion{
			AttemptID: notification.ID, NextAttemptID: retryID, StatusCode: 500,
			TransportError: "initial failure", ResponseBody: "failed",
			CompletedAt: at.Add(time.Second), NextAttemptAt: retryAt,
		}); err != nil || !completed {
			t.Fatalf("complete initial %s = %v, %v", eventID, completed, err)
		}
		return notification, retryID
	}

	// Hold the retry row exactly where ClaimDue would hold it while updating its
	// lease. Queue must wait, recheck its DELETE predicate, and preserve the row
	// once the concurrent claim makes it in-flight.
	notification, retryID := enqueueAndFail("evt-resend-claim-race", base, base.Add(time.Hour))
	lockTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := lockTx.QueryRow(ctx, `SELECT id FROM webhook_delivery_attempts WHERE id=$1 FOR UPDATE`, retryID).Scan(&retryID); err != nil {
		_ = lockTx.Rollback(ctx)
		t.Fatal(err)
	}
	queueErr := make(chan error, 1)
	go func() {
		_, err := s.QueueWebhookResend(ctx, WebhookResendRequest{
			TenantID: tenant.ID, EndpointID: endpoint.ID, SourceAttemptID: notification.ID,
			RequestedBy: "user-races", IdempotencyKey: "resend-claim-race", RequestedAt: base.Add(2 * time.Minute),
		})
		queueErr <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE datname=current_database() AND wait_event_type='Lock'
				  AND query LIKE '%DELETE FROM webhook_delivery_attempts%'
			)
		`).Scan(&waiting); err != nil {
			_ = lockTx.Rollback(ctx)
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			_ = lockTx.Rollback(ctx)
			t.Fatal("QueueWebhookResend did not reach the child-row lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	leaseUntil := base.Add(10 * time.Minute)
	if _, err := lockTx.Exec(ctx, `UPDATE webhook_delivery_attempts SET lease_until=$2 WHERE id=$1`, retryID, leaseUntil); err != nil {
		_ = lockTx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := lockTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-queueErr; !errors.Is(err, ErrWebhookAttemptPending) {
		t.Fatalf("resend racing claim = %v, want ErrWebhookAttemptPending", err)
	}
	var pending, leased bool
	if err := pool.QueryRow(ctx, `
		SELECT status='pending', lease_until=$2
		FROM webhook_delivery_attempts WHERE id=$1
	`, retryID, leaseUntil).Scan(&pending, &leased); err != nil || !pending || !leased {
		t.Fatalf("claimed evidence row survived = pending:%v leased:%v err:%v", pending, leased, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE webhook_delivery_attempts SET lease_until=NULL WHERE id=$1`, retryID); err != nil {
		t.Fatal(err)
	}

	// Complete and Resend share endpoint -> notification -> attempt lock order.
	// Depending on who wins, Resend either observes the in-flight lease or queues
	// immediately after completion; neither outcome can deadlock or lose evidence.
	claimed, err := s.ClaimDueWebhookAttempts(ctx, base.Add(time.Hour), base.Add(time.Hour+time.Minute), 10)
	if err != nil || len(claimed) != 1 || claimed[0].ID != retryID {
		t.Fatalf("claim completion-race retry = %+v, %v", claimed, err)
	}
	type completionResult struct {
		completed bool
		err       error
	}
	type resendResult struct {
		attempt WebhookAttempt
		err     error
	}
	start := make(chan struct{})
	completionCh := make(chan completionResult, 1)
	resendCh := make(chan resendResult, 1)
	go func() {
		<-start
		completed, err := s.CompleteWebhookAttempt(ctx, WebhookAttemptCompletion{
			AttemptID: retryID, StatusCode: 204, ResponseBody: "ok",
			CompletedAt: base.Add(time.Hour + time.Second), NextAttemptAt: base.Add(time.Hour + time.Second), Delivered: true,
		})
		completionCh <- completionResult{completed: completed, err: err}
	}()
	go func() {
		<-start
		attempt, err := s.QueueWebhookResend(ctx, WebhookResendRequest{
			TenantID: tenant.ID, EndpointID: endpoint.ID, SourceAttemptID: notification.ID,
			RequestedBy: "user-races", IdempotencyKey: "resend-complete-race", RequestedAt: base.Add(time.Hour + time.Second),
		})
		resendCh <- resendResult{attempt: attempt, err: err}
	}()
	close(start)
	completion := <-completionCh
	resend := <-resendCh
	if completion.err != nil || !completion.completed {
		t.Fatalf("completion race = %+v", completion)
	}
	if resend.err != nil && !errors.Is(resend.err, ErrWebhookAttemptPending) {
		t.Fatalf("resend completion race = %+v", resend)
	}
	if resend.err == nil {
		if resend.attempt.Status != WebhookAttemptPending || resend.attempt.Origin != WebhookAttemptManual {
			t.Fatalf("post-completion resend reservation = %+v", resend.attempt)
		}
		if completed, err := s.CompleteWebhookAttempt(ctx, WebhookAttemptCompletion{
			AttemptID: resend.attempt.ID, StatusCode: 204, ResponseBody: "ok",
			CompletedAt: base.Add(time.Hour + 2*time.Second), NextAttemptAt: base.Add(time.Hour + 2*time.Second), Delivered: true,
		}); err != nil || !completed {
			t.Fatalf("clean up completion-race resend = %v, %v", completed, err)
		}
	}
	var retryTerminal int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_delivery_attempts WHERE id=$1 AND status='delivered'`, retryID).Scan(&retryTerminal); err != nil || retryTerminal != 1 {
		t.Fatalf("completion evidence count = %d, %v", retryTerminal, err)
	}

	// Exhaustion and endpoint auto-disable are one transaction. A racing Resend
	// can see the claimed retry or the disabled endpoint, but can never receive a
	// reservation that will be parked forever after commit.
	exhaustedNotification, exhaustedRetryID := enqueueAndFail("evt-exhaustion-resend-race", base.Add(2*time.Hour), base.Add(3*time.Hour))
	claimed, err = s.ClaimDueWebhookAttempts(ctx, base.Add(3*time.Hour), base.Add(3*time.Hour+time.Minute), 10)
	if err != nil || len(claimed) != 1 || claimed[0].ID != exhaustedRetryID {
		t.Fatalf("claim exhaustion-race retry = %+v, %v", claimed, err)
	}
	start = make(chan struct{})
	completionCh = make(chan completionResult, 1)
	resendCh = make(chan resendResult, 1)
	go func() {
		<-start
		completed, err := s.CompleteWebhookAttempt(ctx, WebhookAttemptCompletion{
			AttemptID: exhaustedRetryID, StatusCode: 503,
			TransportError: "exhausted", ResponseBody: "failed",
			CompletedAt: base.Add(3*time.Hour + time.Second), NextAttemptAt: base.Add(3*time.Hour + time.Second),
			Exhausted: true, DisableReason: "disabled automatically after repeated delivery failures",
		})
		completionCh <- completionResult{completed: completed, err: err}
	}()
	go func() {
		<-start
		attempt, err := s.QueueWebhookResend(ctx, WebhookResendRequest{
			TenantID: tenant.ID, EndpointID: endpoint.ID, SourceAttemptID: exhaustedNotification.ID,
			RequestedBy: "user-races", IdempotencyKey: "resend-exhaustion-race", RequestedAt: base.Add(3*time.Hour + time.Second),
		})
		resendCh <- resendResult{attempt: attempt, err: err}
	}()
	close(start)
	completion = <-completionCh
	resend = <-resendCh
	if completion.err != nil || !completion.completed {
		t.Fatalf("exhaustion completion race = %+v", completion)
	}
	if !errors.Is(resend.err, ErrWebhookAttemptPending) && !errors.Is(resend.err, ErrWebhookEndpointDisabled) {
		t.Fatalf("exhaustion resend race = %+v, want pending or disabled", resend)
	}
	gotEndpoint, err := s.GetWebhookEndpoint(ctx, tenant.ID, endpoint.ID)
	if err != nil || gotEndpoint.Enabled || gotEndpoint.DisabledReason == "" {
		t.Fatalf("atomic exhaustion disable = %+v, %v", gotEndpoint, err)
	}
	var pendingManuals int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_delivery_attempts WHERE notification_id=$1 AND status='pending' AND origin='manual'`, exhaustedNotification.ID).Scan(&pendingManuals); err != nil || pendingManuals != 0 {
		t.Fatalf("exhaustion left pending manual attempts = %d, %v", pendingManuals, err)
	}
}

func TestWebhookAttemptRollingCompatibility(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `DELETE FROM tenants WHERE name='webhook-attempt-rolling'`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	tenant, err := s.CreateTenant(ctx, "webhook-attempt-rolling", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, tenant.ID) }()
	endpoint, err := s.CreateWebhookEndpoint(ctx, tenant.ID, "rolling", "https://hooks.example.test/rolling", "whsec_rolling", []string{"deploy_started"}, true, "user-rolling")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 17, 16, 0, 0, 0, time.UTC)
	notificationID := ids.New(ids.WebhookDelivery)

	// This is the exact legacy dispatcher write: only the mutable parent. The
	// bridge reserves the initial child in the same transaction.
	if _, err := pool.Exec(ctx, `
		INSERT INTO webhook_deliveries (
			id, endpoint_id, event_id, event_type, service_id, payload, next_attempt_at, created_at
		) VALUES ($1,$2,$3,'deploy_started','rolling-service','{}',$4,$4)
	`, notificationID, endpoint.ID, "evt-rolling-legacy", base); err != nil {
		t.Fatal(err)
	}
	var initialChildren int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_delivery_attempts WHERE notification_id=$1 AND id=$1 AND status='pending'`, notificationID).Scan(&initialChildren); err != nil || initialChildren != 1 {
		t.Fatalf("legacy insert bridge children = %d, %v", initialChildren, err)
	}

	// The old claim-only UPDATE is canceled, so a rolling old worker receives no
	// row and cannot double-send beside the child-ledger worker.
	var legacyClaims int
	if err := pool.QueryRow(ctx, `
		WITH due AS (
			SELECT id FROM webhook_deliveries WHERE id=$1 FOR UPDATE
		), claimed AS (
			UPDATE webhook_deliveries d SET next_attempt_at=$2
			FROM due WHERE d.id=due.id RETURNING d.id
		)
		SELECT count(*) FROM claimed
	`, notificationID, base.Add(2*time.Minute)).Scan(&legacyClaims); err != nil || legacyClaims != 0 {
		t.Fatalf("legacy claim blocker = %d, %v", legacyClaims, err)
	}

	// A replica that claimed just before the migration can still issue the old
	// aggregate completion. The bridge terminalizes its child evidence and makes
	// the next automatic reservation; a new worker then safely completes it.
	retryAt := base.Add(time.Hour)
	if _, err := pool.Exec(ctx, `
		UPDATE webhook_deliveries
		SET attempt_count=attempt_count+1, last_status=503,
			last_error='legacy failure', response_body='legacy body',
			sent_at=COALESCE(sent_at,$2), last_attempted_at=$2,
			next_attempt_at=$3
		WHERE id=$1
	`, notificationID, base.Add(time.Minute), retryAt); err != nil {
		t.Fatal(err)
	}
	var failedEvidence, retryReservations int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status='failed'), count(*) FILTER (WHERE status='pending')
		FROM webhook_delivery_attempts WHERE notification_id=$1
	`, notificationID).Scan(&failedEvidence, &retryReservations); err != nil || failedEvidence != 1 || retryReservations != 1 {
		t.Fatalf("legacy completion bridge = failed:%d pending:%d err:%v", failedEvidence, retryReservations, err)
	}
	claimed, err := s.ClaimDueWebhookAttempts(ctx, retryAt, retryAt.Add(time.Minute), 10)
	if err != nil || len(claimed) != 1 || claimed[0].NotificationID != notificationID {
		t.Fatalf("new worker claim after legacy completion = %+v, %v", claimed, err)
	}
	if completed, err := s.CompleteWebhookAttempt(ctx, WebhookAttemptCompletion{
		AttemptID: claimed[0].ID, StatusCode: 204, ResponseBody: "ok",
		CompletedAt: retryAt.Add(time.Second), NextAttemptAt: retryAt.Add(time.Second), Delivered: true,
	}); err != nil || !completed {
		t.Fatalf("new completion after legacy path = %v, %v", completed, err)
	}

	// Queue a manual replay of that terminal parent, then run the old retention
	// DELETE verbatim. The pending-child guard must park the legacy delete.
	manual, err := s.QueueWebhookResend(ctx, WebhookResendRequest{
		TenantID: tenant.ID, EndpointID: endpoint.ID, SourceAttemptID: claimed[0].ID,
		RequestedBy: "user-rolling", IdempotencyKey: "resend-rolling-sweep", RequestedAt: retryAt.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	legacySweep, err := pool.Exec(ctx, `
		WITH ranked AS (
			SELECT id, created_at,
				row_number() OVER (PARTITION BY endpoint_id ORDER BY created_at DESC, id DESC) AS rn
			FROM webhook_deliveries
			WHERE delivered_at IS NOT NULL OR failed_at IS NOT NULL
		), eligible AS (
			SELECT id FROM ranked WHERE created_at < $1 ORDER BY created_at LIMIT 100
		)
		DELETE FROM webhook_deliveries d USING eligible e WHERE d.id=e.id
	`, retryAt.Add(24*time.Hour))
	if err != nil || legacySweep.RowsAffected() != 0 {
		t.Fatalf("legacy sweep pending-child guard = %d, %v", legacySweep.RowsAffected(), err)
	}
	var manualStillPending bool
	if err := pool.QueryRow(ctx, `SELECT status='pending' FROM webhook_delivery_attempts WHERE id=$1`, manual.ID).Scan(&manualStillPending); err != nil || !manualStillPending {
		t.Fatalf("legacy sweep lost manual reservation = %v, %v", manualStillPending, err)
	}
	// The new abandoned-reservation sweep opts into deletion based on the child
	// age, retaining the pre-0084 bounded-cleanup behavior.
	if n, err := s.SweepWebhookDeliveries(ctx, retryAt.Add(24*time.Hour), 100, 100); err != nil || n != 1 {
		t.Fatalf("new abandoned pending sweep = %d, %v", n, err)
	}

	// Endpoint cascades are nested deletes and remain valid despite the direct
	// parent guard.
	pendingNotificationID := ids.New(ids.WebhookDelivery)
	if _, err := pool.Exec(ctx, `
		INSERT INTO webhook_deliveries (id,endpoint_id,event_id,event_type,payload,next_attempt_at)
		VALUES ($1,$2,'evt-rolling-cascade','deploy_started','{}',$3)
	`, pendingNotificationID, endpoint.ID, base); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWebhookEndpoint(ctx, tenant.ID, endpoint.ID); err != nil {
		t.Fatalf("endpoint pending-history cascade: %v", err)
	}
	var cascadeParents int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries WHERE id=$1`, pendingNotificationID).Scan(&cascadeParents); err != nil || cascadeParents != 0 {
		t.Fatalf("endpoint cascade left parent = %d, %v", cascadeParents, err)
	}

	// The legacy exhaustion aggregate update also disables its endpoint in the
	// same transaction as terminal evidence, closing the pre-0084 enabled gap.
	exhaustedEndpoint, err := s.CreateWebhookEndpoint(ctx, tenant.ID, "rolling-exhausted", "https://hooks.example.test/exhausted", "whsec_exhausted", []string{"deploy_started"}, true, "user-rolling")
	if err != nil {
		t.Fatal(err)
	}
	exhaustedID := ids.New(ids.WebhookDelivery)
	if _, err := pool.Exec(ctx, `
		INSERT INTO webhook_deliveries (id,endpoint_id,event_id,event_type,payload,next_attempt_at)
		VALUES ($1,$2,'evt-rolling-exhausted','deploy_started','{}',$3)
	`, exhaustedID, exhaustedEndpoint.ID, base); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE webhook_deliveries
		SET attempt_count=attempt_count+1, last_status=503,
			last_error='legacy exhaustion', sent_at=$2, last_attempted_at=$2,
			next_attempt_at=$2, failed_at=$2
		WHERE id=$1
	`, exhaustedID, base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	exhaustedEndpoint, err = s.GetWebhookEndpoint(ctx, tenant.ID, exhaustedEndpoint.ID)
	if err != nil || exhaustedEndpoint.Enabled || exhaustedEndpoint.DisabledReason == "" {
		t.Fatalf("legacy exhaustion atomic disable = %+v, %v", exhaustedEndpoint, err)
	}
	var exhaustedEvidence int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_delivery_attempts WHERE notification_id=$1 AND status='failed'`, exhaustedID).Scan(&exhaustedEvidence); err != nil || exhaustedEvidence != 1 {
		t.Fatalf("legacy exhaustion evidence = %d, %v", exhaustedEvidence, err)
	}
}
