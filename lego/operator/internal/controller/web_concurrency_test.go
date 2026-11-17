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

// w8/273, w32/m424: Render pins Rust's ROCKET_PORT, Ruby's PASSENGER_PORT
// and the bind in Python's GUNICORN_CMD_ARGS to its default port, so the
// usual server listens where the platform routes. bex's value is the serving
// port, which can differ from PORT (a detected port keeps PORT at 10000), and
// sits just before PORT.
func TestWithServingPortDefaultFollowsTheServingPortBeforePort(t *testing.T) {
	for _, tc := range []struct{ runtime, name, value string }{
		{"python", "GUNICORN_CMD_ARGS", "--preload --access-logfile - --bind=0.0.0.0:3000"},
		{"ruby", "PASSENGER_PORT", "3000"},
		{"rust", "ROCKET_PORT", "3000"},
	} {
		env := withServingPortDefault([]corev1.EnvVar{{Name: "A", Value: "1"}, {Name: portEnvName, Value: "10000"}}, tc.runtime, 3000, true)
		if len(env) != 3 || env[1].Name != tc.name || env[1].Value != tc.value || env[2] != (corev1.EnvVar{Name: portEnvName, Value: "10000"}) {
			t.Errorf("%s: env = %+v, want A, %s=%s, PORT=10000", tc.runtime, env, tc.name, tc.value)
		}
		if got := withServingPortDefault(env[:2:2], tc.runtime, 3000, false); len(got) != 2 {
			t.Errorf("%s: a service that is not eligible must keep its env, got %+v", tc.runtime, got)
		}
	}
}

func TestServingPortDefaultIsForNativeServicesThatSetNone(t *testing.T) {
	r := &AppReconciler{}
	for _, tc := range []struct {
		name    string
		runtime string
		typ     string
		env     []appv1alpha1.EnvVar
		want    bool
	}{
		{"a Rust web service", "rust", appv1alpha1.TypeWebService, nil, true},
		{"a Rust private service", "rust", appv1alpha1.TypePrivateService, nil, true},
		{"a Ruby web service", "ruby", appv1alpha1.TypeWebService, nil, true},
		{"a Ruby service of the default type", "ruby", "", nil, true},
		{"a Python private service", "python", appv1alpha1.TypePrivateService, nil, true},
		{"a Rust worker has no port", "rust", appv1alpha1.TypeBackgroundWorker, nil, false},
		{"a Python worker keeps the image's", "python", appv1alpha1.TypeBackgroundWorker, nil, false},
		{"a Ruby cron job keeps the image's", "ruby", appv1alpha1.TypeCronJob, nil, false},
		{"a runtime with no port variable", "node", appv1alpha1.TypeWebService, nil, false},
		{"a Docker service", "docker", appv1alpha1.TypeWebService, nil, false},
		{"an image service", "", appv1alpha1.TypeWebService, nil, false},
		{"its own value wins", "rust", appv1alpha1.TypeWebService, []appv1alpha1.EnvVar{{Name: "ROCKET_PORT", Value: "8000"}}, false},
		{"an explicit empty value is still its own", "rust", appv1alpha1.TypeWebService, []appv1alpha1.EnvVar{{Name: "ROCKET_PORT", Value: ""}}, false},
		{"Passenger's own port wins", "ruby", appv1alpha1.TypeWebService, []appv1alpha1.EnvVar{{Name: "PASSENGER_PORT", Value: "4000"}}, false},
		{"empty Gunicorn flags are the service's own", "python", appv1alpha1.TypeWebService, []appv1alpha1.EnvVar{{Name: "GUNICORN_CMD_ARGS", Value: ""}}, false},
		{"another runtime's variable is not its own", "python", appv1alpha1.TypeWebService, []appv1alpha1.EnvVar{{Name: "PASSENGER_PORT", Value: "4000"}}, true},
	} {
		app := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Runtime: tc.runtime, Type: tc.typ, Env: tc.env}}
		got, err := r.servingPortDefault(context.Background(), app)
		if err != nil || got != tc.want {
			t.Errorf("%s: default = %v (err %v), want %v", tc.name, got, err, tc.want)
		}
	}
}

var _ = Describe("WEB_CONCURRENCY (w5/m147) and serving-port (w32/m424) defaults", func() {
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

		serveEnvtestRelease(nn, func() { reconcileN(nn) })
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

	It("binds a Python or Ruby web service's server on its serving port unless the service or a group sets its own", func() {
		for _, tc := range []struct{ name, runtime, env, value string }{
			{"servingport-python", "python", "GUNICORN_CMD_ARGS", "--preload --access-logfile - --bind=0.0.0.0:3000"},
			{"servingport-ruby", "ruby", "PASSENGER_PORT", "3000"},
		} {
			nn := create(tc.name, func(a *appv1alpha1.App) { a.Spec.Runtime = tc.runtime })
			env := envOf(nn)
			Expect(env).To(HaveKeyWithValue(tc.env, tc.value), "%s: the serving port, 3000, not Render's 10000", tc.name)
			Expect(env).To(HaveKeyWithValue(portEnvName, "3000"))

			secret := "evg-" + tc.name + "-env"
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secret, Namespace: "default"},
				Data:       map[string][]byte{tc.env: []byte("")},
			})).To(Succeed())
			grouped := create(tc.name+"-group", func(a *appv1alpha1.App) {
				a.Spec.Runtime = tc.runtime
				a.Spec.EnvFromSecrets = []string{secret}
			})
			Expect(envOf(grouped)).NotTo(HaveKey(tc.env), "%s: a container env entry would override the group's empty value", tc.name)
		}
	})
})
