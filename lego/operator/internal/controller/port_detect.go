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

	"github.com/bex-co/bex/lego/operator/internal/httpport"
	"github.com/bex-co/bex/lego/operator/internal/listeners"
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
// keeps failing, the operator reads the pod's listening sockets from the
// listener reader on its node (w6/m530, listener_reads.go), or, for a node
// without a reader, connects to the pod IP on every TCP port
// (internal/portscan). A process bound only to loopback is in neither answer:
// it never accepts on the pod's address, which is Render's own "0.0.0.0"
// condition. The reader names such a listener apart, and the stall says so,
// as Render's "Detected open ports on localhost" does (w8/388). Only a
// listener that answers HTTP counts (internal/httpport): Render routes a web
// service to one HTTP port and keeps scanning while none answers ("No open
// HTTP ports detected on 0.0.0.0, continuing to scan"), and the stall names
// that case too (w8/388), so a binary listener a release
// opens first, such as the Erlang epmd (4369) and distribution ports a Phoenix
// release starts long before Phoenix binds PORT on a small CPU share, is never
// the service (w32/037). When exactly one HTTP port other than the configured
// one answers, it is recorded as status.detectedPort and the next pass serves,
// routes and probes on it (App.ServingPort); several are named in the stall
// message instead, and the user picks one with PORT.
//
// A private service is detected the same way (w8/m504), except that its
// listener may speak any protocol: Render serves a private service on any
// port it binds (render.com/docs/private-services), so no HTTP check filters
// it, and when it binds several it is served on the one an image-port private
// service's primary would be (preferredPort). That covers a service switched
// from a native build to a prebuilt image, which keeps its configured-port
// mode (ADR089 § Port slice decisions), and a native one that binds another
// port than PORT.

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

// (edgeCacheContainerName, the caching sidecar a pod template stored before
// w3/m489 may still run, listening inside the pod, is declared in
// edge_cache.go; a pod running it is not scanned.)

// explicitServingPort reports whether the App's own configuration already names
// the serving port, so a detected listener must never override it. A PORT the
// service sets is one, as on Render: its staff answer that "if you want to
// disable the port detector and specify the port to use with the public
// proxy, you can set the PORT env var" (community.render.com/t/15678), and its
// deploy log then scans that port alone (envPortStallHint, w8/416).
func explicitServingPort(app *appv1alpha1.App) bool {
	spec := releaseRuntimeSpec(app)
	return (spec.Port > 0 && spec.Port != appv1alpha1.DefaultPort) || app.Status.EnvPort > 0 || spec.HasImagePorts()
}

// detectedServingPort applies a detected port to the serving port decided from
// the App's own configuration. An explicit port or a PORT variable always wins
// and clears a stale detection.
func detectedServingPort(app *appv1alpha1.App, port int) int {
	if explicitServingPort(app) {
		app.Status.DetectedPort, app.Status.DetectedPortSource = 0, ""
		return port
	}
	if detected := carriedDetectedPort(app); detected > 0 {
		return int(detected)
	}
	return port
}

// servingPortDetected reports whether port, the serving port, was found by
// port detection rather than named by the service (detectedServingPort).
func servingPortDetected(app *appv1alpha1.App, port int) bool {
	detected := carriedDetectedPort(app)
	return !explicitServingPort(app) && detected > 0 && int(detected) == port
}

// releaseSource is where the release this pass runs comes from: a prebuilt
// image or a Git build (appv1alpha1.DetectedPortSource*). It is empty when the
// pass runs another release than its spec names: a Restart, rollback or image
// override runs its selection, and a settling cancel the release that served.
func releaseSource(app *appv1alpha1.App) string {
	switch {
	case app.ActiveReleaseConfig() != nil || canceledOverServed(app):
		return ""
	case app.Spec.Image != "":
		return appv1alpha1.DetectedPortSourceImage
	default:
		return appv1alpha1.DetectedPortSourceBuild
	}
}

// carriedDetectedPort is the detected port this pass's release starts from:
// none when the port was found on a release from the other source. A service
// switched between a prebuilt image and a Git build runs another program, so,
// as on Render, which scans each deploy for the port its process binds, it is
// probed on its own port first rather than on the one the other program bound
// (w32/065). A release from the same source keeps the port (w8/177), and a
// detection from before sources were recorded applies to every release.
func carriedDetectedPort(app *appv1alpha1.App) int32 {
	found, current := app.Status.DetectedPortSource, releaseSource(app)
	if found != "" && current != "" && found != current {
		return 0
	}
	return app.Status.DetectedPort
}

// settleDetectedPort runs as a release goes live: a detection it did not carry,
// found on a release from the other source, is dropped, so every surface names
// the port it serves on and its deploy names no detected port. The drop waits
// for the release to serve: until then the release that bound the detected
// port still serves, and a failed or canceled switch keeps serving on it.
func settleDetectedPort(app *appv1alpha1.App) {
	if app.Status.DetectedPort > 0 && carriedDetectedPort(app) == 0 {
		app.Status.DetectedPort, app.Status.DetectedPortSource = 0, ""
	}
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

// detectBoundPort scans the rollout's unready pod when the App qualifies, a
// web or private service that names no port of its own, sets
// status.detectedPort when the scan finds the one other port it serves, and
// returns text to append to the stall message (empty when there is nothing to
// add). Between scans it returns the last scan's text.
func (r *AppReconciler) detectBoundPort(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, port int) string {
	if !r.readsListeners() || explicitServingPort(app) || !app.Spec.InternallyAddressable() {
		return ""
	}
	key, pod := r.dueUnreadyScan(ctx, app, dep)
	if pod == nil {
		return r.replayPortHint(ctx, key, dep, port)
	}
	return r.keepPortHint(key, pod, port, r.scanForBoundPort(ctx, app, key, pod, port))
}

// scanForBoundPort is detectBoundPort's read of pod.
func (r *AppReconciler) scanForBoundPort(ctx context.Context, app *appv1alpha1.App, key string, pod *corev1.Pod, port int) string {
	open, err := r.boundPortListeners(ctx, pod, port)
	if err != nil {
		// A sweep that found nothing and one that failed read the same from
		// outside: a closed port on a tenant pod drops the connect instead of
		// refusing it, so "no listener" always arrives here as unreachable
		// (w8/259), and a failed read of the node's reader is retried at the
		// next interval. The product says what the user can act on, in
		// Render's terms; the operator's log keeps the error (w4/393).
		logf.FromContext(ctx).Info("port detection scan failed", "app", app.Name, "error", err.Error())
		return noOpenPortMessage(app, port)
	}
	if slices.Contains(open.Ports, port) {
		return "" // the configured port answers: the failing check is about the path, not the listener
	}
	if app.Spec.Type == appv1alpha1.TypePrivateService {
		found := listenerSet(open.Ports)
		if len(found) == 0 {
			return noOpenPortMessage(app, port)
		}
		return r.adoptListener(ctx, app, key, port, int(preferredPort(found)))
	}
	candidates := httpCandidates(open.Ports)
	others, err := r.httpListeners(ctx, app, pod.Status.PodIP, candidates)
	switch len(others) {
	case 0:
		if reserved := r.reservedHTTPListener(ctx, app, pod, open.Ports); reserved > 0 {
			return reservedPortMessage(app, port, reserved)
		}
		switch scanFinding(open, candidates, err == nil) {
		case appv1alpha1.PortScanLocalhostOnly:
			return localhostOnlyMessage(app, port, open.Loopback)
		case appv1alpha1.PortScanNoHTTPPort:
			return noHTTPPortMessage(app, port, candidates)
		}
		return noOpenPortMessage(app, port)
	case 1:
		return r.adoptListener(ctx, app, key, port, others[0])
	default:
		return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintListeningPorts, portParam(int32(port)), portsParam(others))
	}
}

// detectLocalhostAfterDiscovery is the localhost finding of a web image service
// whose HTTP discovery ended with no port (w8/394). Discovery names a listener
// bound only to loopback while its window runs (http_port_localhost_only).
// After it falls back, the release waits on the any-open-port check, and
// detectBoundPort leaves an image's port alone, so the stall named only the
// closed default port, and so did the failure reason at the budget. Only the
// node's reader sees loopback, so a node without one adds nothing. Between
// scans it replays the last scan's verdict, as detectBoundPort does. The
// reader's answer also names an HTTP listener on a reserved port, which
// discovery leaves out (w1/m659).
func (r *AppReconciler) detectLocalhostAfterDiscovery(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, port int) string {
	network := app.Status.ImageNetwork
	if r.Listeners == nil || !discoversWebImagePort(app) || network == nil || network.HTTPSelection != appv1alpha1.HTTPSelectionFallback {
		return ""
	}
	key, pod := r.dueUnreadyScan(ctx, app, dep)
	if pod == nil {
		return r.replayPortHint(ctx, key, dep, port)
	}
	text := ""
	open, err := r.readerListeners(ctx, pod)
	switch {
	case err != nil:
		if !errors.Is(err, errNoListenerReader) {
			logf.FromContext(ctx).Info("listener read failed", "app", app.Name, "error", err.Error())
		}
	case scanFinding(open, nil, false) == appv1alpha1.PortScanLocalhostOnly:
		text = localhostOnlyMessage(app, port, open.Loopback)
	default:
		if reserved := r.reservedHTTPListener(ctx, app, pod, open.Ports); reserved > 0 {
			text = reservedPortMessage(app, port, reserved)
		}
	}
	return r.keepPortHint(key, pod, port, text)
}

// reservedHTTPListener is the lowest of open's reserved ports (ReservedImagePorts)
// that answers HTTP, 0 for none (w1/m659). Nothing routes to one, so a web
// service serving only there never goes live, and Render's deploy page warns
// about it. A port a platform container of pod declares is bex's own: a pod
// template from before w3/m489 may still run the edge-cache sidecar on 19099.
func (r *AppReconciler) reservedHTTPListener(ctx context.Context, app *appv1alpha1.App, pod *corev1.Pod, open []int) int {
	var reserved []int32
	for _, p := range open {
		if appv1alpha1.IsReservedImagePort(int32(p)) && !platformPort(pod, int32(p)) {
			reserved = append(reserved, int32(p))
		}
	}
	if len(reserved) == 0 {
		return 0
	}
	slices.Sort(reserved)
	answering, err := r.httpListeners(ctx, app, pod.Status.PodIP, slices.Compact(reserved))
	if err != nil || len(answering) == 0 {
		return 0
	}
	return answering[0]
}

// platformPort reports a port a container of pod other than the app's own
// declares, or the edge-cache sidecar's.
func platformPort(pod *corev1.Pod, port int32) bool {
	for _, c := range pod.Spec.Containers {
		if c.Name == appContainerName {
			continue
		}
		if c.Name == edgeCacheContainerName && port == edgeCachePort {
			return true
		}
		if slices.ContainsFunc(c.Ports, func(p corev1.ContainerPort) bool { return p.ContainerPort == port }) {
			return true
		}
	}
	return false
}

// reservedPortMessage is the stall text for a web service whose only HTTP
// listener is reserved, a reserved port (w1/m659). port is the port probed;
// the text names the one the process receives in PORT (processPort).
func reservedPortMessage(app *appv1alpha1.App, port, reserved int) string {
	return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintReservedPort,
		portParam(int32(processPort(app, port))), appv1alpha1.DeployReasonParam{Name: "reserved", Value: strconv.Itoa(reserved)})
}

// scanFinding is what a web service's scan found when no listener answers
// HTTP, in the cases Render names (w8/388): open ports, candidates, that were
// probed and answer no HTTP; nothing reachable but listeners on loopback;
// else no open port. candidates come from open.Ports, so the first two never
// both hold.
func scanFinding(open listeners.Answer, candidates []int32, probed bool) appv1alpha1.PortScan {
	switch {
	case probed && len(candidates) > 0:
		return appv1alpha1.PortScanNoHTTPPort
	case len(open.Ports) == 0 && len(open.Loopback) > 0:
		return appv1alpha1.PortScanLocalhostOnly
	}
	return appv1alpha1.PortScanNoOpenPort
}

// adoptListener makes candidate, the listener a scan chose, the App's detected
// port once the scan before it chose the same one.
// A detection from the other source counts as none: this release's own process
// may bind the same port, and it is recorded for this release's source.
func (r *AppReconciler) adoptListener(ctx context.Context, app *appv1alpha1.App, key string, port, candidate int) string {
	if carriedDetectedPort(app) == int32(candidate) {
		return ""
	}
	if !r.candidateConfirmed(key, candidate) {
		return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintListening, portParam(int32(candidate)))
	}
	logf.FromContext(ctx).Info("detected the port the process listens on", "app", app.Name, "port", candidate, "was", port)
	app.Status.DetectedPort, app.Status.DetectedPortSource = int32(candidate), releaseSource(app)
	return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintListeningAdopted, portParam(int32(candidate)))
}

// candidateConfirmed records candidate as the listener key's scan chose and
// reports whether the scan before it chose the same one, which then clears the
// record. A port opened early while the server is still booting, such as a
// metrics port, is never taken from a single scan.
func (r *AppReconciler) candidateConfirmed(key string, candidate int) bool {
	if prev, ok := r.portCandidate.Load(key); ok && prev.(int) == candidate {
		r.portCandidate.Delete(key)
		return true
	}
	r.portCandidate.Store(key, candidate)
	return false
}

// httpListeners is the ports among candidates that answer HTTP, lowest first.
// A failed probe finds none, so the scan waits as it does for a closed port,
// and its error says the ports' HTTP answer is unknown.
func (r *AppReconciler) httpListeners(ctx context.Context, app *appv1alpha1.App, ip string, candidates []int32) ([]int, error) {
	answering, err := httpport.Answering(ctx, ip, healthProbeHost(app, r.BaseDomain), candidates)
	if err != nil {
		logf.FromContext(ctx).Info("port detection HTTP probe failed", "app", app.Name, "error", err.Error())
		return nil, err
	}
	ports := make([]int, len(answering))
	for i, p := range answering {
		ports[i] = int(p)
	}
	return ports, nil
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

// localhostOnlyMessage is the stall text for a web service whose process
// listens on nothing the platform can reach but on loopback, ports loopback:
// Render's "No open ports detected on 0.0.0.0" and "Detected open ports on
// localhost -- did you mean to bind one of these to 0.0.0.0?" (w8/388). The
// process does serve, so a background worker is no alternative.
func localhostOnlyMessage(app *appv1alpha1.App, port int, loopback []int) string {
	return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintNoOpenPortLocalhost,
		portParam(int32(processPort(app, port))), portsParam(loopback))
}

// noHTTPPortMessage is the stall text for a web service whose open ports,
// candidates, answer no HTTP: Render's "No open HTTP ports detected on
// 0.0.0.0" and "If you don't need to receive public HTTP traffic, create a
// private service instead" (w8/388).
func noHTTPPortMessage(app *appv1alpha1.App, port int, candidates []int32) string {
	return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintNoOpenHTTPPort,
		portParam(int32(processPort(app, port))), portsParam(candidates))
}

// portsParam is ports, ascending, as a deploy reason's {ports}: the first
// MaxImagePorts, comma-separated. A tenant's process may listen on thousands
// of loopback ports, and the stall text is an App condition's bounded message.
func portsParam[P int | int32](ports []P) appv1alpha1.DeployReasonParam {
	list := make([]string, 0, min(len(ports), appv1alpha1.MaxImagePorts))
	for _, p := range ports[:cap(list)] {
		list = append(list, strconv.Itoa(int(p)))
	}
	return appv1alpha1.DeployReasonParam{Name: "ports", Value: strings.Join(list, ", ")}
}

// commonHTTPPorts are probed first (w4/393): with dropped packets a full
// 65k sweep cannot finish inside portDetectTimeout, but a verdict on the
// ports HTTP servers actually bind lands in well under a second.
var commonHTTPPorts = portscan.LikelyHTTPPorts

// portsScanner is the optional bounded scan a Scanner may offer.
type portsScanner interface {
	ScanPorts(ctx context.Context, ip string, ports []int) ([]int, error)
}

// boundPortListeners is the listeners bound-port detection decides on for
// pod: of the pod's listeners, those among the common HTTP ports and also when
// any of them listens, else all. That is what scanBoundPort's two phases
// answer; a reader's complete answer is narrowed the same way, so detection
// chooses as it did when it swept. Only a node without a reader is swept.
func (r *AppReconciler) boundPortListeners(ctx context.Context, pod *corev1.Pod, also ...int) (listeners.Answer, error) {
	swept := false
	open, err := r.podListeners(ctx, pod, func(ctx context.Context, ip string) ([]int, error) {
		swept = true
		return scanBoundPort(ctx, r.PortScanner, ip, also...)
	})
	if err != nil || swept {
		return open, err
	}
	likely := likelyPorts(also)
	if common := slices.DeleteFunc(slices.Clone(open.Ports), func(p int) bool { return !slices.Contains(likely, p) }); len(common) > 0 {
		open.Ports = common
	}
	return open, nil
}

// likelyPorts is commonHTTPPorts and also.
func likelyPorts(also []int) []int {
	likely := commonHTTPPorts
	for _, p := range also {
		if !slices.Contains(likely, p) {
			likely = append(slices.Clone(likely), p)
		}
	}
	return likely
}

// scanBoundPort runs the bounded common-port phase first, then the full
// sweep when nothing answered there. also joins the bounded phase: a port the
// caller must know the state of, such as the probed primary.
func scanBoundPort(ctx context.Context, scanner portscan.Scanner, ip string, also ...int) ([]int, error) {
	if bounded, ok := scanner.(portsScanner); ok {
		open, err := bounded.ScanPorts(ctx, ip, likelyPorts(also))
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
		if edgeCacheSidecar(p.Spec) != nil {
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
	for _, m := range []*sync.Map{&r.portScanned, &r.portCandidate, &r.portHint, &r.privatePortScanned, &r.privatePortStable, &r.healthCheckRead} {
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
