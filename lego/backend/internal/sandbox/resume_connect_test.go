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
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
)

// w32/m299: a connect-token mint against a suspended sandbox starts its
// resume and answers at once, as Render does: the Render SDK mints under a
// 5 s HTTP timeout, and a cold resume takes longer.
func TestAMintAgainstASuspendedSandboxAnswersAtOnce(t *testing.T) {
	for _, route := range []string{
		"/v1/sandboxes/os-1/runs/stream/token?ownerId=tea-a",
		"/v1/sandboxes/os-1/files/upload/token?ownerId=tea-a&path=rel/f.txt",
	} {
		t.Run(route, func(t *testing.T) {
			f := newIdleFixture(t)
			f.svc.Exec.FileGatewayURL = f.svc.Exec.GatewayURL + "/sandbox-files"
			f.os.add("os-1", "Paused", f.now.Add(-time.Hour), "900", nil)
			f.os.set("os-1", func(b *fakeBox) { b.stuck = true }) // never finishes resuming
			mux := http.NewServeMux()
			f.svc.RegisterREST(mux)
			done := make(chan *httptest.ResponseRecorder, 1)
			go func() {
				rr := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, route, strings.NewReader(`{"command":"true"}`)).WithContext(callerCtx())
				mux.ServeHTTP(rr, req)
				done <- rr
			}()
			select {
			case rr := <-done:
				if rr.Code != http.StatusCreated || !strings.Contains(rr.Body.String(), `"token"`) {
					t.Fatalf("mint = %d %s, want a token", rr.Code, rr.Body.String())
				}
			case <-time.After(3 * time.Second):
				t.Fatal("the mint waited for the resume")
			}
			if got := f.os.lifecycleCalls(); !slices.Equal(got, []string{"resume os-1"}) {
				t.Fatalf("lifecycle calls = %v, want the resume started", got)
			}
		})
	}
}

// A stream that must wait for a resume opens first, so the reader's idle
// timeout does not expire, and a failure then arrives as the gateway's error
// event rather than an HTTP status.
func TestAResumingStreamOpensEarlyAndFailsAsAnEvent(t *testing.T) {
	f := newIdleFixture(t)
	f.svc.Base.Clock = nil // real time, so the bounded wait expires
	f.os.add("os-1", "Paused", time.Now().Add(-time.Hour), "900", nil)
	f.os.set("os-1", func(b *fakeBox) { b.stuck = true })
	rr := httptest.NewRecorder()
	err := f.svc.StreamExec(withResumeWait(callerCtx(), 50*time.Millisecond), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "true"}, rr, rr.Flush)
	if err != nil {
		t.Fatalf("StreamExec = %v, want the failure inside the opened stream", err)
	}
	body := rr.Body.String()
	if rr.Code != http.StatusOK || rr.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("stream = %d %q", rr.Code, rr.Header().Get("Content-Type"))
	}
	if !strings.Contains(body, "event: error\ndata: ") || !strings.Contains(body, `"code":"`+CodeSandboxResuming+`"`) || !strings.Contains(body, `"status":503`) {
		t.Fatalf("body = %q, want a named error event", body)
	}
	if len(f.dialedStates()) != 0 {
		t.Fatal("the command reached a sandbox that was not running")
	}
}

// A running sandbox's stream is unchanged: no early open, and a pre-stream
// refusal is still an HTTP status (the pinned CLI's contract).
func TestARunningSandboxStreamStillRefusesWithAStatus(t *testing.T) {
	f := newIdleFixture(t)
	rr := httptest.NewRecorder()
	err := f.svc.StreamExec(callerCtx(), ExecRequest{OwnerID: "tea-a", SandboxID: "os-missing", Command: "true"}, rr, rr.Flush)
	if err == nil || rr.Body.Len() != 0 {
		t.Fatalf("err = %v, body = %q; want an error and nothing written", err, rr.Body.String())
	}
}

func TestExecStreamKeepsAWaitAlive(t *testing.T) {
	rr := httptest.NewRecorder()
	e := &execStream{w: rr, flush: rr.Flush}
	e.beat()
	if !e.started || rr.Code != http.StatusOK {
		t.Fatal("the first beat did not open the stream")
	}
	e.beat()
	if strings.Contains(rr.Body.String(), "keep-alive") {
		t.Fatal("a keep-alive was sent before the interval")
	}
	e.lastSent = time.Now().Add(-sandboxexec.KeepAliveInterval)
	e.beat()
	if rr.Body.String() != ": keep-alive\n\n" {
		t.Fatalf("body = %q, want one keep-alive comment", rr.Body.String())
	}
}
