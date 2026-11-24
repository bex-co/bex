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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The API server is the judge of the image-volume form (w6/m624): it must
// accept a file subPath out of an image volume, read-only, and store the
// template exactly as projected. A default the projection does not write
// would make every pass write the Deployment again, and a refusal would
// leave the App with no instance.
var _ = Describe("Health-probe helper from an image volume (w6/m624)", func() {
	const namespace = "default"
	ctx := context.Background()

	It("is stored as projected, with no init container, and the next pass does not write", func() {
		const name = "helper-image-volume"
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: appv1alpha1.AppSpec{
				Image: "ghcr.io/example/app:v1", Port: 3000, HealthCheckPath: "/healthz", Tier: "starter",
				Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 1},
			},
		}
		params := deploymentParams{
			image: app.Spec.Image, port: 3000, replicas: 1,
			healthProbeImage:       "ghcr.io/bex-co/bex-operator@sha256:" + strings.Repeat("1", 64),
			healthProbeImageVolume: true,
		}
		projected := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		applyDeploymentSpec(projected, app, params)
		Expect(projected.Spec.Template.Spec.InitContainers).To(BeEmpty())
		Expect(k8sClient.Create(ctx, projected.DeepCopy())).To(Succeed())
		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}})
		})

		nn := types.NamespacedName{Name: name, Namespace: namespace}
		stored := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, stored)).To(Succeed())
		Expect(stored.Spec.Template).To(Equal(projected.Spec.Template),
			"the API server must store the projection unchanged, or every pass writes it again")
		Expect(storedHealthProbeImageVolume(stored.Spec.Template.Spec)).To(Equal(&corev1.ImageVolumeSource{
			Reference: params.healthProbeImage, PullPolicy: corev1.PullIfNotPresent,
		}))

		again := stored.DeepCopy()
		applyDeploymentSpec(again, app, params)
		Expect(k8sClient.Update(ctx, again)).To(Succeed())
		after := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, after)).To(Succeed())
		Expect(after.ResourceVersion).To(Equal(stored.ResourceVersion), "an unchanged App must not write its Deployment")
	})
})
