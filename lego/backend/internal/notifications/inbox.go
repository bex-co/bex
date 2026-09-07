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
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	defaultNotificationInboxLimit = 50
	maxNotificationInboxLimit     = 100
)

// PushNotificationView is the caller-safe projection of a durable logical
// notification. Store routing and delivery fields deliberately never cross
// this boundary.
type PushNotificationView struct {
	ID           string     `json:"id"`
	Event        string     `json:"event"`
	Title        string     `json:"title"`
	Body         string     `json:"body"`
	Urgency      string     `json:"urgency"`
	ResourceKind string     `json:"resourceKind"`
	ResourceID   string     `json:"resourceId"`
	DeepLink     string     `json:"deepLink"`
	OccurredAt   time.Time  `json:"occurredAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	ReadAt       *time.Time `json:"readAt"`
}

func pushNotificationView(n store.PushNotification) PushNotificationView {
	return PushNotificationView{
		ID: n.EventID, Event: n.EventType, Title: n.Title, Body: n.Body,
		Urgency: n.Urgency, ResourceKind: n.ResourceKind, ResourceID: n.ResourceID,
		DeepLink: n.DeepLink, OccurredAt: n.OccurredAt, CreatedAt: n.CreatedAt,
		ReadAt: n.ReadAt,
	}
}

// ListNotificationInbox returns only the authenticated caller's durable push
// notifications in their current workspace.
func (s *Service) ListNotificationInbox(ctx context.Context, limit int) ([]PushNotificationView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrNotificationsUnavailable
	}
	if limit < 1 || limit > maxNotificationInboxLimit {
		return nil, fmt.Errorf("%w: notification limit must be between 1 and %d", core.ErrBadRequest, maxNotificationInboxLimit)
	}
	tenantID, subject, err := s.notificationInboxOwner(ctx)
	if err != nil {
		return nil, err
	}
	// Destination access decides visibility (destination_policy.go): gated
	// event types the caller's REAL current relations don't cover are
	// filtered in SQL, so a downgrade removes historic rows and the badge
	// cannot disagree with the page.
	rows, err := s.Store.ListOwnPushNotifications(ctx, tenantID, subject, limit, s.inboxExclusions(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]PushNotificationView, 0, len(rows))
	for _, row := range rows {
		out = append(out, pushNotificationView(row))
	}
	return out, nil
}

// UnreadPushNotificationCount counts unread items only in the authenticated
// caller's own inbox.
func (s *Service) UnreadPushNotificationCount(ctx context.Context) (int, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return 0, err
	}
	if s.Store == nil {
		return 0, core.ErrNotificationsUnavailable
	}
	tenantID, subject, err := s.notificationInboxOwner(ctx)
	if err != nil {
		return 0, err
	}
	count, err := s.Store.CountUnreadPushNotifications(ctx, tenantID, subject, s.inboxExclusions(ctx))
	if err != nil {
		return 0, err
	}
	// graphql.Int is a signed 32-bit value. Fail closed rather than silently
	// truncating an impossible/corrupt count at the public boundary.
	if count < 0 || count > math.MaxInt32 {
		return 0, fmt.Errorf("notification unread count out of range")
	}
	return int(count), nil
}

// MarkPushNotificationRead marks an exact item in the authenticated caller's
// own inbox. A foreign ID and an unknown ID are intentionally indistinguishable.
func (s *Service) MarkPushNotificationRead(ctx context.Context, eventID string) (bool, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return false, err
	}
	if s.Store == nil {
		return false, core.ErrNotificationsUnavailable
	}
	eventID = strings.TrimSpace(eventID)
	kind, ok := ids.KindOf(eventID)
	if !ok || kind != ids.Event {
		return false, fmt.Errorf("%w: invalid notification id", core.ErrBadRequest)
	}
	tenantID, subject, err := s.notificationInboxOwner(ctx)
	if err != nil {
		return false, err
	}
	return s.Store.MarkOwnPushNotificationRead(ctx, tenantID, subject, eventID, s.Now().UTC())
}

// inboxExclusions probes the caller's current relations (fail-closed
// core.Base.Can) through the shared destination policy.
func (s *Service) inboxExclusions(ctx context.Context) []string {
	return inboxExcludedEventTypes(func(relation string) bool { return s.Can(ctx, relation) })
}

func (s *Service) notificationInboxOwner(ctx context.Context) (string, string, error) {
	tenantID, ok := s.Base.Tenant(ctx)
	if !ok || strings.TrimSpace(tenantID) == "" {
		return "", "", fmt.Errorf("%w: no workspace for notification inbox", core.ErrBadRequest)
	}
	identity, ok := core.IdentityFrom(ctx)
	if !ok || strings.TrimSpace(identity.Subject) == "" {
		return "", "", core.ErrForbidden
	}
	return tenantID, identity.Subject, nil
}
