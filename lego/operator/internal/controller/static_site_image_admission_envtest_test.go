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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A static_site is built from its repo and published to the object-store
// origin — there is no prebuilt-image path (docs/ADR029-static-sites.md;
// Render parity, w8/m32). bex-api refuses the combination as a 400, but a
// direct CR write (kubectl, a Blueprint apply, a future controller) reaches
// the API server without passing through bex-api, so the CRD rule is the
// backstop.
var _ = Describe("Static site image admission", func() {
	It("rejects a static_site App carrying a prebuilt image", func() {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "static-image-guard", Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Type:        appv1alpha1.TypeStaticSite,
				Image:       "nginx:1",
				PublishPath: "dist",
			},
		}
		err := k8sClient.Create(ctx, app)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.image is not supported"))
	})

	It("still accepts a repo-built static_site and image-backed non-static types", func() {
		staticApp := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "static-repo-ok", Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Type:        appv1alpha1.TypeStaticSite,
				Repo:        "https://github.com/bex-co/site",
				PublishPath: "dist",
			},
		}
		Expect(k8sClient.Create(ctx, staticApp)).To(Succeed())
		Expect(k8sClient.Delete(ctx, staticApp)).To(Succeed())

		for index, svcType := range []string{
			appv1alpha1.TypeWebService,
			appv1alpha1.TypePrivateService,
			appv1alpha1.TypeBackgroundWorker,
			appv1alpha1.TypeCronJob,
		} {
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("image-source-ok-%d", index), Namespace: "default"},
				Spec:       appv1alpha1.AppSpec{Type: svcType, Image: "nginx:1"},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed(), "type %s must keep its image source", svcType)
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())
		}
	})
})
