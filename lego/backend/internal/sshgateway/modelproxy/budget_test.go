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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/modelproxy"
)

// countingTransport delegates to the wrapped transport after bumping a
// counter, so tests can assert how many exchanges actually crossed a hop.
type countingTransport struct {
	inner http.RoundTripper
	count *atomic.Int64
}

func (c countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.count.Add(1)
	return c.inner.RoundTrip(r)
}

// cannedUpstream counts then returns the same SSE response captureTransport
// does, without any network.
type cannedUpstream struct{ count *atomic.Int64 }

func (c cannedUpstream) RoundTrip(*http.Request) (*http.Response, error) {
	c.count.Add(1)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("data: {\"ok\":true}\n\n")),
	}, nil
}

// budgetHarness is harness() plus the cumulative budget knobs the default
// harness leaves unset (round-13 #8). The returned counter is the number of
// bex-api credential mints actually performed.
func budgetHarness(t *testing.T, perSession, perWorkspace int, limits *sshgateway.SessionLimiter) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	st := &fakeStore{session: liveSession("https://api.anthropic.com/v1")}
	minter := &agentsession.ModelMinter{
		Keys:     fakeKV{data: map[string]map[string]string{agentsession.ModelKeySecretPath("tea-a"): {agentsession.ModelKeyField: "sk-ant-api03-REAL"}}},
		Sessions: st,
	}
	api := httptest.NewServer(&agentsession.ModelHandler{Secret: secret, Minter: minter})
	t.Cleanup(api.Close)

	mints := &atomic.Int64{}
	apiTransport := api.Client().Transport
	if apiTransport == nil {
		apiTransport = http.DefaultTransport
	}
	broker := &modelproxy.Broker{
		Pods: fakePods{pod: sandboxPod("ags-one")},
		API: &agentsession.ModelClient{
			URL: api.URL + agentsession.InternalModelMintPath, Secret: secret,
			HTTP: &http.Client{Transport: countingTransport{inner: apiTransport, count: mints}},
		},
		HTTP:                    &http.Client{Transport: cannedUpstream{count: &atomic.Int64{}}},
		Limits:                  limits,
		MaxRequestsPerSession:   perSession,
		MaxRequestsPerWorkspace: perWorkspace,
	}
	proxy := httptest.NewServer(broker.Handler())
	t.Cleanup(proxy.Close)
	return proxy, mints
}

// TestProxySessionRequestBudget (round-13 #8): the per-exchange bounds
// (concurrency, bytes, duration) all reset on completion, so a runaway loop in
// tenant code could bill the provider for the session's whole lifetime. The
// cumulative per-session budget refuses the Nth exchange with 429 BEFORE any
// credential is minted.
func TestProxySessionRequestBudget(t *testing.T) {
	proxy, mints := budgetHarness(t, 2, 0, nil)
	for i := 1; i <= 2; i++ {
		resp := request(t, proxy, "ags-one", "/v1/messages", "X-Api-Key", "placeholder")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("exchange #%d = %d, want 200 (within budget)", i, resp.StatusCode)
		}
		resp.Body.Close()
	}
	resp := request(t, proxy, "ags-one", "/v1/messages", "X-Api-Key", "placeholder")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("exchange #3 = %d, want 429 (session budget exhausted)", resp.StatusCode)
	}
	if got := mints.Load(); got != 2 {
		t.Fatalf("credential mints = %d, want 2 — the refused exchange must not mint", got)
	}
}

// The workspace dimension bounds the aggregate across a workspace's sessions
// (keyed by the `<ws>` every `<ws>-sandbox` session derives from), so even
// with the per-session dimension disabled the aggregate still refuses.
func TestProxyWorkspaceRequestBudget(t *testing.T) {
	proxy, mints := budgetHarness(t, 0, 2, nil)
	for i := 1; i <= 2; i++ {
		resp := request(t, proxy, "ags-one", "/v1/messages", "X-Api-Key", "placeholder")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("exchange #%d = %d, want 200 (within workspace budget)", i, resp.StatusCode)
		}
		resp.Body.Close()
	}
	resp := request(t, proxy, "ags-one", "/v1/messages", "X-Api-Key", "placeholder")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("exchange #3 = %d, want 429 (workspace budget exhausted)", resp.StatusCode)
	}
	if got := mints.Load(); got != 2 {
		t.Fatalf("credential mints = %d, want 2", got)
	}
}

// 0 disables a dimension: the pre-round-13 behavior, byte-identical.
func TestProxyBudgetDisabled(t *testing.T) {
	proxy, mints := budgetHarness(t, 0, 0, nil)
	for i := 0; i < 5; i++ {
		resp := request(t, proxy, "ags-one", "/v1/messages", "X-Api-Key", "placeholder")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("exchange #%d = %d, want 200 (budget disabled)", i+1, resp.StatusCode)
		}
		resp.Body.Close()
	}
	if got := mints.Load(); got != 5 {
		t.Fatalf("mints = %d, want 5", got)
	}
}

// The budget is shared across concurrent connections atomically: with a
// budget of N, exactly N exchanges succeed no matter how they race.
func TestProxyBudgetAtomicAcrossConnections(t *testing.T) {
	// Isolate the budget: the default per-source concurrency cap (5) would
	// shed most of the burst before the budget is ever consulted. Generous
	// caps let all 32 exchanges reach the budget simultaneously.
	proxy, mints := budgetHarness(t, 8, 0, sshgateway.NewSessionLimiter(1000, 1000))
	var ok, rejected atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := request(t, proxy, "ags-one", "/v1/messages", "X-Api-Key", "placeholder")
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ok.Add(1)
			} else if resp.StatusCode == http.StatusTooManyRequests {
				rejected.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := mints.Load(); got != 8 {
		t.Fatalf("mints = %d, want 8 — exactly the budget crossed the credential hop", got)
	}
	if ok.Load() != 8 || rejected.Load() != 24 {
		t.Fatalf("budget outcomes ok=%d rejected=%d, want 8/24 under an 8-exchange budget", ok.Load(), rejected.Load())
	}
}
