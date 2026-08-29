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

package modelproxy_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/modelproxy"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// statusTransport returns a fixed upstream status so a test can model the vendor
// rejecting (401/403) or throttling (429) the injected BYO key.
type statusTransport struct{ status int }

func (s statusTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: s.status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":"x"}`)),
	}, nil
}

// authFailStore records whether the auth-failure verb finalized the session.
type authFailStore struct {
	mu        sync.Mutex
	finalized bool
	reason    string
}

func (s *authFailStore) GetAgentSession(context.Context, string) (store.AgentSession, error) {
	return store.AgentSession{ID: "ags-one", WorkspaceID: "tea-a", SandboxID: "sbx-1", Phase: "running"}, nil
}

func (s *authFailStore) FinalizeAgentSession(_ context.Context, _, _, _, _ string, _ int, _ json.RawMessage, reason string) (store.AgentSession, error) {
	s.mu.Lock()
	s.finalized, s.reason = true, reason
	s.mu.Unlock()
	return store.AgentSession{ID: "ags-one", Phase: "failed"}, nil
}

func (s *authFailStore) didFinalize() (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.finalized, s.reason
}

// modelHarness wires a bex-api mint + auth-failure pair behind the shared HMAC
// and a Broker whose upstream returns `upstreamStatus`. `key` is the workspace's
// stored BYO credential (empty ⇒ never provisioned) and `sess` is the session row
// the minter re-reads, so one harness covers both the vendor-hop tests and the
// refusals that never reach a vendor. The session store comes back so a test can
// count mint attempts — the minter reads the session exactly once per attempt.
func modelHarness(t *testing.T, key string, sess store.AgentSession, upstreamStatus int) (*httptest.Server, *authFailStore, *fakeStore) {
	t.Helper()
	failStore := &authFailStore{}
	sessions := &fakeStore{session: sess}
	kv := map[string]map[string]string{}
	if key != "" {
		kv[agentsession.ModelKeySecretPath("tea-a")] = map[string]string{agentsession.ModelKeyField: key}
	}
	minter := &agentsession.ModelMinter{Keys: fakeKV{data: kv}, Sessions: sessions}
	mux := http.NewServeMux()
	mux.Handle(agentsession.InternalModelMintPath, &agentsession.ModelHandler{Secret: secret, Minter: minter})
	mux.Handle(agentsession.InternalModelAuthFailurePath, &agentsession.ModelAuthFailureHandler{
		Secret: secret,
		Failer: &agentsession.ModelAuthFailer{Sessions: failStore},
	})
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)

	broker := &modelproxy.Broker{
		Pods: fakePods{pod: sandboxPod("ags-one")},
		API:  &agentsession.ModelClient{URL: api.URL + agentsession.InternalModelMintPath, Secret: secret, HTTP: api.Client()},
		HTTP: &http.Client{Transport: statusTransport{status: upstreamStatus}},
	}
	proxy := httptest.NewServer(broker.Handler())
	t.Cleanup(proxy.Close)
	return proxy, failStore, sessions
}

// mintAttempts reads the minter's session-read count under the lock the fake
// writes it with — the mint may still be in flight behind a detached report.
func (f *fakeStore) mintAttempts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// authFailHarness is modelHarness with a provisioned key and a live session — the
// shape the vendor-hop tests want, where only the upstream status varies.
func authFailHarness(t *testing.T, upstreamStatus int) (*httptest.Server, *authFailStore) {
	t.Helper()
	proxy, failStore, _ := modelHarness(t, "sk-ant-api03-REAL", liveSession("https://api.anthropic.com/v1"), upstreamStatus)
	return proxy, failStore
}

func eventuallyFinalized(t *testing.T, s *authFailStore) (bool, string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if done, reason := s.didFinalize(); done {
			return true, reason
		}
		time.Sleep(10 * time.Millisecond)
	}
	done, reason := s.didFinalize()
	return done, reason
}

// A vendor 401 (bad/expired BYO key) reported back terminalizes the session with
// the actionable auth reason — the seconds-fast path instead of the ~3min retry.
func TestProxyReportsVendorAuthRejection(t *testing.T) {
	proxy, failStore := authFailHarness(t, http.StatusUnauthorized)
	resp := request(t, proxy, "ags-one", "/v1/messages", "x-api-key", "bex-model-proxy-placeholder-ags-one")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("proxy relayed status %d, want the vendor 401 passthrough", resp.StatusCode)
	}
	resp.Body.Close()
	done, reason := eventuallyFinalized(t, failStore)
	if !done || reason != agentsession.ModelAuthFailureReason {
		t.Fatalf("vendor 401 did not terminalize the session (done=%v reason=%q)", done, reason)
	}
}

// A transient vendor 429 is NOT a bad-key signal: it relays for the agent's normal
// retry and must never fast-fail the session.
func TestProxyDoesNotReportTransientVendorStatus(t *testing.T) {
	proxy, failStore := authFailHarness(t, http.StatusTooManyRequests)
	resp := request(t, proxy, "ags-one", "/v1/messages", "x-api-key", "bex-model-proxy-placeholder-ags-one")
	resp.Body.Close()
	if done, _ := eventuallyFinalized(t, failStore); done {
		t.Fatal("a transient vendor 429 must not fast-fail the session")
	}
}

// A workspace that never provisioned a BYO model key terminalizes with the
// actionable reason on the FIRST refusal — the w1/m136 fix. Before it, this
// refusal returned a bare 403, the mint was retried the full
// mintModelRetryAttempts, and the session died carrying the agent CLI's raw
// JSON-RPC text instead.
func TestProxyReportsMissingModelKey(t *testing.T) {
	proxy, failStore, sessions := modelHarness(t, "", liveSession("https://api.anthropic.com/v1"), http.StatusOK)
	resp := request(t, proxy, "ags-one", "/v1/messages", "x-api-key", "bex-model-proxy-placeholder-ags-one")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("proxy status %d, want 403 for a refused mint", resp.StatusCode)
	}
	resp.Body.Close()
	done, reason := eventuallyFinalized(t, failStore)
	if !done || reason != agentsession.ModelAuthFailureReason {
		t.Fatalf("missing key did not terminalize the session (done=%v reason=%q)", done, reason)
	}
	// Not retried: an absent key is a standing state, not the create-time race
	// mintModelRetryAttempts exists for.
	if n := sessions.mintAttempts(); n != 1 {
		t.Fatalf("mint attempts = %d, want exactly 1 (a missing key must not be retried)", n)
	}
}

// An AUTHORIZATION refusal at the mint — here a stale sandbox whose pod is no
// longer the session's current one — still gets a bare 403 and terminalizes
// NOTHING. This is the guard that keeps the fix above from becoming a way for a
// stale or compromised pod to kill a live session.
func TestProxyDoesNotReportAuthorizationRefusal(t *testing.T) {
	stale := liveSession("https://api.anthropic.com/v1")
	stale.SandboxID = "sbx-9" // the resolved pod is sbx-1-0, so it is not the current caller
	proxy, failStore, _ := modelHarness(t, "sk-ant-api03-REAL", stale, http.StatusOK)
	resp := request(t, proxy, "ags-one", "/v1/messages", "x-api-key", "bex-model-proxy-placeholder-ags-one")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("proxy status %d, want 403", resp.StatusCode)
	}
	resp.Body.Close()
	if done, reason := eventuallyFinalized(t, failStore); done {
		t.Fatalf("an authorization refusal must not terminalize the session (reason=%q)", reason)
	}
}
