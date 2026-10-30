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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w5/m141: the two signals the control plane turns into Render's
// postgres_pitr_checkpoint_started and postgres_wal_archive_failed.

func cnpgBackup(name, phase, startedAt string) *unstructured.Unstructured {
	b := &unstructured.Unstructured{Object: map[string]any{
		"spec":   map[string]any{"cluster": map[string]any{"name": "dpg-x"}},
		"status": map[string]any{"phase": phase},
	}}
	b.SetName(name)
	if startedAt != "" {
		_ = unstructured.SetNestedField(b.Object, startedAt, "status", "startedAt")
	}
	return b
}

func TestRunningBackupSummaryNamesOnlyInFlightBackups(t *testing.T) {
	for _, tc := range []struct {
		phase, startedAt string
		running          bool
	}{
		{"running", "2026-10-20T01:00:00Z", true},
		{"running", "", true},
		{"started", "2026-10-20T01:00:00Z", true},
		{"pending", "", false},
		{"", "", false},
	} {
		got, _, ok := runningBackupSummary(cnpgBackup("b1", tc.phase, tc.startedAt))
		if ok != tc.running || (ok && (got.Name != "b1" || got.StartedAt != tc.startedAt)) {
			t.Errorf("phase=%q startedAt=%q: %+v ok=%v, want running=%v", tc.phase, tc.startedAt, got, ok, tc.running)
		}
	}
	// A finished backup is terminal, never also "running"; CNPG's
	// walArchivingFailing is a failed checkpoint, not one stuck in flight.
	if _, _, ok := terminalBackupSummary(cnpgBackup("b2", "completed", "2026-10-20T01:00:00Z")); !ok {
		t.Fatal("completed backup should be terminal")
	}
	if got, _, ok := terminalBackupSummary(cnpgBackup("b3", "walArchivingFailing", "2026-10-20T01:00:00Z")); !ok || got.Phase != "failed" {
		t.Fatalf("walArchivingFailing = %+v ok=%v, want a failed checkpoint", got, ok)
	}
}

func clusterWithArchiving(status, reason, message string) *unstructured.Unstructured {
	c := &unstructured.Unstructured{Object: map[string]any{"status": map[string]any{}}}
	if status != "" {
		_ = unstructured.SetNestedSlice(c.Object, []any{
			map[string]any{"type": "Ready", "status": "True"},
			map[string]any{"type": "ContinuousArchiving", "status": status, "reason": reason, "message": message},
		}, "status", "conditions")
	}
	return c
}

func TestWALArchivingConditionMirrorsCNPG(t *testing.T) {
	db := &appv1alpha1.Database{}
	stampWALArchivingCondition(db, clusterWithArchiving("False", "ContinuousArchivingFailing", "wal-push: access denied"), true)
	c := meta.FindStatusCondition(db.Status.Conditions, appv1alpha1.ConditionWALArchiving)
	if c == nil || c.Status != metav1.ConditionFalse || c.Reason != "ContinuousArchivingFailing" || c.Message != "wal-push: access denied" {
		t.Fatalf("failing archive = %+v", c)
	}
	stampWALArchivingCondition(db, clusterWithArchiving("True", "ContinuousArchivingSuccess", ""), true)
	if c := meta.FindStatusCondition(db.Status.Conditions, appv1alpha1.ConditionWALArchiving); c == nil || c.Status != metav1.ConditionTrue {
		t.Fatalf("recovered archive = %+v", c)
	}
	// No report yet, or backups off: the condition goes away rather than
	// leaving an old verdict behind.
	stampWALArchivingCondition(db, clusterWithArchiving("", "", ""), true)
	if c := meta.FindStatusCondition(db.Status.Conditions, appv1alpha1.ConditionWALArchiving); c != nil {
		t.Fatalf("unreported archive left %+v", c)
	}
	stampWALArchivingCondition(db, clusterWithArchiving("False", "ContinuousArchivingFailing", "x"), false)
	if c := meta.FindStatusCondition(db.Status.Conditions, appv1alpha1.ConditionWALArchiving); c != nil {
		t.Fatalf("backups off left %+v", c)
	}
	// A blank CNPG reason still yields a valid condition reason.
	stampWALArchivingCondition(db, clusterWithArchiving("False", "", ""), true)
	if c := meta.FindStatusCondition(db.Status.Conditions, appv1alpha1.ConditionWALArchiving); c == nil || c.Reason != "ContinuousArchivingFalse" {
		t.Fatalf("blank reason = %+v", c)
	}
}

// w2/m171: the primary and its promotion time the control plane turns into
// postgres_cluster_leader_changed.
func TestPrimaryStatusMirrorsCNPGPromotion(t *testing.T) {
	db := &appv1alpha1.Database{}
	cluster := &unstructured.Unstructured{Object: map[string]any{"status": map[string]any{
		"currentPrimary":          "dpg-x-1",
		"currentPrimaryTimestamp": "2026-10-29T10:00:00.123456Z",
	}}}
	stampPrimaryStatus(db, cluster)
	if db.Status.CurrentPrimary != "dpg-x-1" || db.Status.CurrentPrimaryTimestamp != "2026-10-29T10:00:00.123456Z" {
		t.Fatalf("status = %q @ %q", db.Status.CurrentPrimary, db.Status.CurrentPrimaryTimestamp)
	}
	// A status read that carries neither field keeps the known primary.
	stampPrimaryStatus(db, &unstructured.Unstructured{Object: map[string]any{"status": map[string]any{}}})
	if db.Status.CurrentPrimary != "dpg-x-1" || db.Status.CurrentPrimaryTimestamp == "" {
		t.Fatalf("partial read blanked the primary: %q @ %q", db.Status.CurrentPrimary, db.Status.CurrentPrimaryTimestamp)
	}
}
