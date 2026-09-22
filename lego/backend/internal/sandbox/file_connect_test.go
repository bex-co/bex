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
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/hmacticket"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	"github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

type fileConnectFixture struct {
	svc   *Service
	api   *httptest.Server
	calls atomic.Int32
	now   time.Time
}

func newFileConnectFixture(t *testing.T, owner string, checker core.Checker, gateway http.HandlerFunc) *fileConnectFixture {
	t.Helper()
	f := &fileConnectFixture{now: time.Now().UTC().Truncate(time.Second)}
	secret := []byte("file-connect-test-secret")
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		if _, err := sandboxfiles.Verify(secret, r.Header.Get(sandboxfiles.TicketHeader), time.Now()); err != nil {
			t.Errorf("gateway ticket: %v", err)
			http.Error(w, "invalid ticket", http.StatusUnauthorized)
			return
		}
		if gateway != nil {
			gateway(w, r)
			return
		}
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Errorf("read upload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(gw.Close)
	f.svc = &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": "tea-a", "id-admin": "tea-a"}, Authz: checker, Clock: func() time.Time { return f.now }},
		Client: execSandboxClient(t, owner),
		Exec:   &ExecConfig{Secret: secret, FileGatewayURL: gw.URL + "/sandbox-files", GatewayURL: gw.URL + "/sandbox-exec", Client: gw.Client(), Nonces: &sshgateway.NonceGuard{Store: &gatewaytest.FakeStore{}}},
	}
	gated := http.NewServeMux()
	f.svc.RegisterREST(gated)
	root := http.NewServeMux()
	root.Handle("/v1/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { gated.ServeHTTP(w, r.WithContext(identityCtx("id-a"))) }))
	root.Handle(ConnectFileUploadPattern, f.svc.ConnectFileHandler())
	root.Handle(ConnectFileDownloadPattern, f.svc.ConnectFileHandler())
	f.api = httptest.NewServer(root)
	t.Cleanup(f.api.Close)
	return f
}

func (f *fileConnectFixture) mint(t *testing.T, operation, path string) ConnectResponse {
	t.Helper()
	target := f.api.URL + "/v1/sandboxes/os-1/files/" + operation + "/token?" + url.Values{"ownerId": {"tea-a"}, "path": {path}}.Encode()
	response, err := http.Post(target, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("mint status %d: %s", response.StatusCode, body)
	}
	var out ConnectResponse
	if err := json.NewDecoder(response.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func redeemFile(t *testing.T, result ConnectResponse, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequest(result.Method, result.URI, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+result.Token)
	req.Header.Set("Content-Type", "application/octet-stream")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func TestFileConnectPinnedMintAndUpload(t *testing.T) {
	payload := []byte("raw bytes\x00\xff")
	f := newFileConnectFixture(t, "id-a", nil, func(w http.ResponseWriter, r *http.Request) {
		claims, err := sandboxfiles.Verify([]byte("file-connect-test-secret"), r.Header.Get(sandboxfiles.TicketHeader), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if claims.Subject != "id-a" || claims.Workspace != "tea-a" || claims.Namespace != "tea-a-sandbox" || claims.SandboxID != "os-1" || claims.Path != "/workspace/out.txt" || claims.Operation != "upload" {
			t.Errorf("wrong bound file target: %+v", claims)
		}
		if r.Method != http.MethodPut || r.URL.Path != "/sandbox-files" || r.ContentLength != int64(len(payload)) {
			t.Errorf("bridge method/path/length = %s %s %d", r.Method, r.URL.Path, r.ContentLength)
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("caller credential reached gateway")
		}
		got, err := io.ReadAll(r.Body)
		if err != nil || !bytes.Equal(got, payload) {
			t.Errorf("upload = %q, %v", got, err)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	token := f.mint(t, "upload", "/workspace/out.txt")
	if token.Method != http.MethodPut || token.Token == "" || token.ExpiresAt.Sub(f.now) > time.Minute || !strings.Contains(token.URI, "/files/upload/") {
		t.Fatalf("invalid connect response: %+v", token)
	}
	if kind, ok := id.KindOf(token.ExecutionID); !ok || kind != id.SandboxExecution {
		t.Fatalf("bad execution id %q", token.ExecutionID)
	}
	if got := redeemFile(t, token, bytes.NewReader(payload)).StatusCode; got != http.StatusNoContent {
		t.Fatalf("upload status %d", got)
	}
	if got := redeemFile(t, token, bytes.NewReader(payload)).StatusCode; got != http.StatusConflict {
		t.Fatalf("replay status %d", got)
	}
	if f.calls.Load() != 1 {
		t.Fatalf("gateway called %d times", f.calls.Load())
	}
}

func TestFileConnectMintRefusesUnauthorizedAndUnsafePaths(t *testing.T) {
	f := newFileConnectFixture(t, "id-a", nil, nil)
	base := FileConnectRequest{OwnerID: "tea-a", SandboxID: "os-1", Operation: "upload", Path: "/workspace/out.txt"}
	for _, bad := range []string{"", "relative", "/", "/workspace/../etc/passwd", "/workspace/./x", "/workspace//x", "/workspace/nul\x00x", strings.Repeat("x", 4097)} {
		req := base
		req.Path = bad
		if _, err := f.svc.ConnectFile(identityCtx("id-a"), req, f.api.URL); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("path %q: %v", bad, err)
		}
	}
	for _, op := range []string{"delete", "stream", ""} {
		req := base
		req.Operation = op
		if _, err := f.svc.ConnectFile(identityCtx("id-a"), req, f.api.URL); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("operation %q: %v", op, err)
		}
	}
	foreign := newFileConnectFixture(t, "id-b", adminChecker{}, nil)
	if _, err := foreign.svc.ConnectFile(identityCtx("id-a"), base, foreign.api.URL); err == nil {
		t.Fatal("foreign owner minted a file token")
	}
	if _, err := foreign.svc.ConnectFile(identityCtx("id-admin"), base, foreign.api.URL); err != nil {
		t.Fatalf("admin override: %v", err)
	}
	base.OwnerID = "tea-other"
	if _, err := f.svc.ConnectFile(identityCtx("id-a"), base, f.api.URL); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("foreign workspace: %v", err)
	}
}

func TestFileConnectWrongTargetCannotConsumeToken(t *testing.T) {
	f := newFileConnectFixture(t, "id-a", nil, nil)
	original := f.mint(t, "upload", "/workspace/out.txt")
	cases := []ConnectResponse{}
	changed := original
	changed.URI = strings.Replace(original.URI, "/os-1/", "/os-other/", 1)
	cases = append(cases, changed)
	changed = original
	changed.URI = strings.Replace(original.URI, original.ExecutionID, "exe-other", 1)
	cases = append(cases, changed)
	changed = original
	changed.URI = strings.Replace(original.URI, "out.txt", "other.txt", 1)
	cases = append(cases, changed)
	changed = original
	changed.URI += "&path=%2Fworkspace%2Fout.txt"
	cases = append(cases, changed)
	changed = original
	changed.URI = strings.Replace(original.URI, "/files/upload/", "/files/download/", 1)
	changed.Method = http.MethodGet
	cases = append(cases, changed)
	for _, bad := range cases {
		if got := redeemFile(t, bad, nil).StatusCode; got != http.StatusForbidden {
			t.Fatalf("misuse %s status %d", bad.URI, got)
		}
	}
	if got := redeemFile(t, original, nil).StatusCode; got != http.StatusNoContent {
		t.Fatalf("valid after misuse: %d", got)
	}
	if f.calls.Load() != 1 {
		t.Fatalf("gateway called %d times", f.calls.Load())
	}
}

func TestFileConnectExpiryAndCredentialSeparation(t *testing.T) {
	f := newFileConnectFixture(t, "id-a", nil, nil)
	original := f.mint(t, "upload", "/workspace/out.txt")
	run, err := f.svc.ConnectRun(identityCtx("id-a"), ConnectRequest{OwnerID: "tea-a", SandboxID: "os-1", Operation: "stream", Command: "true"}, f.api.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, credential := range []string{"oauth-access-token", run.Token} {
		bad := original
		bad.Token = credential
		if got := redeemFile(t, bad, nil).StatusCode; got != http.StatusUnauthorized {
			t.Fatalf("foreign credential status %d", got)
		}
	}
	if _, err := sandboxexec.Verify(f.svc.Exec.Secret, original.Token, f.now); err == nil {
		t.Fatal("file connect token verified as exec ticket")
	}
	if _, err := sandboxfiles.Verify(f.svc.Exec.Secret, original.Token, f.now); err == nil {
		t.Fatal("file connect token verified as gateway file ticket")
	}
	if _, err := f.svc.verifyConnectToken(context.Background(), original.Token, "os-1", original.ExecutionID, "true"); err == nil {
		t.Fatal("file token verified as run connect token")
	}
	f.now = f.now.Add(time.Minute + hmacticket.ClockSkew + time.Second)
	if got := redeemFile(t, original, nil).StatusCode; got != http.StatusGone {
		t.Fatalf("expired status %d", got)
	}
	if f.calls.Load() != 0 {
		t.Fatal("refused credential reached gateway")
	}
}

type fileFreshChecker struct{ revoked atomic.Bool }

func (c *fileFreshChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}
func (c *fileFreshChecker) CheckFresh(context.Context, string, string, string) (bool, error) {
	return !c.revoked.Load(), nil
}

func TestFileConnectRechecksRevocation(t *testing.T) {
	checker := &fileFreshChecker{}
	f := newFileConnectFixture(t, "id-a", checker, nil)
	token := f.mint(t, "upload", "/workspace/out.txt")
	checker.revoked.Store(true)
	if got := redeemFile(t, token, nil).StatusCode; got != http.StatusForbidden {
		t.Fatalf("revoked status %d", got)
	}
	if f.calls.Load() != 0 {
		t.Fatal("revoked caller reached gateway")
	}
}

func TestFileConnectSingleUseAcrossReplicas(t *testing.T) {
	f := newFileConnectFixture(t, "id-a", nil, nil)
	token := f.mint(t, "upload", "/workspace/out.txt")
	shared := &gatewaytest.FakeStore{}
	var winners atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			replica := *f.svc
			replica.Exec = &ExecConfig{Secret: f.svc.Exec.Secret, Nonces: &sshgateway.NonceGuard{Store: shared}}
			r := httptest.NewRequest(token.Method, token.URI, nil)
			r.SetPathValue("id", "os-1")
			r.SetPathValue("executionId", token.ExecutionID)
			r.Header.Set("Authorization", "Bearer "+token.Token)
			if _, err := replica.verifyFileConnectToken(r.Context(), r); err == nil {
				winners.Add(1)
			}
		})
	}
	wg.Wait()
	if winners.Load() != 1 {
		t.Fatalf("redemption winners = %d", winners.Load())
	}
}

func TestFileConnectNonceStoreOutageFailsClosed(t *testing.T) {
	f := newFileConnectFixture(t, "id-a", nil, nil)
	token := f.mint(t, "upload", "/workspace/out.txt")
	f.svc.Exec.Nonces = &sshgateway.NonceGuard{Store: &gatewaytest.FakeStore{ClaimErr: errors.New("nonce store unavailable")}}
	if got := redeemFile(t, token, nil).StatusCode; got != http.StatusConflict {
		t.Fatalf("nonce store failure status %d", got)
	}
	if f.calls.Load() != 0 {
		t.Fatal("nonce-store failure reached gateway")
	}
}

func TestFileConnectProxyPreservesCompressedDownloadAndFailureStatus(t *testing.T) {
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, _ = zw.Write([]byte("byte-identical payload"))
	_ = zw.Close()
	f := newFileConnectFixture(t, "id-a", nil, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Encoding") != "gzip" {
			t.Error("proxy may transparently decode gateway gzip")
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Disposition", `attachment; filename="out.txt"`)
		_, _ = w.Write(compressed.Bytes())
	})
	token := f.mint(t, "download", "/workspace/out.txt")
	request, _ := http.NewRequest(token.Method, token.URI, nil)
	request.Header.Set("Authorization", "Bearer "+token.Token)
	request.Header.Set("Accept-Encoding", "gzip")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	got, err := io.ReadAll(response.Body)
	if err != nil || !bytes.Equal(got, compressed.Bytes()) || response.Header.Get("Content-Encoding") != "gzip" || response.Header.Get("Content-Disposition") == "" {
		t.Fatalf("download bytes/headers lost, err=%v", err)
	}
	denied := newFileConnectFixture(t, "id-a", nil, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "missing path", http.StatusNotFound) })
	if got := redeemFile(t, denied.mint(t, "download", "/missing"), nil).StatusCode; got != http.StatusNotFound {
		t.Fatalf("gateway error status %d", got)
	}
}

func TestFileConnectProxyCancelsGatewayOnDisconnect(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	f := newFileConnectFixture(t, "id-a", nil, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(canceled)
	})
	token := f.mint(t, "download", "/workspace/out.txt")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, token.Method, token.URI, nil)
	req.Header.Set("Authorization", "Bearer "+token.Token)
	done := make(chan struct{})
	go func() {
		response, _ := http.DefaultClient.Do(req)
		if response != nil {
			_ = response.Body.Close()
		}
		close(done)
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("gateway did not start")
	}
	cancel()
	select {
	case <-canceled:
	case <-time.After(5 * time.Second):
		t.Fatal("gateway survived caller disconnect")
	}
	<-done
}

type fileDeadlineWriter struct {
	*httptest.ResponseRecorder
	reads, writes []time.Time
}

func (w *fileDeadlineWriter) SetReadDeadline(deadline time.Time) error {
	w.reads = append(w.reads, deadline)
	return nil
}
func (w *fileDeadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.writes = append(w.writes, deadline)
	return nil
}

func TestFileConnectBoundsAndClearsConnectionDeadlines(t *testing.T) {
	f := newFileConnectFixture(t, "id-a", nil, nil)
	token := f.mint(t, "upload", "/workspace/out.txt")
	r := httptest.NewRequest(token.Method, token.URI, strings.NewReader("bounded"))
	r.SetPathValue("id", "os-1")
	r.SetPathValue("executionId", token.ExecutionID)
	r.Header.Set("Authorization", "Bearer "+token.Token)
	writer := &fileDeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	before := time.Now()
	f.svc.ConnectFileHandler().ServeHTTP(writer, r)
	if writer.Code != http.StatusNoContent {
		t.Fatalf("transfer status %d: %s", writer.Code, writer.Body)
	}
	for direction, deadlines := range map[string][]time.Time{"read": writer.reads, "write": writer.writes} {
		if len(deadlines) != 2 || !deadlines[1].IsZero() || deadlines[0].Before(before) || deadlines[0].After(time.Now().Add(sandboxfiles.DefaultTransferTimeout)) {
			t.Errorf("%s deadline was not bounded then cleared: %v", direction, deadlines)
		}
	}
}

func TestFileConnectRequiresSharedNonceStore(t *testing.T) {
	f := newFileConnectFixture(t, "id-a", nil, nil)
	token := f.mint(t, "upload", "/workspace/out.txt")
	for _, nonces := range []*sshgateway.NonceGuard{nil, {}} {
		f.svc.Exec.Nonces = nonces
		if _, err := f.svc.ConnectFile(identityCtx("id-a"), FileConnectRequest{OwnerID: "tea-a", SandboxID: "os-1", Operation: "upload", Path: "/workspace/out.txt"}, f.api.URL); !errors.Is(err, core.ErrUnavailable) || !strings.Contains(err.Error(), "shared nonce store") {
			t.Errorf("mint without durable guard: %v", err)
		}
		if got := redeemFile(t, token, nil).StatusCode; got != http.StatusServiceUnavailable {
			t.Errorf("redeem without durable guard: status %d", got)
		}
	}
	if f.calls.Load() != 0 {
		t.Fatal("unprotected transfer reached gateway")
	}
}

func TestFileConnectStreamsUnknownLengthUploadBeforeEOF(t *testing.T) {
	firstByte := make(chan struct{})
	f := newFileConnectFixture(t, "id-a", nil, func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength != -1 || r.Header.Get("Content-Type") != "application/x-tar" || r.Header.Get("Content-Encoding") != "gzip" {
			t.Errorf("archive wire metadata changed: length=%d headers=%v", r.ContentLength, r.Header)
		}
		one := make([]byte, 1)
		if _, err := io.ReadFull(r.Body, one); err != nil {
			t.Errorf("first byte: %v", err)
			return
		}
		close(firstByte)
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	})
	token := f.mint(t, "upload", "/workspace/directory")
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	r, _ := http.NewRequest(token.Method, token.URI, reader)
	r.Header.Set("Authorization", "Bearer "+token.Token)
	r.Header.Set("Content-Type", "application/x-tar")
	r.Header.Set("Content-Encoding", "gzip")
	type result struct {
		status int
		err    error
	}
	done := make(chan result, 1)
	go func() {
		response, err := http.DefaultClient.Do(r)
		out := result{err: err}
		if response != nil {
			out.status = response.StatusCode
			_ = response.Body.Close()
		}
		done <- out
	}()
	if _, err := writer.Write([]byte("a")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-firstByte:
	case <-time.After(5 * time.Second):
		t.Fatal("gateway received no data before upload EOF")
	}
	_, _ = writer.Write([]byte("b"))
	_ = writer.Close()
	out := <-done
	if out.err != nil || out.status != http.StatusNoContent {
		t.Fatalf("stream result: %+v", out)
	}
}

type fileSensitiveChecker struct{}

func (fileSensitiveChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}
func (fileSensitiveChecker) CheckFresh(_ context.Context, _, relation, _ string) (bool, error) {
	return relation != core.RelCanViewSensitive, nil
}

func TestFileConnectPreservesAgentSessionSensitiveBoundary(t *testing.T) {
	f := newFileConnectFixture(t, "id-a", fileSensitiveChecker{}, nil)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(osSandbox{ID: "os-1", Metadata: map[string]string{
			metadataOwner: "id-a", metadataWorkspace: "tea-a", metadataRegime: metadataSandboxRegime,
			metadataNetworkPolicy: string(NetworkPolicyDenyAll), metadataAgentSession: "ags-one",
		}})
	}))
	defer provider.Close()
	f.svc.Client = NewClient(provider.URL)
	for _, operation := range []string{"upload", "download"} {
		_, err := f.svc.ConnectFile(identityCtx("id-a"), FileConnectRequest{OwnerID: "tea-a", SandboxID: "os-1", Operation: operation, Path: "/workspace/out.txt"}, f.api.URL)
		if !errors.Is(err, core.ErrForbidden) {
			t.Errorf("%s minted for session without fresh sensitive access: %v", operation, err)
		}
	}
}

func TestFileConnectRechecksSandboxOwnershipAtRedemption(t *testing.T) {
	f := newFileConnectFixture(t, "id-a", nil, nil)
	token := f.mint(t, "download", "/workspace/out.txt")
	f.svc.Client = execSandboxClient(t, "id-other")
	response := redeemFile(t, token, nil)
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusNotFound || !strings.Contains(string(body), "SANDBOX_NOT_FOUND") {
		t.Fatalf("changed owner: status=%d body=%s", response.StatusCode, body)
	}
	if f.calls.Load() != 0 {
		t.Fatal("sandbox whose ownership changed reached gateway")
	}
}
