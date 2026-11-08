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

package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeBox is one sandbox in fakeOpenSandbox. A pause or resume moves it into
// the in-flight phase, and it settles on the next read, as the real server's
// Pausing→Paused and Resuming→Running transitions do.
type fakeBox struct {
	state, next string
	delay       int    // reads before next takes effect
	stuck       bool   // never settle (a resume that hangs)
	afterResume string // where a resume settles; default Running
	metadata    map[string]string
	created     time.Time
	expires     *time.Time // BatchSandbox spec.expireTime; nil = no timeout
}

// fakeOpenSandbox is a stateful OpenSandbox lifecycle server for the idle
// suspension flows: list/get/pause/resume with the real server's 202/409
// answers, plus the metadata merge patch and renew-expiration (409 for a
// sandbox without an expiry) the lifetime stamps use.
type fakeOpenSandbox struct {
	t        *testing.T
	mu       sync.Mutex
	boxes    map[string]*fakeBox
	calls    []string
	renewals []string // "<id> <expiresAt>", in order
	pauseOK  bool
	patchOK  bool
}

func newFakeOpenSandbox(t *testing.T) (*fakeOpenSandbox, *Client) {
	f := &fakeOpenSandbox{t: t, boxes: map[string]*fakeBox{}, pauseOK: true, patchOK: true}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	return f, NewClient(srv.URL)
}

// add registers an ordinary sandbox owned by id-a in tea-a.
func (f *fakeOpenSandbox) add(id, state string, created time.Time, idle string, extra map[string]string) {
	md := map[string]string{
		metadataOwner: "id-a", metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime,
		metadataNetworkPolicy: string(NetworkPolicyDenyAll), metadataComputeWeight: "553", metadataPlan: "starter",
	}
	if idle != "" {
		md[metadataIdleTimeout] = idle
	}
	for k, v := range extra {
		md[k] = v
	}
	box := &fakeBox{state: state, metadata: md, created: created}
	if timeout := timeoutFromMetadata(md[metadataTimeout]); timeout > 0 {
		exp := created.Add(time.Duration(openSandboxTimeout(timeout)) * time.Second)
		box.expires = &exp
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.boxes[id] = box
}

func (f *fakeOpenSandbox) box(id string) fakeBox {
	f.mu.Lock()
	defer f.mu.Unlock()
	b := *f.boxes[id]
	b.metadata = maps.Clone(b.metadata)
	return b
}

func (f *fakeOpenSandbox) renewCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.renewals)
}

func (f *fakeOpenSandbox) state(id string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.boxes[id].state
}

func (f *fakeOpenSandbox) set(id string, mutate func(*fakeBox)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	mutate(f.boxes[id])
}

func (f *fakeOpenSandbox) lifecycleCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func (f *fakeOpenSandbox) view(id string, b *fakeBox) osSandbox {
	created := b.created
	out := osSandbox{ID: id, Metadata: b.metadata, Created: &created}
	out.Status.State = b.state
	return out
}

func (f *fakeOpenSandbox) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, "/sandboxes")
	if r.Method == http.MethodGet && path == "" {
		out := []osSandbox{}
		for id, b := range f.boxes {
			out = append(out, f.view(id, b))
		}
		_ = json.NewEncoder(w).Encode(out)
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	b, ok := f.boxes[parts[0]]
	if !ok {
		http.NotFound(w, r)
		return
	}
	switch {
	case r.Method == http.MethodGet && len(parts) == 1:
		switch {
		case b.next == "" || b.stuck:
		case b.delay > 0:
			b.delay--
		default:
			b.state, b.next = b.next, ""
		}
		_ = json.NewEncoder(w).Encode(f.view(parts[0], b))
	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "pause":
		f.calls = append(f.calls, "pause "+parts[0])
		if !f.pauseOK {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if b.state != "Running" {
			w.WriteHeader(http.StatusConflict)
			return
		}
		b.state, b.next = "Pausing", "Paused"
		w.WriteHeader(http.StatusAccepted)
	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "resume":
		f.calls = append(f.calls, "resume "+parts[0])
		if b.state != "Paused" {
			w.WriteHeader(http.StatusConflict)
			return
		}
		b.state, b.next = "Resuming", "Running"
		if b.afterResume != "" {
			b.next = b.afterResume
		}
		w.WriteHeader(http.StatusAccepted)
	case r.Method == http.MethodDelete && len(parts) == 1:
		f.calls = append(f.calls, "terminate "+parts[0])
		delete(f.boxes, parts[0])
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodPatch && len(parts) == 2 && parts[1] == "metadata":
		if !f.patchOK {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var patch map[string]string
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		maps.Copy(b.metadata, patch)
		_ = json.NewEncoder(w).Encode(f.view(parts[0], b))
	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "renew-expiration":
		var body struct {
			ExpiresAt time.Time `json:"expiresAt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if b.expires == nil {
			w.WriteHeader(http.StatusConflict) // no automatic expiration
			return
		}
		f.renewals = append(f.renewals, parts[0]+" "+body.ExpiresAt.UTC().Format(time.RFC3339))
		b.expires = &body.ExpiresAt
		_ = json.NewEncoder(w).Encode(body)
	default:
		http.NotFound(w, r)
	}
}

// fakeActivity mirrors store.PGStore's sandbox_activity semantics in memory.
type fakeActivity struct {
	mu      sync.Mutex
	last    map[string]time.Time
	claims  map[string]time.Time
	touches map[string]int
	pruned  []string
}

func newFakeActivity() *fakeActivity {
	return &fakeActivity{last: map[string]time.Time{}, claims: map[string]time.Time{}, touches: map[string]int{}}
}

func (f *fakeActivity) TouchSandboxActivity(_ context.Context, ws, id string, at time.Time) (*time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := ws + "/" + id
	f.touches[k]++
	if at.After(f.last[k]) {
		f.last[k] = at
	}
	if c, ok := f.claims[k]; ok {
		return &c, nil
	}
	return nil, nil
}

func (f *fakeActivity) ClaimSandboxIdleSuspend(_ context.Context, ws, id string, since, cutoff, now, staleBefore time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := ws + "/" + id
	if _, ok := f.last[k]; !ok {
		f.last[k] = since
	}
	if f.last[k].After(cutoff) {
		return false, nil
	}
	if c, ok := f.claims[k]; ok && !c.Before(staleBefore) {
		return false, nil
	}
	f.claims[k] = now
	return true, nil
}

func (f *fakeActivity) ClearSandboxSuspendClaim(_ context.Context, ws, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.claims, ws+"/"+id)
	return nil
}

func (f *fakeActivity) PruneSandboxActivity(_ context.Context, ws string, live []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pruned = slices.Clone(live)
	return nil
}

func (f *fakeActivity) touchCount(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.touches["tea-a/"+id]
}

func (f *fakeActivity) claimed(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.claims["tea-a/"+id]
	return ok
}

type idleFixture struct {
	os       *fakeOpenSandbox
	activity *fakeActivity
	meter    *fakeMeterStore
	svc      *Service
	sweeper  *IdleSuspender
	now      time.Time
	gwStates []string // the sandbox's state each time the exec gateway was dialed
	mu       sync.Mutex
}

func newIdleFixture(t *testing.T) *idleFixture {
	t.Helper()
	f := &idleFixture{activity: newFakeActivity(), meter: &fakeMeterStore{keys: []store.SandboxTenantKey{{WorkspaceID: "tea-a", APIKey: "key-a"}}}}
	f.now = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	var client *Client
	f.os, client = newFakeOpenSandbox(t)
	secret := []byte("idle-exec-secret")
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/sandbox-files") {
			// File transfers: the ticket path is covered by file_connect_test.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Tickets are minted on the fixture's clock.
		claims, err := sandboxexec.Verify(secret, r.Header.Get(sandboxexec.TicketHeader), f.now)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.mu.Lock()
		f.gwStates = append(f.gwStates, f.os.state(claims.SandboxID))
		f.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: output\ndata: {\"stream\":\"stdout\",\"data\":\"marker\\n\"}\n\nevent: exit\ndata: {\"exit_code\":0}\n\n"))
	}))
	t.Cleanup(gw.Close)
	f.svc = &Service{
		Base:       &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a"}, Clock: func() time.Time { return f.now }},
		Client:     client,
		Exec:       &ExecConfig{Secret: secret, GatewayURL: gw.URL, FileGatewayURL: gw.URL + "/sandbox-files", Client: gw.Client(), Nonces: &sshgateway.NonceGuard{Store: &gatewaytest.FakeStore{}}},
		Meter:      &Meter{Store: f.meter, Now: func() time.Time { return f.now }},
		Activity:   f.activity,
		ResumePoll: time.Millisecond,
	}
	f.sweeper = &IdleSuspender{Service: f.svc, Tenants: f.meter}
	return f
}

func (f *idleFixture) dialedStates() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.gwStates)
}

// phases is the sandbox's meter phase sequence, repeats collapsed (every
// authorized read also samples the phase).
func (f *idleFixture) phases(id string) []string {
	var out []string
	for _, o := range f.meter.observations {
		if o.SandboxID == id && (len(out) == 0 || out[len(out)-1] != o.Phase) {
			out = append(out, o.Phase)
		}
	}
	return out
}

func TestIdleSweeperSuspendsOnlySandboxesPastTheirIdleTimeout(t *testing.T) {
	f := newIdleFixture(t)
	created := f.now.Add(-20 * time.Minute)
	f.os.add("os-default", "Running", created, "900", nil)                // Render's default, untouched 20 min
	f.os.add("os-five", "Running", f.now.Add(-6*time.Minute), "300", nil) // --idle-timeout 300, 6 min
	f.os.add("os-disabled", "Running", created, "0", nil)                 // --idle-timeout 0
	f.os.add("os-legacy", "Running", created, "", nil)                    // pre-w2/m278: reports 0
	f.os.add("os-young", "Running", f.now.Add(-10*time.Minute), "900", nil)
	f.os.add("os-busy", "Running", created, "900", nil)
	f.os.add("os-paused", "Paused", created, "900", nil)
	f.os.add("os-agent", "Running", created, "900", map[string]string{metadataAgentSession: "ags-1"})
	f.os.add("os-foreign", "Running", created, "900", map[string]string{metadataWorkspace: "tea-b"})
	if _, err := f.activity.TouchSandboxActivity(context.Background(), "tea-a", "os-busy", f.now.Add(-2*time.Minute)); err != nil {
		t.Fatal(err)
	}

	if n := f.sweeper.Sweep(context.Background()); n != 2 {
		t.Fatalf("suspended %d sandboxes, want 2", n)
	}
	calls := f.os.lifecycleCalls()
	slices.Sort(calls)
	if want := []string{"pause os-default", "pause os-five"}; !slices.Equal(calls, want) {
		t.Fatalf("lifecycle calls = %v, want %v", calls, want)
	}
	// Billing closes the running interval at the moment of suspension.
	for _, id := range []string{"os-default", "os-five"} {
		if got := f.phases(id); !slices.Equal(got, []string{"suspending"}) {
			t.Errorf("%s meter phases = %v, want [suspending]", id, got)
		}
	}
	if len(f.activity.pruned) != 9 {
		t.Errorf("pruned against %d live sandboxes, want all 9 listed", len(f.activity.pruned))
	}
	// A second pass pauses nothing more: the paused ones are no longer running.
	if n := f.sweeper.Sweep(context.Background()); n != 0 {
		t.Fatalf("second sweep suspended %d", n)
	}
	// The detail reports the in-flight and settled phases as Render does.
	sb, err := f.svc.Get(callerCtx(), "os-default")
	if err != nil {
		t.Fatal(err)
	}
	if sb.Status != StatusSuspended || sb.IdleTimeoutSeconds != 900 {
		t.Fatalf("detail = %s idle %d, want suspended / 900", sb.Status, sb.IdleTimeoutSeconds)
	}
}

func TestIdleSweeperReleasesItsClaimWhenThePauseFails(t *testing.T) {
	f := newIdleFixture(t)
	f.os.add("os-1", "Running", f.now.Add(-time.Hour), "900", nil)
	f.os.pauseOK = false
	if n := f.sweeper.Sweep(context.Background()); n != 0 {
		t.Fatalf("suspended %d with a failing pause", n)
	}
	if f.activity.claimed("os-1") {
		t.Fatal("a failed pause left its claim behind, which would stall the next exec")
	}
	if len(f.meter.observations) != 0 {
		t.Fatalf("a failed pause recorded meter phases %v", f.meter.observations)
	}
}

// TestIdleSandboxSuspendsAndExecResumesIt is the t003 flow: an idle sandbox
// is suspended by the sweeper, and the next exec resumes it, waits for it to
// run, and only then runs the command.
func TestIdleSandboxSuspendsAndExecResumesIt(t *testing.T) {
	f := newIdleFixture(t)
	f.os.add("os-1", "Running", f.now.Add(-16*time.Minute), "900", nil)
	if n := f.sweeper.Sweep(context.Background()); n != 1 {
		t.Fatalf("sweep suspended %d, want 1", n)
	}
	if got := f.os.state("os-1"); got != "Pausing" {
		t.Fatalf("state after sweep = %s, want Pausing", got)
	}
	sb, err := f.svc.Get(callerCtx(), "os-1") // settles to Paused; a read never resumes
	if err != nil || sb.Status != StatusSuspended {
		t.Fatalf("detail = %+v %v, want suspended", sb, err)
	}

	f.now = f.now.Add(time.Hour)
	rr := httptest.NewRecorder()
	if err := f.svc.StreamExec(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "cat /root/marker"}, rr, rr.Flush); err != nil {
		t.Fatalf("exec on a suspended sandbox: %v", err)
	}
	if !strings.Contains(rr.Body.String(), "marker") || !strings.Contains(rr.Body.String(), `"exit_code":0`) {
		t.Fatalf("exec output = %s", rr.Body.String())
	}
	if got := f.os.lifecycleCalls(); !slices.Equal(got, []string{"pause os-1", "resume os-1"}) {
		t.Fatalf("lifecycle calls = %v", got)
	}
	if got := f.dialedStates(); !slices.Equal(got, []string{"Running"}) {
		t.Fatalf("exec reached the gateway with the sandbox in %v, want only Running", got)
	}
	if got := f.phases("os-1"); !slices.Equal(got, []string{"suspending", "suspended", "resuming"}) {
		t.Fatalf("meter phases = %v, want suspending, suspended, resuming", got)
	}
	if f.activity.claimed("os-1") {
		t.Fatal("the sweeper claim survived the resume")
	}
	// The exec restarted the idle window: the sandbox is not idle again.
	if n := f.sweeper.Sweep(context.Background()); n != 0 {
		t.Fatalf("a just-resumed sandbox was suspended again (%d)", n)
	}
}

// TestExecArrivingDuringAClaimWaitsForThePauseAndResumes closes the race in
// which an exec reads a sandbox as running just after a sweeper claimed it:
// the exec must not run into the pause.
func TestExecArrivingDuringAClaimWaitsForThePauseAndResumes(t *testing.T) {
	f := newIdleFixture(t)
	f.os.add("os-1", "Running", f.now.Add(-time.Hour), "900", nil)
	if ok, _ := f.activity.ClaimSandboxIdleSuspend(context.Background(), "tea-a", "os-1", f.now.Add(-time.Hour), f.now.Add(-15*time.Minute), f.now, f.now.Add(-suspendClaimGrace)); !ok {
		t.Fatal("claim")
	}
	// The sweeper's pause lands after the exec has read the sandbox.
	f.os.set("os-1", func(b *fakeBox) { b.next, b.delay = "Paused", 2 })
	if _, err := f.svc.ExecBuffered(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "true"}); err != nil {
		t.Fatal(err)
	}
	if got := f.os.lifecycleCalls(); !slices.Equal(got, []string{"resume os-1"}) {
		t.Fatalf("lifecycle calls = %v, want one resume", got)
	}
	if got := f.dialedStates(); !slices.Equal(got, []string{"Running"}) {
		t.Fatalf("gateway dialed in %v", got)
	}
}

func TestStaleClaimDoesNotHoldAnExec(t *testing.T) {
	f := newIdleFixture(t)
	f.os.add("os-1", "Running", f.now.Add(-time.Hour), "900", nil)
	f.activity.claims["tea-a/os-1"] = f.now.Add(-time.Minute) // its sweeper died before pausing
	if _, err := f.svc.ExecBuffered(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "true"}); err != nil {
		t.Fatal(err)
	}
	if calls := f.os.lifecycleCalls(); len(calls) != 0 {
		t.Fatalf("lifecycle calls = %v, want none", calls)
	}
	if f.activity.claimed("os-1") {
		t.Fatal("stale claim not cleared")
	}
}

func TestResumeWaitIsBoundedAndNamed(t *testing.T) {
	f := newIdleFixture(t)
	f.svc.Base.Clock = nil // real time, so the wait's deadline passes
	f.os.add("os-1", "Paused", time.Now().Add(-time.Hour), "900", nil)
	f.os.set("os-1", func(b *fakeBox) { b.stuck = true })
	_, err := f.svc.ExecBuffered(withResumeWait(callerCtx(), 30*time.Millisecond), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "true"})
	wantCode(t, err, CodeSandboxResuming)
	if !strings.Contains(err.Error(), "retry") {
		t.Errorf("message %q does not tell the caller to retry", err.Error())
	}
	if len(f.dialedStates()) != 0 {
		t.Fatal("the command reached a sandbox that was not running")
	}
}

func TestFailedResumeIsNamed(t *testing.T) {
	f := newIdleFixture(t)
	f.os.add("os-1", "Paused", f.now.Add(-time.Hour), "900", nil)
	f.os.set("os-1", func(b *fakeBox) { b.afterResume = "Failed" })
	_, err := f.svc.ExecBuffered(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "true"})
	wantCode(t, err, "SANDBOX_RESUME_FAILED")
}

func TestAgentSessionSandboxesAreNotIdleManaged(t *testing.T) {
	f := newIdleFixture(t)
	f.os.add("os-agent", "Paused", f.now.Add(-time.Hour), "", map[string]string{metadataAgentSession: "ags-1"})
	_, _ = f.svc.ExecBuffered(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-agent", Command: "true"})
	if calls := f.os.lifecycleCalls(); len(calls) != 0 {
		t.Fatalf("generic exec drove the agent-session lifecycle: %v", calls)
	}
	if f.activity.touchCount("os-agent") != 0 {
		t.Fatal("agent-session sandbox activity tracked")
	}
}

// TestEveryInteractionPathRecordsActivity is t002: each exec, run-token,
// file-token and file-transfer path bumps activity; reading status does not
// (Render: checking a sandbox's status doesn't resume it).
func TestEveryInteractionPathRecordsActivity(t *testing.T) {
	ctx := callerCtx()
	for _, tc := range []struct {
		name string
		do   func(*idleFixture) error
	}{
		{"stream exec", func(f *idleFixture) error {
			rr := httptest.NewRecorder()
			return f.svc.StreamExec(ctx, ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "true"}, rr, rr.Flush)
		}},
		{"buffered exec (MCP)", func(f *idleFixture) error {
			_, err := f.svc.ExecBuffered(ctx, ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "true"})
			return err
		}},
		{"run connect token", func(f *idleFixture) error {
			_, err := f.svc.ConnectRun(ctx, ConnectRequest{OwnerID: "tea-a", SandboxID: "os-1", Operation: ConnectOperationStream, Command: "true"}, "https://api.example")
			return err
		}},
		{"file connect token", func(f *idleFixture) error {
			_, err := f.svc.ConnectFile(ctx, FileConnectRequest{OwnerID: "tea-a", SandboxID: "os-1", Operation: "upload", Path: "/root/f"}, "https://api.example")
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newIdleFixture(t)
			f.os.add("os-1", "Running", f.now.Add(-time.Hour), "900", nil)
			if err := tc.do(f); err != nil {
				t.Fatal(err)
			}
			if f.activity.touchCount("os-1") == 0 {
				t.Fatal("no activity recorded")
			}
		})
	}
	t.Run("status reads", func(t *testing.T) {
		f := newIdleFixture(t)
		f.os.add("os-1", "Paused", f.now.Add(-time.Hour), "900", nil)
		if _, err := f.svc.Get(ctx, "os-1"); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.List(ctx); err != nil {
			t.Fatal(err)
		}
		if f.activity.touchCount("os-1") != 0 || len(f.os.lifecycleCalls()) != 0 {
			t.Fatal("a status read counted as activity or resumed the sandbox")
		}
	})
}

// TestFileTransferResumesAndHoldsTheSandboxActive covers the redeem half of
// a file transfer: it wakes a suspended sandbox and keeps it active for the
// whole transfer.
func TestFileTransferResumesAndHoldsTheSandboxActive(t *testing.T) {
	f := newIdleFixture(t)
	f.svc.Base.Clock = nil // token bounds are checked against real time
	f.os.add("os-1", "Running", time.Now().Add(-time.Hour), "900", nil)
	mux := http.NewServeMux()
	f.svc.RegisterREST(mux)
	root := http.NewServeMux()
	root.Handle("/v1/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mux.ServeHTTP(w, r.WithContext(callerCtx())) }))
	root.Handle(ConnectFileUploadPattern, f.svc.ConnectFileHandler())
	api := httptest.NewServer(root)
	t.Cleanup(api.Close)
	minted := (&fileConnectFixture{api: api}).mint(t, "upload", "/root/f")
	// Suspended between the mint and the transfer.
	f.os.set("os-1", func(b *fakeBox) { b.state = "Paused" })
	before := f.activity.touchCount("os-1")
	resp := redeemFile(t, minted, strings.NewReader("data"))
	if resp.StatusCode >= 300 {
		t.Fatalf("redeem status %d", resp.StatusCode)
	}
	if got := f.os.lifecycleCalls(); !slices.Equal(got, []string{"resume os-1"}) {
		t.Fatalf("lifecycle calls = %v, want one resume", got)
	}
	// One touch on entry and one when the transfer ends. The closing touch
	// runs as the handler returns, which can trail the client reading the
	// response, so wait briefly for it.
	got := 0
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if got = f.activity.touchCount("os-1") - before; got >= 2 {
			break
		}
	}
	if got < 2 {
		t.Fatalf("transfer recorded %d touches, want start and end", got)
	}
}

func TestOpenExecStreamHeartbeatsActivity(t *testing.T) {
	f := newIdleFixture(t)
	f.os.add("os-1", "Running", f.now.Add(-time.Hour), "900", nil)
	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		<-release
		_, _ = w.Write([]byte("event: exit\ndata: {\"exit_code\":0}\n\n"))
	}))
	t.Cleanup(slow.Close)
	f.svc.Exec.GatewayURL = slow.URL
	f.svc.ActivityHeartbeat = 2 * time.Millisecond
	done := make(chan error, 1)
	go func() {
		_, err := f.svc.ExecBuffered(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "sleep 600"})
		done <- err
	}()
	deadline := time.After(5 * time.Second)
	for f.activity.touchCount("os-1") < 5 {
		select {
		case <-deadline:
			t.Fatalf("only %d touches while the command ran", f.activity.touchCount("os-1"))
		case <-time.After(time.Millisecond):
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestResolveIdleTimeout(t *testing.T) {
	ptr := func(v int) *int { return &v }
	for _, tc := range []struct {
		in   *int
		want int
		ok   bool
	}{
		{nil, 900, true}, {ptr(0), 0, true}, {ptr(300), 300, true}, {ptr(86400), 86400, true},
		{ptr(299), 0, false}, {ptr(86401), 0, false}, {ptr(-1), 0, false},
	} {
		got, err := resolveIdleTimeout(tc.in)
		if (err == nil) != tc.ok || got != tc.want {
			t.Errorf("resolveIdleTimeout(%v) = %d, %v", tc.in, got, err)
		}
	}
	for v, want := range map[string]int{"": 0, "900": 900, "0": 0, "junk": 0, "-5": 0} {
		if got := idleTimeoutFromMetadata(v); got != want {
			t.Errorf("idleTimeoutFromMetadata(%q) = %d, want %d", v, got, want)
		}
	}
}

func TestSuspendLifecycleStatusesMatchRender(t *testing.T) {
	for state, want := range map[string]Status{
		"Running": StatusRunning, "Pausing": StatusSuspending, "Paused": StatusSuspended,
		"Resuming": StatusResuming, "Allocated": StatusCreating, "Failed": StatusErrored,
	} {
		if got := mapOpenSandboxStatus(state); got != want {
			t.Errorf("%s → %s, want %s", state, got, want)
		}
	}
	// Only running accrues compute; every suspend phase has its own name.
	for state, want := range map[string]string{"Pausing": "suspending", "Paused": "suspended", "Resuming": "resuming"} {
		if got := canonicalMeterPhase(state); got != want {
			t.Errorf("meter phase of %s = %s, want %s", state, got, want)
		}
	}
}

func TestExplicitLifecycleConflictIsNamed(t *testing.T) {
	f := newIdleFixture(t)
	f.os.add("os-1", "Paused", f.now.Add(-time.Hour), "900", nil)
	err := f.svc.Suspend(callerCtx(), "os-1")
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "SANDBOX_LIFECYCLE_CONFLICT" {
		t.Fatalf("pausing a paused sandbox = %v, want SANDBOX_LIFECYCLE_CONFLICT", err)
	}
}

// reconcile runs one inventory pass over tea-a on the fixture's clock.
func (f *idleFixture) reconcile(t *testing.T) InventoryCounts {
	t.Helper()
	counts, err := NewAgentSessionLifecycle(f.svc).ReconcileWorkspaceInventory(context.Background(), "tea-a", "key-a", f.now,
		func(context.Context, string) (SessionClaim, error) { return SessionClaim{}, nil })
	if err != nil {
		t.Fatal(err)
	}
	return counts
}

func rfc3339(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// TestResumeRestartsRuntimeTimeoutAndSuspensionKeepsTheSandbox is the t010
// flow (Render's sandboxes-limits): an idle-suspended sandbox outlives its
// runtime timeout while suspended, the OpenSandbox expiry is moved off
// createdAt + timeout for the suspended window, and the exec that resumes it
// restarts the timeout from the resume, on both the reconcile and the expiry.
func TestResumeRestartsRuntimeTimeoutAndSuspensionKeepsTheSandbox(t *testing.T) {
	f := newIdleFixture(t)
	created := f.now.Add(-16 * time.Minute)
	f.os.add("os-1", "Running", created, "900", map[string]string{metadataTimeout: "3600"})
	if n := f.sweeper.Sweep(context.Background()); n != 1 {
		t.Fatalf("sweep suspended %d, want 1", n)
	}
	suspendedAt := f.now
	box := f.os.box("os-1")
	if box.metadata[metadataSuspendedAt] != stampMetadataTime(suspendedAt) {
		t.Fatalf("suspension stamp = %q, want %s", box.metadata[metadataSuspendedAt], stampMetadataTime(suspendedAt))
	}
	if box.expires == nil || !box.expires.Equal(suspendedAt.Add(suspendedRetention)) {
		t.Fatalf("suspended expiry = %v, want suspension + 30 days", box.expires)
	}

	// Two days suspended: far past createdAt + 3600 s, still resumable.
	f.now = f.now.Add(48 * time.Hour)
	if c := f.reconcile(t); c.Terminated != 0 || c.LongLived != 1 {
		t.Fatalf("reconcile of a sandbox suspended past its timeout = %+v, want kept as long_lived", c)
	}
	resumedAt := f.now
	if _, err := f.svc.ExecBuffered(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "true"}); err != nil {
		t.Fatalf("exec on a sandbox suspended for 2 days: %v", err)
	}
	box = f.os.box("os-1")
	if box.metadata[metadataResumedAt] != stampMetadataTime(resumedAt) {
		t.Fatalf("resume stamp = %q, want %s", box.metadata[metadataResumedAt], stampMetadataTime(resumedAt))
	}
	if want := []string{"os-1 " + rfc3339(suspendedAt.Add(suspendedRetention)), "os-1 " + rfc3339(resumedAt.Add(time.Hour))}; !slices.Equal(f.os.renewCalls(), want) {
		t.Fatalf("renewals = %v, want %v", f.os.renewCalls(), want)
	}

	// The timeout runs from the resume: kept at 59 min, reaped at 60.
	f.now = resumedAt.Add(59 * time.Minute)
	if c := f.reconcile(t); c.Terminated != 0 {
		t.Fatalf("reaped %d before resume + timeout", c.Terminated)
	}
	f.now = resumedAt.Add(time.Hour)
	if c := f.reconcile(t); c.Terminated != 1 {
		t.Fatalf("reaped %d at resume + timeout, want 1", c.Terminated)
	}
}

// TestSuspendedSandboxIsReapedAfterThirtyDays: the suspended window bounds
// every sandbox, including one with no timeout (which has no OpenSandbox
// expiry to move, so none is renewed).
func TestSuspendedSandboxIsReapedAfterThirtyDays(t *testing.T) {
	f := newIdleFixture(t)
	f.os.add("os-1", "Running", f.now.Add(-time.Hour), "900", map[string]string{metadataTimeout: timeoutNone})
	if err := f.svc.Suspend(callerCtx(), "os-1"); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	suspendedAt := f.now
	if got := f.os.box("os-1").metadata[metadataSuspendedAt]; got != stampMetadataTime(suspendedAt) {
		t.Fatalf("suspension stamp = %q", got)
	}
	if got := f.os.renewCalls(); len(got) != 0 {
		t.Fatalf("renewed a no-timeout sandbox's expiry: %v", got)
	}
	f.os.set("os-1", func(b *fakeBox) { b.state, b.next = "Paused", "" })
	f.now = suspendedAt.Add(suspendedRetention - time.Second)
	if c := f.reconcile(t); c.Terminated != 0 {
		t.Fatalf("reaped a sandbox suspended under 30 days")
	}
	f.now = suspendedAt.Add(suspendedRetention)
	if c := f.reconcile(t); c.Terminated != 1 {
		t.Fatalf("reaped %d at 30 days suspended, want 1", c.Terminated)
	}
}

// TestExplicitResumeRestartsTimeout: the resume verb stamps and renews like
// wake-on-exec, and a suspend refused for a settled phase does not restart
// the suspended window.
func TestExplicitResumeRestartsTimeout(t *testing.T) {
	f := newIdleFixture(t)
	suspendedAt := f.now.Add(-10 * 24 * time.Hour)
	f.os.add("os-1", "Paused", f.now.Add(-11*24*time.Hour), "900", map[string]string{
		metadataTimeout: "86400", metadataSuspendedAt: stampMetadataTime(suspendedAt),
	})
	if err := f.svc.Suspend(callerCtx(), "os-1"); err == nil {
		t.Fatal("suspending a suspended sandbox succeeded")
	}
	if got := f.os.box("os-1").metadata[metadataSuspendedAt]; got != stampMetadataTime(suspendedAt) {
		t.Fatalf("a refused suspend restamped the suspension: %q", got)
	}
	if err := f.svc.Resume(callerCtx(), "os-1"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if got := f.os.box("os-1").metadata[metadataResumedAt]; got != stampMetadataTime(f.now) {
		t.Fatalf("resume stamp = %q", got)
	}
	if want := []string{"os-1 " + rfc3339(f.now.Add(24*time.Hour))}; !slices.Equal(f.os.renewCalls(), want) {
		t.Fatalf("renewals = %v, want %v", f.os.renewCalls(), want)
	}
}

// TestLifetimeStampFailureRefusesTheTransition: the stamp is the
// transition's precondition. Without it the sweeper leaves the sandbox
// running (and releases its claim), and a resume is refused rather than run
// against a timeout the reconcile would read from creation.
func TestLifetimeStampFailureRefusesTheTransition(t *testing.T) {
	f := newIdleFixture(t)
	f.os.add("os-run", "Running", f.now.Add(-time.Hour), "900", nil)
	f.os.add("os-paused", "Paused", f.now.Add(-time.Hour), "900", nil)
	f.os.patchOK = false
	if n := f.sweeper.Sweep(context.Background()); n != 0 {
		t.Fatalf("suspended %d without a stamp", n)
	}
	if f.activity.claimed("os-run") {
		t.Fatal("a failed stamp left the sweeper claim behind")
	}
	if err := f.svc.Resume(callerCtx(), "os-paused"); err == nil {
		t.Fatal("resume without a stamp succeeded")
	}
	if calls := f.os.lifecycleCalls(); len(calls) != 0 {
		t.Fatalf("lifecycle calls = %v, want none", calls)
	}
}
