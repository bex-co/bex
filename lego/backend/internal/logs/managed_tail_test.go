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

package logs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The datastores' canned process logs: a CNPG record (kept, unwrapped) beside
// its instance manager's own JSON (dropped), and Valkey lines whose markers
// carry their levels beside the start-up banner (no level).
var managedTailLogs = map[string][]string{
	"web-1": {"2026-07-05T00:00:01Z from web"},
	postgresID + "-1": {
		`2026-07-05T00:00:02Z {"level":"info","logger":"instance-manager","msg":"Starting EventSource"}`,
		`2026-07-05T00:00:03Z {"logger":"postgres","msg":"record","record":{"log_time":"2026-07-05 00:00:03 UTC","process_id":"7","error_severity":"FATAL","message":"password authentication failed"}}`,
	},
	keyValueID + "-0": {
		"2026-07-05T00:00:04Z                 Valkey 8.1.1 (00000000/0) 64 bit",
		"2026-07-05T00:00:05Z 1:M 05 Jul 2026 00:00:05.000 * Ready to accept connections tcp",
		"2026-07-05T00:00:06Z 1:M 05 Jul 2026 00:00:06.000 # WARNING Memory overcommit must be enabled!",
	},
}

func newManagedTailService() *Service {
	return newService(managedTailLogs,
		sampleApp("web"), podFor("web", "web-1"),
		sampleDatabase(postgresID), databasePod(postgresID, postgresID+"-1"),
		&appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: keyValueID, Namespace: "default"}},
		keyValuePod(keyValueID, keyValueID+"-0"))
}

// w5/m139: Render's /logs/subscribe takes a postgres or redis resource as it
// takes a service, so `render logs -r dpg-…/red-… --tail` streams the
// datastore's process log — parsed as the historical read parses it — alone or
// beside an App, on every transport.
func TestSubscribeTailsManagedDatastores(t *testing.T) {
	const record = "password authentication failed"
	for _, tc := range []struct {
		query        string
		want, absent []string
	}{
		{"resource=" + postgresID, []string{record}, []string{"Starting EventSource", "from web"}},
		{"resource=" + keyValueID, []string{"Ready to accept connections", "Memory overcommit", "Valkey 8.1.1"}, []string{"from web"}},
		{"resource=" + keyValueID + "&level=warning", []string{"Memory overcommit"}, []string{"Ready to accept connections", "Valkey 8.1.1"}},
		{"resource=web&resource=" + postgresID, []string{"from web", record}, []string{"Starting EventSource"}},
	} {
		for _, accept := range []string{"text/event-stream", ""} {
			mux := http.NewServeMux()
			newManagedTailService().RegisterREST(mux)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?"+tc.query, nil)
			if accept != "" {
				req.Header.Set("Accept", accept)
			}
			serveSubscribe(mux, rec, req)
			body := rec.Body.String()
			if rec.Code != http.StatusOK {
				t.Errorf("accept %q, %s = %d %q, want a stream", accept, tc.query, rec.Code, body)
				continue
			}
			for _, w := range tc.want {
				if !strings.Contains(body, w) {
					t.Errorf("accept %q, %s: stream lacks %q:\n%s", accept, tc.query, w, body)
				}
			}
			for _, a := range tc.absent {
				if strings.Contains(body, a) {
					t.Errorf("accept %q, %s: stream carries %q:\n%s", accept, tc.query, a, body)
				}
			}
		}
	}

	// The pinned CLI tails over WebSocket: a Key Value line arrives as a frame
	// carrying its level label.
	srv := httptest.NewServer(func() http.Handler {
		mux := http.NewServeMux()
		newManagedTailService().RegisterREST(mux)
		return mux
	}())
	defer srv.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?resource="+keyValueID+"&level=warning", nil)
	if err != nil {
		t.Fatalf("WS dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, frame, err := conn.ReadMessage()
	if err != nil || !strings.Contains(string(frame), "Memory overcommit") || !strings.Contains(string(frame), `"value":"warning"`) {
		t.Fatalf("WS frame = %s, err=%v; want the warning line with its level", frame, err)
	}
}

// A datastore tail is refused exactly as its historical read is, before any
// streaming header or WebSocket 101: a caller the datastore's workspace denies
// gets 403, and an app-only filter (level beside an App, or an HTTP filter on a
// datastore) gets the named 400 — the pinned CLI reads a closed socket as a
// finished tail (w8/083).
func TestSubscribeRefusesManagedTailsBeforeStreaming(t *testing.T) {
	for _, tc := range []struct {
		query  string
		deny   bool
		status int
	}{
		{"resource=" + postgresID, true, http.StatusForbidden},
		{"resource=" + keyValueID, true, http.StatusForbidden},
		{"resource=" + keyValueID + "&method=GET", false, http.StatusBadRequest},
		{"resource=" + keyValueID + "&level=warning&resource=web", false, http.StatusBadRequest},
	} {
		for _, accept := range []string{"text/event-stream", ""} {
			svc := newManagedTailService()
			if tc.deny {
				svc.Base.Authz = denyLogChecker{}
			}
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?"+tc.query, nil)
			if accept != "" {
				req.Header.Set("Accept", accept)
			}
			serveSubscribe(mux, rec, req)
			if rec.Code != tc.status || rec.Header().Get("Content-Type") == "text/event-stream" ||
				strings.Contains(rec.Body.String(), "password authentication") || strings.Contains(rec.Body.String(), "Memory overcommit") {
				t.Errorf("accept %q, %s = %d %q, want %d before streaming", accept, tc.query, rec.Code, rec.Body.String(), tc.status)
			}
		}
	}
}
