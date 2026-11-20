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
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/edgecache"
	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("Edge caching (w5/m151, a per-service tier since w3/m489)", func() {
	ctx := context.Background()
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			BackupHelperImage: "ghcr.io/bex-co/bex:test", BaseDomain: "edge.test"}
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
			Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, Tier: tier, CacheProfile: profile, Host: name + ".edge.test"},
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
		return envtestDeployment(nn).Spec.Template.Spec.Containers
	}
	tierOf := func(nn types.NamespacedName) types.NamespacedName {
		return types.NamespacedName{Name: edgeCacheTierName(nn.Name), Namespace: nn.Namespace}
	}
	backendOf := func(nn types.NamespacedName) *networkingv1.IngressServiceBackend {
		GinkgoHelper()
		ing := &networkingv1.Ingress{}
		Expect(k8sClient.Get(ctx, nn, ing)).To(Succeed())
		return ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service
	}
	route := func(nn types.NamespacedName) string { return backendOf(nn).Name }

	It("caches a paid web service in its own tier, routed through it only while a tier pod is Ready", func() {
		nn := create("edgecache-pro", "pro", appv1alpha1.CacheProfileStatic)
		Expect(containers(nn)).To(HaveLen(1), "the service's own pods run no cache")
		tier := envtestDeployment(tierOf(nn))
		Expect(*tier.Spec.Replicas).To(Equal(int32(edgeCacheTierReplicas)))
		Expect(envMap(tier.Spec.Template.Spec.Containers[0].Env)).To(HaveKeyWithValue("BEX_EDGE_CACHE_UPSTREAM", "http://edgecache-pro:10000"))
		Expect(tier.Spec.Template.Labels).NotTo(HaveKey(labelApp), "a tier pod is no instance of the service")
		projected := &appsv1.Deployment{}
		projectEdgeCacheTier(projected, envtestApp(nn), r.BackupHelperImage)
		Expect(apiequality.Semantic.DeepEqual(projected.Spec.Template, tier.Spec.Template)).To(BeTrue(),
			"the projection must match what the API server stores, or every pass rewrites the tier:\n%+v\nstored\n%+v", projected.Spec.Template, tier.Spec.Template)
		svc := &corev1.Service{}
		Expect(k8sClient.Get(ctx, tierOf(nn), svc)).To(Succeed())
		Expect(svc.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone))
		Expect(k8sClient.Get(ctx, nn, svc)).To(Succeed())
		Expect(svc.Spec.Ports[0].TargetPort.StrVal).To(Equal(servingPortName), "the service's own Service targets the app")

		By("the route stays on the service until a tier pod is Ready")
		serveEnvtestRelease(nn, func() { reconcileN(nn) })
		Expect(route(nn)).To(Equal(nn.Name))
		markEnvtestDeploymentReady(tierOf(nn))
		reconcileN(nn)
		Expect(route(nn)).To(Equal(edgeCacheTierName(nn.Name)))
		Expect(backendOf(nn).Port.Number).To(Equal(int32(edgeCacheTierPort)))

		By("a tier pod takes the profile and the active release through its annotations")
		pod := simulateTemplatePod(tierOf(nn), "tier", true)
		reconcileN(nn)
		active := envtestApp(nn).Status.ActiveRevision
		Expect(envtestPod(pod).Annotations).To(And(
			HaveKeyWithValue(edgecache.ProfileAnnotation, appv1alpha1.CacheProfileStatic),
			HaveKeyWithValue(edgecache.RevisionAnnotation, active)))

		template, tierTemplate := envtestDeployment(nn).Spec.Template, envtestDeployment(tierOf(nn)).Spec.Template
		generation, tierGeneration := envtestApp(nn).Status.ReleaseGeneration, envtestDeployment(tierOf(nn)).Generation
		unchanged := func(what string) {
			GinkgoHelper()
			Expect(envtestDeployment(nn).Spec.Template).To(Equal(template), "%s must not roll the service", what)
			Expect(envtestDeployment(tierOf(nn)).Spec.Template).To(Equal(tierTemplate), "%s must not roll the tier", what)
			Expect(envtestDeployment(tierOf(nn)).Generation).To(Equal(tierGeneration), "%s must not rewrite the tier", what)
			Expect(envtestApp(nn).Status.ReleaseGeneration).To(Equal(generation), "%s must open no release", what)
			Expect(envtestPod(pod).Annotations).To(HaveKeyWithValue(edgecache.RevisionAnnotation, active), "%s must not purge as a deploy", what)
		}

		By("a manual purge reaches the running proxy, and rolls nothing")
		app := envtestApp(nn)
		stamp := time.Now().Add(time.Second).UTC().Format(time.RFC3339Nano) // after the pod was created
		metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.CachePurgedAtAnnotation, stamp)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(nn)
		unchanged("a purge")
		Expect(envtestPod(pod).Annotations).To(HaveKeyWithValue(edgecache.PurgedAtAnnotation, stamp))

		By("a profile switch reaches the proxy the same way")
		app = envtestApp(nn)
		app.Spec.CacheProfile = appv1alpha1.CacheProfileAll
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(nn)
		unchanged("a switch")
		Expect(envtestPod(pod).Annotations).To(HaveKeyWithValue(edgecache.ProfileAnnotation, appv1alpha1.CacheProfileAll))

		By("a scale change keeps the cache")
		app = envtestApp(nn)
		app.Spec.Replicas = 3
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(nn)
		Expect(envtestDeployment(tierOf(nn)).Spec.Template).To(Equal(tierTemplate), "a scale-out must not roll the tier")
		Expect(envtestPod(pod).Annotations).To(HaveKeyWithValue(edgecache.RevisionAnnotation, active), "a scale-out must not purge")

		By("a successful deploy empties the cache once its rollout has no old instance left")
		app = envtestApp(nn)
		app.Spec.Env = append(app.Spec.Env, appv1alpha1.EnvVar{Name: "RELEASE", Value: "2"})
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(nn)
		Expect(envtestPod(pod).Annotations).To(HaveKeyWithValue(edgecache.RevisionAnnotation, active), "a deploy still rolling must not purge yet")
		serveEnvtestRelease(nn, func() { reconcileN(nn) })
		reconcileN(nn)
		next := envtestApp(nn).Status.ActiveRevision
		Expect(next).NotTo(Equal(active))
		Expect(envtestPod(pod).Annotations).To(HaveKeyWithValue(edgecache.RevisionAnnotation, next))
		Expect(route(nn)).To(Equal(edgeCacheTierName(nn.Name)), "the deploy keeps the route on the tier")

		By("losing every tier pod returns the route to the service")
		dep := envtestDeployment(tierOf(nn))
		dep.Status.ReadyReplicas, dep.Status.AvailableReplicas = 0, 0
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())
		reconcileN(nn)
		Expect(route(nn)).To(Equal(nn.Name))
		markEnvtestDeploymentReady(tierOf(nn))
		reconcileN(nn)
		Expect(route(nn)).To(Equal(edgeCacheTierName(nn.Name)))

		By("turning caching off moves the route back first, then removes the tier")
		app = envtestApp(nn)
		app.Spec.CacheProfile = appv1alpha1.CacheProfileNone
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileN(nn)
		Expect(route(nn)).To(Equal(nn.Name))
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, tierOf(nn), &appsv1.Deployment{}))).To(BeTrue(), "the tier must go once nothing routes to it")
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, tierOf(nn), &corev1.Service{}))).To(BeTrue())
		Expect(containers(nn)).To(HaveLen(1))
	})

	It("never caches a free web service, as on Render", func() {
		nn := create("edgecache-free", "free", appv1alpha1.CacheProfileAll)
		Expect(containers(nn)).To(HaveLen(1))
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, tierOf(nn), &appsv1.Deployment{}))).To(BeTrue())
	})

	It("caches a paid web service on its image's port through port 10000, whatever its primary", func() {
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

		By("while HTTP discovery chooses among the declared ports, port 10000 targets the provisional primary")
		reconcileToFixedPoint(run)
		Expect(webImageHTTPSelectionPending(envtestApp(nn))).To(BeTrue(), "the test must exercise a pending discovery")
		Expect(containers(nn)).To(HaveLen(1))
		Expect(servicePortTargets(nn.Name)).To(Equal(map[int32]intstr.IntOrString{80: intstr.FromInt32(80), 8080: intstr.FromInt32(8080), 10000: intstr.FromInt32(80)}))
		tierTemplate := envtestDeployment(tierOf(nn)).Spec.Template
		Expect(envMap(tierTemplate.Spec.Containers[0].Env)).To(HaveKeyWithValue("BEX_EDGE_CACHE_UPSTREAM", "http://edgecache-image:10000"))

		By("a choice that moves the primary moves port 10000 with it, and leaves the tier alone")
		app := envtestApp(nn)
		app.Status.ImageNetwork.PrimaryPort, app.Status.ImageNetwork.HTTPSelection = 8080, appv1alpha1.HTTPSelectionDetected
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		Expect(servicePortTargets(nn.Name)).To(Equal(map[int32]intstr.IntOrString{80: intstr.FromInt32(80), 8080: intstr.FromInt32(8080), 10000: intstr.FromInt32(8080)}))
		Expect(envtestDeployment(tierOf(nn)).Spec.Template).To(Equal(tierTemplate), "a port choice must not roll the tier")
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
// may bind any other. The proxy's port was the sidecar's inside the app's own
// pods, which a template stored before w3/m489 may still run, so it is one of
// those three: a fourth, as 18014 was, is a port Render lets a service use and
// bex does not (w21/076, ADR018).
func TestTheEdgeCacheSharesARenderReservedPortAndTakesNoFourth(t *testing.T) {
	if got := appv1alpha1.ReservedImagePorts(); !slices.Equal(got, []int32{18012, 18013, 19099}) {
		t.Fatalf("reserved ports = %v, want exactly Render's 18012, 18013 and 19099", got)
	}
	if !appv1alpha1.IsReservedImagePort(edgeCachePort) {
		t.Fatalf("the proxy's port %d is not reserved, so a service's PORT could collide with a kept sidecar", edgeCachePort)
	}
}

// 19099 is a port Render reserves too (IsReservedImagePort): a caching
// image-port web service whose image exposes it resolves no network, and an
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
		t.Fatal("an explicit port of 19099 must not become the primary")
	}
}

// edgeCachedApp is a paid web service whose edge cache runs.
func edgeCachedApp(mutate ...func(*appv1alpha1.App)) *appv1alpha1.App {
	return projectionApp(append([]func(*appv1alpha1.App){func(a *appv1alpha1.App) {
		a.UID = "edge-uid"
		a.Spec.Tier, a.Spec.Port, a.Spec.CacheProfile = "pro", 8080, appv1alpha1.CacheProfileStatic
	}}, mutate...)...)
}

// storedSidecarDeployment is what an operator before w3/m489 stored for app:
// the edge-cache sidecar after the app container, reading its control file
// (w3/m486), or, when controlled is false, the shape from before it, which
// carries the App's purge stamp on the template instead.
func storedSidecarDeployment(app *appv1alpha1.App, p deploymentParams, controlled bool) *appsv1.Deployment {
	dep := project(app, p)
	withStoredEdgeCacheSidecar(&dep.Spec.Template, app, p.port, controlled)
	return dep
}

// storedEdgeCacheSidecar is the sidecar container operators from w5/m151 to
// w3/m486 projected in front of port, before its control file.
func storedEdgeCacheSidecar(profile string, port int) corev1.Container {
	resources := corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("160Mi")}
	sec := execution.RestrictedContainerSecurityContext()
	sec.RunAsNonRoot, sec.RunAsUser, sec.ReadOnlyRootFilesystem = new(true), new(int64(65532)), new(true)
	return corev1.Container{
		Name: edgeCacheContainerName, Image: "ghcr.io/bex-co/bex@sha256:0000", ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{"/edge-cache"},
		Env: []corev1.EnvVar{
			{Name: "BEX_EDGE_CACHE_LISTEN", Value: ":19099"},
			{Name: "BEX_EDGE_CACHE_UPSTREAM", Value: "http://127.0.0.1:" + strconv.Itoa(port)},
			{Name: "BEX_EDGE_CACHE_PROFILE", Value: profile},
			{Name: "BEX_EDGE_CACHE_MAX_BYTES", Value: strconv.Itoa(48 << 20)},
			{Name: "BEX_EDGE_CACHE_DRAIN_SECONDS", Value: strconv.FormatInt(drainSeconds, 10)},
			{Name: "GOMEMLIMIT", Value: "120MiB"},
		},
		Ports:     []corev1.ContainerPort{{Name: edgeCachePortName, ContainerPort: edgeCachePort, Protocol: corev1.ProtocolTCP}},
		Resources: corev1.ResourceRequirements{Requests: resources, Limits: resources},
		ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(edgeCachePort)},
		}},
		SecurityContext: sec,
	}
}

// withStoredEdgeCacheSidecar appends to tmpl, a current projection for app,
// the sidecar an operator before w3/m489 placed after the app container:
// reading its control file (w3/m486) when controlled, or the shape before it,
// which carries the App's purge stamp on the template instead.
func withStoredEdgeCacheSidecar(tmpl *corev1.PodTemplateSpec, app *appv1alpha1.App, port int, controlled bool) {
	sidecar := storedEdgeCacheSidecar(app.Spec.CacheProfile, port)
	if controlled {
		sidecar.Env = append(sidecar.Env, corev1.EnvVar{Name: edgeCacheControlEnv, Value: edgeCacheControlDir + "/" + edgeCacheControlFile})
		sidecar.VolumeMounts = []corev1.VolumeMount{{Name: edgeCacheControlVolumeName, MountPath: edgeCacheControlDir, ReadOnly: true}}
		tmpl.Spec.Volumes = append(tmpl.Spec.Volumes, edgeCacheControlVolume())
	} else if stamp := app.Annotations[appv1alpha1.CachePurgedAtAnnotation]; stamp != "" {
		metav1.SetMetaDataAnnotation(&tmpl.ObjectMeta, appv1alpha1.CachePurgedAtAnnotation, stamp)
	}
	tmpl.Spec.Containers = append(tmpl.Spec.Containers, sidecar)
	applyPodSpecServerDefaults(&tmpl.Spec)
}

// The operator release that moves caching to the tier rolls no caching
// service (DoD: the upgrade rolls no tenant template): a template stored with
// the sidecar, in either shape, keeps it, its volume and its purge stamp, as
// stored, through the upgrade, a new helper image, a purge, a switch and
// caching turned off, all of which the tier handles now. The next change for
// another reason (a deploy, a restart) drops all three.
func TestAnUpgradeKeepsAStoredEdgeCacheSidecarUntilTheTemplateChanges(t *testing.T) {
	stamped := func(a *appv1alpha1.App) {
		a.Annotations = map[string]string{appv1alpha1.CachePurgedAtAnnotation: "2026-11-01T00:00:00Z"}
	}
	for name, tc := range map[string]struct {
		app        *appv1alpha1.App
		controlled bool
	}{
		"a controlled sidecar (w3/m486)":    {edgeCachedApp(), true},
		"a sidecar from before its control": {edgeCachedApp(), false},
		"a stamped sidecar from before it":  {edgeCachedApp(stamped), false},
	} {
		t.Run(name, func(t *testing.T) {
			stored := storedSidecarDeployment(tc.app, execParams(), tc.controlled)
			for change, mutate := range map[string]func(*appv1alpha1.App, *deploymentParams){
				"the upgrade":    func(*appv1alpha1.App, *deploymentParams) {},
				"a helper image": func(_ *appv1alpha1.App, p *deploymentParams) { p.healthProbeImage = "ghcr.io/bex-co/bex@sha256:2222" },
				"a purge": func(a *appv1alpha1.App, _ *deploymentParams) {
					stamped(a)
					a.Annotations[appv1alpha1.CachePurgedAtAnnotation] = "2026-11-18T00:00:00Z"
				},
				"a switch":    func(a *appv1alpha1.App, _ *deploymentParams) { a.Spec.CacheProfile = appv1alpha1.CacheProfileAll },
				"caching off": func(a *appv1alpha1.App, _ *deploymentParams) { a.Spec.CacheProfile = "" },
				"a scale-out": func(_ *appv1alpha1.App, p *deploymentParams) { p.replicas = 4 },
				"all of the above": func(a *appv1alpha1.App, p *deploymentParams) {
					a.Spec.CacheProfile, p.healthProbeImage, p.replicas = appv1alpha1.CacheProfileAll, "ghcr.io/bex-co/bex@sha256:2222", 4
				},
			} {
				app, p := tc.app.DeepCopy(), execParams()
				mutate(app, &p)
				next := stored.DeepCopy()
				applyDeploymentSpec(next, app, p)
				if diff := podTemplateDiff(stored.Spec.Template, next.Spec.Template); diff != "" {
					t.Fatalf("%s rolled the service: %s", change, diff)
				}
			}
			for change, mutate := range map[string]func(*appv1alpha1.App){
				"a restart": func(a *appv1alpha1.App) { a.Spec.RestartedAt = "2026-11-18T00:00:00Z" },
				"a deploy":  func(a *appv1alpha1.App) { a.Spec.Env = []appv1alpha1.EnvVar{{Name: "RELEASE", Value: "2"}} },
			} {
				app := tc.app.DeepCopy()
				mutate(app)
				next := stored.DeepCopy()
				applyDeploymentSpec(next, app, execParams())
				if equalPodTemplate(next.Spec.Template, stored.Spec.Template) {
					t.Fatalf("%s kept the stored template", change)
				}
				tmpl := next.Spec.Template
				if edgeCacheSidecar(tmpl.Spec) != nil || len(tmpl.Spec.Containers) != 1 {
					t.Fatalf("%s kept the sidecar: %v", change, tmpl.Spec.Containers)
				}
				if slices.ContainsFunc(tmpl.Spec.Volumes, func(v corev1.Volume) bool { return v.Name == edgeCacheControlVolumeName }) {
					t.Fatalf("%s kept the sidecar's control volume", change)
				}
				if _, stamp := tmpl.Annotations[appv1alpha1.CachePurgedAtAnnotation]; stamp {
					t.Fatalf("%s kept the purge stamp on the template", change)
				}
				if want := project(app, execParams()).Spec.Template; !equalPodTemplate(tmpl, want) {
					t.Fatalf("%s did not roll onto the current shape: %s", change, podTemplateDiff(want, tmpl))
				}
			}
		})
	}
}

// podTemplateDiff names what moved between two pod templates in the terms an
// edge-cache change or an operator upgrade can move: template annotations,
// container images, the sidecar, the pod's volumes, then anything else.
func podTemplateDiff(before, after corev1.PodTemplateSpec) string {
	var diffs []string
	for k := range mergedKeys(before.Annotations, after.Annotations) {
		if before.Annotations[k] != after.Annotations[k] {
			diffs = append(diffs, "template annotation "+k+": "+strconv.Quote(before.Annotations[k])+" -> "+strconv.Quote(after.Annotations[k]))
		}
	}
	for _, was := range slices.Concat(before.Spec.InitContainers, before.Spec.Containers) {
		for _, now := range slices.Concat(after.Spec.InitContainers, after.Spec.Containers) {
			if now.Name == was.Name && now.Image != was.Image {
				diffs = append(diffs, "container "+now.Name+" image: "+strconv.Quote(was.Image)+" -> "+strconv.Quote(now.Image))
			}
		}
	}
	b, a := edgeCacheSidecar(before.Spec), edgeCacheSidecar(after.Spec)
	switch {
	case (b == nil) != (a == nil):
		diffs = append(diffs, "edge-cache sidecar present: "+strconv.FormatBool(b != nil)+" -> "+strconv.FormatBool(a != nil))
	case b != nil && !equalPodTemplate(corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{*b}}},
		corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{*a}}}):
		diffs = append(diffs, "the sidecar moved")
	}
	if !equalPodTemplate(corev1.PodTemplateSpec{Spec: corev1.PodSpec{Volumes: before.Spec.Volumes}},
		corev1.PodTemplateSpec{Spec: corev1.PodSpec{Volumes: after.Spec.Volumes}}) {
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

// A new caching service's pods carry no cache, and turning caching on for a
// running one changes nothing in its template: Render's cache is on its CDN,
// so enabling it rolls no instance (DoD).
func TestTurningCachingOnRollsNoInstance(t *testing.T) {
	plain := edgeCachedApp(func(a *appv1alpha1.App) { a.Spec.CacheProfile = "" })
	stored := project(plain, execParams())
	for _, profile := range []string{appv1alpha1.CacheProfileStatic, appv1alpha1.CacheProfileAll} {
		cached := plain.DeepCopy()
		cached.Spec.CacheProfile = profile
		next := stored.DeepCopy()
		applyDeploymentSpec(next, cached, execParams())
		if diff := podTemplateDiff(stored.Spec.Template, next.Spec.Template); diff != "" {
			t.Fatalf("turning on %s rolled the service: %s", profile, diff)
		}
		if got := len(project(cached, execParams()).Spec.Template.Spec.Containers); got != 1 {
			t.Fatalf("a new %s service runs %d containers, want the app alone", profile, got)
		}
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

// The service's own Services target the app on every port path, with caching
// on and while a pod from before the tier still runs its sidecar: the public
// route reaches the cache through the tier, which forwards to port 10000, and
// the private network is never cached, as Render's CDN fronts only the
// public URL.
func TestACachingServicesOwnServicesTargetTheApp(t *testing.T) {
	sidecarPod := func(app *appv1alpha1.App, name, revision string, ports []corev1.ContainerPort) client.Object {
		podLabels := appPodLabels(app, false)
		podLabels[labelRevision] = revision
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace, Labels: podLabels},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: appContainerName, Ports: ports},
				{Name: edgeCacheContainerName, Ports: []corev1.ContainerPort{{Name: edgeCachePortName, ContainerPort: edgeCachePort}}}}},
		}
	}
	applied := func(t *testing.T, app *appv1alpha1.App, port int, pods ...client.Object) map[int32]intstr.IntOrString {
		t.Helper()
		cl := fake.NewClientBuilder().WithScheme(wakeScheme()).WithObjects(pods...).Build()
		r := &AppReconciler{Client: cl, Scheme: wakeScheme(), Mode: ModeKubernetes, BackupHelperImage: "ghcr.io/bex-co/bex:test"}
		if err := r.applyClusterIPService(t.Context(), app, app.Name, port); err != nil {
			t.Fatal(err)
		}
		svc := &corev1.Service{}
		if err := cl.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: app.Name}, svc); err != nil {
			t.Fatal(err)
		}
		return portTargets(svc)
	}

	configured := edgeCachedApp(func(a *appv1alpha1.App) { a.Namespace, a.Spec.Port = "tea-a", 3000 })
	got := applied(t, configured, 3000, sidecarPod(configured, "kept", releaseRevision(configured), []corev1.ContainerPort{{Name: servingPortName, ContainerPort: 3000}}))
	if serving := intstr.FromString(servingPortName); got[3000] != serving || got[appv1alpha1.PrivateHTTPPort] != serving {
		t.Fatalf("configured port: targets = %v, want 3000 and 10000 on the app's own listener", got)
	}

	image := webImageApp(2)
	image.Namespace, image.Spec.Tier, image.Spec.CacheProfile = "tea-a", "pro", appv1alpha1.CacheProfileStatic
	image.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "nginx@sha256:b", Revision: "rev-2", PrimaryPort: 8080, Ports: []int32{8080, 9000},
		HTTPSelection: appv1alpha1.HTTPSelectionDetected, ConfiguredPort: new(int32(0))}
	image.Status.ServingNetwork = image.Status.ImageNetwork.DeepCopy()
	got = applied(t, image, 8080, sidecarPod(image, "kept", "rev-2", nil))
	want := map[int32]intstr.IntOrString{8080: intstr.FromInt32(8080), 9000: intstr.FromInt32(9000), 10000: intstr.FromInt32(8080)}
	if !maps.Equal(got, want) {
		t.Fatalf("image port: targets = %v, want %v", got, want)
	}
}

// tierReconciler is an AppReconciler on a fake client holding objs.
func tierReconciler(t *testing.T, objs ...client.Object) (*AppReconciler, client.Client) {
	t.Helper()
	scheme := wakeScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithStatusSubresource(&appsv1.Deployment{}).Build()
	return &AppReconciler{Client: cl, Scheme: scheme, Mode: ModeKubernetes, BackupHelperImage: "ghcr.io/bex-co/bex@sha256:aaaa"}, cl
}

func tierDeployment(t *testing.T, cl client.Client, app *appv1alpha1.App) *appsv1.Deployment {
	t.Helper()
	dep := &appsv1.Deployment{}
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: edgeCacheTierName(app.Name)}, dep); err != nil {
		t.Fatal(err)
	}
	return dep
}

// The tier is bex's own workload in the tenant's namespace: no instance of
// the service (no app label, so no Service of its own, instance list, metric,
// log or bill selects it), two replicas spread across nodes that never
// leave together, a bounded disk, and the namespace's environment labels so
// the environment's network boundary admits it beside the service.
func TestTheTierIsTwoReplicasOfBexsOwnProxyBesideTheService(t *testing.T) {
	app := edgeCachedApp(func(a *appv1alpha1.App) {
		a.Namespace = "tea-a"
		a.Labels = map[string]string{labelWorkspace: "tea-a", labelEnvironment: "evm-1", labelNetworkIsolation: "evm-1"}
	})
	r, cl := tierReconciler(t, app)
	if err := r.reconcileEdgeCacheTier(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	dep := tierDeployment(t, cl, app)
	tmpl := dep.Spec.Template
	if *dep.Spec.Replicas != 2 || dep.Spec.Strategy.RollingUpdate.MaxUnavailable.IntValue() != 0 {
		t.Fatalf("replicas %d, strategy %+v: want two, never both down in a rollout", *dep.Spec.Replicas, dep.Spec.Strategy)
	}
	if _, ok := tmpl.Labels[labelApp]; ok {
		t.Fatalf("tier pod labels %v carry the app label", tmpl.Labels)
	}
	for _, key := range []string{labelWorkspace, labelEnvironment, labelNetworkIsolation} {
		if tmpl.Labels[key] != app.Labels[key] {
			t.Fatalf("tier pod label %s = %q, want the App's %q", key, tmpl.Labels[key], app.Labels[key])
		}
	}
	if svcSelector, _, _ := privateServiceProjection(app, 8080); labels.SelectorFromSet(svcSelector).Matches(labels.Set(tmpl.Labels)) {
		t.Fatal("the service's own Service selects the tier pods")
	}
	proxy := tmpl.Spec.Containers[0]
	env := envMap(proxy.Env)
	if env["BEX_EDGE_CACHE_UPSTREAM"] != "http://hello:10000" || env["BEX_EDGE_CACHE_PEERS"] != edgeCacheTierName(app.Name) || env["BEX_EDGE_CACHE_DIR"] != edgeCacheStoreDir {
		t.Fatalf("proxy env = %v", env)
	}
	if _, profile := env["BEX_EDGE_CACHE_PROFILE"]; profile {
		t.Fatal("the template carries the profile, so a switch would roll the tier")
	}
	if tmpl.Spec.AutomountServiceAccountToken == nil || *tmpl.Spec.AutomountServiceAccountToken || proxy.SecurityContext.Capabilities == nil ||
		!slices.Equal(proxy.SecurityContext.Capabilities.Drop, []corev1.Capability{"ALL"}) {
		t.Fatalf("the tier pod is not restricted: %+v", tmpl.Spec)
	}
	var store *corev1.EmptyDirVolumeSource
	for _, v := range tmpl.Spec.Volumes {
		if v.Name == edgeCacheStoreVolumeName {
			store = v.EmptyDir
		}
	}
	if store == nil || store.SizeLimit == nil || store.SizeLimit.Value() < edgeCacheMaxBytes+4*edgeCacheMaxObjectBytes {
		t.Fatalf("store volume %+v must bound the cache and its four fills", store)
	}
	svc := &corev1.Service{}
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: edgeCacheTierName(app.Name)}, svc); err != nil {
		t.Fatal(err)
	}
	if svc.Spec.ClusterIP != corev1.ClusterIPNone || !maps.Equal(svc.Spec.Selector, dep.Spec.Selector.MatchLabels) {
		t.Fatalf("tier Service %+v must be headless over the tier pods", svc.Spec)
	}
	// A second pass writes nothing: the projection matches the stored object.
	before := dep.ResourceVersion
	if err := r.reconcileEdgeCacheTier(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	if got := tierDeployment(t, cl, app).ResourceVersion; got != before {
		t.Fatal("a steady pass rewrote the tier Deployment")
	}
}

// An operator release alone keeps the tier's image (the rule w6/m484 sets
// for the service's own pods): restarting the tier would empty the cache.
// A successful deploy, which empties it anyway, moves the tier onto the
// operator's image.
func TestTheTierKeepsItsImageUntilASuccessfulDeploy(t *testing.T) {
	app := edgeCachedApp(func(a *appv1alpha1.App) {
		a.Namespace, a.Generation = "tea-a", 1
		a.Status.ReleaseGeneration, a.Status.ActiveRevision = 1, "rev-1"
	})
	appDep := project(app, execParams())
	appDep.Generation = 1
	// Observed well past any generation the fake client stamps on an update.
	appDep.Status = appsv1.DeploymentStatus{ObservedGeneration: 10, Replicas: 2, UpdatedReplicas: 2}
	r, cl := tierReconciler(t, app, appDep)
	if err := r.reconcileEdgeCacheTier(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	image := func() string { return tierDeployment(t, cl, app).Spec.Template.Spec.Containers[0].Image }
	first := image()
	r.BackupHelperImage = "ghcr.io/bex-co/bex@sha256:bbbb"
	if err := r.reconcileEdgeCacheTier(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	if got := image(); got != first {
		t.Fatalf("an operator release moved the tier to %s", got)
	}
	app.Status.ReleaseGeneration, app.Status.ActiveRevision = 2, "rev-2"
	appDep.Spec.Template.Labels[labelRevision] = "rev-2"
	if err := cl.Update(t.Context(), appDep); err != nil {
		t.Fatal(err)
	}
	if err := r.reconcileEdgeCacheTier(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	if got := image(); got != r.BackupHelperImage {
		t.Fatalf("a successful deploy left the tier on %s, want the operator's %s", got, r.BackupHelperImage)
	}
	if got := tierDeployment(t, cl, app).Annotations[edgecache.RevisionAnnotation]; got != "rev-2" {
		t.Fatalf("tier revision = %q, want rev-2", got)
	}
}

// Render purges "each time you successfully deploy", "failed deploys do not
// trigger a purge", and a manual purge does the same; nothing else empties
// its CDN (render.com/docs/web-service-caching). edgeCacheRevision is the
// release whose deploy last emptied the tier.
func TestTheCacheEmptiesOnlyOnASuccessfulDeploy(t *testing.T) {
	live := func(generation int64, mutate ...func(*appv1alpha1.App)) *appv1alpha1.App {
		a := edgeCachedApp(func(a *appv1alpha1.App) {
			a.Generation = generation
			a.Status.ReleaseGeneration = generation
			a.Status.ActiveRevision = "rev-" + strconv.FormatInt(generation, 10)
		})
		for _, m := range mutate {
			m(a)
		}
		return a
	}
	rolledOut := func(revision string) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Generation: 3},
			Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{labelRevision: revision}}}},
			Status:     appsv1.DeploymentStatus{ObservedGeneration: 3, Replicas: 2, UpdatedReplicas: 2},
		}
	}
	rolling := rolledOut("rev-2")
	rolling.Status.Replicas = 3 // an old instance still serves
	restart := func(a *appv1alpha1.App) {
		a.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 2, SourceGeneration: 1, Image: "registry.test/web@sha256:1", PreserveGroupValues: true}
		a.Status.SelectedReleaseGeneration = 2
	}
	rollback := func(a *appv1alpha1.App) {
		a.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 2, SourceGeneration: 1, Image: "registry.test/web@sha256:0"}
		a.Status.SelectedReleaseGeneration = 2
	}
	failed := live(1, func(a *appv1alpha1.App) { a.Generation, a.Status.ReleaseGeneration = 2, 2 })
	// stamped is bex-api opening release generation g. Until the operator
	// adopts it, the status still names the active release while
	// spec.releaseConfig is the new request's: none for a plain deploy, an
	// image reference for an image deploy.
	stamped := func(g int64, ref *appv1alpha1.ReleaseConfigReference) func(*appv1alpha1.App) {
		return func(a *appv1alpha1.App) {
			metav1.SetMetaDataAnnotation(&a.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(g, 10))
			a.Spec.ReleaseConfig = ref
		}
	}
	imageDeploy := &appv1alpha1.ReleaseConfigReference{Generation: 3, Image: "registry.test/web:missing"}
	for name, tc := range map[string]struct {
		app  *appv1alpha1.App
		dep  *appsv1.Deployment
		last string
		want string
	}{
		"a new tier starts on the active release": {live(1), rolledOut("rev-1"), "", "rev-1"},
		"a steady release":                        {live(1), rolledOut("rev-1"), "rev-1", "rev-1"},
		"a successful deploy, rolled out":         {live(2), rolledOut("rev-2"), "rev-1", "rev-2"},
		"a successful deploy, old pods remain":    {live(2), rolling, "rev-1", "rev-1"},
		"a deploy whose template is still old":    {live(2), rolledOut("rev-1"), "rev-1", "rev-1"},
		"a failed or pending deploy":              {failed, rolledOut("rev-2"), "rev-1", "rev-1"},
		"a Restart":                               {live(2, restart), rolledOut("rev-2"), "rev-1", "rev-1"},
		"a Rollback is a deploy":                  {live(2, rollback), rolledOut("rev-2"), "rev-1", "rev-2"},
		// The deploy after a Restart is not yet adopted: the Restart is still
		// no new version, and that deploy may yet fail.
		"a deploy requested after a Restart":         {live(2, restart, stamped(3, nil)), rolledOut("rev-2"), "rev-1", "rev-1"},
		"an image deploy requested after a Restart":  {live(2, restart, stamped(3, imageDeploy)), rolledOut("rev-2"), "rev-1", "rev-1"},
		"a successful deploy whose stamp is adopted": {live(2, stamped(2, nil)), rolledOut("rev-2"), "rev-1", "rev-2"},
		"no Deployment to read":                      {live(2), nil, "rev-1", "rev-1"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := edgeCacheRevision(tc.app, tc.dep, tc.last); got != tc.want {
				t.Fatalf("revision = %q, want %q", got, tc.want)
			}
		})
	}
}

// The public route goes through the tier only while one of its pods is
// Ready, so turning caching on, a tier that cannot start, and the loss of
// every tier pod serve uncached instead of failing; and the precedence
// before it (maintenance, suspension, sleep) is unchanged.
func TestTheRouteFollowsTheTiersReadiness(t *testing.T) {
	app := edgeCachedApp(func(a *appv1alpha1.App) {
		a.Namespace = "tea-a"
		a.Status.ActiveRevision = "rev-1"
	})
	tier := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: edgeCacheTierName(app.Name), Namespace: app.Namespace}}
	r, cl := tierReconciler(t, app, tier)
	backend := func(a *appv1alpha1.App) string {
		t.Helper()
		svc, _, err := r.ingressBackend(t.Context(), a, 8080, false, true)
		if err != nil {
			t.Fatal(err)
		}
		return svc
	}
	if got := backend(app); got != app.Name {
		t.Fatalf("with no Ready tier pod the route names %s, want the service", got)
	}
	tier.Status.ReadyReplicas = 1
	if err := cl.Status().Update(t.Context(), tier); err != nil {
		t.Fatal(err)
	}
	if got := backend(app); got != edgeCacheTierName(app.Name) {
		t.Fatalf("with a Ready tier pod the route names %s, want the tier", got)
	}
	off := app.DeepCopy()
	off.Spec.CacheProfile = appv1alpha1.CacheProfileNone
	if got := backend(off); got != app.Name {
		t.Fatalf("with caching off the route names %s, want the service", got)
	}
	r.MaintenanceNamespace = app.Namespace
	maintained := app.DeepCopy()
	maintained.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
	if got := backend(maintained); got != r.maintenanceService() {
		t.Fatalf("in maintenance the route names %s, want the maintenance page", got)
	}
}

// Turning caching off keeps the tier until no Ingress of the App routes to
// it, so the route never names a Service that is gone.
func TestRetiringTheTierWaitsForTheRoute(t *testing.T) {
	app := edgeCachedApp(func(a *appv1alpha1.App) { a.Namespace, a.Spec.CacheProfile = "tea-a", "" })
	name := edgeCacheTierName(app.Name)
	ing := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace}, Spec: networkingv1.IngressSpec{
		Rules: []networkingv1.IngressRule{{Host: "hello.edge.test", IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
			Paths: []networkingv1.HTTPIngressPath{{Path: "/", Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
				Name: name, Port: networkingv1.ServiceBackendPort{Number: edgeCacheTierPort}}}}},
		}}}},
	}}
	pod := tierPod(app, "tier-0", func(p *corev1.Pod) {
		p.Annotations = map[string]string{edgecache.ProfileAnnotation: appv1alpha1.CacheProfileStatic}
	})
	r, cl := tierReconciler(t, app, ing, pod,
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace}})
	if err := r.reconcileEdgeCacheTier(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: name}, &appsv1.Deployment{}); err != nil {
		t.Fatalf("the tier went while the route still named it: %v", err)
	}
	// Until the route moves, the proxies stop caching.
	if err := cl.Get(t.Context(), client.ObjectKeyFromObject(pod), pod); err != nil || pod.Annotations[edgecache.ProfileAnnotation] != appv1alpha1.CacheProfileNone {
		t.Fatalf("a tier still routed after caching was turned off caches by %q (%v)", pod.Annotations[edgecache.ProfileAnnotation], err)
	}
	ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name = app.Name
	if err := cl.Update(t.Context(), ing); err != nil {
		t.Fatal(err)
	}
	if err := r.reconcileEdgeCacheTier(t.Context(), app); err != nil {
		t.Fatal(err)
	}
	for _, obj := range []client.Object{&appsv1.Deployment{}, &corev1.Service{}} {
		if err := cl.Get(t.Context(), client.ObjectKey{Namespace: app.Namespace, Name: name}, obj); !apierrors.IsNotFound(err) {
			t.Fatalf("%T outlived the route: %v", obj, err)
		}
	}
}

// tierPod is a running pod of app's tier, owned by a ReplicaSet.
func tierPod(app *appv1alpha1.App, name string, mutate ...func(*corev1.Pod)) *corev1.Pod {
	dep := &appsv1.Deployment{}
	projectEdgeCacheTier(dep, app, "ghcr.io/bex-co/bex:test")
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: app.Namespace, Labels: maps.Clone(dep.Spec.Template.Labels),
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "tier-rs", UID: "rs-uid", Controller: new(true)}}},
		Spec: dep.Spec.Template.Spec,
	}
	for _, m := range mutate {
		m(p)
	}
	return p
}

// The live purge stamp, profile and deploy revision reach every running tier
// pod, and nothing else: not a leaving pod, the service's own pods (a kept
// sidecar receives no request), or another App's tier.
func TestAnnotateEdgeCacheTierPodsHandsTheProxiesThePurgeProfileAndRevision(t *testing.T) {
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
			current := map[string]string{edgecache.ProfileAnnotation: tc.wantProfile, edgecache.PurgedAtAnnotation: "2026-11-18T10:00:00Z", edgecache.RevisionAnnotation: "rev-3"}
			other := edgeCachedApp(func(a *appv1alpha1.App) { a.Name, a.Namespace = "other", "tea-a" })
			own := storedSidecarDeployment(tc.app, execParams(), true).Spec.Template
			pods := []client.Object{
				tierPod(tc.app, "fresh"),
				tierPod(tc.app, "current", func(p *corev1.Pod) { p.Annotations = maps.Clone(current) }),
				tierPod(tc.app, "leaving", terminatingPod),
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "instance", Namespace: "tea-a", Labels: own.Labels}, Spec: own.Spec},
				tierPod(other, "other"),
			}
			cl := fake.NewClientBuilder().WithScheme(wakeScheme()).WithObjects(pods...).Build()
			if err := (&AppReconciler{Client: cl}).annotateEdgeCacheTierPods(t.Context(), tc.app, "rev-3"); err != nil {
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

// A tier pod created after the App's last purge started empty: handing it
// that stamp would only purge it again once its proxy is up, so it is not.
func TestATierPodBornAfterAPurgeIsNotHandedIt(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC))
	app := edgeCachedApp(func(a *appv1alpha1.App) {
		a.Namespace = "tea-a"
		a.Annotations = map[string]string{appv1alpha1.CachePurgedAtAnnotation: "2026-11-18T11:59:59.5Z"}
	})
	pod := tierPod(app, "fresh", func(p *corev1.Pod) { p.CreationTimestamp = created })
	cl := fake.NewClientBuilder().WithScheme(wakeScheme()).WithObjects(pod).Build()
	r := &AppReconciler{Client: cl}
	if err := r.annotateEdgeCacheTierPods(t.Context(), app, "rev-1"); err != nil {
		t.Fatal(err)
	}
	got := &corev1.Pod{}
	if err := cl.Get(t.Context(), client.ObjectKeyFromObject(pod), got); err != nil {
		t.Fatal(err)
	}
	if _, stamped := got.Annotations[edgecache.PurgedAtAnnotation]; stamped {
		t.Fatalf("a pod born after the purge was handed it: %v", got.Annotations)
	}
	written := got.ResourceVersion
	if err := r.annotateEdgeCacheTierPods(t.Context(), app, "rev-1"); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(t.Context(), client.ObjectKeyFromObject(pod), got); err != nil || got.ResourceVersion != written {
		t.Fatalf("a current pod was patched again (%v)", err)
	}
}

func terminatingPod(p *corev1.Pod) {
	p.DeletionTimestamp, p.Finalizers = &metav1.Time{Time: time.Now()}, []string{"test.bex.co/hold"}
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
		Data:       map[string][]byte{appv1alpha1.ReleaseRecordAdoptedSpecKey: recorded},
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
