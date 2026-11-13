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
	"fmt"
	"slices"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/portscan"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's private network (render.com/docs/private-network; ADR089 D3 phase
// 2, w3/m273, w3/m276): a web or private service receives private traffic on
// any port it binds, up to 75, and a web service's port 10000 always reaches
// its primary HTTP server. bex publishes both on the stable ClusterIP Services
// and lists the listeners in openPorts. The listeners come from the w8/177
// scanner: a TCP connect from the operator to a Ready instance's pod IP on
// every port, so a loopback-only listener is never found.

// privatePortScanInterval spaces the scans of one service once its current
// revision has been scanned, so a listener opened after readiness is published
// within one interval.
const privatePortScanInterval = 2 * time.Minute

// discoverPrivatePorts scans a web or private service's current revision when
// a scan is due and records what it binds in status.runtimePorts. rolloutDone
// clears the previous revision's retained ports: no instance serving them
// remains. A changed set is written to both stable Services before status
// records it, so openPorts never names a port the hostname does not serve.
func (r *AppReconciler) discoverPrivatePorts(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, port int, rolloutDone bool) error {
	if !app.Spec.InternallyAddressable() {
		app.Status.RuntimePorts = nil
		return nil
	}
	next := r.scanPrivatePorts(ctx, app, dep)
	if next == nil {
		next = app.Status.RuntimePorts.DeepCopy()
	}
	if rolloutDone && next != nil {
		next.Retained = nil
	}
	if equality.Semantic.DeepEqual(next, app.Status.RuntimePorts) {
		return nil
	}
	projected := app.DeepCopy()
	projected.Status.RuntimePorts = next
	if !slices.Equal(projected.OpenPorts(), app.OpenPorts()) {
		if err := r.applyClusterIPService(ctx, projected, app.Name, port); err != nil {
			return err
		}
		if err := r.reconcileSlugService(ctx, projected, port); err != nil {
			return err
		}
		logf.FromContext(ctx).Info("private network listeners changed", "app", app.Name,
			"openPorts", projected.OpenPorts(), "was", app.OpenPorts())
	}
	app.Status.RuntimePorts = next
	return nil
}

// scanPrivatePorts returns the new runtime-port record, or nil when no scan
// was due, no instance was Ready, or the scan failed (the last record stands).
func (r *AppReconciler) scanPrivatePorts(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment) *appv1alpha1.RuntimePortsStatus {
	revision := dep.Spec.Template.Labels[labelRevision]
	if r.PortScanner == nil || revision == "" {
		return nil
	}
	prev := app.Status.RuntimePorts
	sameRevision := prev != nil && prev.Revision == revision
	key := app.Namespace + "/" + app.Name
	every := privatePortScanInterval
	if r.portDetectEvery > 0 {
		every = r.portDetectEvery
	}
	if last, ok := r.privatePortScanned.Load(key); ok && sameRevision && time.Since(last.(time.Time)) < every {
		return nil
	}
	// A rescan of a revision already scanned waits until the App is Running: a
	// wake keeps its revision, and the sweep (up to portDetectTimeout) on the
	// pass that first sees the woken pod Ready held Running and Ready=True back
	// while that pod was already serving (w4/517). The known listeners stand.
	if sameRevision && app.Status.Phase != appv1alpha1.PhaseRunning {
		return nil
	}
	ip := r.readyPodIP(ctx, dep, revision)
	if ip == "" {
		return nil
	}
	r.privatePortScanned.Store(key, time.Now())

	scanCtx, cancel := context.WithTimeout(ctx, portDetectTimeout)
	defer cancel()
	open, err := r.PortScanner.Scan(scanCtx, ip)
	partial := errors.Is(err, portscan.ErrPartial)
	if err != nil && !partial {
		logf.FromContext(ctx).Info("private network port scan failed; keeping the last listeners", "app", app.Name, "error", err.Error())
		return nil
	}
	found := listenerSet(open)
	next := &appv1alpha1.RuntimePortsStatus{Revision: revision, Ports: found}
	network := releaseNetwork(app, revision)
	if partial {
		// The sweep did not reach every port. A rescan may add, and a new
		// release's first record keeps its image's declared ports as unproven;
		// either way a declared port the direct check shows nothing binds goes.
		switch {
		case sameRevision:
			next.Ports = mergePorts(prev.Ports, found)
		case network != nil:
			next.Ports = mergePorts(found, network.Ports)
		}
		next.Ports = capPorts(withoutPorts(next.Ports, r.declaredPortsClosed(ctx, ip, network, next.Ports, found)))
	}
	if sameRevision {
		next.Retained = prev.Retained
		return next
	}
	if prev != nil {
		// A new revision: the old instances still serve what they bound until
		// the rollout replacing them completes.
		next.Retained = capPorts(withoutPorts(mergePorts(prev.Ports, prev.Retained), next.Ports))
	}
	return next
}

// declaredCheckTimeout bounds the direct check of a release's declared ports
// after its sweep ran out of time. ScanPorts dials 64 at once with a 400 ms
// connect timeout, so 75 declared ports and a control finish in two rounds.
const declaredCheckTimeout = 3 * time.Second

// releaseNetwork is the image network of the release at revision: the
// candidate's, else the serving one's; nil when neither is that release.
func releaseNetwork(app *appv1alpha1.App, revision string) *appv1alpha1.ImageNetworkStatus {
	for _, network := range []*appv1alpha1.ImageNetworkStatus{app.Status.ImageNetwork, app.Status.ServingNetwork} {
		if network != nil && network.Revision == revision {
			return network
		}
	}
	return nil
}

// declaredPortsClosed is the ports of listed that network declares, that a
// sweep which ran out of time did not find, and that a direct connect shows
// nothing binds (w3/m276 t008). Closed ports that drop probes keep a
// production sweep from finishing, so without this an image-declared port
// nothing listens on stayed in openPorts for the life of its release. A port
// the sweep found open, the primary when it can, is dialed in the same pass as
// a control: unless it answers, the pod is not answering and the check proves
// nothing.
func (r *AppReconciler) declaredPortsClosed(ctx context.Context, ip string, network *appv1alpha1.ImageNetworkStatus, listed, found []int32) []int32 {
	bounded, ok := r.PortScanner.(portsScanner)
	if !ok || network == nil || len(found) == 0 {
		return nil
	}
	unproven := withoutPorts(slices.DeleteFunc(slices.Clone(listed), func(p int32) bool { return !slices.Contains(network.Ports, p) }), found)
	if len(unproven) == 0 {
		return nil
	}
	control := found[0]
	if slices.Contains(found, network.PrimaryPort) {
		control = network.PrimaryPort
	}
	dial := []int{int(control)}
	for _, p := range unproven {
		dial = append(dial, int(p))
	}
	checkCtx, cancel := context.WithTimeout(ctx, declaredCheckTimeout)
	defer cancel()
	open, err := bounded.ScanPorts(checkCtx, ip, dial)
	if err != nil || !slices.Contains(open, int(control)) {
		return nil
	}
	return withoutPorts(unproven, listenerSet(open))
}

// detectPrivatePrimary rescues the release of a private image-port service
// whose process never binds the primary its image declares (w3/m273 t010):
// Render serves a private service on the port it binds, but the probe targets
// the declared primary, and scanPrivatePorts reads only Ready instances.
// While the TCP check fails for a service with no PORT or configured port, it
// scans the unready candidate; when the primary is closed and the same other
// listener (3000 if open, else the lowest) answers two scans in a row, that
// listener becomes the candidate network's primary and joins its ports, so
// the next pass probes and publishes on it. It returns text for the stall
// message: between scans, the last scan's.
func (r *AppReconciler) detectPrivatePrimary(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment, port int) string {
	network := app.Status.ImageNetwork
	if r.PortScanner == nil || !app.Spec.UsesImagePorts() || app.Spec.HealthCheckPath != "" || network == nil ||
		int(network.PrimaryPort) != port || network.Revision != releaseRevision(app) ||
		dep.Spec.Template.Labels[labelRevision] != network.Revision || rolloutDeadline(app, dep) != nil {
		return ""
	}
	if _, configured := app.ConfiguredImagePort(); configured {
		return ""
	}
	key, pod := r.dueUnreadyScan(ctx, app, dep)
	if pod == nil {
		return r.replayPortHint(ctx, key, dep, port)
	}
	return r.keepPortHint(key, pod, port, r.scanForPrivatePrimary(ctx, app, key, pod.Status.PodIP, port))
}

// scanForPrivatePrimary is detectPrivatePrimary's scan of ip.
func (r *AppReconciler) scanForPrivatePrimary(ctx context.Context, app *appv1alpha1.App, key, ip string, port int) string {
	network := app.Status.ImageNetwork
	scanCtx, cancel := context.WithTimeout(ctx, portDetectTimeout)
	defer cancel()
	open, err := scanBoundPort(scanCtx, r.PortScanner, ip, port)
	if err != nil {
		logf.FromContext(ctx).Info("private primary port scan failed", "app", app.Name, "error", err.Error())
		return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintNoOpenServicePort, portParam(int32(port)))
	}
	found := listenerSet(open)
	if len(found) == 0 || slices.Contains(found, network.PrimaryPort) {
		return "" // nothing to choose, or the primary answers now and the check will pass
	}
	chosen := found[0]
	if slices.Contains(found, imagePortPreferred) {
		chosen = imagePortPreferred
	}
	if !slices.Contains(network.Ports, chosen) && len(network.Ports) >= appv1alpha1.MaxImagePorts {
		return ""
	}
	// The same listener in two scans: a port opened early while the declared
	// primary is still booting is not mistaken for it.
	if prev, ok := r.portCandidate.Load(key); !ok || prev.(int) != int(chosen) {
		r.portCandidate.Store(key, int(chosen))
		return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintListening, portParam(chosen))
	}
	r.portCandidate.Delete(key)
	logf.FromContext(ctx).Info("selected the port the process listens on as the private primary", "app", app.Name,
		"port", chosen, "was", network.PrimaryPort)
	network.PrimaryPort = chosen
	network.Ports = mergePorts(network.Ports, []int32{chosen})
	return appv1alpha1.FormatDeployReason(appv1alpha1.DeployReasonHintListeningPrimary, portParam(chosen))
}

// listenerSet is a scan's answer as a sorted, deduplicated status set without
// reserved ports, within the CRD's 75 entries.
func listenerSet(open []int) []int32 {
	var ports []int32
	for _, p := range open {
		if p > 0 && p <= 65535 && !appv1alpha1.IsReservedImagePort(int32(p)) {
			ports = append(ports, int32(p))
		}
	}
	return capPorts(mergePorts(ports))
}

func mergePorts(sets ...[]int32) []int32 {
	out := slices.Concat(sets...)
	slices.Sort(out)
	return slices.Compact(out)
}

// withoutPorts is ports without any port in drop, as a new slice.
func withoutPorts(ports, drop []int32) []int32 {
	return slices.DeleteFunc(slices.Clone(ports), func(p int32) bool { return slices.Contains(drop, p) })
}

// capPorts bounds a set to the CRD's 75 entries; an empty set is nil, as
// status reads it back, so an unchanged record compares equal.
func capPorts(ports []int32) []int32 {
	if len(ports) == 0 {
		return nil
	}
	if len(ports) > appv1alpha1.MaxImagePorts {
		return ports[:appv1alpha1.MaxImagePorts]
	}
	return ports
}

// readyPodIP is the address of a Ready, running pod of the given revision.
func (r *AppReconciler) readyPodIP(ctx context.Context, dep *appsv1.Deployment, revision string) string {
	if dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return ""
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace), client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		return ""
	}
	for i := range pods.Items {
		p := &pods.Items[i]
		if p.DeletionTimestamp == nil && p.Status.Phase == corev1.PodRunning && p.Status.PodIP != "" &&
			p.Labels[labelRevision] == revision && podReady(p) {
			return p.Status.PodIP
		}
	}
	return ""
}

// wakeReadySince is when a parked App came back: the earliest Ready time of a
// Ready pod of its current revision, for an App whose Ready condition is about
// to flip from AutoHibernated or Suspended. The woken pod serves held requests
// from that moment, and the pass that writes Ready=True can land well after it
// (w4/517), so bex-api's woken/resumed event is stamped from this time. Zero
// (SetStatusCondition then stamps now) for any other transition, or with no
// such pod.
func (r *AppReconciler) wakeReadySince(ctx context.Context, app *appv1alpha1.App, dep *appsv1.Deployment) metav1.Time {
	cur := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
	if dep == nil || cur == nil || cur.Status != metav1.ConditionFalse ||
		(cur.Reason != reasonAutoHibernated && cur.Reason != reasonSuspended) ||
		dep.Spec.Selector == nil || len(dep.Spec.Selector.MatchLabels) == 0 {
		return metav1.Time{}
	}
	revision := dep.Spec.Template.Labels[labelRevision]
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(dep.Namespace), client.MatchingLabels(dep.Spec.Selector.MatchLabels)); err != nil {
		return metav1.Time{}
	}
	var since metav1.Time
	for i := range pods.Items {
		p := &pods.Items[i]
		if p.DeletionTimestamp != nil || p.Labels[labelRevision] != revision {
			continue
		}
		for _, c := range p.Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue && !c.LastTransitionTime.IsZero() &&
				(since.IsZero() || c.LastTransitionTime.Before(&since)) {
				since = c.LastTransitionTime
			}
		}
	}
	// Never after the write itself, nor before the park it ends.
	if now := metav1.Now(); since.IsZero() || now.Before(&since) || since.Before(&cur.LastTransitionTime) {
		return metav1.Time{}
	}
	return since
}

// withPrivateNetworkPorts adds what a stable Service serves on the private
// network beyond the service's own ports: a web or private service's
// discovered listeners, each targeting itself, and a web service's port 10000
// targeting its primary port's own target (the serving listener, the edge
// cache, or the image network's primary). An image that declares 10000 beside another
// primary has it retargeted, as Render routes 10000 to the primary HTTP server.
// Appended entries keep the first port, which release planning reads, first.
func withPrivateNetworkPorts(app *appv1alpha1.App, ports []corev1.ServicePort) []corev1.ServicePort {
	numbers := make([]int32, len(ports))
	for i, p := range ports {
		numbers[i] = p.Port
	}
	for _, n := range app.RuntimeOnlyPorts(numbers) {
		ports = append(ports, corev1.ServicePort{Name: fmt.Sprintf("tcp-%d", n), Port: n, TargetPort: intstr.FromInt32(n)})
	}
	if web := app.Spec.Type == "" || app.Spec.Type == appv1alpha1.TypeWebService; web && len(ports) > 0 {
		primary := ports[0].Port
		if network := app.ActiveImageNetwork(); network != nil {
			primary = network.PrimaryPort
		}
		at := slices.IndexFunc(ports, func(p corev1.ServicePort) bool { return p.Port == primary })
		if primary != appv1alpha1.PrivateHTTPPort && at >= 0 {
			target := ports[at].TargetPort
			if alias := slices.IndexFunc(ports, func(p corev1.ServicePort) bool { return p.Port == appv1alpha1.PrivateHTTPPort }); alias >= 0 {
				ports[alias].TargetPort = target
			} else {
				ports = append(ports, corev1.ServicePort{Name: fmt.Sprintf("tcp-%d", appv1alpha1.PrivateHTTPPort), Port: appv1alpha1.PrivateHTTPPort, TargetPort: target})
			}
		}
	}
	applyServicePortServerDefaults(ports)
	return ports
}
