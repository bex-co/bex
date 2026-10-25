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
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

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

// detectedServingPort applies a detected port to the serving port decided from
// the App's own configuration. An explicit port or a PORT variable always wins
// and clears a stale detection.
func detectedServingPort(app *appv1alpha1.App, port int) int {
	if app.Spec.Port > 0 || app.Status.EnvPort > 0 || app.Spec.HasImagePorts() {
		app.Status.DetectedPort = 0
		return port
	}
	if app.Status.DetectedPort > 0 {
		return int(app.Status.DetectedPort)
	}
	return port
}

// detectBoundPort scans the rollout's unready pod when the App qualifies, sets
// status.detectedPort when exactly one other port answers, and returns text to
// append to the stall message (empty when there is nothing to add).
func (r *AppReconciler) detectBoundPort(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, port int) string {
	if r.PortScanner == nil || app.Spec.Port > 0 || app.Status.EnvPort > 0 || app.Spec.HasImagePorts() ||
		(app.Spec.Type != "" && app.Spec.Type != appv1alpha1.TypeWebService) {
		return ""
	}
	key := app.Namespace + "/" + app.Name
	every := portDetectInterval
	if r.portDetectEvery > 0 {
		every = r.portDetectEvery
	}
	if last, ok := r.portScanned.Load(key); ok && time.Since(last.(time.Time)) < every {
		return ""
	}
	ip := r.unreadyPodIP(ctx, dep)
	if ip == "" {
		return ""
	}
	r.portScanned.Store(key, time.Now())

	scanCtx, cancel := context.WithTimeout(ctx, portDetectTimeout)
	defer cancel()
	open, err := r.PortScanner.Scan(scanCtx, ip)
	if err != nil {
		logf.FromContext(ctx).V(1).Info("port detection scan failed", "app", app.Name, "error", err.Error())
		return ""
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
		return ""
	case 1:
		if app.Status.DetectedPort == int32(others[0]) {
			return ""
		}
		// The same lone port in two scans: a metrics port opened early while
		// the HTTP server is still booting is not mistaken for the service.
		if prev, ok := r.portCandidate.Load(key); !ok || prev.(int) != others[0] {
			r.portCandidate.Store(key, others[0])
			return fmt.Sprintf(" The process is listening on port %d.", others[0])
		}
		logf.FromContext(ctx).Info("detected the port the process listens on", "app", app.Name, "port", others[0], "was", port)
		app.Status.DetectedPort = int32(others[0])
		return fmt.Sprintf(" The process is listening on port %d, which the service now uses.", others[0])
	default:
		list := make([]string, len(others))
		for i, p := range others {
			list[i] = strconv.Itoa(p)
		}
		return fmt.Sprintf(" The process is listening on ports %s, not on port %d; set the service's port (or its PORT variable) to the one that serves HTTP.",
			strings.Join(list, ", "), port)
	}
}

// unreadyPodIP is the address of the rollout's current-revision pod whose app
// container has been running past the grace period without becoming ready.
func (r *AppReconciler) unreadyPodIP(ctx context.Context, dep *appsv1.Deployment) string {
	if dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return ""
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace), client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		return ""
	}
	revision := dep.Spec.Template.Labels[labelRevision]
	for i := range pods.Items {
		p := &pods.Items[i]
		if p.DeletionTimestamp != nil || p.Status.Phase != corev1.PodRunning || p.Status.PodIP == "" ||
			(revision != "" && p.Labels[labelRevision] != revision) {
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
			if cs.Name == appContainerName && !cs.Ready && cs.State.Running != nil &&
				time.Since(cs.State.Running.StartedAt.Time) > portDetectGrace {
				return p.Status.PodIP
			}
		}
	}
	return ""
}
