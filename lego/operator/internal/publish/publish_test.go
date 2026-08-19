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

package publish

import (
	"context"
	"slices"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func testOptions() Options {
	return Options{
		Image:       "zot.bex-registry.svc:5000/mysite:gen-3",
		PublishPath: "dist",
		AppID:       "mysite",
		AppUID:      "uid-mysite",
		Revision:    "rev-3",
		Store: Store{
			Bucket:   "bex-static",
			Endpoint: "https://s3.eu-central-2.wasabisys.com",
			Secret:   "bex-static-publish-s3",
		},
		Namespace: "bex-system",
	}
}

func TestStoreConfigured(t *testing.T) {
	cases := []struct {
		name string
		s    Store
		want bool
	}{
		{"all set", Store{Bucket: "b", Endpoint: "e", Secret: "s"}, true},
		{"no bucket", Store{Endpoint: "e", Secret: "s"}, false},
		{"no endpoint", Store{Bucket: "b", Secret: "s"}, false},
		{"no secret", Store{Bucket: "b", Endpoint: "e"}, false},
		{"empty", Store{}, false},
	}
	for _, c := range cases {
		if got := c.s.Configured(); got != c.want {
			t.Errorf("%s: Configured() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCredentialBearingPublishImagesAreDigestPinned(t *testing.T) {
	for name, image := range map[string]string{"aws": DefaultAWSCLIImage, "git": DefaultGitImage} {
		if !strings.Contains(image, "@sha256:") {
			t.Errorf("%s image is mutable: %q", name, image)
		}
	}
}

func TestPrefixAndDest(t *testing.T) {
	o := testOptions()
	if got := o.Prefix(); got != "mysite/rev-3/" {
		t.Errorf("Prefix() = %q, want mysite/rev-3/", got)
	}
	if got := o.destURI(); got != "s3://bex-static/mysite/rev-3/" {
		t.Errorf("destURI() = %q, want s3://bex-static/mysite/rev-3/", got)
	}
}

func TestPrefixWorkspaceScoped(t *testing.T) {
	o := testOptions()
	o.Workspace = "tea-aaaaaaaaaaaaaaaaaaaa"
	if got, want := o.Prefix(), "tea-aaaaaaaaaaaaaaaaaaaa/mysite/rev-3/"; got != want {
		t.Errorf("Prefix() = %q, want %q", got, want)
	}
	o.Workspace = ""
	if got, want := o.Prefix(), "mysite/rev-3/"; got != want {
		t.Errorf("unlabeled Prefix() = %q, want %q", got, want)
	}
}

func TestPurgeJobHitsOnlyOwningPrefixes(t *testing.T) {
	o := testOptions()
	job := PurgeJob("web", "uid-a", "tea-aaaaaaaaaaaaaaaaaaaa", "tea-aaaaaaaaaaaaaaaaaaaa",
		o.Store, "bex-system", "", "", "tea-aaaaaaaaaaaaaaaaaaaa/web/")
	script := job.Spec.Template.Spec.Containers[0].Command[2]
	if !strings.Contains(script, "s3://bex-static/tea-aaaaaaaaaaaaaaaaaaaa/web/") {
		t.Errorf("purge missed scoped prefix: %s", script)
	}
	if strings.Contains(script, "s3://bex-static/web/") {
		t.Errorf("purge must not touch a legacy same-named sibling: %s", script)
	}

	legacy := PurgeJob("web", "uid-legacy", "", "default", o.Store, "bex-system", "", "")
	legacyScript := legacy.Spec.Template.Spec.Containers[0].Command[2]
	if !strings.Contains(legacyScript, "s3://bex-static/web/") {
		t.Errorf("unlabeled purge missed legacy prefix: %s", legacyScript)
	}
	if strings.Contains(legacyScript, "tea-aaaaaaaaaaaaaaaaaaaa") {
		t.Errorf("unlabeled purge touched a scoped sibling: %s", legacyScript)
	}
}

func TestPublishJobShape(t *testing.T) {
	job := PublishJob(testOptions())

	if job.Name != "pub-mysite-rev-3" {
		t.Errorf("job name = %q, want pub-mysite-rev-3", job.Name)
	}
	if job.Namespace != "bex-system" {
		t.Errorf("namespace = %q, want bex-system", job.Namespace)
	}
	if job.Labels["app.bex.co/app-uid"] != "uid-mysite" || job.Spec.Template.Labels["app.bex.co/app-uid"] != "uid-mysite" {
		t.Fatalf("publish artifact missing App UID labels: job=%v pod=%v", job.Labels, job.Spec.Template.Labels)
	}

	pod := job.Spec.Template.Spec
	if len(pod.InitContainers) != 1 {
		t.Fatalf("init containers = %d, want 1", len(pod.InitContainers))
	}
	extract := pod.InitContainers[0]
	if extract.Image != "zot.bex-registry.svc:5000/mysite:gen-3" {
		t.Errorf("extract image = %q, want the built image", extract.Image)
	}
	// PublishPath is passed via env, never interpolated into the shell string.
	if got := strings.Join(extract.Command, " "); strings.Contains(got, "dist") {
		t.Errorf("extract command %q must not embed publishPath (injection risk)", got)
	}
	var sawPublishEnv bool
	for _, e := range extract.Env {
		if e.Name == "PUBLISH_PATH" && e.Value == "dist" {
			sawPublishEnv = true
		}
	}
	if !sawPublishEnv {
		t.Errorf("extract missing PUBLISH_PATH=dist env")
	}

	if len(pod.Containers) != 1 {
		t.Fatalf("containers = %d, want 1", len(pod.Containers))
	}
	upload := pod.Containers[0]
	wantArgs := []string{
		"s3", "sync", "/out", "s3://bex-static/mysite/rev-3/",
		"--endpoint-url", "https://s3.eu-central-2.wasabisys.com", "--no-follow-symlinks", "--delete",
	}
	if !slices.Equal(upload.Args, wantArgs) {
		t.Errorf("upload args = %v, want %v", upload.Args, wantArgs)
	}
	// Credentials come from the S3 Secret via envFrom.
	if len(upload.EnvFrom) != 1 || upload.EnvFrom[0].SecretRef == nil ||
		upload.EnvFrom[0].SecretRef.Name != "bex-static-publish-s3" {
		t.Errorf("upload must envFrom the S3 secret bex-static-publish-s3, got %+v", upload.EnvFrom)
	}

	// Both containers share the /out emptyDir.
	if len(pod.Volumes) != 1 || pod.Volumes[0].EmptyDir == nil {
		t.Fatalf("want one emptyDir volume, got %+v", pod.Volumes)
	}
	if pod.RestartPolicy != "Never" {
		t.Errorf("restart policy = %q, want Never", pod.RestartPolicy)
	}
	if pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
		t.Error("publish pod must disable the Kubernetes API token")
	}
	if pod.HostUsers == nil || *pod.HostUsers {
		t.Error("publish pod must use a Pod user namespace (spec.hostUsers=false)")
	}
	if pod.NodeSelector["bex.co/pool"] != "tenant" {
		t.Errorf("publish node selector = %v", pod.NodeSelector)
	}
}

func TestPublishJobIdentityAndVerificationLabels(t *testing.T) {
	o := testOptions()
	o.Namespace = "bex-build"
	o.AppNamespace = "default"
	o.Workspace = "tea-1"
	o.VerifyImage = true
	labels := PublishJob(o).Spec.Template.Labels
	for key, want := range map[string]string{
		"app.bex.co/app":           "mysite",
		"app.bex.co/app-uid":       "uid-mysite",
		"app.bex.co/component":     "publish",
		"app.bex.co/workspace":     "tea-1",
		"app.bex.co/app-namespace": "default",
		"app.bex.co/verify-image":  "enabled",
	} {
		if labels[key] != want {
			t.Errorf("label %s = %q, want %q", key, labels[key], want)
		}
	}
	o.Image = ""
	o.Repo = "https://github.com/bex-co/bex"
	if _, ok := PublishJob(o).Spec.Template.Labels["app.bex.co/verify-image"]; ok {
		t.Error("direct-clone publish must not select the tenant-image webhook")
	}
}

func TestPublishJobNameTruncation(t *testing.T) {
	long := strings.Repeat("a", 80)
	if n := JobName(long, "rev-1"); len(n) > 63 {
		t.Errorf("job name len = %d, want ≤63", len(n))
	}
}

// TestPublishJobPullSecret pins w7/m8: the extract initContainer pulls the built
// image from an auth-enabled registry, so the publish pod carries an
// imagePullSecret when one is configured, and omits it (byte-identical default)
// when unset.
func TestPublishJobPullSecret(t *testing.T) {
	// Unset → no imagePullSecret (dev default).
	if got := PublishJob(testOptions()).Spec.Template.Spec.ImagePullSecrets; got != nil {
		t.Errorf("unset pull secret = %+v; want nil", got)
	}
	// Set → attached so kubelet authenticates the extract pull.
	o := testOptions()
	o.PullSecret = "bex-registry-pull"
	got := PublishJob(o).Spec.Template.Spec.ImagePullSecrets
	if len(got) != 1 || got[0].Name != "bex-registry-pull" {
		t.Errorf("pull secret = %+v; want [bex-registry-pull]", got)
	}
}

func TestRegionEnvOptional(t *testing.T) {
	o := testOptions()
	o.Store.Region = "eu-central-2"
	up := PublishJob(o).Spec.Template.Spec.Containers[0]
	var sawRegion bool
	for _, e := range up.Env {
		if e.Name == "AWS_DEFAULT_REGION" && e.Value == "eu-central-2" {
			sawRegion = true
		}
	}
	if !sawRegion {
		t.Errorf("region set => AWS_DEFAULT_REGION env expected")
	}
	// Unset region => no AWS_DEFAULT_REGION env (comes from the secret if present).
	up = PublishJob(testOptions()).Spec.Template.Spec.Containers[0]
	for _, e := range up.Env {
		if e.Name == "AWS_DEFAULT_REGION" {
			t.Errorf("region unset => no AWS_DEFAULT_REGION env, got %q", e.Value)
		}
	}
}

// TestPublishJobCloneMode pins the direct (no-Dockerfile) publish shape
// (w9/010): a Repo-sourced Options builds a "clone" initContainer on the
// pinned git image — shallow init+fetch (branch OR sha), checkout FETCH_HEAD,
// copy rootDir/publishPath into the emptyDir — with every value passed via
// env, an optional CloneSecret token env for private repos, and NO
// imagePullSecrets (clone mode pulls only public platform images).
func TestPublishJobCloneMode(t *testing.T) {
	o := testOptions()
	o.Image = ""
	o.Repo = "https://github.com/bex-co/bex"
	o.Ref = "main"
	o.RootDir = "examples/static-site"
	o.PublishPath = "."
	o.PullSecret = "bex-registry-pull" // must be dropped in clone mode
	job := PublishJob(o)

	pod := job.Spec.Template.Spec
	if len(pod.ImagePullSecrets) != 0 {
		t.Errorf("imagePullSecrets = %v, want none in clone mode", pod.ImagePullSecrets)
	}
	if len(pod.InitContainers) != 1 || pod.InitContainers[0].Name != "clone" {
		t.Fatalf("initContainers = %+v, want exactly one named clone", pod.InitContainers)
	}
	clone := pod.InitContainers[0]
	if clone.Image != DefaultGitImage {
		t.Errorf("clone image = %q, want %q", clone.Image, DefaultGitImage)
	}
	script := clone.Command[len(clone.Command)-1]
	for _, frag := range []string{"git init", "fetch -q --depth 1 origin \"$REF\"", "checkout -q FETCH_HEAD", `cd "/work/$SRC_DIR"`, "cp -a . /out/"} {
		if !strings.Contains(script, frag) {
			t.Errorf("clone script missing %q:\n%s", frag, script)
		}
	}
	env := map[string]string{}
	for _, e := range clone.Env {
		env[e.Name] = e.Value
	}
	if env["REPO"] != o.Repo || env["REF"] != "main" || env["SRC_DIR"] != "examples/static-site" {
		t.Errorf("clone env = %v, want REPO/REF/SRC_DIR set (SRC_DIR joins rootDir+publishPath cleaned)", env)
	}

	// Private repo: the token arrives via a Secret-backed env, key "token".
	o.CloneSecret = "web-clone"
	clone = PublishJob(o).Spec.Template.Spec.InitContainers[0]
	found := false
	for _, e := range clone.Env {
		if e.Name == "GIT_AUTH_TOKEN" && e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil &&
			e.ValueFrom.SecretKeyRef.Name == "web-clone" && e.ValueFrom.SecretKeyRef.Key == "token" {
			found = true
		}
	}
	if !found {
		t.Errorf("clone env %v missing GIT_AUTH_TOKEN from Secret web-clone key token", clone.Env)
	}

	// Empty Ref falls back to the remote default branch (HEAD).
	o.Ref = ""
	clone = PublishJob(o).Spec.Template.Spec.InitContainers[0]
	for _, e := range clone.Env {
		if e.Name == "REF" && e.Value != "HEAD" {
			t.Errorf("empty Ref => REF = %q, want HEAD", e.Value)
		}
	}
}

// TestPublishSourceValidation pins Ensure's source rules (w9/010): exactly one
// of Image/Repo, and a rootDir/publishPath that cannot escape the checkout.
func TestPublishSourceValidation(t *testing.T) {
	ctx := context.Background()
	both := testOptions()
	both.Repo = "https://github.com/x/y"
	both.Client = fake.NewClientBuilder().Build()
	if _, err := Ensure(ctx, both); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("both sources => %v, want the exactly-one error", err)
	}
	neither := testOptions()
	neither.Image = ""
	neither.Client = fake.NewClientBuilder().Build()
	if _, err := Ensure(ctx, neither); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("no source => %v, want the exactly-one error", err)
	}
	escape := testOptions()
	escape.Image = ""
	escape.Repo = "https://github.com/x/y"
	escape.RootDir = ".."
	escape.PublishPath = "etc"
	escape.Client = fake.NewClientBuilder().Build()
	if _, err := Ensure(ctx, escape); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Errorf("escaping path => %v, want the escape error", err)
	}
}

func TestPublishRequiresAppUID(t *testing.T) {
	o := testOptions()
	o.AppUID = ""
	o.Client = fake.NewClientBuilder().Build()
	if _, err := Ensure(context.Background(), o); err == nil || !strings.Contains(err.Error(), "empty App UID") {
		t.Fatalf("Ensure error = %v, want missing-identity rejection", err)
	}
}

// TestEnsureNeverBlocks (round-13 #6): Ensure reports an in-flight Job as
// PhasePublishing immediately — no poll loop owns the reconcile worker — and
// reports terminal Jobs by their conditions. The Job's own
// activeDeadlineSeconds owns the wall-clock bound the old blocking loop held.
func TestEnsureNeverBlocks(t *testing.T) {
	ctx := context.Background()
	o := testOptions()
	cl := fake.NewClientBuilder().Build()
	o.Client = cl

	obs, err := Ensure(ctx, o)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if obs.Phase != PhasePublishing {
		t.Fatalf("fresh Job phase = %q, want %q (no blocking wait)", obs.Phase, PhasePublishing)
	}

	// Drive the Job to Complete: Ensure then reports success without re-creating.
	job := &batchv1.Job{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(PublishJob(o)), job); err != nil {
		t.Fatalf("get dispatched Job: %v", err)
	}
	job.Status.Conditions = append(job.Status.Conditions, batchv1.JobCondition{
		Type: batchv1.JobComplete, Status: corev1.ConditionTrue,
	})
	if err := cl.Status().Update(ctx, job); err != nil {
		t.Fatalf("complete Job: %v", err)
	}
	obs, err = Ensure(ctx, o)
	if err != nil {
		t.Fatalf("Ensure on completed Job: %v", err)
	}
	if obs.Phase != PhaseSucceeded {
		t.Fatalf("completed Job phase = %q, want %q", obs.Phase, PhaseSucceeded)
	}

	// A failed Job surfaces its message.
	job.Status.Conditions = []batchv1.JobCondition{{
		Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "DeadLineExceeded",
		Message: "Job was active longer than specified deadline",
	}}
	if err := cl.Status().Update(ctx, job); err != nil {
		t.Fatalf("fail Job: %v", err)
	}
	obs, err = Ensure(ctx, o)
	if err != nil {
		t.Fatalf("Ensure on failed Job: %v", err)
	}
	if obs.Phase != PhaseFailed || !strings.Contains(obs.Message, "deadline") {
		t.Fatalf("failed Job = %+v, want PhaseFailed with the deadline message", obs)
	}
}
