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

package webhooks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

func webhookREST(t *testing.T, svc *Service, method, target, body string) *httptest.ResponseRecorder {
	return webhookRESTWithHeaders(t, svc, method, target, body, nil)
}

func webhookRESTWithHeaders(t *testing.T, svc *Service, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func decodeObject(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response %d %q: %v", rr.Code, rr.Body.String(), err)
	}
	return got
}

func TestRenderRESTCreateReadPatchDeleteAndMintOnceFixture(t *testing.T) {
	svc, _ := newTestService()
	missingOwner := webhookREST(t, svc, http.MethodPost, "/v1/webhooks",
		`{"name":"missing-owner","url":"https://hooks.example.com/missing","enabled":true,"eventFilter":[]}`)
	if missingOwner.Code != http.StatusBadRequest || !strings.Contains(missingOwner.Body.String(), "WEBHOOK_OWNER_REQUIRED") {
		t.Fatalf("missing ownerId = %d %s", missingOwner.Code, missingOwner.Body.String())
	}
	createdRR := webhookREST(t, svc, http.MethodPost, "/v1/webhooks",
		`{"ownerId":"default","name":"deploys","url":"https://hooks.example.com/deploys","enabled":false,"eventFilter":[]}`)
	if createdRR.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", createdRR.Code, createdRR.Body.String())
	}
	created := decodeObject(t, createdRR)
	for _, key := range []string{"id", "name", "url", "enabled", "eventFilter", "secret"} {
		if _, ok := created[key]; !ok {
			t.Errorf("create response missing %q: %v", key, created)
		}
	}
	if len(created) != 6 || created["enabled"] != false || created["secret"] == "" {
		t.Fatalf("create response = %v, want exact Render fields + one-time secret", created)
	}
	id := created["id"].(string)

	getRR := webhookREST(t, svc, http.MethodGet, "/v1/webhooks/"+id, "")
	if getRR.Code != http.StatusOK {
		t.Fatalf("get = %d %s", getRR.Code, getRR.Body.String())
	}
	got := decodeObject(t, getRR)
	if _, leaked := got["secret"]; leaked || len(got) != 5 {
		t.Fatalf("get response = %v, secret must remain mint-once", got)
	}

	patchRR := webhookREST(t, svc, http.MethodPatch, "/v1/webhooks/"+id,
		`{"name":"deploys-renamed","enabled":true,"eventFilter":[]}`)
	if patchRR.Code != http.StatusOK {
		t.Fatalf("patch = %d %s", patchRR.Code, patchRR.Body.String())
	}
	patched := decodeObject(t, patchRR)
	if patched["name"] != "deploys-renamed" || patched["enabled"] != true || len(patched) != 5 {
		t.Fatalf("patch response = %v", patched)
	}

	deleteRR := webhookREST(t, svc, http.MethodDelete, "/v1/webhooks/"+id, "")
	if deleteRR.Code != http.StatusNoContent || deleteRR.Body.Len() != 0 {
		t.Fatalf("delete = %d %q", deleteRR.Code, deleteRR.Body.String())
	}
}

func TestRenderRESTWebhookListEnvelopeMultiOwnerAndCursor(t *testing.T) {
	svc, st := newTestService()
	base := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	for i, endpoint := range []store.WebhookEndpoint{
		{ID: "whk-a", TenantID: "tea-a", Name: "a", URL: "https://a.example/h", EventTypes: []string{}, Enabled: true},
		{ID: "whk-b", TenantID: "tea-b", Name: "b", URL: "https://b.example/h", EventTypes: []string{TypeDeployStarted}, Enabled: true},
		{ID: "whk-c", TenantID: "tea-a", Name: "c", URL: "https://c.example/h", EventTypes: []string{}, Enabled: false},
	} {
		endpoint.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		endpoint.UpdatedAt = endpoint.CreatedAt
		st.rows[endpoint.ID] = endpoint
	}

	first := webhookREST(t, svc, http.MethodGet, "/v1/webhooks?ownerId=tea-a,tea-b&ownerId[]=tea-a&limit=2", "")
	if first.Code != http.StatusOK {
		t.Fatalf("first page = %d %s", first.Code, first.Body.String())
	}
	var page1 []struct {
		Webhook map[string]any `json:"webhook"`
		Cursor  string         `json:"cursor"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &page1); err != nil || len(page1) != 2 {
		t.Fatalf("page1 = %+v, err %v", page1, err)
	}
	for _, item := range page1 {
		if item.Cursor == "" || item.Webhook["id"] == nil || item.Webhook["secret"] != nil {
			t.Fatalf("list item = %+v", item)
		}
	}

	secondTarget := "/v1/webhooks?ownerId=tea-a,tea-b&ownerId[]=tea-a&limit=2&cursor=" + url.QueryEscape(page1[1].Cursor)
	second := webhookREST(t, svc, http.MethodGet, secondTarget, "")
	var page2 []struct {
		Webhook map[string]any `json:"webhook"`
		Cursor  string         `json:"cursor"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &page2); err != nil || len(page2) != 1 {
		t.Fatalf("page2 = %+v (%d %s), err %v", page2, second.Code, second.Body.String(), err)
	}
	if page2[0].Webhook["id"] == page1[0].Webhook["id"] || page2[0].Webhook["id"] == page1[1].Webhook["id"] {
		t.Fatalf("cursor duplicated an endpoint: page1=%+v page2=%+v", page1, page2)
	}
}

func TestRenderRESTWebhookEventEnvelopeTimeFiltersAndEvidence(t *testing.T) {
	svc, st := newTestService()
	created, err := svc.Create(t.Context(), CreateRequest{
		Name: "events", URL: "https://events.example/h", EventTypes: []string{}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	for i := range 3 {
		sent := base.Add(time.Duration(i) * time.Hour)
		st.deliveries[created.ID] = append(st.deliveries[created.ID], store.WebhookAttempt{
			ID: "whd-" + string(rune('a'+i)), EndpointID: created.ID,
			NotificationID: "whd-parent", EventID: "evt-stable", EventType: TypeDeployEnded,
			Status: store.WebhookAttemptFailed, AttemptNumber: i + 1,
			StatusCode: 502, TransportError: "endpoint answered 502", ResponseBody: "upstream unavailable",
			SentAt: &sent, CreatedAt: sent,
		})
	}
	target := "/v1/webhooks/" + created.ID + "/events?sentAfter=" +
		url.QueryEscape(base.Add(30*time.Minute).Format(time.RFC3339)) + "&sentBefore=" +
		url.QueryEscape(base.Add(90*time.Minute).Format(time.RFC3339))
	rr := webhookREST(t, svc, http.MethodGet, target, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("events = %d %s", rr.Code, rr.Body.String())
	}
	var page []struct {
		WebhookEvent map[string]any `json:"webhookEvent"`
		Cursor       string         `json:"cursor"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &page); err != nil || len(page) != 1 {
		t.Fatalf("events page = %+v, err %v", page, err)
	}
	event := page[0].WebhookEvent
	for _, key := range []string{"id", "eventId", "eventType", "sentAt", "statusCode", "responseBody"} {
		if _, ok := event[key]; !ok {
			t.Errorf("webhookEvent missing %q: %v", key, event)
		}
	}
	if _, hasError := event["error"]; hasError {
		t.Fatalf("HTTP response must not also expose transport error: %v", event)
	}

	bad := webhookREST(t, svc, http.MethodGet, "/v1/webhooks/"+created.ID+"/events?sentAfter=not-a-time", "")
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad sentAfter = %d %s", bad.Code, bad.Body.String())
	}

	filtered := webhookREST(t, svc, http.MethodGet, "/v1/webhooks/"+created.ID+"/events?status=failed", "")
	if filtered.Code != http.StatusOK || !strings.Contains(filtered.Body.String(), `"eventId":"evt-stable"`) {
		t.Fatalf("failed filter = %d %s", filtered.Code, filtered.Body.String())
	}
}

func TestRESTWebhookResendRequiresKeyAndIsIdempotent(t *testing.T) {
	svc, st := newTestService()
	created, err := svc.Create(t.Context(), CreateRequest{
		Name: "resend", URL: "https://events.example/h", EventTypes: []string{}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	sent := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	st.deliveries[created.ID] = []store.WebhookAttempt{{
		ID: "whd-source", NotificationID: "whd-parent", EndpointID: created.ID,
		EventID: "evt-stable", EventType: TypeDeployEnded, ServiceID: "srv-1",
		Status: store.WebhookAttemptFailed, AttemptNumber: 1, StatusCode: http.StatusBadGateway,
		Payload: `{"type":"deploy_ended"}`, SentAt: &sent, CreatedAt: sent,
	}}
	path := "/v1/webhooks/" + created.ID + "/events/whd-source/resend"

	missing := webhookREST(t, svc, http.MethodPost, path, "")
	if missing.Code != http.StatusBadRequest || !strings.Contains(missing.Body.String(), WebhookResendIdempotencyKeyInvalidCode) {
		t.Fatalf("missing key = %d %s", missing.Code, missing.Body.String())
	}

	headers := map[string]string{"Idempotency-Key": "resend-key-0001"}
	first := webhookRESTWithHeaders(t, svc, http.MethodPost, path, "", headers)
	second := webhookRESTWithHeaders(t, svc, http.MethodPost, path, "", headers)
	if first.Code != http.StatusAccepted || second.Code != http.StatusAccepted {
		t.Fatalf("resend statuses = %d/%d: %s / %s", first.Code, second.Code, first.Body.String(), second.Body.String())
	}
	one, two := decodeObject(t, first), decodeObject(t, second)
	if one["id"] == "" || one["id"] != two["id"] || one["status"] != DeliveryPending || one["eventId"] != "evt-stable" {
		t.Fatalf("idempotent resend = first %v second %v", one, two)
	}
	if one["requestBody"] != `{"type":"deploy_ended"}` || one["attemptNumber"] != float64(2) {
		t.Fatalf("resend evidence = %v", one)
	}
}
