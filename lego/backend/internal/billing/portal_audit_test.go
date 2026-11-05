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

package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// customerUpdatedEvent is a customer.updated event whose cus_1 snapshot has
// the given metadata (raw JSON) and default method, with previous_attributes
// prev (raw JSON), created at the given time.
func customerUpdatedEvent(t *testing.T, eventID, prev string, created time.Time, metadata, defaultPM string) *stripe.Event {
	t.Helper()
	var event stripe.Event
	raw := fmt.Sprintf(`{"id":%q,"object":"event","type":"customer.updated","created":%d,"data":{"object":{"id":"cus_1","object":"customer","metadata":%s,"invoice_settings":{"default_payment_method":%q}},"previous_attributes":%s}}`, eventID, created.Unix(), metadata, defaultPM, prev)
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("event: %v", err)
	}
	return &event
}

// portalCustomerStub serves cus_1 with the given metadata and default method.
func portalCustomerStub(t *testing.T, metadata, defaultPM string) (*StripeClient, *stripeStub) {
	t.Helper()
	return newStripeTest(t, func(method, path string) (int, string) {
		if method == http.MethodGet && path == "/v1/customers/cus_1" {
			return 200, fmt.Sprintf(`{"id":"cus_1","object":"customer","metadata":%s,"invoice_settings":{"default_payment_method":%q}}`, metadata, defaultPM)
		}
		return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
	})
}

const changedDefaultPM = `{"invoice_settings":{"default_payment_method":"pm_old"}}`

// A card changed in the portal is audited once per event, attributed to the
// member who opened the portal (w2/070).
func TestPortalPaymentMethodChangeIsAuditedToTheOpener(t *testing.T) {
	now := time.Now()
	metadata := fmt.Sprintf(`{"bex_workspace":"tea-a","bex_bound_payment_method":"pm_old","bex_portal_actor":"usr-member","bex_portal_actor_method":"session","bex_portal_opened_at":"%d"}`, now.Add(-5*time.Minute).Unix())
	c, _ := portalCustomerStub(t, metadata, "pm_new")
	c.storeCustomer("tea-a", "cus_1")
	audit := &paymentMethodAuditFake{}
	c.audit = audit
	event := customerUpdatedEvent(t, "evt_portal", changedDefaultPM, now, metadata, "pm_new")
	for i := 0; i < 2; i++ {
		if err := c.RecordPortalPaymentMethodChange(context.Background(), event); err != nil {
			t.Fatalf("delivery %d: %v", i+1, err)
		}
	}
	if len(audit.events) != 2 || audit.events[0].ID == "" || audit.events[0].ID != audit.events[1].ID {
		t.Fatalf("audits = %+v, want two deliveries carrying one derived id", audit.events)
	}
	if ev := audit.events[0]; ev.Verb != core.AuditVerbUpdatePaymentMethod || ev.Caller != "usr-member" || ev.CallerMethod != "session" ||
		ev.Resource != core.WorkspaceObject("tea-a") || ev.Outcome != core.AuditAllowed {
		t.Fatalf("audit = %+v", ev)
	}
}

// Past the attribution window, or with no opener recorded, the change is the
// system's (Stripe staff or the API changed it, not a portal visit).
func TestPortalPaymentMethodChangeOutsideTheWindowIsTheSystems(t *testing.T) {
	now := time.Now()
	for name, metadata := range map[string]string{
		"stale opener": fmt.Sprintf(`{"bex_workspace":"tea-a","bex_portal_actor":"usr-member","bex_portal_actor_method":"session","bex_portal_opened_at":"%d"}`, now.Add(-2*time.Hour).Unix()),
		"no opener":    `{"bex_workspace":"tea-a"}`,
	} {
		t.Run(name, func(t *testing.T) {
			c, _ := portalCustomerStub(t, metadata, "pm_new")
			c.storeCustomer("tea-a", "cus_1")
			audit := &paymentMethodAuditFake{}
			c.audit = audit
			if err := c.RecordPortalPaymentMethodChange(context.Background(), customerUpdatedEvent(t, "evt_1", changedDefaultPM, now, metadata, "pm_new")); err != nil {
				t.Fatal(err)
			}
			if len(audit.events) != 1 || audit.events[0].Caller != "" || audit.events[0].CallerMethod != "" {
				t.Fatalf("audits = %+v, want one system row", audit.events)
			}
		})
	}
}

// Events that are not a bex workspace's card change record nothing: another
// field changed, the Customer belongs to another product on the shared
// account, it is not the workspace's current Customer, or bex's own Checkout
// bound this method and already audited it.
func TestPortalPaymentMethodChangeIgnoresUnrelatedEvents(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name, prev, metadata, defaultPM string
	}{
		{"other field", `{"email":"old@example.com"}`, `{"bex_workspace":"tea-a"}`, "pm_new"},
		{"metadata only", `{"metadata":{"bex_portal_opened_at":"1"}}`, `{"bex_workspace":"tea-a"}`, "pm_new"},
		{"foreign customer", changedDefaultPM, `{"other_product":"x"}`, "pm_new"},
		{"not the workspace's customer", changedDefaultPM, `{"bex_workspace":"tea-b"}`, "pm_new"},
		{"tombstoned workspace", changedDefaultPM, `{"bex_workspace":"tea-a","bex_deleted_at":"2026-11-01T00:00:00Z"}`, "pm_new"},
		{"checkout bind", `{"invoice_settings":{"default_payment_method":"pm_old"},"metadata":{"bex_bound_payment_method":"pm_old"}}`, `{"bex_workspace":"tea-a","bex_bound_payment_method":"pm_new"}`, "pm_new"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := portalCustomerStub(t, tc.metadata, tc.defaultPM)
			c.storeCustomer("tea-a", "cus_1")
			c.storeCustomer("tea-b", "cus_other")
			audit := &paymentMethodAuditFake{}
			c.audit = audit
			if err := c.RecordPortalPaymentMethodChange(context.Background(), customerUpdatedEvent(t, "evt_1", tc.prev, now, tc.metadata, tc.defaultPM)); err != nil {
				t.Fatal(err)
			}
			if len(audit.events) != 0 {
				t.Fatalf("audits = %+v, want none", audit.events)
			}
		})
	}
}

// Switching back to a Checkout-bound card in the portal is still a change the
// member made: only the Checkout's own write (which also moves the bound
// marker) is skipped.
func TestPortalSwitchBackToTheCheckoutCardIsAudited(t *testing.T) {
	metadata := `{"bex_workspace":"tea-a","bex_bound_payment_method":"pm_a"}`
	c, _ := portalCustomerStub(t, metadata, "pm_a")
	c.storeCustomer("tea-a", "cus_1")
	audit := &paymentMethodAuditFake{}
	c.audit = audit
	if err := c.RecordPortalPaymentMethodChange(context.Background(), customerUpdatedEvent(t, "evt_back", `{"invoice_settings":{"default_payment_method":"pm_b"}}`, time.Now(), metadata, "pm_a")); err != nil {
		t.Fatal(err)
	}
	if len(audit.events) != 1 {
		t.Fatalf("audits = %+v, want one", audit.events)
	}
}

// Attribution reads the event's snapshot: a later portal visit by another
// member, already on the live Customer, does not take the credit.
func TestPortalAttributionUsesTheEventSnapshot(t *testing.T) {
	now := time.Now()
	opened := func(actor string, at time.Time) string {
		return fmt.Sprintf(`{"bex_workspace":"tea-a","bex_portal_actor":%q,"bex_portal_actor_method":"session","bex_portal_opened_at":"%d"}`, actor, at.Unix())
	}
	c, _ := portalCustomerStub(t, opened("usr-later", now.Add(time.Minute)), "pm_new")
	c.storeCustomer("tea-a", "cus_1")
	audit := &paymentMethodAuditFake{}
	c.audit = audit
	if err := c.RecordPortalPaymentMethodChange(context.Background(), customerUpdatedEvent(t, "evt_1", changedDefaultPM, now, opened("usr-member", now.Add(-time.Minute)), "pm_new")); err != nil {
		t.Fatal(err)
	}
	if len(audit.events) != 1 || audit.events[0].Caller != "usr-member" {
		t.Fatalf("audits = %+v, want usr-member", audit.events)
	}
}

// A malformed event is acknowledged rather than retried for days.
func TestPortalPaymentMethodChangeAcknowledgesAMalformedEvent(t *testing.T) {
	c, stub := portalCustomerStub(t, `{}`, "")
	c.audit = &paymentMethodAuditFake{}
	var event stripe.Event
	if err := json.Unmarshal([]byte(`{"id":"evt_bad","type":"customer.updated","data":{"object":{"object":"customer"},"previous_attributes":`+changedDefaultPM+`}}`), &event); err != nil {
		t.Fatal(err)
	}
	if err := c.RecordPortalPaymentMethodChange(context.Background(), &event); err != nil || len(stub.hits) != 0 {
		t.Fatalf("err = %v hits = %v, want acknowledged without Stripe calls", err, stub.hits)
	}
}

// A Customer deleted since the event is acknowledged, not retried forever.
func TestPortalPaymentMethodChangeOnAMissingCustomerIsAcknowledged(t *testing.T) {
	c, _ := newStripeTest(t, func(string, string) (int, string) {
		return 404, `{"error":{"type":"invalid_request_error","code":"resource_missing","message":"No such customer"}}`
	})
	audit := &paymentMethodAuditFake{}
	c.audit = audit
	if err := c.RecordPortalPaymentMethodChange(context.Background(), customerUpdatedEvent(t, "evt_1", changedDefaultPM, time.Now(), `{"bex_workspace":"tea-a"}`, "pm_new")); err != nil || len(audit.events) != 0 {
		t.Fatalf("err = %v audits = %+v, want acknowledged and none", err, audit.events)
	}
}

// A failed audit write fails the delivery so Stripe retries it.
func TestPortalPaymentMethodChangeFailsWhenTheAuditCannotBeWritten(t *testing.T) {
	c, _ := portalCustomerStub(t, `{"bex_workspace":"tea-a"}`, "pm_new")
	c.storeCustomer("tea-a", "cus_1")
	c.audit = &paymentMethodAuditFake{err: errors.New("audit store down")}
	err := c.RecordPortalPaymentMethodChange(context.Background(), customerUpdatedEvent(t, "evt_1", changedDefaultPM, time.Now(), `{"bex_workspace":"tea-a"}`, "pm_new"))
	if err == nil || !strings.Contains(err.Error(), "audit payment-method change") {
		t.Fatalf("err = %v, want the audit failure", err)
	}
}

// Opening the portal records the opener on the Customer, and a failure to do
// so does not cost the member the portal.
func TestCreatePortalSessionRecordsTheOpener(t *testing.T) {
	for _, metadataStatus := range []int{200, 500} {
		t.Run(fmt.Sprint(metadataStatus), func(t *testing.T) {
			c, stub := newStripeTest(t, func(method, path string) (int, string) {
				switch {
				case method == http.MethodGet && path == "/v1/subscriptions":
					return 200, `{"object":"list","data":[{"id":"sub_1","object":"subscription","status":"active","metadata":{"bex_workspace":"tea-a","bex_billing_contract":"true"}}],"has_more":false,"url":"/v1/subscriptions"}`
				case method == http.MethodPost && path == "/v1/customers/cus_1":
					if metadataStatus != 200 {
						return metadataStatus, `{"error":{"type":"api_error","message":"down"}}`
					}
					return 200, `{"id":"cus_1","object":"customer"}`
				case method == http.MethodPost && path == "/v1/billing_portal/sessions":
					return 200, `{"id":"bps_1","object":"billing_portal.session","livemode":false,"url":"https://billing.stripe.com/p/session/test"}`
				default:
					return 500, `{"error":{"type":"api_error","message":"unexpected route"}}`
				}
			})
			c.dashboardURL = "https://dashboard.bex.co"
			c.storeCustomer("tea-a", "cus_1")
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "usr-member", Method: "session"})
			if _, err := c.CreatePortalSession(ctx, "tea-a", PortalRequest{ReturnURL: "https://dashboard.bex.co/usage"}); err != nil {
				t.Fatalf("CreatePortalSession: %v", err)
			}
			updates := stub.requests("/v1/customers/cus_1")
			if len(updates) != 1 || !strings.Contains(updates[0].body, "metadata[bex_portal_actor]=usr-member") ||
				!strings.Contains(updates[0].body, "metadata[bex_portal_actor_method]=session") || !strings.Contains(updates[0].body, "metadata[bex_portal_opened_at]=") {
				t.Fatalf("opener update = %+v", updates)
			}
		})
	}
}

func TestStripeWebhookDispatchesCustomerUpdated(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(fmt.Sprintf(`{"id":"evt_customer","object":"event","api_version":%q,"type":"customer.updated","data":{"object":{"id":"cus_1","object":"customer"}}}`, stripe.APIVersion))
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: secret, Timestamp: time.Now()})
	for name, handlerErr := range map[string]error{"ok": nil, "failing": errors.New("down")} {
		t.Run(name, func(t *testing.T) {
			var got string
			h := &StripeWebhook{Secret: secret, OnCustomerUpdated: func(_ context.Context, event *stripe.Event) error {
				got = event.ID
				return handlerErr
			}}
			req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/stripe", strings.NewReader(string(payload)))
			req.Header.Set("Stripe-Signature", signed.Header)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			want := http.StatusNoContent
			if handlerErr != nil {
				want = http.StatusServiceUnavailable
			}
			if w.Code != want || got != "evt_customer" {
				t.Fatalf("status=%d event=%q, want %d", w.Code, got, want)
			}
		})
	}
}

func TestHandledStripeEventTypesIncludesCustomerUpdated(t *testing.T) {
	if !slices.Contains(HandledStripeEventTypes(), "customer.updated") {
		t.Fatalf("HandledStripeEventTypes = %v", HandledStripeEventTypes())
	}
}
