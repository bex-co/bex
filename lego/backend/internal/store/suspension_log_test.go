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
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// suspensionStore is the part of both stores these tests drive.
type suspensionStore interface {
	RecordObservedServiceState(context.Context, ObservedServiceState) ([]ServiceEventFact, error)
	RecordServiceSuspensionChange(context.Context, string, bool, time.Time) error
}

func running(appID string, at, specReadAt time.Time, suspended bool) ObservedServiceState {
	return ObservedServiceState{
		AppID: appID, At: at, SpecReadAt: specReadAt, ServicePhase: string(appv1alpha1.PhaseRunning),
		Suspended: suspended, Availability: "healthy", AvailabilityObserved: true,
	}
}

func inPhase(appID string, phase appv1alpha1.AppPhase, at time.Time, suspended bool) ObservedServiceState {
	return ObservedServiceState{AppID: appID, At: at, SpecReadAt: at.Add(-time.Second), ServicePhase: string(phase),
		Suspended: suspended, AvailabilityObserved: true}
}

// exerciseSuspensionLog runs the w4/604 and w21/065 cases against one store.
// newApp returns a fresh app id whose checkpoint has a first, healthy Running
// pass, and pending counts the logged changes no pass has replayed.
func exerciseSuspensionLog(t *testing.T, st suspensionStore, newApp func(t *testing.T, base time.Time) string, pending func(t *testing.T, appID string) int) {
	ctx := context.Background()
	record := func(t *testing.T, obs ObservedServiceState) []ServiceEventFact {
		t.Helper()
		facts, err := st.RecordObservedServiceState(ctx, obs)
		if err != nil {
			t.Fatalf("record observed state: %v", err)
		}
		return facts
	}
	logChange := func(t *testing.T, appID string, suspended bool, at time.Time) {
		t.Helper()
		if err := st.RecordServiceSuspensionChange(ctx, appID, suspended, at); err != nil {
			t.Fatalf("log suspension change: %v", err)
		}
	}
	base := func() time.Time { return time.Now().UTC().Truncate(time.Microsecond).Add(-time.Hour) }
	at := func(b time.Time, seconds int) time.Time { return b.Add(time.Duration(seconds) * time.Second) }
	// types lists the lifecycle facts in order, each with its time in seconds from b.
	type event struct {
		typ ServiceEventFactType
		at  int
	}
	events := func(b time.Time, facts []ServiceEventFact) []event {
		var out []event
		for _, f := range facts {
			if f.Type == EventFactServiceSuspended || f.Type == EventFactServiceResumed {
				out = append(out, event{f.Type, int(f.At.Sub(b) / time.Second)})
			}
		}
		return out
	}
	want := func(t *testing.T, got []event, wanted ...event) {
		t.Helper()
		if fmt.Sprint(got) != fmt.Sprint(wanted) {
			t.Fatalf("events = %v, want %v", got, wanted)
		}
	}
	suspended, resumed := EventFactServiceSuspended, EventFactServiceResumed

	t.Run("a Suspend undone before any pass records both, each at its action", func(t *testing.T) {
		b := base()
		appID := newApp(t, b)
		logChange(t, appID, true, at(b, 10))
		logChange(t, appID, false, at(b, 15))
		// The next pass reads the spec at +20 s.
		facts := record(t, running(appID, at(b, 21), at(b, 20), false))
		want(t, events(b, facts), event{suspended, 10}, event{resumed, 15})
		if !facts[1].At.After(facts[0].At) {
			t.Errorf("service_resumed at %v must follow service_suspended at %v", facts[1].At, facts[0].At)
		}
		if again := record(t, running(appID, at(b, 51), at(b, 50), false)); len(again) != 0 {
			t.Errorf("a later pass recorded %v again", factTypes(again))
		}
		if n := pending(t, appID); n != 0 {
			t.Errorf("%d logged changes left after the replay", n)
		}
	})

	t.Run("a pass that read the spec before the Suspend invents no resume", func(t *testing.T) {
		b := base()
		appID := newApp(t, b)
		logChange(t, appID, true, at(b, 10))
		// Read at +9 s, before the Suspend was written; recorded at +11 s.
		if facts := record(t, running(appID, at(b, 11), at(b, 9), false)); len(facts) != 0 {
			t.Fatalf("a stale pass recorded %v; the service is still suspended", factTypes(facts))
		}
		// The next pass sees the suspension; the service then stops.
		if facts := record(t, running(appID, at(b, 13), at(b, 12), true)); len(facts) != 0 {
			t.Fatalf("a pass still Running recorded %v", factTypes(facts))
		}
		want(t, events(b, record(t, inPhase(appID, appv1alpha1.PhaseHibernated, at(b, 30), true))), event{suspended, 30})
	})

	t.Run("a long suspension still records one service_suspended", func(t *testing.T) {
		b := base()
		appID := newApp(t, b)
		logChange(t, appID, true, at(b, 10))
		if facts := record(t, running(appID, at(b, 13), at(b, 12), true)); len(facts) != 0 {
			t.Fatalf("facts = %v, want none while still Running", factTypes(facts))
		}
		want(t, events(b, record(t, inPhase(appID, appv1alpha1.PhaseHibernated, at(b, 30), true))), event{suspended, 30})
		if again := record(t, inPhase(appID, appv1alpha1.PhaseHibernated, at(b, 60), true)); len(again) != 0 {
			t.Errorf("a later pass recorded %v again", factTypes(again))
		}
	})

	t.Run("a sleeping service's Suspend is recorded at its action", func(t *testing.T) {
		b := base()
		appID := newApp(t, b)
		asleep := ObservedServiceState{AppID: appID, At: at(b, 5), SpecReadAt: at(b, 4),
			ServicePhase: string(appv1alpha1.PhaseHibernated), Asleep: true, AvailabilityObserved: true}
		if got := factTypes(record(t, asleep)); len(got) != 1 || got[0] != EventFactServiceHibernated {
			t.Fatalf("facts = %v, want service_hibernated", got)
		}
		logChange(t, appID, true, at(b, 10))
		// w4/562: the rising flag on a sleeping service is recorded once.
		suspendedPass := asleep
		suspendedPass.At, suspendedPass.SpecReadAt, suspendedPass.Suspended, suspendedPass.Asleep = at(b, 13), at(b, 12), true, false
		want(t, events(b, record(t, suspendedPass)), event{suspended, 10})
	})

	t.Run("six Suspend-Resume runs 5 s apart reach one pass as six pairs", func(t *testing.T) {
		b := base()
		appID := newApp(t, b)
		var wanted []event
		for i := 0; i < 6; i++ {
			s := 10 + i*11
			logChange(t, appID, true, at(b, s))
			logChange(t, appID, false, at(b, s+5))
			wanted = append(wanted, event{suspended, s}, event{resumed, s + 5})
		}
		want(t, events(b, record(t, running(appID, at(b, 90), at(b, 89), false))), wanted...)
		if n := pending(t, appID); n != 0 {
			t.Errorf("%d logged changes left after the replay", n)
		}
	})

	t.Run("passes between and around the actions never lose one", func(t *testing.T) {
		b := base()
		appID := newApp(t, b)
		var got []event
		note := func(facts []ServiceEventFact) { got = append(got, events(b, facts)...) }
		// Each pass reads the spec as of its start, and records a little later:
		// the next action can land in between, which is how the m369 mark
		// overwrote a Resume no pass had recorded yet.
		note(record(t, running(appID, at(b, 12), at(b, 9), false)))
		logChange(t, appID, true, at(b, 10))
		note(record(t, running(appID, at(b, 14), at(b, 11), true)))
		logChange(t, appID, false, at(b, 15))
		logChange(t, appID, true, at(b, 20)) // lands before the pass that read the Resume records
		note(record(t, running(appID, at(b, 21), at(b, 16), false)))
		logChange(t, appID, false, at(b, 25))
		note(record(t, running(appID, at(b, 40), at(b, 39), false)))
		want(t, got, event{suspended, 10}, event{resumed, 15}, event{suspended, 20}, event{resumed, 25})
	})

	t.Run("a service that stops between the actions reports each of them", func(t *testing.T) {
		b := base()
		appID := newApp(t, b)
		var got []event
		note := func(facts []ServiceEventFact) { got = append(got, events(b, facts)...) }
		// S1 at +10, the instance stops at +13; R1 at +20 starts its wake; S2
		// at +30 arrives before the wake finishes, R2 at +40 starts another.
		logChange(t, appID, true, at(b, 10))
		note(record(t, inPhase(appID, appv1alpha1.PhaseHibernated, at(b, 13), true)))
		logChange(t, appID, false, at(b, 20))
		note(record(t, inPhase(appID, appv1alpha1.PhaseDeploying, at(b, 22), false)))
		if len(got) != 1 {
			t.Fatalf("events while the first wake runs = %v, want only the first service_suspended", got)
		}
		logChange(t, appID, true, at(b, 30))
		note(record(t, inPhase(appID, appv1alpha1.PhaseDeploying, at(b, 32), true)))
		logChange(t, appID, false, at(b, 40))
		// The second wake finishes: its instance went Ready at +50.
		ready := running(appID, at(b, 52), at(b, 51), false)
		ready.ReadyTransitionAt = at(b, 50)
		note(record(t, ready))
		want(t, got, event{suspended, 13}, event{resumed, 20}, event{suspended, 30}, event{resumed, 50})
		if again := record(t, running(appID, at(b, 90), at(b, 89), false)); len(again) != 0 {
			t.Errorf("a later pass recorded %v again", factTypes(again))
		}
		if n := pending(t, appID); n != 0 {
			t.Errorf("%d logged changes left once the wake was reported", n)
		}
	})

	t.Run("a held Resume is reported once, at its wake", func(t *testing.T) {
		b := base()
		appID := newApp(t, b)
		logChange(t, appID, true, at(b, 10))
		record(t, inPhase(appID, appv1alpha1.PhaseHibernated, at(b, 13), true))
		logChange(t, appID, false, at(b, 20))
		if facts := record(t, inPhase(appID, appv1alpha1.PhaseDeploying, at(b, 22), false)); len(facts) != 0 {
			t.Fatalf("a waking pass recorded %v; the wake reports the Resume", factTypes(facts))
		}
		if n := pending(t, appID); n != 1 {
			t.Fatalf("%d logged changes while the wake runs, want the held Resume", n)
		}
		ready := running(appID, at(b, 32), at(b, 31), false)
		ready.ReadyTransitionAt = at(b, 30)
		want(t, events(b, record(t, ready)), event{resumed, 30})
		if n := pending(t, appID); n != 0 {
			t.Errorf("%d logged changes after the wake was reported", n)
		}
		if again := record(t, running(appID, at(b, 60), at(b, 59), false)); len(again) != 0 {
			t.Errorf("a later pass recorded %v again", factTypes(again))
		}
	})

	t.Run("passes running at once report each action once", func(t *testing.T) {
		b := base()
		appID := newApp(t, b)
		for i := 0; i < 6; i++ {
			logChange(t, appID, true, at(b, 10+i*11))
			logChange(t, appID, false, at(b, 15+i*11))
		}
		var mu sync.Mutex
		var all []ServiceEventFact
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				facts, err := st.RecordObservedServiceState(ctx, running(appID, at(b, 90), at(b, 89), false))
				if err != nil {
					t.Errorf("record observed state: %v", err)
				}
				mu.Lock()
				all = append(all, facts...)
				mu.Unlock()
			}()
		}
		wg.Wait()
		if got := events(b, all); len(got) != 12 {
			t.Fatalf("concurrent passes recorded %d lifecycle facts (%v), want 12", len(got), got)
		}
	})
}

func TestSuspensionLogReportsEveryAction(t *testing.T) {
	st := newMemStore()
	n := 0
	exerciseSuspensionLog(t, st, func(t *testing.T, base time.Time) string {
		n++
		appID := fmt.Sprintf("srv-log-%d", n)
		if _, err := st.RecordObservedServiceState(context.Background(), running(appID, base, base, false)); err != nil {
			t.Fatal(err)
		}
		return appID
	}, func(_ *testing.T, appID string) int {
		st.mu.Lock()
		defer st.mu.Unlock()
		return len(st.suspensionChanges[appID])
	})
}

func TestPGSuspensionLogReportsEveryAction(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)

	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	tenant, err := st.CreateWorkspace(ctx, "suspension-log-"+stamp, PlanHobby, "alice-"+stamp)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	n := 0
	exerciseSuspensionLog(t, st, func(t *testing.T, base time.Time) string {
		n++
		app, err := st.CreateApp(ctx, App{
			TenantID: tenant.ID, Name: fmt.Sprintf("log-%d-%s", n, stamp), Image: "nginx:1.27-alpine",
			Branch: "main", Port: 80, Replicas: 1, Tier: "free",
		})
		if err != nil {
			t.Fatalf("create app: %v", err)
		}
		if _, err := st.RecordObservedServiceState(ctx, running(app.ID, base, base, false)); err != nil {
			t.Fatal(err)
		}
		return app.ID
	}, func(t *testing.T, appID string) int {
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM service_suspension_changes WHERE app_id = $1`, appID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	})
}

// TestReplayWithNothingLoggedAppliesThePassAlone: the pass reads whether
// changes are logged before it waits for the checkpoint's lock, so another pass
// can have replayed them by the time it reads them (w21/065). It then has the
// pass's own observation to apply and nothing else.
func TestReplayWithNothingLoggedAppliesThePassAlone(t *testing.T) {
	obs := running("srv-empty", time.Now().UTC(), time.Now().UTC(), true)
	var applied []ObservedServiceState
	keep, err := replaySuspensionChanges(nil, obs,
		func() serviceCheckpoint { return serviceCheckpoint{Phase: string(appv1alpha1.PhaseRunning)} },
		func(o ObservedServiceState) error { applied = append(applied, o); return nil })
	if err != nil || keep != 0 || len(applied) != 1 || applied[0].Suspended != obs.Suspended || !applied[0].At.Equal(obs.At) {
		t.Fatalf("keep %d, err %v, applied %v; want the pass's own observation once and nothing kept", keep, err, applied)
	}
}
