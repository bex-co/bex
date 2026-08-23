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
	"bytes"
	"os"
	"path/filepath"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	sigsyaml "sigs.k8s.io/yaml"
)

// parseClusterRoles reads a (possibly multi-document) YAML file and returns all ClusterRole objects.
func parseClusterRoles(path string) []rbacv1.ClusterRole {
	raw, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred(), "reading %s", path)

	var roles []rbacv1.ClusterRole
	for doc := range bytes.SplitSeq(raw, []byte("---")) {
		doc = bytes.TrimSpace(doc)
		if len(doc) == 0 {
			continue
		}
		var cr rbacv1.ClusterRole
		if err := sigsyaml.Unmarshal(doc, &cr); err != nil {
			continue
		}
		if cr.Kind == "ClusterRole" {
			roles = append(roles, cr)
		}
	}
	return roles
}

// parseRoles reads a (possibly multi-document) YAML file and returns all Role objects.
func parseRoles(path string) []rbacv1.Role {
	raw, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred(), "reading %s", path)

	var roles []rbacv1.Role
	for doc := range bytes.SplitSeq(raw, []byte("---")) {
		doc = bytes.TrimSpace(doc)
		if len(doc) == 0 {
			continue
		}
		var r rbacv1.Role
		if err := sigsyaml.Unmarshal(doc, &r); err != nil {
			continue
		}
		if r.Kind == "Role" {
			roles = append(roles, r)
		}
	}
	return roles
}

// clusterRoleHasSecretsRead returns true if any rule grants get/list/watch/* on secrets.
func clusterRoleHasSecretsRead(cr rbacv1.ClusterRole) bool {
	for _, rule := range cr.Rules {
		if !ruleCoversSecrets(rule) {
			continue
		}
		for _, v := range rule.Verbs {
			if v == "get" || v == "list" || v == "watch" || v == "*" {
				return true
			}
		}
	}
	return false
}

func ruleCoversSecrets(rule rbacv1.PolicyRule) bool {
	for _, r := range rule.Resources {
		if r == "secrets" || r == "*" {
			return true
		}
	}
	return false
}

var _ = Describe("RBAC least-privilege invariants", func() {
	// Paths relative to this file's directory (lego/operator/internal/controller/).
	operatorClusterRole := filepath.Join("..", "..", "config", "rbac", "role.yaml")
	apiClusterRole := filepath.Join("..", "..", "config", "api", "rbac.yaml")
	operatorAppsRole := filepath.Join("..", "..", "..", "..", "deploy", "gitops", "base", "operator-apps-rbac.yaml")

	It("operator ClusterRole (manager-role) has no cluster-wide secrets read", func() {
		for _, cr := range parseClusterRoles(operatorClusterRole) {
			Expect(clusterRoleHasSecretsRead(cr)).To(BeFalse(),
				"ClusterRole %q must not grant cluster-wide secrets read — scope it to the apps namespace Role in deploy/gitops/base/operator-apps-rbac.yaml",
				cr.Name)
		}
	})

	It("operator ClusterRole can read ReplicaSets for rollout diagnosis", func() {
		var verbs []string
		for _, cr := range parseClusterRoles(operatorClusterRole) {
			for _, rule := range cr.Rules {
				if !slices.Contains(rule.APIGroups, "apps") || !slices.Contains(rule.Resources, "replicasets") {
					continue
				}
				verbs = append(verbs, rule.Verbs...)
			}
		}
		Expect(verbs).To(ContainElements("get", "list", "watch"),
			"rolloutQuotaBlockMessage lists ReplicaSets through the cluster-wide manager cache")
		for _, verb := range []string{"create", "update", "patch", "delete", "*"} {
			Expect(verbs).NotTo(ContainElement(verb),
				"the operator only diagnoses ReplicaSet status; the Deployment controller owns ReplicaSet mutation")
		}
	})

	It("bex-api ClusterRole (api-role) has no cluster-wide secrets read", func() {
		for _, cr := range parseClusterRoles(apiClusterRole) {
			Expect(clusterRoleHasSecretsRead(cr)).To(BeFalse(),
				"ClusterRole %q must not grant cluster-wide secrets read — scope it to the apps namespace Role in deploy/gitops/base/bex-api-apps-rbac.yaml",
				cr.Name)
		}
	})

	It("operator-apps Role covers full secrets CRUD for tenant-namespace operations", func() {
		roles := parseRoles(operatorAppsRole)
		Expect(roles).To(HaveLen(1), "operator-apps-rbac.yaml should define exactly one Role")

		var secretsVerbs []string
		for _, rule := range roles[0].Rules {
			if ruleCoversSecrets(rule) {
				secretsVerbs = append(secretsVerbs, rule.Verbs...)
			}
		}
		Expect(secretsVerbs).To(ContainElements("get", "list", "watch", "create", "update", "patch", "delete"),
			"operator-apps Role must grant full secrets CRUD for keyvalue + database + clone-secret operations")
	})

	It("bex-api ClusterRole covers keyvalues (bex-api manages KeyValue CRs)", func() {
		var hasKeyvalues bool
		for _, cr := range parseClusterRoles(apiClusterRole) {
			for _, rule := range cr.Rules {
				for _, res := range rule.Resources {
					if res == "keyvalues" {
						hasKeyvalues = true
					}
				}
			}
		}
		Expect(hasKeyvalues).To(BeTrue(),
			"api-role ClusterRole must grant keyvalues access — bex-api/internal/keyvalue/service.go uses List/Get/Create/Delete")
	})
})
