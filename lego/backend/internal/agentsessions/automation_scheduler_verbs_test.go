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

package agentsessions

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// claimingAutomations is the scheduler's side of the automation table: it hands
// the scheduler the runs it "claimed" and keeps what it settled.
type claimingAutomations struct {
	*fakeAutomations
	claims []store.ClaimedRun

	mu      sync.Mutex
	settled map[string]string // automation id -> outcome
}

func (f *claimingAutomations) ClaimDueAutomations(context.Context, time.Time, int, time.Duration,
	func(store.AgentAutomation) (time.Time, *time.Time, int, error)) (store.ClaimResult, error) {
	return store.ClaimResult{Runs: f.claims}, nil
}

func (f *claimingAutomations) ClaimStaleRuns(context.Context, time.Time, time.Duration, int) ([]store.ClaimedRun, error) {
	return nil, nil
}

func (f *claimingAutomations) LatestStartedSessionPhase(context.Context, string) (string, string, bool, error) {
	return "", "", false, nil
}

func (f *claimingAutomations) SettleRun(_ context.Context, automationID string, _ time.Time, outcome, _, _ string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.settled[automationID] = outcome
	return 0, nil
}

// w21/093: the scheduler fires each run from a goroutine of its own, so the
// creator's re-authorization recorded the goroutine's closure
// ("agentsessions.RunOnce.func1", 60 production rows). No HTTP route reaches it:
// this drives the scheduler itself and reads the rows it wrote.
func TestSchedulerFireRecordsItsOwnAuditVerb(t *testing.T) {
	svc, _, _, _ := fixture()
	audit := &auditCapture{}
	svc.Audit = audit
	now := time.Date(2026, time.November, 16, 9, 0, 0, 0, time.UTC)
	template, _ := json.Marshal(AutomationTemplate{Task: "update the dependencies", Repo: "bex-co/example",
		AgentConfig: AutomationAgentConfig{Agent: "codex", Model: "gpt-5"}})
	table := &claimingAutomations{fakeAutomations: newFakeAutomations(), settled: map[string]string{}}
	// alice may create sessions in tea-a, bob is a member of tea-b only.
	for _, creator := range []string{"alice", "bob"} {
		id := "aut-" + creator
		table.claims = append(table.claims, store.ClaimedRun{
			Run:        store.AgentAutomationRun{AutomationID: id, FireAt: now},
			Automation: store.AgentAutomation{ID: id, WorkspaceID: "tea-a", CreatedBy: creator, Template: template},
		})
	}
	svc.Automations = table

	if err := (&AutomationScheduler{Service: svc, Now: func() time.Time { return now }}).RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := table.settled["aut-bob"]; got != store.AutomationRunDenied {
		t.Fatalf("a creator outside the workspace settled %q, want %q", got, store.AutomationRunDenied)
	}
	fires := map[string]core.AuditOutcome{}
	audit.mu.Lock()
	defer audit.mu.Unlock()
	for _, ev := range audit.events {
		if strings.Contains(ev.Verb, "func") || strings.Contains(ev.Verb, "RunOnce") {
			t.Errorf("the scheduler recorded %q, a name that tells a reader of the audit log nothing", ev.Verb)
		}
		if ev.Verb == AuditVerbFireAgentAutomation {
			fires[ev.Caller] = ev.Outcome
			if ev.Relation != core.RelCanCreate || ev.Resource != core.WorkspaceObject("tea-a") {
				t.Errorf("fire row = %+v, want can_create on workspace tea-a", ev)
			}
		}
	}
	if len(fires) != 2 || fires["alice"] != core.AuditAllowed || fires["bob"] != core.AuditDenied {
		t.Fatalf("fire rows by creator = %v, want alice allowed and bob denied under %s", fires, AuditVerbFireAgentAutomation)
	}
}

func (f *claimingAutomations) InsertManualRun(_ context.Context, automationID string, fireAt, _ time.Time, _ time.Duration) (store.AgentAutomationRun, bool, error) {
	return store.AgentAutomationRun{AutomationID: automationID, FireAt: fireAt}, true, nil
}

func (f *claimingAutomations) ListRuns(context.Context, string, time.Time, int) ([]store.AgentAutomationRun, error) {
	return nil, nil
}

// A run-now keeps the verb of the call that asked for it: only the scheduler's
// goroutine needed a name, and a run-now's rows are searched for under its own.
func TestRunNowRecordsItsOwnAuditVerb(t *testing.T) {
	svc, _, _, _ := fixture()
	audit := &auditCapture{}
	svc.Audit = audit
	template, _ := json.Marshal(AutomationTemplate{Task: "update", Repo: "bex-co/example",
		AgentConfig: AutomationAgentConfig{Agent: "codex", Model: "gpt-5"}})
	table := &claimingAutomations{fakeAutomations: newFakeAutomations(), settled: map[string]string{}}
	svc.Automations = table
	row, err := table.CreateAgentAutomation(context.Background(), store.AgentAutomation{
		ID: ids.New(ids.AgentAutomation), WorkspaceID: "tea-a", Name: "nightly", CreatedBy: "alice", Template: template})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.RunAgentAutomationNow(caller("alice"), "tea-a", row.ID); err != nil {
		t.Fatal(err)
	}

	audit.mu.Lock()
	defer audit.mu.Unlock()
	runNow := 0
	for _, ev := range audit.events {
		switch {
		case ev.Verb == AuditVerbFireAgentAutomation:
			t.Errorf("a run-now recorded the scheduler's verb: %+v", ev)
		case ev.Verb == AuditVerbRunAgentAutomationNow:
			runNow++
		case ev.Verb != "agentsessions.Create":
			t.Errorf("a run-now recorded %q beside its own verb and the session create's", ev.Verb)
		}
	}
	// The ask (scopeAutomationManaged) and the creator's re-authorization before the fire.
	if runNow != 2 {
		t.Fatalf("a run-now recorded %d rows under %s, want 2: %+v", runNow, AuditVerbRunAgentAutomationNow, audit.events)
	}
}

// The two verbs are spelled as core.callerVerb derives them, which a run-now
// recorded before they were named: a rename of the method would change them.
func TestAutomationAuditVerbsAreSpelledAsDerived(t *testing.T) {
	if AuditVerbRunAgentAutomationNow != "agentsessions.RunAgentAutomationNow" {
		t.Fatalf("run-now verb = %q", AuditVerbRunAgentAutomationNow)
	}
	if !strings.HasPrefix(AuditVerbFireAgentAutomation, "agentsessions.") {
		t.Fatalf("fire verb = %q", AuditVerbFireAgentAutomation)
	}
}
