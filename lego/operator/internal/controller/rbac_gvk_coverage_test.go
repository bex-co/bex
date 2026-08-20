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
	"os"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// RBAC coverage: every out-of-tree API the controllers reach for must be
// granted (w7/m77/t007).
//
// The sibling least-privilege guard checks that nothing is OVER-granted. Nothing
// checked the other direction, and the gap was not hypothetical: the ADR043 D8.4
// code to project a per-tenant Barman ObjectStore shipped without a
// +kubebuilder:rbac marker, so the generated ClusterRole omitted `objectstores`
// entirely. The failure is invisible in every test — envtest runs without RBAC —
// and invisible in review, because the marker's absence looks like nothing. It
// surfaced only when a Database created in a tenant namespace on production went
// straight to Failed with BackupStoreUnavailable.
//
// These GVKs are the honest boundary to guard: an out-of-tree kind reached
// through the dynamic client is exactly the case where the compiler cannot help,
// because there is no typed scheme entry whose absence would break the build.
var _ = Describe("RBAC covers every dynamically-addressed API (w7/m77)", func() {
	roleFile := filepath.Join("..", "..", "config", "rbac", "role.yaml")

	// gvkDecl matches `var xGVK = schema.GroupVersionKind{Group: "…", Version: "…", Kind: "…"}`.
	gvkDecl := regexp.MustCompile(
		`GVK\s*=\s*schema\.GroupVersionKind\{Group:\s*"([^"]*)",\s*Version:\s*"[^"]*",\s*Kind:\s*"([^"]+)"\}`)

	// declaredGVKs scans this package's own sources, so a kind added later is
	// picked up without anyone remembering to edit a list here.
	declaredGVKs := func() map[string]string {
		out := map[string]string{}
		entries, err := os.ReadDir(".")
		Expect(err).NotTo(HaveOccurred())
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			body, err := os.ReadFile(name)
			Expect(err).NotTo(HaveOccurred())
			for _, m := range gvkDecl.FindAllStringSubmatch(string(body), -1) {
				// Plural resource name. Every kind these controllers address
				// pluralizes by appending "s"; a future kind that does not
				// (…y → …ies, …s → …es) must be special-cased here, and the
				// coverage assertion below is what will say so.
				out[strings.ToLower(m[2])+"s"] = m[1]
			}
		}
		return out
	}

	It("grants every GroupVersionKind the controllers address dynamically", func() {
		gvks := declaredGVKs()
		// Anti-tautology: an empty scan would make every assertion below vacuous.
		Expect(gvks).NotTo(BeEmpty(), "the source scan found no GVK declarations — the regex has drifted")
		Expect(gvks).To(HaveKey("objectstores"), "the scan must see the ObjectStore this guard was written for")

		granted := map[string]map[string]bool{} // apiGroup -> resource -> true
		for _, cr := range parseClusterRoles(roleFile) {
			for _, rule := range cr.Rules {
				for _, group := range rule.APIGroups {
					if granted[group] == nil {
						granted[group] = map[string]bool{}
					}
					for _, res := range rule.Resources {
						granted[group][res] = true
					}
				}
			}
		}

		for resource, group := range gvks {
			Expect(granted[group]).To(SatisfyAny(HaveKey(resource), HaveKey("*")),
				"the operator ClusterRole grants nothing for %q in API group %q, but a controller addresses that kind "+
					"through the dynamic client. Add a +kubebuilder:rbac marker and re-run `make manifests` — "+
					"envtest runs without RBAC, so no other test can catch this.", resource, group)
		}
	})
})
