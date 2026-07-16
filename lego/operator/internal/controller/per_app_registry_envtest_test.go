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
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("Per-App registry pull credentials (w7/m36)", func() {
	const (
		namespace    = "default"
		zotNamespace = "zot-test"
		registry_    = "zot.test:5000"
	)
	ctx := context.Background()

	// Seed the zot-htpasswd Secret (bex-builder entry, as in production).
	seedZotSecrets := func() {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: zotNamespace}}
		_ = k8sClient.Create(ctx, ns)

		htpasswd := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: zotNamespace},
			Data:       map[string][]byte{"htpasswd": []byte("bex-builder:$2y$10$fakehash\n")},
		}
		_ = k8sClient.Create(ctx, htpasswd)
	}

	newReconciler := func() *AppReconciler {
		return &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			Registry: registry_,
			PerAppRegistry: &registry.Creds{
				Client:       k8sClient,
				ZotNamespace: zotNamespace,
				HTPasswdName: "zot-htpasswd",
				ConfigName:   "zot-config",
				Registry:     registry_,
			},
		}
	}
	reconcileN := func(r *AppReconciler, nn types.NamespacedName, count int) {
		for range count {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	cleanupApp := func(r *AppReconciler, name string) {
		nn := types.NamespacedName{Name: name, Namespace: namespace}
		app := &appv1alpha1.App{}
		if k8sClient.Get(ctx, nn, app) == nil {
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())
			saved := r.Registry
			r.Registry = "" // skip registry calls during teardown
			reconcileN(r, nn, 1)
			r.Registry = saved
		}
		_ = k8sClient.Delete(ctx, &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}})
	}

	BeforeEach(seedZotSecrets)

	AfterEach(func() {
		// Clean up Zot Secrets between tests.
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "zot-htpasswd", Namespace: zotNamespace}})
		_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "zot-config", Namespace: zotNamespace}})
	})

	It("creates the per-App pull Secret and Zot entries on reconcile", func() {
		const name = "per-app-cred-create"
		r := newReconciler()
		nn := types.NamespacedName{Name: name, Namespace: namespace}
		image := registry_ + "/" + name + ":gen-1"
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       appv1alpha1.AppSpec{Image: image, Port: 3000},
		})).To(Succeed())
		reconcileN(r, nn, 2) // finalizer, then workload

		// Per-App pull Secret must exist.
		pullSec := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name: registry.PullSecretName(name), Namespace: namespace,
		}, pullSec)).To(Succeed())
		Expect(pullSec.Type).To(Equal(corev1.SecretTypeDockerConfigJson))
		// Docker config must reference the per-App Zot user.
		var cfg struct {
			Auths map[string]struct {
				Username string `json:"username"`
			} `json:"auths"`
		}
		Expect(json.Unmarshal(pullSec.Data[corev1.DockerConfigJsonKey], &cfg)).To(Succeed())
		Expect(cfg.Auths[registry_].Username).To(Equal(registry.ZotUsername(name)))

		// Deployment must reference the per-App pull Secret, not the shared one.
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		Expect(dep.Spec.Template.Spec.ImagePullSecrets).To(HaveLen(1))
		Expect(dep.Spec.Template.Spec.ImagePullSecrets[0].Name).To(Equal(registry.PullSecretName(name)))

		// zot-htpasswd must contain app-<name>.
		htSec := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "zot-htpasswd", Namespace: zotNamespace}, htSec)).To(Succeed())
		htpasswd := string(htSec.Data["htpasswd"])
		Expect(htpasswd).To(ContainSubstring(registry.ZotUsername(name) + ":"))
		// bex-builder original entry must be preserved.
		Expect(htpasswd).To(ContainSubstring("bex-builder:"))

		// zot-config must have per-App ACL entry.
		cfgSec := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "zot-config", Namespace: zotNamespace}, cfgSec)).To(Succeed())
		var zotCfg map[string]interface{}
		Expect(json.Unmarshal(cfgSec.Data["config.json"], &zotCfg)).To(Succeed())
		http, _ := zotCfg["http"].(map[string]interface{})
		ac, _ := http["accessControl"].(map[string]interface{})
		repos, _ := ac["repositories"].(map[string]interface{})
		Expect(repos).To(HaveKey(name))
		// bex-puller must NOT be in ** wildcard.
		wildcard, _ := repos["**"].(map[string]interface{})
		policies, _ := wildcard["policies"].([]interface{})
		for _, p := range policies {
			pm, _ := p.(map[string]interface{})
			users, _ := pm["users"].([]interface{})
			for _, u := range users {
				Expect(u).NotTo(Equal("bex-puller"), "bex-puller must not appear in ** wildcard (w7/m36)")
			}
		}

		cleanupApp(r, name)
	})

	It("revokes per-App credentials on App deletion", func() {
		const name = "per-app-cred-revoke"
		r := newReconciler()
		nn := types.NamespacedName{Name: name, Namespace: namespace}
		image := registry_ + "/" + name + ":gen-1"
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       appv1alpha1.AppSpec{Image: image, Port: 3000},
		})).To(Succeed())
		reconcileN(r, nn, 2)

		// Verify credential exists.
		pullSec := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name: registry.PullSecretName(name), Namespace: namespace,
		}, pullSec)).To(Succeed())
		htSec := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "zot-htpasswd", Namespace: zotNamespace}, htSec)).To(Succeed())
		Expect(string(htSec.Data["htpasswd"])).To(ContainSubstring(registry.ZotUsername(name) + ":"))

		// Delete the App and reconcile the finalizer.
		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(k8sClient.Delete(ctx, app)).To(Succeed())
		reconcileN(r, nn, 1)

		// per-App pull Secret must be deleted.
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name: registry.PullSecretName(name), Namespace: namespace,
		}, &corev1.Secret{})).NotTo(Succeed())

		// htpasswd entry must be revoked.
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "zot-htpasswd", Namespace: zotNamespace}, htSec)).To(Succeed())
		Expect(string(htSec.Data["htpasswd"])).NotTo(ContainSubstring(registry.ZotUsername(name) + ":"))
		// bex-builder must remain after revocation.
		Expect(string(htSec.Data["htpasswd"])).To(ContainSubstring("bex-builder:"))

		// zot-config ACL entry must be removed.
		cfgSec := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "zot-config", Namespace: zotNamespace}, cfgSec)).To(Succeed())
		var zotCfg map[string]interface{}
		Expect(json.Unmarshal(cfgSec.Data["config.json"], &zotCfg)).To(Succeed())
		http, _ := zotCfg["http"].(map[string]interface{})
		ac, _ := http["accessControl"].(map[string]interface{})
		repos, _ := ac["repositories"].(map[string]interface{})
		Expect(repos).NotTo(HaveKey(name), "per-App ACL entry must be removed on delete")
	})

	It("is idempotent — repeated reconciles do not duplicate entries", func() {
		const name = "per-app-cred-idempotent"
		r := newReconciler()
		nn := types.NamespacedName{Name: name, Namespace: namespace}
		image := registry_ + "/" + name + ":gen-1"
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       appv1alpha1.AppSpec{Image: image, Port: 3000},
		})).To(Succeed())
		reconcileN(r, nn, 4) // multiple reconciles must not duplicate

		htSec := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "zot-htpasswd", Namespace: zotNamespace}, htSec)).To(Succeed())
		htpasswd := string(htSec.Data["htpasswd"])
		user := registry.ZotUsername(name)
		prefix := user + ":"
		var count int
		for _, line := range splitLines(htpasswd) {
			if strings.HasPrefix(line, prefix) {
				count++
			}
		}
		Expect(count).To(Equal(1), "htpasswd must have exactly one entry per App user")

		cleanupApp(r, name)
	})

	It("falls back to shared RegistryPullSecret when PerAppRegistry is nil", func() {
		const name = "per-app-fallback"
		r := &AppReconciler{
			Client:             k8sClient,
			Scheme:             k8sClient.Scheme(),
			Mode:               ModeKubernetes,
			Registry:           registry_,
			RegistryPullSecret: "bex-registry-pull",
			// PerAppRegistry is nil — shared credential path.
		}
		nn := types.NamespacedName{Name: name, Namespace: namespace}
		image := registry_ + "/" + name + ":gen-1"
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       appv1alpha1.AppSpec{Image: image, Port: 3000},
		})).To(Succeed())
		reconcileN(r, nn, 2)

		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		Expect(dep.Spec.Template.Spec.ImagePullSecrets).To(HaveLen(1))
		Expect(dep.Spec.Template.Spec.ImagePullSecrets[0].Name).To(Equal("bex-registry-pull"))

		// No per-App pull Secret should be created.
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name: registry.PullSecretName(name), Namespace: namespace,
		}, &corev1.Secret{})).NotTo(Succeed())

		cleanupApp(r, name)
	})
})

func splitLines(s string) []string {
	var out []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
