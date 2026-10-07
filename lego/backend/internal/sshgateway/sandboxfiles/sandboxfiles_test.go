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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	files "github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
)

type executorFunc func(context.Context, []string, io.Reader, io.Writer) (int, error)

func (f executorFunc) Execute(ctx context.Context, _ apps.SSHInstanceTarget, command []string, _ bool, _ remotecommand.TerminalSizeQueue, stdin io.Reader, stdout, _ io.Writer) (int, error) {
	return f(ctx, command, stdin, stdout)
}

var testSecret = []byte("file-transfer-test-secret")

func serverFor(exec executorFunc) *Server {
	return &Server{Secret: testSecret, Executor: exec,
		Nonces: &sshgateway.NonceGuard{Store: &gatewaytest.FakeStore{}}, TransferTimeout: time.Second}
}

func requestFor(t *testing.T, method string, body []byte) *http.Request {
	t.Helper()
	op := files.OperationUpload
	if method == http.MethodGet {
		op = files.OperationDownload
	}
	token, err := files.Mint(testSecret, files.Claims{Subject: "caller", Workspace: "tea-a", SandboxID: "os-a", Namespace: "tea-a-sandbox",
		Operation: op, Path: "/workspace/file", IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, "http://gateway"+files.GatewayPath, bytes.NewReader(body))
	r.Header.Set(files.TicketHeader, token)
	r.Header.Set("Content-Type", "application/octet-stream")
	return r
}

func archive(t *testing.T, headers ...*tar.Header) []byte {
	t.Helper()
	var body bytes.Buffer
	tw := tar.NewWriter(&body)
	for _, h := range headers {
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := io.CopyN(tw, zeroReader{}, h.Size); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func gzipBytes(t *testing.T, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUploadPublishesOnlyAfterCompleteValidation(t *testing.T) {
	valid := archive(t, &tar.Header{Name: "empty", Typeflag: tar.TypeReg, Mode: 0644})
	badGzip := gzipBytes(t, valid)
	badGzip = badGzip[:len(badGzip)-4]
	// The gzip footer opens with the CRC-32 of everything before it.
	badChecksum := gzipBytes(t, valid)
	badChecksum[len(badChecksum)-8] ^= 0xff
	for _, tc := range []struct {
		name            string
		body            []byte
		media, encoding string
		status          int
	}{
		{"raw", []byte("binary\x00data"), "application/octet-stream", "", 204},
		{"empty raw", nil, "application/octet-stream", "", 204},
		{"directory", valid, "application/x-tar", "", 204},
		{"gzip directory", gzipBytes(t, valid), "application/x-tar", "gzip", 204},
		{"truncated tar boundary", valid[:512], "application/x-tar", "", 400},
		{"truncated gzip footer", badGzip, "application/x-tar", "gzip", 400},
		{"corrupt gzip checksum", badChecksum, "application/x-tar", "gzip", 400},
		{"traversal", archive(t, &tar.Header{Name: "../escape", Typeflag: tar.TypeReg}), "application/x-tar", "", 400},
		{"symlink", archive(t, &tar.Header{Name: "escape", Typeflag: tar.TypeSymlink, Linkname: "../outside"}), "application/x-tar", "", 400},
		{"hardlink", archive(t, &tar.Header{Name: "link", Typeflag: tar.TypeLink, Linkname: "file"}), "application/x-tar", "", 400},
		{"fifo", archive(t, &tar.Header{Name: "fifo", Typeflag: tar.TypeFifo}), "application/x-tar", "", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			publishes, cleanups := 0, 0
			s := serverFor(func(_ context.Context, cmd []string, input io.Reader, _ io.Writer) (int, error) {
				if input != nil {
					// Model tar's dangerous behavior: even a malformed/truncated
					// input can look like clean EOF to the remote process.
					_, _ = io.Copy(io.Discard, input)
				} else if cmd[2] == publishScript {
					publishes++
				} else {
					cleanups++
				}
				return 0, nil
			})
			r := requestFor(t, http.MethodPut, tc.body)
			r.Header.Set("Content-Type", tc.media)
			r.Header.Set("Content-Encoding", tc.encoding)
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status = %d: %s", w.Code, w.Body.String())
			}
			if (publishes == 1) != (tc.status == 204) {
				t.Fatalf("published %d times for status %d", publishes, w.Code)
			}
			wantCleanups := 1
			if tc.status == 204 {
				wantCleanups = 0
			}
			if cleanups != wantCleanups {
				t.Fatalf("cleanup count = %d", cleanups)
			}
		})
	}
}

func TestRawUploadLengthAndSizeRefusals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		length int64
		status int
	}{
		{"unknown", -1, 411}, {"oversized", files.MaxTransferBytes + 1, 413}, {"short body", 8, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			published := false
			s := serverFor(func(_ context.Context, cmd []string, input io.Reader, _ io.Writer) (int, error) {
				if input != nil {
					_, _ = io.Copy(io.Discard, input)
				}
				if cmd[2] == publishScript {
					published = true
				}
				return 0, nil
			})
			r := requestFor(t, http.MethodPut, []byte("short"))
			r.ContentLength = tc.length
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != tc.status || published {
				t.Fatalf("status=%d published=%v: %s", w.Code, published, w.Body.String())
			}
		})
	}
}

func TestChunkedEmptyRawUploadMatchesPinnedCLI(t *testing.T) {
	published := false
	s := serverFor(func(_ context.Context, cmd []string, input io.Reader, _ io.Writer) (int, error) {
		if input != nil {
			if n, err := io.Copy(io.Discard, input); n != 0 || err != nil {
				t.Errorf("empty body = %d bytes, %v", n, err)
			}
			if cmd[len(cmd)-1] != "0" {
				t.Errorf("staged length = %q", cmd[len(cmd)-1])
			}
		}
		if cmd[2] == publishScript {
			published = true
		}
		return 0, nil
	})
	r := requestFor(t, http.MethodPut, nil)
	r.ContentLength = -1
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 204 || !published {
		t.Fatalf("status=%d published=%v: %s", w.Code, published, w.Body.String())
	}
}

func TestArchiveEntryPolicyAndBudgets(t *testing.T) {
	for _, name := range []string{"/", "/absolute", "a/../b", "../b", "a//b", "a\nb", "a\\b", strings.Repeat("a", files.MaxPathBytes+1)} {
		t.Run(name, func(t *testing.T) {
			data := archive(t, &tar.Header{Name: name, Typeflag: tar.TypeDir})
			if err := copyArchive(io.Discard, bytes.NewReader(data)); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
	for _, headers := range [][]*tar.Header{
		{{Name: "same", Typeflag: tar.TypeReg}, {Name: "same", Typeflag: tar.TypeReg}},
		{{Name: "missing/child", Typeflag: tar.TypeReg}},
	} {
		if err := copyArchive(io.Discard, bytes.NewReader(archive(t, headers...))); err == nil {
			t.Fatal("duplicate/missing parent accepted")
		}
	}
	var large bytes.Buffer
	tw := tar.NewWriter(&large)
	if err := tw.WriteHeader(&tar.Header{Name: "huge", Typeflag: tar.TypeReg, Size: files.MaxTransferBytes + 1}); err != nil {
		t.Fatal(err)
	}
	if err := copyArchive(io.Discard, &large); err == nil {
		t.Fatal("oversized member accepted")
	}
	entries := make([]*tar.Header, files.MaxArchiveEntries+1)
	for i := range entries {
		entries[i] = &tar.Header{Name: "./", Typeflag: tar.TypeDir}
	}
	if err := copyArchive(io.Discard, bytes.NewReader(archive(t, entries...))); err == nil {
		t.Fatal("entry count bypass through root directories")
	}
	if _, err := io.Copy(io.Discard, &boundedReader{reader: strings.NewReader("123456789"), remaining: 8}); !errors.Is(err, errTooLarge) {
		t.Fatalf("byte cap = %v", err)
	}
}

func TestDownloadGzipFinalizesOnlyAfterExecutorSuccess(t *testing.T) {
	validTar := archive(t, &tar.Header{Name: "empty", Typeflag: tar.TypeReg})
	for _, tc := range []struct {
		name, marker string
		body         []byte
	}{
		{"binary", "file\n", []byte("\x00\xffhello")}, {"empty", "file\n", nil}, {"directory", "directory\n", validTar},
	} {
		for _, exitCode := range []int{0, 7} {
			t.Run(tc.name+"/"+string(rune('0'+exitCode)), func(t *testing.T) {
				s := serverFor(func(_ context.Context, _ []string, _ io.Reader, out io.Writer) (int, error) {
					_, _ = io.WriteString(out, tc.marker)
					_, _ = out.Write(tc.body)
					return exitCode, nil // Failure occurs AFTER every artifact byte.
				})
				srv := httptest.NewServer(s.Handler())
				defer srv.Close()
				req := requestFor(t, http.MethodGet, nil)
				req.RequestURI = ""
				req.URL.Scheme, req.URL.Host = "http", strings.TrimPrefix(srv.URL, "http://")
				client := &http.Client{Transport: &http.Transport{DisableCompression: true}}
				defer client.CloseIdleConnections()
				resp, err := client.Do(req)
				if err != nil {
					if exitCode == 0 {
						t.Fatal(err)
					}
					return
				}
				defer resp.Body.Close()
				gz, err := gzip.NewReader(resp.Body)
				if err != nil {
					if exitCode == 0 {
						t.Fatal(err)
					}
					return
				}
				defer gz.Close()
				body, err := io.ReadAll(gz)
				if exitCode != 0 {
					if err == nil {
						t.Fatal("late executor failure looked like a complete file")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if tc.marker == "file\n" && !bytes.Equal(body, tc.body) {
					t.Fatalf("download = %q", body)
				}
				if tc.marker == "directory\n" {
					if _, err := tar.NewReader(bytes.NewReader(body)).Next(); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}

func TestFileGatewayRequiresDurableReplayGuard(t *testing.T) {
	store := &gatewaytest.FakeStore{}
	exec := executorFunc(func(context.Context, []string, io.Reader, io.Writer) (int, error) { return 66, nil })
	first, second := serverFor(exec), serverFor(exec)
	first.Nonces.Store, second.Nonces.Store = store, store
	r := requestFor(t, http.MethodGet, nil)
	for i, s := range []*Server{first, second} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r.Clone(context.Background()))
		want := 404
		if i == 1 {
			want = 409
		}
		if w.Code != want {
			t.Fatalf("replica %d status = %d: %s", i, w.Code, w.Body.String())
		}
	}
	missing := serverFor(exec)
	missing.Nonces.Store = nil
	w := httptest.NewRecorder()
	missing.Handler().ServeHTTP(w, requestFor(t, http.MethodGet, nil))
	if w.Code != 503 {
		t.Fatalf("without durable store status = %d", w.Code)
	}
}

type revalidatorFunc func(context.Context, files.Claims) error

func (f revalidatorFunc) RevalidateFiles(ctx context.Context, claims files.Claims) error {
	return f(ctx, claims)
}

func TestUploadRevocationStopsBodyAndDoesNotPublish(t *testing.T) {
	var calls atomic.Int32
	var published atomic.Bool
	stopped := make(chan struct{})
	s := serverFor(func(ctx context.Context, cmd []string, input io.Reader, _ io.Writer) (int, error) {
		if input != nil {
			_, _ = io.Copy(io.Discard, input)
			close(stopped)
			return 0, ctx.Err()
		}
		if cmd[2] == publishScript {
			published.Store(true)
		}
		return 0, nil
	})
	s.RevalidateInterval = time.Millisecond
	s.Revalidator = revalidatorFunc(func(context.Context, files.Claims) error {
		if calls.Add(1) > 1 {
			return errors.New("revoked")
		}
		return nil
	})
	r := requestFor(t, http.MethodPut, nil)
	pr, pw := io.Pipe()
	defer pw.Close()
	r.Body, r.ContentLength = pr, 100
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	select {
	case <-stopped:
	default:
		t.Fatal("executor survived revoked transfer")
	}
	if published.Load() {
		t.Fatal("revoked transfer published")
	}
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), errRevoked.message) {
		t.Fatalf("revoked upload answered %d %q, want 403 %q as the exec stream answers a revocation", w.Code, w.Body, errRevoked.message)
	}
}

// TestDownloadRevocationAnswersForbidden (w5/139): a download revoked before
// its first byte answered the 503 of an outage. It answers 403 and counts the
// session revoked, as a revoked upload and the exec stream do, and one revoked
// after its first byte is aborted and still counted. A transfer that runs out
// its time, or whose client leaves, while a watchdog check is in flight is
// neither: that check fails only because its own context ended.
func TestDownloadRevocationAnswersForbidden(t *testing.T) {
	refuseLater := func(calls *atomic.Int32) revalidatorFunc {
		return func(context.Context, files.Claims) error {
			if calls.Add(1) > 1 {
				return errors.New("revoked")
			}
			return nil
		}
	}
	inFlight := func(calls *atomic.Int32) revalidatorFunc {
		return func(ctx context.Context, _ files.Claims) error {
			if calls.Add(1) == 1 {
				return nil
			}
			<-ctx.Done()
			return ctx.Err()
		}
	}
	untilDone := func(ctx context.Context, _ []string, _ io.Reader, _ io.Writer) (int, error) {
		<-ctx.Done()
		time.Sleep(20 * time.Millisecond) // the exec stream's teardown
		return 0, ctx.Err()
	}
	streaming := func(ctx context.Context, _ []string, _ io.Reader, out io.Writer) (int, error) {
		_, _ = out.Write([]byte("file\nsome bytes"))
		<-ctx.Done()
		return 0, ctx.Err()
	}
	for name, tc := range map[string]struct {
		exec        executorFunc
		revalidator func(*atomic.Int32) revalidatorFunc
		timeout     time.Duration
		leave       bool // the client goes away
		want        int  // 0: the response was aborted; -1: anything but 403
		revoked     float64
	}{
		"revoked before its first byte": {exec: untilDone, revalidator: refuseLater, timeout: 10 * time.Second, want: http.StatusForbidden, revoked: 1},
		"revoked while streaming":       {exec: streaming, revalidator: refuseLater, timeout: 10 * time.Second, revoked: 1},
		"timed out mid-check":           {exec: untilDone, revalidator: inFlight, timeout: 50 * time.Millisecond, want: http.StatusGatewayTimeout},
		"left by its client mid-check":  {exec: untilDone, revalidator: inFlight, timeout: 10 * time.Second, leave: true, want: -1},
	} {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			reg := prometheus.NewRegistry()
			s := serverFor(tc.exec)
			s.Metrics = sshgateway.NewMetrics(reg)
			s.TransferTimeout = tc.timeout
			s.RevalidateInterval = 5 * time.Millisecond
			s.Revalidator = tc.revalidator(&calls)
			r := requestFor(t, http.MethodGet, nil)
			if tc.leave {
				ctx, cancel := context.WithCancel(r.Context())
				time.AfterFunc(30*time.Millisecond, cancel)
				r = r.WithContext(ctx)
			}
			w := httptest.NewRecorder()
			aborted := func() (aborted bool) {
				defer func() { aborted = recover() == http.ErrAbortHandler }()
				s.Handler().ServeHTTP(w, r)
				return false
			}()
			switch {
			case tc.want == 0 && !aborted:
				t.Errorf("download answered %d, want it aborted", w.Code)
			case tc.want < 0 && w.Code == http.StatusForbidden:
				t.Error("download answered 403, but nothing revoked it")
			case tc.want > 0 && w.Code != tc.want:
				t.Errorf("download answered %d %q, want %d", w.Code, w.Body, tc.want)
			}
			if got := gatewaytest.MetricValue(t, reg, "bex_ssh_gateway_sessions_total", map[string]string{"result": "revoked"}); got != tc.revoked {
				t.Errorf("sessions counted revoked = %v, want %v", got, tc.revoked)
			}
		})
	}
}

// TestAnExecutorFailureAnswersWithoutItsText (w5/131): the endpoint wrote an
// executor's error to the client, and a Kubernetes exec error names the API
// server, the pod and the node. One wrapping io.ErrUnexpectedEOF or a deadline
// was even answered as the client's bad upload (400) or a timeout (504). An
// executor's failure now answers the fixed 503 sentence and its cause is
// logged, as is any other unclassified failure's, such as a failed read of the
// upload. A terminated target still answers its own 404.
func TestAnExecutorFailureAnswersWithoutItsText(t *testing.T) {
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(prev) })
	answers := func(t *testing.T, s *Server, r *http.Request, status int, body, logged string) {
		t.Helper()
		logs.Reset()
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != status || w.Body.String() != body {
			t.Fatalf("answered %d %q, want %d %q", w.Code, w.Body.String(), status, body)
		}
		if logged != "" && !strings.Contains(logs.String(), logged) {
			t.Fatalf("the cause was not logged: %q", logs.String())
		}
	}
	const failed = "sandbox file operation failed\n"
	const execURL = "https://10.96.0.1:443/api/v1/namespaces/tea-a-sandbox/pods/os-a/exec"
	for name, tc := range map[string]struct {
		err          error
		status       int
		body, logged string
	}{
		"dropped stream":    {&url.Error{Op: "Post", URL: execURL, Err: io.ErrUnexpectedEOF}, http.StatusServiceUnavailable, failed, execURL},
		"exec deadline":     {&url.Error{Op: "Post", URL: execURL, Err: context.DeadlineExceeded}, http.StatusServiceUnavailable, failed, execURL},
		"terminated target": {fmt.Errorf("%w: pod no longer exists", sshgateway.ErrTargetTerminated), http.StatusNotFound, "sandbox target terminated: pod no longer exists\n", ""},
	} {
		for _, method := range []string{http.MethodPut, http.MethodGet} {
			t.Run(name+"/"+method, func(t *testing.T) {
				s := serverFor(func(_ context.Context, _ []string, input io.Reader, _ io.Writer) (int, error) {
					if input != nil {
						_, _ = io.Copy(io.Discard, input)
					}
					return 126, tc.err
				})
				answers(t, s, requestFor(t, method, []byte("data")), tc.status, tc.body, tc.logged)
			})
		}
	}

	const readFailure = "read tcp 10.244.0.7:2222->10.244.1.9:40312: i/o timeout"
	drained := serverFor(func(_ context.Context, _ []string, input io.Reader, _ io.Writer) (int, error) {
		if input != nil {
			_, _ = io.Copy(io.Discard, input)
		}
		return 0, nil
	})
	t.Run("unreadable upload", func(t *testing.T) {
		r := requestFor(t, http.MethodPut, nil)
		r.Body, r.ContentLength = io.NopCloser(iotest.ErrReader(errors.New(readFailure))), -1
		answers(t, drained, r, http.StatusServiceUnavailable, failed, readFailure)
	})
	t.Run("unreadable archive", func(t *testing.T) {
		valid := archive(t, &tar.Header{Name: "file", Typeflag: tar.TypeReg, Size: 4, Mode: 0644})
		r := requestFor(t, http.MethodPut, nil)
		r.Header.Set("Content-Type", "application/x-tar")
		r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(valid[:100]), iotest.ErrReader(errors.New(readFailure))))
		answers(t, drained, r, http.StatusServiceUnavailable, failed, readFailure)
	})
	t.Run("bad download marker", func(t *testing.T) {
		// Refusing the marker cancels the stream; the log names the refusal,
		// not that cancellation.
		s := serverFor(func(ctx context.Context, _ []string, _ io.Reader, out io.Writer) (int, error) {
			_, _ = io.WriteString(out, "symlink\n")
			<-ctx.Done()
			return 126, ctx.Err()
		})
		answers(t, s, requestFor(t, http.MethodGet, nil), http.StatusServiceUnavailable, failed, "invalid sandbox transfer response")
	})
}
