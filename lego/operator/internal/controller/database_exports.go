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
	"context"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/operator/internal/publish"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	exportWorkVolume = "export-work"
	// maxConcurrentExportsPerDB bounds simultaneous active pg_dump Jobs per Database
	// so a tenant cannot fan out unbounded long-running exports (codex-security #13).
	maxConcurrentExportsPerDB = 3
	// exportWorkVolumeSize bounds the disk-backed work volume each pg_dump Job uses.
	exportWorkVolumeSize = 20 * (1 << 30) // 20 GiB
)

// reconcileExports projects append-only Database.spec.exports requests into
// Jobs and reports an honest lifecycle in Database.status.exports. It returns
// the next required poll: short while Jobs run, or the exact next retention
// deadline for an available artifact.
func (r *DatabaseReconciler) reconcileExports(ctx context.Context, db *appv1alpha1.Database, storeConfigured bool) (time.Duration, error) {
	if len(db.Spec.Exports) == 0 {
		db.Status.Exports = nil
		return 0, nil
	}

	statusByID := make(map[string]appv1alpha1.DatabaseExportStatus, len(db.Status.Exports))
	for _, status := range db.Status.Exports {
		statusByID[status.ID] = status
	}

	now := r.exportNow()
	statuses := make([]appv1alpha1.DatabaseExportStatus, 0, len(db.Spec.Exports))
	var nextRequeue time.Duration
	for _, request := range db.Spec.Exports {
		status, exists := statusByID[request.ID]
		if !exists {
			status = r.initialExportStatus(request, db, now)
		}

		switch status.Phase {
		case appv1alpha1.DatabaseExportCreated, appv1alpha1.DatabaseExportRunning:
			if !storeConfigured {
				status.Phase = appv1alpha1.DatabaseExportFailed
				status.CompletedAt = now.Format(time.RFC3339)
				status.FailureReason = "logical exports require a configured backup store"
				break
			}
			if err := r.reconcileExportJob(ctx, db, request, &status, now); err != nil {
				return 0, err
			}

		case appv1alpha1.DatabaseExportAvailable:
			expiresAt, err := time.Parse(time.RFC3339, status.ExpiresAt)
			if err != nil {
				expiresAt = exportCreatedAt(request, now).Add(r.exportRetention())
				status.ExpiresAt = expiresAt.Format(time.RFC3339)
			}
			if !now.Before(expiresAt) {
				status.Phase = appv1alpha1.DatabaseExportExpiring
			}

		case appv1alpha1.DatabaseExportExpiring:
			if err := r.reconcileExportCleanupJob(ctx, db, &status); err != nil {
				return 0, err
			}
		}

		switch status.Phase {
		case appv1alpha1.DatabaseExportCreated, appv1alpha1.DatabaseExportRunning, appv1alpha1.DatabaseExportExpiring:
			nextRequeue = soonerRequeue(nextRequeue, logicalExportPollInterval)
		case appv1alpha1.DatabaseExportAvailable:
			if expiresAt, parseErr := time.Parse(time.RFC3339, status.ExpiresAt); parseErr == nil {
				nextRequeue = soonerRequeue(nextRequeue, expiresAt.Sub(now))
			}
		}

		statuses = append(statuses, status)
	}
	db.Status.Exports = statuses
	return nextRequeue, nil
}

func (r *DatabaseReconciler) reconcileExportJob(
	ctx context.Context,
	db *appv1alpha1.Database,
	request appv1alpha1.DatabaseExportRequest,
	status *appv1alpha1.DatabaseExportStatus,
	now time.Time,
) error {
	job := exportJob(db, request, r.Backup)
	key := client.ObjectKeyFromObject(job)
	if err := r.Get(ctx, key, job); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// Bound concurrent pg_dump Jobs per Database so a tenant can't fan out
		// unbounded long-running exports (codex-security #13). Defer this one
		// until a slot frees; its status stays Created and the requeue retries.
		active, err := r.countActiveExportJobs(ctx, db)
		if err != nil {
			return err
		}
		if active >= maxConcurrentExportsPerDB {
			return nil
		}
		job = exportJob(db, request, r.Backup)
		if err := controllerutil.SetControllerReference(db, job, r.Scheme); err != nil {
			return err
		}
		if err := r.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create postgres export job %s: %w", request.ID, err)
		}
		return nil
	}

	if reason, failed := exportJobFailure(job); failed {
		status.Phase = appv1alpha1.DatabaseExportFailed
		status.CompletedAt = jobFinishedAt(job, now).Format(time.RFC3339)
		status.FailureReason = reason
		return nil
	}
	if exportJobComplete(job) {
		status.Phase = appv1alpha1.DatabaseExportAvailable
		status.CompletedAt = jobFinishedAt(job, now).Format(time.RFC3339)
		status.FailureReason = ""
		// Keep the completed Job until its TTL elapses. If persisting Database
		// status fails, the next reconcile can still observe completion instead of
		// dispatching a duplicate dump.
		return nil
	}
	if job.Status.Active > 0 || job.Status.StartTime != nil {
		status.Phase = appv1alpha1.DatabaseExportRunning
		if status.StartedAt == "" {
			started := now
			if job.Status.StartTime != nil {
				started = job.Status.StartTime.Time
			}
			status.StartedAt = started.UTC().Format(time.RFC3339)
		}
	}
	return nil
}

func (r *DatabaseReconciler) reconcileExportCleanupJob(
	ctx context.Context,
	db *appv1alpha1.Database,
	status *appv1alpha1.DatabaseExportStatus,
) error {
	job := exportCleanupJob(db, *status, r.Backup)
	key := client.ObjectKeyFromObject(job)
	if err := r.Get(ctx, key, job); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		job = exportCleanupJob(db, *status, r.Backup)
		if err := controllerutil.SetControllerReference(db, job, r.Scheme); err != nil {
			return err
		}
		if err := r.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create postgres export cleanup job %s: %w", status.ID, err)
		}
		return nil
	}

	if exportJobComplete(job) {
		status.Phase = appv1alpha1.DatabaseExportExpired
		status.FailureReason = ""
		return nil
	}
	if reason, failed := exportJobFailure(job); failed {
		// Expiry is retried rather than lying that the artifact is gone. Delete the
		// failed Job so the next reconcile can dispatch a clean retry.
		status.FailureReason = "artifact expiry failed: " + reason
		_ = r.Delete(ctx, job)
		return nil
	}
	return nil
}

func (r *DatabaseReconciler) initialExportStatus(request appv1alpha1.DatabaseExportRequest, db *appv1alpha1.Database, now time.Time) appv1alpha1.DatabaseExportStatus {
	created := exportCreatedAt(request, now)
	filename := exportFilename(created)
	return appv1alpha1.DatabaseExportStatus{
		ID:        request.ID,
		Phase:     appv1alpha1.DatabaseExportCreated,
		CreatedAt: created.Format(time.RFC3339),
		ExpiresAt: created.Add(r.exportRetention()).Format(time.RFC3339),
		ObjectKey: logicalExportObjectKey(r.Backup.DestinationPath, db.Name, request.ID, filename),
		Filename:  filename,
	}
}

func exportCreatedAt(request appv1alpha1.DatabaseExportRequest, fallback time.Time) time.Time {
	if parsed, err := time.Parse(time.RFC3339, request.RequestedAt); err == nil {
		return parsed.UTC()
	}
	return fallback.UTC()
}

func (r *DatabaseReconciler) exportNow() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

func (r *DatabaseReconciler) exportRetention() time.Duration {
	if r.ExportRetention > 0 {
		return r.ExportRetention
	}
	return logicalExportRetention
}

func exportFilename(created time.Time) string {
	return created.UTC().Format("2006-01-02T15_04Z") + ".dir.tar.gz"
}

func logicalExportObjectKey(destination, database, exportID, filename string) string {
	return strings.TrimRight(destination, "/") + "/logical-exports/" + database + "/" + exportID + "/" + filename
}

func exportJobName(exportID string) string { return "postgres-export-" + exportID }

func exportCleanupJobName(exportID string) string { return "postgres-export-expire-" + exportID }

// exportJob uses the CNPG Postgres image for a version-compatible pg_dump, then
// the platform's pinned AWS CLI image to upload the archive. The dump only
// traverses an emptyDir shared by the two containers; it never enters bex-api.
func exportJob(db *appv1alpha1.Database, request appv1alpha1.DatabaseExportRequest, store BackupStore) *batchv1.Job {
	created := exportCreatedAt(request, time.Now())
	filename := exportFilename(created)
	directory := created.UTC().Format("2006-01-02T15:04Z")
	objectKey := logicalExportObjectKey(store.DestinationPath, db.Name, request.ID, filename)
	version := db.Spec.Version
	if version == "" {
		version = logicalExportClientVersion
	}
	labels := exportLabels(db, request.ID)
	backoff := int32(0)
	deadline := int64((2 * time.Hour).Seconds())
	ttl := int32((8 * 24 * time.Hour).Seconds())
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: exportJobName(request.ID), Namespace: db.Namespace, Labels: labels},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			ActiveDeadlineSeconds:   &deadline,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{{Name: exportWorkVolume, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{
						SizeLimit: resource.NewQuantity(exportWorkVolumeSize, resource.BinarySI),
					}}}},
					InitContainers: []corev1.Container{{
						Name:            "pg-dump",
						Image:           "ghcr.io/cloudnative-pg/postgresql:" + version,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"/bin/sh", "-ec"},
						Args: []string{`mkdir -p "/work/${EXPORT_DIRECTORY}/${PGDATABASE}"
pg_dump --format=directory --jobs=2 --no-owner --no-privileges --file="/work/${EXPORT_DIRECTORY}/${PGDATABASE}"
tar -C /work -czf "/work/${EXPORT_FILENAME}" "${EXPORT_DIRECTORY}"`},
						Env: append(databaseSecretEnv(db.Status.SecretName),
							corev1.EnvVar{Name: "EXPORT_DIRECTORY", Value: directory},
							corev1.EnvVar{Name: "EXPORT_FILENAME", Value: filename},
						),
						VolumeMounts:    []corev1.VolumeMount{{Name: exportWorkVolume, MountPath: "/work"}},
						Resources:       guaranteedResources("250m", "512Mi"),
						SecurityContext: tenantSecCtx(),
					}},
					Containers: []corev1.Container{{
						Name:            "upload",
						Image:           publish.DefaultAWSCLIImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"/bin/sh", "-ec"},
						Args:            []string{`exec aws s3 cp "/work/${EXPORT_FILENAME}" "${OBJECT_KEY}" --endpoint-url "${ENDPOINT_URL}" --region "${AWS_REGION:-${AWS_DEFAULT_REGION:-us-east-1}}"`},
						Env: []corev1.EnvVar{
							{Name: "EXPORT_FILENAME", Value: filename},
							{Name: "OBJECT_KEY", Value: objectKey},
							{Name: "ENDPOINT_URL", Value: store.EndpointURL},
							{Name: "AWS_EC2_METADATA_DISABLED", Value: "true"},
						},
						EnvFrom:         []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: store.S3Secret}}}},
						VolumeMounts:    []corev1.VolumeMount{{Name: exportWorkVolume, MountPath: "/work"}},
						Resources:       guaranteedResources("100m", "128Mi"),
						SecurityContext: tenantSecCtx(),
					}},
				},
			},
		},
	}
}

func databaseSecretEnv(secretName string) []corev1.EnvVar {
	fromSecret := func(key string) *corev1.EnvVarSource {
		return &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			Key:                  key,
		}}
	}
	return []corev1.EnvVar{
		{Name: "PGHOST", ValueFrom: fromSecret("host")},
		{Name: "PGPORT", ValueFrom: fromSecret("port")},
		{Name: "PGUSER", ValueFrom: fromSecret("username")},
		{Name: "PGPASSWORD", ValueFrom: fromSecret("password")},
		{Name: "PGDATABASE", ValueFrom: fromSecret("dbname")},
	}
}

func exportCleanupJob(db *appv1alpha1.Database, status appv1alpha1.DatabaseExportStatus, store BackupStore) *batchv1.Job {
	labels := exportLabels(db, status.ID)
	backoff := int32(0)
	deadline := int64((10 * time.Minute).Seconds())
	ttl := int32(time.Hour.Seconds())
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: exportCleanupJobName(status.ID), Namespace: db.Namespace, Labels: labels},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			ActiveDeadlineSeconds:   &deadline,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:            "expire",
						Image:           publish.DefaultAWSCLIImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"/bin/sh", "-ec"},
						Args:            []string{`exec aws s3 rm "${OBJECT_KEY}" --endpoint-url "${ENDPOINT_URL}" --region "${AWS_REGION:-${AWS_DEFAULT_REGION:-us-east-1}}"`},
						Env: []corev1.EnvVar{
							{Name: "OBJECT_KEY", Value: status.ObjectKey},
							{Name: "ENDPOINT_URL", Value: store.EndpointURL},
							{Name: "AWS_EC2_METADATA_DISABLED", Value: "true"},
						},
						EnvFrom:         []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: store.S3Secret}}}},
						Resources:       guaranteedResources("100m", "128Mi"),
						SecurityContext: tenantSecCtx(),
					}},
				},
			},
		},
	}
}

func exportLabels(db *appv1alpha1.Database, exportID string) map[string]string {
	labels := map[string]string{logicalExportLabel: exportID, logicalExportDBLabel: db.Name}
	if workspace := db.Labels[labelWorkspace]; workspace != "" {
		labels[labelWorkspace] = workspace
	}
	return labels
}

// countActiveExportJobs reports how many non-terminal pg_dump export Jobs exist
// for db — the long-running dumps, not the short expire-cleanup Jobs (excluded by
// name). Used to cap per-Database concurrency (codex-security #13).
func (r *DatabaseReconciler) countActiveExportJobs(ctx context.Context, db *appv1alpha1.Database) (int, error) {
	var jobs batchv1.JobList
	if err := r.List(ctx, &jobs, client.InNamespace(db.Namespace), client.MatchingLabels{logicalExportDBLabel: db.Name}); err != nil {
		return 0, err
	}
	active := 0
	for i := range jobs.Items {
		j := &jobs.Items[i]
		if strings.HasPrefix(j.Name, "postgres-export-expire-") {
			continue
		}
		if exportJobComplete(j) {
			continue
		}
		if _, failed := exportJobFailure(j); failed {
			continue
		}
		active++
	}
	return active, nil
}

func exportJobComplete(job *batchv1.Job) bool {
	if job.Status.Succeeded > 0 {
		return true
	}
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func exportJobFailure(job *batchv1.Job) (string, bool) {
	for _, condition := range job.Status.Conditions {
		if condition.Type != batchv1.JobFailed || condition.Status != corev1.ConditionTrue {
			continue
		}
		reason := condition.Message
		if reason == "" {
			reason = condition.Reason
		}
		if reason == "" {
			reason = "export Job failed"
		}
		return reason, true
	}
	return "", false
}

func jobFinishedAt(job *batchv1.Job, fallback time.Time) time.Time {
	if job.Status.CompletionTime != nil {
		return job.Status.CompletionTime.UTC()
	}
	for _, condition := range job.Status.Conditions {
		if !condition.LastTransitionTime.IsZero() &&
			(condition.Type == batchv1.JobComplete || condition.Type == batchv1.JobFailed) {
			return condition.LastTransitionTime.UTC()
		}
	}
	return fallback.UTC()
}
