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

package controller

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

// TestCloudflareHeadersOnTheDeployedTraefik is w4/m687's actual-proxy gate. On
// the deployed Traefik release, with the entrypoints configured as in
// production, an app receives the Cloudflare headers a Render app receives
// whatever the caller sent, every answer reads "server: cloudflare" with the
// request's cf-ray, a caller's own CF-Connecting-IP is replaced (bex's edge
// does not refuse it as Render's does, w4/m687 t004), and a CDN-Loop past 16
// gets Cloudflare's 403 "error code: 1000" without reaching the app, on HTTPS
// and plain HTTP alike. Opt-in: needs Docker.
func TestCloudflareHeadersOnTheDeployedTraefik(t *testing.T) {
	if os.Getenv("BEX_TRAEFIK_INTEGRATION") == "" {
		t.Skip("set BEX_TRAEFIK_INTEGRATION=1 to run the Cloudflare headers on the deployed Traefik release")
	}
	var reached atomic.Int32
	origin := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		reached.Add(1)
		rw.Header().Set("Server", "bex-probe")
		_ = json.NewEncoder(rw).Encode(req.Header)
	})
	dockerArgs, traefikArgs := tenantEdgeArgs(t)
	base, tlsBase := runEdgeTraefik(t, startEdgeOrigin(t, origin), edgeTraefikConfig{
		label: "cloudflare", chain: []string{}, tls: true, readyPath: "/",
		middlewares: tenantEdgeMiddlewares(nil),
		dockerArgs:  dockerArgs, traefikArgs: traefikArgs,
	})
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, DisableKeepAlives: true}
	send := func(t *testing.T, url string, header http.Header) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = "probe.onbex.co"
		req.Header = header
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp, body
	}

	for _, tc := range []struct{ name, url, scheme string }{{"HTTPS", tlsBase, "https"}, {"plain HTTP", base, "http"}} {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := send(t, tc.url+"/", http.Header{
				"Cf-Ipcountry": {"KP"}, "Cf-Ray": {"0000000000000000-AAA"}, "Cf-Visitor": {`{"scheme":"gopher"}`},
				"Cdn-Loop": {"forged"}, "Cf-Connecting-Ipv6": {"2001:db8::9"}, "Cf-Connecting-Ip": {"203.0.113.9"},
			})
			var seen http.Header
			if err := json.Unmarshal(body, &seen); err != nil {
				t.Fatalf("%d %q: %v", resp.StatusCode, body, err)
			}
			client := seen.Get("True-Client-Ip")
			ray := seen.Get("Cf-Ray")
			for name, ok := range map[string]bool{
				"Cf-Connecting-Ip":   client != "" && !strings.HasPrefix(client, "203.0.113.") && seen.Get("Cf-Connecting-Ip") == client,
				"Cf-Ipcountry":       len(seen.Values("Cf-Ipcountry")) == 1 && seen.Get("Cf-Ipcountry") != "KP" && len(seen.Get("Cf-Ipcountry")) == 2,
				"Cf-Ray":             len(seen.Values("Cf-Ray")) == 1 && cfRayShape.MatchString(ray),
				"Cf-Visitor":         seen.Get("Cf-Visitor") == `{"scheme":"`+tc.scheme+`"}`,
				"Cdn-Loop":           seen.Get("Cdn-Loop") == "forged, cloudflare; loops=1",
				"Cf-Connecting-Ipv6": len(seen.Values("Cf-Connecting-Ipv6")) == 0,
				"Rndr-Id":            strings.ReplaceAll(seen.Get("Rndr-Id"), "-", "")+"-FRA" == ray,
			} {
				if !ok {
					t.Errorf("app saw %s = %q, want the edge's", name, seen.Values(name))
				}
			}
			if got := resp.Header.Values("Server"); len(got) != 1 || got[0] != "cloudflare" {
				t.Errorf("Server %q, want exactly cloudflare over the app's", got)
			}
			if got := resp.Header.Values("Cf-Ray"); len(got) != 1 || got[0] != ray {
				t.Errorf("answer cf-ray %q, want the request's %q", got, ray)
			}
		})
	}

	before := reached.Load()
	for _, tc := range []struct{ name, url string }{{"HTTPS", tlsBase}, {"plain HTTP", base}} {
		t.Run("a CDN-Loop past 16 over "+tc.name, func(t *testing.T) {
			resp, body := send(t, tc.url+"/", http.Header{"Cdn-Loop": {"cloudflare; loops=16"}})
			if resp.StatusCode != http.StatusForbidden || string(body) != "error code: 1000\n" {
				t.Fatalf("answer %d %q, want 403 %q", resp.StatusCode, body, "error code: 1000\n")
			}
			for name, want := range map[string]string{
				"Content-Type": "text/plain; charset=UTF-8", "Content-Length": "17", "Server": "cloudflare",
				"Cache-Control": "private, max-age=0, no-store, no-cache, must-revalidate, post-check=0, pre-check=0",
				"Expires":       "Thu, 01 Jan 1970 00:00:01 GMT", "Referrer-Policy": "same-origin", "X-Frame-Options": "SAMEORIGIN",
			} {
				if got := resp.Header.Values(name); len(got) != 1 || got[0] != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
			if !cfRayShape.MatchString(resp.Header.Get("Cf-Ray")) || resp.Header.Get("Rndr-Id") != "" {
				t.Errorf("cf-ray %q rndr-id %q, want a ray and no Rndr-Id", resp.Header.Get("Cf-Ray"), resp.Header.Get("Rndr-Id"))
			}
		})
	}
	if n := reached.Load() - before; n != 0 {
		t.Errorf("a refused request reached the app %d times", n)
	}
}
