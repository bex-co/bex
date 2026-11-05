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

package agentsession

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/pricing"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type usageLedger struct {
	session  store.AgentSession
	recorded map[string]store.AgentModelUsage
}

func (l *usageLedger) GetAgentSession(_ context.Context, sid string) (store.AgentSession, error) {
	if sid != l.session.ID {
		return store.AgentSession{}, store.ErrNotFound
	}
	return l.session, nil
}

func (l *usageLedger) RecordAgentModelUsage(_ context.Context, u store.AgentModelUsage) (bool, error) {
	if _, dup := l.recorded[u.RequestID]; dup {
		return false, nil
	}
	l.recorded[u.RequestID] = u
	return true, nil
}

func TestModelUsageRecorderIsIdempotentAndBound(t *testing.T) {
	ledger := &usageLedger{session: store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a"}, recorded: map[string]store.AgentModelUsage{}}
	at := time.Date(2026, 10, 27, 9, 30, 0, 0, time.UTC)
	rec := &ModelUsageRecorder{Store: ledger, Now: func() time.Time { return at }}
	good := ModelUsageReport{RequestID: id.New(id.AgentModelRequest), SessionID: "ags-one", Workspace: "tea-a", Model: DefaultDevModel, InputTokens: 5, OutputTokens: 2}

	first, err := rec.Record(context.Background(), good)
	if err != nil || !first.Acknowledged || first.Duplicate {
		t.Fatalf("first report = %+v, %v", first, err)
	}
	again, err := rec.Record(context.Background(), good)
	if err != nil || !again.Acknowledged || !again.Duplicate || len(ledger.recorded) != 1 {
		t.Fatalf("retried report = %+v, %v (recorded %d)", again, err, len(ledger.recorded))
	}
	if got := ledger.recorded[good.RequestID]; !got.At.Equal(at) || got.InputTokens != 5 {
		t.Fatalf("recorded = %+v", got)
	}

	for name, tc := range map[string]struct {
		mutate func(*ModelUsageReport)
		want   error
	}{
		"other workspace": {func(r *ModelUsageReport) { r.Workspace = "tea-b" }, ErrForbidden},
		"unknown session": {func(r *ModelUsageReport) { r.SessionID = "ags-two" }, ErrForbidden},
		"not an amr id":   {func(r *ModelUsageReport) { r.RequestID = id.New(id.AgentSession) }, ErrInvalidRequest},
		"bad model":       {func(r *ModelUsageReport) { r.Model = "Not A Model" }, ErrInvalidRequest},
		"negative tokens": {func(r *ModelUsageReport) { r.OutputTokens = -1 }, ErrInvalidRequest},
	} {
		rep := good
		rep.RequestID = id.New(id.AgentModelRequest)
		tc.mutate(&rep)
		if _, err := rec.Record(context.Background(), rep); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
}

type spendLedger struct {
	plan     string
	trust    store.BillingTrust
	override *int64
	used     int64
	reads    int
}

func (q *spendLedger) AgentSpendInputs(_ context.Context, ws string, _ time.Time) (store.AgentSpendInputs, error) {
	q.reads++
	return store.AgentSpendInputs{Plan: q.plan, Trust: q.trust, OverrideMicroUSD: q.override, MonthToDateMicroUSD: q.used}, nil
}

// ADR091 D5/D10.1: the effective cap is min(override clamped to the plan's
// bound, or the plan default; the tier ceiling); at or over it the mint
// refuses with a 429-mapped error that is not the retried ErrForbidden.
func TestSpendCapEffectiveCap(t *testing.T) {
	now := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	t1 := store.BillingTrust{LifetimePaidCents: 1000}
	usd := func(v float64) *int64 { m := pricing.USDToMicro(v); return &m }
	for name, tc := range map[string]struct {
		ledger spendLedger
		want   float64
		reason string
	}{
		"T0 on Pro is capped at $10":      {spendLedger{plan: store.PlanPro}, 10, modelSpendNewWorkspaceReason},
		"T0 override above is still $10":  {spendLedger{plan: store.PlanPro, override: usd(500)}, 10, modelSpendNewWorkspaceReason},
		"T0 override below the ceiling":   {spendLedger{plan: store.PlanPro, override: usd(5)}, 5, ModelSpendCapReachedReason},
		"T1 Hobby gets the plan cap":      {spendLedger{plan: store.PlanHobby, trust: t1}, 20, ModelSpendCapReachedReason},
		"T1 Pro override raises it":       {spendLedger{plan: store.PlanPro, trust: t1, override: usd(1500)}, 1500, ModelSpendCapReachedReason},
		"downgraded override is clamped":  {spendLedger{plan: store.PlanHobby, trust: t1, override: usd(1500)}, 200, ModelSpendCapReachedReason},
		"T1 enterprise takes the top cap": {spendLedger{plan: store.PlanEnterprise, trust: t1}, 2000, ModelSpendCapReachedReason},
		"locked T4 is back to T0's $10":   {spendLedger{plan: store.PlanScale, trust: store.BillingTrust{LifetimePaidCents: 1 << 30, PaidCycleInvoices: 10, LockedUntil: store.LockedIndefinitely}}, 10, modelSpendLockedReason},
		"a dispute holds T4 at T0":        {spendLedger{plan: store.PlanScale, trust: store.BillingTrust{LifetimePaidCents: 1 << 30, PaidCycleInvoices: 10, DisputeID: "dp_1"}}, 10, modelSpendLockedReason},
	} {
		status, err := ReadSpendStatus(context.Background(), &tc.ledger, nil, "tea-a", now)
		if err != nil || status.EffectiveCap != pricing.USDToMicro(tc.want) || status.SpendCapReachedReason() != tc.reason {
			t.Errorf("%s: cap = %d (%v) reason %q, want $%v %q", name, status.EffectiveCap, err, status.SpendCapReachedReason(), tc.want, tc.reason)
		}
	}

	clock := now
	q := &spendLedger{plan: store.PlanHobby, trust: t1, used: 19_999_999}
	m := &ModelMinter{Spend: q, Now: func() time.Time { return clock }}
	if err := m.checkSpendCap(context.Background(), "tea-a"); err != nil {
		t.Fatalf("under the cap: %v", err)
	}
	// The mint's view is cached per workspace for spendStatusTTL.
	q.used = 20_000_000
	clock = now.Add(spendStatusTTL - time.Second)
	if err := m.checkSpendCap(context.Background(), "tea-a"); err != nil || q.reads != 1 {
		t.Fatalf("within the cache TTL: %v after %d reads, want the cached allow", err, q.reads)
	}
	clock = now.Add(spendStatusTTL)
	if err := m.checkSpendCap(context.Background(), "tea-a"); !errors.Is(err, ErrModelSpendCapReached) || errors.Is(err, ErrForbidden) || q.reads != 2 {
		t.Fatalf("at the cap: %v, want ErrModelSpendCapReached (not a retried ErrForbidden)", err)
	}
	if err := (&ModelMinter{}).checkSpendCap(context.Background(), "tea-a"); err != nil {
		t.Fatalf("unwired spend store must check nothing: %v", err)
	}
}

func TestModelFailureReasonVocabularyIsClosed(t *testing.T) {
	if reason, code, err := modelFailureReason(ModelFailureCauseSpendCapReached); err != nil || reason != ModelSpendCapReachedReason || code != ModelSpendCapReachedCode {
		t.Fatalf("spend-cap cause = %q %q %v", reason, code, err)
	}
	if reason, code, err := modelFailureReason(""); err != nil || reason != ModelAuthFailureReason || code != "" {
		t.Fatalf("auth cause = %q %q %v", reason, code, err)
	}
	if _, _, err := modelFailureReason("anything-else"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("unknown cause = %v", err)
	}
}
