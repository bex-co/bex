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
	"fmt"
	"log"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/email"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// email.go composes the invite email. Delivery is best-effort (a flaky relay
// must not make an invite look failed); acceptance is by email match on the
// recipient's first login, so the mail is a notification, not the mechanism.

// sendInvite emails the invite, if a Mailer is wired. A send failure is logged,
// not returned — the invite row already exists and redeems on login regardless.
func (s *Service) sendInvite(ctx context.Context, inv store.Invite, tenant store.Tenant) {
	if s.Mailer == nil {
		return
	}
	subject := fmt.Sprintf("You've been invited to the %q workspace on bex", tenant.Name)
	msg := s.inviteMessage(inv, tenant)
	if err := s.Mailer.Send(ctx, inv.Email, subject, msg.Text(), msg.HTML()); err != nil {
		log.Printf("members: sending invite %s to %s: %v", inv.ID, inv.Email, err)
	}
}

// inviteMessage composes the invite as an email.Message. It names the workspace
// and role, and — when an InviteBaseURL is configured — offers a dashboard
// sign-up/log-in call to action. The token in the link is redeemable directly
// (AcceptInvite, w1/m33), so the invite works even when the recipient signs up
// under a different email; email-match acceptance on login remains the linkless
// fallback. Message.Text() reproduces the exact bytes of the pre-w1/m54
// plain-text body (pinned by TestInviteMessageTextByteParity).
func (s *Service) inviteMessage(inv store.Invite, tenant store.Tenant) email.Message {
	m := email.Message{
		Title: "Workspace invitation",
		Paragraphs: []string{
			fmt.Sprintf("You've been invited to join the %q workspace on bex as a %s.",
				tenant.Name, strings.ToLower(inv.Role)),
		},
		Footer: []string{
			fmt.Sprintf("This invitation expires on %s.",
				inv.ExpiresAt.UTC().Format("2006-01-02 15:04 UTC")),
		},
	}
	if base := strings.TrimSuffix(s.InviteBaseURL, "/"); base != "" {
		m.CTA = &email.CTA{
			Lead:  fmt.Sprintf("Sign up or log in with %s to accept", inv.Email),
			Label: "Accept invitation",
			URL:   fmt.Sprintf("%s/auth/sign-up?invite=%s", base, inv.Token),
		}
	} else {
		m.Paragraphs = append(m.Paragraphs,
			fmt.Sprintf("Sign up or log in with %s to accept the invitation.", inv.Email))
	}
	return m
}
