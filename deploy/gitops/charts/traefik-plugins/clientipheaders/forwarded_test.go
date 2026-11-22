// Copyright 2026.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bex_client_ip_headers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// forgedForwarded is every forwarded header a caller can send once the
// entrypoints trust them (w8/402), in both spellings.
func forgedForwarded() http.Header {
	return http.Header{
		"X-Forwarded-For":                  {"203.0.113.7"},
		"X-Forwarded-Proto":                {"gopher"},
		"X-Forwarded-Host":                 {"evil.example"},
		"X-Forwarded-Port":                 {"1"},
		"X-Forwarded-Server":               {"traefik-pod"},
		"X-Real-Ip":                        {"203.0.113.8"},
		"X-Forwarded-Uri":                  {"/admin"},
		"X-Forwarded-Method":               {"DELETE"},
		"X-Forwarded-Prefix":               {"/forged"},
		"X-Forwarded-Tls-Client-Cert":      {"forged"},
		"X-Forwarded-Tls-Client-Cert-Info": {"forged"},
		"X_forwarded_for":                  {"203.0.113.9"},
		"X_forwarded_proto":                {"https"},
		"X_forwarded_host":                 {"evil.example"},
		"X_real_ip":                        {"203.0.113.10"},
		"X_forwarded_server":               {"forged"},
	}
}

// w8/402: X-Forwarded-For passes as the caller sent it (Traefik's proxy then
// appends the client), and every other forwarded header reaches the app as
// Traefik set it for an untrusted caller, whatever the caller sent.
func TestForwardedHeaders(t *testing.T) {
	websocket := http.Header{"Connection": {"keep-alive, Upgrade"}, "Upgrade": {"websocket"}}
	for _, tc := range []struct {
		name, url, host, peer string
		header                http.Header
		want                  map[string]string // "" = absent
	}{
		{"forged over HTTPS", "https://web.onbex.co/", "", "198.51.100.20:51234", forgedForwarded(), map[string]string{
			"X-Forwarded-For": "203.0.113.7", "X-Forwarded-Proto": "https", "X-Forwarded-Host": "web.onbex.co",
			"X-Forwarded-Port": "443", "X-Forwarded-Server": "traefik-pod", "X-Real-Ip": "198.51.100.20",
			"X-Forwarded-Uri": "", "X-Forwarded-Method": "", "X-Forwarded-Prefix": "",
			"X-Forwarded-Tls-Client-Cert": "", "X-Forwarded-Tls-Client-Cert-Info": "",
			"X_forwarded_for": "", "X_forwarded_proto": "", "X_forwarded_host": "", "X_real_ip": "", "X_forwarded_server": "",
		}},
		{"forged over plain HTTP", "http://web.onbex.co/", "", "198.51.100.20:51234", forgedForwarded(), map[string]string{
			"X-Forwarded-For": "203.0.113.7", "X-Forwarded-Proto": "http", "X-Forwarded-Port": "80", "X-Real-Ip": "198.51.100.20",
		}},
		{"none sent", "https://web.onbex.co/", "", "198.51.100.20:51234", http.Header{}, map[string]string{
			"X-Forwarded-For": "", "X-Forwarded-Proto": "https", "X-Forwarded-Host": "web.onbex.co",
			"X-Forwarded-Port": "443", "X-Real-Ip": "198.51.100.20",
		}},
		{"a Host with a port", "https://web.onbex.co:8443/", "", "198.51.100.20:51234", forgedForwarded(), map[string]string{
			"X-Forwarded-Host": "web.onbex.co:8443", "X-Forwarded-Port": "8443",
		}},
		{"a WebSocket upgrade over HTTPS", "https://web.onbex.co/", "", "198.51.100.20:51234", websocket.Clone(), map[string]string{
			"X-Forwarded-Proto": "wss", "X-Forwarded-Port": "443",
		}},
		{"a WebSocket upgrade over plain HTTP", "http://web.onbex.co/", "", "198.51.100.20:51234", websocket.Clone(), map[string]string{
			"X-Forwarded-Proto": "ws", "X-Forwarded-Port": "80",
		}},
		{"an IPv6 peer with a zone", "https://web.onbex.co/", "", "[fe80::1%eth0]:443", forgedForwarded(), map[string]string{
			"X-Real-Ip": "fe80::1",
		}},
		{"an unparseable peer", "https://web.onbex.co/", "", "@unix", forgedForwarded(), map[string]string{
			"X-Real-Ip": "", "X-Forwarded-For": "203.0.113.7",
		}},
		{"no Host", "https://web.onbex.co/", "-", "198.51.100.20:51234", forgedForwarded(), map[string]string{
			"X-Forwarded-Host": "",
		}},
	} {
		var seen http.Header
		next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = r.Header })
		h, err := New(context.Background(), next, CreateConfig(), "client-ip-headers")
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodGet, tc.url, nil)
		r.RemoteAddr = tc.peer
		if tc.host == "-" {
			r.Host = ""
		}
		r.Header = tc.header
		h.ServeHTTP(httptest.NewRecorder(), r)
		for name, want := range tc.want {
			got := strings.Join(seen[name], ",")
			if got != want || want != "" && len(seen[name]) != 1 {
				t.Errorf("%s: %s = %q, want exactly %q", tc.name, name, seen[name], want)
			}
		}
	}
}
