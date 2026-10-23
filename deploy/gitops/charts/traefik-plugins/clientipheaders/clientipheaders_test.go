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
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
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

// TestRndrID (w8/m125): every request gets a fresh Rndr-Id on the request and
// the response; a caller's (either spelling) is replaced.
func TestRndrID(t *testing.T) {
	var seen []string
	h, err := New(context.Background(), http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Rndr-Id"))
		if len(r.Header.Values("Rndr-Id")) != 1 || r.Header.Get("Rndr_id") != "" {
			t.Errorf("app saw Rndr-Id values %q and underscore %q, want one minted value", r.Header.Values("Rndr-Id"), r.Header.Get("Rndr_id"))
		}
	}), CreateConfig(), "client-ip-headers")
	if err != nil {
		t.Fatal(err)
	}
	uuid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "https://web.onbex.co/", nil)
		req.RemoteAddr = "198.51.100.20:1"
		req.Header = http.Header{"Rndr-Id": {"forged"}, "Rndr_id": {"forged-too"}}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		got := rec.Header().Get("Rndr-Id")
		if !uuid.MatchString(got) || got != seen[i] {
			t.Fatalf("request %d: response Rndr-Id %q, app saw %q; want one minted UUID on both", i, got, seen[i])
		}
	}
	if seen[0] == seen[1] {
		t.Errorf("two requests shared Rndr-Id %q", seen[0])
	}
}

// TestRequestStart (w8/196): every request reaches the app with Render's
// X-Request-Start, in microseconds since the epoch and taken at the edge; a
// caller's value, in either spelling, is replaced.
func TestRequestStart(t *testing.T) {
	var seen string
	h, err := New(context.Background(), http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("X-Request-Start")
		if len(r.Header.Values("X-Request-Start")) != 1 || r.Header.Get("X_Request_Start") != "" {
			t.Errorf("app saw X-Request-Start %q and underscore %q, want one value", r.Header.Values("X-Request-Start"), r.Header.Get("X_Request_Start"))
		}
	}), CreateConfig(), "client-ip-headers")
	if err != nil {
		t.Fatal(err)
	}
	for name, forged := range map[string]http.Header{
		"absent":     {},
		"forged":     {"X-Request-Start": {"1"}},
		"underscore": {"X_request_start": {"1"}, "X-Request-Start": {"t=1"}},
	} {
		before := time.Now().UnixMicro()
		req := httptest.NewRequest(http.MethodGet, "https://web.onbex.co/", nil)
		req.RemoteAddr = "198.51.100.20:1"
		req.Header = forged
		h.ServeHTTP(httptest.NewRecorder(), req)
		after := time.Now().UnixMicro()
		got, err := strconv.ParseInt(seen, 10, 64)
		if err != nil || len(seen) != 16 || got < before || got > after {
			t.Errorf("%s: X-Request-Start %q, want 16 digits of microseconds in [%d, %d]", name, seen, before, after)
		}
	}
}
