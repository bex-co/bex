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

package notifications

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/email"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Agent automation emails (w9/m343, ADR101 D7). Devin Automations offer an
// Email notification action sent Always, On failure or On success, and send
// failure notifications on a backoff (after the 1st, 3rd, 9th... consecutive
// failure, resuming on the next success). Each automation's notify_email
// picks the outcomes; a platform auto-pause is always mailed. Notices are
// claimed before they are sent, so each is mailed at most once and never
// retried, as the build-minutes notices are. A notice older than
// automationEmailMaxAge when claimed (the worker was down, or mail was not
// configured yet) is dropped rather than mailed late.
const (
	automationEmailPollInterval = 15 * time.Second
	automationEmailBatch        = 50
	automationEmailMaxAge       = 24 * time.Hour
)

// AutomationNoticeStore is the durable seam; *store.PGStore satisfies it.
type AutomationNoticeStore interface {
	ClaimAutomationRunNotices(ctx context.Context, now time.Time, limit int) ([]store.AutomationRunNotice, error)
	ClaimAutomationPauseNotices(ctx context.Context, now time.Time, limit int) ([]store.AgentAutomation, error)
}

// AutomationEmailWorker mails an automation's creator about its runs and its
// platform pauses. Without SMTP or an identity lookup it still claims notices,
// so the backlog cannot build up and be mailed in bulk once mail is configured.
type AutomationEmailWorker struct {
	Store            AutomationNoticeStore
	Mailer           Mailer
	Identities       EmailLookup
	DashboardBaseURL string
	Clock            func() time.Time
	PollInterval     time.Duration
}

// automationMail is one notice due to its automation's creator.
type automationMail struct {
	automation store.AgentAutomation
	kind       string
	subject    string
	msg        email.Message
}

// Run polls until cancellation.
func (w *AutomationEmailWorker) Run(ctx context.Context) {
	interval := w.PollInterval
	if interval <= 0 {
		interval = automationEmailPollInterval
	}
	core.Poll(ctx, "notifications: automation email worker", interval, w.RunOnce)
}

func (w *AutomationEmailWorker) now() time.Time {
	if w.Clock != nil {
		return w.Clock()
	}
	return time.Now()
}

// RunOnce claims one batch of run notices and one of pause notices and mails
// the ones due.
func (w *AutomationEmailWorker) RunOnce(ctx context.Context) error {
	if w.Store == nil {
		return errors.New("automation email worker store is required")
	}
	now := w.now()
	runs, err := w.Store.ClaimAutomationRunNotices(ctx, now, automationEmailBatch)
	if err != nil {
		return fmt.Errorf("claim automation run notices: %w", err)
	}
	links := resourcemeta.Config{DashboardBaseURL: w.DashboardBaseURL}
	var due []automationMail
	for _, n := range runs {
		if now.Sub(n.EndedAt) > automationEmailMaxAge ||
			!automationRunMailDue(n.Automation.NotifyEmail, n.Result, n.Automation.NotifyFailureStreak) {
			continue
		}
		subject, msg := automationRunEmail(n, links)
		due = append(due, automationMail{n.Automation, "run " + n.Result, subject, msg})
	}
	pauses, err := w.Store.ClaimAutomationPauseNotices(ctx, now, automationEmailBatch)
	if err != nil {
		return fmt.Errorf("claim automation pause notices: %w", err)
	}
	for _, a := range pauses {
		if a.PausedAt != nil && now.Sub(*a.PausedAt) > automationEmailMaxAge {
			continue
		}
		subject, msg := automationPauseEmail(a, links)
		due = append(due, automationMail{a, "pause " + a.PausedReason, subject, msg})
	}
	w.mailCreators(ctx, due)
	return nil
}

// mailCreators sends each notice to its automation's creator, resolving every
// creator's address in one lookup. Notices are already claimed, so a failure
// is logged, not retried; log lines never carry the address.
func (w *AutomationEmailWorker) mailCreators(ctx context.Context, due []automationMail) {
	if len(due) == 0 {
		return
	}
	if w.Mailer == nil || w.Identities == nil {
		log.Printf("notifications: %d automation notices not emailed (SMTP or identity lookup unavailable)", len(due))
		return
	}
	subjects := make([]string, 0, len(due))
	for _, m := range due {
		if !slices.Contains(subjects, m.automation.CreatedBy) {
			subjects = append(subjects, m.automation.CreatedBy)
		}
	}
	emails, lookupErr := w.Identities.LookupEmails(ctx, subjects)
	for _, m := range due {
		a := m.automation
		addr := emails[a.CreatedBy]
		if addr == "" {
			log.Printf("notifications: automation %s %s notice not emailed (creator address unresolved: %v)", a.ID, m.kind, lookupErr)
			continue
		}
		if err := w.Mailer.Send(ctx, addr, m.subject, m.msg.Text(), m.msg.HTML()); err != nil {
			if permanentRejection(err) {
				log.Printf("notifications: automation %s %s notice not emailed (address rejected: %v)", a.ID, m.kind, err)
			} else {
				log.Printf("notifications: automation %s %s notice not emailed: %v", a.ID, m.kind, err)
			}
			continue
		}
		log.Printf("notifications: automation %s %s notice emailed to its creator", a.ID, m.kind)
	}
}

// automationRunMailDue applies the email setting to one run's result. A
// failure is mailed only when the consecutive-failure streak (counting this
// run) is a power of three: the 1st, 3rd, 9th, 27th... A canceled run is
// mailed only under always.
func automationRunMailDue(setting, result string, failureStreak int) bool {
	switch result {
	case store.AutomationRunResultSucceeded:
		return setting == store.AutomationNotifyAlways || setting == store.AutomationNotifyOnSuccess
	case store.AutomationRunResultFailed:
		return (setting == store.AutomationNotifyAlways || setting == store.AutomationNotifyOnFailure) &&
			failureBackoffDue(failureStreak)
	default:
		return setting == store.AutomationNotifyAlways
	}
}

func failureBackoffDue(streak int) bool {
	if streak < 1 {
		return false
	}
	for streak%3 == 0 {
		streak /= 3
	}
	return streak == 1
}

// Dashboard routes the notices link to.
const (
	automationsDashboardRoute   = "automations"
	agentSessionsDashboardRoute = "agents"
)

func automationRunEmail(n store.AutomationRunNotice, links resourcemeta.Config) (string, email.Message) {
	a, run := n.Automation, n.Run
	fired := run.FireAt.UTC().Format("2006-01-02 15:04 UTC")
	var title, paragraph string
	switch n.Result {
	case store.AutomationRunResultSucceeded:
		title = fmt.Sprintf("Automation %s finished its run", a.Name)
		paragraph = fmt.Sprintf("The run of automation %s that fired at %s finished: its agent session completed.", a.Name, fired)
	case store.AutomationRunResultCanceled:
		title = fmt.Sprintf("Automation %s's run was canceled", a.Name)
		paragraph = fmt.Sprintf("The agent session started by automation %s at %s was canceled before it finished.", a.Name, fired)
	default:
		title = fmt.Sprintf("Automation %s's run failed", a.Name)
		switch run.Outcome {
		case store.AutomationRunDenied:
			paragraph = fmt.Sprintf("Automation %s could not run at %s: its creator can no longer start agent sessions in this workspace.", a.Name, fired)
		case store.AutomationRunFailedCreate:
			paragraph = fmt.Sprintf("Automation %s could not start its agent session at %s: %s", a.Name, fired, runErrorText(run.Error))
		default:
			paragraph = fmt.Sprintf("The agent session started by automation %s at %s failed: %s", a.Name, fired, runErrorText(n.SessionFailureReason))
		}
	}
	msg := email.Message{Title: title, Paragraphs: []string{paragraph}}
	if n.Result == store.AutomationRunResultFailed && a.NotifyFailureStreak > 1 {
		msg.Paragraphs = append(msg.Paragraphs, fmt.Sprintf(
			"It has failed %d runs in a row. While it keeps failing, bex emails after the 1st, 3rd and 9th failure and so on, and again after it next succeeds.",
			a.NotifyFailureStreak))
	}
	if url := links.DashboardURL(agentSessionsDashboardRoute, run.SessionID); url != "" {
		msg.CTA = &email.CTA{Lead: "See what the agent did", Label: "Open the session", URL: url}
	} else if url := links.DashboardURL(automationsDashboardRoute, a.ID); url != "" {
		msg.CTA = &email.CTA{Lead: "See the automation's runs", Label: "Open the automation", URL: url}
	}
	msg.Footer = []string{"You get this email because of the automation's email notification setting. Change it on the automation's page."}
	return "[bex] " + title, msg
}

func runErrorText(text string) string {
	if text == "" {
		return "no reason was recorded."
	}
	return text
}

func automationPauseEmail(a store.AgentAutomation, links resourcemeta.Config) (string, email.Message) {
	title := fmt.Sprintf("Automation %s was paused", a.Name)
	var why string
	switch a.PausedReason {
	case store.AutomationPausedRepeatedFailures:
		why = "three runs in a row could not start an agent session"
	case store.AutomationPausedCreatorLostAccess:
		why = "its creator can no longer start agent sessions in this workspace; a workspace admin can take it over"
	case store.AutomationPausedTemplateInvalid:
		why = "its saved schedule or session template no longer validates; edit it before resuming"
	default:
		why = "the platform stopped it"
	}
	msg := email.Message{Title: title, Paragraphs: []string{
		fmt.Sprintf("bex paused automation %s because %s.", a.Name, why),
		"It will not run again until it is resumed.",
	}}
	if url := links.DashboardURL(automationsDashboardRoute, a.ID); url != "" {
		msg.CTA = &email.CTA{Lead: "See the reason and resume it", Label: "Open the automation", URL: url}
	}
	return "[bex] " + title, msg
}
