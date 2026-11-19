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

package edgecache

import (
	"context"
	"hash/fnv"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"slices"
	"strconv"
	"time"
)

// The replicas of a service's cache tier (w3/m489) share one cache by key
// affinity: every cache-eligible resource has one owner among them, chosen by
// rendezvous hashing so that every replica that knows the same peers picks the
// same one, and a replica that is not the owner forwards the request there. A
// key is then filled once for the whole service and stored once, and losing a
// replica moves only the keys it owned. The owner is the resource's, not the
// key's (resourceKey): both encodings of one resource live on one replica, so
// a HEAD finds the page a GET of the other encoding stored (w3/075). A forward that fails before any answer
// is served here instead, so a replica that is gone costs nothing but the
// cache.

// HopHeader marks a request one replica forwarded to the key's owner. The
// owner accepts it only from a peer's address, serves it itself and never
// forwards it again; the app never sees it.
const HopHeader = "X-Bex-Edge-Cache-Hop"

// peerDialTimeout bounds connecting to a peer: a replica that is gone costs a
// request at most this before it is served here.
const peerDialTimeout = time.Second

// peerSet is one view of the tier: this replica's own address and every
// replica's, each host:port, and the hosts a hop is accepted from.
type peerSet struct {
	self  string
	addrs []string
	hosts map[string]bool
}

// hopKey marks, on a request's context, a hop accepted from a peer.
type hopKey struct{}

// forwardKey carries a forward's inbound request and its owner to the
// forwarder's Rewrite and ErrorHandler.
type forwardKey struct{}

type forwarding struct {
	in         *http.Request
	key, owner string
}

// SetPeers replaces the tier this replica shares its cache with: self is its
// own address and peers every replica's, normally self among them, each
// host:port. With no peers, or self the only one, every request is served
// here, as a sidecar serves it.
func (h *Handler) SetPeers(self string, peers []string) {
	set := &peerSet{self: self, addrs: slices.Clone(peers), hosts: map[string]bool{}}
	for _, addr := range peers {
		if host, _, err := net.SplitHostPort(addr); err == nil {
			set.hosts[host] = true
		}
	}
	h.peers.Store(set)
}

// owner is the peer that owns key, or "" when it is this replica or there are
// no peers: the one whose fnv-1a hash of key, a NUL and its address is
// highest.
func (h *Handler) owner(key string) string {
	set := h.peers.Load()
	if set == nil {
		return ""
	}
	best, bestScore := "", uint64(0)
	for _, addr := range set.addrs {
		hash := fnv.New64a()
		_, _ = io.WriteString(hash, key)
		_, _ = hash.Write([]byte{0})
		_, _ = io.WriteString(hash, addr)
		if score := hash.Sum64(); best == "" || score > bestScore {
			best, bestScore = addr, score
		}
	}
	if best == set.self {
		return ""
	}
	return best
}

// acceptedHop reports whether r is a hop from a peer: it carries HopHeader and
// comes from one of the current peers' hosts. A hop header from anywhere else
// is ignored; neither proxy passes it on.
func (h *Handler) acceptedHop(r *http.Request) bool {
	if len(r.Header.Values(HopHeader)) == 0 {
		return false
	}
	set := h.peers.Load()
	if set == nil {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	return err == nil && set.hosts[host]
}

// forward sends r, whose cache key is key, to its owner and passes the owner's
// answer through unchanged, CF-Cache-Status and Age included.
func (h *Handler) forward(w http.ResponseWriter, r *http.Request, key, owner string) {
	ctx := context.WithValue(r.Context(), forwardKey{}, &forwarding{in: r, key: key, owner: owner})
	h.forwarder.ServeHTTP(w, r.WithContext(ctx))
}

// newForwarder is the proxy that carries a request to its key's owner. The
// owner sees the request as it came, its Accept-Encoding included (the
// transport adds none, so the owner computes the same key), with the
// X-Forwarded-* this replica would have sent the app, which the owner passes
// on verbatim. Only GET and HEAD are forwarded, so a failed forward has no
// body to replay and is served here.
func newForwarder(h *Handler) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			fw := pr.In.Context().Value(forwardKey{}).(*forwarding)
			pr.Out.URL.Scheme, pr.Out.URL.Host = "http", fw.owner
			pr.Out.Host = pr.In.Host
			setForwarded(pr)
			pr.Out.Header.Set(HopHeader, "1")
		},
		Transport: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: peerDialTimeout, KeepAlive: 30 * time.Second}).DialContext,
			DisableCompression:  true,
			MaxIdleConns:        256,
			MaxIdleConnsPerHost: 64,
			IdleConnTimeout:     90 * time.Second,
		},
		BufferPool:    &proxyBuffers,
		FlushInterval: -1,
		// The owner did not answer: serve the request here, as a replica
		// without peers would, unless its client is gone.
		ErrorHandler: func(w http.ResponseWriter, out *http.Request, _ error) {
			fw := out.Context().Value(forwardKey{}).(*forwarding)
			if fw.in.Context().Err() != nil {
				return
			}
			h.serve(w, fw.in, fw.key)
		},
	}
}

// RunPeers resolves name, the tier's headless Service, now and every interval
// until ctx ends, and makes its addresses, each at port, h's peers, with self,
// this pod's IP, at port as its own. A lookup that fails keeps the last peer
// set; logf, when not nil, reports each new peer set and a failure once until
// it changes.
func RunPeers(ctx context.Context, h *Handler, name, self string, port int, interval time.Duration,
	lookup func(context.Context, string) ([]string, error), logf func(format string, args ...any)) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	portText := strconv.Itoa(port)
	selfAddr := net.JoinHostPort(self, portText)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var last []string
	lastErr := ""
	for {
		ips, err := lookup(ctx, name)
		switch {
		case err == nil:
			peers := make([]string, 0, len(ips))
			for _, ip := range ips {
				peers = append(peers, net.JoinHostPort(ip, portText))
			}
			slices.Sort(peers)
			peers = slices.Compact(peers)
			h.SetPeers(selfAddr, peers)
			if last == nil || !slices.Equal(peers, last) {
				logf("edge-cache: peers %v (self %s)", peers, selfAddr)
			}
			last, lastErr = peers, ""
		case ctx.Err() == nil && err.Error() != lastErr:
			lastErr = err.Error()
			logf("edge-cache: resolve peers %s: %v", name, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
