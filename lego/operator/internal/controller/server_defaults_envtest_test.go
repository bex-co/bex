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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// stripServerPodDefaults removes the fields applyPodSpecServerDefaults adds,
// reproducing what every projection in this package emitted before w7/m84. Used
// to prove the two produce the same STORED object — which is what "no
// production pod rolls" means, since the pod-template hash is computed from the
// stored, already-defaulted template.
//
// The caller pins this as the exact inverse rather than trusting the field list
// below to stay in step: a default added to applyPodSpecServerDefaults and
// forgotten here would quietly weaken the no-rollout proof into a tautology.
func stripServerPodDefaults(spec *corev1.PodSpec) {
	spec.TerminationGracePeriodSeconds = nil
	spec.DNSPolicy = ""
	spec.SchedulerName = ""
	for i := range spec.Containers {
		c := &spec.Containers[i]
		c.TerminationMessagePath = ""
		c.TerminationMessagePolicy = ""
		for j := range c.Ports {
			c.Ports[j].Protocol = ""
		}
		for _, p := range []*corev1.Probe{c.StartupProbe, c.ReadinessProbe, c.LivenessProbe} {
			if p == nil {
				continue
			}
			p.SuccessThreshold = 0
			if p == c.ReadinessProbe {
				p.PeriodSeconds = 0
				p.FailureThreshold = 0
			}
			if p.HTTPGet != nil {
				p.HTTPGet.Scheme = ""
			}
		}
	}
	for i := range spec.Volumes {
		if spec.Volumes[i].Projected != nil {
			spec.Volumes[i].Projected.DefaultMode = nil
		}
	}
}

var _ = Describe("Server pod-template defaults (w7/m84)", func() {
	const namespace = "default"
	ctx := context.Background()

	// bareDeployment is a valid Deployment that sets nothing optional, so what
	// comes back is exactly the API server's own defaulting.
	bareDeployment := func(name, image string) *appsv1.Deployment {
		labels := map[string]string{"app": name}
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: labels},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: labels},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "app",
							Image: image,
							Ports: []corev1.ContainerPort{{ContainerPort: 3000}},
							ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt(3000)},
							}},
						}},
						// One volume per source applyVolumeServerDefaults knows
						// about, so no branch of it is speculative: projected is
						// the App's secret files, secret is the public KeyValue's
						// TLS material, and configMap is covered before a
						// projection reaches for it.
						Volumes: []corev1.Volume{{
							Name: "projected-files",
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{{
									Secret: &corev1.SecretProjection{
										LocalObjectReference: corev1.LocalObjectReference{Name: "files"},
									},
								}}},
							},
						}, {
							Name: "secret-files",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{SecretName: "tls"},
							},
						}, {
							Name: "configmap-files",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "conf"},
								},
							},
						}},
					},
				},
			},
		}
	}

	// storeAndRead creates dep and returns the pod spec the API server stored.
	storeAndRead := func(dep *appsv1.Deployment) corev1.PodSpec {
		GinkgoHelper()
		Expect(k8sClient.Create(ctx, dep)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, dep) })
		stored := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: dep.Name, Namespace: namespace}, stored)).To(Succeed())
		return stored.Spec.Template.Spec
	}

	It("writes what the API server itself would have defaulted, field by field", func() {
		stored := storeAndRead(bareDeployment("defaults-oracle", "ghcr.io/example/app:v1"))

		defaulted := bareDeployment("defaults-oracle", "ghcr.io/example/app:v1").Spec.Template.Spec
		applyPodSpecServerDefaults(&defaulted)

		// Each field named, so an upstream default that moves fails a line that
		// says which one — not just "the objects differ".
		container, probe := stored.Containers[0], stored.Containers[0].ReadinessProbe
		Expect(stored.TerminationGracePeriodSeconds).To(HaveValue(Equal(defaultTerminationGracePeriodSeconds)))
		Expect(stored.RestartPolicy).To(Equal(corev1.RestartPolicyAlways))
		Expect(stored.DNSPolicy).To(Equal(corev1.DNSClusterFirst))
		Expect(stored.SchedulerName).To(Equal(corev1.DefaultSchedulerName))
		Expect(stored.SecurityContext).To(Equal(&corev1.PodSecurityContext{}))
		Expect(container.TerminationMessagePath).To(Equal(corev1.TerminationMessagePathDefault))
		Expect(container.TerminationMessagePolicy).To(Equal(corev1.TerminationMessageReadFile))
		Expect(container.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
		Expect(probe.SuccessThreshold).To(Equal(defaultProbeSuccessThreshold))
		Expect(probe.FailureThreshold).To(Equal(defaultProbeFailureThreshold))
		Expect(probe.PeriodSeconds).To(Equal(defaultProbePeriodSeconds))
		Expect(probe.TimeoutSeconds).To(Equal(defaultProbeTimeoutSeconds))
		Expect(probe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTP))
		Expect(stored.Volumes[0].Projected.DefaultMode).To(HaveValue(Equal(corev1.ProjectedVolumeSourceDefaultMode)))
		Expect(stored.Volumes[1].Secret.DefaultMode).To(HaveValue(Equal(corev1.SecretVolumeSourceDefaultMode)))
		Expect(stored.Volumes[2].ConfigMap.DefaultMode).To(HaveValue(Equal(corev1.ConfigMapVolumeSourceDefaultMode)))

		// …and the whole spec, so a default this helper does NOT know about is
		// caught the first time a projection starts emitting the field.
		Expect(defaulted).To(Equal(stored), "applyPodSpecServerDefaults must reproduce the API server exactly")
	})

	It("reproduces the API server's imagePullPolicy rule for every reference form", func() {
		for i, image := range []string{
			"ghcr.io/example/app:v1",
			"ghcr.io/example/app:latest",
			"ghcr.io/example/app",
			"zot.bex-registry.svc:5000/app:gen-7",
			"zot.bex-registry.svc:5000/app",
			"ghcr.io/example/app@sha256:0000000000000000000000000000000000000000000000000000000000000000",
		} {
			stored := storeAndRead(bareDeployment(fmt.Sprintf("defaults-pull-%d", i), image))
			Expect(serverDefaultPullPolicy(image)).To(Equal(stored.Containers[0].ImagePullPolicy),
				"image %q", image)
		}
	})

	It("stores byte-identical bytes before and after the fix, so no pod rolls", func() {
		const name = "defaults-no-rollout"
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: appv1alpha1.AppSpec{
				Image: "ghcr.io/example/app:v1", Port: 3000, HealthCheckPath: "/healthz",
				FilesFromSecrets: []string{name + "-files"},
			},
		}
		params := deploymentParams{image: app.Spec.Image, port: 3000, replicas: 1}

		// The pre-w7/m84 projection: today's, minus the server defaults.
		before := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		applyDeploymentSpec(before, app, params)
		projected := before.Spec.Template.Spec.DeepCopy()
		stripServerPodDefaults(&before.Spec.Template.Spec)
		restored := before.Spec.Template.Spec.DeepCopy()
		applyPodSpecServerDefaults(restored)
		Expect(restored).To(Equal(projected),
			"stripServerPodDefaults must be the exact inverse of applyPodSpecServerDefaults, "+
				"or this spec proves nothing")
		Expect(k8sClient.Create(ctx, before)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, before) })

		nn := types.NamespacedName{Name: name, Namespace: namespace}
		storedBefore := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, storedBefore)).To(Succeed())

		// Now the current projection, applied to that stored object exactly as
		// the reconciler's CreateOrUpdate would.
		after := storedBefore.DeepCopy()
		applyDeploymentSpec(after, app, params)
		Expect(k8sClient.Update(ctx, after)).To(Succeed())

		storedAfter := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, storedAfter)).To(Succeed())
		Expect(storedAfter.Spec.Template).To(Equal(storedBefore.Spec.Template),
			"the stored pod template must be unchanged — the pod-template hash is computed from it, "+
				"so an equal template is an equal hash and no rollout")
		Expect(storedAfter.ResourceVersion).To(Equal(storedBefore.ResourceVersion),
			"an upgrade to the fixed operator must not even write")
	})

	It("still writes exactly once when the App actually changes", func() {
		const name = "defaults-real-change"
		rec := &writeRecorder{Client: k8sClient}
		r := &AppReconciler{Client: rec, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
		nn := types.NamespacedName{Name: name, Namespace: namespace}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: appv1alpha1.AppSpec{
				Image: "ghcr.io/example/app:v1", Port: 3000, HealthCheckPath: "/healthz",
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		run := func() (ctrl.Result, error) {
			return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		}
		reconcileToFixedPoint(run)
		DeferCleanup(func() { deleteOwner(ctx, app, run) })

		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		app.Spec.Image = "ghcr.io/example/app:v2"
		Expect(k8sClient.Update(ctx, app)).To(Succeed())

		rec.reset()
		_, err := run()
		Expect(err).NotTo(HaveOccurred())
		Expect(rec.sorted()).To(Equal([]string{"update Deployment/" + name}),
			"a real image change must produce one Deployment write and nothing else")

		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		Expect(dep.Spec.Template.Spec.Containers[0].Image).To(Equal("ghcr.io/example/app:v2"))
	})
})
