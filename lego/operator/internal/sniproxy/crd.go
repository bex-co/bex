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

// Package sniproxy is what kv-sni-proxy and pg-sni-proxy share: the PROXY
// protocol and TLS ClientHello reads, admission, metering, the drain, and the
// routing views of the KeyValue and Database CRDs. The views are no API, so
// controller-gen renders no CRD from this package.
// +kubebuilder:skip
package sniproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
)

// The SNI proxies watch KeyValue and Database through the views below, never
// through lego/types/v1alpha1. Each view decodes only the fields its proxy
// routes by, and the decoder drops the rest of what the API server sends. A
// binary changes with any source edit in a package it links, code it never
// calls and comments included, because the Go build ID hashes every file. So
// only leaving the CRD package unlinked keeps /kv-sni-proxy and /pg-sni-proxy
// byte-identical across a CRD edit: the deploy then keeps both pins, and
// neither DaemonSet rolls or closes an external client (w6/150, ADR021 § 10).
// TestTheViewsReadTheCRDContract pins every field here to the CRD's, so a
// routed field that changes there fails until it changes here, and that change
// then rolls both proxies, as it must.
// TestACRDFieldEditLeavesBothProxiesByteIdentical keeps v1alpha1 out of both.

// GroupVersion is the CRDs' group and version, v1alpha1.SchemeGroupVersion.
var GroupVersion = schema.GroupVersion{Group: "app.bex.co", Version: "v1alpha1"}

// AddToScheme registers both views under the CRDs' kinds.
func AddToScheme(s *runtime.Scheme) error {
	s.AddKnownTypes(GroupVersion, &KeyValue{}, &KeyValueList{}, &Database{}, &DatabaseList{})
	metav1.AddToGroupVersion(s, GroupVersion)
	return nil
}

// IPAllowEntry is one entry of a datastore's own allowlist. Its description
// rides along in the CRD, and enforcement never reads it.
type IPAllowEntry struct {
	CIDR string `json:"cidr"`
}

// KeyValue is a managed Key Value store as kv-sni-proxy routes it.
type KeyValue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec   KeyValueSpec   `json:"spec"`
	Status KeyValueStatus `json:"status,omitzero"`
}

// KeyValueSpec holds the fields of the store's spec the route reads.
type KeyValueSpec struct {
	Plan                   string         `json:"plan,omitempty"`
	Public                 bool           `json:"public,omitempty"`
	IPAllowList            []IPAllowEntry `json:"ipAllowList,omitempty"`
	EnvironmentIPAllowList []string       `json:"environmentIPAllowList,omitempty"`
	WorkspaceIPAllowList   []string       `json:"workspaceIPAllowList,omitempty"`
}

// KeyValueStatus holds the public host the operator has published; a store
// routes only once it equals the proxy's own spelling.
type KeyValueStatus struct {
	ExternalHost string `json:"externalHost,omitempty"`
}

// KeyValueList is the list the KeyValue watch reads.
type KeyValueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []KeyValue `json:"items"`
}

// Database is a managed Postgres database as pg-sni-proxy routes it.
type Database struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec DatabaseSpec `json:"spec"`
}

// DatabaseSpec holds the fields of the database's spec the routes read.
type DatabaseSpec struct {
	Plan                   string                `json:"plan,omitempty"`
	Public                 bool                  `json:"public,omitempty"`
	Pooler                 bool                  `json:"pooler,omitempty"`
	ReadReplicas           []DatabaseReadReplica `json:"readReplicas,omitempty"`
	IPAllowList            []IPAllowEntry        `json:"ipAllowList,omitempty"`
	EnvironmentIPAllowList []string              `json:"environmentIPAllowList,omitempty"`
	WorkspaceIPAllowList   []string              `json:"workspaceIPAllowList,omitempty"`
}

// DatabaseReadReplica is one named read replica, routed under its own labels.
type DatabaseReadReplica struct {
	Name string `json:"name"`
}

// DatabaseList is the list the Database watch reads.
type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Database `json:"items"`
}

// ReadReplicaServiceName is the public SNI label of a replica's instance,
// "<db>-ro-<name>", and the Service it routes to: the operator's
// v1alpha1.ReadReplicaServiceName, which TestTheReplicaLabelsAreTheOperators
// pins.
func ReadReplicaServiceName(db, replica string) string {
	return readReplicaLabel(db, "ro", replica)
}

// ReadReplicaPoolerName is the public SNI label of a replica's PgBouncer,
// "<db>-rp-<name>", and its Service: v1alpha1.ReadReplicaPoolerName.
func ReadReplicaPoolerName(db, replica string) string {
	return readReplicaLabel(db, "rp", replica)
}

// readReplicaLabel is "<db>-<kind>-<name>", or the first 10 hex of the name's
// sha256 in place of a name that cannot form a DNS label (one admitted before
// 2026-10-19 keeps its name).
func readReplicaLabel(db, kind, replica string) string {
	if label := db + "-" + kind + "-" + replica; len(validation.IsDNS1035Label(label)) == 0 {
		return label
	}
	sum := sha256.Sum256([]byte(replica))
	return db + "-" + kind + "-" + hex.EncodeToString(sum[:5])
}

// DeepCopyObject implements runtime.Object.
func (in *KeyValue) DeepCopyObject() runtime.Object {
	out := in.deepCopy()
	return &out
}

func (in *KeyValue) deepCopy() KeyValue {
	out := *in
	(*metav1.ObjectMeta).DeepCopyInto(&in.ObjectMeta, &out.ObjectMeta)
	out.Spec.IPAllowList = slices.Clone(in.Spec.IPAllowList)
	out.Spec.EnvironmentIPAllowList = slices.Clone(in.Spec.EnvironmentIPAllowList)
	out.Spec.WorkspaceIPAllowList = slices.Clone(in.Spec.WorkspaceIPAllowList)
	return out
}

// DeepCopyObject implements runtime.Object.
func (in *KeyValueList) DeepCopyObject() runtime.Object {
	out := *in
	(*metav1.ListMeta).DeepCopyInto(&in.ListMeta, &out.ListMeta)
	out.Items = slices.Clone(in.Items)
	for i := range out.Items {
		out.Items[i] = in.Items[i].deepCopy()
	}
	return &out
}

// DeepCopyObject implements runtime.Object.
func (in *Database) DeepCopyObject() runtime.Object {
	out := in.deepCopy()
	return &out
}

func (in *Database) deepCopy() Database {
	out := *in
	(*metav1.ObjectMeta).DeepCopyInto(&in.ObjectMeta, &out.ObjectMeta)
	out.Spec.ReadReplicas = slices.Clone(in.Spec.ReadReplicas)
	out.Spec.IPAllowList = slices.Clone(in.Spec.IPAllowList)
	out.Spec.EnvironmentIPAllowList = slices.Clone(in.Spec.EnvironmentIPAllowList)
	out.Spec.WorkspaceIPAllowList = slices.Clone(in.Spec.WorkspaceIPAllowList)
	return out
}

// DeepCopyObject implements runtime.Object.
func (in *DatabaseList) DeepCopyObject() runtime.Object {
	out := *in
	(*metav1.ListMeta).DeepCopyInto(&in.ListMeta, &out.ListMeta)
	out.Items = slices.Clone(in.Items)
	for i := range out.Items {
		out.Items[i] = in.Items[i].deepCopy()
	}
	return &out
}
