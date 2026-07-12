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

// defaultAWSCLIImage is the S3 uploader image. Pinned < 2.23 because newer AWS
// CLIs send CRC64 checksums that S3-compatibles (Wasabi/Hetzner) reject — the
// same pin the etcd/OpenBao backup CronJobs use (docs/ADR011-etcd-backup-restore.md).
const defaultAWSCLIImage = "amazon/aws-cli:2.22.35"

// publishTimeout bounds a single publish Job's wall-clock; the Job's own
// activeDeadlineSeconds matches so a stuck upload is reaped, not left lingering.
const publishTimeout = 10 * time.Minute

// pollInterval is how often Publish re-reads the Job while waiting for it.
const pollInterval = 3 * time.Second

// outVolume is the emptyDir the extract init-container copies PublishPath into
// and the upload container syncs from.
const outVolume = "out"
const outMount = "/out"

// Store is the operator's static-site object-store target: the S3-compatible
// bucket + endpoint (deployment-time config, BEX_STATIC_S3_ENDPOINT/BUCKET) and
// the name of a k8s Secret holding AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY
// (BEX_STATIC_S3_SECRET). All three unset => static_site is unavailable, the way
// unset BEX_DB_BACKUP_* disables Postgres backups. Reuses the Terraform-state
// object-store credentials; the bucket MUST be separate from bex-tfstate (never
// serve public content from the private state/backup bucket — see the milestone
// design record and docs/ADR029-static-sites.md).
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

// Options configures a single static-site publish.
type Options struct {
	Image       string        // OCI image the build produced (holds the built site)
	PublishPath string        // dir within the image to serve (App.spec.publishPath)
	AppID       string        // the service name — the first object-key segment
	Revision    string        // e.g. "rev-7" — the second key segment (immutable per revision)
	Store       Store         // object-store target (bucket/endpoint/secret)
	Namespace   string        // namespace the publish Job runs in
	Client      client.Client // cluster client used to create + watch the Job
	// AWSCLIImage overrides the uploader image (tests / air-gapped).
	AWSCLIImage string
}

// Prefix is the object-key prefix a publish writes under: "<appID>/<revision>/".
// The static-server reads the same prefix, so the prefix root is the site root.
func (o Options) Prefix() string {
	return o.AppID + "/" + o.Revision + "/"
}

// destURI is the s3:// sync target for this publish.
func (o Options) destURI() string {
	return "s3://" + o.Store.Bucket + "/" + o.Prefix()
}

// Publish dispatches an in-cluster Job that extracts o.PublishPath from o.Image
// and uploads it to the object-store origin under o.Prefix(); it blocks until
// the Job succeeds or fails. Re-invocation for the same revision is idempotent:
// the Job is named per revision, so a retry adopts the existing Job.
func Publish(ctx context.Context, o Options) error {
	if o.Client == nil {
		return fmt.Errorf("publish: nil client (in-cluster publish requires a cluster client)")
	}
	if o.PublishPath == "" {
		return fmt.Errorf("publish: empty publishPath")
	}
	if !o.Store.Configured() {
		return fmt.Errorf("publish: object store not configured (set BEX_STATIC_S3_ENDPOINT/BUCKET/SECRET)")
	}

	job := PublishJob(o)
	key := client.ObjectKeyFromObject(job)

	if err := o.Client.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("publish: create job %s: %w", key.Name, err)
	}

	wctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()
	for {
		var cur batchv1.Job
		if err := o.Client.Get(wctx, key, &cur); err != nil {
			return fmt.Errorf("publish: get job %s: %w", key.Name, err)
		}
		switch {
		case jobCondition(&cur, batchv1.JobComplete):
			return nil
		case jobCondition(&cur, batchv1.JobFailed):
			return fmt.Errorf("publish: job %s failed: %s", key.Name, jobFailureMessage(&cur))
		}
		select {
		case <-wctx.Done():
			return fmt.Errorf("publish: job %s did not finish within %s", key.Name, publishTimeout)
		case <-time.After(pollInterval):
		}
	}
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
		awsImage = defaultAWSCLIImage
	}

	deadline := int64(publishTimeout / time.Second)
	backoff := int32(1) // one attempt; a failed publish is a user/config error, not a flake
	ttl := int32(3600)  // reap the finished Job after an hour

	uploadEnv := []corev1.EnvVar{}
	if o.Store.Region != "" {
		uploadEnv = append(uploadEnv, corev1.EnvVar{Name: "AWS_DEFAULT_REGION", Value: o.Store.Region})
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JobName(o.AppID, o.Revision),
			Namespace: o.Namespace,
			Labels: map[string]string{
				"app.bex.co/publish":   o.AppID,
				"app.bex.co/component": "publish",
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			ActiveDeadlineSeconds:   &deadline,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app.bex.co/publish": o.AppID},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{{
						Name:         outVolume,
						VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
					}},
					InitContainers: []corev1.Container{{
						Name:  "extract",
						Image: o.Image,
						// PublishPath via env (not the command string) so a hostile
						// value can't inject shell. cp -a preserves the tree; the
						// trailing /. copies contents, not the dir itself.
						Command: []string{"sh", "-c", `set -e; cp -a "$PUBLISH_PATH"/. ` + outMount + `/`},
						Env:     []corev1.EnvVar{{Name: "PUBLISH_PATH", Value: o.PublishPath}},
						VolumeMounts: []corev1.VolumeMount{
							{Name: outVolume, MountPath: outMount},
						},
						Resources: modestResources(),
					}},
					Containers: []corev1.Container{{
						Name:  "upload",
						Image: awsImage,
						// amazon/aws-cli's entrypoint is `aws`, so Args start at the verb.
						Args: []string{
							"s3", "sync", outMount, o.destURI(),
							"--endpoint-url", o.Store.Endpoint,
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
				},
			},
		},
	}
}

// modestResources bounds the publish Job so a large upload can't starve the node
// (the same spirit as the build Job's limits, sized down — copy + sync is light).
func modestResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
	}
}

// JobName is the deterministic per-revision publish Job name (DNS-1123, ≤63):
// re-reconciling the same revision adopts the existing Job; a new revision gets a
// fresh publish. Distinct "pub-" prefix so it never collides with the build Job.
func JobName(appID, revision string) string {
	rev := revision
	if rev == "" {
		rev = "latest"
	}
	n := "pub-" + appID + "-" + rev
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
	return "unknown publish failure"
}
