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
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestMembershipIssuanceRefusesADelegatedThirdPartyToken is round 24's H-1: a
// third-party OAuth client an admin consented to with bex.write could invite
// an admin, re-mint an invite, or promote a member, and that membership
// outlived the consent. Membership issuance now takes AuthorizeMintClass, as
// API-key, SSH-key and webhook creation do: a direct session and a platform
// client still pass.
func TestMembershipIssuanceRefusesADelegatedThirdPartyToken(t *testing.T) {
	st := newFakeStore(store.PlanPro)
	st.seedMember("admin-1", "admin")
	st.seedMember("dev-1", "developer")
	mail := &fakeMailer{}
	s := svc(st, newFakeGranter(), mail, nil)
	s.Base.PlatformClients = coretest.PlatformClients{"platform-cli": true}

	pending, err := s.Invite(ctxWith("admin-1"), "tea-1", "colleague@example.com", "developer")
	if err != nil {
		t.Fatalf("session invite: %v", err)
	}
	invites, mails := len(st.invites), mail.calls

	thirdParty := core.WithIdentity(context.Background(), core.Identity{
		Subject: "admin-1", Method: "oauth2", ClientID: "dcr-agent-7", Human: true, CanonicalScopes: core.ScopeWrite,
	})
	if _, err := s.Invite(thirdParty, "tea-1", "attacker@evil.example", "admin"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("third-party Invite => %v, want ErrForbidden", err)
	}
	if _, err := s.ResendInvite(thirdParty, "tea-1", pending.ID); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("third-party ResendInvite => %v, want ErrForbidden", err)
	}
	if _, err := s.ChangeRole(thirdParty, "tea-1", "dev-1", "admin"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("third-party ChangeRole => %v, want ErrForbidden", err)
	}
	if len(st.invites) != invites || mail.calls != mails || st.members["dev-1"].Role != "developer" {
		t.Fatalf("a refused delegated token still issued membership: invites %d→%d, mails %d→%d, dev-1 %q",
			invites, len(st.invites), mails, mail.calls, st.members["dev-1"].Role)
	}

	platform := core.WithIdentity(context.Background(), core.Identity{
		Subject: "admin-1", Method: "oauth2", ClientID: "platform-cli", Human: true, PlatformClient: true, CanonicalScopes: core.ScopeWrite,
	})
	if _, err := s.Invite(platform, "tea-1", "teammate@example.com", "developer"); err != nil {
		t.Fatalf("platform-client Invite => %v, want success", err)
	}
	if _, err := s.ChangeRole(ctxWith("admin-1"), "tea-1", "dev-1", "admin"); err != nil {
		t.Fatalf("session ChangeRole => %v, want success", err)
	}
}
