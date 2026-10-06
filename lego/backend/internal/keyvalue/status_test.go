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

package keyvalue

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestKeyValueStatusTellsARestartFromCreation is the w4/m137 matrix: a store
// that has served reports Render's `config_restart` while it restarts — never
// `creating`, and never `available` on a Ready left over from the previous
// spec (a config save, or a resume from suspension).
func TestKeyValueStatusTellsARestartFromCreation(t *testing.T) {
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
		status     appv1alpha1.KeyValueStatus
		want       string
	}{
		{"never Ready, provisioning", 1, false, false, appv1alpha1.KeyValueStatus{
			Phase: appv1alpha1.KVPhaseProvisioning, Conditions: ready(metav1.ConditionFalse, "Provisioning", 1),
		}, "creating"},
		{"served, restarting after a config change", 2, false, false, appv1alpha1.KeyValueStatus{
			Phase: appv1alpha1.KVPhaseProvisioning, CredentialRevision: "rev-1",
			Conditions: ready(metav1.ConditionFalse, "Provisioning", 2),
		}, "config_restart"},
		{"served, pod unready", 2, false, false, appv1alpha1.KeyValueStatus{
			Phase: appv1alpha1.KVPhaseProvisioning, CredentialRevision: "rev-1",
			Conditions: ready(metav1.ConditionFalse, "PodUnready", 2),
		}, "config_restart"},
		{"Ready from the previous generation (config saved, not yet observed)", 3, false, false, appv1alpha1.KeyValueStatus{
			Phase: appv1alpha1.KVPhaseReady, CredentialRevision: "rev-1",
			Conditions: ready(metav1.ConditionTrue, appv1alpha1.ReasonProvisioned, 2),
		}, "config_restart"},
		{"resumed: the suspended Ready is stale", 4, false, false, appv1alpha1.KeyValueStatus{
			Phase: appv1alpha1.KVPhaseReady, CredentialRevision: "rev-1",
			Conditions: ready(metav1.ConditionTrue, appv1alpha1.ReasonSuspended, 3),
		}, "config_restart"},
		{"current Ready", 3, false, false, appv1alpha1.KeyValueStatus{
			Phase: appv1alpha1.KVPhaseReady, CredentialRevision: "rev-1",
			Conditions: ready(metav1.ConditionTrue, appv1alpha1.ReasonProvisioned, 3),
		}, "available"},
		{"Ready without a condition trusts the phase", 0, false, false, appv1alpha1.KeyValueStatus{
			Phase: appv1alpha1.KVPhaseReady,
		}, "available"},
		{"failed", 1, false, false, appv1alpha1.KeyValueStatus{
			Phase: appv1alpha1.KVPhaseFailed, CredentialRevision: "rev-1",
		}, "unavailable"},
		{"suspended outranks the stale Ready", 3, true, false, appv1alpha1.KeyValueStatus{
			Phase: appv1alpha1.KVPhaseReady, CredentialRevision: "rev-1",
			Conditions: ready(metav1.ConditionTrue, appv1alpha1.ReasonProvisioned, 2),
		}, "suspended"},
		{"deleting outranks everything", 3, true, true, appv1alpha1.KeyValueStatus{
			Phase: appv1alpha1.KVPhaseProvisioning, CredentialRevision: "rev-1",
		}, "deleting"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kv := &appv1alpha1.KeyValue{
				ObjectMeta: metav1.ObjectMeta{Name: "red-status", Generation: tc.generation},
				Spec:       appv1alpha1.KeyValueSpec{Suspended: tc.suspended},
				Status:     tc.status,
			}
			if tc.deleting {
				now := metav1.Now()
				kv.DeletionTimestamp = &now
			}
			if got := kvView(kv).Status; got != tc.want {
				t.Fatalf("status = %q, want %q", got, tc.want)
			}
		})
	}
}
