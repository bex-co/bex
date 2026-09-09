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

func TestDatastoreBackupFactsEmitOncePerName(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()
	base := ObservedDatastoreState{
		DatastoreID: "dpg-backuptest000000001", WorkspaceID: "tea-ws",
		Kind: DatastoreKindPostgres, At: time.Now().UTC(),
		Phase: string(appv1alpha1.DBPhaseReady),
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
	if len(facts) != 1 || facts[0].Type != DatastoreFactPostgresBackupCompleted {
		t.Fatalf("want one backup_completed, got %+v", facts)
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
	if len(failedFacts) != 1 || failedFacts[0].Type != DatastoreFactPostgresBackupFailed {
		t.Fatalf("want one backup_failed, got %+v", failedFacts)
	}
}

func TestDatastoreRestoreAndUpgradeEdges(t *testing.T) {
	st := newMemStore()
	ctx := context.Background()
	baseline := ObservedDatastoreState{
		DatastoreID: "dpg-restoretest00000001", WorkspaceID: "tea-ws",
		Kind: DatastoreKindPostgres, At: time.Now().UTC(),
		Phase: string(appv1alpha1.DBPhaseProvisioning),
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
		Phase: string(appv1alpha1.DBPhaseReady),
		Availability: "healthy", AvailabilityObserved: true,
		ReadyTransitionAt: time.Now().UTC(),
		SpecVersion: "17", CurrentVersion: "16",
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
