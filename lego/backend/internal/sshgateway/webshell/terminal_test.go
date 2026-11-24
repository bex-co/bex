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

package webshell

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/shellticket"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// terminal_test.go: an agent session's Terminal (w10/m615, ADR054 D10). The
// gateway runs the real agent-session resolver over rows the test settles as
// the Completer would after bex-api's mint started the sandbox.

// sessionRows is the session table the resolver reads while a test settles it.
type sessionRows struct {
	mu   sync.Mutex
	rows map[string]store.AgentSession
}

func (r *sessionRows) GetAgentSession(_ context.Context, id string) (store.AgentSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[id]
	if !ok {
		return store.AgentSession{}, store.ErrNotFound
	}
	return row, nil
}

// AgentSessionWorkspace names a session's workspace for the login
// requirements gate, as the store does.
func (r *sessionRows) AgentSessionWorkspace(ctx context.Context, id string) (string, error) {
	row, err := r.GetAgentSession(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return "", nil
	}
	return row.WorkspaceID, err
}

func (r *sessionRows) set(id, phase, sandboxID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rows == nil {
		r.rows = map[string]store.AgentSession{}
	}
	row := r.rows[id]
	row.ID, row.WorkspaceID, row.Phase, row.SandboxID = id, "tea-a", phase, sandboxID
	r.rows[id] = row
}

// viewers grants can_view_sensitive on every session to the subjects it
// holds, which a test may revoke mid-wait.
type viewers struct {
	mu    sync.Mutex
	allow map[string]bool
}

func (v *viewers) Check(_ context.Context, subject, relation, _ string) (bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return relation == core.RelCanViewSensitive && v.allow[strings.TrimPrefix(subject, "user:")], nil
}

func (v *viewers) revoke(subject string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.allow, subject)
}

// sandboxShell plays the sandbox's shell: it records the pod it ran in and
// the first readSizes terminal sizes, then echoes stdin until stdin closes or
// ctx ends.
type sandboxShell struct {
	mu        sync.Mutex
	pods      []string
	readSizes int
	sizes     []remotecommand.TerminalSize
	started   chan struct{}
}

func (e *sandboxShell) Execute(ctx context.Context, target apps.SSHInstanceTarget, _ []string, _ bool, queue remotecommand.TerminalSizeQueue, stdin io.Reader, stdout, _ io.Writer) (int, error) {
	e.mu.Lock()
	e.pods = append(e.pods, target.Namespace+"/"+target.PodName+"/"+target.Container)
	e.mu.Unlock()
	e.mu.Lock()
	n := e.readSizes
	e.mu.Unlock()
	for range n {
		if size := queue.Next(); size != nil {
			e.mu.Lock()
			e.sizes = append(e.sizes, *size)
			e.mu.Unlock()
		}
	}
	select {
	case e.started <- struct{}{}:
	default:
	}
	return (&gatewaytest.EchoExecutor{}).Execute(ctx, target, nil, true, queue, stdin, stdout, nil)
}

func (e *sandboxShell) ran() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.pods...)
}

type terminalGateway struct {
	gw       *Server
	srv      *httptest.Server
	rows     *sessionRows
	viewers  *viewers
	shell    *sandboxShell
	st       *gatewaytest.FakeStore
	registry *prometheus.Registry
	end      context.CancelCauseFunc
}

// newTerminalGateway serves the Browser Web Shell with the agent-session
// resolver wired, polling every 5 ms, under a context the test may end as the
// gateway's drain does.
func newTerminalGateway(t *testing.T, configure func(*Server)) *terminalGateway {
	t.Helper()
	g := &terminalGateway{rows: &sessionRows{}, viewers: &viewers{allow: map[string]bool{"alice": true}},
		shell: &sandboxShell{started: make(chan struct{}, 4)}, st: &gatewaytest.FakeStore{}, registry: prometheus.NewRegistry()}
	g.gw = &Server{
		TicketSecret: wsSecret, Store: g.st, Apps: &gatewaytest.FakeResolver{Err: errors.New("not a service")},
		AgentSessions: &agentsessions.SSHResolver{Base: &core.Base{Authz: g.viewers}, Store: g.rows},
		Executor:      g.shell, Metrics: sshgateway.NewMetrics(g.registry),
		Nonces: &sshgateway.NonceGuard{Store: g.st}, RevalidateInterval: -1,
		wakePollInterval: 5 * time.Millisecond,
	}
	if configure != nil {
		configure(g.gw)
	}
	sessionCtx, end := context.WithCancelCause(context.Background())
	g.end = end
	g.srv = httptest.NewUnstartedServer(g.gw.Handler())
	g.srv.Config.BaseContext = func(net.Listener) context.Context { return sessionCtx }
	g.srv.Start()
	t.Cleanup(func() {
		end(nil)
		g.srv.Close()
	})
	return g
}

func (g *terminalGateway) session(phase, sandboxID string) string {
	id := ids.New(ids.AgentSession)
	g.rows.set(id, phase, sandboxID)
	return id
}

// open dials the Terminal of session id as subject.
func (g *terminalGateway) open(t *testing.T, subject, id string) *websocket.Conn {
	t.Helper()
	conn, _, err := dialWS(t, g.srv, mintWS(t, shellticket.Claims{Subject: subject, SessionID: id}))
	if err != nil {
		t.Fatalf("dial the Terminal: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func (g *terminalGateway) wakes(t *testing.T, result string) float64 {
	t.Helper()
	return gatewaytest.MetricValue(t, g.registry, "bex_ssh_gateway_web_shell_wakes_total", map[string]string{"result": result})
}

func (g *terminalGateway) shellStarted(t *testing.T) {
	t.Helper()
	select {
	case <-g.shell.started:
	case <-time.After(3 * time.Second):
		t.Fatal("the sandbox's shell never started")
	}
}

// nextControl reads the next text frame, skipping terminal output.
func nextControl(t *testing.T, conn *websocket.Conn) serverControl {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("connection closed before a control frame: %v", err)
		}
		var ctrl serverControl
		if mt == websocket.TextMessage && json.Unmarshal(data, &ctrl) == nil {
			return ctrl
		}
	}
}

func wantControl(t *testing.T, conn *websocket.Conn, typ string) serverControl {
	t.Helper()
	got := nextControl(t, conn)
	if got.Type != typ {
		t.Fatalf("control frame = %+v, want type %q", got, typ)
	}
	return got
}

func TestTerminalOpensALiveSandboxAtOnce(t *testing.T) {
	g := newTerminalGateway(t, nil)
	id := g.session(agentsessions.PhaseCompleted, "os-up")
	conn := g.open(t, "alice", id)
	g.shellStarted(t)
	typeLine(t, conn, "uname -a")
	if ran := g.shell.ran(); len(ran) != 1 || ran[0] != "tea-a-sandbox/os-up-0/sandbox" {
		t.Fatalf("shell ran in %v, want the session's sandbox pod", ran)
	}
	audit := g.st.StartedSessions()
	if len(audit) != 1 || audit[0].ServiceID != id || audit[0].Transport != store.SSHTransportWebShell || audit[0].WorkspaceID != "tea-a" {
		t.Fatalf("audit rows = %+v, want one web shell row on the session", audit)
	}
	if n := g.wakes(t, "opened"); n != 0 {
		t.Fatalf("a live sandbox counted %v wakes, want none", n)
	}
}

// The heart of DoD 2: a sandbox bex-api's mint started is waited for on the
// open WebSocket, which says so, and the shell opens in it once it is up.
func TestTerminalWaitsForAStartingSandboxThenOpensTheShell(t *testing.T) {
	g := newTerminalGateway(t, func(s *Server) { s.wakeProgress = 20 * time.Millisecond })
	g.shell.readSizes = 2
	id := g.session(agentsessions.PhaseResuming, "")
	conn := g.open(t, "alice", id)
	wantControl(t, conn, "starting")
	// Typed and resized while it starts: the keystroke is dropped, and the
	// last of many sizes (more than the shell's size queue holds) is the
	// shell's.
	_ = conn.WriteMessage(websocket.BinaryMessage, []byte("lost"))
	for cols := 100; cols <= 120; cols++ {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"type":"resize","cols":%d,"rows":40}`, cols)))
	}
	wantControl(t, conn, "starting") // it keeps saying so
	if ran := g.shell.ran(); len(ran) != 0 {
		t.Fatalf("a shell ran while the sandbox started: %v", ran)
	}
	g.rows.set(id, agentsessions.PhaseCompleted, "os-new")
	for ctrl := nextControl(t, conn); ctrl.Type != "ready"; ctrl = nextControl(t, conn) {
		if ctrl.Type != "starting" {
			t.Fatalf("control frame = %+v while waiting, want starting then ready", ctrl)
		}
	}
	g.shellStarted(t)
	typeLine(t, conn, "pwd")
	if ran := g.shell.ran(); len(ran) != 1 || ran[0] != "tea-a-sandbox/os-new-0/sandbox" {
		t.Fatalf("shell ran in %v, want the started sandbox", ran)
	}
	g.shell.mu.Lock()
	sizes := append([]remotecommand.TerminalSize(nil), g.shell.sizes...)
	g.shell.mu.Unlock()
	if len(sizes) != 2 || sizes[1] != (remotecommand.TerminalSize{Width: 120, Height: 40}) {
		t.Fatalf("sizes = %v, want the default, then the last size sent while it started", sizes)
	}
	if n := g.wakes(t, "opened"); n != 1 {
		t.Fatalf("opened wakes = %v, want 1", n)
	}
	if audit := g.st.StartedSessions(); len(audit) != 1 || audit[0].InstanceID != id {
		t.Fatalf("audit rows = %+v, want one, written once the shell opened", audit)
	}
}

func TestTerminalWaitEndings(t *testing.T) {
	for name, tc := range map[string]struct {
		configure func(*Server)
		during    func(g *terminalGateway, id string, conn *websocket.Conn)
		reason    string
		message   string
		result    string
	}{
		"the start ends without a sandbox": {
			during: func(g *terminalGateway, id string, _ *websocket.Conn) {
				g.rows.set(id, agentsessions.PhaseCompleted, "")
			},
			reason: endNoSandbox, result: "no_sandbox",
		},
		"the start's ceiling passes": {
			configure: func(s *Server) { s.WakeTimeout = 50 * time.Millisecond },
			reason:    endWakeTimeout, result: "timed_out",
		},
		"the session is canceled meanwhile": {
			during: func(g *terminalGateway, id string, _ *websocket.Conn) {
				g.rows.set(id, agentsessions.PhaseCanceled, "")
			},
			message: "was canceled", result: "no_sandbox",
		},
		"the subject's access ends meanwhile": {
			during: func(g *terminalGateway, _ string, _ *websocket.Conn) { g.viewers.revoke("alice") },
			reason: endRevoked, result: "abandoned",
		},
		"the gateway's drain bound passes": {
			during: func(g *terminalGateway, _ string, _ *websocket.Conn) { g.end(sshgateway.ErrGatewayShutdown) },
			reason: endTimeout, result: "abandoned",
		},
		"the Terminal's cap passes": {
			configure: func(s *Server) { s.SessionTimeout = 50 * time.Millisecond },
			reason:    endTimeout, result: "abandoned",
		},
	} {
		t.Run(name, func(t *testing.T) {
			g := newTerminalGateway(t, tc.configure)
			id := g.session(agentsessions.PhaseResuming, "")
			conn := g.open(t, "alice", id)
			wantControl(t, conn, "starting")
			if tc.during != nil {
				tc.during(g, id, conn)
			}
			got := wantControl(t, conn, "error")
			switch {
			case tc.reason != "" && (got.Reason != tc.reason || got.Message == ""):
				t.Fatalf("end = %+v, want reason %q", got, tc.reason)
			case tc.message != "" && (got.Reason != "" || !strings.Contains(got.Message, tc.message)):
				t.Fatalf("end = %+v, want the resolver's %q", got, tc.message)
			}
			if tc.reason == endRevoked && got.Message != "your access to this agent session ended" {
				t.Fatalf("revoked message = %q, want the agent session's words", got.Message)
			}
			if n := g.wakes(t, tc.result); n != 1 {
				t.Fatalf("%s wakes = %v, want 1", tc.result, n)
			}
			if ran := g.shell.ran(); len(ran) != 0 || len(g.st.StartedSessions()) != 0 {
				t.Fatalf("a wait that ended without a sandbox ran %v with %d audit rows", ran, len(g.st.StartedSessions()))
			}
		})
	}
}

// The browser closing mid-wait ends it and frees the session-limiter slot.
func TestTerminalClosedMidWaitFreesItsSlot(t *testing.T) {
	limits := sshgateway.NewSessionLimiter(10, 1)
	g := newTerminalGateway(t, func(s *Server) { s.Limits = limits })
	id := g.session(agentsessions.PhaseResuming, "")
	conn := g.open(t, "alice", id)
	wantControl(t, conn, "starting")
	// The waiting Terminal holds alice's one slot, as an open shell does.
	if ok, _ := limits.Acquire("alice"); ok {
		t.Fatal("a waiting Terminal held no session-limiter slot")
	}
	_ = conn.Close()
	deadline := time.Now().Add(3 * time.Second)
	for g.wakes(t, "abandoned") != 1 {
		if time.Now().After(deadline) {
			t.Fatal("the wait never ended after the browser closed")
		}
		time.Sleep(5 * time.Millisecond)
	}
	for ok, _ := limits.Acquire("alice"); !ok; ok, _ = limits.Acquire("alice") {
		if time.Now().After(deadline) {
			t.Fatal("the slot was never released")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A session whose start already ended without a sandbox, a canceled one and
// an archived one are refused at connect with why; nothing waits.
func TestTerminalRefusesAtConnect(t *testing.T) {
	g := newTerminalGateway(t, nil)
	closed := g.session(agentsessions.PhaseCompleted, "")
	if got := wantControl(t, g.open(t, "alice", closed), "error"); got.Reason != endNoSandbox {
		t.Fatalf("a closed session = %+v, want %s", got, endNoSandbox)
	}
	canceled := g.session(agentsessions.PhaseCanceled, "")
	if got := wantControl(t, g.open(t, "alice", canceled), "error"); !strings.Contains(got.Message, "was canceled") {
		t.Fatalf("a canceled session = %+v, want its refusal", got)
	}
	live := g.session(agentsessions.PhaseRunning, "os-up")
	if got := wantControl(t, g.open(t, "mallory", live), "error"); got.Message != "this agent session's sandbox can't be opened right now" {
		t.Fatalf("a subject who may not view it = %+v, want the bare refusal", got)
	}
	if ran := g.shell.ran(); len(ran) != 0 {
		t.Fatalf("refused Terminals ran %v", ran)
	}
}

// A bex API key's Terminal is held to its grant while it waits.
func TestTerminalWaitEndsWhenTheKeysGrantNarrows(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "sandbox_execution:execute")
	g := newTerminalGateway(t, func(s *Server) { s.KeyGrants = &sshgateway.KeyGrantGate{Asker: grants.Client(t)} })
	g.viewers.allow["key-1"] = true
	id := g.session(agentsessions.PhaseResuming, "")
	conn, _, err := dialWS(t, g.srv, mintWS(t, shellticket.Claims{Subject: "key-1", SessionID: id, KeyAction: string(agentsessions.ShellKeyAction)}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	wantControl(t, conn, "starting")
	grants.Set("key-1")
	if got := wantControl(t, conn, "error"); got.Reason != endRevoked {
		t.Fatalf("end = %+v, want revoked once the grant narrowed", got)
	}
}

// A ticket is routed by the field it names: a service ticket reaches the
// service resolver unchanged even when its id is an agent session's, and a
// session ticket is refused where no agent-session resolver is wired. A
// redeemed Terminal ticket is not redeemed again.
func TestTerminalTicketsAreRoutedByTheirFieldAndSingleUse(t *testing.T) {
	services := &gatewaytest.FakeResolver{}
	g := newTerminalGateway(t, func(s *Server) { s.Apps = services })
	id := g.session(agentsessions.PhaseRunning, "os-up")
	conn, _, err := dialWS(t, g.srv, mintWS(t, shellticket.Claims{Subject: "alice", ServiceID: id}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	g.shellStarted(t)
	if services.Username != id || g.shell.ran()[0] == "tea-a-sandbox/os-up-0/sandbox" {
		t.Fatalf("a service ticket naming %s reached %v via %q, want the service resolver", id, g.shell.ran(), services.Username)
	}

	token := mintWS(t, shellticket.Claims{Subject: "alice", SessionID: id})
	first, _, err := dialWS(t, g.srv, token)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })
	g.shellStarted(t)
	if _, resp, err := dialWS(t, g.srv, token); err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a replayed Terminal ticket = %v, want 401", err)
	}

	unwired := newTerminalGateway(t, func(s *Server) { s.AgentSessions = nil })
	other := unwired.session(agentsessions.PhaseRunning, "os-up")
	if got := wantControl(t, unwired.open(t, "alice", other), "error"); got.Message != "this agent session's sandbox can't be opened right now" {
		t.Fatalf("no agent-session resolver = %+v, want the refusal", got)
	}
}

// A workspace's login requirement holds on the browser path too: the
// gateway asks it as the ticket's subject (method shell) before a Terminal
// opens, whatever bex-api's mint decided.
func TestTerminalHonorsTheWorkspacesLoginRequirement(t *testing.T) {
	asker := &gatewaytest.FakeTwoFactor{}
	asker.Require("tea-a", true)
	asker.Enroll("alice", false)
	g := newTerminalGateway(t, func(s *Server) {
		resolver := s.AgentSessions.(*agentsessions.SSHResolver)
		resolver.TwoFactor = &sshgateway.TwoFactorGate{Asker: asker, Sessions: resolver.Store.(*sessionRows)}
	})
	id := g.session(agentsessions.PhaseRunning, "os-up")
	if got := wantControl(t, g.open(t, "alice", id), "error"); got.Message != "this agent session's sandbox can't be opened right now" {
		t.Fatalf("a subject without the required second factor = %+v, want the bare refusal", got)
	}
	if methods := asker.Methods(); len(methods) == 0 || methods[0] != core.MethodShell {
		t.Fatalf("the requirement was asked as %v, want the shell method", methods)
	}
	if ran := g.shell.ran(); len(ran) != 0 {
		t.Fatalf("a refused Terminal ran %v", ran)
	}
}
