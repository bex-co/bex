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

// r14_delivery_evidence_test.go guards codex-security round 14 findings #4
// and #6 (docs/ADR069-security-review-round14.md): a webhook destination's
// path/query is the integration capability itself, and it must not reach
// ordinary members or a removed creator through the delivery-evidence path —
// persisted transport errors, the member-readable delivery history, or the
// unauthenticated failure-notice email.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// capabilityURL is the sentinel destination: a Slack-shaped path plus a query
// token, both of which are the secret. Its host resolves to nothing, so a
// POST to it fails in the transport layer — the exact error shape whose
// message Go embeds the full URL into.
const capabilityURL = "https://hooks.example.test/T000/B000/sentinel-path-token?token=sentinel-query-token"

func TestSanitizeDeliveryErrorCollapsesTheDestinationToItsOrigin(t *testing.T) {
	// A real *url.Error, exactly what http.Client.Do returns on a transport
	// failure — rewrapped once, as attempt()'s read-path error is.
	inner := &url.Error{Op: "Post", URL: capabilityURL, Err: errors.New("dial tcp: lookup hooks.example.test: no such host")}
	err := fmt.Errorf("read endpoint response: %w", inner)

	got := SanitizeDeliveryError(err)
	if !strings.Contains(got, "https://hooks.example.test") {
		t.Errorf("sanitized error should keep the origin for diagnosis, got %q", got)
	}
	if strings.Contains(got, "sentinel-path-token") || strings.Contains(got, "sentinel-query-token") {
		t.Errorf("sanitized error leaks the capability path/query: %q", got)
	}
	if !strings.Contains(got, "no such host") {
		t.Errorf("sanitized error lost the diagnostic inner transport error: %q", got)
	}
}

func TestSanitizeDeliveryErrorBoundsArbitraryErrorText(t *testing.T) {
	got := SanitizeDeliveryError(errors.New(strings.Repeat("x", maxDeliveryErrorBytes*4)))
	if len(got) > maxDeliveryErrorBytes {
		t.Errorf("non-url error text bounded to %d bytes, got %d", maxDeliveryErrorBytes, len(got))
	}
}

// TestTransportFailurePersistsOriginOnlyEvidence walks the real send path:
// the endpoint's destination is unreachable, so the POST fails in transport
// with the full URL in its error, and what lands in the row's last_error must
// carry the origin and the failure reason — never the capability.
func TestTransportFailurePersistsOriginOnlyEvidence(t *testing.T) {
	st := newFakeWorkerStore()
	secret, _ := NewSecret()
	st.endpoints = []store.WebhookEndpoint{endpoint("whk-1", "tea-a", capabilityURL, secret, TypeDeployStarted)}
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	st.queue["whd-1"] = &store.WebhookDelivery{
		ID: "whd-1", EndpointID: "whk-1", EventID: "evt-1", EventType: TypeDeployStarted,
		Payload: `{}`, NextAttemptAt: now.Add(-time.Second),
	}
	st.queueOrder = []string{"whd-1"}
	w := &Worker{Store: st, Backoff: []time.Duration{time.Hour}, Clock: func() time.Time { return now }, Client: &http.Client{}}

	if err := w.send(context.Background()); err != nil {
		t.Fatal(err)
	}
	d := st.queue["whd-1"]
	if d.AttemptCount != 1 || d.FailedAt != nil {
		t.Fatalf("expected one failed-but-retryable attempt, got %+v", d)
	}
	if strings.Contains(d.LastError, "sentinel-path-token") || strings.Contains(d.LastError, "sentinel-query-token") {
		t.Errorf("persisted last_error leaks the capability URL: %q", d.LastError)
	}
	if !strings.Contains(d.LastError, "hooks.example.test") {
		t.Errorf("persisted last_error should keep the origin for diagnosis: %q", d.LastError)
	}
}

// TestDeliveryViewScrubsLegacyRowsWithExactURLs: rows persisted before the
// write-side sanitizer may already embed the exact destination; the member
// read collapses it to the redacted origin.
func TestDeliveryViewScrubsLegacyRowsWithExactURLs(t *testing.T) {
	d := store.WebhookAttempt{TransportError: `Post "` + capabilityURL + `": dial tcp: no such host`}
	v := toDeliveryView(d, capabilityURL)
	if strings.Contains(v.TransportError, "sentinel-path-token") || strings.Contains(v.TransportError, "sentinel-query-token") {
		t.Errorf("delivery view leaks the legacy capability URL: %q", v.TransportError)
	}
	if !strings.Contains(v.TransportError, "hooks.example.test/…") {
		t.Errorf("delivery view should show the redacted origin: %q", v.TransportError)
	}
}

// failingEndpointWorker drives one endpoint to the 3rd-failure (and then the
// disable) notice with the given creator-admin state, so both notice emails
// can be asserted without duplicating the schedule walk.
func failingEndpointWorker(t *testing.T, st *fakeWorkerStore) (*Worker, *fakeMailer, *time.Time) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	secret, _ := NewSecret()
	st.endpoints = []store.WebhookEndpoint{endpoint("whk-1", "tea-a", srv.URL, secret, TypeDeployStarted)}
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	clock := &now
	st.queue["whd-1"] = &store.WebhookDelivery{
		ID: "whd-1", EndpointID: "whk-1", EventID: "evt-1", EventType: TypeDeployStarted,
		Payload: `{}`, NextAttemptAt: now.Add(-time.Second),
	}
	st.queueOrder = []string{"whd-1"}
	mailer := &fakeMailer{}
	return &Worker{
		Store: st, Mailer: mailer, Emails: fakeEmails{"user-1": "user-1@example.com"},
		Backoff: []time.Duration{time.Second, time.Second, time.Second},
		Clock:   func() time.Time { return *clock }, Client: &http.Client{},
	}, mailer, clock
}

// TestFailureNoticeGoesToCurrentAdminsOnly (round-14 #6): created_by is
// immutable provenance — a creator since removed or demoted must not receive
// a capability another admin may have (re)pointed after them. Both the
// 3rd-failure and the final disable notices are gated on CURRENT admins.
func TestFailureNoticeGoesToCurrentAdminsOnly(t *testing.T) {
	st := newFakeWorkerStore()
	st.nonAdmins["user-1"] = true // the creator was removed/demoted
	w, mailer, clock := failingEndpointWorker(t, st)
	ctx := context.Background()

	for i := 0; i < 4; i++ { // walk to schedule exhaustion (final notice)
		if err := w.send(ctx); err != nil {
			t.Fatal(err)
		}
		*clock = clock.Add(2 * time.Second)
	}
	if got := mailer.count(); got != 0 {
		t.Errorf("emails = %d, want 0 — a removed creator must receive neither notice", got)
	}
}

// TestFailureNoticeEmailNeverCarriesTheExactDestination (round-14 #6): email
// is an unauthenticated channel; the body gets the redacted origin, never the
// capability path/query — even though the current creator is an admin who
// could see it in the dashboard.
func TestFailureNoticeEmailNeverCarriesTheExactDestination(t *testing.T) {
	st := newFakeWorkerStore()
	w, mailer, clock := failingEndpointWorker(t, st)
	// Repoint the endpoint at a capability-shaped URL after construction (the
	// worker helper needs a live server for the failure; the URL in the email
	// comes from the claimed row, so rewrite just the stored destination).
	st.mu.Lock()
	ep := st.endpoints[0]
	ep.URL = capabilityURL
	st.endpoints[0] = ep
	st.mu.Unlock()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := w.send(ctx); err != nil {
			t.Fatal(err)
		}
		*clock = clock.Add(2 * time.Second)
	}
	if got := mailer.count(); got != 1 {
		t.Fatalf("emails = %d, want the 3rd-failure notice", got)
	}
	if strings.Contains(mailer.lastText, "sentinel-path-token") || strings.Contains(mailer.lastText, "sentinel-query-token") {
		t.Errorf("failure email leaks the capability URL:\n%s", mailer.lastText)
	}
	if !strings.Contains(mailer.lastText, "https://hooks.example.test") {
		t.Errorf("failure email should name the redacted origin so the admin can act:\n%s", mailer.lastText)
	}
}
