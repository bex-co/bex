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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func byIDInboxFixture(t *testing.T) (*Service, *fakeStore, string, string) {
	t.Helper()
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	st := newFakeStore()
	svc := newTestService(st, coretest.Members{"alice": {"tea-a", "tea-b"}, "mallory": {"tea-b"}}, nil, nil)
	svc.Clock = func() time.Time { return now }
	aliceB := ids.Derive(ids.Event, "byid-alice-b")
	malloryB := ids.Derive(ids.Event, "byid-mallory-b")
	item := func(tenant, subject, eventID string) store.PushNotification {
		return store.PushNotification{
			TenantID: tenant, Subject: subject, SourceEventKey: eventID, EventID: eventID,
			EventType: string(DeliveryEventDeployFailed), Title: "Deploy failed", Body: "Open the logs",
			Urgency: string(DeliveryUrgencyCritical), OccurredAt: now, DeliverAt: now, CreatedAt: now,
		}
	}
	st.push[[2]string{"tea-b", "alice"}] = []store.PushNotification{item("tea-b", "alice", aliceB)}
	st.push[[2]string{"tea-b", "mallory"}] = []store.PushNotification{item("tea-b", "mallory", malloryB)}
	// One unread item in alice's default inbox, so a routed read that strays
	// into tea-a is observable.
	st.push[[2]string{"tea-a", "alice"}] = []store.PushNotification{item("tea-a", "alice", ids.Derive(ids.Event, "byid-alice-a"))}
	return svc, st, aliceB, malloryB
}

func pushReadAt(st *fakeStore, tenant, subject, eventID string) *time.Time {
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, row := range st.push[[2]string{tenant, subject}] {
		if row.EventID == eventID {
			return row.ReadAt
		}
	}
	return nil
}

// TestMarkReadRoutesToTheNotificationsOwnWorkspace is w4/m172 for
// POST /v1/notifications/{id}/read: a member of [tea-a (default), tea-b] marks
// their tea-b item read with no ownerId; a mismatched explicit ownerId still
// answers false; another member's id and an unknown id answer the same false
// and leave every row untouched.
func TestMarkReadRoutesToTheNotificationsOwnWorkspace(t *testing.T) {
	svc, st, aliceB, malloryB := byIDInboxFixture(t)
	alice := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "oauth2"})

	if read, err := svc.MarkPushNotificationRead(core.WithWorkspace(alice, "tea-a"), aliceB); err != nil || read {
		t.Fatalf("mismatched ownerId = %v, %v; want false, nil", read, err)
	}
	if pushReadAt(st, "tea-b", "alice", aliceB) != nil {
		t.Fatal("mismatched ownerId marked the tea-b item read")
	}

	foreign, foreignErr := svc.MarkPushNotificationRead(alice, malloryB)
	unknown, unknownErr := svc.MarkPushNotificationRead(alice, ids.Derive(ids.Event, "byid-missing"))
	if foreign || unknown || foreignErr != nil || unknownErr != nil {
		t.Fatalf("foreign/unknown = (%v,%v) / (%v,%v); want identical false,nil", foreign, foreignErr, unknown, unknownErr)
	}
	if pushReadAt(st, "tea-b", "mallory", malloryB) != nil {
		t.Fatal("caller marked another member's notification read")
	}

	if read, err := svc.MarkPushNotificationRead(alice, aliceB); err != nil || !read {
		t.Fatalf("no ownerId = %v, %v; want true, nil", read, err)
	}
	if pushReadAt(st, "tea-b", "alice", aliceB) == nil {
		t.Fatal("tea-b item still unread")
	}
	// The default workspace's inbox is untouched by the routed read.
	count, err := svc.UnreadPushNotificationCount(core.WithWorkspace(alice, "tea-a"))
	if err != nil || count != 1 {
		t.Fatalf("tea-a unread = %d, %v; want its one item still unread", count, err)
	}
}

// TestMarkReadRESTWithoutOwnerID drives the same routing through the REST
// adapter, which only adds ownerId when the query names one.
func TestMarkReadRESTWithoutOwnerID(t *testing.T) {
	svc, st, aliceB, _ := byIDInboxFixture(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "alice", Method: "oauth2"})
	post := func(target string) string {
		req := httptest.NewRequest(http.MethodPost, target, nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST %s = %d %s", target, rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}
	if body := post("/v1/notifications/" + aliceB + "/read?ownerId=tea-a"); body != "{\"read\":false}\n" {
		t.Fatalf("mismatched ownerId body = %q", body)
	}
	if body := post("/v1/notifications/" + aliceB + "/read"); body != "{\"read\":true}\n" {
		t.Fatalf("no ownerId body = %q", body)
	}
	if pushReadAt(st, "tea-b", "alice", aliceB) == nil {
		t.Fatal("REST mark-read left the tea-b item unread")
	}
}
