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

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w9/m450: an audit row the caller's own request caused, on REST, GraphQL and
// MCP alike, records where they acted from: the client the trusted edge names
// (True-Client-IP through a Traefik pod, the production shape behind
// Cloudflare) and their User-Agent, as main.go composes the middleware.
func TestEverySurfaceRecordsWhereTheCallerActedFrom(t *testing.T) {
	const (
		traefikPod     = "10.244.3.17:52114"
		cloudflareEdge = "172.70.1.2"
		client         = "198.51.100.23"
		browser        = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/131.0.0.0 Safari/537.36"
	)
	proxies, err := core.ParseTrustedProxies("10.244.0.0/16")
	if err != nil {
		t.Fatal(err)
	}
	sink := &fakeAuditSink{}
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: &fakeChecker{allow: true}, Audit: sink}
	h := RequestOriginMiddleware(proxies, serverWithKratos(t, base, Deps{}, fakeHydraURL(t), fakeKratos(t).URL))

	call := func(peer, method, path, body string, headers map[string]string) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.RemoteAddr = peer
		for name, value := range headers {
			r.Header.Set(name, value)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code >= 300 || strings.Contains(w.Body.String(), `"errors"`) || strings.Contains(w.Body.String(), `"isError":true`) {
			t.Fatalf("%s %s = %d %s", method, path, w.Code, w.Body.String())
		}
	}
	edge := func(auth map[string]string) map[string]string {
		out := map[string]string{"Content-Type": "application/json", "User-Agent": browser, "X-Forwarded-For": cloudflareEdge, "True-Client-IP": client}
		for k, v := range auth {
			out[k] = v
		}
		return out
	}
	bearer := map[string]string{"Authorization": "Bearer " + testToken}
	session := map[string]string{"Cookie": "ory_kratos_session=live"}

	surfaces := []struct {
		name, method, path, body string
		headers                  map[string]string
		caller                   string
	}{
		{"REST with a key", http.MethodPost, "/v1/services/web/suspend", "", edge(bearer), ""},
		{"GraphQL from the dashboard", http.MethodPost, "/graphql", `{"query":"mutation { suspendService(id:\"web\") { id } }"}`, edge(session), "identity-1"},
		{"MCP with a key", http.MethodPost, "/mcp", `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"suspend_service","arguments":{"serviceId":"web"}}}`,
			edge(map[string]string{"Authorization": "Bearer " + testToken, "Accept": "application/json, text/event-stream"}), ""},
	}
	for _, s := range surfaces {
		t.Run(s.name, func(t *testing.T) {
			// Ensure the service is running before each suspend so every
			// surface executes a real state transition rather than a no-op.
			call(traefikPod, http.MethodPost, "/v1/services/web/resume", "", edge(bearer))
			sink.events = nil
			call(traefikPod, s.method, s.path, s.body, s.headers)
			if len(sink.events) == 0 {
				t.Fatal("the write recorded no audit row")
			}
			for _, ev := range sink.events {
				if ev.Caller == "" || (s.caller != "" && ev.Caller != s.caller) {
					t.Errorf("%s: caller %q, want the requester", ev.Verb, ev.Caller)
				}
				if ev.SourceIP != client || ev.UserAgent != browser {
					t.Errorf("%s: source %q agent %q, want %q and the browser's agent", ev.Verb, ev.SourceIP, ev.UserAgent, client)
				}
			}
		})
	}

	// A peer outside the trusted proxies (reaching bex-api past the edge) is
	// recorded as itself, whatever forwarding headers it sends.
	call(traefikPod, http.MethodPost, "/v1/services/web/resume", "", edge(bearer))
	sink.events = nil
	call("192.0.2.1:443", http.MethodPost, "/v1/services/web/suspend", "", edge(bearer))
	if len(sink.events) == 0 || sink.events[0].SourceIP != "192.0.2.1" {
		t.Fatalf("untrusted peer recorded %+v, want its own address", sink.events)
	}
}
