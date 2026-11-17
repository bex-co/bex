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

package postgres

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// tenantObjectStoreChart is the tenant Postgres ObjectStore every tenant
// namespace receives a copy of (ADR031): its retentionPolicy is how far back
// the stored base backups and WAL reach.
const tenantObjectStoreChart = "deploy/gitops/charts/barman-cloud-objectstores/tenant-postgres.yaml"

// barmanRetention is the plugin's retentionPolicy grammar: a recovery window
// of n days, weeks or months.
var barmanRetention = regexp.MustCompile(`^([1-9][0-9]*)([dwm])$`)

// storedRetentionCovers fails when policy keeps less than the longest plan's
// recovery window (core.LongestPITRRetention): every point a Pro workspace is
// offered must still be restorable. A month counts as its shortest, 28 days.
func storedRetentionCovers(policy string) error {
	m := barmanRetention.FindStringSubmatch(policy)
	if m == nil {
		return fmt.Errorf("retentionPolicy %q is not <n>d, <n>w or <n>m", policy)
	}
	n, _ := strconv.Atoi(m[1])
	unit := map[string]time.Duration{"d": 24 * time.Hour, "w": 7 * 24 * time.Hour, "m": 28 * 24 * time.Hour}[m[2]]
	if kept := time.Duration(n) * unit; kept < core.LongestPITRRetention() {
		return fmt.Errorf("retentionPolicy %s keeps %s, shorter than the longest plan's recovery window %s", policy, kept, core.LongestPITRRetention())
	}
	return nil
}

// w7/m453 t003: bex-api offers up to core.LongestPITRRetention back (7 days on
// Pro and higher), so the tenant backup store must keep at least that. The
// chart keeps 30 days (ADR009 § Backups + point-in-time recovery records why);
// a later cut below the longest window would quietly break the Pro window.
func TestTenantBackupStoreKeepsTheLongestRecoveryWindow(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", filepath.FromSlash(tenantObjectStoreChart)))
	if err != nil {
		t.Fatal(err)
	}
	var store struct {
		Kind string `json:"kind"`
		Spec struct {
			RetentionPolicy string `json:"retentionPolicy"`
		} `json:"spec"`
	}
	if err := yaml.Unmarshal(raw, &store); err != nil {
		t.Fatalf("parse %s: %v", tenantObjectStoreChart, err)
	}
	if store.Kind != "ObjectStore" {
		t.Fatalf("%s holds a %q, want the ObjectStore", tenantObjectStoreChart, store.Kind)
	}
	if err := storedRetentionCovers(store.Spec.RetentionPolicy); err != nil {
		t.Fatalf("%s: %v", tenantObjectStoreChart, err)
	}
}

// The guard reads Barman's grammar and refuses anything shorter than seven
// days, or anything it cannot read.
func TestStoredRetentionCovers(t *testing.T) {
	for policy, ok := range map[string]bool{
		"30d": true, "7d": true, "1w": true, "1m": true,
		"6d": false, "0d": false, "": false, "30": false, "2y": false, "7 days": false,
	} {
		if err := storedRetentionCovers(policy); (err == nil) != ok {
			t.Errorf("storedRetentionCovers(%q) = %v, want ok=%v", policy, err, ok)
		}
	}
}
