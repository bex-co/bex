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
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// fakeTwoFactorPolicy stands in for twofactor.Gate: workspaces in required
// require 2FA, and verdict maps a subject to the error Check answers it with
// (nil = enrolled).
type fakeTwoFactorPolicy struct {
	required    map[string]bool
	verdict     map[string]error
	requiredErr error

	mu      sync.Mutex
	checked []core.Identity
}

func (p *fakeTwoFactorPolicy) Required(_ context.Context, workspaceID string) (bool, error) {
	return p.required[workspaceID], p.requiredErr
}

func (p *fakeTwoFactorPolicy) Check(_ context.Context, _ string, id core.Identity) error {
	p.mu.Lock()
	p.checked = append(p.checked, id)
	p.mu.Unlock()
	return p.verdict[id.Subject]
}

func (p *fakeTwoFactorPolicy) reset() {
	p.mu.Lock()
	p.checked = nil
	p.mu.Unlock()
}

func (p *fakeTwoFactorPolicy) identities() []core.Identity {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]core.Identity(nil), p.checked...)
}

func twoFactorHandler(policy TwoFactorPolicy) *TwoFactorHandler {
	return &TwoFactorHandler{Secret: []byte("exec-secret"), Policy: policy}
}

// The verb answers the gateway's question with what the real gate decides: a
// workspace that never asked costs no identity check and is answered for every
// member at once (required=false), an enrolled member is allowed, an unenrolled
// one is refused, and a gate that cannot decide is an error, never an allow.
func TestTwoFactorRoundTrip(t *testing.T) {
	policy := &fakeTwoFactorPolicy{
		required: map[string]bool{"tea-on": true},
		verdict: map[string]error{
			"enrolled":   nil,
			"unenrolled": core.NewWorkspaceTwoFactorRequiredError("tea-on"),
			"kratos-out": core.ErrAuthzUnavailable,
		},
	}
	srv := httptest.NewServer(twoFactorHandler(policy))
	t.Cleanup(srv.Close)
	client := &TwoFactorClient{URL: srv.URL, Secret: []byte("exec-secret")}

	for name, tc := range map[string]struct {
		workspace, subject        string
		wantRequired, wantAllowed bool
		wantChecks                int
	}{
		"setting off":       {workspace: "tea-off", subject: "unenrolled", wantRequired: false, wantAllowed: true, wantChecks: 0},
		"enrolled member":   {workspace: "tea-on", subject: "enrolled", wantRequired: true, wantAllowed: true, wantChecks: 1},
		"unenrolled member": {workspace: "tea-on", subject: "unenrolled", wantRequired: true, wantAllowed: false, wantChecks: 1},
	} {
		t.Run(name, func(t *testing.T) {
			policy.reset()
			required, allowed, err := client.Ask(context.Background(), tc.workspace, tc.subject, "ssh")
			if err != nil {
				t.Fatalf("Ask: %v", err)
			}
			if required != tc.wantRequired || allowed != tc.wantAllowed {
				t.Fatalf("Ask = (required %v, allowed %v), want (%v, %v)", required, allowed, tc.wantRequired, tc.wantAllowed)
			}
			if got := len(policy.identities()); got != tc.wantChecks {
				t.Fatalf("policy.Check ran %d times, want %d", got, tc.wantChecks)
			}
		})
	}
	t.Run("a gate that cannot decide is an error, never an allow", func(t *testing.T) {
		policy.reset()
		if required, allowed, err := client.Ask(context.Background(), "tea-on", "kratos-out", "ssh"); err == nil || allowed {
			t.Fatalf("Ask = (%v, %v, %v), want an error", required, allowed, err)
		}
	})
	t.Run("an unreadable setting is an error, never an allow", func(t *testing.T) {
		broken := &fakeTwoFactorPolicy{requiredErr: errors.New("db down")}
		srv := httptest.NewServer(twoFactorHandler(broken))
		t.Cleanup(srv.Close)
		client := &TwoFactorClient{URL: srv.URL, Secret: []byte("exec-secret")}
		if required, allowed, err := client.Ask(context.Background(), "tea-on", "enrolled", "ssh"); err == nil || allowed {
			t.Fatalf("Ask = (%v, %v, %v), want an error", required, allowed, err)
		}
	})
}

// Every way the verb can fail to answer must refuse, never allow.
func TestTwoFactorVerbFailsClosed(t *testing.T) {
	allow := &fakeTwoFactorPolicy{}
	for name, tc := range map[string]struct {
		handler      *TwoFactorHandler
		clientSecret string
	}{
		"unwired policy":   {handler: &TwoFactorHandler{Secret: []byte("s")}, clientSecret: "s"},
		"wrong secret":     {handler: &TwoFactorHandler{Secret: []byte("s"), Policy: allow}, clientSecret: "other"},
		"no server secret": {handler: &TwoFactorHandler{Policy: allow}, clientSecret: "s"},
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			t.Cleanup(srv.Close)
			client := &TwoFactorClient{URL: srv.URL, Secret: []byte(tc.clientSecret)}
			if _, allowed, err := client.Ask(context.Background(), "tea-on", "member", "ssh"); err == nil || allowed {
				t.Fatalf("Ask = (allowed %v, err %v) without a trustworthy answer", allowed, err)
			}
		})
	}
	t.Run("unreachable", func(t *testing.T) {
		srv := httptest.NewServer(twoFactorHandler(allow))
		url := srv.URL
		srv.Close()
		client := &TwoFactorClient{URL: url, Secret: []byte("exec-secret")}
		if _, allowed, err := client.Ask(context.Background(), "tea-on", "member", "ssh"); err == nil || allowed {
			t.Fatalf("Ask = (allowed %v, err %v) against an unreachable bex-api", allowed, err)
		}
	})
}

func postTwoFactor(t *testing.T, handler http.Handler, secret []byte, body any, now time.Time) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, signedMintRequest(t, secret, raw, now)())
	return rec
}

// The gateway authenticates with the shared HMAC and nothing else: an unsigned,
// forged or stale request never reaches the policy, and a signed request cannot
// be replayed.
func TestTwoFactorVerbAuthentication(t *testing.T) {
	secret := []byte("exec-secret")
	now := time.Unix(1_700_000_000, 0)
	policy := &fakeTwoFactorPolicy{required: map[string]bool{"tea-on": true}}
	claimer := &fakeNonceClaimer{}
	handler := &TwoFactorHandler{Secret: secret, Policy: policy, Nonce: claimer, Now: func() time.Time { return now }}
	body := TwoFactorRequest{WorkspaceID: "tea-on", Subject: "member", Method: "ssh", Nonce: newNonce()}

	t.Run("no signature", func(t *testing.T) {
		raw, _ := json.Marshal(body)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, InternalTwoFactorPath, bytes.NewReader(raw)))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("unsigned code = %d, want 401", rec.Code)
		}
	})
	t.Run("signed with another secret", func(t *testing.T) {
		if rec := postTwoFactor(t, handler, []byte("not-the-secret"), body, now); rec.Code != http.StatusUnauthorized {
			t.Fatalf("forged code = %d, want 401", rec.Code)
		}
	})
	t.Run("stale timestamp", func(t *testing.T) {
		if rec := postTwoFactor(t, handler, secret, body, now.Add(-time.Hour)); rec.Code != http.StatusUnauthorized {
			t.Fatalf("stale code = %d, want 401", rec.Code)
		}
	})
	t.Run("get is refused", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, InternalTwoFactorPath, nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("GET code = %d, want 405", rec.Code)
		}
	})
	if got := len(policy.identities()); got != 0 {
		t.Fatalf("an unauthenticated request reached the policy %d times", got)
	}
	t.Run("replay of the identical signed request", func(t *testing.T) {
		raw, _ := json.Marshal(body)
		makeReq := signedMintRequest(t, secret, raw, now)
		first := httptest.NewRecorder()
		handler.ServeHTTP(first, makeReq())
		if first.Code != http.StatusOK {
			t.Fatalf("first code = %d, want 200", first.Code)
		}
		second := httptest.NewRecorder()
		handler.ServeHTTP(second, makeReq())
		if second.Code != http.StatusUnauthorized {
			t.Fatalf("replay code = %d, want 401", second.Code)
		}
	})
}

// The gateway may only ask about the identities it authenticates itself (an
// SSH key's owner, a ticket's subject), as Kratos subjects. It cannot pose as
// an API key, which would make the gate read another principal's factor, nor
// as a background session, nor leave the question open.
func TestTwoFactorVerbBuildsOnlyTheIdentityTheGatewayAuthenticates(t *testing.T) {
	secret := []byte("exec-secret")
	now := time.Unix(1_700_000_000, 0)
	for _, method := range []string{"ssh", "shell", "agent-attach", "sandbox-exec"} {
		t.Run("allowed "+method, func(t *testing.T) {
			policy := &fakeTwoFactorPolicy{required: map[string]bool{"tea-on": true}}
			handler := &TwoFactorHandler{Secret: secret, Policy: policy, Now: func() time.Time { return now }}
			rec := postTwoFactor(t, handler, secret, TwoFactorRequest{WorkspaceID: "tea-on", Subject: "member", Method: method, Nonce: newNonce()}, now)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s code = %d, want 200", method, rec.Code)
			}
			got := policy.identities()
			if len(got) != 1 || got[0] != (core.Identity{Subject: "member", Method: method}) {
				t.Fatalf("policy saw %+v, want exactly Identity{Subject: member, Method: %s}", got, method)
			}
		})
	}
	for name, body := range map[string]any{
		"api key method":     TwoFactorRequest{WorkspaceID: "tea-on", Subject: "member", Method: "oauth2", Nonce: newNonce()},
		"background session": TwoFactorRequest{WorkspaceID: "tea-on", Subject: "member", Method: "session", Nonce: newNonce()},
		"no method":          TwoFactorRequest{WorkspaceID: "tea-on", Subject: "member", Nonce: newNonce()},
		"no subject":         TwoFactorRequest{WorkspaceID: "tea-on", Method: "ssh", Nonce: newNonce()},
		"no workspace":       TwoFactorRequest{Subject: "member", Method: "ssh", Nonce: newNonce()},
		// Identity fields beyond the three the verb defines are not read: a
		// client id would make the gate look up an API key's creator.
		"smuggled identity": map[string]any{"workspaceId": "tea-on", "subject": "member", "method": "oauth2", "clientId": "key-1", "human": false, "nonce": newNonce()},
		// A verb-kind body is another verb's envelope (ADR095 share verb).
		"another verb's kind": map[string]any{"workspaceId": "tea-on", "subject": "member", "method": "ssh", "kind": "share-file", "nonce": newNonce()},
	} {
		t.Run("refused "+name, func(t *testing.T) {
			policy := &fakeTwoFactorPolicy{required: map[string]bool{"tea-on": true}}
			handler := &TwoFactorHandler{Secret: secret, Policy: policy, Now: func() time.Time { return now }}
			rec := postTwoFactor(t, handler, secret, body, now)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("code = %d, want 400", rec.Code)
			}
			if got := len(policy.identities()); got != 0 {
				t.Fatalf("a malformed question reached the policy %d times", got)
			}
		})
	}
	t.Run("an email-verification body is not a two-factor question", func(t *testing.T) {
		policy := &fakeTwoFactorPolicy{required: map[string]bool{"tea-on": true}}
		handler := &TwoFactorHandler{Secret: secret, Policy: policy, Now: func() time.Time { return now }}
		rec := postTwoFactor(t, handler, secret, EmailVerificationRequest{Subject: "member", Nonce: newNonce()}, now)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("code = %d, want 400", rec.Code)
		}
	})
}

// A client trusts an answer only if it names the question it asked, and a
// "not required" answer must be an allow.
func TestTwoFactorClientRejectsAnswersThatDoNotMatchTheQuestion(t *testing.T) {
	for name, answer := range map[string]TwoFactorResponse{
		"another workspace":        {WorkspaceID: "tea-other", Subject: "member", Required: true, Allowed: true},
		"another subject":          {WorkspaceID: "tea-on", Subject: "someone-else", Required: true, Allowed: true},
		"not required yet refused": {WorkspaceID: "tea-on", Subject: "member"},
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				WriteNoStoreJSON(w, http.StatusOK, answer)
			}))
			t.Cleanup(srv.Close)
			client := &TwoFactorClient{URL: srv.URL, Secret: []byte("exec-secret")}
			if _, allowed, err := client.Ask(context.Background(), "tea-on", "member", "ssh"); err == nil || allowed {
				t.Fatalf("Ask = (allowed %v, err %v), want an error for a mismatched answer", allowed, err)
			}
		})
	}
}

// The verb accepts a closed set of methods and each producer sets the one it is
// named for. The two must not drift: a producer whose Identity.Method the verb
// does not accept would turn every native SSH login, ticket redemption or
// revalidation of that kind into a fail-closed refusal on an enforcing
// workspace, and no existing flow would notice until one did. The producers
// are read from source because they live in packages that import this one.
func TestEveryMethodTheTwoFactorVerbAcceptsIsTheOneItsProducerSets(t *testing.T) {
	constants := map[string]string{
		"MethodSSH": core.MethodSSH, "MethodShell": core.MethodShell,
		"MethodAgentAttach": core.MethodAgentAttach, "MethodSandboxExec": core.MethodSandboxExec,
	}
	producers := map[string]string{
		"../sshgateway/nativessh/server.go":      core.MethodSSH,
		"../sshgateway/webshell/websocket.go":    core.MethodShell,
		"../sshgateway/sandboxsse/sandboxsse.go": core.MethodSandboxExec,
		"../agentsessions/attach_revalidate.go":  core.MethodAgentAttach,
	}
	if len(producers) != len(twoFactorMethods) {
		t.Fatalf("producers = %d, verb methods = %v: every accepted method needs a named producer", len(producers), twoFactorMethods)
	}
	for file, want := range producers {
		if !twoFactorMethods[want] {
			t.Errorf("%s sets %q, which the verb does not accept", file, want)
		}
		found := 0
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatalf("parsing producer %s: %v", file, err)
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if sel, ok := lit.Type.(*ast.SelectorExpr); !ok || sel.Sel.Name != "Identity" {
				return true
			}
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok || kv.Key.(*ast.Ident).Name != "Method" {
					continue
				}
				found++
				var got string
				switch v := kv.Value.(type) {
				case *ast.SelectorExpr:
					got = constants[v.Sel.Name]
				case *ast.BasicLit:
					got, _ = strconv.Unquote(v.Value)
				}
				if got != want {
					t.Errorf("%s builds a core.Identity with Method %v, want %q", file, kv.Value, want)
				}
			}
			return true
		})
		if found == 0 {
			t.Errorf("%s builds no core.Identity with a Method: the producer moved, update this test", file)
		}
	}
}

// The method values are wire values: the gateway sends them to bex-api, which
// must read the same strings, so they are pinned, not just self-consistent.
func TestTheGatewayIdentityMethodsAreTheirWireValues(t *testing.T) {
	for constant, want := range map[string]string{
		core.MethodSSH: "ssh", core.MethodShell: "shell", core.MethodAgentAttach: "agent-attach", core.MethodSandboxExec: "sandbox-exec",
	} {
		if constant != want {
			t.Errorf("method constant = %q, want the wire value %q", constant, want)
		}
		if !twoFactorMethods[want] {
			t.Errorf("the verb does not accept %q", want)
		}
	}
	// An API key or a background session is never the gateway's to assert.
	for _, method := range []string{core.MethodOAuth2, core.MethodSession} {
		if twoFactorMethods[method] {
			t.Errorf("the verb accepts %q, an identity only bex-api itself builds", method)
		}
	}
}

func TestSessionObjectIsBuiltFromTheCoreAgentSessionPrefix(t *testing.T) {
	if got := SessionObject("ags-1"); got != core.AgentSessionObjectPrefix+"ags-1" {
		t.Fatalf("SessionObject = %q, want the core prefix the two-factor funnel branches on", got)
	}
}
