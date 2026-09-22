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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/gatewaytest"
	filegateway "github.com/bex-co/bex/lego/backend/internal/sshgateway/sandboxfiles"
)

// localFileExecutor runs the gateway's actual fixed programs on disposable
// local paths. It substitutes only pods/exec, not the file-transfer protocol or
// archive processing. This is offline behavioral coverage, never a live Pod test.
type localFileExecutor struct {
	env            []string
	failAfterWrite atomic.Bool
}

func (e *localFileExecutor) Execute(ctx context.Context, _ apps.SSHInstanceTarget, argv []string, _ bool, _ remotecommand.TerminalSizeQueue, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = e.env, stdin, stdout, stderr
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if err == nil && e.failAfterWrite.Load() && stdout != nil {
		return 1, nil // complete output followed by a failed remote process
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 0, err
}

// GNU/Alpine mv -T refuses to follow a destination link. macOS mv lacks -T;
// the local test executor maps that one operation to the same rename syscall.
// The production scripts and Linux tests still run native mv unchanged.
func TestFileRoundtripRenameHelper(t *testing.T) {
	if os.Getenv("BEX_TEST_FILE_RENAME_HELPER") != "1" {
		return
	}
	args := os.Args[len(os.Args)-2:]
	if err := os.Rename(args[0], args[1]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func fileRoundtripTempDir(t *testing.T) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func newLocalFileExecutor(t *testing.T) *localFileExecutor {
	t.Helper()
	e := &localFileExecutor{env: os.Environ()}
	if runtime.GOOS != "darwin" {
		return e
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin := fileRoundtripTempDir(t)
	shim := "#!/bin/sh\nexec \"$BEX_TEST_FILE_EXEC_BINARY\" -test.run=^TestFileRoundtripRenameHelper$ -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "mv"), []byte(shim), 0o700); err != nil {
		t.Fatal(err)
	}
	e.env = append(e.env, "PATH="+bin+":"+os.Getenv("PATH"), "BEX_TEST_FILE_EXEC_BINARY="+binary, "BEX_TEST_FILE_RENAME_HELPER=1")
	return e
}

type fileRoundtripContract struct {
	Workspace string `json:"workspace"`
	SandboxID string `json:"sandboxId"`
	Connect   struct {
		Method       string   `json:"method"`
		PathTemplate string   `json:"pathTemplate"`
		Status       int      `json:"status"`
		Fields       []string `json:"fields"`
	} `json:"connect"`
	BexServerPolicy struct {
		MaxTransferBytes       int64 `json:"maxTransferBytes"`
		MaxArchiveEntries      int   `json:"maxArchiveEntries"`
		MaxPathBytes           int   `json:"maxPathBytes"`
		MaxPathDepth           int   `json:"maxPathDepth"`
		TransferTimeoutSeconds int   `json:"transferTimeoutSeconds"`
	} `json:"bexServerPolicy"`
}

type fileRoundtripFixture struct {
	api      *httptest.Server
	boxID    string
	executor *localFileExecutor
	contract fileRoundtripContract
}

func newFileRoundtripFixture(t *testing.T) *fileRoundtripFixture {
	t.Helper()
	f := &fileRoundtripFixture{boxID: id.New(id.Sandbox), executor: newLocalFileExecutor(t)}
	golden, err := os.ReadFile(filepath.Join("..", "..", "..", "cli", "testdata", "sandbox-file-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(golden, &f.contract); err != nil {
		t.Fatal(err)
	}
	policy := f.contract.BexServerPolicy
	if policy.MaxTransferBytes != sandboxfiles.MaxTransferBytes || policy.MaxArchiveEntries != sandboxfiles.MaxArchiveEntries ||
		policy.MaxPathBytes != sandboxfiles.MaxPathBytes || policy.MaxPathDepth != sandboxfiles.MaxPathDepth ||
		time.Duration(policy.TransferTimeoutSeconds)*time.Second != sandboxfiles.DefaultTransferTimeout {
		t.Fatalf("shared CLI file contract limits drifted from gateway constants: %+v", policy)
	}
	secret := []byte("offline-file-roundtrip-secret")
	nonces := &gatewaytest.FakeStore{}
	gateway := httptest.NewServer((&filegateway.Server{
		Secret: secret, Executor: f.executor,
		Metrics: sshgateway.NewMetrics(prometheus.NewRegistry()),
		Limits:  sshgateway.NewSessionLimiter(10, 5),
		Nonces:  &sshgateway.NonceGuard{Store: nonces},
	}).Handler())
	t.Cleanup(gateway.Close)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := osSandboxJSON(f.contract.SandboxID, f.boxID)
		switch r.URL.Path {
		case "/sandboxes":
			_, _ = io.WriteString(w, "["+body+"]")
		case "/sandboxes/" + f.contract.SandboxID:
			_, _ = io.WriteString(w, body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)
	svc := &Service{
		Base:   &core.Base{Namespace: "default", Workspace: fakeWorkspace{"id-a": f.contract.Workspace}},
		Client: NewClient(upstream.URL),
		Exec:   &ExecConfig{Secret: secret, FileGatewayURL: gateway.URL + sandboxfiles.GatewayPath, Client: gateway.Client(), Nonces: &sshgateway.NonceGuard{Store: nonces}},
	}
	gated := http.NewServeMux()
	svc.RegisterREST(gated)
	root := http.NewServeMux()
	root.Handle("/v1/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gated.ServeHTTP(w, r.WithContext(identityCtx("id-a")))
	}))
	root.Handle(ConnectFileUploadPattern, svc.ConnectFileHandler())
	root.Handle(ConnectFileDownloadPattern, svc.ConnectFileHandler())
	f.api = httptest.NewServer(root)
	t.Cleanup(f.api.Close)
	return f
}

func (f *fileRoundtripFixture) mint(t *testing.T, operation, path string) ConnectResponse {
	t.Helper()
	route := strings.NewReplacer("{sandboxId}", f.boxID, "{operation}", operation).Replace(f.contract.Connect.PathTemplate)
	req, err := http.NewRequest(f.contract.Connect.Method, f.api.URL+route+"?"+url.Values{"ownerId": {f.contract.Workspace}, "path": {path}}.Encode(), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := f.api.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != f.contract.Connect.Status {
		t.Fatalf("mint status=%d body=%s err=%v", resp.StatusCode, body, err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	for _, field := range f.contract.Connect.Fields {
		if len(fields[field]) == 0 {
			t.Errorf("mint omitted contract field %q", field)
		}
	}
	var out ConnectResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFileGatewayHTTPRoundtrip(t *testing.T) {
	f := newFileRoundtripFixture(t)
	for _, data := range [][]byte{{}, {0, 1, 255, '\n'}, bytes.Repeat([]byte("bounded streaming data\n"), 10000)} {
		t.Run(fmt.Sprintf("%d-bytes", len(data)), func(t *testing.T) {
			remote := filepath.Join(fileRoundtripTempDir(t), "remote.bin")
			upload := redeemFile(t, f.mint(t, "upload", remote), bytes.NewReader(data))
			body, err := io.ReadAll(upload.Body)
			if err != nil || upload.StatusCode < 200 || upload.StatusCode >= 300 {
				t.Fatalf("upload status=%d body=%s err=%v", upload.StatusCode, body, err)
			}
			onDisk, err := os.ReadFile(remote)
			if err != nil || !bytes.Equal(onDisk, data) {
				t.Fatalf("upload bytes differ: got %d err=%v", len(onDisk), err)
			}
			download := redeemFile(t, f.mint(t, "download", remote), nil)
			got, err := io.ReadAll(download.Body)
			if err != nil || download.StatusCode != http.StatusOK || !bytes.Equal(got, data) {
				t.Fatalf("download status=%d got %d bytes err=%v", download.StatusCode, len(got), err)
			}
		})
	}
}

func TestFileGatewayPropagatesLateExecutorFailure(t *testing.T) {
	f := newFileRoundtripFixture(t)
	for _, isDirectory := range []bool{false, true} {
		t.Run(fmt.Sprintf("directory=%t", isDirectory), func(t *testing.T) {
			root := fileRoundtripTempDir(t)
			path := filepath.Join(root, "complete.txt")
			if err := os.WriteFile(path, fileRoundtripPayload(), 0o600); err != nil {
				t.Fatal(err)
			}
			if isDirectory {
				path = root
			}
			f.executor.failAfterWrite.Store(true)
			defer f.executor.failAfterWrite.Store(false)
			response := redeemFile(t, f.mint(t, "download", path), nil)
			_, err := io.ReadAll(response.Body)
			if err == nil && response.StatusCode >= 200 && response.StatusCode < 300 {
				t.Fatal("complete bytes followed by executor failure became a successful HTTP download")
			}
		})
	}
}

// Deterministic incompressible bytes force gzip and HTTP to emit real body
// chunks before the executor fails; a tiny repeated string could stay buffered.
func fileRoundtripPayload() []byte {
	data := make([]byte, 80_000)
	state := uint32(0x9e3779b9)
	for i := range data {
		state ^= state << 13
		state ^= state >> 17
		state ^= state << 5
		data[i] = byte(state)
	}
	return data
}

// Build lego/cli and set BEX_TEST_CLI_BINARY to run the distributed command
// against the real API+gateway fixture. The modules never import one another.
func TestPinnedCLISandboxFileRoundtrip(t *testing.T) {
	binary := os.Getenv("BEX_TEST_CLI_BINARY")
	if binary == "" {
		t.Skip("set BEX_TEST_CLI_BINARY to a built lego/cli launcher for the offline process roundtrip")
	}
	f := newFileRoundtripFixture(t)
	config := fileRoundtripTempDir(t)
	run := func(src, dst string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, "ea", "sandboxes", "copy", src, dst, "--output", "json")
		for _, item := range os.Environ() {
			if !strings.HasPrefix(item, "RENDER_") && !strings.HasPrefix(item, "BEX_") {
				cmd.Env = append(cmd.Env, item)
			}
		}
		cmd.Env = append(cmd.Env, "BEX_CLI_CONFIG_DIR="+config, "BEX_HOST="+f.api.URL+"/v1/", "BEX_WORKSPACE="+f.contract.Workspace, "BEX_ACCESS_TOKEN=file-fixture-token", "BEX_CLI_DISABLE_ANALYTICS=1")
		return cmd.CombinedOutput()
	}
	for _, fixture := range []struct {
		name      string
		directory bool
		empty     bool
	}{{name: "binary"}, {name: "empty", empty: true}, {name: "directory", directory: true}} {
		t.Run(fixture.name, func(t *testing.T) {
			directory := fixture.directory
			source := filepath.Join(fileRoundtripTempDir(t), "source")
			payload := fileRoundtripPayload()
			if fixture.empty {
				payload = nil
			}
			file := source
			if directory {
				file = filepath.Join(source, "nested", "binary ü.dat")
				if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(file, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			remote := filepath.Join(fileRoundtripTempDir(t), "copied")
			if output, err := run(source, f.boxID+":"+remote); err != nil {
				t.Fatalf("CLI upload: %v\n%s", err, output)
			}
			if directory {
				if output, err := run(source, f.boxID+":"+remote); err == nil {
					t.Fatalf("CLI merged an upload into an existing directory: %s", output)
				}
			}
			dest := filepath.Join(fileRoundtripTempDir(t), "downloaded")
			if output, err := run(f.boxID+":"+remote, dest); err != nil {
				t.Fatalf("CLI download: %v\n%s", err, output)
			}
			gotFile := dest
			if directory {
				gotFile = filepath.Join(dest, "nested", "binary ü.dat")
			}
			got, err := os.ReadFile(gotFile)
			if err != nil || !bytes.Equal(got, payload) {
				t.Fatalf("CLI roundtrip got %d bytes err=%v", len(got), err)
			}
			f.executor.failAfterWrite.Store(true)
			output, err := run(f.boxID+":"+remote, dest)
			f.executor.failAfterWrite.Store(false)
			if err == nil {
				t.Fatalf("CLI reported success after gateway process failed: %s", output)
			}
			if !directory {
				got, err = os.ReadFile(gotFile)
				if err != nil || !bytes.Equal(got, payload) {
					t.Fatalf("failed CLI download replaced original destination: err=%v", err)
				}
			}
		})
	}
	if output, err := run(f.boxID+":"+filepath.Join(fileRoundtripTempDir(t), "missing"), filepath.Join(fileRoundtripTempDir(t), "dest")); err == nil {
		t.Fatalf("missing source reported success: %s", output)
	}
}
