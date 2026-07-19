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

// Package predeploy is the bex pre-deploy plane: run App.spec.preDeployCommand
// to completion against the new revision's image BEFORE that revision serves
// traffic (Render's Pre-Deploy Command — typically a database migration). Like
// the build plane it does the work in a Kubernetes Job, but the shape differs on
// purpose:
//
//   - The Job's container IS the new image (the same env/secrets/pull secrets as
//     the app pod), so a migration runs against exactly what will serve — wrapped
//     in a shell ("sh -c <command>") so an arbitrary command string works.
//   - It NEVER retries (BackoffLimit 0): re-running a partially applied migration
//     can corrupt data, so a failed pre-deploy is terminal until the user pushes a
//     new revision.
//   - The operator observes it non-blocking (a single Get per reconcile, then
//     requeue), so the App CR can report the step Running/Succeeded/Failed while it
//     runs — which is what lets the deploy record surface the step's progress
//     (docs/ADR004-deployment.md, docs/ADR006-bex-api.md § deploy record).
//
// This package owns only the Job/pod scaffolding and observation; the controller
// supplies the container's image, command, and the same env/secrets the app pod
// gets (appEnv/envFromSources/imagePullSecrets), so "what a migration can reach"
// stays defined next to the app container, not duplicated here.
package predeploy

import (
	"context"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/execution"
)

// Pod/Job labels. The predeploy label carries the service name (like build's
// "app.bex.co/build") so the backend can find the step's pod for its logs; the
// component label lets a namespace-wide list distinguish pre-deploy Jobs.
const (
	LabelService   = "app.bex.co/predeploy"
	LabelComponent = "app.bex.co/component"
	ComponentValue = "predeploy"
	labelWorkspace = "app.bex.co/workspace"
)

// predeployTimeout bounds a single pre-deploy Job's wall-clock (its Job
// activeDeadlineSeconds). A hung migration is reaped as failed rather than
// blocking the rollout forever, matching the build Job's bounded-deadline
// pattern (docs/ADR004-deployment.md, w1/m33/t002).
const predeployTimeout = 10 * time.Minute

// jobTTL reaps a finished pre-deploy Job (and its pod's logs) after an hour,
// same as build. The step's outcome survives on the App CR status / deploy
// record, so reaping the Job only drops the ephemeral pod logs.
const jobTTL = 3600

// State is the observed outcome of a pre-deploy Job.
type State string

const (
	// StatePending means the Job exists but has not reached a terminal condition
	// yet (its pod may still be scheduling or running).
	StatePending State = "Pending"
	// StateSucceeded means the Job completed with a zero exit — the rollout may
	// proceed.
	StateSucceeded State = "Succeeded"
	// StateFailed means the Job failed (non-zero exit or deadline exceeded) — the
	// rollout must be blocked and the previous revision left serving.
	StateFailed State = "Failed"
)

// Options configures one pre-deploy run. The container-shaping fields (Env,
// EnvFrom, ImagePullSecrets, SecurityContext, Resources, and the secret-file
// volume) are supplied by the controller so a migration runs with exactly the
// app pod's configuration.
type Options struct {
	Name         string // service name (image repo name / label value)
	Namespace    string // namespace the pre-deploy Job runs in
	Workspace    string // owning tenant id (app.bex.co/workspace label); empty = omitted
	AppNamespace string // namespace the App CR lives in; used for log attribution
	VerifyImage  bool   // select the Pod for signature admission when signing is enabled
	Image        string // the new revision's image (same ref the Deployment will run)
	Command      string // App.spec.preDeployCommand, run as `sh -c <Command>`
	Revision     string // deterministic per-revision tag, e.g. "gen-7" (names the Job)
	Generation   int64  // release generation this step gates (stamped on the pod for traceability)

	Env              []corev1.EnvVar
	EnvFrom          []corev1.EnvFromSource
	ImagePullSecrets []corev1.LocalObjectReference
	SecurityContext  *corev1.SecurityContext
	Resources        corev1.ResourceRequirements
	// Volumes/VolumeMounts carry the app pod's /etc/secrets projection so a
	// migration can read the same secret files; nil when the app has none.
	Volumes      []corev1.Volume
	VolumeMounts []corev1.VolumeMount

	Client client.Client
}

// JobName is the deterministic per-revision pre-deploy Job name (DNS-1123,
// ≤63 chars): "predeploy-<name>-<revision>", so re-reconciling the same
// revision adopts the existing Job (idempotent — never re-runs a completed
// migration) and a new revision gets a fresh run.
func JobName(name, revision string) string {
	rev := revision
	if rev == "" {
		rev = "latest"
	}
	n := "predeploy-" + name + "-" + rev
	if len(n) > 63 {
		n = n[:63]
	}
	return strings.ToLower(n)
}

// Job constructs the pre-deploy Job for o (a pure function — no cluster access —
// so the Job shape is unit-testable). The single container is the new image with
// the app pod's env/secrets, running `sh -c <Command>`; it never restarts or
// retries and is bounded by predeployTimeout.
func Job(o Options) *batchv1.Job {
	deadline := int64(predeployTimeout / time.Second)
	backoff := int32(0) // never retry a migration — a partial re-run can corrupt data
	ttl := int32(jobTTL)

	container := corev1.Container{
		Name:            "predeploy",
		Image:           o.Image,
		Command:         []string{"sh", "-c", o.Command},
		Env:             o.Env,
		EnvFrom:         o.EnvFrom,
		Resources:       o.Resources,
		SecurityContext: o.SecurityContext,
		VolumeMounts:    o.VolumeMounts,
	}

	appNamespace := ""
	if o.AppNamespace != "" && o.AppNamespace != o.Namespace {
		appNamespace = o.AppNamespace
	}
	labels := execution.PodLabels(o.Name, ComponentValue, o.Workspace, appNamespace, o.VerifyImage)
	labels[LabelService] = o.Name
	podLabels := execution.PodLabels(o.Name, ComponentValue, o.Workspace, appNamespace, o.VerifyImage)
	podLabels[LabelService] = o.Name
	podSpec := corev1.PodSpec{
		RestartPolicy:    corev1.RestartPolicyNever,
		Containers:       []corev1.Container{container},
		ImagePullSecrets: o.ImagePullSecrets,
		Volumes:          o.Volumes,
	}
	execution.HardenPod(&podSpec)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobName(o.Name, o.Revision),
			Namespace: o.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			ActiveDeadlineSeconds:   &deadline,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					// The pod carries the service label so the backend can read the
					// step's logs by selector, plus the generation for traceability.
					Labels: podLabels,
					Annotations: map[string]string{
						"app.bex.co/generation": fmt.Sprintf("%d", o.Generation),
					},
				},
				Spec: podSpec,
			},
		},
	}
}

// Ensure creates the pre-deploy Job if it does not already exist, returning the
// current Job either way. Idempotent per revision: a second call for the same
// revision adopts the existing Job rather than starting a second migration.
func Ensure(ctx context.Context, o Options) (*batchv1.Job, error) {
	job := Job(o)
	if err := o.Client.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("predeploy: create job %s: %w", job.Name, err)
	}
	var cur batchv1.Job
	key := client.ObjectKey{Namespace: o.Namespace, Name: JobName(o.Name, o.Revision)}
	if err := o.Client.Get(ctx, key, &cur); err != nil {
		return nil, fmt.Errorf("predeploy: get job %s: %w", key.Name, err)
	}
	return &cur, nil
}

// Observe reports a Job's pre-deploy state and, on failure, a short human
// message from the JobFailed condition (surfaced on the App status / deploy
// record). A Job with neither terminal condition is StatePending.
func Observe(j *batchv1.Job) (State, string) {
	switch {
	case jobCondition(j, batchv1.JobComplete):
		return StateSucceeded, ""
	case jobCondition(j, batchv1.JobFailed):
		return StateFailed, jobFailureMessage(j)
	default:
		return StatePending, ""
	}
}

// CancelSuperseded deletes active (not Complete, not Failed) pre-deploy Jobs for
// the named service EXCEPT keep (the current revision's Job). This is the
// newest-wins policy — a new revision supersedes an older revision's in-flight
// migration — without disturbing the current one. Not-found on delete is
// tolerated (concurrent GC).
func CancelSuperseded(ctx context.Context, name, namespace, keep string, cl client.Client) error {
	var jobs batchv1.JobList
	if err := cl.List(ctx, &jobs,
		client.InNamespace(namespace),
		client.MatchingLabels{LabelService: name}); err != nil {
		return fmt.Errorf("predeploy: list for %s: %w", name, err)
	}
	for i := range jobs.Items {
		j := &jobs.Items[i]
		if j.Name == keep {
			continue
		}
		if jobCondition(j, batchv1.JobComplete) || jobCondition(j, batchv1.JobFailed) {
			continue
		}
		if err := cl.Delete(ctx, j); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("predeploy: cancel %s: %w", j.Name, err)
		}
	}
	return nil
}

// jobCondition reports whether the Job carries condition t with status True.
func jobCondition(j *batchv1.Job, t batchv1.JobConditionType) bool {
	for _, c := range j.Status.Conditions {
		if c.Type == t && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// jobFailureMessage extracts the JobFailed condition's reason/message for the
// detail surfaced on the App status / deploy record.
func jobFailureMessage(j *batchv1.Job) string {
	for _, c := range j.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			if c.Message != "" {
				return c.Reason + ": " + c.Message
			}
			return c.Reason
		}
	}
	return "pre-deploy command failed"
}

func ptr[T any](v T) *T { return &v }
