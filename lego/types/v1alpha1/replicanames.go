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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/util/validation"
)

// Each named read replica of a Database is its own one-instance CNPG replica
// cluster, with its own Service and its own PgBouncer, as each Render replica
// has its own instance, URLs and pool (w6/m467, ADR009 § Per-replica readers
// and pools). Both sides of the Database CR boundary derive these names: the
// operator creates the objects, pg-sni-proxy routes to them, and bex-api finds
// a replica's Pods, logs and metrics by its cluster name.

const (
	readReplicaClusterInfix = "-r-"
	// nameDigestBytes of sha256 name a replica cluster (and a logical
	// subscription's objects): 10 hex.
	nameDigestBytes = 5
)

// ReadReplicaClusterName is a replica's CNPG Cluster, "<db>-r-<the first 10 hex
// of sha256(name)>". CNPG caps a Cluster name at 50 characters and the replica
// id "<db>-ro-<name>" reaches 63, so the cluster carries a digest of the name.
// Its Pods are "<cluster>-<n>".
func ReadReplicaClusterName(db, replica string) string {
	return db + readReplicaClusterInfix + nameDigest(replica)
}

// DatabaseClustersPattern is an unanchored regexp (RE2, as PromQL and LogQL
// read it) for every CNPG cluster of a Database: the primary's, named after
// it, and each read replica's, removed ones included, so a replica's history
// stays.
func DatabaseClustersPattern(db string) string {
	return fmt.Sprintf("%s(%s[0-9a-f]{%d})?", regexp.QuoteMeta(db), readReplicaClusterInfix, 2*nameDigestBytes)
}

// ReadReplicaServiceName is a replica's direct Service, "<db>-ro-<name>": its
// internal host, and its public SNI label under BEX_DB_DOMAIN.
func ReadReplicaServiceName(db, replica string) string {
	return readReplicaLabel(db, "ro", replica)
}

// ReadReplicaPoolerName is a replica's PgBouncer, the CNPG Pooler and its
// Service "<db>-rp-<name>": its pooled internal host, and its pooled public SNI
// label. "-rp-" is as long as "-ro-", so every name admission accepts fits one
// DNS label.
func ReadReplicaPoolerName(db, replica string) string {
	return readReplicaLabel(db, "rp", replica)
}

// readReplicaLabel is "<db>-<kind>-<name>", or the name's digest in place of a
// name that cannot form a DNS label: admission has refused one since
// 2026-10-19 (w5/m138), but a replica declared before keeps its name.
func readReplicaLabel(db, kind, replica string) string {
	if label := db + "-" + kind + "-" + replica; len(validation.IsDNS1035Label(label)) == 0 {
		return label
	}
	return db + "-" + kind + "-" + nameDigest(replica)
}

func nameDigest(replica string) string {
	sum := sha256.Sum256([]byte(replica))
	return hex.EncodeToString(sum[:nameDigestBytes])
}
