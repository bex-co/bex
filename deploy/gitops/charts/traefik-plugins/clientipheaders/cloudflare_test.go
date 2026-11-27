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
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// The tests install their own country tables, so no instance downloads one.
func init() { countries.once.Do(func() {}) }

// edgeResult is what one request through the entrypoint's sanitizer did: the
// headers the app received (nil when it was not reached), the response, and
// the request's headers after the sanitizer, which the access log records.
type edgeResult struct {
	app, logged http.Header
	resp        *httptest.ResponseRecorder
}

func throughEdge(t *testing.T, config *Config, url, peer string, header http.Header) edgeResult {
	t.Helper()
	var res edgeResult
	h, err := New(context.Background(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res.app = r.Header.Clone()
		_, _ = io.WriteString(w, "app")
	}), config, "client-ip-headers")
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, url, nil)
	r.RemoteAddr = peer
	r.Header = header
	res.resp = httptest.NewRecorder()
	// Traefik's entrypoint pre-sets a nil Content-Type to stop net/http's
	// sniffing (contenttype.DisableAutoDetection).
	res.resp.Header()["Content-Type"] = nil
	h.ServeHTTP(res.resp, r)
	res.logged = r.Header
	return res
}

func only(t *testing.T, h http.Header, name, want string) {
	t.Helper()
	if got := h.Values(name); len(got) != 1 || got[0] != want {
		t.Errorf("%s = %q, want exactly %q", name, got, want)
	}
}

var rayShape = regexp.MustCompile(`^([0-9a-f]{16})-([A-Z]{3})$`)

// w4/m687: a request from a non-Cloudflare peer reaches the app with the
// Cloudflare headers a Render app receives (docs/render-artifacts/
// edge-headers.md), whatever the caller sent, and the answer carries the
// request's cf-ray.
func TestCloudflareHeadersFromAnOrdinaryPeer(t *testing.T) {
	table, err := parseCountries(strings.NewReader(fixtureCountries))
	if err != nil {
		t.Fatal(err)
	}
	setCountries(table)
	t.Cleanup(func() { setCountries(nil) })
	forgedCloudflare := http.Header{
		"Cf-Ipcountry": {"KP"}, "Cf-Ray": {"0000000000000000-AAA"}, "Cf-Visitor": {`{"scheme":"http"}`},
		"Cf-Connecting-Ipv6": {"2001:db8::9"}, "Cf-Worker": {"forged.example"},
	}
	for _, tc := range []struct {
		name, url, peer, colo string
		header                http.Header
		client, country       string
		scheme                string
	}{
		{"HTTPS from Germany, forged values", "https://web.onbex.co/", "46.4.0.1:51234", "", forgedCloudflare, "46.4.0.1", "DE", "https"},
		{"plain HTTP from the US", "http://web.onbex.co/", "8.8.8.8:51234", "", http.Header{}, "8.8.8.8", "US", "http"},
		{"IPv6 from Japan", "https://web.onbex.co/", "[2001:200::1]:443", "", http.Header{}, "2001:200::1", "JP", "https"},
		{"a v4-mapped peer", "https://web.onbex.co/", "[::ffff:46.4.0.1]:443", "", http.Header{}, "46.4.0.1", "DE", "https"},
		{"an unallocated address", "https://web.onbex.co/", "198.51.100.20:1", "", http.Header{}, "198.51.100.20", "XX", "https"},
		{"another colo", "https://web.onbex.co/", "46.4.0.1:1", "HEL", http.Header{}, "46.4.0.1", "DE", "https"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := CreateConfig()
			if tc.colo != "" {
				config.Colo = tc.colo
			}
			res := throughEdge(t, config, tc.url, tc.peer, tc.header)
			if res.app == nil {
				t.Fatalf("the request never reached the app: %d %q", res.resp.Code, res.resp.Body.String())
			}
			only(t, res.app, "CF-Connecting-IP", tc.client)
			only(t, res.app, "True-Client-IP", tc.client)
			only(t, res.app, "CF-IPCountry", tc.country)
			only(t, res.app, "CF-Visitor", `{"scheme":"`+tc.scheme+`"}`)
			only(t, res.app, "CDN-Loop", "cloudflare; loops=1")
			for _, name := range []string{"Cf-Connecting-Ipv6", "Cf-Worker"} {
				if v, ok := res.app[name]; ok {
					t.Errorf("%s = %q reached the app", name, v)
				}
			}
			ray := res.app.Get("CF-Ray")
			m := rayShape.FindStringSubmatch(ray)
			wantColo := tc.colo
			if wantColo == "" {
				wantColo = defaultColo
			}
			if len(res.app.Values("CF-Ray")) != 1 || m == nil || m[2] != wantColo {
				t.Fatalf("CF-Ray = %q, want one 16-hex-%s ray", res.app.Values("CF-Ray"), wantColo)
			}
			if id := strings.ReplaceAll(res.app.Get("Rndr-Id"), "-", ""); id != m[1] {
				t.Errorf("CF-Ray %q and Rndr-Id %q are not one value", ray, res.app.Get("Rndr-Id"))
			}
			only(t, res.resp.Header(), "Cf-Ray", ray)
		})
	}
}

// Each request mints its own ray.
func TestCloudflareRayIsPerRequest(t *testing.T) {
	a := throughEdge(t, CreateConfig(), "https://web.onbex.co/", "198.51.100.20:1", http.Header{})
	b := throughEdge(t, CreateConfig(), "https://web.onbex.co/", "198.51.100.20:1", http.Header{})
	if a.app.Get("CF-Ray") == b.app.Get("CF-Ray") {
		t.Errorf("two requests shared CF-Ray %q", a.app.Get("CF-Ray"))
	}
}

// A tenant custom domain behind its own Cloudflare proxy keeps Cloudflare's
// values, as before w4/m687, and its answer carries Cloudflare's ray.
func TestCloudflarePeerKeepsItsHeaders(t *testing.T) {
	setCountries(nil)
	res := throughEdge(t, CreateConfig(), "https://custom.example/", "172.70.1.2:443", http.Header{
		"Cf-Connecting-Ip": {"46.4.0.1"}, "Cf-Ipcountry": {"DE"}, "Cf-Ray": {"8f00aa11bb22cc33-FRA"},
		"Cf-Visitor": {`{"scheme":"https"}`}, "Cdn-Loop": {"cloudflare; loops=1"},
	})
	if res.app == nil {
		t.Fatalf("a Cloudflare peer was refused: %d", res.resp.Code)
	}
	only(t, res.app, "CF-Connecting-IP", "46.4.0.1")
	only(t, res.app, "True-Client-IP", "46.4.0.1")
	only(t, res.app, "CF-IPCountry", "DE")
	only(t, res.app, "CF-Ray", "8f00aa11bb22cc33-FRA")
	only(t, res.app, "CF-Visitor", `{"scheme":"https"}`)
	only(t, res.app, "CDN-Loop", "cloudflare; loops=1")
	only(t, res.resp.Header(), "Cf-Ray", "8f00aa11bb22cc33-FRA")
	if res.resp.Code != http.StatusOK || res.resp.Header().Get("Rndr-Id") == "" {
		t.Errorf("answer %d with Rndr-Id %q, want the app's 200 with the edge's Rndr-Id", res.resp.Code, res.resp.Header().Get("Rndr-Id"))
	}
}

// Cloudflare keeps a caller's CDN-Loop and adds itself, as captured from
// httpbin.onrender.com on 2026-11-27, and refuses a count past 16.
func TestCDNLoop(t *testing.T) {
	for _, tc := range []struct {
		sent    []string
		want    string
		refused bool
	}{
		{nil, "cloudflare; loops=1", false},
		{[]string{"forged"}, "forged, cloudflare; loops=1", false},
		{[]string{"cloudflare; loops=1"}, "cloudflare; loops=2", false},
		{[]string{"cloudflare"}, "cloudflare; loops=1", false},
		{[]string{"foo, cloudflare; loops=3"}, "foo, cloudflare; loops=4", false},
		{[]string{"Cloudflare; loops=2"}, "Cloudflare; loops=3", false},
		{[]string{"cloudflare;loops=2;x=y"}, "cloudflare; loops=3; x=y", false},
		{[]string{"cloudflare; x=y"}, "cloudflare; x=y; loops=1", false},
		{[]string{"cloudflare; loops=abc"}, "cloudflare; loops=1", false},
		{[]string{"cloudflare; loops=2, cloudflare; loops=5"}, "cloudflare; loops=3, cloudflare; loops=6", false},
		{[]string{"foo;loops=2"}, "foo; loops=2, cloudflare; loops=1", false},
		{[]string{"a", "b"}, "a, b, cloudflare; loops=1", false},
		{[]string{"cloudflare; loops=15"}, "cloudflare; loops=16", false},
		{[]string{"cloudflare; loops=16"}, "", true},
		{[]string{"foo, cloudflare; loops=99"}, "", true},
	} {
		res := throughEdge(t, CreateConfig(), "https://web.onbex.co/", "198.51.100.20:1", http.Header{"Cdn-Loop": tc.sent})
		switch {
		case tc.refused && (res.app != nil || res.resp.Code != http.StatusForbidden):
			t.Errorf("CDN-Loop %q: answer %d, want the 403 refusal", tc.sent, res.resp.Code)
		case !tc.refused && res.app == nil:
			t.Errorf("CDN-Loop %q: refused, want %q", tc.sent, tc.want)
		case !tc.refused:
			if got := res.app.Values("CDN-Loop"); len(got) != 1 || got[0] != tc.want {
				t.Errorf("CDN-Loop %q reached the app as %q, want %q", tc.sent, got, tc.want)
			}
		}
	}
}

// w4/m687 t004: Render's edge refuses a caller's own CF-Connecting-IP from
// outside Cloudflare, but bex's replaces it with the peer, because tenants'
// reverse proxies on bex forward Cloudflare's header to other bex services.
// A Cloudflare peer's own passes.
func TestForgedConnectingIPIsReplaced(t *testing.T) {
	for _, tc := range []struct {
		name, url, peer string
		header          http.Header
		want            string
	}{
		{"an address", "https://web.onbex.co/", "198.51.100.20:1", http.Header{"Cf-Connecting-Ip": {"203.0.113.9"}}, "198.51.100.20"},
		{"not an address", "https://web.onbex.co/", "198.51.100.20:1", http.Header{"Cf-Connecting-Ip": {"hello"}}, "198.51.100.20"},
		{"plain HTTP", "http://web.onbex.co/", "198.51.100.20:1", http.Header{"Cf-Connecting-Ip": {"203.0.113.9"}}, "198.51.100.20"},
		{"an IPv6 peer", "https://web.onbex.co/", "[2001:db8::20]:1", http.Header{"Cf-Connecting-Ip": {"203.0.113.9"}}, "2001:db8::20"},
		{"the underscore spelling", "https://web.onbex.co/", "198.51.100.20:1", http.Header{"Cf_connecting_ip": {"203.0.113.9"}}, "198.51.100.20"},
		{"an unparseable peer", "https://web.onbex.co/", "@unix", http.Header{"Cf-Connecting-Ip": {"203.0.113.9"}}, ""},
		{"a Cloudflare peer", "https://web.onbex.co/", "104.16.0.1:1", http.Header{"Cf-Connecting-Ip": {"203.0.113.9"}}, "203.0.113.9"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := throughEdge(t, CreateConfig(), tc.url, tc.peer, tc.header)
			if res.app == nil {
				t.Fatalf("refused with %d %q, want the app reached", res.resp.Code, res.resp.Body.String())
			}
			if got := res.app.Values("CF-Connecting-IP"); tc.want == "" && len(got) != 0 || tc.want != "" && (len(got) != 1 || got[0] != tc.want) {
				t.Errorf("CF-Connecting-IP = %q, want %q", got, tc.want)
			}
			if v, ok := res.app["Cf_connecting_ip"]; ok {
				t.Errorf("Cf_connecting_ip = %q reached the app", v)
			}
		})
	}
}

// A refusal (a CDN-Loop past 16) is Render's 403 byte for byte (captured
// 2026-11-27) and never reaches the app.
func TestRefusalIsRendersAnswer(t *testing.T) {
	res := throughEdge(t, CreateConfig(), "https://web.onbex.co/", "198.51.100.20:1", http.Header{"Cdn-Loop": {"cloudflare; loops=16"}})
	if res.app != nil {
		t.Fatal("a refused request reached the app")
	}
	resp := res.resp
	if resp.Code != http.StatusForbidden || resp.Body.String() != "error code: 1000\n" {
		t.Errorf("answer %d %q, want 403 %q", resp.Code, resp.Body.String(), "error code: 1000\n")
	}
	for name, want := range map[string]string{
		"Content-Type":    "text/plain; charset=UTF-8",
		"Content-Length":  "17",
		"Cache-Control":   "private, max-age=0, no-store, no-cache, must-revalidate, post-check=0, pre-check=0",
		"Expires":         "Thu, 01 Jan 1970 00:00:01 GMT",
		"Referrer-Policy": "same-origin",
		"X-Frame-Options": "SAMEORIGIN",
	} {
		only(t, resp.Header(), name, want)
	}
	// Cloudflare refuses before Render's edge sees the request: no Rndr-Id
	// on the answer, but the access log's line has one, and the answer's
	// cf-ray names it.
	if v, ok := resp.Header()["Rndr-Id"]; ok {
		t.Errorf("the refusal carries Rndr-Id %q", v)
	}
	m := rayShape.FindStringSubmatch(resp.Header().Get("Cf-Ray"))
	if m == nil || strings.ReplaceAll(res.logged.Get("Rndr-Id"), "-", "") != m[1] {
		t.Errorf("cf-ray %q does not name the logged Rndr-Id %q", resp.Header().Get("Cf-Ray"), res.logged.Get("Rndr-Id"))
	}
}
