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
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	boundedhttp "github.com/bex-co/bex/lego/operator/internal/httpclient"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A free web service sleeps after its idle window "without receiving any inbound
// traffic" (Render). While it is awake its requests go straight to its own
// Service and never pass the activator, so the last-active stamp alone cannot
// see them: before w1/m151 a service under steady traffic slept every 15
// minutes. The operator therefore asks Prometheus, which already scrapes both
// signals, before it puts a service to sleep.

// AppActivityReader returns the latest time after since at which the App served
// traffic, or the zero time when it served none. Production reads Prometheus;
// tests inject a deterministic reader.
type AppActivityReader func(ctx context.Context, app *appv1alpha1.App, since time.Time) (time.Time, error)

const (
	// activityStep is the resolution of the activity lookup: the Traefik and
	// WebSocket-meter scrape interval (deploy/gitops/base/prometheus.yaml).
	activityStep = 15 * time.Second
	// activityRecheck is how soon an App kept awake only because its traffic
	// could not be read is asked again — slower than idleRequeueAfter's 5s floor
	// so a Prometheus outage is not polled in a hot loop.
	activityRecheck = time.Minute
	// activityBreaker is how long the reader answers errActivityUnavailable
	// without querying after a failed read. Every awake free service ends up
	// asking during an outage, and a hung Prometheus would otherwise hold a
	// reconcile worker for the full request timeout on each of them, starving
	// every other App's reconcile.
	activityBreaker = 30 * time.Second
)

var errActivityUnavailable = errors.New("service activity unavailable: a recent Prometheus read failed")

// NewPrometheusAppActivityReader returns a reader over series Prometheus
// already scrapes: Traefik's per-service request counter (every request,
// whatever its status — Render counts inbound traffic, not successes) and the
// websocketegress plugin's per-App frame counters, in both directions. No new
// scrape or RBAC.
func NewPrometheusAppActivityReader(base string, hc *http.Client) AppActivityReader {
	if hc == nil {
		hc = boundedhttp.Shared
	}
	base = strings.TrimRight(base, "/")
	var failedAt atomic.Int64 // unix nanos of the last failed read; 0 = none
	return func(ctx context.Context, app *appv1alpha1.App, since time.Time) (time.Time, error) {
		if last := failedAt.Load(); last != 0 && time.Since(time.Unix(0, last)) < activityBreaker {
			return time.Time{}, errActivityUnavailable
		}
		// Traffic older than the window cannot keep the App awake, so the
		// lookback never needs to reach further back — even when the stamp is
		// days old (a frozen stamp during an outage, a long-parked App).
		lookback := min(time.Since(since), autoSleepWindow(app)+activityStep)
		samples, err := promInstantQuery(ctx, hc, base, activityQuery(app, lookback))
		if err != nil {
			failedAt.Store(time.Now().UnixNano())
			return time.Time{}, err
		}
		var latest time.Time
		for _, s := range samples {
			if t := time.Unix(0, int64(s.value*float64(time.Second))); t.After(latest) {
				latest = t
			}
		}
		if !latest.After(since) {
			return time.Time{}, nil
		}
		return latest, nil
	}
}

// activityQuery asks for the latest instant within lookback at which any
// signal showed traffic. Each original series (one per Traefik pod, method and
// status, or per meter pod) is judged before labels are aggregated away, and
// counts as traffic at an instant when either
//
//   - it rose over the preceding minute — increase() is reset-safe, and a
//     reset to zero is no traffic; or
//   - it first appeared, already positive, within the preceding step. A
//     counter is created by the request it counts, so a new series' first
//     sample is traffic increase() cannot see: it needs two samples, and the
//     sole GET a fresh service answered (1, then 1) has none (w4/m164, sweep
//     71). A flat series is never new again, so it cannot keep a service
//     awake; only a gap longer than Prometheus's 5m lookback, or a stale marker
//     from a failed scrape, makes it look new — once, for one step.
//
// Subquery steps sit on Prometheus's absolute 15s grid, so traffic scraped
// after the last step before now was invisible until the next one (the 503
// sweep 71's steady control got): the query is also asked at now itself, and
// that answer, never older than any step, wins when present.
//
// Every answer is at or after the scrape that recorded the traffic, so the
// stamp never predates the request (to the second), and trails it by at most
// a minute plus a scrape — which only ever delays a sleep. Traffic newer than
// the latest scrape (one 15s interval) cannot be seen at all: a service sleeps
// only after a full window with no traffic Prometheus has recorded, and a
// request in that final unscraped moment, already served, is followed by the
// sleep; its next request wakes the service. A series that went stale when the
// service was parked yields nothing rather than activity.
func activityQuery(app *appv1alpha1.App, lookback time.Duration) string {
	step := int(activityStep.Seconds())
	active := func(series string) string {
		return fmt.Sprintf("increase(%[1]s[1m]) > 0 or (%[1]s > 0 unless %[1]s offset %[2]ds)", series, step)
	}
	service := traefikServiceLabel(app.Namespace, app.Name, app.Spec.EffectivePort())
	// Both WebSocket directions count as traffic: a connection the client alone
	// feeds (telemetry, a log shipper) kept no service awake while only the
	// egress counter was read, so a free service slept under real use (w1/m161,
	// from w1/102). `or` over a series Prometheus does not have yet contributes
	// nothing, so an operator ahead of the plugin roll behaves exactly as before.
	appID := strconv.Quote(appIDOrName(app))
	traffic := fmt.Sprintf("count(%s or %s or %s)",
		active("traefik_service_requests_total{service="+strconv.Quote(service)+"}"),
		active("bex_websocket_egress_bytes_total{app_id="+appID+"}"),
		active("bex_websocket_ingress_bytes_total{app_id="+appID+"}"))
	return fmt.Sprintf("timestamp(%[1]s) or max_over_time(timestamp(%[1]s)[%[2]ds:%[3]ds])",
		traffic, int(math.Ceil(max(lookback, time.Minute).Seconds())), step)
}

// traefikServiceLabel is the Traefik Kubernetes-Ingress provider's service name
// for the App's own Service, "<namespace>-<service>-<port>@kubernetes" — the
// identity bex-api's metrics read under the same name (the operator cannot
// import the backend). While the activator holds the route the Ingress backend
// is a different Service, so requests the activator answers never count here.
func traefikServiceLabel(namespace, service string, port int32) string {
	return fmt.Sprintf("%s-%s-%d@kubernetes", namespace, service, port)
}

// recentlyActive is asked only once the last-active stamp says the idle window
// has elapsed: did the App serve traffic the stamp does not know about? It
// advances the stamp to the latest observed traffic, so the next idle check is
// timed from the last request, and reports whether that traffic still falls
// inside the window. A reader error answers true and leaves the stamp alone —
// an unreadable metrics backend must never put a busy service to sleep — and
// runningRequeue asks again after activityRecheck. With no reader configured
// the stamp alone decides, as before.
func (r *AppReconciler) recentlyActive(ctx context.Context, app *appv1alpha1.App) bool {
	// A parked App's route is the activator; its own Service serves nothing.
	if r.ActivityReader == nil || app.Status.Phase == appv1alpha1.PhaseHibernated {
		return false
	}
	log := logf.FromContext(ctx)
	last := lastActiveTime(app)
	seen, err := r.ActivityReader(ctx, app, last)
	if err != nil {
		log.Error(err, "reading service activity failed; keeping the service awake", "app", app.Name)
		return true
	}
	seen = seen.UTC().Truncate(time.Second)
	if !seen.After(last) {
		return false
	}
	if err := r.stampLastActive(ctx, app, seen); err != nil {
		log.Error(err, "advancing last-active failed; keeping the service awake", "app", app.Name)
		return true
	}
	return !shouldAutoHibernate(app)
}

// stampLastActive merge-patches the App's last-active annotation to at, the one
// format lastActiveTime parses. The patch carries the resourceVersion the pass holds:
// the activator stamps the same annotation on a wake, and a stale cached copy
// must conflict rather than move a newer wake stamp backwards. extra annotations
// ride in the same patch.
func (r *AppReconciler) stampLastActive(ctx context.Context, app *appv1alpha1.App, at time.Time, extra ...[2]string) error {
	return r.patchAppMeta(ctx, app, func(meta *metav1.ObjectMeta) {
		metav1.SetMetaDataAnnotation(meta, annotLastActive, at.UTC().Format(time.RFC3339))
		for _, kv := range extra {
			metav1.SetMetaDataAnnotation(meta, kv[0], kv[1])
		}
	})
}
