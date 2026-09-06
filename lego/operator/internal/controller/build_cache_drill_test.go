//go:build e2e

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
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/identity"
	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestRegistryBuildCacheDrill runs real build Jobs on the selected cluster and
// reads the D5 histograms recorded by buildFromSource. Only App bookkeeping is
// in memory: there is no live App or serving rollout, and the production
// manager's cache gate is untouched. Sources are fixed public dashboard and
// Node API commits (the latter uses the platform native Node runtime). Supply a fresh name minted through backend/internal/id; output repos
// must not already exist. Registry inspection/retirement is recorded separately
// after the run, since the footprint is part of the benchmark evidence.
func TestRegistryBuildCacheDrill(t *testing.T) {
	name := os.Getenv("BEX_CACHE_DRILL_NAME")
	if name == "" {
		t.Skip("opt-in live cluster build benchmark")
	}
	ws, commit := os.Getenv("BEX_CACHE_DRILL_WORKSPACE"), os.Getenv("BEX_CACHE_DRILL_COMMIT")
	if os.Getenv("KUBECONFIG") == "" || !regexp.MustCompile(`^srv-[a-z0-9]{20}$`).MatchString(name) || !regexp.MustCompile(`^tea-[a-z0-9]{20}$`).MatchString(ws) || !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(commit) {
		t.Fatal("explicit KUBECONFIG, fresh canonical service/workspace IDs, and full commit SHA are required")
	}
	const ns, registryHost = "bex-build", "zot.bex-registry.svc:5000"
	cfg, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Timeout = 20 * time.Second
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, batchv1.AddToScheme, appv1alpha1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	live, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	// Only the local observer's registry requests use the port forward. Jobs
	// retain the real cluster service address and actual production networking.
	forward := os.Getenv("BEX_CACHE_DRILL_REGISTRY_FORWARD")
	if forward == "" {
		t.Fatal("registry port-forward address is required")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	dial := &net.Dialer{Timeout: 10 * time.Second}
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == registryHost {
			address = forward
		}
		return dial.DialContext(ctx, network, address)
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = originalTransport; transport.CloseIdleConnections() })
	ctx, cancel := context.WithTimeout(context.Background(), 65*time.Minute)
	defer cancel()
	logDir := os.Getenv("BEX_CACHE_DRILL_LOG_DIR")
	if !filepath.IsAbs(logDir) {
		t.Fatal("explicit absolute benchmark log directory is required")
	}
	if err := os.MkdirAll(logDir, 0700); err != nil {
		t.Fatal(err)
	}
	var credential corev1.Secret
	if err := live.Get(ctx, client.ObjectKey{Namespace: ns, Name: "bex-registry-push"}, &credential); err != nil {
		t.Fatal(err)
	}
	user, password, ok := registry.BasicAuthFromDockerConfig(credential.Data["config.json"], registryHost)
	if !ok {
		t.Fatal("registry push credential is not configured for the benchmark registry")
	}
	http.DefaultTransport = cacheDrillRegistryTransport{RoundTripper: transport, host: registryHost, user: user, password: password}
	appIdentity := identity.ForApp(name, ws)
	for _, repo := range []string{appIdentity.Repo(), appIdentity.CacheRepo()} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+registryHost+"/v2/"+repo+"/tags/list", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.SetBasicAuth(user, password)
		response, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("fresh repository preflight returned HTTP %d; expected 404", response.StatusCode)
		}
	}
	var workspace corev1.Namespace
	if err := live.Get(ctx, client.ObjectKey{Name: ws}, &workspace); err != nil {
		t.Fatal(err)
	}
	// Preflight the exact Jobs before registering cleanup, so a reused name
	// cannot cause this verifier to delete an earlier run's artifacts.
	for _, rev := range []string{appv1alpha1.BuildRevision(1), appv1alpha1.BuildRevision(2)} {
		var job batchv1.Job
		err := live.Get(ctx, client.ObjectKey{Namespace: ns, Name: build.JobName(name, rev)}, &job)
		if client.IgnoreNotFound(err) != nil {
			t.Fatal(err)
		}
		if err == nil {
			t.Fatal("benchmark Job already exists; mint a fresh service id")
		}
	}
	t.Cleanup(func() {
		background := metav1.DeletePropagationBackground
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cleanupCancel()
		for _, rev := range []string{appv1alpha1.BuildRevision(1), appv1alpha1.BuildRevision(2)} {
			var job batchv1.Job
			key := client.ObjectKey{Namespace: ns, Name: build.JobName(name, rev)}
			if err := live.Get(cleanupCtx, key, &job); err != nil {
				if client.IgnoreNotFound(err) != nil {
					t.Error(err)
				}
				continue
			}
			uid := job.UID
			if err := live.Delete(cleanupCtx, &job, &client.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}, PropagationPolicy: &background}); client.IgnoreNotFound(err) != nil {
				t.Error(err)
			}
		}
	})
	t.Logf("repository=%s/%s commit=%s", ws, name, commit)
	for generation := int64(1); generation <= 2; generation++ {
		app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ws, UID: types.UID(name), Generation: generation, Labels: map[string]string{labelWorkspace: ws}},
			Spec:   appv1alpha1.AppSpec{Repo: "https://github.com/bex-co/bex", BuildCommit: commit, RootDir: "dashboard", Builder: "dockerfile", DockerfilePath: "Dockerfile", Port: 3000},
			Status: appv1alpha1.AppStatus{ReleaseGeneration: generation}}
		if os.Getenv("BEX_CACHE_DRILL_WORKLOAD") == "node-api" {
			app.Spec.Repo = "https://github.com/hagopj13/node-express-boilerplate"
			app.Spec.RootDir = ""
			app.Spec.DockerfilePath = ""
			app.Spec.Builder = "native"
			app.Spec.Runtime = "node"
			app.Spec.BuildCommand = "corepack yarn@1.22.22 install --frozen-lockfile"
			app.Spec.StartCommand = "node src/index.js"
		}
		runCacheDrillBuild(t, ctx, live, app)
	}
}

func runCacheDrillBuild(t *testing.T, ctx context.Context, live client.Client, app *appv1alpha1.App) {
	t.Helper()
	const ns, registryHost = "bex-build", "zot.bex-registry.svc:5000"
	local := fake.NewClientBuilder().WithScheme(live.Scheme()).WithStatusSubresource(&appv1alpha1.App{}).WithObjects(app).Build()
	r := &AppReconciler{Client: local, Scheme: live.Scheme(), BuildClient: live, BuildNamespace: ns, Registry: registryHost, RegistryPushSecret: "bex-registry-push", RegistryBuildPullSecret: "bex-registry-pull", BuildCache: true, MaxActiveBuilds: 4, MaxConcurrentBuilds: 2}
	var beforeRun, beforeQueue dto.Metric
	if err := buildRunSeconds.Write(&beforeRun); err != nil {
		t.Fatal(err)
	}
	if err := buildQueueSeconds.Write(&beforeQueue); err != nil {
		t.Fatal(err)
	}
	jobName := build.JobName(app.Name, releaseBuildRevision(app))
	if err := wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		if err := local.Get(ctx, client.ObjectKeyFromObject(app), app); err != nil {
			return false, err
		}
		image, _, halt, err := r.buildFromSource(ctx, app)
		return !halt && image != "", err
	}); err != nil {
		t.Fatalf("%s failed: %v", jobName, err)
	}

	var afterRun, afterQueue dto.Metric
	if err := buildRunSeconds.Write(&afterRun); err != nil {
		t.Fatal(err)
	}
	if err := buildQueueSeconds.Write(&afterQueue); err != nil {
		t.Fatal(err)
	}
	if afterRun.GetHistogram().GetSampleCount()-beforeRun.GetHistogram().GetSampleCount() != 1 {
		t.Fatal("terminal build did not produce exactly one D5 run sample")
	}
	t.Logf("%s bex_build_run_seconds=%g bex_build_queue_seconds=%g", jobName, afterRun.GetHistogram().GetSampleSum()-beforeRun.GetHistogram().GetSampleSum(), afterQueue.GetHistogram().GetSampleSum()-beforeQueue.GetHistogram().GetSampleSum())
	var pods corev1.PodList
	if err := live.List(ctx, &pods, client.InNamespace(ns), client.MatchingLabels{"job-name": jobName}); err != nil {
		t.Fatal(err)
	}
	for _, pod := range pods.Items {
		for _, status := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
			if ended := status.State.Terminated; ended != nil {
				t.Logf("%s phase=%s seconds=%g exit=%d", jobName, status.Name, ended.FinishedAt.Sub(ended.StartedAt.Time).Seconds(), ended.ExitCode)
			}
		}
		logs, err := exec.CommandContext(ctx, "kubectl", "--kubeconfig="+os.Getenv("KUBECONFIG"), "--request-timeout=20s", "-n", ns, "logs", pod.Name, "-c", "buildkit").CombinedOutput()
		if err != nil {
			t.Fatalf("read build log: %v", err)
		}
		if err := os.WriteFile(filepath.Join(os.Getenv("BEX_CACHE_DRILL_LOG_DIR"), jobName+".log"), logs, 0600); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s BuildKit CACHED lines=%d", jobName, strings.Count(string(logs), "CACHED"))
	}
}

// The observer runs outside the cluster without a live App credential. Use
// the same build-plane identity for its read-only digest resolution. No
// request to any other host receives that identity.
type cacheDrillRegistryTransport struct {
	http.RoundTripper
	host, user, password string
}

func (r cacheDrillRegistryTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Host == r.host && request.Header.Get("Authorization") == "" {
		request = request.Clone(request.Context())
		request.SetBasicAuth(r.user, r.password)
	}
	return r.RoundTripper.RoundTrip(request)
}
