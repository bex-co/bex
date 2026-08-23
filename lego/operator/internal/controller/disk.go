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
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Persistent service disks (docs/ADR082-persistent-disks.md). A service with
// spec.disk gets one ReadWriteOnce PVC mounted at spec.disk.mountPath, and with
// it the two Deployment-shape changes a single shared volume forces: Recreate
// deploys (two revisions must never write one volume) and exemption from
// autoscaler consolidation.
//
// Everything here is inert for an App without spec.disk — no PVC, no Secret, no
// strategy, no annotation — which is what keeps the stateless fleet byte-identical.
const (
	// diskVolumeName is the pod-template volume name for the attached disk.
	diskVolumeName = "disk"
	// diskPVCPrefix names the per-App claim. A service has at most one disk, so
	// the App's own name is enough to identify it.
	diskPVCPrefix = "disk-"
	// diskLUKSSecretSuffix names the per-disk encryption passphrase Secret, and
	// diskLUKSSecretKey is the data key the hcloud CSI driver reads it from.
	diskLUKSSecretSuffix = "-luks"
	diskLUKSSecretKey    = "encryption-passphrase"
	// diskSafeToEvictAnnotation keeps the cluster-autoscaler from bin-packing a
	// disk-bearing pod's node away. Same annotation, same reason as the managed
	// KeyValue StatefulSet: for a single-instance stateful pod an eviction is
	// downtime. Manual and CAPI drains (node upgrades) still evict it.
	diskSafeToEvictAnnotation = "cluster-autoscaler.kubernetes.io/safe-to-evict"
	// diskPassphraseBytes is the entropy behind a disk's LUKS passphrase.
	diskPassphraseBytes = 32
	// annotDiskProvisioned marks an App whose disk children exist, so a detach
	// knows there is something to delete without reading the cluster.
	//
	// It is metadata rather than status on purpose. Status is written after the
	// spec it describes and is dropped by any in-memory object that round-trips
	// through an Update, so a status-based check can miss a detach and orphan a
	// volume the tenant keeps paying for. The marker is also written BEFORE the
	// PVC exists, so it can never under-report a child that does.
	annotDiskProvisioned = "app.bex.co/disk-provisioned"
	// diskProvisionedMarker is the only value annotDiskProvisioned ever carries.
	diskProvisionedMarker = "true"
)

// diskPVCName and diskLUKSSecretName derive the disk's child object names from
// the App's. Both truncate to Kubernetes' 63-character limit the same way the
// other derived names in this package do, keeping a digest so two long App
// names cannot collide onto one volume.
func diskPVCName(appName string) string { return derivedDiskName(diskPVCPrefix, appName, "") }

func diskLUKSSecretName(appName string) string {
	return derivedDiskName(diskPVCPrefix, appName, diskLUKSSecretSuffix)
}

func derivedDiskName(prefix, appName, suffix string) string {
	if len(prefix)+len(appName)+len(suffix) <= 63 {
		return prefix + appName + suffix
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(appName)))[:8]
	keep := 63 - len(prefix) - len(suffix) - len(sum) - 1
	return prefix + appName[:keep] + "-" + sum + suffix
}

// applyDiskProjection writes the disk's half of the pod template. Like every
// other field applyDeploymentSpec sets, it is rebuilt from the App each pass, so
// detaching a disk restores rolling deploys and re-admits the pod to autoscaler
// consolidation instead of leaving both pinned forever.
func applyDiskProjection(dep *appsv1.Deployment, container *corev1.Container, app *appv1alpha1.App) {
	if app.Spec.Disk == nil {
		// Touch the strategy only when a disk previously set it. A service that
		// never had one keeps whatever the API server defaulted, so its stored
		// Deployment stays byte-identical to a pre-disk operator's.
		if dep.Spec.Strategy.Type == appsv1.RecreateDeploymentStrategyType {
			dep.Spec.Strategy = appsv1.DeploymentStrategy{}
		}
		delete(dep.Spec.Template.Annotations, diskSafeToEvictAnnotation)
		return
	}
	// Recreate, not RollingUpdate: Kubernetes must stop the old pod before
	// starting the new one, because a ReadWriteOnce volume cannot be attached to
	// both and, more importantly, two revisions writing one filesystem corrupt
	// it. This is the zero-downtime deploy the disk's owner traded away — the
	// same trade Render makes, and the reason spec.disk is opt-in per service.
	dep.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = map[string]string{}
	}
	dep.Spec.Template.Annotations[diskSafeToEvictAnnotation] = "false"

	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      diskVolumeName,
		MountPath: app.Spec.Disk.MountPath,
	})
	dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: diskVolumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: diskPVCName(app.Name),
			},
		},
	})
}

// reconcileDisk converges the App's disk: the passphrase Secret, then the PVC —
// created at spec size, grown (never shrunk) when the spec asks for more.
//
// A blocked grow is not an error. The volume the service is already running on
// keeps working at its current size, so a quota ceiling or a non-expandable
// StorageClass is recorded on the DiskReady condition and retried on the next
// steady-state pass rather than failing the whole reconcile into a hot loop.
func (r *AppReconciler) reconcileDisk(ctx context.Context, app *appv1alpha1.App) error {
	if app.Spec.Disk == nil {
		return r.cleanupDisk(ctx, app)
	}
	if err := r.markDiskProvisioned(ctx, app); err != nil {
		return err
	}
	if err := r.ensureDiskPassphrase(ctx, app); err != nil {
		return err
	}
	pvc := &corev1.PersistentVolumeClaim{}
	key := client.ObjectKey{Namespace: app.Namespace, Name: diskPVCName(app.Name)}
	switch err := r.Get(ctx, key, pvc); {
	case apierrors.IsNotFound(err):
		return r.createDiskPVC(ctx, app)
	case err != nil:
		return err
	}
	return r.growDiskPVC(ctx, app, pvc)
}

// ensureDiskPassphrase mints the disk's LUKS passphrase once, on first
// reconcile. It is never rewritten: the passphrase is the only thing that can
// unlock the volume, so regenerating it would lock the tenant out of their own
// data. Kubernetes garbage-collects it with the App via the owner reference.
//
// The Secret is minted whether or not the configured StorageClass encrypts, so
// that moving a cluster onto the encrypted class later needs no backfill.
func (r *AppReconciler) ensureDiskPassphrase(ctx context.Context, app *appv1alpha1.App) error {
	secret := &corev1.Secret{}
	key := client.ObjectKey{Namespace: app.Namespace, Name: diskLUKSSecretName(app.Name)}
	if err := r.Get(ctx, key, secret); err == nil {
		return nil
	} else if !apierrors.IsNotFound(err) {
		return err
	}
	passphrase := make([]byte, diskPassphraseBytes)
	if _, err := rand.Read(passphrase); err != nil {
		return fmt.Errorf("generate disk passphrase: %w", err)
	}
	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Labels:    map[string]string{labelApp: app.Name},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			diskLUKSSecretKey: base64.RawStdEncoding.EncodeToString(passphrase),
		},
	}
	if err := controllerutil.SetControllerReference(app, secret, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (r *AppReconciler) createDiskPVC(ctx context.Context, app *appv1alpha1.App) error {
	sizeGB := max(app.Spec.Disk.SizeGB, diskAllocatedGB(app))
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      diskPVCName(app.Name),
			Namespace: app.Namespace,
			Labels:    map[string]string{labelApp: app.Name},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: diskQuantity(sizeGB)},
			},
		},
	}
	// Unset leaves StorageClassName nil, which selects the cluster's default
	// class. That is what makes the local CAPD cluster (whose default is
	// local-path) work without pretending to have Hetzner volumes, while
	// production names the encrypted hcloud class explicitly.
	if r.DiskStorageClass != "" {
		pvc.Spec.StorageClassName = ptr.To(r.DiskStorageClass)
	}
	if err := controllerutil.SetControllerReference(app, pvc, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, pvc); err != nil && !apierrors.IsAlreadyExists(err) {
		if isQuotaExceeded(err) {
			setDiskCondition(app, false, "StorageBlockedByQuota",
				fmt.Sprintf("namespace storage quota has no room for a %dGB disk: %v", sizeGB, err))
			return nil
		}
		return err
	}
	setDiskStatus(app, sizeGB, 0)
	setDiskCondition(app, false, "WaitingForPVC", "waiting for the disk volume to be created")
	return nil
}

// growDiskPVC raises the claim's request when the spec asks for more space.
// Storage only ever grows: a shrink is refused by the CRD's own transition rule
// before it reaches the operator, and the high-water arithmetic here means even
// a spec that somehow regressed cannot shrink a live volume.
func (r *AppReconciler) growDiskPVC(ctx context.Context, app *appv1alpha1.App, pvc *corev1.PersistentVolumeClaim) error {
	requestedGB := pvcRequestedStorageGB(pvc)
	capacityGB := pvcCapacityStorageGB(pvc)
	allocatedGB := max(app.Spec.Disk.SizeGB, requestedGB, capacityGB, diskAllocatedGB(app))

	if allocatedGB > requestedGB {
		if blocked, err := r.expandDiskPVC(ctx, app, pvc, allocatedGB); err != nil || blocked {
			return err
		}
	}
	setDiskStatus(app, allocatedGB, capacityGB)

	// The capacity lagging the request is the normal middle of a grow, not a
	// fault: the hcloud CSI driver's maintainers do not claim online expansion,
	// so the filesystem step can land only when the pod next restarts. Say so
	// rather than reporting a disk that is quietly the wrong size.
	if capacityGB > 0 && capacityGB < allocatedGB {
		setDiskCondition(app, false, "DiskResizePending", fmt.Sprintf(
			"volume expanded to %dGB; the filesystem still reports %dGB and finishes growing on the next restart",
			allocatedGB, capacityGB))
		return nil
	}
	if pvc.Status.Phase != corev1.ClaimBound {
		setDiskCondition(app, false, "WaitingForPVCBinding", "waiting for the disk volume to bind")
		return nil
	}
	setDiskCondition(app, true, "DiskReady", fmt.Sprintf("%dGB disk mounted at %s", allocatedGB, app.Spec.Disk.MountPath))
	return nil
}

// expandDiskPVC patches the claim to desiredGB. It reports blocked=true when an
// eligibility gate stopped the grow, having already recorded why on the App.
func (r *AppReconciler) expandDiskPVC(ctx context.Context, app *appv1alpha1.App, pvc *corev1.PersistentVolumeClaim, desiredGB int32) (bool, error) {
	block := func(reason, message string) (bool, error) {
		setDiskStatus(app, desiredGB, pvcCapacityStorageGB(pvc))
		setDiskCondition(app, false, reason, message)
		return true, nil
	}
	className := ""
	if pvc.Spec.StorageClassName != nil {
		className = *pvc.Spec.StorageClassName
	}
	outcome, err := expandPVCTo(ctx, r.Client, pvc, desiredGB)
	if err != nil {
		return false, err
	}
	switch outcome {
	case pvcExpanded:
		return false, nil
	case pvcExpandWaitingForBinding:
		return block("WaitingForPVCBinding", "waiting for the disk volume to bind before requesting expansion")
	case pvcExpandStorageClassMissing:
		return block("StorageClassMissing", "disk volume has no StorageClass; it cannot be expanded")
	case pvcExpandStorageClassNotFound:
		return block("StorageClassNotFound", fmt.Sprintf("StorageClass %q was not found; cannot expand the disk", className))
	case pvcExpandNotExpandable:
		return block("StorageClassNotExpandable", fmt.Sprintf("StorageClass %q does not allow volume expansion", className))
	case pvcExpandQuotaBlocked:
		return block("StorageBlockedByQuota", fmt.Sprintf("namespace storage quota blocked growing the disk to %dGB; growth resumes when headroom is available", desiredGB))
	}
	return false, nil
}

// cleanupDisk removes the volume and its passphrase when spec.disk is cleared.
// Detaching a disk is destructive by definition — the API surface that offers it
// is where the confirmation lives; by the time a cleared spec reaches the
// operator the decision has been made.
func (r *AppReconciler) cleanupDisk(ctx context.Context, app *appv1alpha1.App) error {
	if app.Annotations[annotDiskProvisioned] == "" {
		// Never had a disk — every stateless App in the fleet, on every pass.
		// The marker means this costs nothing: no API call, and no PVC informer
		// started in a manager that has no other reason to watch them.
		return nil
	}
	// Blind deletes rather than the usual get-then-delete: this path runs a
	// handful of times per detach, so one round trip each beats populating a
	// cluster-wide PVC cache to check.
	for _, obj := range []client.Object{
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: diskPVCName(app.Name), Namespace: app.Namespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: diskLUKSSecretName(app.Name), Namespace: app.Namespace}},
	} {
		if err := r.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	// Clear the marker last: while it is set the cleanup is idempotent and
	// retried, so a delete that failed halfway is finished by the next pass
	// instead of leaving a volume nobody is looking for.
	base := app.DeepCopy()
	delete(app.Annotations, annotDiskProvisioned)
	if err := r.Patch(ctx, app, client.MergeFrom(base)); err != nil {
		return err
	}
	app.Status.Disk = nil
	meta.RemoveStatusCondition(&app.Status.Conditions, appv1alpha1.ConditionDiskReady)
	return nil
}

// markDiskProvisioned stamps the App before its first disk child is created.
func (r *AppReconciler) markDiskProvisioned(ctx context.Context, app *appv1alpha1.App) error {
	if app.Annotations[annotDiskProvisioned] == diskProvisionedMarker {
		return nil
	}
	base := app.DeepCopy()
	metav1.SetMetaDataAnnotation(&app.ObjectMeta, annotDiskProvisioned, diskProvisionedMarker)
	return r.Patch(ctx, app, client.MergeFrom(base))
}

func diskQuantity(sizeGB int32) resource.Quantity {
	return resource.MustParse(fmt.Sprintf("%dGi", sizeGB))
}

// diskAllocatedGB is the size already accepted for this disk, or 0 for a disk
// that has never been reconciled.
func diskAllocatedGB(app *appv1alpha1.App) int32 {
	if app.Status.Disk == nil {
		return 0
	}
	return app.Status.Disk.AllocatedSizeGB
}

func setDiskStatus(app *appv1alpha1.App, allocatedGB, capacityGB int32) {
	if app.Status.Disk == nil {
		app.Status.Disk = &appv1alpha1.DiskStatus{}
	}
	app.Status.Disk.AllocatedSizeGB = allocatedGB
	app.Status.Disk.ObservedSizeGB = app.Spec.Disk.SizeGB
	app.Status.Disk.CapacityGB = capacityGB
}

func setDiskCondition(app *appv1alpha1.App, ready bool, reason, message string) {
	status := metav1.ConditionFalse
	if ready {
		status = metav1.ConditionTrue
	}
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionDiskReady, Status: status, Reason: reason,
		Message: message, ObservedGeneration: app.Generation,
	})
}
