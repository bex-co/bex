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

package controller

import (
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// An ordinary restore names only sourceDatabase, and it must still find the
// source's archive (w7/039).
//
// SourceBackupServerName is optional — it exists only to pin a specific
// PostgreSQL-major generation — so passing it alone left serverName unset on
// every ordinary restore. The barman plugin then defaults to the NEW cluster's
// own name, where nothing has ever been written, and recovery died with
// "no target backup found" for every managed Postgres. Verified against
// hetzner-prod before the fix: a real restore of a 409 MB tenant database failed
// exactly that way.
//
// The sibling test covers only the case where the optional field IS set, which
// is why this shipped uncaught.
func TestRecoveryDefaultsServerNameToSourceDatabase(t *testing.T) {
	plan, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free"})
	rec := &appv1alpha1.DatabaseRecovery{SourceDatabase: "dpg-source"} // no SourceBackupServerName
	spec := cnpgClusterSpec(clusterParams{
		plan: plan, storageGB: gb, dbname: "d", owner: "d_user", store: &testStore, recovery: rec,
	})

	ext := spec["externalClusters"].([]any)[0].(map[string]any)
	parameters := ext["plugin"].(map[string]any)["parameters"].(map[string]any)
	if got := parameters["serverName"]; got != "dpg-source" {
		t.Errorf("recovery serverName = %v, want the source Database's name %q — "+
			"without it the plugin looks under the new cluster's own prefix and reports "+
			"\"no target backup found\"", got, "dpg-source")
	}
}

// An explicit generation still wins, so a major-upgrade restore can pin the old
// archive prefix.
func TestRecoveryExplicitServerNameOverridesSourceDatabase(t *testing.T) {
	plan, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free"})
	rec := &appv1alpha1.DatabaseRecovery{SourceDatabase: "dpg-source", SourceBackupServerName: "dpg-source-pg16"}
	spec := cnpgClusterSpec(clusterParams{
		plan: plan, storageGB: gb, dbname: "d", owner: "d_user", store: &testStore, recovery: rec,
	})

	ext := spec["externalClusters"].([]any)[0].(map[string]any)
	parameters := ext["plugin"].(map[string]any)["parameters"].(map[string]any)
	if got := parameters["serverName"]; got != "dpg-source-pg16" {
		t.Errorf("explicit generation must win, got %v", got)
	}
}
