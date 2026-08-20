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
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const testPushToken = "ExponentPushToken[secret-device-capability]"

type auditRecorder struct {
	mu     sync.Mutex
	events []core.AuditEvent
}

func (r *auditRecorder) Record(_ context.Context, ev core.AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
	return nil
}

type multiWorkspace map[string][]string

func (m multiWorkspace) Tenant(_ context.Context, id core.Identity) (string, bool) {
	ids := m[id.Subject]
	return first(ids), len(ids) > 0
}

func (m multiWorkspace) IsMember(_ context.Context, id core.Identity, tenantID string) (bool, error) {
	for _, candidate := range m[id.Subject] {
		if candidate == tenantID {
			return true, nil
		}
	}
	return false, nil
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func subscriptionService(st NotificationsStore, ws core.WorkspaceResolver, audit core.AuditSink) *Service {
	return &Service{Base: &core.Base{Workspace: ws, Audit: audit}, Store: st}
}

func identity(subject string) context.Context {
	return core.WithIdentity(context.Background(), core.Identity{
		Subject: subject, Method: "oauth2", Human: true, PlatformClient: true,
		CanonicalScopes: core.ScopeRead + " " + core.ScopeWrite + " " + core.ScopeSensitive,
	})
}

func TestDeviceSubscriptionRegisterReplaceAndLogoutRevocation(t *testing.T) {
	st := newFakeStore()
	audit := &auditRecorder{}
	svc := subscriptionService(st, fakeWorkspace{"alice": "tea-a"}, audit)
	ctx := identity("alice")

	created, err := svc.RegisterDeviceSubscription(ctx, RegisterDeviceInput{
		DeviceID: " device-ios ", SessionID: " session-a ", Provider: " EXPO ", Platform: " IOS ", Token: " " + testPushToken + " ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.DeviceID != "device-ios" || created.Provider != "expo" || created.Platform != "ios" {
		t.Fatalf("normalized subscription = %+v", created)
	}
	encoded, _ := json.Marshal(created)
	if strings.Contains(string(encoded), "secret-device-capability") || strings.Contains(string(encoded), "token") {
		t.Fatalf("public subscription leaked token material: %s", encoded)
	}

	replaced, err := svc.RegisterDeviceSubscription(ctx, RegisterDeviceInput{
		DeviceID: "device-ios", SessionID: "session-a", Provider: "expo", Platform: "ios", Token: "ExponentPushToken[rotated]",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !replaced.CreatedAt.Equal(created.CreatedAt) || replaced.LastRegisteredAt.Before(created.LastRegisteredAt) {
		t.Fatalf("replacement timestamps created=%s/%s registered=%s/%s", created.CreatedAt, replaced.CreatedAt, created.LastRegisteredAt, replaced.LastRegisteredAt)
	}
	listed, err := svc.ListDeviceSubscriptions(ctx)
	if err != nil || len(listed) != 1 || listed[0].DeviceID != "device-ios" {
		t.Fatalf("own devices = %+v err=%v", listed, err)
	}

	count, err := svc.RevokeDeviceSubscriptions(ctx)
	if err != nil || count != 1 {
		t.Fatalf("logout revoke = %d err=%v", count, err)
	}
	if count, err = svc.RevokeDeviceSubscriptions(ctx); err != nil || count != 0 {
		t.Fatalf("idempotent logout revoke = %d err=%v", count, err)
	}
	listed, _ = svc.ListDeviceSubscriptions(ctx)
	if len(listed) != 0 {
		t.Fatalf("devices after logout = %+v", listed)
	}
	for _, ev := range audit.events {
		if strings.Contains(ev.Verb+ev.Resource+ev.Target, "ExponentPushToken") || strings.Contains(ev.Verb+ev.Resource+ev.Target, "rotated") {
			t.Fatalf("audit leaked token: %+v", ev)
		}
	}
}

func TestDeviceSubscriptionsIsolateSubjectAndWorkspace(t *testing.T) {
	st := newFakeStore()
	ws := multiWorkspace{"alice": {"tea-a", "tea-b"}, "bob": {"tea-a"}}
	svc := subscriptionService(st, ws, nil)
	alice := identity("alice")
	bob := identity("bob")

	register := func(ctx context.Context, deviceID, token string) {
		t.Helper()
		if _, err := svc.RegisterDeviceSubscription(ctx, RegisterDeviceInput{
			DeviceID: deviceID, SessionID: "session-a", Provider: "expo", Platform: "android", Token: token,
		}); err != nil {
			t.Fatal(err)
		}
	}
	register(alice, "alice-a", "ExponentPushToken[alice-a]")
	register(core.WithWorkspace(alice, "tea-b"), "alice-b", "ExponentPushToken[alice-b]")
	register(bob, "bob-a", "ExponentPushToken[bob-a]")

	assertDevices := func(ctx context.Context, want string) {
		t.Helper()
		got, err := svc.ListDeviceSubscriptions(ctx)
		if err != nil || len(got) != 1 || got[0].DeviceID != want {
			t.Fatalf("devices for %s = %+v err=%v", want, got, err)
		}
	}
	assertDevices(alice, "alice-a")
	assertDevices(core.WithWorkspace(alice, "tea-b"), "alice-b")
	assertDevices(bob, "bob-a")

	// Bob cannot probe/revoke Alice's device even inside the shared workspace.
	changed, err := svc.UnregisterDeviceSubscription(bob, "alice-a")
	if err != nil || changed {
		t.Fatalf("cross-subject revoke changed=%v err=%v", changed, err)
	}
	assertDevices(alice, "alice-a")
}

func TestDeviceTokenAccountSwitchRevokesPriorOwner(t *testing.T) {
	st := newFakeStore()
	svc := subscriptionService(st, fakeWorkspace{"alice": "tea-a", "bob": "tea-a"}, nil)
	input := RegisterDeviceInput{DeviceID: "shared-device", SessionID: "session-a", Provider: "expo", Platform: "ios", Token: testPushToken}
	if _, err := svc.RegisterDeviceSubscription(identity("alice"), input); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RegisterDeviceSubscription(identity("bob"), input); err != nil {
		t.Fatal(err)
	}
	alice, _ := svc.ListDeviceSubscriptions(identity("alice"))
	bob, _ := svc.ListDeviceSubscriptions(identity("bob"))
	if len(alice) != 0 || len(bob) != 1 {
		t.Fatalf("account switch devices alice=%+v bob=%+v", alice, bob)
	}
}

func TestDeviceRegistrationValidationIsBoundedAndRedacted(t *testing.T) {
	svc := subscriptionService(newFakeStore(), fakeWorkspace{"alice": "tea-a"}, nil)
	ctx := identity("alice")
	for name, input := range map[string]RegisterDeviceInput{
		"device":         {DeviceID: strings.Repeat("d", maxDeviceIDBytes+1), SessionID: "session-a", Provider: "expo", Platform: "ios", Token: testPushToken},
		"device control": {DeviceID: "device\ninjected", SessionID: "session-a", Provider: "expo", Platform: "ios", Token: testPushToken},
		"session":        {DeviceID: "device", SessionID: "bad session", Provider: "expo", Platform: "ios", Token: testPushToken},
		"provider":       {DeviceID: "device", SessionID: "session-a", Provider: "apns", Platform: "ios", Token: testPushToken},
		"platform":       {DeviceID: "device", SessionID: "session-a", Provider: "expo", Platform: "web", Token: testPushToken},
		"token":          {DeviceID: "device", SessionID: "session-a", Provider: "expo", Platform: "ios", Token: strings.Repeat("s", maxPushTokenBytes+1)},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := svc.RegisterDeviceSubscription(ctx, input)
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("error = %v, want bad request", err)
			}
			if strings.Contains(err.Error(), input.Token) && len(input.Token) > 0 {
				t.Fatalf("error leaked token: %v", err)
			}
		})
	}
}

type registrationErrorStore struct {
	NotificationsStore
	err error
}

func (s registrationErrorStore) UpsertDevicePushSubscription(context.Context, store.DevicePushSubscription) (store.DevicePushSubscription, error) {
	return store.DevicePushSubscription{}, s.err
}

func TestDeviceRegistrationMapsQuotaErrors(t *testing.T) {
	for _, test := range []struct {
		err  error
		code string
	}{
		{store.ErrPushDeviceSubjectLimit, "PUSH_DEVICE_SUBJECT_LIMIT"},
		{store.ErrPushDeviceWorkspaceLimit, "PUSH_DEVICE_WORKSPACE_LIMIT"},
	} {
		base := newFakeStore()
		svc := subscriptionService(registrationErrorStore{NotificationsStore: base, err: test.err}, fakeWorkspace{"alice": "tea-a"}, nil)
		_, err := svc.RegisterDeviceSubscription(identity("alice"), RegisterDeviceInput{
			DeviceID: "device", SessionID: "session-a", Provider: "expo", Platform: "ios", Token: testPushToken,
		})
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != test.code || !errors.Is(err, core.ErrConflict) {
			t.Fatalf("quota error = %#v, want conflict code %s", err, test.code)
		}
	}
}

func TestPushAvailabilityDisablesRegistrationButNotCleanup(t *testing.T) {
	available := false
	st := newFakeStore()
	svc := subscriptionService(st, fakeWorkspace{"alice": "tea-a"}, nil)
	svc.PushAvailable = &available
	ctx := identity("alice")

	got, err := svc.IsPushAvailable(ctx)
	if err != nil || got {
		t.Fatalf("IsPushAvailable() = (%v, %v), want false", got, err)
	}
	_, err = svc.RegisterDeviceSubscription(ctx, RegisterDeviceInput{
		DeviceID: "device", SessionID: "session-a", Provider: "expo", Platform: "ios", Token: testPushToken,
	})
	if !errors.Is(err, core.ErrPushUnavailable) {
		t.Fatalf("RegisterDeviceSubscription() error = %v, want push unavailable", err)
	}
	if count, cleanupErr := svc.RevokeDeviceSubscriptions(ctx); cleanupErr != nil || count != 0 {
		t.Fatalf("cleanup while disabled = (%d, %v)", count, cleanupErr)
	}
}

func TestWebPushRegistrationIsSecretFreeAndIndependentOfExpo(t *testing.T) {
	st := newFakeStore()
	audit := &auditRecorder{}
	svc := subscriptionService(st, fakeWorkspace{"alice": "tea-a"}, audit)
	ctx := identity("alice")
	webOn := true
	expoOff := false
	svc.WebPushAvailable = &webOn
	svc.PushAvailable = &expoOff

	ua, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p256dh := base64.RawURLEncoding.EncodeToString(ua.PublicKey().Bytes())
	auth := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{3}, 16))
	endpoint := "https://fcm.googleapis.com/fcm/send/secret-endpoint"

	created, err := svc.RegisterWebPushSubscription(ctx, RegisterWebPushInput{
		BrowserID: "wp-browser", Endpoint: endpoint, P256dh: p256dh, Auth: auth,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(created)
	if strings.Contains(string(raw), "secret-endpoint") || strings.Contains(string(raw), p256dh) || strings.Contains(string(raw), `"auth"`) {
		t.Fatalf("webpush view leaked capability: %s", raw)
	}
	listed, err := svc.ListWebPushSubscriptions(ctx)
	if err != nil || len(listed) != 1 || listed[0].BrowserID != "wp-browser" {
		t.Fatalf("list = %+v err=%v", listed, err)
	}
	changed, err := svc.UnregisterWebPushSubscription(ctx, "wp-browser")
	if err != nil || !changed {
		t.Fatalf("unregister = (%v, %v)", changed, err)
	}
}

func TestWebPushAvailabilityDisablesRegistrationIndependently(t *testing.T) {
	off := false
	svc := subscriptionService(newFakeStore(), fakeWorkspace{"alice": "tea-a"}, nil)
	svc.WebPushAvailable = &off
	ctx := identity("alice")
	got, err := svc.IsWebPushAvailable(ctx)
	if err != nil || got {
		t.Fatalf("IsWebPushAvailable = (%v, %v)", got, err)
	}
	_, err = svc.RegisterWebPushSubscription(ctx, RegisterWebPushInput{
		BrowserID: "wp-browser", Endpoint: "https://fcm.googleapis.com/fcm/send/x",
		P256dh: "BNxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		Auth:   "dGVzdGF1dGhzZWNyZXQxNg",
	})
	if !errors.Is(err, core.ErrWebPushUnavailable) {
		t.Fatalf("register error = %v, want webpush unavailable", err)
	}
}
