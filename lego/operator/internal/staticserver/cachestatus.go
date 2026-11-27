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
	"net/http"
	"strings"

	"github.com/bex-co/bex/lego/operator/internal/edgecache"
)

// Every answer the static server sends reports how Render's CDN would have
// answered it, in CF-Cache-Status (w4/m703, captured on Render's static sites
// 2026-11-27): a published file reads MISS when this request fetched it from
// the origin and HIT when it came from the pod's cache; any other answer to a
// GET or HEAD, a missing file's 404 (its 404.html page too), a route
// redirect, an error and the suspended site's 503, reads BYPASS; and an answer
// to any other method, or one an external rewrite fetched elsewhere, DYNAMIC.
// A header rule cannot change it, as Cloudflare overwrites an origin's.
//
// A revision's objects never change, so the cache never expires one, where
// Render's CDN revalidates a file after its s-maxage and reports EXPIRED once;
// no answer carries the Age Render's HIT does either.

// defaultCacheStatus is the status of an answer to method that is not a
// published file.
func defaultCacheStatus(method string) string {
	if method == http.MethodGet || method == http.MethodHead {
		return "BYPASS"
	}
	return "DYNAMIC"
}

// fileCacheStatus is a published file's status: MISS or HIT by whether it
// came from the cache, unless its final headers opt out of a CDN's cache
// (a no-store, private or no-cache rule, a zero lifetime, a Set-Cookie or a
// Vary past Accept-Encoding), which Render's edge reports as BYPASS. A HEAD, a
// 304 and a 206 read as the file they answer for.
func fileCacheStatus(cached bool, hdr http.Header) string {
	if ttl, status := edgecache.ResponseTTL(http.StatusOK, hdr); ttl <= 0 {
		return status
	}
	if cached {
		return "HIT"
	}
	return "MISS"
}

// edgeOwnedHeader reports whether a header rule names a header Render's edge
// overwrites: the API accepts the rule, and it has no effect.
func edgeOwnedHeader(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), edgecache.StatusHeader)
}
