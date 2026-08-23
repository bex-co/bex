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
	"crypto/sha256"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/operator/internal/disksnapshot"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Nightly disk snapshots (docs/ADR082-persistent-disks.md D5).
//
// Render takes a block snapshot of every disk daily and keeps it at least seven
// days. Hetzner has no volume snapshots at any level, so bex takes the same
// cadence and the same retention at the file level: one CronJob per disk that
// streams the volume through tar → gzip → age into the platform's object store.
const (
	diskBackupComponent = "disk-backup"
	diskBackupPrefix    = "dskbak-"
	diskPurgePrefix     = "dskpurge-"
	// diskBackupRetention is Render's documented "available for at least seven
	// days", and the same number the KeyValue backups keep.
	diskBackupRetention = 7
	// diskBackupDeadlineSeconds bounds one run. A snapshot streams at network
	// speed with no staging, but a large volume full of incompressible data can
	// still take a while; six hours is generous and still finite.
	diskBackupDeadlineSeconds = int64(6 * time.Hour / time.Second)
	// diskSnapshotMountPath is where the Job mounts the tenant's volume. It is
	// deliberately not the App's own mountPath: what is captured is the
	// volume's contents, wherever the service happens to mount them.
	diskSnapshotMountPath = "/disk"
	// backupTimeZone pins every backup schedule to UTC so a cluster's local
	// zone cannot silently move the window.
	backupTimeZone = "Etc/UTC"
)

// DiskSnapshotStore is the object-store contract for disk snapshots. It is
// separate from BackupStore (the Postgres/KeyValue one) for a reason worth
// stating: restoring a disk needs the DECRYPT half of the key inside the
// cluster, and ADR050 deliberately keeps the platform backup key out of it. A
// dedicated keypair confines that: the data a disk snapshot protects is already
// sitting on the volume in this same cluster, so holding its key here adds no
// exposure — while reusing the platform key would put etcd and OpenBao backups
// within reach of a cluster compromise.
type DiskSnapshotStore struct {
	// Endpoint is the S3-compatible endpoint; Bucket must be a bucket dedicated
	// to backups, never the Terraform state bucket.
	Endpoint string
	Bucket   string
	Prefix   string
	Region   string
	// S3Secret is a Secret in the App's namespace carrying AWS_ACCESS_KEY_ID
	// and AWS_SECRET_ACCESS_KEY.
	S3Secret string
	// AgePublicKey is the recipient snapshots are encrypted to (safe to inline).
	AgePublicKey string
	// AgeSecret is a Secret in the App's namespace holding the matching private
	// key under "private". Only restore Jobs mount it; a backup never needs it.
	AgeSecret string
}

// configured reports whether snapshots can be taken at all. Encryption is not
// optional here, unlike the KeyValue path's opt-in Tier A: a disk snapshot is a
// full copy of a tenant's filesystem leaving the cluster for a third-party
// bucket, so without a recipient key bex takes no snapshot rather than an
// unencrypted one.
func (d DiskSnapshotStore) configured() bool {
	return d.Endpoint != "" && d.Bucket != "" && d.S3Secret != "" && d.AgePublicKey != ""
}

// restorable additionally requires the decrypt half, which only restore needs.
func (d DiskSnapshotStore) restorable() bool {
	return d.configured() && d.AgeSecret != ""
}

func diskBackupName(appName string) string { return derivedDiskName(diskBackupPrefix, appName, "") }
func diskPurgeName(appName string) string  { return derivedDiskName(diskPurgePrefix, appName, "") }

// diskBackupSchedule spreads tenant snapshots across 02:00–02:59 UTC — an hour
// before the KeyValue window, so the two backup fleets do not contend for the
// same egress. The App name makes the minute stable across reconciles.
func diskBackupSchedule(appName string) string {
	sum := sha256.Sum256([]byte(appName))
	return fmt.Sprintf("%d 2 * * *", int(sum[0])%60)
}

// reconcileDiskBackup converges the nightly snapshot CronJob for a disk-bearing
// App, and removes it once the disk is gone.
func (r *AppReconciler) reconcileDiskBackup(ctx context.Context, app *appv1alpha1.App) error {
	name := diskBackupName(app.Name)
	if app.Spec.Disk == nil || !r.DiskSnapshots.configured() {
		// No disk, or no store to write to: the CronJob must not linger, or a
		// detached disk would keep a nightly Job pointed at a missing PVC.
		if app.Annotations[annotDiskProvisioned] == "" {
			return nil
		}
		return r.deleteStaleChildren(ctx,
			&batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}})
	}
	cron := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cron, func() error {
		cron.Labels = diskBackupLabels(app, diskBackupComponent)
		cron.Spec = r.diskBackupCronJobSpec(app)
		return controllerutil.SetControllerReference(app, cron, r.Scheme)
	})
	return err
}

func diskBackupLabels(app *appv1alpha1.App, component string) map[string]string {
	return map[string]string{
		labelApp:                     app.Name,
		"app.kubernetes.io/name":     component,
		"app.kubernetes.io/instance": app.Name,
	}
}

func (r *AppReconciler) diskBackupCronJobSpec(app *appv1alpha1.App) batchv1.CronJobSpec {
	timeZone := backupTimeZone
	labels := diskBackupLabels(app, diskBackupComponent)
	spec := batchv1.CronJobSpec{
		Schedule:                diskBackupSchedule(app.Name),
		TimeZone:                &timeZone,
		StartingDeadlineSeconds: ptr.To(int64(time.Hour / time.Second)),
		// A second run while one is still uploading would read the volume twice
		// and race the retention sweep.
		ConcurrencyPolicy:          batchv1.ForbidConcurrent,
		FailedJobsHistoryLimit:     ptr.To(int32(3)),
		SuccessfulJobsHistoryLimit: ptr.To(int32(3)),
		JobTemplate: batchv1.JobTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: labels},
			Spec:       r.diskSnapshotJobSpec(app, labels, "backup", nil),
		},
	}
	applyPodSpecServerDefaults(&spec.JobTemplate.Spec.Template.Spec)
	return spec
}

// diskSnapshotJobSpec builds the Job that runs one disk-snapshot command. The
// same shape serves backup, restore and purge — they differ only in the
// argument, the volume they need, and whether they carry the decrypt key.
func (r *AppReconciler) diskSnapshotJobSpec(app *appv1alpha1.App, labels map[string]string, command string, extraEnv []corev1.EnvVar) batchv1.JobSpec {
	mountsVolume := command != "purge"
	env := append([]corev1.EnvVar{
		{Name: "BEX_DISK_SNAPSHOT_ENDPOINT", Value: r.DiskSnapshots.Endpoint},
		{Name: "BEX_DISK_SNAPSHOT_BUCKET", Value: r.DiskSnapshots.Bucket},
		{Name: "BEX_DISK_SNAPSHOT_PREFIX", Value: r.DiskSnapshots.Prefix},
		{Name: "BEX_DISK_SNAPSHOT_REGION", Value: r.DiskSnapshots.Region},
		{Name: "BEX_DISK_SNAPSHOT_RETAIN", Value: fmt.Sprint(diskBackupRetention)},
		{Name: "BEX_DISK_MOUNT_PATH", Value: diskSnapshotMountPath},
		{Name: "BEX_DISK_WORKSPACE", Value: diskSnapshotWorkspace(app)},
		{Name: "BEX_DISK_ID", Value: app.Name},
		{Name: "HOME", Value: "/tmp"},
		{Name: "AWS_EC2_METADATA_DISABLED", Value: "true"},
	}, extraEnv...)
	if command != "purge" {
		env = append(env, corev1.EnvVar{Name: "AGE_PUBLIC_KEY", Value: r.DiskSnapshots.AgePublicKey})
	}

	container := corev1.Container{
		Name:    "snapshot",
		Image:   r.BackupHelperImage,
		Command: []string{"/disk-snapshot"},
		Args:    []string{command},
		Env:     env,
		EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: r.DiskSnapshots.S3Secret},
		}}},
		// The stream is produced and uploaded in one process with nothing
		// staged, so this needs working memory for the gzip/age buffers only —
		// not a budget derived from the volume's size the way the KeyValue
		// pipeline's EmptyDir must be.
		Resources:       backupResources("100m", "256Mi", diskSnapshotEphemeralBudget()),
		SecurityContext: tenantSecCtx(),
	}
	podSpec := corev1.PodSpec{
		RestartPolicy:                corev1.RestartPolicyNever,
		AutomountServiceAccountToken: ptr.To(false),
	}
	if mountsVolume {
		readOnly := command == "backup"
		container.VolumeMounts = []corev1.VolumeMount{{
			Name: diskVolumeName, MountPath: diskSnapshotMountPath, ReadOnly: readOnly,
		}}
		podSpec.Volumes = []corev1.Volume{{
			Name: diskVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: diskPVCName(app.Name), ReadOnly: readOnly,
				},
			},
		}}
		// The volume is ReadWriteOnce. A second pod may mount it only on the
		// node that already has it attached, so the snapshot pod is pinned to
		// wherever the service's own pod is running. Preferred rather than
		// required: a suspended or scaled-to-zero service has no pod to sit
		// beside, and its volume is free to attach anywhere — refusing to back
		// that up would leave parked services silently unprotected.
		podSpec.Affinity = &corev1.Affinity{PodAffinity: &corev1.PodAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
				Weight: 100,
				PodAffinityTerm: corev1.PodAffinityTerm{
					LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelApp: app.Name}},
					TopologyKey:   "kubernetes.io/hostname",
				},
			}},
		}}
	}
	podSpec.Containers = []corev1.Container{container}

	spec := batchv1.JobSpec{
		BackoffLimit:          ptr.To(int32(2)),
		ActiveDeadlineSeconds: ptr.To(diskBackupDeadlineSeconds),
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: labels},
			Spec:       podSpec,
		},
	}
	applyPodSpecServerDefaults(&spec.Template.Spec)
	return spec
}

// diskSnapshotEphemeralBudget bounds the Job's writable layer. The pipeline
// stages nothing, so this only has to cover logs and the multipart uploader's
// in-flight parts — not a budget derived from the volume's size.
func diskSnapshotEphemeralBudget() resource.Quantity { return resource.MustParse("1Gi") }

// diskSnapshotWorkspace is the object-store path segment a disk's snapshots
// live under. The tenant label is the workspace when the App carries one; a
// hand-applied CR without it falls back to its namespace, which is still a
// per-tenant scope under ADR043.
func diskSnapshotWorkspace(app *appv1alpha1.App) string {
	if tenant := app.Labels[labelWorkspace]; tenant != "" {
		return tenant
	}
	// The control plane also stamps the workspace as a bare tenant label on the
	// CR itself; either is a per-tenant scope under ADR043.
	if tenant := app.Labels["bex.co/tenant"]; tenant != "" {
		return tenant
	}
	return app.Namespace
}

// purgeDiskSnapshots runs a one-shot Job that deletes every snapshot a disk
// had. It is fire-and-forget: the disk's own deletion must not wait on the
// object store, and the Job is retried by its own backoff.
func (r *AppReconciler) purgeDiskSnapshots(ctx context.Context, app *appv1alpha1.App) error {
	if !r.DiskSnapshots.configured() {
		return nil
	}
	labels := diskBackupLabels(app, diskBackupComponent)
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: diskPurgeName(app.Name), Namespace: app.Namespace, Labels: labels,
	}}
	job.Spec = r.diskSnapshotJobSpec(app, labels, "purge", nil)
	job.Spec.TTLSecondsAfterFinished = ptr.To(int32(3600))
	if err := controllerutil.SetControllerReference(app, job, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// deleteDiskSnapshotChildren removes a disk's snapshot Jobs when the disk goes
// away, so a re-attached disk starts from a clean slate rather than inheriting
// a CronJob pointed at the previous volume.
func (r *AppReconciler) deleteDiskSnapshotChildren(ctx context.Context, app *appv1alpha1.App) error {
	return r.deleteStaleChildren(ctx,
		&batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: diskBackupName(app.Name), Namespace: app.Namespace}},
	)
}

// snapshotPrefixFor is the object-store prefix a disk's snapshots live under —
// shared with the backend so a listing and a Job agree on where to look.
func snapshotPrefixFor(app *appv1alpha1.App) string {
	return disksnapshot.DiskPrefix(diskSnapshotWorkspace(app), app.Name)
}
