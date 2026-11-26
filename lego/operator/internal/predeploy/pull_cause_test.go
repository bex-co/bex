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

package predeploy

import (
	"context"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A node's failed pull of an image the user supplied names one of Render's
// four Image Pull Failed causes, as the image metadata read's refusal does
// (w4/m669). The messages are containerd's and the kubelet's own, ErrImagePull
// and the back-off line that carries the pull error since Kubernetes 1.30,
// for Docker Hub, GHCR and a self-hosted registry.
func TestUserPullFailureNamesRendersCause(t *testing.T) {
	const (
		hub    = "docker.io/library/nginx:does-not-exist-7f3a"
		ghcr   = "ghcr.io/acme/private:v1"
		self   = "registry.example.com:5000/team/api:2"
		failed = `failed to pull and unpack image "%s": failed to resolve reference "%s": `
	)
	pull := func(image, tail string) string {
		return "rpc error: code = Unknown desc = " + strings.ReplaceAll(failed, "%s", image) + tail
	}
	for _, tc := range []struct {
		name         string
		image        string
		kubelet      string
		credentialed bool
		code         string
		want         []string
	}{
		{
			name:    "Docker Hub: a tag that does not exist",
			image:   hub,
			kubelet: "rpc error: code = NotFound desc = " + strings.ReplaceAll(failed, "%s", hub) + hub + ": not found",
			code:    appv1alpha1.DeployReasonImageNotFound,
			want:    []string{"the image " + hub + " was not found in its registry", "Check the image URL and tag"},
		},
		{
			name:    "the back-off line keeps the not-found error behind it",
			image:   hub,
			kubelet: `Back-off pulling image "` + hub + `": ErrImagePull: rpc error: code = NotFound desc = failed to pull and unpack image: not found`,
			code:    appv1alpha1.DeployReasonImageNotFound,
		},
		{
			name:    "a self-hosted registry: manifest unknown",
			image:   self,
			kubelet: pull(self, "unexpected status from GET request to https://"+self+"/v2/team/api/manifests/2: 404 Not Found"),
			code:    appv1alpha1.DeployReasonImageNotFound,
		},
		{
			name:    "Docker Hub: a private or missing repository without a credential",
			image:   "docker.io/acme/private:v1",
			kubelet: pull("docker.io/acme/private:v1", "pull access denied, repository does not exist or may require authorization: server message: insufficient_scope: authorization failed"),
			code:    appv1alpha1.DeployReasonImageAccess,
			want:    []string{"add a registry credential for docker.io to this service"},
		},
		{
			name:         "Docker Hub: the service's credential refused",
			image:        "docker.io/acme/private:v1",
			kubelet:      pull("docker.io/acme/private:v1", "failed to authorize: failed to fetch oauth token: unexpected status from POST request to https://auth.docker.io/token: 401 Unauthorized"),
			credentialed: true,
			code:         appv1alpha1.DeployReasonImageCredential,
			want:         []string{"docker.io refused this service's registry credential", "(HTTP 401)"},
		},
		{
			name:         "GHCR: the service's credential refused",
			image:        ghcr,
			kubelet:      pull(ghcr, "failed to authorize: failed to fetch oauth token: unexpected status from GET request to https://ghcr.io/token?scope=repository%3Aacme%2Fprivate%3Apull&service=ghcr.io: 403 Forbidden"),
			credentialed: true,
			code:         appv1alpha1.DeployReasonImageCredential,
			want:         []string{"ghcr.io refused this service's registry credential", "(HTTP 403)"},
		},
		{
			name:    "GHCR: a private image without a credential",
			image:   ghcr,
			kubelet: pull(ghcr, "failed to authorize: failed to fetch anonymous token: unexpected status from GET request to https://ghcr.io/token: 403 Forbidden"),
			code:    appv1alpha1.DeployReasonImageAccess,
			want:    []string{"add a registry credential for ghcr.io"},
		},
		{
			name:         "a self-hosted registry refuses basic auth",
			image:        self,
			kubelet:      pull(self, "pulling from host registry.example.com:5000 failed with status code [manifests 2]: 401 Unauthorized"),
			credentialed: true,
			code:         appv1alpha1.DeployReasonImageCredential,
			want:         []string{"registry.example.com:5000 refused"},
		},
		{
			name:    "Docker Hub: the anonymous pull rate limit",
			image:   hub,
			kubelet: `failed to copy: httpReadSeeker: failed open: unexpected status code https://registry-1.docker.io/v2/library/nginx/manifests/sha256:abc: 429 Too Many Requests - Server message: toomanyrequests: You have reached your pull rate limit. You may increase the limit by authenticating and upgrading: https://www.docker.com/increase-rate-limit`,
			code:    appv1alpha1.DeployReasonImageRegistryRateLimited,
			want:    []string{"(HTTP 429)", "Wait for the rate limit to reset", "higher rate limit", "mirror the image to another registry"},
		},
		{
			name:    "a rate limit with a credential is still a rate limit",
			image:   ghcr,
			kubelet: pull(ghcr, "unexpected status from HEAD request to https://ghcr.io/v2/acme/private/manifests/v1: 429 Too Many Requests"),
			code:    appv1alpha1.DeployReasonImageRegistryRateLimited,
		},
		{
			name:    "a registry that answers 503",
			image:   self,
			kubelet: pull(self, "unexpected status from HEAD request to https://"+self+"/v2/team/api/manifests/2: 503 Service Unavailable"),
			code:    appv1alpha1.DeployReasonImageRegistryUnavailable,
			want:    []string{"the registry registry.example.com:5000 is unavailable", "check the registry provider's status page"},
		},
		{
			name:    "a registry that cannot be resolved",
			image:   self,
			kubelet: pull(self, "dial tcp: lookup registry.example.com on 10.96.0.10:53: no such host"),
			code:    appv1alpha1.DeployReasonImageRegistryUnavailable,
		},
		{
			name:    "the CRI's own registry-unavailable verdict",
			image:   hub,
			kubelet: "image pull failed for " + hub + " because the registry is unavailable",
			code:    appv1alpha1.DeployReasonImageRegistryUnavailable,
		},
	} {
		got, ok := UserPullFailure(tc.image, tc.kubelet, tc.credentialed)
		if !ok {
			t.Errorf("%s: no cause named for %q", tc.name, tc.kubelet)
			continue
		}
		if code, _, parsed := appv1alpha1.ParseDeployReason(got); !parsed || code != tc.code {
			t.Errorf("%s: %q parses as %q (ok %v), want %q", tc.name, got, code, parsed, tc.code)
		}
		for _, want := range tc.want {
			if !strings.Contains(got, want) {
				t.Errorf("%s: %q does not say %q", tc.name, got, want)
			}
		}
		if strings.Contains(got, "rpc error") || strings.Contains(got, "Back-off") {
			t.Errorf("%s: %q repeats the kubelet's line", tc.name, got)
		}
	}
}

// A kubelet line that names no registry answer, the back-off line of a
// kubelet older than 1.30 or an error bex has no words for, names no cause:
// the caller keeps the kubelet's own text (w4/m669).
func TestUserPullFailureWithoutACauseKeepsTheKubeletsText(t *testing.T) {
	for _, kubelet := range []string{
		`Back-off pulling image "docker.io/x/y:does-not-exist-999"`,
		`rpc error: code = Unknown desc = failed to pull and unpack image "docker.io/x/y:1": failed to extract layer: no space left on device`,
		"",
	} {
		if got, ok := UserPullFailure("docker.io/x/y:1", kubelet, false); ok {
			t.Errorf("%q named a cause: %q", kubelet, got)
		}
	}
	w := &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: `Back-off pulling image "docker.io/x/y:1"`}
	if got, want := pullFailureMessage(w, "docker.io/x/y:1", nil, false), `image pull is failing: Back-off pulling image "docker.io/x/y:1"; the pre-deploy command never ran`; got != want {
		t.Errorf("pre-deploy message = %q, want %q", got, want)
	}
}

// A pre-deploy Job's pull failure names the same cause as the rollout's, and
// still says the command never ran; a Job that pulls with the service's
// credential reads a 401 as that credential refused.
func TestPreDeployPullFailureNamesTheCause(t *testing.T) {
	const image = "docker.io/acme/private:v1"
	w := &corev1.ContainerStateWaiting{Reason: "ErrImagePull", Message: "rpc error: code = Unknown desc = failed to authorize: failed to fetch oauth token: unexpected status from POST request to https://auth.docker.io/token: 401 Unauthorized"}
	want := appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageCredential,
		appv1alpha1.DeployReasonParam{Name: "host", Value: "docker.io"},
		appv1alpha1.DeployReasonParam{Name: "image", Value: image},
		appv1alpha1.DeployReasonParam{Name: "status", Value: "401"}) + "; the pre-deploy command never ran"
	if got := pullFailureMessage(w, image, nil, true); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
	if got := pullFailureMessage(w, image, nil, false); !strings.HasPrefix(got, "the registry refused access to the image "+image) {
		t.Errorf("without a credential, message = %q, want the access refusal", got)
	}
}

// A pre-deploy Job that pulls with the service's registry credential, as the
// app pod does, reads a 401 as that credential refused, on the failed Job and
// on the pending one past the grace window alike.
func TestAPreDeployJobWithACredentialNamesItRefused(t *testing.T) {
	refused := corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ErrImagePull",
		Message: "rpc error: code = Unknown desc = failed to authorize: failed to fetch oauth token: unexpected status from POST request to https://auth.docker.io/token: 401 Unauthorized"}}
	credentialed := func(j *batchv1.Job) *batchv1.Job {
		j.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "web-registry-rgc-0123456789abcdefghij"}}
		return j
	}
	const want = "docker.io refused this service's registry credential for the image docker.io/x/y:nope (HTTP 401)"
	failed := credentialed(failedJob(batchv1.JobReasonDeadlineExceeded))
	if got := FailureMessage(context.Background(), fakeClient(failed, userImagePod(refused)), failed, nil); !strings.HasPrefix(got, want) {
		t.Errorf("FailureMessage = %q, want it to open %q", got, want)
	}
	pending := credentialed(failedJob(""))
	pending.Status.Conditions = nil
	if got := PullFailure(context.Background(), fakeClient(pending, userImagePod(refused)), pending, time.Now(), nil); !strings.HasPrefix(got, want) {
		t.Errorf("PullFailure = %q, want it to open %q", got, want)
	}
	plain := failedJob(batchv1.JobReasonDeadlineExceeded)
	if got := FailureMessage(context.Background(), fakeClient(plain, userImagePod(refused)), plain, nil); !strings.HasPrefix(got, "the registry refused access to the image docker.io/x/y:nope") {
		t.Errorf("without a credential, FailureMessage = %q, want the access refusal", got)
	}
}

func TestRegistryHost(t *testing.T) {
	for image, want := range map[string]string{
		"nginx:1.27":                    "docker.io",
		"acme/web:v1":                   "docker.io",
		"docker.io/library/nginx":       "docker.io",
		"ghcr.io/acme/web:v1":           "ghcr.io",
		"localhost/web":                 "localhost",
		"registry.example.com:5000/x:1": "registry.example.com:5000",
	} {
		if got := RegistryHost(image); got != want {
			t.Errorf("RegistryHost(%q) = %q, want %q", image, got, want)
		}
	}
}
