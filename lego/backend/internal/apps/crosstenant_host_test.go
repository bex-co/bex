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
