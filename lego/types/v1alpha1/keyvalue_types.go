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

// KeyValueSpec is the desired state of a managed Valkey (Redis-compatible)
// key-value store — the Render-style "add a Key Value" unit. The operator
// projects it to a single-instance Valkey StatefulSet in the same namespace; the
// plan sets resources/storage. See docs/ADR021-keyvalue-management.md.
type KeyValueSpec struct {
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
	// store at "<name>.<BEX_KV_DOMAIN>" via a Traefik TCP/SNI route (TLS
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

	// IPAllowList restricts the EXTERNAL (public) endpoint to these CIDRs via a
	// Traefik TCP ipAllowList middleware on the SNI route. Empty => the external
	// route is open to all source IPs. The internal path is never affected.
	// Render's ipAllowList; only meaningful when Public.
	// +optional
	IPAllowList []string `json:"ipAllowList,omitempty"`
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
