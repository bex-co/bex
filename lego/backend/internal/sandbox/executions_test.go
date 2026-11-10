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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// fakeExecutions is an in-memory ExecutionStore with the PG store's
// semantics (newest first, a stop recorded once).
type fakeExecutions struct {
	mu        sync.Mutex
	rows      []store.SandboxExecution
	recordErr error
	seen      map[string]time.Time
}

func (f *fakeExecutions) TouchSandboxExecution(_ context.Context, ws, id string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.WorkspaceID == ws && r.ID == id && r.StoppedAt == nil {
			if f.seen == nil {
				f.seen = map[string]time.Time{}
			}
			f.seen[id] = at
		}
	}
	return nil
}

func (f *fakeExecutions) StopStaleSandboxExecutions(_ context.Context, ws, box string, before time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.rows {
		r := &f.rows[i]
		last, seen := f.seen[r.ID]
		if !seen {
			last = r.StartedAt
		}
		if r.WorkspaceID == ws && r.SandboxID == box && r.StoppedAt == nil && last.Before(before) {
			r.StoppedAt = &last
		}
	}
	return nil
}

func (f *fakeExecutions) RecordSandboxExecution(_ context.Context, e store.SandboxExecution) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recordErr != nil {
		return f.recordErr
	}
	f.rows = append(f.rows, e)
	return nil
}

func (f *fakeExecutions) StopSandboxExecution(_ context.Context, ws, id string, at time.Time, exit *int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.rows {
		if r := &f.rows[i]; r.WorkspaceID == ws && r.ID == id && r.StoppedAt == nil {
			r.StoppedAt, r.ExitCode = &at, exit
		}
	}
	return nil
}

func (f *fakeExecutions) StopOpenSandboxExecutions(_ context.Context, ws, box string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.rows {
		if r := &f.rows[i]; r.WorkspaceID == ws && r.SandboxID == box && r.StoppedAt == nil {
			r.StoppedAt = &at
		}
	}
	return nil
}

func (f *fakeExecutions) SandboxExecutions(_ context.Context, ws, box, after string, limit int) ([]store.SandboxExecution, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.SandboxExecution
	for _, r := range slices.Backward(f.rows) {
		if r.WorkspaceID == ws && r.SandboxID == box {
			out = append(out, r)
		}
	}
	if after != "" {
		i := slices.IndexFunc(out, func(r store.SandboxExecution) bool { return r.ID == after })
		out = out[i+1:]
	}
	return out[:min(limit, len(out))], nil
}

func (f *fakeExecutions) SandboxExecution(_ context.Context, ws, box, id string) (store.SandboxExecution, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.rows {
		if r.WorkspaceID == ws && r.SandboxID == box && r.ID == id {
			return r, nil
		}
	}
	return store.SandboxExecution{}, store.ErrNotFound
}

func (f *fakeExecutions) PruneSandboxExecutions(context.Context, string, []string) error { return nil }

func (f *fakeExecutions) snapshot() []store.SandboxExecution {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.rows)
}

type fakeUserIDs map[string]string

func (f fakeUserIDs) LookupOwnerIDs(_ context.Context, subjects []string) (map[string]string, error) {
	out := map[string]string{}
	for _, s := range subjects {
		if own, ok := f[s]; ok {
			out[s] = own
		}
	}
	return out, nil
}

func getJSON(t *testing.T, url string, out any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}

// w32/m298: a run is recorded when its token is minted (Render's startedAt is
// the provisioning time), stopped with the exit code its stream reported, and
// listed on Render's /execs paths with Render's fields.
func TestARunIsRecordedAtMintAndStoppedWithItsExitCode(t *testing.T) {
	c := loadExecContract(t)
	f := newConnectFixture(t, "id-a", nil)
	execs := &fakeExecutions{}
	f.svc.Executions = execs
	f.svc.UserIDs = fakeUserIDs{"id-a": "own-cph1rs3idesc73a2b2mg"}
	minted := *f.clock

	_, out := f.mint(t, c.Connect.Path+"?"+c.Connect.Query, c.Command)
	rows := execs.snapshot()
	if len(rows) != 1 || rows[0].ID != out["executionId"] || rows[0].Type != "runs" || rows[0].Operation != "stream" ||
		rows[0].Command != c.Command || rows[0].Subject != "id-a" || !rows[0].StartedAt.Equal(minted) || rows[0].StoppedAt != nil {
		t.Fatalf("after mint = %+v, want one open runs/stream record for %v", rows, out["executionId"])
	}
	*f.clock = minted.Add(3 * time.Second)
	f.redeem(t, out["uri"].(string), out["token"].(string), c.Command)
	rows = execs.snapshot()
	if rows[0].StoppedAt == nil || rows[0].ExitCode == nil || *rows[0].ExitCode != 7 {
		t.Fatalf("after the stream = %+v, want stopped with exit 7", rows[0])
	}

	var list []map[string]any
	if code := getJSON(t, f.api.URL+"/v1/sandboxes/os-1/execs?ownerId=tea-a", &list); code != http.StatusOK || len(list) != 1 {
		t.Fatalf("list = %d %v", code, list)
	}
	exec, _ := list[0]["execution"].(map[string]any)
	if list[0]["cursor"] != out["executionId"] || exec["id"] != out["executionId"] || exec["sandboxId"] != "os-1" ||
		exec["type"] != "runs" || exec["operation"] != "stream" || exec["exitCode"] != float64(7) ||
		exec["userId"] != "usr-cph1rs3idesc73a2b2mg" || exec["startedAt"] == nil || exec["stoppedAt"] == nil {
		t.Errorf("list item = %v, want Render's execution with cursor", list[0])
	}
	var one map[string]any
	if code := getJSON(t, f.api.URL+"/v1/sandboxes/os-1/execs/"+out["executionId"].(string), &one); code != http.StatusOK || one["id"] != out["executionId"] {
		t.Errorf("get = %d %v", code, one)
	}
	if code := getJSON(t, f.api.URL+"/v1/sandboxes/os-1/execs/exe-cph1rs3idesc73a2b2mg", &one); code != http.StatusNotFound || one["code"] != "SANDBOX_EXECUTION_NOT_FOUND" {
		t.Errorf("unknown execution = %d %v", code, one)
	}
}

// w8/m151: a record whose stream never stopped it (bex-api restarted
// mid-exec, or a minted token was never redeemed) reads as stopped at its last
// heartbeat once it has gone unrefreshed past executionStaleAfter; a record
// still inside that window stays open.
func TestAnAbandonedExecutionRecordReadsStopped(t *testing.T) {
	c := loadExecContract(t)
	f := newConnectFixture(t, "id-a", nil)
	execs := &fakeExecutions{}
	f.svc.Executions = execs
	minted := *f.clock
	_, out := f.mint(t, c.Connect.Path+"?"+c.Connect.Query, c.Command)

	list := func() map[string]any {
		t.Helper()
		var page []map[string]any
		if code := getJSON(t, f.api.URL+"/v1/sandboxes/os-1/execs?ownerId=tea-a", &page); code != http.StatusOK || len(page) != 1 {
			t.Fatalf("list = %d %v", code, page)
		}
		exec, _ := page[0]["execution"].(map[string]any)
		return exec
	}
	*f.clock = minted.Add(executionStaleAfter - time.Second)
	if exec := list(); exec["stoppedAt"] != nil {
		t.Fatalf("inside the window: %v, want still open", exec)
	}
	*f.clock = minted.Add(executionStaleAfter + time.Second)
	exec := list()
	if stopped, _ := exec["stoppedAt"].(string); stopped == "" || exec["exitCode"] != nil || exec["id"] != out["executionId"] {
		t.Fatalf("abandoned: %v, want stopped with no exit code", exec)
	}
	if got, err := time.Parse(time.RFC3339Nano, exec["stoppedAt"].(string)); err != nil || !got.Equal(minted) {
		t.Errorf("stoppedAt = %v (%v), want the last time it was seen, %v", exec["stoppedAt"], err, minted)
	}

	// A running stream's heartbeat refreshes the record at once and on each
	// tick until it stops.
	svc := &Service{Base: f.svc.Base, Executions: &fakeExecutions{rows: []store.SandboxExecution{{ID: "exe-live", WorkspaceID: "tea-a", SandboxID: "os-1", StartedAt: minted}}}}
	stop := svc.keepExecutionAlive(context.Background(), "tea-a", "exe-live")
	stop()
	live := svc.Executions.(*fakeExecutions)
	if _, seen := live.seen["exe-live"]; !seen {
		t.Fatal("keepExecutionAlive never refreshed the record")
	}
}

// Another member's sandbox has no history they can see, exactly as its Get.
func TestAnotherMembersSandboxHistoryIsNotFound(t *testing.T) {
	f := newConnectFixture(t, "id-b", nil)
	f.svc.Executions = &fakeExecutions{rows: []store.SandboxExecution{{ID: "exe-1", WorkspaceID: "tea-a", SandboxID: "os-1", Type: "runs", Operation: "sync", Subject: "id-b"}}}
	var out map[string]any
	if code := getJSON(t, f.api.URL+"/v1/sandboxes/os-1/execs", &out); code != http.StatusNotFound {
		t.Errorf("list of a foreign sandbox = %d %v", code, out)
	}
	if code := getJSON(t, f.api.URL+"/v1/sandboxes/os-1/execs/exe-1", &out); code != http.StatusNotFound {
		t.Errorf("get of a foreign sandbox's execution = %d %v", code, out)
	}
}

// An exec with no connect token (MCP sandbox_exec) opens its own sync record.
func TestABufferedExecRecordsASyncRun(t *testing.T) {
	f := newConnectFixture(t, "id-a", nil)
	execs := &fakeExecutions{}
	f.svc.Executions = execs
	if _, err := f.svc.ExecBuffered(identityCtx("id-a"), ExecRequest{OwnerID: "tea-a", SandboxID: "os-1", Command: "make test"}); err != nil {
		t.Fatal(err)
	}
	rows := execs.snapshot()
	if len(rows) != 1 || rows[0].Operation != "sync" || rows[0].Command != "make test" || rows[0].SandboxID != "os-1" ||
		rows[0].ExitCode == nil || *rows[0].ExitCode != 7 {
		t.Fatalf("records = %+v, want one finished runs/sync with exit 7", rows)
	}
}

// History is informational: a store outage never refuses the command.
func TestAnExecutionStoreFailureDoesNotRefuseTheMint(t *testing.T) {
	c := loadExecContract(t)
	f := newConnectFixture(t, "id-a", nil)
	f.svc.Executions = &fakeExecutions{recordErr: errors.New("db down")}
	resp, out := f.mint(t, c.Connect.Path+"?"+c.Connect.Query, c.Command)
	if resp.StatusCode != c.Connect.Status || out["token"] == nil {
		t.Fatalf("mint = %d %v, want a token despite the store outage", resp.StatusCode, out)
	}
}

func TestATransferIsRecordedAndStoppedWithoutAnExitCode(t *testing.T) {
	f := newFileConnectFixture(t, "id-a", nil, nil)
	execs := &fakeExecutions{}
	f.svc.Executions = execs
	token := f.mint(t, "upload", "/workspace/out.txt")
	if got := redeemFile(t, token, bytes.NewReader([]byte("x"))).StatusCode; got != http.StatusNoContent {
		t.Fatalf("upload status %d", got)
	}
	rows := execs.snapshot()
	if len(rows) != 1 || rows[0].ID != token.ExecutionID || rows[0].Type != "files" || rows[0].Operation != "upload" ||
		rows[0].Command != "/workspace/out.txt" || rows[0].StoppedAt == nil || rows[0].ExitCode != nil {
		t.Fatalf("records = %+v, want one stopped files/upload of the path with no exit code", rows)
	}
}

func TestExitSnifferReadsTheExitFrameAcrossChunks(t *testing.T) {
	for name, chunks := range map[string][]string{
		"one write":          {"event: output\ndata: {\"stream\":\"stdout\",\"data\":\"event: exit\"}\n\nevent: exit\ndata: {\"exit_code\":3}\n\n"},
		"split mid-line":     {"event: ex", "it\ndata: {\"exit_", "code\":3}\n", "\n"},
		"CRLF":               {"event: exit\r\ndata: {\"exit_code\":3}\r\n\r\n"},
		"pre-m147 exitCode":  {"event: exit\ndata: {\"exitCode\":3}\n\n"},
		"keep-alive between": {": keep-alive\n\nevent: exit\n", ": keep-alive\n", "data: {\"exit_code\":3}\n\n"},
	} {
		var x exitSniffer
		for _, c := range chunks {
			x.observe([]byte(c))
		}
		if x.exitCode == nil || *x.exitCode != 3 {
			t.Errorf("%s: exit = %v, want 3", name, x.exitCode)
		}
	}
	var x exitSniffer
	x.observe([]byte("event: output\ndata: {\"stream\":\"stdout\",\"data\":\"" + strings.Repeat("a", 3*maxSniffLine) + "\"}\n"))
	if x.exitCode != nil || len(x.line) != 0 || cap(x.line) > 2*maxSniffLine {
		t.Errorf("an output line was misread or held unbounded: exit=%v held=%d", x.exitCode, cap(x.line))
	}
}

func TestTruncateCommandKeepsWholeRunes(t *testing.T) {
	long := strings.Repeat("a", maxExecutionCommand-1) + "é" + "tail"
	got := truncateCommand(long)
	if len(got) != maxExecutionCommand-1 || !strings.HasSuffix(got, "a") {
		t.Errorf("truncated to %d bytes ending %q, want the rune that would straddle the cap dropped", len(got), got[len(got)-1:])
	}
	if truncateCommand("ls") != "ls" {
		t.Error("a short command changed")
	}
}
