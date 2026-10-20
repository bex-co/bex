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
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestDatastoreCheckpointOutcomeEmitsOncePerName(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()
	base := ObservedDatastoreState{
		DatastoreID: "dpg-backuptest000000001", WorkspaceID: "tea-ws",
		Kind: DatastoreKindPostgres, At: time.Now().UTC(),
		Phase:        string(appv1alpha1.DBPhaseReady),
		Availability: "healthy", AvailabilityObserved: true,
		ReadyTransitionAt: time.Now().UTC(),
	}
	if _, err := st.RecordObservedDatastoreState(ctx, base); err != nil {
		t.Fatal(err)
	}
	completed := base
	completed.At = base.At.Add(time.Second)
	completed.LastBackupName = "dpg-backuptest000000001-b1"
	completed.LastBackupPhase = "completed"
	completed.LastBackupAt = completed.At
	facts, err := st.RecordObservedDatastoreState(ctx, completed)
	if err != nil {
		t.Fatal(err)
	}
	// A CNPG base backup is a point-in-time-recovery checkpoint (w5/m141);
	// postgres_backup_* belongs to exports.
	if len(facts) != 1 || facts[0].Type != DatastoreFactPostgresPITRCheckpointCompleted {
		t.Fatalf("want one pitr_checkpoint_completed, got %+v", facts)
	}
	// Resync with the same lastBackup must not re-emit.
	resync, err := st.RecordObservedDatastoreState(ctx, completed)
	if err != nil {
		t.Fatal(err)
	}
	if len(resync) != 0 {
		t.Fatalf("resync re-emitted: %+v", resync)
	}
	failed := completed
	failed.At = completed.At.Add(time.Second)
	failed.LastBackupName = "dpg-backuptest000000001-b2"
	failed.LastBackupPhase = "failed"
	failed.LastBackupError = "disk full"
	failedFacts, err := st.RecordObservedDatastoreState(ctx, failed)
	if err != nil {
		t.Fatal(err)
	}
	if len(failedFacts) != 1 || failedFacts[0].Type != DatastoreFactPostgresPITRCheckpointFailed {
		t.Fatalf("want one pitr_checkpoint_failed, got %+v", failedFacts)
	}
}

func TestDatastoreRestoreAndUpgradeEdges(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()
	baseline := ObservedDatastoreState{
		DatastoreID: "dpg-restoretest00000001", WorkspaceID: "tea-ws",
		Kind: DatastoreKindPostgres, At: time.Now().UTC(),
		Phase:      string(appv1alpha1.DBPhaseProvisioning),
		Recovering: true,
	}
	if _, err := st.RecordObservedDatastoreState(ctx, baseline); err != nil {
		t.Fatal(err)
	}
	ready := baseline
	ready.At = baseline.At.Add(time.Second)
	ready.Phase = string(appv1alpha1.DBPhaseReady)
	ready.Availability = "healthy"
	ready.AvailabilityObserved = true
	ready.ReadyTransitionAt = ready.At
	facts, err := st.RecordObservedDatastoreState(ctx, ready)
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 1 || facts[0].Type != DatastoreFactPostgresRestoreSucceeded {
		t.Fatalf("want restore_succeeded, got %+v", facts)
	}

	st2 := newMemStore()
	upBase := ObservedDatastoreState{
		DatastoreID: "dpg-upgradetest00000001", WorkspaceID: "tea-ws",
		Kind: DatastoreKindPostgres, At: time.Now().UTC(),
		Phase:        string(appv1alpha1.DBPhaseReady),
		Availability: "healthy", AvailabilityObserved: true,
		ReadyTransitionAt: time.Now().UTC(),
		SpecVersion:       "17", CurrentVersion: "16",
	}
	if _, err := st2.RecordObservedDatastoreState(ctx, upBase); err != nil {
		t.Fatal(err)
	}
	upgrading := upBase
	upgrading.At = upBase.At.Add(time.Second)
	upgrading.Phase = string(appv1alpha1.DBPhaseUpgrading)
	upgrading.AvailabilityObserved = false
	upgrading.Availability = ""
	started, err := st2.RecordObservedDatastoreState(ctx, upgrading)
	if err != nil {
		t.Fatal(err)
	}
	if len(started) != 1 || started[0].Type != DatastoreFactPostgresUpgradeStarted {
		t.Fatalf("want upgrade_started, got %+v", started)
	}
	done := upgrading
	done.At = upgrading.At.Add(time.Second)
	done.Phase = string(appv1alpha1.DBPhaseReady)
	done.Availability = "healthy"
	done.AvailabilityObserved = true
	done.ReadyTransitionAt = done.At
	done.CurrentVersion = "17"
	succeeded, err := st2.RecordObservedDatastoreState(ctx, done)
	if err != nil {
		t.Fatal(err)
	}
	if len(succeeded) != 1 || succeeded[0].Type != DatastoreFactPostgresUpgradeSucceeded {
		t.Fatalf("want upgrade_succeeded, got %+v", succeeded)
	}
}

// pitrBase is a healthy Postgres checkpoint baseline for the w5/m141 edges.
func pitrBase(t *testing.T, st *memStore, id string) ObservedDatastoreState {
	t.Helper()
	base := ObservedDatastoreState{
		DatastoreID: id, WorkspaceID: "tea-ws",
		Kind: DatastoreKindPostgres, At: time.Now().UTC(),
		Phase:        string(appv1alpha1.DBPhaseReady),
		Availability: "healthy", AvailabilityObserved: true,
		ReadyTransitionAt: time.Now().UTC(),
	}
	if facts, err := st.RecordObservedDatastoreState(context.Background(), base); err != nil || len(facts) != 0 {
		t.Fatalf("baseline: %+v, %v", facts, err)
	}
	return base
}

func recordPITR(t *testing.T, st *memStore, obs ObservedDatastoreState) []DatastoreEventFact {
	t.Helper()
	facts, err := st.RecordObservedDatastoreState(context.Background(), obs)
	if err != nil {
		t.Fatal(err)
	}
	return facts
}

func pitrFactTypes(facts []DatastoreEventFact) []DatastoreEventFactType {
	out := make([]DatastoreEventFactType, 0, len(facts))
	for _, f := range facts {
		out = append(out, f.Type)
	}
	return out
}

func TestDatastoreCheckpointStartEmitsOnceThenItsOutcome(t *testing.T) {
	st := newMemStore()
	obs := pitrBase(t, st, "dpg-pitrstart0000000001")
	obs.At = obs.At.Add(time.Second)
	obs.ActiveBackupName = "nightly-1"
	if got := pitrFactTypes(recordPITR(t, st, obs)); len(got) != 1 || got[0] != DatastoreFactPostgresPITRCheckpointStarted {
		t.Fatalf("start = %v", got)
	}
	if got := recordPITR(t, st, obs); len(got) != 0 {
		t.Fatalf("resync re-emitted the start: %+v", got)
	}
	obs.At = obs.At.Add(time.Second)
	obs.ActiveBackupName = ""
	obs.LastBackupName, obs.LastBackupPhase, obs.LastBackupScheduled = "nightly-1", "completed", true
	if got := pitrFactTypes(recordPITR(t, st, obs)); len(got) != 1 || got[0] != DatastoreFactPostgresPITRCheckpointCompleted {
		t.Fatalf("outcome = %v", got)
	}
	// The next night's backup starts a new checkpoint.
	obs.At = obs.At.Add(time.Second)
	obs.ActiveBackupName = "nightly-2"
	if got := pitrFactTypes(recordPITR(t, st, obs)); len(got) != 1 || got[0] != DatastoreFactPostgresPITRCheckpointStarted {
		t.Fatalf("second start = %v", got)
	}
}

func TestDatastoreExportOutcomesEmitOnceAfterTheFloor(t *testing.T) {
	st := newMemStore()
	obs := pitrBase(t, st, "dpg-pitrexport000000001")
	old := ObservedDatastoreExport{ID: "exp-old", CompletedAt: obs.At.Add(-time.Hour)}
	obs.At = obs.At.Add(time.Second)
	obs.Exports = []ObservedDatastoreExport{old}
	if got := recordPITR(t, st, obs); len(got) != 0 {
		t.Fatalf("an export that finished before the checkpoint is history: %+v", got)
	}
	done := ObservedDatastoreExport{ID: "exp-1", CompletedAt: obs.At}
	failed := ObservedDatastoreExport{ID: "exp-2", Failed: true, Reason: "pg_dump: permission denied", CompletedAt: obs.At}
	obs.At = obs.At.Add(time.Second)
	obs.Exports = []ObservedDatastoreExport{old, done, failed}
	facts := recordPITR(t, st, obs)
	if got := pitrFactTypes(facts); len(got) != 2 || got[0] != DatastoreFactPostgresBackupCompleted || got[1] != DatastoreFactPostgresBackupFailed {
		t.Fatalf("export outcomes = %v", got)
	}
	if facts[0].BackupName != "exp-1" || facts[1].BackupError != "pg_dump: permission denied" {
		t.Errorf("export details = %+v", facts)
	}
	obs.At = obs.At.Add(time.Second)
	if got := recordPITR(t, st, obs); len(got) != 0 {
		t.Fatalf("resync re-emitted exports: %+v", got)
	}
}

// An export Job that finished shortly before the checkpoint's first pass but
// reached status only afterwards still reports: the floor backs off a grace
// period from the pass that set it.
func TestDatastoreExportRecordedLateAfterTheFirstPassStillReports(t *testing.T) {
	st := newMemStore()
	obs := pitrBase(t, st, "dpg-pitrlate00000000001")
	late := ObservedDatastoreExport{ID: "exp-late", CompletedAt: obs.At.Add(-30 * time.Second)}
	obs.At = obs.At.Add(time.Second)
	obs.Exports = []ObservedDatastoreExport{late}
	if got := pitrFactTypes(recordPITR(t, st, obs)); len(got) != 1 || got[0] != DatastoreFactPostgresBackupCompleted {
		t.Fatalf("late export = %v", got)
	}
}

func TestDatastoreWALArchiveFailureLatchesAndRearms(t *testing.T) {
	st := newMemStore()
	obs := pitrBase(t, st, "dpg-pitrwal00000000001")
	step := func(state string) []DatastoreEventFactType {
		obs.At = obs.At.Add(time.Second)
		obs.WALArchiving = state
		return pitrFactTypes(recordPITR(t, st, obs))
	}
	if got := step("ok"); len(got) != 0 {
		t.Fatalf("ok = %v", got)
	}
	if got := step("failed"); len(got) != 1 || got[0] != DatastoreFactPostgresWALArchiveFailed {
		t.Fatalf("first failure = %v", got)
	}
	if got := step("failed"); len(got) != 0 {
		t.Fatalf("a persisting failure re-emitted: %v", got)
	}
	step("ok")
	if got := step("failed"); len(got) != 1 {
		t.Fatalf("after recovery the next failure should emit again: %v", got)
	}
	// While suspended (or upgrading, or restoring) a failure is expected: no
	// event and no latch, so it fires once the database is back if it persists.
	step("ok")
	obs.Suspended = true
	if got := step("failed"); len(got) != 0 {
		t.Fatalf("suspended failure emitted: %v", got)
	}
	obs.Suspended = false
	if got := step("failed"); len(got) != 1 {
		t.Fatalf("failure persisting after resume = %v", got)
	}
}

func TestDatastoreBaselineRecordsPITRStateAsSeen(t *testing.T) {
	st := newMemStore()
	obs := ObservedDatastoreState{
		DatastoreID: "dpg-pitrbaseline0000001", WorkspaceID: "tea-ws",
		Kind: DatastoreKindPostgres, At: time.Now().UTC(),
		Phase: string(appv1alpha1.DBPhaseReady), Availability: "healthy", AvailabilityObserved: true,
		ActiveBackupName: "nightly-1", WALArchiving: "failed",
	}
	if got := recordPITR(t, st, obs); len(got) != 0 {
		t.Fatalf("baseline emitted %+v", got)
	}
	obs.At = obs.At.Add(time.Second)
	if got := recordPITR(t, st, obs); len(got) != 0 {
		t.Fatalf("the state the baseline saw re-emitted: %+v", got)
	}
}
