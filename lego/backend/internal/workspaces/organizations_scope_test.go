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

package workspaces

import (
	"context"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// A narrowed OAuth token's capability failure is insufficient_scope, never an
// existence answer: the read gate runs before OpenFGA, and its refusal must
// not be converted to ORGANIZATION_NOT_FOUND the way a stranger's OpenFGA
// denial is. A read-scoped stranger still reads NOT_FOUND (pinned).
func TestGetOrganizationNarrowedTokenSeesScope(t *testing.T) {
	svc := &Service{Base: &core.Base{Namespace: "default", Authz: &fakeChecker{allow: false}}}
	scopeless := core.WithIdentity(context.Background(), core.Identity{
		Subject: "stranger", Method: core.MethodOAuth2, Human: true,
	})
	readOnly := core.WithIdentity(context.Background(), core.Identity{
		Subject: "stranger", Method: core.MethodOAuth2, Human: true,
		CanonicalScopes: core.ScopeRead,
	})
	for _, org := range []string{"org-a", "org-missing"} {
		if _, err := svc.GetOrganization(scopeless, org); !isCoded(err, core.ErrForbidden, core.InsufficientScopeCode) {
			t.Errorf("scopeless OAuth token reading %s = %v, want %s", org, err, core.InsufficientScopeCode)
		}
		if _, err := svc.GetOrganization(readOnly, org); !isCoded(err, core.ErrForbidden, core.CodeOrganizationNotFound) {
			t.Errorf("read-only stranger reading %s = %v, want %s", org, err, core.CodeOrganizationNotFound)
		}
	}
}
