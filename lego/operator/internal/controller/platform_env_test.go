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
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func envMap(env []corev1.EnvVar) map[string]string {
	out := map[string]string{}
	for _, e := range env {
		out[e.Name] = e.Value
	}
	return out
}

func TestPlatformEnvMatchesRendersDocumentedSet(t *testing.T) {
	web := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web-cr", Labels: map[string]string{labelAppID: "srv-abc"}},
		Spec: appv1alpha1.AppSpec{Subdomain: "web-x1", Tier: "pro", Expose: true,
			Repo: "https://github.com/acme/web", Branch: "release"},
	}
	env := platformEnv(web, platformEnvInputs{baseDomain: "onbex.co", gitCommit: strings.Repeat("a", 40)})
	got := envMap(env)
	for k, want := range map[string]string{
		"RENDER": "true", "IS_PULL_REQUEST": "false", "RENDER_CPU_COUNT": "2",
		"RENDER_SERVICE_ID": "srv-abc", "RENDER_SERVICE_NAME": "web-x1", "RENDER_SERVICE_TYPE": "web",
		"RENDER_DISCOVERY_SERVICE": "web-x1-discovery",
		"RENDER_EXTERNAL_HOSTNAME": "web-x1.onbex.co", "RENDER_EXTERNAL_URL": "https://web-x1.onbex.co",
		"RENDER_GIT_BRANCH": "release", "RENDER_GIT_REPO_SLUG": "acme/web", "RENDER_GIT_COMMIT": strings.Repeat("a", 40),
		"RENDER_WEB_CONCURRENCY": "2",
	} {
		if got[k] != want {
			t.Errorf("%s = %q, want %q", k, got[k], want)
		}
	}
	for _, e := range env {
		if e.Name == "RENDER_INSTANCE_ID" && (e.ValueFrom == nil || e.ValueFrom.FieldRef.FieldPath != "metadata.name") {
			t.Errorf("RENDER_INSTANCE_ID must be the pod name, got %+v", e)
		}
	}

	worker := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "jobs"}, Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeBackgroundWorker, Image: "x"}}
	got = envMap(platformEnv(worker, platformEnvInputs{baseDomain: "onbex.co"}))
	if got["RENDER_SERVICE_TYPE"] != "worker" || got["RENDER_DISCOVERY_SERVICE"] != "jobs-discovery" {
		t.Errorf("worker env = %v", got)
	}
	for _, k := range []string{"RENDER_EXTERNAL_URL", "RENDER_WEB_CONCURRENCY", "RENDER_GIT_BRANCH", "RENDER_GIT_COMMIT"} {
		if _, ok := got[k]; ok {
			t.Errorf("a prebuilt worker must not get %s", k)
		}
	}
	cron := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "nightly"}, Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeCronJob}}
	if got := envMap(platformEnv(cron, platformEnvInputs{})); got["RENDER_SERVICE_TYPE"] != "cron" || got["RENDER_DISCOVERY_SERVICE"] != "" {
		t.Errorf("cron env = %v", got)
	}
	private := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "api"}, Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypePrivateService}}
	if got := envMap(platformEnv(private, platformEnvInputs{baseDomain: "onbex.co"})); got["RENDER_SERVICE_TYPE"] != "pserv" || got["RENDER_EXTERNAL_HOSTNAME"] != "" {
		t.Errorf("a private service is never public: %v", got)
	}
}

func TestWithPlatformEnvOwnsItsNamesAndKeepsPortLast(t *testing.T) {
	env := withPlatformEnv(
		[]corev1.EnvVar{{Name: "A", Value: "1"}, {Name: "RENDER_GIT_BRANCH", Value: "stale"}, {Name: portEnvName, Value: "3000"}},
		[]corev1.EnvVar{{Name: "RENDER", Value: "true"}, {Name: "RENDER_GIT_BRANCH", Value: "main"}})
	if len(env) != 4 || env[0].Name != "A" || env[3].Name != portEnvName || envMap(env)["RENDER_GIT_BRANCH"] != "main" {
		t.Fatalf("env = %+v", env)
	}
}

func TestBuildPlatformEnvHasNoInstanceOrConcurrency(t *testing.T) {
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web"}, Spec: appv1alpha1.AppSpec{Repo: "https://github.com/acme/web"}}
	got := envMap(withBuildPlatformEnv([]corev1.EnvVar{{Name: "GREETING", Value: "hi"}, {Name: "RENDER", Value: "spoof"}}, app, "onbex.co"))
	if got["GREETING"] != "hi" || got["RENDER"] != "true" || got["RENDER_GIT_REPO_SLUG"] != "acme/web" {
		t.Fatalf("build env = %v", got)
	}
	for _, k := range []string{"RENDER_INSTANCE_ID", "RENDER_WEB_CONCURRENCY"} {
		if _, ok := got[k]; ok {
			t.Errorf("a build must not get %s", k)
		}
	}
}

var _ = Describe("Render platform environment (w5/m148)", func() {
	ctx := context.Background()
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes, BaseDomain: "onbex.test"}
	})
	reconcileN := func(nn types.NamespacedName) {
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}

	It("projects Render's variables and a discovery Service that selects every instance", func() {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "platenv-web", Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, Tier: "pro", Expose: true},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		nn := types.NamespacedName{Name: app.Name, Namespace: "default"}
		DeferCleanup(func() {
			if got := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, got) == nil {
				Expect(k8sClient.Delete(ctx, got)).To(Succeed())
				reconcileN(nn)
			}
		})
		reconcileN(nn)

		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		c := dep.Spec.Template.Spec.Containers[0]
		env := envMap(c.Env)
		Expect(env).To(HaveKeyWithValue("RENDER", "true"))
		Expect(env).To(HaveKeyWithValue("RENDER_SERVICE_TYPE", "web"))
		Expect(env).To(HaveKeyWithValue("RENDER_EXTERNAL_URL", "https://platenv-web.onbex.test"))
		Expect(env).To(HaveKeyWithValue("RENDER_DISCOVERY_SERVICE", "platenv-web-discovery"))
		Expect(c.Env[len(c.Env)-1].Name).To(Equal(portEnvName))

		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "platenv-web-discovery", Namespace: "default"}, svc)).To(Succeed())
		Expect(svc.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone))
		Expect(svc.Spec.Selector).To(HaveKeyWithValue(labelApp, "platenv-web"))
		Expect(svc.Spec.Selector).NotTo(HaveKey(labelRevision), "every instance, across revisions")
		got := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, got)).To(Succeed())
		Expect(metav1.IsControlledBy(svc, got)).To(BeTrue())

		By("a converged App re-reconciles without rewriting the discovery Service")
		rv := svc.ResourceVersion
		reconcileN(nn)
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "platenv-web-discovery", Namespace: "default"}, svc)).To(Succeed())
		Expect(svc.ResourceVersion).To(Equal(rv))
	})

	It("leaves a discovery name another object owns alone and gives a cron job none", func() {
		squat := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "platenv-squat-discovery", Namespace: "default"},
			Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 80}}},
		}
		Expect(k8sClient.Create(ctx, squat)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, squat) })
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "platenv-squat", Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		nn := types.NamespacedName{Name: app.Name, Namespace: "default"}
		DeferCleanup(func() {
			if got := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, got) == nil {
				Expect(k8sClient.Delete(ctx, got)).To(Succeed())
				reconcileN(nn)
			}
		})
		reconcileN(nn)
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: squat.Name, Namespace: "default"}, squat)).To(Succeed())
		Expect(squat.Spec.ClusterIP).NotTo(Equal(corev1.ClusterIPNone), "an unowned Service is never rewritten")

		cron := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "platenv-cron", Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeCronJob, Image: "busybox", Schedule: "*/5 * * * *", StartCommand: "true"},
		}
		Expect(k8sClient.Create(ctx, cron)).To(Succeed())
		cnn := types.NamespacedName{Name: cron.Name, Namespace: "default"}
		DeferCleanup(func() {
			if got := (&appv1alpha1.App{}); k8sClient.Get(ctx, cnn, got) == nil {
				Expect(k8sClient.Delete(ctx, got)).To(Succeed())
				reconcileN(cnn)
			}
		})
		reconcileN(cnn)
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "platenv-cron-discovery", Namespace: "default"}, &corev1.Service{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "a cron job has no discovery Service")
	})
})

func TestUndocumentedRenderNamesStayTheUsersAndUnexposedHostsAreNotAdvertised(t *testing.T) {
	env := withPlatformEnv(
		[]corev1.EnvVar{{Name: "RENDER_API_KEY", Value: "user"}, {Name: portEnvName, Value: "3000"}},
		platformEnv(&appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web"}}, platformEnvInputs{baseDomain: "onbex.co"}))
	got := envMap(env)
	if got["RENDER_API_KEY"] != "user" {
		t.Errorf("a user's own RENDER_API_KEY must survive, env = %v", got)
	}
	if _, ok := got["RENDER_EXTERNAL_URL"]; ok {
		t.Errorf("an unexposed web service has no routed platform host, env = %v", got)
	}
}
