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
	"slices"
	"strings"
	"testing"
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
