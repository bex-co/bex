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

// Package notifications is the deploy-notification feature (w3/m9): email a
// workspace's members when one of its services' deploys succeeds or fails,
// matching Render's /notification-settings surface. It has two seams: the
// caller-facing GetSettings/UpdateSettings verbs (REST/GraphQL/MCP, this
// package's own Service methods, each authorized like any other verb) and
// NotifyDeploy, which the control-plane reconciler calls directly (not a
// caller verb — see internal/store.DeployNotifier) the instant it closes a
// deploy row, so mail goes out in the same reconcile pass the status write
// happens in. Delivery reuses the w4/m12 invite-email pattern: best-effort,
// logged not returned, over the shared mailer.SMTP relay.
package notifications

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// NotificationsStore is the slice of the control-plane store this feature
// reads/writes through. *store.PGStore satisfies it structurally.
type NotificationsStore interface {
	GetNotificationSettings(ctx context.Context, tenantID, subject string) (store.NotificationSettings, error)
	UpsertNotificationSettings(ctx context.Context, tenantID, subject string, deploySucceeded, deployFailed bool) (store.NotificationSettings, error)
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
// preferences — Render's /notification-settings shape, narrowed to the two
// events bex actually fires on (see docs/ADR018-render-parity.md; "deploy
// started" has no trigger point in this pass, so it is not modeled rather
// than shipped inert).
type SettingsView struct {
	DeploySucceeded bool `json:"deploySucceeded"`
	DeployFailed    bool `json:"deployFailed"`
}

// defaultSettings is what a member who never customized their preferences
// gets: notified on both outcomes — the useful default for a deploy platform.
var defaultSettings = SettingsView{DeploySucceeded: true, DeployFailed: true}

func toView(n store.NotificationSettings) SettingsView {
	return SettingsView{DeploySucceeded: n.DeploySucceeded, DeployFailed: n.DeployFailed}
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
func (s *Service) UpdateSettings(ctx context.Context, deploySucceeded, deployFailed bool) (SettingsView, error) {
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
	row, err := s.Store.UpsertNotificationSettings(ctx, tenantID, id.Subject, deploySucceeded, deployFailed)
	if err != nil {
		return SettingsView{}, err
	}
	return toView(row), nil
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
	if s.Store == nil || s.Mailer == nil {
		return
	}
	var succeeded bool
	switch n.Status {
	case store.DeployLive:
		succeeded = true
	case store.DeployUpdateFailed:
		succeeded = false
	default:
		return
	}
	recipients, err := s.Store.ListNotifyRecipients(ctx, n.TenantID)
	if err != nil {
		log.Printf("notifications: list recipients for %s: %v", n.TenantID, err)
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
		wants := r.DeploySucceeded
		if !succeeded {
			switch n.NotifyOnFail {
			case "ignore":
				wants = false // muted for everyone, regardless of their own preference
			case "notify":
				wants = true // forced on for everyone, regardless of their own preference
			default: // "default", "", or an unrecognized value — defer to member preference
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
			s.notifyOne(ctx, subject, n.AppName, n.Status, succeeded)
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
func (s *Service) notifyOne(ctx context.Context, subject, appName, status string, succeeded bool) {
	email, ok := s.Identities.LookupEmail(ctx, subject)
	if !ok {
		return // no known address — nothing to send to (honest omit)
	}
	emailSubject, body := deployEmail(appName, succeeded)
	if err := s.Mailer.Send(ctx, email, emailSubject, body); err != nil {
		log.Printf("notifications: sending deploy %s email for %s to %s: %v", status, appName, email, err)
	}
}

// deployEmail composes the plain-text deploy notification.
func deployEmail(appName string, succeeded bool) (subject, body string) {
	if succeeded {
		return fmt.Sprintf("Deploy succeeded: %s", appName),
			fmt.Sprintf("A new deploy of %q went live.\n", appName)
	}
	return fmt.Sprintf("Deploy failed: %s", appName),
		fmt.Sprintf("A deploy of %q failed.\n", appName)
}
