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
	"cmp"
	"context"
	"maps"
	"slices"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/types/deploybudgets"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The disk holder (w6/m559, docs/ADR082-persistent-disks.md D3) keeps a disk
// service's volume on its node through a Recreate deploy, restart or plan
// change whose new instance starts on that same node.
//
// Without it, nothing uses the volume between the old instance's teardown and
// the new instance's binding (0.3-0.6 s). The attach/detach controller detaches
// a volume that no scheduled pod uses once the node reports it free, so when
// the node's status sync fell inside that gap, Hetzner detached the volume and
// attached it again to the same node: 25.6 s more downtime, in one same-node
// swap in ten on production (m547 t005, run D1).
//
// The holder is a Job's one pod on the volume's node that mounts the claim and
// runs /pause. The operator starts it before it writes the template that rolls
// the instance, writes the template once the holder has the volume mounted, and
// ends it once the new instance has mounted the volume there, or at once when
// the new instance is bound to another node, whose volume must move. Through
// the swap the controller's desired state keeps the volume on that node, and
// the kubelet keeps it mounted and reported in use, so the new instance joins
// a volume it already holds. Any failure to hold is a rollout without one,
// never an error: a holder can delay a deploy by diskHoldMountBound while the
// old instance still serves, and never stop it.
//
// The operator cannot author a bare pod (its RBAC grants pods no create, and
// bex-operator-workloads refuses nodeName), so the holder is an App-owned Job
// placed by a required node-affinity term.
const (
	diskHolderPrefix    = "dskhld-"
	diskHolderComponent = appv1alpha1.ComponentDiskHolder
	diskHolderContainer = "hold"
	diskHolderMountPath = "/disk"
	// diskHolderImage is the tenant kubelets' own sandbox image (containerd
	// 2.3's default; every tenant node listing its images on 2026-11-21 had
	// it). containerd pins the sandbox image, so it is on every node and is
	// never garbage-collected: the holder starts with no pull, which bex's own
	// image cannot promise (a disk worker never pulls it, and an operator
	// upgrade changes it). /pause blocks until SIGTERM or SIGINT, exits 0, and
	// needs nothing writable. A node on another sandbox version pulls it once
	// (about 300 KB) inside diskHoldMountBound, while the old instance serves.
	diskHolderImage = "registry.k8s.io/pause:3.10.2"
	// diskHolderUser is the uid the pause image runs as.
	diskHolderUser = int64(65535)
	// diskHoldMountBound is how long a rollout waits for its holder to have
	// the volume mounted. The volume is attached and mounted on that node
	// already, so the holder's mount is a publish only: seconds.
	diskHoldMountBound = 15 * time.Second
	// diskHoldPoll is the wait between checks of a holder not mounted yet.
	// Its pod's own events wake the pass sooner.
	diskHoldPoll = time.Second
	// diskHoldMaxAge ends a holder whose rollout never reached an end the
	// operator saw. A Recreate rollout has the 15-minute budget.
	diskHoldMaxAge = deploybudgets.RolloutBudget
	// diskHolderTTLSeconds reaps a holder Job that ended by its own deadline.
	diskHolderTTLSeconds = int32(60)
)

// diskHolderDeadlineSeconds is the Job's own backstop, just past the rollout
// budget the operator ends it at.
var diskHolderDeadlineSeconds = int64(rolloutBudgetSeconds) + 60

// What a disk-bearing Recreate rollout did about the hold (diskHoldsTotal).
const (
	diskHoldHeld                 = "held"
	diskHoldFallbackRefused      = "fallback_refused"
	diskHoldFallbackFailed       = "fallback_failed"
	diskHoldFallbackTimeout      = "fallback_timeout"
	diskHoldSkippedNoInstance    = "skipped_no_instance"
	diskHoldSkippedUnschedulable = "skipped_unschedulable"
	diskHoldSkippedOtherPool     = "skipped_other_pool"
)

// Why the operator ended a holder (diskHoldEndsTotal).
const (
	diskHoldEndMounted  = "mounted"
	diskHoldEndMoved    = "moved"
	diskHoldEndSettled  = "settled"
	diskHoldEndStopped  = "stopped"
	diskHoldEndFinished = "finished"
	diskHoldEndExpired  = "expired"
	diskHoldEndFallback = "fallback"
)

var diskHoldsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "bex_disk_holds_total",
	Help: "Disk-bearing Recreate rollouts by how the volume was kept on its node: held (a holder had it mounted before the template changed), fallback_refused/fallback_failed/fallback_timeout (the rollout went on without one: the holder was refused, failed, or was not mounted in time), skipped_no_instance/skipped_unschedulable/skipped_other_pool (no holder started: no running instance, a node that takes no new pod, or a new template that cannot land there).",
}, []string{"result"})

var diskHoldEndsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "bex_disk_hold_ends_total",
	Help: "Disk holders the operator ended, by reason: mounted (the new instance has the volume on the holder's node), moved (the new instance was bound to another node), settled (no rollout in progress), stopped (nothing to hold for), finished, expired, fallback (the rollout went on without the hold).",
}, []string{"reason"})

func init() {
	ctrlmetrics.Registry.MustRegister(diskHoldsTotal, diskHoldEndsTotal)
}

func diskHolderName(appName string) string {
	return appv1alpha1.DiskChildName(diskHolderPrefix, appName)
}

// diskHolderJob is the Job whose one pod holds app's disk claim on node and
// does nothing else. tier is the compute tier of the template being rolled,
// whose serving pool the node is in when the tier names one.
//
// It is shaped like the disk snapshot Job beside a running service: the claim
// source is read-write and only the container's mount is read-only, because a
// read-only source makes the LUKS publish a `mount -o ro` of the mapper device
// the instance holds rw, which the kernel refuses (EBUSY, disk_backup.go). It
// carries the App's app label, so the App reconciles on its pod's events, and a
// component label, so no instance surface lists it (core.OwnAppPods). It never
// carries app.bex.co/app-id, which the Services select on.
func diskHolderJob(app *appv1alpha1.App, tier, node string, scheme *runtime.Scheme) (*batchv1.Job, error) {
	labels := execution.PodLabels(app.Name, string(app.UID), diskHolderComponent, "", "", false)
	spec := corev1.PodSpec{
		RestartPolicy:                corev1.RestartPolicyNever,
		AutomountServiceAccountToken: new(false),
		EnableServiceLinks:           new(false),
		// /pause exits on SIGTERM. The bound matters when the volume must move:
		// it cannot detach until the holder is gone.
		TerminationGracePeriodSeconds: new(int64(1)),
		Affinity:                      diskNodeAffinity(node),
		Containers: []corev1.Container{{
			Name:            diskHolderContainer,
			Image:           diskHolderImage,
			Command:         []string{"/pause"},
			ImagePullPolicy: corev1.PullIfNotPresent,
			VolumeMounts:    []corev1.VolumeMount{{Name: diskVolumeName, MountPath: diskHolderMountPath, ReadOnly: true}},
			Resources:       diskHolderResources(),
			SecurityContext: diskHolderSecurityContext(),
		}},
		Volumes: []corev1.Volume{{
			Name: diskVolumeName,
			VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: diskPVCName(app.Name),
			}},
		}},
	}
	// The node may be in the App's dedicated serving pool (w2/m250).
	execution.TolerateServingPool(&spec, tiers.Compute.ServingPool(tier))
	applyPodSpecServerDefaults(&spec)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: diskHolderName(app.Name), Namespace: app.Namespace, Labels: labels},
		Spec: batchv1.JobSpec{
			// One pod, never replaced: a holder that failed is a rollout
			// without one, not a retry.
			BackoffLimit:            new(int32(0)),
			ActiveDeadlineSeconds:   new(diskHolderDeadlineSeconds),
			TTLSecondsAfterFinished: new(diskHolderTTLSeconds),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: maps.Clone(labels)},
				Spec:       spec,
			},
		},
	}
	return job, controllerutil.SetControllerReference(app, job, scheme)
}

// diskNodeAffinity places a pod on node by a required node-affinity term on
// its name: bex-operator-workloads refuses nodeName.
func diskNodeAffinity(node string) *corev1.Affinity {
	return &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{
			MatchFields: []corev1.NodeSelectorRequirement{{Key: diskNodeField, Operator: corev1.NodeSelectorOpIn, Values: []string{node}}},
		}}},
	}}
}

// diskHolderSecurityContext is the holder's and the pre-pull's container
// hardening: restricted, as the pause image's non-root user, with a read-only
// root filesystem.
func diskHolderSecurityContext() *corev1.SecurityContext {
	security := execution.RestrictedContainerSecurityContext()
	security.RunAsNonRoot = new(true)
	security.RunAsUser, security.RunAsGroup = new(diskHolderUser), new(diskHolderUser)
	security.ReadOnlyRootFilesystem = new(true)
	return security
}

// diskHolderResources requests nothing, so the holder fits beside the
// instance on a node packed to its last millicore (diskSnapshotResources does
// the same), and limits what /pause could ever use. The explicit zeros keep
// tenant-limits' default requests (100m, 128Mi, 1Gi) off it.
//
// The CPU limit decides how fast the holder starts, not what /pause uses. The
// kubelet puts it on the pod's cgroup, so runc's init runs under that quota
// for the sandbox and again for the container, about 0.25 s of CPU in all. At
// 10m the holder took 24.7 s to start on production, past diskHoldMountBound,
// so no rollout held (w6/m559 t005). 500m, a Starter instance's CPU and the
// least any disk service runs on, starts it in about a second. A limit takes
// no room on a node, and a workspace's limits.cpu quota is twice its
// requests.cpu quota, which instances (whose limits equal their requests)
// cannot fill.
func diskHolderResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("0"),
			corev1.ResourceMemory:           resource.MustParse("0"),
			corev1.ResourceEphemeralStorage: resource.MustParse("0"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("500m"),
			corev1.ResourceMemory:           resource.MustParse("32Mi"),
			corev1.ResourceEphemeralStorage: resource.MustParse("16Mi"),
		},
	}
}

// diskJobNode is the node a holder or pre-pull Job's pod is placed on.
func diskJobNode(job *batchv1.Job) string {
	affinity := job.Spec.Template.Spec.Affinity
	if affinity == nil || affinity.NodeAffinity == nil || affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		return ""
	}
	for _, term := range affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
		for _, field := range term.MatchFields {
			if field.Key == diskNodeField && field.Operator == corev1.NodeSelectorOpIn && len(field.Values) == 1 {
				return field.Values[0]
			}
		}
	}
	return ""
}

// diskHoldStep is what a pass does about the hold before it writes the
// Deployment: wait > 0 halts the pass with the template unwritten, because a
// holder does not have the volume mounted yet or the new image may still be
// pulling onto its node; otherwise the template is written, held says a
// mounted holder covers that write, and prePull what the write waited for
// when the new template runs a new image (diskPrePullsTotal).
type diskHoldStep struct {
	wait    time.Duration
	held    bool
	prePull string
}

// holdDiskThroughRollout decides, before a pass writes a disk-bearing App's
// Deployment, whether the write waits for a holder, ends a holder its rollout
// no longer needs, and starts the pre-pull of a new instance whose volume
// moves. tier is the compute tier the template is projected for, and project
// is the pass's own write, run here on a copy of the stored Deployment. Every
// failure is logged and answered without a hold.
func (r *AppReconciler) holdDiskThroughRollout(ctx context.Context, app *appv1alpha1.App, tier string, replicas int32, project func(*appsv1.Deployment) *corev1.PodTemplateSpec) diskHoldStep {
	if app.Spec.Disk == nil && app.Annotations[annotDiskProvisioned] == "" {
		return diskHoldStep{} // never had a disk: every stateless App, no API call
	}
	log := logf.FromContext(ctx).WithValues("app", app.Name)
	holder, err := r.diskJob(ctx, app, diskHolderName(app.Name))
	if err != nil {
		log.Error(err, "reading the disk holder failed; rolling without it")
		return diskHoldStep{}
	}
	prePull, err := r.diskJob(ctx, app, diskPrePullName(app.Name))
	if err != nil {
		log.Error(err, "reading the disk pre-pull failed; rolling without it")
		return diskHoldStep{}
	}
	stored := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, stored); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "reading the Deployment for the disk hold failed; rolling without it")
			return diskHoldStep{}
		}
		// A first deploy: no instance has the volume yet.
		r.endOwnedDiskHold(ctx, app, holder, diskHoldEndStopped)
		r.endOwnedDiskPrePull(ctx, app, prePull, diskHoldEndStopped)
		return diskHoldStep{}
	}
	if holder == nil && prePull == nil &&
		(stored.Spec.Strategy.Type != appsv1.RecreateDeploymentStrategyType || !mountsDiskClaim(stored.Spec.Template.Spec, app.Name)) {
		return diskHoldStep{} // the running template shares no volume to hold
	}
	projected := stored.DeepCopy()
	project(projected)
	if equality.Semantic.DeepEqual(stored.Spec.Template, projected.Spec.Template) {
		// The volume is where the holder holds it, or else on the node the
		// template prefers: its instance's when the template was written.
		from := storedDiskNode(stored.Spec.Template.Spec)
		if holder != nil && metav1.IsControlledBy(holder, app) {
			from = cmp.Or(diskJobNode(holder), from)
			if reason := r.diskHoldEnd(ctx, app, holder, stored, replicas); reason != "" {
				r.endDiskHold(ctx, holder, reason)
			}
		}
		if !r.prePullOnMove(ctx, app, tier, from, prePull, stored, replicas) {
			r.settleDiskPrePull(ctx, app, prePull, stored, replicas)
		}
		return diskHoldStep{}
	}
	return r.startDiskHold(ctx, app, tier, holder, prePull, stored, projected, replicas)
}

// startDiskHold is holdDiskThroughRollout for a pass whose write would roll
// the template.
func (r *AppReconciler) startDiskHold(ctx context.Context, app *appv1alpha1.App, tier string, holder, prePull *batchv1.Job, stored, projected *appsv1.Deployment, replicas int32) diskHoldStep {
	log := logf.FromContext(ctx).WithValues("app", app.Name)
	// Nothing starts on the volume's node (a suspend), or one of the two
	// templates does not mount the disk (it is being attached or detached).
	if replicas == 0 || stored.Spec.Strategy.Type != appsv1.RecreateDeploymentStrategyType ||
		!mountsDiskClaim(stored.Spec.Template.Spec, app.Name) || !mountsDiskClaim(projected.Spec.Template.Spec, app.Name) {
		r.endOwnedDiskHold(ctx, app, holder, diskHoldEndStopped)
		r.endOwnedDiskPrePull(ctx, app, prePull, diskHoldEndStopped)
		return diskHoldStep{}
	}
	if holder != nil && !metav1.IsControlledBy(holder, app) {
		log.Info("a Job of another lifetime of this App has the disk holder's name; rolling without a hold", "job", holder.Name)
		return skipDiskHold(diskHoldFallbackRefused)
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(app.Namespace), client.MatchingLabels{labelApp: app.Name}); err != nil {
		log.Error(err, "listing the App's pods for the disk hold failed; rolling without it")
		r.endDiskHold(ctx, holder, diskHoldEndFallback)
		return skipDiskHold(diskHoldFallbackFailed)
	}
	node, holder, skipped := r.diskHoldNode(ctx, tier, holder, pods.Items, projected)
	if skipped != "" {
		// The volume must move, so the new instance may start anywhere.
		r.endOwnedDiskPrePull(ctx, app, prePull, diskHoldEndStopped)
		return skipDiskHold(skipped)
	}
	var step diskHoldStep
	if holder == nil {
		step = r.createDiskHolder(ctx, app, tier, node)
	} else {
		step = r.awaitDiskHolder(ctx, holder, pods.Items, node)
	}
	if step.wait == 0 && !step.held {
		// The rollout goes on without a hold, at once. A pull already
		// started goes on, and settleDiskPrePull ends it once it has.
		return step
	}
	pull := r.prePullThroughHold(ctx, app, tier, node, prePull, pods.Items,
		newTemplateImages(&stored.Spec.Template.Spec, &projected.Spec.Template.Spec), &projected.Spec.Template)
	switch {
	case step.wait > 0 && pull.wait > 0:
		return diskHoldStep{wait: min(step.wait, pull.wait)}
	case step.wait > 0 || pull.wait > 0:
		return diskHoldStep{wait: max(step.wait, pull.wait)}
	}
	step.prePull = pull.prePull
	return step
}

// count records what the template write this step let through waited for: a
// held write is one the hold found rolling the template.
func (s diskHoldStep) count() {
	if s.held {
		diskHoldsTotal.WithLabelValues(diskHoldHeld).Inc()
	}
	if s.prePull != "" {
		diskPrePullsTotal.WithLabelValues(s.prePull).Inc()
	}
}

// skipDiskHold counts a rollout that goes on without a hold.
func skipDiskHold(result string) diskHoldStep {
	diskHoldsTotal.WithLabelValues(result).Inc()
	return diskHoldStep{}
}

// diskHoldNode is the node to hold the volume on: where the instance being
// replaced runs, or, between two rollouts, where holder already holds it.
// skipped names why there is none, having ended holder, and holder comes back
// nil when it held the volume somewhere it no longer is. The volume must be
// free to move when the new instance cannot start on its node: a node that is
// cordoned, tainted or gone, or a plan change onto another pool.
func (r *AppReconciler) diskHoldNode(ctx context.Context, tier string, holder *batchv1.Job, pods []corev1.Pod, projected *appsv1.Deployment) (node string, kept *batchv1.Job, skipped string) {
	if instance := servingInstance(pods); instance != nil {
		node = instance.Spec.NodeName
	}
	if holder != nil {
		if held := diskJobNode(holder); node == "" {
			node = held
		} else if held != node {
			r.endDiskHold(ctx, holder, diskHoldEndMoved)
			holder = nil
		}
	}
	if node == "" {
		return "", nil, diskHoldSkippedNoInstance
	}
	skip := func(result string) (string, *batchv1.Job, string) {
		r.endDiskHold(ctx, holder, diskHoldEndStopped)
		return "", nil, result
	}
	n := &corev1.Node{}
	if err := cmp.Or(r.APIReader, client.Reader(r.Client)).Get(ctx, client.ObjectKey{Name: node}, n); err != nil {
		if !apierrors.IsNotFound(err) {
			logf.FromContext(ctx).Error(err, "reading the disk's node failed; rolling without a hold", "node", node)
		}
		return skip(diskHoldSkippedUnschedulable)
	}
	if !nodeTakesPod(n, diskHolderTolerations(tier)) {
		return skip(diskHoldSkippedUnschedulable)
	}
	if !nodeTakesPod(n, projected.Spec.Template.Spec.Tolerations) || !nodeMatchesPod(n, &projected.Spec.Template.Spec) {
		return skip(diskHoldSkippedOtherPool)
	}
	return node, holder, ""
}

// createDiskHolder starts app's holder on node; the rollout waits for it.
func (r *AppReconciler) createDiskHolder(ctx context.Context, app *appv1alpha1.App, tier, node string) diskHoldStep {
	log := logf.FromContext(ctx).WithValues("app", app.Name)
	job, err := diskHolderJob(app, tier, node, r.Scheme)
	if err == nil {
		err = r.Create(ctx, job)
	}
	switch {
	case err == nil:
		log.Info("holding the disk on its node through the rollout", "node", node, "holder", job.Name)
		return diskHoldStep{wait: diskHoldPoll}
	case apierrors.IsAlreadyExists(err):
		return diskHoldStep{wait: diskHoldPoll} // the cache has not seen it yet
	default:
		// Quota (count/jobs.batch) or admission: the rollout goes on.
		log.Error(err, "the disk holder was refused; rolling without it", "node", node)
		return skipDiskHold(diskHoldFallbackRefused)
	}
}

// awaitDiskHolder is the rollout's step while holder exists: write once its
// pod has the volume mounted on node, wait while it may still, and go on
// without it once it cannot.
func (r *AppReconciler) awaitDiskHolder(ctx context.Context, holder *batchv1.Job, pods []corev1.Pod, node string) diskHoldStep {
	log := logf.FromContext(ctx).WithValues("holder", holder.Name, "node", node)
	if !holder.DeletionTimestamp.IsZero() || execution.JobFinished(holder) {
		r.endDiskHold(ctx, holder, diskHoldEndFallback)
		return skipDiskHold(diskHoldFallbackFailed)
	}
	pod := jobPod(pods, holder)
	if pod != nil && pod.Spec.NodeName == node && volumesMounted(pod) {
		return diskHoldStep{held: true}
	}
	if pod != nil && (!pod.DeletionTimestamp.IsZero() || podEnded(pod) || (pod.Spec.NodeName != "" && pod.Spec.NodeName != node)) {
		log.Info("the disk holder failed; rolling without it", "pod", pod.Name, "phase", pod.Status.Phase)
		r.endDiskHold(ctx, holder, diskHoldEndFallback)
		return skipDiskHold(diskHoldFallbackFailed)
	}
	age := jobAge(holder)
	if age >= diskHoldMountBound {
		log.Info("the disk holder did not mount the volume in time; rolling without it", "bound", diskHoldMountBound)
		r.endDiskHold(ctx, holder, diskHoldEndFallback)
		return skipDiskHold(diskHoldFallbackTimeout)
	}
	return diskHoldStep{wait: min(diskHoldPoll, diskHoldMountBound-age)}
}

// diskHoldEnd is why a holder whose template is already written ends now, ""
// while its rollout still needs it.
func (r *AppReconciler) diskHoldEnd(ctx context.Context, app *appv1alpha1.App, holder *batchv1.Job, stored *appsv1.Deployment, replicas int32) string {
	switch {
	case !holder.DeletionTimestamp.IsZero():
		return ""
	case execution.JobFinished(holder):
		return diskHoldEndFinished
	case replicas == 0 || !mountsDiskClaim(stored.Spec.Template.Spec, app.Name):
		return diskHoldEndStopped
	case jobAge(holder) >= diskHoldMaxAge:
		return diskHoldEndExpired
	}
	instances, err := r.currentInstances(ctx, stored)
	if err != nil {
		logf.FromContext(ctx).Error(err, "reading the new instance for the disk hold failed", "app", app.Name)
		return ""
	}
	node := diskJobNode(holder)
	mounted := false
	for _, pod := range instances {
		if pod.Spec.NodeName != node {
			return diskHoldEndMoved // the volume must move; at once
		}
		mounted = mounted || volumesMounted(pod)
	}
	if mounted {
		return diskHoldEndMounted
	}
	if deploymentSettled(stored) {
		return diskHoldEndSettled
	}
	return ""
}

// currentInstances is every scheduled, live pod of the ReplicaSet that runs
// dep's current template: the new instance a rollout starts. The Deployment
// controller finds that ReplicaSet the same way, by its template with the
// pod-template-hash label set aside, so a pod of the template being replaced
// never reads as the new one, nor does one of a template a later deploy
// replaced again.
func (r *AppReconciler) currentInstances(ctx context.Context, dep *appsv1.Deployment) ([]*corev1.Pod, error) {
	if dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return nil, nil
	}
	selector := client.MatchingLabels(dep.Spec.Selector.MatchLabels)
	var rss appsv1.ReplicaSetList
	if err := r.List(ctx, &rss, client.InNamespace(dep.Namespace), selector); err != nil {
		return nil, err
	}
	current := map[types.UID]bool{}
	for i := range rss.Items {
		rs := &rss.Items[i]
		if !metav1.IsControlledBy(rs, dep) {
			continue
		}
		tmpl := rs.Spec.Template.DeepCopy()
		delete(tmpl.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
		if equality.Semantic.DeepEqual(*tmpl, dep.Spec.Template) {
			current[rs.UID] = true
		}
	}
	if len(current) == 0 {
		return nil, nil
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace), selector); err != nil {
		return nil, err
	}
	var instances []*corev1.Pod
	for i := range pods.Items {
		pod := &pods.Items[i]
		owner := metav1.GetControllerOf(pod)
		if owner != nil && current[owner.UID] && pod.Spec.NodeName != "" && pod.DeletionTimestamp.IsZero() {
			instances = append(instances, pod)
		}
	}
	return instances, nil
}

// endOwnedDiskHold ends holder when it is this App's, for a caller that has
// not checked.
func (r *AppReconciler) endOwnedDiskHold(ctx context.Context, app *appv1alpha1.App, holder *batchv1.Job, reason string) {
	if holder != nil && metav1.IsControlledBy(holder, app) {
		r.endDiskHold(ctx, holder, reason)
	}
}

// releaseDiskHold ends app's holder and pre-pull, if it has them: its disk is
// detached, or a restore is about to take the volume.
func (r *AppReconciler) releaseDiskHold(ctx context.Context, app *appv1alpha1.App, reason string) {
	holder, err := r.diskJob(ctx, app, diskHolderName(app.Name))
	if err != nil {
		logf.FromContext(ctx).Error(err, "reading the disk holder failed", "app", app.Name)
	} else {
		r.endOwnedDiskHold(ctx, app, holder, reason)
	}
	prePull, err := r.diskJob(ctx, app, diskPrePullName(app.Name))
	if err != nil {
		logf.FromContext(ctx).Error(err, "reading the disk pre-pull failed", "app", app.Name)
		return
	}
	r.endOwnedDiskPrePull(ctx, app, prePull, reason)
}

// diskJob is app's Job named name (its holder or pre-pull), nil when there is
// none. It may belong to another lifetime of the App; callers check.
func (r *AppReconciler) diskJob(ctx context.Context, app *appv1alpha1.App, name string) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, job); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return job, nil
}

// endDiskHold ends holder, counting why.
func (r *AppReconciler) endDiskHold(ctx context.Context, holder *batchv1.Job, reason string) {
	if r.endDiskJob(ctx, holder, reason) {
		diskHoldEndsTotal.WithLabelValues(reason).Inc()
	}
}

// endDiskJob deletes a holder or pre-pull Job, whose pod the garbage
// collector then deletes, and reports whether it did. A failure is only
// logged: the next pass tries again, and the Job's own deadline is the last
// backstop.
func (r *AppReconciler) endDiskJob(ctx context.Context, job *batchv1.Job, reason string) bool {
	if job == nil || !job.DeletionTimestamp.IsZero() {
		return false
	}
	uid := job.UID
	if err := r.Delete(ctx, job, client.Preconditions{UID: &uid}, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
		if !apierrors.IsNotFound(err) {
			logf.FromContext(ctx).Error(err, "ending the disk rollout Job failed", "job", job.Name, "reason", reason)
		}
		return false
	}
	logf.FromContext(ctx).Info("ended the disk rollout Job", "job", job.Name, "node", diskJobNode(job), "reason", reason)
	return true
}

// jobAge is how long ago job was created. A Job the API server has not
// stamped yet is new.
func jobAge(job *batchv1.Job) time.Duration {
	if job.CreationTimestamp.IsZero() {
		return 0
	}
	return time.Since(job.CreationTimestamp.Time)
}

// servingInstance is the App's running instance, the one a rollout replaces
// and whose node has the volume; nil when none runs.
func servingInstance(pods []corev1.Pod) *corev1.Pod {
	for i := range pods {
		pod := &pods[i]
		owner := metav1.GetControllerOf(pod)
		if owner != nil && owner.Kind == kindReplicaSet && pod.Spec.NodeName != "" &&
			pod.Status.Phase == corev1.PodRunning && pod.DeletionTimestamp.IsZero() {
			return pod
		}
	}
	return nil
}

// jobPod is the pod of job among pods, nil before the Job controller has
// made it.
func jobPod(pods []corev1.Pod, job *batchv1.Job) *corev1.Pod {
	for i := range pods {
		if metav1.IsControlledBy(&pods[i], job) {
			return &pods[i]
		}
	}
	return nil
}

// volumesMounted reports that the kubelet has mounted pod's volumes. It
// creates the sandbox, and so sets PodReadyToStartContainers, only once every
// volume is attached and mounted, and a running pod is past both.
func volumesMounted(pod *corev1.Pod) bool {
	if !pod.DeletionTimestamp.IsZero() {
		return false
	}
	switch pod.Status.Phase {
	case corev1.PodRunning:
		return true
	case corev1.PodPending:
		return slices.ContainsFunc(pod.Status.Conditions, func(c corev1.PodCondition) bool {
			return c.Type == corev1.PodReadyToStartContainers && c.Status == corev1.ConditionTrue
		})
	}
	return false
}

func podEnded(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed
}

// deploymentSettled reports a Deployment with no rollout in progress: the
// controller has seen its template, and every instance runs it and is
// available.
func deploymentSettled(dep *appsv1.Deployment) bool {
	want := ptr.Deref(dep.Spec.Replicas, 1)
	status := dep.Status
	return status.ObservedGeneration >= dep.Generation && status.Replicas == want &&
		status.UpdatedReplicas == want && status.AvailableReplicas == want
}

// mountsDiskClaim reports a pod spec that mounts the disk claim of appName.
func mountsDiskClaim(spec corev1.PodSpec, appName string) bool {
	claim := diskPVCName(appName)
	return slices.ContainsFunc(spec.Volumes, func(v corev1.Volume) bool {
		return v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == claim
	})
}

// diskHolderTolerations is what a holder for a template of tier tolerates.
func diskHolderTolerations(tier string) []corev1.Toleration {
	var spec corev1.PodSpec
	execution.TolerateServingPool(&spec, tiers.Compute.ServingPool(tier))
	return spec.Tolerations
}

// nodeTakesPod reports a node that admits a new pod with tolerations: not
// cordoned or being deleted, and no NoSchedule or NoExecute taint the pod
// does not tolerate (not-ready, the cluster-autoscaler's
// ToBeDeletedByClusterAutoscaler, a pool the pod is not for).
func nodeTakesPod(node *corev1.Node, tolerations []corev1.Toleration) bool {
	if node.Spec.Unschedulable || !node.DeletionTimestamp.IsZero() {
		return false
	}
	for i := range node.Spec.Taints {
		taint := &node.Spec.Taints[i]
		if taint.Effect == corev1.TaintEffectPreferNoSchedule {
			continue
		}
		if !slices.ContainsFunc(tolerations, func(t corev1.Toleration) bool { return t.ToleratesTaint(logr.Discard(), taint, false) }) {
			return false
		}
	}
	return true
}

// nodeMatchesPod reports a node that satisfies spec's node selector and
// required node affinity: the serving pool a tier names (PlaceOnServingPool).
func nodeMatchesPod(node *corev1.Node, spec *corev1.PodSpec) bool {
	for key, value := range spec.NodeSelector {
		if got, ok := node.Labels[key]; !ok || got != value {
			return false
		}
	}
	if spec.Affinity == nil || spec.Affinity.NodeAffinity == nil || spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		return true
	}
	return slices.ContainsFunc(spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
		func(term corev1.NodeSelectorTerm) bool { return nodeMatchesTerm(node, term) })
}

// nodeMatchesTerm is one ORed node selector term, as the scheduler reads it:
// every requirement holds, and an empty term matches no node.
func nodeMatchesTerm(node *corev1.Node, term corev1.NodeSelectorTerm) bool {
	if len(term.MatchExpressions) == 0 && len(term.MatchFields) == 0 {
		return false
	}
	for _, req := range term.MatchExpressions {
		value, ok := node.Labels[req.Key]
		if !requirementHolds(req, value, ok) {
			return false
		}
	}
	for _, req := range term.MatchFields {
		if req.Key != diskNodeField || !requirementHolds(req, node.Name, true) {
			return false
		}
	}
	return true
}

// requirementHolds is one node selector requirement on a value that is
// present or not. Gt and Lt, which no bex template uses, hold for no node.
func requirementHolds(req corev1.NodeSelectorRequirement, value string, present bool) bool {
	switch req.Operator {
	case corev1.NodeSelectorOpIn:
		return present && slices.Contains(req.Values, value)
	case corev1.NodeSelectorOpNotIn:
		return !present || !slices.Contains(req.Values, value)
	case corev1.NodeSelectorOpExists:
		return present
	case corev1.NodeSelectorOpDoesNotExist:
		return !present
	}
	return false
}
