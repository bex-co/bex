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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/044 (with w32/043): bex-api's container policy migration saves
// render-v1 on a service whose only build failed, for its next deploy, the
// way Render's Update Service saves a setting. The save builds nothing and the
// service stays Failed; the next deploy builds under the policy. An App
// created before bex-api stamped the first release carries no release stamp
// at all, as the production service w32/043 found does.
var _ = Describe("A saved container policy on a service whose first build failed (w32/044)", func() {
	for _, stamped := range []bool{true, false} {
		It(fmt.Sprintf("builds nothing until the next deploy (first release stamped: %t)", stamped), func() {
			name := fmt.Sprintf("saved-policy-failed-build-%t", stamped)
			nn := types.NamespacedName{Name: name, Namespace: "default"}
			r := &AppReconciler{
				Client: k8sClient, Scheme: k8sClient.Scheme(),
				Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: "default",
			}
			// Three passes settle each step; one may report the build failure
			// it settles.
			settle := func() {
				GinkgoHelper()
				for range 3 {
					_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				}
			}
			buildExists := func(gen int64) bool {
				_, err := buildJobFor(name, appv1alpha1.BuildRevision(gen))
				return err == nil
			}
			var saved, next int64
			DeferCleanup(func() {
				if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
					r.Registry = "" // no registry server in this suite
					deleteOwner(ctx, app, func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) })
				}
				for _, gen := range []int64{1, saved, next} {
					_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, appv1alpha1.BuildRevision(gen)), Namespace: "default"}})
				}
			})

			annotations := map[string]string{}
			if stamped {
				annotations[appv1alpha1.AnnotationReleaseGeneration] = "1"
			}
			Expect(k8sClient.Create(ctx, &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Annotations: annotations},
				Spec: appv1alpha1.AppSpec{
					Type: appv1alpha1.TypeWebService, Repo: "https://github.com/bex-co/hello", Branch: "main",
					Builder: "dockerfile", Port: 3000, Replicas: 1,
				},
			})).To(Succeed())
			settle()
			Expect(buildExists(1)).To(BeTrue(), "the first release builds")
			failBuildJob(name, appv1alpha1.BuildRevision(1))
			settle()
			failed := envtestApp(nn)
			Expect(failed.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
			Expect(failed.Status.ActiveRevision).To(BeEmpty())

			By("saving the policy as the migration does")
			app := envtestApp(nn)
			app.Spec.ContainerPolicy = appv1alpha1.ContainerPolicyRenderV1
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationPendingSourceGeneration, fmt.Sprint(app.Generation+1))
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			saved = envtestApp(nn).Generation
			settle()
			Expect(buildExists(saved)).To(BeFalse(), "a saved policy builds nothing")
			Expect(envtestApp(nn).Status.Phase).To(Equal(appv1alpha1.PhaseFailed), "and the service stays as it was")
			err := k8sClient.Get(ctx, nn, &appsv1.Deployment{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "nothing runs: %v", err)

			By("deploying: the next release builds")
			app = envtestApp(nn)
			app.Spec.RestartedAt = "next-deploy"
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, fmt.Sprint(app.Generation+1))
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			next = envtestApp(nn).Generation
			settle()
			Expect(buildExists(next)).To(BeTrue(), "the next deploy builds")
		})
	}
})
