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

package s3

import (
	"bytes"
	"encoding/json"
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
)

const (
	testWorkspace = "tea-bexcanary"
	testKeyID     = "s3k-d0abc123def456ghij78"
	testSecret    = "fake-secret-shown-once"
)

var testNow = time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

// recorded is one request the fake bex-api saw.
type recorded struct {
	method, path string
	body         map[string]any
}

// fakeAPI is bex-api's object-storage routes, answering what a test scripts
// and recording every request.
type fakeAPI struct {
	srv *httptest.Server
	mu  sync.Mutex
	got []recorded
	// expiry is what a minted key's expiresAt says.
	expiry func() time.Time
	// status overrides the answer of a route ("METHOD path") with a status and
	// body.
	status map[string]answer
	// delay holds every mint, so concurrent launches overlap.
	delay time.Duration
	mints int
}

// answer is a scripted reply.
type answer struct {
	code int
	body string
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{expiry: func() time.Time { return testNow.Add(keyLife) }}
	f.status = map[string]answer{}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	f.mu.Lock()
	f.got = append(f.got, recorded{r.Method, r.URL.EscapedPath(), body})
	override, overridden := f.status[r.Method+" "+r.URL.EscapedPath()]
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if overridden {
		w.WriteHeader(override.code)
		_, _ = w.Write([]byte(override.body))
		return
	}
	prefix := "/v1/object-storage/" + testWorkspace
	switch {
	case r.Method == http.MethodPost && r.URL.Path == prefix+"/access-keys":
		time.Sleep(f.delay)
		f.mu.Lock()
		f.mints++
		f.mu.Unlock()
		expires := "null"
		if secs, _ := body["expiresInSeconds"].(float64); secs > 0 {
			expires = `"` + f.expiry().UTC().Format(time.RFC3339) + `"`
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"id":%q,"name":%q,"permission":%q,"createdBy":"usr-1","createdAt":"2026-11-20T10:00:00Z","expiresAt":%s,"lastUsedAt":null,"status":"active","secret":%q}`,
			testKeyID, body["name"], body["permission"], expires, testSecret)
	case r.Method == http.MethodGet && r.URL.Path == prefix+"/access-keys":
		_, _ = w.Write([]byte(`[{"id":"` + testKeyID + `","name":"ci","permission":"readwrite","createdBy":"usr-1","createdAt":"2026-11-20T10:00:00Z","expiresAt":null,"lastUsedAt":"2026-11-20T11:30:00Z","status":"active"},` +
			`{"id":"s3k-zzzzzzzzzzzzzzzzzzzz","name":"bex s3 on laptop","permission":"readwrite","createdBy":"usr-1","createdAt":"2026-11-20T09:00:00Z","expiresAt":"2026-11-20T21:00:00Z","lastUsedAt":null,"status":"expired"}]`))
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, prefix+"/access-keys/"):
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && r.URL.Path == prefix+"/buckets/photos":
		_, _ = w.Write([]byte(`{"id":"obk-1","name":"photos","region":"frankfurt","system":false,"public":true,"publicHost":"pub-abc.onbex.co","objects":12,"bytes":1468006,"observedAt":"2026-11-20T09:00:00Z","createdAt":"2026-11-19T08:00:00Z"}`))
	case r.Method == http.MethodGet && r.URL.Path == prefix+"/buckets/photos/public":
		_, _ = w.Write([]byte(`{"public":true,"host":"pub-abc.onbex.co"}`))
	case r.Method == http.MethodPut && r.URL.Path == prefix+"/buckets/photos/public":
		_, _ = fmt.Fprintf(w, `{"public":%v,"host":"pub-abc.onbex.co"}`, body["enabled"])
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"BUCKET_NOT_FOUND","message":"bucket not found"}`))
	}
}

func (f *fakeAPI) requests() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.got)
}

func (f *fakeAPI) mintCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.mints
}

// testDeps wires deps to the fake bex-api, a temporary state directory and, when
// there is one, a fake S3 endpoint.
func testDeps(t *testing.T, f *fakeAPI) (deps, string) {
	t.Helper()
	state := t.TempDir()
	d := deps{
		api: func() (*api, error) {
			return &api{server: f.srv.URL + "/v1/", http: f.srv.Client()}, nil
		},
		workspace:  func() (string, error) { return testWorkspace, nil },
		host:       func() string { return "https://api.bex.co/v1/" },
		lookupEnv:  func(string) (string, bool) { return "", false },
		state:      func() (string, error) { return state, nil },
		now:        func() time.Time { return testNow },
		hostname:   func() string { return "laptop" },
		stdin:      strings.NewReader(""),
		isTerminal: func(io.Writer) bool { return false },
	}
	return d, state
}

// bex executes args under a root that has bex's output flag, returning what
// the command printed.
func bex(t *testing.T, d deps, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := &cobra.Command{Use: "bex", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().StringP("output", "o", "text", "")
	root.AddCommand(newCommands(d)...)
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errOut.String(), err
}

func TestKeyIsCachedPrivatelyAndReusedUntilAnHourIsLeft(t *testing.T) {
	f := newFakeAPI(t)
	d, state := testDeps(t, f)

	launch := func() {
		t.Helper()
		if _, _, err := bex(t, d, "s3", "env"); err != nil {
			t.Fatal(err)
		}
	}
	launch()
	if f.mintCount() != 1 {
		t.Fatalf("mints = %d after the first run, want 1", f.mintCount())
	}
	files, _ := filepath.Glob(filepath.Join(state, "s3", "key-*.json"))
	if len(files) != 1 {
		t.Fatalf("cache files = %v, want one", files)
	}
	info, _ := os.Stat(files[0])
	if info.Mode().Perm() != 0o600 {
		t.Errorf("key cache mode = %v, want 0600", info.Mode().Perm())
	}
	dirInfo, _ := os.Stat(filepath.Join(state, "s3"))
	if dirInfo.Mode().Perm() != 0o700 {
		t.Errorf("state directory mode = %v, want 0700", dirInfo.Mode().Perm())
	}

	// Eleven hours later, an hour and a minute is left: the key is reused.
	d.now = func() time.Time { return testNow.Add(11*time.Hour - time.Minute) }
	launch()
	if f.mintCount() != 1 {
		t.Errorf("mints = %d with 1h01m left, want the cached key reused", f.mintCount())
	}

	// Under an hour left: a new key.
	d.now = func() time.Time { return testNow.Add(11*time.Hour + time.Minute) }
	f.expiry = func() time.Time { return testNow.Add(11*time.Hour + time.Minute + keyLife) }
	launch()
	if f.mintCount() != 2 {
		t.Errorf("mints = %d with 59m left, want a replacement", f.mintCount())
	}

	// An expired key is replaced too, and a key of another workspace or control plane is never reused.
	d.now = func() time.Time { return testNow.Add(40 * time.Hour) }
	f.expiry = func() time.Time { return testNow.Add(40*time.Hour + keyLife) }
	launch()
	if f.mintCount() != 3 {
		t.Errorf("mints = %d after expiry, want 3", f.mintCount())
	}
	other := d
	other.host = func() string { return "https://api.other.example/v1/" }
	other.lookupEnv = func(name string) (string, bool) {
		if name == endpointEnv {
			return "https://objects.other.example", true
		}
		return "", false
	}
	if _, _, err := bex(t, other, "s3", "env"); err != nil {
		t.Fatal(err)
	}
	if f.mintCount() != 4 {
		t.Errorf("mints = %d for another control plane, want its own key", f.mintCount())
	}
}

// TestConcurrentFirstLaunchesMintOneKey: `xargs -P 16 bex s3 cp …` on a machine with no key
// must not make sixteen keys, which would pass the limit of twenty live ephemeral ones.
func TestConcurrentFirstLaunchesMintOneKey(t *testing.T) {
	f := newFakeAPI(t)
	f.delay = 50 * time.Millisecond
	d, _ := testDeps(t, f)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := d.session(t.Context(), "")
			if err == nil && s.AccessKeyID != testKeyID {
				err = fmt.Errorf("key %q", s.AccessKeyID)
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	if f.mintCount() != 1 {
		t.Errorf("mints = %d for 16 concurrent first launches, want 1", f.mintCount())
	}
}

func TestDamagedKeyCacheIsReplacedNotTrusted(t *testing.T) {
	f := newFakeAPI(t)
	d, state := testDeps(t, f)
	dir := filepath.Join(state, "s3")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile(dir, d.host(), testWorkspace), []byte(`{"accessKeyId":"s3k-x","secret":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := bex(t, d, "s3", "env"); err != nil {
		t.Fatal(err)
	}
	if f.mintCount() != 1 {
		t.Errorf("mints = %d, want a new key for a damaged cache", f.mintCount())
	}
}

// TestCachedKeyOfAnotherWorkspaceOrControlPlaneIsNotReused: the cache file is
// named by workspace and control plane, and a record inside it for another one
// (a file copied between machines or renamed) is refused too.
func TestCachedKeyOfAnotherWorkspaceOrControlPlaneIsNotReused(t *testing.T) {
	for _, stale := range []cachedKey{
		{Host: "https://api.bex.co/v1/", Workspace: "tea-someoneelse"},
		{Host: "https://api.other.example/v1/", Workspace: testWorkspace},
	} {
		f := newFakeAPI(t)
		d, state := testDeps(t, f)
		dir := filepath.Join(state, "s3")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		stale.AccessKeyID, stale.Secret, stale.ExpiresAt = "s3k-stale", "stale-secret", testNow.Add(10*time.Hour)
		if err := saveKey(keyFile(dir, d.host(), testWorkspace), stale); err != nil {
			t.Fatal(err)
		}
		if _, _, err := bex(t, d, "s3", "env"); err != nil {
			t.Fatal(err)
		}
		if f.mintCount() != 1 {
			t.Errorf("a cached key for %s %s was reused (mints = %d)", stale.Host, stale.Workspace, f.mintCount())
		}
	}
}

func TestEndpointOfAControlPlaneIsKnownOrSaid(t *testing.T) {
	f := newFakeAPI(t)
	d, _ := testDeps(t, f)
	d.host = func() string { return "http://localhost:8090/v1/" }
	_, _, err := bex(t, d, "s3", "env")
	if err == nil || !strings.Contains(err.Error(), endpointEnv) {
		t.Fatalf("err = %v, want it to name %s for a control plane with no known endpoint", err, endpointEnv)
	}
	for _, tc := range []struct {
		value string
		ok    bool
	}{
		{"https://objects.example.com", true},
		{"https://objects.example.com/", true},
		{"http://localhost:9000", true},
		{"http://objects.example.com", false}, // a key is never sent in clear over a network
		{"https://objects.example.com/path", false},
		{"https://user:pass@objects.example.com", false},
		{"objects.example.com", false},
	} {
		got, err := normalizeEndpoint(tc.value)
		if (err == nil) != tc.ok {
			t.Errorf("normalizeEndpoint(%q) = %q, %v; ok want %v", tc.value, got, err, tc.ok)
		}
	}
}

func TestRefusedMintSaysWhatItNeeds(t *testing.T) {
	for _, tc := range []struct {
		code int
		body string
		want string
	}{
		{http.StatusForbidden, `{"code":"FORBIDDEN","message":"forbidden"}`, "bex login"},
		{http.StatusNotFound, `{"code":"NOT_FOUND","message":"object storage is not offered"}`, "not available on this control plane"},
		{http.StatusConflict, `{"code":"ACCESS_KEY_LIMIT","message":"a person holds at most 20 live ephemeral access keys"}`, "at most 20"},
	} {
		f := newFakeAPI(t)
		f.status["POST /v1/object-storage/"+testWorkspace+"/access-keys"] = struct {
			code int
			body string
		}{tc.code, tc.body}
		d, _ := testDeps(t, f)
		_, _, err := bex(t, d, "s3", "env")
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("status %d: err = %v, want it to say %q", tc.code, err, tc.want)
		}
	}
}

func TestEnvPrintsTheSettingsForOtherTools(t *testing.T) {
	f := newFakeAPI(t)
	d, _ := testDeps(t, f)
	stdout, _, err := bex(t, d, "s3", "env")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"export AWS_ACCESS_KEY_ID='" + testKeyID + "'",
		"export AWS_SECRET_ACCESS_KEY='" + testSecret + "'",
		"export AWS_ENDPOINT_URL_S3='https://objects.onbex.co'",
		"export S3_ENDPOINT_URL='https://objects.onbex.co'",
		"export RCLONE_S3_PROVIDER='Other'",
		"export RCLONE_S3_ENDPOINT='https://objects.onbex.co'",
		"2026-11-20T22:00:00Z", // when the key expires
		"s3_use_path_style = true",
		"addressing_style",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("env output lacks %q:\n%s", want, stdout)
		}
	}

	stdout, _, err = bex(t, d, "s3", "env", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var got envOutput
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("env -o json is not JSON: %v\n%s", err, stdout)
	}
	if got.AccessKeyID != testKeyID || got.Secret != testSecret || got.Endpoint != "https://objects.onbex.co" ||
		got.Variables["RCLONE_S3_SECRET_ACCESS_KEY"] != testSecret || got.Workspace != testWorkspace {
		t.Errorf("env json = %+v", got)
	}
	if f.mintCount() != 1 {
		t.Errorf("mints = %d across two env runs, want the cached key reused", f.mintCount())
	}
}

func TestShellQuoteKeepsAValueOneWord(t *testing.T) {
	for in, want := range map[string]string{"plain": "'plain'", "it's": `'it'\''s'`, "a b$c": "'a b$c'"} {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestKeysCreatePrintsTheSecretOnceAndNeverListsIt(t *testing.T) {
	f := newFakeAPI(t)
	d, _ := testDeps(t, f)

	stdout, _, err := bex(t, d, "s3", "keys", "create", "--name", "ci")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(stdout, testSecret) != 1 || !strings.Contains(stdout, testKeyID) || !strings.Contains(stdout, "persistent") {
		t.Errorf("create printed:\n%s\nwant the id, the secret exactly once and a persistent key", stdout)
	}
	req := f.requests()[0]
	if req.body["name"] != "ci" || req.body["permission"] != "readwrite" {
		t.Errorf("create body = %v", req.body)
	}
	if _, set := req.body["expiresInSeconds"]; set {
		t.Errorf("a persistent key was asked to expire: %v", req.body)
	}

	// -o json is the API's own answer, secret included, and only here.
	stdout, _, err = bex(t, d, "s3", "keys", "create", "-o", "json", "--permission", "read")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"secret": "`+testSecret+`"`) {
		t.Errorf("create -o json = %s, want the API's answer", stdout)
	}

	// Listing shows no secret.
	stdout, _, err = bex(t, d, "s3", "keys", "list")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout, testSecret) || !strings.Contains(stdout, testKeyID) || !strings.Contains(stdout, "never") || !strings.Contains(stdout, "expired") {
		t.Errorf("list printed:\n%s", stdout)
	}
	stdout, _, _ = bex(t, d, "s3", "keys", "list", "-o", "json")
	if !strings.Contains(stdout, testKeyID) || strings.Contains(stdout, "secret") {
		t.Errorf("list -o json = %s", stdout)
	}
}

func TestKeysCreateRefusesWhatTheServerWould(t *testing.T) {
	f := newFakeAPI(t)
	d, _ := testDeps(t, f)
	for _, args := range [][]string{
		{"s3", "keys", "create", "--permission", "admin"},
		{"s3", "keys", "create", "--expires-in", "13h"},
		{"s3", "keys", "create", "--expires-in", "-1h"},
	} {
		if _, _, err := bex(t, d, args...); err == nil {
			t.Errorf("%v succeeded", args)
		}
	}
	if len(f.requests()) != 0 {
		t.Errorf("a refused command line still called the API: %v", f.requests())
	}
	// An expiring key is asked for in seconds.
	if _, _, err := bex(t, d, "s3", "keys", "create", "--expires-in", "2h"); err != nil {
		t.Fatal(err)
	}
	if got := f.requests()[0].body["expiresInSeconds"]; got != float64(7200) {
		t.Errorf("expiresInSeconds = %v, want 7200", got)
	}
}

func TestKeysCreateExplainsAMintClassRefusal(t *testing.T) {
	f := newFakeAPI(t)
	f.status["POST /v1/object-storage/"+testWorkspace+"/access-keys"] = answer{http.StatusForbidden, `{"code":"FORBIDDEN","message":"forbidden"}`}
	d, _ := testDeps(t, f)
	_, _, err := bex(t, d, "s3", "keys", "create")
	if err == nil || !strings.Contains(err.Error(), "bex login") {
		t.Errorf("err = %v, want it to say a person's login is needed", err)
	}
}

func TestKeysRevokeEndsTheKeyAndForgetsACachedCopy(t *testing.T) {
	f := newFakeAPI(t)
	d, state := testDeps(t, f)
	if _, _, err := bex(t, d, "s3", "env"); err != nil { // caches the key
		t.Fatal(err)
	}
	if files, _ := filepath.Glob(filepath.Join(state, "s3", "key-*.json")); len(files) != 1 {
		t.Fatalf("cache files = %v", files)
	}
	stdout, _, err := bex(t, d, "s3", "keys", "revoke", testKeyID, "s3k-zzzzzzzzzzzzzzzzzzzz")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "Revoked "+testKeyID) {
		t.Errorf("revoke printed %q", stdout)
	}
	var deletes []string
	for _, r := range f.requests() {
		if r.method == http.MethodDelete {
			deletes = append(deletes, r.path)
		}
	}
	want := []string{"/v1/object-storage/" + testWorkspace + "/access-keys/" + testKeyID, "/v1/object-storage/" + testWorkspace + "/access-keys/s3k-zzzzzzzzzzzzzzzzzzzz"}
	if !slices.Equal(deletes, want) {
		t.Errorf("DELETEs = %v, want %v", deletes, want)
	}
	if files, _ := filepath.Glob(filepath.Join(state, "s3", "key-*.json")); len(files) != 0 {
		t.Errorf("the revoked key is still cached: %v", files)
	}

	// An id that is not a key id never reaches a URL.
	before := len(f.requests())
	for _, bad := range []string{"../buckets/photos", "s3k-short", "obk-d0abc123def456ghij78"} {
		if _, _, err := bex(t, d, "s3", "keys", "revoke", bad); err == nil {
			t.Errorf("revoke %q succeeded", bad)
		}
	}
	if len(f.requests()) != before {
		t.Error("a malformed key id reached bex-api")
	}
}

func TestPublicEnableDisableAndStatus(t *testing.T) {
	f := newFakeAPI(t)
	d, _ := testDeps(t, f)

	stdout, _, err := bex(t, d, "s3", "public", "photos", "enable")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "https://pub-abc.onbex.co/<key>") || !strings.Contains(stdout, "anyone can read") {
		t.Errorf("enable printed %q", stdout)
	}
	if req := f.requests()[0]; req.method != http.MethodPut || req.body["enabled"] != true {
		t.Errorf("enable sent %+v", req)
	}

	stdout, _, err = bex(t, d, "s3", "public", "s3://photos/", "disable")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "photos is private") {
		t.Errorf("disable printed %q", stdout)
	}
	if req := f.requests()[1]; req.method != http.MethodPut || req.body["enabled"] != false {
		t.Errorf("disable sent %+v", req)
	}

	stdout, _, err = bex(t, d, "s3", "public", "photos", "status", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"host": "pub-abc.onbex.co"`) {
		t.Errorf("status -o json = %s", stdout)
	}
	if req := f.requests()[2]; req.method != http.MethodGet {
		t.Errorf("status sent %+v", req)
	}
}

func TestPublicRefusalForANonAdminSaysWhoMayPublish(t *testing.T) {
	f := newFakeAPI(t)
	f.status["PUT /v1/object-storage/"+testWorkspace+"/buckets/photos/public"] = answer{http.StatusForbidden, `{"code":"FORBIDDEN","message":"forbidden"}`}
	d, _ := testDeps(t, f)
	_, _, err := bex(t, d, "s3", "public", "photos", "enable")
	if err == nil || !strings.Contains(err.Error(), "admin role") || !strings.Contains(err.Error(), "403") {
		t.Errorf("err = %v, want the admin-role refusal with the server's status", err)
	}
	// Reading the status is not publishing: a 403 there is not about the admin role.
	f.status["GET /v1/object-storage/"+testWorkspace+"/buckets/photos/public"] = f.status["PUT /v1/object-storage/"+testWorkspace+"/buckets/photos/public"]
	_, _, err = bex(t, d, "s3", "public", "photos", "status")
	if err == nil || strings.Contains(err.Error(), "admin role") {
		t.Errorf("status err = %v, want a plain refusal", err)
	}
}

func TestPublicAndInfoNameOnlyABucket(t *testing.T) {
	f := newFakeAPI(t)
	d, _ := testDeps(t, f)
	for _, args := range [][]string{
		{"s3", "public", "../services", "enable"},
		{"s3", "public", "Photos", "enable"},
		{"s3", "public", "photos/key", "enable"},
		{"s3", "public", "photos", "toggle"},
		{"s3", "public", "photos"},
		{"s3", "info"},
		{"s3", "info", "photos", "extra"},
		{"s3", "info", "a%2Fb"},
	} {
		if _, _, err := bex(t, d, args...); err == nil {
			t.Errorf("%v succeeded", args)
		}
	}
	if len(f.requests()) != 0 {
		t.Errorf("a malformed command line reached bex-api: %v", f.requests())
	}
	_, _, err := bex(t, d, "s3", "info", "missing-bucket")
	if err == nil || !strings.Contains(err.Error(), "no bucket missing-bucket") {
		t.Errorf("err = %v, want a missing bucket named", err)
	}
}

func TestInfoShowsWhatS3CannotAnswer(t *testing.T) {
	f := newFakeAPI(t)
	d, _ := testDeps(t, f)
	stdout, _, err := bex(t, d, "s3", "info", "photos")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"frankfurt", "https://pub-abc.onbex.co/<key>", "12", "1.4 MiB", "counted 2026-11-20 09:00 UTC"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("info lacks %q:\n%s", want, stdout)
		}
	}
	stdout, _, err = bex(t, d, "s3", "info", "photos", "-o", "yaml")
	if err != nil || !strings.Contains(stdout, "region: frankfurt") {
		t.Errorf("info -o yaml = %q, %v", stdout, err)
	}
}

func TestPathElementsCannotClimbOutOfTheirRoute(t *testing.T) {
	f := newFakeAPI(t)
	a := &api{server: f.srv.URL + "/v1/", http: f.srv.Client()}
	_, _ = a.do(t.Context(), http.MethodGet, nil, bucketPath(testWorkspace, "../../services")...)
	got := f.requests()
	if len(got) != 1 || strings.Contains(got[0].path, "/services") && !strings.Contains(got[0].path, "%2F") {
		t.Errorf("request path = %v, want the element escaped", got)
	}
}
