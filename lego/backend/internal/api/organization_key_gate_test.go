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

package api

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// An organization's audit-log key (w1/m657, ADR115 D11) passes the dispatch
// gate on exactly Render's two audit-log reads. Every other live REST, GraphQL
// and MCP operation refuses it with a scoped key's refusal, whatever that
// operation asks of a scoped key (core.KeyAnyGrant included), and so does an
// operation added later.
func TestOrganizationKeyPassesOnlyTheAuditLogReads(t *testing.T) {
	srv, grants := scopedKeyServer(t)
	grants.set("org-key", core.OrganizationKeyGrant("org-a"))
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "org-key", ClientID: "org-key", Method: core.MethodOAuth2})

	live := liveScopeOperations(t)
	var passed []string
	for _, op := range live {
		err := srv.requireScopeClass(ctx, op)
		if err == nil {
			passed = append(passed, op)
			continue
		}
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != core.KeyPermissionDeniedCode {
			t.Errorf("%s refused the org key with %v, want %s", op, err, core.KeyPermissionDeniedCode)
		}
	}
	slices.Sort(passed)
	want := slices.Sorted(maps.Keys(organizationKeyOps))
	if !slices.Equal(passed, want) {
		t.Fatalf("the org key passed %v, want exactly %v", passed, want)
	}
	if err := srv.requireScopeClass(ctx, "REST GET /v1/brand-new"); err == nil {
		t.Error("an operation added later let the org key through")
	}
	// A GraphQL document is one decision over every top-level field, so a
	// permitted name cannot carry a refused one.
	if err := srv.requireKeyGrant(ctx, "REST GET /v1/owners/{ownerId}/audit-logs", "GQL Query.auditLogs"); err == nil {
		t.Error("a document mixing an allowed and a refused operation passed")
	}
}

// Through REST the same: the two reads reach their handlers, the rest is
// refused before any handler runs.
func TestOrganizationKeyRESTRefusal(t *testing.T) {
	srv, grants := scopedKeyServer(t)
	grants.set("org-key", core.OrganizationKeyGrant("org-a"))
	key := core.Identity{Subject: "org-key", ClientID: "org-key", Method: core.MethodOAuth2}
	for _, path := range []string{"/v1/services", "/v1/users", "/v1/owners", "/v1/api-keys"} {
		if status, body := restAs(t, srv, key, "GET", path, ""); !grantRefused(status, body) {
			t.Errorf("GET %s with the org key = %d %v, want the scoped-key refusal", path, status, body)
		}
	}
	// The audit-log route is let through the gate; with no audit store here
	// its handler answers unavailable, never the gate's refusal.
	if status, body := restAs(t, srv, key, "GET", "/v1/owners/tea-a/audit-logs", ""); grantRefused(status, body) {
		t.Errorf("the workspace audit log refused the org key at the gate: %d %v", status, body)
	}
}

// An org grant covers no catalog action, not even one any key may call, so
// a field-level projection or a stream's watchdog that asks the grant refuses
// it too.
func TestOrganizationKeyGrantAllowsNothing(t *testing.T) {
	g := core.OrganizationKeyGrant("org-a")
	for _, actions := range [][]core.KeyAction{{core.KeyAnyGrant}, {core.KeyAuditLogsView}, {core.KeyNoGrant}, nil} {
		if g.Allows(actions...) {
			t.Errorf("an org grant allows %v", actions)
		}
	}
}
