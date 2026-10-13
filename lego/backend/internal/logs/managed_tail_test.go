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

	"github.com/gorilla/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/083: a managed datastore's live tail is unsupported, but the refusal ran
// after the WebSocket 101, so the pinned CLI read a closed socket as a
// finished tail and exited 0. Every transport now gets the named 400 before
// any streaming header, alone or beside a supported App.
func TestSubscribeRefusesManagedLiveTailsBeforeStreaming(t *testing.T) {
	newManaged := func() *Service {
		return newService(map[string][]string{"web-1": {"2026-07-05T00:00:01Z from web"}},
			sampleApp("web"), podFor("web", "web-1"), sampleDatabase(postgresID),
			&appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: keyValueID, Namespace: "default"}})
	}
	for _, tc := range []struct{ query, family string }{
		{"resource=" + postgresID, "Postgres"},
		{"resource=" + keyValueID, "Key Value"},
		{"resource=web&resource=" + keyValueID, "Key Value"},
		{"resource=" + postgresID + "&resource=web", "Postgres"},
	} {
		want := "managed " + tc.family + " live log subscription is not supported; use the historical logs query"
		for _, accept := range []string{"text/event-stream", ""} {
			mux := http.NewServeMux()
			newManaged().RegisterREST(mux)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/logs/subscribe?"+tc.query, nil)
			if accept != "" {
				req.Header.Set("Accept", accept)
			}
			serveSubscribe(mux, rec, req)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), want) ||
				strings.Contains(rec.Body.String(), "from web") || rec.Header().Get("Content-Type") == "text/event-stream" {
				t.Errorf("accept %q, %s = %d %q, want the named 400 before streaming", accept, tc.query, rec.Code, rec.Body.String())
			}
		}

		srv := httptest.NewServer(func() http.Handler {
			mux := http.NewServeMux()
			newManaged().RegisterREST(mux)
			return mux
		}())
		conn, resp, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/logs/subscribe?"+tc.query, nil)
		if err == nil {
			_ = conn.Close()
			t.Errorf("WS %s upgraded, want a refused handshake", tc.query)
		} else if resp == nil || resp.StatusCode != http.StatusBadRequest {
			t.Errorf("WS %s handshake = %v / %+v, want HTTP 400", tc.query, err, resp)
		}
		srv.Close()
	}
}
