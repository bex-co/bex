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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// The codex-security round-18 custom-domain cardinality contract through a REAL
// apiserver (envtest): spec.hosts and spec.hostRedirects are capped at 100
// entries each (MaxItems/MaxProperties in lego/types/v1alpha1/app_types.go, the
// round-12 #3 routes/headers precedent), so an over-cap host set is rejected at
// admission even when bex-api's CUSTOM_DOMAIN_LIMIT gate is bypassed. Mirrors
// the KeyValue ipAllowList structural-schema spec.
var _ = Describe("App custom-domain cardinality schema", func() {
	ctx := context.Background()

	appWithHostSets := func(name string, hosts, redirects int) *unstructured.Unstructured {
		spec := map[string]any{}
		if hosts > 0 {
			hostList := make([]any, hosts)
			for i := range hostList {
				hostList[i] = fmt.Sprintf("h%d.example.com", i)
			}
			spec["hosts"] = hostList
		}
		if redirects > 0 {
			redirectMap := map[string]any{}
			for i := range redirects {
				redirectMap[fmt.Sprintf("r%d.example.com", i)] = "example.com"
			}
			spec["hostRedirects"] = redirectMap
		}
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "app.bex.co/v1alpha1",
			"kind":       "App",
			"metadata":   map[string]any{"name": name, "namespace": "default"},
			"spec":       spec,
		}}
	}

	It("rejects hosts/hostRedirects beyond the per-service cap and accepts the cap", func() {
		By("refusing 101 hosts at admission")
		Expect(k8sClient.Create(ctx, appWithHostSets("host-cap-over", 101, 0))).NotTo(Succeed(),
			"the MaxItems=100 schema must reject an over-cap hosts list")

		By("refusing 101 hostRedirects at admission")
		Expect(k8sClient.Create(ctx, appWithHostSets("redirect-cap-over", 0, 101))).NotTo(Succeed(),
			"the MaxProperties=100 schema must reject an over-cap hostRedirects map")

		By("accepting exactly 100 hosts and 100 redirects")
		atCap := appWithHostSets("host-cap-at", 100, 100)
		Expect(k8sClient.Create(ctx, atCap)).To(Succeed())
		Expect(k8sClient.Delete(ctx, atCap)).To(Succeed())
	})
})
