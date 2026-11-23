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

package agentsession

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeResumer is Service.ResumePhase: it records who asked and answers as
// configured.
type fakeResumer struct {
	mu     sync.Mutex
	phase  string
	err    error
	asked  []core.Identity
	origin []string
}

func (f *fakeResumer) ResumePhase(ctx context.Context, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, _ := core.IdentityFrom(ctx)
	f.asked = append(f.asked, id)
	origin, _ := core.RequestOriginFrom(ctx)
	f.origin = append(f.origin, origin.SourceIP)
	return f.phase, f.err
}

func (f *fakeResumer) calls() []core.Identity {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]core.Identity(nil), f.asked...)
}

type fakeSSHKeys map[string]store.SSHKey

func (k fakeSSHKeys) SSHKeyByFingerprint(_ context.Context, fingerprint string) (store.SSHKey, error) {
	key, ok := k[fingerprint]
	if !ok {
		return store.SSHKey{}, store.ErrNotFound
	}
	return key, nil
}

type wakeVerb struct {
	handler *SessionWakeHandler
	client  *SessionWakeClient
	resumer *fakeResumer
	grants  *fakeKeyGrants
}

func newWakeVerb(t *testing.T) *wakeVerb {
	t.Helper()
	v := &wakeVerb{resumer: &fakeResumer{phase: "resuming"}, grants: &fakeKeyGrants{grants: map[string]core.KeyGrant{}}}
	v.handler = &SessionWakeHandler{
		Secret: []byte("exec-secret"), Sessions: v.resumer, Grants: v.grants, Nonce: &fakeNonceClaimer{},
		Keys: fakeSSHKeys{
			"SHA256:person": {Subject: "user-1", Fingerprint: "SHA256:person", CreatedSignIn: core.SignInGoogle},
			"SHA256:key":    {Subject: "key-1", Fingerprint: "SHA256:key"},
		},
	}
	srv := httptest.NewServer(v.handler)
	t.Cleanup(srv.Close)
	v.client = &SessionWakeClient{URL: srv.URL, Secret: []byte("exec-secret")}
	return v
}

func wakeRequest(session, subject, fingerprint string) SessionWakeRequest {
	return SessionWakeRequest{SessionID: session, Subject: subject, Fingerprint: fingerprint, SourceIP: "203.0.113.7"}
}

// The verb runs resume as the registered key's owner, authenticated the way
// a native SSH login is (its sign-in included), with the client's address
// for the audit row, and answers the phase resume left the session in.
func TestSessionWakeResumesAsTheKeysOwner(t *testing.T) {
	v := newWakeVerb(t)
	session := ids.New(ids.AgentSession)
	out, err := v.client.Wake(context.Background(), wakeRequest(session, "user-1", "SHA256:person"))
	if err != nil || out.Phase != "resuming" || out.Refusal != "" || out.SessionID != session {
		t.Fatalf("Wake = %+v, %v; want resuming", out, err)
	}
	calls := v.resumer.calls()
	want := core.Identity{Subject: "user-1", Method: core.MethodSSH, SignIn: core.SignInGoogle}
	if len(calls) != 1 || calls[0] != want {
		t.Fatalf("resume ran as %+v, want once as %+v", calls, want)
	}
	if v.resumer.origin[0] != "203.0.113.7" {
		t.Fatalf("source IP %q, want the client's", v.resumer.origin[0])
	}
}

// A caller who may not start the session, a key that is gone or another
// subject's, and a bex API key whose grant does not cover managing agent
// sessions all answer forbidden, and the last two never reach resume.
func TestSessionWakeRefusesWhatTheOwnerMayNotStart(t *testing.T) {
	session := ids.New(ids.AgentSession)
	for name, tc := range map[string]struct {
		req         SessionWakeRequest
		grant       []string
		resumeErr   error
		wantResumes int
	}{
		"no can_operate or can_create": {wakeRequest(session, "user-1", "SHA256:person"), nil, core.ErrForbidden, 1},
		"a login requirement": {wakeRequest(session, "user-1", "SHA256:person"), nil,
			core.LoginRefusal{Code: core.CodeWorkspaceTwoFactorRequired}.Err("tea-a"), 1},
		"the key is gone":             {wakeRequest(session, "user-1", "SHA256:deleted"), nil, nil, 0},
		"the key is someone else's":   {wakeRequest(session, "user-2", "SHA256:person"), nil, nil, 0},
		"a key without agents manage": {wakeRequest(session, "key-1", "SHA256:key"), []string{"sandbox_execution:execute"}, nil, 0},
		"a key without sandbox exec":  {wakeRequest(session, "key-1", "SHA256:key"), []string{"agents:read_write"}, nil, 0},
	} {
		t.Run(name, func(t *testing.T) {
			v := newWakeVerb(t)
			if tc.grant != nil {
				v.grants.set("key-1", tc.grant...)
			}
			v.resumer.err = tc.resumeErr
			out, err := v.client.Wake(context.Background(), tc.req)
			if err != nil || out.Refusal != SessionWakeForbidden || out.Phase != "" {
				t.Fatalf("Wake = %+v, %v; want forbidden", out, err)
			}
			if got := len(v.resumer.calls()); got != tc.wantResumes {
				t.Fatalf("resume ran %d times, want %d", got, tc.wantResumes)
			}
		})
	}
	// A key whose grant covers both is allowed through to resume.
	v := newWakeVerb(t)
	v.grants.set("key-1", "agents:read_write", "sandbox_execution:execute")
	if out, err := v.client.Wake(context.Background(), wakeRequest(session, "key-1", "SHA256:key")); err != nil || out.Phase != "resuming" {
		t.Fatalf("a key whose grant covers the start: %+v, %v", out, err)
	}
}

// A refusal of the start itself (admission, or the session's state) is told
// with its own code and message, as every other surface says it.
func TestSessionWakeTellsWhyTheStartWasRefused(t *testing.T) {
	v := newWakeVerb(t)
	v.resumer.err = core.NewConflictError("AGENT_SESSION_LIVE_LIMIT", "this workspace already has 3 live agent-session sandboxes (limit 3)", nil)
	out, err := v.client.Wake(context.Background(), wakeRequest(ids.New(ids.AgentSession), "user-1", "SHA256:person"))
	if err != nil || out.Refusal != SessionWakeRefused || out.Code != "AGENT_SESSION_LIVE_LIMIT" || out.Message != "this workspace already has 3 live agent-session sandboxes (limit 3)" {
		t.Fatalf("Wake = %+v, %v; want the admission refusal", out, err)
	}
}

// What the verb cannot establish is no answer, which the gateway turns
// into the banner: a dependency down, the feature unwired, no grant store.
func TestSessionWakeWithoutAnAnswerIsAnError(t *testing.T) {
	for name, configure := range map[string]func(*wakeVerb){
		"resume unavailable":    func(v *wakeVerb) { v.resumer.err = core.ErrAgentSessionsUnavailable },
		"authz unavailable":     func(v *wakeVerb) { v.resumer.err = core.ErrAuthzUnavailable },
		"store error":           func(v *wakeVerb) { v.resumer.err = errors.New("postgres down") },
		"grant store down":      func(v *wakeVerb) { v.grants.err = errors.New("postgres down") },
		"no resumer wired":      func(v *wakeVerb) { v.handler.Sessions = nil },
		"no nonce store":        func(v *wakeVerb) { v.handler.Nonce = nil },
		"another gateway's key": func(v *wakeVerb) { v.client.Secret = []byte("not-the-secret") },
	} {
		t.Run(name, func(t *testing.T) {
			v := newWakeVerb(t)
			configure(v)
			if out, err := v.client.Wake(context.Background(), wakeRequest(ids.New(ids.AgentSession), "user-1", "SHA256:person")); err == nil {
				t.Fatalf("Wake = %+v, want an error", out)
			}
		})
	}
	// A malformed question is refused before anything is read.
	v := newWakeVerb(t)
	for name, req := range map[string]SessionWakeRequest{
		"not a session id": wakeRequest("srv-d0example", "user-1", "SHA256:person"),
		"no subject":       wakeRequest(ids.New(ids.AgentSession), "", "SHA256:person"),
		"no fingerprint":   wakeRequest(ids.New(ids.AgentSession), "user-1", ""),
		"a bad address":    {SessionID: ids.New(ids.AgentSession), Subject: "user-1", Fingerprint: "SHA256:person", SourceIP: "not-an-ip"},
	} {
		if out, err := v.client.Wake(context.Background(), req); err == nil {
			t.Errorf("%s: Wake = %+v, want an error", name, out)
		}
	}
	if n := len(v.resumer.calls()); n != 0 {
		t.Fatalf("a malformed question ran resume %d times", n)
	}
}

// The verb verifies the gateway's HMAC under its own MAC domain, so a
// signature made for another verb never starts a sandbox, and each signed
// request runs once: a replay of its exact bytes, or one past the clock
// skew, is refused.
func TestSessionWakeIsSignedForItsOwnDomainAndRunsOnce(t *testing.T) {
	v := newWakeVerb(t)
	now := time.Now()
	v.handler.Now = func() time.Time { return now }
	body, _ := json.Marshal(SessionWakeRequest{Kind: SessionWakeKind, SessionID: ids.New(ids.AgentSession), Subject: "user-1", Fingerprint: "SHA256:person", Nonce: "nonce-1"})
	post := func(signed []byte, at time.Time) int {
		ts, sig := Sign([]byte("exec-secret"), signed, at)
		req := httptest.NewRequest(http.MethodPost, InternalSessionWakePath, bytes.NewReader(body))
		req.Header.Set(TimestampHeader, ts)
		req.Header.Set(SignatureHeader, sig)
		rec := httptest.NewRecorder()
		v.handler.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := post(body, now); code != http.StatusUnauthorized {
		t.Fatalf("a bare-body signature (another verb's domain) = %d, want 401", code)
	}
	if code := post(kindMACInput(SpawnKind, body), now); code != http.StatusUnauthorized {
		t.Fatalf("the spawn verb's domain = %d, want 401", code)
	}
	if code := post(kindMACInput(SessionWakeKind, body), now.Add(-2*mintClockSkew)); code != http.StatusUnauthorized {
		t.Fatalf("a stale timestamp = %d, want 401", code)
	}
	if code := post(kindMACInput(SessionWakeKind, body), now); code != http.StatusOK {
		t.Fatalf("the signed request = %d, want 200", code)
	}
	if code := post(kindMACInput(SessionWakeKind, body), now); code != http.StatusUnauthorized {
		t.Fatalf("its replay = %d, want 401", code)
	}
	if n := len(v.resumer.calls()); n != 1 {
		t.Fatalf("resume ran %d times, want once", n)
	}
	// A body with an unknown field, or another verb's kind, is refused.
	for name, raw := range map[string]string{
		"unknown field": `{"kind":"wake-session","sessionId":"x","subject":"user-1","fingerprint":"f","nonce":"n2","phase":"running"}`,
		"another kind":  `{"kind":"spawn-session","sessionId":"x","subject":"user-1","fingerprint":"f","nonce":"n3"}`,
	} {
		body = []byte(raw)
		if code := post(kindMACInput(SessionWakeKind, body), now); code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", name, code)
		}
	}
}

// The client takes no answer for another session and no answer that is
// neither a phase nor a known refusal.
func TestSessionWakeClientRefusesAMismatchedAnswer(t *testing.T) {
	session := ids.New(ids.AgentSession)
	for name, out := range map[string]SessionWakeResponse{
		"another session":       {SessionID: ids.New(ids.AgentSession), Phase: "resuming"},
		"no phase, no refusal":  {SessionID: session},
		"an unknown refusal":    {SessionID: session, Refusal: "maybe"},
		"a refusal, no message": {SessionID: session, Refusal: SessionWakeRefused, Code: "X"},
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(out)
			}))
			t.Cleanup(srv.Close)
			client := &SessionWakeClient{URL: srv.URL, Secret: []byte("exec-secret")}
			if got, err := client.Wake(context.Background(), wakeRequest(session, "user-1", "SHA256:person")); err == nil {
				t.Fatalf("Wake took %+v as %+v", out, got)
			}
		})
	}
}
