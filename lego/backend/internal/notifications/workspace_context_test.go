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
