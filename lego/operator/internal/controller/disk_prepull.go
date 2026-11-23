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
	"maps"
	"slices"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The disk pre-pull (w6/m595, docs/ADR082-persistent-disks.md D3) puts a code
// deploy's new image on the volume's node before Recreate stops the old
// instance, so the new instance's pull is the imagePullPolicy: Always re-check
// alone (under 1 s) instead of a fetch of every new layer (1.0-48.9 s for the
// built images new to their node on production, 2026-11-21 to 2026-11-23).
//
// It is a Job beside the disk holder, on the holder's node: one pod with a
// container per image the new template runs that the running one does not.
// Each container's command is "/", a directory, which no image can execute:
// the kubelet pulls the image and creates the container, runc's exec fails
// ("is a directory"), and the container ends in StartError. So no tenant code
// runs, and the pod ends once every image is on the node. The kubelet posts a
// pod's container states once per sync, after the sync's last pull, so the pod
// reads as pulling until then (measured on the CAPD cluster, t002).
//
// The rollout writes its template once the holder has the volume mounted and
// the pull has ended, failed, or passed diskPrePullBound. The old instance
// serves throughout, so the wait costs the deploy time, not downtime. A
// restart or a plan change keeps every image and starts none, and so does a
// rollout without a hold: the volume then moves, and the new instance may
// start on any node.
//
// The operator deletes the Job once the template is written and its pull has
// ended, never while it still pulls: the kubelet serializes pulls, so the new
// instance's pull waits for that one and then finds the layers there, where a
// deletion would cancel it.
const (
	diskPrePullPrefix    = "dskpul-"
	diskPrePullComponent = appv1alpha1.ComponentDiskPrePull
	// diskPrePullBound is how long a rollout waits for its pull. The largest
	// pull of a built image new to its node on production took 48.9 s (102
	// such pulls, median 8.2 s, p90 23.7 s; w6/m595), and the kubelet pulls
	// one image at a time, so a pull can also wait behind another. The old
	// instance serves while the rollout waits, and a pull past the bound
	// goes on in the kubelet with the new instance's pull behind it.
	diskPrePullBound = 90 * time.Second
	// diskPrePullPoll is the wait between checks of a pull still running.
	// The pre-pull pod's own events wake the pass when it ends.
	diskPrePullPoll = 5 * time.Second
	// diskPrePullPodBound is how long a pre-pull Job may be without a pod.
	// The Job controller makes it within a second unless the pod is refused:
	// the workspace's pod quota, or the signature webhook on an unsigned
	// image, which refuses the new instance too.
	diskPrePullPodBound = diskHoldMountBound
)

// What a held rollout's write waited for (diskPrePullsTotal).
const (
	diskPrePullPulled  = "pulled"
	diskPrePullFailed  = "failed"
	diskPrePullTimeout = "timeout"
	diskPrePullRefused = "refused"
)

var diskPrePullsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "bex_disk_prepulls_total",
	Help: "Held disk-bearing rollouts whose new template runs an image the running one does not, by what the template write waited for: pulled (every new image was on the volume's node), failed (a pull or the pre-pull's pod failed; the instance's own pull reports it), timeout (the pull was still running past its bound and goes on in the kubelet), refused (the pre-pull Job could not be created).",
}, []string{"result"})

func init() {
	ctrlmetrics.Registry.MustRegister(diskPrePullsTotal)
}

func diskPrePullName(appName string) string {
	return appv1alpha1.DiskChildName(diskPrePullPrefix, appName)
}

// diskPrePullJob is the Job whose one pod pulls images onto node with the
// pull credentials of template, the pod template being rolled, and runs none
// of them. tier is that template's compute tier, whose serving pool the node
// is in when the tier names one.
//
// It carries template's verify-image label, so the signature webhook refuses
// an unsigned tenant image before any node pulls it, as it does for the
// instance. Like the holder, it carries the App's app label, so the App
// reconciles on its pod's events, and a component label, so no instance
// surface lists it, and never app.bex.co/app-id or the container policy label:
// bex-operator-workloads holds it to the strict container shape.
func diskPrePullJob(app *appv1alpha1.App, tier, node string, images []string, template *corev1.PodTemplateSpec, scheme *runtime.Scheme) (*batchv1.Job, error) {
	verify := template.Labels[execution.LabelVerifyImage] == execution.VerifyImageEnabled
	labels := execution.PodLabels(app.Name, string(app.UID), diskPrePullComponent, "", "", verify)
	containers := make([]corev1.Container, len(images))
	for i, image := range images {
		containers[i] = corev1.Container{
			Name:  fmt.Sprintf("pull-%d", i),
			Image: image,
			// A directory: the exec fails in runc's init, after the pull.
			Command: []string{"/"},
			// As the instance pulls: a cached image is still re-checked
			// against the registry's credentials (w4/426).
			ImagePullPolicy: tenantImagePullPolicy,
			Resources:       diskHolderResources(),
			// An explicit non-root user: the kubelet's runAsNonRoot check
			// would otherwise read the image's user and could stop the
			// container before its start (CreateContainerConfigError), and
			// the pod would never end.
			SecurityContext: diskHolderSecurityContext(),
		}
	}
	spec := corev1.PodSpec{
		RestartPolicy:                 corev1.RestartPolicyNever,
		AutomountServiceAccountToken:  new(false),
		EnableServiceLinks:            new(false),
		TerminationGracePeriodSeconds: new(int64(1)),
		ImagePullSecrets:              slices.Clone(template.Spec.ImagePullSecrets),
		Affinity:                      diskNodeAffinity(node),
		Containers:                    containers,
	}
	execution.TolerateServingPool(&spec, tiers.Compute.ServingPool(tier))
	applyPodSpecServerDefaults(&spec)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: diskPrePullName(app.Name), Namespace: app.Namespace, Labels: labels},
		Spec: batchv1.JobSpec{
			// One pod, never replaced: a failed pull is a rollout without one.
			BackoffLimit: new(int32(0)),
			// The holder's: a pull still running when the template is written
			// is one the new instance's pull waits for, so the Job's own
			// deadline must not cancel it early.
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

// diskPrePullImages is the images job pulls, in order.
func diskPrePullImages(job *batchv1.Job) []string {
	images := make([]string, len(job.Spec.Template.Spec.Containers))
	for i, c := range job.Spec.Template.Spec.Containers {
		images[i] = c.Image
	}
	return images
}

// newTemplateImages is every image projected runs that stored does not,
// sorted: what a rollout's new instance pulls that the volume's node may not
// have. A restart or a plan change keeps every image, so it names none.
func newTemplateImages(stored, projected *corev1.PodSpec) []string {
	running := map[string]bool{}
	for _, c := range slices.Concat(stored.InitContainers, stored.Containers) {
		running[c.Image] = true
	}
	var images []string
	for _, c := range slices.Concat(projected.InitContainers, projected.Containers) {
		if !running[c.Image] && !slices.Contains(images, c.Image) {
			images = append(images, c.Image)
		}
	}
	slices.Sort(images)
	return images
}

// prePullState is how far the kubelet is with a pre-pull pod's images.
type prePullState int

const (
	prePullPulling prePullState = iota
	prePullPulled
	prePullFailed
)

// diskPrePullState reads job and its pod (nil before the Job controller has
// made it). A container has its image once it has been created: it reads
// CreateContainerError or CreateContainerConfigError, or it has ended with
// the image resolved (StartError, the normal end). A pull that failed, a pod
// that cannot be scheduled or is deleted, and a Job that ended or never got
// its pod are failures.
func diskPrePullState(job *batchv1.Job, pod *corev1.Pod) prePullState {
	if !job.DeletionTimestamp.IsZero() {
		return prePullFailed
	}
	if pod == nil {
		if execution.JobFinished(job) || jobAge(job) >= diskPrePullPodBound {
			return prePullFailed
		}
		return prePullPulling
	}
	if !pod.DeletionTimestamp.IsZero() || podUnschedulable(pod) {
		return prePullFailed
	}
	pulled := 0
	for _, cs := range pod.Status.ContainerStatuses {
		switch {
		case cs.State.Waiting != nil:
			switch cs.State.Waiting.Reason {
			case "ErrImagePull", imagePullBackOff, reasonInvalidImageName, "ErrImageNeverPull", "RegistryUnavailable", "SignatureValidationFailed":
				return prePullFailed
			case createContainerConfigError, "CreateContainerError":
				pulled++
			}
		case cs.ImageID != "":
			pulled++
		default:
			// Ended or running with no image: killed before its create.
			return prePullFailed
		}
	}
	if pulled == len(pod.Spec.Containers) {
		return prePullPulled
	}
	if podEnded(pod) || execution.JobFinished(job) {
		return prePullFailed
	}
	return prePullPulling
}

// podUnschedulable reports a pod the scheduler could not place: its node
// takes no new pod any more.
func podUnschedulable(pod *corev1.Pod) bool {
	return pod.Spec.NodeName == "" && slices.ContainsFunc(pod.Status.Conditions, func(c corev1.PodCondition) bool {
		return c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse && c.Reason == corev1.PodReasonUnschedulable
	})
}

// prePullThroughHold is the pre-pull's part of a held rollout's step onto
// node: start the pull of images, the new template's, and wait while it may
// still end within its bound. prePull names what the write waited for.
func (r *AppReconciler) prePullThroughHold(ctx context.Context, app *appv1alpha1.App, tier, node string, prePull *batchv1.Job, pods []corev1.Pod, images []string, template *corev1.PodTemplateSpec) diskHoldStep {
	if len(images) == 0 {
		return diskHoldStep{} // a restart or a plan change: nothing new to pull
	}
	log := logf.FromContext(ctx).WithValues("app", app.Name, "node", node)
	if prePull != nil && !metav1.IsControlledBy(prePull, app) {
		log.Info("a Job of another lifetime of this App has the pre-pull's name; rolling without a pre-pull", "job", prePull.Name)
		return diskHoldStep{prePull: diskPrePullRefused}
	}
	if prePull != nil && (diskJobNode(prePull) != node || !slices.Equal(diskPrePullImages(prePull), images)) {
		// An earlier rollout's, or one on a node the volume left.
		r.endDiskJob(ctx, prePull, "replaced")
		prePull = nil
	}
	if prePull == nil {
		return r.createDiskPrePull(ctx, app, tier, node, images, template)
	}
	switch diskPrePullState(prePull, jobPod(pods, prePull)) {
	case prePullPulled:
		return diskHoldStep{prePull: diskPrePullPulled}
	case prePullFailed:
		log.Info("the pre-pull failed; rolling without it", "job", prePull.Name)
		return diskHoldStep{prePull: diskPrePullFailed}
	}
	age := jobAge(prePull)
	if age >= diskPrePullBound {
		log.Info("the new image is still pulling; rolling on, and the instance's pull waits for it", "bound", diskPrePullBound)
		return diskHoldStep{prePull: diskPrePullTimeout}
	}
	return diskHoldStep{wait: min(diskPrePullPoll, diskPrePullBound-age)}
}

// createDiskPrePull starts app's pre-pull of images on node; the rollout
// waits for it.
func (r *AppReconciler) createDiskPrePull(ctx context.Context, app *appv1alpha1.App, tier, node string, images []string, template *corev1.PodTemplateSpec) diskHoldStep {
	log := logf.FromContext(ctx).WithValues("app", app.Name, "node", node)
	job, err := diskPrePullJob(app, tier, node, images, template, r.Scheme)
	if err == nil {
		err = r.Create(ctx, job)
	}
	switch {
	case err == nil:
		log.Info("pulling the new image onto the disk's node before the old instance stops", "job", job.Name, "images", images)
		return diskHoldStep{wait: diskHoldPoll}
	case apierrors.IsAlreadyExists(err):
		return diskHoldStep{wait: diskHoldPoll} // the cache has not seen it yet
	default:
		// Quota (count/jobs.batch) or admission: the rollout goes on.
		log.Error(err, "the pre-pull was refused; rolling without it")
		return diskHoldStep{prePull: diskPrePullRefused}
	}
}

// settleDiskPrePull ends prePull once its rollout's template is written and
// its pull has ended, and at once when nothing starts on its node any more
// (the service is suspended or its disk detached) or it is older than the
// rollout budget. A pull still running is left to end: the new instance's
// pull waits for it.
func (r *AppReconciler) settleDiskPrePull(ctx context.Context, app *appv1alpha1.App, prePull *batchv1.Job, stored *appsv1.Deployment, replicas int32) {
	if prePull == nil || !metav1.IsControlledBy(prePull, app) || !prePull.DeletionTimestamp.IsZero() {
		return
	}
	switch {
	case replicas == 0 || !mountsDiskClaim(stored.Spec.Template.Spec, app.Name):
		r.endDiskJob(ctx, prePull, diskHoldEndStopped)
		return
	case jobAge(prePull) >= diskHoldMaxAge:
		r.endDiskJob(ctx, prePull, diskHoldEndExpired)
		return
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(app.Namespace), client.MatchingLabels{labelApp: app.Name, execution.LabelComponent: diskPrePullComponent}); err != nil {
		logf.FromContext(ctx).Error(err, "listing the pre-pull's pod failed", "app", app.Name)
		return
	}
	if diskPrePullState(prePull, jobPod(pods.Items, prePull)) != prePullPulling {
		r.endDiskJob(ctx, prePull, diskHoldEndFinished)
	}
}

// endOwnedDiskPrePull ends prePull when it is this App's.
func (r *AppReconciler) endOwnedDiskPrePull(ctx context.Context, app *appv1alpha1.App, prePull *batchv1.Job, reason string) {
	if prePull != nil && metav1.IsControlledBy(prePull, app) {
		r.endDiskJob(ctx, prePull, reason)
	}
}
