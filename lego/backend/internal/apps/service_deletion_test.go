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

package apps

import (
	"context"
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// deletingApp returns an App whose deletion Kubernetes has accepted — a
// DeletionTimestamp plus the App finalizer that keeps the CR alive while the
// operator tears it down. The fake client requires a finalizer alongside the
// timestamp, which also mirrors the real terminating shape.
func deletingApp(name string) *appv1alpha1.App {
	a := sampleApp(name)
	now := metav1.NewTime(time.Unix(1_700_000_000, 0))
	a.DeletionTimestamp = &now
	a.Finalizers = []string{"app.bex.co/finalizer"}
	return a
}

func staticSite(a *appv1alpha1.App) *appv1alpha1.App {
	a.Spec.Type = appv1alpha1.TypeStaticSite
	a.Spec.Routes = []appv1alpha1.StaticRoute{{Type: "rewrite", Source: "/*", Destination: "/index.html"}}
	a.Spec.Headers = []appv1alpha1.StaticHeader{{Path: "/*", Name: "X-Frame-Options", Value: "DENY"}}
	return a
}

// TestDeletingServiceIsAbsentFromEveryByIDRead is the core w3/m81 read contract:
// the moment a service's deletion is accepted, every by-id surface returns the
// same core.ErrNotFound that List already applies by omitting the row — so a
// tenant sees one answer to "does this service still exist?" instead of a list
// that drops it while by-id keeps serving `phase: Deleting` plus a dead URL.
func TestDeletingServiceIsAbsentFromEveryByIDRead(t *testing.T) {
	svc, _ := newService(nil, staticSite(deletingApp("gone")))

	if _, err := svc.Get(context.Background(), "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Get on a deleting service = %v, want core.ErrNotFound", err)
	}
	if _, err := svc.ListRoutes(context.Background(), "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("ListRoutes on a deleting service = %v, want core.ErrNotFound", err)
	}
	if _, err := svc.ListHeaders(context.Background(), "gone"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("ListHeaders on a deleting service = %v, want core.ErrNotFound", err)
	}

	// And List agrees — the deleting row is absent, so the by-id 404 and the
	// list omission describe the same resource identically.
	list, err := svc.List(context.Background(), "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List returned %d rows, want 0 — a deleting service must not appear", len(list))
	}
}

// TestActiveServiceReadsAreUnaffected guards the regression the gate must not
// cause: a live service reads normally on every by-id surface, URL included.
func TestActiveServiceReadsAreUnaffected(t *testing.T) {
	svc, _ := newService(nil, staticSite(sampleApp("live")))

	v, err := svc.Get(context.Background(), "live")
	if err != nil {
		t.Fatalf("Get on a live service: %v", err)
	}
	if v.Phase != string(appv1alpha1.PhaseRunning) {
		t.Errorf("live phase = %q, want Running", v.Phase)
	}
	if v.URL == "" {
		t.Error("live service must still advertise its serving URL")
	}
	if routes, err := svc.ListRoutes(context.Background(), "live"); err != nil || len(routes) != 1 {
		t.Errorf("ListRoutes on a live static site = %v len=%d, want the one rule", err, len(routes))
	}
	if headers, err := svc.ListHeaders(context.Background(), "live"); err != nil || len(headers) != 1 {
		t.Errorf("ListHeaders on a live static site = %v len=%d, want the one rule", err, len(headers))
	}
}

// TestDeletingProjectionDropsDeadURL pins the defense-in-depth half: even the
// shared projection (bypassing the verb gate) never pairs a Deleting phase with
// a serving URL — the status.URL is blanked and the pending intent URL is not
// synthesized — so no code path can resurrect the dead-URL pair the m81 fixture
// served for 2+ hours.
func TestDeletingProjectionDropsDeadURL(t *testing.T) {
	// status.URL already set — must be dropped.
	a := deletingApp("torn")
	a.Status.URL = "https://torn.onbex.co"
	a.Status.URLs = []string{"https://torn.onbex.co"}
	if v := view(a); v.Phase != "Deleting" || v.URL != "" || len(v.URLs) != 0 {
		t.Fatalf("view(deleting) = phase %q url %q urls %v, want Deleting / empty / none", v.Phase, v.URL, v.URLs)
	}

	// No status.URL yet — the pending public URL fallback must also stay quiet
	// for a deleting web service whose host is being withdrawn.
	svc, _ := newService(nil)
	pending := deletingApp("pending-web")
	pending.Status.URL = ""
	pending.Spec.Type = appv1alpha1.TypeWebService
	svc.BaseDomain = "onbex.co"
	if v := svc.view(pending); v.URL != "" {
		t.Fatalf("pending URL for a deleting web service = %q, want empty", v.URL)
	}
}
