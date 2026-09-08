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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/graphql-go/graphql"
)

func TestNotificationGraphQLSelectedWorkspace(t *testing.T) {
	svc, st, ctx, defaultID, _ := inboxFixture(t)
	svc.Base.Workspace = multiWorkspace{"alice": {"tea-a", "tea-b"}, "bob": {"tea-b"}}
	selectedID := ids.Derive(ids.Event, "selected-workspace")
	row := st.push[[2]string{"tea-a", "alice"}][0]
	row.TenantID, row.EventID = "tea-b", selectedID
	st.push[[2]string{"tea-b", "alice"}] = []store.PushNotification{row}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	run := func(c context.Context, query string) map[string]any {
		t.Helper()
		result := graphql.Do(graphql.Params{Schema: schema, Context: c, RequestString: query})
		if len(result.Errors) > 0 {
			t.Fatalf("query failed: %v", result.Errors)
		}
		return result.Data.(map[string]any)
	}
	data := run(ctx, `{ selected: notificationInbox(ownerId:"tea-b") { id } defaultInbox: notificationInbox { id } unreadPushNotificationCount(ownerId:"tea-b") }`)
	selected := data["selected"].([]any)
	if len(selected) != 1 || selected[0].(map[string]any)["id"] != selectedID {
		t.Fatalf("wrong selected inbox: %v", selected)
	}
	if len(data["defaultInbox"].([]any)) != 2 || data["unreadPushNotificationCount"] != 1 {
		t.Fatalf("default or count drift: %v", data)
	}
	if run(ctx, fmt.Sprintf(`mutation { markPushNotificationRead(ownerId:"tea-b", id:%q) }`, defaultID))["markPushNotificationRead"] != false {
		t.Fatal("changed default workspace item through selected context")
	}
	if run(ctx, fmt.Sprintf(`mutation { markPushNotificationRead(ownerId:"tea-b", id:%q) }`, selectedID))["markPushNotificationRead"] != true {
		t.Fatal("selected item not marked read")
	}
	if run(ctx, `{ unreadPushNotificationCount(ownerId:"tea-b") }`)["unreadPushNotificationCount"] != 0 {
		t.Fatal("selected badge did not update")
	}
	if run(ctx, `{ unreadPushNotificationCount }`)["unreadPushNotificationCount"] != 1 {
		t.Fatal("selected mark-read changed default badge")
	}
	bob := core.WithIdentity(context.Background(), core.Identity{Subject: "bob", Method: "oauth2"})
	if run(bob, fmt.Sprintf(`mutation { markPushNotificationRead(ownerId:"tea-b", id:%q) }`, selectedID))["markPushNotificationRead"] != false {
		t.Fatal("caller boundary crossed")
	}
	run(ctx, `mutation { registerNotificationDeviceSubscription(ownerId:"tea-b", deviceId:"phone-b", sessionId:"session", provider:"expo", platform:"ios", token:"ExponentPushToken[synthetic]") { deviceId } }`)
	data = run(ctx, `{ selected: notificationDeviceSubscriptions(ownerId:"tea-b") { deviceId } defaultDevices: notificationDeviceSubscriptions { deviceId } pushNotificationsAvailable(ownerId:"tea-b") }`)
	if len(data["selected"].([]any)) != 1 || len(data["defaultDevices"].([]any)) != 0 {
		t.Fatalf("device workspace mismatch: %v", data)
	}
	if run(ctx, `mutation { unregisterNotificationDeviceSubscription(deviceId:"phone-b") }`)["unregisterNotificationDeviceSubscription"] != false {
		t.Fatal("default unregister changed selected registration")
	}
	if run(ctx, `mutation { unregisterNotificationDeviceSubscription(ownerId:"tea-b", deviceId:"phone-b") }`)["unregisterNotificationDeviceSubscription"] != true {
		t.Fatal("selected registration not removed")
	}
	for _, query := range []string{
		`{ pushNotificationsAvailable(ownerId:%q) }`,
		`{ webPushAvailable(ownerId:%q) }`,
		`{ webPushVapidPublicKey(ownerId:%q) }`,
		`mutation { revokeNotificationDeviceSubscriptions(ownerId:%q) }`,
		`{ notificationDeviceSubscriptions(ownerId:%q) { deviceId } }`,
		`{ notificationInbox(ownerId:%q) { id } }`,
		`{ unreadPushNotificationCount(ownerId:%q) }`,
		fmt.Sprintf(`mutation { markPushNotificationRead(ownerId:%%q,id:%q) }`, selectedID),
		`mutation { unregisterNotificationDeviceSubscription(ownerId:%q,deviceId:"phone-b") }`,
		`mutation { registerNotificationDeviceSubscription(ownerId:%q,deviceId:"phone-b",sessionId:"session",provider:"expo",platform:"ios",token:"ExponentPushToken[synthetic]") {deviceId} }`,
	} {
		result := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: fmt.Sprintf(query, "tea-foreign")})
		if len(result.Errors) == 0 || !strings.Contains(strings.ToLower(result.Errors[0].Message), "forbidden") {
			t.Fatalf("nonmember workspace must be forbidden: %s: %v", query, result.Errors)
		}
	}
}

func TestNotificationRESTSelectedWorkspace(t *testing.T) {
	svc, st, ctx, defaultID, _ := inboxFixture(t)
	svc.Base.Workspace = multiWorkspace{"alice": {"tea-a", "tea-b"}}
	selectedID := ids.Derive(ids.Event, "rest-selected-workspace")
	row := st.push[[2]string{"tea-a", "alice"}][0]
	row.TenantID, row.EventID = "tea-b", selectedID
	st.push[[2]string{"tea-b", "alice"}] = []store.PushNotification{row}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	request := func(method, path, body string, status int) json.RawMessage {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		mux.ServeHTTP(res, req)
		if res.Code != status {
			t.Fatalf("%s %s: status %d, want %d: %s", method, path, res.Code, status, res.Body.String())
		}
		return res.Body.Bytes()
	}
	assertRows := func(path, expectedID string, count int) {
		t.Helper()
		var rows []map[string]any
		if err := json.Unmarshal(request("GET", path, "", 200), &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != count {
			t.Fatalf("%s: got %d rows, want %d", path, len(rows), count)
		}
		if expectedID != "" && rows[0]["id"] != expectedID {
			t.Fatalf("wrong workspace row: %v", rows)
		}
	}
	assertRows("/v1/notifications?ownerId=tea-b", selectedID, 1)
	assertRows("/v1/notifications", "", 2)
	if string(request("POST", "/v1/notifications/"+defaultID+"/read?ownerId=tea-b", "", 200)) != "{\"read\":false}\n" {
		t.Fatal("cross-workspace read acknowledged")
	}
	if string(request("POST", "/v1/notifications/"+selectedID+"/read?ownerId=tea-b", "", 200)) != "{\"read\":true}\n" {
		t.Fatal("selected read not acknowledged")
	}
	body := `{"deviceId":"phone-b","sessionId":"session","provider":"expo","platform":"ios","token":"ExponentPushToken[synthetic]"}`
	request("POST", "/v1/notification-device-subscriptions?ownerId=tea-b", body, 201)
	assertRows("/v1/notification-device-subscriptions?ownerId=tea-b", "", 1)
	assertRows("/v1/notification-device-subscriptions", "", 0)
	request("DELETE", "/v1/notification-device-subscriptions/phone-b", "", 200)
	assertRows("/v1/notification-device-subscriptions?ownerId=tea-b", "", 1)
	request("DELETE", "/v1/notification-device-subscriptions/phone-b?ownerId=tea-b", "", 200)
	assertRows("/v1/notification-device-subscriptions?ownerId=tea-b", "", 0)
	for _, route := range []struct{ method, path, body string }{
		{"GET", "/v1/notifications", ""},
		{"POST", "/v1/notifications/" + selectedID + "/read", ""},
		{"GET", "/v1/notification-device-subscriptions", ""},
		{"POST", "/v1/notification-device-subscriptions", body},
		{"DELETE", "/v1/notification-device-subscriptions/phone-b", ""},
		{"DELETE", "/v1/notification-device-subscriptions", ""},
		{"GET", "/v1/notification-settings/push/availability", ""},
	} {
		request(route.method, route.path+"?ownerId=tea-foreign", route.body, 403)
	}
}
