package main

import (
	"context"
	"encoding/json"
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
