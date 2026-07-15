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
	"sync"

	"github.com/bex-co/bex/lego/backend/internal/core"
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

// Mailer delivers a plain-text email — the same seam members.Service uses,
// kept as this package's own interface (rather than importing members') so
// feature packages stay independent; mailer.SMTP satisfies both structurally.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// EmailLookup resolves a subject's email from the identity provider — the
// same seam members.Service.Identities uses, kept as this package's own
// interface for the same independence reason. The composition root's
// identityEmailLookup adapter satisfies both.
type EmailLookup interface {
	LookupEmail(ctx context.Context, subject string) (string, bool)
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
	s.notifyDeploy(ctx, tenantID, appName, deployMailStarted, notificationsToSend)
}

// NotifyDeploy is store.DeployNotifier: called by the reconciler the instant
// it closes a deploy as succeeded or failed, in the same reconcile pass as
// the status write (the milestone's DoD). NOT a caller verb — no Authorize;
// the reconciler is a trusted internal caller, same as CloneSecreter. A
// status other than DeployLive/DeployUpdateFailed (e.g. a future addition)
// is silently ignored rather than erroring, so this stays forward-compatible
// without a matching change here. n.NotifyOnFail (w4/m21, docs/render-
// artifacts/notify-on-fail.md) governs FAILURE notifications only — a
// succeeded deploy always follows each member's own preference, unmodified.
func (s *Service) NotifyDeploy(ctx context.Context, n store.DeployNotification) {
	var kind deployMailKind
	switch n.Status {
	case store.DeployLive:
		kind = deployMailSucceeded
	case store.DeployUpdateFailed:
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
	s.notifyDeploy(ctx, n.TenantID, n.AppName, kind, policy)
}

// notifyDeploy performs the common preference lookup and bounded email fan-out
// for both the request-time and reconcile-time notifier seams.
func (s *Service) notifyDeploy(ctx context.Context, tenantID, appName string, kind deployMailKind, notificationsToSend string) {
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
			s.notifyOne(ctx, subject, appName, kind)
		}(r.Subject)
	}
	wg.Wait()
}

// notifyConcurrency bounds how many recipients' email lookup + send run at
// once — enough to amortize per-recipient network latency without opening an
// unbounded number of SMTP connections for a very large workspace.
const notifyConcurrency = 8

// notifyOne resolves one recipient's address and sends the deploy email,
// logging (never propagating) any failure — see NotifyDeploy's doc.
func (s *Service) notifyOne(ctx context.Context, subject, appName string, kind deployMailKind) {
	email, ok := s.Identities.LookupEmail(ctx, subject)
	if !ok {
		return // no known address — nothing to send to (honest omit)
	}
	emailSubject, body := deployEmail(appName, kind)
	if err := s.Mailer.Send(ctx, email, emailSubject, body); err != nil {
		log.Printf("notifications: sending deploy %s email for %s to %s: %v", kind, appName, email, err)
	}
}

// deployEmail composes the plain-text deploy notification.
func deployEmail(appName string, kind deployMailKind) (subject, body string) {
	switch kind {
	case deployMailStarted:
		return fmt.Sprintf("Deploy started: %s", appName),
			fmt.Sprintf("A deploy of %q has started.\n", appName)
	case deployMailSucceeded:
		return fmt.Sprintf("Deploy succeeded: %s", appName),
			fmt.Sprintf("A new deploy of %q went live.\n", appName)
	default:
		return fmt.Sprintf("Deploy failed: %s", appName),
			fmt.Sprintf("A deploy of %q failed.\n", appName)
	}
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
