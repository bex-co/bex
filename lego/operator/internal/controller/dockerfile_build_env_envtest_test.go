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
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w1/m178: a Dockerfile service's build receives its saved env as build args
// and its secret files as Render-id build secrets — through the same App-owned
// projection a native build mounts, with no Secret-backed value in the Job.
var _ = Describe("Dockerfile build env and secret files (w1/m178)", func() {
	ctx := context.Background()
	const (
		name      = "dock-env-app"
		ns        = "default"
		buildNS   = "dock-env-builds"
		envValue  = "sekrit-value-m178"
		fileValue = "TOKEN=file-value-m178\n"
	)
	nn := types.NamespacedName{Name: name, Namespace: ns}

	It("projects the env Secret and secret files into the Dockerfile build Job", func() {
		_ = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: buildNS}})
		own := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name + "-env", Namespace: ns},
			StringData: map[string]string{"GREETING": envValue}}
		files := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name + "-files", Namespace: ns},
			StringData: map[string]string{".env": fileValue}}
		Expect(k8sClient.Create(ctx, own)).To(Succeed())
		Expect(k8sClient.Create(ctx, files)).To(Succeed())
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main",
				Builder: "dockerfile", Port: 3000,
				EnvFromSecret:    own.Name,
				FilesFromSecrets: []string{files.Name},
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
			_ = k8sClient.Delete(ctx, own)
			_ = k8sClient.Delete(ctx, files)
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, "gen-1"), Namespace: buildNS}})
		})

		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}

		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: build.JobName(name, "gen-1")}, job)).To(Succeed())
		var volumes []string
		for _, v := range job.Spec.Template.Spec.Volumes {
			if v.Secret != nil {
				volumes = append(volumes, v.Name+"="+v.Secret.SecretName)
			}
		}
		Expect(volumes).To(ContainElements("runtime-env="+nativeEnvSecretName(name), "native-files="+nativeFilesSecretName(name)))

		spec := job.Spec.Template.Spec
		buildkit := findContainer(append(spec.InitContainers, spec.Containers...), "buildkit")
		Expect(buildkit).NotTo(BeNil())
		Expect(slices.Contains(buildkit.Args, "id=_env,src=/bex/build-inputs/files/.env")).To(BeTrue(),
			"the .env secret file must be a build secret with Render's id: %q", buildkit.Args)

		raw, err := json.Marshal(job)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(raw)).NotTo(ContainSubstring(envValue), "a Secret-backed value must never enter the Job spec")
		Expect(string(raw)).NotTo(ContainSubstring("file-value-m178"))

		merged := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: nativeEnvSecretName(name)}, merged)).To(Succeed())
		Expect(string(merged.Data["GREETING"])).To(Equal(envValue))
		projected := &corev1.Secret{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: nativeFilesSecretName(name)}, projected)).To(Succeed())
		Expect(string(projected.Data[".env"])).To(Equal(fileValue))
	})

	It("passes a literal-only env as build args without minting an env Secret", func() {
		const litName = "dock-env-literal"
		litNN := types.NamespacedName{Name: litName, Namespace: ns}
		_ = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: buildNS}})
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: litName, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main",
				Builder: "dockerfile", Port: 3000,
				Env: []appv1alpha1.EnvVar{{Name: "GREETING", Value: "hello"}},
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: buildNS}
		DeferCleanup(func() {
			a := &appv1alpha1.App{}
			if k8sClient.Get(ctx, litNN, a) == nil {
				Expect(k8sClient.Delete(ctx, a)).To(Succeed())
				r.Registry = ""
				for range 3 {
					_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: litNN})
				}
			}
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(litName, "gen-1"), Namespace: buildNS}})
		})
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: litNN})
			Expect(err).NotTo(HaveOccurred())
		}

		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: build.JobName(litName, "gen-1")}, job)).To(Succeed())
		for _, v := range job.Spec.Template.Spec.Volumes {
			Expect(v.Name).NotTo(Equal("runtime-env"), "a literal-only Dockerfile build needs no env Secret volume")
		}
		spec := job.Spec.Template.Spec
		buildkit := findContainer(append(spec.InitContainers, spec.Containers...), "buildkit")
		Expect(buildkit).NotTo(BeNil())
		// The service's own literal and Render's build-time platform set (w5/m148).
		var keys string
		for _, e := range buildkit.Env {
			if e.Name == "BEX_BUILD_ARG_KEYS" {
				keys = e.Value
			}
		}
		// Not exposed, so no RENDER_EXTERNAL_*; the commit is stamped in the pod.
		Expect(strings.Split(keys, "\n")).To(ConsistOf("GREETING", "RENDER", "IS_PULL_REQUEST", "RENDER_CPU_COUNT",
			"RENDER_SERVICE_ID", "RENDER_SERVICE_NAME", "RENDER_SERVICE_TYPE", "RENDER_DISCOVERY_SERVICE",
			"RENDER_GIT_BRANCH", "RENDER_GIT_REPO_SLUG"))
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: nativeEnvSecretName(litName)}, &corev1.Secret{})).NotTo(Succeed(),
			"no merged env Secret is written for literals alone")
	})
})
