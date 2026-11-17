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
	"fmt"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w7/m429 (docs/ADR030 §16): Render charges $0.25 a month for each custom
// domain beyond the plan's included count and refuses none for it, so a Hobby
// workspace (2 included) adds and verifies three more. Only the abuse caps
// (BEX_MAX_CUSTOM_DOMAINS_PER_SERVICE / _WORKSPACE) refuse a domain.
func TestAddDomainPastThePlansIncludedCountSucceeds(t *testing.T) {
	ctx := context.Background()
	claims := newMemoryDomainClaimStore()
	app := managedApp("web", "srv-1")
	app.Labels[core.LabelTenant] = "tea-hobby"
	claims.workspaces["srv-1"] = "tea-hobby"
	svc, cl := newService(claims, app)
	svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return store.PlanHobby, nil })
	svc.MaxCustomDomainsPerService, svc.MaxCustomDomainsPerWorkspace = 100, 500

	included, unlimited := core.IncludedCustomDomains(store.PlanHobby)
	if unlimited || included != 2 {
		t.Fatalf("Hobby includes %d custom domains (unlimited %v), want 2", included, unlimited)
	}
	var hosts []string
	for i := range included + 3 {
		host := fmt.Sprintf("app%d.example.com", i)
		hosts = append(hosts, host)
		if _, err := svc.AddDomain(ctx, "web", host); err != nil {
			t.Fatalf("AddDomain(%s), domain %d of %d included: %v", host, i+1, included, err)
		}
		verified, err := svc.VerifyDomain(ctx, "web", host)
		if err != nil || verified.VerificationStatus != "verified" {
			t.Fatalf("VerifyDomain(%s) = %+v, %v", host, verified, err)
		}
	}
	if got := getApp(t, cl, "web").Spec.Hosts; !slices.Equal(got, hosts) {
		t.Fatalf("serving hosts = %v, want all %v", got, hosts)
	}
}
