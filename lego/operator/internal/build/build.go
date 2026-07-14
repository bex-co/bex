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
// containerd app nodes (w1/m5, w6/m22). Dockerfile builds use a rootless
// BuildKit Job; Cloud Native Buildpack builds use a kpack Image CR. In both
// cases the operator only dispatches the workload and observes it while the
// heavy lifting stays inside the cluster. See docs/ADR004-deployment.md.
package build

import (
	"context"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Builder strategies (mirror App.spec.builder).
const (
	BuilderAuto       = "auto"
	BuilderDockerfile = "dockerfile"
	BuilderBuildpack  = "buildpack"
	BuilderNative     = "native"
	defaultRevision   = "latest"
)

// defaultBuildkitImage is the rootless BuildKit image the build Job runs. Rootless
// so the Job needs no privileged container — only seccomp/AppArmor unconfined
// (set on the pod below), which containerd nodes allow.
const defaultBuildkitImage = "moby/buildkit:v0.13.2-rootless"

// defaultSignImage is the cosign image the signing container runs when tenant
// image signing is enabled (w6/006). Pinned; bump deliberately.
const defaultSignImage = "gcr.io/projectsigstore/cosign:v2.4.1"

// buildTimeout bounds a single build Job's wall-clock before Build gives up
// waiting (the Job's own activeDeadlineSeconds matches, so a stuck build is
// reaped rather than lingering).
const buildTimeout = 20 * time.Minute

// pollInterval is how often Build re-reads the Job while waiting for it.
const pollInterval = 3 * time.Second

// dockerConfigMount is where the registry push credential (a docker-config
// Secret named by Options.PushSecret, key "config.json") is mounted so buildkitd
// (and cosign, when signing) authenticate the push against the auth-enabled Zot.
// DOCKER_CONFIG points here. It lives in the container's own filesystem — outside
// BuildKit's build-execution namespace — so tenant Dockerfile RUN steps cannot
// read it (w7/m8, docs/ADR022-tenant-isolation.md § Registry access control).
const dockerConfigMount = "/docker-config"

// mountRegistryCred attaches the docker-config volume (read-only) + DOCKER_CONFIG
// env to c so buildkitd/cosign authenticate against the in-cluster registry. It
// is a no-op when secret is empty, keeping the unset path byte-identical.
func mountRegistryCred(c *corev1.Container, secret string) {
	if secret == "" {
		return
	}
	c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{
		Name: "registry-cred", ReadOnly: true, MountPath: dockerConfigMount,
	})
	c.Env = append(c.Env, corev1.EnvVar{Name: "DOCKER_CONFIG", Value: dockerConfigMount})
}

// Options configures a single in-cluster build.
type Options struct {
	Repo           string // git URL to clone (BuildKit fetches it)
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
	// passed to BuildKit as the GIT_AUTH_TOKEN build secret so a private Repo's
	// https git context authenticates (App.spec.cloneSecret). Empty = public
	// clone (unchanged). The operator only mounts it; bex-api mints the token.
	CloneSecret string
	// BuildkitImage overrides the rootless BuildKit image (tests / air-gapped).
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
	// PushSecret names a Secret (in Namespace, key "config.json") whose docker
	// config authenticates the push against an auth-enabled registry (w7/m8). It
	// is mounted into the buildkit container's own filesystem at dockerConfigMount
	// with DOCKER_CONFIG pointing there — used by buildkitd to authenticate the
	// push, but OUTSIDE BuildKit's build-execution namespace, so a tenant
	// Dockerfile RUN step (or a declared --mount=type=secret) cannot read it
	// (docs/ADR022-tenant-isolation.md § Registry access control). When signing is
	// also enabled, cosign gets the same mount so its signature push authenticates.
	// Empty = unauthenticated push (the dev default; byte-identical).
	PushSecret string
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

// BuildJob constructs the rootless-BuildKit build Job for o, targeting image. It
// is a pure function (no cluster access) so the Job shape is unit-testable.
//
// BuildKit's dockerfile frontend fetches the git context itself
// (context=<repo>#<ref>) and pushes the built image; registry.insecure=true lets
// it push to the in-cluster Zot over plain HTTP (local/dev). The container runs
// rootless — only seccomp/AppArmor unconfined are needed, no privileged.
func BuildJob(o Options, image string) *batchv1.Job {
	ctxArg := gitContext(o.Repo, o.Ref, o.RootDir)
	buildkitImage := o.BuildkitImage
	if buildkitImage == "" {
		buildkitImage = defaultBuildkitImage
	}
	signImage := o.SignImage
	if signImage == "" {
		signImage = defaultSignImage
	}

	// buildctl-daemonless.sh starts an ephemeral buildkitd and runs the build.
	args := []string{
		"build",
		"--frontend", "dockerfile.v0",
		"--opt", "context=" + ctxArg,
		"--output", "type=image,name=" + image + ",push=true,registry.insecure=true",
	}
	if o.Builder == BuilderNative {
		args = append(args,
			"--local", "dockerfile=/native",
			"--opt", "filename=Dockerfile",
			"--secret", "id=render-env,src=/native/render-env",
		)
	} else if o.DockerfilePath != "" {
		args = append(args, "--opt", "filename="+o.DockerfilePath)
	}

	// Rootless buildkitd needs a writable state dir under the unprivileged
	// user's home.
	env := []corev1.EnvVar{{Name: "BUILDKITD_FLAGS", Value: "--oci-worker-no-process-sandbox"}}

	// Private-repo clone: hand BuildKit the token from the App's clone Secret as
	// its standard GIT_AUTH_TOKEN build secret (from env, so no volume). BuildKit's
	// git source uses it as x-access-token basic auth for the https git context.
	if o.CloneSecret != "" {
		args = append(args, "--secret", "id=GIT_AUTH_TOKEN,env=GIT_AUTH_TOKEN")
		env = append(env, corev1.EnvVar{
			Name: "GIT_AUTH_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: o.CloneSecret},
					Key:                  "token",
				},
			},
		})
	}

	deadline := int64(buildTimeout / time.Second)
	backoff := int32(1) // one build attempt; a failed build is a user error, not a flake to retry
	ttl := int32(3600)  // reap the finished Job after an hour

	// Registry push credential (w7/m8): when PushSecret is set, mount the
	// docker-config into the buildkit (and cosign) container's own filesystem so
	// buildkitd authenticates the push. Tenant RUN steps cannot read it — see
	// mountRegistryCred / docs/ADR022-tenant-isolation.md § Registry access control.
	var volumes []corev1.Volume
	if o.PushSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name:         "registry-cred",
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: o.PushSecret}},
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

	// buildkit container (rootless BuildKit builds + pushes the image).
	buildkit := corev1.Container{
		Name:    "buildkit",
		Image:   buildkitImage,
		Command: []string{"buildctl-daemonless.sh"},
		Args:    args,
		Env:     env,
		// Rootless BuildKit runs as UID 1000; unconfined seccomp is required
		// (privileged seccomp would block some syscalls the userspace overlay FS
		// needs). Resources are bounded so a long-running or runaway build can't
		// starve the node (w7/m2/t003).
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:  ptr(int64(1000)),
			RunAsGroup: ptr(int64(1000)),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeUnconfined,
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
		},
	}
	if o.Builder == BuilderNative {
		buildkit.VolumeMounts = append(buildkit.VolumeMounts, corev1.VolumeMount{Name: "native-build", MountPath: "/native"})
	}
	mountRegistryCred(&buildkit, o.PushSecret) // w7/m8: docker-config for the push

	podSpec := corev1.PodSpec{
		RestartPolicy: corev1.RestartPolicyNever,
		Containers:    []corev1.Container{buildkit},
	}
	if o.Builder == BuilderNative {
		podSpec.InitContainers = []corev1.Container{nativeBuildPreparer(o)}
		podSpec.SecurityContext = &corev1.PodSecurityContext{FSGroup: ptr(int64(65532))}
	}
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
			Args:    []string{"sign", "--yes", "--allow-insecure-registry", "--key", "/keys/cosign.key", image},
			Env: []corev1.EnvVar{{
				Name: "COSIGN_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: o.SignKeySecret},
					Key:                  "cosign.password",
				}},
			}},
			VolumeMounts: []corev1.VolumeMount{{Name: "cosign-key", ReadOnly: true, MountPath: "/keys"}},
			SecurityContext: &corev1.SecurityContext{
				RunAsNonRoot:             ptr(true),
				AllowPrivilegeEscalation: ptr(false),
				Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
			},
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
		mountRegistryCred(&sign, o.PushSecret) // w7/m8: cosign's signature push authenticates too
		podSpec.InitContainers = append(podSpec.InitContainers, buildkit)
		podSpec.Containers = []corev1.Container{sign}
		volumes = append(volumes, corev1.Volume{
			Name:         "cosign-key",
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: o.SignKeySecret}},
		})
		annotations["container.apparmor.security.beta.kubernetes.io/sign"] = "unconfined"
	}
	podSpec.Volumes = volumes // single assign (registry-cred, plus cosign-key when signing)

	labels := map[string]string{
		"app.bex.co/build":     o.Name,
		"app.bex.co/component": "build",
	}
	if o.Workspace != "" {
		labels["app.bex.co/workspace"] = o.Workspace
	}
	// Pod labels: carry build + component so the log-shipper can select and
	// attribute build pods. app-namespace overrides the shipper's namespace label
	// only when the build namespace differs from the App's namespace (w7/m28).
	podLabels := map[string]string{
		"app.bex.co/build":     o.Name,
		"app.bex.co/component": "build",
	}
	if o.AppNamespace != "" && o.AppNamespace != o.Namespace {
		podLabels["app.bex.co/app-namespace"] = o.AppNamespace
	}
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

// gitContext builds BuildKit's context= argument, forcing it to be recognized
// as a GIT context rather than an HTTP one. BuildKit fetches a plain http(s) URL
// as a single file/tarball; only a git-remote-looking URL (a .git suffix, or an
// ssh/git scheme) triggers a real clone. Without this, an https GitHub URL is
// downloaded as an HTML page and parsed as the Dockerfile ("<!DOCTYPE …") — so
// we ensure the .git suffix for http(s) repos (github/gitlab/gitea/bitbucket all
// accept it); ssh/git-scheme and local paths are left untouched.
//
// rootDir (App.spec.rootDir, monorepo support) appends BuildKit's ":<subdir>"
// git-context suffix, scoping the build to that subdirectory's Dockerfile. It
// follows the ref after a "#", e.g. "repo.git#main:services/api"; if ref is
// empty but rootDir is set, "#" alone still introduces the ":<subdir>" so
// BuildKit resolves the default branch scoped to that subdirectory.
func gitContext(repo, ref, rootDir string) string {
	ctx := repo
	if (strings.HasPrefix(ctx, "https://") || strings.HasPrefix(ctx, "http://")) && !strings.HasSuffix(ctx, ".git") {
		ctx += ".git"
	}
	if ref != "" || rootDir != "" {
		ctx += "#" + ref
	}
	if rootDir != "" {
		ctx += ":" + rootDir
	}
	return ctx
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
		if err := cl.Delete(ctx, j); err != nil && !apierrors.IsNotFound(err) {
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
