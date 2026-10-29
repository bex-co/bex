package main_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// The pinned client (Render CLI v2.29+) reconnects a `logs --tail` after a
// transient drop and resumes without replaying lines, and bex-api ends a tail
// it will not continue with close code 4000 (lego/backend/internal/logs
// CloseTailRefused), which this client treats as terminal (w8/m67).

const closeTailRefused = 4000 // mirrors bex-api; the CLI module imports no server package

func tailLine(id, ts, message string) []byte {
	b, _ := json.Marshal(map[string]any{
		"id": id, "timestamp": ts, "message": message,
		"labels": []map[string]string{{"name": "type", "value": "app"}, {"name": "resource", "value": "srv-abcdefghij0123456789"}},
	})
	return b
}

// startBexTail runs `bex logs --tail` against apiURL and returns its lines on
// a channel that closes when the process closes stdout.
func startBexTail(t *testing.T, apiURL string) (*exec.Cmd, *bytes.Buffer, <-chan string) {
	t.Helper()
	command := bexAgainst(t, apiURL, "logs", "-r", "srv-abcdefghij0123456789", "--tail", "-o", "text")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	pipe, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill() })
	lines := make(chan string, 64)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	return command, &stderr, lines
}

func TestLogsTailResumesAfterADropWithoutReplay(t *testing.T) {
	var connections atomic.Int32
	upgrader := websocket.Upgrader{}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/logs/subscribe") {
			http.NotFound(w, r)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		a := tailLine("log-a", "2026-10-29T00:00:01Z", "LINE-A")
		if connections.Add(1) == 1 {
			_ = conn.WriteMessage(websocket.TextMessage, a)
			_ = conn.UnderlyingConn().Close() // drop without a close frame
			return
		}
		_ = conn.WriteMessage(websocket.TextMessage, a) // the resume overlaps A
		_ = conn.WriteMessage(websocket.TextMessage, tailLine("log-b", "2026-10-29T00:00:02Z", "LINE-B"))
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(api.Close)

	command, stderr, lines := startBexTail(t, api.URL)
	var got []string
	timeout := time.After(20 * time.Second)
	for !slices.ContainsFunc(got, func(l string) bool { return strings.Contains(l, "LINE-B") }) {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatalf("tail exited after the drop; output:\n%s\nstderr:\n%s", strings.Join(got, "\n"), stderr)
			}
			got = append(got, line)
		case <-timeout:
			t.Fatalf("no LINE-B within 20s; output:\n%s\nstderr:\n%s", strings.Join(got, "\n"), stderr)
		}
	}
	// The server wrote A and B back to back on the second connection, so a
	// replayed A would already be in got; the process must still be following.
	if err := command.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("tail exited after resuming (%v); stderr:\n%s", err, stderr)
	}
	text := strings.Join(got, "\n")
	if strings.Count(text, "LINE-A") != 1 || strings.Count(text, "LINE-B") != 1 {
		t.Fatalf("want A and B exactly once, got:\n%s\nstderr:\n%s", text, stderr)
	}
	if connections.Load() != 2 {
		t.Errorf("connections = %d, want 2", connections.Load())
	}
	if !strings.Contains(stderr.String(), "Retrying") {
		t.Errorf("no visible retry status:\n%s", stderr)
	}
}

func TestLogsTailEndsOnBexsTerminalClose(t *testing.T) {
	var connections atomic.Int32
	const reason = "no build is running for this service"
	upgrader := websocket.Upgrader{}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		connections.Add(1)
		_ = conn.WriteMessage(websocket.TextMessage, tailLine("log-r", "2026-10-29T00:00:01Z", "==> "+reason))
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(closeTailRefused, reason), time.Now().Add(time.Second))
		_, _, _ = conn.ReadMessage()
	}))
	t.Cleanup(api.Close)

	command, stderr, lines := startBexTail(t, api.URL)
	var out strings.Builder
	timeout := time.After(15 * time.Second)
	for open := true; open; {
		select {
		case line, ok := <-lines:
			if ok {
				out.WriteString(line + "\n")
			}
			open = ok
		case <-timeout:
			t.Fatalf("tail kept running after a terminal close (connections=%d); stderr:\n%s", connections.Load(), stderr)
		}
	}
	if err := command.Wait(); err == nil {
		t.Errorf("tail exited 0 on a refused tail; want non-zero")
	}
	if connections.Load() != 1 {
		t.Errorf("connections = %d, want 1 (no retry)", connections.Load())
	}
	if strings.Count(out.String(), reason) != 1 || !strings.Contains(stderr.String(), reason) {
		t.Errorf("want the refusal once on stdout and the reason on stderr; stdout:\n%s\nstderr:\n%s", out.String(), stderr)
	}
}
