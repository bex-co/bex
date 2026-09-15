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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// These tests pin w1/m151: a free web service sleeps only after its idle window
// passes with no inbound traffic. At filing time a service answering a request
// every 15 s hibernated exactly 15:00 after its wake, because nothing that
// served an awake service advanced last-active.

// activityApp is a free web service (15-minute window) stamped at lastActive.
func activityApp(lastActive time.Time) *appv1alpha1.App {
	app := hibernatingApp(defaultAppsNamespace)
	app.Labels[labelAppID] = "srv-activity"
	app.Spec.IdleTTLSeconds = 900
	app.Annotations[annotLastActive] = lastActive.UTC().Format(time.RFC3339)
	return app
}

func activityReconciler(t *testing.T, app *appv1alpha1.App, reader AppActivityReader) (*AppReconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	return &AppReconciler{
		Client: cl, Scheme: scheme, Mode: ModeKubernetes, BaseDomain: "onbex.co",
		ActivatorService: "bex-activator", ActivatorNamespace: "bex-system", ActivatorPort: 8888,
		ActivityReader: reader,
	}, cl
}

func storedStamp(t *testing.T, cl client.Client, app *appv1alpha1.App) string {
	t.Helper()
	var live appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(app), &live); err != nil {
		t.Fatal(err)
	}
	return live.Annotations[annotLastActive]
}

func stamp(at time.Time) string { return at.UTC().Truncate(time.Second).Format(time.RFC3339) }

func TestIdleDecisionConsultsServedTraffic(t *testing.T) {
	now := time.Now()
	errPrometheus := errors.New("prometheus: status 503")
	cases := []struct {
		name        string
		lastActive  time.Time
		phase       appv1alpha1.AppPhase
		reader      func(calls *int) AppActivityReader
		wantSleep   bool
		wantStamp   func(original string) string
		wantAsked   bool
		wantRequeue time.Duration // checked when > 0
	}{
		{
			name:       "window elapsed by the stamp, traffic 30s ago: stays awake, stamp follows the traffic",
			lastActive: now.Add(-time.Hour),
			reader:     seenAt(now.Add(-30 * time.Second)),
			wantSleep:  false,
			wantStamp:  func(string) string { return stamp(now.Add(-30 * time.Second)) },
			wantAsked:  true,
		},
		{
			name:       "traffic after the stamp but itself older than the window: sleeps, stamp still follows it",
			lastActive: now.Add(-time.Hour),
			reader:     seenAt(now.Add(-20 * time.Minute)),
			wantSleep:  true,
			wantStamp:  func(string) string { return stamp(now.Add(-20 * time.Minute)) },
			wantAsked:  true,
		},
		{
			name:       "no traffic since the stamp: sleeps",
			lastActive: now.Add(-time.Hour),
			reader:     seenAt(time.Time{}),
			wantSleep:  true,
			wantStamp:  func(original string) string { return original },
			wantAsked:  true,
		},
		{
			name:       "metrics unreadable: stays awake, stamp untouched, asks again in a minute",
			lastActive: now.Add(-time.Hour),
			reader: func(calls *int) AppActivityReader {
				return func(context.Context, *appv1alpha1.App, time.Time) (time.Time, error) {
					*calls++
					return time.Time{}, errPrometheus
				}
			},
			wantSleep:   false,
			wantStamp:   func(original string) string { return original },
			wantAsked:   true,
			wantRequeue: activityRecheck,
		},
		{
			name:       "stamp still inside the window: traffic is not read",
			lastActive: now.Add(-5 * time.Minute),
			reader:     seenAt(now),
			wantSleep:  false,
			wantStamp:  func(original string) string { return original },
			wantAsked:  false,
		},
		{
			name:       "already hibernated: its route is the activator, traffic is not read",
			lastActive: now.Add(-time.Hour),
			phase:      appv1alpha1.PhaseHibernated,
			reader:     seenAt(now),
			wantSleep:  true,
			wantStamp:  func(original string) string { return original },
			wantAsked:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			app := activityApp(tc.lastActive)
			app.Status.Phase = tc.phase
			original := app.Annotations[annotLastActive]
			calls := 0
			r, cl := activityReconciler(t, app, tc.reader(&calls))

			_, _, sleeping := r.desiredReplicas(ctx, app)
			if sleeping != tc.wantSleep {
				t.Fatalf("auto-hibernating = %v, want %v", sleeping, tc.wantSleep)
			}
			if asked := calls > 0; asked != tc.wantAsked {
				t.Fatalf("activity read = %v (%d calls), want %v", asked, calls, tc.wantAsked)
			}
			if got, want := storedStamp(t, cl, app), tc.wantStamp(original); got != want {
				t.Fatalf("last-active = %q, want %q", got, want)
			}
			if tc.wantRequeue > 0 {
				res, err := r.runningRequeue(ctx, app, false)
				if err != nil || res.RequeueAfter != tc.wantRequeue {
					t.Fatalf("requeue = %v (err %v), want %v", res.RequeueAfter, err, tc.wantRequeue)
				}
			}
		})
	}
}

// Without a configured metrics backend (local clusters) the stamp alone
// decides, exactly as before.
func TestIdleDecisionWithoutAReaderUsesTheStamp(t *testing.T) {
	app := activityApp(time.Now().Add(-time.Hour))
	r, _ := activityReconciler(t, app, nil)
	if _, _, sleeping := r.desiredReplicas(context.Background(), app); !sleeping {
		t.Fatal("with no activity reader an App past its window must still hibernate")
	}
}

// After the idle check advances the stamp, the next check is timed from that
// traffic, not from the 5s floor.
func TestRequeueAfterTrafficIsTimedFromTheTraffic(t *testing.T) {
	now := time.Now()
	app := activityApp(now.Add(-time.Hour))
	r, _ := activityReconciler(t, app, seenAt(now.Add(-time.Minute))(new(int)))

	if _, _, sleeping := r.desiredReplicas(context.Background(), app); sleeping {
		t.Fatal("recent traffic must keep the service awake")
	}
	res, err := r.runningRequeue(context.Background(), app, false)
	if err != nil || res.RequeueAfter < 13*time.Minute || res.RequeueAfter > 14*time.Minute {
		t.Fatalf("requeue = %v (err %v), want ~14m: the window from the last request", res.RequeueAfter, err)
	}
}

// The DoD path through a full reconcile: a service answering traffic keeps its
// replica, then sleeps once the traffic stops and the window passes.
func TestSteadyTrafficKeepsAFreeServiceAwakeThenItSleepsWhenQuiet(t *testing.T) {
	ctx := context.Background()
	app := activityApp(time.Now().Add(-time.Hour))
	seen := time.Now().Add(-15 * time.Second)
	r, cl := activityReconciler(t, app, func(context.Context, *appv1alpha1.App, time.Time) (time.Time, error) {
		return seen, nil
	})
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	reconcileTwice(t, r, nn)
	var dep appsv1.Deployment
	if err := cl.Get(ctx, nn, &dep); err != nil {
		t.Fatal(err)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 1 {
		t.Fatalf("replicas under steady traffic = %v, want 1", dep.Spec.Replicas)
	}

	// Traffic stops; the last request is now older than the window.
	seen = time.Time{}
	var live appv1alpha1.App
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Annotations[annotLastActive] = stamp(time.Now().Add(-16 * time.Minute))
	if err := cl.Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)
	if err := cl.Get(ctx, nn, &dep); err != nil {
		t.Fatal(err)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 0 {
		t.Fatalf("replicas after a quiet window = %v, want 0", dep.Spec.Replicas)
	}
}

func TestPrometheusAppActivityReaderQueriesBothSignals(t *testing.T) {
	now := time.Now()
	requestAt := now.Add(-2 * time.Minute).Truncate(time.Second)
	frameAt := now.Add(-40 * time.Second).Truncate(time.Second)
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query().Get("query"))
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[%d,"%d"]},{"metric":{},"value":[%d,"%d"]}]}}`,
			now.Unix(), requestAt.Unix(), now.Unix(), frameAt.Unix())
	}))
	defer srv.Close()
	// A stamp far older than the window: the lookback still stops at the window.
	app := activityApp(now.Add(-3 * 24 * time.Hour))
	app.Namespace = "tea-ws"

	got, err := NewPrometheusAppActivityReader(srv.URL, srv.Client())(context.Background(), app, lastActiveTime(app))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(frameAt) {
		t.Fatalf("activity = %v, want the latest sample %v", got, frameAt)
	}
	if len(queries) != 1 {
		t.Fatalf("queries = %q, want one round trip for both signals", queries)
	}
	q := queries[0]
	for _, want := range []string{
		`traefik_service_requests_total{service="tea-ws-web-3000@kubernetes"}`,
		`bex_websocket_egress_bytes_total{app_id="srv-activity"}`,
		" or ",
		"[915s:15s])", // the 15-minute window plus one step, not three days
	} {
		if !strings.Contains(q, want) {
			t.Errorf("query = %q, want it to contain %q", q, want)
		}
	}
	// Every request counts, whatever its status: no code matcher.
	if strings.Contains(q, "code") {
		t.Errorf("query %q filters by status; Render counts all inbound traffic", q)
	}
}

func TestPrometheusAppActivityReaderNothingNewer(t *testing.T) {
	since := time.Now().Add(-10 * time.Minute).Truncate(time.Second)
	for name, body := range map[string]string{
		// A step at the stamp itself is the traffic the stamp already knows.
		"step at the stamp": fmt.Sprintf(`{"status":"success","data":{"result":[{"metric":{},"value":[0,"%d"]}]}}`, since.Unix()),
		"no steps":          `{"status":"success","data":{"result":[]}}`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, body)
		}))
		got, err := NewPrometheusAppActivityReader(srv.URL, srv.Client())(context.Background(), activityApp(since), since)
		srv.Close()
		if err != nil || !got.IsZero() {
			t.Errorf("%s: activity = %v (err %v), want none newer than the stamp", name, got, err)
		}
	}
}

// An unavailable Prometheus is an error, never "no traffic" — and after one
// failure the reader stops asking for a while, so a hung backend cannot hold a
// reconcile worker for every awake free service in turn.
func TestPrometheusAppActivityReaderFailsFastAfterAnError(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	read := NewPrometheusAppActivityReader(srv.URL, srv.Client())
	since := time.Now().Add(-time.Hour)

	if _, err := read(context.Background(), activityApp(since), since); err == nil {
		t.Fatal("an unavailable Prometheus must be an error")
	}
	if _, err := read(context.Background(), activityApp(since), since); !errors.Is(err, errActivityUnavailable) {
		t.Fatalf("second read error = %v, want errActivityUnavailable without a query", err)
	}
	if requests != 1 {
		t.Fatalf("Prometheus requests = %d, want 1 while the breaker is open", requests)
	}
}

func seenAt(at time.Time) func(calls *int) AppActivityReader {
	return func(calls *int) AppActivityReader {
		return func(context.Context, *appv1alpha1.App, time.Time) (time.Time, error) {
			*calls++
			return at, nil
		}
	}
}
