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
	"time"
)

// NotificationOwnerPolicy is a workspace's notification policy (w3/m271),
// Render's owner-level notification settings: whether email is a destination
// and which level of notifications a service on "default" sends. A workspace
// with no row has Render's default (email on, failure only).
type NotificationOwnerPolicy struct {
	WorkspaceID         string
	EmailEnabled        bool
	NotificationsToSend string
	UpdatedAt           time.Time
}

// NotificationOwnerPolicyPatch is a partial update: a nil field keeps the
// stored value (or the column default on first write).
type NotificationOwnerPolicyPatch struct {
	EmailEnabled        *bool
	NotificationsToSend *string
}

// GetNotificationOwnerPolicy returns the workspace's explicit row, or
// ErrNotFound when it never changed the policy (the caller applies the default).
func (s *PGStore) GetNotificationOwnerPolicy(ctx context.Context, workspaceID string) (NotificationOwnerPolicy, error) {
	p := NotificationOwnerPolicy{WorkspaceID: workspaceID}
	err := s.Pool.QueryRow(ctx, `
		SELECT email_enabled, notifications_to_send, updated_at
		FROM notification_owner_policies WHERE workspace_id = $1`, workspaceID,
	).Scan(&p.EmailEnabled, &p.NotificationsToSend, &p.UpdatedAt)
	if err != nil {
		return NotificationOwnerPolicy{}, classify("notification owner policy", err)
	}
	return p, nil
}

// UpsertNotificationOwnerPolicy applies patch in one statement, so two
// concurrent partial updates of different fields never lose each other's change.
func (s *PGStore) UpsertNotificationOwnerPolicy(ctx context.Context, workspaceID string, patch NotificationOwnerPolicyPatch) (NotificationOwnerPolicy, error) {
	p := NotificationOwnerPolicy{WorkspaceID: workspaceID}
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO notification_owner_policies (workspace_id, email_enabled, notifications_to_send)
		VALUES ($1, COALESCE($2, true), COALESCE($3, 'failure'))
		ON CONFLICT (workspace_id) DO UPDATE
			SET email_enabled         = COALESCE($2, notification_owner_policies.email_enabled),
			    notifications_to_send = COALESCE($3, notification_owner_policies.notifications_to_send),
			    updated_at            = now()
		RETURNING email_enabled, notifications_to_send, updated_at`,
		workspaceID, patch.EmailEnabled, patch.NotificationsToSend,
	).Scan(&p.EmailEnabled, &p.NotificationsToSend, &p.UpdatedAt)
	if err != nil {
		return NotificationOwnerPolicy{}, classify("notification owner policy", err)
	}
	return p, nil
}
