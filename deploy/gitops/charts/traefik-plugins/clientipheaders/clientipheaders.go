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

// Package clientipheaders is a Traefik local HTTP middleware on the tenant
// entrypoints (w8/m123). On Render every web service sits behind Cloudflare,
// which overwrites CF-Connecting-IP and True-Client-IP with the real client,
// and Render tells apps to trust them. Traefik rewrites only its own
// X-Forwarded-*/X-Real-IP set, so without this an app trusted whatever the
// caller sent. Every request leaves here with True-Client-IP naming the real
// client: the TCP peer (after PROXY protocol), or Cloudflare's CF-Connecting-IP
// when the peer is a Cloudflare edge — a tenant custom domain behind its own
// Cloudflare proxy. CF-* headers survive only from a Cloudflare peer.
package bex_client_ip_headers

import (
	"context"
	"net"
	"net/http"
	"strings"
)

// cloudflareRanges is Cloudflare's published edge address space. It must equal
// lego/backend/internal/core/proxiedhosts.go's cloudflareRanges;
// scripts/gitops-validate.sh fails when the two drift.
var cloudflareRanges = []string{
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
	"141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
	"197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
	"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
	"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32",
	"2405:8100::/32", "2a06:98c0::/29", "2c0f:f248::/32",
}

// cloudflareHeaders are the client-describing headers Cloudflare sets; only a
// Cloudflare peer may pass them on. Keys are lowercase with "-" for "_":
// CGI-style frameworks (Rack, PHP, WSGI) read True_Client_IP and
// True-Client-IP as the same HTTP_TRUE_CLIENT_IP, so both spellings count.
var cloudflareHeaders = map[string]bool{
	"cf-connecting-ip": true, "cf-connecting-ipv6": true, "cf-ipcountry": true,
	"cf-ray": true, "cf-visitor": true, "cf-worker": true,
}

// alwaysStripped never comes from a party bex trusts; True-Client-IP is
// rewritten below.
var alwaysStripped = map[string]bool{"x-client-ip": true, "true-client-ip": true}

type Config struct{}

func CreateConfig() *Config { return &Config{} }

type sanitizer struct {
	next       http.Handler
	cloudflare []*net.IPNet
}

func New(_ context.Context, next http.Handler, _ *Config, _ string) (http.Handler, error) {
	s := &sanitizer{next: next}
	for _, cidr := range cloudflareRanges {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}
		s.cloudflare = append(s.cloudflare, n)
	}
	return s, nil
}

func (s *sanitizer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := r.Header
	peer := peerIP(r.RemoteAddr)
	// Ordinary traffic carries no CF-Connecting-IP, so it skips the range scan.
	var client net.IP
	if v := h.Get("CF-Connecting-IP"); v != "" && peer != nil && s.fromCloudflare(peer) {
		client = net.ParseIP(v)
	}
	for name := range h {
		key := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
		// A trusted edge's CF-* stay only in Cloudflare's own hyphenated form.
		if alwaysStripped[key] || (cloudflareHeaders[key] && (client == nil || strings.Contains(name, "_"))) {
			delete(h, name)
		}
	}
	switch {
	case client != nil:
		h.Set("CF-Connecting-IP", client.String())
		h.Set("True-Client-IP", client.String())
	case peer != nil:
		h.Set("True-Client-IP", peer.String())
	}
	s.next.ServeHTTP(w, r)
}

func (s *sanitizer) fromCloudflare(ip net.IP) bool {
	for _, n := range s.cloudflare {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// peerIP is the request's TCP peer, which Traefik has already replaced with
// the PROXY-protocol client on the load-balanced entrypoints. nil when
// RemoteAddr is not an address.
func peerIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return ip
}
