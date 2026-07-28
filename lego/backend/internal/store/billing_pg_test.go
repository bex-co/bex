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

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// newBillingTestStore is the shared harness for the billing outbox/exclusion
// integration tests: skip unless BEX_TEST_DB_URI is set, migrate, and truncate
// so each test starts clean. Same gating as TestPGStore.
func newBillingTestStore(t *testing.T) (*PGStore, context.Context) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE audit_events`); err != nil {
		t.Fatal(err)
	}
	return NewPGStore(pool), ctx
}

func TestPGStoreStripeBillingLifecycleIsIdempotentOrderedAndReplicaSafe(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	tenant, err := s.CreateTenant(ctx, "dunning", "hobby")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	base := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	event := func(id, outcome string, at time.Time) StripeBillingEvent {
		return StripeBillingEvent{
			EventID: id, EventType: "invoice.payment_failed", WorkspaceID: tenant.ID,
			CustomerID: "cus_test_lifecycle", SubscriptionID: "sub_test_lifecycle", ObjectID: "in_test_lifecycle",
			ProviderCreatedAt: at, ReceivedAt: at.Add(time.Second), Outcome: outcome, Reason: "test",
		}
	}

	failed := event("evt_failure_1", BillingOutcomeFailure, base)
	state, inserted, changed, err := s.RecordStripeBillingEvent(ctx, failed, time.Hour)
	if err != nil || !inserted || !changed || state.Status != BillingGrace || state.GraceDeadline == nil || !state.GraceDeadline.Equal(base.Add(time.Hour)) {
		t.Fatalf("first failure = state %+v inserted=%v changed=%v err=%v", state, inserted, changed, err)
	}
	state, inserted, changed, err = s.RecordStripeBillingEvent(ctx, failed, time.Hour)
	if err != nil || inserted || changed || state.Status != BillingGrace {
		t.Fatalf("duplicate = state %+v inserted=%v changed=%v err=%v", state, inserted, changed, err)
	}
	stale := event("evt_stale_success", BillingOutcomeSuccess, base.Add(-time.Minute))
	state, inserted, changed, err = s.RecordStripeBillingEvent(ctx, stale, time.Hour)
	if err != nil || !inserted || changed || state.Status != BillingGrace {
		t.Fatalf("stale success = state %+v inserted=%v changed=%v err=%v", state, inserted, changed, err)
	}
	var retained int
	if err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM stripe_billing_events WHERE workspace_id=$1 AND outcome IN ('failure','success')`, tenant.ID).Scan(&retained); err != nil || retained != 2 {
		t.Fatalf("retained normalized events=%d err=%v", retained, err)
	}

	// One due row can be leased by only one replica.
	var wg sync.WaitGroup
	var mu sync.Mutex
	claims := 0
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, claimErr := s.ClaimDueBillingLifecycle(ctx, base.Add(2*time.Hour), time.Minute)
			if claimErr != nil {
				errs <- claimErr
				return
			}
			if ok {
				mu.Lock()
				claims++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	close(errs)
	for claimErr := range errs {
		t.Fatalf("claim: %v", claimErr)
	}
	if claims != 1 {
		t.Fatalf("replica claims = %d, want 1", claims)
	}
	claimedState, err := s.GetBillingLifecycle(ctx, tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompleteBillingLifecycleWork(ctx, tenant.ID, claimedState.TransitionVersion, BillingEnforced, base.Add(2*time.Hour)); err != nil {
		t.Fatalf("complete enforcement: %v", err)
	}
	if err := s.CheckBillingMutationAllowed(ctx, tenant.ID); !errors.Is(err, core.ErrBillingEnforced) {
		t.Fatalf("mutation gate = %v, want ErrBillingEnforced", err)
	}

	// Structural exclusion wins over later provider events and uses the same
	// recovery target instead of leaving billing-owned suspension behind.
	changedFlag, state, err := s.SetBillingException(ctx, tenant.ID, BillingExcluded, true, "ops", "test workspace", base.Add(3*time.Hour))
	if err != nil || !changedFlag || state.Status != BillingRecovering || state.RecoveryTarget != BillingExcluded {
		t.Fatalf("exclude enforced workspace = %+v changed=%v err=%v", state, changedFlag, err)
	}
	providerSuccess := event("evt_success_ignored", BillingOutcomeSuccess, base.Add(4*time.Hour))
	state, _, changed, err = s.RecordStripeBillingEvent(ctx, providerSuccess, time.Hour)
	if err != nil || changed || state.Status != BillingRecovering || state.RecoveryTarget != BillingExcluded {
		t.Fatalf("provider event during exclusion = %+v changed=%v err=%v", state, changed, err)
	}
	claimed, ok, err := s.ClaimDueBillingLifecycle(ctx, base.Add(4*time.Hour), time.Minute)
	if err != nil || !ok || claimed.RecoveryTarget != BillingExcluded {
		t.Fatalf("claim excluded recovery = %+v ok=%v err=%v", claimed, ok, err)
	}
	if _, err := s.CompleteBillingLifecycleWork(ctx, tenant.ID, claimed.TransitionVersion, claimed.RecoveryTarget, base.Add(4*time.Hour)); err != nil {
		t.Fatalf("complete exclusion recovery: %v", err)
	}
	deferredFailure := event("evt_failure_while_excluded", BillingOutcomeFailure, base.Add(4*time.Hour+30*time.Minute))
	if state, inserted, changed, err = s.RecordStripeBillingEvent(ctx, deferredFailure, time.Hour); err != nil || !inserted || changed || state.Status != BillingExcluded {
		t.Fatalf("failure while excluded = %+v inserted=%v changed=%v err=%v", state, inserted, changed, err)
	}
	if changedFlag, state, err = s.SetBillingException(ctx, tenant.ID, BillingExcluded, false, "ops", "test complete", base.Add(5*time.Hour)); err != nil || !changedFlag || state.Status != BillingHealthy {
		t.Fatalf("include = %+v changed=%v err=%v", state, changedFlag, err)
	}
	state, inserted, changed, err = s.RecordStripeBillingEvent(ctx, deferredFailure, time.Hour)
	if err != nil || inserted || !changed || state.Status != BillingGrace {
		t.Fatalf("deferred duplicate after include = %+v inserted=%v changed=%v err=%v", state, inserted, changed, err)
	}

	// A later failure can enforce again; a newer success enters recovery and
	// returns the workspace to healthy without touching any unrelated intent.
	failedAgain := event("evt_failure_2", BillingOutcomeFailure, base.Add(6*time.Hour))
	if _, _, _, err := s.RecordStripeBillingEvent(ctx, failedAgain, time.Hour); err != nil {
		t.Fatal(err)
	}
	claimed, ok, err = s.ClaimDueBillingLifecycle(ctx, base.Add(8*time.Hour), time.Minute)
	if err != nil || !ok {
		t.Fatalf("claim second enforcement ok=%v err=%v", ok, err)
	}
	if _, err := s.CompleteBillingLifecycleWork(ctx, tenant.ID, claimed.TransitionVersion, BillingEnforced, base.Add(8*time.Hour)); err != nil {
		t.Fatal(err)
	}
	recovered := event("evt_success_2", BillingOutcomeSuccess, base.Add(9*time.Hour))
	state, _, changed, err = s.RecordStripeBillingEvent(ctx, recovered, time.Hour)
	if err != nil || !changed || state.Status != BillingRecovering {
		t.Fatalf("payment recovery = %+v changed=%v err=%v", state, changed, err)
	}
	claimed, ok, err = s.ClaimDueBillingLifecycle(ctx, base.Add(9*time.Hour), time.Minute)
	if err != nil || !ok || claimed.Status != BillingRecovering {
		t.Fatalf("claim payment recovery = %+v ok=%v err=%v", claimed, ok, err)
	}
	state, err = s.CompleteBillingLifecycleWork(ctx, tenant.ID, claimed.TransitionVersion, BillingHealthy, base.Add(9*time.Hour))
	if err != nil || state.Status != BillingHealthy {
		t.Fatalf("healthy = %+v err=%v", state, err)
	}
}

func TestPGStoreBillingWorkerCompletionUsesTransitionCAS(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	tenant, err := s.CreateTenant(ctx, "billing-race", "hobby")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	event := func(id, outcome string, at time.Time) StripeBillingEvent {
		return StripeBillingEvent{
			EventID: id, EventType: "billing.test", WorkspaceID: tenant.ID,
			CustomerID: "cus_test_race", SubscriptionID: "sub_test_race", ObjectID: "in_test_race",
			ProviderCreatedAt: at, ReceivedAt: at, Outcome: outcome, Reason: "race_test",
		}
	}
	if _, _, _, err := s.RecordStripeBillingEvent(ctx, event("evt_race_fail", BillingOutcomeFailure, base), time.Hour); err != nil {
		t.Fatal(err)
	}
	enforcementClaim, ok, err := s.ClaimDueBillingLifecycle(ctx, base.Add(2*time.Hour), time.Minute)
	if err != nil || !ok || enforcementClaim.Status != BillingEnforcing {
		t.Fatalf("enforcement claim = %+v ok=%v err=%v", enforcementClaim, ok, err)
	}
	if state, _, _, err := s.RecordStripeBillingEvent(ctx, event("evt_race_paid", BillingOutcomeSuccess, base.Add(3*time.Hour)), time.Hour); err != nil || state.Status != BillingRecovering {
		t.Fatalf("success during enforcement = %+v err=%v", state, err)
	}
	state, err := s.CompleteBillingLifecycleWork(ctx, tenant.ID, enforcementClaim.TransitionVersion, BillingEnforced, base.Add(3*time.Hour))
	if err != nil || state.Status != BillingRecovering {
		t.Fatalf("stale enforcement completion overwrote recovery: %+v err=%v", state, err)
	}

	recoveryClaim, ok, err := s.ClaimDueBillingLifecycle(ctx, base.Add(3*time.Hour), time.Minute)
	if err != nil || !ok || recoveryClaim.Status != BillingRecovering {
		t.Fatalf("recovery claim = %+v ok=%v err=%v", recoveryClaim, ok, err)
	}
	if state, _, _, err = s.RecordStripeBillingEvent(ctx, event("evt_race_failed_again", BillingOutcomeFailure, base.Add(4*time.Hour)), time.Hour); err != nil || state.Status != BillingEnforcing {
		t.Fatalf("failure during recovery = %+v err=%v", state, err)
	}
	state, err = s.CompleteBillingLifecycleWork(ctx, tenant.ID, recoveryClaim.TransitionVersion, BillingHealthy, base.Add(4*time.Hour))
	if err != nil || state.Status != BillingEnforcing {
		t.Fatalf("stale recovery completion overwrote re-enforcement: %+v err=%v", state, err)
	}
}

func TestPGStoreBillingOutbox(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	billable, err := s.CreateTenant(ctx, "billable", "hobby")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	excluded, err := s.CreateTenant(ctx, "excluded", "hobby")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	if _, err := s.SetTenantBillingExcluded(ctx, excluded.ID, true, "ops", time.Now().UTC()); err != nil {
		t.Fatalf("exclude tenant: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Hour)
	sealed := now.Add(-72 * time.Hour) // older than the 48h seal horizon
	recent := now.Add(-1 * time.Hour)  // still inside the rewrite horizon
	belowFloor := now.Add(-40 * 24 * time.Hour)

	put := func(ws, svc, kind, tier string, window time.Time, qty int64) {
		t.Helper()
		if err := s.UpsertUsageHourly(ctx, HourlyRow{
			WorkspaceID: ws, ServiceID: svc, Kind: kind, Tier: tier,
			ResourceKind: ResourceKindService, WindowStart: window, Quantity: qty,
		}); err != nil {
			t.Fatalf("upsert usage: %v", err)
		}
	}
	put(billable.ID, "srv-1", UsageKindInstanceSeconds, "starter", sealed, 3600)  // qualifies
	put(billable.ID, "srv-1", UsageKindInstanceSeconds, "starter", recent, 100)   // not sealed
	put(billable.ID, "srv-1", UsageKindInstanceSeconds, "starter", belowFloor, 5) // below floor
	put(excluded.ID, "srv-2", UsageKindInstanceSeconds, "starter", sealed, 999)   // excluded tenant

	floor := now.Add(-34 * 24 * time.Hour) // billing.BackfillHorizon; below `sealed`, above `belowFloor`
	sealBefore := now.Add(-48 * time.Hour)

	rows, err := s.SelectUnemittedUsage(ctx, floor, sealBefore, 100)
	if err != nil {
		t.Fatalf("select unemitted: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("selected %d rows, want 1 (only the sealed, above-floor, non-excluded one): %+v", len(rows), rows)
	}
	if rows[0].WorkspaceID != billable.ID || rows[0].Quantity != 3600 {
		t.Fatalf("selected wrong row: %+v", rows[0])
	}

	// Stamp, then re-select: exactly-once — the emitted row is gone.
	if err := s.MarkUsageEmitted(ctx, rows, now); err != nil {
		t.Fatalf("mark emitted: %v", err)
	}
	again, err := s.SelectUnemittedUsage(ctx, floor, sealBefore, 100)
	if err != nil {
		t.Fatalf("re-select: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("re-selected %d rows after stamping, want 0 (exactly-once)", len(again))
	}
	// Re-stamping the same rows is a harmless no-op (crash-recovery safety).
	if err := s.MarkUsageEmitted(ctx, rows, now); err != nil {
		t.Fatalf("re-mark emitted (idempotent): %v", err)
	}
}

func TestPGStoreBillingExclusionAudited(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	ten, err := s.CreateTenant(ctx, "acme", "hobby")
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	at := time.Now().UTC()

	changed, err := s.SetTenantBillingExcluded(ctx, ten.ID, true, "admin@bex.co", at)
	if err != nil {
		t.Fatalf("exclude: %v", err)
	}
	if !changed {
		t.Fatal("first exclude reported changed=false, want true")
	}

	// The flag persisted, and an audit row records it with the value it was set to.
	var excluded bool
	if err := s.Pool.QueryRow(ctx, `SELECT billing_excluded FROM tenants WHERE id=$1`, ten.ID).Scan(&excluded); err != nil {
		t.Fatalf("read flag: %v", err)
	}
	if !excluded {
		t.Fatal("billing_excluded not persisted")
	}
	var verb string
	var to *bool
	if err := s.Pool.QueryRow(ctx,
		`SELECT verb, billing_excluded_to FROM audit_events WHERE workspace_id=$1 ORDER BY at DESC LIMIT 1`,
		ten.ID).Scan(&verb, &to); err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if verb != "billing.SetExclusion" {
		t.Fatalf("audit verb = %q, want billing.SetExclusion", verb)
	}
	if to == nil || *to != true {
		t.Fatalf("audit billing_excluded_to = %v, want true", to)
	}

	// A no-op toggle changes nothing and writes no second audit row.
	changed, err = s.SetTenantBillingExcluded(ctx, ten.ID, true, "admin@bex.co", at)
	if err != nil {
		t.Fatalf("re-exclude: %v", err)
	}
	if changed {
		t.Fatal("no-op toggle reported changed=true, want false")
	}
	var n int
	if err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_events WHERE workspace_id=$1`, ten.ID).Scan(&n); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if n != 1 {
		t.Fatalf("audit rows = %d, want 1 (no-op wrote none)", n)
	}

	// Unknown workspace → ErrNotFound.
	if _, err := s.SetTenantBillingExcluded(ctx, "tea-nope", true, "admin", at); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown tenant err = %v, want ErrNotFound", err)
	}
}

func TestPGStoreBillingExportRejectAmbiguityAndAuditedRepair(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	tenant, err := s.CreateTenant(ctx, "billing-ops", "hobby")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Hour)
	window := now.Add(-72 * time.Hour)
	ambiguousRow := HourlyRow{WorkspaceID: tenant.ID, ServiceID: "srv-ambiguity", Kind: UsageKindBuildSeconds, ResourceKind: ResourceKindService, WindowStart: window, Quantity: 10}
	rejectedRow := HourlyRow{WorkspaceID: tenant.ID, ServiceID: "srv-reject", Kind: UsageKindEgressBytes, ResourceKind: ResourceKindService, WindowStart: window.Add(time.Hour), Quantity: 1024}
	for _, row := range []HourlyRow{ambiguousRow, rejectedRow} {
		if err := s.UpsertUsageHourly(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	ambiguous := UsageExportAttempt{Row: ambiguousRow, TransactionID: "tx-ambiguity", EventName: "build_seconds"}
	if err := s.MarkUsageAttempted(ctx, []UsageExportAttempt{ambiguous}, now.Add(-25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	count, err := s.QuarantineOldUsageAttempts(ctx, now.Add(-24*time.Hour), now)
	if err != nil || count != 1 {
		t.Fatalf("quarantine count=%d err=%v", count, err)
	}
	stats, err := s.BillingExportStats(ctx, now.Add(-34*24*time.Hour), now.Add(-48*time.Hour), now)
	if err != nil || stats.AmbiguousRows != 1 {
		t.Fatalf("stats=%+v err=%v", stats, err)
	}
	issues, err := s.ListBillingExportIssues(ctx, true, 100)
	if err != nil || len(issues) != 1 || issues[0].IssueKind != "stamp_ambiguity" {
		t.Fatalf("issues=%+v err=%v", issues, err)
	}
	if _, err := s.ResolveBillingExportIssue(ctx, ambiguous.TransactionID, "retry", "ops", "unsafe retry test", now); !errors.Is(err, ErrConflict) {
		t.Fatalf("ambiguous retry err=%v, want conflict", err)
	}
	if _, err := s.ResolveBillingExportIssue(ctx, ambiguous.TransactionID, "mark_repaired", "ops", "summary proved provider receipt", now); err != nil {
		t.Fatal(err)
	}

	rejected := UsageExportAttempt{Row: rejectedRow, TransactionID: "tx-reject", EventName: "egress_gib"}
	if err := s.MarkUsageAttempted(ctx, []UsageExportAttempt{rejected}, now); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordUsageExportResult(ctx, nil, []UsageExportReject{{Attempt: rejected, Code: "parameter_invalid", Message: "bounded rejection"}}, now); err != nil {
		t.Fatal(err)
	}
	report, err := s.BillingExportReport(ctx, tenant.ID, window, now)
	if err != nil || len(report.Rows) != 2 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	if _, err := s.ResolveBillingExportIssue(ctx, rejected.TransactionID, "retry", "ops", "catalog fixed", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	report, err = s.BillingExportReport(ctx, tenant.ID, window, now)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, row := range report.Rows {
		states[row.ServiceID] = row.State
	}
	if states["srv-ambiguity"] != "emitted" || states["srv-reject"] != "pending" {
		t.Fatalf("states=%v", states)
	}
	// A sibling replica can observe a duplicate response after this replica
	// stamps the deterministic event accepted. The stale reject must not reopen
	// an issue or overwrite the final emitted state.
	if err := s.RecordUsageExportResult(ctx, []UsageExportAttempt{rejected}, nil, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordUsageExportResult(ctx, nil, []UsageExportReject{{Attempt: rejected, Code: "duplicate_meter_event", Message: "already submitted"}}, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	report, err = s.BillingExportReport(ctx, tenant.ID, window, now)
	if err != nil {
		t.Fatal(err)
	}
	states = map[string]string{}
	for _, row := range report.Rows {
		states[row.ServiceID] = row.State
	}
	if states["srv-reject"] != "emitted" {
		t.Fatalf("state after stale reject=%q, want emitted", states["srv-reject"])
	}
	openIssues, err := s.ListBillingExportIssues(ctx, true, 100)
	if err != nil || len(openIssues) != 0 {
		t.Fatalf("open issues after stale reject=%+v err=%v", openIssues, err)
	}
	var auditCount int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE workspace_id=$1 AND verb='billing.ResolveExportIssue'`, tenant.ID).Scan(&auditCount); err != nil || auditCount != 2 {
		t.Fatalf("audit count=%d err=%v", auditCount, err)
	}
	stats, err = s.BillingExportStats(ctx, now.Add(-34*24*time.Hour), now.Add(-48*time.Hour), now.Add(4*time.Minute))
	if err != nil || stats.RejectedRows != 0 || stats.AmbiguousRows != 0 {
		t.Fatalf("resolved issue stats=%+v err=%v", stats, err)
	}
}
