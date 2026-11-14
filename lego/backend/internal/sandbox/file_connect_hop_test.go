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
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
)

func readFileError(t *testing.T, response *http.Response) (int, string) {
	t.Helper()
	var body struct{ Message string }
	raw, _ := io.ReadAll(response.Body)
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("status %d, body %q is not a JSON error", response.StatusCode, raw)
	}
	return response.StatusCode, body.Message
}

// w32/m380 t003: Go's transport replays a request with no body when a reused
// connection closes before any response. The gateway spends the ticket's nonce
// on arrival, so a replayed download read "409 file ticket already used". The
// hop now reaches the gateway once and answers what happened.
func TestFileConnectNeverReplaysTheGatewayHop(t *testing.T) {
	var drop atomic.Bool
	f := newFileConnectFixture(t, "id-a", nil, func(w http.ResponseWriter, _ *http.Request) {
		if drop.Load() {
			// The gateway read the request, then its connection closed.
			if conn, _, err := http.NewResponseController(w).Hijack(); err == nil {
				_ = conn.Close()
			}
			return
		}
		_, _ = io.WriteString(w, "warm")
	})
	// The first transfer leaves an idle connection for the second to reuse.
	if response := redeemFile(t, f.mint(t, "download", "/workspace/a"), nil); response.StatusCode != http.StatusOK {
		t.Fatalf("warm-up download = %d", response.StatusCode)
	}
	drop.Store(true)
	status, message := readFileError(t, redeemFile(t, f.mint(t, "download", "/workspace/b"), nil))
	if status != http.StatusServiceUnavailable || message != "the sandbox file transfer ended before a response" {
		t.Fatalf("dropped hop = %d %q", status, message)
	}
	if got := f.calls.Load(); got != 2 {
		t.Fatalf("the gateway saw %d requests for 2 transfers: the dropped one was replayed", got)
	}
}

// A hop that drops mid-stream reaches its client truncated, once.
func TestFileConnectTruncatesADownloadTheGatewayAbortsMidStream(t *testing.T) {
	f := newFileConnectFixture(t, "id-a", nil, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(make([]byte, 64<<10))
		_ = http.NewResponseController(w).Flush()
		panic(http.ErrAbortHandler)
	})
	response := redeemFile(t, f.mint(t, "download", "/workspace/out.txt"), nil)
	if _, err := io.ReadAll(response.Body); err == nil {
		t.Fatalf("an aborted download read as complete (status %d)", response.StatusCode)
	}
	if got := f.calls.Load(); got != 1 {
		t.Fatalf("the gateway saw %d requests, want 1", got)
	}
}

// A gateway that cannot be reached is still "unavailable".
func TestFileConnectNamesAnUnreachableGateway(t *testing.T) {
	f := newFileConnectFixture(t, "id-a", nil, nil)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := "http://" + listener.Addr().String() + "/sandbox-files"
	_ = listener.Close()
	f.svc.Exec.FileGatewayURL = closed
	status, message := readFileError(t, redeemFile(t, f.mint(t, "download", "/workspace/out.txt"), nil))
	if status != http.StatusServiceUnavailable || message != "sandbox file gateway unavailable" {
		t.Fatalf("unreachable gateway = %d %q", status, message)
	}
}

// Through the real gateway and its scripts, a directory download refused
// before its first byte (here, an entry name with a newline) answers its cause
// on a reused upstream connection: never 409 or 503 (w32/m380 t003).
func TestFileGatewayRefusalBeforeTheFirstByteReachesTheClient(t *testing.T) {
	// The local executor runs the gateway's shell scripts with the host's
	// tar. macOS bsdtar archives the directory with AppleDouble and xattr
	// entries the sandbox's tar never writes, so the stream ends differently
	// there; the sandbox image (and CI, on Linux) is what this pins.
	if runtime.GOOS == "darwin" {
		t.Skip("needs the sandbox's tar; macOS bsdtar adds AppleDouble/xattr entries")
	}
	f := newFileRoundtripFixture(t)
	root := fileRoundtripTempDir(t)
	if err := os.WriteFile(filepath.Join(root, "plain.txt"), []byte("plain"), 0o600); err != nil {
		t.Fatal(err)
	}
	if download := redeemFile(t, f.mint(t, "download", filepath.Join(root, "plain.txt")), nil); download.StatusCode != http.StatusOK {
		t.Fatalf("warm-up download = %d", download.StatusCode)
	}
	refused := filepath.Join(root, "refused")
	if err := os.MkdirAll(refused, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refused, "a\nb"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	status, message := readFileError(t, redeemFile(t, f.mint(t, "download", refused), nil))
	if status != http.StatusBadRequest || message != "invalid or unsupported sandbox archive: unsafe entry" {
		t.Fatalf("refused download = %d %q", status, message)
	}
}
