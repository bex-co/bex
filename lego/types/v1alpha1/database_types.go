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

package v1alpha1

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

// ValidDatabaseName reports whether name is a valid user-facing managed
// Postgres name. Keeping this next to DatabaseSpec.Name makes the CRD contract
// and every API/Blueprint writer share one validation rule.
func ValidDatabaseName(name string) bool {
	return len(name) <= 30 && len(validation.IsDNS1123Label(name)) == 0
}

const postgresIdentifierMaxBytes = 63

// ValidPostgresIdentifier reports whether name is a safe, unquoted PostgreSQL
// identifier. bex deliberately keeps the create-time database/owner contract
// to lowercase ASCII + underscores: it avoids quoting ambiguities in psql,
// connection URLs, CNPG bootstrap, and Blueprint fromDatabase projections.
func ValidPostgresIdentifier(name string) bool {
	if len(name) == 0 || len(name) > postgresIdentifierMaxBytes {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || c == '_' || (i > 0 && c >= '0' && c <= '9') {
			continue
		}
		return false
	}
	return true
}

// DefaultPostgresDatabaseName is the stable legacy/default physical database
// identity derived from the immutable Database metadata.name.
func DefaultPostgresDatabaseName(resourceID string) string {
	return strings.ToLower(strings.ReplaceAll(resourceID, "-", "_"))
}

// EffectiveDatabaseName returns the requested create-time physical database,
// falling back to the stable legacy/default identity when omitted.
func (s DatabaseSpec) EffectiveDatabaseName(resourceID string) string {
	if s.DatabaseName != "" {
		return s.DatabaseName
	}
	return DefaultPostgresDatabaseName(resourceID)
}

// EffectiveDatabaseUser returns the requested create-time owner role,
// independently falling back to the legacy/default role when omitted.
func (s DatabaseSpec) EffectiveDatabaseUser(resourceID string) string {
	if s.DatabaseUser != "" {
		return s.DatabaseUser
	}
	return DefaultPostgresDatabaseName(resourceID) + "_user"
}

// DatabaseSpec is the desired state of a managed PostgreSQL — the Render-style
// "add a Postgres" unit. The operator projects it to a CloudNativePG Cluster in
// the same namespace; the plan sets resources/storage. See
// docs/ADR009-postgresql-management.md.
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.databaseName) ? !has(self.databaseName) : has(self.databaseName) && self.databaseName == oldSelf.databaseName",message="databaseName is immutable"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.databaseUser) ? !has(self.databaseUser) : has(self.databaseUser) && self.databaseUser == oldSelf.databaseUser",message="databaseUser is immutable"
type DatabaseSpec struct {
	// Name is the required mutable, user-facing database name. metadata.name is
	// the immutable dpg-... resource id and the data-plane identity used for
	// every CNPG/PVC/Secret/route name.
	// +required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=30
	// +kubebuilder:validation:Pattern=`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`
	Name string `json:"name"`

	// DatabaseName is the optional create-time physical PostgreSQL database.
	// Empty preserves the stable legacy/default derived from metadata.name.
	// Once present it is immutable: changing it after CNPG initdb would only
	// change control-plane intent, not the already-created SQL database.
	// +optional
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z_][a-z0-9_]{0,62}$`
	DatabaseName string `json:"databaseName,omitempty"`

	// DatabaseUser is the optional create-time physical PostgreSQL owner role.
	// Empty independently preserves the stable legacy/default role derived from
	// metadata.name. Once present it is immutable for the same reason as
	// DatabaseName.
	// +optional
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z_][a-z0-9_]{0,62}$`
	DatabaseUser string `json:"databaseUser,omitempty"`

	// Plan selects the resource allocation (compute + storage + availability).
	// MVP plans are single-instance and fit one node.
	// +optional
	// +kubebuilder:validation:Enum=free;basic-256mb;basic-1gb
	// +kubebuilder:default=free
	Plan string `json:"plan,omitempty"`

	// Version is the major PostgreSQL version (e.g. "16"). Empty => operator default.
	// +optional
	// +kubebuilder:validation:Enum="13";"14";"15";"16";"17";"18"
	Version string `json:"version,omitempty"`

	// StorageGB overrides the plan's default volume size (GB). Storage can only
	// grow, never shrink. 0 => plan default.
	// +optional
	StorageGB int32 `json:"storageGB,omitempty"`

	// DiskAutoscaling enables automatic grow-only storage increases when the
	// fullest CNPG instance PVC reaches the platform threshold. The operator
	// records every resize in status.diskResizeHistory. It never shrinks a PVC.
	// Render's enableDiskAutoscaling write field / diskAutoscalingEnabled read
	// field map to this intent.
	// +optional
	DiskAutoscaling bool `json:"diskAutoscaling,omitempty"`

	// Public, when true and the controller's BEX_DB_DOMAIN is set, exposes the
	// database at "<name>.<BEX_DB_DOMAIN>" through the shared Postgres-aware SNI
	// proxy (TLS passthrough — Postgres terminates its own TLS). Default: in-cluster only.
	// External connections use sslmode=verify-full. See docs/ADR009-postgresql-management.md.
	// +optional
	Public bool `json:"public,omitempty"`

	// Suspended hibernates the CNPG cluster (cnpg.io/hibernation=on): compute is
	// stopped but the PVC is kept (data survives) — Render's Postgres suspend, the
	// sibling of App/KeyValue suspend. Default: running. See
	// docs/ADR007-restart-suspend-and-resume.md.
	// +optional
	Suspended bool `json:"suspended,omitempty"`

	// RestartedAt, when bumped to a fresh timestamp, triggers a CNPG rolling
	// restart of the primary (the operator stamps the cluster's
	// kubectl.kubernetes.io/restartedAt annotation). Verb-as-timestamp, mirroring
	// App.spec.restartedAt.
	// +optional
	RestartedAt string `json:"restartedAt,omitempty"`

	// IPAllowList restricts the EXTERNAL (public) endpoint to these CIDRs in the
	// shared SNI proxy. Empty => the external endpoint is open to all source IPs.
	// The internal "-rw" path is never affected.
	// Render's ipAllowList; only meaningful when Public. Each entry carries the
	// CIDR enforcement reads plus an optional description that rides along
	// untouched (see IPAllowEntry; the schema is structural — a malformed
	// entry is rejected at admission).
	// +optional
	IPAllowList []IPAllowEntry `json:"ipAllowList,omitempty"`

	// EnvironmentIPAllowList is the environment-layer inbound-IP rule set
	// (w4/m28) — written only by the environment fan-out, rendered as a
	// SECOND MiddlewareTCP chained with IPAllowList's on the external SNI
	// route, so a source must pass both layers (Render's intersection
	// semantics; pre-m28 the fan-out full-replaced IPAllowList itself,
	// clobbering the resource's own rules). CIDRs only — descriptions live on
	// the environment row. Empty means no environment layer; an explicitly
	// empty environment rule list projects the deny-all placeholder instead.
	// +optional
	EnvironmentIPAllowList []string `json:"environmentIPAllowList,omitempty"`

	// Pooler, when true, provisions a PgBouncer connection pooler (a CNPG Pooler
	// in transaction mode) for this database; the connection info then surfaces
	// pooled connection strings. Render's connection pooling.
	// +optional
	Pooler bool `json:"pooler,omitempty"`

	// Users are additional managed PostgreSQL login roles, projected to the CNPG
	// cluster's spec.managed.roles. The effective create-time owner role is
	// provisioned by CNPG's bootstrap and is not listed here. Each user's
	// password lives in the referenced Secret's "password" key (created by
	// bex-api, never in status).
	// +optional
	// +listType=map
	// +listMapKey=name
	Users []DatabaseUser `json:"users,omitempty"`

	// DeletedUsers are login-role names removed via the API that must be
	// explicitly dropped from PostgreSQL (ensure:absent). Without this tombstone
	// the operator would simply stop listing the role, leaving it valid in
	// Postgres after the API reports successful deletion (codex #8). Cleared by
	// the operator once CNPG confirms the role is absent.
	// +optional
	DeletedUsers []string `json:"deletedUsers,omitempty"`

	// Recovery, when set, provisions this Database by restoring another Database's
	// object-store backups to a point in time (PITR) into a NEW instance, instead
	// of initializing an empty database (CNPG bootstrap.recovery). Immutable after
	// first provision. Recovery requires the controller's backup store to be
	// configured. See docs/ADR009-postgresql-management.md.
	// +optional
	Recovery *DatabaseRecovery `json:"recovery,omitempty"`

	// HighAvailability, when true, provisions a replicated CNPG cluster (primary +
	// standby with pod anti-affinity) for this database. Render's
	// enableHighAvailability. Independent of ReadReplicas — a DB can have read
	// replicas without HA. See docs/render-artifacts/postgres-ha.md.
	// +optional
	HighAvailability bool `json:"highAvailability,omitempty"`

	// ReadReplicas declares named read-only replica endpoints backed by the CNPG
	// read-only service. Each entry yields its own internal host and, when Public
	// is true and BEX_DB_DOMAIN is set, its own external SNI hostname. Independent
	// of the HA toggle — Render's readReplicas: [{name}] field.
	// See docs/render-artifacts/postgres-ha.md.
	// +optional
	// +listType=map
	// +listMapKey=name
	ReadReplicas []DatabaseReadReplica `json:"readReplicas,omitempty"`

	// FailoverAt, when bumped to a fresh RFC3339 timestamp, requests a CNPG
	// switchover (promote a standby to primary). Only meaningful when
	// HighAvailability is true. Verb-as-timestamp pattern, like RestartedAt.
	// Maps to Render's POST /v1/postgres/{id}/failover.
	// +optional
	FailoverAt string `json:"failoverAt,omitempty"`

	// Exports is the append-only logical-export intent log. bex-api mints an
	// export id and appends a request; the operator projects each request to a
	// pg_dump Job and reports its lifecycle in status.exports. Keeping mechanism
	// out of bex-api preserves the operator -> types <- backend boundary.
	// +optional
	// +listType=map
	// +listMapKey=id
	Exports []DatabaseExportRequest `json:"exports,omitempty"`

	// Parameters are non-default PostgreSQL configuration parameters projected
	// to the CNPG Cluster's spec.postgresql.parameters (postgresql.conf).
	// Key is the parameter name (e.g. "log_min_duration_statement"), value is
	// the string setting. Changes take effect on the next CNPG reconcile;
	// parameters requiring a restart trigger a rolling restart.
	// shared_preload_libraries is always overridden to include pg_stat_statements.
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

// DatabaseReadReplica declares one named read-only replica endpoint for a Database.
// Each entry maps to a CNPG read-only service connection, addressable by its own
// SNI hostname when the Database is Public. Render's readReplicas item shape.
type DatabaseReadReplica struct {
	// Name is the replica's label, used to build its external SNI hostname
	// (<db>-<name>.<BEX_DB_DOMAIN>) and in the status readReplicaStatuses array.
	// +required
	Name string `json:"name"`
}

// DatabaseUser is an additional managed PostgreSQL login role on a Database.
type DatabaseUser struct {
	// Name is the role name (a valid unquoted PostgreSQL identifier).
	// +required
	Name string `json:"name"`

	// SecretName references a Secret in the Database's namespace whose "password"
	// key holds the role's password (a CNPG basic-auth Secret referenced by
	// spec.managed.roles[].passwordSecret). bex-api creates it as
	// "<db>-user-<name>" with a generated password on user creation.
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// DatabaseRecovery restores a new Database from a source Database's backups.
type DatabaseRecovery struct {
	// SourceDatabase is the name of the Database whose object-store backups to
	// restore from (its CNPG serverName in the shared destination path).
	// +required
	SourceDatabase string `json:"sourceDatabase"`

	// SourceBackupServerName is the exact CNPG archive generation to restore.
	// Major upgrades move WAL/base backups to a new per-major serverName; keeping
	// the resolved value in recovery intent avoids guessing whether a source is
	// still on its legacy unsuffixed archive.
	// +optional
	SourceBackupServerName string `json:"sourceBackupServerName,omitempty"`

	// TargetTime is the RFC3339 point in time to recover to (PITR). Empty =>
	// recover to the latest available point (the end of the WAL stream).
	// +optional
	TargetTime string `json:"targetTime,omitempty"`
}

// DatabaseExportRequest is one logical pg_dump requested through bex-api.
// The id is minted by lego/backend/internal/id and is safe as a Kubernetes
// label/name component. RequestedAt is RFC3339 and anchors artifact retention.
type DatabaseExportRequest struct {
	// ID is the opaque exp-... export id.
	// +required
	ID string `json:"id"`

	// RequestedAt is when bex-api accepted the request (RFC3339).
	// +required
	RequestedAt string `json:"requestedAt"`
}

// DatabaseExportPhase is the honest lifecycle of a logical export.
// +kubebuilder:validation:Enum=created;running;available;failed;expiring;expired
type DatabaseExportPhase string

const (
	DatabaseExportCreated   DatabaseExportPhase = "created"
	DatabaseExportRunning   DatabaseExportPhase = "running"
	DatabaseExportAvailable DatabaseExportPhase = "available"
	DatabaseExportFailed    DatabaseExportPhase = "failed"
	DatabaseExportExpiring  DatabaseExportPhase = "expiring"
	DatabaseExportExpired   DatabaseExportPhase = "expired"
)

// DatabaseExportStatus is the operator-observed state for one request. The
// artifact key and object-store coordinates are control-plane data only; API
// adapters expose a freshly presigned URL, never the credential Secret.
type DatabaseExportStatus struct {
	// ID matches DatabaseExportRequest.ID.
	// +required
	ID string `json:"id"`

	// Phase is created, running, available, failed, expiring, or expired.
	// +required
	Phase DatabaseExportPhase `json:"phase"`

	// CreatedAt is the accepted request time (RFC3339).
	// +optional
	CreatedAt string `json:"createdAt,omitempty"`

	// StartedAt is when Kubernetes first reported the export Job active.
	// +optional
	StartedAt string `json:"startedAt,omitempty"`

	// CompletedAt is when the artifact became available or the Job failed.
	// +optional
	CompletedAt string `json:"completedAt,omitempty"`

	// ExpiresAt is the seven-day artifact-retention deadline (RFC3339).
	// +optional
	ExpiresAt string `json:"expiresAt,omitempty"`

	// ObjectKey is the full s3:// bucket/key written by the export Job.
	// +optional
	ObjectKey string `json:"objectKey,omitempty"`

	// Filename is the browser download name (Render-compatible .dir.tar.gz).
	// +optional
	Filename string `json:"filename,omitempty"`

	// FailureReason is populated for failed export or expiry Jobs.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`
}

// DatabasePhase mirrors the provisioning and major-upgrade lifecycle.
// +kubebuilder:validation:Enum=Pending;Provisioning;Upgrading;Ready;Failed
type DatabasePhase string

const (
	DBPhasePending      DatabasePhase = "Pending"
	DBPhaseProvisioning DatabasePhase = "Provisioning"
	DBPhaseUpgrading    DatabasePhase = "Upgrading"
	DBPhaseReady        DatabasePhase = "Ready"
	DBPhaseFailed       DatabasePhase = "Failed"
)

// DatabaseStatus is the observed state of a Database.
type DatabaseStatus struct {
	// Phase is the high-level lifecycle state.
	// +optional
	Phase DatabasePhase `json:"phase,omitempty"`

	// AllocatedStorageGB is the grow-only storage high-water mark accepted by
	// the operator. It is intentionally status-only: API adapters use it to
	// reject an explicit shrink before persisting intent, while the operator
	// still verifies the live CNPG Cluster before every projection.
	// +optional
	AllocatedStorageGB int32 `json:"allocatedStorageGB,omitempty"`

	// ObservedStorageGB is the spec.storageGB value associated with the accepted
	// high-water mark. It lets the controller distinguish a new explicit shrink
	// request from an unchanged below-plan legacy value during plan changes.
	// +optional
	ObservedStorageGB int32 `json:"observedStorageGB,omitempty"`

	// CurrentVersion is the PostgreSQL major version reported by CNPG for the
	// active PGDATA image. During a major upgrade this remains the source version
	// until pg_upgrade succeeds, while spec.version already holds the target.
	// +optional
	CurrentVersion string `json:"currentVersion,omitempty"`

	// DiskResizeHistory is a bounded, newest-last audit trail of automatic
	// storage growth. The operator is the sole writer; credentials and other
	// sensitive values never enter it.
	// +optional
	DiskResizeHistory []DatabaseDiskResizeStatus `json:"diskResizeHistory,omitempty"`

	// Host is the in-cluster read-write hostname (CNPG "<cluster>-rw" Service).
	// +optional
	Host string `json:"host,omitempty"`

	// Port the database listens on.
	// +optional
	Port int32 `json:"port,omitempty"`

	// SecretName is the CNPG-generated Secret holding the app credentials and the
	// ready-made connection URI (keys: username, password, dbname, host, port, uri).
	// The internal Database URL is that Secret's "uri" — the operator never copies
	// the password into status.
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// ExternalHost is the public SNI hostname when Public is set (empty otherwise).
	// The external URL is postgresql://<user>:<pass>@<ExternalHost>:5432/<db>?sslmode=verify-full
	// (credentials from SecretName). DNS for the host must point at the edge.
	// +optional
	ExternalHost string `json:"externalHost,omitempty"`

	// PoolerHost is the in-cluster PgBouncer service host ("<name>-pooler.<ns>.svc")
	// when Pooler is enabled (empty otherwise). The pooled internal URL routes
	// through it.
	// +optional
	PoolerHost string `json:"poolerHost,omitempty"`

	// PoolerExternalHost is the public SNI hostname for the pooled endpoint when
	// both Pooler and Public are set (empty otherwise).
	// +optional
	PoolerExternalHost string `json:"poolerExternalHost,omitempty"`

	// BackupsEnabled is true when the controller projected the Barman Cloud WAL
	// archiver plugin plus a plugin-method ScheduledBackup for this database (the
	// plan opts in and the backup store is configured) — the signal that
	// recovery/PITR is available.
	// +optional
	BackupsEnabled bool `json:"backupsEnabled,omitempty"`

	// BackupServerName is the Barman archive generation currently receiving WAL
	// and base backups. The first major uses the Database name; each
	// successful major upgrade moves to <name>-pg<major> to avoid timeline/system
	// ID collisions across pg_upgrade boundaries.
	// +optional
	BackupServerName string `json:"backupServerName,omitempty"`

	// BackupEndpointURL and BackupS3SecretName are the non-secret coordinates
	// bex-api needs to presign an available logical export. Credentials remain in
	// the named Secret and are read only after can_view_sensitive authorization.
	// +optional
	BackupEndpointURL string `json:"backupEndpointURL,omitempty"`

	// +optional
	BackupS3SecretName string `json:"backupS3SecretName,omitempty"`

	// Exports reports the lifecycle and artifact identity for requested logical
	// pg_dump exports. The operator is the sole writer.
	// +optional
	// +listType=map
	// +listMapKey=id
	Exports []DatabaseExportStatus `json:"exports,omitempty"`

	// HighAvailabilityEnabled reports whether a replicated CNPG cluster (≥2 ready
	// instances) is active. Render's highAvailabilityEnabled read field; reflects
	// the cluster's real state, not just spec.highAvailability.
	// +optional
	HighAvailabilityEnabled bool `json:"highAvailabilityEnabled,omitempty"`

	// ReadReplicaStatuses has one entry per spec.readReplicas entry with the
	// resolved internal + external connection hosts for that named replica.
	// +optional
	// +listType=map
	// +listMapKey=name
	ReadReplicaStatuses []DatabaseReadReplicaStatus `json:"readReplicaStatuses,omitempty"`

	// CurrentPrimary is the CNPG pod name currently serving as primary, surfaced
	// as an observability aid after a failover. Empty until HA is active.
	// bex extension (Render reports HA state but not the pod name).
	// +optional
	CurrentPrimary string `json:"currentPrimary,omitempty"`

	// LastFailoverAt records the spec.failoverAt value the controller last acted
	// on, so the controller only triggers a switchover once per request.
	// +optional
	LastFailoverAt string `json:"lastFailoverAt,omitempty"`

	// ObservedGeneration is the .metadata.generation the controller last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the current state (Ready).
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// DatabaseDiskResizeStatus records one operator-triggered, grow-only storage
// resize. UsedBytes and CapacityBytes are the kubelet volume-stat sample that
// crossed the threshold; FromGB/ToGB are the persisted Database intent.
type DatabaseDiskResizeStatus struct {
	// At is the RFC3339 time the operator accepted the resize.
	// +required
	At string `json:"at"`

	// +required
	FromGB int32 `json:"fromGB"`

	// +required
	ToGB int32 `json:"toGB"`

	// +optional
	UsedBytes int64 `json:"usedBytes,omitempty"`

	// +optional
	CapacityBytes int64 `json:"capacityBytes,omitempty"`
}

// DatabaseReadReplicaStatus reports the resolved connection URLs for one named
// read replica declared in spec.readReplicas.
type DatabaseReadReplicaStatus struct {
	// Name matches the spec.readReplicas entry name.
	// +required
	Name string `json:"name"`
	// InternalHost is the in-cluster read-only hostname (CNPG "<cluster>-ro" Service,
	// load-balances across standbys). Shared across all named replicas of the same DB.
	// +optional
	InternalHost string `json:"internalHost,omitempty"`
	// ExternalHost is the per-replica public SNI hostname
	// ("<db>-<name>.<BEX_DB_DOMAIN>") when the Database is Public.
	// +optional
	ExternalHost string `json:"externalHost,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Plan",type=string,JSONPath=`.spec.plan`
// +kubebuilder:printcolumn:name="Host",type=string,JSONPath=`.status.host`,priority=1

// Database is a managed PostgreSQL instance (projected to a CloudNativePG Cluster).
type Database struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec DatabaseSpec `json:"spec"`
	// +optional
	Status DatabaseStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// DatabaseList contains a list of Database.
type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Database `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Database{}, &DatabaseList{})
}
