package main

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// dial opens a WebSocket to the test server with the given query.
func dial(t *testing.T, srv *httptest.Server, query string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws"+query, nil)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func eventually(t *testing.T, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatal("condition never held")
}

// Silent mode: the client's frames arrive and are counted, and the server
// writes nothing back — not even a control frame, since the client sends no
// ping.
func TestSilentModeReceivesAndNeverWrites(t *testing.T) {
	st := &stats{}
	srv := httptest.NewServer(newServer(st, time.Now()))
	defer srv.Close()
	conn := dial(t, srv, "")
	defer conn.Close()
	for range 3 {
		if err := conn.WriteMessage(websocket.TextMessage, []byte("hi")); err != nil {
			t.Fatal(err)
		}
	}
	eventually(t, func() bool { return st.ClientMessages.Load() == 3 })

	_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, msg, err := conn.ReadMessage(); err == nil {
		t.Fatalf("silent server sent %q", msg)
	}
	if st.ServerMessages.Load() != 0 || st.ServerPongs.Load() != 0 {
		t.Fatalf("silent server wrote frames: %+v", st.snapshot(time.Now()))
	}
}

// The server-sending control writes on its own schedule with no client input.
func TestSendModeIsTheServerSendingControl(t *testing.T) {
	st := &stats{}
	srv := httptest.NewServer(newServer(st, time.Now()))
	defer srv.Close()
	conn := dial(t, srv, "?mode=send&every=20ms")
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, msg, err := conn.ReadMessage(); err != nil || string(msg) != "server tick" {
		t.Fatalf("send mode: %q, %v", msg, err)
	}
	if st.ClientMessages.Load() != 0 {
		t.Fatalf("the control counted client traffic it never received")
	}
}

// A client ping is answered with a pong — server→client control traffic — and
// both are counted, so it can never hide inside a "silent" result.
func TestControlFramesAreCountedApart(t *testing.T) {
	st := &stats{}
	srv := httptest.NewServer(newServer(st, time.Now()))
	defer srv.Close()
	conn := dial(t, srv, "")
	defer conn.Close()
	if err := conn.WriteControl(websocket.PingMessage, []byte("p"), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	go func() { _, _, _ = conn.ReadMessage() }() // processes the pong
	eventually(t, func() bool { return st.ServerPongs.Load() == 1 })
	if st.ClientPings.Load() != 1 || st.ClientMessages.Load() != 0 || st.ServerMessages.Load() != 0 {
		t.Fatalf("control frames leaked into application counters: %+v", st.snapshot(time.Now()))
	}
}
