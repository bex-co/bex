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

package store

import (
	"context"
	"encoding/json"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// NotificationSettings is a row of `notification_settings` — one member's
// override of their per-workspace deploy-email preferences (w3/m9). A member
// with no row is not "opted out" — internal/notifications.Service applies the
// failure-only default when GetNotificationSettings returns ErrNotFound.
type NotificationSettings struct {
	ID              string          `json:"id"`
	TenantID        string          `json:"tenantId"`
	Subject         string          `json:"subject"`
	DeployStarted   bool            `json:"deployStarted"`
	DeploySucceeded bool            `json:"deploySucceeded"`
	DeployFailed    bool            `json:"deployFailed"`
	PushPolicy      json.RawMessage `json:"-"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// NotifyRecipient is one workspace member's resolved deploy-notification
// preferences — an explicit notification_settings row's values, or the
// default (started=false, succeeded=false, failed=true) for a member who never
// customized them. Returned by
// ListNotifyRecipients, the notification service's fan-out source: it names WHO to
// consider emailing, not their address (email resolution is the identity
// provider's job, outside the store).
type NotifyRecipient struct {
	Subject         string
	DeployStarted   bool
	DeploySucceeded bool
	DeployFailed    bool
}

// GetNotificationSettings returns a member's EXPLICIT preference row —
// ErrNotFound when they never customized it (the caller applies the default).
func (s *PGStore) GetNotificationSettings(ctx context.Context, tenantID, subject string) (NotificationSettings, error) {
	var n NotificationSettings
	err := s.Pool.QueryRow(ctx, `
		SELECT id, tenant_id, subject, deploy_started, deploy_succeeded, deploy_failed, push_policy, created_at, updated_at
		FROM notification_settings WHERE tenant_id = $1 AND subject = $2`,
		tenantID, subject,
	).Scan(&n.ID, &n.TenantID, &n.Subject, &n.DeployStarted, &n.DeploySucceeded, &n.DeployFailed, &n.PushPolicy, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return NotificationSettings{}, classify("notification settings", err)
	}
	return n, nil
}

// UpsertNotificationSettings writes a member's preferences, creating the row
// on first write and updating it (bumping updated_at) thereafter — the single
// write path both REST/GraphQL/MCP `UpdateSettings` uses.
func (s *PGStore) UpsertNotificationSettings(ctx context.Context, tenantID, subject string, deployStarted, deploySucceeded, deployFailed bool) (NotificationSettings, error) {
	n := NotificationSettings{TenantID: tenantID, Subject: subject, DeployStarted: deployStarted, DeploySucceeded: deploySucceeded, DeployFailed: deployFailed}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO notification_settings (id, tenant_id, subject, deploy_started, deploy_succeeded, deploy_failed)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tenant_id, subject) DO UPDATE
			SET deploy_started   = EXCLUDED.deploy_started,
			    deploy_succeeded = EXCLUDED.deploy_succeeded,
			    deploy_failed    = EXCLUDED.deploy_failed,
			    updated_at       = now()
		RETURNING id, push_policy, created_at, updated_at`,
		ids.New(ids.Notification), tenantID, subject, deployStarted, deploySucceeded, deployFailed,
	).Scan(&n.ID, &n.PushPolicy, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return NotificationSettings{}, classify("notification settings", err)
	}
	return n, nil
}

// UpsertNotificationPushPolicy replaces only the caller's native-push policy.
// On first write it creates the same existing notification_settings row with
// the established failure-only email defaults; later writes leave every email
// preference byte-for-byte unchanged.
func (s *PGStore) UpsertNotificationPushPolicy(ctx context.Context, tenantID, subject string, policy json.RawMessage) (NotificationSettings, error) {
	var n NotificationSettings
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO notification_settings (id, tenant_id, subject, deploy_started, deploy_succeeded, deploy_failed, push_policy)
		VALUES ($1, $2, $3, false, false, true, $4)
		ON CONFLICT (tenant_id, subject) DO UPDATE
			SET push_policy = EXCLUDED.push_policy,
			    updated_at  = now()
		RETURNING id, tenant_id, subject, deploy_started, deploy_succeeded, deploy_failed, push_policy, created_at, updated_at`,
		ids.New(ids.Notification), tenantID, subject, policy,
	).Scan(&n.ID, &n.TenantID, &n.Subject, &n.DeployStarted, &n.DeploySucceeded, &n.DeployFailed, &n.PushPolicy, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return NotificationSettings{}, classify("notification push policy", err)
	}
	return n, nil
}

// ListNotifyRecipients returns every member of tenantID with their resolved
// deploy-notification preferences: an explicit row's values via the LEFT
// JOIN, or the failure-only default via COALESCE for a member who never
// customized them. One query serves the notification fan-out on every deploy
// lifecycle event, rather than a settings lookup per member.
func (s *PGStore) ListNotifyRecipients(ctx context.Context, tenantID string) ([]NotifyRecipient, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT m.subject,
		       COALESCE(n.deploy_started, false),
		       COALESCE(n.deploy_succeeded, false),
		       COALESCE(n.deploy_failed, true)
		FROM tenant_members m
		LEFT JOIN notification_settings n ON n.tenant_id = m.tenant_id AND n.subject = m.subject
		WHERE m.tenant_id = $1`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NotifyRecipient
	for rows.Next() {
		var r NotifyRecipient
		if err := rows.Scan(&r.Subject, &r.DeployStarted, &r.DeploySucceeded, &r.DeployFailed); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
