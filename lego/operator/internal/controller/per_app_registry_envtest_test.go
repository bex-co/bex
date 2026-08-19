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
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/identity"
	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// staticStatusTransport answers every registry probe with the given HTTP
// status, standing in for zot: 200 = credential loaded, 401 = stale (loaded
// only at zot startup — w9/m43). Togglable per test via the atomic.
type staticStatusTransport struct{ status *atomic.Int32 }

func (t staticStatusTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: int(t.status.Load()),
		Body:       http.NoBody,
		Header:     http.Header{},
	}, nil
}

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

	// newReconcilerAt wires a reconciler whose registry probe always answers
	// with the given (togglable) status; most specs pin 200 (credential
	// accepted) so the w9/m43 activation gate stays open.
	newReconcilerAt := func(status *atomic.Int32) *AppReconciler {
		return &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			Registry: registry_,
			PerAppRegistry: &registry.Creds{
				Client:       k8sClient,
				ZotNamespace: zotNamespace,
				HTPasswdName: "zot-htpasswd",
				ConfigName:   "zot-config",
				Registry:     registry_,
				HTTPClient:   &http.Client{Transport: staticStatusTransport{status: status}},
			},
		}
	}
	newReconciler := func() *AppReconciler {
		accepted := &atomic.Int32{}
		accepted.Store(http.StatusOK)
		return newReconcilerAt(accepted)
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
			reconcileN(r, nn, 3)
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
		var zotCfg map[string]any
		Expect(json.Unmarshal(cfgSec.Data["config.json"], &zotCfg)).To(Succeed())
		http, _ := zotCfg["http"].(map[string]any)
		ac, _ := http["accessControl"].(map[string]any)
		repos, _ := ac["repositories"].(map[string]any)
		Expect(repos).To(HaveKey(name))
		// Exact per-App rules shadow ** in Zot, so builder push access must be
		// granted by the global admin policy.
		adminPolicy, _ := ac["adminPolicy"].(map[string]any)
		adminUsers, _ := adminPolicy["users"].([]any)
		adminActions, _ := adminPolicy["actions"].([]any)
		Expect(adminUsers).To(ContainElement("bex-builder"))
		Expect(adminActions).To(ConsistOf("read", "create", "update", "delete"))
		// bex-puller must NOT be in ** wildcard.
		wildcard, _ := repos["**"].(map[string]any)
		policies, _ := wildcard["policies"].([]any)
		for _, p := range policies {
			pm, _ := p.(map[string]any)
			users, _ := pm["users"].([]any)
			for _, u := range users {
				Expect(u).NotTo(Equal("bex-puller"), "bex-puller must not appear in ** wildcard (w7/m36)")
			}
		}

		// Simulate the production config written before builder adminPolicy was
		// introduced. Reconciliation must migrate it even though this App's exact
		// repository entry already exists.
		delete(ac, "adminPolicy")
		historicalConfig, err := json.Marshal(zotCfg)
		Expect(err).NotTo(HaveOccurred())
		cfgSec.Data["config.json"] = historicalConfig
		Expect(k8sClient.Update(ctx, cfgSec)).To(Succeed())
		reconcileN(r, nn, 1)

		migratedCfgSec := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "zot-config", Namespace: zotNamespace}, migratedCfgSec)).To(Succeed())
		var migratedZotCfg map[string]any
		Expect(json.Unmarshal(migratedCfgSec.Data["config.json"], &migratedZotCfg)).To(Succeed())
		migratedHTTP, _ := migratedZotCfg["http"].(map[string]any)
		migratedAC, _ := migratedHTTP["accessControl"].(map[string]any)
		migratedAdmin, _ := migratedAC["adminPolicy"].(map[string]any)
		migratedUsers, _ := migratedAdmin["users"].([]any)
		migratedActions, _ := migratedAdmin["actions"].([]any)
		Expect(migratedUsers).To(ContainElement("bex-builder"))
		Expect(migratedActions).To(ConsistOf("read", "create", "update", "delete"))

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
		savedRegistry := r.Registry
		r.Registry = "" // registry manifest cleanup is covered by its own bounded finalizer tests
		reconcileN(r, nn, 2)
		r.Registry = savedRegistry

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
		var zotCfg map[string]any
		Expect(json.Unmarshal(cfgSec.Data["config.json"], &zotCfg)).To(Succeed())
		http, _ := zotCfg["http"].(map[string]any)
		ac, _ := http["accessControl"].(map[string]any)
		repos, _ := ac["repositories"].(map[string]any)
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

	It("gates the workload on credential activation and bounces zot when stale (w9/m43)", func() {
		const name = "per-app-cred-gate"
		status := &atomic.Int32{}
		status.Store(http.StatusUnauthorized) // zot hasn't loaded the credential
		r := newReconcilerAt(status)
		r.PerAppRegistry.ActivationGrace = time.Nanosecond // past grace on the second probe

		// A labeled zot pod for the bounce to find.
		zotPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "zot-0", Namespace: zotNamespace,
				Labels: map[string]string{registry.ZotPodLabelKey: registry.ZotPodLabelValue},
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "zot", Image: "zot:test"}}},
		}
		Expect(k8sClient.Create(ctx, zotPod)).To(Succeed())

		nn := types.NamespacedName{Name: name, Namespace: namespace}
		image := registry_ + "/" + name + ":gen-1"
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       appv1alpha1.AppSpec{Image: image, Port: 3000},
		})).To(Succeed())

		// While the credential is rejected: requeue, no Deployment rolled.
		reconcileN(r, nn, 3)
		Expect(k8sClient.Get(ctx, nn, &appsv1.Deployment{})).NotTo(Succeed(),
			"no workload may roll to an image the registry would refuse to serve")
		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseDeploying))

		// Past the grace the reconciler bounced the zot pod (envtest has no
		// StatefulSet controller, so the pod is simply gone/terminating).
		gone := &corev1.Pod{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "zot-0", Namespace: zotNamespace}, gone)
		Expect(err != nil || gone.DeletionTimestamp != nil).To(BeTrue(),
			"stale credential past grace must bounce the zot pod")

		// "Restarted" zot accepts the credential: the workload proceeds.
		status.Store(http.StatusOK)
		reconcileN(r, nn, 1)
		Expect(k8sClient.Get(ctx, nn, &appsv1.Deployment{})).To(Succeed())

		cleanupApp(r, name)
	})

	It("gates a source build before creating its Job when the push credential is stale", func() {
		const name = "per-app-build-cred-gate"
		status := &atomic.Int32{}
		status.Store(http.StatusUnauthorized)
		r := newReconcilerAt(status)
		r.PerAppRegistry.ActivationGrace = time.Hour

		nn := types.NamespacedName{Name: name, Namespace: namespace}
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/example", Branch: "main",
				Builder: "dockerfile", Port: 3000,
			},
		})).To(Succeed())

		// First pass adds the finalizer; the second mints/mirrors the credential
		// and must stop at activation instead of starting a doomed push-capable Job.
		reconcileN(r, nn, 2)
		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseBuilding))
		Expect(app.Status.Conditions).NotTo(BeEmpty())
		Expect(app.Status.Conditions[0].Reason).To(Equal("RegistryCredsPending"))

		jobs := &batchv1.JobList{}
		Expect(k8sClient.List(ctx, jobs, client.InNamespace(namespace))).To(Succeed())
		for i := range jobs.Items {
			Expect(jobs.Items[i].Name).NotTo(ContainSubstring(name),
				"no build Job may start before Zot accepts its push credential")
		}

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

	It("mints disjoint workspace-scoped credentials for same-named Apps", func() {
		const name = "collide"
		wsA := "tea-aaaaaaaaaaaaaaaaaaaa"
		wsB := "tea-bbbbbbbbbbbbbbbbbbbb"
		for _, ns := range []string{wsA, wsB} {
			_ = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		}
		r := newReconciler()
		idA := identity.ForApp(name, wsA)
		idB := identity.ForApp(name, wsB)
		for _, id := range []identity.Identity{idA, idB} {
			Expect(k8sClient.Create(ctx, &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: id.Workspace,
					Labels:    map[string]string{"app.bex.co/workspace": id.Workspace},
				},
				Spec: appv1alpha1.AppSpec{Image: registry_ + "/" + id.Repo() + ":gen-1", Port: 3000},
			})).To(Succeed())
			reconcileN(r, types.NamespacedName{Name: name, Namespace: id.Workspace}, 2)
		}
		Expect(idA.PullSecretName()).NotTo(Equal(idB.PullSecretName()))
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name: idA.PullSecretName(), Namespace: wsA,
		}, &corev1.Secret{})).To(Succeed())
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name: idB.PullSecretName(), Namespace: wsB,
		}, &corev1.Secret{})).To(Succeed())

		htSec := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "zot-htpasswd", Namespace: zotNamespace}, htSec)).To(Succeed())
		htpasswd := string(htSec.Data["htpasswd"])
		Expect(htpasswd).To(ContainSubstring(idA.ZotUsername() + ":"))
		Expect(htpasswd).To(ContainSubstring(idB.ZotUsername() + ":"))

		cfgSec := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "zot-config", Namespace: zotNamespace}, cfgSec)).To(Succeed())
		var zotCfg map[string]any
		Expect(json.Unmarshal(cfgSec.Data["config.json"], &zotCfg)).To(Succeed())
		http, _ := zotCfg["http"].(map[string]any)
		ac, _ := http["accessControl"].(map[string]any)
		repos, _ := ac["repositories"].(map[string]any)
		Expect(repos).To(HaveKey(idA.Repo()))
		Expect(repos).To(HaveKey(idB.Repo()))

		for _, id := range []identity.Identity{idA, idB} {
			nn := types.NamespacedName{Name: name, Namespace: id.Workspace}
			app := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())
			saved := r.Registry
			r.Registry = ""
			reconcileN(r, nn, 3)
			r.Registry = saved
		}
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
