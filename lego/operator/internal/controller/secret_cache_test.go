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
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// This is the production incident's missing regression pin. Static RBAC tests
// already prohibited cluster-wide Secret reads, but the KeyValue Owns(Secret)
// watch still asked the shared cache for a cluster-wide informer and stopped all
// controllers from starting. Run a real envtest manager as a restricted user:
// cluster-scoped access for CRs/workloads, Secret access only in "default".
var _ = Describe("Namespace-scoped Secret cache", func() {
	It("starts App and KeyValue watches without cluster-wide Secret permission", func() {
		admin := k8sClient
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		userName := "secret-cache-" + suffix
		clusterRoleName := "secret-cache-" + suffix
		clusterBindingName := clusterRoleName
		roleName := clusterRoleName
		roleBindingName := clusterRoleName

		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: clusterRoleName},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{appv1alpha1.GroupVersion.Group},
					Resources: []string{"apps", "apps/status", "apps/finalizers", "keyvalues"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{appsv1.GroupName},
					Resources: []string{"deployments"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{networkingv1.GroupName},
					Resources: []string{"ingresses", "networkpolicies"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{"traefik.io"},
					Resources: []string{"middlewares"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{batchv1.GroupName},
					// App reconciliation lists Jobs for cron-run projection and
					// finalizer cleanup. Grant the non-Secret workload permission
					// this fixture needs while keeping the assertion's cluster-wide
					// Secret denial intact.
					Resources: []string{"cronjobs", "jobs"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods", "services"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
			},
		}
		Expect(admin.Create(ctx, clusterRole)).To(Succeed())
		DeferCleanup(func() { _ = admin.Delete(context.Background(), clusterRole) })

		clusterBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: clusterBindingName},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     clusterRoleName,
			},
			Subjects: []rbacv1.Subject{{Kind: "User", Name: userName}},
		}
		Expect(admin.Create(ctx, clusterBinding)).To(Succeed())
		DeferCleanup(func() { _ = admin.Delete(context.Background(), clusterBinding) })

		secretRole := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: "default"},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			}},
		}
		Expect(admin.Create(ctx, secretRole)).To(Succeed())
		DeferCleanup(func() { _ = admin.Delete(context.Background(), secretRole) })

		secretBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: roleBindingName, Namespace: "default"},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "Role",
				Name:     roleName,
			},
			Subjects: []rbacv1.Subject{{Kind: "User", Name: userName}},
		}
		Expect(admin.Create(ctx, secretBinding)).To(Succeed())
		DeferCleanup(func() { _ = admin.Delete(context.Background(), secretBinding) })

		user, err := testEnv.AddUser(envtest.User{Name: userName, Groups: []string{"system:authenticated"}}, cfg)
		Expect(err).NotTo(HaveOccurred())
		restrictedClient, err := client.New(user.Config(), client.Options{Scheme: k8sClient.Scheme()})
		Expect(err).NotTo(HaveOccurred())

		By("proving the test identity cannot list Secrets cluster-wide")
		Expect(restrictedClient.List(ctx, &corev1.SecretList{})).To(MatchError(apierrors.IsForbidden,
			"restricted user should be forbidden from listing Secrets cluster-wide"))
		Expect(restrictedClient.List(ctx, &corev1.SecretList{}, client.InNamespace("default"))).To(Succeed())

		appName := "secret-cache-app-" + suffix
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: appName, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Image: "traefik/whoami", Port: 3000, Suspended: true,
			},
		}
		Expect(admin.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() {
			current := &appv1alpha1.App{}
			if admin.Get(context.Background(), types.NamespacedName{Name: appName, Namespace: "default"}, current) == nil {
				current.Finalizers = nil
				_ = admin.Update(context.Background(), current)
				_ = admin.Delete(context.Background(), current)
			}
		})

		kvName := "secret-cache-kv-" + suffix
		kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: kvName, Namespace: "default"}}
		Expect(admin.Create(ctx, kv)).To(Succeed())
		DeferCleanup(func() { _ = admin.Delete(context.Background(), kv) })
		Expect(admin.Get(ctx, types.NamespacedName{Name: kvName, Namespace: "default"}, kv)).To(Succeed())
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kvName,
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(kv, schema.GroupVersionKind{
					Group: appv1alpha1.GroupVersion.Group, Version: appv1alpha1.GroupVersion.Version, Kind: "KeyValue",
				})},
			},
		}
		Expect(admin.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = admin.Delete(context.Background(), secret) })

		skipNameValidation := true // another envtest spec also exercises the App controller
		mgr, err := ctrl.NewManager(user.Config(), ctrl.Options{
			Scheme:     k8sClient.Scheme(),
			Cache:      NamespacedSecretCacheOptions("default"),
			Metrics:    metricsserver.Options{BindAddress: "0"},
			Controller: controllerconfig.Controller{SkipNameValidation: &skipNameValidation},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect((&AppReconciler{
			Client: mgr.GetClient(), BuildClient: restrictedClient,
			Scheme: mgr.GetScheme(), Mode: ModeKubernetes,
		}).SetupWithManager(mgr)).To(Succeed())

		var kvReconciles atomic.Int32
		Expect(ctrl.NewControllerManagedBy(mgr).
			For(&appv1alpha1.KeyValue{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
			Owns(&corev1.Secret{}, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
			Named("secret-cache-keyvalue-" + suffix).
			Complete(reconcile.Func(func(_ context.Context, _ reconcile.Request) (reconcile.Result, error) {
				kvReconciles.Add(1)
				return reconcile.Result{}, nil
			}))).To(Succeed())

		mgrCtx, stopManager := context.WithCancel(ctx)
		managerDone := make(chan error, 1)
		go func() { managerDone <- mgr.Start(mgrCtx) }()
		DeferCleanup(func() {
			stopManager()
			Eventually(managerDone, 10*time.Second).Should(Receive(BeNil()))
		})

		By("starting the shared cache and reconciling an existing App")
		Eventually(func(g Gomega) {
			current := &appv1alpha1.App{}
			g.Expect(admin.Get(ctx, types.NamespacedName{Name: appName, Namespace: "default"}, current)).To(Succeed())
			g.Expect(current.Status.Phase).To(Equal(appv1alpha1.PhaseHibernated))
			g.Expect(current.Status.ObservedGeneration).To(Equal(current.Generation))
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
		Consistently(managerDone, 500*time.Millisecond).ShouldNot(Receive())

		By("observing KeyValue replay, then a Secret data update through the namespaced informer")
		Eventually(kvReconciles.Load, 10*time.Second, 100*time.Millisecond).Should(BeNumerically(">=", 1))
		baseline := kvReconciles.Load()
		Consistently(kvReconciles.Load, 300*time.Millisecond, 50*time.Millisecond).Should(Equal(baseline))
		Expect(admin.Get(ctx, types.NamespacedName{Name: kvName, Namespace: "default"}, secret)).To(Succeed())
		secret.Data = map[string][]byte{"password": []byte("changed")}
		Expect(admin.Update(ctx, secret)).To(Succeed())
		Eventually(kvReconciles.Load, 10*time.Second, 100*time.Millisecond).Should(BeNumerically(">", baseline))
	})
})
