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
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// refusedTag is the tag refusingImageRegistry refuses; every other reference
// reads as imageConfigRegistry's one image, which declares port 80.
const refusedTag = "v0.0.99-qa3221"

// noHostingPlatform is the refusingImageRegistry status that serves an index
// without the hosting platform's manifest: a read that fails without a
// registry refusal (ImageMetadataUnavailable).
const noHostingPlatform = 0

// refusingImageRegistry is imageConfigRegistry with one refused tag. status is
// the registry's answer to that tag's manifest read; a 401 comes after the
// Bearer challenge and token exchange, as Docker Hub answers an anonymous read
// of an image it will not serve. reads counts the refused tag's manifest reads.
func refusingImageRegistry(status int, reads *atomic.Int32) *http.Client {
	served := imageConfigRegistry(GinkgoT(), `"80/tcp":{}`).Transport
	armOnly := fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[{"digest":"sha256:%x","platform":{"os":"linux","architecture":"arm64"}}]}`, sha256.Sum256([]byte("arm64")))
	answer := func(code int, body string, header http.Header) (*http.Response, error) {
		if header == nil {
			header = make(http.Header)
		}
		return &http.Response{StatusCode: code, Header: header, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
	return &http.Client{Transport: imageNetworkTransport(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.Host == "auth.registry.test":
			return answer(http.StatusOK, `{"token":"anonymous"}`, nil)
		case !strings.HasSuffix(request.URL.Path, "/manifests/"+refusedTag):
			return served.RoundTrip(request)
		}
		reads.Add(1)
		switch {
		case status == noHostingPlatform:
			return answer(http.StatusOK, armOnly, nil)
		case status == http.StatusUnauthorized && request.Header.Get("Authorization") == "":
			challenge := http.Header{"Www-Authenticate": []string{`Bearer realm="https://auth.registry.test/token",service="registry.test",scope="repository:traefik/whoami:pull"`}}
			return answer(status, `{"errors":[{"code":"UNAUTHORIZED"}]}`, challenge)
		}
		return answer(status, `{"errors":[{"code":"MANIFEST_UNKNOWN","message":"manifest unknown"}]}`, nil)
	})}
}

// delayedTag is the tag throttlingImageRegistry delays.
const delayedTag = "v1.10.4"

// throttlingImageRegistry is imageConfigRegistry whose delayed tag fails
// transiently until recovered is set: its manifest read answers status with
// Retry-After 120, or fails as a request when status is 0. reads counts that
// tag's failed reads.
func throttlingImageRegistry(status int, recovered *atomic.Bool, reads *atomic.Int32) *http.Client {
	served := imageConfigRegistry(GinkgoT(), `"80/tcp":{}`).Transport
	return &http.Client{Transport: imageNetworkTransport(func(request *http.Request) (*http.Response, error) {
		if recovered.Load() || !strings.HasSuffix(request.URL.Path, "/manifests/"+delayedTag) {
			return served.RoundTrip(request)
		}
		reads.Add(1)
		if status == 0 {
			return nil, errors.New("connection reset by peer")
		}
		header := http.Header{"Retry-After": []string{"120"}}
		return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(`{"errors":[{"code":"TOOMANYREQUESTS"}]}`))}, nil
	})}
}

// w32/m419: a deploy whose image the registry refused before any pod was
// dispatched (a tag that does not exist, a refused credential) left a served
// image service frozen. Every later pass re-read the refused image and stopped
// at failBeforeDispatch, so no setting that acts without a deploy reached the
// running service: an IP allow-list saved, read back and was never enforced, a
// suspend left the service answering, and reopening a restricted service locked
// its owner out until the next good deploy. The serving release now keeps
// following the App, as it does after a failed build (w1/m157), and the
// registry is not read again for a refusal already recorded.
var _ = Describe("Settings over a refused image release (w32/m419)", func() {
	const (
		served  = "docker.io/traefik/whoami:v1.10.3"
		refused = "docker.io/traefik/whoami:" + refusedTag
	)

	type fixture struct {
		nn    types.NamespacedName
		r     *AppReconciler
		reads *atomic.Int32
	}
	run := func(f *fixture) func() (ctrl.Result, error) {
		return func() (ctrl.Result, error) { return f.r.Reconcile(ctx, reconcile.Request{NamespacedName: f.nn}) }
	}

	// newFixture creates an image service of serviceType on tier running image,
	// whose registry answers the refused tag with status.
	newFixture := func(name, serviceType, tier, image string, status int) *fixture {
		GinkgoHelper()
		f := &fixture{nn: types.NamespacedName{Name: name, Namespace: "default"}, reads: &atomic.Int32{}}
		f.r = &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			BaseDomain: "onbex.co", ActivatorService: "bex-activator", ActivatorPort: 8888,
			MaintenanceService: "bex-activator", MaintenancePort: 8888,
			HTTPClient: refusingImageRegistry(status, f.reads),
		}
		portMode := appv1alpha1.PortModeWebImageV1
		if serviceType == appv1alpha1.TypePrivateService {
			portMode = appv1alpha1.PortModeImageV1
		}
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default",
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{
				Image: image, Type: serviceType, PortMode: portMode, Replicas: 1, Tier: tier,
				Expose: serviceType == appv1alpha1.TypeWebService,
			},
		})).To(Succeed())
		DeferCleanup(func() { deleteOwner(ctx, envtestApp(f.nn), run(f)) })
		return f
	}

	// pass reconciles a few times. A held pass returns no error: the failure
	// is recorded as the refused release's verdict, not retried.
	pass := func(f *fixture) {
		GinkgoHelper()
		for range 4 {
			_, err := run(f)()
			Expect(err).NotTo(HaveOccurred())
		}
	}
	update := func(f *fixture, mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		app := envtestApp(f.nn)
		mutate(app)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		pass(f)
	}
	// deploy stamps what bex-api's deploy verbs do: a release generation, a
	// restart stamp and, for a per-deploy image or a rollback, the release's
	// configuration reference.
	deploy := func(f *fixture, ref *appv1alpha1.ReleaseConfigReference) int64 {
		GinkgoHelper()
		var gen int64
		update(f, func(app *appv1alpha1.App) {
			gen = app.Generation + 1
			app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if ref != nil {
				ref.Generation = gen
				app.Spec.ReleaseConfig = ref
			}
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(gen, 10))
		})
		return gen
	}
	// saveImage is Update Service saving spec.image without a deploy.
	saveImage := func(f *fixture, image string) {
		GinkgoHelper()
		update(f, func(app *appv1alpha1.App) {
			app.Spec.Image = image
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationPendingSourceGeneration, strconv.FormatInt(app.Generation+1, 10))
		})
	}
	serve := func(f *fixture) {
		GinkgoHelper()
		markEnvtestDeploymentReady(f.nn)
		pass(f)
	}
	goLive := func(f *fixture) {
		GinkgoHelper()
		if app := envtestApp(f.nn); app.Spec.UsesWebImagePorts() && app.Status.ImageNetwork != nil && app.Status.ImageNetwork.HTTPSelection == "" {
			// Stand in for the completed HTTP port probe (w8/m142).
			app.Status.ImageNetwork.HTTPSelection = appv1alpha1.HTTPSelectionDetected
			Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
			pass(f)
		}
		finishImagePortRollout(f.nn, run(f))
		pass(f)
	}
	middleware := func(f *fixture, suffix string) []any {
		GinkgoHelper()
		o := &unstructured.Unstructured{}
		o.SetGroupVersionKind(traefikHTTPMiddlewareGVK)
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: f.nn.Name + suffix, Namespace: f.nn.Namespace}, o); err != nil {
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "%v", err)
			return nil
		}
		ranges, _, _ := unstructured.NestedSlice(o.Object, "spec", "ipAllowList", "sourceRange")
		return ranges
	}
	ingress := func(f *fixture) *networkingv1.Ingress {
		GinkgoHelper()
		ing := &networkingv1.Ingress{}
		Expect(k8sClient.Get(ctx, f.nn, ing)).To(Succeed())
		return ing
	}
	chained := func(f *fixture) string {
		GinkgoHelper()
		return ingress(f).Annotations[traefikRouterMiddlewaresAnnotation]
	}
	backend := func(f *fixture) string {
		GinkgoHelper()
		b := ingress(f).Spec.Rules[0].HTTP.Paths[0].Backend.Service
		return fmt.Sprintf("%s:%d", b.Name, b.Port.Number)
	}
	replicas := func(f *fixture) int32 {
		GinkgoHelper()
		return *envtestDeployment(f.nn).Spec.Replicas
	}
	// refuse brings f to a served release whose newest deploy the registry
	// refused, through a per-deploy image or a saved one, failing under reason.
	// It returns the refused release's generation and held, which asserts
	// w4/533's verdict: the refused release's Rollout condition carries the
	// refusal to its deploy row, which bex-api closes update_failed, and the
	// service reads the release that serves, never a failure bex-api would
	// report as server_failed. The refused release never reaches the pod
	// template.
	refuse := func(f *fixture, saved bool, reason string) (int64, func()) {
		GinkgoHelper()
		By("serving whoami:v1.10.3")
		pass(f)
		goLive(f)
		Expect(envtestApp(f.nn).Status.ActiveRevision).To(Equal("rev-1"))
		servedTemplate := envtestDeployment(f.nn).Spec.Template.DeepCopy()
		By("deploying a tag the registry refuses")
		var gen int64
		if saved {
			saveImage(f, refused)
			Expect(f.reads.Load()).To(BeZero(), "a save reads nothing")
			gen = deploy(f, nil)
		} else {
			gen = deploy(f, &appv1alpha1.ReleaseConfigReference{Image: refused})
		}
		Expect(f.reads.Load()).To(BeNumerically(">", 0))
		held := func() {
			GinkgoHelper()
			app := envtestApp(f.nn)
			rollout := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionRollout)
			Expect(rollout).NotTo(BeNil())
			Expect(rollout.Status).To(Equal(metav1.ConditionFalse))
			Expect(rollout.Reason).To(Equal(reason))
			Expect(rollout.ObservedGeneration).To(Equal(gen))
			Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
			Expect(app.Status.Phase).To(BeElementOf(appv1alpha1.PhaseRunning, appv1alpha1.PhaseHibernated))
			ready := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
			Expect(ready).NotTo(BeNil())
			Expect(ready.Reason).To(BeElementOf(reasonPriorReleaseServing, reasonSuspended, reasonAutoHibernated), "%+v", ready)
			got := envtestDeployment(f.nn).Spec.Template
			Expect(equality.Semantic.DeepEqual(*servedTemplate, got)).To(BeTrue(),
				"want labels %v\ngot  labels %v", servedTemplate.Labels, got.Labels)
		}
		held()
		return gen, held
	}

	// settings walks the settings a held release must follow without a deploy:
	// the IP allow-list restricted and reopened, then a suspend and a resume,
	// which leave the phases given. check runs after each but the reopen.
	settings := func(f *fixture, check func(), suspended, resumed appv1alpha1.AppPhase) {
		GinkgoHelper()
		name := f.nn.Name
		By("restricting the IP allow-list: enforced on the serving route")
		update(f, func(app *appv1alpha1.App) {
			app.Spec.SetIPAllowListEntries([]appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.0/24", Description: "testnet"}})
		})
		Expect(middleware(f, "-ip-allow")).To(Equal([]any{"203.0.113.0/24"}))
		Expect(chained(f)).To(ContainSubstring(name + "-ip-allow"))
		check()

		By("reopening it")
		update(f, func(app *appv1alpha1.App) { app.Spec.SetIPAllowListEntries(nil) })
		Expect(middleware(f, "-ip-allow")).To(BeNil())
		Expect(chained(f)).NotTo(ContainSubstring(name + "-ip-allow"))

		By("suspending: zero replicas, the hosts on the suspended responder")
		update(f, func(app *appv1alpha1.App) { app.Spec.Suspended = true })
		Expect(replicas(f)).To(BeZero())
		Expect(backend(f)).To(Equal(activatorAliasName(name) + ":8888"))
		Expect(envtestApp(f.nn).Status.Phase).To(Equal(suspended))
		check()

		By("resuming")
		update(f, func(app *appv1alpha1.App) { app.Spec.Suspended = false })
		Expect(replicas(f)).To(Equal(int32(1)))
		serve(f)
		Expect(backend(f)).To(Equal(name + ":80"))
		Expect(envtestApp(f.nn).Status.Phase).To(Equal(resumed))
		check()
	}

	type refusal struct {
		status   int
		reason   string
		saved    bool // the tag saved with Update Service, then deployed
		rollback bool // the way out is a rollback to the served release
	}
	DescribeTable("keeps the serving release following the App",
		func(c refusal) {
			name := fmt.Sprintf("refused-%d", c.status)
			if c.saved {
				name += "-saved"
			}
			if c.rollback {
				name += "-rollback"
			}
			f := newFixture(name, appv1alpha1.TypeWebService, "starter", served, c.status)
			_, held := refuse(f, c.saved, c.reason)
			Expect(backend(f)).To(Equal(name + ":80"))

			By("not reading a recorded refusal again")
			reads := f.reads.Load()
			pass(f)
			Expect(f.reads.Load()).To(Equal(reads))

			settings(f, held, appv1alpha1.PhaseHibernated, appv1alpha1.PhaseRunning)
			Expect(f.reads.Load()).To(Equal(reads), "no setting reads the refused image again")

			if c.saved {
				By("saving an image the registry serves: nothing rolls until a deploy")
				saveImage(f, served)
				held()
			}
			var next int64
			if c.rollback {
				By("rolling back to the served release: it goes live")
				next = deploy(f, &appv1alpha1.ReleaseConfigReference{
					SourceGeneration: 1, Image: envtestApp(f.nn).Status.ServingNetwork.Image, PreserveGroupValues: true,
				})
			} else {
				By("deploying an image the registry serves: it goes live")
				next = deploy(f, nil)
			}
			goLive(f)
			app := envtestApp(f.nn)
			Expect(app.Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", next)))
			Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
			Expect(envtestDeployment(f.nn).Spec.Template.Labels[labelRevision]).To(Equal(fmt.Sprintf("rev-%d", next)))
		},
		Entry("a missing tag (404) deployed as a per-deploy image", refusal{status: http.StatusNotFound, reason: appv1alpha1.ReasonImagePullFailed}),
		Entry("a missing tag (404) saved, then deployed", refusal{status: http.StatusNotFound, reason: appv1alpha1.ReasonImagePullFailed, saved: true}),
		Entry("a refused credential (401), then a rollback", refusal{status: http.StatusUnauthorized, reason: appv1alpha1.ReasonImagePullFailed, rollback: true}),
		Entry("a forbidden image (403), saved", refusal{status: http.StatusForbidden, reason: appv1alpha1.ReasonImagePullFailed, saved: true}),
		Entry("another refusal (400)", refusal{status: http.StatusBadRequest, reason: appv1alpha1.ReasonImagePullFailed}),
		Entry("an image without the hosting platform (ImageMetadataUnavailable)", refusal{status: noHostingPlatform, reason: reasonImageMetadataUnavailable}),
	)

	It("converges every other setting that acts without a deploy", func() {
		const name = "refused-matrix"
		f := newFixture(name, appv1alpha1.TypeWebService, "starter", served, http.StatusNotFound)
		gen, held := refuse(f, false, appv1alpha1.ReasonImagePullFailed)

		By("the environment layer (w4/m28)")
		update(f, func(app *appv1alpha1.App) { app.Spec.EnvironmentIPAllowList = []string{"203.0.113.0/24"} })
		Expect(middleware(f, "-env-ip-allow")).To(Equal([]any{"203.0.113.0/24"}))
		Expect(chained(f)).To(ContainSubstring(name + "-env-ip-allow"))
		update(f, func(app *appv1alpha1.App) { app.Spec.EnvironmentIPAllowList = nil })
		Expect(middleware(f, "-env-ip-allow")).To(BeNil())

		By("the workspace layer (w9/m388)")
		update(f, func(app *appv1alpha1.App) { app.Spec.WorkspaceIPAllowList = []string{"198.51.100.0/24"} })
		Expect(middleware(f, "-ws-ip-allow")).To(Equal([]any{"198.51.100.0/24"}))
		Expect(chained(f)).To(ContainSubstring(name + "-ws-ip-allow"))
		update(f, func(app *appv1alpha1.App) { app.Spec.WorkspaceIPAllowList = nil })
		Expect(middleware(f, "-ws-ip-allow")).To(BeNil())
		held()

		By("a custom domain, added and removed")
		update(f, func(app *appv1alpha1.App) { app.Spec.Hosts = []string{"www.qa-refused.example.com"} })
		Expect(ingressHosts(ingress(f))).To(ContainElement("www.qa-refused.example.com"))
		update(f, func(app *appv1alpha1.App) { app.Spec.Hosts = nil })
		Expect(ingressHosts(ingress(f))).NotTo(ContainElement("www.qa-refused.example.com"))

		By("numInstances")
		update(f, func(app *appv1alpha1.App) { app.Spec.Replicas = 2 })
		Expect(replicas(f)).To(Equal(int32(2)))
		update(f, func(app *appv1alpha1.App) { app.Spec.Replicas = 1 })
		Expect(replicas(f)).To(Equal(int32(1)))

		By("autoscaling: the running count enters its range")
		f.r.MetricsReader = func(context.Context, string, string) ([]PodUsage, error) { return nil, nil }
		update(f, func(app *appv1alpha1.App) {
			app.Spec.Autoscaling = &appv1alpha1.AutoscalingSpec{Enabled: true, MinReplicas: 2, MaxReplicas: 3, TargetCPUPercent: new(int32(70))}
		})
		Expect(replicas(f)).To(Equal(int32(2)))
		update(f, func(app *appv1alpha1.App) { app.Spec.Autoscaling = nil })
		Expect(replicas(f)).To(Equal(int32(1)))
		held()

		By("maintenance mode")
		update(f, func(app *appv1alpha1.App) {
			app.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
		})
		Expect(backend(f)).To(Equal(maintenanceAliasName(name) + ":8888"))
		update(f, func(app *appv1alpha1.App) { app.Spec.MaintenanceMode = nil })
		serve(f)
		Expect(backend(f)).To(Equal(name + ":80"))
		held()

		By("an edge-cache profile acts without a deploy, as Render's CDN setting does (w3/m486)")
		update(f, func(app *appv1alpha1.App) { app.Spec.CacheProfile = appv1alpha1.CacheProfileStatic })
		Expect(envtestApp(f.nn).Status.ReleaseGeneration).To(Equal(gen))
		held()
	})

	It("sleeps and wakes a free service", func() {
		const name = "refused-free"
		f := newFixture(name, appv1alpha1.TypeWebService, "", served, http.StatusNotFound)
		_, held := refuse(f, false, appv1alpha1.ReasonImagePullFailed)

		By("idle past its window: the route moves to the activator, then it parks")
		update(f, func(app *appv1alpha1.App) {
			app.Annotations[annotLastActive] = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
		})
		Expect(backend(f)).To(Equal(activatorAliasName(name) + ":8888"))
		Expect(replicas(f)).To(BeZero())
		Expect(envtestApp(f.nn).Status.Phase).To(Equal(appv1alpha1.PhaseHibernated))
		held()

		By("a request wakes it on the serving release")
		update(f, func(app *appv1alpha1.App) {
			app.Annotations[annotLastActive] = time.Now().UTC().Format(time.RFC3339)
		})
		Expect(replicas(f)).To(Equal(int32(1)))
		serve(f)
		Expect(backend(f)).To(Equal(name + ":80"))
		Expect(envtestApp(f.nn).Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		held()
	})

	It("holds an image-port private service", func() {
		const name = "refused-pserv"
		f := newFixture(name, appv1alpha1.TypePrivateService, "starter", served, http.StatusNotFound)
		_, held := refuse(f, false, appv1alpha1.ReasonImagePullFailed)
		assertImagePortService(f.nn, 80)

		By("suspend and resume")
		update(f, func(app *appv1alpha1.App) { app.Spec.Suspended = true })
		Expect(replicas(f)).To(BeZero())
		held()
		update(f, func(app *appv1alpha1.App) { app.Spec.Suspended = false })
		Expect(replicas(f)).To(Equal(int32(1)))
		serve(f)
		held()

		By("numInstances")
		update(f, func(app *appv1alpha1.App) { app.Spec.Replicas = 2 })
		Expect(replicas(f)).To(Equal(int32(2)))
		assertImagePortService(f.nn, 80)
		held()

		By("the next deploy goes live")
		next := deploy(f, nil)
		goLive(f)
		Expect(envtestApp(f.nn).Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", next)))
	})

	It("keeps a first release's refusal an ordinary failure", func() {
		f := newFixture("refused-first", appv1alpha1.TypeWebService, "starter", refused, http.StatusNotFound)
		// The refusal comes back as a pass's error, for a retry.
		for range 4 {
			_, _ = run(f)()
		}
		app := envtestApp(f.nn)
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
		ready := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Reason).To(Equal(appv1alpha1.ReasonImagePullFailed))
		Expect(meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionRollout)).To(BeNil())
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, f.nn, &appsv1.Deployment{}))).To(BeTrue(), "nothing is dispatched")
	})

	// w32/031: a deploy whose image metadata read waits on a throttled or
	// failing registry (a 429, a 5xx, a failed request) returned before the
	// hold, so no setting reached the serving release for as long as the wait
	// lasted, and Docker Hub's anonymous limit can last hours. The wait holds
	// that release too, while the deploy stays open on it: no verdict, the
	// Deploying phase and the wait's stall message, until the registry answers
	// and the release goes live.
	Context("when the registry delays the deploy instead (w32/031)", func() {
		const delayed = "docker.io/traefik/whoami:" + delayedTag

		// newDelayedFixture is newFixture on a registry that delays delayedTag
		// with status, read through a gate whose clock stands still. answer
		// ends the delay and moves that clock past the host's breaker.
		newDelayedFixture := func(name, image string, status int) (f *fixture, clock *fakeClock, answer func()) {
			GinkgoHelper()
			f = newFixture(name, appv1alpha1.TypeWebService, "starter", image, http.StatusOK)
			recovered := &atomic.Bool{}
			f.r.HTTPClient = throttlingImageRegistry(status, recovered, f.reads)
			clock = &fakeClock{now: time.Now()}
			f.r.registryReads().now = clock.Now
			return f, clock, func() {
				recovered.Store(true)
				clock.Advance(imageRegistryRetryMax + time.Second)
			}
		}
		stall := func(f *fixture) *metav1.Condition {
			GinkgoHelper()
			app := envtestApp(f.nn)
			Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseDeploying))
			ready := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
			Expect(ready).NotTo(BeNil())
			Expect(ready.ObservedGeneration).To(Equal(app.Generation), "the deploy row reads a current stall")
			return ready
		}

		// requeue is the wait's own cadence on the second failed read: the
		// registry's Retry-After, else the App's backoff (registryRetryDelay).
		DescribeTable("keeps the serving release following the App while the deploy waits",
			func(status int, reason, message string, requeue time.Duration) {
				name := fmt.Sprintf("delayed-%d", status)
				f, clock, answer := newDelayedFixture(name, served, status)
				By("serving whoami:v1.10.3")
				pass(f)
				goLive(f)
				servedTemplate := envtestDeployment(f.nn).Spec.Template.DeepCopy()

				By("deploying a tag the registry delays")
				gen := deploy(f, &appv1alpha1.ReleaseConfigReference{Image: delayed})
				waiting := func() {
					GinkgoHelper()
					ready := stall(f)
					Expect(ready.Reason).To(Equal(reason))
					Expect(ready.Message).To(Equal(message))
					app := envtestApp(f.nn)
					if rollout := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionRollout); rollout != nil {
						Expect(rollout.ObservedGeneration).To(BeNumerically("<", gen), "the waiting release has no verdict")
					}
					Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
					Expect(equality.Semantic.DeepEqual(*servedTemplate, envtestDeployment(f.nn).Spec.Template)).To(BeTrue())
				}
				waiting()
				Expect(f.reads.Load()).To(Equal(int32(1)), "the host's breaker turns the later passes away")

				settings(f, waiting, appv1alpha1.PhaseDeploying, appv1alpha1.PhaseDeploying)
				Expect(f.reads.Load()).To(Equal(int32(1)))

				By("reading again once the breaker allows: the pass keeps the wait's cadence")
				clock.Advance(imageRegistryRetry + time.Second)
				res, err := run(f)()
				Expect(err).NotTo(HaveOccurred())
				Expect(f.reads.Load()).To(Equal(int32(2)))
				Expect(res.RequeueAfter).To(Equal(requeue))
				waiting()

				By("the registry answering: the waiting release goes live")
				answer()
				pass(f)
				goLive(f)
				app := envtestApp(f.nn)
				Expect(app.Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", gen)))
				Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
				Expect(envtestDeployment(f.nn).Spec.Template.Labels[labelRevision]).To(Equal(fmt.Sprintf("rev-%d", gen)))
			},
			Entry("a rate limit (429)", http.StatusTooManyRequests, reasonImageRegistryRateLimited, imageRegistryRateLimitedMessage, 2*time.Minute),
			Entry("a failed request", 0, reasonImageRegistryUnavailable, imageRegistryWaitMessage, registryRetryDelay(2)),
		)

		It("keeps a first release on its plain wait", func() {
			f, _, _ := newDelayedFixture("delayed-first", delayed, http.StatusServiceUnavailable)
			pass(f)
			ready := stall(f)
			Expect(ready.Reason).To(Equal(reasonImageRegistryUnavailable))
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, f.nn, &appsv1.Deployment{}))).To(BeTrue(), "nothing is dispatched")
		})
	})
})
