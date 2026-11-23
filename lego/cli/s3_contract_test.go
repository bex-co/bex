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

package main_test

// `bex s3` against the built binary (ADR112 D3, ADR113 D7): a fake bex-api that
// mints keys, a fake S3 endpoint, and a PATH with nothing on it. It proves the
// commands run with no other program installed, report themselves once each
// through the ordinary path, keep the key owner-only in bex's own state
// directory, and never read or write a person's own credential files.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	s3Workspace = "tea-bex"
	s3KeyID     = "s3k-d0abc123def456ghij78"
	s3Secret    = "fake-secret-of-the-launch-contract"
	// theirSecret is a credential of the person's own setup, which must never be
	// read, sent or written.
	theirSecret = "THEIR-OWN-AWS-SECRET"
)

// s3MintRoutes is the key-mint answer bex-api gives: an ephemeral key good for
// twelve hours from the real clock.
func s3MintRoutes() map[string]string {
	expires := time.Now().Add(12 * time.Hour).UTC().Format(time.RFC3339)
	return map[string]string{
		"/v1/object-storage/" + s3Workspace + "/access-keys": `{"id":"` + s3KeyID + `","name":"bex s3","permission":"readwrite",` +
			`"createdBy":"usr-1","createdAt":"2026-11-20T10:00:00Z","expiresAt":"` + expires + `","lastUsedAt":null,"status":"active","secret":"` + s3Secret + `"}`,
	}
}

// fakeS3Endpoint is a small S3 over plain HTTP on the loopback address (the one
// kind of endpoint other than HTTPS bex accepts), answering the calls the
// commands below make and recording who signed them.
type fakeS3Endpoint struct {
	srv *httptest.Server
	mu  sync.Mutex
	// auth is the Authorization header of every request.
	auth []string
	// calls is "METHOD /path" of every request.
	calls []string
}

func newFakeS3Endpoint(t *testing.T) *fakeS3Endpoint {
	t.Helper()
	f := &fakeS3Endpoint{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		f.calls = append(f.calls, r.Method+" "+r.URL.Path)
		f.mu.Unlock()
		_, _ = io.Copy(io.Discard, r.Body) // the request's payload is not what these tests are about
		w.Header().Set("Content-Type", "application/xml")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/":
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><ListAllMyBucketsResult><Owner><ID>o</ID></Owner><Buckets>` +
				`<Bucket><Name>launch-contract-bucket</Name><CreationDate>2026-11-19T08:00:00.000Z</CreationDate></Bucket></Buckets></ListAllMyBucketsResult>`))
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult><Name>launch-contract-bucket</Name><KeyCount>0</KeyCount><MaxKeys>1000</MaxKeys><IsTruncated>false</IsTruncated></ListBucketResult>`))
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("ETag", `"d41d8cd98f00b204e9800998ecf8427e"`)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeS3Endpoint) seen() (auth, calls []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.auth...), append([]string(nil), f.calls...)
}

// s3Env is the environment of a `bex s3` run: no program on the PATH at all
// (the case that failed on a machine with nothing else installed), the active
// workspace and the endpoint of the fake S3.
func s3Env(t *testing.T, home, apiURL, endpoint string, extra ...string) []string {
	t.Helper()
	return telemetryEnv(t, home, apiURL, append([]string{
		"PATH=" + t.TempDir(), "BEX_NO_UPDATE_NOTIFIER=1", "BEX_WORKSPACE=" + s3Workspace, "BEX_S3_ENDPOINT=" + endpoint,
	}, extra...)...)
}

// s3QuietEnv is s3Env for a test that does not look at what was reported: it
// opts out, so no detached sender outlives the command into the test's cleanup.
func s3QuietEnv(t *testing.T, home, apiURL, endpoint string, extra ...string) []string {
	t.Helper()
	return s3Env(t, home, apiURL, endpoint, append([]string{"BEX_CLI_DISABLE_ANALYTICS=1"}, extra...)...)
}

// s3LaunchEnv is s3Env for a test that does not look at the fake S3 itself.
func s3LaunchEnv(t *testing.T) []string {
	t.Helper()
	return []string{"PATH=" + t.TempDir(), "BEX_WORKSPACE=" + s3Workspace, "BEX_S3_ENDPOINT=" + newFakeS3Endpoint(t).srv.URL}
}

// TestBexS3ReportsEachCommandOnceThroughTheOrdinaryPath: every `bex s3` command
// returns through Cobra, so upstream's hook reports it, once, named by its
// words, with none of the buckets, keys, files or key ids typed after them.
func TestBexS3ReportsEachCommandOnceThroughTheOrdinaryPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "launch-contract-file"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(dir, "launch-contract-tree")
	if err := os.MkdirAll(tree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "leaf"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}
	const bucket = "s3://launch-contract-bucket"
	for _, run := range []struct {
		args    []string
		command string
		routes  map[string]string
	}{
		{[]string{"s3", "ls"}, "bex s3 ls", s3MintRoutes()},
		{[]string{"s3", "ls", bucket + "/"}, "bex s3 ls", s3MintRoutes()},
		{[]string{"s3", "cp", filepath.Join(dir, "launch-contract-file"), bucket + "/key"}, "bex s3 cp", s3MintRoutes()},
		{[]string{"s3", "sync", tree, bucket + "/p"}, "bex s3 sync", s3MintRoutes()},
		{[]string{"s3", "rm", bucket + "/key"}, "bex s3 rm", s3MintRoutes()},
		{[]string{"s3", "mb", bucket}, "bex s3 mb", s3MintRoutes()},
		{[]string{"s3", "presign", bucket + "/key"}, "bex s3 presign", s3MintRoutes()},
		{[]string{"s3", "env"}, "bex s3 env", s3MintRoutes()},
		{[]string{"s3", "keys", "list"}, "bex s3 keys list", map[string]string{"/v1/object-storage/" + s3Workspace + "/access-keys": `[]`}},
		{[]string{"s3", "info", "photos"}, "bex s3 info", map[string]string{"/v1/object-storage/" + s3Workspace + "/buckets/photos": `{"id":"obk-1","name":"photos","region":"oregon","public":false,"objects":0,"bytes":0,"observedAt":null,"createdAt":"2026-11-20T10:00:00Z"}`}},
	} {
		t.Run(strings.Join(run.args, " "), func(t *testing.T) {
			t.Parallel()
			stub := &telemetryStub{routes: run.routes}
			api := httptest.NewServer(stub.handler())
			t.Cleanup(api.Close)
			endpoint := newFakeS3Endpoint(t)
			home := t.TempDir()
			seedNoticeMarker(t, home)

			command := exec.Command(buildBex(), run.args...)
			command.Env = s3Env(t, home, api.URL, endpoint.srv.URL)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("bex %v: %v\n%s", run.args, err, output)
			}
			assertOneLaunchEvent(t, stub, run.command, "launch-contract-bucket", "launch-contract-file", "launch-contract-tree", s3KeyID, s3Secret)
		})
	}
}

// TestBexS3RunsWithNoOtherProgramInstalled is the case that failed on 2026-11-20:
// a machine with nothing on its PATH. The commands are bex's own, so they run,
// and the one a person may try with a tool in the way never starts it.
func TestBexS3RunsWithNoOtherProgramInstalled(t *testing.T) {
	t.Parallel()
	stub := &telemetryStub{routes: s3MintRoutes()}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	endpoint := newFakeS3Endpoint(t)
	home := t.TempDir()

	// A program named aws that records being run, on a PATH that is only that.
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "ran")
	script := "#!/bin/sh\necho ran > " + marker + "\n"
	for _, name := range []string{"aws", "rclone", "s5cmd"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command(buildBex(), "s3", "ls")
	command.Env = append(s3QuietEnv(t, home, api.URL, endpoint.srv.URL), "PATH="+bin)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("bex s3 ls: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "launch-contract-bucket") {
		t.Errorf("bex s3 ls printed %q, want the bucket", output)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("bex s3 started another program")
	}
	_, calls := endpoint.seen()
	if len(calls) != 1 || calls[0] != "GET /" {
		t.Errorf("the endpoint saw %v, want one ListBuckets", calls)
	}
}

// TestBexS3SignsWithBexsKeyAndTouchesNoAWSFile: whatever credential files and
// variables the person has, the requests are signed with the key bex made, which
// is kept owner-only in bex's state directory, and nothing is read from or
// written into ~/.aws.
func TestBexS3SignsWithBexsKeyAndTouchesNoAWSFile(t *testing.T) {
	t.Parallel()
	stub := &telemetryStub{routes: s3MintRoutes()}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	endpoint := newFakeS3Endpoint(t)
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".aws"), 0o755); err != nil {
		t.Fatal(err)
	}
	theirs := "[default]\naws_access_key_id = AKIATHEIRS\naws_secret_access_key = " + theirSecret + "\nendpoint_url = https://elsewhere.invalid\n"
	for _, name := range []string{"config", "credentials"} {
		if err := os.WriteFile(filepath.Join(home, ".aws", name), []byte(theirs), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	command := exec.Command(buildBex(), "s3", "ls")
	command.Env = s3QuietEnv(t, home, api.URL, endpoint.srv.URL,
		"AWS_PROFILE=personal", "AWS_SESSION_TOKEN=their-token", "AWS_ACCESS_KEY_ID=AKIATHEIRS", "AWS_SECRET_ACCESS_KEY="+theirSecret,
		"AWS_ENDPOINT_URL_S3=https://elsewhere.invalid", "AWS_REGION=eu-west-1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("bex s3 ls: %v\n%s", err, output)
	}

	auth, _ := endpoint.seen()
	if len(auth) == 0 {
		t.Fatal("the endpoint saw no request")
	}
	for _, header := range auth {
		if !strings.Contains(header, "Credential="+s3KeyID+"/") || strings.Contains(header, "AKIATHEIRS") {
			t.Errorf("a request was signed %q, want bex's key and not the person's", header)
		}
	}
	for _, name := range []string{"config", "credentials"} {
		if got, _ := os.ReadFile(filepath.Join(home, ".aws", name)); string(got) != theirs {
			t.Errorf("~/.aws/%s was changed:\n%s", name, got)
		}
	}
	state := filepath.Join(home, ".bex", "state", "s3")
	cached, _ := filepath.Glob(filepath.Join(state, "key-*.json"))
	if len(cached) != 1 {
		t.Fatalf("cached keys = %v, want one", cached)
	}
	if info, _ := os.Stat(cached[0]); info.Mode().Perm() != 0o600 {
		t.Errorf("cached key mode = %v, want 0600", info.Mode().Perm())
	}
	if entries, _ := os.ReadDir(state); len(entries) != 2 { // the key and its lock
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("bex's state directory holds %v, want only the key and its lock", names)
	}
}

// TestBexS3APIIsGone: bex has no `s3api` command, at the root or below `s3`,
// and a word `bex s3` does not define is an unknown command.
func TestBexS3APIIsGone(t *testing.T) {
	t.Parallel()
	stub := &telemetryStub{routes: s3MintRoutes()}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	home := t.TempDir()
	for _, args := range [][]string{{"s3api", "list-buckets"}, {"s3", "s3api", "list-buckets"}, {"s3", "nonsense"}} {
		command := exec.Command(buildBex(), args...)
		command.Env = s3QuietEnv(t, home, api.URL, "http://127.0.0.1:1")
		output, err := command.CombinedOutput()
		if err == nil || !strings.Contains(string(output), "unknown command") {
			t.Errorf("bex %v: err %v, output %q, want unknown command", args, err, output)
		}
	}
	command := exec.Command(buildBex(), "s3", "nonsense")
	command.Env = s3QuietEnv(t, home, api.URL, "http://127.0.0.1:1")
	if output, _ := command.CombinedOutput(); !strings.Contains(string(output), "bex s3 env") {
		t.Errorf("an unknown s3 command does not point at bex s3 env: %q", output)
	}
}
