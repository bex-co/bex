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
	"k8s.io/apimachinery/pkg/util/validation"
)

// ValidKeyValueName reports whether name is a valid user-facing managed
// key-value name. Keeping this next to KeyValueSpec.Name makes the CRD contract
// and every API/Blueprint writer share one validation rule — the same shape
// ValidDatabaseName enforces for managed Postgres (w9/m6, mirroring w9/m3).
func ValidKeyValueName(name string) bool {
	return len(name) <= 30 && len(validation.IsDNS1123Label(name)) == 0
}

// KeyValueSpec is the desired state of a managed Valkey (Redis-compatible)
// key-value store — the Render-style "add a Key Value" unit. The operator
// projects it to a single-instance Valkey StatefulSet in the same namespace; the
// plan sets resources/storage. See docs/ADR021-keyvalue-management.md.
type KeyValueSpec struct {
	// Name is the mutable, user-facing key-value name. metadata.name is the
	// immutable red-... resource id and the data-plane identity used for every
	// StatefulSet/PVC/Secret/route name. Empty is the legacy representation:
	// readers fall back to metadata.name until the backfill sets this field
	// (w9/m6, mirroring Postgres's spec.name split from w9/m3).
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=30
	// +kubebuilder:validation:Pattern=`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`
	Name string `json:"name,omitempty"`

	// Plan selects the resource allocation (compute + storage). MVP plans are
	// single-instance and fit one node. Names follow Render's Key Value product
	// (free / starter / standard — the web-service vocabulary, not Postgres).
	// +optional
	// +kubebuilder:validation:Enum=free;starter;standard
	// +kubebuilder:default=free
	Plan string `json:"plan,omitempty"`

	// Version is the major Valkey version (e.g. "8"). Empty => operator default.
	// +optional
	// +kubebuilder:validation:Enum="7";"8"
	Version string `json:"version,omitempty"`

	// StorageGB overrides the plan's default volume size (GB). Storage can only
	// grow, never shrink. 0 => plan default.
	// +optional
	StorageGB int32 `json:"storageGB,omitempty"`

	// MaxmemoryPolicy is Valkey's key-eviction policy once the store reaches its
	// memory budget — Render's "Maxmemory Policy". The operator sets `maxmemory`
	// to a fraction of the plan's RAM and applies this policy; empty preserves the
	// prior behavior (no maxmemory limit set). Default: allkeys-lru (Render's
	// cache-oriented default). See docs/ADR021-keyvalue-management.md.
	// +optional
	// +kubebuilder:validation:Enum=noeviction;allkeys-lru;allkeys-lfu;volatile-lru;volatile-lfu;allkeys-random;volatile-random;volatile-ttl
	// +kubebuilder:default=allkeys-lru
	MaxmemoryPolicy string `json:"maxmemoryPolicy,omitempty"`

	// PersistenceMode selects how Valkey persists to the PVC — Render's
	// "Persistence Mode": journal-snapshot (AOF + RDB snapshots, the prior
	// default), snapshot (RDB snapshots only), or off (no persistence). Empty
	// preserves the prior behavior (appendonly yes). Default: journal-snapshot.
	// See docs/ADR021-keyvalue-management.md.
	// +optional
	// +kubebuilder:validation:Enum=journal-snapshot;snapshot;off
	// +kubebuilder:default=journal-snapshot
	PersistenceMode string `json:"persistenceMode,omitempty"`

	// Public, when true and the controller's BEX_KV_DOMAIN is set, exposes the
	// store at "<name>.<BEX_KV_DOMAIN>" through the shared metered SNI proxy (TLS
	// passthrough — Valkey terminates its own TLS for direct-TLS clients). Default:
	// in-cluster only. See docs/ADR021-keyvalue-management.md.
	// +optional
	Public bool `json:"public,omitempty"`

	// Suspended scales the Valkey StatefulSet to zero replicas while preserving
	// its PVC (data survives) — Render's Key Value suspend/resume. The credentials
	// Secret and any external route are kept, so resume restores the same
	// password and endpoint. Default: running.
	// +optional
	Suspended bool `json:"suspended,omitempty"`

	// IPAllowList restricts the EXTERNAL (public) endpoint to these CIDRs in the
	// shared SNI proxy. Empty => the external endpoint is open to all source IPs.
	// The internal path is never affected.
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
}

// KeyValuePhase mirrors the provisioning lifecycle.
// +kubebuilder:validation:Enum=Pending;Provisioning;Ready;Failed
type KeyValuePhase string

const (
	KVPhasePending      KeyValuePhase = "Pending"
	KVPhaseProvisioning KeyValuePhase = "Provisioning"
	KVPhaseReady        KeyValuePhase = "Ready"
	KVPhaseFailed       KeyValuePhase = "Failed"
)

// KeyValueStatus is the observed state of a KeyValue.
type KeyValueStatus struct {
	// Phase is the high-level lifecycle state.
	// +optional
	Phase KeyValuePhase `json:"phase,omitempty"`

	// AllocatedStorageGB is the grow-only PVC request accepted by the operator.
	// It remains status-only and is not a separate public API surface.
	// +optional
	AllocatedStorageGB int32 `json:"allocatedStorageGB,omitempty"`

	// ObservedStorageGB is the spec.storageGB value last accepted by the
	// grow-only reconciler, distinguishing a new shrink from an unchanged legacy
	// value that was already raised to its plan floor.
	// +optional
	ObservedStorageGB int32 `json:"observedStorageGB,omitempty"`

	// StorageCapacityGB is the capacity most recently observed on the Valkey
	// PVC. While it trails AllocatedStorageGB, Ready remains false and the
	// StorageReady condition explains the in-progress filesystem resize.
	// +optional
	StorageCapacityGB int32 `json:"storageCapacityGB,omitempty"`

	// Host is the in-cluster hostname (the "<name>" ClusterIP Service).
	// +optional
	Host string `json:"host,omitempty"`

	// Port the Valkey instance listens on.
	// +optional
	Port int32 `json:"port,omitempty"`

	// SecretName is the operator-generated Secret holding the connection
	// credentials and the ready-made connection URI (keys: username, password,
	// host, port, uri, and externalUri when public). The operator never copies the
	// password into status.
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// ExternalHost is the public SNI hostname when Public is set (empty otherwise).
	// The external URL is rediss://:<password>@<ExternalHost>:6379 (credentials
	// from SecretName). DNS for the host must point at the edge.
	// +optional
	ExternalHost string `json:"externalHost,omitempty"`

	// ObservedGeneration is the .metadata.generation the controller last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the current state (Ready).
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Plan",type=string,JSONPath=`.spec.plan`
// +kubebuilder:printcolumn:name="Host",type=string,JSONPath=`.status.host`,priority=1

// KeyValue is a managed Valkey (Redis-compatible) key-value store.
type KeyValue struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec KeyValueSpec `json:"spec"`
	// +optional
	Status KeyValueStatus `json:"status,omitzero"`
}

// DisplayName returns the mutable user-facing name. KeyValue CRs created before
// spec.name existed keep working by falling back to their metadata.name until
// the migration backfills the field (mirrors Database.DisplayName, w9/m3).
func (kv *KeyValue) DisplayName() string {
	if kv.Spec.Name != "" {
		return kv.Spec.Name
	}
	return kv.Name
}

// +kubebuilder:object:root=true

// KeyValueList contains a list of KeyValue.
type KeyValueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KeyValue `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KeyValue{}, &KeyValueList{})
}
