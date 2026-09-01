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
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// The operator dispatches four kinds of Kubernetes Job — build, pre-deploy,
// static publish, and delete-time cleanup — and every one of them has to answer
// the same two questions about a Job it is watching: has a condition gone true,
// and if it failed, why. Each package carried its own byte-identical copy of
// both answers; these are the shared ones. The Job *specs* deliberately stay
// per-package (their names, labels, annotations, and pod shapes genuinely
// differ) — it is only the reading of status that is common.

// EnsureOwnedJob dispatches job if it does not already exist and returns the
// Job's CURRENT server-side state either way — never a wait (the non-blocking
// observe shape all three Job planes share, ADR060 §D1). It reads before it
// writes: the steady-state reconcile of an App whose Job already exists is one
// cached Get, not a live API-server Create rejected with 409 (the same idiom
// as the controller's deleteStaleChildren — a blind write is a live API round
// trip). Only a NotFound dispatches a Create, which still tolerates
// AlreadyExists so the create/create race a concurrent reconcile wins adopts
// the winner. A name hit is adopted only after owned.CheckOwner proves the
// existing Job belongs to this exact parent lifetime, so deterministic-name
// reuse across recreations can never resurrect another lifetime's artifact.
// created reports whether THIS call dispatched the Job (false when it adopted
// an existing one, including the create/create race a concurrent reconcile
// wins). pkg prefixes the returned errors ("build", "publish", "predeploy") so
// each plane's error strings stay byte-identical to its pre-consolidation
// wording.
func EnsureOwnedJob(ctx context.Context, cl client.Client, job *batchv1.Job, owned ArtifactIdentity, pkg string) (*batchv1.Job, bool, error) {
	key := client.ObjectKeyFromObject(job)
	cur := &batchv1.Job{}
	created := false
	if err := cl.Get(ctx, key, cur); apierrors.IsNotFound(err) {
		switch createErr := cl.Create(ctx, job); {
		case createErr == nil:
			cur, created = job, true
		case apierrors.IsAlreadyExists(createErr):
			// A concurrent reconcile won the create/create race between the
			// Get and the Create (or the cache lags a prior dispatch): fetch
			// the winner and adopt it below.
			if err := cl.Get(ctx, key, cur); err != nil {
				return nil, false, fmt.Errorf("%s: get job %s: %w", pkg, key.Name, err)
			}
		default:
			return nil, false, fmt.Errorf("%s: create job %s: %w", pkg, key.Name, createErr)
		}
	} else if err != nil {
		return nil, false, fmt.Errorf("%s: get job %s: %w", pkg, key.Name, err)
	}
	if err := owned.CheckOwner(cur); err != nil {
		return nil, false, fmt.Errorf("%s: check job owner %s: %w", pkg, key.Name, err)
	}
	return cur, created, nil
}

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

// JobFailedReason returns the JobFailed condition's Reason (empty when the Job
// has not failed). It lets a caller distinguish an activeDeadlineSeconds reap
// ("DeadlineExceeded") from a tenant/infra build fault so the two are metered
// and surfaced differently (ADR060 §D1a/§D5).
func JobFailedReason(j *batchv1.Job) string {
	for _, c := range j.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return c.Reason
		}
	}
	return ""
}

// JobFailedAt is when the Job's JobFailed condition became true (zero when the
// Job has not failed). Kubernetes sets CompletionTime only on success, so this
// is a failed Job's own end-of-run timestamp — the build path derives a failed
// build's real duration from it instead of reading zero (w6/m123).
func JobFailedAt(j *batchv1.Job) time.Time {
	for _, c := range j.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return c.LastTransitionTime.Time
		}
	}
	return time.Time{}
}
