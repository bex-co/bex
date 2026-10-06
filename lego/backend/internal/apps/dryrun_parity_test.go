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

// dryrun_parity_test.go — w8/045: a dry-run service create runs every
// read-only check the real create runs, so a preview never answers "would
// succeed" for a create the real call refuses, while still writing nothing.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestCreateDryRunRefusesWhatTheRealCreateRefuses is the parity table: each
// refusal is produced by the dry-run with the SAME error the real create
// returns, and the dry-run writes no App.
func TestCreateDryRunRefusesWhatTheRealCreateRefuses(t *testing.T) {
	victim := func() *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "victim", Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Image: "img:1", Hosts: []string{"victim.example.com"}},
		}
	}
	taken := func() *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "taken", Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Image: "img:1"},
		}
	}
	credential := "rgc-x"
	for _, tc := range []struct {
		name    string
		req     CreateRequest
		setup   func(*Service)
		wantErr error
	}{
		{name: "reserved dashboard host", req: CreateRequest{Name: "web", Image: "img:1", Hosts: []string{"dashboard.bex.co"}}, wantErr: core.ErrBadRequest},
		{name: "reserved platform subdomain", req: CreateRequest{Name: "web", Image: "img:1", Hosts: []string{"someoneelse.onbex.co"}}, wantErr: core.ErrBadRequest},
		{name: "reserved base apex", req: CreateRequest{Name: "web", Image: "img:1", Hosts: []string{"onbex.co"}}, wantErr: core.ErrBadRequest},
		{name: "domain claimed by another service", req: CreateRequest{Name: "web", Image: "img:1", Hosts: []string{"victim.example.com"}}, wantErr: core.ErrConflict},
		{
			name:    "per-service domain cap",
			req:     CreateRequest{Name: "web", Image: "img:1", Hosts: []string{"a.example.com", "b.example.com"}},
			setup:   func(s *Service) { s.MaxCustomDomainsPerService = 1 },
			wantErr: core.ErrBadRequest,
		},
		{name: "duplicate name", req: CreateRequest{Name: "taken", Image: "img:1"}, wantErr: core.ErrConflict},
		{name: "reserved PORT env", req: CreateRequest{Name: "web", Image: "img:1", Env: []appv1alpha1.EnvVar{{Name: "PORT", Value: "8080"}}}, wantErr: core.ErrBadRequest},
		{
			name: "inaccessible repository",
			req:  CreateRequest{Name: "web", Type: appv1alpha1.TypeWebService, Repo: "https://github.com/render-oss/render-examples-go-gin"},
			setup: func(s *Service) {
				s.GitHub = &fakeCloneTokens{validateErr: fmt.Errorf("%w: repository is not accessible", core.ErrBadRequest)}
			},
			wantErr: core.ErrBadRequest,
		},
		{
			name:    "registry credential without registry support",
			req:     CreateRequest{Name: "web", Image: "ghcr.io/acme/web:1", RegistryCredentialID: &credential},
			wantErr: core.ErrRegistryCredentialsUnavailable,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(dryRun bool) error {
				svc, cl := newBaseDomainService("onbex.co", "dashboard.bex.co", victim(), taken())
				if tc.setup != nil {
					tc.setup(svc)
				}
				req := tc.req
				req.DryRun = dryRun
				_, err := svc.Create(context.Background(), req)
				if dryRun && countApps(t, cl) != 2 {
					t.Errorf("dry-run wrote an App (have %d, want the 2 seeded)", countApps(t, cl))
				}
				return err
			}
			real, dry := run(false), run(true)
			if !errors.Is(real, tc.wantErr) {
				t.Fatalf("real create = %v, want %v (fixture no longer exercises the refusal)", real, tc.wantErr)
			}
			if dry == nil || dry.Error() != real.Error() {
				t.Fatalf("dry-run = %v, want the real create's refusal %q", dry, real)
			}
		})
	}

	// No false positive: a create every check admits still previews.
	svc, cl := newBaseDomainService("onbex.co", "dashboard.bex.co", victim(), taken())
	v, err := svc.Create(context.Background(), CreateRequest{Name: "web", Image: "img:1", Hosts: []string{"mine.example.com"}, DryRun: true})
	if err != nil || v.Name != "web" {
		t.Fatalf("admissible dry-run = %+v, %v", v, err)
	}
	if n := countApps(t, cl); n != 2 {
		t.Fatalf("admissible dry-run wrote an App: %d", n)
	}
}

// TestCreateDryRunRunsTheBillingGate pins the explicit decision: a dry-run
// returns the same 402 the real create does (real-create parity), and never
// writes.
func TestCreateDryRunRunsTheBillingGate(t *testing.T) {
	svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	gate := &rejectingPaymentGate{}
	svc.Payment = gate
	_, err := svc.Create(paidGateContext(), CreateRequest{Name: "paid", Image: "nginx:alpine", Plan: "starter", DryRun: true})
	if !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) != 1 {
		t.Fatalf("paid dry-run err=%v calls=%v, want the 402", err, gate.calls)
	}
	var apps appv1alpha1.AppList
	if err := cl.List(context.Background(), &apps); err != nil || len(apps.Items) != 0 {
		t.Fatalf("dry-run refusal wrote Apps: %+v err=%v", apps.Items, err)
	}
}

// TestCreateExemptsTheServicesOwnPlatformHost (w5/m116): with the store on,
// store.CreateApp mints the slug as the service name (a suffix only when
// another workspace already holds it), so a create may claim `<name>.<base>`.
// Its dry-run and the real create agree on that, and both refuse another
// service's platform host. Before, the preview refused its own host too, and
// running the create's plan ahead of the real write carried that onto it.
func TestCreateExemptsTheServicesOwnPlatformHost(t *testing.T) {
	for _, dryRun := range []bool{true, false} {
		svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
		svc.Store = &recordingStore{}
		svc.BaseDomain = "onbex.co"
		if _, err := svc.Create(paidGateContext(), CreateRequest{Name: "web", Image: "img:1", Hosts: []string{"web.onbex.co"}, DryRun: dryRun}); err != nil {
			t.Fatalf("dryRun=%v: a create claiming its own web.onbex.co = %v, want it accepted", dryRun, err)
		}
		if _, err := svc.Create(paidGateContext(), CreateRequest{Name: "api", Image: "img:1", Hosts: []string{"other.onbex.co"}, DryRun: dryRun}); !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("dryRun=%v: a create claiming other.onbex.co = %v, want the reserved-host refusal", dryRun, err)
		}
	}
}

// TestRESTDryRunCreateRefusesReservedDashboardHost is the live QA repro
// (2026-10-03) at the REST boundary: `?dryRun=true` with the dashboard host
// answers the same 400 the real create does, not a preview.
func TestRESTDryRunCreateRefusesReservedDashboardHost(t *testing.T) {
	svc, cl := newBaseDomainService("onbex.co", "dashboard.bex.co")
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	body := `{"name":"preview-svc","type":"web_service","image":{"imagePath":"img:v1"},"domains":["dashboard.bex.co"]}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services?dryRun=true", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "reserved platform hostname") {
		t.Fatalf("dry-run create with dashboard host => %d %s, want 400 reserved platform hostname", rec.Code, rec.Body)
	}
	if n := countApps(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d App(s)", n)
	}
}
