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

// Package publish is the static-site publish plane: take the OCI image a
// static_site build produced (build plane, in-cluster BuildKit) and copy its
// PublishPath directory to the object-store origin, keyed by
// <bucket>/<appID>/<revision>/. It is the "different output sink" a static_site
// uses instead of running the image as a Deployment (w1/m21): the shared
// static-server then serves that prefix. Like the build plane it dispatches an
// in-cluster Job and waits — no docker daemon, no host tools, no S3 client in
// the manager (the aws-cli container holds the credentials via the S3 Secret,
// exactly the etcd/OpenBao/CNPG backup pattern, docs/ADR029-static-sites.md).
package publish

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/identity"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// DefaultAWSCLIImage is the S3 uploader image. Pinned < 2.23 because newer AWS
// CLIs send CRC64 checksums that S3-compatibles (Wasabi/Hetzner) reject — the
// same pin the etcd/OpenBao backup CronJobs use (docs/ADR011-etcd-backup-restore.md).
const DefaultAWSCLIImage = "amazon/aws-cli:2.22.35@sha256:6977c83ae3dc99f28fcf8276b9ea5eec33833cd5be40574b34112e98113ec7a2"

// DefaultGitImage is the clone image the direct (no-Dockerfile) publish path
// uses (w9/010) — pinned like every other platform-plane image.
const DefaultGitImage = "alpine/git:v2.43.0@sha256:76fdb7210689fc26c6ff101c4adacf9d12d3d25a919a7d8ff42ebaef5bedba65"

// publishTimeout bounds a single publish Job's wall-clock; the Job's own
// activeDeadlineSeconds matches so a stuck upload is reaped, not left lingering.
const publishTimeout = 10 * time.Minute

// ObserveInterval is how often the App reconciler re-reads an in-flight
// publish Job: Ensure never blocks (round-13 #6 — two slow publishes must not
// occupy every App reconciler worker for the Job's whole lifetime), so the
// controller returns RequeueAfter this and re-observes, exactly the build
// plane's ADR060 §D1 shape.
const ObserveInterval = 3 * time.Second

// outVolume is the emptyDir the extract init-container copies PublishPath into
// and the upload container syncs from.
const outVolume = "out"
const outMount = "/out"

// Store is the operator's static-site object-store target: the S3-compatible
// bucket + endpoint (deployment-time config, BEX_STATIC_S3_ENDPOINT/BUCKET) and
// the name of a k8s Secret holding AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY
// (BEX_STATIC_PUBLISH_S3_SECRET). All three unset => static_site is unavailable,
// the way unset BEX_DB_BACKUP_* disables Postgres backups. The publish identity
// is write/delete scoped to the dedicated static bucket and is deliberately
// distinct from the static-server's read-only identity and every backup/tfstate
// credential (docs/ADR029-static-sites.md).
type Store struct {
	// Bucket is the object-store bucket static content is published to (a bucket
	// dedicated to static sites, e.g. "bex-static").
	Bucket string
	// Endpoint is the S3-compatible endpoint URL, e.g.
	// "https://s3.eu-central-2.wasabisys.com".
	Endpoint string
	// Region is the S3 region (optional; some S3-compatibles need it). Read from
	// the Secret's AWS_DEFAULT_REGION when set there instead.
	Region string
	// Secret is a Secret (in the publish Job's namespace) with AWS_ACCESS_KEY_ID
	// and AWS_SECRET_ACCESS_KEY (optionally AWS_DEFAULT_REGION) keys.
	Secret string
}

// Configured reports whether every field needed to publish is set. Unset => the
// static_site service type is unavailable (the reconciler fails such Apps with a
// clear message rather than half-reconciling).
func (s Store) Configured() bool {
	return s.Bucket != "" && s.Endpoint != "" && s.Secret != ""
}

// Options configures a single static-site publish. Exactly one source is set:
// Image (extract the built site from an OCI image — the build path) or Repo
// (direct publish, w9/010 — clone the repo and publish RootDir/PublishPath
// as-is, the way Render handles a static site with no build step).
type Options struct {
	Image       string // OCI image the build produced (holds the built site)
	PublishPath string // dir within the image (or repo) to serve (App.spec.publishPath)
	// Direct-publish source (used only when Image is empty):
	Repo    string // git URL to clone
	Ref     string // branch or commit; empty = the remote's default branch (HEAD)
	RootDir string // subdirectory of Repo the site lives under (App.spec.rootDir)
	// CloneSecret names a Secret (in Namespace, key "token") whose value
	// authenticates a private Repo clone — the same contract as
	// build.Options.CloneSecret; empty = public clone.
	CloneSecret string
	// GitImage overrides the clone image (tests / air-gapped).
	GitImage  string
	AppID     string        // the service name — used with Workspace for the object-key prefix (docs/ADR074)
	AppUID    string        // immutable App UID; prevents same-name recreation from reusing stale artifacts
	Revision  string        // e.g. "rev-7" — the last key segment (immutable per revision)
	Store     Store         // object-store target (bucket/endpoint/secret)
	Namespace string        // namespace the publish Job runs in
	Client    client.Client // cluster client used to create + watch the Job
	// Workspace/AppNamespace carry the tenant identity into the shared execution
	// namespace for logs, network policy, and admission selection.
	Workspace    string
	AppNamespace string
	// VerifyImage selects image-extract Pods for fail-closed signature admission.
	// It is set only when tenant signing is enabled; direct-clone publish does not
	// execute a tenant image and leaves it false.
	VerifyImage bool
	// AWSCLIImage overrides the uploader image (tests / air-gapped).
	AWSCLIImage string
	// PullSecret names an imagePullSecret (in Namespace) that authenticates the
	// extract initContainer's pull of o.Image from an auth-enabled registry
	// (w7/m8). The publish Job runs in the build namespace, so this Secret must
	// exist there (scripts/registry-secrets.sh creates bex-registry-pull in both
	// the apps and build namespaces). Empty => unauthenticated pull (dev default).
	PullSecret string
}

// Prefix is the object-key prefix a publish writes under. Unlabeled Apps keep
// "<appID>/<revision>/"; labeled Apps use "<workspace>/<appID>/<revision>/"
// (docs/ADR074). The static-server reads the same prefix from App status.
func (o Options) Prefix() string {
	return identity.ForApp(o.AppID, o.Workspace).StaticPrefix(o.Revision)
}

// destURI is the s3:// sync target for this publish.
func (o Options) destURI() string {
	return "s3://" + o.Store.Bucket + "/" + o.Prefix()
}

// srcDir is the checkout-relative directory a direct publish copies:
// RootDir/PublishPath, cleaned. "." when both are empty/".".
func (o Options) srcDir() string {
	return path.Join(o.RootDir, o.PublishPath)
}

// imagePullSecrets returns the imagePullSecrets the publish pod carries so its
// extract initContainer authenticates the pull of o.Image from an auth-enabled
// registry (w7/m8). o.Image is always the in-cluster build output, so there is
// no registry-hosted check (unlike app_controller's imagePullSecrets) — it is
// attached whenever a pull Secret is configured, nil (omitted) otherwise.
func imagePullSecrets(pullSecret string) []corev1.LocalObjectReference {
	if pullSecret == "" {
		return nil
	}
	return []corev1.LocalObjectReference{{Name: pullSecret}}
}

// Phase is one publish Job's observed state — the build plane's Observation
// shape (ADR060 §D1) applied to the publish plane.
type Phase string

const (
	// PhasePublishing: the Job exists and has not reached a terminal condition;
	// the reconciler requeues after ObserveInterval instead of blocking.
	PhasePublishing Phase = "publishing"
	// PhaseSucceeded: the Job completed — the revision is live at the origin.
	PhaseSucceeded Phase = "succeeded"
	// PhaseFailed: the Job failed (or its deadline reaped it). Message carries
	// the surfaced reason.
	PhaseFailed Phase = "failed"
)

// Observation is Ensure's non-blocking result: what the publish Job currently
// looks like, never a wait.
type Observation struct {
	Phase   Phase
	Message string
}

// Ensure dispatches the publish Job if absent and returns its CURRENT state
// without blocking (round-13 #6): the pre-round-13 helper polled inside the
// reconcile call for up to publishTimeout, so two slow tenant publishes
// occupied both production App workers and stalled unrelated reconciles. The
// Job's own activeDeadlineSeconds enforces the wall-clock bound the blocking
// loop used to own. Re-invocation for the same revision is idempotent: the Job
// is named per revision, so a retry reuses it after exact UID validation.
func Ensure(ctx context.Context, o Options) (Observation, error) {
	if err := validate(o); err != nil {
		return Observation{}, err
	}

	owned := execution.ArtifactIdentity{Name: o.AppID, UID: o.AppUID, Workspace: o.Workspace, Namespace: o.AppNamespace}
	cur, _, err := execution.EnsureOwnedJob(ctx, o.Client, PublishJob(o), owned, "publish")
	if err != nil {
		return Observation{}, err
	}
	switch {
	case execution.JobHasCondition(cur, batchv1.JobComplete):
		return Observation{Phase: PhaseSucceeded}, nil
	case execution.JobHasCondition(cur, batchv1.JobFailed):
		return Observation{Phase: PhaseFailed, Message: execution.JobFailureMessage(cur, "unknown publish failure")}, nil
	}
	return Observation{Phase: PhasePublishing}, nil
}

// validate holds Ensure's input rules (formerly Publish's prologue).
func validate(o Options) error {
	if o.Client == nil {
		return fmt.Errorf("publish: nil client (in-cluster publish requires a cluster client)")
	}
	if o.AppUID == "" {
		return fmt.Errorf("publish: empty App UID")
	}
	if o.PublishPath == "" {
		return fmt.Errorf("publish: empty publishPath")
	}
	if (o.Image == "") == (o.Repo == "") {
		return fmt.Errorf("publish: exactly one of Image (extract) or Repo (direct publish) must be set")
	}
	if o.Image == "" {
		// The clone container cds to /work/<rootDir>/<publishPath>; refuse a
		// path that would escape the checkout.
		if src := o.srcDir(); src == ".." || strings.HasPrefix(src, "../") || path.IsAbs(src) {
			return fmt.Errorf("publish: rootDir/publishPath %q escapes the repository checkout", src)
		}
	}
	if !o.Store.Configured() {
		return fmt.Errorf("publish: object store not configured (set BEX_STATIC_S3_ENDPOINT/BUCKET/SECRET)")
	}
	return nil
}

// PublishJob constructs the extract+upload Job for o. It is a pure function (no
// cluster access) so the Job shape is unit-testable.
//
// initContainer "extract" runs the built image and copies PublishPath's contents
// into a shared emptyDir (PublishPath is passed via env, never interpolated into
// the shell string, so a hostile spec.publishPath can't inject). Container
// "upload" runs aws-cli, syncing that emptyDir to the object store under
// <appID>/<revision>/ with --delete (the revision prefix is exclusively this
// publish's, so --delete only prunes a re-publish of the same revision). The S3
// credentials come from the Store.Secret via envFrom; the endpoint/bucket are
// explicit args from operator config.
func PublishJob(o Options) *batchv1.Job {
	awsImage := o.AWSCLIImage
	if awsImage == "" {
		awsImage = DefaultAWSCLIImage
	}

	deadline := int64(publishTimeout / time.Second)
	backoff := int32(1) // one attempt; a failed publish is a user/config error, not a flake
	ttl := int32(3600)  // reap the finished Job after an hour

	uploadEnv := []corev1.EnvVar{}
	if o.Store.Region != "" {
		uploadEnv = append(uploadEnv, corev1.EnvVar{Name: "AWS_DEFAULT_REGION", Value: o.Store.Region})
	}

	// First step: stage the site's files into the shared emptyDir — from the
	// built image (extract) or straight from a git checkout (direct publish).
	stage := extractContainer(o)
	var pullSecrets []corev1.LocalObjectReference
	if o.Image == "" {
		stage = cloneContainer(o)
	} else {
		// The extract initContainer pulls o.Image (the just-built Zot image) —
		// authenticate it under an auth-enabled registry (w7/m8). Clone mode
		// pulls only public platform images, so it carries no pull secret.
		pullSecrets = imagePullSecrets(o.PullSecret)
	}

	appNamespace := ""
	if o.AppNamespace != "" && o.AppNamespace != o.Namespace {
		appNamespace = o.AppNamespace
	}
	verifyImage := o.VerifyImage && o.Image != ""
	labels := execution.PodLabels(o.AppID, o.AppUID, "publish", o.Workspace, appNamespace, verifyImage)
	labels["app.bex.co/publish"] = o.AppID
	podLabels := execution.PodLabels(o.AppID, o.AppUID, "publish", o.Workspace, appNamespace, verifyImage)
	podLabels["app.bex.co/publish"] = o.AppID
	podSpec := corev1.PodSpec{
		RestartPolicy:    corev1.RestartPolicyNever,
		ImagePullSecrets: pullSecrets,
		Volumes: []corev1.Volume{{
			Name:         outVolume,
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{SizeLimit: mustSizeLimit(publishEmptyDirLimit)}},
		}},
		InitContainers: []corev1.Container{stage},
		Containers: []corev1.Container{{
			Name:  "upload",
			Image: awsImage,
			// --no-follow-symlinks prevents tenant-controlled output from
			// turning /proc or another uploader-local path into an S3 object.
			Args: []string{
				"s3", "sync", outMount, o.destURI(),
				"--endpoint-url", o.Store.Endpoint,
				"--no-follow-symlinks",
				"--delete",
			},
			Env: uploadEnv,
			EnvFrom: []corev1.EnvFromSource{{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: o.Store.Secret},
				},
			}},
			VolumeMounts: []corev1.VolumeMount{
				{Name: outVolume, MountPath: outMount, ReadOnly: true},
			},
			Resources: modestResources(),
		}},
	}
	execution.HardenPod(&podSpec)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobName(o.AppID, o.Revision),
			Namespace: o.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			ActiveDeadlineSeconds:   &deadline,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: podSpec,
			},
		},
	}
}

// extractContainer stages the site from the built OCI image: run the image and
// copy PublishPath's contents into the shared emptyDir. PublishPath is passed
// via env (not the command string) so a hostile spec.publishPath can't inject
// shell. cp -a preserves the tree; the trailing /. copies contents, not the
// dir itself.
func extractContainer(o Options) corev1.Container {
	return corev1.Container{
		Name:    "extract",
		Image:   o.Image,
		Command: []string{"sh", "-c", `set -e; cp -a "$PUBLISH_PATH"/. ` + outMount + `/`},
		Env:     []corev1.EnvVar{{Name: "PUBLISH_PATH", Value: o.PublishPath}},
		VolumeMounts: []corev1.VolumeMount{
			{Name: outVolume, MountPath: outMount},
		},
		Resources: modestResources(),
	}
}

// cloneContainer stages the site straight from the repository — the direct,
// no-Dockerfile publish path (w9/010): shallow-fetch Repo@Ref, then copy
// RootDir/PublishPath's contents into the shared emptyDir. All values arrive
// via env, never interpolated into the script, so a hostile spec can't inject
// shell. init+fetch (not clone --branch) so Ref may be a branch OR a commit
// sha; an unset Ref fetches the remote's default branch (HEAD). A private
// repo authenticates through the same CloneSecret token contract as the build
// plane (x-access-token basic auth), supplied to git via an inline credential
// helper so the token never lands in argv or on disk.
func cloneContainer(o Options) corev1.Container {
	img := o.GitImage
	if img == "" {
		img = DefaultGitImage
	}
	ref := o.Ref
	if ref == "" {
		ref = "HEAD"
	}
	// The credential helper is execution.GitHubCredentialHelper — host-bound
	// to github.com; its SECURITY comment lives on the shared constant.
	script := `set -e
mkdir -p /work && cd /work
git init -q .
git remote add origin "$REPO"
git -c ` + execution.GitHubCredentialHelper + ` fetch -q --depth 1 origin "$REF"
git checkout -q FETCH_HEAD
cd "/work/$SRC_DIR"
cp -a . ` + outMount + `/`
	env := []corev1.EnvVar{
		{Name: "REPO", Value: o.Repo},
		{Name: "REF", Value: ref},
		{Name: "SRC_DIR", Value: o.srcDir()},
	}
	if o.CloneSecret != "" {
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
	return corev1.Container{
		Name:    "clone",
		Image:   img,
		Command: []string{"sh", "-c", script},
		Env:     env,
		VolumeMounts: []corev1.VolumeMount{
			{Name: outVolume, MountPath: outMount},
		},
		Resources: modestResources(),
	}
}

// Ephemeral-storage bounds for the publish Job (codex-security #3): tenant-
// controlled static output fills a disk-backed emptyDir with no SizeLimit and the
// containers carried only CPU/memory. Sized down — copy + sync is light; the build
// Job carries the larger bound.
const (
	publishEphemeralRequest = "1Gi"
	publishEphemeralLimit   = "2Gi"
	publishEmptyDirLimit    = "2Gi"
)

// mustSizeLimit parses s into a *resource.Quantity for an emptyDir SizeLimit.
func mustSizeLimit(s string) *resource.Quantity {
	q := resource.MustParse(s)
	return &q
}

// modestResources bounds the publish Job so a large upload can't starve the node
// (the same spirit as the build Job's limits, sized down — copy + sync is light).
func modestResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("100m"),
			corev1.ResourceMemory:           resource.MustParse("128Mi"),
			corev1.ResourceEphemeralStorage: resource.MustParse(publishEphemeralRequest),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse("1"),
			corev1.ResourceMemory:           resource.MustParse("512Mi"),
			corev1.ResourceEphemeralStorage: resource.MustParse(publishEphemeralLimit),
		},
	}
}

// JobName is the deterministic per-revision publish Job name; the distinct
// "pub-" prefix avoids build-Job collisions. A name hit is read as "this
// revision is already published", so two revisions sharing a name would serve
// stale content — which is why truncation hashes rather than slices.
func JobName(appID, revision string) string {
	return appv1alpha1.RevisionJobName("pub-", appID, revision)
}

// PurgeJob constructs the durable terminal cleanup Job for one exact App
// lifetime. The App finalizer creates, observes, and deletes this Job; success
// is persisted on the App before the Job is removed, so a manager restart can
// never acknowledge cleanup that has not completed.
func PurgeJob(appName, appUID, workspace, appNamespace string, store Store, namespace, pullSecret, awsCLIImage string, prefixes ...string) *batchv1.Job {
	if awsCLIImage == "" {
		awsCLIImage = DefaultAWSCLIImage
	}
	if len(prefixes) == 0 {
		prefixes = identity.ForApp(appName, workspace).PurgePrefixes("")
	}
	var b strings.Builder
	for i, p := range prefixes {
		if i > 0 {
			b.WriteString(" && ")
		}
		b.WriteString("aws s3 rm ")
		b.WriteString("s3://" + store.Bucket + "/" + strings.TrimPrefix(p, "/"))
		b.WriteString(" --recursive --endpoint-url ")
		b.WriteString(store.Endpoint)
	}
	script := b.String()
	deadline := int64((15 * time.Minute) / time.Second)
	backoff := int32(3)
	var env []corev1.EnvVar
	if store.Region != "" {
		env = append(env, corev1.EnvVar{Name: "AWS_DEFAULT_REGION", Value: store.Region})
	}
	sum := sha256.Sum256([]byte(appUID))
	jobName := fmt.Sprintf("purge-%s-%x", appName, sum[:4])
	if len(jobName) > 63 {
		jobName = fmt.Sprintf("purge-%.45s-%x", appName, sum[:6])
	}
	labels := execution.PodLabels(appName, appUID, "static-purge", workspace, appNamespace, false)
	labels["app.bex.co/purge"] = appName
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoff,
			ActiveDeadlineSeconds: &deadline,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:    corev1.RestartPolicyNever,
					ImagePullSecrets: imagePullSecrets(pullSecret),
					Containers: []corev1.Container{{
						Name:    "purge",
						Image:   awsCLIImage,
						Command: []string{"sh", "-c", script},
						Env:     env,
						EnvFrom: []corev1.EnvFromSource{{
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: store.Secret},
							},
						}},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("64Mi"),
							},
						},
					}},
				},
			},
		},
	}
	execution.HardenPod(&job.Spec.Template.Spec)
	return job
}
