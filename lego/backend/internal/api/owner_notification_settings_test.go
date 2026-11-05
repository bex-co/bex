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

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/notifications"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// ownerPolicyStore is the owner-policy half of the notification store; the
// embedded interface is nil, so any other notification verb would panic.
type ownerPolicyStore struct {
	notifications.NotificationsStore
	mu   sync.Mutex
	rows map[string]store.NotificationOwnerPolicy
}

func (s *ownerPolicyStore) GetNotificationOwnerPolicy(_ context.Context, workspaceID string) (store.NotificationOwnerPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[workspaceID]
	if !ok {
		return store.NotificationOwnerPolicy{}, store.ErrNotFound
	}
	return row, nil
}

func (s *ownerPolicyStore) UpsertNotificationOwnerPolicy(_ context.Context, workspaceID string, patch store.NotificationOwnerPolicyPatch) (store.NotificationOwnerPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[workspaceID]
	if !ok {
		row = store.NotificationOwnerPolicy{WorkspaceID: workspaceID, EmailEnabled: true, NotificationsToSend: "failure"}
	}
	if patch.EmailEnabled != nil {
		row.EmailEnabled = *patch.EmailEnabled
	}
	if patch.NotificationsToSend != nil {
		row.NotificationsToSend = *patch.NotificationsToSend
	}
	row.UpdatedAt = time.Now()
	s.rows[workspaceID] = row
	return row, nil
}

// TestOwnerNotificationSettingsRenderConformance (w3/m271): Render's two owner
// notification-settings operations answer in Render's shape on the pinned
// contract, refuse previewNotificationsEnabled:true by name, and GraphQL and
// MCP serve the same object.
func TestOwnerNotificationSettingsRenderConformance(t *testing.T) {
	spec := loadRenderSpec(t)
	st := &ownerPolicyStore{rows: map[string]store.NotificationOwnerPolicy{}}
	base := &core.Base{Client: fakeClient(), Namespace: "default", Workspace: fakeWorkspace{"client-1": "tea-cli"}}
	h, srv := serverWith(t, base, Deps{NotificationsStore: st})
	path := "/v1/notification-settings/owners/tea-cli"

	read := func() map[string]any {
		t.Helper()
		w := do(t, h, http.MethodGet, path, testToken, "")
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s = %d %s", path, w.Code, w.Body)
		}
		if errs := spec.validate("retrieve-owner-notification-settings", w.Body.Bytes()); len(errs) > 0 {
			t.Fatalf("retrieve-owner-notification-settings diverges from Render: %v\n%s", errs, w.Body)
		}
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	want := map[string]any{"ownerId": "tea-cli", "slackEnabled": false, "emailEnabled": true, "previewNotificationsEnabled": false, "notificationsToSend": "failure"}
	if got := read(); !reflect.DeepEqual(got, want) {
		t.Fatalf("unsaved policy = %v, want Render's default %v", got, want)
	}

	w := do(t, h, http.MethodPatch, path, testToken, `{"emailEnabled":false,"notificationsToSend":"all","previewNotificationsEnabled":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH = %d %s", w.Code, w.Body)
	}
	if errs := spec.validate("patch-owner-notification-settings", w.Body.Bytes()); len(errs) > 0 {
		t.Fatalf("patch-owner-notification-settings diverges from Render: %v\n%s", errs, w.Body)
	}
	want["emailEnabled"], want["notificationsToSend"] = false, "all"
	if got := read(); !reflect.DeepEqual(got, want) {
		t.Fatalf("after PATCH = %v, want %v", got, want)
	}

	for body, field := range map[string]string{
		`{"previewNotificationsEnabled":true}`: "previewNotificationsEnabled",
		`{"notificationsToSend":"default"}`:    "notificationsToSend",
		// Not in Render's PATCH body: the Render-contract request gate
		// refuses it rather than silently ignoring a Slack opt-in.
		`{"slackEnabled":true}`: "slackEnabled",
	} {
		w := do(t, h, http.MethodPatch, path, testToken, body)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), field) {
			t.Fatalf("PATCH %s = %d %s, want a 400 naming %s", body, w.Code, w.Body, field)
		}
	}
	if got := read(); !reflect.DeepEqual(got, want) {
		t.Fatalf("a refused PATCH changed the policy to %v", got)
	}

	// GraphQL serves the same fields with the same values.
	data := gql(t, h, `{ ownerNotificationSettings(ownerId: "tea-cli") { ownerId slackEnabled emailEnabled previewNotificationsEnabled notificationsToSend } }`)
	if got := data["ownerNotificationSettings"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("GraphQL ownerNotificationSettings = %v, want the REST object %v", got, want)
	}
	data = gql(t, h, `mutation { updateOwnerNotificationSettings(ownerId: "tea-cli", input: {notificationsToSend: "none"}) { notificationsToSend emailEnabled } }`)
	if got := data["updateOwnerNotificationSettings"]; !reflect.DeepEqual(got, map[string]any{"notificationsToSend": "none", "emailEnabled": false}) {
		t.Fatalf("GraphQL update = %v", got)
	}
	want["notificationsToSend"] = "none"

	// MCP serves the same object, and refuses previews the same way.
	cs := mcpSessionAs(t, srv, "client-1")
	got := callTool[map[string]any](t, cs, "get_owner_notification_settings", map[string]any{"workspaceId": "tea-cli"})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MCP get_owner_notification_settings = %v, want %v", got, want)
	}
	callToolError(t, cs, "update_owner_notification_settings", map[string]any{"workspaceId": "tea-cli", "previewNotificationsEnabled": true})
	got = callTool[map[string]any](t, cs, "update_owner_notification_settings", map[string]any{"workspaceId": "tea-cli", "emailEnabled": true})
	want["emailEnabled"] = true
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MCP update_owner_notification_settings = %v, want %v", got, want)
	}
}
