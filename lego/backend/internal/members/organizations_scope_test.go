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

package members

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// A narrowed OAuth token's capability failure is insufficient_scope, never an
// existence answer: the view gate runs before OpenFGA, and its refusal must
// not be converted to ORGANIZATION_NOT_FOUND the way a stranger's OpenFGA
// denial is. A read-scoped stranger still reads NOT_FOUND (pinned).
func TestOrganizationReadsNarrowedTokenSeesScope(t *testing.T) {
	svc := &Service{Base: &core.Base{Namespace: "default", Authz: denyChecker{}}}
	scopeless := core.WithIdentity(context.Background(), core.Identity{
		Subject: "stranger", Method: core.MethodOAuth2, Human: true,
	})
	readOnly := core.WithIdentity(context.Background(), core.Identity{
		Subject: "stranger", Method: core.MethodOAuth2, Human: true,
		CanonicalScopes: core.ScopeRead,
	})
	scoped := func(err error) bool {
		var coded *core.CodedError
		return errors.Is(err, core.ErrForbidden) && errors.As(err, &coded) && coded.Code == core.InsufficientScopeCode
	}
	missing := func(err error) bool {
		var coded *core.CodedError
		return errors.Is(err, core.ErrForbidden) && errors.As(err, &coded) && coded.Code == core.CodeOrganizationNotFound
	}
	for _, org := range []string{"org-a", "org-missing"} {
		if _, err := svc.OrgUserAuthStatus(scopeless, "usr-x", org); !scoped(err) {
			t.Errorf("scopeless OAuth token status in %s = %v, want %s", org, err, core.InsufficientScopeCode)
		}
		if _, err := svc.ListOrganizationMembers(scopeless, org); !scoped(err) {
			t.Errorf("scopeless OAuth token listing %s = %v, want %s", org, err, core.InsufficientScopeCode)
		}
		if _, err := svc.OrgUserAuthStatus(readOnly, "usr-x", org); !missing(err) {
			t.Errorf("read-only stranger status in %s = %v, want %s", org, err, core.CodeOrganizationNotFound)
		}
		if _, err := svc.ListOrganizationMembers(readOnly, org); !missing(err) {
			t.Errorf("read-only stranger listing %s = %v, want %s", org, err, core.CodeOrganizationNotFound)
		}
	}
}
