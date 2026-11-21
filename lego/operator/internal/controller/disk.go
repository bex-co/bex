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
	"encoding/base64"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/bex-co/bex/lego/operator/internal/build"
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
	// diskLUKSSecretKey is the data key the hcloud CSI driver reads a disk's
	// encryption passphrase from. The Secret's NAME (and the PVC's) is derived
	// in lego/types, because bex-api must derive the same claim name to graph
	// the disk's usage.
	diskLUKSSecretKey = "encryption-passphrase"
	// These non-secret fields carry current logical intent only to the CSI node.
	// The bounded-ext4 StorageClass opts in; old classes ignore both fields.
	diskFilesystemModeKey  = "boundedFilesystem"
	diskFilesystemMode     = "ext4-v1"
	diskFilesystemBytesKey = "filesystemSizeBytes"
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
	// annotDiskResizeRequested carries the size, in GB, of the last grow the
	// node was asked to finish, stamped on each running instance mounting the
	// disk (w6/m557). The kubelet grows a mounted volume only when it syncs the
	// volume's pod, about once a minute by default, so a claim waiting for the
	// node sat there 7.5-63.7 s on production (w6/157). Any pod update starts
	// that sync: the filesystem grew 0.35 s after one. The
	// bex-operator-pod-labels admission policy admits this one annotation on a
	// pod mounting a disk.
	annotDiskResizeRequested = "app.bex.co/disk-resize-requested"
	// annotDiskLayout marks a disk claim made since a native service runs from
	// its root directory (w6/m558), with diskLayoutRootDirectory, its only
	// value. A claim without it was made before, when every native service
	// but Go ran from /opt/render/project/src itself, so a path under it named
	// a place in the service's own tree (diskMountPaths).
	annotDiskLayout         = "app.bex.co/disk-layout"
	diskLayoutRootDirectory = "root-directory"
)

// diskPVCName and diskLUKSSecretName are this package's names for the disk's
// child objects. The derivation itself lives in the leaf contract module
// because bex-api must reproduce the claim name exactly to graph the disk's
// usage — see appv1alpha1.DiskPVCName.
func diskPVCName(appName string) string { return appv1alpha1.DiskPVCName(appName) }

func diskLUKSSecretName(appName string) string { return appv1alpha1.DiskLUKSSecretName(appName) }

// applyDiskProjection writes the disk's half of the pod template. Like every
// other field applyDeploymentSpec sets, it is rebuilt from the App each pass, so
// detaching a disk restores rolling deploys and re-admits the pod to autoscaler
// consolidation instead of leaving both pinned forever. move is where a disk
// made before w6/m558 is in the service's tree now (diskMountPaths).
func applyDiskProjection(dep *appsv1.Deployment, container *corev1.Container, app *appv1alpha1.App, move diskMove) {
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

	for _, mountPath := range diskMountPaths(app.Spec.Disk.MountPath, move) {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      diskVolumeName,
			MountPath: mountPath,
		})
	}
	dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, corev1.Volume{
		Name: diskVolumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: diskPVCName(app.Name),
			},
		},
	})
}

// diskMove is where a disk made before w6/m558 is in its service's tree now,
// and whether its own path would hide that tree (movedDiskMount); the zero
// value for a disk that stays where it is.
type diskMove struct {
	path  string
	hides bool
}

// diskMountPaths is where the container mounts a disk at mountPath: there,
// and where move puts it. Until w6/m558 a native service other than Go ran
// from /opt/render/project/src, holding its root directory's contents, so a
// disk at /opt/render/project/src/uploads held what it wrote to uploads/. It
// now runs from /opt/render/project/src/<root directory>, as on Render, so a
// disk made before is also mounted at /opt/render/project/src/<root
// directory>/uploads, and keeps both the relative and the absolute path it
// had. A path that is the new directory or one of its parents would hide the
// service's source, so that disk is mounted only at its new place.
func diskMountPaths(mountPath string, move diskMove) []string {
	switch {
	case move.path == "":
		return []string{mountPath}
	case move.hides:
		return []string{move.path}
	}
	return []string{mountPath, move.path}
}

// movedDiskMount is where mountPath, read as before w6/m558, is in the tree
// of a service running source's build now. ok is false when that service's
// working directory did not move (no root directory, Go, a Docker or prebuilt
// image) or the path names no place in its old tree. A path already in the
// root directory is where Render puts it, and stays.
func movedDiskMount(source appv1alpha1.AppSpec, mountPath string) (move diskMove, ok bool) {
	if effectiveBuilder(source) != build.BuilderNative {
		return diskMove{}, false
	}
	rootDir, found := strings.CutPrefix(build.NativeSourceDir(source.Runtime, source.Repo, source.RootDir), build.NativeWorkdir+"/")
	if !found {
		return diskMove{}, false
	}
	rel, found := strings.CutPrefix(path.Clean(mountPath), build.NativeWorkdir+"/")
	if !found || strings.HasPrefix(rel, rootDir+"/") {
		return diskMove{}, false
	}
	return diskMove{
		path:  build.NativeWorkdir + "/" + rootDir + "/" + rel,
		hides: rel == rootDir || strings.HasPrefix(rootDir, rel+"/"),
	}, true
}

// earlierDiskMove is where app's disk moves (movedDiskMount) when its claim
// predates w6/m558, which a claim without annotDiskLayout does; the zero value
// otherwise, and for a disk with no claim yet, which is a new one. A service
// linked to a build source runs the source's build, so its directory is the
// source's.
func (r *AppReconciler) earlierDiskMove(ctx context.Context, app *appv1alpha1.App) (diskMove, error) {
	if app.Spec.Disk == nil {
		return diskMove{}, nil
	}
	source := app.Spec
	if app.Spec.BuildSource != nil {
		carrier, err := r.linkedCarrier(ctx, app)
		if err != nil {
			// No source to read is no build to run, and nothing to mount in it.
			return diskMove{}, nil
		}
		source = carrier.Spec
	}
	move, ok := movedDiskMount(source, app.Spec.Disk.MountPath)
	if !ok {
		return diskMove{}, nil
	}
	var pvc corev1.PersistentVolumeClaim
	switch err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: diskPVCName(app.Name)}, &pvc); {
	case apierrors.IsNotFound(err):
		return diskMove{}, nil
	case err != nil:
		return diskMove{}, fmt.Errorf("read the disk claim: %w", err)
	}
	if pvc.Annotations[annotDiskLayout] == diskLayoutRootDirectory {
		return diskMove{}, nil
	}
	return move, nil
}

// storedDiskMountPaths is every path a stored pod spec mounts the disk at.
func storedDiskMountPaths(spec corev1.PodSpec) []string {
	var paths []string
	for _, c := range spec.Containers {
		for _, m := range c.VolumeMounts {
			if m.Name == diskVolumeName {
				paths = append(paths, m.MountPath)
			}
		}
	}
	return paths
}

// preferDiskNode asks the scheduler to start a disk-bearing App's next
// instance on node, the one its volume is attached to (w6/m547). Hetzner
// volumes are bound to a location, not a node, so the scheduler is otherwise
// free to choose any node there, and on production it chose another node in
// half the rollouts measured. Each move cost 27-32s of Recreate downtime: the
// old node reporting the volume free, a 9s detach and a 15s attach. A
// preference, never a requirement: a node that is cordoned, draining, full or
// gone is skipped and the volume moves, as before. node is "" on a first
// deploy, which has no volume to follow.
//
// It must run after execution.PlaceOnServingPool, which resets the affinity,
// and adds to the serving pool's required term rather than replacing it.
func preferDiskNode(spec *corev1.PodSpec, app *appv1alpha1.App, node string) {
	if app.Spec.Disk == nil || node == "" {
		return
	}
	if spec.Affinity == nil {
		spec.Affinity = &corev1.Affinity{}
	}
	if spec.Affinity.NodeAffinity == nil {
		spec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
	}
	spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = []corev1.PreferredSchedulingTerm{{
		Weight: 100,
		Preference: corev1.NodeSelectorTerm{MatchFields: []corev1.NodeSelectorRequirement{{
			Key:      diskNodeField,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{node},
		}}},
	}}
}

// diskNodeField is the node field preferDiskNode matches on: the Node's own
// name, which a hostname label need not equal.
const diskNodeField = "metadata.name"

// storedDiskNode is the node a stored pod spec prefers (preferDiskNode), ""
// for none.
func storedDiskNode(spec corev1.PodSpec) string {
	if spec.Affinity == nil || spec.Affinity.NodeAffinity == nil {
		return ""
	}
	for _, term := range spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
		for _, field := range term.Preference.MatchFields {
			if field.Key == diskNodeField && field.Operator == corev1.NodeSelectorOpIn && len(field.Values) == 1 {
				return field.Values[0]
			}
		}
	}
	return ""
}

// diskNode is the node a disk-bearing App's volume is on: the node of its
// serving instance, or, mid-rollout, of the instance being replaced. "" when
// no instance has a node or the lookup fails. The preference is a hint, never
// worth failing a reconcile over, and applyDeploymentSpec falls back to the
// node the stored template already prefers.
func (r *AppReconciler) diskNode(ctx context.Context, app *appv1alpha1.App) string {
	if app.Spec.Disk == nil {
		return ""
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(app.Namespace), client.MatchingLabels{labelApp: app.Name}); err != nil {
		return ""
	}
	node := ""
	for i := range pods.Items {
		pod := &pods.Items[i]
		owner := metav1.GetControllerOf(pod)
		if pod.Spec.NodeName == "" || owner == nil || owner.Kind != kindReplicaSet {
			continue
		}
		if pod.DeletionTimestamp == nil && pod.Status.Phase == corev1.PodRunning {
			return pod.Spec.NodeName
		}
		node = pod.Spec.NodeName
	}
	return node
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
// reconcile. The passphrase is never rewritten: it is the only thing that can
// unlock the volume, so regenerating it would lock the tenant out of their own
// data. Kubernetes garbage-collects it with the App via the owner reference.
//
// The Secret is minted whether or not the configured StorageClass encrypts, so
// that moving a cluster onto the encrypted class later needs no backfill.
func (r *AppReconciler) ensureDiskPassphrase(ctx context.Context, app *appv1alpha1.App) error {
	// Uncached on both reads and writes: the disk's Secret lives in the App's
	// OWN namespace, which the manager's single-namespace Secret informer does
	// not cover — see uncachedSecretClient.
	secrets := r.uncachedSecretClient()
	secret := &corev1.Secret{}
	key := client.ObjectKey{Namespace: app.Namespace, Name: diskLUKSSecretName(app.Name)}
	if err := secrets.Get(ctx, key, secret); err == nil {
		if len(secret.Data[diskLUKSSecretKey]) == 0 && secret.StringData[diskLUKSSecretKey] == "" {
			return fmt.Errorf("disk passphrase Secret %s is missing its encryption key; refusing to replace or use it", key)
		}
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
	if err := secrets.Create(ctx, secret); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// syncDiskFilesystemIntent authorizes only a request admitted by the PVC API.
// App spec/status can name a grow refused by quota or the StorageClass, and the
// expander mutates its in-memory PVC even when its patch fails. Neither is
// proof that the node may enlarge the filesystem.
func (r *AppReconciler) syncDiskFilesystemIntent(ctx context.Context, app *appv1alpha1.App, admittedGB int32) error {
	secrets := r.uncachedSecretClient()
	secret := &corev1.Secret{}
	if err := secrets.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: diskLUKSSecretName(app.Name)}, secret); err != nil {
		return err
	}
	return updateDiskFilesystemIntent(ctx, secrets, secret, admittedGB)
}

// updateDiskFilesystemIntent preserves the passphrase and the high-water bound
// of previously admitted requests, including across stale reconciles.
func updateDiskFilesystemIntent(ctx context.Context, secrets client.Client, secret *corev1.Secret, admittedGB int32) error {
	currentMode := string(secret.Data[diskFilesystemModeKey])
	if currentMode != "" && currentMode != diskFilesystemMode {
		return fmt.Errorf("unsupported disk filesystem mode %q", currentMode)
	}
	if admittedGB < 1 || admittedGB > 10000 {
		return fmt.Errorf("invalid admitted disk filesystem capacity")
	}
	desired := int64(admittedGB) << 30
	current := int64(0)
	if raw := string(secret.Data[diskFilesystemBytesKey]); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 1<<30 || value > 10000<<30 || value%(1<<30) != 0 {
			return fmt.Errorf("invalid existing disk filesystem capacity")
		}
		current = value
	}
	desired = max(desired, current)
	if currentMode == diskFilesystemMode && current == desired {
		return nil
	}
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[diskFilesystemModeKey] = []byte(diskFilesystemMode)
	secret.Data[diskFilesystemBytesKey] = []byte(strconv.FormatInt(desired, 10))
	return secrets.Update(ctx, secret)
}

func (r *AppReconciler) createDiskPVC(ctx context.Context, app *appv1alpha1.App) error {
	sizeGB := max(app.Spec.Disk.SizeGB, diskAllocatedGB(app))
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        diskPVCName(app.Name),
			Namespace:   app.Namespace,
			Labels:      map[string]string{labelApp: app.Name},
			Annotations: map[string]string{annotDiskLayout: diskLayoutRootDirectory},
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
		pvc.Spec.StorageClassName = new(r.DiskStorageClass)
	}
	if err := controllerutil.SetControllerReference(app, pvc, r.Scheme); err != nil {
		return err
	}
	if err := r.Create(ctx, pvc); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// A competing reconcile created it. Its request, which can differ from
			// this attempted create, will authorize the bound on the next read.
			return nil
		}
		if isQuotaExceeded(err) {
			setDiskCondition(app, false, "StorageBlockedByQuota",
				fmt.Sprintf("namespace storage quota has no room for a %dGB disk: %v", sizeGB, err))
			return nil
		}
		return err
	}
	// Publish the bound only after PVC creation passed admission. A node that
	// races this write fails closed and retries with the converged Secret.
	if err := r.syncDiskFilesystemIntent(ctx, app, sizeGB); err != nil {
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
	// Capture admitted intent before the expansion helper can mutate this PVC.
	if err := r.syncDiskFilesystemIntent(ctx, app, requestedGB); err != nil {
		return err
	}
	capacityGB := pvcCapacityStorageGB(pvc)
	// The allocation is what bex asked for, never the physical capacity: a
	// provider floor (Hetzner's 10 GB minimum volume) backs a 1 GB disk with a
	// 10 GB volume, and folding that in recorded the disk as 10 GB (w8/215, the
	// same rule growOnlyIntent states for datastores). A grow the volume
	// already holds (1 → 5 GB on a 10 GB volume) still raises the request. The
	// bounded CSI class reports logical capacity, so its node-only filesystem
	// growth runs even while the physical provider volume remains unchanged.
	allocatedGB := max(app.Spec.Disk.SizeGB, requestedGB, diskAllocatedGB(app))

	if allocatedGB > requestedGB {
		if blocked, err := r.expandDiskPVC(ctx, app, pvc, allocatedGB); err != nil || blocked {
			return err
		}
		// Only pvcExpanded with no error proves the API admitted this request.
		if err := r.syncDiskFilesystemIntent(ctx, app, allocatedGB); err != nil {
			return err
		}
	}
	setDiskStatus(app, allocatedGB, capacityGB)
	if targetGB := diskNodeExpansionTarget(pvc); targetGB > 0 {
		r.nudgeDiskPods(ctx, app, targetGB)
	}

	// The capacity lagging the request is the normal middle of a grow, not a
	// fault: the volume grows first, then the node grows the mounted
	// filesystem, both online. Only a node that reported it could not is left
	// to the restart, which mounts the volume afresh.
	if capacityGB > 0 && capacityGB < allocatedGB {
		message := fmt.Sprintf("growing the disk online to %dGB; the filesystem reports %dGB so far", allocatedGB, capacityGB)
		if diskNodeExpansionFailed(pvc) {
			message = fmt.Sprintf("the node could not grow the filesystem online past %dGB; it reaches %dGB on the next restart",
				capacityGB, allocatedGB)
		}
		setDiskCondition(app, false, "DiskResizePending", message)
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

// diskNodeExpansionTarget is the size, in GB, the node is to grow the claim's
// mounted filesystem to, or 0 while the claim does not wait for the node. It is
// the volume's allocated size, not the request: a grow requested while the
// node still owes the previous one must not mark that newer size as handed
// over before the volume reaches it.
func diskNodeExpansionTarget(pvc *corev1.PersistentVolumeClaim) int32 {
	if pvc.Status.AllocatedResourceStatuses[corev1.ResourceStorage] != corev1.PersistentVolumeClaimNodeResizePending &&
		!pvcFileSystemResizePending(pvc) {
		return 0
	}
	if allocated := quantityGiCeil(pvc.Status.AllocatedResources[corev1.ResourceStorage]); allocated > 0 {
		return allocated
	}
	return pvcRequestedStorageGB(pvc)
}

// diskNodeExpansionFailed reports a node that tried to grow the mounted
// filesystem and could not.
func diskNodeExpansionFailed(pvc *corev1.PersistentVolumeClaim) bool {
	if pvc.Status.AllocatedResourceStatuses[corev1.ResourceStorage] == corev1.PersistentVolumeClaimNodeResizeInfeasible {
		return true
	}
	return slices.ContainsFunc(pvc.Status.Conditions, func(c corev1.PersistentVolumeClaimCondition) bool {
		return c.Type == corev1.PersistentVolumeClaimNodeResizeError && c.Status == corev1.ConditionTrue
	})
}

// nudgeDiskPods starts the kubelet's sync of each running instance mounting
// the App's disk, once per size, so the node grows the filesystem now rather
// than on its next periodic sync (annotDiskResizeRequested). It patches the
// pod, never the Deployment's template, so nothing rolls. A failure is only
// logged: the periodic sync still finishes the grow.
func (r *AppReconciler) nudgeDiskPods(ctx context.Context, app *appv1alpha1.App, sizeGB int32) {
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(app.Namespace), client.MatchingLabels{labelApp: app.Name}); err != nil {
		logf.FromContext(ctx).Error(err, "listing the disk's instances to grow it failed", "app", app.Name)
		return
	}
	want := strconv.Itoa(int(sizeGB))
	for i := range pods.Items {
		pod := &pods.Items[i]
		owner := metav1.GetControllerOf(pod)
		serving := owner != nil && owner.Kind == kindReplicaSet &&
			pod.Status.Phase == corev1.PodRunning && pod.DeletionTimestamp.IsZero()
		if !serving || pod.Annotations[annotDiskResizeRequested] == want || !mountsDiskClaim(pod.Spec, app.Name) {
			continue
		}
		patch := client.MergeFrom(pod.DeepCopy())
		metav1.SetMetaDataAnnotation(&pod.ObjectMeta, annotDiskResizeRequested, want)
		if err := client.IgnoreNotFound(r.Patch(ctx, pod, patch)); err != nil {
			logf.FromContext(ctx).Error(err, "asking the node to grow the disk failed", "app", app.Name, "pod", pod.Name)
		}
	}
}

// diskClaimStatusChanged passes a disk claim's status changes: its capacity,
// allocated size, resize state and conditions are how a grow advances. Without
// them DiskReady waited for an unrelated event, up to 30 s after the
// filesystem had grown (w6/157), and nudgeDiskPods could not run the moment the
// claim began to wait for the node. Spec and metadata edits are the operator's
// own and need no pass.
var diskClaimStatusChanged = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		before, ok := e.ObjectOld.(*corev1.PersistentVolumeClaim)
		after, okAfter := e.ObjectNew.(*corev1.PersistentVolumeClaim)
		return ok && okAfter && !equality.Semantic.DeepEqual(before.Status, after.Status)
	},
}

// cleanupDisk removes the volume and its passphrase when spec.disk is cleared.
// Detaching a disk is destructive by definition — the API surface that offers it
// is where the confirmation lives; by the time a cleared spec reaches the
// operator the decision has been made.
func (r *AppReconciler) cleanupDisk(ctx context.Context, app *appv1alpha1.App) error {
	if app.Annotations[annotDiskProvisioned] == "" {
		// Never had a disk — every stateless App in the fleet, on every pass.
		// The marker means this costs nothing: no API call.
		return nil
	}
	if done, err := r.reconcileDiskSnapshotCleanup(ctx, app); err != nil || !done {
		return err
	}
	// A holder would keep the claim in use, and its deletion waiting.
	r.releaseDiskHold(ctx, app, diskHoldEndStopped)
	// Blind deletes rather than the usual get-then-delete: this path runs a
	// handful of times per detach, and a delete that finds nothing is already
	// the outcome it wants.
	//
	// The Secret goes through the uncached client for the same reason it was
	// created with one: it lives in the App's own namespace, outside the Secret
	// informer's single-namespace scope.
	deletes := []struct {
		cl  client.Client
		obj client.Object
	}{
		{r.Client, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: diskPVCName(app.Name), Namespace: app.Namespace}}},
		{r.uncachedSecretClient(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: diskLUKSSecretName(app.Name), Namespace: app.Namespace}}},
	}
	for _, d := range deletes {
		if err := d.cl.Delete(ctx, d.obj); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	// Clear the marker last: while it is set the cleanup is idempotent and
	// retried, so a delete that failed halfway is finished by the next pass
	// instead of leaving a volume nobody is looking for.
	if err := r.patchAppMeta(ctx, app, func(meta *metav1.ObjectMeta) {
		delete(meta.Annotations, annotDiskProvisioned)
		clearDiskSnapshotMarkers(meta)
	}); err != nil {
		return err
	}
	app.Status.Disk = nil
	meta.RemoveStatusCondition(&app.Status.Conditions, appv1alpha1.ConditionDiskReady)
	return nil
}

func clearDiskSnapshotMarkers(metadata *metav1.ObjectMeta) {
	delete(metadata.Annotations, annotDiskSnapshots)
	delete(metadata.Annotations, annotDiskPurgePending)
	delete(metadata.Annotations, annotDiskPurgeComplete)
}

// markDiskProvisioned stamps the App before its first disk child is created.
func (r *AppReconciler) markDiskProvisioned(ctx context.Context, app *appv1alpha1.App) error {
	if app.Annotations[annotDiskProvisioned] == diskProvisionedMarker {
		return nil
	}
	return r.patchAppMeta(ctx, app, func(meta *metav1.ObjectMeta) {
		metav1.SetMetaDataAnnotation(meta, annotDiskProvisioned, diskProvisionedMarker)
	})
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
