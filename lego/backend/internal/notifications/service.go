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

// Package notifications is the deploy-notification feature (w3/m9, w3/005):
// email a workspace's members when one of its services' deploys starts,
// succeeds, or fails,
// matching Render's /notification-settings surface. It has three seams: the
// caller-facing GetSettings/UpdateSettings verbs (REST/GraphQL/MCP, this
// package's own Service methods, each authorized like any other verb),
// NotifyDeployStarted for the request-time trigger paths, and NotifyDeploy,
// which the control-plane reconciler calls directly (not a caller verb — see
// internal/store.DeployNotifier) the instant it closes a deploy row. Delivery
// reuses the w4/m12 invite-email pattern: best-effort, logged not returned,
// over the shared mailer.SMTP relay.
package notifications

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/email"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// NotificationsStore is the slice of the control-plane store this feature
// reads/writes through. *store.PGStore satisfies it structurally.
type NotificationsStore interface {
	GetNotificationSettings(ctx context.Context, tenantID, subject string) (store.NotificationSettings, error)
	UpsertNotificationSettings(ctx context.Context, tenantID, subject string, deployStarted, deploySucceeded, deployFailed bool) (store.NotificationSettings, error)
	ListNotifyRecipients(ctx context.Context, tenantID string) ([]store.NotifyRecipient, error)
}

// Mailer delivers an email with a plain-text body and an optional HTML
// alternative — the same seam members.Service uses, kept as this package's own
// interface (rather than importing members') so feature packages stay
// independent; mailer.SMTP satisfies both structurally.
type Mailer interface {
	Send(ctx context.Context, to, subject, text, html string) error
}

// EmailLookup resolves a subject's email from the identity provider — the
// same seam members.Service.Identities uses, kept as this package's own
// interface for the same independence reason. The composition root's
// identityEmailLookup adapter satisfies both.
type EmailLookup interface {
	LookupEmail(ctx context.Context, subject string) (string, bool)
}

type billingOwnerStore interface {
	ListBillingOwnerSubjects(context.Context, string) ([]string, error)
}

// Service is the notifications feature. Store nil (BEX_CP_DB_URI unset)
// leaves the settings verbs answering core.ErrNotificationsUnavailable and
// NotifyDeploy a no-op. Mailer/Identities nil leaves NotifyDeploy a no-op too
// (nothing to send, nowhere to resolve an address) — the same degrade-quietly
// shape members.Service uses for invite delivery.
type Service struct {
	*core.Base
	Store      NotificationsStore
	Mailer     Mailer
	Identities EmailLookup
	// DashboardBaseURL (BEX_DASHBOARD_URL) builds the "View Logs" deep link in
	// the deploy email (w7/m44); empty ⇒ the link is omitted, honest-omit like
	// the invite email's link.
	DashboardBaseURL string
}

// BillingNotifier is the asynchronous m52 lifecycle sink. It is deliberately
// separate from the caller-facing Service so auth/audit verb sweeps cannot
// mistake an internal worker callback for a public API verb.
type BillingNotifier struct{ Service *Service }

func (n BillingNotifier) NotifyBilling(ctx context.Context, notice store.BillingNotification) error {
	if n.Service == nil {
		return fmt.Errorf("billing notifier unavailable")
	}
	return n.Service.notifyBilling(ctx, notice)
}

// notifyBilling sends the lifecycle notice. Billing notices are
// mandatory operational messages to workspace admins, independent of deploy
// notification preferences. The durable billing worker owns deduplication and
// retry; this method returns delivery errors but never runs on a webhook path.
func (s *Service) notifyBilling(ctx context.Context, n store.BillingNotification) error {
	owners, ok := s.Store.(billingOwnerStore)
	if !ok {
		return fmt.Errorf("billing owner store unavailable")
	}
	if s.Mailer == nil || s.Identities == nil {
		log.Printf("notifications: billing %s for %s not emailed (SMTP or identity lookup unavailable)", n.Status, n.WorkspaceID)
		return nil
	}
	subjects, err := owners.ListBillingOwnerSubjects(ctx, n.WorkspaceID)
	if err != nil {
		return err
	}
	subject, msg := billingEmail(n, s.DashboardBaseURL)
	text, html := msg.Text(), msg.HTML()
	var errs []error
	for _, owner := range subjects {
		addr, found := s.Identities.LookupEmail(ctx, owner)
		if !found {
			errs = append(errs, fmt.Errorf("owner %s email unavailable", owner))
			continue
		}
		if err := s.Mailer.Send(ctx, addr, subject, text, html); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func billingEmail(n store.BillingNotification, dashboardBaseURL string) (string, email.Message) {
	mode := "Live Mode"
	if !n.Livemode {
		mode = "Test Mode"
	}
	title, paragraph := "Billing status updated", "Your workspace billing status changed to "+n.Status+"."
	switch n.Status {
	case store.BillingGrace:
		title = "Payment failed — grace period started"
		paragraph = "Stripe could not collect payment. Your workspace is in a grace period."
		if n.GraceDeadline != nil {
			paragraph += " Reversible suspension is scheduled after " + n.GraceDeadline.UTC().Format(time.RFC3339) + "."
		}
	case store.BillingEnforced:
		title = "Workspace compute suspended for billing"
		paragraph = "The payment grace period ended. Workspace compute was suspended without deleting databases, key-value data, or secrets."
	case store.BillingHealthy:
		title = "Billing recovered"
		paragraph = "Payment recovered and resources changed by billing enforcement were restored. Independently suspended resources remain suspended."
	case store.BillingExcluded:
		title = "Workspace excluded from collection"
		paragraph = "An operator excluded this workspace from Stripe collection."
	case store.BillingComped:
		title = "Workspace billing comp applied"
		paragraph = "An operator applied the rated-but-free billing comp."
	}
	msg := email.Message{Title: title, Paragraphs: []string{paragraph}, Footer: []string{"Stripe " + mode + " · no payment details are included in this message."}}
	if u := strings.TrimRight(dashboardBaseURL, "/"); u != "" {
		msg.CTA = &email.CTA{Lead: "Review billing and payment method", Label: "Open Billing", URL: u + "/usage"}
	}
	return "[bex " + mode + "] " + title, msg
}

// SettingsView is the neutral projection of a member's deploy-notification
// preferences — the three deploy lifecycle events bex actually fires on.
type SettingsView struct {
	DeployStarted   bool `json:"deployStarted"`
	DeploySucceeded bool `json:"deploySucceeded"`
	DeployFailed    bool `json:"deployFailed"`
}

// defaultSettings is what a member who never customized their preferences
// gets: failures are actionable; routine start and success messages are quiet.
// The store resolves the same values for members with no explicit row.
var defaultSettings = SettingsView{DeployStarted: false, DeploySucceeded: false, DeployFailed: true}

func toView(n store.NotificationSettings) SettingsView {
	return SettingsView{DeployStarted: n.DeployStarted, DeploySucceeded: n.DeploySucceeded, DeployFailed: n.DeployFailed}
}

// GetSettings returns the CALLER's own deploy-notification preferences within
// their workspace — viewer-and-up, like usage's month-to-date (a personal
// preference, not a workspace-admin concern). Defaults are returned, not an
// error, for a caller who never customized them or has no tenant to key on.
func (s *Service) GetSettings(ctx context.Context) (SettingsView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return SettingsView{}, err
	}
	if s.Store == nil {
		return SettingsView{}, core.ErrNotificationsUnavailable
	}
	tenantID, ok := s.Base.Tenant(ctx)
	if !ok {
		return defaultSettings, nil
	}
	id, ok := core.IdentityFrom(ctx)
	if !ok {
		return defaultSettings, nil
	}
	row, err := s.Store.GetNotificationSettings(ctx, tenantID, id.Subject)
	if errors.Is(err, store.ErrNotFound) {
		return defaultSettings, nil
	}
	if err != nil {
		return SettingsView{}, err
	}
	return toView(row), nil
}

// UpdateSettings writes the CALLER's own deploy-notification preferences.
// Viewer-and-up, same rationale as GetSettings: every member manages their
// own notifications regardless of workspace role.
func (s *Service) UpdateSettings(ctx context.Context, deployStarted, deploySucceeded, deployFailed bool) (SettingsView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return SettingsView{}, err
	}
	if s.Store == nil {
		return SettingsView{}, core.ErrNotificationsUnavailable
	}
	tenantID, ok := s.Base.Tenant(ctx)
	if !ok {
		return SettingsView{}, fmt.Errorf("%w: no workspace to save preferences in", core.ErrBadRequest)
	}
	id, ok := core.IdentityFrom(ctx)
	if !ok {
		return SettingsView{}, core.ErrForbidden
	}
	row, err := s.Store.UpsertNotificationSettings(ctx, tenantID, id.Subject, deployStarted, deploySucceeded, deployFailed)
	if err != nil {
		return SettingsView{}, err
	}
	return toView(row), nil
}

// deployMailKind selects the recipient preference and email copy for one
// lifecycle event. It stays internal so the public notifier seams remain named
// after their distinct write paths (request-time start vs reconcile-time end).
type deployMailKind int

const (
	deployMailStarted deployMailKind = iota
	deployMailSucceeded
	deployMailFailed
)

// NotifyDeployStarted is the request-time seam used immediately after an API,
// deploy-hook, or git-push trigger successfully starts a deploy. It is not the
// reconciler's close-time DeployNotifier: callers invoke it off their hot path,
// and this method applies each member's deployStarted preference.
func (s *Service) NotifyDeployStarted(ctx context.Context, tenantID, appName, notificationsToSend string) {
	// The request-time start path has no deploy row in hand (it fires off the
	// App-CR trigger), so the started email carries the framing but omits the
	// commit + View Logs link — the honest-omit "when available" contract.
	s.notifyDeploy(ctx, tenantID, appName, deployMailStarted, notificationsToSend, deployDetails{})
}

// NotifyDeploy is store.DeployNotifier: called by the reconciler the instant
// it closes a deploy as succeeded or failed, in the same reconcile pass as
// the status write (the milestone's DoD). NOT a caller verb — no Authorize;
// the reconciler is a trusted internal caller, same as CloneSecreter. A
// build/pre-deploy/update failures all use the same member preference; other
// terminal states are silently ignored. n.NotifyOnFail (w4/m21, docs/render-
// artifacts/notify-on-fail.md) governs FAILURE notifications only — a
// succeeded deploy always follows each member's own preference, unmodified.
func (s *Service) NotifyDeploy(ctx context.Context, n store.DeployNotification) {
	var kind deployMailKind
	switch n.Status {
	case store.DeployLive:
		kind = deployMailSucceeded
	case store.DeployBuildFailed, store.DeployPreDeployFailed, store.DeployUpdateFailed:
		kind = deployMailFailed
	default:
		return
	}
	policy := n.NotificationsToSend
	if policy == "" {
		// Preserve the old notifyOnFail contract for CRs and clients that have
		// not written the richer policy: it only affects failed deploys.
		policy = appv1alpha1.NotificationsToSendDefault
		if kind == deployMailFailed {
			switch n.NotifyOnFail {
			case "notify":
				policy = appv1alpha1.NotificationsToSendAll
			case "ignore":
				policy = appv1alpha1.NotificationsToSendNone
			}
		}
	}
	s.notifyDeploy(ctx, n.TenantID, n.AppName, kind, policy, deployDetails{
		deployID:      n.DeployID,
		commitMessage: n.CommitMessage,
		commitSHA:     n.CommitSHA,
		repoURL:       n.RepoURL,
	})
}

// deployDetails carries the optional per-deploy facts the email renders when
// they are available (w7/m44): the deploy id (for the "View Logs" link), the
// commit message, and the commit SHA + repo URL (for the "View commit" link).
// All empty for the started path / image-backed deploys.
type deployDetails struct {
	deployID      string
	commitMessage string
	commitSHA     string
	repoURL       string
}

// notifyDeploy performs the common preference lookup and bounded email fan-out
// for both the request-time and reconcile-time notifier seams.
func (s *Service) notifyDeploy(ctx context.Context, tenantID, appName string, kind deployMailKind, notificationsToSend string, details deployDetails) {
	if s.Store == nil || s.Mailer == nil {
		return
	}
	recipients, err := s.Store.ListNotifyRecipients(ctx, tenantID)
	if err != nil {
		log.Printf("notifications: list recipients for %s: %v", tenantID, err)
		return
	}
	if s.Identities == nil {
		return // nowhere to resolve an address — nothing this pass can send
	}
	// The email is identical for every recipient — compose it once, before the
	// fan-out, rather than rebuilding the same URL + body inside each goroutine
	// (w7/m44 simplify). Render both bodies here too, not per-recipient.
	logsURL := s.deployLogsURL(appName, details.deployID)
	emailSubject, msg := deployEmail(appName, kind, details, logsURL)
	text, html := msg.Text(), msg.HTML()
	// Each recipient costs two blocking network round-trips (an identity
	// lookup, then an SMTP send) — run them concurrently, capped, so a large
	// workspace's fan-out costs roughly one round-trip's latency instead of
	// N in sequence.
	var wg sync.WaitGroup
	sem := make(chan struct{}, notifyConcurrency)
	for _, r := range recipients {
		var wants bool
		switch notificationsToSend {
		case appv1alpha1.NotificationsToSendNone:
			wants = false
		case appv1alpha1.NotificationsToSendFailure:
			wants = kind == deployMailFailed
		case appv1alpha1.NotificationsToSendAll:
			wants = true
		default:
			switch kind {
			case deployMailStarted:
				wants = r.DeployStarted
			case deployMailSucceeded:
				wants = r.DeploySucceeded
			case deployMailFailed:
				wants = r.DeployFailed
			}
		}
		if !wants {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(subject string) {
			defer wg.Done()
			defer func() { <-sem }()
			s.notifyOne(ctx, subject, appName, kind, emailSubject, text, html)
		}(r.Subject)
	}
	wg.Wait()
}

// notifyConcurrency bounds how many recipients' email lookup + send run at
// once — enough to amortize per-recipient network latency without opening an
// unbounded number of SMTP connections for a very large workspace.
const notifyConcurrency = 8

// notifyOne resolves one recipient's address and sends the pre-composed deploy
// email (text + HTML), logging (never propagating) any failure — see
// NotifyDeploy's doc. appName/kind are carried only to label that log line.
func (s *Service) notifyOne(ctx context.Context, recipient, appName string, kind deployMailKind, emailSubject, text, html string) {
	addr, ok := s.Identities.LookupEmail(ctx, recipient)
	if !ok {
		return // no known address — nothing to send to (honest omit)
	}
	if err := s.Mailer.Send(ctx, addr, emailSubject, text, html); err != nil {
		log.Printf("notifications: sending deploy %s email for %s to %s: %v", kind, appName, addr, err)
	}
}

// deployLogsURL is the "View Logs" deep link to the deploy's detail page, which
// renders build/deploy logs (w7/m44). Empty when the dashboard URL is unset or
// there is no deploy id — the email then omits the link. Reuses the shared
// dashboard-URL joiner (trailing-slash-safe, guards a malformed base).
func (s *Service) deployLogsURL(appName, deployID string) string {
	if s.DashboardBaseURL == "" || appName == "" || deployID == "" {
		return ""
	}
	cfg := resourcemeta.Config{DashboardBaseURL: s.DashboardBaseURL}
	return cfg.DashboardURL(path.Join("services", appName, "deploys"), deployID)
}

// deployEmail composes the deploy notification (w7/m44): impact framing that
// matches Render's register, then the commit message, a "View logs" button, and
// a "View commit" link to the repo's web commit page — each rendered only when
// its data is available (an image-backed deploy carries no commit; the logs link
// needs the dashboard URL; the commit link needs the repo URL + resolved SHA).
// Message.HTML() renders it branded; Message.Text() carries the same content.
func deployEmail(appName string, kind deployMailKind, details deployDetails, logsURL string) (subject string, msg email.Message) {
	var lead string
	switch kind {
	case deployMailStarted:
		subject = fmt.Sprintf("Deploy started: %s", appName)
		lead = fmt.Sprintf("A deploy of %q has started. We'll email you when it finishes.", appName)
	case deployMailSucceeded:
		subject = fmt.Sprintf("Deploy succeeded: %s", appName)
		lead = fmt.Sprintf("A new deploy of %q is live. Your latest changes are now serving.", appName)
	default:
		subject = fmt.Sprintf("Deploy failed: %s", appName)
		lead = fmt.Sprintf("We encountered an error during the deploy process for %q. "+
			"This means your deploy didn't complete successfully and your latest changes may not be live.", appName)
	}
	msg = email.Message{Title: subject, Paragraphs: []string{lead}}
	// The commit renders as one "Commit <sha>" block: a linked short SHA (when
	// the repo URL resolves to a web commit page) above the message. With no
	// resolvable link it stays the plain "Commit:\n<message>" paragraph — no
	// separate "View commit" line duplicating the same reference.
	cm := strings.TrimSpace(details.commitMessage)
	if cu := commitURL(details.repoURL, details.commitSHA); cu != "" {
		msg.Reference = &email.Reference{Label: "Commit", Token: shortSHA(details.commitSHA), URL: cu, Desc: cm}
	} else if cm != "" {
		msg.Paragraphs = append(msg.Paragraphs, "Commit:\n"+cm)
	}
	if logsURL != "" {
		msg.CTA = &email.CTA{Lead: "View logs", Label: "View logs", URL: logsURL}
	}
	return subject, msg
}

// commitURL builds the repo's web commit page URL from an App's spec.repo and a
// commit SHA — the deploy email's "View commit" link. It normalizes the common
// clone-URL shapes (https, scp-like SSH `git@host:owner/repo`, ssh://, a
// trailing .git) to an https web URL and picks the host's commit path segment
// (GitHub/Gitea `/commit/`, GitLab `/-/commit/`, Bitbucket `/commits/`).
// Returns "" when either input is empty or the repo URL can't be parsed to a
// host — an image-backed deploy (no repo) then simply carries no commit link.
func commitURL(repo, sha string) string {
	repo, sha = strings.TrimSpace(repo), strings.TrimSpace(sha)
	if repo == "" || sha == "" {
		return ""
	}
	// scp-like SSH (git@github.com:owner/repo) → URL form the parser accepts.
	if strings.HasPrefix(repo, "git@") {
		if i := strings.Index(repo, ":"); i > len("git@") {
			repo = "https://" + repo[len("git@"):i] + "/" + repo[i+1:]
		}
	}
	repo = strings.TrimSuffix(repo, ".git")
	u, err := url.Parse(repo)
	if err != nil || u.Host == "" {
		return ""
	}
	// The web link is always https, without any embedded credentials.
	u.Scheme, u.User = "https", nil
	seg := "/commit/"
	switch {
	case strings.Contains(u.Host, "gitlab"):
		seg = "/-/commit/"
	case strings.Contains(u.Host, "bitbucket"):
		seg = "/commits/"
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + seg + sha
	return u.String()
}

// shortSHA abbreviates a commit SHA to its first 7 characters (the git default),
// leaving anything already shorter untouched.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func (k deployMailKind) String() string {
	switch k {
	case deployMailStarted:
		return "started"
	case deployMailSucceeded:
		return "succeeded"
	default:
		return "failed"
	}
}
