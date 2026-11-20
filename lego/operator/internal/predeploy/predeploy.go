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

// Package predeploy is the bex pre-deploy plane: run App.spec.preDeployCommand
// to completion against the new revision's image BEFORE that revision serves
// traffic (Render's Pre-Deploy Command — typically a database migration). Like
// the build plane it does the work in a Kubernetes Job, but the shape differs on
// purpose:
//
//   - The Job's container IS the new image (the same env/secrets/pull secrets as
//     the app pod), so a migration runs against exactly what will serve — launched
//     the way the app's container is (the controller picks Command or Args).
//   - It NEVER retries (BackoffLimit 0): re-running a partially applied migration
//     can corrupt data, so a failed pre-deploy is terminal until the user pushes a
//     new revision.
//   - The operator observes it non-blocking (a single Get per reconcile, then
//     requeue), so the App CR can report the step Running/Succeeded/Failed while it
//     runs — which is what lets the deploy record surface the step's progress
//     (docs/ADR004-app-deployment.md, docs/ADR006-bex-api.md § deploy record).
//
// This package owns only the Job/pod scaffolding and observation; the controller
// supplies the container's image, command, and the same env/secrets the app pod
// gets (appEnv/envFromSources/imagePullSecrets), so "what a migration can reach"
// stays defined next to the app container, not duplicated here.
//
// The plane runs a second command in the same Job shape: Render's one-time
// initialDeployHook (StepInitialDeployHook, w33/m435). It differs in when the
// controller starts it (after the first release is live, never before a
// rollout), in what it may mount (the service's persistent disk) and in that a
// newer release never cancels it.
package predeploy

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/types/deploybudgets"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Pod/Job labels. The predeploy label carries the service name (like build's
// "app.bex.co/build") so the backend can find the step's pod for its logs; the
// component label lets a namespace-wide list distinguish pre-deploy Jobs.
// LabelService aliases the canonical key in lego/types (v1alpha1), which the
// backend also reads.
const (
	LabelService = appv1alpha1.LabelPreDeploy
	// LabelEgressAppID is what the egress meter reads a pre-deploy pod's
	// service from. It aliases the canonical key in lego/types (v1alpha1),
	// shared with the meter; like the other labels here it avoids a
	// hand-written copy of the key.
	LabelEgressAppID = appv1alpha1.LabelEgressAppID
	// LabelComponent and labelWorkspace alias the canonical keys in
	// lego/types (v1alpha1), which the backend also reads.
	LabelComponent = appv1alpha1.LabelComponent
	ComponentValue = "predeploy"
	labelWorkspace = appv1alpha1.LabelWorkspace
	// containerName is the Job's single container.
	containerName = "predeploy"
)

// LabelStep says which of the plane's two commands a Job and its pod run
// (w33/m435). A Job without it was created by an operator that ran the hook
// as the pre-deploy step.
const LabelStep = "app.bex.co/predeploy-step"

// Step is a LabelStep value.
type Step string

const (
	// StepPreDeploy is a release's preDeployCommand, run before every rollout.
	// The zero Step means it.
	StepPreDeploy Step = "pre-deploy"
	// StepInitialDeployHook is the service's one-time initialDeployHook, run
	// after the first release is live.
	StepInitialDeployHook Step = "initial-deploy-hook"
)

// predeployTimeout bounds a single pre-deploy Job's wall-clock (its Job
// activeDeadlineSeconds). A hung migration is reaped as failed rather than
// blocking the rollout forever, matching the build Job's bounded-deadline
// pattern (docs/ADR004-app-deployment.md, w1/m33/t002). It is
// deploybudgets.PreDeployBudget — Render's pre-deploy limit
// (render.com/docs/deploys, deploy steps: build 120, pre-deploy 30, start
// 15). The backend's control-plane fallback, deploybudgets'
// PreDeployGateTimeout, lives beside it and must stay longer than this.
const predeployTimeout = deploybudgets.PreDeployBudget

// jobTTL reaps a finished pre-deploy Job (and its pod's logs) after an hour,
// same as build. The step's outcome survives on the App CR status / deploy
// record, so reaping the Job only drops the ephemeral pod logs.
const jobTTL = 3600

// State is the observed outcome of a pre-deploy Job.
type State string

const (
	// StatePending means the Job exists but has not reached a terminal condition
	// yet (its pod may still be scheduling or running).
	StatePending State = "Pending"
	// StateSucceeded means the Job completed with a zero exit — the rollout may
	// proceed.
	StateSucceeded State = "Succeeded"
	// StateFailed means the Job failed (non-zero exit or deadline exceeded) — the
	// rollout must be blocked and the previous revision left serving.
	StateFailed State = "Failed"
)

// Options configures one pre-deploy run. The container-shaping fields (Env,
// EnvFrom, ImagePullSecrets, SecurityContext, Resources, and the secret-file
// volume) are supplied by the controller so a migration runs with exactly the
// app pod's configuration.
type Options struct {
	Name         string   // service name (image repo name / label value)
	AppUID       string   // immutable App UID; prevents same-name recreation from reusing stale artifacts
	Namespace    string   // namespace the pre-deploy Job runs in
	Workspace    string   // owning tenant id (app.bex.co/workspace label); empty = omitted
	Environment  string   // the App's environment id (pod label only); empty = omitted
	AppNamespace string   // namespace the App CR lives in; used for log attribution
	VerifyImage  bool     // select the Pod for signature admission when signing is enabled
	Image        string   // the new revision's image (same ref the Deployment will run)
	Command      []string // the controller's launch of App.spec.preDeployCommand (w8/172): in place of the image's program,
	Args         []string // or as arguments to its entrypoint, a native image's native-init (w8/350)
	Revision     string   // deterministic per-revision tag, e.g. "gen-7" (names the Job)
	Generation   int64    // release generation this step gates (stamped on the pod for traceability)
	Step         Step     // which command this is; empty means StepPreDeploy

	Env              []corev1.EnvVar
	EnvFrom          []corev1.EnvFromSource
	ImagePullSecrets []corev1.LocalObjectReference
	SecurityContext  *corev1.SecurityContext
	ContainerPolicy  string // persisted App policy, projected for workload admission
	FreePlan         bool   // a free-plan App: the pod carries execution.LabelFreePlan (w8/m127)
	// EgressAppID is the App's public id, stamped as LabelEgressAppID so the
	// direct egress meter bills the command's outbound traffic to its service,
	// as Render counts traffic any service initiates (w4/453). The pod still
	// carries no bex.co/app-id, so no Service selects it (w4/m181).
	EgressAppID string
	Resources   corev1.ResourceRequirements
	// ServingPool is the tier's dedicated serving node pool ("" = standard):
	// a migration runs at the App's tier size, so it needs the same node the
	// App does (w2/m250).
	ServingPool string
	// Volumes/VolumeMounts carry the app pod's /etc/secrets projection so a
	// migration can read the same secret files; nil when the app has none.
	//
	// A claim among them is the service's persistent disk, which only the
	// initial deploy hook mounts (Render: "downloading files to a persistent
	// disk"; its pre-deploy command has no disk access). The volume is
	// ReadWriteOnce, so that Job's pod sits on the instance's node
	// (sharesDisk), and it mounts the volume as the instance does: HostUsers
	// is the instance pod's own setting, since a pod user namespace needs an
	// idmappable filesystem, which the local-path class is not.
	Volumes      []corev1.Volume
	VolumeMounts []corev1.VolumeMount
	HostUsers    *bool

	Client client.Client
}

// sharesDisk reports a Job that mounts the service's persistent disk.
func (o Options) sharesDisk() bool {
	return slices.ContainsFunc(o.Volumes, func(v corev1.Volume) bool { return v.PersistentVolumeClaim != nil })
}

func (o Options) step() Step {
	if o.Step == "" {
		return StepPreDeploy
	}
	return o.Step
}

// JobName is the deterministic per-revision pre-deploy Job name: re-reconciling
// the same revision reuses that revision's Job, so a completed migration is
// never re-run. Two revisions must therefore never share a name — reuse would
// skip the second one's migration entirely.
func JobName(name, revision string) string {
	return appv1alpha1.PreDeployJobName(name, revision)
}

// HookJobName is the initial deploy hook's Job name for the release that was
// live when it started. One Job per release generation, like JobName: a hook
// that ended for a release is never run again for it.
func HookJobName(name, revision string) string {
	return appv1alpha1.InitialDeployHookJobName(name, revision)
}

func (o Options) jobName() string {
	if o.step() == StepInitialDeployHook {
		return HookJobName(o.Name, o.Revision)
	}
	return JobName(o.Name, o.Revision)
}

// Job constructs the pre-deploy Job for o (a pure function — no cluster access —
// so the Job shape is unit-testable). The single container is the new image with
// the app pod's env/secrets, running the argv the controller chose; it never
// restarts or retries and is bounded by predeployTimeout. The controller keeps
// any shell it names absolute because the runtime resolves a bare program
// against the app's own PATH, which may be empty or narrow (w8/m61).
func Job(o Options) *batchv1.Job {
	deadline := int64(predeployTimeout / time.Second)
	backoff := int32(0) // never retry a migration — a partial re-run can corrupt data
	ttl := int32(jobTTL)

	container := corev1.Container{
		Name:  containerName,
		Image: o.Image,
		// Always, digest or not: a cached image would otherwise start without
		// the pod's pull credentials, so another workspace naming a private
		// image@sha256 on a shared node could run it (w4/426).
		ImagePullPolicy: corev1.PullAlways,
		Command:         o.Command,
		Args:            o.Args,
		Env:             o.Env,
		EnvFrom:         o.EnvFrom,
		Resources:       o.Resources,
		SecurityContext: o.SecurityContext,
		VolumeMounts:    o.VolumeMounts,
	}

	appNamespace := ""
	if o.AppNamespace != "" && o.AppNamespace != o.Namespace {
		appNamespace = o.AppNamespace
	}
	labels := execution.PodLabels(o.Name, o.AppUID, ComponentValue, o.Workspace, appNamespace, o.VerifyImage)
	labels[LabelService] = o.Name
	labels[LabelStep] = string(o.step())
	// The release revision, as on build Jobs: bex-api meters pre-deploy runs as
	// build minutes and exempts the platform's own rebuilds by it (w6/m307).
	labels[appv1alpha1.LabelBuildRevision] = o.Revision
	podLabels := execution.PodLabels(o.Name, o.AppUID, ComponentValue, o.Workspace, appNamespace, o.VerifyImage)
	podLabels[LabelService] = o.Name
	podLabels[LabelStep] = string(o.step())
	// Every widened policy (image-v1, render-v1) carries its admission marker;
	// strict pre-deploy pods stay unlabeled.
	if len(appv1alpha1.ContainerPolicyCapabilities(o.ContainerPolicy)) > 0 {
		podLabels[execution.LabelContainerPolicy] = o.ContainerPolicy
	}
	if o.FreePlan {
		podLabels[execution.LabelFreePlan] = execution.FreePlanValue
	}
	if o.EgressAppID != "" {
		podLabels[LabelEgressAppID] = o.EgressAppID
	}
	// A migration runs inside its service's environment: an isolated one's
	// boundary admits it to that environment's datastores and keeps it from
	// the rest of the workspace, as for the service's own pods.
	if o.Environment != "" {
		podLabels[execution.LabelEnvironment] = o.Environment
	}
	var owners []metav1.OwnerReference
	// App owner references are valid only for co-located Jobs. They also bind
	// the policy admission marker to the same App lifetime as the UID label.
	if o.AppUID != "" && o.AppNamespace == o.Namespace {
		owners = []metav1.OwnerReference{{
			APIVersion: appv1alpha1.SchemeGroupVersion.String(), Kind: "App",
			Name: o.Name, UID: types.UID(o.AppUID), Controller: new(true),
		}}
	}
	podSpec := corev1.PodSpec{
		// No legacy Docker-link env vars (w4/m123) — the pre-deploy command — tenant code.
		EnableServiceLinks: new(false),
		RestartPolicy:      corev1.RestartPolicyNever,
		Containers:         []corev1.Container{container},
		ImagePullSecrets:   o.ImagePullSecrets,
		Volumes:            o.Volumes,
	}
	execution.HardenPod(&podSpec)
	execution.PlaceOnServingPool(&podSpec, o.ServingPool)
	if o.sharesDisk() {
		placeBesideInstance(&podSpec, o.Name, o.HostUsers)
		podSpec.Containers[0].Resources = spareCapacityResources(o.Resources)
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            o.jobName(),
			Namespace:       o.Namespace,
			Labels:          labels,
			OwnerReferences: owners,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			ActiveDeadlineSeconds:   &deadline,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					// The pod carries the service label so the backend can read the
					// step's logs by selector, plus the generation for traceability.
					Labels: podLabels,
					Annotations: map[string]string{
						"app.bex.co/generation": fmt.Sprintf("%d", o.Generation),
					},
				},
				Spec: podSpec,
			},
		},
	}
}

// placeBesideInstance pins a disk-sharing pod to the node the service's own
// instance runs on. A ReadWriteOnce volume mounts a second time only there;
// anywhere else the pod waits on Multi-Attach until the Job's deadline. Only
// the service's instances carry a revision label (Job pods carry the app label
// alone), so the term never matches this pod or a sibling Job's. The pool
// selector HardenPod set is dropped: the instance's node is the one node this
// pod can run on, whatever pool it is in.
func placeBesideInstance(spec *corev1.PodSpec, service string, hostUsers *bool) {
	spec.HostUsers = hostUsers
	spec.NodeSelector = nil
	if spec.Affinity == nil {
		spec.Affinity = &corev1.Affinity{}
	}
	spec.Affinity.PodAffinity = &corev1.PodAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{execution.LabelApp: service},
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key: appv1alpha1.LabelRevision, Operator: metav1.LabelSelectorOpExists,
				}},
			},
			TopologyKey: corev1.LabelHostname,
		}},
	}
}

// spareCapacityMemory is what a disk-sharing pod reserves of its node's
// memory, as the disk snapshot Job does.
var spareCapacityMemory = resource.MustParse("64Mi")

// spareCapacityResources keeps r's limits and reserves next to nothing. A
// disk-sharing pod can run on one node only, the instance's, and bin-packing
// fills nodes to their last millicores: a pod requesting the tier's full size
// there would wait out its Job's deadline unscheduled, and no autoscaler can
// add room to a node. So it runs on that node's spare capacity, up to the
// tier's limits (the disk snapshot Job's rule, learned the same way). Each
// zero is explicit: an absent request defaults to its limit.
func spareCapacityResources(r corev1.ResourceRequirements) corev1.ResourceRequirements {
	if len(r.Limits) == 0 {
		return r
	}
	requests := make(corev1.ResourceList, len(r.Limits))
	for name := range r.Limits {
		requests[name] = resource.MustParse("0")
	}
	if limit, ok := r.Limits[corev1.ResourceMemory]; ok {
		requests[corev1.ResourceMemory] = spareCapacityMemory
		if limit.Cmp(spareCapacityMemory) < 0 {
			requests[corev1.ResourceMemory] = limit
		}
	}
	return corev1.ResourceRequirements{Limits: r.Limits, Requests: requests}
}

// Ensure creates the pre-deploy Job if it does not already exist, returning the
// current Job either way. Idempotent per revision: a second call for the same
// revision reuses the exact-lifetime Job rather than starting another migration.
func Ensure(ctx context.Context, o Options) (*batchv1.Job, error) {
	if o.AppUID == "" {
		return nil, fmt.Errorf("predeploy: empty App UID")
	}
	identity := execution.ArtifactIdentity{Name: o.Name, UID: o.AppUID, Workspace: o.Workspace, Namespace: o.AppNamespace}
	cur, _, err := execution.EnsureOwnedJob(ctx, o.Client, Job(o), identity, "predeploy")
	return cur, err
}

// Observe reports a Job's pre-deploy state. A Job with neither terminal
// condition is StatePending; FailureMessage explains a StateFailed one.
func Observe(j *batchv1.Job) State {
	switch {
	case execution.JobHasCondition(j, batchv1.JobComplete):
		return StateSucceeded
	case execution.JobHasCondition(j, batchv1.JobFailed):
		return StateFailed
	default:
		return StatePending
	}
}

// FailureMessage explains a failed pre-deploy Job in the service owner's terms
// (w1/m149): the window it ran past, an out-of-memory kill, or the command's
// exit code read from the pod's terminated container. Never the Job
// controller's condition text — "BackoffLimitExceeded" only says a zero-retry
// Job failed once. Image failures name the cause instead of pointing to logs
// for a command that never started. Its own sentences are worded from the
// shared templates, which the dashboard words in its own language (w4/591).
func FailureMessage(ctx context.Context, cl client.Reader, j *batchv1.Job) string {
	return wordedFor(j, failureMessage(ctx, cl, j))
}

// isHook reports a Job that runs the initial deploy hook.
func isHook(j *batchv1.Job) bool {
	return j.Labels[LabelStep] == string(StepInitialDeployHook)
}

// hookWording turns a pre-deploy failure sentence into the hook's: the same
// diagnosis names the command that failed, and the hook's output is in the
// deploy's log, not behind a "pre-deploy logs" filter.
var hookWording = strings.NewReplacer(
	"the pre-deploy command", "the initial deploy hook",
	"; check the pre-deploy logs", "",
	"there are no pre-deploy logs", "it wrote no output",
)

// wordedFor is msg for j's own command: a hook Job's failure names the hook.
func wordedFor(j *batchv1.Job, msg string) string {
	if msg == "" || !isHook(j) {
		return msg
	}
	return hookWording.Replace(msg)
}

func failureMessage(ctx context.Context, cl client.Reader, j *batchv1.Job) string {
	// A pod that never got its image never ran the command: no log to point
	// at, and the deadline only measured the pull retries (w8/m44).
	if w, _ := pullFailure(ctx, cl, j); w != nil {
		return pullFailureMessage(w)
	}
	if execution.JobFailedReason(j) == batchv1.JobReasonDeadlineExceeded {
		return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonPreDeployWindowExceeded,
			appv1alpha1.DeployReasonParam{Name: "minutes", Value: strconv.Itoa(int(predeployTimeout / time.Minute))})
	}
	if t := terminatedContainer(ctx, cl, j); t != nil {
		exitCode := appv1alpha1.DeployReasonParam{Name: "code", Value: strconv.Itoa(int(t.ExitCode))}
		switch {
		case t.Reason == "StartError":
			return startErrorMessage(t.Message)
		case t.Reason == "OOMKilled":
			return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonPreDeployOutOfMemory, exitCode)
		case t.ExitCode != 0:
			return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonPreDeployExited, exitCode)
		}
	}
	return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonPreDeployFailed)
}

// StartExecFailure matches runc's exec failure inside a StartError message:
// `exec: "/bin/sh": stat /bin/sh: no such file or directory`. It is the one
// parser for this shape: the controller's crash-loop diagnosis shares it so a
// runc message change updates both call sites at once.
var StartExecFailure = regexp.MustCompile(`exec: "([A-Za-z0-9._/+-]{1,256})": (.*)$`)

// startErrorMessage names the program the runtime could not launch, so the
// owner is not pointed at logs a command that never ran cannot have written.
func startErrorMessage(runtimeMsg string) string {
	const neverRan = "; the program never ran, so there are no pre-deploy logs"
	m := StartExecFailure.FindStringSubmatch(runtimeMsg)
	if m == nil {
		return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonPreDeployCouldNotLaunch)
	}
	program, cause := m[1], m[2]
	switch {
	case strings.Contains(cause, "no such file or directory"), strings.Contains(cause, "executable file not found"):
		return fmt.Sprintf("the pre-deploy command could not start: the program %q does not exist in this image%s", program, neverRan)
	case strings.Contains(cause, "permission denied"):
		return fmt.Sprintf("the pre-deploy command could not start: the program %q is not executable in this image%s", program, neverRan)
	}
	return fmt.Sprintf("the pre-deploy command could not start: the container runtime could not execute %q%s", program, neverRan)
}

// imagePullGrace is how long the Job's pod may wait on a failing image pull
// before the step fails (w8/m44). The kubelet's first ErrImagePull can be a
// registry blip; by 90s it has retried with back-off, and a tag that does not
// exist will not appear. Waiting out the 10-minute Job deadline instead only
// delayed the same verdict behind a message about a command that never ran.
const imagePullGrace = 90 * time.Second

// PullFailure explains a pending pre-deploy Job whose image cannot start.
// Malformed references fail immediately; permanent pull errors wait through
// imagePullGrace. Transient errors retain the Job and rollout deadlines.
func PullFailure(ctx context.Context, cl client.Reader, j *batchv1.Job, now time.Time) string {
	w, created := pullFailure(ctx, cl, j)
	if !PermanentPullFailure(w, created, now) {
		return ""
	}
	return wordedFor(j, pullFailureMessage(w))
}

// PermanentPullFailure is shared by pre-deploy Jobs and rollout pods. Malformed
// references cannot recover through retries. Missing-image and denied-auth
// answers wait through imagePullGrace; network errors and plain backoff cannot
// shorten the deadline. Docker Hub answers a missing repository with "pull
// access denied, repository does not exist … insufficient_scope" rather than a
// 404, so that vocabulary counts too (w8/131): a fixed tag or credential
// arrives with a new deploy. GHCR answers every missing, private or
// wrong-credential pull with "403 Forbidden" at its token endpoint, so a
// 403 (and the "denied:" code) counts the same way (w4/392); rate limits
// and 5xx stay transient.
func PermanentPullFailure(w *corev1.ContainerStateWaiting, created, now time.Time) bool {
	if w == nil {
		return false
	}
	if w.Reason == "InvalidImageName" {
		return true
	}
	if (w.Reason != "ErrImagePull" && w.Reason != "ImagePullBackOff") ||
		created.IsZero() || now.Sub(created) < imagePullGrace {
		return false
	}
	message := strings.ToLower(w.Message)
	for _, answer := range []string{"code = notfound", "manifest unknown", "manifest_unknown", ": not found", "404 not found", "unauthorized",
		"authentication required", "pull access denied", "repository does not exist", "insufficient_scope", "403 forbidden", "denied:"} {
		if strings.Contains(message, answer) {
			return true
		}
	}
	return false
}

// Image failures cannot be diagnosed from command logs: the command never ran.
func pullFailureMessage(w *corev1.ContainerStateWaiting) string {
	if w.Reason == "InvalidImageName" {
		return "image reference is invalid: " + w.Message + "; the pre-deploy command never ran"
	}
	return "image pull is failing: " + w.Message + "; the pre-deploy command never ran"
}

// pullFailure is the pre-deploy container's image-pull wait on the Job's pod,
// with the pod's creation time, or nil when it is not waiting on a pull error.
func pullFailure(ctx context.Context, cl client.Reader, j *batchv1.Job) (*corev1.ContainerStateWaiting, time.Time) {
	for _, p := range jobPods(ctx, cl, j) {
		for _, cs := range p.Status.ContainerStatuses {
			if w := cs.State.Waiting; cs.Name == containerName && w != nil &&
				(w.Reason == "ErrImagePull" || w.Reason == "ImagePullBackOff" || w.Reason == "InvalidImageName") {
				return w, p.CreationTimestamp.Time
			}
		}
	}
	return nil, time.Time{}
}

// terminatedContainer is the pre-deploy container's terminated state on the
// Job's pod, or nil when no pod reports one (reaped, or never started). With
// BackoffLimit 0 a Job runs exactly one pod.
func terminatedContainer(ctx context.Context, cl client.Reader, j *batchv1.Job) *corev1.ContainerStateTerminated {
	for _, p := range jobPods(ctx, cl, j) {
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Name == containerName && cs.State.Terminated != nil {
				return cs.State.Terminated
			}
		}
	}
	return nil
}

// jobPods lists the Job's pods, nil on error.
func jobPods(ctx context.Context, cl client.Reader, j *batchv1.Job) []corev1.Pod {
	var pods corev1.PodList
	// The controller-uid label, not the Job name: a TTL-reaped Job recreated
	// under the same name must never borrow the old run's pod.
	if err := cl.List(ctx, &pods, client.InNamespace(j.Namespace),
		client.MatchingLabels{batchv1.ControllerUidLabel: string(j.UID)}); err != nil {
		return nil
	}
	return pods.Items
}

// CancelSuperseded deletes active (not Complete, not Failed) pre-deploy Jobs for
// the named service EXCEPT keep (the current revision's Job). This is the
// newest-wins policy — a new revision supersedes an older revision's in-flight
// migration — without disturbing the current one. Not-found on delete is
// tolerated (concurrent GC). An initial deploy hook's Job is never superseded:
// a one-time seed stopped halfway is worse than one that finishes, and its
// outcome gates nothing.
//
// appUID scopes the selection to this App's immutable, globally-unique UID
// (round-5 finding 5): the Job name derives from the workspace-local service
// name, so a name-only selector would interrupt (and could replay) a same-named
// App's migration from another lifetime/namespace. Empty appUID degrades to the
// prior name-only behavior.
func CancelSuperseded(ctx context.Context, name, appUID, namespace, keep string, cl client.Client) error {
	sel := client.MatchingLabels{LabelService: name}
	if appUID != "" {
		sel[execution.LabelAppUID] = appUID
	}
	var jobs batchv1.JobList
	if err := cl.List(ctx, &jobs, client.InNamespace(namespace), sel); err != nil {
		return fmt.Errorf("predeploy: list for %s: %w", name, err)
	}
	for i := range jobs.Items {
		j := &jobs.Items[i]
		if j.Name == keep || isHook(j) {
			continue
		}
		if execution.JobFinished(j) {
			continue
		}
		if err := cl.Delete(ctx, j); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("predeploy: cancel %s: %w", j.Name, err)
		}
	}
	return nil
}
