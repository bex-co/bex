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

// Package build is the bex build plane: turn a git repo @ ref into an OCI image
// pushed to the in-cluster registry (Zot), built ENTIRELY IN-CLUSTER by a
// Kubernetes workload — no docker daemon and no host tools, so it works on
// containerd app nodes (w1/m5, w6/m22). Dockerfile builds use a BuildKit Job
// confined by a Kubernetes Pod user namespace; Cloud Native Buildpack builds
// use a kpack Image CR. In both
// cases the operator only dispatches the workload and observes it while the
// heavy lifting stays inside the cluster. See docs/ADR004-app-deployment.md.
package build

import (
	"cmp"
	"context"
	"fmt"
	"net"
	"path"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/identity"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// buildComponent is the app.bex.co/component value every build artifact the
// platform creates carries — the selector every build counter, gauge and trim
// matches on. A literal here and a literal in the selector would be two facts
// that must agree; this is one.
const buildComponent = "build"

// Builder strategies (mirror App.spec.builder).
const (
	BuilderAuto       = "auto"
	BuilderDockerfile = "dockerfile"
	BuilderBuildpack  = "buildpack"
	BuilderNative     = "native"
	defaultRevision   = appv1alpha1.DefaultRevision
)

// Platform-plane images are pinned and bumped deliberately. BuildKit runs as
// root inside a Kubernetes Pod user namespace: it has the namespaced
// capabilities needed for its OCI worker while remaining unprivileged on the
// node, and its default process sandbox keeps Dockerfile RUN processes out of
// the daemon's PID namespace.
const (
	defaultBuildkitImage = "moby/buildkit:v0.30.0@sha256:0168606be2315b7c807a03b3d8aa79beefdb31c98740cebdffdfeebf31190c9f"
	defaultGitImage      = "alpine/git:v2.43.0@sha256:76fdb7210689fc26c6ff101c4adacf9d12d3d25a919a7d8ff42ebaef5bedba65"
	defaultPushImage     = "quay.io/skopeo/stable@sha256:c7d3c512612f52805023cd38351081dad7e2729fc13d14b701e47c7c8bdd6615" // v1.22.2 multi-arch manifest
)

// defaultSignImage is the cosign image the signing container runs when tenant
// image signing is enabled (w6/006). Pinned; bump deliberately.
const defaultSignImage = "gcr.io/projectsigstore/cosign:v2.4.1@sha256:b03690aa52bfe94054187142fba24dc54137650682810633901767d8a3e15b31"

// Reserved build-phase exit codes (docs/ADR060 D2). A failing phase exits with
// one of these so the Job's podFailurePolicy can act on the failure CLASS, and
// so the controller can attribute it. WHICH phase failed is already carried by
// the container name in the pod status, so a class plus that name is a complete
// classification without a per-phase code space to keep in sync.
//
// The band must stay below 126 (the shell's "not executable"/"not found") and
// below 128, so a classified failure can never be confused with a 128+N signal
// exit — 137 OOM and 143 SIGTERM in particular.
const (
	// ExitTenantError is a deterministic failure in tenant input; retrying
	// cannot change the outcome, so podFailurePolicy fails the Job at once.
	ExitTenantError = 90
	// ExitTransient is a failure a retry might fix (network, registry 5xx, DNS).
	ExitTransient = 91
)

// transientLogPattern recognizes failures worth retrying, by matching the
// error text the phase actually printed. Everything unmatched is treated as a
// TENANT error, and that default is deliberate: ADR060 D2's rule is "never
// auto-retry a deterministic user failure; always absorb infrastructure
// disruption", and the two misclassifications are not symmetric. Calling a
// transient failure "tenant" costs one free retry; calling a tenant failure
// "transient" burns two doomed attempts holding a 7 GiB node-exclusive slot.
//
// Infrastructure disruption proper (eviction, preemption, drain) never reaches
// this classifier at all: it surfaces as the DisruptionTarget pod condition,
// which podFailurePolicy ignores before any exit code is considered.
//
// Lines BuildKit prefixes with a step marker ("#12 0.42 …") are excluded before
// matching, because those carry the tenant's own RUN output. Without that, a
// Dockerfile containing `RUN echo "connection refused"` would classify its own
// deterministic failure as transient — earning a free retry, and, once the
// budget is exhausted, landing in infra_failed where it would move the platform
// SLO and feed the correlated-failure page.
const transientLogPattern = `dial tcp|connection refused|connection reset|network is unreachable|` +
	`no route to host|i/o timeout|TLS handshake timeout|context deadline exceeded|` +
	`temporary failure in name resolution|could not resolve host|server misbehaving|` +
	`unexpected status: 5[0-9][0-9]|response status code 5[0-9][0-9]|` +
	`50[0234] (internal server error|bad gateway|service unavailable|gateway time-?out)|` +
	`failed to do request|unexpected eof|early eof|rpc failed|the remote end hung up|` +
	`failed to connect to|operation timed out|unable to access`

// classifyPrelude is prepended to every classifying phase script. It runs the
// phase, streams its output live (so build logs keep reaching the log shipper
// unbuffered — w7/m28), captures a copy for classification, and exits with the
// reserved code for the class.
//
// SECURITY: bex_run passes the phase's arguments through "$@" as discrete
// positional parameters and never interpolates them into the script text, so
// tenant-controlled values (context dir, Dockerfile path, image ref) cannot
// become shell syntax.
//
// The exit code is written inside the brace group rather than read from the
// pipeline, because $? after a pipeline is tee's status and `set -o pipefail`
// is not portable across the busybox shells these images ship. The `set +e`
// around the pipeline is load-bearing: the phases run under `sh -eu`, and
// without it the brace group aborts the instant the command fails, so `echo $?`
// never runs and the exit code is lost.
var classifyPrelude = fmt.Sprintf(`bex_rc_file=/tmp/bex-rc; bex_log=/tmp/bex-phase.log
bex_run() {
  rm -f "$bex_rc_file" "$bex_log"
  set +e
  { "$@" 2>&1; echo $? > "$bex_rc_file"; } | tee /dev/stderr | tail -c %d > "$bex_log"
  bex_rc="$(cat "$bex_rc_file" 2>/dev/null || echo missing)"
  set -e
  [ "$bex_rc" = 0 ] && return 0
  if [ "$bex_rc" = missing ]; then
    echo "bex: phase exit status unavailable; treating as retryable" >&2
    exit %d
  fi
  if grep -v '^#[0-9][0-9]* ' "$bex_log" 2>/dev/null | grep -qiE %s; then
    exit %d
  fi
  exit %d
}
`, classifyBufferBytes, ExitTransient, shellSingleQuote(transientLogPattern), ExitTransient, ExitTenantError)

// classifyBufferBytes bounds the copy of phase output kept for classification.
// The full stream still reaches the container log; only this tail is retained.
//
// Two reasons it must be bounded. The file lives on the container's writable
// layer, which shares the phase's ephemeral-storage limit — and exceeding that
// limit gets the pod evicted with DisruptionTarget set, which buildPodFailurePolicy
// IGNORES, so an unbounded log turns a deterministic disk-filling build into a
// retry loop that stops only at activeDeadlineSeconds. Second, the classifying
// grep is proportional to what it scans, and busybox grep on a multi-hundred-MB
// log would spend minutes of the build's own deadline.
const classifyBufferBytes = 256 * 1024

// shellSingleQuote renders s as a single-quoted shell word, so the pattern
// stays correct if it ever grows a quote.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// BuildTimeout bounds a single build Job's wall-clock before Build gives up
// waiting (the Job's own activeDeadlineSeconds matches, so a stuck build is
// reaped rather than lingering).
//
// Exported because it is also the floor for the registry's GC delay: a blob
// from an in-flight push must not be collectable while its build can still be
// running (docs/ADR060 D4).
const BuildTimeout = 30 * time.Minute

// Build execution resources match Render's Starter pipeline tier (2 CPU, 8 GB
// RAM) while remaining schedulable on the baseline 8 GB tenant nodes. The 7 GiB
// memory request leaves allocatable headroom for kubelet and DaemonSets but still
// makes a builder effectively node-exclusive, so Cluster Autoscaler adds a node
// instead of co-locating a memory-hungry build with serving workloads. Use the
// decimal 8G limit because the advertised tier is 8 GB, not 8 GiB.
const (
	buildCPURequest    = "2"
	buildMemoryRequest = "7Gi"
	buildCPULimit      = "2"
	buildMemoryLimit   = "8G"

	// Ephemeral-storage bounds (codex-security #3): tenant-controlled build output
	// fills disk-backed emptyDirs that had no SizeLimit, and the containers carried
	// only CPU/memory limits — a malicious repo/Dockerfile/image could fill the
	// node and evict co-tenant workloads. Render cancels builds above 16 GB of disk
	// use, so use the exact decimal limit for the build workspace and every phase
	// that mounts it. In particular, kubelet charges the read-only OCI archive to
	// Skopeo while it pushes; its old light-container limit evicted valid images
	// larger than 2 GiB. Signing does not mount the archive and stays lightweight.
	buildEphemeralRequest = "10Gi"
	buildEphemeralLimit   = "16G"
	pushEphemeralRequest  = "1Gi"
	pushEphemeralLimit    = "16G"
	lightEphemeralRequest = "1Gi"
	lightEphemeralLimit   = "2Gi"

	// emptyDirSizeLimit bounds each tenant-controlled disk-backed volume.
	emptyDirSizeLimit = "16G"
)

// mustSizeLimit parses s into a *resource.Quantity for an emptyDir SizeLimit
// (Kubernetes wants a pointer; resource.MustParse returns a value).
func mustSizeLimit(s string) *resource.Quantity {
	q := resource.MustParse(s)
	return &q
}

const (
	dockerConfigMount = "/docker-config"
	sourceMount       = "/source"
	outputMount       = "/output"
)

// mountRegistryCred attaches the docker-config volume (read-only) + DOCKER_CONFIG
// env to one explicitly selected phase. BuildKit receives only an optional
// private-base pull credential; skopeo/cosign receive only the output-repository
// credential. It is a no-op when secret is empty.
func mountRegistryCred(c *corev1.Container, volumeName, secret string) {
	if secret == "" {
		return
	}
	c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
		Name: volumeName, ReadOnly: true, MountPath: dockerConfigMount,
	})
	c.Env = append(c.Env, corev1.EnvVar{Name: "DOCKER_CONFIG", Value: dockerConfigMount})
}

// Options configures a single in-cluster build.
type Options struct {
	Repo           string // git URL fetched by the credential-isolated clone phase
	Ref            string // branch or commit; defaults to the repo's default branch
	ExpectedCommit string // optional full object id that the fetched checkout must match
	RootDir        string // subdirectory of Repo to build from (App.spec.rootDir); empty = repo root
	DockerfilePath string // Dockerfile path relative to RootDir; empty = Dockerfile
	Name           string // image repo name (the service name)
	AppUID         string // immutable App UID; prevents same-name recreation from reusing stale artifacts
	Registry       string // in-cluster registry host, e.g. zot.bex-registry.svc:5000
	// KpackRegistry is an optional alias for Registry used only by kpack. Kpack's
	// upstream registry client deliberately treats *.local names as plain HTTP,
	// which lets the GitOps-provided zot.local alias reach the same development
	// Zot Service without disabling TLS for arbitrary registries. Empty uses
	// Registry (the normal production/TLS path).
	KpackRegistry string
	Revision      string // image tag, operator-supplied (e.g. "gen-7") — deterministic per revision
	Builder       string // auto | dockerfile | buildpack | native (App.spec.builder)
	Runtime       string // Render native runtime: node | python | ruby | go | rust | elixir
	BuildCommand  string // Render native build command (BuilderNative only)
	StartCommand  string // Render native start command (BuilderNative only)
	// StaticSite marks a static_site App's build: a native build then needs no
	// startCommand (the image is only the publish Job's extract source, never
	// run) and an undeclared runtime defaults to the node toolchain — Render's
	// static build environment (ADR029, w8/m19).
	StaticSite bool
	// DockerContext moves the Docker build context to this repo-root-relative
	// directory (Render's dockerContext; independent of RootDir). Empty keeps
	// the RootDir-derived context. Dockerfile builds only.
	DockerContext string
	// BuildEnv carries selected literal build-time environment. Kpack receives
	// BP_*/BPE_* entries in Image.spec.build.env; native builds encode all
	// literals into a BuildKit secret alongside RuntimeEnvSecret.
	BuildEnv         []corev1.EnvVar
	RuntimeEnvSecret string // optional Secret whose keys also enter a native build
	Namespace        string // namespace the build Job runs in
	// Workspace is the owning tenant id (app.bex.co/workspace label value) stamped
	// on the build Job so per-workspace concurrent-build counting works (w7/m9).
	// Empty = label omitted (legacy/hand-applied Apps without a workspace label).
	Workspace string
	// AppNamespace is the namespace the App CR lives in — which may differ from
	// Namespace (the build Job's namespace) when BEX_BUILD_NAMESPACE is set. When
	// set and different from Namespace, it is stamped as app.bex.co/app-namespace
	// on the build pod so the log-shipper can attribute logs to the right stream.
	// Empty or equal to Namespace = label omitted (the shipper uses the pod's own
	// namespace, which already matches the App's).
	AppNamespace string
	Client       client.Client // cluster client used to create + watch the Job
	// CloneSecret names a Secret (in Namespace, key "token") whose value is
	// exposed only to the short-lived clone init container. It never enters the
	// BuildKit process boundary. Empty = public clone.
	CloneSecret string
	// GitImage overrides the source-clone image (tests / air-gapped).
	GitImage string
	// BuildkitImage overrides the user-namespace-confined BuildKit image (tests / air-gapped).
	BuildkitImage string
	// SignKeySecret names a Secret (in Namespace, keys "cosign.key",
	// "cosign.password", and "cosign.pub") whose cosign key signs the pushed
	// tenant image after a successful build (w6/006). Empty = unsigned (the
	// default; existing builds are byte-identical). When set, build+push moves
	// to an initContainer and a cosign container signs as the main container
	// (k8s runs init → containers sequentially, so signing only fires on a
	// successful push). Admission-time signature verification is enforced by the
	// /validate-v1-pod webhook when "cosign.pub" is present in this Secret
	// (w7/m11, docs/ADR028-security-review.md).
	SignKeySecret string
	// SignImage overrides the cosign image used by the signing container.
	SignImage string
	// PushImage overrides the skopeo image used by the credential-isolated push
	// phase (tests / air-gapped).
	PushImage string
	// PushSecret authenticates only the post-build skopeo/cosign phases. BuildKit
	// exports an OCI archive and never receives this credential.
	PushSecret string
	// PullSecret is the optional, per-App read-only credential for private
	// Dockerfile base images. Unlike PushSecret, it may be mounted only in the
	// BuildKit phase. It never grants access to the platform output repository.
	PullSecret string
	// RegistryConfig makes buildkitd consume buildkitd.toml from PullSecret.
	RegistryConfig bool
}

// Phase is the observed lifecycle of a dispatched build (ADR060 §D1: the
// operator observes builds non-blocking, one create + one read per reconcile,
// mirroring internal/predeploy).
type Phase int

const (
	// PhaseBuilding: the build is running — a pod is placed on a node (Dockerfile/
	// native) or kpack reports it building. This is the "build clock is ticking"
	// state.
	PhaseBuilding Phase = iota
	// PhaseWaiting: dispatched, but no pod has been placed on a node yet — waiting
	// for capacity. A build requests 2 CPU + 7Gi and is node-exclusive by design,
	// so it can sit Pending for many minutes; charging that wait to the build's own
	// budget is what let a healthy build be reported failed (2026-08-11: 22 minutes
	// Pending closed the deploy while the build was still running). The control
	// plane maps this to the deploy row's queued status and restarts the build
	// budget on the Waiting→Building edge.
	PhaseWaiting
	// PhaseSucceeded: terminal — Image is set.
	PhaseSucceeded
	// PhaseFailed: terminal — Message explains why; Timeout distinguishes an
	// activeDeadlineSeconds reap from a tenant/infra build failure.
	PhaseFailed
)

// Observation is the non-blocking result of EnsureBuild: the build's current
// phase plus, when terminal, the resolved image or a failure message.
type Observation struct {
	Phase      Phase
	Image      string  // set when Phase == PhaseSucceeded (<registry>/<name>:<revision>, or kpack's canonical digest)
	Message    string  // failure detail (PhaseFailed) or capacity reason (PhaseWaiting)
	RunSeconds float64 // terminal build run duration from the Job's own status timestamps (0 = unknown / kpack)
	// Fault attributes a PhaseFailed build (docs/ADR060 D2). Empty for
	// non-terminal phases.
	Fault Fault
	// Created reports that THIS call created the build Job, as opposed to
	// adopting one an earlier reconcile already dispatched. It is what makes the
	// admission gate's re-count-after-create honest (docs/ADR060 D6): the caller
	// re-counts only on the pass that actually added a build, so the correction
	// costs nothing on the many observation passes that follow. False on the
	// kpack path, which dispatches an Image rather than a Job.
	Created bool
}

// Fault is who a failed build is attributable to. The split is what makes an
// infrastructure-success SLO definable: a tenant's broken Dockerfile is not a
// platform failure and must not burn the error budget.
type Fault string

const (
	// FaultNone is a build that did not fail, or one whose class is not known.
	FaultNone Fault = ""
	// FaultTenant is a deterministic failure in tenant input — a bad git ref, a
	// Dockerfile that does not build. Not retried, not counted against the SLO.
	FaultTenant Fault = "tenant"
	// FaultInfra is a failure the platform owns: a transient that exhausted its
	// retries, or any unclassified failure that did the same.
	FaultInfra Fault = "infra"
	// FaultTimeout is the build exceeding activeDeadlineSeconds. Its own class
	// rather than a fault of either side: the deadline is a platform policy, but
	// hitting it is usually the tenant's build being slow.
	FaultTimeout Fault = "timeout"
)

// faultFromJob reads the class off the Job's own failure reason, which needs no
// extra API call and no pod listing: buildPodFailurePolicy's only FailJob rule
// matches ExitTenantError, so Kubernetes writing "PodFailurePolicy" as the
// reason IS the statement that a tenant-classified phase failed. Anything that
// instead exhausted the backoff budget was, by construction, not tenant-classified.
func faultFromJob(reason string) Fault {
	switch reason {
	case batchv1.JobReasonPodFailurePolicy:
		return FaultTenant
	case batchv1.JobReasonBackoffLimitExceeded:
		return FaultInfra
	case batchv1.JobReasonDeadlineExceeded:
		return FaultTimeout
	default:
		return FaultNone
	}
}

// RepoPath is the OCI repository (no host, no tag) this build pushes to.
func (o Options) RepoPath() string {
	return identity.ForApp(o.Name, o.Workspace).Repo()
}

// ImageRef is the deterministic image reference a build produces — the operator
// knows it before the build runs (registry + name + revision), so it can gate
// rebuilds on the revision changing and set status.image without parsing Job
// output. Labeled Apps use the workspace-scoped repository (docs/ADR074).
func (o Options) ImageRef() string {
	rev := o.Revision
	if rev == "" {
		rev = defaultRevision
	}
	return fmt.Sprintf("%s/%s:%s", o.Registry, o.RepoPath(), rev)
}

// KpackImageRef is the deterministic tag a buildpack build pushes. It normally
// equals ImageRef; the separate registry alias exists for the in-cluster HTTP
// Zot endpoint described on Options.KpackRegistry.
func (o Options) KpackImageRef() string {
	registry := o.KpackRegistry
	if registry == "" {
		registry = o.Registry
	}
	rev := o.Revision
	if rev == "" {
		rev = defaultRevision
	}
	return fmt.Sprintf("%s/%s:%s", registry, o.RepoPath(), rev)
}

// EnsureBuild dispatches the selected in-cluster builder if it is not already
// running and returns its current Observation without blocking — one create +
// one read per call, the shape internal/predeploy uses (ADR060 §D1). The caller
// requeues while the phase is non-terminal, so a reconcile worker is never
// parked for the minutes a build takes and a newer generation's reconcile can
// run promptly. Re-invocation for one App generation is idempotent: both
// mechanisms use the same deterministic build name and adopt an existing
// Job/Image only after exact UID validation.
func EnsureBuild(ctx context.Context, o Options) (Observation, error) {
	if o.Client == nil {
		return Observation{}, fmt.Errorf("build: nil client (in-cluster builds require a cluster client)")
	}
	if o.AppUID == "" {
		return Observation{}, fmt.Errorf("build: empty App UID")
	}
	if o.Builder == BuilderBuildpack {
		return ensureBuildpack(ctx, o)
	}
	if o.Builder == BuilderNative {
		if err := validateNativeOptions(o); err != nil {
			return Observation{}, err
		}
	}

	image := o.ImageRef()
	owned := execution.ArtifactIdentity{Name: o.Name, UID: o.AppUID, Workspace: o.Workspace, Namespace: o.AppNamespace}
	cur, created, err := execution.EnsureOwnedJob(ctx, o.Client, BuildJob(o, image), owned, "build")
	if err != nil {
		return Observation{}, err
	}
	switch {
	case execution.JobHasCondition(cur, batchv1.JobComplete):
		return Observation{Phase: PhaseSucceeded, Image: image, RunSeconds: jobRunSeconds(cur), Created: created}, nil
	case execution.JobHasCondition(cur, batchv1.JobFailed):
		return Observation{
			Phase:      PhaseFailed,
			Message:    execution.JobFailureMessage(cur, "unknown build failure"),
			RunSeconds: jobRunSeconds(cur),
			Fault:      faultFromJob(execution.JobFailedReason(cur)),
			Created:    created,
		}, nil
	}
	if queued, reason := buildQueued(ctx, o, cur.Name); queued {
		return Observation{Phase: PhaseWaiting, Message: reason, Created: created}, nil
	}
	return Observation{Phase: PhaseBuilding, Created: created}, nil
}

// jobRunSeconds is a finished build Job's run duration from its own status
// timestamps (0 when either is unset). Reading the Job's clock, not the
// operator's, is what makes the duration survive a manager restart mid-build.
func jobRunSeconds(j *batchv1.Job) float64 {
	if j.Status.StartTime == nil || j.Status.CompletionTime == nil {
		return 0
	}
	s := j.Status.CompletionTime.Sub(j.Status.StartTime.Time).Seconds()
	if s < 0 {
		return 0
	}
	return s
}

// jobNameLabel is the label the Job controller stamps on every pod it creates.
const jobNameLabel = "job-name"

// pushContainer names the phase that pushes the built archive to the registry.
// It is the container BuildJob creates AND the one the push SLIs are read off
// (internal/build/signals.go), so the two must be the same string by
// construction rather than by coincidence.
const pushContainer = "push"

// buildQueued reports whether the Job's build is still waiting for capacity
// rather than doing work, and the scheduler's reason when it is.
//
// Errors and the no-pods-yet window are reported as NOT queued: this only gates
// a status nicety, and guessing "queued" on a failed List would stall the
// caller's clock on evidence it does not have. (The census gauges make the
// opposite choice for the no-pods window — see CountBuilds — because there the
// gap between a Job's creation and its first pod is exactly the cold
// scale-from-zero wait the gauge exists to show.)
func buildQueued(ctx context.Context, o Options, jobName string) (bool, string) {
	pods, err := listJobPods(ctx, o.Client, o.Namespace, jobName)
	if err != nil || len(pods) == 0 {
		return false, ""
	}
	if podsPlaced(pods) {
		return false, ""
	}
	return true, unschedulableReason(pods)
}

// listJobPods returns the pods one Job has created, across all of its attempts.
func listJobPods(ctx context.Context, cl client.Client, namespace, jobName string) ([]corev1.Pod, error) {
	var pods corev1.PodList
	if err := cl.List(ctx, &pods,
		client.InNamespace(namespace),
		client.MatchingLabels{jobNameLabel: jobName},
	); err != nil {
		return nil, fmt.Errorf("list pods for job %s: %w", jobName, err)
	}
	return pods.Items, nil
}

// podsPlaced reports whether any live attempt of a build has been bound to a
// node — the ONE definition of the queued/running boundary, shared by the App's
// Waiting phase and the census gauges so they can never disagree.
//
// "Placed on a node" is that boundary, NOT pod phase: the build runs in
// initContainers (clone, then BuildKit), so a pod actively compiling reports
// phase Pending for most of its life, and reading phase would classify a healthy
// mid-build pod as queued. A pod with spec.nodeName set is consuming capacity
// and making progress; one the scheduler could not place is not. A spent attempt
// (Succeeded/Failed) says nothing about the current one.
func podsPlaced(pods []corev1.Pod) bool {
	for i := range pods {
		pod := &pods[i]
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		if pod.Spec.NodeName != "" {
			return true
		}
	}
	return false
}

// unschedulableReason is the scheduler's own explanation for why a build could
// not be placed, so the tenant sees "0/3 nodes are available: insufficient
// memory" rather than a generic wait.
func unschedulableReason(pods []corev1.Pod) string {
	for i := range pods {
		pod := &pods[i]
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse && cond.Message != "" {
				return cond.Message
			}
		}
	}
	return "waiting for a node with capacity for the build"
}

// BuildJob constructs the credential-separated source build Job for o. Its
// phases are strictly serial: clone -> build OCI archive -> push -> optional
// sign. Kubernetes gives each container a separate root filesystem and PID
// namespace, so tenant Dockerfile processes never coexist with clone, platform
// push, or signing credentials. Only an explicitly configured private-base
// pull credential enters BuildKit, and it grants no platform-repository write.
// emptyDirVolume and secretVolume are the two volume shapes every build Job
// mounts; the size limit is uniform so a build cannot fill a node's ephemeral
// storage regardless of which scratch volume it writes to.
func emptyDirVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name:         name,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: mustSizeLimit(emptyDirSizeLimit)}},
	}
}

func secretVolume(name, secretName string) corev1.Volume {
	return corev1.Volume{
		Name:         name,
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretName}},
	}
}

// boundedSourceDir resolves a repo-relative directory inside the source
// mount, structurally containing traversal input (path.Clean under a rooted
// prefix) — the shared rule for RootDir and DockerContext.
func boundedSourceDir(dir string) string {
	if clean := strings.TrimPrefix(path.Clean("/"+dir), "/"); clean != "." && clean != "" {
		return path.Join(sourceMount, clean)
	}
	return sourceMount
}

func BuildJob(o Options, image string) *batchv1.Job {
	buildkitImage := cmp.Or(o.BuildkitImage, defaultBuildkitImage)
	gitImage := cmp.Or(o.GitImage, defaultGitImage)
	pushImage := cmp.Or(o.PushImage, defaultPushImage)
	signImage := cmp.Or(o.SignImage, defaultSignImage)

	contextDir := boundedSourceDir(o.RootDir)
	// The Dockerfile keeps resolving against RootDir even when DockerContext
	// moves the build context — Docker's own context-vs-dockerfile split, and
	// Render's dockerContext semantics (repo-root-relative, independent of
	// rootDir).
	dockerfileDir := contextDir
	if o.DockerContext != "" {
		contextDir = boundedSourceDir(o.DockerContext)
	}
	args := []string{
		"build",
		"--frontend", "dockerfile.v0",
		"--local", "context=" + contextDir,
		"--output", "type=oci,dest=" + outputMount + "/image.tar",
	}
	if o.Builder == BuilderNative {
		args = append(args,
			"--local", "dockerfile=/native",
			"--opt", "filename=Dockerfile",
			"--secret", "id=render-env,src=/native/render-env",
		)
	} else {
		args = append(args, "--local", "dockerfile="+dockerfileDir)
		if o.DockerfilePath != "" {
			args = append(args, "--opt", "filename="+o.DockerfilePath)
		}
	}

	// Keep BuildKit's default OCI process sandbox enabled; the Pod user namespace
	// (spec.hostUsers=false) scopes the capabilities it needs without exposing
	// host-root capabilities outside the pod.
	buildkitdFlags := ""
	if o.RegistryConfig {
		buildkitdFlags = "--config " + dockerConfigMount + "/buildkitd.toml"
	}
	var env []corev1.EnvVar
	if buildkitdFlags != "" {
		env = []corev1.EnvVar{{Name: "BUILDKITD_FLAGS", Value: buildkitdFlags}}
	}

	deadline := int64(BuildTimeout / time.Second)
	// Two attempts for an UNCLASSIFIED failure only: podFailurePolicy fails a
	// tenant-classified failure outright, so the extra attempt is spent only
	// where a retry can change the outcome (docs/ADR060 D2).
	backoff := int32(2)
	ttl := int32(3600) // reap the finished Job after an hour

	volumes := []corev1.Volume{emptyDirVolume("source"), emptyDirVolume("output")}
	if o.PushSecret != "" {
		volumes = append(volumes, secretVolume("push-registry-cred", o.PushSecret))
	}
	if o.PullSecret != "" {
		volumes = append(volumes, secretVolume("pull-registry-cred", o.PullSecret))
	}
	if o.Builder == BuilderNative {
		volumes = append(volumes, emptyDirVolume("native-build"))
		if o.RuntimeEnvSecret != "" {
			volumes = append(volumes, secretVolume("runtime-env", o.RuntimeEnvSecret))
		}
	}

	clone := buildCloneContainer(o, gitImage)

	// BuildKit receives the checked-out tree and writes an OCI archive. It has no
	// clone, platform push, or signing credential.
	// Tenant code, so it classifies. Args stay positional (see classifyPrelude).
	buildkit := corev1.Container{
		Name:    "buildkit",
		Image:   buildkitImage,
		Command: []string{"sh", "-eu", "-c", classifyPrelude + `bex_run buildctl-daemonless.sh "$@"`, "bex-buildkit"},
		Args:    args,
		Env:     env,
		VolumeMounts: []corev1.VolumeMount{
			{Name: "source", MountPath: sourceMount, ReadOnly: true},
			{Name: "output", MountPath: outputMount},
		},
		// These capabilities are confined to the Pod user namespace
		// (spec.hostUsers=false), so BuildKit can mount snapshots and create nested
		// namespaces without acquiring the corresponding powers on the node.
		// Unconfined seccomp/AppArmor remains scoped to this one container; the
		// default OCI worker process sandbox still isolates Dockerfile RUN PIDs.
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:                ptr.To(int64(0)),
			RunAsGroup:               ptr.To(int64(0)),
			AllowPrivilegeEscalation: ptr.To(true),
			Capabilities: &corev1.Capabilities{Add: []corev1.Capability{
				"AUDIT_WRITE", "CHOWN", "DAC_OVERRIDE", "FOWNER", "FSETID",
				"KILL", "MKNOD", "NET_ADMIN", "NET_BIND_SERVICE", "NET_RAW",
				"SETFCAP", "SETGID", "SETPCAP", "SETUID", "SYS_ADMIN", "SYS_CHROOT",
			}},
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeUnconfined,
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse(buildCPURequest),
				corev1.ResourceMemory:           resource.MustParse(buildMemoryRequest),
				corev1.ResourceEphemeralStorage: resource.MustParse(buildEphemeralRequest),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse(buildCPULimit),
				corev1.ResourceMemory:           resource.MustParse(buildMemoryLimit),
				corev1.ResourceEphemeralStorage: resource.MustParse(buildEphemeralLimit),
			},
		},
	}
	if o.Builder == BuilderNative {
		buildkit.VolumeMounts = append(buildkit.VolumeMounts, corev1.VolumeMount{Name: "native-build", MountPath: "/native"})
	}
	mountRegistryCred(&buildkit, "pull-registry-cred", o.PullSecret)

	// Whole-blob retry, deliberately not chunked-upload resume: resume paths are
	// where registries historically corrupt uploads (docs/ADR060 D4).
	pushArgs := []string{"copy", "--retry-times", "3"}
	// Only the cluster-local endpoint may skip verification: the push credential
	// travels with this request, so an external registry must be authenticated
	// (.pm/w1/046.md F11).
	if registryIsClusterLocal(o.Registry) {
		pushArgs = append(pushArgs, "--dest-tls-verify=false")
	}
	if o.PushSecret != "" {
		pushArgs = append(pushArgs, "--authfile", dockerConfigMount+"/config.json")
	}
	pushArgs = append(pushArgs, "oci-archive:"+outputMount+"/image.tar", "docker://"+image)
	pusher := corev1.Container{
		Name:    pushContainer,
		Image:   pushImage,
		Command: []string{"skopeo"},
		Args:    pushArgs,
		VolumeMounts: []corev1.VolumeMount{
			{Name: "output", MountPath: outputMount, ReadOnly: true},
		},
		SecurityContext: restrictedContainerSecurityContext(),
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("100m"),
				corev1.ResourceMemory:           resource.MustParse("128Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse(pushEphemeralRequest),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("1"),
				corev1.ResourceMemory:           resource.MustParse("512Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse(pushEphemeralLimit),
			},
		},
	}
	mountRegistryCred(&pusher, "push-registry-cred", o.PushSecret)

	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		InitContainers: []corev1.Container{
			clone,
		},
		Containers: []corev1.Container{pusher},
	}
	if o.Builder == BuilderNative {
		podSpec.InitContainers = append(podSpec.InitContainers, nativeBuildPreparer(o))
	}
	podSpec.InitContainers = append(podSpec.InitContainers, buildkit)
	execution.HardenPod(&podSpec)
	execution.TolerateBuildPool(&podSpec)
	podSpec.SecurityContext.FSGroup = ptr.To(int64(0))
	// No safe-to-evict pin: buildPodFailurePolicy absorbs eviction rather than
	// preventing it, so pinning would only block node consolidation.
	annotations := map[string]string{
		"container.apparmor.security.beta.kubernetes.io/buildkit": "unconfined",
	}
	// Tenant-image signing (w6/006): when a signing key Secret is configured, the
	// build+push moves to an initContainer and a cosign container signs the pushed
	// image as the main container. k8s runs initContainers → containers
	// sequentially, so cosign only runs after a successful push — no sh -c wrapping
	// of the build (the validated context string stays a discrete container arg).
	if o.SignKeySecret != "" {
		sign := corev1.Container{
			Name:    "sign",
			Image:   signImage,
			Command: []string{"cosign"},
			Args: []string{
				"sign", "--yes", "--tlog-upload=false", "--allow-http-registry",
				"--key", "/keys/cosign.key", image,
			},
			Env: []corev1.EnvVar{{
				Name: "COSIGN_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: o.SignKeySecret},
					Key:                  "cosign.password",
				}},
			}},
			VolumeMounts:    []corev1.VolumeMount{{Name: "cosign-key", ReadOnly: true, MountPath: "/keys"}},
			SecurityContext: restrictedContainerSecurityContext(),
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("100m"),
					corev1.ResourceMemory:           resource.MustParse("128Mi"),
					corev1.ResourceEphemeralStorage: resource.MustParse(lightEphemeralRequest),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("1"),
					corev1.ResourceMemory:           resource.MustParse("512Mi"),
					corev1.ResourceEphemeralStorage: resource.MustParse(lightEphemeralLimit),
				},
			},
		}
		mountRegistryCred(&sign, "push-registry-cred", o.PushSecret)
		podSpec.InitContainers = append(podSpec.InitContainers, pusher)
		podSpec.Containers = []corev1.Container{sign}
		volumes = append(volumes, corev1.Volume{
			Name:         "cosign-key",
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: o.SignKeySecret}},
		})
	}
	// Must run AFTER the signing block above, which moves the push phase into
	// InitContainers and replaces Containers wholesale — applying it any earlier
	// silently misses both of those phases.
	captureFailureTail(&podSpec)
	podSpec.Volumes = volumes

	appNamespace := ""
	if o.AppNamespace != "" && o.AppNamespace != o.Namespace {
		appNamespace = o.AppNamespace
	}
	labels := execution.PodLabels(o.Name, o.AppUID, buildComponent, o.Workspace, appNamespace, false)
	labels["app.bex.co/build"] = o.Name
	podLabels := execution.PodLabels(o.Name, o.AppUID, buildComponent, o.Workspace, appNamespace, false)
	podLabels["app.bex.co/build"] = o.Name
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobName(o.Name, o.Revision),
			Namespace: o.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			PodFailurePolicy:        buildPodFailurePolicy(),
			ActiveDeadlineSeconds:   &deadline,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: annotations,
				},
				Spec: podSpec,
			},
		},
	}
}

func buildCloneContainer(o Options, image string) corev1.Container {
	ref := o.Ref
	if ref == "" {
		ref = "HEAD"
	}
	env := []corev1.EnvVar{
		{Name: "REPO", Value: o.Repo},
		{Name: "REF", Value: ref},
		{Name: "GIT_TERMINAL_PROMPT", Value: "0"},
	}
	if o.ExpectedCommit != "" {
		env = append(env, corev1.EnvVar{Name: "EXPECTED_COMMIT", Value: o.ExpectedCommit})
	}
	if o.CloneSecret != "" {
		env = append(env, corev1.EnvVar{
			Name: "GIT_AUTH_TOKEN",
			ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: o.CloneSecret},
				Key:                  "token",
			}},
		})
	}
	return corev1.Container{
		Name:    "clone",
		Image:   image,
		Command: []string{"sh", "-eu", "-c"},
		// The credential helper is execution.GitHubCredentialHelper — host-bound
		// to github.com; its SECURITY comment lives on the shared constant.
		// The fetch is the only step here that can fail on tenant input (a bad
		// repo URL, a ref that does not exist, or a token that cannot read the
		// repo), so it is the step that classifies. bex_run leaves $REPO/$REF as
		// environment lookups exactly as before — they are never spliced into the
		// script text.
		Args: []string{classifyPrelude + `cd /source
git init -q .
git remote add origin "$REPO"
if [ -n "${GIT_AUTH_TOKEN:-}" ]; then
  bex_run git -c ` + execution.GitHubCredentialHelper + ` fetch --depth 1 origin "$REF"
else
  bex_run git fetch --depth 1 origin "$REF"
fi
git checkout -q FETCH_HEAD
if [ -n "${EXPECTED_COMMIT:-}" ]; then
  actual_commit="$(git rev-parse HEAD)"
  [ "$actual_commit" = "$EXPECTED_COMMIT" ] || { echo "fetched commit does not match EXPECTED_COMMIT" >&2; exit 1; }
fi
rm -rf .git`},
		Env:             env,
		VolumeMounts:    []corev1.VolumeMount{{Name: "source", MountPath: sourceMount}},
		SecurityContext: restrictedContainerSecurityContext(),
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("50m"),
				corev1.ResourceMemory:           resource.MustParse("64Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse(buildEphemeralRequest),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("500m"),
				corev1.ResourceMemory:           resource.MustParse("256Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse(buildEphemeralLimit),
			},
		},
	}
}

// registryIsClusterLocal reports whether a registry host is the in-cluster or
// local-dev endpoint that legitimately speaks plain HTTP. Everything else is
// treated as a real registry whose TLS must be verified.
//
// The signal is the hostname rather than a scheme because BEX_REGISTRY carries
// no scheme (it is a host:port such as "zot.bex-registry.svc:5000").
func registryIsClusterLocal(registry string) bool {
	host := registry
	if h, _, err := net.SplitHostPort(registry); err == nil {
		host = h
	}
	host = strings.ToLower(strings.Trim(host, "[]"))
	switch host {
	case "", "localhost", "127.0.0.1", "::1":
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	for _, suffix := range []string{".svc", ".svc.cluster.local", ".cluster.local", ".local", ".internal"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	// A single-label host (no dot) cannot be a public DNS name; it is a
	// cluster-local Service short name or a dev alias.
	return !strings.Contains(host, ".")
}

// buildPodFailurePolicy encodes ADR060 D2: retry only what retrying can fix.
//
//   - DisruptionTarget => Ignore. A node drain, preemption or taint eviction is
//     the platform's doing, so it must not consume the tenant's attempt budget.
//     Ignore does not merely permit a retry, it refuses to count the attempt --
//     which is what makes build capacity reclaimable without failing deploys,
//     and what a future move to interruptible capacity depends on.
//   - ExitTenantError => FailJob. Deterministic tenant input; a second attempt
//     cannot succeed and would hold a 7 GiB node-exclusive slot to prove it.
//   - everything else counts against backoffLimit (2 attempts).
//
// OOM (137) is deliberately absent: the kubelet never sets DisruptionTarget for
// it, so it falls through to the backoff budget rather than being absorbed as a
// disruption -- capped like any other unclassified failure rather than retried
// freely.
func buildPodFailurePolicy() *batchv1.PodFailurePolicy {
	return &batchv1.PodFailurePolicy{
		Rules: []batchv1.PodFailurePolicyRule{
			{
				Action: batchv1.PodFailurePolicyActionIgnore,
				OnPodConditions: []batchv1.PodFailurePolicyOnPodConditionsPattern{
					{Type: corev1.DisruptionTarget, Status: corev1.ConditionTrue},
				},
			},
			{
				Action: batchv1.PodFailurePolicyActionFailJob,
				OnExitCodes: &batchv1.PodFailurePolicyOnExitCodesRequirement{
					Operator: batchv1.PodFailurePolicyOnExitCodesOpIn,
					Values:   []int32{ExitTenantError},
				},
			},
		},
	}
}

// captureFailureTail sets terminationMessagePolicy on every build phase so a
// failing container's output is retrievable from the pod status. Applied after
// the containers are assembled so it cannot be forgotten when a phase is added
// or reordered (signing moves the push container between slices, for example).
func captureFailureTail(spec *corev1.PodSpec) {
	for i := range spec.InitContainers {
		spec.InitContainers[i].TerminationMessagePolicy = corev1.TerminationMessageFallbackToLogsOnError
	}
	for i := range spec.Containers {
		spec.Containers[i].TerminationMessagePolicy = corev1.TerminationMessageFallbackToLogsOnError
	}
}

func restrictedContainerSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
}

// JobName is the deterministic per-revision build Job name. bex-api addresses
// this exact Job (cancel, built-image lookup), so the derivation lives in the
// contract module both sides import.
func JobName(name, revision string) string {
	return appv1alpha1.BuildJobName(name, revision)
}

// ActiveWorkspaceBuilds counts active (not Complete, not Failed) build Jobs in
// namespace that carry the given workspace label — used by the operator to
// enforce the per-workspace concurrent-build cap (w7/m9). Returns 0 for an
// empty workspace string (caller should skip the cap check in that case).
func ActiveWorkspaceBuilds(ctx context.Context, cl client.Client, namespace, workspace string) (int, error) {
	if workspace == "" {
		return 0, nil
	}
	return countActiveBuilds(ctx, cl, namespace, workspaceSelector(workspace))
}

// ActiveBuilds counts every active build in namespace regardless of workspace —
// the cluster-wide ceiling BEX_MAX_ACTIVE_BUILDS enforces (docs/ADR060 D6).
// After D1 made observation non-blocking, admission is the build plane's only
// concurrency control, so a per-workspace cap alone bounds each tenant's fan-out
// but not the estate's: N workspaces at their cap still saturate shared capacity.
func ActiveBuilds(ctx context.Context, cl client.Client, namespace string) (int, error) {
	return countActiveBuilds(ctx, cl, namespace, workspaceSelector(""))
}

// ActiveAppBuilds counts active (not Complete, not Failed) build Jobs + kpack
// Images that belong to this exact App (name + immutable UID). Once builds are
// observed non-blocking across many reconciles (ADR060 §D1), the per-workspace
// cap must gate only a NEW dispatch, never stall observation of a build this App
// already started — so the caller skips the cap when this returns non-zero.
// appUID scopes to the App's globally-unique UID (round-5 finding 5): the build
// namespace is shared, so a name-only selector would also count a same-named
// App's builds in ANOTHER workspace.
func ActiveAppBuilds(ctx context.Context, cl client.Client, namespace, name, appUID string) (int, error) {
	sel := client.MatchingLabels{"app.bex.co/build": name}
	if appUID != "" {
		sel[execution.LabelAppUID] = appUID
	}
	return countActiveBuilds(ctx, cl, namespace, sel)
}

// workspaceSelector selects the platform's build artifacts, narrowed to one
// workspace when workspace is non-empty and to the whole namespace when it is.
func workspaceSelector(workspace string) client.MatchingLabels {
	sel := client.MatchingLabels{execution.LabelComponent: buildComponent}
	if workspace != "" {
		sel[execution.LabelWorkspace] = workspace
	}
	return sel
}

// countActiveBuilds is the one definition of "how many builds are in flight":
// unfinished Jobs plus non-terminal kpack Images matching sel. Every cap, gauge
// and trim counts through it, so a scope can never mean two different things to
// the gate that admits a build and the correction that sheds it.
func countActiveBuilds(ctx context.Context, cl client.Client, namespace string, sel client.MatchingLabels) (int, error) {
	jobs, err := listBuildJobs(ctx, cl, namespace, sel)
	if err != nil {
		return 0, err
	}
	active := 0
	for i := range jobs {
		if !execution.JobFinished(&jobs[i]) {
			active++
		}
	}
	kpackActive, err := activeKpackImages(ctx, cl, namespace, sel)
	if err != nil {
		return 0, err
	}
	return active + kpackActive, nil
}

// listBuildJobs returns the build Jobs in namespace matching sel.
func listBuildJobs(ctx context.Context, cl client.Client, namespace string, sel client.MatchingLabels) ([]batchv1.Job, error) {
	var jobs batchv1.JobList
	if err := cl.List(ctx, &jobs, client.InNamespace(namespace), sel); err != nil {
		return nil, fmt.Errorf("list build jobs in %s: %w", namespace, err)
	}
	return jobs.Items, nil
}
