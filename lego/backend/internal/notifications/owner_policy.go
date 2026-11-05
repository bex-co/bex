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

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The workspace notification policy (w3/m271, docs/ADR052-notifications.md
// D2): Render's owner-level notification settings, served on Render's own
// GET/PATCH /notification-settings/owners/{ownerId}. It decides, for every
// service whose notificationsToSend is "default", which level of
// notifications is sent (none, failure or all), and whether email is a
// destination at all. An explicit service override still wins on the level;
// each member's own preferences then narrow personal email delivery.

// OwnerSettingsView is Render's owner notification settings object.
// slackEnabled and previewNotificationsEnabled are always false: Slack and PR
// previews are recorded non-goals (.pm/DO_NOT_DO.md).
type OwnerSettingsView struct {
	OwnerID                     string `json:"ownerId"`
	SlackEnabled                bool   `json:"slackEnabled"`
	EmailEnabled                bool   `json:"emailEnabled"`
	PreviewNotificationsEnabled bool   `json:"previewNotificationsEnabled"`
	NotificationsToSend         string `json:"notificationsToSend"`
}

// OwnerSettingsInput is Render's PATCH body: an omitted field keeps its value.
type OwnerSettingsInput struct {
	EmailEnabled                *bool   `json:"emailEnabled,omitempty"`
	PreviewNotificationsEnabled *bool   `json:"previewNotificationsEnabled,omitempty"`
	NotificationsToSend         *string `json:"notificationsToSend,omitempty"`
}

// OwnerPolicy is the part of the workspace policy the fan-out reads.
type OwnerPolicy struct {
	EmailEnabled        bool
	NotificationsToSend string
}

// defaultOwnerPolicy is Render's default for a workspace that never changed
// it: email on, failure notifications only.
var defaultOwnerPolicy = OwnerPolicy{EmailEnabled: true, NotificationsToSend: appv1alpha1.NotificationsToSendFailure}

// OwnerPolicyStore persists the workspace policy; *store.PGStore satisfies it.
type OwnerPolicyStore interface {
	GetNotificationOwnerPolicy(ctx context.Context, workspaceID string) (store.NotificationOwnerPolicy, error)
	UpsertNotificationOwnerPolicy(ctx context.Context, workspaceID string, patch store.NotificationOwnerPolicyPatch) (store.NotificationOwnerPolicy, error)
}

// OwnerPolicyLookup resolves a workspace's policy for a fan-out.
type OwnerPolicyLookup func(ctx context.Context, tenantID string) (OwnerPolicy, error)

// StoredOwnerPolicy reads the policy from st: Render's default for a
// workspace with no row, or for a nil store.
func StoredOwnerPolicy(st OwnerPolicyStore) OwnerPolicyLookup {
	return func(ctx context.Context, tenantID string) (OwnerPolicy, error) {
		if st == nil {
			return defaultOwnerPolicy, nil
		}
		row, err := st.GetNotificationOwnerPolicy(ctx, tenantID)
		if errors.Is(err, store.ErrNotFound) {
			return defaultOwnerPolicy, nil
		}
		if err != nil {
			return OwnerPolicy{}, err
		}
		return OwnerPolicy{EmailEnabled: row.EmailEnabled, NotificationsToSend: row.NotificationsToSend}, nil
	}
}

// ownerPolicy is lookup(ctx, tenantID), or Render's default when lookup is nil.
func ownerPolicy(ctx context.Context, lookup OwnerPolicyLookup, tenantID string) (OwnerPolicy, error) {
	if lookup == nil {
		return defaultOwnerPolicy, nil
	}
	return lookup(ctx, tenantID)
}

// GetOwnerSettings reads a workspace's notification policy. Any member may
// read it: the service page shows what "Use workspace default" means.
func (s *Service) GetOwnerSettings(ctx context.Context, ownerID string) (OwnerSettingsView, error) {
	workspaceID, err := s.AuthorizeWorkspace(ctx, core.RelCanView, ownerID)
	if err != nil {
		return OwnerSettingsView{}, err
	}
	if s.Store == nil {
		return OwnerSettingsView{}, core.ErrNotificationsUnavailable
	}
	policy, err := StoredOwnerPolicy(s.Store)(ctx, workspaceID)
	if err != nil {
		return OwnerSettingsView{}, err
	}
	return ownerSettingsView(workspaceID, policy), nil
}

// UpdateOwnerSettings applies Render's partial PATCH. Admin-only (can_manage
// on the exact workspace), like every other workspace setting. Each field is
// validated before anything is written, so a refused update changes nothing.
func (s *Service) UpdateOwnerSettings(ctx context.Context, ownerID string, input OwnerSettingsInput) (OwnerSettingsView, error) {
	workspaceID, err := s.AuthorizeWorkspace(ctx, core.RelCanManage, ownerID)
	if err != nil {
		return OwnerSettingsView{}, err
	}
	if s.Store == nil {
		return OwnerSettingsView{}, core.ErrNotificationsUnavailable
	}
	if input.PreviewNotificationsEnabled != nil && *input.PreviewNotificationsEnabled {
		return OwnerSettingsView{}, core.NewBadRequestError("NOTIFICATION_PREVIEWS_UNSUPPORTED",
			"previewNotificationsEnabled cannot be true: bex has no preview environments",
			map[string]any{"field": "previewNotificationsEnabled"})
	}
	if input.NotificationsToSend != nil && !ownerLevel(*input.NotificationsToSend) {
		return OwnerSettingsView{}, core.NewBadRequestError("NOTIFICATION_LEVEL_INVALID",
			"notificationsToSend must be none, failure or all",
			map[string]any{"field": "notificationsToSend"})
	}
	row, err := s.Store.UpsertNotificationOwnerPolicy(ctx, workspaceID, store.NotificationOwnerPolicyPatch{
		EmailEnabled: input.EmailEnabled, NotificationsToSend: input.NotificationsToSend,
	})
	if errors.Is(err, store.ErrNotFound) {
		return OwnerSettingsView{}, core.NewNotFoundError("WORKSPACE_NOT_FOUND", "workspace not found", map[string]any{"ownerId": workspaceID})
	}
	if err != nil {
		return OwnerSettingsView{}, err
	}
	return ownerSettingsView(workspaceID, OwnerPolicy{EmailEnabled: row.EmailEnabled, NotificationsToSend: row.NotificationsToSend}), nil
}

func ownerSettingsView(workspaceID string, p OwnerPolicy) OwnerSettingsView {
	return OwnerSettingsView{OwnerID: workspaceID, EmailEnabled: p.EmailEnabled, NotificationsToSend: p.NotificationsToSend}
}

// ownerLevel reports an explicit level: none, failure or all. Render's owner
// enum has no "default", which only a service can name.
func ownerLevel(level string) bool {
	switch level {
	case appv1alpha1.NotificationsToSendNone, appv1alpha1.NotificationsToSendFailure, appv1alpha1.NotificationsToSendAll:
		return true
	}
	return false
}

// notifyLevel is the Render tier an event belongs to (render.com/docs/
// notifications): failure events are sent at "Only failure notifications" and
// above; the rest (a deploy going live, a recovery) only at "All".
type notifyLevel int

const (
	levelFailure notifyLevel = iota
	levelAll
)

// effectiveLevel is the level a service sends at: its explicit override, else
// the workspace's. fromWorkspace reports the second case.
func effectiveLevel(service string, owner OwnerPolicy) (level string, fromWorkspace bool) {
	if ownerLevel(service) {
		return service, false
	}
	return owner.NotificationsToSend, true
}

// levelAdmits reports whether a service sending at level sends an event of l.
func levelAdmits(level string, l notifyLevel) bool {
	switch level {
	case appv1alpha1.NotificationsToSendAll:
		return true
	case appv1alpha1.NotificationsToSendFailure:
		return l == levelFailure
	}
	return false
}

// memberToggle names the member email preference that gates an event.
type memberToggle int

const (
	toggleFailed memberToggle = iota
	toggleStarted
	toggleSucceeded
	// toggleNone: no member preference speaks about the event (a disk back
	// below its threshold), so the level alone decides.
	toggleNone
)

// memberPrefs is one member's email preferences in a workspace.
type memberPrefs struct{ started, succeeded, failed bool }

// customized reports preferences other than the failure-only default. A member
// on the default follows the workspace level for every event; a member who
// changed them receives only what they turned on.
func (m memberPrefs) customized() bool { return m.started || m.succeeded || !m.failed }

func (m memberPrefs) wants(t memberToggle) bool {
	switch t {
	case toggleFailed:
		return m.failed
	case toggleStarted:
		return m.started || !m.customized()
	case toggleSucceeded:
		return m.succeeded || !m.customized()
	}
	return true
}

// emailPolicy is the email decision for one service's event in one workspace.
type emailPolicy struct {
	owner   OwnerPolicy
	service string
}

// wants reports whether a member is emailed an event of level l. Email off
// for the workspace sends nothing; an explicit service level mails every
// member; the workspace level mails the members whose preference allows it.
func (p emailPolicy) wants(l notifyLevel, t memberToggle, m memberPrefs) bool {
	if !p.owner.EmailEnabled {
		return false
	}
	level, fromWorkspace := effectiveLevel(p.service, p.owner)
	if !levelAdmits(level, l) {
		return false
	}
	return !fromWorkspace || m.wants(t)
}
