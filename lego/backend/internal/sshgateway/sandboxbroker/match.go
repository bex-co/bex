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

package sandboxbroker

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/sandbox"
)

// NormalizeHost lowercases a hostname and strips whitespace and a trailing dot.
func NormalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

// HasCredentialForHost reports whether any enabled broker credential targets
// host by exact lowercase hostname match.
func HasCredentialForHost(creds []sandbox.BrokerCredential, host string) bool {
	target := NormalizeHost(host)
	if target == "" {
		return false
	}
	for _, c := range creds {
		if NormalizeHost(c.Domain) == target {
			return true
		}
	}
	return false
}

// MatchCredential selects the single winning credential for an outbound HTTPS
// (port 443) request using Render's matching semantics (domain, methods, paths)
// and ADR122 D5's deterministic single-match non-combination rule
// (lexicographically smallest ID wins). overlap is true when at least two
// credentials matched the request.
func MatchCredential(creds []sandbox.BrokerCredential, host, method, reqPath string) (*sandbox.BrokerCredential, bool) {
	target := NormalizeHost(host)
	if target == "" || len(creds) == 0 {
		return nil, false
	}
	normMethod := strings.ToUpper(strings.TrimSpace(method))
	cleanPath := normalizeRequestPath(reqPath)

	ordered := creds
	if !slices.IsSortedFunc(creds, func(a, b sandbox.BrokerCredential) int {
		return cmp.Compare(a.ID, b.ID)
	}) {
		ordered = slices.Clone(creds)
		slices.SortFunc(ordered, func(a, b sandbox.BrokerCredential) int {
			return cmp.Compare(a.ID, b.ID)
		})
	}

	var winner *sandbox.BrokerCredential
	for i := range ordered {
		c := &ordered[i]
		if NormalizeHost(c.Domain) != target {
			continue
		}
		if !matchMethod(c.Methods, normMethod) {
			continue
		}
		if !matchAnyPath(c.Paths, cleanPath) {
			continue
		}
		if winner != nil {
			return winner, true
		}
		copyCred := *c
		winner = &copyCred
	}
	return winner, false
}

func matchMethod(allowed []string, normMethod string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, m := range allowed {
		if strings.EqualFold(m, normMethod) {
			return true
		}
	}
	return false
}

func matchAnyPath(patterns []string, cleanPath string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pat := range patterns {
		if matchCleanPath(pat, cleanPath) {
			return true
		}
	}
	return false
}

// MatchPath evaluates one Render credential path pattern against a request
// path (ignoring query strings and normalizing dot segments against traversal).
//
// Semantics (Render Public OpenAPI):
//   - Pattern matches the entire absolute path.
//   - `*` matches exactly one nonempty path segment.
//   - `**` matches one or more nonempty path segments.
//   - Example: `/a/*` matches `/a/b`; `/a/**` matches `/a/b` and `/a/b/c`;
//     neither matches `/a`.
func MatchPath(pattern, reqPath string) bool {
	return matchCleanPath(pattern, normalizeRequestPath(reqPath))
}

func normalizeRequestPath(raw string) string {
	// Strip query or fragment defensively; query strings never affect path match.
	if idx := strings.IndexAny(raw, "?#"); idx >= 0 {
		raw = raw[:idx]
	}
	if raw == "" {
		return "/"
	}
	if raw[0] != '/' {
		raw = "/" + raw
	}
	hadTrailingSlash := len(raw) > 1 && strings.HasSuffix(raw, "/")
	endsWithDot := strings.HasSuffix(raw, "/.") || strings.HasSuffix(raw, "/..")
	cleaned := path.Clean(raw)
	if cleaned == "." || cleaned == "" {
		return "/"
	}
	if hadTrailingSlash && !endsWithDot && cleaned != "/" {
		cleaned += "/"
	}
	return cleaned
}

func splitPathSegments(p string) []string {
	if p == "/" {
		return nil
	}
	return strings.Split(p[1:], "/")
}

func matchCleanPath(pattern, cleanPath string) bool {
	if pattern == "" || pattern[0] != '/' {
		return false
	}
	patSegs := splitPathSegments(pattern)
	reqSegs := splitPathSegments(cleanPath)

	n, m := len(patSegs), len(reqSegs)
	// Rolling 1D DP: next[j] holds dp[i+1][j], cur[j] holds dp[i][j].
	next := make([]bool, m+1)
	cur := make([]bool, m+1)
	next[m] = true
	for i := n - 1; i >= 0; i-- {
		clear(cur)
		for j := m - 1; j >= 0; j-- {
			switch patSegs[i] {
			case "**":
				// One or more nonempty path segments.
				cur[j] = reqSegs[j] != "" && (next[j+1] || cur[j+1])
			case "*":
				// Exactly one nonempty path segment.
				cur[j] = reqSegs[j] != "" && next[j+1]
			default:
				cur[j] = patSegs[i] == reqSegs[j] && next[j+1]
			}
		}
		next, cur = cur, next
	}
	return next[0]
}

// AllowsDestination reports whether policy allows outbound HTTPS to host.
// Fails closed on deny-all or any unrecognized policy default.
func AllowsDestination(policy sandbox.NetworkPolicy, host string) bool {
	target := NormalizeHost(host)
	if target == "" {
		return false
	}
	switch policy.Default {
	case sandbox.NetworkPolicyAllowAll:
		return true
	case sandbox.NetworkPolicyDenyAll:
		return false
	case sandbox.NetworkPolicyAllowList:
		for _, rule := range policy.Rules {
			if rule.Protocol != "" && rule.Protocol != sandbox.NetworkPolicyProtocolHTTPS {
				continue
			}
			if matchAllowListDomain(rule.Domain, target) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// matchAllowListDomain matches Render's allow-list domain rules (matching
// sessionegress.SandboxAllowList): exact match, or a single leftmost `*.`
// wildcard label covering exactly one subdomain level and NOT the apex.
func matchAllowListDomain(pattern, host string) bool {
	pat := NormalizeHost(pattern)
	if pat == "" || host == "" {
		return false
	}
	if !strings.HasPrefix(pat, "*.") {
		return pat == host
	}
	suffix := pat[1:] // ".example.com"
	if !strings.HasSuffix(host, suffix) || len(host) <= len(suffix) {
		return false
	}
	label := host[:len(host)-len(suffix)]
	return label != "" && !strings.Contains(label, ".")
}
