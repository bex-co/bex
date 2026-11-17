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
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// auth_onboarding_fault_test.go pins what the auth gate does with a failure it
// cannot serve through (w32/m446): the caller gets a 503 that names the cause
// class, the log gets the cause with the edge's request id, and neither gets
// the credential or the person's email.

const faultRequestID = "303516bf-4995-4cb9"

type failingOnboard struct{ err error }

func (o failingOnboard) EnsureTenant(context.Context, string, string, bool) (string, error) {
	return "", o.err
}

// captureLog sends the standard logger to a buffer for the rest of the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(previous) })
	return &buf
}

func decodeErrBody(t *testing.T, w *httptest.ResponseRecorder) (id, message string) {
	t.Helper()
	var body struct{ ID, Message string }
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not the error envelope: %v", w.Body.String(), err)
	}
	return body.ID, body.Message
}

func TestGateNamesAndLogsAnOnboardingFault(t *testing.T) {
	logged := captureLog(t)
	cause := stepFailed(stepOwnerRead, depStore, errStoreReset)
	s := &Server{HydraAdminURL: fakeHydraURL(t), KratosURL: fakeKratos(t).URL, Onboard: failingOnboard{cause}}
	w := serveGate(t, s, func(r *http.Request) {
		withSession(r)
		r.Header.Set("Rndr-Id", faultRequestID)
	})

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body %s", w.Code, w.Body.String())
	}
	id, message := decodeErrBody(t, w)
	if id != "unavailable" || message != "tenant onboarding unavailable: personal workspace lookup failed (workspace store)" {
		t.Errorf("body id %q message %q, want the step and its dependency", id, message)
	}
	if strings.Contains(w.Body.String(), "10.96.0.20") {
		t.Errorf("the 503 body carries the cause's own text: %s", w.Body.String())
	}
	line := logged.String()
	for _, want := range []string{"tenant onboarding unavailable", `rndr-id="` + faultRequestID + `"`, "GET /v1/services", "credential=session", "personal workspace lookup failed", "connection reset by peer"} {
		if !strings.Contains(line, want) {
			t.Errorf("log %q lacks %q", line, want)
		}
	}
	for _, secret := range []string{"live-session", "ada@example.com", "Ada@Example.com"} {
		if strings.Contains(line, secret) {
			t.Errorf("log carries %q: %s", secret, line)
		}
	}
}

// A coded refusal keeps its own answer and is not logged as an outage.
func TestGateWritesACodedOnboardingRefusalAsIs(t *testing.T) {
	logged := captureLog(t)
	s := &Server{HydraAdminURL: fakeHydraURL(t), KratosURL: fakeKratos(t).URL, Onboard: failingOnboard{core.NewAccountDeletionPendingError()}}
	w := serveGate(t, s, withSession)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "ACCOUNT_DELETION_PENDING") {
		t.Fatalf("status %d body %s, want 409 ACCOUNT_DELETION_PENDING", w.Code, w.Body.String())
	}
	if strings.Contains(logged.String(), "unavailable") {
		t.Errorf("a coded refusal was logged as an outage: %s", logged.String())
	}
}

func TestGateLogsAnAuthUpstreamFault(t *testing.T) {
	logged := captureLog(t)
	// A Hydra that is gone: every introspection fails at the transport.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gone := "http://" + listener.Addr().String()
	_ = listener.Close()

	w := serveGate(t, &Server{HydraAdminURL: gone}, func(r *http.Request) {
		withBearer(r)
		r.Header.Set("Rndr-Id", faultRequestID)
	})
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body %s", w.Code, w.Body.String())
	}
	if _, message := decodeErrBody(t, w); message != "auth upstream unavailable" {
		t.Errorf("message = %q", message)
	}
	line := logged.String()
	for _, want := range []string{"auth upstream unavailable", `rndr-id="` + faultRequestID + `"`, "credential=bearer", "/admin/oauth2/introspect"} {
		if !strings.Contains(line, want) {
			t.Errorf("log %q lacks %q", line, want)
		}
	}
	if strings.Contains(line, testToken) {
		t.Errorf("log carries the bearer: %s", line)
	}
}

// An introspection that lands on a keep-alive connection Hydra's side already
// dropped is resent on a fresh one: the caller is served, not answered 503
// auth upstream unavailable (w32/m446; production's remaining 503s on
// 2026-11-17 were each one failed introspection).
func TestIntrospectionSurvivesAResetReusedConnection(t *testing.T) {
	var calls atomic.Int32
	hydra := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/oauth2/introspect" {
			http.NotFound(w, r)
			return
		}
		if calls.Add(1) == 2 {
			// The second introspection rides the connection the first kept
			// alive; drop it with a RST, as a peer that timed it out.
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			_ = conn.(*net.TCPConn).SetLinger(0)
			_ = conn.Close()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"active":true,"sub":"client-1","client_id":"client-1"}`))
	}))
	t.Cleanup(hydra.Close)
	gate, err := (&Server{HydraAdminURL: hydra.URL}).newAuthGate()
	if err != nil {
		t.Fatal(err)
	}
	for i, token := range []string{"token-1", "token-2"} { // distinct, so neither is cached
		w := serveWith(gate, func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) })
		if w.Code != http.StatusOK {
			t.Fatalf("request %d = %d %s, want 200", i+1, w.Code, w.Body.String())
		}
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("introspections = %d, want 3 (one reset, resent once)", got)
	}
}

// t005: an onboarding fault reaches every surface the same way. The gate sits
// in front of REST, GraphQL and MCP alike and answers before any of them, so
// each gets the same 503 and the same Render-shaped error body; a dashboard
// (Kratos session) and a CLI (human OAuth token) caller get the same answer.
func TestOnboardingFaultIsOneAnswerOnEverySurface(t *testing.T) {
	_ = captureLog(t)
	base := &core.Base{Client: fakeClient(), Namespace: "default"}
	srv := NewServer(base, Deps{Onboard: failingOnboard{stepFailed(stepGrant, depAuthz, errFGAReset)}})
	srv.HydraAdminURL, srv.KratosURL = fakeHumanHydra(t).URL, fakeKratos(t).URL
	srv.OAuthPlatformClients = []string{"platform-cli"}
	h := buildHandler(t, srv)

	const want = "tenant onboarding unavailable: workspace admin grant failed (authorization service)"
	for credential, header := range map[string][2]string{
		"session": {"Cookie", "ory_kratos_session=live"},
		"bearer":  {"Authorization", "Bearer " + testToken},
	} {
		calls := map[string]*httptest.ResponseRecorder{
			"REST":    request(h, http.MethodGet, "/v1/services", "", map[string]string{header[0]: header[1]}),
			"GraphQL": request(h, http.MethodPost, "/graphql", `{"query":"{ services { name } }"}`, map[string]string{header[0]: header[1], "Content-Type": "application/json"}),
			"MCP": request(h, http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"list_services","arguments":{}}}`,
				map[string]string{header[0]: header[1], "Content-Type": "application/json", "Accept": "application/json, text/event-stream"}),
		}
		var first string
		for surface, w := range calls {
			if w.Code != http.StatusServiceUnavailable {
				t.Errorf("%s %s = %d %s, want 503", credential, surface, w.Code, w.Body.String())
				continue
			}
			if id, message := decodeErrBody(t, w); id != "unavailable" || message != want {
				t.Errorf("%s %s body id %q message %q, want %q", credential, surface, id, message, want)
			}
			if first == "" {
				first = w.Body.String()
			} else if w.Body.String() != first {
				t.Errorf("%s %s body %s differs from %s", credential, surface, w.Body.String(), first)
			}
		}
	}
}
