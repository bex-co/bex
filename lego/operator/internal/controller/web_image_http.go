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
	"slices"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/httpport"
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
)

func webImageHTTPSelectionPending(app *appv1alpha1.App) bool {
	return app.Spec.UsesWebImagePorts() && !configuredPortMode(app.Spec.PortMode) && app.Status.EnvPort == 0 &&
		app.Status.ImageNetwork != nil && app.Status.ImageNetwork.HTTPSelection == ""
}

// Legacy serving networks keep their choice on an operator upgrade; a genuinely
// new digest is discovered once. Removing an explicit PORT starts discovery.
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
	if previous := app.Status.ImageNetwork; previous != nil && previous != network && previous.Image == network.Image &&
		previous.HTTPSelection != "" && previous.HTTPSelection != appv1alpha1.HTTPSelectionConfigured {
		network.PrimaryPort, network.HTTPSelection = previous.PrimaryPort, previous.HTTPSelection
		network.Ports = slices.Clone(previous.Ports)
		return
	}

	if serving := app.Status.ServingNetwork; serving != nil && serving.Image == network.Image && serving.HTTPSelection != appv1alpha1.HTTPSelectionConfigured {
		network.PrimaryPort = serving.PrimaryPort
		network.Ports = slices.Clone(serving.Ports)
		network.HTTPSelection = serving.HTTPSelection
		if network.HTTPSelection == "" {
			network.HTTPSelection = appv1alpha1.HTTPSelectionRetained
		}
	}
}

// selectWebImageHTTP runs before route projection or promotion, including when
// a binary protocol already passed the candidate's original TCP health check.
func (r *AppReconciler) selectWebImageHTTP(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment) (bool, string) {
	if !webImageHTTPSelectionPending(app) {
		return false, ""
	}
	ip, started := r.webImageCandidateIP(ctx, dep)
	if ip == "" || time.Since(started) < portDetectGrace {
		return true, "Waiting for the container to start before detecting its HTTP port"
	}
	key := "http/" + app.Namespace + "/" + app.Name
	if last, ok := r.portScanned.Load(key); ok && time.Since(last.(time.Time)) < portDetectInterval {
		return true, "Detecting the container's HTTP port"
	}
	r.portScanned.Store(key, time.Now())
	network := app.Status.ImageNetwork
	port, err := httpport.Probe(ctx, ip, healthProbeHost(app, r.BaseDomain), network.Ports)
	if port == 0 && r.PortScanner != nil {
		scanCtx, cancel := context.WithTimeout(ctx, portDetectTimeout)
		open, scanErr := r.PortScanner.Scan(scanCtx, ip)
		cancel()
		if scanErr == nil || len(open) > 0 {
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
			port, err = httpport.Probe(ctx, ip, healthProbeHost(app, r.BaseDomain), ports)
		} else {
			err = scanErr
		}
	}
	if port > 0 {
		if !slices.Contains(network.Ports, port) {
			if len(network.Ports) >= appv1alpha1.MaxImagePorts {
				return true, "The detected HTTP port exceeds the service's 75-port limit; set PORT explicitly"
			}
			if network.NoDeclaredPorts {
				network.Ports = nil
			}
			network.Ports = append(network.Ports, port)
			slices.Sort(network.Ports)
		}
		r.portScanned.Delete(key)
		network.PrimaryPort, network.HTTPSelection = port, appv1alpha1.HTTPSelectionDetected
		message := fmt.Sprintf("Detected HTTP on port %d; serving it", port)
		logf.FromContext(ctx).Info(message, "app", app.Name)
		return true, message // reproject probes and routing on the chosen port first
	}
	if time.Since(started) < webHTTPDiscoveryWindow {
		if err != nil {
			return true, "Port detection could not reach the instance; retrying the HTTP scan"
		}
		return true, "No port answers HTTP yet; continuing to scan while the container starts"
	}
	r.portScanned.Delete(key)
	network.HTTPSelection = appv1alpha1.HTTPSelectionFallback
	return true, fmt.Sprintf("No port answered HTTP; using fallback port %d. Set PORT to the HTTP listener if it binds elsewhere", network.PrimaryPort)
}

func (r *AppReconciler) webImageCandidateIP(ctx context.Context, dep *appsv1.Deployment) (string, time.Time) {
	if dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return "", time.Time{}
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace), client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		return "", time.Time{}
	}
	revision := dep.Spec.Template.Labels[labelRevision]
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning || pod.Status.PodIP == "" || (revision != "" && pod.Labels[labelRevision] != revision) {
			continue
		}
		for _, status := range pod.Status.ContainerStatuses {
			if status.Name == appContainerName && status.State.Running != nil {
				return pod.Status.PodIP, status.State.Running.StartedAt.Time
			}
		}
	}
	return "", time.Time{}
}

func (r *AppReconciler) reportWebHTTPSelection(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, image, message string, port int) (ctrl.Result, error) {
	if rolloutDeadline(app, dep) != nil {
		return r.settleFailedRollout(ctx, app, dep, port)
	}
	if reason, pullMessage := r.permanentRolloutPullFailure(ctx, dep, time.Now()); pullMessage != "" {
		return r.settleFailedRolloutMessage(ctx, app, dep, reason, pullMessage)
	}
	app.Status.Image = image
	app.Status.Phase = appv1alpha1.PhaseDeploying
	replicas := int32(1)
	if dep.Spec.Replicas != nil {
		replicas = *dep.Spec.Replicas
	}
	r.observeServingRevision(ctx, app, dep, replicas)
	meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: appv1alpha1.ReasonHTTPPortDiscovery, Message: message, ObservedGeneration: app.Generation})
	if err := updateStatusIfChanged(ctx, r.Client, app); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

func (r *AppReconciler) reportDeploymentStarting(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, image string, port int, deploying bool) (ctrl.Result, bool, error) {
	if deploying {
		if waiting, message := r.selectWebImageHTTP(ctx, app, dep); waiting {
			res, err := r.reportWebHTTPSelection(ctx, app, dep, image, message, port)
			return res, true, err
		}
		if rolloutPending(app, image) && rolloutDeadline(app, dep) == nil {
			r.setPhase(ctx, app, appv1alpha1.PhaseDeploying, "Deploying", "Reconciling Deployment for "+image)
		}
	}
	return ctrl.Result{}, false, nil
}
