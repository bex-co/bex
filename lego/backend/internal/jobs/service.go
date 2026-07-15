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

// Package jobs implements Render's one-off jobs surface
// (GET/POST /v1/services/{id}/jobs, GET/POST .../jobs/{jobId}/cancel).
// Each job runs a startCommand in the service's current container image as a
// Kubernetes Job; status is tracked via the cluster and synced back to the
// control-plane store. The store (BEX_CP_DB_URI) is required — without it
// every verb returns ErrJobsUnavailable (503), the standard bex degrade shape.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ErrJobsUnavailable is returned by every verb when BEX_CP_DB_URI is unset —
// the store is the job log; without it there is nothing to return.
var ErrJobsUnavailable = errors.New("jobs unavailable: BEX_CP_DB_URI is not set")

// jobTTL is how long Kubernetes keeps a finished Job's pod for log inspection.
const jobTTL = int32(3600) // 1 hour

// JobStore is the Service's narrow seam to the control-plane store.
type JobStore interface {
	CreateJob(ctx context.Context, serviceName, tenantID, startCommand, planID string) (store.Job, error)
	ListJobs(ctx context.Context, serviceName, tenantID string, filter store.JobListFilter) ([]store.Job, error)
	GetJob(ctx context.Context, serviceName, tenantID, jobID string) (store.Job, error)
	UpdateJobStatus(ctx context.Context, jobID, status string) (store.Job, error)
}

// Service is the one-off jobs feature service. It embeds *core.Base for the
// auth gate, App CR fetch, and Kubernetes client. Store is nil when
// BEX_CP_DB_URI is not set — every verb then returns ErrJobsUnavailable.
type Service struct {
	*core.Base
	Store JobStore
}

// JobView is the presentation projection of a store.Job: the Render-shaped
// wire output all three adapters render. ServiceID is the service name the
// CLI passes as serviceId.
type JobView struct {
	ID           string
	ServiceID    string
	StartCommand string
	PlanID       string
	Status       string
	CreatedAt    time.Time
	StartedAt    *time.Time
	FinishedAt   *time.Time
}

func view(j store.Job) JobView {
	return JobView{
		ID:           j.ID,
		ServiceID:    j.ServiceName,
		StartCommand: j.StartCommand,
		PlanID:       j.PlanID,
		Status:       j.Status,
		CreatedAt:    j.CreatedAt,
		StartedAt:    j.StartedAt,
		FinishedAt:   j.FinishedAt,
	}
}

// ListFilter narrows List — the neutral shape the REST/GraphQL/MCP adapters
// translate Render's query params into.
type ListFilter struct {
	Statuses       []string
	CreatedBefore  time.Time
	CreatedAfter   time.Time
	StartedBefore  time.Time
	StartedAfter   time.Time
	FinishedBefore time.Time
	FinishedAfter  time.Time
	Cursor         string
	Limit          int
}

// toStoreFilter converts a ListFilter to the store's JobListFilter.
func toStoreFilter(f ListFilter) store.JobListFilter {
	return store.JobListFilter{
		Statuses:       f.Statuses,
		CreatedBefore:  f.CreatedBefore,
		CreatedAfter:   f.CreatedAfter,
		StartedBefore:  f.StartedBefore,
		StartedAfter:   f.StartedAfter,
		FinishedBefore: f.FinishedBefore,
		FinishedAfter:  f.FinishedAfter,
		Cursor:         f.Cursor,
		Limit:          f.Limit,
	}
}

// k8sJobName constructs the Kubernetes Job name for a bex job record. The
// job's own id already carries the "job-" prefix (ADR020), giving names like
// "job-c3tqrv7ks6kn..." that are valid k8s names (≤63 chars, DNS-safe). We
// truncate at 63 if somehow exceeded.
func k8sJobName(jobID string) string {
	if len(jobID) > 63 {
		return jobID[:63]
	}
	return jobID
}

// List returns a service's one-off job history, newest first.
func (s *Service) List(ctx context.Context, serviceID string, filter ListFilter) ([]JobView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, serviceID)
	if err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, ErrJobsUnavailable
	}
	tenantID := a.Labels[core.LabelTenant]
	if tenantID == "" {
		tenantID = core.DefaultTenant
	}
	jobs, err := s.Store.ListJobs(ctx, serviceID, tenantID, toStoreFilter(filter))
	if err != nil {
		return nil, err
	}
	// Sync status from the cluster for non-terminal jobs so the list stays
	// fresh without a separate background reconciler.
	for i, j := range jobs {
		if j.Status == store.JobPending || j.Status == store.JobRunning {
			jobs[i] = s.syncStatus(ctx, j)
		}
	}
	out := make([]JobView, len(jobs))
	for i, j := range jobs {
		out[i] = view(j)
	}
	return out, nil
}

// Create starts a new one-off job for the named service. It creates a DB
// record and a Kubernetes Job using the service's current image. If the
// service has no image yet (not yet deployed), it returns ErrConflict.
func (s *Service) Create(ctx context.Context, serviceID, startCommand, planID string) (JobView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, serviceID)
	if err != nil {
		return JobView{}, err
	}
	if s.Store == nil {
		return JobView{}, ErrJobsUnavailable
	}
	tenantID := a.Labels[core.LabelTenant]
	if tenantID == "" {
		tenantID = core.DefaultTenant
	}

	// Resolve the image the job will run in.
	image := a.Status.Image
	if image == "" {
		image = a.Spec.Image
	}
	if image == "" {
		return JobView{}, fmt.Errorf("%w: service %q has no image yet (not deployed)", core.ErrConflict, serviceID)
	}

	// Persist the job record first so we have its id for the k8s Job name.
	j, err := s.Store.CreateJob(ctx, serviceID, tenantID, startCommand, planID)
	if err != nil {
		return JobView{}, err
	}

	// Create the Kubernetes Job. Failure here is logged but not fatal: the
	// record exists and can be seen as pending; the caller may cancel it.
	if createErr := s.createK8sJob(ctx, j.ID, a.Namespace, image, startCommand, tenantID); createErr != nil {
		// Mark the job failed immediately so it doesn't hang in "pending".
		_, _ = s.Store.UpdateJobStatus(ctx, j.ID, store.JobFailed)
		j.Status = store.JobFailed
		now := s.Now()
		j.FinishedAt = &now
	}

	return view(j), nil
}

// Get fetches a single job by id, syncing status from the cluster if non-terminal.
func (s *Service) Get(ctx context.Context, serviceID, jobID string) (JobView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, serviceID)
	if err != nil {
		return JobView{}, err
	}
	if s.Store == nil {
		return JobView{}, ErrJobsUnavailable
	}
	tenantID := a.Labels[core.LabelTenant]
	if tenantID == "" {
		tenantID = core.DefaultTenant
	}
	j, err := s.Store.GetJob(ctx, serviceID, tenantID, jobID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return JobView{}, core.ErrNotFound
		}
		return JobView{}, err
	}
	if j.Status == store.JobPending || j.Status == store.JobRunning {
		j = s.syncStatus(ctx, j)
	}
	return view(j), nil
}

// Cancel cancels a running or pending job. It deletes the Kubernetes Job and
// marks the record canceled. Canceling an already-terminal job is a 409.
func (s *Service) Cancel(ctx context.Context, serviceID, jobID string) (JobView, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanOperate, serviceID)
	if err != nil {
		return JobView{}, err
	}
	if s.Store == nil {
		return JobView{}, ErrJobsUnavailable
	}
	tenantID := a.Labels[core.LabelTenant]
	if tenantID == "" {
		tenantID = core.DefaultTenant
	}
	j, err := s.Store.GetJob(ctx, serviceID, tenantID, jobID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return JobView{}, core.ErrNotFound
		}
		return JobView{}, err
	}
	if j.Status == store.JobSucceeded || j.Status == store.JobFailed || j.Status == store.JobCanceled {
		return JobView{}, fmt.Errorf("%w: job %q is already %s", core.ErrConflict, jobID, j.Status)
	}

	// Delete the Kubernetes Job (best-effort; not-found is fine).
	k8sJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: k8sJobName(jobID), Namespace: a.Namespace}}
	if delErr := s.Client.Delete(ctx, k8sJob, client.PropagationPolicy(metav1.DeletePropagationForeground)); delErr != nil && !apierrors.IsNotFound(delErr) {
		return JobView{}, fmt.Errorf("cancel k8s job: %w", delErr)
	}

	j, err = s.Store.UpdateJobStatus(ctx, jobID, store.JobCanceled)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return JobView{}, fmt.Errorf("%w: job %q already reached a terminal state", core.ErrConflict, jobID)
		}
		return JobView{}, err
	}
	return view(j), nil
}

// createK8sJob creates a Kubernetes Job that runs startCommand in image.
// The job is named by jobID (already DNS-safe: "job-<xid>").
func (s *Service) createK8sJob(ctx context.Context, jobID, namespace, image, startCommand, tenantID string) error {
	backoff := int32(0)
	ttl := jobTTL
	kj := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sJobName(jobID),
			Namespace: namespace,
			Labels: map[string]string{
				core.LabelTenant:    tenantID,
				"app.bex.co/job-id": jobID,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						core.LabelTenant:    tenantID,
						"app.bex.co/job-id": jobID,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:                corev1.RestartPolicyNever,
					AutomountServiceAccountToken: ptr(false),
					Containers: []corev1.Container{
						{
							Name:    "job",
							Image:   image,
							Command: []string{"sh", "-c", startCommand},
						},
					},
				},
			},
		},
	}
	if err := s.Client.Create(ctx, kj); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// syncStatus reads the Kubernetes Job's status and updates the DB record if it
// has progressed. It swallows all errors: status sync is best-effort, and
// a temporary cluster outage must not fail a job list/get call.
func (s *Service) syncStatus(ctx context.Context, j store.Job) store.Job {
	var kj batchv1.Job
	if err := s.Client.Get(ctx, client.ObjectKey{Name: k8sJobName(j.ID), Namespace: s.Namespace}, &kj); err != nil {
		if !apierrors.IsNotFound(err) {
			return j
		}
		// k8s Job gone (TTL GC'd or canceled): if still pending/running, mark failed.
		if j.Status == store.JobPending || j.Status == store.JobRunning {
			updated, err := s.Store.UpdateJobStatus(ctx, j.ID, store.JobFailed)
			if err == nil {
				return updated
			}
		}
		return j
	}

	newStatus := k8sJobStatus(&kj)
	if newStatus == j.Status {
		return j
	}
	updated, err := s.Store.UpdateJobStatus(ctx, j.ID, newStatus)
	if err != nil {
		return j
	}
	return updated
}

// k8sJobStatus maps a Kubernetes Job's conditions to a Render job status.
func k8sJobStatus(kj *batchv1.Job) string {
	for _, c := range kj.Status.Conditions {
		if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
			return store.JobSucceeded
		}
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			return store.JobFailed
		}
	}
	if kj.Status.Active > 0 {
		return store.JobRunning
	}
	if kj.Status.StartTime != nil {
		return store.JobRunning
	}
	return store.JobPending
}

// ptr returns a pointer to v — the one-liner predeploy uses.
func ptr[T any](v T) *T { return &v }

// IsCancellable returns true when the job's status allows cancellation.
func IsCancellable(status string) bool {
	return status != store.JobSucceeded && status != store.JobFailed && status != store.JobCanceled
}

// parseTime parses an optional RFC3339 time string, returning zero on "".
func parseTime(name, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s must be RFC3339 (e.g. 2026-01-02T15:04:05Z)", core.ErrBadRequest, name)
	}
	return t, nil
}

// StatusValid reports whether s is one of the Render job status values.
func StatusValid(s string) bool {
	switch s {
	case store.JobPending, store.JobRunning, store.JobSucceeded, store.JobFailed, store.JobCanceled:
		return true
	}
	return false
}

// FilterFromStrings builds a ListFilter from string-typed values coming from
// REST query params or GraphQL/MCP args. All time fields accept RFC3339;
// statuses is a list of JobStatus strings. Cursor and Limit flow through
// unchanged.
func FilterFromStrings(statuses []string, createdBefore, createdAfter, startedBefore, startedAfter, finishedBefore, finishedAfter, cursor string, limit int) (ListFilter, error) {
	f := ListFilter{Statuses: statuses, Cursor: cursor, Limit: limit}
	var err error
	pairs := []struct {
		name string
		val  string
		dst  *time.Time
	}{
		{"createdBefore", createdBefore, &f.CreatedBefore},
		{"createdAfter", createdAfter, &f.CreatedAfter},
		{"startedBefore", startedBefore, &f.StartedBefore},
		{"startedAfter", startedAfter, &f.StartedAfter},
		{"finishedBefore", finishedBefore, &f.FinishedBefore},
		{"finishedAfter", finishedAfter, &f.FinishedAfter},
	}
	for _, p := range pairs {
		if *p.dst, err = parseTime(p.name, p.val); err != nil {
			return ListFilter{}, err
		}
	}
	for _, s := range statuses {
		if !StatusValid(strings.ToLower(s)) {
			return ListFilter{}, fmt.Errorf("%w: unknown status %q", core.ErrBadRequest, s)
		}
	}
	return f, nil
}
