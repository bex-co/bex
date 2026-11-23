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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/m575: a native image built since native-init names each command it
// starts carries build.NativeInitAnnouncesLabel. The operator reads it with
// the image's commit, records it on the release's rollout so the deploy log
// narrates no second `==> Running '<command>'`, and quiets only that
// native-init in a pre-deploy step, whose line the log narrates.

const announceRegistry = "zot.bex-registry.svc:5000"

// labelledImageRegistry serves one image whose config carries labels, and
// counts the config reads.
func labelledImageRegistry(t *testing.T, labels map[string]string) (*http.Client, *atomic.Int32) {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"architecture": "amd64", "os": "linux", "config": map[string]any{"Labels": labels}})
	if err != nil {
		t.Fatal(err)
	}
	config := string(raw)
	configDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
	manifest := fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"digest":%q}}`, configDigest)
	var reads atomic.Int32
	return &http.Client{Transport: imageNetworkTransport(func(request *http.Request) (*http.Response, error) {
		body := manifest
		if strings.HasSuffix(request.URL.Path, "/blobs/"+configDigest) {
			body = config
			reads.Add(1)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}, &reads
}

// announceApp is a native Node web service with a pre-deploy command.
func announceApp() *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tea-ws", Generation: 1, UID: "web-uid",
			Labels: map[string]string{labelWorkspace: "tea-ws"}},
		Spec: appv1alpha1.AppSpec{
			Repo: "https://github.com/bex-co/hello", Branch: "main", Port: 3000, Runtime: "node", Builder: build.BuilderNative,
			BuildCommand: "npm ci", StartCommand: "node main.js", PreDeployCommand: "node migrate.js",
		},
		Status: appv1alpha1.AppStatus{ReleaseGeneration: 1},
	}
}

// ownImage is app's own build of generation 1 in the platform registry.
func ownImage(app *appv1alpha1.App) string {
	return announceRegistry + "/" + appIdentity(app).Repo() + ":gen-1"
}

func TestImageProvenanceReadsWhetherNativeInitNamesItsCommand(t *testing.T) {
	const commit = "0123456789abcdef0123456789abcdef01234567"
	for _, tc := range []struct {
		name   string
		labels map[string]string
		want   appv1alpha1.ImageRevisionStatus
	}{
		{"an image built since native-init names its command",
			map[string]string{"org.opencontainers.image.revision": commit, build.NativeInitAnnouncesLabel: "true"},
			appv1alpha1.ImageRevisionStatus{Commit: commit, NativeInitAnnounces: true}},
		{"an image built before it", map[string]string{"org.opencontainers.image.revision": commit},
			appv1alpha1.ImageRevisionStatus{Commit: commit}},
		{"a label with another value", map[string]string{build.NativeInitAnnouncesLabel: "1"}, appv1alpha1.ImageRevisionStatus{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := announceApp()
			httpClient, reads := labelledImageRegistry(t, tc.labels)
			r := &AppReconciler{Registry: announceRegistry, HTTPClient: httpClient}
			image := ownImage(app)
			tc.want.Image = image
			if got := r.imageProvenance(context.Background(), app, image); got != tc.want {
				t.Fatalf("provenance = %+v, want %+v", got, tc.want)
			}
			if cached := app.Status.ImageRevision; cached == nil || *cached != tc.want {
				t.Fatalf("cached = %+v, want %+v", cached, tc.want)
			}
			if got := r.imageProvenance(context.Background(), app, image); got != tc.want || reads.Load() != 1 {
				t.Fatalf("second read = %+v after %d registry reads, want the cache", got, reads.Load())
			}
		})
	}

	app := announceApp()
	httpClient, reads := labelledImageRegistry(t, map[string]string{build.NativeInitAnnouncesLabel: "true"})
	r := &AppReconciler{Registry: announceRegistry, HTTPClient: httpClient}
	if got := r.imageProvenance(context.Background(), app, "docker.io/library/node:22"); got.NativeInitAnnounces || reads.Load() != 0 ||
		app.Status.ImageRevision != nil {
		t.Fatalf("a prebuilt image = %+v after %d reads, cached %+v: it is not the App's own build", got, reads.Load(), app.Status.ImageRevision)
	}
	app.Status.ImageRevision = &appv1alpha1.ImageRevisionStatus{Image: ownImage(app), Commit: "abc"}
	if got := r.imageProvenance(context.Background(), app, ownImage(app)); got.NativeInitAnnounces || reads.Load() != 0 {
		t.Fatalf("a read an older operator cached = %+v after %d reads, want it as cached", got, reads.Load())
	}
}

// The pre-deploy step's line stays the narration's (w32/m572): an image whose
// native-init names its command is told not to, and an older image, whose
// native-init would pass the variable on to the command, and a launch that
// is not native-init's, get nothing.
func TestPreDeployQuietsOnlyANativeInitThatNamesItsCommand(t *testing.T) {
	for _, tc := range []struct {
		name   string
		app    func() *appv1alpha1.App
		labels map[string]string
		quiet  bool
	}{
		{"a native image built since native-init names its command", announceApp,
			map[string]string{build.NativeInitAnnouncesLabel: "true"}, true},
		{"a native image built before it", announceApp, nil, false},
		{"a Dockerfile image, which runs no native-init", func() *appv1alpha1.App {
			app := announceApp()
			app.Spec.Runtime, app.Spec.Builder = "docker", "dockerfile"
			return app
		}, map[string]string{build.NativeInitAnnouncesLabel: "true"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			app := tc.app()
			cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).WithObjects(app).
				WithStatusSubresource(&appv1alpha1.App{}).Build()
			httpClient, _ := labelledImageRegistry(t, tc.labels)
			r := &AppReconciler{Client: cl, BuildClient: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes,
				Registry: announceRegistry, HTTPClient: httpClient}
			if _, _, err := r.reconcilePreDeploy(ctx, app, ownImage(app), 3000); err != nil {
				t.Fatalf("reconcilePreDeploy: %v", err)
			}
			var job batchv1.Job
			key := client.ObjectKey{Namespace: app.Namespace, Name: predeploy.JobName(app.Name, appv1alpha1.BuildRevision(1))}
			if err := cl.Get(ctx, key, &job); err != nil {
				t.Fatalf("pre-deploy Job: %v", err)
			}
			env := job.Spec.Template.Spec.Containers[0].Env
			quiet := slices.ContainsFunc(env, func(e corev1.EnvVar) bool { return e.Name == "BEX_NATIVE_INIT_QUIET" && e.Value == "1" })
			named := slices.ContainsFunc(env, func(e corev1.EnvVar) bool { return e.Name == "BEX_NATIVE_INIT_QUIET" })
			if quiet != tc.quiet || named != tc.quiet {
				t.Fatalf("env = %v, want the quiet signal %v", env, tc.quiet)
			}
		})
	}
}

var _ = Describe("The operator records whether a release's instance names its start command (w32/m575)", func() {
	ctx := context.Background()
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	})
	create := func(name string) (types.NamespacedName, func() (ctrl.Result, error)) {
		GinkgoHelper()
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nn.Namespace}, Spec: appv1alpha1.AppSpec{
			Repo: "https://github.com/bex-co/hello", Branch: "main", Port: 3000, Replicas: 1,
			Runtime: "node", Builder: build.BuilderNative, BuildCommand: "npm ci", StartCommand: "node main.js",
		}})).To(Succeed())
		run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
		DeferCleanup(func() {
			deleteOwner(ctx, envtestApp(nn), run)
			background := metav1.DeletePropagationBackground
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, "gen-1"), Namespace: nn.Namespace}},
				&client.DeleteOptions{PropagationPolicy: &background})
		})
		reconcileToFixedPoint(run)
		return nn, run
	}
	// builtImage is the image nn's first build pushes.
	builtImage := func(nn types.NamespacedName) string {
		return "/" + appIdentity(envtestApp(nn)).Repo() + ":gen-1"
	}
	// readImage stands in for the registry read: what the image's labels said.
	readImage := func(nn types.NamespacedName, announces bool) {
		GinkgoHelper()
		app := envtestApp(nn)
		app.Status.ImageRevision = &appv1alpha1.ImageRevisionStatus{Image: builtImage(nn), NativeInitAnnounces: announces}
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())
	}
	rollout := func(nn types.NamespacedName) *appv1alpha1.ReleaseRolloutStatus {
		GinkgoHelper()
		return envtestApp(nn).Status.ReleaseRollout
	}

	It("records it for a release on an image whose native-init names its command, never for an older one, and never moves it", func() {
		announced, announcedRun := create("announce-new")
		older, olderRun := create("announce-older")
		readImage(announced, true)
		completeBuildJob(announced.Name, "gen-1")
		reconcileToFixedPoint(announcedRun)
		completeBuildJob(older.Name, "gen-1")
		reconcileToFixedPoint(olderRun)

		Expect(envtestDeployment(announced).Spec.Template.Spec.Containers[0].Image).To(Equal(builtImage(announced)),
			"the image read is the release's own")
		Expect(rollout(announced)).NotTo(BeNil())
		Expect(rollout(announced).Command).To(Equal("node main.js"))
		Expect(rollout(announced).InstanceAnnounces).To(BeTrue(), "its native-init writes the line")
		Expect(rollout(older)).NotTo(BeNil())
		Expect(rollout(older).Command).To(Equal("node main.js"))
		Expect(rollout(older).InstanceAnnounces).To(BeFalse(), "an image with no label keeps the narrated line")
		Expect(envtestDeployment(announced).Spec.Template.Spec.Containers[0].Env).NotTo(ContainElement(
			HaveField("Name", build.NativeInitQuietEnv)), "a service's own launch is never quieted")

		By("a resync, and a later read that says otherwise, keep what the release's start decided")
		started := rollout(announced).StartedAt
		readImage(announced, false)
		reconcileToFixedPoint(announcedRun)
		Expect(rollout(announced).InstanceAnnounces).To(BeTrue())
		Expect(rollout(announced).StartedAt.Equal(&started)).To(BeTrue())
		readImage(older, true)
		reconcileToFixedPoint(olderRun)
		Expect(rollout(older).InstanceAnnounces).To(BeFalse())
	})
})
