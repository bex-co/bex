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

package sshgateway

import (
	"net"
	"net/http"
	"time"
)

const (
	// upstreamDialTimeout bounds the dial and the TLS handshake — the two phases
	// where a hung upstream would otherwise pin a proxy goroutine indefinitely.
	upstreamDialTimeout = 15 * time.Second
	// upstreamIdleConnTimeout keeps a pooled connection warm between requests.
	// It only has an effect when the client is reused; see NewUpstreamClient.
	upstreamIdleConnTimeout = 90 * time.Second
)

// NewUpstreamClient builds the HTTP client the credential-injecting proxies use
// for their upstream hop (the Git smart-HTTP proxy, ADR047 D2; the model proxy,
// ADR062).
//
// A total http.Client.Timeout is deliberately absent: it bounds the whole
// exchange INCLUDING the response body, so a large `git clone` whose upload-pack
// legitimately streams past the budget is truncated mid-transfer ("fatal: early
// EOF"), and a model turn's SSE response is cut off the same way. The phases
// that guard against a hung upstream — dial, TLS handshake, and
// time-to-first-response-header — are bounded instead, and the request context
// still bounds overall lifetime.
//
// Callers must build this ONCE (sync.Once) and reuse it: a per-request client
// discards its connection pool, so every request re-dials and re-runs the TLS
// handshake and upstreamIdleConnTimeout above means nothing.
func NewUpstreamClient(responseHeaderTimeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: upstreamDialTimeout}).DialContext,
			TLSHandshakeTimeout:   upstreamDialTimeout,
			ResponseHeaderTimeout: responseHeaderTimeout,
			IdleConnTimeout:       upstreamIdleConnTimeout,
		},
		// Never auto-follow: a redirect would replay the injected credential to
		// whatever host the upstream names.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
