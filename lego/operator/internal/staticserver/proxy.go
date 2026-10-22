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
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/types/netutil"
)

// External rewrites (w8/184). Render's redirects-and-rewrites page says a
// rewrite's destination "can be either a path or a full, publicly accessible
// URL", and serves the destination's content at the original path: the way a
// static frontend reaches its API without CORS. The static server therefore
// fetches the expanded URL itself, for any method, and streams the answer back.
//
// The fetch is the server-side-request-forgery surface of the product, so it is
// closed three ways: validation refuses a literal non-public host when a rule is
// saved; the dialer resolves the host, refuses every non-public address
// (loopback, private, link-local and metadata ranges, cluster-internal names
// resolve to private addresses) and dials the checked address, so a DNS answer
// cannot change between check and use; and the proxy never follows a redirect
// (it passes the 3xx through, so the browser decides), never re-enters the
// platform's own static hosts, and carries a hop marker that refuses a loop.

const (
	// proxyHopHeader marks a request this server proxied, so a destination that
	// leads back here is refused instead of amplified.
	proxyHopHeader = "X-Bex-Static-Proxy"
	// maxProxyBodyBytes bounds a proxied request body, and maxProxyResponseBytes
	// the response streamed back: the static budgets, not an open relay.
	maxProxyBodyBytes     = 32 << 20
	maxProxyResponseBytes = 64 << 20
	proxyTimeout          = 60 * time.Second
)

// hopByHop are the headers a proxy must not forward (RFC 9110 §7.6.1).
var hopByHop = []string{
	"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
	"Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

// externalProxy fetches an external rewrite destination.
type externalProxy struct {
	client *http.Client
}

// newExternalProxy builds the proxy over the platform's SSRF dialer; unsafe
// decides which resolved addresses are refused (netutil.UnsafeOriginIP in
// production, a looser one in tests that dial a loopback server).
func newExternalProxy(unsafe func(net.IP) bool) *externalProxy {
	return &externalProxy{client: &http.Client{
		Transport: &http.Transport{
			DialContext:           netutil.SafeDialContextFunc(10*time.Second, unsafe),
			ForceAttemptHTTP2:     true,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			MaxIdleConns:          64,
			MaxIdleConnsPerHost:   8,
			IdleConnTimeout:       30 * time.Second,
		},
		// A redirect is the destination's answer, not this server's to follow: it
		// would be a second, unvalidated destination.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// serveExternalRewrite proxies r to target and writes the answer, with the
// site's custom headers applied to it as to any response. Errors answer 502
// (an unreachable or refused destination) or 508 (a loop).
func (h *Handler) serveExternalRewrite(w http.ResponseWriter, r *http.Request, site Site, requestPath string, target *url.URL) {
	if r.Header.Get(proxyHopHeader) != "" {
		serveSiteError(w, site, requestPath, http.StatusLoopDetected, "rewrite loop")
		return
	}
	// A destination on one of the platform's own static hosts would re-enter this
	// server for another tenant's site; refuse it by name, ahead of any dial.
	if _, own := h.resolver.Resolve(hostOnly(target.Host)); own {
		serveSiteError(w, site, requestPath, http.StatusLoopDetected, "rewrite loop")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), proxyTimeout)
	defer cancel()
	body := io.Reader(nil)
	if r.Body != nil && r.Body != http.NoBody {
		body = http.MaxBytesReader(w, r.Body, maxProxyBodyBytes)
	}
	out, err := http.NewRequestWithContext(ctx, r.Method, proxyURL(target, r.URL.RawQuery).String(), body)
	if err != nil {
		serveSiteError(w, site, requestPath, http.StatusBadGateway, "bad gateway")
		return
	}
	if body != nil {
		out.ContentLength = r.ContentLength
	}
	for name, values := range r.Header {
		out.Header[name] = append([]string(nil), values...)
	}
	stripHopByHop(out.Header)
	out.Host = target.Host
	out.Header.Set(proxyHopHeader, "1")
	out.Header.Set("X-Forwarded-Host", r.Host)
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		out.Header.Set("X-Forwarded-Proto", proto)
	}

	resp, err := h.external.client.Do(out)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			serveSiteError(w, site, requestPath, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		serveSiteError(w, site, requestPath, http.StatusBadGateway, "bad gateway")
		return
	}
	defer func() { _ = resp.Body.Close() }()

	hdr := w.Header()
	for name, values := range resp.Header {
		hdr[name] = append([]string(nil), values...)
	}
	stripHopByHop(hdr)
	hdr.Del("Content-Length") // re-framed by the response writer below
	applyHeaders(hdr, site.Headers, requestPath)
	w.WriteHeader(resp.StatusCode)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(w, io.LimitReader(resp.Body, maxProxyResponseBytes))
}

// proxyURL joins the destination's own query with the visitor's.
func proxyURL(target *url.URL, visitorQuery string) *url.URL {
	u := *target
	switch {
	case u.RawQuery == "":
		u.RawQuery = visitorQuery
	case visitorQuery != "":
		u.RawQuery += "&" + visitorQuery
	}
	return &u
}

func stripHopByHop(h http.Header) {
	// Names the Connection header lists are hop-by-hop too.
	for _, value := range h.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			if name = strings.TrimSpace(name); name != "" {
				h.Del(name)
			}
		}
	}
	for _, name := range hopByHop {
		h.Del(name)
	}
}
