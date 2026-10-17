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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestPostgresStatusTellsARestartFromCreation is the w4/m137 matrix for
// Postgres: `Status.Provisioned` (set on first Ready) separates a restart from
// a first provision, and a suspended or previous-generation Ready is not
// `available`.
func TestPostgresStatusTellsARestartFromCreation(t *testing.T) {
	ready := func(status metav1.ConditionStatus, reason string, observed int64) []metav1.Condition {
		return []metav1.Condition{{
			Type: appv1alpha1.ConditionReady, Status: status, Reason: reason, ObservedGeneration: observed,
		}}
	}
	cases := []struct {
		name       string
		generation int64
		suspended  bool
		deleting   bool
		status     appv1alpha1.DatabaseStatus
		want       string
	}{
		{"never Ready, provisioning", 1, false, false, appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseProvisioning, Conditions: ready(metav1.ConditionFalse, "Provisioning", 1),
		}, "creating"},
		{"served, manual restart", 1, false, false, appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseProvisioning, Provisioned: true,
			Conditions: ready(metav1.ConditionFalse, "Provisioning", 1),
		}, "config_restart"},
		{"resumed: the suspended Ready is stale", 3, false, false, appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseReady, Provisioned: true,
			Conditions: ready(metav1.ConditionFalse, appv1alpha1.ReasonSuspended, 2),
		}, "config_restart"},
		{"current Ready", 2, false, false, appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseReady, Provisioned: true,
			Conditions: ready(metav1.ConditionTrue, appv1alpha1.ReasonProvisioned, 2),
		}, "available"},
		{"upgrading is unchanged", 2, false, false, appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseUpgrading, Provisioned: true,
		}, "upgrading"},
		{"failed", 1, false, false, appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseFailed, Provisioned: true,
		}, "unavailable"},
		{"suspended outranks readiness", 2, true, false, appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseReady, Provisioned: true,
			Conditions: ready(metav1.ConditionFalse, appv1alpha1.ReasonSuspended, 2),
		}, "suspended"},
		// w4/m223: a healthy new database waiting for its managed extensions.
		{"never served, extensions installing", 1, false, false, appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseProvisioning, Conditions: ready(metav1.ConditionFalse, "InitializingExtensions", 1),
		}, "creating"},
		{"never served, just resumed: the suspended Ready is stale", 3, false, false, appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseReady, Conditions: ready(metav1.ConditionFalse, appv1alpha1.ReasonSuspended, 2),
		}, "creating"},
		{"deleting outranks everything", 2, true, true, appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseProvisioning, Provisioned: true,
		}, "deleting"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := &appv1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{Name: "dpg-status", Generation: tc.generation},
				Spec:       appv1alpha1.DatabaseSpec{Suspended: tc.suspended},
				Status:     tc.status,
			}
			if tc.deleting {
				now := metav1.Now()
				db.DeletionTimestamp = &now
			}
			if got := pgView(db).Status; got != tc.want {
				t.Fatalf("status = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestPendingMajorUpgradeReadsUpgradingWithTheSourceVersion (w4/m215): the
// operator now writes this status from an accepted upgrade's acceptance until
// the target major is healthy — including before CNPG announces its phase —
// so every surface reads upgrading with the observed source version, never
// the desired one and never available.
func TestPendingMajorUpgradeReadsUpgradingWithTheSourceVersion(t *testing.T) {
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-upgrade", Generation: 4},
		Spec:       appv1alpha1.DatabaseSpec{Version: "17"},
		Status: appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseUpgrading, Provisioned: true, CurrentVersion: "16",
			Conditions: []metav1.Condition{{
				Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
				Reason: appv1alpha1.ReasonMajorVersionUpgrade, ObservedGeneration: 4,
			}},
		},
	}
	view := pgView(db)
	if view.Status != "upgrading" || view.Version != "16" {
		t.Fatalf("pending upgrade view = status %q version %q, want upgrading on 16", view.Status, view.Version)
	}
}

// TestInitialExtensionFailureIsANamedUnavailableState (w4/m223): a new
// database whose managed extensions could not be installed is unavailable
// with a safe sentence and a code clients translate, never the raw CNPG text.
func TestInitialExtensionFailureIsANamedUnavailableState(t *testing.T) {
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-ext", Generation: 1},
		Status: appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseFailed,
			Conditions: []metav1.Condition{{
				Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
				Reason: appv1alpha1.ReasonExtensionsFailed, ObservedGeneration: 1,
				Message: `extension "vector" is not available in secret/dpg-ext-app`,
			}},
		},
	}
	view := pgView(db)
	if view.Status != "unavailable" || view.StatusReasonCode != appv1alpha1.ReasonExtensionsFailed ||
		view.StatusReason != unavailableReasons[appv1alpha1.ReasonExtensionsFailed] {
		t.Fatalf("view = status %q reason %q code %q", view.Status, view.StatusReason, view.StatusReasonCode)
	}
}
