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

// Package eventvocab owns Render event names shared by independent event
// projections. Keeping datastore audit mappings here prevents a webhook from
// advertising an event that retrieve-by-ID cannot translate.
package eventvocab

import "github.com/bex-co/bex/lego/backend/internal/core"

const (
	TypePlanChanged                = "plan_changed"
	TypePostgresCreated            = "postgres_created"
	TypePostgresRestarted          = "postgres_restarted"
	TypePostgresCredentialsCreated = "postgres_credentials_created"
	TypePostgresCredentialsDeleted = "postgres_credentials_deleted"
	TypePostgresBackupStarted      = "postgres_backup_started"
)

// Observed-lifecycle names (w3/m82). Unlike the audit-derived names above,
// these are produced by the control-plane reconciler from Database/KeyValue
// status, so no user verb maps to them; they live here so the projector, the
// retrievable event index, and the subscription picker share one spelling.
// Postgres and Key Value deliberately differ (unavailable vs unhealthy) —
// that is Render's own vocabulary, not an inconsistency to normalize away.
const (
	TypePostgresUnavailable = "postgres_unavailable"
	TypePostgresAvailable   = "postgres_available"
	TypeKeyValueUnhealthy   = "key_value_unhealthy"
	TypeKeyValueAvailable   = "key_value_available"
)

// Backup / restore / major-upgrade names (w3/m82 t002). Declared with their
// availability siblings so the closed vocabulary is readable in one place;
// migration 0107's fact_type CHECK is the persisted half of the same list.
const (
	TypePostgresBackupCompleted  = "postgres_backup_completed"
	TypePostgresBackupFailed     = "postgres_backup_failed"
	TypePostgresRestoreSucceeded = "postgres_restore_succeeded"
	TypePostgresRestoreFailed    = "postgres_restore_failed"
	TypePostgresUpgradeStarted   = "postgres_upgrade_started"
	TypePostgresUpgradeSucceeded = "postgres_upgrade_succeeded"
	TypePostgresUpgradeFailed    = "postgres_upgrade_failed"
)

// Configuration-change names (w3/m82 t003). Audit-derived like the block at the
// top of the file, but field-level rather than lifecycle: each is produced by a
// successful PATCH that changed exactly that setting, and the audit row carries
// the value it was set to (see core.AuditEvent's datastore fields).
//
// Render's postgres_read_replicas_changed is deliberately NOT here: read
// replicas are a create-time-only array in bex (PostgresPatch has no
// ReadReplicas field, and the Blueprint apply path writes spec.readReplicas
// without recording any datastore audit effect), so there is no post-create
// mutation to source the event from. Advertising it would name an event nothing
// can emit. Reopen when a replica add/remove verb exists.
const (
	TypePostgresHAStatusChanged              = "postgres_ha_status_changed"
	TypePostgresConnectionPoolEnabledChanged = "postgres_connection_pool_enabled_changed"
	TypePostgresDiskSizeChanged              = "postgres_disk_size_changed"
	// TypeKeyValueConfigRestart is Render's "a config change restarted the
	// instance". Verified against the mechanism before advertising: the operator
	// folds spec.maxmemoryPolicy and spec.persistenceMode into the Valkey server
	// flags (keyvalue_controller.go valkeyArgs), those flags are the
	// StatefulSet's container Args, and the reconcile is a CreateOrUpdate on
	// that StatefulSet — so changing either setting rolls the single pod.
	TypeKeyValueConfigRestart = "key_value_config_restart"
)

// DatastoreAuditTypes returns the closed mapping from source audit verbs to
// webhook/retrieval event names. Callers receive their own map so package-local
// additions cannot mutate the shared vocabulary.
func DatastoreAuditTypes() map[string]string {
	return map[string]string{
		core.AuditVerbPostgresCreated:            TypePostgresCreated,
		core.AuditVerbPostgresRestarted:          TypePostgresRestarted,
		core.AuditVerbPostgresCredentialsCreated: TypePostgresCredentialsCreated,
		core.AuditVerbPostgresCredentialsDeleted: TypePostgresCredentialsDeleted,
		core.AuditVerbPostgresBackupStarted:      TypePostgresBackupStarted,
		core.AuditVerbPostgresPlanChanged:        TypePlanChanged,
		core.AuditVerbKeyValuePlanChanged:        TypePlanChanged,
		core.AuditVerbPostgresHAChanged:          TypePostgresHAStatusChanged,
		core.AuditVerbPostgresPoolerChanged:      TypePostgresConnectionPoolEnabledChanged,
		core.AuditVerbPostgresDiskSizeChanged:    TypePostgresDiskSizeChanged,
		core.AuditVerbKeyValueConfigChanged:      TypeKeyValueConfigRestart,
	}
}
