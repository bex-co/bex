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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// startcommand_test.go pins w6/m21: spec.startCommand overrides the running
// container's Command whenever the image comes from an opaque Dockerfile build
// (or a prebuilt image) — the operator has no control over that image's own
// ENTRYPOINT/CMD, so effectiveBuilder gates the override on every non-native,
// non-buildpack build strategy (app_controller.go), not narrowly on
// runtime:docker. A native/buildpack build instead bakes the command into the
// image's own CMD at build time, so no Deployment-level override applies.
var _ = Describe("StartCommand override (kubernetes runtime)", func() {
	ctx := context.Background()

	DescribeTable("container Command reflects spec.startCommand for the effective build strategy",
		func(name string, spec appv1alpha1.AppSpec, wantCommand []string) {
			// k8sClient is only live once BeforeSuite has run, so the
			// reconciler is built here (inside the leaf node), not while the
			// spec tree itself is under construction.
			r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
			nn := types.NamespacedName{Name: name, Namespace: "default"}
			reconcileN := func(nn types.NamespacedName) {
				for range 3 {
					_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
					Expect(err).NotTo(HaveOccurred())
				}
			}
			spec.Image = "traefik/whoami"
			spec.Port = 3000
			app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}, Spec: spec}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcileN(nn)
			defer func() {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcileN(nn)
			}()

			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
			if wantCommand == nil {
				Expect(dep.Spec.Template.Spec.Containers[0].Command).To(BeEmpty())
			} else {
				Expect(dep.Spec.Template.Spec.Containers[0].Command).To(Equal(wantCommand))
			}
		},
		Entry("runtime docker, startCommand set — overrides",
			"docker-startcmd-app",
			appv1alpha1.AppSpec{Runtime: "docker", StartCommand: "bin/server --flag"},
			[]string{"/bin/sh", "-c", "bin/server --flag"}),
		Entry("runtime docker, startCommand unset — image default",
			"docker-nostartcmd-app",
			appv1alpha1.AppSpec{Runtime: "docker"},
			nil),
		Entry("legacy builder:dockerfile (no runtime), startCommand set — still overrides",
			"legacy-dockerfile-startcmd-app",
			appv1alpha1.AppSpec{Builder: "dockerfile", StartCommand: "bin/server --flag"},
			[]string{"/bin/sh", "-c", "bin/server --flag"}),
		Entry("native runtime, startCommand set — baked into the image's own CMD, no Deployment override",
			"native-startcmd-app",
			appv1alpha1.AppSpec{Runtime: "node", BuildCommand: "npm ci", StartCommand: "npm start"},
			nil),
	)
})
