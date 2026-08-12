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
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Builder strategies (mirror App.spec.builder).
const (
	BuilderAuto       = "auto"
	BuilderDockerfile = "dockerfile"
	BuilderBuildpack  = "buildpack"
	BuilderNative     = "native"
	defaultRevision   = "latest"
)

// Platform-plane images are pinned and bumped deliberately. BuildKit runs as
// root inside a Kubernetes Pod user namespace: it has the namespaced
// capabilities needed for its OCI worker while remaining unprivileged on the
// node, and its default process sandbox keeps Dockerfile RUN processes out of
// the daemon's PID namespace.
const (
	defaultBuildkitImage = "moby/buildkit:v0.30.0"
	defaultGitImage      = "alpine/git:v2.43.0"
	defaultPushImage     = "quay.io/skopeo/stable@sha256:c7d3c512612f52805023cd38351081dad7e2729fc13d14b701e47c7c8bdd6615" // v1.22.2 multi-arch manifest
)

// defaultSignImage is the cosign image the signing container runs when tenant
// image signing is enabled (w6/006). Pinned; bump deliberately.
const defaultSignImage = "gcr.io/projectsigstore/cosign:v2.4.1"

// buildTimeout bounds a single build Job's wall-clock before Build gives up
// waiting (the Job's own activeDeadlineSeconds matches, so a stuck build is
// reaped rather than lingering).
const buildTimeout = 30 * time.Minute

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

// pollInterval is how often Build re-reads the Job while waiting for it.
const pollInterval = 3 * time.Second

// ErrAppDeleting terminates a synchronous build wait when its owning App has
// entered deletion (or is already gone). The build artifact is intentionally
// left behind: the App finalizer inventories and removes it with the immutable
// App UID, while the controller is freed to observe deletion immediately
// instead of holding the reconcile worker for the full build timeout.
var ErrAppDeleting = errors.New("build: owning App is deleting")

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
	// OnWaiting, when set, is called on each poll of the wait loop with whether
	// the build is still QUEUED — dispatched but with no pod yet placed on a
	// node — plus the scheduler's own explanation.
	//
	// It exists because "we dispatched a Job" and "the build is running" are
	// different facts, and only the second should start a build clock. A build
	// requests 2 CPU + 7Gi and is therefore node-exclusive by design, so it can
	// sit Pending for many minutes waiting for capacity. Charging that wait to
	// the build's budget is what let a healthy build be reported failed
	// (2026-08-11: 22 minutes Pending consumed most of the control plane's
	// 35-minute build gate, which then closed the deploy while the build was
	// still running and about to succeed).
	//
	// Mechanism only: this package reports the distinction and does not know
	// what a caller does with it. The App controller maps it onto the
	// BuildQueued/Building phase reasons the control plane already understands.
	OnWaiting func(queued bool, reason string)
}

// Result is a successful build.
type Result struct {
	Image string // <registry>/<name>:<revision>
}

// ImageRef is the deterministic image reference a build produces — the operator
// knows it before the build runs (registry + name + revision), so it can gate
// rebuilds on the revision changing and set status.image without parsing Job
// output.
func (o Options) ImageRef() string {
	rev := o.Revision
	if rev == "" {
		rev = defaultRevision
	}
	return fmt.Sprintf("%s/%s:%s", o.Registry, o.Name, rev)
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
	return fmt.Sprintf("%s/%s:%s", registry, o.Name, rev)
}

// Build dispatches the selected in-cluster builder and blocks until it returns
// an immutable image reference or a useful failure. Re-invocation for one App
// generation is idempotent because both mechanisms use the same deterministic
// build name and reuse an existing Job/Image only after exact UID validation.
func Build(ctx context.Context, o Options) (Result, error) {
	if o.Client == nil {
		return Result{}, fmt.Errorf("build: nil client (in-cluster builds require a cluster client)")
	}
	if o.AppUID == "" {
		return Result{}, fmt.Errorf("build: empty App UID")
	}
	if o.Builder == BuilderBuildpack {
		return buildpack(ctx, o)
	}
	if o.Builder == BuilderNative {
		if err := validateNativeOptions(o); err != nil {
			return Result{}, err
		}
	}

	image := o.ImageRef()
	job := BuildJob(o, image)
	key := client.ObjectKeyFromObject(job)
	identity := execution.ArtifactIdentity{Name: o.Name, UID: o.AppUID, Workspace: o.Workspace, Namespace: o.AppNamespace}

	// Create the Job if it doesn't already exist (idempotent per revision).
	if err := o.Client.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
		return Result{}, fmt.Errorf("build: create job %s: %w", key.Name, err)
	}
	if deleting, err := appDeleting(ctx, o); err != nil {
		return Result{}, err
	} else if deleting {
		return Result{}, ErrAppDeleting
	}

	// Wait for the Job to finish, bounded by buildTimeout.
	wctx, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()
	for {
		if deleting, err := appDeleting(wctx, o); err != nil {
			return Result{}, err
		} else if deleting {
			return Result{}, ErrAppDeleting
		}
		var cur batchv1.Job
		if err := o.Client.Get(wctx, key, &cur); err != nil {
			return Result{}, fmt.Errorf("build: get job %s: %w", key.Name, err)
		}
		if err := identity.CheckOwner(&cur); err != nil {
			return Result{}, fmt.Errorf("build: check job owner %s: %w", key.Name, err)
		}
		switch {
		case jobCondition(&cur, batchv1.JobComplete):
			return Result{Image: image}, nil
		case jobCondition(&cur, batchv1.JobFailed):
			return Result{}, fmt.Errorf("build: job %s failed: %s", key.Name, jobFailureMessage(&cur))
		}
		if o.OnWaiting != nil {
			queued, reason := buildQueued(wctx, o, key.Name)
			o.OnWaiting(queued, reason)
		}
		select {
		case <-wctx.Done():
			return Result{}, fmt.Errorf("build: job %s did not finish within %s", key.Name, buildTimeout)
		case <-time.After(pollInterval):
		}
	}
}

// buildQueued reports whether the Job's build is still waiting for capacity
// rather than doing work, and the scheduler's reason when it is.
//
// "Placed on a node" is the dividing line, NOT pod phase: the build runs in
// initContainers (clone, then BuildKit), so a pod actively compiling reports
// phase Pending for most of its life. Reading phase would classify a healthy
// mid-build pod as queued. A pod with spec.nodeName set is consuming capacity
// and making progress; one the scheduler could not place is not.
//
// Errors and the no-pods-yet window are reported as NOT queued: this only
// gates a status nicety, and guessing "queued" on a failed List would stall the
// caller's clock on evidence it does not have.
func buildQueued(ctx context.Context, o Options, jobName string) (bool, string) {
	var pods corev1.PodList
	if err := o.Client.List(ctx, &pods,
		client.InNamespace(o.Namespace),
		client.MatchingLabels{"job-name": jobName},
	); err != nil || len(pods.Items) == 0 {
		return false, ""
	}
	reason := ""
	for idx := range pods.Items {
		pod := &pods.Items[idx]
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue // a spent attempt says nothing about the current one
		}
		if pod.Spec.NodeName != "" {
			return false, "" // scheduled ⇒ really building
		}
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse && cond.Message != "" {
				reason = cond.Message
			}
		}
	}
	if reason == "" {
		reason = "waiting for a node with capacity for the build"
	}
	return true, reason
}

// appDeleting reads the uncached client supplied to the build plane so a
// deletion update can interrupt the synchronous polling loop even while the
// controller-runtime reconcile that dispatched the build is still running.
// Empty AppNamespace preserves the package's standalone/test behavior.
func appDeleting(ctx context.Context, o Options) (bool, error) {
	if o.AppNamespace == "" {
		return false, nil
	}
	var app appv1alpha1.App
	if err := o.Client.Get(ctx, client.ObjectKey{Namespace: o.AppNamespace, Name: o.Name}, &app); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("build: check owning App deletion: %w", err)
	}
	return !app.DeletionTimestamp.IsZero(), nil
}

// BuildJob constructs the credential-separated source build Job for o. Its
// phases are strictly serial: clone -> build OCI archive -> push -> optional
// sign. Kubernetes gives each container a separate root filesystem and PID
// namespace, so tenant Dockerfile processes never coexist with clone, platform
// push, or signing credentials. Only an explicitly configured private-base
// pull credential enters BuildKit, and it grants no platform-repository write.
func BuildJob(o Options, image string) *batchv1.Job {
	buildkitImage := o.BuildkitImage
	if buildkitImage == "" {
		buildkitImage = defaultBuildkitImage
	}
	gitImage := o.GitImage
	if gitImage == "" {
		gitImage = defaultGitImage
	}
	pushImage := o.PushImage
	if pushImage == "" {
		pushImage = defaultPushImage
	}
	signImage := o.SignImage
	if signImage == "" {
		signImage = defaultSignImage
	}

	contextDir := sourceMount
	if root := strings.TrimPrefix(path.Clean("/"+o.RootDir), "/"); root != "." && root != "" {
		contextDir = path.Join(sourceMount, root)
	}
	dockerfileDir := contextDir
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

	deadline := int64(buildTimeout / time.Second)
	backoff := int32(1) // one build attempt; a failed build is a user error, not a flake to retry
	ttl := int32(3600)  // reap the finished Job after an hour

	volumes := []corev1.Volume{
		{Name: "source", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: mustSizeLimit(emptyDirSizeLimit)}}},
		{Name: "output", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: mustSizeLimit(emptyDirSizeLimit)}}},
	}
	if o.PushSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name:         "push-registry-cred",
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: o.PushSecret}},
		})
	}
	if o.PullSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name:         "pull-registry-cred",
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: o.PullSecret}},
		})
	}
	if o.Builder == BuilderNative {
		volumes = append(volumes, corev1.Volume{
			Name:         "native-build",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: mustSizeLimit(emptyDirSizeLimit)}},
		})
		if o.RuntimeEnvSecret != "" {
			volumes = append(volumes, corev1.Volume{
				Name:         "runtime-env",
				VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: o.RuntimeEnvSecret}},
			})
		}
	}

	clone := buildCloneContainer(o, gitImage)

	// BuildKit receives the checked-out tree and writes an OCI archive. It has no
	// clone, platform push, or signing credential.
	buildkit := corev1.Container{
		Name:    "buildkit",
		Image:   buildkitImage,
		Command: []string{"buildctl-daemonless.sh"},
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
			RunAsUser:                ptr(int64(0)),
			RunAsGroup:               ptr(int64(0)),
			AllowPrivilegeEscalation: ptr(true),
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

	pushArgs := []string{"copy", "--dest-tls-verify=false"}
	if o.PushSecret != "" {
		pushArgs = append(pushArgs, "--authfile", dockerConfigMount+"/config.json")
	}
	pushArgs = append(pushArgs, "oci-archive:"+outputMount+"/image.tar", "docker://"+image)
	pusher := corev1.Container{
		Name:    "push",
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
	podSpec.SecurityContext.FSGroup = ptr(int64(0))
	annotations := map[string]string{
		"container.apparmor.security.beta.kubernetes.io/buildkit": "unconfined",
		// The tenant-burst node pool scales from zero for exactly this Job and its
		// aggressive scale-down-unneeded-time (5m, infra/clusterapi/autoscaler-values.yaml)
		// is otherwise free to reclaim the node mid-build: BackoffLimit is 1, so a
		// killed buildkit container fails the whole build outright rather than
		// retrying (observed in production — a build pod evicted by cluster-autoscaler
		// every few minutes until BackoffLimitExceeded, with no application-code fault).
		"cluster-autoscaler.kubernetes.io/safe-to-evict": "false",
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
	podSpec.Volumes = volumes

	appNamespace := ""
	if o.AppNamespace != "" && o.AppNamespace != o.Namespace {
		appNamespace = o.AppNamespace
	}
	labels := execution.PodLabels(o.Name, o.AppUID, "build", o.Workspace, appNamespace, false)
	labels["app.bex.co/build"] = o.Name
	podLabels := execution.PodLabels(o.Name, o.AppUID, "build", o.Workspace, appNamespace, false)
	podLabels["app.bex.co/build"] = o.Name
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobName(o.Name, o.Revision),
			Namespace: o.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
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
		// SECURITY: the credential helper is host-bound — it answers only when git
		// asks for github.com credentials (the "host=" line of git's credential
		// protocol). bex-api only mints a GIT_AUTH_TOKEN for a structurally
		// verified github.com origin, so this is defense in depth: even if a
		// crafted REPO caused git to connect elsewhere, the helper returns nothing
		// and the token never leaves for a non-GitHub host.
		Args: []string{`cd /source
git init -q .
git remote add origin "$REPO"
if [ -n "${GIT_AUTH_TOKEN:-}" ]; then
  git -c credential.helper='!f() { [ "$1" = get ] || exit 0; h=; while IFS= read -r l; do [ -z "$l" ] && break; case "$l" in host=*) h=${l#host=};; esac; done; [ "$h" = github.com ] || exit 0; echo "username=x-access-token"; echo "password=$GIT_AUTH_TOKEN"; }; f' fetch -q --depth 1 origin "$REF"
else
  git fetch -q --depth 1 origin "$REF"
fi
git checkout -q FETCH_HEAD
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

func restrictedContainerSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr(false),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
}

// JobName is the deterministic per-revision Job name (DNS-1123, ≤63 chars):
// service name (≤30) + revision, so re-reconciling the same revision reuses the
// exact-lifetime Job and a new revision gets a fresh build.
func JobName(name, revision string) string {
	rev := revision
	if rev == "" {
		rev = defaultRevision
	}
	n := "bld-" + name + "-" + rev
	if len(n) > 63 {
		n = n[:63]
	}
	return strings.ToLower(n)
}

// jobCondition reports whether the Job carries condition t with status True.
func jobCondition(j *batchv1.Job, t batchv1.JobConditionType) bool {
	for _, c := range j.Status.Conditions {
		if c.Type == t && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// jobFailureMessage extracts the JobFailed condition's reason/message for the
// error surfaced to the App's status.
func jobFailureMessage(j *batchv1.Job) string {
	for _, c := range j.Status.Conditions {
		if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
			if c.Message != "" {
				return c.Reason + ": " + c.Message
			}
			return c.Reason
		}
	}
	return "unknown build failure"
}

// CancelActiveBuilds deletes all active (not Complete, not Failed) build Jobs
// for the named service in namespace. This implements the newest-wins policy
// (w7/m9): before dispatching a fresh build for a new revision, the operator
// cancels any superseded in-progress build so push-spam never runs more than
// one build at a time per App. Not-found on delete is tolerated (concurrent GC).
func CancelActiveBuilds(ctx context.Context, name, namespace string, cl client.Client) error {
	var jobs batchv1.JobList
	if err := cl.List(ctx, &jobs,
		client.InNamespace(namespace),
		client.MatchingLabels{"app.bex.co/build": name}); err != nil {
		return fmt.Errorf("list builds for %s: %w", name, err)
	}
	for i := range jobs.Items {
		j := &jobs.Items[i]
		if jobCondition(j, batchv1.JobComplete) || jobCondition(j, batchv1.JobFailed) {
			continue
		}
		if err := cl.Delete(ctx, j, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("cancel build %s: %w", j.Name, err)
		}
	}
	if err := cancelActiveKpackImages(ctx, name, namespace, cl); err != nil {
		return err
	}
	return nil
}

// ActiveWorkspaceBuilds counts active (not Complete, not Failed) build Jobs in
// namespace that carry the given workspace label — used by the operator to
// enforce the per-workspace concurrent-build cap (w7/m9). Returns 0 for an
// empty workspace string (caller should skip the cap check in that case).
func ActiveWorkspaceBuilds(ctx context.Context, workspace, namespace string, cl client.Client) (int, error) {
	if workspace == "" {
		return 0, nil
	}
	var jobs batchv1.JobList
	if err := cl.List(ctx, &jobs,
		client.InNamespace(namespace),
		client.MatchingLabels{
			"app.bex.co/component": "build",
			"app.bex.co/workspace": workspace,
		}); err != nil {
		return 0, fmt.Errorf("list workspace builds: %w", err)
	}
	active := 0
	for i := range jobs.Items {
		if !jobCondition(&jobs.Items[i], batchv1.JobComplete) && !jobCondition(&jobs.Items[i], batchv1.JobFailed) {
			active++
		}
	}
	kpackActive, err := activeWorkspaceKpackImages(ctx, workspace, namespace, cl)
	if err != nil {
		return 0, err
	}
	return active + kpackActive, nil
}

func ptr[T any](v T) *T { return &v }
