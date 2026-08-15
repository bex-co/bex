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

package execution

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

// The operator dispatches four kinds of Kubernetes Job — build, pre-deploy,
// static publish, and delete-time cleanup — and every one of them has to answer
// the same two questions about a Job it is watching: has a condition gone true,
// and if it failed, why. Each package carried its own byte-identical copy of
// both answers; these are the shared ones. The Job *specs* deliberately stay
// per-package (their names, labels, annotations, and pod shapes genuinely
// differ) — it is only the reading of status that is common.

// JobHasCondition reports whether j carries condition type t with status True.
//
// Kubernetes models Job outcome as a condition list rather than a single field,
// so "did it finish" is a scan, not a comparison. Note this asks only about
// conditions: a Job whose pods have succeeded may not yet carry JobComplete,
// which is why the database-export path deliberately keeps its own check that
// also consults Status.Succeeded.
func JobHasCondition(j *batchv1.Job, t batchv1.JobConditionType) bool {
	for _, c := range j.Status.Conditions {
		if c.Type == t && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// JobFinished reports whether j has reached either terminal condition. It is
// the "stop watching this Job" predicate, spelled once so a caller cannot check
// only JobComplete and hang forever on a failed Job.
func JobFinished(j *batchv1.Job) bool {
	return JobHasCondition(j, batchv1.JobComplete) || JobHasCondition(j, batchv1.JobFailed)
}

// JobFailureMessage explains a failed Job as "reason: message", or just the
// reason when Kubernetes supplied no message. fallback is returned when no
// JobFailed condition is present — each caller passes its own wording, since
// the string surfaces to the tenant on the App's status and "unknown build
// failure" is the wrong thing to say about a pre-deploy command.
func JobFailureMessage(j *batchv1.Job, fallback string) string {
	for _, c := range j.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			if c.Message != "" {
				return c.Reason + ": " + c.Message
			}
			return c.Reason
		}
	}
	return fallback
}
