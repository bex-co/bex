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
	"bufio"
	"cmp"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The w3/m406 stream rule: `[DONE]` ends a read stream only once the turn has
// settled. A working session whose sandbox does not serve yet is held (with
// heartbeats) until it does; every other end before the turn settles closes
// without `[DONE]`, for the client to attach again.

const heldSession = "ags-0000000000000000000w1"

// switchPods resolves the ticket's pod to ip once ready is set, and fails
// before, as a pod without an address does.
type switchPods struct {
	mu    sync.Mutex
	ip    string
	ready bool
	calls atomic.Int32
}

func (p *switchPods) PodIP(context.Context, string, string) (string, error) {
	p.calls.Add(1)
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.ready {
		return "", errors.New("pod tea-a-sandbox/sandbox-1-0 has no live IP")
	}
	return p.ip, nil
}

func (p *switchPods) serve() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ready = true
}

// heldDriver serves one live part then `[DONE]` on /stream, answering 503
// until it is ready (a driver that does not listen yet).
type heldDriver struct {
	srv   *httptest.Server
	host  string
	port  int
	ready atomic.Bool
}

func newHeldDriver(t *testing.T, ready bool) *heldDriver {
	t.Helper()
	d := &heldDriver{}
	d.ready.Store(ready)
	d.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stream" || !d.ready.Load() {
			http.Error(w, "not listening yet", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		// The driver replays its history (the replayed start) before going live.
		_, _ = io.WriteString(w, "data: {\"type\":\"start\"}\n\ndata: {\"type\":\"text-start\",\"id\":\"live\"}\n\n"+
			"data: {\"type\":\"text-delta\",\"id\":\"live\",\"delta\":\"turn two\"}\n\n"+
			"data: {\"type\":\"text-end\",\"id\":\"live\"}\n\ndata: [DONE]\n\n")
	}))
	t.Cleanup(d.srv.Close)
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(d.srv.URL, "http://"))
	d.host = host
	d.port, _ = strconv.Atoi(port)
	return d
}

// heldGateway is an attach gateway for heldSession, working on sandbox-1 (the
// ticket's), with one replayed part and fast heartbeats and polls.
func heldGateway(t *testing.T, pods PodIPResolver, port int) (*Server, *fakeAttachStore) {
	t.Helper()
	st := newFakeAttachStore()
	st.setSession(heldSession, "running", "sandbox-1")
	if err := st.AppendAgentSessionTranscript(context.Background(), heldSession, []store.AgentSessionTranscriptPart{
		{Seq: 0, Part: []byte(`{"type":"start"}`)},
	}); err != nil {
		t.Fatal(err)
	}
	gw := newAttachGateway(st, pods, []byte("shell-ticket-secret"), port)
	gw.HeartbeatInterval = 10 * time.Millisecond
	gw.sandboxPoll = 5 * time.Millisecond
	return gw, st
}

// sseLines streams the attach response line by line: lines carries each line
// as it arrives, and body the whole response once it ends.
func sseLines(t *testing.T, gw *Server) (lines <-chan string, body <-chan string) {
	t.Helper()
	srv := httptest.NewServer(gw.Handler())
	t.Cleanup(srv.Close)
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set(TicketHeader, attachTicket(t, []byte("shell-ticket-secret"), heldSession))
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	lineCh, bodyCh := make(chan string, 1024), make(chan string, 1)
	go func() {
		reader := bufio.NewReader(resp.Body)
		var all strings.Builder
		for {
			line, err := reader.ReadString('\n')
			all.WriteString(line)
			if line != "" {
				lineCh <- strings.TrimRight(line, "\n")
			}
			if err != nil {
				bodyCh <- all.String()
				return
			}
		}
	}()
	return lineCh, bodyCh
}

// awaitHeld reads lines until n keep-alive comments have arrived, failing on a
// `[DONE]` or any live part first: the stream is held, not ended.
func awaitHeld(t *testing.T, lines <-chan string, n int) {
	t.Helper()
	timeout := time.After(3 * time.Second)
	for n > 0 {
		select {
		case line := <-lines:
			switch {
			case line == "data: [DONE]":
				t.Fatal("the stream ended with [DONE] while the session was still working and its sandbox did not serve")
			case strings.Contains(line, "turn two"):
				t.Fatal("a live part arrived before the sandbox served")
			case line == ": keep-alive":
				n--
			}
		case <-timeout:
			t.Fatal("the held stream sent no keep-alive")
		}
	}
}

func awaitBody(t *testing.T, body <-chan string) string {
	t.Helper()
	select {
	case b := <-body:
		return b
	case <-time.After(5 * time.Second):
		t.Fatal("the stream did not end")
		return ""
	}
}

// A working session's sandbox that does not serve yet — its pod has no
// address, or its driver does not answer — holds the stream instead of ending
// it, then splices the turn live once the sandbox serves; [DONE] comes only
// with the driver's.
func TestAgentAttachHoldsAWorkingSessionUntilItsSandboxServes(t *testing.T) {
	for _, tc := range []struct {
		name string
		// unready returns the pods + driver in their not-serving state, and
		// serve brings them up.
		unready func(t *testing.T) (PodIPResolver, int, func())
	}{
		{"pod without an address", func(t *testing.T) (PodIPResolver, int, func()) {
			driver := newHeldDriver(t, true)
			pods := &switchPods{ip: driver.host}
			return pods, driver.port, pods.serve
		}},
		{"driver not answering", func(t *testing.T) (PodIPResolver, int, func()) {
			driver := newHeldDriver(t, false)
			return fixedPodIP{ip: driver.host}, driver.port, func() { driver.ready.Store(true) }
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pods, port, serve := tc.unready(t)
			gw, _ := heldGateway(t, pods, port)
			lines, body := sseLines(t, gw)
			awaitHeld(t, lines, 3)
			serve()
			got := awaitBody(t, body)
			if !strings.Contains(got, `"delta":"turn two"`) {
				t.Fatalf("the turn was not spliced once the sandbox served:\n%s", got)
			}
			if strings.Count(got, "data: [DONE]") != 1 || !strings.HasSuffix(strings.TrimSpace(got), "data: [DONE]") {
				t.Fatalf("the spliced turn did not end with exactly one [DONE]:\n%s", got)
			}
		})
	}
}

// Past SandboxStartWait the held stream closes without [DONE]: the documented
// signal to attach again.
func TestAgentAttachClosesWithoutDoneWhenTheSandboxNeverServes(t *testing.T) {
	gw, _ := heldGateway(t, &switchPods{}, 1)
	gw.SandboxStartWait = 150 * time.Millisecond
	lines, body := sseLines(t, gw)
	awaitHeld(t, lines, 1)
	got := awaitBody(t, body)
	if strings.Contains(got, "[DONE]") {
		t.Fatalf("a stream whose sandbox never served ended with [DONE]:\n%s", got)
	}
	if !strings.Contains(got, `data: {"type":"start"}`) {
		t.Fatalf("the durable replay was not sent before the wait:\n%s", got)
	}
}

// A session that settles while its stream waits gets what the turn stored
// meanwhile, then [DONE]; one that moves to another sandbox (a new turn) is
// cut without [DONE], so the client's next ticket names that sandbox.
func TestAgentAttachHeldStreamFollowsTheSession(t *testing.T) {
	t.Run("settles", func(t *testing.T) {
		gw, st := heldGateway(t, &switchPods{}, 1)
		lines, body := sseLines(t, gw)
		awaitHeld(t, lines, 1)
		if err := st.AppendAgentSessionTranscript(context.Background(), heldSession, []store.AgentSessionTranscriptPart{
			{Seq: 1, Part: []byte(`{"type":"error","errorText":"the sandbox ran out of memory (exit 137)"}`)},
		}); err != nil {
			t.Fatal(err)
		}
		st.setSession(heldSession, "failed", "sandbox-1")
		got := awaitBody(t, body)
		if !strings.Contains(got, "ran out of memory") || !strings.HasSuffix(strings.TrimSpace(got), "data: [DONE]") || strings.Count(got, "[DONE]") != 1 {
			t.Fatalf("a session settled while held must get the rest of its transcript, then [DONE]:\n%s", got)
		}
	})
	t.Run("moves to another sandbox", func(t *testing.T) {
		gw, st := heldGateway(t, &switchPods{}, 1)
		lines, body := sseLines(t, gw)
		awaitHeld(t, lines, 1)
		st.setSession(heldSession, "redispatching", "")
		if got := awaitBody(t, body); strings.Contains(got, "[DONE]") {
			t.Fatalf("a session that left the ticket's sandbox ended with [DONE]:\n%s", got)
		}
	})
}

// A ticket naming a pod that is gone, for a session that has settled (or was
// deleted), still replays and ends with [DONE] at once: the terminal path.
func TestAgentAttachSettledSessionWithAGonePodReplaysThenDone(t *testing.T) {
	for _, phase := range []string{"completed", "failed", "canceled", ""} {
		t.Run(cmp.Or(phase, "deleted"), func(t *testing.T) {
			pods := &switchPods{}
			gw, st := heldGateway(t, pods, 1)
			gw.SandboxStartWait = time.Minute
			if phase == "" {
				st.mu.Lock()
				delete(st.sessions, heldSession)
				st.mu.Unlock()
			} else {
				st.setSession(heldSession, phase, "sandbox-1")
			}
			_, body := sseLines(t, gw)
			got := awaitBody(t, body)
			if !strings.Contains(got, `data: {"type":"start"}`) || !strings.HasSuffix(strings.TrimSpace(got), "data: [DONE]") {
				t.Fatalf("settled session = %q, want the replay then [DONE]", got)
			}
			if n := pods.calls.Load(); n != 1 {
				t.Fatalf("pod resolved %d times, want only the first attempt", n)
			}
		})
	}
}

// A driver connection that ends without the driver's [DONE] (its pod died
// mid-turn) is cut without [DONE], so the client attaches again and learns
// how the turn ended.
func TestAgentAttachDriverCutMidTurnClosesWithoutDone(t *testing.T) {
	driver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"start\"}\n\ndata: {\"type\":\"text-start\",\"id\":\"cut\"}\n\n")
	}))
	defer driver.Close()
	host, portStr, _ := net.SplitHostPort(strings.TrimPrefix(driver.URL, "http://"))
	port, _ := strconv.Atoi(portStr)
	gw, _ := heldGateway(t, fixedPodIP{ip: host}, port)
	_, body := sseLines(t, gw)
	got := awaitBody(t, body)
	if !strings.Contains(got, `"id":"cut"`) {
		t.Fatalf("the live part before the cut was not forwarded:\n%s", got)
	}
	if strings.Contains(got, "[DONE]") {
		t.Fatalf("a driver connection cut mid-turn ended the stream with [DONE]:\n%s", got)
	}
}
