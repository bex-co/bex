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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's databaseStatus values a managed datastore (Postgres or Key Value,
// which reuses the enum) moves through around readiness. The operators reuse
// one not-ready phase for a first provision and for every later restart, so the
// two helpers below recover the distinction from status they already persist
// (w4/m137).
const (
	DatastoreAvailable     = "available"
	DatastoreCreating      = "creating"
	DatastoreConfigRestart = "config_restart"
)

// DatastoreReadyStatus is the status of a datastore whose phase is Ready. The
// phase alone can be stale: resuming a suspended store leaves the suspended
// Ready in place until the controller observes the new generation, and a
// config change is live in spec before the pod restarts. So it is `available`
// only when the Ready condition is true, not the suspended placeholder, and
// current for the object's generation. Otherwise the store is restarting onto
// a spec it has not yet served. A store without a Ready condition trusts its
// phase.
func DatastoreReadyStatus(conditions []metav1.Condition, generation int64) string {
	ready := meta.FindStatusCondition(conditions, appv1alpha1.ConditionReady)
	if ready == nil ||
		(ready.Status == metav1.ConditionTrue &&
			ready.Reason != appv1alpha1.ReasonSuspended &&
			ready.ObservedGeneration >= generation) {
		return DatastoreAvailable
	}
	return DatastoreConfigRestart
}

// DatastoreNotReadyStatus is the status of a datastore that is neither Ready
// nor Failed: `creating` until it has served once, then `config_restart`.
func DatastoreNotReadyStatus(servedBefore bool) string {
	if servedBefore {
		return DatastoreConfigRestart
	}
	return DatastoreCreating
}
