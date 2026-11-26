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
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// movableTagRegistry serves one image config under manifests that differ only
// in an annotation, so each push has its own digest. Tags are re-pointed with
// push and removed with remove; a manifest stays readable by its digest.
type movableTagRegistry struct {
	mu        sync.Mutex
	tags      map[string]string // tag → manifest body
	manifests map[string]string // digest → manifest body
	config    string
	configSum string
	// unreachable fails every manifest read as a request, as a registry that
	// does not answer.
	unreachable bool
	// rateLimited answers every manifest read 429, as Docker Hub answers an
	// anonymous puller past its rate limit.
	rateLimited bool
	// answer, when set, is every manifest read's status (w4/m671).
	answer int
	// manifestReads records each manifest request as "METHOD reference".
	manifestReads []string
}

func newMovableTagRegistry() *movableTagRegistry {
	config := `{"architecture":"amd64","os":"linux","config":{"ExposedPorts":{"80/tcp":{}}}}`
	return &movableTagRegistry{
		tags: map[string]string{}, manifests: map[string]string{},
		config: config, configSum: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(config))),
	}
}

// push points tag at a new manifest for build and returns its digest.
func (m *movableTagRegistry) push(tag, build string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	body := fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"digest":%q},"annotations":{"build":%q}}`, m.configSum, build)
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(body)))
	m.tags[tag], m.manifests[digest] = body, body
	return digest
}

func (m *movableTagRegistry) remove(tag string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tags, tag)
}

// forget deletes a manifest by its digest, as a registry's retention or an
// expired ttl.sh tag does.
func (m *movableTagRegistry) forget(digest string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.manifests, digest)
	for tag, body := range m.tags {
		if fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(body))) == digest {
			delete(m.tags, tag)
		}
	}
}

// setAnswer makes every manifest read answer status; 0 serves them again.
func (m *movableTagRegistry) setAnswer(status int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.answer = status
}

// takeManifestReads returns the manifest requests since the last call.
func (m *movableTagRegistry) takeManifestReads() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	reads := m.manifestReads
	m.manifestReads = nil
	return reads
}

func (m *movableTagRegistry) setUnreachable(unreachable bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.unreachable = unreachable
}

func (m *movableTagRegistry) setRateLimited(rateLimited bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rateLimited = rateLimited
}

func (m *movableTagRegistry) client() *http.Client {
	return &http.Client{Transport: imageNetworkTransport(func(request *http.Request) (*http.Response, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		status, body := http.StatusOK, ""
		_, reference, isManifest := strings.Cut(request.URL.Path, "/manifests/")
		if isManifest {
			m.manifestReads = append(m.manifestReads, request.Method+" "+reference)
		}
		switch {
		case isManifest && m.answer != 0:
			status = m.answer
		case isManifest && m.unreachable:
			return nil, errors.New("connection refused")
		case isManifest && m.rateLimited:
			status, body = http.StatusTooManyRequests, `{"errors":[{"code":"TOOMANYREQUESTS","message":"You have reached your pull rate limit."}]}`
		case strings.HasSuffix(request.URL.Path, "/blobs/"+m.configSum):
			body = m.config
		case m.manifests[reference] != "":
			body = m.manifests[reference]
		case m.tags[reference] != "":
			body = m.tags[reference]
		default:
			status, body = http.StatusNotFound, `{"errors":[{"code":"MANIFEST_UNKNOWN","message":"manifest unknown"}]}`
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
}

// w4/m432: bex-api's rollback of a registry image selects the target deploy's
// own reference (Render, render.com/docs/rollbacks § Registry-hosted Docker
// images: "With a tag, Render pulls the latest image under that tag ... A
// digest always refers to the same image. If the image is no longer available
// or reachable, the rollback fails."). The operator resolves that reference as
// the rollback's own release.
var _ = Describe("Rolling back to a registry image's reference (w4/m432)", func() {
	const (
		repo = "docker.io/traefik/whoami"
		v1   = repo + ":v1"
		v2   = repo + ":v2"
	)
	var (
		nn       types.NamespacedName
		r        *AppReconciler
		registry *movableTagRegistry
	)
	run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
	pass := func() {
		GinkgoHelper()
		for range 4 {
			_, err := run()
			Expect(err).NotTo(HaveOccurred())
		}
	}
	goLive := func() {
		GinkgoHelper()
		if app := envtestApp(nn); app.Status.ImageNetwork != nil && app.Status.ImageNetwork.HTTPSelection == "" {
			// Stand in for the completed HTTP port probe (w8/m142).
			app.Status.ImageNetwork.HTTPSelection = appv1alpha1.HTTPSelectionDetected
			Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
			pass()
		}
		finishImagePortRollout(nn, run)
		pass()
	}
	// deploy stamps what bex-api's verbs do: a release generation, a restart
	// stamp, and the saved image or the rollback's selection.
	deploy := func(image string, ref *appv1alpha1.ReleaseConfigReference) int64 {
		GinkgoHelper()
		app := envtestApp(nn)
		gen := app.Generation + 1
		app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if image != "" {
			app.Spec.Image = image
		}
		app.Spec.ReleaseConfig = nil
		if ref != nil {
			ref.Generation = gen
			app.Spec.ReleaseConfig = ref
		}
		metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(gen, 10))
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		pass()
		return gen
	}
	rollback := func(image string) int64 {
		GinkgoHelper()
		return deploy("", &appv1alpha1.ReleaseConfigReference{SourceGeneration: 1, Image: image})
	}
	serves := func(digest string) {
		GinkgoHelper()
		app := envtestApp(nn)
		Expect(app.Status.Image).To(Equal(repo + "@" + digest))
		Expect(envtestDeployment(nn).Spec.Template.Spec.Containers[0].Image).To(Equal(repo + "@" + digest))
	}

	var digestA string
	BeforeEach(func() {
		registry = newMovableTagRegistry()
		digestA = registry.push("v1", "A")
		registry.push("v2", "C")
		nn = types.NamespacedName{Name: fmt.Sprintf("rollback-ref-%d", GinkgoParallelProcess()), Namespace: "default"}
		r = &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			BaseDomain: "onbex.co", ActivatorService: "bex-activator", ActivatorPort: 8888,
			MaintenanceService: "bex-activator", MaintenancePort: 8888,
			HTTPClient: registry.client(),
		}
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace,
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{
				Image: v1, Type: appv1alpha1.TypeWebService, PortMode: appv1alpha1.PortModeWebImageV1,
				Replicas: 1, Tier: "starter", Expose: true,
			},
		})).To(Succeed())
		DeferCleanup(func() { deleteOwner(ctx, envtestApp(nn), run) })

		By("serving :v1 as digest A, then deploying :v2")
		pass()
		goLive()
		serves(digestA)
		deploy(v2, nil)
		goLive()
		Expect(envtestApp(nn).Status.ActiveRevision).To(Equal("rev-2"))
	})

	It("pulls the image a tag names now, and records its digest", func() {
		digestB := registry.push("v1", "B")
		gen := rollback(v1)
		goLive()
		app := envtestApp(nn)
		Expect(app.Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", gen)))
		serves(digestB)
		Expect(app.Status.ImageDigest).NotTo(BeNil())
		Expect(app.Status.ImageDigest.Digest).To(Equal(digestB))
		Expect(app.Status.ImageDigest.Generation).To(Equal(gen))
		// The saved image is untouched: the next standard deploy runs :v2.
		Expect(app.Spec.Image).To(Equal(v2))
	})

	It("runs exactly a digest, whatever its tag names now", func() {
		registry.push("v1", "B")
		rollback(repo + "@" + digestA)
		goLive()
		serves(digestA)
	})

	It("fails when the tag no longer resolves, and the live release keeps serving", func() {
		registry.remove("v1")
		served := envtestDeployment(nn).Spec.Template.DeepCopy()
		gen := rollback(v1)
		app := envtestApp(nn)
		rollout := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionRollout)
		Expect(rollout).NotTo(BeNil())
		Expect(rollout.Status).To(Equal(metav1.ConditionFalse))
		Expect(rollout.Reason).To(Equal(appv1alpha1.ReasonImagePullFailed))
		Expect(rollout.ObservedGeneration).To(Equal(gen))
		Expect(rollout.Message).To(ContainSubstring(v1))
		Expect(app.Status.ActiveRevision).To(Equal("rev-2"))
		Expect(equality.Semantic.DeepEqual(*served, envtestDeployment(nn).Spec.Template)).To(BeTrue())

		By("rolling back to the digest instead: it goes live")
		next := rollback(repo + "@" + digestA)
		goLive()
		Expect(envtestApp(nn).Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", next)))
		serves(digestA)
	})

	// A registry that does not answer delays the rollback as it delays every
	// image deploy (w32/031): the live release keeps serving, and the rollback
	// goes live with the tag's image once the registry answers.
	It("waits on an unreachable registry with the live release serving", func() {
		clock := &fakeClock{now: time.Now()}
		r.registryReads().now = clock.Now
		registry.setUnreachable(true)
		served := envtestDeployment(nn).Spec.Template.DeepCopy()
		gen := rollback(v1)
		app := envtestApp(nn)
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseDeploying))
		Expect(meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady).Reason).To(Equal(reasonImageRegistryUnavailable))
		Expect(app.Status.ActiveRevision).To(Equal("rev-2"))
		Expect(equality.Semantic.DeepEqual(*served, envtestDeployment(nn).Spec.Template)).To(BeTrue())

		digestB := registry.push("v1", "B")
		registry.setUnreachable(false)
		clock.Advance(imageRegistryRetryMax + time.Second)
		pass()
		goLive()
		Expect(envtestApp(nn).Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", gen)))
		serves(digestB)
	})

	// Past the deploy's rollout budget the wait ends: the rollback fails naming
	// the registry, as Render fails a rollback whose image is "no longer
	// available or reachable", and the live release keeps serving (w4/692).
	It("fails a rollback whose registry stays unreachable past the rollout budget", func() {
		clock := &fakeClock{now: time.Now()}
		r.registryReads().now = clock.Now
		registry.setUnreachable(true)
		served := envtestDeployment(nn).Spec.Template.DeepCopy()
		gen := rollback(v1)
		Expect(envtestApp(nn).Status.Phase).To(Equal(appv1alpha1.PhaseDeploying))

		clock.Advance(time.Duration(rolloutBudgetSeconds)*time.Second + time.Second)
		pass()
		app := envtestApp(nn)
		rollout := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionRollout)
		Expect(rollout).NotTo(BeNil())
		Expect(rollout.Status).To(Equal(metav1.ConditionFalse))
		Expect(rollout.Reason).To(Equal(appv1alpha1.ReasonImagePullFailed))
		Expect(rollout.ObservedGeneration).To(Equal(gen))
		Expect(rollout.Message).To(ContainSubstring("the registry docker.io did not answer the read of the image " + v1))
		Expect(app.Status.Phase).NotTo(Equal(appv1alpha1.PhaseDeploying))
		Expect(app.Status.ActiveRevision).To(Equal("rev-2"))
		Expect(equality.Semantic.DeepEqual(*served, envtestDeployment(nn).Spec.Template)).To(BeTrue())

		By("the verdict is final: a registry that answers later does not roll it")
		registry.setUnreachable(false)
		clock.Advance(imageRegistryRetryMax + time.Second)
		pass()
		Expect(envtestApp(nn).Status.ActiveRevision).To(Equal("rev-2"))
		Expect(equality.Semantic.DeepEqual(*served, envtestDeployment(nn).Spec.Template)).To(BeTrue())
	})

	// A registry that answered only 429 for the whole budget fails the deploy
	// as Render's "Rate limited (429 error)", with its three remedies, not as
	// a registry that did not answer; it still fails under ImagePullFailed, so
	// bex-api records image_pull_failed (w4/m669).
	It("fails a deploy whose registry rate-limits it past the rollout budget as rate limited", func() {
		clock := &fakeClock{now: time.Now()}
		r.registryReads().now = clock.Now
		registry.setRateLimited(true)
		gen := rollback(v1)
		app := envtestApp(nn)
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseDeploying))
		Expect(meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady).Reason).To(Equal(reasonImageRegistryRateLimited))

		clock.Advance(time.Duration(rolloutBudgetSeconds)*time.Second + time.Second)
		pass()
		rollout := meta.FindStatusCondition(envtestApp(nn).Status.Conditions, appv1alpha1.ConditionRollout)
		Expect(rollout).NotTo(BeNil())
		Expect(rollout.Reason).To(Equal(appv1alpha1.ReasonImagePullFailed))
		Expect(rollout.ObservedGeneration).To(Equal(gen))
		Expect(rollout.Message).To(Equal(appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonImageRegistryRateLimited,
			appv1alpha1.DeployReasonParam{Name: "host", Value: "docker.io"}, appv1alpha1.DeployReasonParam{Name: "image", Value: v1})))
		Expect(rollout.Message).To(ContainSubstring("Wait for the rate limit to reset"))
		Expect(envtestApp(nn).Status.ActiveRevision).To(Equal("rev-2"))
	})
})
