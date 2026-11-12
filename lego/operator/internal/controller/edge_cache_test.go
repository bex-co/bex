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
	"reflect"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("Edge caching (w5/m151)", func() {
	ctx := context.Background()
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes, BackupHelperImage: "ghcr.io/bex-co/bex:test"}
	})
	reconcileN := func(nn types.NamespacedName) {
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	create := func(name, tier, profile string) types.NamespacedName {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, Tier: tier, CacheProfile: profile},
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
	containers := func(nn types.NamespacedName) []corev1.Container {
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		return dep.Spec.Template.Spec.Containers
	}

	It("fronts a paid web service with the cache and points its Service at it", func() {
		nn := create("edgecache-pro", "pro", appv1alpha1.CacheProfileStatic)
		cs := containers(nn)
		Expect(cs).To(HaveLen(2))
		Expect(cs[1].Name).To(Equal(edgeCacheContainerName))
		Expect(envMap(cs[1].Env)).To(HaveKeyWithValue(edgeCacheUpstreamEnv, "http://127.0.0.1:3000"))
		Expect(envMap(cs[1].Env)).To(HaveKeyWithValue("BEX_EDGE_CACHE_PROFILE", "origin-controlled"))
		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
		Expect(svc.Spec.Ports[0].TargetPort.StrVal).To(Equal(edgeCachePortName))
		Expect(cs[0].ReadinessProbe).NotTo(BeNil(), "the app keeps its own health check")

		By("a manual purge rolls the pods, emptying the cache")
		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		app.Annotations = map[string]string{appv1alpha1.CachePurgedAtAnnotation: "2026-10-21T00:00:00Z"}
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(nn)
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		Expect(dep.Spec.Template.Annotations).To(HaveKeyWithValue(appv1alpha1.CachePurgedAtAnnotation, "2026-10-21T00:00:00Z"))

		By("turning caching off removes the proxy and targets the app again")
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		app.Spec.CacheProfile = appv1alpha1.CacheProfileNone
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(nn)
		Expect(containers(nn)).To(HaveLen(1))
		Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
		Expect(svc.Spec.Ports[0].TargetPort.StrVal).To(Equal(servingPortName))
	})

	It("never caches a free web service, as on Render", func() {
		nn := create("edgecache-free", "free", appv1alpha1.CacheProfileAll)
		Expect(containers(nn)).To(HaveLen(1))
	})

	It("fronts a paid web service on its image's port with the cache on its resolved primary", func() {
		nn := types.NamespacedName{Name: "edgecache-image", Namespace: "default"}
		r.HTTPClient = imageConfigRegistry(GinkgoT(), `"80/tcp":{},"8080/tcp":{}`)
		run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec: appv1alpha1.AppSpec{
				Image: "docker.io/example/cached:v1", Type: appv1alpha1.TypeWebService,
				PortMode: appv1alpha1.PortModeWebImageV1, Replicas: 1,
				Tier: "pro", CacheProfile: appv1alpha1.CacheProfileStatic,
			},
		})).To(Succeed())
		DeferCleanup(func() { deleteOwner(ctx, envtestApp(nn), run) })
		cache := intstr.FromString(edgeCachePortName)
		upstream := func() string {
			GinkgoHelper()
			cs := containers(nn)
			Expect(cs).To(HaveLen(2))
			Expect(cs[1].Name).To(Equal(edgeCacheContainerName))
			return envMap(cs[1].Env)[edgeCacheUpstreamEnv]
		}

		By("while HTTP discovery chooses among the declared ports, the cache fronts the provisional primary")
		reconcileToFixedPoint(run)
		Expect(webImageHTTPSelectionPending(envtestApp(nn))).To(BeTrue(), "the test must exercise a pending discovery")
		Expect(upstream()).To(Equal("http://127.0.0.1:80"))
		assertImagePortRuntime(nn, 80)
		Expect(servicePortTargets(nn.Name)).To(Equal(map[int32]intstr.IntOrString{80: cache, 8080: intstr.FromInt32(8080), 10000: cache}))

		By("a choice that moves the primary re-projects the cache; a pod still caching the old port keeps the Service direct")
		stale := simulateTemplatePod(nn, "provisional", true)
		app := envtestApp(nn)
		app.Status.ImageNetwork.PrimaryPort, app.Status.ImageNetwork.HTTPSelection = 8080, appv1alpha1.HTTPSelectionDetected
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		Expect(upstream()).To(Equal("http://127.0.0.1:8080"))
		assertImagePortRuntime(nn, 8080)
		Expect(servicePortTargets(nn.Name)).To(Equal(map[int32]intstr.IntOrString{80: intstr.FromInt32(80), 8080: intstr.FromInt32(8080), 10000: intstr.FromInt32(8080)}))

		By("once that pod is gone the Service targets the cache on the chosen primary")
		Expect(k8sClient.Delete(ctx, stale)).To(Succeed())
		reconcileToFixedPoint(run)
		markEnvtestDeploymentReady(nn)
		reconcileToFixedPoint(run)
		Expect(envtestApp(nn).Status.ServingNetwork).NotTo(BeNil(), "the release must have served")
		Expect(servicePortTargets(nn.Name)).To(Equal(map[int32]intstr.IntOrString{80: intstr.FromInt32(80), 8080: cache, 10000: cache}))

		By("a manual purge rolls the pods, emptying the cache")
		app = envtestApp(nn)
		app.Annotations = map[string]string{appv1alpha1.CachePurgedAtAnnotation: "2026-11-12T00:00:00Z"}
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		Expect(envtestDeployment(nn).Spec.Template.Annotations).To(HaveKeyWithValue(appv1alpha1.CachePurgedAtAnnotation, "2026-11-12T00:00:00Z"))

		By("turning caching off removes the sidecar and targets the app again")
		app = envtestApp(nn)
		app.Spec.CacheProfile = appv1alpha1.CacheProfileNone
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		Expect(containers(nn)).To(HaveLen(1))
		Expect(servicePortTargets(nn.Name)).To(Equal(map[int32]intstr.IntOrString{80: intstr.FromInt32(80), 8080: intstr.FromInt32(8080), 10000: intstr.FromInt32(8080)}))
	})
})

// The sidecar listens on 18014, a reserved port (IsReservedImagePort): a
// caching image-port web service whose image exposes it resolves no network,
// so no pod is projected with the app and the cache on one port, and an
// explicit port cannot select it (a PORT of 18014 is ignored, envPort).
func TestImageExposingTheEdgeCachePortResolvesNoNetwork(t *testing.T) {
	cached := func(a *appv1alpha1.App) {
		a.Name, a.Generation = "cached", 1
		a.Spec.PortMode, a.Spec.Tier, a.Spec.CacheProfile = appv1alpha1.PortModeWebImageV1, "pro", appv1alpha1.CacheProfileAll
	}
	r := &AppReconciler{Registry: "registry.test", HTTPClient: imageConfigRegistry(t, `"8080/tcp":{},"18014/tcp":{}`)}
	app := projectionApp(cached)
	if _, _, err := r.resolveImageNetwork(t.Context(), app, "registry.test/cached:gen-1"); err == nil || !strings.Contains(err.Error(), "18014 is reserved") {
		t.Fatalf("resolveImageNetwork error = %v, want the reserved 18014 refused", err)
	}
	if app.Status.ImageNetwork != nil {
		t.Fatalf("network = %+v, want none resolved", app.Status.ImageNetwork)
	}
	r.HTTPClient = imageConfigRegistry(t, `"8080/tcp":{}`)
	app = projectionApp(cached, func(a *appv1alpha1.App) {
		a.Spec.PortMode, a.Spec.Port = appv1alpha1.PortModeWebImageConfiguredV1, edgeCachePort
	})
	if _, _, err := r.resolveImageNetwork(t.Context(), app, "registry.test/cached:gen-1"); err == nil {
		t.Fatal("an explicit port of 18014 must not become the primary the cache would share")
	}
}

// edgeCacheTestPod is a pod of app's release revision: an app container with
// appPorts, plus a sidecar in front of cached unless cached is 0.
func edgeCacheTestPod(app *appv1alpha1.App, name, revision string, cached int, appPorts []corev1.ContainerPort, mutate ...func(*corev1.Pod)) client.Object {
	labels := appPodLabels(app, false)
	labels[labelRevision] = revision
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace, Labels: labels},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: appContainerName, Ports: appPorts}}},
	}
	if cached > 0 {
		p.Spec.Containers = append(p.Spec.Containers, edgeCacheContainer(app, "ghcr.io/bex-co/bex:test", cached, drainSeconds))
	}
	for _, m := range mutate {
		m(p)
	}
	return p
}

func terminatingPod(p *corev1.Pod) {
	p.DeletionTimestamp, p.Finalizers = &metav1.Time{Time: time.Now()}, []string{"test.bex.co/hold"}
}

func endedPod(phase corev1.PodPhase) func(*corev1.Pod) {
	return func(p *corev1.Pod) { p.Status.Phase = phase }
}

// appliedServiceTargets applies app's CR-named Service over pods and returns it
// with its port targets.
func appliedServiceTargets(t *testing.T, app *appv1alpha1.App, port int, pods ...client.Object) (*corev1.Service, map[int32]intstr.IntOrString) {
	t.Helper()
	scheme := wakeScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pods...).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme, BackupHelperImage: "ghcr.io/bex-co/bex:test"}
	if err := r.applyClusterIPService(t.Context(), app, app.Name, port); err != nil {
		t.Fatal(err)
	}
	svc := &corev1.Service{}
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, svc); err != nil {
		t.Fatal(err)
	}
	return svc, portTargets(svc)
}

// A pod that ended (an eviction, a graceful node shutdown) keeps its labels
// and is never deleted by its ReplicaSet, but it is no endpoint: without a
// sidecar it must not keep a caching Service off the cache, on either path.
func TestEndedPodsDoNotHoldTheEdgeCacheBack(t *testing.T) {
	named := []corev1.ContainerPort{{Name: servingPortName, ContainerPort: 3000}}
	configured := projectionApp(func(a *appv1alpha1.App) {
		a.Namespace, a.Spec.Port, a.Spec.Tier, a.Spec.CacheProfile = "tea-a", 3000, "pro", appv1alpha1.CacheProfileStatic
	})
	for _, phase := range []corev1.PodPhase{corev1.PodFailed, corev1.PodSucceeded} {
		_, got := appliedServiceTargets(t, configured, 3000,
			edgeCacheTestPod(configured, "new", releaseRevision(configured), 3000, named),
			edgeCacheTestPod(configured, "evicted", releaseRevision(configured), 0, named, endedPod(phase)))
		if cache := intstr.FromString(edgeCachePortName); got[3000] != cache || got[appv1alpha1.PrivateHTTPPort] != cache {
			t.Fatalf("configured port with a %s pod: targets = %v, want 3000 and 10000 on the cache", phase, got)
		}
	}
	_, got := appliedServiceTargets(t, configured, 3000,
		edgeCacheTestPod(configured, "new", releaseRevision(configured), 3000, named),
		edgeCacheTestPod(configured, "plain", releaseRevision(configured), 0, named))
	if got[3000] != intstr.FromString(servingPortName) {
		t.Fatalf("configured port with a running pod without the cache: targets = %v, want the named listener", got)
	}
}

// Through a port change routed early (w4/428), the image-port Service pins the
// new revision and aliases the replaced primary to the new one, so the cache
// serves both, and the private-network 10000. Any routable pinned pod that does
// not cache the primary (another port's cache, or none yet) keeps every port
// direct; a terminating or ended pod does not count.
func TestImagePortServiceTargetsTheEdgeCacheOnItsPrimary(t *testing.T) {
	prior := &appv1alpha1.ImageNetworkStatus{Image: "nginx@sha256:a", Revision: "rev-1", PrimaryPort: 80, Ports: []int32{80},
		HTTPSelection: appv1alpha1.HTTPSelectionDetected, ConfiguredPort: new(int32(0))}
	serving := &appv1alpha1.ImageNetworkStatus{Image: "nginx@sha256:b", Revision: "rev-2", PrimaryPort: 8080, Ports: []int32{8080, 9000},
		HTTPSelection: appv1alpha1.HTTPSelectionDetected, ConfiguredPort: new(int32(0))}
	cache, direct := intstr.FromString(edgeCachePortName), intstr.FromInt32(8080)
	pod := func(app *appv1alpha1.App, name, revision string, cached int, mutate ...func(*corev1.Pod)) client.Object {
		return edgeCacheTestPod(app, name, revision, cached, nil, mutate...)
	}
	for name, tc := range map[string]struct {
		profile string
		pods    func(*appv1alpha1.App) []client.Object
		want    map[int32]intstr.IntOrString
	}{
		"the pinned pods cache the new primary": {
			profile: appv1alpha1.CacheProfileStatic,
			pods: func(a *appv1alpha1.App) []client.Object {
				return []client.Object{pod(a, "new", "rev-2", 8080), pod(a, "old", "rev-1", 80), pod(a, "leaving", "rev-2", 80, terminatingPod)}
			},
			want: map[int32]intstr.IntOrString{8080: cache, 80: cache, 9000: intstr.FromInt32(9000), 10000: cache},
		},
		"an ended pinned pod without a cache": {
			profile: appv1alpha1.CacheProfileStatic,
			pods: func(a *appv1alpha1.App) []client.Object {
				return []client.Object{pod(a, "new", "rev-2", 8080), pod(a, "evicted", "rev-2", 0, endedPod(corev1.PodFailed)),
					pod(a, "shut-down", "rev-2", 0, endedPod(corev1.PodSucceeded))}
			},
			want: map[int32]intstr.IntOrString{8080: cache, 80: cache, 9000: intstr.FromInt32(9000), 10000: cache},
		},
		"a pinned pod caches another port": {
			profile: appv1alpha1.CacheProfileStatic,
			pods: func(a *appv1alpha1.App) []client.Object {
				return []client.Object{pod(a, "new", "rev-2", 8080), pod(a, "stale", "rev-2", 80)}
			},
			want: map[int32]intstr.IntOrString{8080: direct, 80: direct, 9000: intstr.FromInt32(9000), 10000: direct},
		},
		"a pinned pod has no cache yet": {
			profile: appv1alpha1.CacheProfileStatic,
			pods: func(a *appv1alpha1.App) []client.Object {
				return []client.Object{pod(a, "new", "rev-2", 8080), pod(a, "plain", "rev-2", 0)}
			},
			want: map[int32]intstr.IntOrString{8080: direct, 80: direct, 9000: intstr.FromInt32(9000), 10000: direct},
		},
		"caching is off": {
			profile: appv1alpha1.CacheProfileNone,
			pods: func(a *appv1alpha1.App) []client.Object {
				return []client.Object{pod(a, "new", "rev-2", 8080)}
			},
			want: map[int32]intstr.IntOrString{8080: direct, 80: direct, 9000: intstr.FromInt32(9000), 10000: direct},
		},
	} {
		t.Run(name, func(t *testing.T) {
			app := webImageApp(2)
			app.Namespace = "tea-a"
			app.Spec.Tier, app.Spec.CacheProfile = "pro", tc.profile
			app.Status.PriorServingNetwork, app.Status.ServingNetwork, app.Status.ImageNetwork = prior.DeepCopy(), serving.DeepCopy(), serving.DeepCopy()
			svc, got := appliedServiceTargets(t, app, 8080, tc.pods(app)...)
			if svc.Spec.Selector[labelRevision] != "rev-2" {
				t.Fatalf("selector = %v, want pinned to the promoted rev-2", svc.Spec.Selector)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Service targets = %v, want %v", got, tc.want)
			}
		})
	}
}
