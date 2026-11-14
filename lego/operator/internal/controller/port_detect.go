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
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/portscan"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Bound-port detection (w8/177). Render's web-service docs say that when a
// service binds a port other than PORT, "Render is usually able to detect and
// use it": it scans the running container for a listener on 0.0.0.0. bex
// probes and routes only the configured port (10000 by default), so a process
// bound elsewhere — an official Render example runs uvicorn on 8000, a stock
// nginx image on 80 — never goes live.
//
// While a web service with no explicit port is waiting on a health check that
// keeps failing, the operator connects to the pod IP on every TCP port
// (internal/portscan). A process bound only to loopback never answers on the
// pod's address, which is Render's own "0.0.0.0" condition. When exactly one
// port other than the configured one answers, it is recorded as
// status.detectedPort and the next pass serves, routes and probes on it
// (App.ServingPort); several ports are named in the stall message instead, and
// the user picks one with PORT.

const (
	// portDetectGrace is how long the container must have been running before
	// its listeners are read: long enough for an ordinary boot.
	portDetectGrace = 20 * time.Second
	// portDetectInterval spaces the scans of one App; the rollout pass that
	// asks runs every few seconds.
	portDetectInterval = 15 * time.Second
	// portDetectTimeout bounds one scan.
	portDetectTimeout = 15 * time.Second
)

// (edgeCacheContainerName, the caching sidecar that also listens inside the
// pod, is declared in edge_cache.go; a cached service is not scanned.)

// explicitServingPort reports whether the App's own configuration already names
// the serving port, so a detected listener must never override it.
func explicitServingPort(app *appv1alpha1.App) bool {
	return (app.Spec.Port > 0 && app.Spec.Port != appv1alpha1.DefaultPort) || app.Status.EnvPort > 0 || app.Spec.HasImagePorts()
}

// detectedServingPort applies a detected port to the serving port decided from
// the App's own configuration. An explicit port or a PORT variable always wins
// and clears a stale detection.
func detectedServingPort(app *appv1alpha1.App, port int) int {
	if explicitServingPort(app) {
		app.Status.DetectedPort = 0
		return port
	}
	if app.Status.DetectedPort > 0 {
		return int(app.Status.DetectedPort)
	}
	return port
}

// servingPortDetected reports whether port, the serving port, was found by
// port detection rather than named by the service (detectedServingPort).
func servingPortDetected(app *appv1alpha1.App, port int) bool {
	return !explicitServingPort(app) && app.Status.DetectedPort > 0 && int(app.Status.DetectedPort) == port
}

// processPort is the PORT the process is given. It is the serving port, the
// one the platform routes to and probes, except when that port was found
// rather than named: Render leaves PORT at its default and routes to the port
// it found, and so does bex. That covers a detected listener (w8/267) and a
// web image service whose image chooses its own HTTP port, whether by EXPOSE
// or by HTTP discovery (w4/615; ImageNetworkStatus.ProcessPort). Handing the
// found port back as PORT restarted the process with a different environment
// than the one it was found in, and an app with a public listener on one port
// and an internal one on $PORT then bound the same port twice and never went
// live.
func processPort(app *appv1alpha1.App, port int) int {
	if servingPortDetected(app, port) {
		return appv1alpha1.DefaultPort
	}
	if network := app.Status.ImageNetwork; network != nil && network.ProcessPort > 0 && discoversWebImagePort(app) {
		return int(network.ProcessPort)
	}
	return port
}

// crashHintPort is the port a crash diagnosis may name as $PORT: none when the
// process is given another PORT than the serving port, because it is then not
// expected to listen on $PORT at all.
func crashHintPort(app *appv1alpha1.App, port int) int {
	if processPort(app, port) != port {
		return 0
	}
	return port
}

// dueUnreadyScan returns the App's scan key and its rollout's Running but
// unready pod when a scan is due (at most one per portDetectInterval per App,
// shared by both detectors), recording the scan; pod is nil otherwise.
func (r *AppReconciler) dueUnreadyScan(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment) (key string, pod *corev1.Pod) {
	key = app.Namespace + "/" + app.Name
	every := portDetectInterval
	if r.portDetectEvery > 0 {
		every = r.portDetectEvery
	}
	if last, ok := r.portScanned.Load(key); ok && time.Since(last.(time.Time)) < every {
		return key, nil
	}
	if pod = r.unreadyPod(ctx, dep); pod != nil {
		r.portScanned.Store(key, time.Now())
	}
	return key, pod
}

// detectBoundPort scans the rollout's unready pod when the App qualifies, sets
// status.detectedPort when exactly one other port answers, and returns text to
// append to the stall message (empty when there is nothing to add). Between
// scans it returns the last scan's text.
func (r *AppReconciler) detectBoundPort(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, port int) string {
	if r.PortScanner == nil || explicitServingPort(app) ||
		(app.Spec.Type != "" && app.Spec.Type != appv1alpha1.TypeWebService) {
		return ""
	}
	key, pod := r.dueUnreadyScan(ctx, app, dep)
	if pod == nil {
		return r.replayPortHint(ctx, key, dep, port)
	}
	return r.keepPortHint(key, pod, port, r.scanForBoundPort(ctx, app, key, pod.Status.PodIP, port))
}

// scanForBoundPort is detectBoundPort's scan of ip.
func (r *AppReconciler) scanForBoundPort(ctx context.Context, app *appv1alpha1.App, key, ip string, port int) string {
	scanCtx, cancel := context.WithTimeout(ctx, portDetectTimeout)
	defer cancel()
	open, err := scanBoundPort(scanCtx, r.PortScanner, ip)
	if err != nil {
		// A scan that found nothing and one that failed read the same from
		// outside: a closed port on a tenant pod drops the connect instead of
		// refusing it, so "no listener" always arrives here as unreachable
		// (w8/259). The product says what the user can act on, in Render's
		// terms; the operator's log keeps the scan's own error (w4/393).
		logf.FromContext(ctx).Info("port detection scan failed", "app", app.Name, "error", err.Error())
		return noOpenPortMessage(app, port)
	}
	var others []int
	for _, p := range open {
		if p == port {
			return "" // the configured port answers: the failing check is about the path, not the listener
		}
		others = append(others, p)
	}
	switch len(others) {
	case 0:
		return noOpenPortMessage(app, port)
	case 1:
		if app.Status.DetectedPort == int32(others[0]) {
			return ""
		}
		// The same lone port in two scans: a metrics port opened early while
		// the HTTP server is still booting is not mistaken for the service.
		if prev, ok := r.portCandidate.Load(key); !ok || prev.(int) != others[0] {
			r.portCandidate.Store(key, others[0])
			return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintListening, portParam(int32(others[0])))
		}
		logf.FromContext(ctx).Info("detected the port the process listens on", "app", app.Name, "port", others[0], "was", port)
		app.Status.DetectedPort = int32(others[0])
		return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintListeningAdopted, portParam(int32(others[0])))
	default:
		list := make([]string, len(others))
		for i, p := range others {
			list[i] = strconv.Itoa(p)
		}
		return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintListeningPorts, portParam(int32(port)),
			appv1alpha1.DeployReasonParam{Name: "ports", Value: strings.Join(list, ", ")})
	}
}

// noOpenPortMessage is the stall text for an instance with nothing listening
// where the platform can reach it: no server, or one bound to 127.0.0.1.
// Render calls it "Port scan timeout reached, no open ports detected. Bind
// your service to at least one port. If you don't need to receive traffic on
// any port, create a background worker instead", and its troubleshooting page
// names the two fixes: bind host 0.0.0.0, and the port in PORT (w8/259).
//
// port is the port probed. The number in PORT is what the process receives
// (processPort): on a service whose port was detected, PORT is still the
// default, and the detected port is named as where the server listened
// before (w8/281).
func noOpenPortMessage(app *appv1alpha1.App, port int) string {
	env := processPort(app, port)
	code := appv1alpha1.DeployReasonHintNoOpenPort
	if env != port {
		code = appv1alpha1.DeployReasonHintNoOpenPortBefore
	}
	return appv1alpha1.FormatDeployReason(code, portParam(int32(env)),
		appv1alpha1.DeployReasonParam{Name: "previous", Value: strconv.Itoa(port)}) + backgroundWorkerHint
}

// backgroundWorkerHint is Render's alternative for a service that listens on no
// port: it does not belong in a web or private service (w21/059).
var backgroundWorkerHint = appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintBackgroundWorker)

// commonHTTPPorts are probed first (w4/393): with dropped packets a full
// 65k sweep cannot finish inside portDetectTimeout, but a verdict on the
// ports HTTP servers actually bind lands in well under a second.
var commonHTTPPorts = portscan.LikelyHTTPPorts

// portsScanner is the optional bounded scan a Scanner may offer.
type portsScanner interface {
	ScanPorts(ctx context.Context, ip string, ports []int) ([]int, error)
}

// scanBoundPort runs the bounded common-port phase first, then the full
// sweep when nothing answered there. also joins the bounded phase: a port the
// caller must know the state of, such as the probed primary.
func scanBoundPort(ctx context.Context, scanner portscan.Scanner, ip string, also ...int) ([]int, error) {
	if bounded, ok := scanner.(portsScanner); ok {
		likely := commonHTTPPorts
		for _, p := range also {
			if !slices.Contains(likely, p) {
				likely = append(slices.Clone(likely), p)
			}
		}
		open, err := bounded.ScanPorts(ctx, ip, likely)
		if err != nil {
			return nil, err
		}
		if len(open) > 0 {
			return open, nil
		}
	}
	open, err := scanner.Scan(ctx, ip)
	if errors.Is(err, portscan.ErrPartial) {
		// Likely ports went first, so what the sweep found before its
		// deadline is the answer to use, not a failure (w9/072).
		logf.FromContext(ctx).Info("port detection scan ran out of time; using the ports found so far", "open", open)
		return open, nil
	}
	return open, err
}

// unreadyPod is the rollout's current-revision pod whose app container has
// been running past the grace period without becoming ready, or nil.
func (r *AppReconciler) unreadyPod(ctx context.Context, dep *appsv1.Deployment) *corev1.Pod {
	if dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return nil
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace), client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		return nil
	}
	for i := range pods.Items {
		p := &pods.Items[i]
		if p.Status.Phase != corev1.PodRunning || p.Status.PodIP == "" || !rolloutPodUnready(p, dep) {
			continue
		}
		cached := false
		for _, c := range p.Spec.Containers {
			cached = cached || c.Name == edgeCacheContainerName
		}
		if cached {
			continue
		}
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Name == appContainerName && cs.State.Running != nil &&
				time.Since(cs.State.Running.StartedAt.Time) > portDetectGrace {
				return p
			}
		}
	}
	return nil
}

// rolloutPodUnready reports a live pod of dep's current revision whose app
// container is not ready.
func rolloutPodUnready(p *corev1.Pod, dep *appsv1.Deployment) bool {
	if revision := dep.Spec.Template.Labels[labelRevision]; p.DeletionTimestamp != nil || (revision != "" && p.Labels[labelRevision] != revision) {
		return false
	}
	for _, cs := range p.Status.ContainerStatuses {
		if cs.Name == appContainerName {
			return !cs.Ready
		}
	}
	return false
}

// scannedHint is what the last port scan of an App's rollout pod, checked
// against port, added to its stall message.
type scannedHint struct {
	pod  string
	uid  types.UID
	port int
	text string
}

// keepPortHint records a scan's verdict on pod as the App's hint and returns
// it; an empty verdict (the configured port answers, nothing to add) clears it.
func (r *AppReconciler) keepPortHint(key string, pod *corev1.Pod, port int, text string) string {
	if text == "" {
		r.portHint.Delete(key)
	} else {
		r.portHint.Store(key, scannedHint{pod: pod.Name, uid: pod.UID, port: port, text: text})
	}
	return text
}

// forgetPortScans drops a deleted App's scan state.
func (r *AppReconciler) forgetPortScans(key string) {
	for _, m := range []*sync.Map{&r.portScanned, &r.portCandidate, &r.portHint} {
		m.Delete(key)
	}
	r.portScanned.Delete("http/" + key)
}

// replayPortHint is the last scan's verdict on a pass that does not scan, while
// the pod it scanned is still the rollout's and unready. A scan runs once per
// portDetectInterval and an in-flight rollout requeues every 5 s, so without it
// the stall's text dropped its hint on most passes (w5/m356).
func (r *AppReconciler) replayPortHint(ctx context.Context, key string, dep *appsv1.Deployment, port int) string {
	v, ok := r.portHint.Load(key)
	if !ok {
		return ""
	}
	hint := v.(scannedHint)
	var pod corev1.Pod
	if hint.port != port {
		r.portHint.Delete(key)
		return ""
	}
	if err := r.Get(ctx, client.ObjectKey{Namespace: dep.Namespace, Name: hint.pod}, &pod); err != nil ||
		pod.UID != hint.uid || !rolloutPodUnready(&pod, dep) {
		r.portHint.Delete(key)
		return ""
	}
	return hint.text
}
