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
	"fmt"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestCreateRejectsReservedAndClaimedHosts is w7/m57's regression guard for the
// cross-tenant host-hijack finding. BEFORE the fix, create bound spec.hosts
// verbatim (the reserved-platform + cross-App collision gate ran ONLY on
// AddDomain), so a tenant could create a service claiming a platform host
// (dashboard/`*.<base>`) or one another App already owned and have the operator
// mint an Ingress that hijacks it. The gate now runs on the shared create write
// seam (writeNewApp), so every create/blueprint/deploy-manifest path is covered.
func TestCreateRejectsReservedAndClaimedHosts(t *testing.T) {
	victim := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "victim", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: "img:1", Hosts: []string{"victim.example.com"}},
	}
	svc, _ := newBaseDomainService("onbex.co", "dashboard.bex.co", victim)
	ctx := context.Background()

	// Each of these host claims must be REFUSED — pre-fix they all succeeded.
	for _, tc := range []struct {
		name, host string
		wantErr    error
	}{
		{"reserved-dashboard-host", "dashboard.bex.co", core.ErrBadRequest},
		{"reserved-platform-subdomain", "someoneelse.onbex.co", core.ErrBadRequest},
		{"reserved-base-apex", "onbex.co", core.ErrBadRequest},
		{"host-owned-by-another-app", "victim.example.com", core.ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.create(ctx, CreateRequest{Name: "attacker", Image: "img:1", Hosts: []string{tc.host}})
			if err == nil {
				t.Fatalf("create claiming %q was ACCEPTED — cross-tenant host hijack", tc.host)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("claiming %q: got %v, want a %v refusal", tc.host, err, tc.wantErr)
			}
			// The hijacking App must not have been written.
			if _, gErr := svc.Get(ctx, "attacker"); gErr == nil {
				t.Errorf("the refused create still wrote an App for host %q", tc.host)
			}
		})
	}

	// A create with a genuinely free host still succeeds — no false positive.
	if _, err := svc.create(ctx, CreateRequest{Name: "legit", Image: "img:1", Hosts: []string{"mine.example.com"}}); err != nil {
		t.Fatalf("create with a free custom host was refused: %v", err)
	}
}

// TestCreateCanonicalizesHostnamesAtWrite is the regression guard for hostname
// canonicalization at the write boundary. BEFORE the fix, create persisted
// req.Hosts verbatim while the uniqueness sweep compared raw strings and
// AddDomain only lowercased — so a case- or trailing-dot variant of another
// tenant's host slipped past as a distinct string, while downstream consumers
// (the activator's lowercasing last-write-wins host map) collapsed them into
// the same route. Every write boundary now runs canonicalHostname (trim, drop
// terminal dot, lowercase, DNS-1123 validate) and persists only the canonical
// value, and the sweep normalizes stored claims before comparing.
func TestCreateCanonicalizesHostnamesAtWrite(t *testing.T) {
	victim := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "victim", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: "img:1", Hosts: []string{"victim.example.com"}},
	}
	svc, cl := newService(nil, victim)
	ctx := context.Background()

	// Canonical form is persisted: whitespace, mixed case, and a terminal dot
	// in — one clean lowercase host stored, on spec.host and spec.hosts alike.
	if _, err := svc.create(ctx, CreateRequest{Name: "canon", Image: "img:1", Hosts: []string{"  Foo.Example.com. ", "WWW.foo.example.com"}}); err != nil {
		t.Fatalf("create with canonicalizable hosts: %v", err)
	}
	if got := getApp(t, cl, "canon"); got.Spec.Host != "foo.example.com" ||
		len(got.Spec.Hosts) != 1 || got.Spec.Hosts[0] != "www.foo.example.com" {
		t.Fatalf("persisted hosts = %q %v, want canonical [foo.example.com www.foo.example.com]", got.Spec.Host, got.Spec.Hosts)
	}

	// A second App whose host differs only by case — or only by a trailing dot —
	// from an existing claim is refused at create, across workspaces too (the
	// sweep is cluster-wide). Pre-fix these all slipped through.
	for i, tc := range []struct{ name, host string }{
		{"case-variant-of-other-app", "Victim.Example.com"},
		{"trailing-dot-variant-of-other-app", "victim.example.com."},
		{"case-variant-of-canonicalized-host", "FOO.EXAMPLE.COM"},
		{"trailing-dot-variant-of-canonicalized-host", "foo.example.com."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := fmt.Sprintf("attacker-%d", i)
			if _, err := svc.create(ctx, CreateRequest{Name: name, Image: "img:1", Hosts: []string{tc.host}}); !errors.Is(err, core.ErrConflict) {
				t.Errorf("create claiming %q: got %v, want core.ErrConflict", tc.host, err)
			}
			// The same variants are refused through AddDomain on a third App.
			if _, err := svc.AddDomain(ctx, "canon", tc.host); tc.name == "case-variant-of-other-app" || tc.name == "trailing-dot-variant-of-other-app" {
				if !errors.Is(err, core.ErrConflict) {
					t.Errorf("AddDomain(%q): got %v, want core.ErrConflict", tc.host, err)
				}
			}
		})
	}

	// AddDomain persists only the canonical value.
	if _, err := svc.AddDomain(ctx, "canon", "API.Example.com."); err != nil {
		t.Fatalf("AddDomain with canonicalizable host: %v", err)
	}
	if hosts := getApp(t, cl, "canon").Spec.Hosts; !slices.Contains(hosts, "api.example.com") {
		t.Errorf("spec.hosts = %v, want canonical api.example.com present", hosts)
	}

	// Invalid host syntax is a named 400 at both write boundaries.
	if _, err := svc.create(ctx, CreateRequest{Name: "bad", Image: "img:1", Hosts: []string{"not a host!"}}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("create with invalid host: got %v, want core.ErrBadRequest", err)
	}
	if _, err := svc.AddDomain(ctx, "canon", "not a host!"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("AddDomain with invalid host: got %v, want core.ErrBadRequest", err)
	}
}
