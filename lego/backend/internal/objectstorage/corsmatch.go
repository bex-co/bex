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

package objectstorage

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// A bucket's CORS rules, applied where bex serves it to browsers: the public gateway
// (ADR112 D4) and the S3 host's preflight (D3). Wasabi's own CORS stays off, so a browser
// at a customer's origin can read a public bucket only as far as the rules the customer
// stored say. The first rule that matches the request wins, as in S3. Credentials are
// never allowed: the gateway answers without Access-Control-Allow-Credentials, so a
// cross-origin read carries no cookies.

// CORSMatch is the rule a cross-origin request matched.
type CORSMatch struct {
	rule CORSRule
	// wildcard is true when the rule allows every origin, so the answer can say
	// "*" instead of echoing the caller's.
	wildcard bool
}

// MatchCORS finds the first rule that allows origin to make a request with the
// given method and, for a preflight, the given request headers. ok is false when
// no rule does, which is a response without any CORS header.
func MatchCORS(rules []CORSRule, origin, method string, requestHeaders []string) (m CORSMatch, ok bool) {
	if origin == "" {
		return CORSMatch{}, false
	}
	for _, rule := range rules {
		wildcard, allowedOrigin := originAllowed(rule.Allowed.Origins, origin)
		if !allowedOrigin || !slices.Contains(rule.Allowed.Methods, method) || !headersAllowed(rule.Allowed.Headers, requestHeaders) {
			continue
		}
		return CORSMatch{rule: rule, wildcard: wildcard}, true
	}
	return CORSMatch{}, false
}

// originAllowed reports whether any of the rule's origins admits origin, and
// whether it was the "*" origin that did. An origin may carry one "*" inside a
// host (https://*.example.com), which stands for one or more characters of it.
func originAllowed(allowed []string, origin string) (wildcard, ok bool) {
	for _, pattern := range allowed {
		switch {
		case pattern == "*":
			return true, true
		case strings.EqualFold(pattern, origin):
			ok = true
		case strings.Contains(pattern, "*"):
			prefix, suffix, _ := strings.Cut(pattern, "*")
			if len(origin) > len(prefix)+len(suffix) &&
				strings.EqualFold(origin[:len(prefix)], prefix) && strings.EqualFold(origin[len(origin)-len(suffix):], suffix) {
				ok = true
			}
		}
	}
	return false, ok
}

// headersAllowed reports whether every requested header is one the rule allows
// ("*" allows any). No request headers is always allowed.
func headersAllowed(allowed, requested []string) bool {
	if slices.Contains(allowed, "*") {
		return true
	}
	for _, h := range requested {
		if !slices.ContainsFunc(allowed, func(a string) bool { return strings.EqualFold(a, h) }) {
			return false
		}
	}
	return true
}

// SplitHeaderList reads a comma-separated header such as
// Access-Control-Request-Headers into its trimmed, non-empty names.
func SplitHeaderList(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// SetActual adds the headers an actual (non-preflight) cross-origin response
// carries.
func (m CORSMatch) SetActual(h http.Header, origin string) {
	h.Set("Access-Control-Allow-Origin", m.allowOrigin(origin))
	if len(m.rule.ExposeHeaders) > 0 {
		h.Set("Access-Control-Expose-Headers", strings.Join(m.rule.ExposeHeaders, ", "))
	}
}

// SetPreflight adds the headers a successful preflight answer carries.
func (m CORSMatch) SetPreflight(h http.Header, origin string, requestHeaders []string) {
	h.Set("Access-Control-Allow-Origin", m.allowOrigin(origin))
	h.Set("Access-Control-Allow-Methods", strings.Join(m.rule.Allowed.Methods, ", "))
	switch {
	case len(requestHeaders) > 0:
		// Echo what was asked: the rule has already admitted all of it.
		h.Set("Access-Control-Allow-Headers", strings.Join(requestHeaders, ", "))
	case len(m.rule.Allowed.Headers) > 0:
		h.Set("Access-Control-Allow-Headers", strings.Join(m.rule.Allowed.Headers, ", "))
	}
	if len(m.rule.ExposeHeaders) > 0 {
		h.Set("Access-Control-Expose-Headers", strings.Join(m.rule.ExposeHeaders, ", "))
	}
	if m.rule.MaxAgeSeconds != nil {
		h.Set("Access-Control-Max-Age", strconv.FormatInt(*m.rule.MaxAgeSeconds, 10))
	}
}

func (m CORSMatch) allowOrigin(origin string) string {
	if m.wildcard {
		return "*"
	}
	return origin
}
