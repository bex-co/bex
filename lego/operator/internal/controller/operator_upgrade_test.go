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
	"maps"
	"slices"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Two operator releases: every helper a tenant pod runs from bex's own image
// moves from operatorImageA to operatorImageB between them, while the code
// those helpers run stays the same.
const (
	operatorImageA = "ghcr.io/bex-co/bex-operator@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	operatorImageB = "ghcr.io/bex-co/bex-operator@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	// operatorImageC is a later release whose Key Value proxy binary differs.
	operatorImageC = "ghcr.io/bex-co/bex-operator@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

// Render restarts a service's instances, and empties its edge cache, on the
// customer's own deploys, restarts, settings changes and purges, never on a
// release of Render's platform. bex's platform releases move the operator's
// own image several times a day, and tenant pods run helpers from it: the
// health-probe installer, the edge-cache tier's proxy, the Key Value proxy.
// So a new operator image alone must change no long-lived tenant pod
// template, of any kind the operator writes: a changed template rolls every
// instance, and a changed tier empties the cache (w6/m484, after w8/m133,
// w6/m469 and w3/m489). Every App passes through the whole reconciler, and
// every Deployment and StatefulSet in its namespace is compared, so the next
// field or workload that takes the operator's image fails here, wherever it
// is wired.
func TestANewOperatorImageChangesNoLongLivedTenantPodTemplate(t *testing.T) {
	tier := "Deployment " + edgeCacheTierName("web")
	for name, tc := range map[string]struct {
		app *appv1alpha1.App
		// helpers are the containers of each workload that run the
		// operator's image: the guard must see them, or it guards nothing.
		helpers map[string][]string
	}{
		"an edge-cached web service with a health check": {
			app: paidWebApp(func(a *appv1alpha1.App) {
				a.Spec.CacheProfile = appv1alpha1.CacheProfileStatic
				a.Spec.HealthCheckPath = "/healthz"
			}),
			helpers: map[string][]string{"Deployment web": {healthProbeInitName}, tier: {edgeCacheContainerName}},
		},
		"an edge-cached web service with a disk": {
			app: paidWebApp(func(a *appv1alpha1.App) {
				a.Spec.CacheProfile = appv1alpha1.CacheProfileAll
				a.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}
			}),
			helpers: map[string][]string{tier: {edgeCacheContainerName}},
		},
		"a web service": {
			app:     paidWebApp(),
			helpers: map[string][]string{"Deployment web": {healthProbeInitName}},
		},
		"a free web service": {
			app:     activeApp("tea-m484"),
			helpers: map[string][]string{"Deployment web": {healthProbeInitName}},
		},
		"a private service": {
			app: paidWebApp(func(a *appv1alpha1.App) {
				a.Spec.Type, a.Spec.Expose = appv1alpha1.TypePrivateService, false
				a.Spec.HealthCheckPath = "/healthz"
			}),
			helpers: map[string][]string{"Deployment web": {healthProbeInitName}},
		},
		"a background worker": {app: heldWorkerApp("tea-m484"), helpers: map[string][]string{}},
	} {
		t.Run(name, func(t *testing.T) {
			r, cl, nn := lifecycleFixture(t, tc.app)
			r.BackupHelperImage = operatorImageA
			serveReleaseOne(t, r, cl, nn)
			before := tenantPodTemplates(t, cl, nn.Namespace)
			if got := containersRunningPerWorkload(before, operatorImageA); !maps.EqualFunc(got, tc.helpers, slices.Equal) {
				t.Fatalf("setup: the containers running the operator's image are %v, want %v", got, tc.helpers)
			}

			r.BackupHelperImage = operatorImageB
			reconcileTwice(t, r, nn)
			assertNoTemplateMoved(t, before, tenantPodTemplates(t, cl, nn.Namespace))
		})
	}

	t.Run("a Key Value store and its proxy", func(t *testing.T) {
		kv := &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: "red-m484", Namespace: "tea-m484", Labels: map[string]string{labelWorkspace: "tea-m484"}},
			Spec:       appv1alpha1.KeyValueSpec{Name: "cache", Plan: "starter"},
		}
		cl := fake.NewClientBuilder().WithScheme(wakeScheme()).WithObjects(kv).
			WithStatusSubresource(&appv1alpha1.KeyValue{}).Build()
		release := func(image string) map[string]corev1.PodTemplateSpec {
			t.Helper()
			r := &KeyValueReconciler{Client: cl, Scheme: cl.Scheme(), InternalProxyImage: image, BackupHelperImage: image}
			for range 3 {
				if _, err := r.Reconcile(t.Context(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(kv)}); err != nil {
					t.Fatalf("reconcile: %v", err)
				}
			}
			return tenantPodTemplates(t, cl, kv.Namespace)
		}
		before := release(operatorImageA)
		if _, ok := before["StatefulSet "+kv.Name]; !ok {
			t.Fatalf("setup: no StatefulSet for the store among %v", slices.Collect(maps.Keys(before)))
		}
		want := map[string][]string{"Deployment " + keyValueInternalProxyName(kv): {kvInternalProxyContainer}}
		if got := containersRunningPerWorkload(before, operatorImageA); !maps.EqualFunc(got, want, slices.Equal) {
			t.Fatalf("setup: the containers running the operator's image are %v, want %v", got, want)
		}
		assertNoTemplateMoved(t, before, release(operatorImageB))

		// A release that changes the proxy's code and adds a field to its
		// rendering moves no template either: the proxy keeps the one it runs
		// until the store's own change rolls it (w6/m517). The stored template
		// stands for the older rendering, without the field.
		proxy := &appsv1.Deployment{}
		if err := cl.Get(t.Context(), client.ObjectKey{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}, proxy); err != nil {
			t.Fatal(err)
		}
		c := &proxy.Spec.Template.Spec.Containers[0]
		c.Env = slices.DeleteFunc(c.Env, func(e corev1.EnvVar) bool { return e.Name == "BEX_KV_DRAIN_SECONDS" })
		if err := cl.Update(t.Context(), proxy); err != nil {
			t.Fatal(err)
		}
		before = tenantPodTemplates(t, cl, kv.Namespace)
		assertNoTemplateMoved(t, before, release(operatorImageC))
	})
}

// tenantPodTemplates is every long-lived workload's pod template in
// namespace, keyed by kind and name: each Deployment and StatefulSet the
// reconciler wrote, so a workload it starts writing later is guarded too.
func tenantPodTemplates(t *testing.T, cl client.Client, namespace string) map[string]corev1.PodTemplateSpec {
	t.Helper()
	var deployments appsv1.DeploymentList
	var statefulSets appsv1.StatefulSetList
	if err := cl.List(t.Context(), &deployments, client.InNamespace(namespace)); err != nil {
		t.Fatal(err)
	}
	if err := cl.List(t.Context(), &statefulSets, client.InNamespace(namespace)); err != nil {
		t.Fatal(err)
	}
	templates := map[string]corev1.PodTemplateSpec{}
	for _, d := range deployments.Items {
		templates["Deployment "+d.Name] = d.Spec.Template
	}
	for _, s := range statefulSets.Items {
		templates["StatefulSet "+s.Name] = s.Spec.Template
	}
	return templates
}

// assertNoTemplateMoved fails for each workload in before whose pod template
// after changed or lost.
func assertNoTemplateMoved(t *testing.T, before, after map[string]corev1.PodTemplateSpec) {
	t.Helper()
	for workload, tmpl := range before {
		now, ok := after[workload]
		if !ok {
			t.Errorf("a new operator image alone removed %s", workload)
			continue
		}
		if diff := podTemplateDiff(tmpl, now); diff != "" {
			t.Errorf("a new operator image alone rolled %s: %s", workload, diff)
		}
	}
}

// paidWebApp is a serving web service on a paid plan, which never sleeps.
func paidWebApp(mutate ...func(*appv1alpha1.App)) *appv1alpha1.App {
	app := activeApp("tea-m484")
	app.Spec.Tier, app.Spec.IdleTTLSeconds = "pro", 0
	for _, m := range mutate {
		m(app)
	}
	return app
}

// containersRunningPerWorkload names, for each workload in templates that
// runs image, its init and app containers that do, in pod order.
func containersRunningPerWorkload(templates map[string]corev1.PodTemplateSpec, image string) map[string][]string {
	running := map[string][]string{}
	for workload, tmpl := range templates {
		for _, c := range slices.Concat(tmpl.Spec.InitContainers, tmpl.Spec.Containers) {
			if c.Image == image {
				running[workload] = append(running[workload], c.Name)
			}
		}
	}
	return running
}
