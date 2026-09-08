/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

    10|Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webshell

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ProbeUnauthenticatedRefusal is the credential-free Web Shell edge liveness
// check (w2/m90): TLS-dial the public /shell route, attempt a WebSocket upgrade
// with no ticket, and require the gateway's deterministic 401 "missing ticket"
// refusal. That shape means the edge is alive and fail-closed; a timeout,
// connection refusal, TLS error, wrong route (404/502), or disabled transport
// (503) is a dead or unactivated edge and must fail the probe loudly.
//
// No ticket, key, or tenant fixture is involved — the failure class is
// pre-authentication and hits every connection alike, so an unauthenticated
// probe is sufficient and honest (same lesson as nativessh.ProbeKEXINIT).
func ProbeUnauthenticatedRefusal(rawURL string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	switch u.Scheme {
	case "wss":
		u.Scheme = "https"
	case "ws":
		u.Scheme = "http"
	case "https", "http":
		// already HTTP(S) for the upgrade request
	default:
		return fmt.Errorf("unsupported scheme %q (want wss/ws/https/http)", u.Scheme)
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/shell"
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	// Minimal WebSocket upgrade headers — enough for the gateway to reach the
	// ticket check before Upgrade. No Sec-WebSocket-Protocol ticket entry.
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	client := &http.Client{
		Timeout: timeout,
		// The gateway refuses before upgrading; never follow redirects that
		// could turn a dead route into an accidental 200 elsewhere.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("dial %s: %w", u.Host, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	got := strings.TrimSpace(string(body))

	if resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("at %s, status = %d body %q, want 401 missing ticket (alive-but-refusing)",
			u.Host, resp.StatusCode, got)
	}
	if got != "missing ticket" {
		return fmt.Errorf("at %s, 401 body = %q, want %q", u.Host, got, "missing ticket")
	}
	return nil
}
