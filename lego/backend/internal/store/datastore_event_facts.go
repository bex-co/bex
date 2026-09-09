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
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/eventvocab"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// DatastoreEventFactType is the closed vocabulary of managed-datastore event
// facts — the Database/KeyValue sibling of ServiceEventFactType. The names come
// from eventvocab so the projector here, the retrievable event index, and the
// subscription picker cannot drift; migration 0107's fact_type CHECK is the
// persisted half of the same list.
type DatastoreEventFactType string

const (
	DatastoreFactPostgresUnavailable DatastoreEventFactType = eventvocab.TypePostgresUnavailable
	DatastoreFactPostgresAvailable   DatastoreEventFactType = eventvocab.TypePostgresAvailable
	DatastoreFactKeyValueUnhealthy   DatastoreEventFactType = eventvocab.TypeKeyValueUnhealthy
	DatastoreFactKeyValueAvailable   DatastoreEventFactType = eventvocab.TypeKeyValueAvailable

	DatastoreFactPostgresBackupCompleted  DatastoreEventFactType = eventvocab.TypePostgresBackupCompleted
	DatastoreFactPostgresBackupFailed     DatastoreEventFactType = eventvocab.TypePostgresBackupFailed
	DatastoreFactPostgresRestoreSucceeded DatastoreEventFactType = eventvocab.TypePostgresRestoreSucceeded
	DatastoreFactPostgresRestoreFailed    DatastoreEventFactType = eventvocab.TypePostgresRestoreFailed
	DatastoreFactPostgresUpgradeStarted   DatastoreEventFactType = eventvocab.TypePostgresUpgradeStarted
	DatastoreFactPostgresUpgradeSucceeded DatastoreEventFactType = eventvocab.TypePostgresUpgradeSucceeded
	DatastoreFactPostgresUpgradeFailed    DatastoreEventFactType = eventvocab.TypePostgresUpgradeFailed
)

// DatastoreKind separates the two managed datastore products. It is the CHECK
// constraint's vocabulary, and it decides which availability names an edge
// takes — Render says a Postgres is "unavailable" and a Key Value is
// "unhealthy", and bex matches that rather than normalizing it away.
const (
	DatastoreKindPostgres = "postgres"
	DatastoreKindKeyValue = "keyvalue"
)

// DatastoreEventFact is one closed, non-secret managed-datastore event record.
// SourceKey is a stable producer identity used for idempotency; it is never
// exposed directly. The backup/version/schedule detail fields are empty on
// every availability fact — they belong to the backup, restore, and major-
// upgrade kinds that share this table.
type DatastoreEventFact struct {
	SourceKey   string
	WorkspaceID string
	DatastoreID string
	Kind        string
	Type        DatastoreEventFactType
	At          time.Time
	ReasonCode  string
	BackupName  string
	BackupError string
	VersionFrom string
	VersionTo   string
	// Scheduled distinguishes a nightly backup from an on-demand one. nil
	// means the fact has no schedule dimension at all (every availability
	// fact), which is why it is a pointer rather than a plain bool.
	Scheduled *bool
}

// ObservedDatastoreState is the small level-triggered snapshot the control-plane
// reconciler derives from typed Database/KeyValue status — ObservedServiceState
// with the datastore's own identity (its immutable dpg-/red- metadata.name plus
// the workspace its CR label carries, since a datastore has no apps row to join
// through). Availability is empty outside a running datastore, healthy, or
// unhealthy, and carries the same AvailabilityObserved / ReadyTransitionAt
// contract the App snapshot documents.
type ObservedDatastoreState struct {
	DatastoreID string
	WorkspaceID string
	Kind        string
	At          time.Time
	Phase       string
	// Suspended is spec.suspended at observation time. Only the user-driven
	// Suspend verb writes it, so it separates intentional downtime from an
	// outage — a suspended datastore is observed availability-empty, never
	// unavailable (the hibernated-App precedent).
	Suspended            bool
	Availability         string
	AvailabilityObserved bool
	ReasonCode           string
	ReadyTransitionAt    time.Time

	// LastBackup* mirror Database.status.lastBackup for t002 edge detection.
	// Empty Name means the operator has not yet projected a terminal backup.
	LastBackupName      string
	LastBackupPhase     string // completed | failed
	LastBackupError     string
	LastBackupScheduled bool
	LastBackupAt        time.Time

	// Recovering is true when Spec.Recovery.SourceBackupServerName is set —
	// the Database was created as a restore target (ADR009).
	Recovering bool

	// SpecVersion / CurrentVersion drive major-upgrade edges. UpgradeFailed
	// is Ready=False with reason MajorVersionUpgradeFailed.
	SpecVersion    string
	CurrentVersion string
	UpgradeFailed  bool

	// Checkpoint-only extras round-tripped by the memStore (PG keeps them in
	// dedicated columns). Producers leave them empty; RecordObserved* fills them.
	RestoreOutcome string
	UpgradeKey     string
}

// datastoreCheckpointExtras is the t002 half of the checkpoint row — fields
// the availability CAS does not touch but that must round-trip with it so a
// backup/upgrade edge cannot be lost between passes.
type datastoreCheckpointExtras struct {
	LastBackupName  string
	LastBackupPhase string
	RestoreOutcome  string
	UpgradeKey      string
}

// datastoreFactTypeFor names the availability edge for one datastore kind.
func datastoreFactTypeFor(kind string, available bool) DatastoreEventFactType {
	if kind == DatastoreKindKeyValue {
		if available {
			return DatastoreFactKeyValueAvailable
		}
		return DatastoreFactKeyValueUnhealthy
	}
	if available {
		return DatastoreFactPostgresAvailable
	}
	return DatastoreFactPostgresUnavailable
}

func validDatastoreKind(kind string) bool {
	return kind == DatastoreKindPostgres || kind == DatastoreKindKeyValue
}

// nextDatastoreAvailability decides what the checkpoint latches, and is where
// "the first Provisioned arms the edge" lives.
//
// A datastore that has never reported Ready is provisioning, not down: for
// managed Postgres the two states are literally indistinguishable at the CR
// (zero ready CNPG instances reports Phase=Provisioning / Reason=Provisioning
// whether the cluster is being created or has just lost its only instance).
// Refusing to latch 'unhealthy' from anything but 'healthy' resolves that
// without asking the CR a question it cannot answer: before the first healthy
// observation the checkpoint stays empty, so no unavailable edge fires and —
// just as importantly — no spurious available edge fires when provisioning
// finally succeeds.
func nextDatastoreAvailability(previous, observed string) string {
	if observed == "unhealthy" && previous != "healthy" && previous != "unhealthy" {
		return previous
	}
	return observed
}

// observedDatastoreStateFacts returns the availability edges this observation
// crossed. Deliberately narrower than the App path's observedStateFacts: a
// datastore's phase moves are covered by audit-derived facts (create, restart,
// plan change), so only the availability dimension produces edges here.
func observedDatastoreStateFacts(obs ObservedDatastoreState, previousAvailability string) []DatastoreEventFact {
	if obs.Availability == previousAvailability {
		return nil
	}
	var typ DatastoreEventFactType
	switch {
	case previousAvailability == "healthy" && obs.Availability == "unhealthy":
		typ = datastoreFactTypeFor(obs.Kind, false)
	case previousAvailability == "unhealthy" && obs.Availability == "healthy":
		typ = datastoreFactTypeFor(obs.Kind, true)
	default:
		return nil
	}
	return []DatastoreEventFact{{
		SourceKey:   datastoreFactSourceKey(obs, typ),
		WorkspaceID: obs.WorkspaceID,
		DatastoreID: obs.DatastoreID,
		Kind:        obs.Kind,
		Type:        typ,
		At:          obs.At,
		ReasonCode:  obs.ReasonCode,
	}}
}

// datastoreFactSourceKey keys an availability edge on the operator-side Ready
// transition backing it, so a resync or a process restart that re-derives the
// same conclusion replays onto the same row instead of appending a duplicate.
// A condition carrying no timestamp falls back to the observation clock: two
// successive edges of the same kind would otherwise collapse onto one key and
// the second would be silently dropped, which is the one failure mode worse
// than a duplicate.
func datastoreFactSourceKey(obs ObservedDatastoreState, typ DatastoreEventFactType) string {
	at := obs.ReadyTransitionAt
	if at.IsZero() {
		at = obs.At
	}
	return fmt.Sprintf("observed:%s:%s:%d", obs.DatastoreID, typ, at.UTC().UnixNano())
}

const insertDatastoreEventFactSQL = `
INSERT INTO datastore_event_facts (
    source_key, workspace_id, datastore_id, kind, fact_type, at, reason_code,
    backup_name, backup_error, version_from, version_to, scheduled
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (source_key) DO NOTHING`

// RecordObservedDatastoreState atomically advances a datastore checkpoint and
// appends any availability edge it crossed — the Database/KeyValue twin of
// RecordObservedServiceState. The first observation is a baseline; replaying
// the same observation is a no-op.
func (s *PGStore) RecordObservedDatastoreState(ctx context.Context, obs ObservedDatastoreState) ([]DatastoreEventFact, error) {
	if obs.DatastoreID == "" || obs.WorkspaceID == "" {
		return nil, fmt.Errorf("observed datastore state requires datastore and workspace id")
	}
	if !validDatastoreKind(obs.Kind) {
		return nil, fmt.Errorf("invalid datastore kind %q", obs.Kind)
	}
	if obs.Availability != "" && obs.Availability != "healthy" && obs.Availability != "unhealthy" {
		return nil, fmt.Errorf("invalid availability %q", obs.Availability)
	}
	if obs.ReasonCode != "" && obs.ReasonCode != EventReasonReadinessFailed {
		return nil, fmt.Errorf("invalid observed reason code %q", obs.ReasonCode)
	}
	if obs.At.IsZero() {
		obs.At = time.Now().UTC()
	}
	// Only an observed healthy conclusion moves the recorded healthy-transition
	// reference; COALESCE below keeps the stored value when this pass has none
	// to offer, so a healthy re-observation without a timestamp never erases a
	// newer recorded one. Same contract as the App checkpoint's.
	var healthyTransition *time.Time
	if obs.AvailabilityObserved && obs.Availability == "healthy" {
		healthyTransition = nullTime(obs.ReadyTransitionAt.UTC())
	}

	var inserted []DatastoreEventFact
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		selectCheckpoint := `SELECT phase, availability, suspended,
			last_backup_name, last_backup_phase, restore_outcome, upgrade_key
			FROM datastore_observed_checkpoints WHERE datastore_id = $1 FOR UPDATE`
		var previousPhase, previousAvailability string
		var previousSuspended bool
		var previousExtras datastoreCheckpointExtras
		err := tx.QueryRow(ctx, selectCheckpoint, obs.DatastoreID).
			Scan(&previousPhase, &previousAvailability, &previousSuspended,
				&previousExtras.LastBackupName, &previousExtras.LastBackupPhase,
				&previousExtras.RestoreOutcome, &previousExtras.UpgradeKey)
		if errors.Is(err, pgx.ErrNoRows) {
			// The baseline latches through the same arming rule as every later
			// pass, so a first-ever observation caught mid-provisioning records
			// an empty availability rather than an unhealthy one. Lifecycle
			// extras (backup/restore/upgrade) also baseline without emitting —
			// a Database created mid-restore must not fire restore_succeeded
			// until it actually reaches Ready after the checkpoint exists.
			baseline := nextDatastoreAvailability("", obs.Availability)
			baselineExtras := datastoreCheckpointExtras{
				LastBackupName:  obs.LastBackupName,
				LastBackupPhase: obs.LastBackupPhase,
			}
			if obs.Recovering && obs.AvailabilityObserved && obs.Availability == "healthy" {
				baselineExtras.RestoreOutcome = "succeeded"
			}
			if obs.Recovering && obs.Phase == string(appv1alpha1.DBPhaseFailed) {
				baselineExtras.RestoreOutcome = "failed"
			}
			if obs.Phase == string(appv1alpha1.DBPhaseUpgrading) && obs.SpecVersion != "" && obs.CurrentVersion != "" && obs.SpecVersion != obs.CurrentVersion {
				baselineExtras.UpgradeKey = fmt.Sprintf("%s:upgrade:%s-%s:started", obs.DatastoreID, obs.CurrentVersion, obs.SpecVersion)
			}
			tag, insertErr := tx.Exec(ctx,
				`INSERT INTO datastore_observed_checkpoints
				     (datastore_id, workspace_id, kind, phase, availability, suspended, updated_at, healthy_transition_at,
				      last_backup_name, last_backup_phase, restore_outcome, upgrade_key)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
				 ON CONFLICT (datastore_id) DO NOTHING`,
				obs.DatastoreID, obs.WorkspaceID, obs.Kind, obs.Phase, baseline, obs.Suspended, obs.At, healthyTransition,
				baselineExtras.LastBackupName, baselineExtras.LastBackupPhase,
				baselineExtras.RestoreOutcome, baselineExtras.UpgradeKey)
			if insertErr != nil {
				return insertErr
			}
			if tag.RowsAffected() > 0 {
				return nil
			}
			err = tx.QueryRow(ctx, selectCheckpoint, obs.DatastoreID).
				Scan(&previousPhase, &previousAvailability, &previousSuspended,
					&previousExtras.LastBackupName, &previousExtras.LastBackupPhase,
					&previousExtras.RestoreOutcome, &previousExtras.UpgradeKey)
		}
		if err != nil {
			return err
		}

		availability := previousAvailability
		if obs.AvailabilityObserved {
			availability = nextDatastoreAvailability(previousAvailability, obs.Availability)
		}
		obs.Availability = availability
		facts := observedDatastoreStateFacts(obs, previousAvailability)
		lifecycle, nextExtras := observedDatastoreLifecycleFacts(obs, previousExtras)
		facts = append(facts, lifecycle...)
		for _, fact := range facts {
			if err := insertDatastoreEventFactTx(ctx, tx, fact); err != nil {
				return err
			}
			inserted = append(inserted, fact)
		}
		// IS DISTINCT FROM makes the steady state a no-op instead of a row
		// write: the reconciler observes every datastore on every resync and
		// almost none of them changed. Nothing reads updated_at — it is
		// write-only bookkeeping — so letting it stop advancing costs nothing.
		// Same guard the App checkpoint uses. Lifecycle extras join the guard
		// so a backup completion that doesn't change availability still writes.
		_, err = tx.Exec(ctx,
			`UPDATE datastore_observed_checkpoints
			 SET workspace_id = $2, phase = $3, availability = $4, suspended = $5, updated_at = $6,
			     healthy_transition_at = COALESCE($7, healthy_transition_at),
			     last_backup_name = $8, last_backup_phase = $9,
			     restore_outcome = $10, upgrade_key = $11
			 WHERE datastore_id = $1
			   AND (workspace_id, phase, availability, suspended,
			        last_backup_name, last_backup_phase, restore_outcome, upgrade_key)
			       IS DISTINCT FROM ($2, $3, $4, $5, $8, $9, $10, $11)`,
			obs.DatastoreID, obs.WorkspaceID, obs.Phase, availability, obs.Suspended, obs.At, healthyTransition,
			nextExtras.LastBackupName, nextExtras.LastBackupPhase,
			nextExtras.RestoreOutcome, nextExtras.UpgradeKey)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("record observed datastore state: %w", err)
	}
	return inserted, nil
}

func insertDatastoreEventFactTx(ctx context.Context, tx pgx.Tx, fact DatastoreEventFact) error {
	_, err := tx.Exec(ctx, insertDatastoreEventFactSQL,
		fact.SourceKey, fact.WorkspaceID, fact.DatastoreID, fact.Kind, fact.Type, fact.At,
		fact.ReasonCode, fact.BackupName, fact.BackupError, fact.VersionFrom, fact.VersionTo,
		fact.Scheduled)
	return err
}

// LastDatastoreHealthyTransitionAt returns the Ready=True transition time
// recorded with the datastore's CURRENT healthy checkpoint — the reference the
// reconciler's stale-conclusion guard orders an unhealthy edge against. Zero
// when there is no healthy checkpoint right now or its transition time is
// unknown; like the App path, the guard must then fail open toward recording
// real outages, never toward silence.
func (s *PGStore) LastDatastoreHealthyTransitionAt(ctx context.Context, datastoreID string) (time.Time, error) {
	var at *time.Time
	err := s.Pool.QueryRow(ctx,
		`SELECT healthy_transition_at FROM datastore_observed_checkpoints
		 WHERE datastore_id = $1 AND availability = 'healthy'`,
		datastoreID).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("last datastore healthy transition: %w", err)
	}
	if at == nil {
		return time.Time{}, nil
	}
	return at.UTC(), nil
}
