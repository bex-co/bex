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
	"regexp"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	maxDeviceIDBytes  = 200
	maxPushTokenBytes = 4096

	auditRegisterDevice   = "notifications.RegisterDeviceSubscription"
	auditUnregisterDevice = "notifications.UnregisterDeviceSubscription"
	auditRevokeDevices    = "notifications.RevokeDeviceSubscriptions"
)

var deviceIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,199}$`)

// RegisterDeviceInput is deliberately provider-neutral in shape while t001's
// accepted vocabulary stays closed to the one configured transport, Expo.
type RegisterDeviceInput struct {
	DeviceID  string `json:"deviceId"`
	SessionID string `json:"sessionId"`
	Provider  string `json:"provider"`
	Platform  string `json:"platform"`
	Token     string `json:"token"`
}

// DeviceSubscriptionView is safe to return to the owning member. It omits the
// provider token, its digest, workspace, and subject by construction.
type DeviceSubscriptionView struct {
	DeviceID         string    `json:"deviceId"`
	Provider         string    `json:"provider"`
	Platform         string    `json:"platform"`
	PreferenceRef    string    `json:"preferenceRef,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	LastRegisteredAt time.Time `json:"lastRegisteredAt"`
}

// RegisterDeviceSubscription creates, rotates, or reactivates the caller's
// device atomically. Viewer-and-up matches the existing personal notification
// settings policy; the workspace and subject always come from authenticated
// context, never request fields.
func (s *Service) RegisterDeviceSubscription(ctx context.Context, in RegisterDeviceInput) (DeviceSubscriptionView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return DeviceSubscriptionView{}, err
	}
	if s.Store == nil {
		return DeviceSubscriptionView{}, core.ErrNotificationsUnavailable
	}
	if !s.pushTransportAvailable() {
		return DeviceSubscriptionView{}, core.ErrPushUnavailable
	}
	tenantID, subject, err := s.deviceOwner(ctx)
	if err != nil {
		return DeviceSubscriptionView{}, err
	}
	in, err = normalizeDeviceInput(in)
	if err != nil {
		return DeviceSubscriptionView{}, err
	}
	row, err := s.Store.UpsertDevicePushSubscription(ctx, store.DevicePushSubscription{
		TenantID: tenantID, Subject: subject, DeviceID: in.DeviceID,
		SessionID: in.SessionID, Provider: in.Provider, Platform: in.Platform, Token: in.Token,
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrPushDeviceSubjectLimit):
			return DeviceSubscriptionView{}, core.NewConflictError(
				"PUSH_DEVICE_SUBJECT_LIMIT",
				fmt.Sprintf("a member may register at most %d active push devices", store.MaxActivePushDevicesPerSubject), nil)
		case errors.Is(err, store.ErrPushDeviceWorkspaceLimit):
			return DeviceSubscriptionView{}, core.NewConflictError(
				"PUSH_DEVICE_WORKSPACE_LIMIT",
				fmt.Sprintf("a workspace may register at most %d active push devices", store.MaxActivePushDevicesPerWorkspace), nil)
		default:
			return DeviceSubscriptionView{}, store.MapError(err)
		}
	}
	s.recordDeviceAudit(ctx, tenantID, auditRegisterDevice, in.DeviceID)
	return deviceSubscriptionView(row), nil
}

// ListDeviceSubscriptions lists only the caller's active devices in the
// effective workspace. Store and view projections both omit token material.
func (s *Service) ListDeviceSubscriptions(ctx context.Context) ([]DeviceSubscriptionView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrNotificationsUnavailable
	}
	tenantID, subject, err := s.deviceOwner(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.Store.ListOwnDevicePushSubscriptions(ctx, tenantID, subject)
	if err != nil {
		return nil, err
	}
	out := make([]DeviceSubscriptionView, 0, len(rows))
	for _, row := range rows {
		out = append(out, deviceSubscriptionView(row))
	}
	return out, nil
}

// UnregisterDeviceSubscription idempotently revokes one caller-owned device.
// A foreign or unknown id returns false without revealing which case it was.
func (s *Service) UnregisterDeviceSubscription(ctx context.Context, deviceID string) (bool, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return false, err
	}
	if s.Store == nil {
		return false, core.ErrNotificationsUnavailable
	}
	tenantID, subject, err := s.deviceOwner(ctx)
	if err != nil {
		return false, err
	}
	deviceID = strings.TrimSpace(deviceID)
	if !deviceIDPattern.MatchString(deviceID) {
		return false, fmt.Errorf("%w: deviceId must be a safe opaque identifier", core.ErrBadRequest)
	}
	changed, err := s.Store.RevokeDevicePushSubscription(ctx, tenantID, subject, deviceID)
	if err != nil {
		return false, err
	}
	s.recordDeviceAudit(ctx, tenantID, auditUnregisterDevice, deviceID)
	return changed, nil
}

// RevokeDeviceSubscriptions revokes every active device for the caller in this
// workspace. Mobile logout calls this best-effort after clearing local auth;
// the store operation itself is idempotent for retries.
func (s *Service) RevokeDeviceSubscriptions(ctx context.Context) (int64, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return 0, err
	}
	if s.Store == nil {
		return 0, core.ErrNotificationsUnavailable
	}
	tenantID, subject, err := s.deviceOwner(ctx)
	if err != nil {
		return 0, err
	}
	count, err := s.Store.RevokeAllDevicePushSubscriptions(ctx, tenantID, subject)
	if err != nil {
		return 0, err
	}
	s.recordDeviceAudit(ctx, tenantID, auditRevokeDevices, "all")
	return count, nil
}

func (s *Service) deviceOwner(ctx context.Context) (tenantID, subject string, err error) {
	tenantID, ok := s.Base.Tenant(ctx)
	if !ok {
		return "", "", fmt.Errorf("%w: no workspace for device subscription", core.ErrBadRequest)
	}
	id, ok := core.IdentityFrom(ctx)
	if !ok || strings.TrimSpace(id.Subject) == "" {
		return "", "", core.ErrForbidden
	}
	return tenantID, id.Subject, nil
}

func normalizeDeviceInput(in RegisterDeviceInput) (RegisterDeviceInput, error) {
	in.DeviceID = strings.TrimSpace(in.DeviceID)
	in.SessionID = strings.TrimSpace(in.SessionID)
	in.Provider = strings.ToLower(strings.TrimSpace(in.Provider))
	in.Platform = strings.ToLower(strings.TrimSpace(in.Platform))
	in.Token = strings.TrimSpace(in.Token)
	if !deviceIDPattern.MatchString(in.DeviceID) {
		return RegisterDeviceInput{}, fmt.Errorf("%w: deviceId must be a safe opaque identifier", core.ErrBadRequest)
	}
	if !deviceIDPattern.MatchString(in.SessionID) {
		return RegisterDeviceInput{}, fmt.Errorf("%w: sessionId must be a safe opaque identifier", core.ErrBadRequest)
	}
	if in.Provider != "expo" {
		return RegisterDeviceInput{}, fmt.Errorf("%w: provider must be expo", core.ErrBadRequest)
	}
	if in.Platform != "ios" && in.Platform != "android" {
		return RegisterDeviceInput{}, fmt.Errorf("%w: platform must be ios or android", core.ErrBadRequest)
	}
	if !validBounded(in.Token, maxPushTokenBytes) {
		return RegisterDeviceInput{}, fmt.Errorf("%w: token must be between 1 and %d bytes", core.ErrBadRequest, maxPushTokenBytes)
	}
	return in, nil
}

func validBounded(value string, max int) bool {
	return len(value) > 0 && len(value) <= max && !strings.ContainsRune(value, '\x00')
}

func deviceSubscriptionView(row store.DevicePushSubscription) DeviceSubscriptionView {
	return DeviceSubscriptionView{
		DeviceID: row.DeviceID, Provider: row.Provider, Platform: row.Platform,
		PreferenceRef: row.PreferenceID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		LastRegisteredAt: row.LastRegisteredAt,
	}
}

// recordDeviceAudit uses fixed verbs and a redacted device-only target. The
// bearer token/digest can never enter this event. Authorization denials remain
// recorded by Base.Authorize's normal read-relation denial path.
func (s *Service) recordDeviceAudit(ctx context.Context, tenantID, verb, deviceID string) {
	id, _ := core.IdentityFrom(ctx)
	core.RecordAuditEvent(ctx, s.Base.Audit, core.AuditEvent{
		Caller: id.Subject, CallerMethod: id.Method,
		Verb: verb, Resource: core.WorkspaceObject(tenantID),
		Target:  "notification-device:" + deviceID,
		Outcome: core.AuditAllowed, At: s.Now(),
	})
}
