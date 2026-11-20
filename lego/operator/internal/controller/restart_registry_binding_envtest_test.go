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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A Restart pulls the serving release with the registry credential it went
// live with, and a rollback its target with the target's (w8/m529). On
// production a switch to a bad Docker Hub credential failed, and Restart, the
// documented recovery, then pulled the served nginx digest with that
// credential: every pod the operator built named spec.externalRegistryPullSecret,
// whichever release it ran, and Docker Hub refused it with 401 (w8/353).
var _ = Describe("Restart and rollback pull with their release's registry credential (w8/m529)", func() {
	const (
		credentialA = "rgc-a0000000000000000000"
		credentialB = "rgc-b0000000000000000000"
	)
	var name string
	var nn types.NamespacedName
	var r *AppReconciler
	BeforeEach(func() {
		name = "restart-registry-binding"
		nn = types.NamespacedName{Name: name, Namespace: "default"}
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	})
	pass := func(n int) {
		GinkgoHelper()
		for range n {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	getApp := func() *appv1alpha1.App { GinkgoHelper(); return envtestApp(nn) }
	update := func(mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		app := getApp()
		mutate(app)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
	}
	record := func(gen int64) *runtimeConfigRecord {
		GinkgoHelper()
		rec, err := r.readRuntimeConfigRecord(ctx, getApp(), gen)
		Expect(err).NotTo(HaveOccurred())
		return rec
	}
	pullSecret := func(credential string) string { return appv1alpha1.ExternalRegistryPullSecretName(name, credential) }
	// materialize writes credential's own pull Secret, as bex-api's trigger does.
	materialize := func(credential, token string) {
		GinkgoHelper()
		sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: pullSecret(credential), Namespace: "default",
			Labels: map[string]string{appv1alpha1.LabelProtectedFromTenantMount: appv1alpha1.ProtectedFromTenantMount}},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{corev1.DockerConfigJsonKey: fmt.Appendf(nil, `{"auths":{"docker.io":{"username":"u","password":%q}}}`, token)}}
		Expect(k8sClient.Create(ctx, sec)).To(Succeed())
	}
	// bind saves credential (or none) as the service's binding.
	bind := func(app *appv1alpha1.App, credential string) {
		app.Spec.RegistryCredentialID, app.Spec.ExternalRegistryPullSecret = nil, ""
		if credential != "" {
			app.Spec.RegistryCredentialID, app.Spec.ExternalRegistryPullSecret = new(credential), pullSecret(credential)
		}
	}
	// saveOnly is a save that waits for the next deploy, as Render's is.
	saveOnly := func(credential string) {
		GinkgoHelper()
		update(func(app *appv1alpha1.App) {
			bind(app, credential)
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationPendingSourceGeneration, fmt.Sprint(app.Generation+1))
		})
		pass(2)
	}
	// release stamps a deploy verb's generation, selecting a recorded release
	// as bex-api's targetReleaseConfig does when from is positive; legacy
	// drops the record's binding, as a selection from before w8/m529 has none.
	release := func(from int64, restart, legacy bool) int64 {
		GinkgoHelper()
		var ref *appv1alpha1.ReleaseConfigReference
		if from > 0 {
			rec := record(from)
			ref = &appv1alpha1.ReleaseConfigReference{SourceGeneration: from, Image: "busybox:1.36",
				PreserveGroupValues: restart, Runtime: rec.spec.Runtime, Registry: rec.spec.Registry}
			if legacy {
				ref.Registry = nil
			}
		}
		update(func(app *appv1alpha1.App) {
			gen := app.Generation + 1
			app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
			app.Spec.ReleaseConfig = nil
			if ref != nil {
				ref.Generation = gen
				app.Spec.ReleaseConfig = ref
			}
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, fmt.Sprint(gen))
		})
		return getApp().Generation
	}
	external := func(refs []corev1.LocalObjectReference) []string {
		names := []string{}
		for _, ref := range refs {
			if appv1alpha1.IsExternalRegistryPullSecretName(name, ref.Name) {
				names = append(names, ref.Name)
			}
		}
		return names
	}
	// serve runs gen's pre-deploy step and rollout to live, and returns the
	// external pull Secrets the step and the release's pods name.
	serve := func(gen int64) (step, pods []string) {
		GinkgoHelper()
		pass(2)
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: predeploy.JobName(name, fmt.Sprintf("gen-%d", gen))}, job)).To(Succeed())
		step = external(job.Spec.Template.Spec.ImagePullSecrets)
		completeEnvtestJob(job, metav1.Now())
		pass(3)
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", gen)))
		return step, external(envtestDeployment(nn).Spec.Template.Spec.ImagePullSecrets)
	}
	expectPulls := func(gen int64, want ...string) {
		GinkgoHelper()
		if want == nil {
			want = []string{}
		}
		step, pods := serve(gen)
		Expect(pods).To(Equal(want), "the release's pods pull with its own binding")
		Expect(step).To(Equal(want), "its pre-deploy step pulls as its pods do")
		Expect(external(envtestDeployment(nn).Spec.Template.Spec.ImagePullSecrets)).To(Equal(want))
	}

	AfterEach(func() {
		if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
			deleteOwner(ctx, app, func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) })
		}
		background := metav1.DeletePropagationBackground
		for gen := range 10 {
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: predeploy.JobName(name, fmt.Sprintf("gen-%d", gen)), Namespace: "default"}},
				&client.DeleteOptions{PropagationPolicy: &background})
		}
		for _, credential := range []string{credentialA, credentialB} {
			_ = k8sClient.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: pullSecret(credential), Namespace: "default"}})
		}
	})

	It("keeps each release on the credential it went live with, whatever the service binds now", func() {
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default",
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{Image: "busybox:1.36", Port: 3000, Replicas: 1, PreDeployCommand: "echo migrate"},
		})).To(Succeed())
		expectPulls(1)
		Expect(record(1).spec.Registry).To(Equal(&appv1alpha1.ReleaseRegistryBinding{}), "a release records that it pulled anonymously")

		By("a switch to a credential that never goes live")
		materialize(credentialB, "bad-token")
		saveOnly(credentialB)
		Expect(external(envtestDeployment(nn).Spec.Template.Spec.ImagePullSecrets)).To(BeEmpty(), "a save rolls nothing")

		By("a Restart pulls the serving release anonymously, as it went live")
		restarted := release(1, true, false)
		expectPulls(restarted)
		Expect(record(restarted).spec.Registry).To(Equal(&appv1alpha1.ReleaseRegistryBinding{}), "the Restart records what it pulled with")
		Expect(getApp().ReleasePullSecret()).To(BeEmpty())

		By("a deploy with credential A, then a switch to B that waits for the next deploy")
		materialize(credentialA, "token-a")
		update(func(app *appv1alpha1.App) { bind(app, credentialA) })
		withA := release(0, false, false)
		expectPulls(withA, pullSecret(credentialA))
		saveOnly(credentialB)

		By("a Restart of the release that went live with A pulls with A, whose Secret is still A's")
		expectPulls(release(withA, true, false), pullSecret(credentialA))
		secretA := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: pullSecret(credentialA)}, secretA)).To(Succeed())
		Expect(string(secretA.Data[corev1.DockerConfigJsonKey])).To(ContainSubstring("token-a"))

		By("rollbacks pull with their target's binding")
		expectPulls(release(1, false, false))
		expectPulls(release(withA, false, false), pullSecret(credentialA))

		By("a selection from before w8/m529 carries no binding and pulls with the saved one")
		expectPulls(release(withA, true, true), pullSecret(credentialB))
		Expect(getApp().Spec.ExternalRegistryPullSecret).To(Equal(pullSecret(credentialB)), "the saved binding is untouched")
	})
})
