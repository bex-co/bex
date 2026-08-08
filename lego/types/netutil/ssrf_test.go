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

package netutil_test

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/types/netutil"
)

func TestUnsafeOriginIP(t *testing.T) {
	for _, raw := range []string{
		"127.0.0.1",       // loopback
		"::1",             // IPv6 loopback
		"10.0.0.1",        // RFC 1918
		"172.16.0.1",      // RFC 1918
		"192.168.1.1",     // RFC 1918
		"169.254.169.254", // link-local / AWS+GCP metadata
		"169.254.1.1",     // link-local
		"fc00::1",         // IPv6 ULA (private)
		"fe80::1",         // IPv6 link-local
		"0.0.0.0",         // unspecified
	} {
		ip := net.ParseIP(raw)
		if ip == nil {
			t.Fatalf("net.ParseIP(%q) = nil", raw)
		}
		if !netutil.UnsafeOriginIP(ip) {
			t.Errorf("UnsafeOriginIP(%s) = false; want true", raw)
		}
	}
}

// TestUnsafeOriginIPRFC6598 pins the w1/m65 F15 fix: RFC 6598 shared address
// space (100.64.0.0/10) is blocked for outbound origin dials — net.IP.IsPrivate
// does NOT cover it — across literal, boundary, and IPv4-mapped-IPv6 forms,
// while genuinely public 100.x addresses just outside the range stay allowed.
func TestUnsafeOriginIPRFC6598(t *testing.T) {
	blocked := []string{
		"100.64.0.0",        // network (lowest in /10)
		"100.64.0.1",        // literal
		"100.100.50.7",      // middle
		"100.127.255.255",   // highest in /10
		"::ffff:100.64.0.1", // IPv4-mapped IPv6
	}
	for _, raw := range blocked {
		ip := net.ParseIP(raw)
		if ip == nil {
			t.Fatalf("net.ParseIP(%q) = nil", raw)
		}
		if !netutil.UnsafeOriginIP(ip) {
			t.Errorf("UnsafeOriginIP(%s) = false; want true (RFC 6598 must be blocked)", raw)
		}
	}
	// Just outside 100.64.0.0/10 — genuinely public, must NOT be blocked.
	for _, raw := range []string{"100.63.255.255", "100.128.0.0", "99.64.0.1", "8.8.8.8"} {
		ip := net.ParseIP(raw)
		if ip == nil {
			t.Fatalf("net.ParseIP(%q) = nil", raw)
		}
		if netutil.UnsafeOriginIP(ip) {
			t.Errorf("UnsafeOriginIP(%s) = true; want false (public, outside RFC 6598)", raw)
		}
	}
}

func TestSafeDialContextBlocksPrivateAddresses(t *testing.T) {
	dial := netutil.SafeDialContext(5 * time.Second)
	// Literal IP addresses are resolved locally (no DNS query) — safe for unit tests.
	for _, addr := range []string{
		"127.0.0.1:80",       // loopback
		"10.0.0.1:80",        // RFC 1918
		"169.254.169.254:80", // cloud metadata
		"[::1]:80",           // IPv6 loopback
	} {
		_, err := dial(context.Background(), "tcp", addr)
		if err == nil {
			t.Errorf("SafeDialContext(%s): expected block, got nil error", addr)
			continue
		}
		if !strings.Contains(err.Error(), "blocked address") {
			t.Errorf("SafeDialContext(%s): error %q; want to contain \"blocked address\"", addr, err.Error())
		}
	}
}

// TestSafeDialContextDNSRebindBlocked verifies that a hostname resolving to a
// private address is rejected even when the caller supplies a domain name
// rather than a literal IP — the guard covers the DNS-rebind case.
func TestSafeDialContextDNSRebindBlocked(t *testing.T) {
	dial := netutil.SafeDialContext(5 * time.Second)
	// "localhost" resolves to 127.0.0.1 (or ::1), both unsafe.
	_, err := dial(context.Background(), "tcp", "localhost:80")
	if err == nil {
		t.Fatal("SafeDialContext(localhost): expected SSRF block, got nil error")
	}
	if !strings.Contains(err.Error(), "blocked address") {
		t.Errorf("SafeDialContext(localhost): error %q; want \"blocked address\"", err.Error())
	}
}

// TestUnsafeMetadataIP: the operator's shared-transport guard blocks the metadata
// endpoint / loopback / link-local but deliberately PERMITS private ClusterIP
// ranges, which it must still reach (Zot registry, Prometheus) — w1/m53.
func TestUnsafeMetadataIP(t *testing.T) {
	// The cloud-metadata endpoint (link-local) plus multicast/unspecified are
	// blocked; loopback, private ClusterIP ranges, and public are all allowed —
	// the operator's shared client must still reach Zot/Prometheus (private) and
	// httptest servers (loopback).
	blocked := []string{"169.254.169.254", "169.254.1.1", "fe80::1", "0.0.0.0"}
	for _, raw := range blocked {
		if ip := net.ParseIP(raw); ip == nil || !netutil.UnsafeMetadataIP(ip) {
			t.Errorf("UnsafeMetadataIP(%s) = false; want true (must be blocked)", raw)
		}
	}
	allowed := []string{"127.0.0.1", "::1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "fc00::1", "8.8.8.8"}
	for _, raw := range allowed {
		if ip := net.ParseIP(raw); ip == nil || netutil.UnsafeMetadataIP(ip) {
			t.Errorf("UnsafeMetadataIP(%s) = true; want false (loopback/private/public must be allowed)", raw)
		}
	}
}

// TestSafeDialContextFuncMetadataGuard: a dial through the metadata guard blocks
// the cloud-metadata endpoint.
func TestSafeDialContextFuncMetadataGuard(t *testing.T) {
	dial := netutil.SafeDialContextFunc(2*time.Second, netutil.UnsafeMetadataIP)
	for _, addr := range []string{"169.254.169.254:80", "[fe80::1]:80"} {
		if _, err := dial(context.Background(), "tcp", addr); err == nil ||
			!strings.Contains(err.Error(), "blocked address") {
			t.Errorf("metadata guard dial(%s): err=%v; want blocked", addr, err)
		}
	}
}
