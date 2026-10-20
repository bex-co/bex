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

func forged() http.Header {
	return http.Header{
		"Cf-Connecting-Ip": {"203.0.113.66"},
		"True-Client-Ip":   {"203.0.113.77"},
		"X-Client-Ip":      {"203.0.113.88"},
		"Cf-Ipcountry":     {"KP"},
		"Cf-Ray":           {"forged-ray"},
		"X-Real-Ip":        {"198.51.100.20"},
		"X-Forwarded-For":  {"198.51.100.20"},
	}
}

// TestSanitizer pins the header contract an app sees (w8/m123).
func TestSanitizer(t *testing.T) {
	for _, tc := range []struct {
		name   string
		peer   string
		header http.Header
		want   map[string]string // "" = absent
	}{{
		name:   "forged headers from an ordinary client",
		peer:   "198.51.100.20:51234",
		header: forged(),
		want: map[string]string{
			"True-Client-IP": "198.51.100.20", "CF-Connecting-IP": "", "CF-IPCountry": "", "CF-Ray": "",
			"X-Client-IP": "", "X-Real-IP": "198.51.100.20", "X-Forwarded-For": "198.51.100.20",
		},
	}, {
		name:   "a plain request gains True-Client-IP",
		peer:   "198.51.100.20:51234",
		header: http.Header{},
		want:   map[string]string{"True-Client-IP": "198.51.100.20", "CF-Connecting-IP": ""},
	}, {
		name:   "an IPv6 client",
		peer:   "[2001:db8::20]:443",
		header: forged(),
		want:   map[string]string{"True-Client-IP": "2001:db8::20", "CF-Connecting-IP": ""},
	}, {
		name:   "a v4-mapped peer reads as IPv4",
		peer:   "[::ffff:198.51.100.20]:443",
		header: http.Header{},
		want:   map[string]string{"True-Client-IP": "198.51.100.20"},
	}, {
		name: "a Cloudflare edge keeps its genuine client headers",
		peer: "172.70.1.2:443",
		header: http.Header{
			"Cf-Connecting-Ip": {"203.0.113.66"}, "Cf-Ipcountry": {"DE"}, "Cf-Ray": {"8f00-FRA"},
			"True-Client-Ip": {"203.0.113.99"}, "X-Client-Ip": {"203.0.113.88"},
		},
		want: map[string]string{
			"True-Client-IP": "203.0.113.66", "CF-Connecting-IP": "203.0.113.66", "CF-IPCountry": "DE",
			"CF-Ray": "8f00-FRA", "X-Client-IP": "",
		},
	}, {
		name:   "a Cloudflare IPv6 edge",
		peer:   "[2606:4700:10::1]:443",
		header: http.Header{"Cf-Connecting-Ip": {"2001:db8::66"}},
		want:   map[string]string{"True-Client-IP": "2001:db8::66", "CF-Connecting-IP": "2001:db8::66"},
	}, {
		name:   "a Cloudflare edge with a garbage CF-Connecting-IP is untrusted",
		peer:   "172.70.1.2:443",
		header: http.Header{"Cf-Connecting-Ip": {"not-an-ip"}, "Cf-Ipcountry": {"KP"}},
		want:   map[string]string{"True-Client-IP": "172.70.1.2", "CF-Connecting-IP": "", "CF-IPCountry": ""},
	}, {
		name: "underscore spellings are stripped too",
		peer: "198.51.100.20:51234",
		header: http.Header{
			"True_client_ip": {"203.0.113.77"}, "Cf_connecting_ip": {"203.0.113.66"}, "X_client_ip": {"203.0.113.88"},
		},
		want: map[string]string{"True-Client-IP": "198.51.100.20", "True_client_ip": "", "Cf_connecting_ip": "", "X_client_ip": ""},
	}, {
		name:   "a Cloudflare edge's extra CF-Connecting-IP values collapse to the first",
		peer:   "172.70.1.2:443",
		header: http.Header{"Cf-Connecting-Ip": {"203.0.113.66", "203.0.113.67"}, "Cf_connecting_ip": {"203.0.113.68"}},
		want:   map[string]string{"True-Client-IP": "203.0.113.66", "CF-Connecting-IP": "203.0.113.66", "Cf_connecting_ip": ""},
	}, {
		name:   "an unparseable peer gets no True-Client-IP",
		peer:   "@unix",
		header: forged(),
		want:   map[string]string{"True-Client-IP": "", "CF-Connecting-IP": "", "X-Client-IP": ""},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			var seen http.Header
			h, err := New(context.Background(), http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				seen = r.Header.Clone()
			}), CreateConfig(), "client-ip-headers")
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodGet, "https://web.onbex.co/", nil)
			req.RemoteAddr = tc.peer
			req.Header = tc.header
			h.ServeHTTP(httptest.NewRecorder(), req)
			for name, want := range tc.want {
				got := seen.Get(name)
				if strings.Contains(name, "_") {
					got = strings.Join(seen[name], ",")
				}
				if got != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
				vals := seen.Values(name)
				if strings.Contains(name, "_") {
					vals = seen[name]
				}
				if n := len(vals); (want == "" && n != 0) || (want != "" && n != 1) {
					t.Errorf("%s = %q, want exactly %q", name, vals, want)
				}
			}
		})
	}
}
