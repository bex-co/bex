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
// One-off jobs are off-roadmap (.pm/DO_NOT_DO.md): create refuses with
// ErrOneOffJobsUnsupported (410) on every surface, while list/get/cancel keep
// serving the history of jobs created before that gate — their status is still
// synced from any surviving Kubernetes Job. The store (BEX_CP_DB_URI) is
// required for the history verbs — without it they return ErrJobsUnavailable
// (503), the standard bex degrade shape.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
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
var ErrJobsUnavailable = core.Unavailable("jobs unavailable: BEX_CP_DB_URI is not set")

// JobStore is the Service's narrow seam to the control-plane store.
type JobStore interface {
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
	// EventFacts records the observed job_run_ended fact when a one-off job
	// finishes (w7/m66) — the completion beat Render shows, which is neither an
	// API write (job_started/job_canceled ride audit rows) nor a deploy row. nil
	// ⇒ no fact (store-off / feature-off, byte-identical). Idempotent by
	// source_key, so re-observing a finished job never double-records.
	EventFacts store.EventFactWriter
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
	appID := store.ManagedAppID(a.Labels)
	for i, j := range jobs {
		if j.Status == store.JobPending || j.Status == store.JobRunning {
			jobs[i] = s.syncStatus(ctx, appID, j)
		}
	}
	out := make([]JobView, len(jobs))
	for i, j := range jobs {
		out[i] = view(j)
	}
	return out, nil
}

// CodeOneOffJobsUnsupported is the stable refusal code for creating a one-off
// job. REST answers 410 with it in `code`, GraphQL in `extensions.code`, and MCP
// prefixes it onto the tool error, so a client can branch on it without
// matching message text.
const CodeOneOffJobsUnsupported = "ONE_OFF_JOBS_UNSUPPORTED"

// ErrOneOffJobsUnsupported is what Create returns on every surface.
//
// Scope decision (w4/m116/t003, 2026-10-02): one-off jobs stay off-roadmap
// (.pm/DO_NOT_DO.md — the 2026-07-27 pillar-5 re-open covers hosted sandboxes
// only), so bex-api is not granted batch/jobs create and never submits one.
// Before this gate every create was accepted, persisted, refused by Kubernetes
// RBAC at admission, and handed back as a job that was already `failed`
// (live 2026-09-17/21/26). Refusing up front — after authorization, so a
// caller who cannot see the service still gets 404/403 — is the honest answer.
// List/get/cancel keep serving the jobs created before the gate.
var ErrOneOffJobsUnsupported = core.NewGoneError(
	CodeOneOffJobsUnsupported,
	"one-off jobs are not supported on bex; run the command in the service itself (`bex ssh <service>`), as a pre-deploy command, or as a cron job — existing jobs remain listable",
	map[string]any{"feature": "one-off jobs"},
)

// Create is Render's one-off job create. bex refuses it with
// ErrOneOffJobsUnsupported (410) once the caller is authorized for the
// service; see that sentinel for the scope decision.
func (s *Service) Create(ctx context.Context, serviceID, _, _ string) (JobView, error) {
	// SECURITY (codex round-5 F2): a one-off job would run a caller-supplied
	// command in the service's image — the same sink SetCommands is gated for —
	// so the gate stays can_create (developer and up), not can_operate, even
	// though the verb now only refuses. Deferred audit (w4/m122): a refusal is
	// not a job that started, so nothing is recorded.
	if _, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanCreate, serviceID); err != nil {
		return JobView{}, err
	}
	return JobView{}, ErrOneOffJobsUnsupported
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
			return JobView{}, core.NotFound("job")
		}
		return JobView{}, err
	}
	if j.Status == store.JobPending || j.Status == store.JobRunning {
		j = s.syncStatus(ctx, store.ManagedAppID(a.Labels), j)
	}
	return view(j), nil
}

// Cancel cancels a running or pending job. It deletes the Kubernetes Job and
// marks the record canceled. Canceling an already-terminal job is a 409.
func (s *Service) Cancel(ctx context.Context, serviceID, jobID string) (JobView, error) {
	// Deferred audit (w4/m122): canceling an already-terminal job is a 409, and
	// that is the common mistake — a UI showing a stale row.
	a, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanOperate, serviceID)
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
			return JobView{}, core.NotFound("job")
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
	s.RecordAppConfigChanged(ctx, a, core.AuditVerbJobCancel)
	return view(j), nil
}

// syncStatus reads the Kubernetes Job's status and updates the DB record if it
// has progressed, recording a job_run_ended fact (w7/m66) when it reaches a
// finished state. It swallows all errors: status sync is best-effort, and a
// temporary cluster outage must not fail a job list/get call. appID keys the
// completion fact ("" for a hand-applied service ⇒ no fact).
func (s *Service) syncStatus(ctx context.Context, appID string, j store.Job) store.Job {
	var kj batchv1.Job
	// The Job was created in its App's namespace (a.Namespace) — the
	// per-tenant `<ws>` namespace under ADR043 — so read it back from there, not
	// the shared s.Namespace. AppNamespace(j.TenantID) == s.Namespace when off.
	if err := s.Client.Get(ctx, client.ObjectKey{Name: k8sJobName(j.ID), Namespace: s.AppNamespace(j.TenantID)}, &kj); err != nil {
		if !apierrors.IsNotFound(err) {
			return j
		}
		// k8s Job gone (TTL GC'd or canceled): if still pending/running, mark failed.
		if j.Status == store.JobPending || j.Status == store.JobRunning {
			updated, err := s.Store.UpdateJobStatus(ctx, j.ID, store.JobFailed)
			if err == nil {
				s.recordJobRunEnded(ctx, appID, updated)
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
	s.recordJobRunEnded(ctx, appID, updated)
	return updated
}

// recordJobRunEnded appends the observed job_run_ended fact when a one-off job
// finishes succeeded or failed — the completion beat that, unlike Create/Cancel,
// is not an authorized API write. A canceled job keeps its job_canceled audit
// event and records no run-ended fact. Idempotent by source_key, so re-observing
// a finished job (or a Create-failure that races the same close) never
// double-records. Best-effort: a store error is logged, never surfaced.
func (s *Service) recordJobRunEnded(ctx context.Context, appID string, j store.Job) {
	if s.EventFacts == nil || appID == "" {
		return
	}
	var status string
	switch j.Status {
	case store.JobSucceeded:
		status = store.EventStatusSucceeded
	case store.JobFailed:
		status = store.EventStatusFailed
	default:
		return
	}
	at := s.Now().UTC()
	if j.FinishedAt != nil {
		at = j.FinishedAt.UTC()
	}
	fact := store.ServiceEventFact{
		SourceKey: "job:" + j.ID + ":run_ended",
		AppID:     appID,
		Type:      store.EventFactJobRunEnded,
		At:        at,
		Status:    status,
	}
	if _, err := s.EventFacts.InsertServiceEventFact(ctx, fact); err != nil {
		log.Printf("events: record job run ended for %s: %v", j.ID, err)
	}
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
// statuses is a list of JobStatus strings. Cursor flows through unchanged.
//
// A NEGATIVE limit is rejected here, not in each adapter, so every surface is
// covered by construction — this is the backstop, and MCP has no other guard.
// Zero stays legal: it is how an omitted limit arrives, and only an adapter
// that can still see presence may reject an explicit one. See
// core.ErrLimitNotPositive for why these lists reject rather than clamp.
func FilterFromStrings(statuses []string, createdBefore, createdAfter, startedBefore, startedAfter, finishedBefore, finishedAfter, cursor string, limit int) (ListFilter, error) {
	if limit < 0 {
		return ListFilter{}, core.ErrLimitNotPositive
	}
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
		if *p.dst, err = core.ParseTime(p.name, p.val); err != nil {
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
