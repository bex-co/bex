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
	"regexp"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation"
)

// A Database id is "dpg-" plus a 20-character xid.
const replicaTestDB = "dpg-d4abcdefghijklmnopqr"

// TestReadReplicaNames pins the per-replica object names. They are on the
// wire: tenants copy the Service and Pooler names as hosts, pg-sni-proxy routes
// them, and bex-api selects a replica's Pods by its cluster name.
func TestReadReplicaNames(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{ReadReplicaClusterName(replicaTestDB, "a"), replicaTestDB + "-r-ca978112ca"},
		{ReadReplicaClusterName(replicaTestDB, "b"), replicaTestDB + "-r-3e23e81600"},
		{ReadReplicaServiceName(replicaTestDB, "a"), replicaTestDB + "-ro-a"},
		{ReadReplicaPoolerName(replicaTestDB, "a"), replicaTestDB + "-rp-a"},
	} {
		if tc.got != tc.want {
			t.Errorf("got %q, want %q", tc.got, tc.want)
		}
	}
}

// TestReadReplicaNamesFitTheirLimits: the longest name admission accepts
// (63 minus "<id>-ro-") keeps its Service and Pooler within one DNS label, and
// every cluster within CNPG's 50 characters (its validateName).
func TestReadReplicaNamesFitTheirLimits(t *testing.T) {
	longest := strings.Repeat("x", validation.DNS1035LabelMaxLength-len(replicaTestDB+"-ro-"))
	for _, name := range []string{
		ReadReplicaServiceName(replicaTestDB, longest),
		ReadReplicaPoolerName(replicaTestDB, longest),
	} {
		if errs := validation.IsDNS1035Label(name); len(errs) != 0 {
			t.Errorf("%q: %v", name, errs)
		}
		if !strings.HasSuffix(name, longest) {
			t.Errorf("%q does not carry the replica's name", name)
		}
	}
	cluster := ReadReplicaClusterName(replicaTestDB, longest)
	if len(cluster) > 50 || len(validation.IsDNS1035Label(cluster)) != 0 {
		t.Errorf("cluster %q (%d characters) is not a CNPG cluster name", cluster, len(cluster))
	}
}

// TestReadReplicaNamesOfALegacyName: a name admitted before the DNS-label rule
// still gets valid, distinct object names, from its digest.
func TestReadReplicaNamesOfALegacyName(t *testing.T) {
	for _, legacy := range []string{"Reports_EU", strings.Repeat("x", 40)} {
		service, pooler := ReadReplicaServiceName(replicaTestDB, legacy), ReadReplicaPoolerName(replicaTestDB, legacy)
		for _, name := range []string{service, pooler} {
			if errs := validation.IsDNS1035Label(name); len(errs) != 0 {
				t.Errorf("%q from %q: %v", name, legacy, errs)
			}
		}
		if service == pooler {
			t.Errorf("%q: service and pooler share %q", legacy, service)
		}
	}
	if ReadReplicaServiceName(replicaTestDB, "Reports_EU") == ReadReplicaServiceName(replicaTestDB, "reports_eu") {
		t.Error("two legacy names share a Service")
	}
}

// TestDatabaseClustersPattern: the pattern bex-api queries logs and metrics
// with matches the primary's cluster and every replica cluster, and no other
// object of the Database.
func TestDatabaseClustersPattern(t *testing.T) {
	re := regexp.MustCompile("^" + DatabaseClustersPattern(replicaTestDB) + "$")
	for _, name := range []string{replicaTestDB, ReadReplicaClusterName(replicaTestDB, "a"), ReadReplicaClusterName(replicaTestDB, "Reports_EU")} {
		if !re.MatchString(name) {
			t.Errorf("%q does not match", name)
		}
	}
	for _, name := range []string{
		replicaTestDB + "-ro-a", replicaTestDB + "-rp-a", replicaTestDB + "-pooler", replicaTestDB + "-r-ca978112ca-rw",
		strings.Replace(replicaTestDB, "-", ".", 1),
	} {
		if re.MatchString(name) {
			t.Errorf("%q matches", name)
		}
	}
}
