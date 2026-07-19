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
// heavy lifting stays inside the cluster. See docs/ADR004-deployment.md.
package build

import (
	"context"
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
const buildTimeout = 20 * time.Minute

// Build execution resources are shared by BuildKit and kpack. The memory
// request deliberately prevents two builders from co-locating on the baseline
// 8 GB tenant nodes; Cluster Autoscaler can then add one node per admitted
// build. The limit leaves headroom for kubelet and node daemons instead of
// offering a container more memory than those nodes can safely supply.
const (
	buildCPURequest    = "500m"
	buildMemoryRequest = "4Gi"
	buildCPULimit      = "4"
	buildMemoryLimit   = "6Gi"
)

// pollInterval is how often Build re-reads the Job while waiting for it.
const pollInterval = 3 * time.Second

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
// build name and adopt an existing Job/Image.
func Build(ctx context.Context, o Options) (Result, error) {
	if o.Client == nil {
		return Result{}, fmt.Errorf("build: nil client (in-cluster builds require a cluster client)")
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

	// Create the Job if it doesn't already exist (idempotent per revision).
	if err := o.Client.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
		return Result{}, fmt.Errorf("build: create job %s: %w", key.Name, err)
	}

	// Wait for the Job to finish, bounded by buildTimeout.
	wctx, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()
	for {
		var cur batchv1.Job
		if err := o.Client.Get(wctx, key, &cur); err != nil {
			return Result{}, fmt.Errorf("build: get job %s: %w", key.Name, err)
		}
		switch {
		case jobCondition(&cur, batchv1.JobComplete):
			return Result{Image: image}, nil
		case jobCondition(&cur, batchv1.JobFailed):
			return Result{}, fmt.Errorf("build: job %s failed: %s", key.Name, jobFailureMessage(&cur))
		}
		select {
		case <-wctx.Done():
			return Result{}, fmt.Errorf("build: job %s did not finish within %s", key.Name, buildTimeout)
		case <-time.After(pollInterval):
		}
	}
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

	// Private-base credentials may be present in this phase. Keep BuildKit's
	// default OCI process sandbox enabled; the Pod user namespace supplies the
	// namespaced capabilities it needs without exposing host-root capabilities.
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
		{Name: "source", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		{Name: "output", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
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
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
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
				corev1.ResourceCPU:    resource.MustParse(buildCPURequest),
				corev1.ResourceMemory: resource.MustParse(buildMemoryRequest),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(buildCPULimit),
				corev1.ResourceMemory: resource.MustParse(buildMemoryLimit),
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
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
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
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
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
	labels := execution.PodLabels(o.Name, "build", o.Workspace, appNamespace, false)
	labels["app.bex.co/build"] = o.Name
	podLabels := execution.PodLabels(o.Name, "build", o.Workspace, appNamespace, false)
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
		Args: []string{`cd /source
git init -q .
git remote add origin "$REPO"
if [ -n "${GIT_AUTH_TOKEN:-}" ]; then
  git -c credential.helper='!f() { echo "username=x-access-token"; echo "password=$GIT_AUTH_TOKEN"; }; f' fetch -q --depth 1 origin "$REF"
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
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
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
// service name (≤30) + revision, so re-reconciling the same revision adopts the
// existing Job and a new revision gets a fresh build.
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

// DeleteAppArtifacts deletes ALL build Jobs, their Pods, predeploy Jobs and
// their Pods, and kpack Images for the named app in namespace — called by the
// App finalizer to clean up cross-namespace artifacts that ownerRefs can't
// cascade (build/predeploy run in the build namespace, a different namespace
// from the App CR). Pods are deleted explicitly as a safety net: an orphan
// propagation policy or a failed garbage-collection pass can otherwise leave a
// completed build Pod behind after its Job disappears.
func DeleteAppArtifacts(ctx context.Context, name, namespace string, cl client.Client) error {
	// Build Jobs (labeled app.bex.co/build: <name>)
	var buildJobs batchv1.JobList
	if err := cl.List(ctx, &buildJobs, client.InNamespace(namespace),
		client.MatchingLabels{"app.bex.co/build": name}); err != nil {
		return fmt.Errorf("list build jobs for %s: %w", name, err)
	}
	for i := range buildJobs.Items {
		if err := cl.Delete(ctx, &buildJobs.Items[i], client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete build job %s: %w", buildJobs.Items[i].Name, err)
		}
	}
	// Predeploy Jobs (labeled app.bex.co/predeploy: <name>)
	var preJobs batchv1.JobList
	if err := cl.List(ctx, &preJobs, client.InNamespace(namespace),
		client.MatchingLabels{"app.bex.co/predeploy": name}); err != nil {
		return fmt.Errorf("list predeploy jobs for %s: %w", name, err)
	}
	for i := range preJobs.Items {
		if err := cl.Delete(ctx, &preJobs.Items[i], client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete predeploy job %s: %w", preJobs.Items[i].Name, err)
		}
	}
	for label, description := range map[string]string{
		"app.bex.co/build":     "build",
		"app.bex.co/predeploy": "predeploy",
	} {
		var pods corev1.PodList
		if err := cl.List(ctx, &pods, client.InNamespace(namespace),
			client.MatchingLabels{label: name}); err != nil {
			return fmt.Errorf("list %s pods for %s: %w", description, name, err)
		}
		for i := range pods.Items {
			if err := cl.Delete(ctx, &pods.Items[i]); err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("delete %s pod %s: %w", description, pods.Items[i].Name, err)
			}
		}
	}
	// kpack Images (if kpack is installed; tolerate "no matches for kind")
	images := newKpackImageList()
	if err := cl.List(ctx, images, client.InNamespace(namespace),
		client.MatchingLabels{"app.bex.co/build": name}); err != nil {
		if !apierrors.IsNotFound(err) && !strings.Contains(err.Error(), "no matches for kind") {
			return fmt.Errorf("list kpack images for %s: %w", name, err)
		}
	} else {
		for i := range images.Items {
			if err := cl.Delete(ctx, &images.Items[i]); err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("delete kpack image %s: %w", images.Items[i].GetName(), err)
			}
		}
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
