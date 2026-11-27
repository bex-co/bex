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
	"maps"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
)

// tenantEdgeArgs configures both tenant entrypoints as traefik.values.yaml
// does: the edge's Server header and the client-identity plugin first
// (tenantEdgeMiddlewares), and every caller's forwarded headers trusted
// (w8/402).
func tenantEdgeArgs(t *testing.T) (dockerArgs, traefikArgs []string) {
	dockerArgs, traefikArgs = edgePluginArgs(t, clientIPHeadersPlugin)
	for _, ep := range []string{"web", "websecure"} {
		traefikArgs = append(traefikArgs, "--entrypoints."+ep+".http.middlewares=edge-headers@file,client-ip@file",
			"--entrypoints."+ep+".forwardedheaders.insecure=true")
	}
	return dockerArgs, traefikArgs
}

// tenantEdgeMiddlewares are the entrypoint Middlewares tenantEdgeArgs names,
// plus any router middlewares.
func tenantEdgeMiddlewares(router map[string]any) map[string]any {
	m := map[string]any{"client-ip": entrypointSanitizerSpec, "edge-headers": edgeHeadersSpec}
	maps.Copy(m, router)
	return m
}

// TestForwardedHeadersOnTheDeployedTraefik is w8/402's actual-proxy gate. On
// the deployed Traefik release, with the entrypoints configured as in
// production, a caller's X-Forwarded-For reaches the app with the client
// appended, as on Render, while every other forwarded or client-identity
// header is the edge's own, and a forged X-Forwarded-For does not pass an
// inbound IP rule. Opt-in: needs Docker.
func TestForwardedHeadersOnTheDeployedTraefik(t *testing.T) {
	if os.Getenv("BEX_TRAEFIK_INTEGRATION") == "" {
		t.Skip("set BEX_TRAEFIK_INTEGRATION=1 to run the forwarded headers on the deployed Traefik release")
	}
	origin := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_ = json.NewEncoder(rw).Encode(req.Header)
	})
	dockerArgs, traefikArgs := tenantEdgeArgs(t)
	base, tlsBase := runEdgeTraefik(t, startEdgeOrigin(t, origin), edgeTraefikConfig{
		label: "forwarded", chain: []string{}, tls: true, readyPath: "/",
		middlewares: tenantEdgeMiddlewares(nil),
		dockerArgs:  dockerArgs, traefikArgs: traefikArgs,
	})
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, DisableKeepAlives: true}
	send := func(t *testing.T, url string, header http.Header) http.Header {
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
		var seen http.Header
		if err := json.NewDecoder(resp.Body).Decode(&seen); err != nil {
			t.Fatalf("%d: %v", resp.StatusCode, err)
		}
		return seen
	}

	forged := http.Header{
		"X-Forwarded-For": {"203.0.113.7"}, "X-Forwarded-Proto": {"http"}, "X-Forwarded-Host": {"evil.example"},
		"X-Forwarded-Port": {"1"}, "X-Forwarded-Server": {"forged"}, "X-Real-Ip": {"203.0.113.8"},
		"X-Forwarded-Uri": {"/admin"}, "X-Forwarded-Method": {"DELETE"}, "X-Forwarded-Prefix": {"/forged"},
		"X-Forwarded-Tls-Client-Cert": {"forged"}, "X_forwarded_for": {"203.0.113.9"}, "X_real_ip": {"203.0.113.10"},
		"True-Client-Ip": {"203.0.113.11"}, "X-Request-Start": {"1"}, "Rndr-Id": {"forged-id"},
	}
	seen := send(t, tlsBase+"/", forged)
	client := seen.Get("True-Client-Ip")
	if client == "" || strings.HasPrefix(client, "203.0.113.") {
		t.Fatalf("True-Client-IP = %q, want the real client", client)
	}
	rndrID := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}$`)
	for name, ok := range map[string]bool{
		"X-Forwarded-For":    seen.Get("X-Forwarded-For") == "203.0.113.7, "+client,
		"X-Real-Ip":          seen.Get("X-Real-Ip") == client,
		"X-Forwarded-Proto":  seen.Get("X-Forwarded-Proto") == "https",
		"X-Forwarded-Host":   seen.Get("X-Forwarded-Host") == "probe.onbex.co",
		"X-Forwarded-Port":   seen.Get("X-Forwarded-Port") == "443",
		"X-Forwarded-Server": seen.Get("X-Forwarded-Server") != "" && seen.Get("X-Forwarded-Server") != "forged",
		"X-Request-Start":    len(seen.Get("X-Request-Start")) == 16,
		"Rndr-Id":            rndrID.MatchString(seen.Get("Rndr-Id")),
	} {
		if !ok {
			t.Errorf("%s = %q, want the edge's value", name, seen.Values(name))
		}
	}
	for _, name := range []string{"X-Forwarded-Uri", "X-Forwarded-Method", "X-Forwarded-Prefix", "X-Forwarded-Tls-Client-Cert", "X_forwarded_for", "X_real_ip"} {
		if v, ok := seen[name]; ok {
			t.Errorf("%s = %q reached the app", name, v)
		}
	}

	for _, tc := range []struct {
		name, url string
		header    http.Header
		want      map[string]string
	}{
		{"none sent", tlsBase + "/", http.Header{}, map[string]string{"X-Forwarded-For": client}},
		{"two lines join in order", tlsBase + "/", http.Header{"X-Forwarded-For": {"203.0.113.7", "198.51.100.9"}},
			map[string]string{"X-Forwarded-For": "203.0.113.7, 198.51.100.9, " + client}},
		{"plain HTTP with a forged https", base + "/", http.Header{"X-Forwarded-Proto": {"https"}, "X-Forwarded-For": {"203.0.113.7"}},
			map[string]string{"X-Forwarded-Proto": "http", "X-Forwarded-Port": "80", "X-Forwarded-For": "203.0.113.7, " + client}},
		{"a WebSocket upgrade", tlsBase + "/", http.Header{
			"Connection": {"Upgrade"}, "Upgrade": {"websocket"}, "X-Forwarded-Proto": {"ws"},
			"Sec-Websocket-Version": {"13"}, "Sec-Websocket-Key": {"dGhlIHNhbXBsZSBub25jZQ=="},
		}, map[string]string{"X-Forwarded-Proto": "wss"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			seen := send(t, tc.url, tc.header)
			for name, want := range tc.want {
				if got := seen.Values(name); len(got) != 1 || got[0] != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
		})
	}

	// The inbound IP rule the operator projects matches the TCP source, so a
	// forged X-Forwarded-For or X-Real-Ip naming an allowed address is refused.
	locked, _ := runEdgeTraefik(t, startEdgeOrigin(t, origin), edgeTraefikConfig{
		label: "forwarded-locked", chain: []string{"probe-ip-allow"}, tls: true, readyPath: "/", readyStatus: http.StatusForbidden,
		middlewares: tenantEdgeMiddlewares(map[string]any{"probe-ip-allow": cidrMiddlewareSpec([]string{"203.0.113.0/24"})}),
		dockerArgs:  dockerArgs, traefikArgs: traefikArgs,
	})
	req, _ := http.NewRequest(http.MethodGet, locked+"/", nil)
	req.Header = http.Header{"X-Forwarded-For": {"203.0.113.7"}, "X-Real-Ip": {"203.0.113.7"}}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("a forged X-Forwarded-For inside the rule got %d, want 403", resp.StatusCode)
	}
}
