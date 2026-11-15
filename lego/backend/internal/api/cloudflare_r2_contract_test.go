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

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// The wrangler contract (ADR112 D11). Cloudflare's tool is the client and its
// wire can change in any release, so two tests pin it:
//
//   - TestWranglerFixturesReplay replays the requests recorded from the real
//     wrangler 4.162.0 against the handlers, on every run, with no node.
//   - TestWranglerEndToEnd runs the real wrangler against the handlers over a
//     socket. It needs the binary (scripts/wrangler-r2-contract.sh installs the
//     pinned version and sets BEX_WRANGLER_BIN) and runs weekly in CI. With
//     BEX_WRANGLER_RECORD set it also rewrites the fixtures from the run.
const (
	wranglerVersion  = "4.162.0"
	wranglerFixtures = "testdata/wrangler-" + wranglerVersion + "/requests.json"
	accountToken     = "{account}"
)

// recordedRequest is one request wrangler sent, in the order it sent them. The
// account id is a placeholder so the fixture does not depend on a minted id.
type recordedRequest struct {
	Step    string            `json:"step"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	Status  int               `json:"status"`
}

// recordedHeaders are the headers of a request that matter to the contract;
// the rest (Host, User-Agent, Authorization, Accept) are the client's own.
var recordedHeaders = []string{
	"Content-Type", "Content-Length", "Cache-Control", "Content-Disposition", "Content-Encoding", "Content-Language", "Expires",
	"cf-r2-jurisdiction", "cf-r2-storage-class", "cf-r2-data-catalog-check",
}

// recorder wraps a handler and records every request and its status.
type recorder struct {
	next    http.Handler
	account string
	mu      sync.Mutex
	step    string
	out     []recordedRequest
}

func (r *recorder) setStep(s string) { r.mu.Lock(); r.step = s; r.mu.Unlock() }

func (r *recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	req.Body = io.NopCloser(bytes.NewReader(body))
	sw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	r.next.ServeHTTP(sw, req)
	rec := recordedRequest{Method: req.Method, Path: strings.ReplaceAll(req.URL.EscapedPath(), r.account, accountToken), Status: sw.status, Body: string(body), Headers: map[string]string{}}
	for _, name := range recordedHeaders {
		if v := req.Header.Get(name); v != "" {
			rec.Headers[name] = v
		}
	}
	if req.ContentLength >= 0 && len(body) > 0 {
		rec.Headers["Content-Length"] = fmt.Sprint(len(body))
	}
	r.mu.Lock()
	rec.Step = r.step
	r.out = append(r.out, rec)
	r.mu.Unlock()
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int)        { s.status = code; s.ResponseWriter.WriteHeader(code) }
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// wranglerStep is one wrangler command and what its output must show.
type wranglerStep struct {
	name    string
	args    []string
	want    []string // substrings the output must contain
	notWant []string
	fail    bool // the command is expected to exit non-zero
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// wranglerScript is every command of ADR112's contract, in an order where each
// one finds the state the one before left.
func wranglerScript(dir string) []wranglerStep {
	file := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			panic(err)
		}
		return p
	}
	corsFile := file("cors.json", wranglerCORS)
	lifeFile := file("lifecycle.json", wranglerLife)
	catFile := file("cat.txt", "meow meow\n")
	outFile := filepath.Join(dir, "cat.out")
	return []wranglerStep{
		{name: "bucket create", args: []string{"r2", "bucket", "create", "photos"}, want: []string{"Created bucket 'photos'"}},
		{name: "bucket create location", args: []string{"r2", "bucket", "create", "euro", "--location", "weur"}, want: []string{"Created bucket 'euro'"}},
		{name: "bucket create jurisdiction", args: []string{"r2", "bucket", "create", "eujur", "--jurisdiction", "eu"}, want: []string{"Created bucket 'eujur"}},
		{name: "bucket create storage class", args: []string{"r2", "bucket", "create", "standard", "--storage-class", "Standard"}, want: []string{"Created bucket 'standard'"}},
		{name: "bucket create unoffered location", args: []string{"r2", "bucket", "create", "east", "--location", "enam"}, fail: true, want: []string{"10042"}},
		{name: "bucket create infrequent access", args: []string{"r2", "bucket", "create", "cold", "--storage-class", "InfrequentAccess"}, fail: true, want: []string{"10042"}},
		{name: "bucket create twice", args: []string{"r2", "bucket", "create", "photos"}, fail: true, want: []string{"10073"}},
		{name: "bucket list", args: []string{"r2", "bucket", "list"}, want: []string{"photos", "euro", "eujur", "standard"}},
		{name: "bucket info", args: []string{"r2", "bucket", "info", "photos"}, want: []string{"photos", "object_count"}},
		{name: "object put", args: []string{"r2", "object", "put", "photos/dir/cat.txt", "--file", catFile, "--content-type", "text/plain", "--cache-control", "max-age=60", "--content-disposition", `attachment; filename="cat.txt"`, "--content-language", "en", "--remote"}, want: []string{"Upload complete"}},
		{name: "object put no options", args: []string{"r2", "object", "put", "photos/plain", "--file", catFile, "--remote"}, want: []string{"Upload complete"}},
		{name: "object get", args: []string{"r2", "object", "get", "photos/dir/cat.txt", "--file", outFile, "--remote"}, want: []string{"Download complete"}},
		{name: "object get missing", args: []string{"r2", "object", "get", "photos/ghost", "--file", filepath.Join(dir, "ghost.out"), "--remote"}, fail: true, want: []string{"does not exist"}},
		{name: "object delete", args: []string{"r2", "object", "delete", "photos/plain", "--remote"}, want: []string{"Delete complete"}},
		{name: "object delete missing", args: []string{"r2", "object", "delete", "photos/plain", "--remote"}, want: []string{"Delete complete"}},
		{name: "object into the system bucket", args: []string{"r2", "object", "put", "render-oregon/from-wrangler.txt", "--file", catFile, "--remote"}, want: []string{"Upload complete"}},
		{name: "cors list empty", args: []string{"r2", "bucket", "cors", "list", "photos"}, want: []string{"no CORS configuration"}},
		{name: "cors set", args: []string{"r2", "bucket", "cors", "set", "photos", "--file", corsFile, "--force"}, want: []string{"CORS configuration"}},
		{name: "cors list", args: []string{"r2", "bucket", "cors", "list", "photos"}, want: []string{"https://example.com", "GET"}},
		{name: "cors delete", args: []string{"r2", "bucket", "cors", "delete", "photos", "--force"}, want: []string{"CORS"}},
		{name: "lifecycle list empty", args: []string{"r2", "bucket", "lifecycle", "list", "photos"}, want: []string{"no lifecycle rules"}},
		{name: "lifecycle add expire days", args: []string{"r2", "bucket", "lifecycle", "add", "photos", "expire-logs", "logs/", "--expire-days", "7", "--force"}, want: []string{"expire-logs"}},
		{name: "lifecycle add expire date", args: []string{"r2", "bucket", "lifecycle", "add", "photos", "sunset", "", "--expire-date", "2027-01-01", "--force"}, want: []string{"sunset"}},
		{name: "lifecycle add abort multipart", args: []string{"r2", "bucket", "lifecycle", "add", "photos", "tidy", "up/", "--abort-multipart-days", "1", "--force"}, want: []string{"tidy"}},
		{name: "lifecycle add infrequent access", args: []string{"r2", "bucket", "lifecycle", "add", "photos", "cold", "old/", "--ia-transition-days", "30", "--force"}, fail: true, want: []string{"10042"}},
		{name: "lifecycle list", args: []string{"r2", "bucket", "lifecycle", "list", "photos"}, want: []string{"expire-logs", "sunset", "tidy"}, notWant: []string{"cold"}},
		{name: "lifecycle remove", args: []string{"r2", "bucket", "lifecycle", "remove", "photos", "--name", "sunset"}, want: []string{"sunset"}},
		{name: "lifecycle set", args: []string{"r2", "bucket", "lifecycle", "set", "photos", "--file", lifeFile, "--force"}, want: []string{"lifecycle"}},
		{name: "lifecycle list after set", args: []string{"r2", "bucket", "lifecycle", "list", "photos"}, want: []string{"expire-logs", "sunset", "tidy"}},
		{name: "domain list", args: []string{"r2", "bucket", "domain", "list", "photos"}, want: []string{"photos"}},
		{name: "domain add", args: []string{"r2", "bucket", "domain", "add", "photos", "--domain", "cdn.example.com", "--zone-id", "z1", "--force"}, fail: true, want: []string{"10042"}},
		{name: "domain update", args: []string{"r2", "bucket", "domain", "update", "photos", "--domain", "cdn.example.com", "--min-tls", "1.2"}, fail: true, want: []string{"10042"}},
		{name: "domain remove", args: []string{"r2", "bucket", "domain", "remove", "photos", "--domain", "cdn.example.com", "--force"}, fail: true, want: []string{"10042"}},
		{name: "dev-url get", args: []string{"r2", "bucket", "dev-url", "get", "photos"}, fail: true, want: []string{"10042"}},
		{name: "dev-url enable", args: []string{"r2", "bucket", "dev-url", "enable", "photos", "--force"}, fail: true, want: []string{"10042"}},
		{name: "dev-url disable", args: []string{"r2", "bucket", "dev-url", "disable", "photos", "--force"}, fail: true, want: []string{"10042"}},
		{name: "lock list", args: []string{"r2", "bucket", "lock", "list", "photos"}, fail: true, want: []string{"10042"}},
		{name: "sippy get", args: []string{"r2", "bucket", "sippy", "get", "photos"}, fail: true, want: []string{"10042"}},
		{name: "notification list", args: []string{"r2", "bucket", "notification", "list", "photos"}, fail: true, want: []string{"10042"}},
		{name: "bucket delete not empty", args: []string{"r2", "bucket", "delete", "photos"}, fail: true, want: []string{"10008"}},
		{name: "object delete last", args: []string{"r2", "object", "delete", "photos/dir/cat.txt", "--remote"}, want: []string{"Delete complete"}},
		{name: "bucket delete", args: []string{"r2", "bucket", "delete", "photos"}, want: []string{"photos"}},
		{name: "bucket delete missing", args: []string{"r2", "bucket", "delete", "photos"}, fail: true, want: []string{"10006"}},
		{name: "bucket list after", args: []string{"r2", "bucket", "list"}, want: []string{"euro"}, notWant: []string{"photos"}},
	}
}

func TestWranglerEndToEnd(t *testing.T) {
	bin := os.Getenv("BEX_WRANGLER_BIN")
	if bin == "" {
		t.Skip("BEX_WRANGLER_BIN is not set; scripts/wrangler-r2-contract.sh installs wrangler " + wranglerVersion + " and sets it")
	}
	// The fixtures are wrangler's wire at one version; a binary at another is a
	// new measurement, so the pin in the script and in this file must agree.
	if want := os.Getenv("BEX_WRANGLER_VERSION"); want != "" && want != wranglerVersion {
		t.Fatalf("scripts/wrangler-r2-contract.sh installs wrangler %s, but the fixtures are for %s", want, wranglerVersion)
	}
	if out, err := exec.Command(bin, "--version").Output(); err != nil || !strings.Contains(string(out), wranglerVersion) {
		t.Fatalf("%s --version = %q, %v; the contract is pinned to wrangler %s", bin, out, err, wranglerVersion)
	}
	h := newCFHarness(t)
	rec := &recorder{next: withGzip(h.h), account: h.ws}
	ts := httptest.NewServer(rec)
	t.Cleanup(ts.Close)

	dir := t.TempDir()
	env := append(os.Environ(),
		"CLOUDFLARE_API_BASE_URL="+ts.URL+cloudflareMount,
		"CLOUDFLARE_API_TOKEN=bex-contract-test-token",
		"CLOUDFLARE_ACCOUNT_ID="+h.ws,
		"HOME="+filepath.Join(dir, "home"), "XDG_CONFIG_HOME="+filepath.Join(dir, "xdg"),
		"CI=1", "NO_COLOR=1", "WRANGLER_SEND_METRICS=false", "WRANGLER_LOG=log", "NO_UPDATE_NOTIFIER=1",
	)
	for _, step := range wranglerScript(dir) {
		rec.setStep(step.name)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		cmd := exec.CommandContext(ctx, bin, step.args...)
		cmd.Dir, cmd.Env = dir, env
		raw, err := cmd.CombinedOutput()
		cancel()
		out := ansi.ReplaceAllString(string(raw), "")
		if (err != nil) != step.fail {
			t.Errorf("%s: exit error %v, want failure=%v\n%s", step.name, err, step.fail, out)
			continue
		}
		for _, want := range step.want {
			if !strings.Contains(out, want) {
				t.Errorf("%s: output lacks %q:\n%s", step.name, want, out)
			}
		}
		for _, notWant := range step.notWant {
			if strings.Contains(out, notWant) {
				t.Errorf("%s: output has %q:\n%s", step.name, notWant, out)
			}
		}
	}

	// What came back is what went in.
	if got, err := os.ReadFile(filepath.Join(dir, "cat.out")); err != nil || string(got) != "meow meow\n" {
		t.Errorf("object get wrote %q, %v", got, err)
	}
	// An object put with wrangler into render-oregon is in Render's namespace.
	if page, err := h.svc.ListObjects(t.Context(), h.ws, "oregon", "", 100); err != nil || len(page.Items) != 1 || page.Items[0].Key != "from-wrangler.txt" {
		t.Errorf("render-oregon through Render's list = %+v, %v", page, err)
	}

	if dest := os.Getenv("BEX_WRANGLER_RECORD"); dest != "" {
		raw, _ := json.MarshalIndent(rec.out, "", "  ")
		if err := os.WriteFile(dest, append(raw, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("recorded %d requests to %s", len(rec.out), dest)
	}
}

// TestWranglerFixturesReplay replays what the real wrangler sent, in order,
// against the handlers and checks each answer's status and shape. It is the
// part of the contract that runs on every build.
func TestWranglerFixturesReplay(t *testing.T) {
	raw, err := os.ReadFile(wranglerFixtures)
	if err != nil {
		t.Fatalf("the recorded wrangler %s requests are missing (BEX_WRANGLER_RECORD rewrites them): %v", wranglerVersion, err)
	}
	var requests []recordedRequest
	if err := json.Unmarshal(raw, &requests); err != nil {
		t.Fatal(err)
	}
	if len(requests) < 40 {
		t.Fatalf("only %d recorded requests; the fixture looks truncated", len(requests))
	}
	h := newCFHarness(t)
	steps := map[string]bool{}
	for i, rq := range requests {
		steps[rq.Step] = true
		path := strings.ReplaceAll(rq.Path, accountToken, h.ws)
		req := httptest.NewRequest(rq.Method, path, strings.NewReader(rq.Body))
		for name, value := range rq.Headers {
			req.Header.Set(name, value)
		}
		if rq.Body == "" {
			req.ContentLength = 0
		}
		rec := httptest.NewRecorder()
		h.h.ServeHTTP(rec, req)
		where := fmt.Sprintf("request %d (%s): %s %s", i+1, rq.Step, rq.Method, rq.Path)
		if rec.Code != rq.Status {
			t.Errorf("%s answered %d, wrangler 4.162.0 got %d on record: %s", where, rec.Code, rq.Status, rec.Body)
			continue
		}
		isObjectGet := rq.Method == http.MethodGet && strings.Contains(rq.Path, "/objects/") && rec.Code == http.StatusOK
		if isObjectGet {
			if rec.Body.String() != "meow meow\n" {
				t.Errorf("%s returned %q, want the bytes put", where, rec.Body)
			}
			continue
		}
		var env struct {
			Success  *bool           `json:"success"`
			Errors   []cfErrorEntry  `json:"errors"`
			Messages []string        `json:"messages"`
			Result   json.RawMessage `json:"result"`
		}
		if rec.Body.Len() == 0 || json.Unmarshal(rec.Body.Bytes(), &env) != nil {
			t.Errorf("%s answered %d with %q, which is not JSON: wrangler fails on a bodiless or malformed answer", where, rec.Code, rec.Body)
			continue
		}
		if strings.HasSuffix(rq.Path, "/graphql") {
			continue // GraphQL's own shape, checked by TestCloudflareBucketInfoShowsTheLastSweep
		}
		if env.Success == nil || *env.Success != (rec.Code < 300) || env.Errors == nil || env.Messages == nil {
			t.Errorf("%s answered %s, want the full envelope", where, rec.Body)
		}
		if rec.Code >= 300 && (len(env.Errors) != 1 || env.Errors[0].Code == 0 || env.Errors[0].Message == "") {
			t.Errorf("%s: an error needs a code and a message: %s", where, rec.Body)
		}
		if rec.Code < 300 && (len(env.Result) == 0 || string(env.Result) == "null") {
			t.Errorf("%s: a success needs a result: %s", where, rec.Body)
		}
	}

	// The fixture covers every command ADR112 D11 names, so one that stops being
	// recorded cannot silently stop being tested.
	for _, step := range wranglerScript(t.TempDir()) {
		if !steps[step.name] {
			t.Errorf("the fixture has no request for the step %q: record it again", step.name)
		}
	}
	// What the real client sends: an object put always carries its length and
	// is never chunked, and the system bucket was written through the same routes.
	for _, rq := range requests {
		if rq.Method == http.MethodPut && strings.Contains(rq.Path, "/objects/") {
			if rq.Headers["Content-Length"] != fmt.Sprint(len(rq.Body)) {
				t.Errorf("recorded put %s has Content-Length %q for %d bytes", rq.Path, rq.Headers["Content-Length"], len(rq.Body))
			}
		}
	}
	if page, err := h.svc.ListObjects(t.Context(), h.ws, "oregon", "", 100); err != nil || len(page.Items) != 1 {
		t.Errorf("render-oregon after the replay = %+v, %v", page, err)
	}
}
