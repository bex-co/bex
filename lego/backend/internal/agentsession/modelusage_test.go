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

type quotaLedger struct {
	reads int
	used  int64
}

func (q *quotaLedger) GetTenant(context.Context, string) (store.Tenant, error) {
	q.reads++
	return store.Tenant{Plan: store.PlanHobby}, nil
}

func (q *quotaLedger) AgentModelTokensMonthToDate(context.Context, string, time.Time) (int64, error) {
	q.reads++
	return q.used, nil
}

// The shipped sheet sets no quota: the check reads nothing and allows, so
// the platform route behaves exactly as before t005.
func TestModelQuotaIsUnlimitedByDefault(t *testing.T) {
	q := &quotaLedger{used: 1 << 40}
	m := &ModelMinter{Quota: q}
	if err := m.checkModelQuota(context.Background(), "tea-a"); err != nil || q.reads != 0 {
		t.Fatalf("default quota check = %v after %d reads, want allow with no reads", err, q.reads)
	}
	m.QuotaFor = func(string) (int64, bool) { return 100, true }
	q.used = 99
	if err := m.checkModelQuota(context.Background(), "tea-a"); err != nil {
		t.Fatalf("under quota: %v", err)
	}
	q.used = 100
	if err := m.checkModelQuota(context.Background(), "tea-a"); !errors.Is(err, ErrModelQuotaExceeded) || errors.Is(err, ErrForbidden) {
		t.Fatalf("at quota: %v, want ErrModelQuotaExceeded (not a retried ErrForbidden)", err)
	}
}

func TestModelFailureReasonVocabularyIsClosed(t *testing.T) {
	if reason, code, err := modelFailureReason(ModelFailureCauseQuotaExceeded); err != nil || reason != ModelQuotaExceededReason || code != ModelQuotaExceededCode {
		t.Fatalf("quota cause = %q %q %v", reason, code, err)
	}
	if reason, code, err := modelFailureReason(""); err != nil || reason != ModelAuthFailureReason || code != "" {
		t.Fatalf("auth cause = %q %q %v", reason, code, err)
	}
	if _, _, err := modelFailureReason("anything-else"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("unknown cause = %v", err)
	}
}
