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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// finalizerOverrunWindow returns the window past which a still-running
// finalization is surfaced as stalled. override is the reconciler's
// FinalizerOverrunAfter test seam; a non-positive value falls back to the
// package default so the overrun path can be exercised without a multi-minute
// wall-clock wait.
func finalizerOverrunWindow(override time.Duration) time.Duration {
	if override > 0 {
		return override
	}
	return finalizerOverrunAfter
}

// deletionOverran reports whether finalization has run past window. It keys on
// the object's own DeletionTimestamp — the wall-clock instant Kubernetes
// accepted the delete — so no extra status field is needed to remember when
// teardown began, and a manager restart mid-teardown still measures from the
// same origin. Shared by the App, Database and KeyValue reconcilers.
func deletionOverran(obj metav1.Object, window time.Duration) bool {
	ts := obj.GetDeletionTimestamp()
	if ts == nil || ts.IsZero() {
		return false
	}
	return time.Since(ts.Time) >= window
}
