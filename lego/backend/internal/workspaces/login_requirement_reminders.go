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
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/email"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// login_requirement_reminders.go is what an admin sees once a workspace
// requires 2FA or Google login (w1/m647,
// docs/render-artifacts/workspace-2fa-reminders.md): the members who do not
// meet it, and for 2FA an email reminding them, at most one batch per
// workspace per UTC day, as Render's Workspace Settings has. Turning 2FA on
// sends the day's batch itself, as Render's confirmation says it will
// ("Reminders to enable 2FA will be sent to these members."). Google login
// gets the list only: Render has no reminder for it.

// TwoFactorReminderDays records the day of each workspace's last batch of
// reminders (*store.PGStore): a batch claims today, and gives the claim back
// when it sent nothing.
type TwoFactorReminderDays interface {
	ClaimTwoFactorReminderDay(ctx context.Context, workspaceID string, day time.Time) (bool, error)
	ReleaseTwoFactorReminderDay(ctx context.Context, workspaceID string, day time.Time) error
}

// Mailer delivers one email with a plain-text body and an HTML body
// (*mailer.SMTP).
type Mailer interface {
	Send(ctx context.Context, to, subject, text, html string) error
}

// NonCompliantMember is a member who does not meet a login requirement: no
// second factor, or no Google account of their email. UserID is the opaque
// own- id, never the identity subject.
type NonCompliantMember struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}

// nonCompliant is a NonCompliantMember with the subject a reminder's audit row
// targets.
type nonCompliant struct {
	NonCompliantMember
	subject string
}

// NonCompliantMembers lists the members of workspaceID who do not meet
// requirement right now, each judged by the enforcement gate's own check of
// that requirement alone (Render's "Members without two-factor
// authentication" and "Members without Google sign-on"), in the workspace's
// member order. People only: an API key's binding is no member. Admin only,
// like the setters; a read, so only denials are recorded. The default
// workspace answers empty. A lookup that cannot answer fails the read with
// core.ErrAuthzUnavailable, never an empty list.
func (s *Service) NonCompliantMembers(ctx context.Context, workspaceID, requirement string) ([]NonCompliantMember, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	id, err := s.AuthorizeWorkspace(ctx, core.RelCanManage, workspaceID)
	if err != nil {
		return nil, err
	}
	required, err := parseLoginRequirement(requirement)
	if err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrWorkspacesUnavailable
	}
	if id == core.DefaultTenant {
		return []NonCompliantMember{}, nil
	}
	members, err := s.nonCompliantMembers(ctx, id, required)
	if err != nil {
		return nil, err
	}
	out := make([]NonCompliantMember, len(members))
	for i, m := range members {
		out[i] = m.NonCompliantMember
	}
	return out, nil
}

// SendTwoFactorReminders emails each member of workspaceID who has not enabled
// 2FA a reminder that the workspace requires it, and answers how many were
// sent: Render's "Send reminders". Admin only, and only where the workspace
// requires 2FA, itself or through its org (409 WORKSPACE_2FA_NOT_REQUIRED).
// One batch per workspace per UTC day, the batch turning the requirement on
// included: a second is refused with 429 WORKSPACE_2FA_REMINDER_LIMIT and
// sends nothing. Each reminder sent is one audit row naming its member. With
// nobody to remind it sends nothing and leaves the day unclaimed.
func (s *Service) SendTwoFactorReminders(ctx context.Context, workspaceID string) (int, error) {
	ctx = core.WithDeferredAllowedWriteAudit(ctx)
	id, err := s.AuthorizeWorkspace(ctx, core.RelCanManage, workspaceID)
	if err != nil {
		return 0, err
	}
	if s.Store == nil {
		return 0, core.ErrWorkspacesUnavailable
	}
	if id == core.DefaultTenant {
		return 0, core.NewWorkspaceTwoFactorNotRequiredError(id)
	}
	t, err := s.Store.GetTenant(ctx, id)
	if err != nil {
		return 0, mapStoreErr(err)
	}
	// The rows, not the gate's cached copy: another replica's admin may have
	// turned it on a moment ago.
	required := t.RequireTwoFactor
	if !required {
		o, err := s.workspaceOrganization(ctx, view(t, ""))
		if err != nil {
			return 0, mapStoreErr(err)
		}
		required = o != nil && o.RequireTwoFactor
	}
	if !required {
		return 0, core.NewWorkspaceTwoFactorNotRequiredError(id)
	}
	return s.sendTwoFactorReminders(ctx, t)
}

// remindOnEnforce sends the day's batch once turning 2FA on in t has landed.
// The requirement is on whatever happens here: a failure, or a batch an admin
// already sent today, is logged, never returned. A deployment that sends no
// email sends none.
func (s *Service) remindOnEnforce(ctx context.Context, t store.Tenant) {
	if s.Mailer == nil || s.Reminders == nil || s.LoginAssessor == nil {
		return
	}
	if _, err := s.sendTwoFactorReminders(ctx, t); err != nil {
		log.Printf("workspaces: reminders on requiring 2FA in %s: %v", t.ID, err)
	}
}

// sendTwoFactorReminders is one batch in t: it claims today, emails each
// member without 2FA who has an email, and records a row per reminder sent.
// A batch that delivered nothing gives the day back.
func (s *Service) sendTwoFactorReminders(ctx context.Context, t store.Tenant) (int, error) {
	if s.Mailer == nil || s.Reminders == nil {
		return 0, core.NewWorkspaceTwoFactorRemindersUnavailableError("two-factor reminders are unavailable: this deployment does not send email")
	}
	members, err := s.nonCompliantMembers(ctx, t.ID, core.LoginRequirements{TwoFactor: true})
	if err != nil {
		return 0, err
	}
	members = slices.DeleteFunc(members, func(m nonCompliant) bool { return m.Email == "" })
	if len(members) == 0 {
		return 0, nil
	}
	now := s.Now().UTC()
	claimed, err := s.Reminders.ClaimTwoFactorReminderDay(ctx, t.ID, now)
	if err != nil {
		log.Printf("workspaces: claiming today's 2FA reminders in %s: %v", t.ID, err)
		return 0, core.ErrWorkspacesUnavailable
	}
	if !claimed {
		return 0, core.NewWorkspaceTwoFactorReminderLimitError(t.ID, nextUTCDay(now))
	}
	subject, msg := twoFactorReminder(t, s.DashboardURL)
	text, html := msg.Text(), msg.HTML()
	sent := 0
	for _, m := range members {
		if err := s.Mailer.Send(ctx, m.Email, subject, text, html); err != nil {
			log.Printf("workspaces: 2FA reminder in %s to user %s: %v", t.ID, m.UserID, err)
			continue
		}
		sent++
		s.RecordTwoFactorReminderSent(ctx, t.ID, m.subject, m.Email)
	}
	if sent == 0 {
		if err := s.Reminders.ReleaseTwoFactorReminderDay(context.WithoutCancel(ctx), t.ID, now); err != nil {
			log.Printf("workspaces: releasing today's 2FA reminders in %s: %v", t.ID, err)
		}
		return 0, core.NewWorkspaceTwoFactorRemindersUnavailableError("no reminder could be sent; try again")
	}
	return sent, nil
}

// nextUTCDay is the start of the UTC day after now: when the next batch may go.
func nextUTCDay(now time.Time) time.Time {
	y, m, d := now.UTC().Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, time.UTC)
}

// nonCompliantMembers assesses each person in workspaceID against required
// alone, by the gate's own check, and returns those who fail it in member
// order.
func (s *Service) nonCompliantMembers(ctx context.Context, workspaceID string, required core.LoginRequirements) ([]nonCompliant, error) {
	if s.LoginAssessor == nil {
		return nil, core.ErrAuthzUnavailable
	}
	members, err := s.Store.ListTenantMembers(ctx, workspaceID)
	if err != nil {
		return nil, impactUnavailable("listing the members", err)
	}
	var people []string
	for _, m := range members {
		if m.Kind != store.MemberKindMachine {
			people = append(people, m.Subject)
		}
	}
	fails := make([]bool, len(people))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(impactLookupLimit)
	for i, p := range people {
		g.Go(func() error {
			ok, err := s.LoginAssessor.AssessMember(gctx, required, p)
			fails[i] = err == nil && !ok
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err // the gate's own ErrAuthzUnavailable, already logged
	}
	var listed []string
	for i, p := range people {
		if fails[i] {
			listed = append(listed, p)
		}
	}
	out := make([]nonCompliant, 0, len(listed))
	if len(listed) == 0 {
		return out, nil
	}
	userIDs, err := s.Store.OwnerIDsForSubjects(ctx, listed)
	if err != nil {
		return nil, impactUnavailable("reading user ids", err)
	}
	var attrs map[string]IdentityAttrs
	if s.Identities != nil {
		attrs = s.Identities.LookupMany(ctx, listed)
	}
	for _, p := range listed {
		out = append(out, nonCompliant{
			NonCompliantMember: NonCompliantMember{UserID: userIDs[p], Email: attrs[p].Email, Name: attrs[p].Name},
			subject:            p,
		})
	}
	return out, nil
}

// twoFactorReminder is the reminder's subject and body (w1/m647 t003): which
// workspace, that the member cannot use it until they enable 2FA, and the way
// to account security. Product emails are English, as the invite is.
func twoFactorReminder(t store.Tenant, dashboardURL string) (string, email.Message) {
	name := t.Label()
	subject := fmt.Sprintf("Enable two-factor authentication to access the %q workspace on bex", name)
	m := email.Message{
		Title: "Two-factor authentication required",
		Paragraphs: []string{
			fmt.Sprintf("The %q workspace on bex requires two-factor authentication (2FA). You can't access the workspace or its resources until you enable it for your account.", name),
			"Until then, your API keys and SSH keys can't be used with the workspace either.",
		},
	}
	if base := strings.TrimSuffix(dashboardURL, "/"); base != "" {
		m.CTA = &email.CTA{
			Lead:  "Enable two-factor authentication in your account settings",
			Label: "Enable two-factor authentication",
			URL:   base + "/settings#security",
		}
	} else {
		m.Paragraphs = append(m.Paragraphs, "Enable it under Security in your bex account settings.")
	}
	return subject, m
}
