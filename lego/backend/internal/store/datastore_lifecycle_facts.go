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
	facts = append(facts, backupFacts(obs, previous, &next)...)
	facts = append(facts, restoreFacts(obs, previous, &next)...)
	facts = append(facts, upgradeFacts(obs, previous, &next)...)
	return facts, next
}

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
		typ = DatastoreFactPostgresBackupCompleted
	case "failed":
		typ = DatastoreFactPostgresBackupFailed
	default:
		return nil
	}
	at := obs.LastBackupAt
	if at.IsZero() {
		at = obs.At
	}
	scheduled := obs.LastBackupScheduled
	return []DatastoreEventFact{{
		SourceKey:   fmt.Sprintf("%s:backup:%s", obs.DatastoreID, obs.LastBackupName),
		WorkspaceID: obs.WorkspaceID,
		DatastoreID: obs.DatastoreID,
		Kind:        obs.Kind,
		Type:        typ,
		At:          at,
		BackupName:  obs.LastBackupName,
		BackupError: obs.LastBackupError,
		Scheduled:   &scheduled,
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
	if parsed, err := time.Parse(time.RFC3339, backup.CompletedAt); err == nil {
		obs.LastBackupAt = parsed
		return
	}
	if parsed, err := time.Parse(time.RFC3339, backup.StartedAt); err == nil {
		obs.LastBackupAt = parsed
	}
}
