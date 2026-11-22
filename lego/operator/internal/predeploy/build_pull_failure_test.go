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
	"regexp"
	"strings"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A pre-deploy plane Job runs the release's image, which for a Git-built
// service is a build in the platform's own registry. Its pull failure must
// read as the rollout's does, in product terms, never with the kubelet's line:
// that names the cluster-internal registry host, the node's registry mirror,
// the repository path and the digest.

const testRegistry = "zot.bex-registry.svc:5000"

// testBuilds stands in for the controller's recognizer: an image under the
// platform registry is a build, named without the host or digest.
func testBuilds(image string) (string, bool) {
	name, ok := strings.CutPrefix(image, testRegistry+"/")
	if !ok {
		return "", false
	}
	build, _, _ := strings.Cut(name, "@")
	return build, true
}

// internalText is what no build pull failure may print.
var internalText = regexp.MustCompile(`\.svc|cluster\.local|sha256:|30500|\b\d{1,3}(\.\d{1,3}){3}\b`)

func TestBuildPullFailureReadsInProductTerms(t *testing.T) {
	image := testRegistry + "/tea-ws/tea-ws-web@sha256:" + strings.Repeat("0123abcd", 8)
	manifest := "https://" + testRegistry + "/v2/tea-ws/tea-ws-web/manifests/sha256:" + strings.Repeat("0123abcd", 8)
	pulling := `Failed to pull image "` + image + `": rpc error: `
	const (
		gone = "image pull is failing: the build this release runs on (tea-ws/tea-ws-web) is no longer stored," +
			" so its image cannot be pulled; the pre-deploy command never ran." +
			" Only a service's recent builds are kept. Deploy the commit again to build it."
		unpulled = "image pull is failing: the platform could not pull this release's build (tea-ws/tea-ws-web); the pre-deploy command never ran"
	)
	for _, tc := range []struct {
		name    string
		kubelet string
		want    string
		// permanent says the pending-Job check ends the step after the grace
		// window; a transient error ends it only with the Job's deadline.
		permanent bool
	}{
		{"not found",
			pulling + `code = NotFound desc = failed to pull and unpack image "` + image + `": failed to resolve reference "` + image + `": ` + image + `: not found`,
			gone, true},
		{"unauthorized",
			pulling + `code = Unknown desc = failed to pull and unpack image "` + image + `": failed to resolve reference "` + image +
				`": unexpected status from HEAD request to ` + manifest + `: 401 Unauthorized`,
			unpulled, true},
		{"registry unreachable",
			pulling + `code = Unavailable desc = failed to pull and unpack image "` + image + `": failed to resolve reference "` + image +
				`": failed to do request: Head "https://127.0.0.1:30500/v2/tea-ws/tea-ws-web/manifests/sha256:0123abcd": dial tcp 127.0.0.1:30500: connect: connection refused`,
			unpulled, false},
	} {
		for _, hook := range []bool{false, true} {
			want := tc.want
			if hook {
				want = strings.ReplaceAll(want, "the pre-deploy command", "the initial deploy hook")
			}
			pod := func(reason string) *corev1.Pod {
				p := jobPod(jobUID, "tea-ws", corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reason, Message: tc.kubelet}})
				p.Status.ContainerStatuses[0].Image = image
				p.CreationTimestamp = metav1.NewTime(time.Now().Add(-2 * time.Minute))
				return p
			}
			job := func(reason string) *batchv1.Job {
				j := failedJob(reason)
				if reason == "" {
					j.Status.Conditions = nil
				}
				if hook {
					j.Labels = map[string]string{LabelStep: string(StepInitialDeployHook)}
				}
				return j
			}
			check := func(how, got string) {
				t.Helper()
				if got != want {
					t.Errorf("%s (hook %v) %s = %q, want %q", tc.name, hook, how, got, want)
				}
				if leak := internalText.FindString(got); leak != "" {
					t.Errorf("%s (hook %v) %s prints %q: %q", tc.name, hook, how, leak, got)
				}
				if strings.Contains(got, "The pull is retried") {
					t.Errorf("%s (hook %v) %s says an ended step retries: %q", tc.name, hook, how, got)
				}
			}

			// The Job failed, on its deadline, with its pod still waiting.
			for _, reason := range []string{"ErrImagePull", "ImagePullBackOff"} {
				failed := job(batchv1.JobReasonDeadlineExceeded)
				check("FailureMessage on "+reason, FailureMessage(context.Background(), fakeClient(failed, pod(reason)), failed, testBuilds))
			}
			// The Job is still pending, its pod past the grace window.
			pending := job("")
			got := PullFailure(context.Background(), fakeClient(pending, pod("ErrImagePull")), pending, time.Now(), testBuilds)
			if !tc.permanent {
				if got != "" {
					t.Errorf("%s (hook %v): a transient pull error ended the step early: %q", tc.name, hook, got)
				}
				continue
			}
			check("PullFailure", got)
		}
	}
}

// An image the user supplied keeps naming itself and the registry's answer,
// byte for byte, with the build recognizer in place.
func TestUserImagePullFailureKeepsTheRegistryAnswer(t *testing.T) {
	waiting := pullWaiting("ErrImagePull")
	want := "image pull is failing: " + waiting.Waiting.Message + "; the pre-deploy command never ran"

	failed := failedJob(batchv1.JobReasonDeadlineExceeded)
	if got := FailureMessage(context.Background(), fakeClient(failed, userImagePod(waiting)), failed, testBuilds); got != want {
		t.Errorf("FailureMessage = %q, want %q", got, want)
	}
	pending := failedJob("")
	pending.Status.Conditions = nil
	if got := PullFailure(context.Background(), fakeClient(pending, userImagePod(waiting)), pending, time.Now(), testBuilds); got != want {
		t.Errorf("PullFailure = %q, want %q", got, want)
	}
}

func userImagePod(state corev1.ContainerState) *corev1.Pod {
	p := jobPod(jobUID, "tea-ws", state)
	p.Status.ContainerStatuses[0].Image = "docker.io/x/y:nope"
	p.CreationTimestamp = metav1.NewTime(time.Now().Add(-2 * time.Minute))
	return p
}
