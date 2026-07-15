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
		if !strings.Contains(err.Error(), "private address") {
			t.Errorf("SafeDialContext(%s): error %q; want to contain \"private address\"", addr, err.Error())
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
	if !strings.Contains(err.Error(), "private address") {
		t.Errorf("SafeDialContext(localhost): error %q; want \"private address\"", err.Error())
	}
}
