package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/sandbox"
)

// sandboxExecContract mirrors lego/cli/testdata/sandbox-exec-contract.json,
// the golden bex-api's handlers are proven to emit byte for byte
// (lego/backend/internal/sandbox/connect_test.go). Driving the PINNED
// pkg/sandbox.Repo against it here makes a pin bump that changes the exec
// transport — the connect path, the SSE key names, the bearer handshake —
// fail this test until the contract (and bex-api) move with it (w7/m147).
type sandboxExecContract struct {
	Workspace string `json:"workspace"`
	SandboxID string `json:"sandboxId"`
	Command   string `json:"command"`
	Connect   struct {
		Method      string   `json:"method"`
		Path        string   `json:"path"`
		Query       string   `json:"query"`
		Status      int      `json:"status"`
		Fields      []string `json:"fields"`
		RespMethod  string   `json:"responseMethod"`
		URIPath     string   `json:"uriPath"`
		RequestBody struct {
			Command string `json:"command"`
		} `json:"requestBody"`
	} `json:"connect"`
	Stream struct {
		Headers     map[string]string `json:"headers"`
		Status      int               `json:"status"`
		ContentType string            `json:"contentType"`
		RequestBody struct {
			Command string `json:"command"`
		} `json:"requestBody"`
	} `json:"stream"`
	Transcripts struct {
		Exit7      string `json:"exit7"`
		Terminated string `json:"terminated"`
	} `json:"transcripts"`
	Expected struct {
		Stdout          string `json:"stdout"`
		Stderr          string `json:"stderr"`
		ExitCode        int    `json:"exitCode"`
		TerminatedError string `json:"terminatedError"`
	} `json:"expected"`
}

type recordedRequest struct {
	method, path, query, body string
	header                    http.Header
	contentLength             int64
}

// contractBexServer replays the golden as bex-api would: it answers the
// connect mint with a token and a uri on its own origin, then serves the
// chosen transcript at that uri once the token comes back as the Bearer.
func contractBexServer(t *testing.T, c sandboxExecContract, transcript string) (*httptest.Server, *sync.Map) {
	t.Helper()
	var seen sync.Map
	const executionID = "exe-d3b7ed3eqa1c73btqk80"
	const token = "connect-token-fixture"
	mux := http.NewServeMux()
	var base string
	record := func(name string, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen.Store(name, recordedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: string(body), header: r.Header.Clone()})
	}
	mux.HandleFunc(c.Connect.Path, func(w http.ResponseWriter, r *http.Request) {
		record("connect", r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(c.Connect.Status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"executionId": executionID,
			"expiresAt":   time.Now().Add(time.Minute).UTC().Format(time.RFC3339),
			"method":      c.Connect.RespMethod,
			"token":       token,
			"uri":         base + strings.ReplaceAll(c.Connect.URIPath, "{executionId}", executionID),
		})
	})
	mux.HandleFunc(strings.ReplaceAll(c.Connect.URIPath, "{executionId}", executionID), func(w http.ResponseWriter, r *http.Request) {
		record("stream", r)
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", c.Stream.ContentType)
		w.WriteHeader(c.Stream.Status)
		_, _ = io.WriteString(w, transcript)
	})
	srv := httptest.NewServer(mux)
	base = srv.URL
	t.Cleanup(srv.Close)
	return srv, &seen
}

func loadSandboxExecContract(t *testing.T) sandboxExecContract {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "sandbox-exec-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c sandboxExecContract
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPinnedSandboxExecTransportMatchesBexContract(t *testing.T) {
	c := loadSandboxExecContract(t)
	t.Setenv("RENDER_WORKSPACE", c.Workspace)
	srv, seen := contractBexServer(t, c, c.Transcripts.Exit7)
	api, err := client.NewClientWithResponses(srv.URL + "/v1/")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	code, err := sandbox.NewRepo(api).ExecSandboxStream(context.Background(), c.SandboxID, c.Command, func(ev *sandbox.ExecOutputEvent) error {
		if ev.Stream == sandbox.ExecOutputStreamStderr {
			stderr.WriteString(ev.Data)
		} else {
			stdout.WriteString(ev.Data)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ExecSandboxStream: %v", err)
	}
	if code != c.Expected.ExitCode || stdout.String() != c.Expected.Stdout || stderr.String() != c.Expected.Stderr {
		t.Fatalf("exit=%d stdout=%q stderr=%q, want %d/%q/%q — the pinned decoder no longer reads bex's exit/output shapes",
			code, stdout.String(), stderr.String(), c.Expected.ExitCode, c.Expected.Stdout, c.Expected.Stderr)
	}

	// Step one: the connect mint bex-api serves at the gated token route.
	got, ok := seen.Load("connect")
	if !ok {
		t.Fatal("the pinned client never called the connect route the contract names")
	}
	connect := got.(recordedRequest)
	if connect.method != c.Connect.Method || connect.path != c.Connect.Path || connect.query != c.Connect.Query {
		t.Errorf("connect request = %s %s?%s, want %s %s?%s", connect.method, connect.path, connect.query, c.Connect.Method, c.Connect.Path, c.Connect.Query)
	}
	var connectBody map[string]string
	if json.Unmarshal([]byte(connect.body), &connectBody) != nil || connectBody["command"] != c.Connect.RequestBody.Command {
		t.Errorf("connect body = %s, want command %q", connect.body, c.Connect.RequestBody.Command)
	}
	// Step two: the redeem at bex's uri, authenticated by the connect token.
	got, ok = seen.Load("stream")
	if !ok {
		t.Fatal("the pinned client never redeemed the connect token at the returned uri")
	}
	stream := got.(recordedRequest)
	if stream.method != c.Connect.RespMethod {
		t.Errorf("stream method = %s, want %s", stream.method, c.Connect.RespMethod)
	}
	for name, want := range c.Stream.Headers {
		want = strings.ReplaceAll(want, "{token}", "connect-token-fixture")
		if got := stream.header.Get(name); got != want {
			t.Errorf("stream header %s = %q, want %q", name, got, want)
		}
	}
	var streamBody map[string]string
	if json.Unmarshal([]byte(stream.body), &streamBody) != nil || streamBody["command"] != c.Stream.RequestBody.Command {
		t.Errorf("stream body = %s, want command %q", stream.body, c.Stream.RequestBody.Command)
	}
}

func TestPinnedSandboxExecSurfacesBexErrorEvents(t *testing.T) {
	c := loadSandboxExecContract(t)
	t.Setenv("RENDER_WORKSPACE", c.Workspace)
	srv, _ := contractBexServer(t, c, c.Transcripts.Terminated)
	api, err := client.NewClientWithResponses(srv.URL + "/v1/")
	if err != nil {
		t.Fatal(err)
	}
	_, err = sandbox.NewRepo(api).ExecSandboxStream(context.Background(), c.SandboxID, c.Command, nil)
	if err == nil || err.Error() != c.Expected.TerminatedError {
		t.Fatalf("terminated-target error = %v, want %q — the pinned decoder no longer reads bex's {status,message}", err, c.Expected.TerminatedError)
	}
}

// sandboxFileContract mirrors lego/cli/testdata/sandbox-file-contract.json.
// Same purpose as the exec contract above, for the file transport w7/m150
// implements: drive the PINNED pkg/sandbox.Repo and fail if a pin bump moves
// the connect route, the query shape, the bearer handshake, or the streamed
// body — before bex-api is built against a contract that has already changed.
type sandboxFileContract struct {
	Workspace  string `json:"workspace"`
	SandboxID  string `json:"sandboxId"`
	RemotePath string `json:"remotePath"`
	Connect    struct {
		Method       string   `json:"method"`
		PathTemplate string   `json:"pathTemplate"`
		Operations   []string `json:"operations"`
		Query        string   `json:"query"`
		Status       int      `json:"status"`
		Fields       []string `json:"fields"`
	} `json:"connect"`
	Upload struct {
		DirectoryContentType     string `json:"directoryContentType"`
		DirectoryContentEncoding string `json:"directoryContentEncoding"`
	} `json:"upload"`
}

func loadSandboxFileContract(t *testing.T) sandboxFileContract {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "sandbox-file-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c sandboxFileContract
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// fileContractServer answers the file connect mint, then serves the transfer
// at the uri it returned — the same two-step handshake bex-api must implement.
func fileContractServer(t *testing.T, c sandboxFileContract, operation string, payload []byte) (*httptest.Server, *sync.Map) {
	return fileContractServerWithTransfer(t, c, operation, func(w http.ResponseWriter, _ *http.Request) {
		if operation == "download" {
			_, _ = w.Write(payload)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}

func fileContractServerWithTransfer(t *testing.T, c sandboxFileContract, operation string, transfer http.HandlerFunc) (*httptest.Server, *sync.Map) {
	t.Helper()
	var seen sync.Map
	const token = "file-connect-token-fixture"
	const executionID = "exe-file-fixture"
	transferPath := "/v1/sandboxes/" + c.SandboxID + "/files/" + operation + "/" + executionID
	connectPath := strings.NewReplacer(
		"{sandboxId}", c.SandboxID, "{operation}", operation,
	).Replace(c.Connect.PathTemplate)

	var base string
	mux := http.NewServeMux()
	record := func(name string, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen.Store(name, recordedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: string(body), header: r.Header.Clone(), contentLength: r.ContentLength})
	}
	mux.HandleFunc(connectPath, func(w http.ResponseWriter, r *http.Request) {
		record("connect", r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(c.Connect.Status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"executionId": executionID,
			"expiresAt":   time.Now().Add(time.Minute).UTC().Format(time.RFC3339),
			"method":      map[string]string{"upload": http.MethodPut, "download": http.MethodGet}[operation],
			"token":       token,
			"uri":         base + transferPath,
		})
	})
	mux.HandleFunc(transferPath, func(w http.ResponseWriter, r *http.Request) {
		record("transfer", r)
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		transfer(w, r)
	})
	srv := httptest.NewServer(mux)
	base = srv.URL
	t.Cleanup(srv.Close)
	return srv, &seen
}

func TestPinnedSandboxFileTransportMatchesBexContract(t *testing.T) {
	c := loadSandboxFileContract(t)
	t.Setenv("RENDER_WORKSPACE", c.Workspace)

	local := filepath.Join(t.TempDir(), "payload.txt")
	want := []byte("contract payload\n")
	if err := os.WriteFile(local, want, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("upload mints at the contract route and streams under the bearer", func(t *testing.T) {
		srv, seen := fileContractServer(t, c, "upload", nil)
		api, err := client.NewClientWithResponses(srv.URL + "/v1/")
		if err != nil {
			t.Fatal(err)
		}
		svc := sandbox.NewService(sandbox.NewRepo(api))
		if err := svc.Upload(context.Background(), c.SandboxID, local, c.RemotePath); err != nil {
			t.Fatalf("Upload: %v", err)
		}

		got, ok := seen.Load("connect")
		if !ok {
			t.Fatal("the pinned client never called the file connect route the contract names")
		}
		connect := got.(recordedRequest)
		wantPath := strings.NewReplacer("{sandboxId}", c.SandboxID, "{operation}", "upload").Replace(c.Connect.PathTemplate)
		if connect.method != c.Connect.Method || connect.path != wantPath {
			t.Errorf("connect = %s %s, want %s %s", connect.method, connect.path, c.Connect.Method, wantPath)
		}
		// ownerId and path ride as query parameters, not as a body: the mint
		// carries no request body at all, which is what bex-api must accept.
		if !strings.Contains(connect.query, "ownerId="+c.Workspace) || !strings.Contains(connect.query, "path=") {
			t.Errorf("connect query = %q, want ownerId and path form parameters", connect.query)
		}
		if connect.body != "" {
			t.Errorf("connect body = %q, want empty — the pinned client sends none", connect.body)
		}

		got, ok = seen.Load("transfer")
		if !ok {
			t.Fatal("the pinned client never redeemed the file connect token at the returned uri")
		}
		transfer := got.(recordedRequest)
		if transfer.header.Get("Authorization") != "Bearer file-connect-token-fixture" {
			t.Errorf("transfer Authorization = %q, want the minted connect token as a bearer", transfer.header.Get("Authorization"))
		}
		if transfer.body != string(want) {
			t.Errorf("uploaded body = %q, want %q", transfer.body, want)
		}
	})

	t.Run("download redeems the token and writes the returned bytes", func(t *testing.T) {
		srv, seen := fileContractServer(t, c, "download", want)
		api, err := client.NewClientWithResponses(srv.URL + "/v1/")
		if err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(t.TempDir(), "downloaded.txt")
		svc := sandbox.NewService(sandbox.NewRepo(api))
		if _, err := svc.Download(context.Background(), c.SandboxID, c.RemotePath, dest); err != nil {
			t.Fatalf("Download: %v", err)
		}
		gotBytes, err := os.ReadFile(dest)
		if err != nil {
			t.Fatal(err)
		}
		if string(gotBytes) != string(want) {
			t.Errorf("downloaded = %q, want %q — byte-identical round trip is the contract", gotBytes, want)
		}
		if _, ok := seen.Load("transfer"); !ok {
			t.Fatal("the pinned client never redeemed the download token")
		}
	})
}

func TestPinnedSandboxDirectoryTransferContract(t *testing.T) {
	c := loadSandboxFileContract(t)
	t.Setenv("RENDER_WORKSPACE", c.Workspace)
	source := t.TempDir()
	want := map[string][]byte{
		"nested/binary.dat": {0, 1, 255, '\n', 0},
		"empty.txt":         {},
		"space ü.txt":       []byte("directory contract\n"),
	}
	for name, data := range want {
		path := filepath.Join(source, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("nested/binary.dat", filepath.Join(source, "relative-link")); err != nil {
		t.Fatal(err)
	}
	upload, seen := fileContractServer(t, c, "upload", nil)
	api, err := client.NewClientWithResponses(upload.URL + "/v1/")
	if err != nil {
		t.Fatal(err)
	}
	if err := sandbox.NewService(sandbox.NewRepo(api)).Upload(context.Background(), c.SandboxID, source, "/workspace/tree"); err != nil {
		t.Fatal(err)
	}
	recorded, ok := seen.Load("transfer")
	if !ok {
		t.Fatal("directory upload did not reach transfer")
	}
	transfer := recorded.(recordedRequest)
	if transfer.header.Get("Content-Type") != c.Upload.DirectoryContentType || transfer.header.Get("Content-Encoding") != c.Upload.DirectoryContentEncoding || transfer.contentLength != -1 {
		t.Fatalf("directory headers=%v length=%d, want x-tar/gzip with unknown length", transfer.header, transfer.contentLength)
	}
	download, _ := fileContractServerWithTransfer(t, c, "download", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", c.Upload.DirectoryContentType)
		w.Header().Set("Content-Encoding", c.Upload.DirectoryContentEncoding)
		_, _ = io.WriteString(w, transfer.body)
	})
	api, err = client.NewClientWithResponses(download.URL + "/v1/")
	if err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "downloaded")
	if _, err := sandbox.NewService(sandbox.NewRepo(api)).Download(context.Background(), c.SandboxID, "/workspace/tree", dest); err != nil {
		t.Fatal(err)
	}
	for name, data := range want {
		got, err := os.ReadFile(filepath.Join(dest, name))
		if err != nil || !bytes.Equal(got, data) {
			t.Errorf("round trip %s: bytes=%q err=%v, want %q", name, got, err, data)
		}
	}
	if link, err := os.Readlink(filepath.Join(dest, "relative-link")); err != nil || link != "nested/binary.dat" {
		t.Errorf("round-trip symlink=%q err=%v", link, err)
	}
}

func TestPinnedSandboxDownloadRejectsIncompleteSuccessBodies(t *testing.T) {
	c := loadSandboxFileContract(t)
	t.Setenv("RENDER_WORKSPACE", c.Workspace)
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	if err := tw.WriteHeader(&tar.Header{Name: "complete.txt", Mode: 0o600, Size: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(tw, "yes"); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	gzipBytes := func(payload []byte) []byte {
		var wire bytes.Buffer
		gz := gzip.NewWriter(&wire)
		if _, err := gz.Write(payload); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		return wire.Bytes()
	}
	fileGzip := gzipBytes([]byte("complete file bytes"))
	tarGzip := gzipBytes(archive.Bytes())
	for _, tc := range []struct {
		name, contentType, encoding string
		payload                     []byte
		contentLength               string
	}{
		{"raw short Content-Length", sandbox.FileContentTypeOctetStream, "", []byte("short"), "20"},
		{"raw complete bytes without gzip footer", sandbox.FileContentTypeOctetStream, "gzip", fileGzip[:len(fileGzip)-8], ""},
		{"tar stops on a block boundary before end marker", sandbox.FileContentTypeTar, "", archive.Bytes()[:archive.Len()-1024], ""},
		{"complete tar without gzip footer", sandbox.FileContentTypeTar, "gzip", tarGzip[:len(tarGzip)-8], ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := fileContractServerWithTransfer(t, c, "download", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				if tc.encoding != "" {
					w.Header().Set("Content-Encoding", tc.encoding)
				}
				if tc.contentLength != "" {
					w.Header().Set("Content-Length", tc.contentLength)
				}
				_, _ = w.Write(tc.payload)
			})
			api, err := client.NewClientWithResponses(srv.URL + "/v1/")
			if err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(t.TempDir(), "destination")
			if tc.contentType != sandbox.FileContentTypeTar {
				if err := os.WriteFile(dest, []byte("existing contents"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			_, err = sandbox.NewService(sandbox.NewRepo(api)).Download(context.Background(), c.SandboxID, c.RemotePath, dest)
			if err == nil {
				t.Fatal("truncated transfer reported success")
			}
			if tc.contentType != sandbox.FileContentTypeTar {
				got, err := os.ReadFile(dest)
				if err != nil || string(got) != "existing contents" {
					t.Errorf("failed download replaced existing file: %q err=%v", got, err)
				}
				partials, err := filepath.Glob(filepath.Join(filepath.Dir(dest), ".render-partial-*"))
				if err != nil || len(partials) != 0 {
					t.Errorf("failed download left partials=%v err=%v", partials, err)
				}
			}
		})
	}
}

func TestPinnedSandboxFileTransferReportsRefusals(t *testing.T) {
	c := loadSandboxFileContract(t)
	t.Setenv("RENDER_WORKSPACE", c.Workspace)
	for _, operation := range c.Connect.Operations {
		for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict, http.StatusRequestEntityTooLarge} {
			t.Run(operation+"/"+http.StatusText(status), func(t *testing.T) {
				srv, _ := fileContractServerWithTransfer(t, c, operation, func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(status)
					_, _ = io.WriteString(w, `{"message":"transfer refused"}`)
				})
				api, err := client.NewClientWithResponses(srv.URL + "/v1/")
				if err != nil {
					t.Fatal(err)
				}
				repo := sandbox.NewRepo(api)
				if operation == "upload" {
					err = repo.UploadFile(context.Background(), c.SandboxID, c.RemotePath, sandbox.FileContentTypeOctetStream, "", 0, strings.NewReader(""))
				} else {
					_, err = repo.DownloadFile(context.Background(), c.SandboxID, c.RemotePath)
				}
				switch status {
				case http.StatusUnauthorized:
					if !errors.Is(err, client.ErrUnauthorized) {
						t.Fatalf("error=%v, want unauthorized", err)
					}
				case http.StatusForbidden:
					if !errors.Is(err, client.ErrForbidden) {
						t.Fatalf("error=%v, want forbidden", err)
					}
				default:
					if err == nil || !strings.Contains(err.Error(), "transfer refused") {
						t.Fatalf("error=%v lost refusal message", err)
					}
				}
			})
		}
	}
}
