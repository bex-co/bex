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
	"fmt"
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
	t.Logf("repository=%s/%s commit=%s", ws, name, commit)
	workload := os.Getenv("BEX_CACHE_DRILL_WORKLOAD")
	generations := int64(2)
	switch workload {
	case "", "node-api":
	case "native-env":
		// Cold MESSAGE=A, env-only MESSAGE=B (must miss cache), unchanged MESSAGE=B (may hit).
		generations = 3
	case "native-clear":
		// A random build artifact must survive a warm build, change on clear,
		// then survive the next warm build. Image digests alone cannot prove this.
		generations = 4
	default:
		t.Fatalf("unknown cache drill workload %q", workload)
	}
	for generation := int64(1); generation <= generations; generation++ {
		var job batchv1.Job
		err := live.Get(ctx, client.ObjectKey{Namespace: ns, Name: build.JobName(name, appv1alpha1.BuildRevision(generation))}, &job)
		if client.IgnoreNotFound(err) != nil {
			t.Fatal(err)
		}
		if err == nil {
			t.Fatal("benchmark Job already exists; mint a fresh service id")
		}
	}
	t.Cleanup(func() { cleanupCacheDrill(t, live, name, generations) })
	previousArtifact := ""
	for generation := int64(1); generation <= generations; generation++ {
		app := cacheDrillApp(name, ws, commit, workload, generation)
		runCacheDrillBuild(t, ctx, live, app)
		if workload == "native-env" || workload == "native-clear" {
			got := readCacheDrillNativeMessage(t, ctx, live, app, registryHost)
			checkCacheDrillArtifact(t, workload, generation, previousArtifact, got)
			previousArtifact = got
		}
	}
}

func cacheDrillApp(name, ws, commit, workload string, generation int64) *appv1alpha1.App {
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ws, UID: types.UID(name), Generation: generation, Labels: map[string]string{labelWorkspace: ws}},
		Spec:   appv1alpha1.AppSpec{Repo: "https://github.com/bex-co/bex", BuildCommit: commit, RootDir: "dashboard", Builder: "dockerfile", DockerfilePath: "Dockerfile", Port: 3000},
		Status: appv1alpha1.AppStatus{ReleaseGeneration: generation}}
	if workload == "" {
		return app
	}
	app.Spec.Repo = "https://github.com/hagopj13/node-express-boilerplate"
	app.Spec.RootDir = ""
	app.Spec.DockerfilePath = ""
	app.Spec.Builder = "native"
	app.Spec.Runtime = "node"
	app.Spec.StartCommand = "node -e \"process.exit(0)\""
	switch workload {
	case "node-api":
		app.Spec.BuildCommand = "corepack yarn@1.22.22 install --frozen-lockfile"
		app.Spec.StartCommand = "node src/index.js"
	case "native-env":
		// Only MESSAGE changes at this fixed source commit.
		app.Spec.BuildCommand = `printf '%s' "$MESSAGE" > message.txt`
		app.Spec.Env = []appv1alpha1.EnvVar{{Name: "MESSAGE", Value: []string{"A", "B", "B"}[generation-1]}}
	case "native-clear":
		// A random artifact proves clear and subsequent reuse.
		app.Spec.BuildCommand = `node -e 'require("fs").writeFileSync("message.txt", require("crypto").randomUUID())'`
		if generation == 3 {
			app.Annotations = map[string]string{appv1alpha1.AnnotationClearCacheReleaseGeneration: "3"}
		}
	}
	return app
}

func cleanupCacheDrill(t *testing.T, live client.Client, name string, generations int64) {
	t.Helper()
	const ns = "bex-build"
	background := metav1.DeletePropagationBackground
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cleanupCancel()
	for gen := int64(1); gen <= generations; gen++ {
		var job batchv1.Job
		key := client.ObjectKey{Namespace: ns, Name: build.JobName(name, appv1alpha1.BuildRevision(gen))}
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
	// Native literal env creates an App-owned projection even though App
	// bookkeeping is in memory. It has no live owner to garbage-collect it.
	var secrets corev1.SecretList
	if err := live.List(cleanupCtx, &secrets, client.InNamespace(ns), client.MatchingLabels{
		"app.bex.co/app": name, "app.bex.co/app-uid": name,
	}); err != nil {
		t.Error(err)
		return
	}
	for i := range secrets.Items {
		secret := &secrets.Items[i]
		uid := secret.UID
		if err := live.Delete(cleanupCtx, secret, &client.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}); client.IgnoreNotFound(err) != nil {
			t.Error(err)
		}
	}
}

func checkCacheDrillArtifact(t *testing.T, workload string, generation int64, previous, got string) {
	t.Helper()
	if workload == "native-env" {
		want := "B"
		if generation == 1 {
			want = "A"
		}
		t.Logf("generation=%d MESSAGE want=%q got=%q", generation, want, got)
		if got != want {
			t.Fatalf("native-env generation %d artifact MESSAGE=%q, want %q", generation, got, want)
		}
		return
	}
	if !regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`).MatchString(got) {
		t.Fatalf("native-clear generation %d did not produce a UUID artifact: %q", generation, got)
	}
	if generation == 3 {
		if got == previous {
			t.Fatal("clear build reused the pre-clear artifact")
		}
	} else if generation > 1 && got != previous {
		t.Fatalf("warm generation %d did not reuse the preceding artifact", generation)
	}
	t.Logf("generation=%d clear=%t artifact_reused=%t artifact=%s", generation, generation == 3, got == previous, got)
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
	err := wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		if err := local.Get(ctx, client.ObjectKeyFromObject(app), app); err != nil {
			return false, err
		}
		image, _, halt, err := r.buildFromSource(ctx, app)
		if err == nil && app.Status.Phase == appv1alpha1.PhaseFailed {
			return false, fmt.Errorf("build entered Failed phase")
		}
		return !halt && image != "", err
	})
	if err != nil {
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

// readCacheDrillNativeMessage pulls the just-built image inside bex-build and
// reads the synthetic message.txt the native-env drill writes. Using an
// in-cluster Pod keeps registry auth on the existing build-plane pull Secret
// and avoids a host-side skopeo/crane dependency.
func readCacheDrillNativeMessage(t *testing.T, ctx context.Context, live client.Client, app *appv1alpha1.App, registryHost string) string {
	t.Helper()
	const ns = "bex-build"
	image := build.Options{
		Name: app.Name, AppUID: string(app.UID), Registry: registryHost,
		Revision: releaseBuildRevision(app), Workspace: app.Labels[labelWorkspace],
	}.ImageRef()
	podName := fmt.Sprintf("cache-msg-%s-%d", app.Name[len(app.Name)-8:], app.Generation)
	var zero, deadline int64 = 0, 120
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: ns},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: "bex-registry-pull"},
			},
			Containers: []corev1.Container{{
				Name:            "read",
				Image:           image,
				ImagePullPolicy: corev1.PullAlways,
				Command:         []string{"/bin/sh", "-ec", "cat /opt/render/project/src/message.txt"},
			}},
			ActiveDeadlineSeconds:         &deadline,
			TerminationGracePeriodSeconds: &zero,
		},
	}
	if err := live.Create(ctx, pod); err != nil {
		t.Fatalf("create message reader pod: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		uid := pod.UID
		if err := live.Delete(cleanupCtx, pod, &client.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}); client.IgnoreNotFound(err) != nil {
			t.Error(err)
		}
	})
	if err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		var current corev1.Pod
		if err := live.Get(ctx, client.ObjectKeyFromObject(pod), &current); err != nil {
			return false, err
		}
		for _, status := range current.Status.ContainerStatuses {
			if status.State.Terminated != nil {
				return true, nil
			}
		}
		return current.Status.Phase == corev1.PodFailed || current.Status.Phase == corev1.PodSucceeded, nil
	}); err != nil {
		t.Fatalf("wait for message reader: %v", err)
	}
	out, err := exec.CommandContext(ctx, "kubectl", "--kubeconfig="+os.Getenv("KUBECONFIG"),
		"--request-timeout=20s", "-n", ns, "logs", podName, "-c", "read").CombinedOutput()
	if err != nil {
		t.Fatalf("read message.txt from %s: %v: %s", image, err, out)
	}
	return string(out)
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
