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

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func testOptions() Options {
	return Options{
		Image:       "zot.bex-registry.svc:5000/mysite:gen-3",
		PublishPath: "dist",
		AppID:       "mysite",
		Revision:    "rev-3",
		Store: Store{
			Bucket:   "bex-static",
			Endpoint: "https://s3.eu-central-2.wasabisys.com",
			Secret:   "static-s3",
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

func TestPrefixAndDest(t *testing.T) {
	o := testOptions()
	if got := o.Prefix(); got != "mysite/rev-3/" {
		t.Errorf("Prefix() = %q, want mysite/rev-3/", got)
	}
	if got := o.destURI(); got != "s3://bex-static/mysite/rev-3/" {
		t.Errorf("destURI() = %q, want s3://bex-static/mysite/rev-3/", got)
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
		"--endpoint-url", "https://s3.eu-central-2.wasabisys.com", "--delete",
	}
	if !slices.Equal(upload.Args, wantArgs) {
		t.Errorf("upload args = %v, want %v", upload.Args, wantArgs)
	}
	// Credentials come from the S3 Secret via envFrom.
	if len(upload.EnvFrom) != 1 || upload.EnvFrom[0].SecretRef == nil ||
		upload.EnvFrom[0].SecretRef.Name != "static-s3" {
		t.Errorf("upload must envFrom the S3 secret static-s3, got %+v", upload.EnvFrom)
	}

	// Both containers share the /out emptyDir.
	if len(pod.Volumes) != 1 || pod.Volumes[0].EmptyDir == nil {
		t.Fatalf("want one emptyDir volume, got %+v", pod.Volumes)
	}
	if pod.RestartPolicy != "Never" {
		t.Errorf("restart policy = %q, want Never", pod.RestartPolicy)
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

// TestPublishSourceValidation pins Publish's source rules (w9/010): exactly one
// of Image/Repo, and a rootDir/publishPath that cannot escape the checkout.
func TestPublishSourceValidation(t *testing.T) {
	ctx := context.Background()
	both := testOptions()
	both.Repo = "https://github.com/x/y"
	both.Client = fake.NewClientBuilder().Build()
	if err := Publish(ctx, both); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("both sources => %v, want the exactly-one error", err)
	}
	neither := testOptions()
	neither.Image = ""
	neither.Client = fake.NewClientBuilder().Build()
	if err := Publish(ctx, neither); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("no source => %v, want the exactly-one error", err)
	}
	escape := testOptions()
	escape.Image = ""
	escape.Repo = "https://github.com/x/y"
	escape.RootDir = ".."
	escape.PublishPath = "etc"
	escape.Client = fake.NewClientBuilder().Build()
	if err := Publish(ctx, escape); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Errorf("escaping path => %v, want the escape error", err)
	}
}
