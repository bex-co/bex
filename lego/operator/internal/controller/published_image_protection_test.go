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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/682: bex-api closes a deploy row live on the status that names its
// release active, so the registry round trips that protect a built image from
// retention must come after that status is stored, not before it. On
// production they held a git-built release's row in progress 2–4 s after its
// new body answered. Each case records what the stored App said when the
// registry received the live tag.
func TestPublishedReleaseIsStoredBeforeItsImageIsTagged(t *testing.T) {
	for _, tc := range []struct {
		name    string
		publish func(context.Context, *AppReconciler, *appv1alpha1.App, *appsv1.Deployment) error
	}{
		{"rollout settling", func(ctx context.Context, r *AppReconciler, app *appv1alpha1.App, dep *appsv1.Deployment) error {
			_, err := r.reportRolloutProgress(ctx, app, dep, 1, 3000, "waiting")
			return err
		}},
		{"running", func(ctx context.Context, r *AppReconciler, app *appv1alpha1.App, dep *appsv1.Deployment) error {
			_, err := r.settleReady(ctx, app, dep, "Deployed", "1/1 replicas ready")
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			reg := &fakeLiveRegistry{digest: digestA}
			var cl client.Client
			var activeAtTag []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodPut {
					var stored appv1alpha1.App
					if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "web"}, &stored); err != nil {
						t.Errorf("read the stored App at tag time: %v", err)
					}
					activeAtTag = append(activeAtTag, stored.Status.ActiveRevision)
				}
				reg.ServeHTTP(w, req)
			}))
			defer srv.Close()
			host := strings.TrimPrefix(srv.URL, "http://")

			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 3},
				Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseDeploying, ReleaseGeneration: 3, ActiveRevision: "rev-2",
					ImageDigest: &appv1alpha1.ImageDigestStatus{Generation: 2, Digest: digestB}, ProtectedImageTag: "live-2"},
			}
			reg.repo = appIdentity(app).Repo()
			app.Status.Image = host + "/" + reg.repo + ":gen-3@" + digestA
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-0", Namespace: "default", Labels: map[string]string{"app": "web", labelRevision: "rev-3"}}}
			pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
			cl = fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).
				WithObjects(app, pod).WithStatusSubresource(&appv1alpha1.App{}).Build()
			r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, Registry: host}

			dep := progressDeadlineDep(app.Name)
			dep.Spec.Template.Labels[labelRevision] = "rev-3"
			dep.Status.Conditions = nil
			if err := tc.publish(ctx, r, app, dep); err != nil {
				t.Fatal(err)
			}
			if len(activeAtTag) != 1 || activeAtTag[0] != "rev-3" {
				t.Fatalf("stored activeRevision when live-3 was tagged = %q, want [rev-3]: the release must be published first", activeAtTag)
			}
			var stored appv1alpha1.App
			if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
				t.Fatal(err)
			}
			if stored.Status.ProtectedImageTag != "live-3" {
				t.Errorf("stored protectedImageTag = %q, want live-3 written after the tag", stored.Status.ProtectedImageTag)
			}
		})
	}
}
