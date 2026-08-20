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
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/publish"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	kvFinalizer                   = "app.bex.co/kv-finalizer"
	annotKVBackupPurgeComplete    = "app.bex.co/kv-backup-purge-complete"
	keyValueBackupComponent       = "keyvalue-backup"
	keyValueBackupPurgeComponent  = "keyvalue-backup-purge"
	keyValueBackupRetention       = 7
	keyValueBackupDeadlineSeconds = int64(15 * time.Minute / time.Second)
	// keyValueBackupAgeImage is the Alpine base for the ADR050 Tier A encrypt
	// step. Digest-pinned (round-14 #5): this container reads the plaintext
	// backup volume, so a retagged upstream tag must not become its code. Only
	// pulled/used when encryption is enabled (BackupStore.AgePublicKey set).
	// The age binary itself is NOT installed via apk — see ageReleaseVersion /
	// ageReleaseSHA256 below (round-16 #11, mirrors etcd/OpenBao backup charts).
	keyValueBackupAgeImage = "alpine:3.21@sha256:48b0309ca019d89d40f670aa1bc06e426dc0931948452e8491e3d65087abc07d"
	// ageReleaseVersion + ageReleaseSHA256 pin FiloSottile/age v1.3.1
	// (age-v1.3.1-linux-amd64.tar.gz), same reviewed artifact as the etcd and
	// OpenBao backup CronJobs. A mismatch fails the Job before encrypt, so a
	// tampered download never becomes an unencrypted upload.
	ageReleaseVersion = "1.3.1"
	ageReleaseSHA256  = "bdc69c09cbdd6cf8b1f333d372a1f58247b3a33146406333e30c0f26e8f51377"
	// keyValueBackupBusyboxImage compresses the plaintext RDB (round-14 #5):
	// digest-pinned like the build preparer's busybox so the mutable tag cannot
	// resolve to different bytes.
	keyValueBackupBusyboxImage = "busybox:1.37@sha256:9db7b59979c38555a39def84a31fb98b5296952f9e3afd4f6f11f05b07adfab0"
)

// keyValueBackupWorkHeadroomGiB is the slack added on top of the derived peak
// so a backup that is merely at the top of its plan does not fail (codex
// round-5 F10).
const keyValueBackupWorkHeadroomGiB = 1

// keyValueBackupWorkBudget bounds the backup Job's EmptyDir and its containers'
// ephemeral storage, derived per instance rather than as one global constant so
// a starter instance cannot claim a standard instance's ceiling.
//
// The derivation, from the pipeline in keyValueBackupCronJobSpec:
//
//	snapshot  writes /backup/dump.rdb                        => S
//	compress  gzip -9 dump.rdb (both files exist mid-run)     => S + S(worst case,
//	          because gzip on incompressible data is ~1.0x)      i.e. PEAK = 2S
//	encrypt   age -o dump.rdb.gz.age dump.rdb.gz, then rm     => <= 2 * gz size <= 2S
//	upload    reads one file                                  => unchanged
//
// So the peak is 2S, at the compress step — the pipeline deletes as it goes, so
// the three representations never coexist. S is bounded by the instance's
// ALLOCATED storage rather than its memory: maxmemory is only applied when a
// MaxmemoryPolicy is chosen (see keyvalue_controller.go), so memory is not a
// reliable ceiling on the dataset, while the PVC always is.
//
// Revisit this if the pipeline ever stops deleting intermediates, or if a plan
// gains a storage size far above the current 5 GiB top.
func keyValueBackupWorkBudget(kv *appv1alpha1.KeyValue, plan tiers.ValkeyTier) resource.Quantity {
	storageGB := int64(kv.Status.AllocatedStorageGB)
	if intent := int64(kv.Spec.StorageGB); intent > storageGB {
		storageGB = intent
	}
	if base := int64(plan.StorageGB); base > storageGB {
		storageGB = base
	}
	if storageGB < 1 {
		storageGB = 1
	}
	return *resource.NewQuantity((2*storageGB+keyValueBackupWorkHeadroomGiB)*(1<<30), resource.BinarySI)
}

// backupResources is guaranteedResources plus an ephemeral-storage bound. The
// EmptyDir's own SizeLimit is not sufficient on its own: an EmptyDir's usage
// counts against the POD's ephemeral-storage limit, so a container without one
// leaves the node's eviction manager as the only thing standing between a large
// tenant backup and its co-scheduled neighbours.
func backupResources(cpu, memory string, ephemeral resource.Quantity) corev1.ResourceRequirements {
	list := corev1.ResourceList{
		corev1.ResourceCPU:              resource.MustParse(cpu),
		corev1.ResourceMemory:           resource.MustParse(memory),
		corev1.ResourceEphemeralStorage: ephemeral,
	}
	return corev1.ResourceRequirements{Requests: list, Limits: list}
}

func keyValueBackupsEnabled(plan tiers.ValkeyTier, store BackupStore) bool {
	return plan.ID != tiers.Valkey.Default().ID && store.configured()
}

func keyValueBackupName(name string) string {
	const prefix = "kvbak-"
	candidate := prefix + name
	if len(candidate) <= 63 {
		return candidate
	}
	sum := sha256.Sum256([]byte(name))
	return fmt.Sprintf("%s%.43s-%x", prefix, name, sum[:4])
}

// keyValueBackupSchedule spreads tenant snapshots across 03:20–03:39 UTC.
// The resource id makes the choice stable across reconciles and restarts.
func keyValueBackupSchedule(name string) string {
	sum := sha256.Sum256([]byte(name))
	return fmt.Sprintf("%d 3 * * *", 20+int(sum[0])%20)
}

func keyValueBackupLabels(kv *appv1alpha1.KeyValue, component string) map[string]string {
	labels := map[string]string{
		labelKeyValue:              kv.Name,
		execution.LabelComponent:   component,
		execution.LabelKeyValueUID: string(kv.UID),
	}
	if workspace := kv.Labels[labelWorkspace]; workspace != "" {
		labels[labelWorkspace] = workspace
	}
	return labels
}

// reconcileTenantBackupCredential projects the S3 credential into a KeyValue's
// own namespace — the Secret half of ADR043 D8.4, without the ObjectStore (the
// KeyValue backup path talks to S3 directly from a CronJob rather than through
// the Barman plugin). A no-op when the KeyValue already sits in the source
// namespace, so the pre-D8 topology stays byte-identical.
func (r *KeyValueReconciler) reconcileTenantBackupCredential(ctx context.Context, kv *appv1alpha1.KeyValue) error {
	src := r.BackupSourceNamespace
	if src == "" || src == kv.Namespace {
		return nil
	}
	return projectBackupCredential(ctx, r.secretClient(), src, kv.Namespace, r.Backup.S3Secret)
}

func (r *KeyValueReconciler) reconcileKeyValueBackup(
	ctx context.Context,
	kv *appv1alpha1.KeyValue,
	plan tiers.ValkeyTier,
	authSecretName string,
) error {
	name := keyValueBackupName(kv.Name)
	if !keyValueBackupsEnabled(plan, r.Backup) {
		cron := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: kv.Namespace}}
		// Existence check first, via the same primitive the deletion path uses:
		// with backups off this CronJob is absent on every pass, and a blind
		// Delete is a live API round trip that deletes nothing (w7/m84).
		if _, err := deleteAndWait(ctx, r.Client, cron); err != nil {
			return fmt.Errorf("delete disabled KeyValue backup CronJob: %w", err)
		}
		return nil
	}

	// The CronJob mounts the S3 credential by name from its own namespace, so a
	// KeyValue in a tenant namespace (ADR043 D8.4) needs it projected there
	// first. Without this the CronJob is created happily and every run fails at
	// mount time — a nightly backup that silently never produces a snapshot.
	if err := r.reconcileTenantBackupCredential(ctx, kv); err != nil {
		return err
	}

	cron := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: kv.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, cron, func() error {
		cron.Labels = keyValueBackupLabels(kv, keyValueBackupComponent)
		cron.Spec = r.keyValueBackupCronJobSpec(kv, plan, authSecretName)
		return controllerutil.SetControllerReference(kv, cron, r.Scheme)
	})
	return err
}

func (r *KeyValueReconciler) keyValueBackupCronJobSpec(kv *appv1alpha1.KeyValue, plan tiers.ValkeyTier, authSecretName string) batchv1.CronJobSpec {
	failedHistory := int32(3)
	successfulHistory := int32(3)
	backoff := int32(2)
	startingDeadline := int64(time.Hour / time.Second)
	timeZone := "Etc/UTC"
	labels := keyValueBackupLabels(kv, keyValueBackupComponent)
	volumeMount := corev1.VolumeMount{Name: "backup", MountPath: "/backup"}
	workBudget := keyValueBackupWorkBudget(kv, plan)

	// ADR050 Tier A: age-encrypt the RDB before upload when a public key is
	// configured. The encrypt step slots between compress and upload; the object
	// suffix follows (.rdb.gz.age when encrypted, .rdb.gz when not).
	encrypt := r.Backup.AgePublicKey != ""
	uploadSuffix := "rdb.gz"
	uploadSource := "/backup/dump.rdb.gz"
	if encrypt {
		uploadSuffix = "rdb.gz.age"
		uploadSource = "/backup/dump.rdb.gz.age"
	}

	initContainers := []corev1.Container{
		{
			Name:    "snapshot",
			Image:   valkeyImage(kv.Spec.Version),
			Command: []string{"/bin/sh", "-ceu"},
			Args: []string{`rm -f /backup/dump.rdb
valkey-cli -h "${VALKEY_HOST}" -p "6379" --rdb /backup/dump.rdb
test -s /backup/dump.rdb`},
			Env: []corev1.EnvVar{
				{Name: "VALKEY_HOST", Value: kv.Name},
				{Name: "REDISCLI_AUTH", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: authSecretName}, Key: "password",
				}}},
			},
			Resources:       backupResources("100m", "128Mi", workBudget),
			SecurityContext: tenantSecCtx(),
			VolumeMounts:    []corev1.VolumeMount{volumeMount},
		},
		{
			Name:            "compress",
			Image:           keyValueBackupBusyboxImage,
			Command:         []string{"gzip", "-9", "/backup/dump.rdb"},
			Resources:       backupResources("10m", "32Mi", workBudget),
			SecurityContext: tenantSecCtx(),
			VolumeMounts:    []corev1.VolumeMount{volumeMount},
		},
	}
	if encrypt {
		initContainers = append(initContainers, corev1.Container{
			Name:    "encrypt",
			Image:   keyValueBackupAgeImage,
			Command: []string{"/bin/sh", "-ceu"},
			// NO RUNTIME PACKAGE INSTALL (round-16 #11 / ADR050 etcd pattern):
			// fetch one pinned release artifact, verify SHA-256, then encrypt.
			Args: []string{`cd /tmp
wget -q -O age.tgz \
  "https://github.com/FiloSottile/age/releases/download/v${AGE_VERSION}/age-v${AGE_VERSION}-linux-amd64.tar.gz"
echo "${AGE_SHA256}  age.tgz" | sha256sum -c -
tar xzf age.tgz age/age
./age/age -r "${AGE_PUBLIC_KEY}" -o /backup/dump.rdb.gz.age /backup/dump.rdb.gz
rm -f /backup/dump.rdb.gz`},
			Env: []corev1.EnvVar{
				{Name: "AGE_PUBLIC_KEY", Value: r.Backup.AgePublicKey},
				{Name: "AGE_VERSION", Value: ageReleaseVersion},
				{Name: "AGE_SHA256", Value: ageReleaseSHA256},
			},
			Resources:       backupResources("50m", "64Mi", workBudget),
			SecurityContext: tenantSecCtx(),
			VolumeMounts:    []corev1.VolumeMount{volumeMount},
		})
	}

	spec := batchv1.CronJobSpec{
		Schedule:                   keyValueBackupSchedule(kv.Name),
		TimeZone:                   &timeZone,
		ConcurrencyPolicy:          batchv1.ForbidConcurrent,
		Suspend:                    ptr.To(kv.Spec.Suspended),
		StartingDeadlineSeconds:    &startingDeadline,
		FailedJobsHistoryLimit:     &failedHistory,
		SuccessfulJobsHistoryLimit: &successfulHistory,
		JobTemplate: batchv1.JobTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: labels},
			Spec: batchv1.JobSpec{
				BackoffLimit:          &backoff,
				ActiveDeadlineSeconds: ptr.To(keyValueBackupDeadlineSeconds),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					Spec: corev1.PodSpec{
						RestartPolicy:                corev1.RestartPolicyNever,
						AutomountServiceAccountToken: ptr.To(false),
						InitContainers:               initContainers,
						Containers: []corev1.Container{{
							Name:    "upload",
							Image:   publish.DefaultAWSCLIImage,
							Command: []string{"/bin/bash", "-ceu"},
							Args: []string{fmt.Sprintf(`retain=%d
aws configure set default.s3.addressing_style path
timestamp="$(date -u +%%Y-%%m-%%dT%%H:%%M:%%SZ)"
prefix="${DESTINATION%%/}/${KEYVALUE}/"
aws --endpoint-url "${ENDPOINT}" s3 cp %s "${prefix}${timestamp}.%s"
aws --endpoint-url "${ENDPOINT}" s3 ls "${prefix}" \
  | awk '{print $NF}' \
  | grep -E '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z[.]rdb[.]gz([.]age)?$' \
  | sort \
  | head -n "-${retain}" \
  | while IFS= read -r old; do
      aws --endpoint-url "${ENDPOINT}" s3 rm "${prefix}${old}"
    done`, keyValueBackupRetention, uploadSource, uploadSuffix)},
							Env: []corev1.EnvVar{
								{Name: "HOME", Value: "/tmp"},
								{Name: "AWS_EC2_METADATA_DISABLED", Value: "true"},
								{Name: "DESTINATION", Value: strings.TrimRight(r.Backup.DestinationPath, "/")},
								{Name: "ENDPOINT", Value: r.Backup.EndpointURL},
								{Name: "KEYVALUE", Value: kv.Name},
							},
							EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: r.Backup.S3Secret},
							}}},
							Resources:       backupResources("50m", "128Mi", workBudget),
							SecurityContext: tenantSecCtx(),
							VolumeMounts:    []corev1.VolumeMount{volumeMount},
						}},
						Volumes: []corev1.Volume{{Name: "backup", VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: &workBudget},
						}}},
					},
				},
			},
		},
	}
	// Last, always — this pod template is built from nothing every pass. See
	// server_defaults.go.
	applyPodSpecServerDefaults(&spec.JobTemplate.Spec.Template.Spec)
	return spec
}

func (r *KeyValueReconciler) handleKeyValueDeletion(ctx context.Context, kv *appv1alpha1.KeyValue) (result ctrl.Result, err error) {
	if !controllerutil.ContainsFinalizer(kv, kvFinalizer) {
		return result, nil
	}

	cron := &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: keyValueBackupName(kv.Name), Namespace: kv.Namespace}}
	if gone, err := deleteAndWait(ctx, r.Client, cron); err != nil {
		return result, fmt.Errorf("delete KeyValue backup CronJob: %w", err)
	} else if !gone {
		return ctrl.Result{RequeueAfter: settleRequeue}, nil
	}

	jobsGone, err := r.deleteKeyValueBackupJobs(ctx, kv)
	if err != nil {
		return result, err
	}
	if !jobsGone {
		return ctrl.Result{RequeueAfter: settleRequeue}, nil
	}

	if r.Backup.configured() {
		done, err := reconcileCleanupJob(ctx, r.Client, kv, r.keyValueBackupPurgeJob(kv), annotKVBackupPurgeComplete)
		if err != nil {
			return result, err
		}
		if !done {
			return ctrl.Result{RequeueAfter: settleRequeue}, nil
		}
	}

	controllerutil.RemoveFinalizer(kv, kvFinalizer)
	if err := r.Update(ctx, kv); err != nil {
		return result, err
	}
	return result, nil
}

func (r *KeyValueReconciler) deleteKeyValueBackupJobs(ctx context.Context, kv *appv1alpha1.KeyValue) (bool, error) {
	var jobs batchv1.JobList
	if err := r.List(ctx, &jobs, client.InNamespace(kv.Namespace), client.MatchingLabels{
		labelKeyValue:          kv.Name,
		"app.bex.co/component": keyValueBackupComponent,
	}); err != nil {
		return false, fmt.Errorf("list KeyValue backup Jobs: %w", err)
	}
	for idx := range jobs.Items {
		job := &jobs.Items[idx]
		if job.DeletionTimestamp.IsZero() {
			if err := r.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil && !apierrors.IsNotFound(err) {
				return false, fmt.Errorf("delete KeyValue backup Job %s: %w", job.Name, err)
			}
		}
	}
	return len(jobs.Items) == 0, nil
}

func (r *KeyValueReconciler) keyValueBackupPurgeJob(kv *appv1alpha1.KeyValue) *batchv1.Job {
	backoff := int32(3)
	name := cleanupJobName("purge-kv-", kv.Name, kv.UID)
	labels := keyValueBackupLabels(kv, keyValueBackupPurgeComponent)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: kv.Namespace, Labels: labels},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoff,
			ActiveDeadlineSeconds: ptr.To(keyValueBackupDeadlineSeconds),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy:                corev1.RestartPolicyNever,
					AutomountServiceAccountToken: ptr.To(false),
					Containers: []corev1.Container{{
						Name:    "purge",
						Image:   publish.DefaultAWSCLIImage,
						Command: []string{"/bin/bash", "-ceu"},
						Args: []string{`aws configure set default.s3.addressing_style path
aws --endpoint-url "${ENDPOINT}" s3 rm "${DESTINATION%/}/${KEYVALUE}/" --recursive`},
						Env: []corev1.EnvVar{
							{Name: "HOME", Value: "/tmp"},
							{Name: "AWS_EC2_METADATA_DISABLED", Value: "true"},
							{Name: "DESTINATION", Value: strings.TrimRight(r.Backup.DestinationPath, "/")},
							{Name: "ENDPOINT", Value: r.Backup.EndpointURL},
							{Name: "KEYVALUE", Value: kv.Name},
						},
						EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: r.Backup.S3Secret},
						}}},
						// The purge Job only issues S3 deletes — it mounts no work
						// volume, so it needs no ephemeral-storage budget.
						Resources:       guaranteedResources("50m", "128Mi"),
						SecurityContext: tenantSecCtx(),
					}},
				},
			},
		},
	}
}
