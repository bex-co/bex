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
	"fmt"
	"slices"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/id"
)

// w3/m366: a cron job's history past App.status.runs' ten entries comes back
// from its facts — every finished run once, newest first, across pages, by id.
func TestPGCronRunHistoryPagesEveryFinishedRun(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	_, appID := seedDiskApp(t, ctx, s)
	base := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)

	// Fourteen finished runs a minute apart, alternating outcomes, newest last.
	var facts []ServiceEventFact
	var newestFirst []string
	for i := range 14 {
		name := fmt.Sprintf("web-%d", 29000000+i)
		runID := id.Derive(id.CronRun, name)
		status := appv1alpha1.CronRunSucceeded
		if i%2 == 1 {
			status = appv1alpha1.CronRunFailed
		}
		facts = append(facts, recordedRun(appID, name, base.Add(time.Duration(i)*time.Minute), status, true)...)
		newestFirst = append([]string{runID}, newestFirst...)
	}
	// A manual run canceled before it started has only its ended fact.
	canceled := "web-manual-1762934700"
	facts = append(facts, recordedRun(appID, canceled, base.Add(30*time.Minute), appv1alpha1.CronRunCanceled, false)...)
	newestFirst = append([]string{id.Derive(id.CronRun, canceled)}, newestFirst...)
	// A run started but never seen ending has no outcome to show.
	orphan := "web-29999999"
	facts = append(facts, ServiceEventFact{
		SourceKey: "cron:" + appID + ":" + orphan + ":started", AppID: appID, Type: EventFactCronRunStarted,
		At: base.Add(40 * time.Minute), RunID: id.Derive(id.CronRun, orphan),
	})
	if err := s.InsertServiceEventFacts(ctx, facts); err != nil {
		t.Fatalf("insert facts: %v", err)
	}

	// The two newest runs are still in status: the caller excludes them.
	exclude := newestFirst[:2]
	want := newestFirst[2:]
	var got []string
	after := ""
	for page := 0; page < 10; page++ {
		recs, err := s.ListCronRunRecords(ctx, appID, after, exclude, 5)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if len(recs) == 0 {
			break
		}
		for _, r := range recs {
			got = append(got, r.ID)
		}
		after = recs[len(recs)-1].ID
	}
	if !slices.Equal(got, want) {
		t.Fatalf("paged ids =\n%v\nwant\n%v", got, want)
	}
	if recs, err := s.ListCronRunRecords(ctx, appID, "crr-0000000000000000000x", nil, 5); err != nil || len(recs) != 0 {
		t.Fatalf("unknown cursor = (%v, %v), want an empty tail", recs, err)
	}
	if recs, err := s.ListCronRunRecords(ctx, "srv-someoneelse", "", nil, 5); err != nil || len(recs) != 0 {
		t.Fatalf("another app's history = (%v, %v), want none", recs, err)
	}

	// By id: a started run carries both times; the canceled-first run only its end.
	oldest, ok, err := s.CronRunRecord(ctx, appID, id.Derive(id.CronRun, "web-29000000"))
	if err != nil || !ok {
		t.Fatalf("oldest run = (%v, %v)", ok, err)
	}
	if oldest.Name != "web-29000000" || !oldest.StartedAt.Equal(base) || !oldest.FinishedAt.Equal(base.Add(30*time.Second)) || oldest.Status != EventStatusSucceeded {
		t.Errorf("oldest run = %+v", oldest)
	}
	c, ok, err := s.CronRunRecord(ctx, appID, id.Derive(id.CronRun, canceled))
	if err != nil || !ok || c.Status != EventStatusCanceled || !c.StartedAt.IsZero() || !c.FinishedAt.Equal(base.Add(30*time.Minute)) || c.Name != canceled {
		t.Errorf("canceled-first run = %+v (%v, %v), want canceled with no start", c, ok, err)
	}
	if _, ok, err := s.CronRunRecord(ctx, appID, id.Derive(id.CronRun, orphan)); err != nil || ok {
		t.Errorf("a run with no recorded outcome = (%v, %v), want not found", ok, err)
	}
}

// Facts from before migration 0141 carry no run_id; 0245 derives it in SQL. It
// must be exactly id.Derive(id.CronRun, <job name>), or every old run's id
// changes under the clients that paged with it.
func TestPGCronRunIDBackfillMatchesGoDerivation(t *testing.T) {
	s, ctx := newBillingTestStore(t)
	_, appID := seedDiskApp(t, ctx, s)
	names := []string{"web-29012345", "a", "web-manual-1762934700", "backup-nightly-cron-job-with-a-long-name-28999999"}
	at := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i, name := range names {
		for _, f := range recordedRun(appID, name, at.Add(time.Duration(i)*time.Hour), appv1alpha1.CronRunSucceeded, true) {
			f.RunID = "" // as recorded before 0141
			if _, err := s.InsertServiceEventFact(ctx, f); err != nil {
				t.Fatalf("insert %s: %v", f.SourceKey, err)
			}
		}
	}
	up, err := migrationsFS.ReadFile("migrations/0245_cron_run_history.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, string(up)); err != nil {
		t.Fatalf("re-run 0245: %v", err)
	}
	for _, name := range names {
		want := id.Derive(id.CronRun, name)
		for _, phase := range []string{"started", "ended"} {
			var got string
			key := "cron:" + appID + ":" + name + ":" + phase
			if err := s.Pool.QueryRow(ctx, `SELECT run_id FROM service_event_facts WHERE source_key = $1`, key).Scan(&got); err != nil {
				t.Fatalf("%s: %v", key, err)
			}
			if got != want {
				t.Errorf("%s run_id = %q, want id.Derive's %q", key, got, want)
			}
		}
	}
	// A recorded id is never rewritten.
	kept := ServiceEventFact{SourceKey: "cron:" + appID + ":kept:ended", AppID: appID, Type: EventFactCronRunEnded, At: at, Status: EventStatusFailed, RunID: "crr-keptkeptkeptkeptkept0"}
	if _, err := s.InsertServiceEventFact(ctx, kept); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := s.Pool.QueryRow(ctx, `SELECT run_id FROM service_event_facts WHERE source_key = $1`, kept.SourceKey).Scan(&got); err != nil || got != kept.RunID {
		t.Errorf("recorded run_id became %q (%v)", got, err)
	}
}

// recordedRun is a finished run's facts exactly as the reconciler records them
// from App.status.runs (cronRunFacts): a start (when it started) and an end
// 30 s later, or at the cancel time for a run canceled before it started.
func recordedRun(appID, name string, at time.Time, status string, started bool) []ServiceEventFact {
	run := appv1alpha1.CronRun{Name: name, Status: status, FinishedAt: at.Format(time.RFC3339)}
	if started {
		run.StartedAt = at.Format(time.RFC3339)
		run.FinishedAt = at.Add(30 * time.Second).Format(time.RFC3339)
	}
	return cronRunFacts(appID, run)
}
