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
	"maps"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m129 end to end through the reconciler: Render's django example declares
// DATABASE_URL fromDatabase and runs `manage.py migrate` in its build command.
// The build waits for the connection Secret, then receives the value through
// its env Secret mount only, gets datastore reach into its own workspace while
// it runs, and loses it when it ends.
var _ = Describe("Build reach to the workspace's own datastores (w8/m129)", func() {
	ctx := context.Background()
	const (
		name    = "srv-m129-django"
		ws      = "tea-m129m129m129m129m129"
		buildNS = "m129-builds"
		secret  = "dpg-m129-app"
		uri     = "postgresql://mysite:m129-pw-literal@dpg-m129-rw.tea-m129m129m129m129m129.svc:5432/mysite"
	)
	nn := types.NamespacedName{Name: name, Namespace: ws}

	It("waits for the connection Secret, mounts DATABASE_URL, and scopes reach to the build", func() {
		for _, ns := range []string{ws, buildNS} {
			_ = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ws, Labels: map[string]string{labelWorkspace: ws}},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/render-examples/django", Branch: "main",
				Builder: "dockerfile", Port: 3000,
				Env: []appv1alpha1.EnvVar{{Name: "DATABASE_URL", ValueFrom: &appv1alpha1.EnvVarSource{
					SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: secret, Key: "uri"}}}},
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: buildNS}
		conn := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secret, Namespace: ws},
			StringData: map[string]string{"uri": uri}}
		DeferCleanup(func() {
			a := &appv1alpha1.App{}
			if k8sClient.Get(ctx, nn, a) == nil {
				Expect(k8sClient.Delete(ctx, a)).To(Succeed())
				r.Registry = ""
				for range 3 {
					_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				}
			}
			_ = k8sClient.Delete(ctx, conn)
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, "gen-1"), Namespace: buildNS}})
		})
		reconcileN := func(n int) {
			for range n {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				Expect(err).NotTo(HaveOccurred())
			}
		}
		jobKey := client.ObjectKey{Namespace: buildNS, Name: build.JobName(name, "gen-1")}
		egressKey := client.ObjectKey{Namespace: buildNS, Name: buildEgressPolicyName(name)}
		ingressKey := client.ObjectKey{Namespace: ws, Name: buildIngressPolicyName(name)}

		By("holding the build while the database's connection Secret does not exist yet")
		reconcileN(3)
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, jobKey, &batchv1.Job{}))).To(BeTrue(),
			"no build may start without DATABASE_URL — its migrate would hit localhost")
		held := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, held)).To(Succeed())
		Expect(held.Status.Phase).To(Equal(appv1alpha1.PhaseBuilding))
		ready := meta.FindStatusCondition(held.Status.Conditions, appv1alpha1.ConditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Reason).To(Equal(reasonBuildQueued))
		Expect(ready.Message).NotTo(ContainSubstring(secret), "the wait message names no internal Secret")

		By("dispatching once CNPG has written the Secret")
		Expect(k8sClient.Create(ctx, conn)).To(Succeed())
		reconcileN(2)
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, jobKey, job)).To(Succeed())
		raw, err := json.Marshal(job)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(raw)).NotTo(ContainSubstring("m129-pw-literal"), "the connection string must never be a Job-spec literal")
		var mounted bool
		for _, v := range job.Spec.Template.Spec.Volumes {
			mounted = mounted || (v.Secret != nil && v.Secret.SecretName == nativeEnvSecretName(name))
		}
		Expect(mounted).To(BeTrue(), "the build mounts the merged env Secret")
		merged := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: nativeEnvSecretName(name)}, merged)).To(Succeed())
		Expect(string(merged.Data["DATABASE_URL"])).To(Equal(uri))

		By("granting reach only between this build's pods and its own workspace namespace")
		egress, ingress := &networkingv1.NetworkPolicy{}, &networkingv1.NetworkPolicy{}
		Expect(k8sClient.Get(ctx, egressKey, egress)).To(Succeed())
		Expect(k8sClient.Get(ctx, ingressKey, ingress)).To(Succeed())
		Expect(job.Spec.Template.Labels).To(Equal(mergeLabels(job.Spec.Template.Labels, egress.Spec.PodSelector.MatchLabels)),
			"the egress policy must select the build Job's own pods")
		Expect(egress.Spec.Egress).To(HaveLen(1))
		Expect(egress.Spec.Egress[0].To).To(HaveLen(1))
		Expect(egress.Spec.Egress[0].To[0].NamespaceSelector.MatchLabels).To(Equal(map[string]string{corev1.LabelMetadataName: ws}))
		Expect(ingress.Spec.Ingress[0].From[0].NamespaceSelector.MatchLabels).To(Equal(map[string]string{corev1.LabelMetadataName: buildNS}))
		Expect(ingress.Spec.Ingress[0].From[0].PodSelector.MatchLabels).To(Equal(egress.Spec.PodSelector.MatchLabels))

		By("revoking both halves when the build ends")
		completeEnvtestJob(job, metav1.Now())
		reconcileN(2)
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, egressKey, &networkingv1.NetworkPolicy{}))).To(BeTrue())
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, ingressKey, &networkingv1.NetworkPolicy{}))).To(BeTrue())
	})
})

// w9/m495 end to end: a Blueprint's web service whose buildCommand queries a
// sibling private service through fromService hostport (a literal
// `<slug>:<port>` in spec.env). The build Job can resolve the bare slug from
// the build namespace, its pods may reach the sibling's serving pods while it
// runs, and every per-build policy is gone when it ends.
var _ = Describe("Build reach to the workspace's own services (w9/m495)", func() {
	ctx := context.Background()
	const (
		name    = "srv-m495-site"
		sibling = "srv-m495-cms"
		ws      = "tea-m495m495m495m495m495"
		buildNS = "m495-builds"
	)
	nn := types.NamespacedName{Name: name, Namespace: ws}

	It("grants the build its sibling service and revokes every half when it ends", func() {
		for _, ns := range []string{ws, buildNS} {
			_ = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		}
		cms := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: sibling, Namespace: ws, Labels: map[string]string{labelWorkspace: ws}},
			Spec: appv1alpha1.AppSpec{Image: "ghcr.io/acme/cms:1", Type: appv1alpha1.TypePrivateService,
				Tier: "starter", Subdomain: "cms-x9z2"},
		}
		Expect(k8sClient.Create(ctx, cms)).To(Succeed())
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ws, Labels: map[string]string{labelWorkspace: ws}},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/acme/site", Branch: "main", Builder: "dockerfile", Port: 3000,
				Env: []appv1alpha1.EnvVar{{Name: "CMS_HOSTPORT", Value: "cms-x9z2:10000"}},
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: buildNS}
		DeferCleanup(func() {
			a := &appv1alpha1.App{}
			if k8sClient.Get(ctx, nn, a) == nil {
				Expect(k8sClient.Delete(ctx, a)).To(Succeed())
				r.Registry = ""
				for range 3 {
					_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				}
			}
			_ = k8sClient.Delete(ctx, cms)
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, "gen-1"), Namespace: buildNS}})
		})
		reconcileN := func(n int) {
			for range n {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				Expect(err).NotTo(HaveOccurred())
			}
		}
		egressKey := client.ObjectKey{Namespace: buildNS, Name: buildEgressPolicyName(name)}
		policyKeys := []client.ObjectKey{egressKey,
			{Namespace: ws, Name: buildIngressPolicyName(name)},
			{Namespace: ws, Name: buildServiceIngressPolicyName(name)}}

		By("dispatching a build that resolves the bare slug through the App namespace")
		reconcileN(3)
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: build.JobName(name, "gen-1")}, job)).To(Succeed())
		Expect(job.Spec.Template.Spec.DNSConfig).NotTo(BeNil())
		Expect(job.Spec.Template.Spec.DNSConfig.Searches).To(Equal([]string{ws + ".svc.cluster.local"}))

		By("granting the build pods the sibling's serving pods, any TCP port, in this workspace only")
		egress := &networkingv1.NetworkPolicy{}
		Expect(k8sClient.Get(ctx, egressKey, egress)).To(Succeed())
		Expect(job.Spec.Template.Labels).To(Equal(mergeLabels(job.Spec.Template.Labels, egress.Spec.PodSelector.MatchLabels)),
			"the egress policy must select the build Job's own pods")
		Expect(egress.Spec.Egress).To(HaveLen(2), "datastore ports + the workspace's services")
		serviceRule := egress.Spec.Egress[1]
		Expect(serviceRule.To).To(HaveLen(1))
		Expect(serviceRule.To[0].NamespaceSelector.MatchLabels).To(Equal(map[string]string{corev1.LabelMetadataName: ws}))
		Expect(serviceRule.To[0].PodSelector.MatchExpressions).To(ContainElement(metav1.LabelSelectorRequirement{
			Key: labelApp, Operator: metav1.LabelSelectorOpIn, Values: []string{sibling}}))
		Expect(serviceRule.Ports).To(HaveLen(1))
		Expect(serviceRule.Ports[0].Port).To(BeNil(), "a service listens on the port it chose")
		Expect(*serviceRule.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
		serviceIngress := &networkingv1.NetworkPolicy{}
		Expect(k8sClient.Get(ctx, policyKeys[2], serviceIngress)).To(Succeed())
		Expect(serviceIngress.Spec.PodSelector).To(Equal(*serviceRule.To[0].PodSelector))
		Expect(serviceIngress.Spec.Ingress[0].From[0].NamespaceSelector.MatchLabels).To(Equal(map[string]string{corev1.LabelMetadataName: buildNS}))
		Expect(serviceIngress.Spec.Ingress[0].From[0].PodSelector.MatchLabels).To(Equal(egress.Spec.PodSelector.MatchLabels))

		By("revoking every half when the build ends")
		completeEnvtestJob(job, metav1.Now())
		reconcileN(2)
		for _, key := range policyKeys {
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key, &networkingv1.NetworkPolicy{}))).To(BeTrue(), key.String())
		}
	})
})

// w9/124: a build that ends without a terminal observation of its own loses
// its reach where it ends, on that pass, with no later build to wait for: a
// user's cancel, the build of a release a cancel settled below (superseded by
// the canceled deploy, then ended with it), a timeout, and the App's deletion.
// Every half goes: the build-namespace egress, the datastore ingress and
// w9/m495's service ingress. A second App building in the same workspace
// keeps all three of its own throughout, and the release a cancel leaves
// running keeps its grant until it ends.
var _ = Describe("Build reach ends with a canceled or settled build (w9/124)", func() {
	ctx := context.Background()
	const (
		name    = "srv-w9124-site"
		peer    = "srv-w9124-peer"
		cms     = "srv-w9124-cms"
		ws      = "tea-w9124w9124w9124w9124"
		buildNS = "w9124-builds"
	)
	nn := types.NamespacedName{Name: name, Namespace: ws}
	peerNN := types.NamespacedName{Name: peer, Namespace: ws}
	var r *AppReconciler

	reconcileN := func(key types.NamespacedName, n int) {
		GinkgoHelper()
		for range n {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	getApp := func() *appv1alpha1.App {
		GinkgoHelper()
		a := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, a)).To(Succeed())
		return a
	}
	update := func(mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		a := getApp()
		if a.Annotations == nil {
			a.Annotations = map[string]string{}
		}
		mutate(a)
		Expect(k8sClient.Update(ctx, a)).To(Succeed())
	}
	jobKey := func(app, rev string) client.ObjectKey {
		return client.ObjectKey{Namespace: buildNS, Name: build.JobName(app, rev)}
	}
	jobExists := func(app, rev string) bool {
		return k8sClient.Get(ctx, jobKey(app, rev), &batchv1.Job{}) == nil
	}
	policyKeys := func(app string) []client.ObjectKey {
		return []client.ObjectKey{
			{Namespace: buildNS, Name: buildEgressPolicyName(app)},
			{Namespace: ws, Name: buildIngressPolicyName(app)},
			{Namespace: ws, Name: buildServiceIngressPolicyName(app)},
		}
	}
	expectReach := func(app string, held bool) {
		GinkgoHelper()
		for _, key := range policyKeys(app) {
			err := k8sClient.Get(ctx, key, &networkingv1.NetworkPolicy{})
			if held {
				Expect(err).NotTo(HaveOccurred(), "%s must keep %s", app, key)
			} else {
				Expect(apierrors.IsNotFound(err)).To(BeTrue(), "%s must lose %s at once: %v", app, key, err)
			}
		}
	}
	sourceApp := func(app string) *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: app, Namespace: ws, Labels: map[string]string{labelWorkspace: ws},
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{Repo: "https://github.com/acme/" + app, Branch: "main", Builder: "dockerfile", Port: 3000},
		}
	}

	BeforeEach(func() {
		for _, ns := range []string{ws, buildNS} {
			_ = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}})
		}
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: buildNS}
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: cms, Namespace: ws, Labels: map[string]string{labelWorkspace: ws}},
			Spec: appv1alpha1.AppSpec{Image: "ghcr.io/acme/cms:1", Type: appv1alpha1.TypePrivateService,
				Tier: "starter", Subdomain: "cms-w9124"},
		})).To(Succeed())
		for _, app := range []string{name, peer} {
			Expect(k8sClient.Create(ctx, sourceApp(app))).To(Succeed())
		}
		reconcileN(nn, 3)
		reconcileN(peerNN, 3)
		Expect(jobExists(name, "gen-1")).To(BeTrue(), "precondition: the build runs")
		Expect(jobExists(peer, "gen-1")).To(BeTrue(), "precondition: the other App's build runs")
		expectReach(name, true)
		expectReach(peer, true)
	})
	AfterEach(func() {
		r.Registry = ""
		for _, key := range []types.NamespacedName{nn, peerNN} {
			if a := (&appv1alpha1.App{}); k8sClient.Get(ctx, key, a) == nil {
				Expect(k8sClient.Delete(ctx, a)).To(Succeed())
				for range 3 {
					_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
				}
			}
		}
		_ = k8sClient.Delete(ctx, &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: cms, Namespace: ws}})
		for _, app := range []string{name, peer} {
			for _, rev := range []string{"gen-1", "gen-2", "gen-3"} {
				_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(app, rev), Namespace: buildNS}},
					client.PropagationPolicy(metav1.DeletePropagationBackground))
			}
			// envtest runs no garbage collector.
			for _, key := range policyKeys(app) {
				_ = k8sClient.Delete(ctx, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace}})
			}
		}
	})

	It("revokes every half when the user cancels the build", func() {
		update(func(a *appv1alpha1.App) {
			a.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = strconv.FormatInt(a.Generation, 10)
		})
		reconcileN(nn, 1)

		Expect(getApp().Status.Phase).To(Equal(appv1alpha1.PhaseCanceled))
		Expect(jobExists(name, "gen-1")).To(BeFalse(), "the canceled build is stopped")
		expectReach(name, false)
		expectReach(peer, true)
		Expect(jobExists(name, "gen-2")).To(BeFalse(), "no later build ran to revoke it")
	})

	It("keeps the grant for the release a cancel leaves running, and revokes it when that build is ended too", func() {
		By("a second deploy waits behind the build, and is canceled")
		update(func(a *appv1alpha1.App) {
			a.Spec.RestartedAt = "2026-11-19T08:00:00Z"
			a.Annotations[appv1alpha1.AnnotationReleaseGeneration] = strconv.FormatInt(a.Generation+1, 10)
		})
		reconcileN(nn, 1)
		Expect(getApp().Status.PendingReleaseGeneration).To(Equal(int64(2)), "precondition: generation 2 is the pending slot")
		update(func(a *appv1alpha1.App) {
			a.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = strconv.FormatInt(a.Generation, 10)
			a.Annotations[appv1alpha1.AnnotationSurvivingReleaseGeneration] = "1"
		})
		reconcileN(nn, 2)
		Expect(jobExists(name, "gen-1")).To(BeTrue(), "the release the cancel left running keeps building")
		expectReach(name, true)

		By("bex-api ends that release too: its build is superseded by the canceled deploy and settles with it")
		update(func(a *appv1alpha1.App) { delete(a.Annotations, appv1alpha1.AnnotationSurvivingReleaseGeneration) })
		reconcileN(nn, 1)
		Expect(getApp().Status.Phase).To(Equal(appv1alpha1.PhaseCanceled))
		Expect(jobExists(name, "gen-1")).To(BeFalse(), "the abandoned build is stopped")
		expectReach(name, false)
		expectReach(peer, true)
		Expect(jobExists(name, "gen-2")).To(BeFalse(), "no later build ran to revoke it")
	})

	It("revokes every half when the build times out", func() {
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, jobKey(name, "gen-1"), job)).To(Succeed())
		failEnvtestJob(job, batchv1.JobReasonDeadlineExceeded, "Job was active longer than specified deadline")
		// The pass that records a build failure returns it.
		_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})

		cond := meta.FindStatusCondition(getApp().Status.Conditions, appv1alpha1.ConditionBuild)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Message).To(ContainSubstring("time limit"), "precondition: the build settled as a timeout")
		expectReach(name, false)
		expectReach(peer, true)
	})

	It("revokes every half when the App is deleted mid-build", func() {
		r.Registry = ""
		Expect(k8sClient.Delete(ctx, getApp())).To(Succeed())
		_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})

		Expect(jobExists(name, "gen-1")).To(BeFalse(), "the deletion stops the build")
		expectReach(name, false)
		expectReach(peer, true)
	})
})

// mergeLabels returns base with extra laid over it; equal to base exactly when
// extra is a subset of base.
func mergeLabels(base, extra map[string]string) map[string]string {
	out := maps.Clone(base)
	maps.Copy(out, extra)
	return out
}
