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

// Package accounts owns irreversible self-deletion. It deliberately exposes
// REST and GraphQL but no MCP registration.
package accounts

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const Confirmation = "delete my account"

const (
	CodeBlocked     = "ACCOUNT_DELETION_BLOCKED"
	CodeUnavailable = "ACCOUNT_DELETION_UNAVAILABLE"
)

type Store interface {
	PreviewAccountDeletion(context.Context, string, []string) ([]store.AccountWorkspaceDisposition, error)
	BeginAccountDeletion(context.Context, string, string, []string) (store.AccountDeletion, error)
	ClaimAccountDeletions(context.Context, int) ([]store.AccountDeletion, error)
	AdvanceAccountDeletion(context.Context, string, string, string) error
	FailAccountDeletion(context.Context, string, string) error
	CleanupAccountSubject(context.Context, string, string) error
}

type WorkspaceTeardown interface {
	Delete(context.Context, string) error
}

type MemberOffboarder interface {
	Remove(context.Context, string, string) error
}

type MachineCredentials interface {
	List(context.Context) ([]apikeys.APIKey, error)
	CleanupSubject(context.Context, string) error
}

type SubjectCleaner interface {
	CleanupSubject(context.Context, string) error
}

type IdentityCleaner interface {
	DeleteSessions(context.Context, string) error
	DeleteIdentity(context.Context, string) error
}

type Preview struct {
	Delete  []store.AccountWorkspaceDisposition `json:"delete"`
	Leave   []store.AccountWorkspaceDisposition `json:"leave"`
	Blocked []store.AccountWorkspaceDisposition `json:"blocked"`
}

type DeletionView struct {
	State string `json:"state"`
}

type Service struct {
	*core.Base
	Store      Store
	Workspaces WorkspaceTeardown
	Members    MemberOffboarder
	APIKeys    MachineCredentials
	OAuth      SubjectCleaner
	Kratos     IdentityCleaner
	// Settler charges each workspace the deletion will delete before the
	// intent commits (ADR091 D10.4): a declined charge refuses the request
	// with BILLING_BALANCE_DUE while the account can still act on it. The
	// worker's teardown settles again, for usage in between, unless the
	// intent is younger than settledReuseWindow. Nil (Stripe disabled) skips
	// it.
	Settler WorkspaceSettler
	// Mailer and Emails send the hosted invoice link to the account's email
	// when the worker's teardown is refused for an unpaid balance: by then
	// the account can no longer sign in. Either nil logs instead.
	Mailer Mailer
	Emails EmailLookup
}

// Mailer sends one email; mailer.SMTP satisfies it.
type Mailer interface {
	Send(ctx context.Context, to, subject, text, html string) error
}

// EmailLookup resolves subjects' emails from the identity provider, which
// still holds the identity until the deletion's last step.
type EmailLookup interface {
	LookupEmails(ctx context.Context, subjects []string) (map[string]string, error)
}

const (
	// requestSettleTimeout bounds the synchronous settle in Request inside
	// bex-api's 30-second HTTP write timeout; the settle answers the
	// retryable BILLING_SETTLING before it rather than timing out.
	requestSettleTimeout = 25 * time.Second
	// settledReuseWindow is how long after the request's own settle the
	// worker's teardown skips settling again.
	settledReuseWindow = 5 * time.Minute
)

// WorkspaceSettler settles a workspace's balance; *billing.Settler satisfies
// it.
type WorkspaceSettler interface {
	SettleWorkspace(ctx context.Context, workspaceID string) error
}

func directSession(ctx context.Context) (core.Identity, error) {
	id, ok := core.IdentityFrom(ctx)
	if !ok || id.Subject == "" || id.Method != "session" || !id.Human {
		return core.Identity{}, core.NewForbiddenError(
			"ACCOUNT_DELETION_SESSION_REQUIRED",
			"account deletion requires a direct signed-in browser session",
			nil,
		)
	}
	return id, nil
}

func (s *Service) Preview(ctx context.Context) (Preview, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return Preview{}, err
	}
	id, err := directSession(ctx)
	if err != nil {
		return Preview{}, err
	}
	if s.Store == nil {
		return Preview{}, core.NewUnavailableError(CodeUnavailable, "account deletion is not configured", nil)
	}
	preview, _, err := s.previewForSubject(ctx, id.Subject)
	return preview, err
}

func (s *Service) previewForSubject(ctx context.Context, subject string) (Preview, []string, error) {
	if s.APIKeys == nil {
		return Preview{}, nil, core.NewUnavailableError(CodeUnavailable, "API-key inventory is not configured", nil)
	}
	keys, err := s.APIKeys.List(ctx)
	if err != nil {
		return Preview{}, nil, err
	}
	machineSubjects := make([]string, 0, len(keys))
	for _, key := range keys {
		machineSubjects = append(machineSubjects, key.ID)
	}
	rows, err := s.Store.PreviewAccountDeletion(ctx, subject, machineSubjects)
	if err != nil {
		return Preview{}, nil, err
	}
	// The three buckets start empty-but-non-nil: every one of them is a declared
	// array on the wire, and a `var out Preview` leaves the unused ones nil,
	// which encoding/json prints as `null` (w4/m116/t006). A preview with
	// nothing to delete is the COMMON case, not an edge one.
	out := Preview{
		Delete:  []store.AccountWorkspaceDisposition{},
		Leave:   []store.AccountWorkspaceDisposition{},
		Blocked: []store.AccountWorkspaceDisposition{},
	}
	for _, row := range rows {
		switch row.Action {
		case store.AccountWorkspaceDelete:
			out.Delete = append(out.Delete, row)
		case store.AccountWorkspaceLeave:
			out.Leave = append(out.Leave, row)
		case store.AccountWorkspaceBlocked:
			out.Blocked = append(out.Blocked, row)
		}
	}
	return out, machineSubjects, nil
}

func blockedError(rows []store.AccountWorkspaceDisposition) error {
	workspaces := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		workspaces = append(workspaces, map[string]string{"id": row.ID, "name": row.Name})
	}
	return core.NewConflictError(CodeBlocked,
		"Promote another workspace member to admin, remove the other members, or delete the blocking workspace first.",
		map[string]any{"workspaces": workspaces})
}

func (s *Service) Request(ctx context.Context, confirmation string) (DeletionView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return DeletionView{}, err
	}
	id, err := directSession(ctx)
	if err != nil {
		return DeletionView{}, err
	}
	if confirmation != Confirmation {
		return DeletionView{}, core.NewBadRequestError("ACCOUNT_DELETION_CONFIRMATION",
			fmt.Sprintf("confirmation must be %q", Confirmation), nil)
	}
	if s.Store == nil {
		return DeletionView{}, core.NewUnavailableError(CodeUnavailable, "account deletion is not configured", nil)
	}
	preview, machineSubjects, err := s.previewForSubject(ctx, id.Subject)
	if err != nil {
		return DeletionView{}, err
	}
	if len(preview.Blocked) > 0 {
		return DeletionView{}, blockedError(preview.Blocked)
	}
	if err := s.settle(ctx, preview.Delete); err != nil {
		return DeletionView{}, err
	}
	d, err := s.Store.BeginAccountDeletion(ctx, id.Subject, id.Email, machineSubjects)
	var blocked *store.AccountDeletionBlockedError
	if errors.As(err, &blocked) {
		return DeletionView{}, blockedError(blocked.Workspaces)
	}
	if err != nil {
		return DeletionView{}, err
	}
	return DeletionView{State: d.State}, nil
}

// settle charges each workspace the deletion will delete, within
// requestSettleTimeout.
func (s *Service) settle(ctx context.Context, workspaces []store.AccountWorkspaceDisposition) error {
	if s.Settler == nil || len(workspaces) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, requestSettleTimeout)
	defer cancel()
	for _, workspace := range workspaces {
		if err := s.Settler.SettleWorkspace(ctx, workspace.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) process(ctx context.Context, d store.AccountDeletion) error {
	for {
		switch d.State {
		case store.AccountDeletionPending:
			if s.APIKeys == nil {
				return core.NewUnavailableError(CodeUnavailable, "API-key cleanup is not configured", nil)
			}
			if err := s.APIKeys.CleanupSubject(ctx, d.Subject); err != nil {
				return err
			}
			if s.OAuth == nil {
				return core.NewUnavailableError(CodeUnavailable, "OAuth cleanup is not configured", nil)
			}
			if err := s.OAuth.CleanupSubject(ctx, d.Subject); err != nil {
				return err
			}
			if err := s.Store.AdvanceAccountDeletion(ctx, d.Subject, d.State, store.AccountDeletionCleaning); err != nil {
				return err
			}
			d.State = store.AccountDeletionCleaning
		case store.AccountDeletionCleaning:
			if s.Workspaces == nil || s.Members == nil {
				return core.NewUnavailableError(CodeUnavailable, "workspace cleanup is not configured", nil)
			}
			// Request settled every deleted workspace just before the intent
			// committed; a teardown moments later does not charge again.
			teardownCtx := ctx
			if !d.CreatedAt.IsZero() && s.now().Sub(d.CreatedAt) < settledReuseWindow {
				teardownCtx = core.WithSettledBalance(ctx)
			}
			for _, workspace := range d.Workspaces {
				switch workspace.Action {
				case store.AccountWorkspaceDelete:
					if err := s.Workspaces.Delete(teardownCtx, workspace.ID); err != nil {
						return err
					}
				case store.AccountWorkspaceLeave:
					if err := s.Members.Remove(ctx, workspace.ID, d.Subject); err != nil {
						return err
					}
				}
			}
			if err := s.Store.CleanupAccountSubject(ctx, d.Subject, d.DeletedMarker); err != nil {
				return err
			}
			if err := s.Store.AdvanceAccountDeletion(ctx, d.Subject, d.State, store.AccountDeletionIdentity); err != nil {
				return err
			}
			d.State = store.AccountDeletionIdentity
		case store.AccountDeletionIdentity:
			if s.Kratos == nil {
				return core.NewUnavailableError(CodeUnavailable, "identity cleanup is not configured", nil)
			}
			if err := s.Kratos.DeleteSessions(ctx, d.Subject); err != nil {
				return err
			}
			if err := s.Kratos.DeleteIdentity(ctx, d.Subject); err != nil {
				return err
			}
			return s.Store.AdvanceAccountDeletion(ctx, d.Subject, d.State, store.AccountDeletionDone)
		case store.AccountDeletionDone:
			return nil
		default:
			return fmt.Errorf("unknown account deletion state %q", d.State)
		}
	}
}

func (s *Service) processPending(ctx context.Context, limit int) error {
	if s.Store == nil {
		return nil
	}
	rows, err := s.Store.ClaimAccountDeletions(ctx, limit)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := s.process(ctx, row); err != nil {
			_ = s.Store.FailAccountDeletion(ctx, row.Subject, s.failure(ctx, row, err))
			if errors.Is(err, context.Canceled) {
				return err
			}
		}
	}
	return nil
}

// failure is the bounded last_error of a failed worker pass. A billing
// refusal is stored as its code only (never the payment link), and the
// first BILLING_BALANCE_DUE of a deletion emails the hosted invoice link to
// the account, which can no longer sign in to see it.
func (s *Service) failure(ctx context.Context, d store.AccountDeletion, err error) string {
	var coded *core.CodedError
	if !errors.As(err, &coded) || (coded.Code != core.CodeBillingBalanceDue && coded.Code != core.CodeBillingSettling) {
		return err.Error()
	}
	if coded.Code == core.CodeBillingBalanceDue && d.LastError != core.CodeBillingBalanceDue {
		s.notifyBalanceDue(ctx, d.Subject, coded)
	}
	return coded.Code
}

// notifyBalanceDue emails the hosted invoice link of a refused teardown.
// Best effort: a failure is logged, and the deletion retries regardless.
func (s *Service) notifyBalanceDue(ctx context.Context, subject string, refusal *core.CodedError) {
	link, _ := refusal.Params["hostedInvoiceUrl"].(string)
	if s.Mailer == nil || s.Emails == nil || link == "" {
		log.Printf("accounts: deletion of %s is waiting for an unpaid balance; no email sent", subject)
		return
	}
	emails, err := s.Emails.LookupEmails(ctx, []string{subject})
	to := emails[subject]
	if err != nil || to == "" {
		log.Printf("accounts: deletion of %s is waiting for an unpaid balance; email lookup failed", subject)
		return
	}
	amount := ""
	if cents, ok := refusal.Params["amountDueCents"].(int64); ok {
		currency, _ := refusal.Params["currency"].(string)
		amount = fmt.Sprintf(" of %d.%02d %s", cents/100, cents%100, strings.ToUpper(currency))
	}
	text := "Your bex account deletion is paused: a workspace has an unpaid balance" + amount + ".\n\n" +
		"Pay the invoice here, and the deletion finishes automatically:\n" + link + "\n"
	body := "<p>Your bex account deletion is paused: a workspace has an unpaid balance" + html.EscapeString(amount) + ".</p>" +
		`<p><a href="` + html.EscapeString(link) + `">Pay the invoice</a>, and the deletion finishes automatically.</p>`
	if err := s.Mailer.Send(ctx, to, "Pay your balance to finish deleting your bex account", text, body); err != nil {
		log.Printf("accounts: send balance-due notice for the deletion of %s: %v", subject, err)
	}
}

func (s *Service) now() time.Time {
	if s.Base != nil {
		return s.Base.Now()
	}
	return time.Now()
}

func (s *Service) Run(ctx context.Context) {
	if s.Store == nil {
		return
	}
	core.Poll(ctx, "accounts: deletion sweep", 5*time.Second,
		func(ctx context.Context) error { return s.processPending(ctx, 1) })
}
