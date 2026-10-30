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

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/operator/internal/disksnapshot"
	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/types/k8sname"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Nightly disk snapshots (docs/ADR082-persistent-disks.md D5).
//
// Render takes a block snapshot of every disk daily and keeps it at least seven
// days. Hetzner has no volume snapshots at any level, so bex takes the same
// cadence and the same retention at the file level: one CronJob per disk that
// streams the volume through tar → gzip → age into the platform's object store.
const (
	diskBackupComponent    = "disk-backup"
	diskBackupPrefix       = "dskbak-"
	diskPurgePrefix        = "dskpurge-"
	diskPurgeComponent     = "disk-snapshot-purge"
	annotDiskSnapshots     = "app.bex.co/disk-snapshots"
	annotDiskPurgeComplete = "app.bex.co/disk-snapshot-purge-complete"
	annotDiskPurgePending  = "app.bex.co/disk-snapshot-purge-pending"
	// diskBackupRetention is Render's documented "available for at least seven
	// days", and the same number the KeyValue backups keep.
	diskBackupRetention = 7
	// diskBackupDeadlineSeconds bounds one run. A snapshot streams at network
	// speed with no staging, but a large volume full of incompressible data can
	// still take a while; six hours is generous and still finite.
	diskBackupDeadlineSeconds = int64(appv1alpha1.DiskBackupActiveDeadline / time.Second)
	// diskSnapshotMountPath is where the Job mounts the tenant's volume. It is
	// deliberately not the App's own mountPath: what is captured is the
	// volume's contents, wherever the service happens to mount them.
	diskSnapshotMountPath = "/disk"
	// diskAgePrivateKeyKey is the data key the restore Job reads its identity
	// from, inside the namespace-local Secret DiskSnapshots.AgeSecret names.
	diskAgePrivateKeyKey = "private"
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
	// SourceNamespace is where the canonical S3Secret/AgeSecret pair lives (the
	// apps namespace, mirroring the datastore BackupSourceNamespace). When set,
	// the reconciler projects both into each disk-bearing App's own namespace,
	// so a workspace namespace created after `disk-snapshot-secret.sh install`
	// ran still gets its backup credential — previously its first backup Job
	// died in CreateContainerConfigError until an operator re-ran the script
	// (w2/033). Empty ⇒ no projection (single-namespace/dev, byte-identical).
	SourceNamespace string
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

func diskBackupName(appName string) string {
	return k8sname.FitCronJob(appv1alpha1.DiskChildName(diskBackupPrefix, appName))
}
func diskPurgeName(app *appv1alpha1.App) string {
	return cleanupJobName(diskPurgePrefix, app.Name, app.UID)
}

// diskBackupSchedule spreads tenant snapshots across 02:00–02:59 UTC — an hour
// before the KeyValue window, so the two backup fleets do not contend for the
// same egress. The App name makes the minute stable across reconciles.
func diskBackupSchedule(appName string) string {
	sum := sha256.Sum256([]byte(appName))
	return fmt.Sprintf("%d %d * * *", int(sum[0])%60, appv1alpha1.DiskBackupHourUTC)
}

// reconcileDiskBackup converges the nightly snapshot CronJob for a disk-bearing
// App, and removes it once the disk is gone.
func (r *AppReconciler) reconcileDiskBackup(ctx context.Context, app *appv1alpha1.App) error {
	if app.Annotations[annotDiskPurgePending] != "" {
		return fmt.Errorf("disk snapshot cleanup must finish before scheduling snapshots")
	}
	name := diskBackupName(app.Name)
	if app.Spec.Disk == nil || !r.DiskSnapshots.configured() {
		// No disk, or no store to write to: the CronJob must not linger, or a
		// detached disk would keep a nightly Job pointed at a missing PVC.
		if app.Annotations[annotDiskProvisioned] == "" {
			return nil
		}
		cron := &batchv1.CronJob{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, cron); err != nil {
			return client.IgnoreNotFound(err)
		}
		if !metav1.IsControlledBy(cron, app) {
			return nil
		}
		if err := r.markDiskSnapshots(ctx, app); err != nil {
			return err
		}
		return client.IgnoreNotFound(r.Delete(ctx, cron, client.Preconditions{UID: &cron.UID}))
	}
	if r.BackupHelperImage == "" {
		// The Job runs bex's own image, resolved from the operator's Pod. With
		// no image the CronJob would be rejected by the API server on every
		// pass — an error loop that says nothing about the real problem. Fail
		// closed and name it instead: the owner can see that snapshots are not
		// running, which is the one thing a silent backup must never hide.
		setDiskCondition(app, false, appv1alpha1.DiskReasonSnapshotImageUnresolved,
			"cannot schedule disk snapshots: the operator could not resolve its own image (POD_NAME/BEX_BACKUP_HELPER_IMAGE)")
		return nil
	}
	switch err := r.projectDiskSnapshotSecrets(ctx, app); {
	case apierrors.IsNotFound(err):
		// The canonical credential pair is absent from the source namespace, so
		// a CronJob would only produce CreateContainerConfigError pods at 02:xx
		// — the silent-unprotected-disk failure this projection exists to end.
		// Name it and take no snapshot instead.
		setDiskCondition(app, false, appv1alpha1.DiskReasonSnapshotCredentialUnavailable,
			fmt.Sprintf("cannot schedule disk snapshots: %v (run scripts/disk-snapshot-secret.sh install)", err))
		return nil
	case err != nil:
		return err
	}
	if err := r.markDiskSnapshots(ctx, app); err != nil {
		return err
	}
	cron := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cron, func() error {
		cron.Labels = diskBackupLabels(app)
		cron.Spec = r.diskBackupCronJobSpec(app)
		return controllerutil.SetControllerReference(app, cron, r.Scheme)
	})
	return err
}

// projectDiskSnapshotSecrets copies the canonical disk-snapshot credential pair
// from the apps namespace into a disk-bearing App's own namespace, exactly the
// way the Database/KeyValue reconcilers project their backup credential
// (ADR043 D8.4). This is what lets a workspace namespace minted AFTER the
// provision script ran still take its nightly snapshot: the copy happens at
// reconcile time, refreshed on drift, wherever the App actually lives —
// including the bootstrap apps namespace itself, where projection no-ops
// because source and destination coincide.
//
// The age Secret carries the DECRYPT half, but projecting it alongside adds no
// exposure: tenant workloads cannot read either copy (no Kubernetes API
// access, automountServiceAccountToken disabled, and the protected-mount label
// keeps both out of tenant pod specs), and restore Jobs need it namespace-local.
func (r *AppReconciler) projectDiskSnapshotSecrets(ctx context.Context, app *appv1alpha1.App) error {
	src := r.DiskSnapshots.SourceNamespace
	if src == "" || src == app.Namespace {
		return nil
	}
	cl := r.uncachedSecretClient()
	if err := projectBackupCredential(ctx, cl, src, app.Namespace, r.DiskSnapshots.S3Secret); err != nil {
		return err
	}
	if r.DiskSnapshots.AgeSecret == "" {
		return nil
	}
	return projectBackupCredential(ctx, cl, src, app.Namespace, r.DiskSnapshots.AgeSecret)
}

func diskBackupLabels(app *appv1alpha1.App) map[string]string {
	labels := (execution.ArtifactIdentity{Name: app.Name, UID: string(app.UID),
		Workspace: diskSnapshotWorkspace(app), Namespace: app.Namespace}).Labels(diskBackupComponent)
	labels["app.kubernetes.io/name"] = diskBackupComponent
	labels["app.kubernetes.io/instance"] = app.Name
	return labels
}

func (r *AppReconciler) diskBackupCronJobSpec(app *appv1alpha1.App) batchv1.CronJobSpec {
	timeZone := backupTimeZone
	labels := diskBackupLabels(app)
	spec := batchv1.CronJobSpec{
		Schedule:                diskBackupSchedule(app.Name),
		TimeZone:                &timeZone,
		StartingDeadlineSeconds: ptr.To(int64(appv1alpha1.DiskBackupStartingDeadline / time.Second)),
		// A second run while one is still uploading would read the volume twice
		// and race the retention sweep.
		ConcurrencyPolicy:          batchv1.ForbidConcurrent,
		FailedJobsHistoryLimit:     new(int32(3)),
		SuccessfulJobsHistoryLimit: new(int32(3)),
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
		{Name: awsIMDSDisabledEnv, Value: awsIMDSDisabled},
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
		// No legacy Docker-link env vars (w4/m123) — a platform maintenance pod.
		EnableServiceLinks:           new(false),
		RestartPolicy:                corev1.RestartPolicyNever,
		AutomountServiceAccountToken: new(false),
	}
	if mountsVolume {
		readOnly := command == "backup"
		container.VolumeMounts = []corev1.VolumeMount{{
			Name: diskVolumeName, MountPath: diskSnapshotMountPath, ReadOnly: readOnly,
		}}
		// Volume-touching commands run as root with exactly the DAC bypass a
		// backup needs. The operator image is USER 65532, and a non-root uid
		// cannot read a real ext4 volume: lost+found is root-owned 0700, and
		// tenant files carry whatever uid the service wrote them with — so
		// every backup died with "permission denied" on the first real PVC
		// (found live by the w2/m86 prod drill; the CAPD e2e's local-path
		// volume is a 777 hostPath dir, which hid it). fsGroup is not an
		// alternative: kubelet would recursively chown/chmod the tenant's
		// files at publish time. All three added capabilities are on the PSS
		// baseline allowlist (tenant namespaces enforce baseline); restore
		// additionally needs CHOWN/FOWNER to re-create archived ownership, and
		// backup's read-only bind makes the write caps inert there.
		container.SecurityContext = &corev1.SecurityContext{
			RunAsUser:                new(int64(0)),
			RunAsGroup:               new(int64(0)),
			AllowPrivilegeEscalation: new(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
				Add:  []corev1.Capability{"DAC_OVERRIDE", "CHOWN", "FOWNER"},
			},
			SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		}
		// The claim SOURCE is deliberately never ReadOnly, only the container's
		// bind above. A read-only source makes the CSI node-publish itself ro,
		// and on the LUKS class that is a direct `mount -o ro` of the mapper
		// device — which the kernel refuses (EBUSY) while the service's own pod
		// holds the same device rw, so a running app's nightly backup could
		// never mount (found live by the w2/m86 prod drill). Publishing rw
		// shares the existing superblock; the container still sees a read-only
		// bind, which is the guarantee that matters.
		podSpec.Volumes = []corev1.Volume{{
			Name: diskVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: diskPVCName(app.Name),
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
		BackoffLimit:          new(int32(2)),
		ActiveDeadlineSeconds: new(diskBackupDeadlineSeconds),
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

// markDiskSnapshots records cleanup authority before any snapshot writer is
// created or removed. It survives a configuration loss or manager restart.
func (r *AppReconciler) markDiskSnapshots(ctx context.Context, app *appv1alpha1.App) error {
	if app.Annotations[annotDiskSnapshots] == string(app.UID) && app.Annotations[annotDiskPurgeComplete] == "" {
		return nil
	}
	return r.patchAppMeta(ctx, app, func(meta *metav1.ObjectMeta) {
		metav1.SetMetaDataAnnotation(meta, annotDiskSnapshots, string(app.UID))
		delete(meta.Annotations, annotDiskPurgeComplete)
	})
}

func (r *AppReconciler) markDiskSnapshotCleanup(ctx context.Context, app *appv1alpha1.App) error {
	if app.Annotations[annotDiskPurgePending] == string(app.UID) {
		return nil
	}
	return r.patchAppMeta(ctx, app, func(meta *metav1.ObjectMeta) {
		metav1.SetMetaDataAnnotation(meta, annotDiskSnapshots, string(app.UID))
		metav1.SetMetaDataAnnotation(meta, annotDiskPurgePending, string(app.UID))
	})
}

// reconcileDiskSnapshotCleanup stops every owned writer before purging, then
// retains the App until the successful purge Job and its Pods are observed gone.
func (r *AppReconciler) reconcileDiskSnapshotCleanup(ctx context.Context, app *appv1alpha1.App) (bool, error) {
	hadDisk := app.Spec.Disk != nil || app.Annotations[annotDiskProvisioned] != ""
	if !hadDisk && app.Annotations[annotDiskSnapshots] == "" {
		return true, nil
	}
	if workspace, alias := app.Labels[labelWorkspace], app.Labels["bex.co/tenant"]; app.Namespace == "" ||
		(workspace != "" && workspace != app.Namespace) || (alias != "" && alias != app.Namespace) {
		return false, fmt.Errorf("disk snapshot cleanup scope does not match the App namespace")
	}
	for _, key := range []string{annotDiskSnapshots, annotDiskPurgePending, annotDiskPurgeComplete} {
		if marker := app.Annotations[key]; marker != "" && marker != string(app.UID) {
			return false, fmt.Errorf("disk snapshot cleanup belongs to a different App lifetime")
		}
	}
	if hadDisk && r.DiskSnapshots.configured() && app.Annotations[annotDiskSnapshots] == "" {
		if err := r.markDiskSnapshots(ctx, app); err != nil {
			return false, err
		}
	}
	if app.Annotations[annotDiskSnapshots] != "" {
		if err := r.markDiskSnapshotCleanup(ctx, app); err != nil {
			return false, err
		}
	}
	writersGone, err := r.quiesceDiskSnapshotWriters(ctx, app)
	if err != nil || !writersGone {
		return false, err
	}
	if app.Annotations[annotDiskSnapshots] == "" {
		return true, nil // a disk in a cluster where snapshots were never enabled
	}
	if app.UID == "" {
		return false, fmt.Errorf("cannot purge disk snapshots without an App UID")
	}
	if app.Annotations[annotDiskPurgeComplete] != string(app.UID) {
		if err := r.prepareDiskSnapshotPurge(ctx, app); err != nil {
			return false, err
		}
	}
	job, err := r.diskSnapshotPurgeJob(app)
	if err != nil {
		return false, err
	}
	return reconcileCleanupJob(ctx, r.Client, app, job, annotDiskPurgeComplete)
}

func (r *AppReconciler) prepareDiskSnapshotPurge(ctx context.Context, app *appv1alpha1.App) error {
	store := r.DiskSnapshots
	if store.Endpoint == "" || store.Bucket == "" || store.S3Secret == "" || r.BackupHelperImage == "" {
		return fmt.Errorf("disk snapshot purge configuration is unavailable")
	}
	cl := r.uncachedSecretClient()
	if src := store.SourceNamespace; src != "" && src != app.Namespace {
		if err := projectBackupCredential(ctx, cl, src, app.Namespace, store.S3Secret); err != nil {
			return fmt.Errorf("project disk snapshot purge credential: %w", err)
		}
	}
	if err := validateAWSCredentialSecret(ctx, cl, app.Namespace, store.S3Secret); err != nil {
		return fmt.Errorf("disk snapshot purge %w", err)
	}
	return nil
}

func (r *AppReconciler) diskSnapshotPurgeJob(app *appv1alpha1.App) (*batchv1.Job, error) {
	labels := (execution.ArtifactIdentity{Name: app.Name, UID: string(app.UID),
		Workspace: diskSnapshotWorkspace(app), Namespace: app.Namespace}).Labels(diskPurgeComponent)
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: diskPurgeName(app), Namespace: app.Namespace, Labels: labels,
	}}
	job.Spec = r.diskSnapshotJobSpec(app, labels, "purge", nil)
	if err := controllerutil.SetControllerReference(app, job, r.Scheme); err != nil {
		return nil, err
	}
	return job, nil
}

func diskSnapshotWriterOwned(app *appv1alpha1.App, obj metav1.Object, parents map[types.UID]bool) bool {
	owner := metav1.GetControllerOf(obj)
	if owner != nil && owner.Kind == "App" && owner.UID != app.UID {
		return false
	}
	if uid := obj.GetLabels()[execution.LabelAppUID]; uid != "" {
		return (execution.ArtifactIdentity{Name: app.Name, UID: string(app.UID)}).Owns(obj)
	}
	return owner != nil && ((owner.Kind == "App" && owner.UID == app.UID && app.UID != "") || parents[owner.UID])
}

func (r *AppReconciler) quiesceDiskSnapshotWriters(ctx context.Context, app *appv1alpha1.App) (bool, error) {
	// Legacy writers lack App UID labels. Foreground deletion retains their
	// CronJob/Job owner chain until the dependent Pods are gone.
	parents := map[types.UID]bool{}
	var objects []client.Object
	cron := &batchv1.CronJob{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: diskBackupName(app.Name)}, cron); err == nil {
		if diskSnapshotWriterOwned(app, cron, parents) {
			parents[cron.UID] = true
			objects = append(objects, cron)
		} else if app.Annotations[annotDiskSnapshots] != "" {
			return false, fmt.Errorf("disk snapshot schedule belongs to a different App lifetime")
		}
	} else if !apierrors.IsNotFound(err) {
		return false, err
	}
	selector := client.MatchingLabels{labelApp: app.Name}
	isWriter := func(obj metav1.Object) bool {
		return obj.GetLabels()[execution.LabelComponent] == diskBackupComponent || obj.GetLabels()["app.kubernetes.io/name"] == diskBackupComponent
	}
	var jobs batchv1.JobList
	if err := r.List(ctx, &jobs, client.InNamespace(app.Namespace), selector); err != nil {
		return false, err
	}
	for i := range jobs.Items {
		if !isWriter(&jobs.Items[i]) {
			continue
		}
		if !diskSnapshotWriterOwned(app, &jobs.Items[i], parents) {
			if app.Annotations[annotDiskSnapshots] != "" {
				return false, fmt.Errorf("disk snapshot Job %s belongs to an unverified App lifetime", jobs.Items[i].Name)
			}
			continue
		}
		parents[jobs.Items[i].UID] = true
		objects = append(objects, &jobs.Items[i])
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(app.Namespace), selector); err != nil {
		return false, err
	}
	for i := range pods.Items {
		if !isWriter(&pods.Items[i]) {
			continue
		}
		if !diskSnapshotWriterOwned(app, &pods.Items[i], parents) {
			if app.Annotations[annotDiskSnapshots] != "" {
				return false, fmt.Errorf("disk snapshot Pod %s belongs to an unverified App lifetime", pods.Items[i].Name)
			}
			continue
		}
		objects = append(objects, &pods.Items[i])
	}
	if len(objects) == 0 {
		return true, nil
	}
	if err := r.markDiskSnapshotCleanup(ctx, app); err != nil {
		return false, err
	}
	for _, obj := range objects {
		if obj.GetDeletionTimestamp().IsZero() {
			uid := obj.GetUID()
			if err := r.Delete(ctx, obj, client.Preconditions{UID: &uid}, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil && !apierrors.IsNotFound(err) {
				return false, err
			}
		}
	}
	return false, nil
}

// snapshotPrefixFor is the object-store prefix a disk's snapshots live under —
// shared with the backend so a listing and a Job agree on where to look.
func snapshotPrefixFor(app *appv1alpha1.App) string {
	return disksnapshot.DiskPrefix(diskSnapshotWorkspace(app), app.Name)
}

// --- Restore (ADR082 D5) ---

const (
	diskRestorePrefix = "dskrst-"
	// annotDiskRestored records the snapshot the last completed restore used.
	// It is what makes the intent edge-triggered: the operator acts when
	// spec.disk.restoreSnapshot differs from this, so a finished restore does
	// not loop, and re-requesting the SAME snapshot deliberately runs again.
	annotDiskRestored = "app.bex.co/disk-restored"
	// annotDiskRestoreRequest records, on the restore Job, which request it
	// serves (diskRestoreRequest).
	annotDiskRestoreRequest = "app.bex.co/disk-restore-request"
	// diskRestorePoll is how often a mid-restore App is re-checked. A restore
	// streams a whole volume back, so this is a progress poll, not a timeout.
	diskRestorePoll = 15 * time.Second
)

func diskRestoreName(appName string) string {
	return appv1alpha1.DiskChildName(diskRestorePrefix, appName)
}

// reconcileDiskRestore runs a requested restore to completion.
//
// The sequence is forced by the volume: a ReadWriteOnce disk cannot be rewritten
// underneath a running service, and a half-restored filesystem must never be
// served. So the service is scaled to zero first, the Job gets the freed volume
// to itself, and only a SUCCEEDED Job clears the request — a failed one leaves
// the intent in place, visible, and retryable, rather than quietly bringing the
// service back up on partially-restored data.
//
// It reports whether the App is mid-restore, which halts the rest of the
// reconcile so nothing scales the service back up while the Job is running.
func (r *AppReconciler) reconcileDiskRestore(ctx context.Context, app *appv1alpha1.App) (bool, error) {
	if app.Spec.Disk == nil || app.Spec.Disk.RestoreSnapshot == "" {
		return false, nil
	}
	requested := app.Spec.Disk.RestoreSnapshot
	request := diskRestoreRequest(app)
	if app.Annotations[annotDiskRestored] == request {
		return false, nil // already served this request
	}
	if !r.DiskSnapshots.restorable() || r.BackupHelperImage == "" {
		// Never start a restore that cannot finish: the Job wipes the volume
		// before it extracts, so one that fails to start correctly would
		// destroy data and restore nothing.
		setDiskCondition(app, false, "SnapshotStoreUnavailable",
			"a snapshot restore was requested but the snapshot store, decrypt key, or operator image is not available")
		return false, nil
	}

	job := &batchv1.Job{}
	key := client.ObjectKey{Namespace: app.Namespace, Name: diskRestoreName(app.Name)}
	switch err := r.Get(ctx, key, job); {
	case apierrors.IsNotFound(err):
		// Scale to zero BEFORE the Job exists: the restore container mounts the
		// same claim, and starting it while the service still holds the volume
		// would either block on attach or, worse, rewrite files under a live
		// process.
		if err := r.scaleDeploymentToZero(ctx, app); err != nil {
			return true, err
		}
		if ready, err := r.diskDetachedFromService(ctx, app); err != nil || !ready {
			setDiskCondition(app, false, "DiskRestorePending", "stopping the service before restoring its disk")
			return true, err
		}
		return true, r.createDiskRestoreJob(ctx, app, requested, request)
	case err != nil:
		return true, err
	}

	if job.DeletionTimestamp != nil {
		setDiskCondition(app, false, "DiskRestorePending", "clearing the previous restore before this one")
		return true, nil
	}
	if restoreJobRequest(job) != request {
		if job.Status.Succeeded == 0 && job.Status.Failed == 0 {
			// A restore for an earlier request is still writing the volume.
			// Never interrupt it: the new one starts after it ends.
			setDiskCondition(app, false, "DiskRestoreRunning", "finishing an earlier restore before this one")
			return true, nil
		}
		// A finished Job for an earlier request (a retry after a failure, or
		// the same snapshot asked for again). Clear it; the next pass stops
		// the service and starts this request's Job.
		if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil &&
			!apierrors.IsNotFound(err) {
			return true, err
		}
		setDiskCondition(app, false, "DiskRestorePending", "clearing the previous restore before this one")
		return true, nil
	}

	switch {
	case job.Status.Succeeded > 0:
		// Record what was restored, delete the Job, and let the normal reconcile
		// bring the service back on the restored volume.
		if err := r.patchAppMeta(ctx, app, func(meta *metav1.ObjectMeta) {
			metav1.SetMetaDataAnnotation(meta, annotDiskRestored, request)
		}); err != nil {
			return true, err
		}
		if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil &&
			!apierrors.IsNotFound(err) {
			return true, err
		}
		setDiskCondition(app, true, "DiskRestored", fmt.Sprintf("restored the disk from %s", requested))
		return false, nil
	case job.Status.Failed > 0:
		// Deliberately terminal: the volume may hold a partial extraction, so
		// the service stays down and the request stays pending until an owner
		// retries or clears it. Bringing it up here would serve half a disk.
		setDiskCondition(app, false, "DiskRestoreFailed",
			fmt.Sprintf("restoring the disk from %s failed; the service is stopped and the request is still pending", requested))
		return true, nil
	default:
		setDiskCondition(app, false, "DiskRestoreRunning", fmt.Sprintf("restoring the disk from %s", requested))
		return true, nil
	}
}

// scaleDeploymentToZero takes the service to zero without touching
// spec.replicas, so the owner's own instance count is what it comes back to —
// for a disk restore, and for a canceled release nothing served (w1/132).
func (r *AppReconciler) scaleDeploymentToZero(ctx context.Context, app *appv1alpha1.App) error {
	dep := &appsv1.Deployment{}
	key := client.ObjectKey{Namespace: app.Namespace, Name: app.Name}
	if err := r.Get(ctx, key, dep); err != nil {
		if apierrors.IsNotFound(err) {
			return nil // nothing running; the volume is already free
		}
		return err
	}
	if dep.Spec.Replicas != nil && *dep.Spec.Replicas == 0 {
		return nil
	}
	base := dep.DeepCopy()
	dep.Spec.Replicas = new(int32(0))
	return r.Patch(ctx, dep, client.MergeFrom(base))
}

// diskDetachedFromService reports whether the service's pods are gone, which is
// what frees a ReadWriteOnce volume for the restore Job to mount.
//
// It reads the Deployment's own status rather than listing pods by app label.
// The snapshot Jobs' pods carry that same label and linger after completing
// (the CronJob keeps a few for history), so a label list would count a finished
// backup pod as "the service is still running" and the restore would never
// start — which is exactly what happened the first time this ran on a cluster.
func (r *AppReconciler) diskDetachedFromService(ctx context.Context, app *appv1alpha1.App) (bool, error) {
	dep := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, dep); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil // nothing to hold the volume
		}
		return false, err
	}
	// Status.Replicas counts pods the Deployment still owns, terminating ones
	// included, so it goes to zero exactly when the volume is released.
	return dep.Status.Replicas == 0, nil
}

// diskRestoreRequest identifies the owner's current restore request: the
// snapshot plus when it was asked for, or the snapshot alone on a CR from
// before requests were stamped, so no earlier restore is ever re-run.
func diskRestoreRequest(app *appv1alpha1.App) string {
	requested := app.Spec.Disk.RestoreSnapshot
	if at := app.Annotations[appv1alpha1.DiskRestoreRequestedAtAnnotation]; at != "" {
		return requested + "@" + at
	}
	return requested
}

// restoreJobRequest is the request a restore Job serves. A Job created before
// requests were recorded serves its snapshot alone.
func restoreJobRequest(job *batchv1.Job) string {
	if request, ok := job.Annotations[annotDiskRestoreRequest]; ok {
		return request
	}
	for _, c := range job.Spec.Template.Spec.Containers {
		for _, e := range c.Env {
			if e.Name == "BEX_DISK_SNAPSHOT_KEY" {
				return e.Value
			}
		}
	}
	return ""
}

func (r *AppReconciler) createDiskRestoreJob(ctx context.Context, app *appv1alpha1.App, object, request string) error {
	labels := diskBackupLabels(app)
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: diskRestoreName(app.Name), Namespace: app.Namespace, Labels: labels,
		Annotations: map[string]string{annotDiskRestoreRequest: request},
	}}
	job.Spec = r.diskSnapshotJobSpec(app, labels, "restore", []corev1.EnvVar{
		{Name: "BEX_DISK_SNAPSHOT_KEY", Value: object},
		{Name: "AGE_PRIVATE_KEY", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: r.DiskSnapshots.AgeSecret},
			Key:                  diskAgePrivateKeyKey,
		}}},
	})
	// One attempt. A restore is destructive and re-running it automatically
	// would repeat the wipe; an owner decides whether to try again.
	job.Spec.BackoffLimit = new(int32(0))
	if err := controllerutil.SetControllerReference(app, job, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}
