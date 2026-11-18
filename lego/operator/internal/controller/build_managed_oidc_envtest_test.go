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
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w2/m480, docs/ADR116-managed-oidc.md D12: a native build in a Pro
// workspace whose environment sets a trigger records its own decision, and
// its Job reads the tokens bex-api minted for that build, copied next to it;
// a Dockerfile build and a Hobby one get nothing, as on Render.
var _ = Describe("Managed OIDC at build time (w2/m480)", func() {
	ctx := context.Background()
	const (
		ns      = "default"
		buildNS = "oidc-builds"
		gcpWIP  = "projects/123456789012/locations/global/workloadIdentityPools/bex-pool/providers/bex-provider"
	)
	var r *AppReconciler
	BeforeEach(func() {
		_ = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: buildNS}})
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: buildNS}
	})
	passN := func(nn types.NamespacedName) {
		GinkgoHelper()
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	create := func(name, builder string, pro bool) (types.NamespacedName, *appv1alpha1.App) {
		GinkgoHelper()
		env := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name + "-env", Namespace: ns}, StringData: map[string]string{
			"AWS_ROLE_ARN":                   "arn:aws:iam::123456789012:role/build",
			"GCP_WORKLOAD_IDENTITY_PROVIDER": gcpWIP,
		}}
		Expect(k8sClient.Create(ctx, env)).To(Succeed())
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main", Port: 3000,
				Builder: builder, EnvFromSecret: env.Name,
			},
		}
		if builder == build.BuilderNative {
			app.Spec.Runtime, app.Spec.BuildCommand, app.Spec.StartCommand = "node", "npm ci", "node server.js"
		}
		if pro {
			app.Labels = map[string]string{appv1alpha1.LabelManagedOIDC: appv1alpha1.ManagedOIDCEnabled}
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		nn := types.NamespacedName{Name: name, Namespace: ns}
		DeferCleanup(func() {
			if a := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, a) == nil {
				Expect(k8sClient.Delete(ctx, a)).To(Succeed())
				r.Registry = ""
				passN(nn)
			}
			_ = k8sClient.Delete(ctx, env)
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, "gen-1"), Namespace: buildNS}})
		})
		return nn, app
	}
	jobOf := func(name string) corev1.PodSpec {
		GinkgoHelper()
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: build.JobName(name, "gen-1")}, job)).To(Succeed())
		return job.Spec.Template.Spec
	}
	tokenVolume := func(spec corev1.PodSpec) *corev1.Volume {
		for i := range spec.Volumes {
			if spec.Volumes[i].Name == "managed-oidc" {
				return &spec.Volumes[i]
			}
		}
		return nil
	}
	copyOf := func(name string) (*corev1.Secret, error) {
		s := &corev1.Secret{}
		return s, k8sClient.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: buildManagedOIDCSecretName(name)}, s)
	}

	It("records a native build's decision and hands its Job only the tokens minted for that build", func() {
		const name = "oidc-native-build"
		nn, _ := create(name, build.BuilderNative, true)
		passN(nn)
		app := envtestApp(nn)
		gen := app.Status.ReleaseGeneration
		Expect(gen).To(BeNumerically(">", 0))
		st := app.Status.BuildManagedOIDC
		Expect(st).NotTo(BeNil())
		Expect(st.Generation).To(Equal(gen))
		Expect(st.Mounted()).To(Equal([]string{"aws", "gcp"}))
		Expect(st.GCP).NotTo(BeNil())
		Expect(st.GCP.WorkloadIdentityProvider).To(Equal(gcpWIP))

		// A copy an earlier build of this App lifetime left behind.
		stale := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: buildManagedOIDCSecretName(name), Namespace: buildNS,
			Labels:      artifactLabels(app, "managed-oidc-secret"),
			Annotations: map[string]string{appv1alpha1.AnnotationManagedOIDCGeneration: strconv.FormatInt(gen-1, 10)}},
			Data: map[string][]byte{"aws-token": []byte("an earlier build's token")}}
		Expect(k8sClient.Create(ctx, stale)).To(Succeed())

		// The Job waits on the copy, listing every file; only the preparer
		// reads it.
		spec := jobOf(name)
		vol := tokenVolume(spec)
		Expect(vol).NotTo(BeNil())
		Expect(vol.Secret.SecretName).To(Equal(buildManagedOIDCSecretName(name)))
		Expect(vol.Secret.Items).To(Equal([]corev1.KeyToPath{
			{Key: "aws-token", Path: "aws-token"}, {Key: "gcp-token", Path: "gcp-token"},
			{Key: "gcp-credentials.json", Path: "gcp-credentials.json"}}))
		prep := findContainer(spec.InitContainers, "prepare-native-build")
		Expect(prep).NotTo(BeNil())
		Expect(prep.VolumeMounts).To(ContainElement(corev1.VolumeMount{Name: "managed-oidc", MountPath: "/managed-oidc", ReadOnly: true}))

		// Not minted yet: the earlier build's copy goes, and nothing replaces it.
		passN(nn)
		_, err := copyOf(name)
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "an earlier build's copy must not stand in: %v", err)

		// bex-api mints for another build: still nothing.
		src := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.ManagedOIDCBuildSecretName(name), Namespace: ns,
			Labels:          map[string]string{appv1alpha1.LabelProtectedFromTenantMount: appv1alpha1.ProtectedFromTenantMount},
			Annotations:     map[string]string{appv1alpha1.AnnotationManagedOIDCGeneration: strconv.FormatInt(gen+1, 10)},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "app.bex.co/v1alpha1", Kind: "App", Name: name, UID: app.UID}}},
			Data: map[string][]byte{"aws-token": []byte("aws-jwt"), "gcp-token": []byte("gcp-jwt"), "gcp-credentials.json": []byte("{}"), "extra": []byte("x")}}
		Expect(k8sClient.Create(ctx, src)).To(Succeed())
		passN(nn)
		_, err = copyOf(name)
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "tokens minted for another build: %v", err)

		// Minted for this build: copied, with exactly the build's files.
		src.Annotations[appv1alpha1.AnnotationManagedOIDCGeneration] = strconv.FormatInt(gen, 10)
		Expect(k8sClient.Update(ctx, src)).To(Succeed())
		passN(nn)
		copied, err := copyOf(name)
		Expect(err).NotTo(HaveOccurred())
		Expect(copied.Data).To(Equal(map[string][]byte{"aws-token": []byte("aws-jwt"), "gcp-token": []byte("gcp-jwt"), "gcp-credentials.json": []byte("{}")}))
		Expect(copied.Annotations[appv1alpha1.AnnotationManagedOIDCGeneration]).To(Equal(strconv.FormatInt(gen, 10)))
		Expect(copied.Labels[appv1alpha1.LabelProtectedFromTenantMount]).To(Equal(appv1alpha1.ProtectedFromTenantMount))
		Expect(copied.Labels["app.bex.co/app-uid"]).To(Equal(string(app.UID)))

		// A refresh reaches the copy on the next pass.
		src.Data["aws-token"] = []byte("aws-jwt-refreshed")
		Expect(k8sClient.Update(ctx, src)).To(Succeed())
		passN(nn)
		copied, err = copyOf(name)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(copied.Data["aws-token"])).To(Equal("aws-jwt-refreshed"))

		raw, err := json.Marshal(jobOf(name))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(raw)).NotTo(ContainSubstring("aws-jwt"), "a token must never enter the Job spec")
	})

	It("gives a Dockerfile build and a Hobby native build nothing", func() {
		nn, _ := create("oidc-docker-build", build.BuilderDockerfile, true)
		passN(nn)
		Expect(envtestApp(nn).Status.BuildManagedOIDC).To(BeNil(), "a Dockerfile build decides nothing")
		Expect(tokenVolume(jobOf(nn.Name))).To(BeNil())
		raw, err := json.Marshal(jobOf(nn.Name))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(raw)).NotTo(ContainSubstring("/var/lib/render/oidc"))

		hobby, _ := create("oidc-hobby-build", build.BuilderNative, false)
		passN(hobby)
		st := envtestApp(hobby).Status.BuildManagedOIDC
		Expect(st).NotTo(BeNil())
		Expect(st.Refused()).To(Equal([]string{"aws", "gcp"}))
		Expect(st.Mounted()).To(BeEmpty())
		Expect(tokenVolume(jobOf(hobby.Name))).To(BeNil())
	})
})
