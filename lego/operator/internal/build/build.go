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
// Kubernetes Job — no docker daemon, no host tools, so it works on containerd
// app nodes (w1/m5). The Job runs rootless BuildKit, which fetches the git
// context itself (the dockerfile frontend's built-in git support) and pushes the
// result. The operator only dispatches the Job and waits for it; the heavy
// lifting happens on the cluster.
//
// Dockerfile builds (spec.builder auto|dockerfile) are supported here. Cloud
// Native Buildpacks (spec.builder buildpack) need a k8s-native builder (kpack) —
// not yet deployed — so they report a clear error rather than silently shelling
// out to `pack` (impossible on containerd). See docs/ADR004-deployment.md.
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
)

// defaultBuildkitImage is the rootless BuildKit image the build Job runs. Rootless
// so the Job needs no privileged container — only seccomp/AppArmor unconfined
// (set on the pod below), which containerd nodes allow.
const defaultBuildkitImage = "moby/buildkit:v0.13.2-rootless"

// buildTimeout bounds a single build Job's wall-clock before Build gives up
// waiting (the Job's own activeDeadlineSeconds matches, so a stuck build is
// reaped rather than lingering).
const buildTimeout = 20 * time.Minute

// pollInterval is how often Build re-reads the Job while waiting for it.
const pollInterval = 3 * time.Second

// Options configures a single in-cluster build.
type Options struct {
	Repo       string        // git URL to clone (BuildKit fetches it)
	Ref        string        // branch or commit; defaults to the repo's default branch
	RootDir    string        // subdirectory of Repo to build from (App.spec.rootDir); empty = repo root
	Name       string        // image repo name (the service name)
	Registry   string        // in-cluster registry host, e.g. zot.bex-registry.svc:5000
	Revision   string        // image tag, operator-supplied (e.g. "gen-7") — deterministic per revision
	Builder    string        // auto | dockerfile | buildpack (App.spec.builder)
	CNBBuilder string        // CNB builder image (buildpack strategy; not yet in-cluster)
	Namespace  string        // namespace the build Job runs in
	Client     client.Client // cluster client used to create + watch the Job
	// CloneSecret names a Secret (in Namespace, key "token") whose value is
	// passed to BuildKit as the GIT_AUTH_TOKEN build secret so a private Repo's
	// https git context authenticates (App.spec.cloneSecret). Empty = public
	// clone (unchanged). The operator only mounts it; bex-api mints the token.
	CloneSecret string
	// BuildkitImage overrides the rootless BuildKit image (tests / air-gapped).
	BuildkitImage string
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
		rev = "latest"
	}
	return fmt.Sprintf("%s/%s:%s", o.Registry, o.Name, rev)
}

// Build dispatches an in-cluster BuildKit Job that fetches Repo@Ref, builds its
// Dockerfile, and pushes <registry>/<name>:<revision> to the registry; it blocks
// until the Job succeeds (returning the image ref) or fails. Re-invocation for
// the same revision is idempotent: the Job is named per revision, so a retry
// adopts the existing Job instead of starting a second build.
func Build(ctx context.Context, o Options) (Result, error) {
	if o.Client == nil {
		return Result{}, fmt.Errorf("build: nil client (in-cluster builds require a cluster client)")
	}
	if o.Builder == BuilderBuildpack {
		return Result{}, fmt.Errorf("build: buildpack (CNB) builds are not yet supported in-cluster — add a Dockerfile or wait for kpack (w1/m5 follow-up)")
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

	// buildctl-daemonless.sh starts an ephemeral buildkitd and runs the build.
	args := []string{
		"build",
		"--frontend", "dockerfile.v0",
		"--opt", "context=" + ctxArg,
		"--output", "type=image,name=" + image + ",push=true,registry.insecure=true",
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

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobName(o.Name, o.Revision),
			Namespace: o.Namespace,
			Labels: map[string]string{
				"app.bex.co/build":     o.Name,
				"app.bex.co/component": "build",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			ActiveDeadlineSeconds:   &deadline,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app.bex.co/build": o.Name},
					// Rootless BuildKit needs an unconfined AppArmor profile; the
					// annotation form works on clusters older than k8s 1.30 too.
					Annotations: map[string]string{
						"container.apparmor.security.beta.kubernetes.io/buildkit": "unconfined",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name:    "buildkit",
						Image:   buildkitImage,
						Command: []string{"buildctl-daemonless.sh"},
						Args:    args,
						Env:     env,
						// Rootless BuildKit runs as UID 1000; unconfined seccomp
						// is required (privileged seccomp would block some syscalls
						// the userspace overlay FS needs). Resources are bounded so
						// a long-running or runaway build can't starve the node
						// (w7/m2/t003).
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
					}},
				},
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
		rev = "latest"
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

func ptr[T any](v T) *T { return &v }
