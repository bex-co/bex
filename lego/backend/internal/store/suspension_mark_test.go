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
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// suspensionStore is the part of both stores these tests drive.
type suspensionStore interface {
	RecordObservedServiceState(context.Context, ObservedServiceState) ([]ServiceEventFact, error)
	RecordServiceSuspensionRequested(context.Context, string, time.Time) error
}

func running(appID string, at, specReadAt time.Time, suspended bool) ObservedServiceState {
	return ObservedServiceState{
		AppID: appID, At: at, SpecReadAt: specReadAt, ServicePhase: string(appv1alpha1.PhaseRunning),
		Suspended: suspended, Availability: "healthy", AvailabilityObserved: true,
	}
}

// exerciseSuspensionMark runs the w4/604 cases against one store. newApp
// returns a fresh app id whose checkpoint has a first, healthy Running pass.
func exerciseSuspensionMark(t *testing.T, st suspensionStore, newApp func(t *testing.T, base time.Time) string) {
	ctx := context.Background()
	record := func(t *testing.T, obs ObservedServiceState) []ServiceEventFact {
		t.Helper()
		facts, err := st.RecordObservedServiceState(ctx, obs)
		if err != nil {
			t.Fatalf("record observed state: %v", err)
		}
		return facts
	}
	mark := func(t *testing.T, appID string, at time.Time) {
		t.Helper()
		if err := st.RecordServiceSuspensionRequested(ctx, appID, at); err != nil {
			t.Fatalf("mark suspension: %v", err)
		}
	}

	t.Run("a Suspend undone before any pass records both, the suspension at the mark", func(t *testing.T) {
		base := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Minute)
		appID := newApp(t, base)
		suspendedAt := base.Add(10 * time.Second)
		mark(t, appID, suspendedAt)
		// Resumed at +15 s; the next pass reads the spec at +20 s.
		facts := record(t, running(appID, base.Add(21*time.Second), base.Add(20*time.Second), false))
		if got := factTypes(facts); len(got) != 2 || got[0] != EventFactServiceSuspended || got[1] != EventFactServiceResumed {
			t.Fatalf("facts = %v, want service_suspended then service_resumed", got)
		}
		if !facts[0].At.Equal(suspendedAt) {
			t.Errorf("service_suspended at %v, want the mark %v (after the Suspend's audit row)", facts[0].At, suspendedAt)
		}
		if !facts[1].At.After(facts[0].At) {
			t.Errorf("service_resumed at %v must follow service_suspended at %v", facts[1].At, facts[0].At)
		}
		if again := record(t, running(appID, base.Add(51*time.Second), base.Add(50*time.Second), false)); len(again) != 0 {
			t.Errorf("a later pass recorded %v again", factTypes(again))
		}
	})

	t.Run("a pass that read the spec before the Suspend invents no resume", func(t *testing.T) {
		base := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Minute)
		appID := newApp(t, base)
		mark(t, appID, base.Add(10*time.Second))
		// Read at +9 s, before the Suspend was written; recorded at +11 s.
		if facts := record(t, running(appID, base.Add(11*time.Second), base.Add(9*time.Second), false)); len(facts) != 0 {
			t.Fatalf("a stale pass recorded %v; the service is still suspended", factTypes(facts))
		}
		// The next pass sees the suspension; the service then stops.
		if facts := record(t, running(appID, base.Add(13*time.Second), base.Add(12*time.Second), true)); len(facts) != 0 {
			t.Fatalf("a pass still Running recorded %v", factTypes(facts))
		}
		stopped := ObservedServiceState{AppID: appID, At: base.Add(30 * time.Second), SpecReadAt: base.Add(29 * time.Second),
			ServicePhase: string(appv1alpha1.PhaseHibernated), Suspended: true, AvailabilityObserved: true}
		if got := factTypes(record(t, stopped)); len(got) != 1 || got[0] != EventFactServiceSuspended {
			t.Fatalf("facts when the service stopped = %v, want exactly one service_suspended", got)
		}
	})

	t.Run("a long suspension still records one service_suspended", func(t *testing.T) {
		base := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Minute)
		appID := newApp(t, base)
		mark(t, appID, base.Add(10*time.Second))
		if facts := record(t, running(appID, base.Add(13*time.Second), base.Add(12*time.Second), true)); len(facts) != 0 {
			t.Fatalf("facts = %v, want none while still Running", factTypes(facts))
		}
		stopped := ObservedServiceState{AppID: appID, At: base.Add(30 * time.Second), SpecReadAt: base.Add(29 * time.Second),
			ServicePhase: string(appv1alpha1.PhaseHibernated), Suspended: true, AvailabilityObserved: true}
		if got := factTypes(record(t, stopped)); len(got) != 1 || got[0] != EventFactServiceSuspended {
			t.Fatalf("facts = %v, want exactly one service_suspended", got)
		}
	})

	t.Run("a sleeping service's checkpoint is left to the pass", func(t *testing.T) {
		base := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Minute)
		appID := newApp(t, base)
		asleep := ObservedServiceState{AppID: appID, At: base.Add(5 * time.Second), SpecReadAt: base.Add(4 * time.Second),
			ServicePhase: string(appv1alpha1.PhaseHibernated), Asleep: true, AvailabilityObserved: true}
		if got := factTypes(record(t, asleep)); len(got) != 1 || got[0] != EventFactServiceHibernated {
			t.Fatalf("facts = %v, want service_hibernated", got)
		}
		mark(t, appID, base.Add(10*time.Second))
		// w4/562: the rising flag on a sleeping service is the pass's to record.
		suspended := asleep
		suspended.At, suspended.SpecReadAt, suspended.Suspended, suspended.Asleep = base.Add(13*time.Second), base.Add(12*time.Second), true, false
		if got := factTypes(record(t, suspended)); len(got) != 1 || got[0] != EventFactServiceSuspended {
			t.Fatalf("facts = %v, want the pass's own service_suspended", got)
		}
	})
}

func TestSuspensionMarkRecordsTheQuickPair(t *testing.T) {
	st := newMemStore()
	n := 0
	exerciseSuspensionMark(t, st, func(t *testing.T, base time.Time) string {
		n++
		appID := fmt.Sprintf("srv-mark-%d", n)
		if _, err := st.RecordObservedServiceState(context.Background(), running(appID, base, base, false)); err != nil {
			t.Fatal(err)
		}
		return appID
	})
}

func TestPGSuspensionMarkRecordsTheQuickPair(t *testing.T) {
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
	tenant, err := st.CreateWorkspace(ctx, "suspension-mark-"+stamp, PlanHobby, "alice-"+stamp)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	n := 0
	exerciseSuspensionMark(t, st, func(t *testing.T, base time.Time) string {
		n++
		app, err := st.CreateApp(ctx, App{
			TenantID: tenant.ID, Name: fmt.Sprintf("mark-%d-%s", n, stamp), Image: "nginx:1.27-alpine",
			Branch: "main", Port: 80, Replicas: 1, Tier: "free",
		})
		if err != nil {
			t.Fatalf("create app: %v", err)
		}
		if _, err := st.RecordObservedServiceState(ctx, running(app.ID, base, base, false)); err != nil {
			t.Fatal(err)
		}
		return app.ID
	})
}
