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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/build"
)

// This suite pins ADR060 D2 against a REAL API server, which the unit tests
// structurally cannot do: they assemble the podFailurePolicy struct in memory
// and never submit it. A policy that violates the batch/v1 schema — an invalid
// rule combination, an out-of-range exit code, a field the server prunes —
// builds and unit-tests perfectly and then fails at Job creation, which in
// production means every build stops dispatching.
//
// So the assertion is round-trip survival: the server accepted it, stored it,
// and gave it back unchanged.
var _ = Describe("build Job failure policy", func() {
	const ns = "default"

	It("is accepted by the API server and survives a round trip intact", func() {
		ctx := context.Background()
		job := build.BuildJob(build.Options{
			Name:      "policy-probe",
			Namespace: ns,
			Revision:  "gen-1",
			Repo:      "https://github.com/example/repo",
			Registry:  "zot.bex-registry.svc:5000",
		}, "zot.bex-registry.svc:5000/policy-probe:gen-1")

		Expect(k8sClient.Create(ctx, job)).To(Succeed(),
			"the API server rejected the build Job; a malformed podFailurePolicy stops every build from dispatching")
		DeferCleanup(func() {
			_ = k8sClient.Delete(ctx, job)
		})

		var stored batchv1.Job
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(job), &stored)).To(Succeed())

		Expect(stored.Spec.PodFailurePolicy).NotTo(BeNil(),
			"the server pruned podFailurePolicy — disruptions would consume the tenant's retry budget again")
		Expect(stored.Spec.BackoffLimit).NotTo(BeNil())
		Expect(*stored.Spec.BackoffLimit).To(BeEquivalentTo(2))

		var ignoresDisruption, failsTenantExit bool
		for _, r := range stored.Spec.PodFailurePolicy.Rules {
			for _, c := range r.OnPodConditions {
				if c.Type == corev1.DisruptionTarget && r.Action == batchv1.PodFailurePolicyActionIgnore {
					ignoresDisruption = true
				}
			}
			if r.OnExitCodes != nil && r.Action == batchv1.PodFailurePolicyActionFailJob {
				for _, v := range r.OnExitCodes.Values {
					if v == build.ExitTenantError {
						failsTenantExit = true
					}
				}
			}
		}
		Expect(ignoresDisruption).To(BeTrue(), "the stored policy lost its DisruptionTarget Ignore rule")
		Expect(failsTenantExit).To(BeTrue(), "the stored policy lost its FailJob-on-tenant-error rule")

		// The pin that used to force this: eviction is now absorbed, not prevented.
		Expect(stored.Spec.Template.Annotations).NotTo(HaveKey("cluster-autoscaler.kubernetes.io/safe-to-evict"))
	})
})
