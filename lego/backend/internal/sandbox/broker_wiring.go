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

package sandbox

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// GatewayBrokerCACertPath is the internal :8081 path on bex-ssh-gateway
	// that serves ONLY the public Sandbox Broker CA certificate PEM to bex-api
	// (w2/m535, ADR122 D6).
	GatewayBrokerCACertPath = "/sandbox-broker-ca.crt"

	// SandboxBrokerCACertFile is the standalone public Sandbox Broker CA PEM
	// materialized inside allow-all and allow-list /v1/sandboxes Pods (used by
	// NODE_EXTRA_CA_CERTS).
	SandboxBrokerCACertFile = "/etc/bex/sandbox-broker-ca.crt"

	// SandboxBrokerCABundleFile is the combined system public CAs + Sandbox
	// Broker CA bundle materialized inside allow-all and allow-list
	// /v1/sandboxes Pods (used by SSL_CERT_FILE, REQUESTS_CA_BUNDLE,
	// CURL_CA_BUNDLE).
	SandboxBrokerCABundleFile = "/etc/bex/sandbox-broker-ca-bundle.crt"

	// SandboxBrokerNoProxy keeps loopback traffic inside the sandbox Pod off
	// the outbound HTTPS credential broker.
	SandboxBrokerNoProxy = "localhost,127.0.0.1,::1"

	defaultBrokerCACacheTTL = 5 * time.Minute
	maxBrokerCAPEMBytes     = 64 * 1024
)

var defaultBrokerCAHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// BrokerConfig wires allow-all and allow-list /v1/sandboxes Pods to
// bex-ssh-gateway's Pod-bound outbound HTTPS credential broker (w2/m535,
// ADR122 D6). Zero ProxyURL disables broker injection cleanly (no-op).
//
// Only the PUBLIC Sandbox Broker CA certificate ever enters bex-api or a
// sandbox Pod; ValidatePublicBrokerCAPEM rejects any PEM containing a private
// key block.
type BrokerConfig struct {
	// ProxyURL is the cluster-internal HTTP proxy origin for the broker
	// listener (e.g. http://bex-ssh-gateway.bex-system.svc.cluster.local:8086).
	ProxyURL string
	// CACertPath is an optional path on disk to the public-only Broker CA PEM.
	CACertPath string
	// CACertURL is an optional internal URL on bex-ssh-gateway (:8081) serving
	// the public-only Broker CA PEM.
	CACertURL string
	// CACertPEM is an optional pre-loaded public-only Broker CA PEM (tests).
	CACertPEM []byte
	// HTTP overrides the client used to fetch CACertURL.
	HTTP *http.Client
	// CacheTTL bounds how long a loaded CA PEM is cached before re-reading.
	CacheTTL time.Duration
	// Now overrides the clock for tests.
	Now func() time.Time

	mu        sync.Mutex
	cachedPEM []byte
	cachedAt  time.Time
}

// Enabled reports whether sandbox credential broker Pod wiring is configured.
func (b *BrokerConfig) Enabled() bool {
	return b != nil && strings.TrimSpace(b.ProxyURL) != ""
}

func (b *BrokerConfig) now() time.Time {
	if b != nil && b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

func (b *BrokerConfig) cacheTTL() time.Duration {
	if b != nil && b.CacheTTL > 0 {
		return b.CacheTTL
	}
	return defaultBrokerCACacheTTL
}

// ValidateBrokerProxyURL validates BEX_SANDBOX_BROKER_URL: empty (disabled) or
// a plain http://host:port origin with no userinfo, path, query, or fragment.
func ValidateBrokerProxyURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("BEX_SANDBOX_BROKER_URL: %w", err)
	}
	if u.Scheme != "http" {
		return "", fmt.Errorf("BEX_SANDBOX_BROKER_URL must use http://, got %q", u.Scheme)
	}
	if u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("BEX_SANDBOX_BROKER_URL must be a bare http://host:port origin")
	}
	host := u.Hostname()
	portStr := u.Port()
	if host == "" || portStr == "" {
		return "", errors.New("BEX_SANDBOX_BROKER_URL must include both host and port")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("BEX_SANDBOX_BROKER_URL port %q is invalid", portStr)
	}
	return "http://" + u.Host, nil
}

// ValidatePublicBrokerCAPEM validates that raw contains ONLY a single valid,
// currently-active X.509 CA CERTIFICATE PEM block and ZERO private-key or
// other PEM blocks. Returns a canonical re-encoded CERTIFICATE PEM.
func ValidatePublicBrokerCAPEM(raw []byte, now time.Time) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("sandbox broker CA PEM is empty")
	}
	if len(raw) > maxBrokerCAPEMBytes {
		return nil, fmt.Errorf("sandbox broker CA PEM exceeds %d bytes", maxBrokerCAPEMBytes)
	}
	rest := raw
	var certBlock *pem.Block
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			if len(bytes.TrimSpace(rest)) != 0 {
				return nil, errors.New("sandbox broker CA PEM contains trailing non-PEM data")
			}
			break
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("sandbox broker CA PEM must contain only CERTIFICATE blocks, found %q", block.Type)
		}
		if certBlock != nil {
			return nil, errors.New("sandbox broker CA PEM must contain exactly one CERTIFICATE block")
		}
		certBlock = block
	}
	if certBlock == nil {
		return nil, errors.New("sandbox broker CA PEM contains no CERTIFICATE block")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse sandbox broker CA certificate: %w", err)
	}
	if !cert.IsCA {
		return nil, errors.New("sandbox broker CA certificate is not a CA (IsCA=false)")
	}
	checkAt := now.UTC()
	if checkAt.Before(cert.NotBefore) || !checkAt.Before(cert.NotAfter) {
		return nil, errors.New("sandbox broker CA certificate is expired or not yet valid")
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), nil
}

// ResolveCAPEM returns the canonical public Sandbox Broker CA PEM. Caches
// validated results for CacheTTL; on a transient refresh error after a prior
// valid load, returns the still-valid cached PEM rather than disrupting new
// sandboxes.
func (b *BrokerConfig) ResolveCAPEM(ctx context.Context) ([]byte, error) {
	if !b.Enabled() {
		return nil, errors.New("sandbox broker is not configured")
	}
	now := b.now()
	if len(b.CACertPEM) > 0 {
		return ValidatePublicBrokerCAPEM(b.CACertPEM, now)
	}

	b.mu.Lock()
	cached := b.cachedPEM
	fresh := len(cached) > 0 && now.Sub(b.cachedAt) < b.cacheTTL()
	b.mu.Unlock()
	if fresh {
		return bytes.Clone(cached), nil
	}

	var (
		raw []byte
		err error
	)
	switch {
	case strings.TrimSpace(b.CACertPath) != "":
		raw, err = os.ReadFile(b.CACertPath)
	case strings.TrimSpace(b.CACertURL) != "":
		raw, err = b.fetchCAFromURL(ctx, b.CACertURL)
	default:
		return nil, errors.New("sandbox broker CA source is not configured")
	}
	if err == nil {
		var canonical []byte
		canonical, err = ValidatePublicBrokerCAPEM(raw, now)
		if err == nil {
			b.mu.Lock()
			b.cachedPEM = bytes.Clone(canonical)
			b.cachedAt = now
			b.mu.Unlock()
			return canonical, nil
		}
	}
	// If a previously cached CA cert is still valid (not expired), prefer it
	// over failing during a brief gateway restart.
	if len(cached) > 0 {
		if valid, validErr := ValidatePublicBrokerCAPEM(cached, now); validErr == nil {
			log.Printf("sandbox broker: CA refresh failed (%v); reusing cached public CA", err)
			return valid, nil
		}
	}
	return nil, err
}

func (b *BrokerConfig) fetchCAFromURL(ctx context.Context, caURL string) ([]byte, error) {
	httpClient := b.HTTP
	if httpClient == nil {
		httpClient = defaultBrokerCAHTTPClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, caURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch sandbox broker CA %s: HTTP %d", caURL, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBrokerCAPEMBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBrokerCAPEMBytes {
		return nil, fmt.Errorf("fetch sandbox broker CA: response exceeds %d bytes", maxBrokerCAPEMBytes)
	}
	return body, nil
}

// Inject returns the sandbox container entrypoint and env updated with the
// credential broker's proxy env vars and public CA trust-store bootstrap for
// allow-all and allow-list /v1/sandboxes Pods.
//
// Invariants:
//   - Never injects into deny-all sandboxes (zero egress) or SessionOnly
//     (agent-session) templates.
//   - No-op when BrokerConfig is nil or ProxyURL is unset.
//   - Atomic: never injects HTTPS_PROXY without also materializing the public
//     Sandbox Broker CA bundle.
//   - Broker env vars sit below user-provided env so explicit caller values win.
//   - Zero credential secret values ever enter env or the entrypoint script.
func (b *BrokerConfig) Inject(ctx context.Context, policy *NetworkPolicy, tmpl Template, userEnv map[string]string) ([]string, map[string]string) {
	if !b.Enabled() || tmpl.SessionOnly || policy == nil || policy.Default == NetworkPolicyDenyAll {
		return tmpl.Entrypoint, userEnv
	}
	caPEM, err := b.ResolveCAPEM(ctx)
	if err != nil {
		log.Printf("sandbox broker: skipping broker wiring for sandbox create (CA unavailable): %v", err)
		return tmpl.Entrypoint, userEnv
	}
	proxyURL := strings.TrimSpace(b.ProxyURL)
	merged := map[string]string{
		"HTTPS_PROXY":         proxyURL,
		"https_proxy":         proxyURL,
		"NO_PROXY":            SandboxBrokerNoProxy,
		"no_proxy":            SandboxBrokerNoProxy,
		"SSL_CERT_FILE":       SandboxBrokerCABundleFile,
		"REQUESTS_CA_BUNDLE":  SandboxBrokerCABundleFile,
		"CURL_CA_BUNDLE":      SandboxBrokerCABundleFile,
		"NODE_EXTRA_CA_CERTS": SandboxBrokerCACertFile,
	}
	maps.Copy(merged, userEnv)
	return wrapEntrypointWithBrokerCA(tmpl.Entrypoint, caPEM), merged
}

// brokerCABootstrapScript returns an idempotent POSIX sh snippet that writes
// the public Sandbox Broker CA PEM to SandboxBrokerCACertFile, combines it with
// the image's pristine system CA bundle into SandboxBrokerCABundleFile (saving
// a backup of the original system bundle so resume/snapshot restores never
// duplicate entries and pick up CA rotations cleanly), and updates existing
// system bundle files so tools that ignore SSL_CERT_FILE also trust both public
// CAs and the Sandbox Broker CA.
//
// caPEM MUST already be canonical PEM produced by ValidatePublicBrokerCAPEM
// (ASCII base64 + BEGIN/END CERTIFICATE only — never arbitrary untrusted text).
func brokerCABootstrapScript(caPEM []byte) string {
	trimmed := strings.TrimRight(string(caPEM), "\n")
	return fmt.Sprintf(`(
umask 022
_bex_cert=%s
_bex_bundle=%s
_bex_orig=/etc/bex/system-ca-bundle.orig.crt
_bex_sys_cas="/etc/ssl/certs/ca-certificates.crt /etc/pki/tls/certs/ca-bundle.crt /etc/ssl/cert.pem /etc/ssl/ca-bundle.pem"
mkdir -p /etc/bex
cat <<'BEX_SANDBOX_BROKER_CA_EOF' >"$_bex_cert.tmp"
%s
BEX_SANDBOX_BROKER_CA_EOF
mv -f "$_bex_cert.tmp" "$_bex_cert"
if [ ! -s "$_bex_orig" ]; then
  for _bex_ca in $_bex_sys_cas; do
    if [ -s "$_bex_ca" ]; then
      cp "$_bex_ca" "$_bex_orig"
      break
    fi
  done
  if [ ! -s "$_bex_orig" ] && command -v python3 >/dev/null 2>&1; then
    _bex_cf="$(python3 -c 'import certifi; print(certifi.where())' 2>/dev/null || true)"
    if [ -n "$_bex_cf" ] && [ -s "$_bex_cf" ]; then
      cp "$_bex_cf" "$_bex_orig"
    fi
  fi
fi
if [ -s "$_bex_orig" ]; then
  { cat "$_bex_orig"; printf '\n'; cat "$_bex_cert"; } >"$_bex_bundle.tmp"
else
  cp "$_bex_cert" "$_bex_bundle.tmp"
fi
mv -f "$_bex_bundle.tmp" "$_bex_bundle"
for _bex_ca in $_bex_sys_cas; do
  if [ -f "$_bex_ca" ] && [ ! -L "$_bex_ca" ] && [ -w "$_bex_ca" ]; then
    cp "$_bex_bundle" "$_bex_ca"
  fi
done
)`,
		SandboxBrokerCACertFile,
		SandboxBrokerCABundleFile,
		trimmed,
	)
}

func wrapEntrypointWithBrokerCA(entry []string, caPEM []byte) []string {
	bootstrap := brokerCABootstrapScript(caPEM)
	if len(entry) == 0 {
		return []string{"/bin/sh", "-c", bootstrap + "\nexec sleep infinity"}
	}
	if len(entry) == 3 && (entry[0] == "/bin/sh" || entry[0] == "sh") && entry[1] == "-c" {
		return []string{entry[0], "-c", bootstrap + "\n" + entry[2]}
	}
	out := make([]string, 0, 4+len(entry))
	out = append(out, "/bin/sh", "-c", bootstrap+"\nexec \"$@\"", "--")
	out = append(out, entry...)
	return out
}
