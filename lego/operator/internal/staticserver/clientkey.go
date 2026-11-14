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

	"github.com/bex-co/bex/lego/types/netutil"
)

// trueClientIPHeader is canonical, so Header.Get does not rebuild the key on
// every request.
var trueClientIPHeader = textproto.CanonicalMIMEHeaderKey("True-Client-IP")

// defaultMaxClientFetches caps the origin fetches one client may hold at once
// (R25-05). The per-site (4) and per-replica (16) caps let one client take every
// slot with unique missing paths on four sites. 3 keeps a single visitor's cold
// page working (its HTML, stylesheet and script are fetched at once, the rest
// follow as those finish) while leaving 13 of 16 slots, and one of any site's 4,
// to others. An over-cap request waits briefly for a slot (admission.go) and
// is then shed like any overload: 503 + Retry-After.
const defaultMaxClientFetches = 3

// keyedLimiter counts in-flight work per key: per client here, per site in
// fetchGate. It never blocks: a key at its cap is shed. An entry exists only
// while the key has work in flight, so the map is bounded by the replica's slots.
type keyedLimiter struct {
	mu       sync.Mutex
	inflight map[string]int
	max      int
}

// newKeyedLimiter builds a limiter; maxPerKey <= 0 disables it.
func newKeyedLimiter(maxPerKey int) *keyedLimiter {
	if maxPerKey <= 0 {
		return nil
	}
	return &keyedLimiter{inflight: map[string]int{}, max: maxPerKey}
}

func (l *keyedLimiter) acquire(key string) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inflight[key] >= l.max {
		return false
	}
	l.inflight[key]++
	return true
}

func (l *keyedLimiter) release(key string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inflight[key] <= 1 {
		delete(l.inflight, key)
	} else {
		l.inflight[key]--
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
// to its /64 (netutil.SourceKey), so rotating through a prefix's addresses does
// not dodge the cap.
// "" means no usable address; such a request is not limited by the client cap.
func clientKey(r *http.Request) string {
	if addr, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get(trueClientIPHeader))); err == nil {
		return netutil.SourceKey(addr).String()
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return netutil.SourceKey(addr).String()
	}
	return ""
}
