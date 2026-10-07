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

package core

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestDatastoreStatusReason (w5/079, w5/m129): an unavailable datastore's
// reason is a fixed sentence keyed by its Ready reason, never the condition's
// raw message, except the operator-authored storage shrink refusal.
func TestDatastoreStatusReason(t *testing.T) {
	sentences := map[string]string{"PoolerFailed": "The connection pooler could not be provisioned."}
	ready := func(status metav1.ConditionStatus, reason, message string) []metav1.Condition {
		var conditions []metav1.Condition
		meta.SetStatusCondition(&conditions, metav1.Condition{
			Type: appv1alpha1.ConditionReady, Status: status, Reason: reason, Message: message,
		})
		return conditions
	}
	for _, tc := range []struct {
		name                   string
		conditions             []metav1.Condition
		wantSentence, wantCode string
	}{
		{"no Ready condition", nil, "The widget failed to reconcile.", ""},
		{"a Ready condition that is true", ready(metav1.ConditionTrue, "Provisioned", "raw"), "The widget failed to reconcile.", ""},
		{"a known reason", ready(metav1.ConditionFalse, "PoolerFailed", "raw: secret x in tea-a"), "The connection pooler could not be provisioned.", "PoolerFailed"},
		{"an unknown reason", ready(metav1.ConditionFalse, "SomethingNew", "raw: secret x in tea-a"), "The widget failed to reconcile (SomethingNew).", "SomethingNew"},
		{"the storage shrink refusal", ready(metav1.ConditionFalse, appv1alpha1.ReasonStorageShrinkRejected, "storage is grow-only: requested 1 GB is below the allocated 2 GB"),
			"storage is grow-only: requested 1 GB is below the allocated 2 GB", appv1alpha1.ReasonStorageShrinkRejected},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sentence, code := DatastoreStatusReason(tc.conditions, "widget", sentences)
			if sentence != tc.wantSentence || code != tc.wantCode {
				t.Fatalf("= %q (%s), want %q (%s)", sentence, code, tc.wantSentence, tc.wantCode)
			}
		})
	}
}
