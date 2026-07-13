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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseSpec is the desired state of a managed PostgreSQL — the Render-style
// "add a Postgres" unit. The operator projects it to a CloudNativePG Cluster in
// the same namespace; the plan sets resources/storage. See
// docs/ADR009-postgresql-management.md.
type DatabaseSpec struct {
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

	// Public, when true and the controller's BEX_DB_DOMAIN is set, exposes the
	// database at "<name>.<BEX_DB_DOMAIN>" via a Traefik TCP/SNI route (TLS
	// passthrough — Postgres terminates its own TLS). Default: in-cluster only.
	// External connections use sslmode=require. See docs/ADR009-postgresql-management.md.
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

	// IPAllowList restricts the EXTERNAL (public) endpoint to these CIDRs via a
	// Traefik TCP ipAllowList middleware on the SNI route. Empty => the external
	// route is open to all source IPs. The internal "-rw" path is never affected.
	// Render's ipAllowList; only meaningful when Public.
	// +optional
	IPAllowList []string `json:"ipAllowList,omitempty"`

	// Pooler, when true, provisions a PgBouncer connection pooler (a CNPG Pooler
	// in transaction mode) for this database; the connection info then surfaces
	// pooled connection strings. Render's connection pooling.
	// +optional
	Pooler bool `json:"pooler,omitempty"`

	// Users are additional managed PostgreSQL login roles, projected to the CNPG
	// cluster's spec.managed.roles. The owner role (<db>_user) is provisioned by
	// CNPG's bootstrap and is not listed here. Each user's password lives in the
	// referenced Secret's "password" key (created by bex-api, never in status).
	// +optional
	// +listType=map
	// +listMapKey=name
	Users []DatabaseUser `json:"users,omitempty"`

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

	// TargetTime is the RFC3339 point in time to recover to (PITR). Empty =>
	// recover to the latest available point (the end of the WAL stream).
	// +optional
	TargetTime string `json:"targetTime,omitempty"`
}

// DatabasePhase mirrors the provisioning lifecycle.
// +kubebuilder:validation:Enum=Pending;Provisioning;Ready;Failed
type DatabasePhase string

const (
	DBPhasePending      DatabasePhase = "Pending"
	DBPhaseProvisioning DatabasePhase = "Provisioning"
	DBPhaseReady        DatabasePhase = "Ready"
	DBPhaseFailed       DatabasePhase = "Failed"
)

// DatabaseStatus is the observed state of a Database.
type DatabaseStatus struct {
	// Phase is the high-level lifecycle state.
	// +optional
	Phase DatabasePhase `json:"phase,omitempty"`

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
	// The external URL is postgresql://<user>:<pass>@<ExternalHost>:5432/<db>?sslmode=require
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

	// BackupsEnabled is true when the controller projected a barmanObjectStore +
	// ScheduledBackup for this database (the plan opts in and the backup store is
	// configured) — the signal that recovery/PITR is available.
	// +optional
	BackupsEnabled bool `json:"backupsEnabled,omitempty"`

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
