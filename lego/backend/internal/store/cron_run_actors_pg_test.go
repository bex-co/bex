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
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/id"
)

// w4/631: a cron run's trigger and cancel are recorded under its crr- id, and
// its cron_job_run_ended event names the canceler of a canceled run, else the
// user who triggered it, through the feed and the by-id lookup alike.
func TestPGCronRunActorsNameTheEndedEventsUser(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	tenantID, appID := seedDiskApp(t, ctx, s)
	base := time.Date(2026, 11, 14, 8, 0, 0, 0, time.UTC)

	runs := []struct {
		name      string
		status    string
		trigger   string
		cancel    string
		wantEvent string
	}{
		{"web-manual-1", appv1alpha1.CronRunCanceled, "id-alice", "id-bob", "id-bob"},
		// A cancel that lost the race to the run's finish names no one.
		{"web-manual-2", appv1alpha1.CronRunSucceeded, "id-alice", "id-bob", "id-alice"},
		// A scheduled run a user canceled names the canceler.
		{"web-29000001", appv1alpha1.CronRunCanceled, "", "id-bob", "id-bob"},
		// A scheduled run nobody touched names no one.
		{"web-29000002", appv1alpha1.CronRunFailed, "", "", ""},
	}
	var facts []ServiceEventFact
	want := map[string]string{}
	for i, r := range runs {
		runID := id.Derive(id.CronRun, r.name)
		if r.trigger != "" {
			if err := s.RecordCronRunTrigger(ctx, appID, runID, r.trigger); err != nil {
				t.Fatal(err)
			}
		}
		if r.cancel != "" {
			// A first cancel whose patch failed is replaced by the one that took.
			if err := s.RecordCronRunCancel(ctx, appID, runID, "id-stale"); err != nil {
				t.Fatal(err)
			}
			if err := s.RecordCronRunCancel(ctx, appID, runID, r.cancel); err != nil {
				t.Fatal(err)
			}
		}
		facts = append(facts, recordedRun(appID, r.name, base.Add(time.Duration(i)*time.Minute), r.status, true)...)
		want[runID] = r.wantEvent
	}
	if err := s.InsertServiceEventFacts(ctx, facts); err != nil {
		t.Fatalf("insert facts: %v", err)
	}

	actors, err := s.CronRunActors(ctx, appID, []string{id.Derive(id.CronRun, "web-manual-1"), id.Derive(id.CronRun, "web-29000002"), "crr-unknown"})
	if err != nil {
		t.Fatal(err)
	}
	if got := actors[id.Derive(id.CronRun, "web-manual-1")]; got != (CronRunActor{TriggeredBy: "id-alice", CanceledBy: "id-bob"}) || len(actors) != 1 {
		t.Fatalf("actors = %+v, want only web-manual-1 triggered by alice and canceled by bob", actors)
	}

	ended, err := s.ListServiceEvents(ctx, appID, tenantID, ServiceEventFilter{FactTypes: []string{string(EventFactCronRunEnded)}})
	if err != nil || len(ended) != len(runs) {
		t.Fatalf("cron_job_run_ended feed = %+v (%v)", ended, err)
	}
	for _, row := range ended {
		if row.Caller != want[row.FactRunID] {
			t.Errorf("feed run %s names %q, want %q", row.FactRunID, row.Caller, want[row.FactRunID])
		}
		lookup, err := s.GetServiceEvent(ctx, tenantID, id.Derive(id.Event, row.Key))
		if err != nil || lookup.Event.Caller != want[row.FactRunID] {
			t.Errorf("by-id run %s = %q (%v), want %q", row.FactRunID, lookup.Event.Caller, err, want[row.FactRunID])
		}
	}
	// A start names no one: Render's cron_job_run_started has no user.
	started, err := s.ListServiceEvents(ctx, appID, tenantID, ServiceEventFilter{FactTypes: []string{string(EventFactCronRunStarted)}})
	if err != nil || len(started) != len(runs) {
		t.Fatalf("cron_job_run_started feed = %+v (%v)", started, err)
	}
	for _, row := range started {
		if row.Caller != "" {
			t.Errorf("start of %s names %q", row.FactRunID, row.Caller)
		}
	}

	// Deleting an account replaces its subject on the runs it acted on (ADR086).
	if err := s.CleanupAccountSubject(ctx, "id-bob", "deleted-bob"); err != nil {
		t.Fatal(err)
	}
	manual1 := id.Derive(id.CronRun, "web-manual-1")
	actors, err = s.CronRunActors(ctx, appID, []string{manual1})
	if err != nil || actors[manual1] != (CronRunActor{TriggeredBy: "id-alice", CanceledBy: "deleted-bob"}) {
		t.Fatalf("after account cleanup = %+v (%v), want bob replaced by the marker", actors, err)
	}
}
