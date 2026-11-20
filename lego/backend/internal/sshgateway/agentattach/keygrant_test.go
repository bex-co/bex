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

package agentattach

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsessionticket"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

// w9/125: an attach a bex API key opened ("Start, steer, and delete agent
// sessions", what every attach-ticket mint needs) is held to the key's grant
// at redemption and on every revalidation tick, over bex-api's real grant
// verb.

const keyAttachTick = 10 * time.Millisecond

// allowAttach is a SessionRevalidator whose subject keeps its relation on the
// session throughout, so only the grant can end the stream.
type allowAttach struct{}

func (allowAttach) RevalidateAttach(context.Context, string, string, string) error { return nil }

type grantAttach struct {
	code    int
	body    chan string
	dropped chan struct{} // the driver's stream was canceled
}

// startGrantAttach redeems one read attach for subject naming keyAction (""
// for a person's) against a driver that holds its stream open, and waits for
// its first part (unless the redemption is refused).
func startGrantAttach(t *testing.T, grants *gatewaytest.KeyGrants, subject, keyAction string) grantAttach {
	t.Helper()
	secret := []byte("shell-ticket-secret")
	session := "ags-00000000000000000000k"
	out := grantAttach{body: make(chan string, 1), dropped: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"start\"}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(out.dropped)
	})
	driver := httptest.NewServer(mux)
	t.Cleanup(driver.Close)
	host, portStr, _ := net.SplitHostPort(strings.TrimPrefix(driver.URL, "http://"))
	port, _ := strconv.Atoi(portStr)

	gw := newAttachGateway(newFakeAttachStore(), fixedPodIP{ip: host}, secret, port)
	gw.RevalidateInterval = keyAttachTick
	gw.Revalidator = allowAttach{}
	gw.KeyGrants = &sshgateway.KeyGrantGate{Asker: grants.Client(t)}
	srv := httptest.NewServer(gw.Handler())
	t.Cleanup(srv.Close)

	tok, err := agentsessionticket.Mint(secret, agentsessionticket.Claims{
		Subject: subject, SessionID: session, SandboxID: "sandbox-1", Pod: "sandbox-1-0",
		Workspace: "tea-a", Namespace: "tea-a-sandbox", Action: agentsessionticket.ActionRead,
		KeyAction: keyAction, IssuedAt: 1_800_000_000, ExpiresAt: 1_800_000_090,
	})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set(TicketHeader, tok)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	out.code = resp.StatusCode
	if resp.StatusCode == http.StatusOK {
		first := make([]byte, 16)
		if _, err := io.ReadAtLeast(resp.Body, first, 1); err != nil {
			t.Fatalf("the stream never delivered its first part: %v", err)
		}
	}
	go func() {
		b, _ := io.ReadAll(resp.Body)
		out.body <- string(b)
	}()
	return out
}

func (g grantAttach) ended(t *testing.T) {
	t.Helper()
	select {
	case <-g.body:
	case <-time.After(3 * time.Second):
		t.Fatal("the attach stream did not end")
	}
	select {
	case <-g.dropped:
	case <-time.After(2 * time.Second):
		t.Fatal("the driver's stream was not canceled")
	}
}

func (g grantAttach) wantLive(t *testing.T, after string) {
	t.Helper()
	select {
	case <-g.dropped:
		t.Fatalf("the attach ended after %s", after)
	default:
	}
}

func TestAgentAttachKeyStreamEndsAtTheFirstTickAfterItsGrantNarrows(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "agents:read_write")
	run := startGrantAttach(t, grants, "key-1", string(core.KeyAgentsManage))
	if run.code != http.StatusOK {
		t.Fatalf("a covered key's attach = %d, want 200", run.code)
	}
	mark := grants.Set("key-1", "agents:read")
	run.ended(t)
	if n := grants.Reads() - mark; n != 1 {
		t.Fatalf("the attach ended after %d grant reads since the narrowing, want 1 (the first tick)", n)
	}
}

func TestAgentAttachKeyStreamSurvivesAWideningAndAnUnrelatedChange(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "agents:read_write")
	run := startGrantAttach(t, grants, "key-1", string(core.KeyAgentsManage))
	for _, change := range []struct {
		name string
		set  func() int
	}{
		{"no change", grants.Reads},
		{"a widening", func() int { return grants.Set("key-1", "agents:read_write", "agent_secrets:write") }},
		{"an unrelated change", func() int { return grants.Set("key-1", "agents:read_write", "deploys:read") }},
		{"a widening to All", func() int { return grants.SetAll("key-1") }},
	} {
		mark := change.set()
		grants.WaitReads(t, mark+3)
		run.wantLive(t, change.name)
	}
	grants.Set("key-1", "deploys:read")
	run.ended(t)
}

func TestAgentAttachKeyTicketIsRefusedAtRedemptionOnceItsGrantNarrowed(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "agents:read")
	if run := startGrantAttach(t, grants, "key-1", string(core.KeyAgentsManage)); run.code != http.StatusForbidden {
		t.Fatalf("an uncovered key's redemption = %d, want 403", run.code)
	}
}

func TestAgentAttachKeyStreamEndsWhenItsGrantCannotBeRead(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "agents:read_write")
	run := startGrantAttach(t, grants, "key-1", string(core.KeyAgentsManage))
	grants.Fail(errors.New("postgres down"))
	run.ended(t)
}

func TestAgentAttachPersonStreamNeverAsksForAGrant(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Fail(errors.New("bex-api unreachable"))
	run := startGrantAttach(t, grants, "alice", "")
	if run.code != http.StatusOK {
		t.Fatalf("a person's attach = %d, want 200", run.code)
	}
	time.Sleep(10 * keyAttachTick)
	run.wantLive(t, "ten ticks with the verb down")
	if n := grants.Reads(); n != 0 {
		t.Fatalf("a person's attach asked the grant verb %d times", n)
	}
}
