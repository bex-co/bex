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
	"reflect"
	"slices"
	"strconv"
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
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/edgecache"
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
		Expect(envMap(cs[1].Env)).To(HaveKeyWithValue(edgeCacheProfileEnv, "origin-controlled"))
		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
		Expect(svc.Spec.Ports[0].TargetPort.StrVal).To(Equal(edgeCachePortName))
		Expect(cs[0].ReadinessProbe).NotTo(BeNil(), "the app keeps its own health check")

		By("a manual purge reaches the running pod's proxy, and rolls nothing")
		serveEnvtestRelease(nn, func() { reconcileN(nn) })
		pod := simulateTemplatePod(nn, "cached", true)
		template := envtestDeployment(nn).Spec.Template
		generation := envtestApp(nn).Status.ReleaseGeneration
		app := envtestApp(nn)
		stamp := time.Now().Add(time.Second).UTC().Format(time.RFC3339Nano) // after the pod was created
		metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.CachePurgedAtAnnotation, stamp)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(nn)
		Expect(envtestDeployment(nn).Spec.Template).To(Equal(template), "a purge must not roll the pods")
		Expect(envtestPod(pod).Annotations).To(And(
			HaveKeyWithValue(appv1alpha1.CachePurgedAtAnnotation, stamp),
			HaveKeyWithValue(edgecache.ProfileAnnotation, appv1alpha1.CacheProfileStatic)))

		By("a profile switch reaches the proxy the same way, with no release")
		switchProfile := func(profile string) {
			GinkgoHelper()
			app := envtestApp(nn)
			app.Spec.CacheProfile = profile
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileN(nn)
			Expect(envtestDeployment(nn).Spec.Template).To(Equal(template), "a switch to %q must not roll the pods", profile)
			Expect(envtestApp(nn).Status.ReleaseGeneration).To(Equal(generation), "a switch to %q must open no release", profile)
		}
		switchProfile(appv1alpha1.CacheProfileAll)
		Expect(envtestPod(pod).Annotations).To(HaveKeyWithValue(edgecache.ProfileAnnotation, appv1alpha1.CacheProfileAll))
		Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
		Expect(svc.Spec.Ports[0].TargetPort.StrVal).To(Equal(edgeCachePortName))

		By("turning caching off keeps the proxy as a pass-through until the next deploy, and targets the app again")
		switchProfile(appv1alpha1.CacheProfileNone)
		Expect(containers(nn)).To(HaveLen(2))
		Expect(envtestPod(pod).Annotations).To(HaveKeyWithValue(edgecache.ProfileAnnotation, appv1alpha1.CacheProfileNone))
		Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
		Expect(svc.Spec.Ports[0].TargetPort.StrVal).To(Equal(servingPortName))

		By("turning it back on while the proxy is still there rolls nothing either")
		switchProfile(appv1alpha1.CacheProfileStatic)
		Expect(envtestPod(pod).Annotations).To(HaveKeyWithValue(edgecache.ProfileAnnotation, appv1alpha1.CacheProfileStatic))
		Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
		Expect(svc.Spec.Ports[0].TargetPort.StrVal).To(Equal(edgeCachePortName))

		By("the next deploy drops a proxy whose caching is off")
		Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
		app = envtestApp(nn)
		app.Spec.CacheProfile = appv1alpha1.CacheProfileNone
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(nn)
		app = envtestApp(nn)
		app.Spec.RestartedAt = "2026-10-21T01:00:00Z"
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(nn)
		Expect(containers(nn)).To(HaveLen(1))
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

		By("a manual purge keeps the pod template")
		template := envtestDeployment(nn).Spec.Template
		app = envtestApp(nn)
		metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.CachePurgedAtAnnotation, "2026-11-12T00:00:00Z")
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		Expect(envtestDeployment(nn).Spec.Template).To(Equal(template), "a purge must not roll the pods")

		By("turning caching off keeps the sidecar as a pass-through and targets the app again")
		app = envtestApp(nn)
		app.Spec.CacheProfile = appv1alpha1.CacheProfileNone
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		Expect(envtestDeployment(nn).Spec.Template).To(Equal(template), "turning caching off must not roll the pods")
		Expect(servicePortTargets(nn.Name)).To(Equal(map[int32]intstr.IntOrString{80: intstr.FromInt32(80), 8080: intstr.FromInt32(8080), 10000: intstr.FromInt32(8080)}))
	})
})

// envtestPod re-reads p.
func envtestPod(p *corev1.Pod) *corev1.Pod {
	GinkgoHelper()
	got := &corev1.Pod{}
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(p), got)).To(Succeed())
	return got
}

// Render documents three reserved ports and refuses nothing else, so a service
// may bind any other. The sidecar has to sit on one of those three: a fourth,
// as 18014 was, is a port Render lets a service use and bex does not
// (w21/076, ADR018).
func TestTheEdgeCacheSharesARenderReservedPortAndTakesNoFourth(t *testing.T) {
	if got := appv1alpha1.ReservedImagePorts(); !slices.Equal(got, []int32{18012, 18013, 19099}) {
		t.Fatalf("reserved ports = %v, want exactly Render's 18012, 18013 and 19099", got)
	}
	if !appv1alpha1.IsReservedImagePort(edgeCachePort) {
		t.Fatalf("the sidecar's port %d is not reserved, so a service's PORT could collide with it", edgeCachePort)
	}
}

// The sidecar listens on 19099, a port Render reserves too (IsReservedImagePort):
// a caching image-port web service whose image exposes it resolves no network,
// so no pod is projected with the app and the cache on one port, and an
// explicit port cannot select it (a PORT of 19099 is ignored, envPort).
func TestImageExposingTheEdgeCachePortResolvesNoNetwork(t *testing.T) {
	cached := func(a *appv1alpha1.App) {
		a.Name, a.Generation = "cached", 1
		a.Spec.PortMode, a.Spec.Tier, a.Spec.CacheProfile = appv1alpha1.PortModeWebImageV1, "pro", appv1alpha1.CacheProfileAll
	}
	r := &AppReconciler{Registry: "registry.test", HTTPClient: imageConfigRegistry(t, `"8080/tcp":{},"19099/tcp":{}`)}
	app := projectionApp(cached)
	if _, _, err := r.resolveImageNetwork(t.Context(), app, "registry.test/cached:gen-1"); err == nil || !strings.Contains(err.Error(), "19099 is reserved") {
		t.Fatalf("resolveImageNetwork error = %v, want the reserved 19099 refused", err)
	}
	if app.Status.ImageNetwork != nil {
		t.Fatalf("network = %+v, want none resolved", app.Status.ImageNetwork)
	}
	r.HTTPClient = imageConfigRegistry(t, `"8080/tcp":{}`)
	app = projectionApp(cached, func(a *appv1alpha1.App) {
		a.Spec.PortMode, a.Spec.Port = appv1alpha1.PortModeWebImageConfiguredV1, edgeCachePort
	})
	if _, _, err := r.resolveImageNetwork(t.Context(), app, "registry.test/cached:gen-1"); err == nil {
		t.Fatal("an explicit port of 19099 must not become the primary the cache would share")
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
		sidecar := edgeCacheContainer("ghcr.io/bex-co/bex:test", app.Spec.CacheProfile, cached, drainSeconds)
		withEdgeCacheControl(&p.Spec, &sidecar)
		p.Spec.Containers = append(p.Spec.Containers, sidecar)
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

// edgeCachedApp is a paid web service whose edge cache runs.
func edgeCachedApp(mutate ...func(*appv1alpha1.App)) *appv1alpha1.App {
	return projectionApp(append([]func(*appv1alpha1.App){func(a *appv1alpha1.App) {
		a.Spec.Tier, a.Spec.Port, a.Spec.CacheProfile = "pro", 8080, appv1alpha1.CacheProfileStatic
	}}, mutate...)...)
}

// edgeCacheParams are app's projection parameters as applyServingDeployment
// sets them for the edge cache.
func edgeCacheParams(app *appv1alpha1.App) deploymentParams {
	p := execParams()
	p.edgeCacheImage = probeTestImage
	if app.Spec.ServesEdgeCache() {
		p.edgeCacheProfile = app.Spec.CacheProfile
	}
	return p
}

// podTemplateDiff names what moved between two pod templates in the terms an
// edge-cache change can move: template annotations, the sidecar's
// environment, its mounts and the pod's volumes, then anything else.
func podTemplateDiff(before, after corev1.PodTemplateSpec) string {
	var diffs []string
	for k := range mergedKeys(before.Annotations, after.Annotations) {
		if before.Annotations[k] != after.Annotations[k] {
			diffs = append(diffs, "template annotation "+k+": "+strconv.Quote(before.Annotations[k])+" -> "+strconv.Quote(after.Annotations[k]))
		}
	}
	b, a := edgeCacheSidecar(before.Spec), edgeCacheSidecar(after.Spec)
	switch {
	case (b == nil) != (a == nil):
		diffs = append(diffs, "edge-cache sidecar present: "+strconv.FormatBool(b != nil)+" -> "+strconv.FormatBool(a != nil))
	case b != nil:
		be, ae := envMap(b.Env), envMap(a.Env)
		for k := range mergedKeys(be, ae) {
			if be[k] != ae[k] {
				diffs = append(diffs, "sidecar env "+k+": "+strconv.Quote(be[k])+" -> "+strconv.Quote(ae[k]))
			}
		}
		if !reflect.DeepEqual(b.VolumeMounts, a.VolumeMounts) {
			diffs = append(diffs, "sidecar volume mounts moved")
		}
	}
	if !reflect.DeepEqual(before.Spec.Volumes, after.Spec.Volumes) {
		diffs = append(diffs, "pod volumes moved")
	}
	if len(diffs) == 0 && !equalPodTemplate(before, after) {
		diffs = append(diffs, "the pod template moved outside the edge cache")
	}
	return strings.Join(diffs, "; ")
}

func mergedKeys(a, b map[string]string) map[string]bool {
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	return keys
}

// Render's edge cache lives on its CDN, so neither a manual purge nor a switch
// between its two caching profiles touches an instance
// (render.com/docs/web-service-caching). Each must leave the projected pod
// template alone: a template change rolls every instance, and a disk-bearing
// service, which rolls by Recreate, goes down while it does (w3/m486).
func TestAPurgeOrAProfileSwitchKeepsTheEdgeCachedPodTemplate(t *testing.T) {
	disk := func(a *appv1alpha1.App) {
		a.Spec.Disk = &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10}
	}
	purged := func(stamp string) func(*appv1alpha1.App) {
		return func(a *appv1alpha1.App) {
			a.Annotations = map[string]string{appv1alpha1.CachePurgedAtAnnotation: stamp}
		}
	}
	profile := func(p string) func(*appv1alpha1.App) {
		return func(a *appv1alpha1.App) { a.Spec.CacheProfile = p }
	}
	for name, tc := range map[string]struct {
		before, after []func(*appv1alpha1.App)
	}{
		"a first purge":                         {nil, []func(*appv1alpha1.App){purged("2026-11-18T10:00:00Z")}},
		"a later purge":                         {[]func(*appv1alpha1.App){purged("2026-11-18T10:00:00Z")}, []func(*appv1alpha1.App){purged("2026-11-18T11:00:00Z")}},
		"static to all":                         {nil, []func(*appv1alpha1.App){profile(appv1alpha1.CacheProfileAll)}},
		"all to static":                         {[]func(*appv1alpha1.App){profile(appv1alpha1.CacheProfileAll)}, []func(*appv1alpha1.App){profile(appv1alpha1.CacheProfileStatic)}},
		"a purge on a disk-bearing service":     {[]func(*appv1alpha1.App){disk}, []func(*appv1alpha1.App){disk, purged("2026-11-18T10:00:00Z")}},
		"a switch on a disk-bearing service":    {[]func(*appv1alpha1.App){disk}, []func(*appv1alpha1.App){disk, profile(appv1alpha1.CacheProfileAll)}},
		"turning caching off":                   {nil, []func(*appv1alpha1.App){profile(appv1alpha1.CacheProfileNone)}},
		"turning caching off on a disk service": {[]func(*appv1alpha1.App){disk}, []func(*appv1alpha1.App){disk, profile("")}},
	} {
		t.Run(name, func(t *testing.T) {
			before := edgeCachedApp(tc.before...)
			stored := project(before, edgeCacheParams(before))
			if got := len(stored.Spec.Template.Spec.Containers); got != 2 {
				t.Fatalf("the stored template runs %d containers, want the app and the edge cache", got)
			}
			after := edgeCachedApp(tc.after...)
			if after.Spec.Disk != nil && stored.Spec.Strategy.Type != appsv1.RecreateDeploymentStrategyType {
				t.Fatalf("a disk-bearing service's strategy = %q, want Recreate", stored.Spec.Strategy.Type)
			}
			next := stored.DeepCopy()
			applyDeploymentSpec(next, after, edgeCacheParams(after))
			if diff := podTemplateDiff(stored.Spec.Template, next.Spec.Template); diff != "" {
				t.Fatalf("the pod template moved, rolling every instance: %s", diff)
			}
		})
	}
}

// A caching profile is a CDN setting on Render: saving one starts or stops
// caching without a deploy. So a switch is no new release (no deploy row, no
// release generation, no revision label to roll the pods on), in either
// direction and on either side of the API (w3/m486).
func TestACacheProfileSwitchIsNoNewRelease(t *testing.T) {
	profiles := []string{"", appv1alpha1.CacheProfileStatic, appv1alpha1.CacheProfileAll}
	for _, from := range profiles {
		for _, to := range profiles {
			before := appv1alpha1.AppSpec{Image: "docker.io/example/web:v1", Tier: "pro", Port: 8080, CacheProfile: from}
			after := before
			after.CacheProfile = to
			if desiredAppReleaseIdentity(before).release != desiredAppReleaseIdentity(after).release {
				t.Errorf("%q -> %q changed the release fingerprint", from, to)
			}
			if appv1alpha1.SpecRollsRelease(before, after) {
				t.Errorf("%q -> %q rolls a release on bex-api's side", from, to)
			}
		}
	}
}

// legacyEdgeCacheDeployment is what an operator before w3/m486 stored for app:
// the sidecar with no control file, and the App's purge stamp on the template.
func legacyEdgeCacheDeployment(app *appv1alpha1.App, p deploymentParams) *appsv1.Deployment {
	dep := project(app, p)
	tmpl := &dep.Spec.Template
	sidecar := edgeCacheSidecar(tmpl.Spec)
	sidecar.Env = slices.DeleteFunc(sidecar.Env, func(e corev1.EnvVar) bool { return e.Name == edgeCacheControlEnv })
	sidecar.VolumeMounts = nil
	if tmpl.Spec.Volumes = slices.DeleteFunc(tmpl.Spec.Volumes, func(v corev1.Volume) bool { return v.Name == edgeCacheControlVolumeName }); len(tmpl.Spec.Volumes) == 0 {
		tmpl.Spec.Volumes = nil
	}
	if stamp := app.Annotations[appv1alpha1.CachePurgedAtAnnotation]; stamp != "" {
		metav1.SetMetaDataAnnotation(&tmpl.ObjectMeta, appv1alpha1.CachePurgedAtAnnotation, stamp)
	}
	return dep
}

func controlled(t *testing.T, tmpl corev1.PodTemplateSpec) bool {
	t.Helper()
	sidecar := edgeCacheSidecar(tmpl.Spec)
	if sidecar == nil {
		t.Fatal("the template has no edge-cache sidecar")
	}
	_, env := containerEnv(sidecar, edgeCacheControlEnv)
	volume := slices.ContainsFunc(tmpl.Spec.Volumes, func(v corev1.Volume) bool { return v.Name == edgeCacheControlVolumeName })
	if env != volume || env != (len(sidecar.VolumeMounts) == 1) {
		t.Fatalf("a half-controlled sidecar: env %v, volume %v, mounts %v", env, volume, sidecar.VolumeMounts)
	}
	return env
}

// The operator release that ships the control file rolls no caching service:
// a template stored before it keeps its sidecar, profile and purge stamp. Its
// pods cannot read a purge or a switch at run time, so either still rolls
// them, onto the controlled shape; so does any other change, and turning
// caching off removes the sidecar it cannot hand a pass-through to.
func TestAnUpgradeKeepsAnEdgeCacheTemplateFromBeforeItsControlFile(t *testing.T) {
	stamped := func(a *appv1alpha1.App) {
		a.Annotations = map[string]string{appv1alpha1.CachePurgedAtAnnotation: "2026-11-01T00:00:00Z"}
	}
	for name, app := range map[string]*appv1alpha1.App{"never purged": edgeCachedApp(), "purged before": edgeCachedApp(stamped)} {
		t.Run(name, func(t *testing.T) {
			legacy := legacyEdgeCacheDeployment(app, edgeCacheParams(app))
			upgraded := legacy.DeepCopy()
			applyDeploymentSpec(upgraded, app, edgeCacheParams(app))
			if diff := podTemplateDiff(legacy.Spec.Template, upgraded.Spec.Template); diff != "" {
				t.Fatalf("the upgrade rolled the service: %s", diff)
			}

			for change, mutate := range map[string]func(*appv1alpha1.App){
				"a purge": func(a *appv1alpha1.App) {
					metav1.SetMetaDataAnnotation(&a.ObjectMeta, appv1alpha1.CachePurgedAtAnnotation, "2026-11-18T00:00:00Z")
				},
				"a switch":    func(a *appv1alpha1.App) { a.Spec.CacheProfile = appv1alpha1.CacheProfileAll },
				"a restart":   func(a *appv1alpha1.App) { a.Spec.RestartedAt = "2026-11-18T00:00:00Z" },
				"caching off": func(a *appv1alpha1.App) { a.Spec.CacheProfile = "" },
			} {
				next := app.DeepCopy()
				mutate(next)
				rolled := legacy.DeepCopy()
				applyDeploymentSpec(rolled, next, edgeCacheParams(next))
				if equalPodTemplate(rolled.Spec.Template, legacy.Spec.Template) {
					t.Fatalf("%s left the legacy template, so its pods never see it", change)
				}
				if !next.Spec.ServesEdgeCache() {
					if edgeCacheSidecar(rolled.Spec.Template.Spec) != nil {
						t.Fatalf("%s: a legacy sidecar was kept", change)
					}
					continue
				}
				if _, stamp := rolled.Spec.Template.Annotations[appv1alpha1.CachePurgedAtAnnotation]; stamp {
					t.Fatalf("%s: the rolled template still carries the purge stamp", change)
				}
				if !controlled(t, rolled.Spec.Template) {
					t.Fatalf("%s rolled onto another legacy sidecar", change)
				}
				if got, _ := containerEnv(edgeCacheSidecar(rolled.Spec.Template.Spec), edgeCacheProfileEnv); got != next.Spec.CacheProfile {
					t.Fatalf("%s: the rolled sidecar starts on %q, want %q", change, got, next.Spec.CacheProfile)
				}
			}
		})
	}
}

// A sidecar with its control file keeps the profile it started with alongside
// every other value a stored template keeps (applyDeploymentSpec): a switch
// together with an operator upgrade that moved the health-probe image rolls
// nothing, and any real change adopts both the current image and the current
// profile.
func TestAKeptCacheProfileComposesWithTheOtherKeptValues(t *testing.T) {
	app := edgeCachedApp(func(a *appv1alpha1.App) { a.Spec.HealthCheckPath = "/healthz" })
	stored := project(app, edgeCacheParams(app))
	switched := app.DeepCopy()
	switched.Spec.CacheProfile = appv1alpha1.CacheProfileAll
	upgraded := edgeCacheParams(switched)
	upgraded.healthProbeImage = "ghcr.io/bex-co/bex@sha256:2222"
	kept := stored.DeepCopy()
	applyDeploymentSpec(kept, switched, upgraded)
	if diff := podTemplateDiff(stored.Spec.Template, kept.Spec.Template); diff != "" {
		t.Fatalf("a switch over a new helper image rolled the pods: %s", diff)
	}
	restarted := switched.DeepCopy()
	restarted.Spec.RestartedAt = "2026-11-18T00:00:00Z"
	rolled := stored.DeepCopy()
	applyDeploymentSpec(rolled, restarted, upgraded)
	if got, _ := containerEnv(edgeCacheSidecar(rolled.Spec.Template.Spec), edgeCacheProfileEnv); got != appv1alpha1.CacheProfileAll {
		t.Fatalf("the next roll starts the sidecar on %q, want the saved %q", got, appv1alpha1.CacheProfileAll)
	}
	if got := storedHealthProbeImage(rolled.Spec.Template.Spec); got != upgraded.healthProbeImage {
		t.Fatalf("the next roll installs the probe from %q, want %q", got, upgraded.healthProbeImage)
	}
}

// The live purge stamp and profile reach every running pod whose sidecar reads
// its control file, and nothing else: not a legacy sidecar (its template rolls
// instead), a leaving pod, a pod without the cache, or another App's.
func TestAnnotateEdgeCachePodsHandsTheProxiesThePurgeAndProfile(t *testing.T) {
	named := []corev1.ContainerPort{{Name: servingPortName, ContainerPort: 8080}}
	legacy := func(p *corev1.Pod) {
		c := edgeCacheSidecar(p.Spec)
		c.Env = slices.DeleteFunc(c.Env, func(e corev1.EnvVar) bool { return e.Name == edgeCacheControlEnv })
	}
	annotated := func(annotations map[string]string) func(*corev1.Pod) {
		return func(p *corev1.Pod) { p.Annotations = annotations }
	}
	for name, tc := range map[string]struct {
		app         *appv1alpha1.App
		wantProfile string
	}{
		"a purge and a switch": {edgeCachedApp(func(a *appv1alpha1.App) {
			a.Namespace, a.Spec.CacheProfile = "tea-a", appv1alpha1.CacheProfileAll
			a.Annotations = map[string]string{appv1alpha1.CachePurgedAtAnnotation: "2026-11-18T10:00:00Z"}
		}), appv1alpha1.CacheProfileAll},
		"caching turned off": {edgeCachedApp(func(a *appv1alpha1.App) {
			a.Namespace, a.Spec.CacheProfile = "tea-a", ""
			a.Annotations = map[string]string{appv1alpha1.CachePurgedAtAnnotation: "2026-11-18T10:00:00Z"}
		}), appv1alpha1.CacheProfileNone},
	} {
		t.Run(name, func(t *testing.T) {
			static := tc.app.DeepCopy()
			static.Spec.CacheProfile = appv1alpha1.CacheProfileStatic
			current := map[string]string{edgecache.ProfileAnnotation: tc.wantProfile, appv1alpha1.CachePurgedAtAnnotation: "2026-11-18T10:00:00Z"}
			other := edgeCachedApp(func(a *appv1alpha1.App) { a.Name, a.Namespace = "other", "tea-a" })
			pods := []client.Object{
				edgeCacheTestPod(static, "fresh", "rev-1", 8080, named),
				edgeCacheTestPod(static, "current", "rev-1", 8080, named, annotated(current)),
				edgeCacheTestPod(static, "legacy", "rev-1", 8080, named, legacy),
				edgeCacheTestPod(static, "leaving", "rev-1", 8080, named, terminatingPod),
				edgeCacheTestPod(static, "plain", "rev-1", 0, named),
				edgeCacheTestPod(other, "other", "rev-1", 8080, named),
			}
			cl := fake.NewClientBuilder().WithScheme(wakeScheme()).WithObjects(pods...).Build()
			before := map[string]string{}
			for _, p := range pods {
				before[p.GetName()] = p.GetResourceVersion()
			}
			if err := (&AppReconciler{Client: cl}).annotateEdgeCachePods(t.Context(), tc.app); err != nil {
				t.Fatal(err)
			}
			for _, p := range pods {
				got := &corev1.Pod{}
				if err := cl.Get(t.Context(), client.ObjectKeyFromObject(p), got); err != nil {
					t.Fatal(err)
				}
				patched := got.ResourceVersion != p.GetResourceVersion()
				if want := p.GetName() == "fresh"; patched != want {
					t.Errorf("pod %s patched = %v, want %v (annotations %v)", p.GetName(), patched, want, got.Annotations)
				}
				if p.GetName() == "fresh" && !maps.Equal(got.Annotations, current) {
					t.Errorf("fresh pod annotations = %v, want %v", got.Annotations, current)
				}
			}
		})
	}
}

// A pod whose proxy already caches by the App's profile, from its template,
// needs no patch when the App was never purged or was purged before the pod
// was created, which started it empty: a new pod costs no write and no second
// purge.
func TestAnnotateEdgeCachePodsWritesNothingForAPodAlreadyCurrent(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC))
	for name, purged := range map[string]string{"never purged": "", "purged before the pod": "2026-11-18T11:59:59.5Z"} {
		t.Run(name, func(t *testing.T) {
			app := edgeCachedApp(func(a *appv1alpha1.App) {
				a.Namespace = "tea-a"
				if purged != "" {
					a.Annotations = map[string]string{appv1alpha1.CachePurgedAtAnnotation: purged}
				}
			})
			pod := edgeCacheTestPod(app, "fresh", "rev-1", 8080, nil, func(p *corev1.Pod) { p.CreationTimestamp = created })
			cl := fake.NewClientBuilder().WithScheme(wakeScheme()).WithObjects(pod).Build()
			if err := (&AppReconciler{Client: cl}).annotateEdgeCachePods(t.Context(), app); err != nil {
				t.Fatal(err)
			}
			got := &corev1.Pod{}
			if err := cl.Get(t.Context(), client.ObjectKeyFromObject(pod), got); err != nil {
				t.Fatal(err)
			}
			if got.ResourceVersion != pod.GetResourceVersion() || len(got.Annotations) != 0 {
				t.Fatalf("a current pod was patched: %v", got.Annotations)
			}
		})
	}
}

// A purge stamps only an annotation, which bumps no generation, so the App
// watch must pass it, or the purge would wait out the steady-state requeue.
func TestAPurgeStampWakesTheReconciler(t *testing.T) {
	old := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Generation: 4}}
	purged := old.DeepCopy()
	metav1.SetMetaDataAnnotation(&purged.ObjectMeta, appv1alpha1.CachePurgedAtAnnotation, "2026-11-18T10:00:00Z")
	if !(generationOrDeletionPredicate{}).Update(event.UpdateEvent{ObjectOld: old, ObjectNew: purged}) {
		t.Fatal("a purge stamp did not wake the reconciler")
	}
	touched := old.DeepCopy()
	metav1.SetMetaDataAnnotation(&touched.ObjectMeta, "app.bex.co/updated-at", "2026-11-18T10:00:00Z")
	if (generationOrDeletionPredicate{}).Update(event.UpdateEvent{ObjectOld: old, ObjectNew: touched}) {
		t.Fatal("an unrelated annotation woke the reconciler")
	}
}

// The edge-cache proxy reads the stamp under its own name; the two must not
// drift.
func TestTheProxyReadsTheAppsPurgeAnnotation(t *testing.T) {
	if edgecache.PurgedAtAnnotation != appv1alpha1.CachePurgedAtAnnotation {
		t.Fatalf("the proxy reads %q, the API stamps %q", edgecache.PurgedAtAnnotation, appv1alpha1.CachePurgedAtAnnotation)
	}
}

// A release fingerprinted with the edge-cache profile in it, as operators
// before w3/m486 stored every caching service's, is the same release today:
// it is adopted, whether or not a profile was saved since, and through its
// recorded spec while another setting waits for a deploy, so the upgrade opens
// no release. A real difference is still one.
func TestAReleaseFingerprintedWithItsCacheProfileIsAdopted(t *testing.T) {
	spec := appv1alpha1.AppSpec{Image: "docker.io/example/web:v1", Tier: "pro", Port: 8080, CacheProfile: appv1alpha1.CacheProfileStatic}
	legacy := releaseIdentityWith(spec, releaseIdentityVersion, appv1alpha1.CacheProfileStatic).release
	if legacy == desiredAppReleaseIdentity(spec).release {
		t.Fatal("the test needs a fingerprint the profile moved")
	}
	app := func(mutate func(*appv1alpha1.AppSpec)) *appv1alpha1.App {
		a := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-a"}, Spec: spec}
		mutate(&a.Spec)
		a.Status.ReleaseGeneration, a.Status.ReleaseFingerprint = 3, legacy
		return a
	}
	recorded, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	record := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.ReleaseRecordName("web", 3), Namespace: "tea-a"},
		Data:       map[string][]byte{releaseRecordAdoptedSpecKey: recorded},
	}
	for name, tc := range map[string]struct {
		mutate  func(*appv1alpha1.AppSpec)
		record  bool
		adopted bool
	}{
		"the release as saved":                 {func(*appv1alpha1.AppSpec) {}, false, true},
		"a profile saved since":                {func(s *appv1alpha1.AppSpec) { s.CacheProfile = appv1alpha1.CacheProfileAll }, false, true},
		"caching turned off since":             {func(s *appv1alpha1.AppSpec) { s.CacheProfile = "" }, false, true},
		"a held save, with the release record": {func(s *appv1alpha1.AppSpec) { s.HealthCheckPath = "/healthz" }, true, true},
		"a held save, with no record":          {func(s *appv1alpha1.AppSpec) { s.HealthCheckPath = "/healthz" }, false, false},
	} {
		t.Run(name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(wakeScheme())
			if tc.record {
				builder = builder.WithObjects(record.DeepCopy())
			}
			a := app(tc.mutate)
			if err := (&AppReconciler{Client: builder.Build()}).adoptCacheProfileReleaseIdentity(t.Context(), a); err != nil {
				t.Fatal(err)
			}
			want := legacy
			if tc.adopted {
				want = desiredAppReleaseIdentity(spec).release
			}
			if a.Status.ReleaseFingerprint != want {
				t.Fatalf("fingerprint = %s, want %s", a.Status.ReleaseFingerprint, want)
			}
		})
	}

	// A held save keeps the fingerprint apart from the saved spec's until it
	// deploys, so the record would be read on every pass: it is read once.
	reads := 0
	counted := fake.NewClientBuilder().WithScheme(wakeScheme()).WithObjects(record.DeepCopy()).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			reads++
			return c.Get(ctx, key, obj, opts...)
		},
	}).Build()
	held := app(func(s *appv1alpha1.AppSpec) { s.HealthCheckPath = "/healthz" })
	r := &AppReconciler{Client: counted}
	for range 3 {
		if err := r.adoptCacheProfileReleaseIdentity(t.Context(), held); err != nil {
			t.Fatal(err)
		}
	}
	if reads != 1 || held.Status.ReleaseFingerprint != desiredAppReleaseIdentity(spec).release {
		t.Fatalf("record reads = %d, fingerprint adopted = %v; want one read and the adoption", reads, held.Status.ReleaseFingerprint == desiredAppReleaseIdentity(spec).release)
	}

	other := app(func(s *appv1alpha1.AppSpec) { s.Image = "docker.io/example/web:v2" })
	if err := (&AppReconciler{Client: fake.NewClientBuilder().WithScheme(wakeScheme()).Build()}).adoptCacheProfileReleaseIdentity(t.Context(), other); err != nil {
		t.Fatal(err)
	}
	if other.Status.ReleaseFingerprint != legacy {
		t.Fatal("a new image was adopted into the running release")
	}
	if !recordedSettingsMatch(appv1alpha1.ReleaseRecordSpec{SettingsFingerprint: releaseIdentityWith(spec, releaseIdentityVersion, appv1alpha1.CacheProfileStatic).release}, app(func(s *appv1alpha1.AppSpec) { s.CacheProfile = appv1alpha1.CacheProfileAll }).Spec) {
		t.Fatal("a release record from before w3/m486 reads as unsaved settings after a profile save")
	}
}
