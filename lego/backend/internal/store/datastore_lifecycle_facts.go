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
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// observedDatastoreLifecycleFacts returns backup, restore, and major-upgrade
// edges crossed between the previous checkpoint extras and this observation
// (w3/m82 t002). Availability edges live in observedDatastoreStateFacts —
// keep them apart so a backup completion cannot accidentally clear an outage.
func observedDatastoreLifecycleFacts(obs ObservedDatastoreState, previous datastoreCheckpointExtras) (facts []DatastoreEventFact, next datastoreCheckpointExtras) {
	next = previous
	if obs.Kind != DatastoreKindPostgres {
		return nil, next
	}
	facts = append(facts, checkpointStartedFacts(obs, previous, &next)...)
	facts = append(facts, backupFacts(obs, previous, &next)...)
	facts = append(facts, exportFacts(obs, previous, &next)...)
	facts = append(facts, walArchiveFacts(obs, previous, &next)...)
	facts = append(facts, restoreFacts(obs, previous, &next)...)
	facts = append(facts, upgradeFacts(obs, previous, &next)...)
	return facts, next
}

// checkpointStartedFacts reports a CNPG base backup starting — Render's
// postgres_pitr_checkpoint_started — once per backup name, while it runs. A
// backup that both started and finished between two passes gets its start
// from backupFacts instead.
func checkpointStartedFacts(obs ObservedDatastoreState, previous datastoreCheckpointExtras, next *datastoreCheckpointExtras) []DatastoreEventFact {
	if obs.ActiveBackupName == "" || obs.ActiveBackupName == previous.StartedBackup {
		return nil
	}
	next.StartedBackup = obs.ActiveBackupName
	at := obs.ActiveBackupAt
	if at.IsZero() {
		at = obs.At
	}
	return []DatastoreEventFact{checkpointStartedFact(obs, obs.ActiveBackupName, at)}
}

// checkpointStartedFact is one backup's start. Its source key is per backup,
// so the running and the finished observation of it record one row.
func checkpointStartedFact(obs ObservedDatastoreState, backup string, at time.Time) DatastoreEventFact {
	return DatastoreEventFact{
		SourceKey:   fmt.Sprintf("%s:checkpoint:%s:started", obs.DatastoreID, backup),
		WorkspaceID: obs.WorkspaceID,
		DatastoreID: obs.DatastoreID,
		Kind:        obs.Kind,
		Type:        DatastoreFactPostgresPITRCheckpointStarted,
		At:          at,
		BackupName:  backup,
	}
}

// backupFacts reports a CNPG base backup's outcome. Every base backup —
// nightly or the post-upgrade one — is a point-in-time-recovery checkpoint in
// Render's vocabulary; postgres_backup_* belongs to exports (exportFacts).
func backupFacts(obs ObservedDatastoreState, previous datastoreCheckpointExtras, next *datastoreCheckpointExtras) []DatastoreEventFact {
	if obs.LastBackupName == "" {
		return nil
	}
	// Same backup name already projected — no new edge, even if the operator
	// rewrites status.lastBackup on every pass.
	if obs.LastBackupName == previous.LastBackupName && obs.LastBackupPhase == previous.LastBackupPhase {
		return nil
	}
	next.LastBackupName = obs.LastBackupName
	next.LastBackupPhase = obs.LastBackupPhase
	var typ DatastoreEventFactType
	switch obs.LastBackupPhase {
	case "completed":
		typ = DatastoreFactPostgresPITRCheckpointCompleted
	case "failed":
		typ = DatastoreFactPostgresPITRCheckpointFailed
	default:
		return nil
	}
	at := obs.LastBackupAt
	if at.IsZero() {
		at = obs.At
	}
	scheduled := obs.LastBackupScheduled
	var facts []DatastoreEventFact
	// Render reports every checkpoint's start. A small database's backup often
	// starts and ends between two passes (seconds, live in w5/m141), so report
	// its start here, dated by CNPG's own startedAt, unless it is the backup
	// last reported running; an earlier recorded start dedupes on its key.
	// Compared with next, not previous: checkpointStartedFacts ran first and
	// may have just reported this very backup still listed as active.
	if next.StartedBackup != obs.LastBackupName {
		startedAt := obs.LastBackupStartedAt
		if startedAt.IsZero() {
			startedAt = at
		}
		facts = append(facts, checkpointStartedFact(obs, obs.LastBackupName, startedAt))
		if obs.ActiveBackupName == "" {
			next.StartedBackup = obs.LastBackupName
		}
	}
	return append(facts, DatastoreEventFact{
		SourceKey:   fmt.Sprintf("%s:backup:%s", obs.DatastoreID, obs.LastBackupName),
		WorkspaceID: obs.WorkspaceID,
		DatastoreID: obs.DatastoreID,
		Kind:        obs.Kind,
		Type:        typ,
		At:          at,
		BackupName:  obs.LastBackupName,
		BackupError: obs.LastBackupError,
		Scheduled:   &scheduled,
	})
}

// WAL-archiving states an observation and the checkpoint latch carry; ""
// means unreported or backups off.
const (
	walArchivingOK     = "ok"
	walArchivingFailed = "failed"
)

// exportFloorGrace backs the first export floor off the observation time: an
// export Job that finished just before this pass may not be in status yet.
const exportFloorGrace = 5 * time.Minute

// exportFloor is the first ExportsSince for a database: below every export
// already visible (they are history), and a grace period before this pass so
// an export the operator records a little late still reports.
func exportFloor(obs ObservedDatastoreState) time.Time {
	floor := obs.At.Add(-exportFloorGrace)
	for _, export := range obs.Exports {
		if export.CompletedAt.After(floor) {
			floor = export.CompletedAt
		}
	}
	return floor
}

// exportFacts reports each logical export reaching available or failed —
// Render's postgres_backup_completed/_failed — exactly once (the source key is
// the export id). Exports that finished before ExportsSince are history: the
// first pass that sees a database only sets the floor.
func exportFacts(obs ObservedDatastoreState, previous datastoreCheckpointExtras, next *datastoreCheckpointExtras) []DatastoreEventFact {
	if previous.ExportsSince.IsZero() {
		next.ExportsSince = exportFloor(obs)
		return nil
	}
	var facts []DatastoreEventFact
	for _, export := range obs.Exports {
		if export.ID == "" || !export.CompletedAt.After(previous.ExportsSince) {
			continue
		}
		typ := DatastoreFactPostgresBackupCompleted
		if export.Failed {
			typ = DatastoreFactPostgresBackupFailed
		}
		facts = append(facts, DatastoreEventFact{
			SourceKey:   fmt.Sprintf("%s:export:%s", obs.DatastoreID, export.ID),
			WorkspaceID: obs.WorkspaceID,
			DatastoreID: obs.DatastoreID,
			Kind:        obs.Kind,
			Type:        typ,
			At:          export.CompletedAt,
			BackupName:  export.ID,
			BackupError: export.Reason,
		})
	}
	return facts
}

// walArchiveFacts reports WAL archiving starting to fail — Render's
// postgres_wal_archive_failed — once per failure; recovery re-arms it. A
// failure seen while the database is suspended, upgrading or restoring is
// expected noise: it neither fires nor latches, so it fires afterwards if it
// persists.
func walArchiveFacts(obs ObservedDatastoreState, previous datastoreCheckpointExtras, next *datastoreCheckpointExtras) []DatastoreEventFact {
	if obs.WALArchiving == walArchivingFailed && (obs.Suspended || obs.Recovering || obs.Phase == string(appv1alpha1.DBPhaseUpgrading)) {
		return nil
	}
	next.WALArchiving = obs.WALArchiving
	if obs.WALArchiving != walArchivingFailed || previous.WALArchiving == walArchivingFailed {
		return nil
	}
	return []DatastoreEventFact{{
		SourceKey:   fmt.Sprintf("%s:wal_archive_failed:%d", obs.DatastoreID, obs.At.UnixNano()),
		WorkspaceID: obs.WorkspaceID,
		DatastoreID: obs.DatastoreID,
		Kind:        obs.Kind,
		Type:        DatastoreFactPostgresWALArchiveFailed,
		At:          obs.At,
		BackupError: obs.WALArchivingMessage,
	}}
}

func restoreFacts(obs ObservedDatastoreState, previous datastoreCheckpointExtras, next *datastoreCheckpointExtras) []DatastoreEventFact {
	if !obs.Recovering || previous.RestoreOutcome != "" {
		return nil
	}
	var typ DatastoreEventFactType
	var outcome string
	switch {
	case obs.AvailabilityObserved && obs.Availability == "healthy":
		typ = DatastoreFactPostgresRestoreSucceeded
		outcome = "succeeded"
	case obs.Phase == string(appv1alpha1.DBPhaseFailed):
		typ = DatastoreFactPostgresRestoreFailed
		outcome = "failed"
	default:
		return nil
	}
	next.RestoreOutcome = outcome
	return []DatastoreEventFact{{
		SourceKey:   fmt.Sprintf("%s:restore:%s", obs.DatastoreID, outcome),
		WorkspaceID: obs.WorkspaceID,
		DatastoreID: obs.DatastoreID,
		Kind:        obs.Kind,
		Type:        typ,
		At:          obs.At,
	}}
}

func upgradeFacts(obs ObservedDatastoreState, previous datastoreCheckpointExtras, next *datastoreCheckpointExtras) []DatastoreEventFact {
	// Without both ends of the version pair there is no stable key and no
	// honest "from→to" payload — stay silent rather than inventing "unknown".
	if obs.SpecVersion == "" || obs.CurrentVersion == "" {
		return nil
	}
	from, to := obs.CurrentVersion, obs.SpecVersion
	keyFor := func(edge string) string {
		return fmt.Sprintf("%s:upgrade:%s-%s:%s", obs.DatastoreID, from, to, edge)
	}

	var facts []DatastoreEventFact
	emit := func(edge string, typ DatastoreEventFactType) {
		key := keyFor(edge)
		if previous.UpgradeKey == key || next.UpgradeKey == key {
			return
		}
		next.UpgradeKey = key
		facts = append(facts, DatastoreEventFact{
			SourceKey:   key,
			WorkspaceID: obs.WorkspaceID,
			DatastoreID: obs.DatastoreID,
			Kind:        obs.Kind,
			Type:        typ,
			At:          obs.At,
			VersionFrom: obs.CurrentVersion,
			VersionTo:   obs.SpecVersion,
		})
	}

	switch {
	case obs.UpgradeFailed:
		emit("failed", DatastoreFactPostgresUpgradeFailed)
	case obs.Phase == string(appv1alpha1.DBPhaseUpgrading) && from != to:
		emit("started", DatastoreFactPostgresUpgradeStarted)
	case previous.UpgradeKey != "" &&
		(obs.Phase == string(appv1alpha1.DBPhaseReady) ||
			(obs.AvailabilityObserved && obs.Availability == "healthy")) &&
		!hasUpgradeTerminal(previous.UpgradeKey) &&
		containsUpgradeEdge(previous.UpgradeKey, "started"):
		emit("succeeded", DatastoreFactPostgresUpgradeSucceeded)
	}
	return facts
}

func hasUpgradeTerminal(key string) bool {
	return containsUpgradeEdge(key, "succeeded") || containsUpgradeEdge(key, "failed")
}

func containsUpgradeEdge(key, edge string) bool {
	return len(key) >= len(edge) && key[len(key)-len(edge):] == edge
}

// stampPITRObservation copies the in-flight backup, the WALArchiving
// condition and the terminal exports off a Database's status.
func stampPITRObservation(obs *ObservedDatastoreState, status *appv1alpha1.DatabaseStatus) {
	if active := status.ActiveBackup; active != nil && active.Name != "" {
		obs.ActiveBackupName = active.Name
		if parsed, err := time.Parse(time.RFC3339, active.StartedAt); err == nil {
			obs.ActiveBackupAt = parsed
		}
	}
	if c := meta.FindStatusCondition(status.Conditions, appv1alpha1.ConditionWALArchiving); c != nil {
		switch c.Status {
		case metav1.ConditionTrue:
			obs.WALArchiving = walArchivingOK
		case metav1.ConditionFalse:
			obs.WALArchiving = walArchivingFailed
			obs.WALArchivingMessage = c.Message
		}
	}
	for _, export := range status.Exports {
		failed := export.Phase == appv1alpha1.DatabaseExportFailed
		if export.Phase != appv1alpha1.DatabaseExportAvailable && !failed {
			continue
		}
		completed, err := time.Parse(time.RFC3339, export.CompletedAt)
		if err != nil {
			continue
		}
		obs.Exports = append(obs.Exports, ObservedDatastoreExport{
			ID: export.ID, Failed: failed, Reason: export.FailureReason, CompletedAt: completed,
		})
	}
}

// stampBackupObservation copies Database.status.lastBackup onto the observed
// snapshot. A nil LastBackup leaves the fields empty so the checkpoint stays
// put.
func stampBackupObservation(obs *ObservedDatastoreState, backup *appv1alpha1.DatabaseLastBackupStatus) {
	if backup == nil || backup.Name == "" {
		return
	}
	obs.LastBackupName = backup.Name
	obs.LastBackupPhase = backup.Phase
	obs.LastBackupError = backup.Error
	obs.LastBackupScheduled = backup.Scheduled
	if parsed, err := time.Parse(time.RFC3339, backup.StartedAt); err == nil {
		obs.LastBackupStartedAt = parsed
	}
	if parsed, err := time.Parse(time.RFC3339, backup.CompletedAt); err == nil {
		obs.LastBackupAt = parsed
	} else {
		obs.LastBackupAt = obs.LastBackupStartedAt
	}
}
