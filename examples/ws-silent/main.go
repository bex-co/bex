// Command ws-silent is the acceptance fixture for w1/m161: a WebSocket server
// that RECEIVES client frames and, by default, sends no application data back.
//
// An echo server cannot tell whether a platform counts client→server traffic as
// activity, because every client frame produces a server frame. This one keeps
// the two directions apart and counts each, so a live check can show that only
// the client spoke.
//
//	ws-silent                       serve on $PORT (default 3000)
//	ws-silent client -url URL ...   dial URL and send a text frame periodically
//
// Server endpoints:
//
//	GET /ws           WebSocket; silent by default, ?mode=send sends every
//	                  ?every= (default 10s) — the server-sending control
//	GET /stats        JSON counters since start (application and control
//	                  frames are counted separately)
//	GET /             one-line description (a plain HTTP probe)
//
// The idle control needs nothing from this program: deploy it and connect no
// client.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// stats are the server's counters. Control frames are tracked apart from
// application frames: a pong the server writes in reply to a client ping is
// server→client traffic too, and must be visible rather than hidden inside
// "silent".
type stats struct {
	Connections       atomic.Int64
	ClientMessages    atomic.Int64
	ServerMessages    atomic.Int64
	ClientPings       atomic.Int64
	ServerPongs       atomic.Int64
	LastClientMessage atomic.Int64 // unix seconds
}

func (s *stats) snapshot(started time.Time) map[string]any {
	last := ""
	if at := s.LastClientMessage.Load(); at > 0 {
		last = time.Unix(at, 0).UTC().Format(time.RFC3339)
	}
	return map[string]any{
		"startedAt":         started.UTC().Format(time.RFC3339),
		"connections":       s.Connections.Load(),
		"clientMessages":    s.ClientMessages.Load(),
		"serverMessages":    s.ServerMessages.Load(),
		"clientPings":       s.ClientPings.Load(),
		"serverPongs":       s.ServerPongs.Load(),
		"lastClientMessage": last,
	}
}

var upgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

func newServer(st *stats, started time.Time) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ws-silent: GET /ws (silent; ?mode=send for the server-sending control), GET /stats")
	})
	mux.HandleFunc("GET /stats", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(st.snapshot(started))
	})
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		every := 10 * time.Second
		if raw := r.URL.Query().Get("every"); raw != "" {
			d, err := time.ParseDuration(raw)
			if err != nil || d <= 0 {
				http.Error(w, "every must be a positive duration", http.StatusBadRequest)
				return
			}
			every = d
		}
		sending := r.URL.Query().Get("mode") == "send"
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		st.Connections.Add(1)
		conn.SetPingHandler(func(data string) error {
			st.ClientPings.Add(1)
			st.ServerPongs.Add(1)
			return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
		})
		done := make(chan struct{})
		if sending {
			go func() {
				t := time.NewTicker(every)
				defer t.Stop()
				for {
					select {
					case <-done:
						return
					case <-t.C:
						if conn.WriteMessage(websocket.TextMessage, []byte("server tick")) != nil {
							return
						}
						st.ServerMessages.Add(1)
					}
				}
			}()
		}
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
			st.ClientMessages.Add(1)
			st.LastClientMessage.Store(time.Now().Unix())
		}
	})
	return mux
}

// runClient dials url and sends one text frame every interval until duration
// elapses, logging every frame it receives. It sends no pings, so the only
// client→server traffic is application data.
func runClient(args []string) error {
	fs := flag.NewFlagSet("client", flag.ExitOnError)
	url := fs.String("url", "", "WebSocket URL, e.g. wss://svc.onbex.co/ws")
	every := fs.Duration("every", 30*time.Second, "send interval")
	duration := fs.Duration("duration", 30*time.Minute, "how long to keep sending")
	_ = fs.Parse(args)
	if *url == "" {
		return fmt.Errorf("-url is required")
	}
	conn, _, err := websocket.DefaultDialer.Dial(*url, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	var received atomic.Int64
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			received.Add(1)
			log.Printf("received %q (total %d)", msg, received.Load())
		}
	}()
	deadline := time.Now().Add(*duration)
	for sent := 1; time.Now().Before(deadline); sent++ {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("client %d", sent))); err != nil {
			return fmt.Errorf("send %d: %w", sent, err)
		}
		log.Printf("sent client %d (received so far %d)", sent, received.Load())
		time.Sleep(*every)
	}
	return nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "client" {
		if err := runClient(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		return
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	started := time.Now()
	log.Printf("ws-silent listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, newServer(&stats{}, started)))
}
