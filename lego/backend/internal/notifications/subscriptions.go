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
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	maxDeviceIDBytes  = 200
	maxPushTokenBytes = 4096

	auditRegisterDevice    = "notifications.RegisterDeviceSubscription"
	auditUnregisterDevice  = "notifications.UnregisterDeviceSubscription"
	auditRevokeDevices     = "notifications.RevokeDeviceSubscriptions"
	auditRegisterWebPush   = "notifications.RegisterWebPushSubscription"
	auditUnregisterWebPush = "notifications.UnregisterWebPushSubscription"
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

type RegisterWebPushInput struct {
	BrowserID string `json:"browserId"`
	Endpoint  string `json:"endpoint"`
	P256dh    string `json:"p256dh"`
	Auth      string `json:"auth"`
}

type WebPushSubscriptionView struct {
	BrowserID        string    `json:"browserId"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	LastRegisteredAt time.Time `json:"lastRegisteredAt"`
}

func (s *Service) RegisterWebPushSubscription(ctx context.Context, in RegisterWebPushInput) (WebPushSubscriptionView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return WebPushSubscriptionView{}, err
	}
	if s.Store == nil {
		return WebPushSubscriptionView{}, core.ErrNotificationsUnavailable
	}
	if !s.webPushTransportAvailable() {
		return WebPushSubscriptionView{}, core.ErrWebPushUnavailable
	}
	tenantID, subject, err := s.deviceOwner(ctx)
	if err != nil {
		return WebPushSubscriptionView{}, err
	}
	in, err = normalizeWebPushInput(in)
	if err != nil {
		return WebPushSubscriptionView{}, err
	}
	row, err := s.Store.UpsertWebPushSubscription(ctx, store.WebPushSubscription{
		TenantID: tenantID, Subject: subject, BrowserID: in.BrowserID,
		Endpoint: in.Endpoint, P256dh: in.P256dh, Auth: in.Auth,
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrWebPushSubjectLimit):
			return WebPushSubscriptionView{}, core.NewConflictError(
				"PUSH_WEBPUSH_SUBJECT_LIMIT",
				fmt.Sprintf("a member may register at most %d active web-push browsers", store.MaxActiveWebPushBrowsersPerSubject), nil)
		case errors.Is(err, store.ErrWebPushWorkspaceLimit):
			return WebPushSubscriptionView{}, core.NewConflictError(
				"PUSH_WEBPUSH_WORKSPACE_LIMIT",
				fmt.Sprintf("a workspace may register at most %d active web-push browsers", store.MaxActiveWebPushBrowsersPerWorkspace), nil)
		default:
			return WebPushSubscriptionView{}, store.MapError(err)
		}
	}
	s.recordDeviceAudit(ctx, tenantID, auditRegisterWebPush, in.BrowserID)
	return webPushSubscriptionView(row), nil
}

func (s *Service) ListWebPushSubscriptions(ctx context.Context) ([]WebPushSubscriptionView, error) {
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
	rows, err := s.Store.ListOwnWebPushSubscriptions(ctx, tenantID, subject)
	if err != nil {
		return nil, err
	}
	out := make([]WebPushSubscriptionView, 0, len(rows))
	for _, row := range rows {
		out = append(out, webPushSubscriptionView(row))
	}
	return out, nil
}

func (s *Service) UnregisterWebPushSubscription(ctx context.Context, browserID string) (bool, error) {
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
	browserID = strings.TrimSpace(browserID)
	if !deviceIDPattern.MatchString(browserID) {
		return false, fmt.Errorf("%w: browserId must be a safe opaque identifier", core.ErrBadRequest)
	}
	changed, err := s.Store.RevokeWebPushSubscription(ctx, tenantID, subject, browserID)
	if err != nil {
		return false, err
	}
	s.recordDeviceAudit(ctx, tenantID, auditUnregisterWebPush, browserID)
	return changed, nil
}

func normalizeWebPushInput(in RegisterWebPushInput) (RegisterWebPushInput, error) {
	in.BrowserID = strings.TrimSpace(in.BrowserID)
	in.Endpoint = strings.TrimSpace(in.Endpoint)
	in.P256dh = strings.TrimSpace(in.P256dh)
	in.Auth = strings.TrimSpace(in.Auth)
	if !deviceIDPattern.MatchString(in.BrowserID) {
		return RegisterWebPushInput{}, fmt.Errorf("%w: browserId must be a safe opaque identifier", core.ErrBadRequest)
	}
	if err := validatePublicPushEndpoint(in.Endpoint); err != nil {
		return RegisterWebPushInput{}, fmt.Errorf("%w: endpoint must be an HTTPS push URL", core.ErrBadRequest)
	}
	p256dh, err := decodeWebPushKey(in.P256dh)
	if err != nil || len(p256dh) != 65 || p256dh[0] != 0x04 {
		return RegisterWebPushInput{}, fmt.Errorf("%w: p256dh must be an uncompressed P-256 key", core.ErrBadRequest)
	}
	auth, err := decodeWebPushKey(in.Auth)
	if err != nil || len(auth) < 16 || len(auth) > 32 {
		return RegisterWebPushInput{}, fmt.Errorf("%w: auth must be a base64url secret", core.ErrBadRequest)
	}
	return in, nil
}

func validatePublicPushEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Host == "" || len(raw) < 8 || len(raw) > 2048 {
		return errors.New("invalid")
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1") {
		return nil
	}
	return errors.New("invalid")
}

func decodeWebPushKey(s string) ([]byte, error) {
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil && len(b) > 0 {
		return b, nil
	}
	return base64.URLEncoding.DecodeString(s)
}

func webPushSubscriptionView(row store.WebPushSubscription) WebPushSubscriptionView {
	return WebPushSubscriptionView{
		BrowserID: row.BrowserID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		LastRegisteredAt: row.LastRegisteredAt,
	}
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
