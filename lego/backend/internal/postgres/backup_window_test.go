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
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// recoveryStatusREST reads Render's GET /v1/postgres/{id}/recovery.
func recoveryStatusREST(t *testing.T, svc *Service, id string) (status, startsAt string) {
	t.Helper()
	res := serveREST(svc, http.MethodGet, "/v1/postgres/"+id+"/recovery", "")
	if res.Code != http.StatusOK {
		t.Fatalf("REST recovery of %s: %d %s", id, res.Code, res.Body)
	}
	var body struct {
		Status   string `json:"recoveryStatus"`
		StartsAt string `json:"startsAt"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Status, body.StartsAt
}

// upgradeDryRunREST answers REST PATCH {"version":"18","dryRun":true}: the
// status and the error code, if any.
func upgradeDryRunREST(t *testing.T, svc *Service, id string) (int, string) {
	t.Helper()
	res := serveREST(svc, http.MethodPatch, "/v1/postgres/"+id, `{"version":"18","dryRun":true}`)
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &body)
	return res.Code, body.Code
}

// A workspace database's completed backup, as the upgrade gate and recovery
// read it. The Barman Cloud plugin leaves a Backup's serverName empty and the
// database archives under a namespace-scoped name, so the ObjectStore's window
// for the generation is one record of the backup (w34/m543 t011). The plugin
// drops that window write when another database's write to the shared
// ObjectStore lands first (w34/009, production 2026-11-21), and then the
// generation's completed Backups are the record. The gate and recovery agree in
// every case, and a backup from before the last upgrade authorizes neither.
func TestTheGenerationsBackupAuthorizesUpgradeAndRecovery(t *testing.T) {
	const (
		base     = "tea-x-upgrade-db"
		sibling  = "tea-x-sibling-db" // another database's window in the shared ObjectStore
		earlier  = "2026-11-20T03:00:00Z"
		backedUp = "2026-11-21T22:19:53Z"
		later    = "2026-11-22T03:00:00Z"
	)
	tests := []struct {
		name       string
		generation string // Database.status.backupServerName; the database runs 17
		window     string // the server whose ObjectStore window is established
		backups    []*unstructured.Unstructured
		// The recovery window; "" is none, BACKUP_NOT_READY and a refused upgrade.
		wantEarliest, wantLatest string
	}{
		{
			name: "the plugin's window for this generation", generation: base, window: base,
			// An object that carries neither its server nor its major cannot be
			// placed; the window vouches for it.
			backups:      []*unstructured.Unstructured{pluginBackup("upgrade-db", "b1", "completed", backedUp, 0)},
			wantEarliest: "2026-11-21T16:13:23Z", wantLatest: "2026-11-21T16:13:57Z",
		},
		{
			name: "the plugin dropped its window write", generation: base, window: sibling,
			backups:      []*unstructured.Unstructured{pluginBackup("upgrade-db", "b1", "completed", backedUp, 17)},
			wantEarliest: backedUp, wantLatest: backedUp,
		},
		{
			name: "the generation's backups span the window", generation: base, window: sibling,
			backups: []*unstructured.Unstructured{
				pluginBackup("upgrade-db", "b2", "completed", backedUp, 17),
				pluginBackup("upgrade-db", "b3", "completed", later, 17),
				pluginBackup("upgrade-db", "b1", "completed", earlier, 17),
				pluginBackup("upgrade-db", "b4", "failed", later, 17),
			},
			wantEarliest: earlier, wantLatest: later,
		},
		{
			name: "the first backup still running", generation: base, window: sibling,
			backups: []*unstructured.Unstructured{pluginBackup("upgrade-db", "b1", "running", "", 17)},
		},
		{
			name: "the first backup failed", generation: base, window: sibling,
			backups: []*unstructured.Unstructured{pluginBackup("upgrade-db", "b1", "failed", backedUp, 17)},
		},
		{
			name: "a backup carrying neither its server nor its major", generation: base, window: sibling,
			backups: []*unstructured.Unstructured{pluginBackup("upgrade-db", "b1", "completed", backedUp, 0)},
		},
		{
			// The legacy apps namespace's first generation, the bare name: the
			// only generation such an object can belong to.
			name: "the bare name's backup carrying neither", generation: "upgrade-db", window: sibling,
			backups:      []*unstructured.Unstructured{pluginBackup("upgrade-db", "b1", "completed", backedUp, 0)},
			wantEarliest: backedUp, wantLatest: backedUp,
		},
		{
			name: "another database's backup", generation: base, window: sibling,
			backups: []*unstructured.Unstructured{pluginBackup("sibling-db", "b1", "completed", backedUp, 17)},
		},
		{
			name: "only the generation before the last upgrade", generation: base + "-pg17", window: base,
			backups: []*unstructured.Unstructured{pluginBackup("upgrade-db", "b1", "completed", earlier, 16)},
		},
		{
			name: "the post-upgrade backup's window write dropped", generation: base + "-pg17", window: base,
			backups: []*unstructured.Unstructured{
				pluginBackup("upgrade-db", "b1", "completed", earlier, 16),
				pluginBackup("upgrade-db", "post-upgrade-pg17", "completed", backedUp, 17),
			},
			wantEarliest: backedUp, wantLatest: backedUp,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := versionDatabase("upgrade-db", "basic-1gb", "17", true)
			db.Status.BackupServerName = tt.generation
			objects := []client.Object{db}
			for _, backup := range tt.backups {
				objects = append(objects, backup)
			}
			svc, cl := newServicePostgresResources(objects...)
			seedBarmanRecoveryWindow(t, cl, tt.window, "2026-11-21T16:13:23Z", "2026-11-21T16:13:57Z")

			status, startsAt := recoveryStatusREST(t, svc, "upgrade-db")
			info, err := svc.RecoveryInfo(context.Background(), "upgrade-db")
			if err != nil {
				t.Fatal(err)
			}
			code, errCode := upgradeDryRunREST(t, svc, "upgrade-db")
			if tt.wantEarliest == "" {
				if status != "BACKUP_NOT_READY" || info.EarliestRecoveryTime != "" || info.LatestRecoveryTime != "" {
					t.Errorf("recovery = %s from %q, window %q–%q; want BACKUP_NOT_READY and no window", status, startsAt, info.EarliestRecoveryTime, info.LatestRecoveryTime)
				}
				if code != http.StatusConflict || errCode != "POSTGRES_UPGRADE_BACKUP_REQUIRED" {
					t.Errorf("upgrade dry run = %d %s, want 409 POSTGRES_UPGRADE_BACKUP_REQUIRED", code, errCode)
				}
				return
			}
			if status != "AVAILABLE" || startsAt != tt.wantEarliest || info.LatestRecoveryTime != tt.wantLatest {
				t.Errorf("recovery = %s from %q to %q, want AVAILABLE from %s to %s", status, startsAt, info.LatestRecoveryTime, tt.wantEarliest, tt.wantLatest)
			}
			if code != http.StatusOK {
				t.Errorf("upgrade dry run = %d %s, want 200", code, errCode)
			}
		})
	}
}

// w34/009 on production: two databases created together in one workspace
// finished their first backups three seconds apart, and the plugin kept only
// the first one's window write. Both read AVAILABLE from their own backup on
// every surface, both accept an upgrade dry run, and the second recovers.
func TestTwoDatabasesBackedUpTogetherBothReadTheirWindow(t *testing.T) {
	svc, cl := newServicePostgresResources()
	gate := &rejectingPaymentGate{}
	svc.Payment = gate
	databases := []struct {
		name, version, backedUp string
		major                   int64
	}{
		{"dpg-sibling", "16", "2026-11-21T22:19:50Z", 16},
		{"dpg-src", "17", "2026-11-21T22:19:53Z", 17}, // its window write was dropped
	}
	for _, d := range databases {
		db := seedDatabaseSpec(t, cl, d.name, appv1alpha1.DatabaseSpec{Plan: "basic-256mb", Version: d.version}, true)
		db.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelWorkspace: "tea-a"}
		db.Status.CurrentVersion = d.version
		db.Status.BackupServerName = "tea-a-" + d.name
		if err := cl.Update(context.Background(), db); err != nil {
			t.Fatal(err)
		}
		if err := cl.Create(context.Background(), pluginBackup(d.name, d.name+"-backup", "completed", d.backedUp, d.major)); err != nil {
			t.Fatal(err)
		}
	}
	seedBarmanRecoveryWindow(t, cl, "tea-a-dpg-sibling", "2026-11-21T22:19:50Z", "2026-11-21T22:19:50Z")

	for _, d := range databases {
		if status, startsAt := recoveryStatusREST(t, svc, d.name); status != "AVAILABLE" || startsAt != d.backedUp {
			t.Errorf("%s recovery = %s from %q, want AVAILABLE from its own backup %s", d.name, status, startsAt, d.backedUp)
		}
		if code, errCode := upgradeDryRunREST(t, svc, d.name); code != http.StatusOK {
			t.Errorf("%s upgrade dry run = %d %s, want 200", d.name, code, errCode)
		}
	}
	_, gql, mcpOut := recoveryInfoOnEverySurface(t, svc)
	for surface, out := range map[string]map[string]any{"GraphQL": gql, "MCP": mcpOut} {
		if out["earliestRecoveryTime"] != "2026-11-21T22:19:53Z" || out["latestRecoveryTime"] != "2026-11-21T22:19:53Z" {
			t.Errorf("%s window = %v, want the database's own backup on both bounds", surface, out)
		}
	}
	if _, err := svc.Recover(context.Background(), "dpg-src", RecoverRequest{Name: "restored"}); !errors.Is(err, core.ErrPaymentRequired) {
		t.Errorf("recover => %v, want it to pass the window and reach the payment gate", err)
	}
}
