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
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestWebConcurrencyEligibleOnlyForNewWebAndPrivateServices(t *testing.T) {
	after := metav1.NewTime(webConcurrencyCutoff.Add(time.Hour))
	before := metav1.NewTime(webConcurrencyCutoff.Add(-time.Hour))
	for _, tc := range []struct {
		typ     string
		created metav1.Time
		want    bool
	}{
		{"", after, true},
		{appv1alpha1.TypeWebService, after, true},
		{appv1alpha1.TypePrivateService, after, true},
		{appv1alpha1.TypeBackgroundWorker, after, false},
		{appv1alpha1.TypeCronJob, after, false},
		{appv1alpha1.TypeStaticSite, after, false},
		{appv1alpha1.TypeWebService, before, false},
	} {
		app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{CreationTimestamp: tc.created}, Spec: appv1alpha1.AppSpec{Type: tc.typ}}
		if got := webConcurrencyEligible(app); got != tc.want {
			t.Errorf("type %q created %s: eligible = %v, want %v", tc.typ, tc.created.UTC(), got, tc.want)
		}
	}
}

func TestWithWebConcurrencyKeepsPortLast(t *testing.T) {
	env := withWebConcurrency([]corev1.EnvVar{{Name: "A", Value: "1"}, {Name: portEnvName, Value: "3000"}}, 2)
	if len(env) != 3 || env[1].Name != webConcurrencyEnvName || env[1].Value != "2" || env[2].Name != portEnvName {
		t.Fatalf("env = %+v, want A, WEB_CONCURRENCY=2, PORT", env)
	}
	if got := withWebConcurrency(env[:1], 0); len(got) != 1 {
		t.Fatalf("n=0 must leave env alone, got %+v", got)
	}
}

var _ = Describe("WEB_CONCURRENCY default (w5/m147)", func() {
	ctx := context.Background()
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	})
	reconcileN := func(nn types.NamespacedName) {
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	envOf := func(nn types.NamespacedName) map[string]string {
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		out := map[string]string{}
		for _, e := range dep.Spec.Template.Spec.Containers[0].Env {
			out[e.Name] = e.Value
		}
		return out
	}
	create := func(name string, mutate func(*appv1alpha1.App)) types.NamespacedName {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, Tier: "pro"},
		}
		if mutate != nil {
			mutate(app)
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		DeferCleanup(func() {
			if got := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, got) == nil {
				Expect(k8sClient.Delete(ctx, got)).To(Succeed())
				reconcileN(nn)
			}
		})
		reconcileN(nn)
		return nn
	}

	It("gives a new web service its plan's CPU count, and a plan change rolls the new value out", func() {
		nn := create("webconc-plan", nil)
		Expect(envOf(nn)).To(HaveKeyWithValue(webConcurrencyEnvName, "2"))

		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		app.Spec.Tier = "pro-ultra"
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(nn)
		Expect(envOf(nn)).To(HaveKeyWithValue(webConcurrencyEnvName, "8"))
	})

	It("never sets it on a background worker", func() {
		nn := create("webconc-worker", func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeBackgroundWorker })
		Expect(envOf(nn)).NotTo(HaveKey(webConcurrencyEnvName))
	})

	It("yields to the service's own literal value, an empty one included", func() {
		nn := create("webconc-literal", func(a *appv1alpha1.App) {
			a.Spec.Env = []appv1alpha1.EnvVar{{Name: webConcurrencyEnvName, Value: ""}}
		})
		env := envOf(nn)
		Expect(env).To(HaveKeyWithValue(webConcurrencyEnvName, ""), "only the service's own entry, unchanged")
	})

	It("yields to a value in the service's secret-backed env or a linked group", func() {
		for _, tc := range []struct{ name, secret string }{{"webconc-own", "webconc-own-env"}, {"webconc-group", "evg-webconc-env"}} {
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: tc.secret, Namespace: "default"},
				Data:       map[string][]byte{webConcurrencyEnvName: []byte("5")},
			})).To(Succeed())
			nn := create(tc.name, func(a *appv1alpha1.App) {
				if tc.name == "webconc-own" {
					a.Spec.EnvFromSecret = tc.secret
				} else {
					a.Spec.EnvFromSecrets = []string{tc.secret}
				}
			})
			Expect(envOf(nn)).NotTo(HaveKey(webConcurrencyEnvName),
				"%s: a container env entry would override the Secret's 5", tc.name)
		}
	})
})
