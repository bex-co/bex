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
	"time"

	"github.com/bex-co/bex/lego/operator/internal/httpport"
	"github.com/bex-co/bex/lego/operator/internal/listeners"
	"github.com/bex-co/bex/lego/operator/internal/portscan"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	webHTTPDiscoveryWindow = 90 * time.Second
	// Keep the old release through final bounded probes and the next projection.
	webImageHTTPSelectionHoldSeconds = int32((webHTTPDiscoveryWindow + time.Minute) / time.Second)
	// webHTTPChosenRequeue follows the pass that chose a release's HTTP port
	// with the one that projects and promotes it, once the informer can carry
	// the choice's status write (w6/162).
	webHTTPChosenRequeue = 100 * time.Millisecond
)

// discoversWebImagePort reports a web service whose image chooses its own
// HTTP port: no configured port and no PORT of its own (w8/m141).
func discoversWebImagePort(app *appv1alpha1.App) bool {
	return app.Spec.UsesWebImagePorts() && !appv1alpha1.IsConfiguredPortMode(app.Spec.PortMode) && app.Status.EnvPort == 0
}

func webImageHTTPSelectionPending(app *appv1alpha1.App) bool {
	return discoversWebImagePort(app) && app.Status.ImageNetwork != nil && app.Status.ImageNetwork.HTTPSelection == ""
}

// webImageHTTPSelectionGuardsServing reports a pending HTTP selection while an
// earlier release serves: only then must the candidate's probes wait for HTTP,
// so TCP readiness on a binary listener cannot drain the serving pods. A first
// release has no serving pods, and its probe must not change once discovery
// ends, or the first deploy rolls twice (w2/078).
func webImageHTTPSelectionGuardsServing(app *appv1alpha1.App) bool {
	return webImageHTTPSelectionPending(app) && (app.Status.ActiveRevision != "" || app.Status.ServingNetwork != nil)
}

// webImageReleaseHeld reports a web image release that must not take traffic
// yet: its HTTP port is still being discovered while an earlier release
// serves, or it changes the serving release's image (so its port was
// discovered for it, w8/m141) and has not been promoted. Render's zero-downtime
// deploy keeps the original instance receiving all traffic until the new one
// is healthy, then moves it all at once. Until promotion the Services pin the
// serving revision and the Deployment holds the serving pods, so neither a
// Ready candidate nor a second roll after discovery (the provisional probe on
// / giving way to the final one) puts it in front of users early (w4/m320).
func webImageReleaseHeld(app *appv1alpha1.App) bool {
	serving, candidate := app.Status.ServingNetwork, app.Status.ImageNetwork
	return serving != nil && candidate != nil && discoversWebImagePort(app) &&
		(candidate.HTTPSelection == "" || candidate.Image != serving.Image)
}

// Keep an unconfigured listener decision across releases of the same image.
// Unknown legacy provenance may only retain a choice matching fresh metadata;
// otherwise a removed PORT could be injected into the next candidate again.
func retainWebHTTPSelection(app *appv1alpha1.App, network *appv1alpha1.ImageNetworkStatus, configured bool) {
	if !app.Spec.UsesWebImagePorts() {
		return
	}
	if configured {
		network.HTTPSelection = appv1alpha1.HTTPSelectionConfigured
		return
	}
	if network.HTTPSelection == appv1alpha1.HTTPSelectionConfigured {
		network.HTTPSelection = ""
	}
	if network.HTTPSelection != "" {
		return
	}
	if previous := app.Status.ImageNetwork; previous != network && retainableWebHTTPSelection(previous, network) && previous.HTTPSelection != "" {
		network.PrimaryPort, network.HTTPSelection = previous.PrimaryPort, previous.HTTPSelection
		network.Ports = slices.Clone(previous.Ports)
		return
	}

	if serving := app.Status.ServingNetwork; retainableWebHTTPSelection(serving, network) {
		network.PrimaryPort = serving.PrimaryPort
		network.Ports = slices.Clone(serving.Ports)
		network.HTTPSelection = serving.HTTPSelection
		if network.HTTPSelection == "" {
			network.HTTPSelection = appv1alpha1.HTTPSelectionRetained
		}
	}
}

// Legacy HTTP markers can also carry a stale configured primary, so only a
// known-unconfigured network may replace fresh ports. Exact metadata equality
// preserves ordinary legacy upgrades without importing an ambiguous listener.
// A choice made while the process had another PORT is not kept either: a
// process that listens on $PORT has moved (w4/615).
func retainableWebHTTPSelection(previous, network *appv1alpha1.ImageNetworkStatus) bool {
	if previous == nil || previous.Image != network.Image || previous.HTTPSelection == appv1alpha1.HTTPSelectionConfigured ||
		previous.ProcessPORT() != network.ProcessPORT() {
		return false
	}
	if previous.ConfiguredPort != nil {
		return *previous.ConfiguredPort == 0
	}
	return previous.PrimaryPort == network.PrimaryPort && slices.Equal(previous.Ports, network.Ports)
}

// selectWebImageHTTP runs before route projection or promotion, including when
// a binary protocol already passed the candidate's original TCP health check.
func (r *AppReconciler) selectWebImageHTTP(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment) (bool, string) {
	if !webImageHTTPSelectionPending(app) {
		return false, ""
	}
	pod, started := r.webImageCandidate(ctx, dep)
	if pod == nil || (time.Since(started) < portDetectGrace && !detectsAtReady(dep, pod)) {
		return true, appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortWaiting)
	}
	key := "http/" + app.Namespace + "/" + app.Name
	if last, ok := r.portScanned.Load(key); ok && time.Since(last.(time.Time)) < portDetectInterval {
		return true, appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortDetecting)
	}
	r.portScanned.Store(key, time.Now())
	network := app.Status.ImageNetwork
	ip := pod.Status.PodIP
	found, err := httpport.Probe(ctx, ip, healthProbeHost(app, r.BaseDomain), network.Ports)
	var open listeners.Answer
	var candidates []int32
	if found.Port == 0 && r.readsListeners() {
		// Every listener of the pod, from its node's reader (w6/m530), or
		// swept where the node has none.
		var scanErr error
		open, scanErr = r.podListeners(ctx, pod, r.fullSweep)
		if errors.Is(scanErr, portscan.ErrPartial) {
			logf.FromContext(ctx).Info("HTTP port scan ran out of time; probing the ports found so far", "app", app.Name, "open", open.Ports)
		}
		if scanErr == nil || len(open.Ports) > 0 {
			candidates = httpCandidates(open.Ports)
			found, err = httpport.Probe(ctx, ip, healthProbeHost(app, r.BaseDomain), candidates)
		} else {
			err = scanErr
		}
	}
	// A lower listener that accepted the connection but has not answered yet
	// (a cold first request on a small CPU share) may still be the HTTP port:
	// keep scanning it within the discovery window rather than commit a
	// higher one. After the window the lowest answering port wins.
	inWindow := time.Since(started) < webHTTPDiscoveryWindow
	if port := found.Port; port > 0 && (found.Unanswered == 0 || !inWindow) {
		if !slices.Contains(network.Ports, port) {
			if len(network.Ports) >= appv1alpha1.MaxImagePorts {
				return true, appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortLimit)
			}
			if network.NoDeclaredPorts {
				network.Ports = nil
			}
			network.Ports = append(network.Ports, port)
			slices.Sort(network.Ports)
		}
		r.portScanned.Delete(key)
		network.PrimaryPort, network.HTTPSelection = port, appv1alpha1.HTTPSelectionDetected
		message := appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortDetected, portParam(port))
		logf.FromContext(ctx).Info(message, "app", app.Name)
		return true, message // reproject probes and routing on the chosen port first
	}
	if inWindow {
		// Waiting on a lower listener while a higher one answers HTTP, or on
		// a declared port no listener read can put in context, names it. When
		// the reader's every listener was probed and none answered, Render's
		// "No open HTTP ports detected on 0.0.0.0" says it (w8/388).
		if found.Unanswered > 0 && (found.Port > 0 || len(candidates) == 0) {
			return true, appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortNotAnswering, portParam(found.Unanswered))
		}
		if err != nil {
			// A failed probe and one that found nothing read the same from
			// outside (noOpenPortMessage, w8/259): the product says what the
			// user can act on and the log keeps the probe's own error.
			logf.FromContext(ctx).Info("HTTP port probe failed", "app", app.Name, "error", err.Error())
		}
		switch scanFinding(open, candidates, err == nil) {
		case appv1alpha1.PortScanNoHTTPPort:
			return true, appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortNoneHTTP, portsParam(candidates))
		case appv1alpha1.PortScanLocalhostOnly:
			return true, appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortLocalhostOnly, portsParam(open.Loopback))
		}
		return true, appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortNoneYet)
	}
	r.portScanned.Delete(key)
	network.HTTPSelection = appv1alpha1.HTTPSelectionFallback
	return true, appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHTTPPortFallback, portParam(network.PrimaryPort))
}

// detectsAtReady reports a candidate whose HTTP port is looked for as soon as
// it is Ready, rather than portDetectGrace after it started: one rolled with
// Recreate (a service with a disk, ADR082), whose earlier instance stopped
// before it started, so every second of the grace was downtime (w6/162). Ready
// means its probe found a listener, so a server that listens at once is probed
// at once, and the lowest port answering HTTP then wins, as Render detects the
// port as the instance starts and serves the first one it finds. A rolling
// release keeps the grace: its serving pods carry the traffic meanwhile, and a
// lower HTTP port that binds later still wins (w8/205).
func detectsAtReady(dep *appsv1.Deployment, pod *corev1.Pod) bool {
	return dep.Spec.Strategy.Type == appsv1.RecreateDeploymentStrategyType && podReady(pod)
}

// httpCandidates is a scan's open ports as httpport candidates: the lowest 75,
// without the ports Render reserves.
func httpCandidates(open []int) []int32 {
	ports := make([]int32, 0, len(open))
	for _, p := range open {
		if p > 0 && p <= 65535 && !appv1alpha1.IsReservedImagePort(int32(p)) {
			ports = append(ports, int32(p))
		}
	}
	slices.Sort(ports)
	if len(ports) > appv1alpha1.MaxImagePorts {
		ports = ports[:appv1alpha1.MaxImagePorts]
	}
	return ports
}

// portParam is a port as a deploy reason's {port}.
func portParam(port int32) appv1alpha1.DeployReasonParam {
	return appv1alpha1.DeployReasonParam{Name: "port", Value: strconv.Itoa(int(port))}
}

// webImageCandidate is a running pod of dep's revision with an address and
// when its app container started, or nil.
func (r *AppReconciler) webImageCandidate(ctx context.Context, dep *appsv1.Deployment) (*corev1.Pod, time.Time) {
	if dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return nil, time.Time{}
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace), client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		return nil, time.Time{}
	}
	revision := dep.Spec.Template.Labels[labelRevision]
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning || pod.Status.PodIP == "" || (revision != "" && pod.Labels[labelRevision] != revision) {
			continue
		}
		for _, status := range pod.Status.ContainerStatuses {
			if status.Name == appContainerName && status.State.Running != nil {
				return pod, status.State.Running.StartedAt.Time
			}
		}
	}
	return nil, time.Time{}
}

func (r *AppReconciler) reportWebHTTPSelection(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, image, message string, port int) (ctrl.Result, error) {
	if rolloutDeadline(app, dep) != nil {
		return r.settleFailedRollout(ctx, app, dep, port)
	}
	if reason, pullMessage := r.permanentRolloutPullFailure(ctx, dep, time.Now()); pullMessage != "" {
		return r.settleFailedRolloutMessage(ctx, app, dep, reason, pullMessage)
	}
	if reason, crashMessage := r.permanentRolloutCrash(ctx, dep, crashHintPort(app, port)); crashMessage != "" {
		return r.settleFailedRolloutMessage(ctx, app, dep, reason, crashMessage)
	}
	app.Status.Image = image
	app.Status.Phase = appv1alpha1.PhaseDeploying
	r.observeServingRevision(ctx, app, dep, specReplicas(dep))
	reason := appv1alpha1.ReasonHTTPPortDiscovery
	// A pod the workspace quota refuses never exists, so port discovery would
	// wait on it until the deadline; name the quota instead, as the ordinary
	// rollout path does.
	//
	// Likewise a candidate that crash-loops, cannot resolve its config, or is
	// stuck unscheduled never reaches Running, so "waiting for the container"
	// would stand until the deadline: report the same diagnosis the
	// configured-port path does (w4/437). A failing health check means nothing
	// before a port is chosen, so it keeps the discovery hint.
	if quotaReason, quotaMessage := r.rolloutQuotaBlockMessage(ctx, dep); quotaMessage != "" {
		reason, message = quotaReason, quotaMessage
	} else if stallReason, stallMessage := r.stuckPodMessage(ctx, dep, crashHintPort(app, port)); stallMessage != "" && stallReason != reasonHealthCheckFailing {
		reason, message = stallReason, stallMessage
	}
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason, Message: message, ObservedGeneration: app.Generation})
	if err := updateStatusIfChanged(ctx, r.Client, app); err != nil {
		return ctrl.Result{}, err
	}
	if !webImageHTTPSelectionPending(app) {
		return ctrl.Result{RequeueAfter: webHTTPChosenRequeue}, nil
	}
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *AppReconciler) reportDeploymentStarting(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, image string, port int, deploying bool) (ctrl.Result, bool, error) {
	if deploying {
		if waiting, message := r.selectWebImageHTTP(ctx, app, dep); waiting {
			res, err := r.reportWebHTTPSelection(ctx, app, dep, image, message, port)
			return res, true, err
		}
		if rolloutPending(app, dep, image) && rolloutDeadline(app, dep) == nil {
			r.setPhase(ctx, app, appv1alpha1.PhaseDeploying, "Deploying", "Reconciling Deployment for "+image)
		}
	}
	return ctrl.Result{}, false, nil
}
