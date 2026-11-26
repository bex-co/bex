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

package controller

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// When bex's image metadata read passed but the node's kubelet then fails the
// pull, the rollout's stall text and its settled failure name the Render cause
// the kubelet's message carries, worded from the same templates as a refused
// read, and keep the ImagePullBackOff reason bex-api closes the deploy with as
// image_pull_failed (w4/m669). A pod with a pull secret pulls with the
// service's credential; a message that names no cause keeps the kubelet's line.
func TestAKubeletPullFailureNamesRendersCause(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	const (
		image    = "docker.io/acme/private:v1"
		revision = "rev-3"
		notFound = `Back-off pulling image "docker.io/acme/private:v1": ErrImagePull: rpc error: code = NotFound desc = failed to pull and unpack image "docker.io/acme/private:v1": failed to resolve reference "docker.io/acme/private:v1": docker.io/acme/private:v1: not found`
		refused  = `rpc error: code = Unknown desc = failed to pull and unpack image "docker.io/acme/private:v1": failed to resolve reference "docker.io/acme/private:v1": failed to authorize: failed to fetch oauth token: unexpected status from POST request to https://auth.docker.io/token: 401 Unauthorized`
		bare     = `Back-off pulling image "docker.io/acme/private:v1"`
	)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{labelRevision: revision}}},
		},
	}
	param := func(name, value string) appv1alpha1.DeployReasonParam {
		return appv1alpha1.DeployReasonParam{Name: name, Value: value}
	}
	for _, tc := range []struct {
		name      string
		reason    string
		kubelet   string
		secret    bool
		want      string
		permanent bool
	}{
		{"a missing tag", imagePullBackOff, notFound, false,
			appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageNotFound, param("image", image)), true},
		{"a refused credential", "ErrImagePull", refused, true,
			appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageCredential, param("host", "docker.io"), param("image", image), param("status", "401")), true},
		{"a refusal without a credential", "ErrImagePull", refused, false,
			appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageAccess, param("image", image), param("host", "docker.io")), true},
		{"a throttled pull", "ErrImagePull", "unexpected status from HEAD request: 429 Too Many Requests", false,
			appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageRegistryRateLimited, param("host", "docker.io"), param("image", image)), false},
		{"a registry down", "ErrImagePull", "unexpected status from HEAD request: 502 Bad Gateway", false,
			appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageRegistryUnavailable, param("host", "docker.io"), param("image", image)), false},
		{"no cause named", imagePullBackOff, bare, false,
			appv1alpha1.ImagePullStallWaiting + ": " + image + ": " + bare, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "web-1", Namespace: "default", Labels: map[string]string{"app": "web", labelRevision: revision},
					CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
				},
				Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
					Name: appContainerName, Image: image,
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: tc.reason, Message: tc.kubelet}},
				}}},
			}
			if tc.secret {
				pod.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "web-registry-rgc-0123456789abcdefghij"}}
			}
			r := &AppReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build()}

			reason, msg := r.stuckPodMessage(context.Background(), dep, 0)
			if reason != imagePullBackOff || msg != tc.want {
				t.Errorf("stall = (%q, %q), want (%q, %q)", reason, msg, imagePullBackOff, tc.want)
			}
			reason, msg = r.permanentRolloutPullFailure(context.Background(), dep, time.Now())
			switch {
			case tc.permanent && (reason != imagePullBackOff || msg != tc.want):
				t.Errorf("settled = (%q, %q), want (%q, %q)", reason, msg, imagePullBackOff, tc.want)
			case !tc.permanent && msg != "":
				t.Errorf("a pull that may recover settled the rollout: %q", msg)
			}
			// The settled verdict reads the same once restated as the outcome.
			if tc.permanent && terminalStallMessage(imagePullBackOff, tc.want) != tc.want {
				t.Errorf("restated = %q, want it unchanged", terminalStallMessage(imagePullBackOff, tc.want))
			}
		})
	}
}
