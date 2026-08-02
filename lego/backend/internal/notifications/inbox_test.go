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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func inboxFixture(t *testing.T) (*Service, *fakeStore, context.Context, string, string) {
	t.Helper()
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	st := newFakeStore()
	svc := newTestService(st, fakeWorkspace{"alice": "tea-a", "bob": "tea-a"}, nil, nil)
	svc.Clock = func() time.Time { return now }
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "oauth2"})
	ownID := ids.Derive(ids.Event, "own-unread")
	foreignID := ids.Derive(ids.Event, "foreign")
	readAt := now.Add(-time.Hour)
	st.push[[2]string{"tea-a", "alice"}] = []store.PushNotification{
		{
			TenantID: "tea-a", Subject: "alice", SourceEventKey: "secret-source-key",
			EventID: ownID, EventType: string(DeliveryEventDeployFailed), Title: "Deploy failed",
			Body: "Open the logs", Urgency: string(DeliveryUrgencyCritical), ResourceKind: "service",
			ResourceID: "srv-00000000000000000000", DeepLink: "bex://services/srv-00000000000000000000",
			OccurredAt: now.Add(-time.Minute), DeliverAt: now, CreatedAt: now,
		},
		{
			TenantID: "tea-a", Subject: "alice", SourceEventKey: "another-secret",
			EventID: ids.Derive(ids.Event, "own-read"), EventType: string(DeliveryEventDeploySucceeded),
			Title: "Deploy live", Body: "Healthy", Urgency: string(DeliveryUrgencyRoutine),
			ResourceKind: "service", ResourceID: "srv-00000000000000000000", DeepLink: "bex://services/srv-00000000000000000000",
			OccurredAt: now.Add(-2 * time.Hour), DeliverAt: now.Add(-2 * time.Hour), CreatedAt: now.Add(-2 * time.Hour), ReadAt: &readAt,
		},
	}
	st.push[[2]string{"tea-a", "bob"}] = []store.PushNotification{{
		TenantID: "tea-a", Subject: "bob", SourceEventKey: "bob-secret", EventID: foreignID,
		EventType: string(DeliveryEventDeployFailed), Title: "Bob only", Body: "Private",
		Urgency: string(DeliveryUrgencyCritical), OccurredAt: now, CreatedAt: now,
	}}
	return svc, st, ctx, ownID, foreignID
}

func TestNotificationInboxIsCallerScopedAndSafe(t *testing.T) {
	svc, _, ctx, ownID, foreignID := inboxFixture(t)

	items, err := svc.ListNotificationInbox(ctx, 1)
	if err != nil {
		t.Fatalf("ListNotificationInbox: %v", err)
	}
	if len(items) != 1 || items[0].ID != ownID {
		t.Fatalf("ListNotificationInbox = %+v, want caller's newest item", items)
	}
	wire, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("marshal projection: %v", err)
	}
	for _, forbidden := range []string{"secret-source-key", "sourceEventKey", "tenantId", "subject", "providerTicket", "token"} {
		if strings.Contains(string(wire), forbidden) {
			t.Errorf("caller projection leaked %q: %s", forbidden, wire)
		}
	}
	count, err := svc.UnreadPushNotificationCount(ctx)
	if err != nil || count != 1 {
		t.Fatalf("UnreadPushNotificationCount = %d, %v; want 1, nil", count, err)
	}

	changed, err := svc.MarkPushNotificationRead(ctx, ownID)
	if err != nil || !changed {
		t.Fatalf("MarkPushNotificationRead(own) = %v, %v; want true, nil", changed, err)
	}
	count, err = svc.UnreadPushNotificationCount(ctx)
	if err != nil || count != 0 {
		t.Fatalf("UnreadPushNotificationCount after read = %d, %v; want 0, nil", count, err)
	}
	foreign, foreignErr := svc.MarkPushNotificationRead(ctx, foreignID)
	unknown, unknownErr := svc.MarkPushNotificationRead(ctx, ids.Derive(ids.Event, "missing"))
	if foreignErr != nil || unknownErr != nil || foreign || unknown {
		t.Fatalf("foreign/missing read = (%v,%v) / (%v,%v); want identical false,nil", foreign, foreignErr, unknown, unknownErr)
	}
}

func TestNotificationInboxValidatesPublicInputs(t *testing.T) {
	svc, _, ctx, _, _ := inboxFixture(t)
	for _, limit := range []int{0, 101} {
		if _, err := svc.ListNotificationInbox(ctx, limit); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("ListNotificationInbox(%d) error = %v, want bad request", limit, err)
		}
	}
	if _, err := svc.MarkPushNotificationRead(ctx, "evt-not-canonical"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("MarkPushNotificationRead(malformed) error = %v, want bad request", err)
	}
}

func TestNotificationInboxREST(t *testing.T) {
	svc, _, ctx, ownID, foreignID := inboxFixture(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	request := func(method, target string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, target, nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	list := request(http.MethodGet, "/v1/notifications?limit=1")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), ownID) || strings.Contains(list.Body.String(), "secret-source-key") {
		t.Fatalf("GET inbox = %d %s", list.Code, list.Body.String())
	}
	if bad := request(http.MethodGet, "/v1/notifications?limit=101"); bad.Code != http.StatusBadRequest {
		t.Fatalf("GET invalid limit = %d, want 400", bad.Code)
	}
	if own := request(http.MethodPost, "/v1/notifications/"+ownID+"/read"); own.Code != http.StatusOK || own.Body.String() != "{\"read\":true}\n" {
		t.Fatalf("POST own read = %d %s", own.Code, own.Body.String())
	}
	foreign := request(http.MethodPost, "/v1/notifications/"+foreignID+"/read")
	missing := request(http.MethodPost, "/v1/notifications/"+ids.Derive(ids.Event, "rest-missing")+"/read")
	if foreign.Code != http.StatusOK || missing.Code != http.StatusOK || foreign.Body.String() != missing.Body.String() || foreign.Body.String() != "{\"read\":false}\n" {
		t.Fatalf("foreign/missing REST reads differ: %d %s / %d %s", foreign.Code, foreign.Body.String(), missing.Code, missing.Body.String())
	}
}

func TestNotificationInboxGraphQLContract(t *testing.T) {
	svc, _, ctx, ownID, foreignID := inboxFixture(t)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	result := graphql.Do(graphql.Params{
		Schema: schema, Context: ctx,
		RequestString: `{ notificationInbox(limit: 1) { id event title body urgency resourceKind resourceId deepLink occurredAt createdAt readAt } unreadPushNotificationCount }`,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("query errors: %v", result.Errors)
	}
	wire, _ := json.Marshal(result.Data)
	if !strings.Contains(string(wire), ownID) || strings.Contains(string(wire), "secret-source-key") {
		t.Fatalf("GraphQL inbox projection = %s", wire)
	}
	for _, forbidden := range []string{"tenantId", "subject", "sourceEventKey", "providerTicketId", "token"} {
		if _, ok := pushNotificationGQLType.Fields()[forbidden]; ok {
			t.Errorf("PushNotification unexpectedly exposes %s", forbidden)
		}
	}

	foreignResult := graphql.Do(graphql.Params{
		Schema: schema, Context: ctx,
		RequestString: `mutation { markPushNotificationRead(id: "` + foreignID + `") }`,
	})
	missingResult := graphql.Do(graphql.Params{
		Schema: schema, Context: ctx,
		RequestString: `mutation { markPushNotificationRead(id: "` + ids.Derive(ids.Event, "gql-missing") + `") }`,
	})
	if len(foreignResult.Errors) > 0 || len(missingResult.Errors) > 0 {
		t.Fatalf("foreign/missing mutation errors: %v / %v", foreignResult.Errors, missingResult.Errors)
	}
	foreignWire, _ := json.Marshal(foreignResult.Data)
	missingWire, _ := json.Marshal(missingResult.Data)
	if string(foreignWire) != string(missingWire) || string(foreignWire) != `{"markPushNotificationRead":false}` {
		t.Fatalf("foreign/missing GraphQL reads differ: %s / %s", foreignWire, missingWire)
	}
}
