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
	"slices"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A Database name is unique only within its workspace, so the archive identity
// must carry the namespace outside the legacy shared apps namespace.
//
// Found on hetzner-prod (w7/040) by migrating three tenant databases into their
// workspace namespace under their existing names: all three CNPG clusters
// reported ContinuousArchivingFailing with "WAL archive check failed ...
// Expected empty archive", because the old same-named cluster still owned the
// prefix. Three production databases ran with zero archived WAL and no base
// backup while every readiness signal stayed green.
func TestDatabaseArchiveBaseIsNamespaceScopedOutsideTheAppsNamespace(t *testing.T) {
	if got := databaseArchiveBase(defaultAppsNamespace, "dpg-orders"); got != "dpg-orders" {
		t.Errorf("legacy apps namespace base = %q, want the bare name — moving it would strand existing history", got)
	}
	if got := databaseArchiveBase("", "dpg-orders"); got != "dpg-orders" {
		t.Errorf("namespaceless base = %q, want the bare name", got)
	}
	if got := databaseArchiveBase("tea-ws1", "dpg-orders"); got != "tea-ws1-dpg-orders" {
		t.Errorf("tenant base = %q, want it namespace-scoped", got)
	}
	if databaseArchiveBase("tea-ws1", "dpg-orders") == databaseArchiveBase("tea-ws2", "dpg-orders") {
		t.Error("two workspaces holding the same Database name resolved to one archive prefix")
	}
}

// The serverName a tenant cluster actually archives into follows the scoped
// base, and an already-pinned status value still wins (it is where the database
// demonstrably has history).
func TestDatabaseBackupServerNamesHonourScopeAndPin(t *testing.T) {
	tenant := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-orders", Namespace: "tea-ws1"}}
	current, target := databaseBackupServerNames(tenant)
	if current != "tea-ws1-dpg-orders" || target != "tea-ws1-dpg-orders" {
		t.Errorf("fresh tenant serverNames = (%q, %q), want both scoped", current, target)
	}

	legacy := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "dpg-orders", Namespace: defaultAppsNamespace}}
	if current, target = databaseBackupServerNames(legacy); current != "dpg-orders" || target != "dpg-orders" {
		t.Errorf("legacy serverNames = (%q, %q), want the bare name unchanged", current, target)
	}

	pinned := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-orders", Namespace: "tea-ws1"},
		Status:     appv1alpha1.DatabaseStatus{BackupServerName: "dpg-orders"},
	}
	if current, _ = databaseBackupServerNames(pinned); current != "dpg-orders" {
		t.Errorf("pinned serverName = %q, want the pin to win — it is where the history is", current)
	}

	// A major upgrade moves to a new generation of the SCOPED base, not of the
	// bare name, and never double-suffixes an already-versioned pin.
	upgrading := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-orders", Namespace: "tea-ws1"},
		Spec:       appv1alpha1.DatabaseSpec{Version: "18"},
		Status:     appv1alpha1.DatabaseStatus{BackupServerName: "tea-ws1-dpg-orders-pg17", CurrentVersion: "17"},
	}
	if _, target = databaseBackupServerNames(upgrading); target != "tea-ws1-dpg-orders-pg18" {
		t.Errorf("upgrade target = %q, want tea-ws1-dpg-orders-pg18", target)
	}
}

// An ordinary restore names only sourceDatabase; in a tenant namespace the
// source archives under the scoped prefix, so the default must resolve there.
// Reading the bare name would repeat w7/039's "no target backup found" for
// every tenant-namespace database.
func TestRecoveryDefaultsToTheScopedSourceArchive(t *testing.T) {
	plan, gb := resolvePlan(appv1alpha1.DatabaseSpec{Plan: "free"})
	spec := cnpgClusterSpec(clusterParams{
		plan: plan, storageGB: gb, dbname: "d", owner: "d_user", namespace: "tea-ws1",
		store:    &BackupStore{DestinationPath: "s3://b/postgres", EndpointURL: "https://s3.example", S3Secret: "creds"},
		recovery: &appv1alpha1.DatabaseRecovery{SourceDatabase: "dpg-source"},
	})
	ext, ok := spec["externalClusters"].([]any)
	if !ok || len(ext) != 1 {
		t.Fatalf("externalClusters = %v", spec["externalClusters"])
	}
	params := ext[0].(map[string]any)["plugin"].(map[string]any)["parameters"].(map[string]any)
	if params["serverName"] != "tea-ws1-dpg-source" {
		t.Errorf("recovery serverName = %v, want tea-ws1-dpg-source", params["serverName"])
	}
}

// The purge walks prefixes, not objects. Keyed on the bare CR name it would
// delete a same-named database's archive in another workspace — a live database
// losing its backups because an unrelated one was deleted (w7/040).
func TestBackupPurgeTargetsOnlyThisDatabasesPrefixes(t *testing.T) {
	r := &DatabaseReconciler{Backup: BackupStore{
		DestinationPath: "s3://backups/postgres", EndpointURL: "https://s3.example", S3Secret: "backup-creds",
	}}
	tenant := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-orders", Namespace: "tea-ws1", UID: "uid-1"},
		Status:     appv1alpha1.DatabaseStatus{BackupServerName: "tea-ws1-dpg-orders"},
	}
	prefixes := databaseArchivePrefixes(tenant)
	if slices.Contains(prefixes, "dpg-orders") {
		t.Error("purge set contains the bare name — it would erase another workspace's live archive")
	}
	for _, want := range []string{"tea-ws1-dpg-orders", "tea-ws1-dpg-orders-pg17", "tea-ws1-dpg-orders-pg18"} {
		if !slices.Contains(prefixes, want) {
			t.Errorf("purge set missing %q: %v — a generation would be left behind", want, prefixes)
		}
	}
	if len(prefixes) != len(uniqueStrings(prefixes)) {
		t.Errorf("purge set has duplicates: %v", prefixes)
	}

	env := purgeJobEnv(r.dbBackupPurgeJob(tenant))
	if !strings.Contains(env["PREFIXES"], "tea-ws1-dpg-orders") {
		t.Errorf("purge Job PREFIXES = %q, want the scoped prefixes", env["PREFIXES"])
	}
	if slices.Contains(strings.Fields(env["PREFIXES"]), "dpg-orders") {
		t.Errorf("purge Job would delete the bare prefix: %q", env["PREFIXES"])
	}

	// A legacy database in the apps namespace keeps purging exactly what it did.
	legacy := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{
		Name: "dpg-orders", Namespace: defaultAppsNamespace, UID: "uid-2",
	}}
	got := strings.Fields(purgeJobEnv(r.dbBackupPurgeJob(legacy))["PREFIXES"])
	want := []string{"dpg-orders", "dpg-orders-pg13", "dpg-orders-pg14", "dpg-orders-pg15", "dpg-orders-pg16", "dpg-orders-pg17", "dpg-orders-pg18"}
	if !slices.Equal(got, want) {
		t.Errorf("legacy purge set = %v, want %v — legacy behaviour must be unchanged", got, want)
	}
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func purgeJobEnv(job *batchv1.Job) map[string]string {
	env := map[string]string{}
	for _, e := range job.Spec.Template.Spec.Containers[0].Env {
		env[e.Name] = e.Value
	}
	return env
}
