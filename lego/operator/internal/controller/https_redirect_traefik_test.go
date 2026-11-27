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
	"io"
	"maps"
	"net/http"
	"os"
	"testing"
)

// TestHTTPSRedirectOnTheDeployedTraefik is w8/357's actual-proxy gate. Each
// redirect spec, a web service's, a static site's and the unknown host's,
// runs in the deployed Traefik release, behind the entrypoint sanitizer as in
// production, on a plain router and its TLS sibling. Plain HTTP gets Render's
// codes: 301 for GET, HEAD and a WebSocket upgrade, 307 for every other
// method. Every redirect keeps the path and raw query, even when the caller
// forges X-Forwarded-Proto, and HTTPS reaches the app. Every redirect carries
// Render's cf-cache-status, the request's cf-ray and no rndr-id (w4/m703): a
// static site's 301 reads MISS, then HIT with an Age, and BYPASS with a query,
// every other redirect DYNAMIC; the unknown host's passed-through answer reads
// DYNAMIC with no rndr-id. Opt-in: needs Docker.
func TestHTTPSRedirectOnTheDeployedTraefik(t *testing.T) {
	if os.Getenv("BEX_TRAEFIK_INTEGRATION") == "" {
		t.Skip("set BEX_TRAEFIK_INTEGRATION=1 to run the redirect on the deployed Traefik release")
	}
	for _, route := range []struct {
		name string
		spec map[string]any
		// static301 is the status of a 301 for path, which has a query.
		static301 string
		// passedStatus is the cf-cache-status of an HTTPS answer, and
		// passedRndrID whether it keeps its rndr-id.
		passedStatus string
		passedRndrID bool
	}{
		{"web service", httpsRedirectSpec, "DYNAMIC", "", true},
		{"static site", staticSiteRedirectSpec, "BYPASS", "", true},
		{"unknown host", platformFallbackRedirectSpec, "DYNAMIC", "DYNAMIC", false},
	} {
		t.Run(route.name, func(t *testing.T) {
			testHTTPSRedirectRoute(t, route.spec, route.static301, route.passedStatus, route.passedRndrID)
		})
	}
}

func testHTTPSRedirectRoute(t *testing.T, spec map[string]any, status301, passedStatus string, passedRndrID bool) {
	origin := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_, _ = io.WriteString(rw, "app "+req.Method)
	})
	dockerArgs, traefikArgs := tenantEdgeArgs(t)
	base, tlsBase := runEdgeTraefik(t, startEdgeOrigin(t, origin), edgeTraefikConfig{
		label: "https-redirect", chain: []string{"probe-https"}, tls: true,
		middlewares: tenantEdgeMiddlewares(map[string]any{"probe-https": spec}),
		readyPath:   "/ready?wait=1", readyStatus: http.StatusMovedPermanently,
		dockerArgs: dockerArgs, traefikArgs: traefikArgs,
	})

	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, DisableKeepAlives: true}
	send := func(t *testing.T, method, url string, header http.Header) (*http.Response, string) {
		t.Helper()
		req, err := http.NewRequest(method, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = "probe.onbex.co"
		maps.Copy(req.Header, header)
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		return resp, string(body)
	}
	// edge checks a redirect's or passed-through answer's edge headers.
	edge := func(t *testing.T, resp *http.Response, status string, rndrID bool) {
		t.Helper()
		h := resp.Header
		if h.Get("Cf-Cache-Status") != status || !cfRayShape.MatchString(h.Get("Cf-Ray")) || h.Get("Server") != "cloudflare" ||
			(h.Get("Rndr-Id") != "") != rndrID {
			t.Errorf("%d: cf-cache-status %q cf-ray %q server %q rndr-id %q; want %q, a ray, cloudflare and rndr-id %v",
				resp.StatusCode, h.Get("Cf-Cache-Status"), h.Get("Cf-Ray"), h.Get("Server"), h.Get("Rndr-Id"), status, rndrID)
		}
	}

	const path = "/x/a%20b?q=1&r=%2F"
	const want = "https://probe.onbex.co" + path
	// Go's http.Redirect body, as Render sends it; HEAD and 307s carry none.
	const html = `<a href="https://probe.onbex.co/x/a%20b?q=1&amp;r=%2F">Moved Permanently</a>.` + "\n\n"
	for _, tc := range []struct {
		name, method string
		header       http.Header
		code         int
		contentType  string
		body         string
	}{
		{"GET", http.MethodGet, nil, 301, "text/html; charset=utf-8", html},
		{"HEAD", http.MethodHead, nil, 301, "text/html; charset=utf-8", ""},
		{"POST", http.MethodPost, nil, 307, "", ""},
		{"PUT", http.MethodPut, nil, 307, "", ""},
		{"PATCH", http.MethodPatch, nil, 307, "", ""},
		{"DELETE", http.MethodDelete, nil, 307, "", ""},
		{"OPTIONS", http.MethodOptions, nil, 307, "", ""},
		{"a forged https proto", http.MethodPost, http.Header{"X-Forwarded-Proto": {"https"}}, 307, "", ""},
		{"a WebSocket upgrade", http.MethodGet, http.Header{
			"Connection": {"Upgrade"}, "Upgrade": {"websocket"},
			"Sec-Websocket-Version": {"13"}, "Sec-Websocket-Key": {"dGhlIHNhbXBsZSBub25jZQ=="},
		}, 301, "text/html; charset=utf-8", html},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := send(t, tc.method, base+path, tc.header)
			if resp.StatusCode != tc.code || resp.Header.Get("Location") != want || resp.Header.Get("Content-Type") != tc.contentType || body != tc.body {
				t.Fatalf("%d Location %q Content-Type %q body %q; want %d %q %q %q",
					resp.StatusCode, resp.Header.Get("Location"), resp.Header.Get("Content-Type"), body, tc.code, want, tc.contentType, tc.body)
			}
			status := "DYNAMIC"
			if tc.code == http.StatusMovedPermanently {
				status = status301
			}
			edge(t, resp, status, false)
		})
	}

	if status301 != "DYNAMIC" {
		// A static site's path with no query: the first 301 is stored, and
		// GET and HEAD then read the stored copy with its Age.
		for i, tc := range []struct{ method, status string }{
			{http.MethodGet, "MISS"}, {http.MethodGet, "HIT"}, {http.MethodHead, "HIT"},
		} {
			resp, _ := send(t, tc.method, base+"/stored", nil)
			edge(t, resp, tc.status, false)
			if age := resp.Header.Get("Age"); (age != "") != (tc.status == "HIT") || resp.Header.Get("Vary") != "accept-encoding" {
				t.Errorf("#%d %s /stored: Age %q Vary %q; want an Age only with HIT, and Vary accept-encoding", i, tc.method, age, resp.Header.Get("Vary"))
			}
		}
	}

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run("HTTPS "+method+" reaches the app", func(t *testing.T) {
			resp, body := send(t, method, tlsBase+path, nil)
			if resp.StatusCode != http.StatusOK || body != "app "+method || resp.Header.Get("Location") != "" {
				t.Fatalf("%d Location %q body %q; want the app's 200", resp.StatusCode, resp.Header.Get("Location"), body)
			}
			edge(t, resp, passedStatus, passedRndrID)
		})
	}
}
