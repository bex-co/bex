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

package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// fakeAPI is bex-api's agent-session REST surface, in memory. A session's
// state follows its script: each GET advances one step, the last repeats.
type fakeAPI struct {
	t       *testing.T
	mu      sync.Mutex
	storage *httptest.Server
	srv     *httptest.Server

	script     map[string][]string // session id -> states served by successive GETs
	gets       map[string]int
	owner      map[string]string
	ssh        map[string]string
	failGets   map[string]int // transient 503s before the next GET succeeds
	createErrs []int          // statuses the next creates answer before succeeding
	retryAfter string         // Retry-After sent with a createErrs or failTail answer
	transcript map[string][]string
	files      map[string][]attachmentFixture
	raw        map[string]string // session id -> a GET answer served verbatim
	// The end of a turn: once a GET has served a session as settled, tailRows
	// join its transcript and the next transcript reads answer failTail.
	settled  map[string]bool
	tailRows map[string][]string
	failTail []int

	creates      []capturedRequest
	steers       []capturedRequest
	uploads      []capturedRequest
	lastList     string
	canceled     []string
	transcrGet   int
	slept        []time.Duration
	transcrQuery string
	refreshed    []string // attachment ids re-read for a fresh download URL
}

type capturedRequest struct {
	query  string
	header http.Header
	body   []byte
}

type attachmentFixture struct {
	att  attachment
	body string
}

func newFakeAPI(t *testing.T) *fakeAPI {
	f := &fakeAPI{
		t: t, script: map[string][]string{}, gets: map[string]int{}, owner: map[string]string{},
		ssh: map[string]string{}, failGets: map[string]int{}, transcript: map[string][]string{},
		files: map[string][]attachmentFixture{}, settled: map[string]bool{}, tailRows: map[string][]string{},
		raw: map[string]string{},
	}
	f.storage = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("the storage origin saw a bearer: %q", r.Header.Get("Authorization"))
		}
		if strings.Contains(r.URL.Path, "/expired/") {
			http.Error(w, "signature expired", http.StatusForbidden)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, list := range f.files {
			for _, file := range list {
				if strings.HasSuffix(r.URL.Path, "/"+file.att.ID) {
					_, _ = io.WriteString(w, file.body)
					return
				}
			}
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(f.storage.Close)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/agent-sessions", f.create)
	mux.HandleFunc("GET /v1/agent-sessions", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.lastList = r.URL.RawQuery
		f.mu.Unlock()
		writeJSON(w, http.StatusOK, []map[string]any{f.view("ags-listed1", stateSucceeded)})
	})
	mux.HandleFunc("GET /v1/agent-sessions/{id}", f.get)
	mux.HandleFunc("POST /v1/agent-sessions/{id}/steer", f.steer)
	mux.HandleFunc("POST /v1/agent-sessions/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.canceled = append(f.canceled, r.PathValue("id"))
		f.mu.Unlock()
		writeJSON(w, http.StatusOK, f.view(r.PathValue("id"), stateCanceled))
	})
	mux.HandleFunc("GET /v1/agent-sessions/{id}/transcript", f.transcriptPage)
	mux.HandleFunc("GET /v1/agent-sessions/{id}/attachments", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		out := []attachment{}
		for _, file := range f.files[r.PathValue("id")] {
			if d := r.URL.Query().Get("direction"); d == "" || d == file.att.Direction {
				out = append(out, file.att)
			}
		}
		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("GET /v1/agent-attachments/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		f.mu.Lock()
		defer f.mu.Unlock()
		f.refreshed = append(f.refreshed, id)
		for _, list := range f.files {
			for _, file := range list {
				if file.att.ID == id {
					fresh := file.att
					fresh.DownloadURL = f.storage.URL + "/fresh/" + id
					writeJSON(w, http.StatusOK, fresh)
					return
				}
			}
		}
		writeAPIError(w, http.StatusNotFound, "AGENT_ATTACHMENT_NOT_FOUND", "agent attachment not found")
	})
	mux.HandleFunc("POST /v1/agent-attachments", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.uploads = append(f.uploads, capturedRequest{query: r.URL.RawQuery, header: r.Header.Clone(), body: body})
		n := len(f.uploads)
		f.mu.Unlock()
		if r.ContentLength != int64(len(body)) {
			t.Errorf("upload Content-Length %d for %d bytes", r.ContentLength, len(body))
		}
		writeJSON(w, http.StatusCreated, map[string]any{"id": fmt.Sprintf("att-up%d", n), "direction": "input", "name": r.URL.Query().Get("name")})
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeAPIError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": msg, "message": msg, "code": code})
}

func (f *fakeAPI) view(id, state string) map[string]any {
	v := map[string]any{
		"id": id, "ownerId": f.owner[id], "state": state, "phase": "running",
		"agentConfig": map[string]any{"agent": "claude", "task": "t"}, "turns": 1,
		"outcome": map[string]any{"state": state, "turn": 1, "summary": "did " + id},
	}
	if f.owner[id] == "" {
		v["ownerId"] = "tea-fake"
	}
	if state == stateFailed {
		v["failureReasonCode"] = "AGENT_TURN_FAILED"
		v["failureReason"] = "the tests did not pass"
	}
	if ssh := f.ssh[id]; ssh != "" {
		v["sshAddress"] = ssh
	}
	return v
}

func (f *fakeAPI) create(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.creates = append(f.creates, capturedRequest{query: r.URL.RawQuery, header: r.Header.Clone(), body: body})
	var status int
	if len(f.createErrs) > 0 {
		status, f.createErrs = f.createErrs[0], f.createErrs[1:]
	}
	f.mu.Unlock()
	if f.refuse(w, status, "AGENT_SESSION_INPUT_INVALID", "refused") {
		return
	}
	writeJSON(w, http.StatusCreated, f.view("ags-new1", stateWorking))
}

// refuse answers a scripted failure status, with the fake's Retry-After, and
// reports whether there was one.
func (f *fakeAPI) refuse(w http.ResponseWriter, status int, code, msg string) bool {
	if status == 0 {
		return false
	}
	if f.retryAfter != "" {
		w.Header().Set("Retry-After", f.retryAfter)
	}
	writeAPIError(w, status, code, msg)
	return true
}

func (f *fakeAPI) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	f.mu.Lock()
	if f.failGets[id] > 0 {
		f.failGets[id]--
		f.mu.Unlock()
		writeAPIError(w, http.StatusServiceUnavailable, "", "rolling")
		return
	}
	if raw, ok := f.raw[id]; ok {
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, raw)
		return
	}
	states, ok := f.script[id]
	if !ok {
		f.mu.Unlock()
		writeAPIError(w, http.StatusNotFound, "AGENT_SESSION_NOT_FOUND", "agent session not found")
		return
	}
	i := min(f.gets[id], len(states)-1)
	f.gets[id]++
	if states[i] != stateWorking && !f.settled[id] {
		f.settled[id] = true
		f.transcript[id] = append(f.transcript[id], f.tailRows[id]...)
	}
	f.mu.Unlock()
	writeJSON(w, http.StatusOK, f.view(id, states[i]))
}

func (f *fakeAPI) steer(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.steers = append(f.steers, capturedRequest{body: body})
	f.mu.Unlock()
	v := f.view(r.PathValue("id"), stateWorking)
	v["turns"] = 2
	writeJSON(w, http.StatusOK, v)
}

// transcriptPage serves the session's rows after afterSeq, one row per page
// so paging is exercised.
func (f *fakeAPI) transcriptPage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	f.mu.Lock()
	f.transcrGet++
	f.transcrQuery = r.URL.RawQuery
	rows := f.transcript[id]
	var status int
	if f.settled[id] && len(f.failTail) > 0 {
		status, f.failTail = f.failTail[0], f.failTail[1:]
	}
	f.mu.Unlock()
	if f.refuse(w, status, "", "rolling") {
		return
	}
	after := int64(-1)
	if raw := r.URL.Query().Get("afterSeq"); raw != "" {
		_, _ = fmt.Sscan(raw, &after)
	}
	page := map[string]any{"parts": []json.RawMessage{}, "turns": []map[string]any{{"turn": 1, "prompt": "fix it"}, {"turn": 2, "prompt": "again"}}, "nextAfterSeq": after}
	for _, row := range rows {
		var seq struct {
			Seq int64 `json:"seq"`
		}
		_ = json.Unmarshal([]byte(row), &seq)
		if seq.Seq > after {
			page["parts"] = []json.RawMessage{json.RawMessage(row)}
			page["nextAfterSeq"] = seq.Seq
			break
		}
	}
	writeJSON(w, http.StatusOK, page)
}

// testDeps wires the command to the fake; opened records what open did.
type testDeps struct {
	deps
	opened   []string
	desktops []string
}

func (f *fakeAPI) deps(t *testing.T) *testDeps {
	td := &testDeps{}
	td.deps = deps{
		api: func() (*api, error) {
			return &api{
				server: f.srv.URL + "/v1/", http: f.srv.Client(), download: f.storage.Client(),
				sleep: func(ctx context.Context, d time.Duration) error {
					f.mu.Lock()
					f.slept = append(f.slept, d)
					f.mu.Unlock()
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(time.Millisecond):
						return nil
					}
				},
			}, nil
		},
		workspace:   func() (string, error) { return "tea-ws", nil },
		stdin:       strings.NewReader(""),
		interactive: func() bool { return false },
		dashboard:   func() string { return "https://dashboard.bex.co" },
		openURL:     func(u string) error { td.opened = append(td.opened, u); return nil },
		canDesktop:  func() bool { return false },
		openDesktop: func(_ context.Context, id, _ string, before func()) error {
			td.desktops = append(td.desktops, id)
			return nil
		},
		newKey: func() string { return "bex-cli-testkey" },
		lookupEnv: func(name string) (string, bool) {
			v, ok := map[string]string{"CI_STRIPE": "sk_test_from_env", "API_TOKEN": "tok-123"}[name]
			return v, ok
		},
	}
	return td
}

// run executes `bex agent args…` under a root that carries upstream's
// persistent --output flag, and returns stdout, stderr and the exit code.
func run(t *testing.T, d deps, args ...string) (string, string, int) {
	t.Helper()
	root := &cobra.Command{Use: "bex", SilenceErrors: true}
	root.PersistentFlags().StringP("output", "o", "interactive", "")
	root.AddCommand(newCommand(d))
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(append([]string{"agent"}, args...))
	err := root.Execute()
	return stdout.String(), stderr.String() + errText(err), exitCode(err)
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var coded interface{ ExitCode() int }
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	return 1
}

func TestContractFixtureDecodes(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/agent-cli-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Session        session        `json:"session"`
		Attachment     attachment     `json:"attachment"`
		TranscriptPage transcriptPage `json:"transcriptPage"`
		Error          json.RawMessage
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	s := golden.Session
	if s.ID == "" || s.OwnerID == "" || s.State != stateSucceeded || s.SSHAddress == "" || s.Outcome == nil {
		t.Fatalf("session fields lost: %+v", s)
	}
	o := s.Outcome
	if o.Turn != 1 || o.Summary == "" || o.PullRequest == nil || o.PullRequest.URL == "" || len(o.Files) != 1 || o.Files[0].ID == "" || len(o.Result) == 0 || o.Cost == nil || o.Cost.ModelUSD != "0.42" || o.Cost.ComputeUSD != "0.0895" || o.Cost.TotalUSD != "0.5095" {
		t.Fatalf("outcome fields lost: %+v", o)
	}
	a := golden.Attachment
	if a.ID == "" || a.Direction != "output" || a.SHA256 == "" || a.DownloadURL == "" || a.Turn != 1 {
		t.Fatalf("attachment fields lost: %+v", a)
	}
	page := golden.TranscriptPage
	if len(page.Parts) != 1 || page.Parts[0].Seq != 7 || page.Parts[0].PartIndex != 3 || !page.Parts[0].Partial || len(page.Parts[0].Part) == 0 || page.NextAfterSeq != 7 || page.Turns[0].Prompt == "" {
		t.Fatalf("transcript fields lost: %+v", page)
	}
	var envelope struct {
		Error json.RawMessage `json:"error"`
	}
	_ = json.Unmarshal(raw, &envelope)
	apiErr := decodeAPIError(http.StatusNotFound, envelope.Error)
	if !strings.Contains(apiErr.Error(), "AGENT_SESSION_NOT_FOUND") || !strings.Contains(apiErr.Error(), "agent session not found") {
		t.Fatalf("error envelope = %v", apiErr)
	}
}

func TestRunWaitExitsByTheTurnState(t *testing.T) {
	for _, tc := range []struct {
		final string
		code  int
		says  string
	}{
		{stateSucceeded, 0, ""},
		{stateFailed, exitFailed, "AGENT_TURN_FAILED: the tests did not pass"},
		{stateCanceled, exitCanceled, "was canceled"},
		{stateNeedsInput, exitNeedsInput, "bex agent send ags-new1"},
	} {
		t.Run(tc.final, func(t *testing.T) {
			f := newFakeAPI(t)
			f.script["ags-new1"] = []string{stateWorking, stateWorking, tc.final}
			stdout, stderr, code := run(t, f.deps(t).deps, "run", "--wait", "-o", "json", "--repo", "acme/api", "fix", "the", "crash")
			if code != tc.code || !strings.Contains(stderr, tc.says) {
				t.Fatalf("exit %d (want %d), stderr %q", code, tc.code, stderr)
			}
			var final map[string]any
			if err := json.Unmarshal([]byte(stdout), &final); err != nil {
				t.Fatalf("stdout is not the session JSON: %v\n%s", err, stdout)
			}
			if final["state"] != tc.final || final["id"] != "ags-new1" {
				t.Fatalf("printed %v", final)
			}
		})
	}
}

func TestRunSendsTheCreateContract(t *testing.T) {
	f := newFakeAPI(t)
	dir := t.TempDir()
	logPath := filepath.Join(dir, "crash.log")
	if err := os.WriteFile(logPath, []byte("panic: nil map"), 0o600); err != nil {
		t.Fatal(err)
	}
	schemaPath := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(schemaPath, []byte(`{"type": "object", "required": ["ok"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := run(t, f.deps(t).deps, "run", "--repo", "acme/api", "--branch", "dev", "--model", "m1", "--title", "T",
		"--tag", "ci", "--tag", "nightly", "--attach", logPath, "--output-schema", schemaPath, "--max-spend", "2.5", "Fix", "it")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if len(f.uploads) != 1 || string(f.uploads[0].body) != "panic: nil map" || !strings.Contains(f.uploads[0].query, "name=crash.log") || !strings.Contains(f.uploads[0].query, "ownerId=tea-ws") {
		t.Fatalf("upload = %+v", f.uploads)
	}
	if len(f.creates) != 1 {
		t.Fatalf("creates = %d", len(f.creates))
	}
	c := f.creates[0]
	if c.header.Get("Idempotency-Key") != "bex-cli-testkey" || c.query != "ownerId=tea-ws" {
		t.Fatalf("create header/query = %q %q", c.header.Get("Idempotency-Key"), c.query)
	}
	var body map[string]any
	if err := json.Unmarshal(c.body, &body); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"repo": "acme/api", "branch": "dev", "title": "T", "maxSpendUSD": 2.5,
		"agentConfig":   map[string]any{"agent": "claude", "model": "m1", "task": "Fix it"},
		"tags":          []any{"ci", "nightly"},
		"attachmentIds": []any{"att-up1"},
		"outputSchema":  map[string]any{"type": "object", "required": []any{"ok"}},
	}
	got, _ := json.Marshal(body)
	exp, _ := json.Marshal(want)
	if string(got) != string(exp) {
		t.Fatalf("create body\n got %s\nwant %s", got, exp)
	}
}

func TestRunRetriesAGatewayErrorWithTheSameKey(t *testing.T) {
	f := newFakeAPI(t)
	f.createErrs = []int{http.StatusServiceUnavailable, http.StatusBadGateway}
	stdout, stderr, code := run(t, f.deps(t).deps, "run", "task")
	if code != 0 || !strings.Contains(stdout, "ags-new1") {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if len(f.creates) != 3 {
		t.Fatalf("creates = %d, want 3", len(f.creates))
	}
	for _, c := range f.creates {
		if c.header.Get("Idempotency-Key") != "bex-cli-testkey" {
			t.Fatalf("a retry changed the key: %q", c.header.Get("Idempotency-Key"))
		}
	}
}

func TestRunDoesNotRetryARefusal(t *testing.T) {
	f := newFakeAPI(t)
	f.createErrs = []int{http.StatusBadRequest}
	_, stderr, code := run(t, f.deps(t).deps, "run", "task")
	if code != 1 || !strings.Contains(stderr, "400 AGENT_SESSION_INPUT_INVALID: refused") || len(f.creates) != 1 {
		t.Fatalf("exit %d creates %d stderr %q", code, len(f.creates), stderr)
	}
}

func TestWaitTimesOutWith124(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-slow1"] = []string{stateWorking}
	_, stderr, code := run(t, f.deps(t).deps, "wait", "ags-slow1", "--timeout", "30ms")
	if code != exitTimeout || !strings.Contains(stderr, "ags-slow1 still working after 30ms") {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
}

func TestWaitOnSeveralReportsTheWorstState(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-a1"] = []string{stateWorking, stateSucceeded}
	f.script["ags-b1"] = []string{stateFailed}
	f.script["ags-c1"] = []string{stateCanceled}
	stdout, stderr, code := run(t, f.deps(t).deps, "wait", "ags-a1", "ags-b1", "ags-c1", "-o", "json")
	if code != exitFailed || !strings.Contains(stderr, "ags-b1 failed") {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	var all []map[string]any
	if err := json.Unmarshal([]byte(stdout), &all); err != nil || len(all) != 3 {
		t.Fatalf("stdout = %s (%v)", stdout, err)
	}
	// --any returns at the first settled session.
	f.script["ags-d1"] = []string{stateWorking}
	f.script["ags-e1"] = []string{stateSucceeded}
	_, stderr, code = run(t, f.deps(t).deps, "wait", "--any", "ags-d1", "ags-e1", "--timeout", "5s")
	if code != 0 {
		t.Fatalf("--any exit %d: %s", code, stderr)
	}
}

func TestWaitSurvivesTransientReadsButNotMissingSessions(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-flaky1"] = []string{stateSucceeded}
	f.failGets["ags-flaky1"] = 3
	if _, stderr, code := run(t, f.deps(t).deps, "wait", "ags-flaky1"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	f.failGets["ags-flaky1"] = transientLimit + 1
	if _, _, code := run(t, f.deps(t).deps, "wait", "ags-flaky1"); code != 1 {
		t.Fatalf("a persistent outage exit %d, want 1", code)
	}
	_, stderr, code := run(t, f.deps(t).deps, "wait", "ags-gone1")
	if code != 1 || !strings.Contains(stderr, "AGENT_SESSION_NOT_FOUND") {
		t.Fatalf("missing session exit %d stderr %q", code, stderr)
	}
}

func TestSendUploadsToTheSessionWorkspaceAndWaits(t *testing.T) {
	f := newFakeAPI(t)
	f.owner["ags-s1"] = "tea-other"
	f.script["ags-s1"] = []string{stateSucceeded, stateWorking, stateSucceeded}
	file := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(file, []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := run(t, f.deps(t).deps, "send", "ags-s1", "--attach", file, "--wait", "add", "a", "test")
	if code != 0 || !strings.Contains(stdout, "ags-s1 · succeeded") {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if len(f.uploads) != 1 || !strings.Contains(f.uploads[0].query, "ownerId=tea-other") {
		t.Fatalf("upload went to the wrong workspace: %+v", f.uploads)
	}
	var body steerRequest
	if err := json.Unmarshal(f.steers[0].body, &body); err != nil || body.Prompt != "add a test" || !slices.Equal(body.AttachmentIDs, []string{"att-up1"}) {
		t.Fatalf("steer body = %s", f.steers[0].body)
	}
}

// ADR098: --secret names stored secrets and --secret-from-env sends a local
// env var as an inline secret, on both create and steer. A missing local
// variable is a usage error before anything reaches the API.
func TestSecretsReachCreateAndSteerFromNamesAndTheEnvironment(t *testing.T) {
	f := newFakeAPI(t)
	_, stderr, code := run(t, f.deps(t).deps, "run", "--secret", "STRIPE_KEY", "--secret-from-env", "STRIPE_TEST=CI_STRIPE",
		"--secret-from-env", "API_TOKEN", "Run", "the", "integration", "tests")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var created createRequest
	if err := json.Unmarshal(f.creates[0].body, &created); err != nil {
		t.Fatal(err)
	}
	wantInline := []inlineSecret{{Name: "STRIPE_TEST", Value: "sk_test_from_env"}, {Name: "API_TOKEN", Value: "tok-123"}}
	if !slices.Equal(created.SecretNames, []string{"STRIPE_KEY"}) || !slices.Equal(created.Secrets, wantInline) {
		t.Fatalf("create secrets = %s", f.creates[0].body)
	}

	f.script["ags-s1"] = []string{stateSucceeded}
	if _, stderr, code := run(t, f.deps(t).deps, "send", "ags-s1", "--secret", "LATER", "again"); code != 0 {
		t.Fatalf("send exit %d: %s", code, stderr)
	}
	var steered steerRequest
	if err := json.Unmarshal(f.steers[0].body, &steered); err != nil || !slices.Equal(steered.SecretNames, []string{"LATER"}) || steered.Secrets != nil {
		t.Fatalf("steer secrets = %s", f.steers[0].body)
	}

	for _, args := range [][]string{
		{"run", "--secret-from-env", "UNSET_VAR", "task"},
		{"run", "--secret-from-env", "NAME=", "task"},
		{"send", "ags-s1", "--secret-from-env", "UNSET_VAR", "again"},
	} {
		creates, steers := len(f.creates), len(f.steers)
		if _, stderr, code := run(t, f.deps(t).deps, args...); code != 2 || len(f.creates) != creates || len(f.steers) != steers {
			t.Fatalf("%v: exit %d (want 2, nothing sent) stderr %q", args, code, stderr)
		}
	}
}

// ADR099 D4: --skill NAME[:ARGUMENTS] reaches create and steer as
// skills[{name, arguments}]; an empty name is a usage error.
func TestSkillsReachCreateAndSteer(t *testing.T) {
	f := newFakeAPI(t)
	if _, stderr, code := run(t, f.deps(t).deps, "run", "--skill", "release:v1.2 staging", "--skill", "lint", "Ship", "it"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var created createRequest
	if err := json.Unmarshal(f.creates[0].body, &created); err != nil {
		t.Fatal(err)
	}
	want := []skillInvocation{{Name: "release", Arguments: "v1.2 staging"}, {Name: "lint"}}
	if !slices.Equal(created.Skills, want) {
		t.Fatalf("create skills = %s", f.creates[0].body)
	}
	f.script["ags-s1"] = []string{stateSucceeded}
	if _, stderr, code := run(t, f.deps(t).deps, "send", "ags-s1", "--skill", "lint", "again"); code != 0 {
		t.Fatalf("send exit %d: %s", code, stderr)
	}
	var steered steerRequest
	if err := json.Unmarshal(f.steers[0].body, &steered); err != nil || !slices.Equal(steered.Skills, []skillInvocation{{Name: "lint"}}) {
		t.Fatalf("steer skills = %s", f.steers[0].body)
	}
	creates := len(f.creates)
	if _, _, code := run(t, f.deps(t).deps, "run", "--skill", ":args", "task"); code != 2 || len(f.creates) != creates {
		t.Fatalf("empty skill name: exit %d, creates %d", code, len(f.creates)-creates)
	}
}

func TestPromptFromStdinAndUsageErrors(t *testing.T) {
	f := newFakeAPI(t)
	d := f.deps(t)
	d.stdin = strings.NewReader("task from stdin\n")
	if _, stderr, code := run(t, d.deps, "run", "-f", "-"); code != 0 {
		t.Fatalf("stdin prompt exit %d: %s", code, stderr)
	}
	var body createRequest
	_ = json.Unmarshal(f.creates[0].body, &body)
	if body.AgentConfig.Task != "task from stdin" {
		t.Fatalf("task = %q", body.AgentConfig.Task)
	}
	for _, args := range [][]string{
		{"run"},
		{"run", "--wait", "--follow", "x"},
		{"run", "--max-spend", "0", "x"},
		{"run", "-o", "xml", "x"},
		{"run", "--no-such-flag", "x"},
		{"get", "srv-123"},
		{"get"},
		{"get", "ags-a", "ags-b"},
		{"wait"},
		{"files", "ags-a", "--direction", "sideways"},
	} {
		if _, stderr, code := run(t, d.deps, args...); code != exitUsage {
			t.Errorf("%v: exit %d, want %d (%s)", args, code, exitUsage, stderr)
		}
	}
	if len(f.creates) != 1 {
		t.Fatalf("a usage error still created a session (%d creates)", len(f.creates))
	}
}

func row(seq, index int64, turn int, part string, partial bool) string {
	return fmt.Sprintf(`{"seq":%d,"partIndex":%d,"turn":%d,"part":%s,"partial":%t,"createdAt":"2026-11-05T10:00:00Z"}`, seq, index, turn, part, partial)
}

func TestLogsPrintsEachPartOnce(t *testing.T) {
	f := newFakeAPI(t)
	f.transcript["ags-l1"] = []string{
		row(1, 1, 1, `{"type":"reasoning","text":"hmm"}`, false),
		row(2, 2, 1, `{"type":"text","text":"Looking at the log."}`, false),
		row(3, 3, 1, `{"type":"dynamic-tool","toolName":"Terminal","state":"output-error","input":{"command":"go   test ./..."},"errorText":"exit 1\nmore"}`, false),
		row(4, 4, 1, `{"type":"text","text":"Fix","state":"streaming"}`, true),
		row(5, 4, 1, `{"type":"text","text":"Fixed it."}`, false),
		row(6, 1, 2, `{"type":"tool-read","state":"output-available","input":{"file_path":"main.go"}}`, false),
	}
	stdout, stderr, code := run(t, f.deps(t).deps, "logs", "ags-l1")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	want := "── turn 1 ──\n> fix it\nLooking at the log.\n▸ Terminal go test ./... — error: exit 1\nFixed it.\n── turn 2 ──\n> again\n▸ read main.go\n"
	if stdout != want {
		t.Fatalf("logs\n got %q\nwant %q", stdout, want)
	}
	stdout, _, _ = run(t, f.deps(t).deps, "logs", "ags-l1", "--reasoning", "--turn", "2", "-o", "json")
	if lines := strings.Split(strings.TrimSpace(stdout), "\n"); len(lines) != 1 || !strings.Contains(lines[0], `"seq":6`) {
		t.Fatalf("json lines = %q", stdout)
	}
}

func TestFollowWaitsForCompleteCopiesAndExitsByState(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-f1"] = []string{stateWorking, stateWorking, stateFailed}
	f.transcript["ags-f1"] = []string{
		row(1, 1, 1, `{"type":"text","text":"Work","state":"streaming"}`, true),
		row(2, 1, 1, `{"type":"text","text":"Working on it."}`, false),
		row(3, 2, 1, `{"type":"text","text":"interrupted","state":"streaming"}`, true),
	}
	stdout, stderr, code := run(t, f.deps(t).deps, "logs", "ags-f1", "--follow")
	if code != exitFailed {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	// The partial row 1 is superseded by its complete copy; row 3 never got
	// one, so it is shown once the turn ends.
	if stdout != "── turn 1 ──\n> fix it\nWorking on it.\ninterrupted\n" {
		t.Fatalf("follow printed %q", stdout)
	}
}

// tailWarning opens the one stderr line a follow prints when it could not
// read the end of the transcript; tailOutage is enough failing reads for that.
const tailWarning = "Warning: the end of the transcript may be missing: read transcript of agent session "

var tailOutage = slices.Repeat([]int{http.StatusServiceUnavailable}, transientLimit+1)

// w32/010: the transcript read that follows the settled session read is
// retried like every other read of a follow. When it still fails, the end of
// the transcript is lost with a warning, never the result: the command prints
// the session and exits by its state.
func TestFollowKeepsTheResultWhenTheLastTranscriptReadFails(t *testing.T) {
	const (
		head     = "── turn 1 ──\n> fix it\nWorking on it.\n"
		complete = head + "Finished the edit.\n" // the tail arrived and replaced the partial row
		partial  = head + "Finishing the ed\n"   // it did not: the held partial row is shown
	)
	for _, tc := range []struct {
		name       string
		failTail   []int
		retryAfter string
		stdout     string
		unread     int // failTail answers the follow never asked for
		warnings   int
	}{
		{name: "a 503 is retried", failTail: []int{http.StatusServiceUnavailable}, stdout: complete},
		{name: "a 429 waits out its Retry-After", failTail: []int{http.StatusTooManyRequests}, retryAfter: "7", stdout: complete},
		{name: "an outage costs the tail only", failTail: tailOutage, stdout: partial, warnings: 1},
		{name: "a refusal is not retried", failTail: []int{http.StatusForbidden, http.StatusServiceUnavailable}, stdout: partial, unread: 1, warnings: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAPI(t)
			f.script["ags-t1"] = []string{stateWorking, stateFailed}
			f.transcript["ags-t1"] = []string{
				row(1, 1, 1, `{"type":"text","text":"Working on it."}`, false),
				row(2, 2, 1, `{"type":"text","text":"Finishing the ed","state":"streaming"}`, true),
			}
			f.tailRows["ags-t1"] = []string{row(3, 2, 1, `{"type":"text","text":"Finished the edit."}`, false)}
			f.failTail, f.retryAfter = tc.failTail, tc.retryAfter
			stdout, stderr, code := run(t, f.deps(t).deps, "logs", "ags-t1", "--follow")
			if code != exitFailed || stdout != tc.stdout {
				t.Fatalf("exit %d, want %d by the session's state\nstdout %q\n  want %q\nstderr %q", code, exitFailed, stdout, tc.stdout, stderr)
			}
			if len(f.failTail) != tc.unread {
				t.Fatalf("%d failing answers were left unread, want %d", len(f.failTail), tc.unread)
			}
			if got := strings.Count(stderr, tailWarning); got != tc.warnings {
				t.Fatalf("warned %d times, want %d: %q", got, tc.warnings, stderr)
			}
			if tc.warnings > 0 && !strings.Contains(stderr, "`bex agent logs ags-t1` prints the whole transcript.") {
				t.Fatalf("the warning does not say how to read the transcript: %q", stderr)
			}
			if tc.retryAfter != "" && f.slept[len(f.slept)-1] < 7*time.Second {
				t.Fatalf("slept %v, want at least the 7s Retry-After before the retry", f.slept)
			}
		})
	}
}

// --timeout and Ctrl-C end the retries; the session that settled is still the
// result, as it is when the last read itself is cut short.
func TestFollowReturnsTheSettledSessionWhenStoppedWhileRetryingTheLastRead(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-s1"] = []string{stateSucceeded}
	f.failTail = []int{http.StatusServiceUnavailable}
	a, _ := f.deps(t).api()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.sleep = func(ctx context.Context, _ time.Duration) error { cancel(); return ctx.Err() }
	result, err := a.follow(ctx, "ags-s1", 1, newPartPrinter(io.Discard, false, false))
	if err != nil || len(result.sessions) != 1 || result.sessions[0].State != stateSucceeded || result.tailErr != nil || result.timedOut {
		t.Fatalf("follow = %+v, %v", result, err)
	}
}

// `run --follow -o json` keeps stdout to the session, also when the last
// transcript read fails (w32/010: it used to print nothing and exit 1).
func TestRunFollowKeepsJSONStdoutClean(t *testing.T) {
	for _, tc := range []struct {
		name     string
		failTail []int
		warns    bool
	}{
		{name: "every read answers"},
		{name: "the last transcript read is retried", failTail: []int{http.StatusBadGateway}},
		{name: "the last transcript read is given up", failTail: tailOutage, warns: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAPI(t)
			f.script["ags-new1"] = []string{stateWorking, stateSucceeded}
			f.transcript["ags-new1"] = []string{row(1, 1, 1, `{"type":"text","text":"streamed"}`, false)}
			f.failTail = tc.failTail
			stdout, stderr, code := run(t, f.deps(t).deps, "run", "--follow", "-o", "json", "task")
			if code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			var final map[string]any
			if err := json.Unmarshal([]byte(stdout), &final); err != nil || final["state"] != stateSucceeded {
				t.Fatalf("stdout %q (%v)", stdout, err)
			}
			if !strings.Contains(stderr, "streamed") || !strings.Contains(stderr, "Started ags-new1") {
				t.Fatalf("the stream did not go to stderr: %q", stderr)
			}
			if warned := strings.Contains(stderr, tailWarning); warned != tc.warns {
				t.Fatalf("warned = %t, want %t: %q", warned, tc.warns, stderr)
			}
		})
	}
}

func TestFilesDownloadChecksHashesAndNeverOverwrites(t *testing.T) {
	f := newFakeAPI(t)
	sum := func(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
	f.files["ags-d1"] = []attachmentFixture{
		{attachment{ID: "att-1", Turn: 1, Direction: "output", Name: "report.md", Size: 6, SHA256: sum("report"), DownloadURL: f.storage.URL + "/b/att-1"}, "report"},
		{attachment{ID: "att-2", Turn: 2, Direction: "output", Name: "../../report.md", Size: 7, SHA256: sum("report2"), DownloadURL: f.storage.URL + "/b/att-2"}, "report2"},
		{attachment{ID: "att-3", Turn: 1, Direction: "input", Name: "in.txt", Size: 2, SHA256: sum("in"), DownloadURL: f.storage.URL + "/b/att-3"}, "in"},
	}
	stdout, _, code := run(t, f.deps(t).deps, "files", "ags-d1")
	if code != 0 || !strings.Contains(stdout, "att-1") || strings.Contains(stdout, "att-3") {
		t.Fatalf("default lists outputs only: exit %d %q", code, stdout)
	}
	dir := t.TempDir()
	if _, stderr, code := run(t, f.deps(t).deps, "files", "ags-d1", "--download", dir); code != 0 {
		t.Fatalf("download exit %d: %s", code, stderr)
	}
	for name, want := range map[string]string{"report.md": "report", "report-att-2.md": "report2"} {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v", name, got, err)
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("download left %d entries (a traversal or temp file?)", len(entries))
	}
	if _, stderr, code := run(t, f.deps(t).deps, "files", "ags-d1", "--download", dir); code != 1 || !strings.Contains(stderr, "already exists") {
		t.Fatalf("a second download overwrote: exit %d %q", code, stderr)
	}
	// A corrupt transfer leaves nothing behind.
	f.files["ags-d1"][0].body = "tampered"
	fresh := t.TempDir()
	if _, stderr, code := run(t, f.deps(t).deps, "files", "ags-d1", "--download", fresh); code != 1 || !strings.Contains(stderr, "SHA-256") {
		t.Fatalf("hash mismatch exit %d %q", code, stderr)
	}
	if entries, _ := os.ReadDir(fresh); len(entries) != 0 {
		t.Fatalf("a corrupt download left %d files", len(entries))
	}
}

func TestOpenPrefersTheDesktopOnlyForARunningSandbox(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-o1"] = []string{stateSucceeded}
	d := f.deps(t)
	d.canDesktop = func() bool { return true }
	d.interactive = func() bool { return true }
	stdout, _, code := run(t, d.deps, "open", "ags-o1")
	if code != 0 || stdout != "https://dashboard.bex.co/agents/ags-o1\n" || len(d.desktops) != 0 || len(d.opened) != 1 {
		t.Fatalf("no sandbox: exit %d stdout %q desktops %v opened %v", code, stdout, d.desktops, d.opened)
	}
	f.ssh["ags-o1"] = "ags-o1@ssh.bex.co"
	if _, _, code := run(t, d.deps, "open", "ags-o1"); code != 0 || !slices.Equal(d.desktops, []string{"ags-o1"}) {
		t.Fatalf("running sandbox: exit %d desktops %v", code, d.desktops)
	}
	if _, _, code := run(t, d.deps, "open", "ags-o1", "--dashboard"); code != 0 || len(d.desktops) != 1 || len(d.opened) != 2 {
		t.Fatalf("--dashboard: desktops %v opened %v", d.desktops, d.opened)
	}
}

func TestListCancelAndGet(t *testing.T) {
	f := newFakeAPI(t)
	stdout, _, code := run(t, f.deps(t).deps, "list", "--state", "failed", "--state", "working", "--tag", "ci", "--limit", "5")
	if code != 0 || !strings.Contains(stdout, "ags-listed1") || !strings.Contains(stdout, "STATE") {
		t.Fatalf("list exit %d %q", code, stdout)
	}
	for _, want := range []string{"ownerId=tea-ws", "limit=5", "state=failed", "state=working", "tag=ci"} {
		if !strings.Contains(f.lastList, want) {
			t.Errorf("list query %q lacks %s", f.lastList, want)
		}
	}
	if stdout, _, code := run(t, f.deps(t).deps, "cancel", "ags-c1"); code != 0 || !strings.Contains(stdout, "canceled") || !slices.Equal(f.canceled, []string{"ags-c1"}) {
		t.Fatalf("cancel exit %d %q %v", code, stdout, f.canceled)
	}
	f.script["ags-g1"] = []string{stateSucceeded}
	stdout, _, code = run(t, f.deps(t).deps, "get", "ags-g1", "-o", "yaml")
	if code != 0 || !strings.Contains(stdout, "state: succeeded") {
		t.Fatalf("get yaml exit %d %q", code, stdout)
	}
	// get is a read: it never exits by state.
	f.script["ags-g2"] = []string{stateFailed}
	if _, _, code := run(t, f.deps(t).deps, "get", "ags-g2"); code != 0 {
		t.Fatalf("get of a failed session exit %d", code)
	}
}

// w32/011: -o yaml prints the numbers the API sent. An integer keeps its
// digits whatever its size; -o json is the API's bytes, re-indented.
func TestYAMLOutputKeepsIntegersExact(t *testing.T) {
	f := newFakeAPI(t)
	const answer = `{"id":"ags-n1","state":"succeeded","turns":3,"outcome":{"files":[{"name":"dump.sql","size":2097152}],` +
		`"result":{"rows":1500000,"issueId":9007199254740993,"huge":123456789012345678901234567890,"count":999999,"delta":-42,` +
		`"ratio":0.5,"thousand":1e3,"over":1e400,"version":"1500000","ok":true,"none":null}}}`
	f.raw["ags-n1"] = answer
	stdout, stderr, code := run(t, f.deps(t).deps, "get", "ags-n1", "-o", "yaml")
	if code != 0 {
		t.Fatalf("get -o yaml exit %d: %s", code, stderr)
	}
	for _, line := range []string{
		"size: 2097152\n", "rows: 1500000\n", "issueId: 9007199254740993\n", "huge: 123456789012345678901234567890\n",
		"count: 999999\n", "delta: -42\n", "turns: 3\n",
		// Not integers: a float stays a float (its literal, past float64's
		// range), and a string of digits a string.
		"ratio: 0.5\n", "thousand: 1000\n", "over: 1e400\n", "version: \"1500000\"\n", "ok: true\n", "none: null\n",
	} {
		if !strings.Contains(stdout, line) {
			t.Errorf("get -o yaml lacks %q:\n%s", line, stdout)
		}
	}
	// What a script reading the YAML gets back: the float it used to get
	// read as 9007199254740992.
	var read struct {
		Outcome struct {
			Result struct {
				IssueID int64 `yaml:"issueId"`
			}
		}
	}
	if err := yaml.Unmarshal([]byte(stdout), &read); err != nil || read.Outcome.Result.IssueID != 9007199254740993 {
		t.Fatalf("the YAML reads issueId back as %d (%v):\n%s", read.Outcome.Result.IssueID, err, stdout)
	}

	stdout, stderr, code = run(t, f.deps(t).deps, "get", "ags-n1", "-o", "json")
	var indented bytes.Buffer
	_ = json.Indent(&indented, []byte(answer), "", "  ")
	if code != 0 || stdout != indented.String()+"\n" {
		t.Fatalf("get -o json exit %d (%s) changed the API's answer:\n%s", code, stderr, stdout)
	}

	f.files["ags-n1"] = []attachmentFixture{{att: attachment{ID: "att-big", Turn: 1, Direction: "output", Name: "dump.sql", Size: 123456789}}}
	stdout, stderr, code = run(t, f.deps(t).deps, "files", "ags-n1", "-o", "yaml")
	if code != 0 || !strings.Contains(stdout, "size: 123456789\n") {
		t.Fatalf("files -o yaml exit %d: %q %s", code, stdout, stderr)
	}
}

func TestNoWorkspaceIsExplained(t *testing.T) {
	f := newFakeAPI(t)
	d := f.deps(t)
	d.workspace = func() (string, error) { return "", errNoWorkspaceForTest }
	if _, stderr, code := run(t, d.deps, "list"); code != 1 || !strings.Contains(stderr, "no-workspace") {
		t.Fatalf("exit %d %q", code, stderr)
	}
}

var errNoWorkspaceForTest = errors.New("no-workspace")

// A wait the user interrupts is not a --timeout: Ctrl-C or SIGTERM exits 130,
// the deadline exits 124 with where the session stood.
func TestWaitTellsAnInterruptFromATimeout(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-w1"] = []string{stateWorking}
	a, _ := f.deps(t).api()

	interrupted, stop := context.WithCancel(context.Background())
	stop()
	if _, err := a.wait(interrupted, []string{"ags-w1"}, false); exitCode(err) != exitInterrupted {
		t.Fatalf("canceled wait = %v (exit %d), want exit %d", err, exitCode(err), exitInterrupted)
	}
	if _, err := a.follow(interrupted, "ags-w1", 1, newPartPrinter(io.Discard, false, false)); exitCode(err) != exitInterrupted {
		t.Fatalf("canceled follow = %v, want exit %d", err, exitInterrupted)
	}

	deadline, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	result, err := a.wait(deadline, []string{"ags-w1"}, false)
	if err != nil || !result.timedOut || exitCode(stateExit(result, time.Second)) != exitTimeout {
		t.Fatalf("deadline wait = %+v, %v", result, err)
	}
}

// --any returns at the first settled session even while a sibling's read is
// failing, and its JSON is still one valid array of the sessions it read.
func TestWaitAnyPrintsValidJSONWhenASiblingReadFailed(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-a1"] = []string{stateSucceeded}
	f.script["ags-b1"] = []string{stateWorking}
	f.failGets["ags-b1"] = 3
	stdout, stderr, code := run(t, f.deps(t).deps, "wait", "--any", "-o", "json", "ags-a1", "ags-b1")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var sessions []map[string]any
	if err := json.Unmarshal([]byte(stdout), &sessions); err != nil || len(sessions) != 1 || sessions[0]["id"] != "ags-a1" {
		t.Fatalf("--any JSON = %q (%v)", stdout, err)
	}
}

// A 429 is retried with the same key after at least its Retry-After; a
// non-transient failure (here a bad server URL) is not retried at all.
func TestCreateHonorsRetryAfterAndRetriesOnlyTransientErrors(t *testing.T) {
	f := newFakeAPI(t)
	f.createErrs = []int{http.StatusTooManyRequests}
	f.retryAfter = "7"
	if _, stderr, code := run(t, f.deps(t).deps, "run", "task"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if len(f.creates) != 2 || f.creates[0].header.Get("Idempotency-Key") != f.creates[1].header.Get("Idempotency-Key") {
		t.Fatalf("creates = %d", len(f.creates))
	}
	if len(f.slept) != 1 || f.slept[0] < 7*time.Second {
		t.Fatalf("slept %v before the retry, want at least the 7s Retry-After", f.slept)
	}

	a, _ := f.deps(t).api()
	a.server = "http://[::1]:namedport/v1/"
	f.slept = nil
	if _, _, err := a.create(context.Background(), "tea-ws", createRequest{IdempotencyKey: "k"}); err == nil || len(f.slept) != 0 {
		t.Fatalf("a non-transient error was retried: %v, slept %v", err, f.slept)
	}
}

// --follow backs off while the agent is quiet instead of polling every 2s,
// and sleeps no longer than pollMax.
func TestFollowBacksOffWhileQuiet(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-q1"] = []string{stateWorking, stateWorking, stateWorking, stateWorking, stateWorking, stateWorking, stateWorking, stateSucceeded}
	a, _ := f.deps(t).api()
	result, err := a.follow(context.Background(), "ags-q1", 1, newPartPrinter(io.Discard, false, false))
	if err != nil || len(result.sessions) != 1 || result.sessions[0].State != stateSucceeded {
		t.Fatalf("follow = %+v, %v", result, err)
	}
	if len(f.slept) < 3 || f.slept[0] != pollFirst || f.slept[1] <= f.slept[0] {
		t.Fatalf("quiet follow slept %v, want a backoff from %s", f.slept, pollFirst)
	}
	for _, d := range f.slept {
		if d > pollMax {
			t.Fatalf("slept %s, over pollMax", d)
		}
	}
	if got := f.transcrQuery; !strings.Contains(got, "limit=1000") {
		t.Fatalf("transcript query %q does not ask for the largest page", got)
	}
}

// An expired signed URL (403) is re-read once for a fresh one; names that
// differ only in case do not collide on a case-insensitive disk; and a
// failure still lists, in -o json, the files that did land.
func TestDownloadRefreshesExpiredURLsAndReportsPartialResults(t *testing.T) {
	f := newFakeAPI(t)
	sum := func(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
	f.files["ags-x1"] = []attachmentFixture{
		{attachment{ID: "att-1", Turn: 1, Direction: "output", Name: "A.txt", SHA256: sum("one"), DownloadURL: f.storage.URL + "/expired/att-1"}, "one"},
		{attachment{ID: "att-2", Turn: 1, Direction: "output", Name: "a.txt", SHA256: sum("two"), DownloadURL: f.storage.URL + "/b/att-2"}, "two"},
		{attachment{ID: "att-3", Turn: 1, Direction: "output", Name: "c.txt", SHA256: sum("not this"), DownloadURL: f.storage.URL + "/b/att-3"}, "three"},
	}
	dir := t.TempDir()
	stdout, stderr, code := run(t, f.deps(t).deps, "files", "ags-x1", "--download", dir, "-o", "json")
	if code != 1 || !strings.Contains(stderr, "SHA-256") {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if !slices.Equal(f.refreshed, []string{"att-1"}) {
		t.Fatalf("refreshed %v, want only the expired att-1", f.refreshed)
	}
	var saved []savedFile
	if err := json.Unmarshal([]byte(stdout), &saved); err != nil || len(saved) != 2 {
		t.Fatalf("partial result = %q (%v)", stdout, err)
	}
	for name, want := range map[string]string{"A.txt": "one", "a-att-2.txt": "two"} {
		if got, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v", name, got, err)
		}
	}
}

// Without --force, a file that appears at the destination while the download
// runs is never replaced.
func TestPlaceNeverReplacesAFileThatAppeared(t *testing.T) {
	dir := t.TempDir()
	tmp, path := filepath.Join(dir, ".tmp"), filepath.Join(dir, "out.txt")
	for name, body := range map[string]string{tmp: "new", path: "theirs"} {
		if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := place(tmp, path, false); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("place over an existing file = %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "theirs" {
		t.Fatalf("the existing file was replaced: %q", got)
	}
	if err := place(tmp, path, true); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Fatalf("--force did not replace: %q", got)
	}
}

// An answer over the cap is refused, not decoded from a truncated body.
func TestOversizedAnswerIsRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte(" "), maxResponseBytes+1))
	}))
	t.Cleanup(srv.Close)
	a := &api{server: srv.URL + "/v1/", http: srv.Client()}
	if _, err := a.getJSON(context.Background(), nil, "agent-sessions"); !errors.Is(err, errResponseTooLarge) {
		t.Fatalf("oversized answer = %v", err)
	}
}

// A dropped connection (the client sees EOF inside *url.Error) is transient.
func TestADroppedConnectionIsTransient(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.Close()
	}))
	t.Cleanup(srv.Close)
	a := &api{server: srv.URL + "/v1/", http: srv.Client()}
	_, err := a.getJSON(context.Background(), nil, "agent-sessions", "ags-x")
	if err == nil || !transient(err) {
		t.Fatalf("dropped connection = %v, transient %v", err, transient(err))
	}
	if transient(errors.New("token refresh failed")) {
		t.Fatal("a plain error counted as transient")
	}
}

// The text view shows outcome.cost as one total with its parts (w9/m178), and
// falls back to modelUSD from a server that predates computeUSD.
func TestSessionTextShowsTheTotalCost(t *testing.T) {
	var out strings.Builder
	writeSession(&out, session{ID: "ags-c1", State: stateSucceeded, Outcome: &outcome{Turn: 1,
		Cost: &cost{ModelUSD: "0.42", ComputeUSD: "0.0895", TotalUSD: "0.5095"}}})
	if !strings.Contains(out.String(), "$0.5095 (model $0.42, compute $0.0895)") {
		t.Fatalf("text view:\n%s", out.String())
	}
	out.Reset()
	writeSession(&out, session{ID: "ags-c2", State: stateSucceeded, Outcome: &outcome{Turn: 1,
		Cost: &cost{ModelUSD: "0.42"}}})
	if !strings.Contains(out.String(), "$0.42") || strings.Contains(out.String(), "compute") {
		t.Fatalf("text view from an older server:\n%s", out.String())
	}
}
