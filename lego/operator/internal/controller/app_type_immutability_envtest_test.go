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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("App type immutability admission", func() {
	types := []string{
		appv1alpha1.TypeWebService,
		appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker,
		appv1alpha1.TypeCronJob,
		appv1alpha1.TypeStaticSite,
	}

	It("rejects every service-kind transition before a child reconciler can see it", func() {
		for sourceIndex, source := range types {
			for targetIndex, target := range types {
				if source == target {
					continue
				}
				name := fmt.Sprintf("immutable-type-%d-%d", sourceIndex, targetIndex)
				spec := appv1alpha1.AppSpec{Type: source, Image: "nginx:1"}
				if source == appv1alpha1.TypeStaticSite {
					// A static_site cannot carry spec.image (its own admission
					// rule) — it sources from a repo.
					spec = appv1alpha1.AppSpec{Type: source, Repo: "https://github.com/bex-co/site", PublishPath: "dist"}
				}
				app := &appv1alpha1.App{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
					Spec:       spec,
				}
				Expect(k8sClient.Create(ctx, app)).To(Succeed())
				app.Spec.Type = target
				err := k8sClient.Update(ctx, app)
				Expect(err).To(HaveOccurred(), "transition %s -> %s", source, target)
				Expect(strings.ToLower(err.Error())).To(ContainSubstring("spec.type is immutable"))
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
			}
		}
	})

	It("allows the legacy empty spelling to normalize to semantic web_service", func() {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "legacy-web-type", Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Image: "nginx:1"},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		app.Spec.Type = appv1alpha1.TypeWebService
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		Expect(k8sClient.Delete(ctx, app)).To(Succeed())
	})
})
