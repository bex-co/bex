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

package sandboxfiles

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	files "github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

// w9/125: a file transfer a bex API key started ("Upload and download
// files", what its file token's mint needed) is held to the key's grant at
// redemption and on every revalidation tick, over bex-api's real grant verb.

const keyFileTick = 5 * time.Millisecond

type allowAll struct{}

func (allowAll) Check(context.Context, string, string, string) (bool, error) { return true, nil }

type grantTransfer struct {
	w        *httptest.ResponseRecorder
	done     chan struct{}
	canceled *atomic.Bool
	calls    *atomic.Int64
}

// startGrantDownload redeems one download ticket for subject naming
// keyAction ("" for a person's), whose exec streams until its context ends,
// and waits until the exec runs (or the redemption is refused).
func startGrantDownload(t *testing.T, grants *gatewaytest.KeyGrants, subject, keyAction string) grantTransfer {
	t.Helper()
	started := make(chan struct{}, 1)
	out := grantTransfer{w: httptest.NewRecorder(), done: make(chan struct{}), canceled: &atomic.Bool{}, calls: &atomic.Int64{}}
	s := serverFor(func(ctx context.Context, _ []string, _ io.Reader, _ io.Writer) (int, error) {
		out.calls.Add(1)
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		out.canceled.Store(true)
		return 0, ctx.Err()
	})
	s.TransferTimeout = time.Minute
	s.RevalidateInterval = keyFileTick
	s.Revalidator = &ExecRevalidator{Base: &core.Base{Authz: allowAll{}}, KeyGrants: &sshgateway.KeyGrantGate{Asker: grants.Client(t)}}
	token, err := files.Mint(testSecret, files.Claims{Subject: subject, Workspace: "tea-a", SandboxID: "os-a", Namespace: "tea-a-sandbox",
		Operation: files.OperationDownload, Path: "/workspace/file", KeyAction: keyAction,
		IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "http://gateway"+files.GatewayPath, bytes.NewReader(nil))
	r.Header.Set(files.TicketHeader, token)
	ctx, cancel := context.WithCancel(r.Context())
	t.Cleanup(func() { cancel(); <-out.done })
	go func() {
		defer close(out.done)
		defer func() { // an aborted response panics with http.ErrAbortHandler
			if p := recover(); p != nil && p != http.ErrAbortHandler {
				panic(p)
			}
		}()
		s.Handler().ServeHTTP(out.w, r.WithContext(ctx))
	}()
	select {
	case <-started:
	case <-out.done:
	case <-time.After(2 * time.Second):
		t.Fatal("the transfer neither started nor answered")
	}
	return out
}

func (g grantTransfer) ended(t *testing.T) {
	t.Helper()
	select {
	case <-g.done:
	case <-time.After(3 * time.Second):
		t.Fatal("the transfer did not end")
	}
}

func (g grantTransfer) wantLive(t *testing.T, after string) {
	t.Helper()
	select {
	case <-g.done:
		t.Fatalf("the transfer ended after %s: %d %q", after, g.w.Code, g.w.Body)
	default:
	}
}

func TestSandboxFileKeyTransferEndsAtTheFirstTickAfterItsGrantNarrows(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "sandbox_execution:execute")
	run := startGrantDownload(t, grants, "key-1", string(core.KeySandboxesFiles))
	mark := grants.Set("key-1", "sandboxes:read_write")
	run.ended(t)
	if run.w.Code != http.StatusForbidden || !strings.Contains(run.w.Body.String(), errRevoked.message) {
		t.Fatalf("a narrowed key's transfer answered %d %q, want 403 %q", run.w.Code, run.w.Body, errRevoked.message)
	}
	if n := grants.Reads() - mark; n != 1 {
		t.Fatalf("the transfer ended after %d grant reads since the narrowing, want 1 (the first tick)", n)
	}
	if !run.canceled.Load() {
		t.Fatal("the transfer's exec was never canceled")
	}
}

func TestSandboxFileKeyTransferSurvivesAWideningAndAnUnrelatedChange(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "sandbox_execution:execute")
	run := startGrantDownload(t, grants, "key-1", string(core.KeySandboxesFiles))
	for _, change := range []struct {
		name string
		set  func() int
	}{
		{"no change", grants.Reads},
		{"a widening", func() int { return grants.Set("key-1", "sandbox_execution:execute", "sandboxes:read_write") }},
		{"an unrelated change", func() int { return grants.Set("key-1", "sandbox_execution:execute", "objects:read") }},
		{"a widening to All", func() int { return grants.SetAll("key-1") }},
	} {
		mark := change.set()
		grants.WaitReads(t, mark+3)
		run.wantLive(t, change.name)
	}
	grants.Set("key-1", "objects:read")
	run.ended(t)
	if run.w.Code != http.StatusForbidden {
		t.Fatalf("a narrowing after the changes answered %d, want 403", run.w.Code)
	}
}

func TestSandboxFileKeyTicketIsRefusedAtRedemptionOnceItsGrantNarrowed(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "sandboxes:read_write")
	run := startGrantDownload(t, grants, "key-1", string(core.KeySandboxesFiles))
	run.ended(t)
	if run.w.Code != http.StatusForbidden {
		t.Fatalf("an uncovered key's redemption answered %d, want 403", run.w.Code)
	}
	if n := run.calls.Load(); n != 0 {
		t.Fatalf("a refused redemption exec'd %d time(s)", n)
	}
}

func TestSandboxFileKeyTransferEndsWhenItsGrantCannotBeRead(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Set("key-1", "sandbox_execution:execute")
	run := startGrantDownload(t, grants, "key-1", string(core.KeySandboxesFiles))
	grants.Fail(errors.New("postgres down"))
	run.ended(t)
	if run.w.Code != http.StatusForbidden {
		t.Fatalf("an unanswerable grant's transfer answered %d, want 403", run.w.Code)
	}
}

func TestSandboxFilePersonTransferNeverAsksForAGrant(t *testing.T) {
	grants := &gatewaytest.KeyGrants{}
	grants.Fail(errors.New("bex-api unreachable"))
	run := startGrantDownload(t, grants, "caller", "")
	time.Sleep(10 * keyFileTick)
	run.wantLive(t, "ten ticks with the verb down")
	if n := grants.Reads(); n != 0 {
		t.Fatalf("a person's transfer asked the grant verb %d times", n)
	}
}
