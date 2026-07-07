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
// docs/postgresql-management.md.
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
	// External connections use sslmode=require. See docs/postgresql-management.md.
	// +optional
	Public bool `json:"public,omitempty"`
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
