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

package staticserver

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"net/textproto"
	"strings"
	"sync"
)

// trueClientIPHeader is canonical, so Header.Get does not rebuild the key on
// every request.
var trueClientIPHeader = textproto.CanonicalMIMEHeaderKey("True-Client-IP")

// defaultMaxClientFetches caps the origin fetches one client may hold at once
// (R25-05). The per-site (4) and per-replica (16) caps let one client take every
// slot with unique missing paths on four sites. 3 keeps a single visitor's cold
// page working (its HTML, stylesheet and script are fetched at once, the rest
// follow as those finish) while leaving 13 of 16 slots, and one of any site's 4,
// to others. An over-cap request is shed like any overload: 503 + Retry-After.
const defaultMaxClientFetches = 3

// clientLimiter counts in-flight origin fetches per client. Like fetchGate it
// never blocks: a client at its cap is shed. An entry exists only while the
// client has a fetch in flight, so the map is bounded by the replica's slots.
type clientLimiter struct {
	mu       sync.Mutex
	inflight map[string]int
	max      int
}

// newClientLimiter builds a limiter; max <= 0 disables it.
func newClientLimiter(maxPerClient int) *clientLimiter {
	if maxPerClient <= 0 {
		return nil
	}
	return &clientLimiter{inflight: map[string]int{}, max: maxPerClient}
}

func (l *clientLimiter) acquire(client string) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inflight[client] >= l.max {
		return false
	}
	l.inflight[client]++
	return true
}

func (l *clientLimiter) release(client string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inflight[client] <= 1 {
		delete(l.inflight, client)
	} else {
		l.inflight[client]--
	}
}

type clientCtxKey struct{}

func withClient(ctx context.Context, client string) context.Context {
	return context.WithValue(ctx, clientCtxKey{}, client)
}

func clientFrom(ctx context.Context) string {
	client, _ := ctx.Value(clientCtxKey{}).(string)
	return client
}

// clientKey names the client a request's origin fetches are charged to: the
// True-Client-IP the Traefik clientipheaders plugin sets on every request that
// reaches the static server, else the connection's peer address. IPv6 collapses
// to its /64, so rotating through a prefix's addresses does not dodge the cap.
// "" means no usable address; such a request is not limited by the client cap.
func clientKey(r *http.Request) string {
	if addr, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get(trueClientIPHeader))); err == nil {
		return canonicalClient(addr)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return canonicalClient(addr)
	}
	return ""
}

func canonicalClient(addr netip.Addr) string {
	addr = addr.Unmap().WithZone("")
	if addr.Is6() {
		p, _ := addr.Prefix(64) // a valid length for an IPv6 address
		return p.String()
	}
	return addr.String()
}
