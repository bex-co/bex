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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/183 through a REAL apiserver (envtest): Render's API schema puts no length
// limit on a service's commands, and its pg_pinecone_render template inlines a
// 4,717-byte dockerCommand. The App CRD capped every command at 4096 bytes, so
// production's API server refused the template. The CRD now stores commands
// well past that, in the spec and in the status records that copy them.
var _ = Describe("App command length schema", func() {
	ctx := context.Background()

	It("stores commands longer than 4096 bytes in the spec and the status", func() {
		long := "echo " + strings.Repeat("x", 64<<10)
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "long-commands", Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Image:             "nginx:1.27",
				StartCommand:      long,
				BuildCommand:      long,
				PreDeployCommand:  long,
				InitialDeployHook: long,
				ReleaseConfig: &appv1alpha1.ReleaseConfigReference{Generation: 1, SourceGeneration: 1, Image: "nginx:1.27",
					Runtime: &appv1alpha1.ReleaseRuntimeSettings{PreDeployCommand: long}},
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, app)).To(Succeed()) })

		app.Status.PreDeploy = &appv1alpha1.PreDeployStatus{Job: "pre-long-commands-gen-1", Status: "Running", Command: long}
		app.Status.InitialDeployHook = &appv1alpha1.PreDeployStatus{Job: "hook-long-commands-gen-1", Status: "Running", Command: long}
		app.Status.ReleaseRollout = &appv1alpha1.ReleaseRolloutStatus{Generation: 1, StartedAt: metav1.NowMicro(), Command: long}
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())

		var stored appv1alpha1.App
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(app), &stored)).To(Succeed())
		for field, got := range map[string]string{
			"spec.startCommand":                           stored.Spec.StartCommand,
			"spec.buildCommand":                           stored.Spec.BuildCommand,
			"spec.preDeployCommand":                       stored.Spec.PreDeployCommand,
			"spec.initialDeployHook":                      stored.Spec.InitialDeployHook,
			"spec.releaseConfig.runtime.preDeployCommand": stored.Spec.ReleaseConfig.Runtime.PreDeployCommand,
			"status.preDeploy.command":                    stored.Status.PreDeploy.Command,
			"status.initialDeployHook.command":            stored.Status.InitialDeployHook.Command,
			"status.releaseRollout.command":               stored.Status.ReleaseRollout.Command,
		} {
			Expect(got).To(Equal(long), field)
		}
	})
})
