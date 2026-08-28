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

package api

import (
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// withSecurityHeaders adds standard hardening headers to every response.
//
// HSTS is emitted only for a request that is genuinely served over TLS: either
// the connection itself terminated TLS at this process (r.TLS != nil), or the
// immediate peer is a TRUSTED proxy that forwarded X-Forwarded-Proto: https.
// Gating on the trusted-peer check (codex-security target #10) is what stops an
// unauthenticated client from forging X-Forwarded-Proto to elicit — or, absent
// the check, to suppress — HSTS; a spoofed header from an untrusted peer is
// ignored exactly as the rate limiter ignores a spoofed X-Forwarded-For. In
// plain-HTTP local dev (no trusted proxies) neither condition holds, so HSTS
// never fires — byte-identical to before for that path.
func withSecurityHeaders(trusted core.TrustedProxies, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		// bex-api is a pure JSON/SSE API: deny all document-loading sources.
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if requestIsHTTPS(trusted, r) {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// requestIsHTTPS reports whether the request reached bex-api over TLS, believing
// X-Forwarded-Proto only when the immediate peer is a configured trusted proxy.
func requestIsHTTPS(trusted core.TrustedProxies, r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return trusted.TrustsPeer(r) && r.Header.Get("X-Forwarded-Proto") == "https"
}
