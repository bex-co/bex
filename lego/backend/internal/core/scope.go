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

package core

import (
	"fmt"
	"slices"
	"strings"
)

// Closed OAuth capability vocabulary for third-party human tokens. These are
// the only control-plane scopes bex grants, advertises, or authorizes against.
// bex.api remains a platform-client compatibility alias and is never an
// umbrella grant for a non-platform client (w8/m27, docs/ADR012-auth.md §7).
const (
	ScopeRead             = "bex.read"
	ScopeWrite            = "bex.write"
	ScopeSensitive        = "bex.sensitive"
	ScopeAPICompatibility = "bex.api"
)

// Bounds on Hydra introspection values retained on Identity. Larger or
// malformed grants fail closed (the token introspects inactive) rather than
// being truncated into a different authorization decision.
const (
	MaxOAuthScopes      = 32
	MaxOAuthScopeLen    = 64
	MaxOAuthAudiences   = 8
	MaxOAuthAudienceLen = 256
	MaxOAuthClientIDLen = 128
)

// ClosedOAuthScopes is every scope the audit_events CHECK admits: the three
// granular capabilities plus the platform-client compatibility alias. Adding
// a scope requires a follow-on CHECK migration (0088's ARRAY; do not rewrite
// 0088 after it has shipped). TestAuditRelationCheckMatchesRelCan pins this.
func ClosedOAuthScopes() []string {
	return []string{ScopeRead, ScopeWrite, ScopeSensitive, ScopeAPICompatibility}
}

// AdvertisedScopes is the RFC 9728 scopes_supported list in one stable order.
// Discovery-driven clients request exactly these; bex.api is compatibility-only
// for platform-marked clients and is not advertised.
func AdvertisedScopes() []string {
	return GranularScopes()
}

// GranularScopes is the closed least-privilege vocabulary a third-party human
// token must carry at least one member of.
func GranularScopes() []string {
	return []string{ScopeRead, ScopeWrite, ScopeSensitive}
}

// relationCapability maps every RelCan… relation onto exactly one capability.
// An unknown relation must not inherit authority — RequiredCapability reports
// ok=false so the gate fails closed.
var relationCapability = map[string]string{
	RelCanView:          ScopeRead,
	RelCanViewLogs:      ScopeRead,
	RelCanOperate:       ScopeWrite,
	RelCanCreate:        ScopeWrite,
	RelCanViewSensitive: ScopeSensitive,
	RelCanManageKeys:    ScopeWrite,
	RelCanManageSSHKeys: ScopeWrite,
	RelCanManage:        ScopeWrite,
	RelCanManageBilling: ScopeWrite,
}

// RelCanRelations is every RelCan… constant. Tests range over it so a new
// relation cannot ship unmapped. The audit_events relation CHECK must list
// exactly these (see TestAuditRelationCheckMatchesRelCan).
func RelCanRelations() []string {
	return []string{
		RelCanView,
		RelCanViewLogs,
		RelCanOperate,
		RelCanCreate,
		RelCanViewSensitive,
		RelCanManageKeys,
		RelCanManageSSHKeys,
		RelCanManage,
		RelCanManageBilling,
	}
}

// RequiredCapability returns the OAuth capability relation needs. ok is false
// for an unknown relation — callers must fail closed rather than treating the
// miss as "no scope required".
func RequiredCapability(relation string) (string, bool) {
	cap, ok := relationCapability[relation]
	return cap, ok
}

// OAuthGrant is the bounded, normalized provenance extracted from a Hydra
// introspection. It never carries the bearer token or unbounded extra claims.
type OAuthGrant struct {
	// Scopes is the sorted, space-separated closed vocabulary present on the
	// token (granular capabilities plus bex.api when Hydra granted it).
	Scopes string
	// AcceptedAudience is the resource THIS API accepted, not the token's
	// full audience list.
	AcceptedAudience string
}

// HasGranular reports whether the grant carries at least one of the three
// least-privilege capabilities.
func (g OAuthGrant) HasGranular() bool {
	return HasGranularCapability(g.Scopes)
}

// HasGranularCapability reports whether a canonical scope string includes at
// least one of bex.read / bex.write / bex.sensitive. Near-matches (prefix,
// substring) do not count.
func HasGranularCapability(canonical string) bool {
	for _, s := range strings.Fields(canonical) {
		switch s {
		case ScopeRead, ScopeWrite, ScopeSensitive:
			return true
		}
	}
	return false
}

// ScopeList splits a canonical space-separated scope string.
func ScopeList(canonical string) []string {
	if canonical == "" {
		return nil
	}
	return strings.Fields(canonical)
}

// ContainsScope reports an exact member of a canonical scope string.
func ContainsScope(canonical, want string) bool {
	if want == "" {
		return false
	}
	return slices.Contains(strings.Fields(canonical), want)
}

// NormalizeOAuthGrant parses Hydra's space-separated scope and audience list
// into the closed vocabulary. Malformed, oversized, or ambiguous values return
// an error so the token is treated as inactive. retained is only the closed
// capability set plus the compatibility scope — identity scopes and unknown
// strings are dropped, never stored.
func NormalizeOAuthGrant(scope string, audiences []string, clientID, resource string) (OAuthGrant, error) {
	if len(clientID) > MaxOAuthClientIDLen {
		return OAuthGrant{}, fmt.Errorf("oauth client id exceeds %d bytes", MaxOAuthClientIDLen)
	}
	raw := strings.Fields(scope)
	if len(raw) > MaxOAuthScopes {
		return OAuthGrant{}, fmt.Errorf("oauth scope set exceeds %d members", MaxOAuthScopes)
	}
	seen := map[string]struct{}{}
	var kept []string
	for _, s := range raw {
		if len(s) > MaxOAuthScopeLen {
			return OAuthGrant{}, fmt.Errorf("oauth scope exceeds %d bytes", MaxOAuthScopeLen)
		}
		switch s {
		case ScopeRead, ScopeWrite, ScopeSensitive, ScopeAPICompatibility:
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			kept = append(kept, s)
		}
	}
	slices.Sort(kept)
	if len(audiences) > MaxOAuthAudiences {
		return OAuthGrant{}, fmt.Errorf("oauth audience list exceeds %d members", MaxOAuthAudiences)
	}
	accepted := ""
	if resource != "" {
		for _, a := range audiences {
			if len(a) > MaxOAuthAudienceLen {
				return OAuthGrant{}, fmt.Errorf("oauth audience exceeds %d bytes", MaxOAuthAudienceLen)
			}
			if a == resource {
				accepted = resource
				break
			}
		}
	}
	return OAuthGrant{
		Scopes:           strings.Join(kept, " "),
		AcceptedAudience: accepted,
	}, nil
}

// RequireCapability is the OAuth half of the shared authorization seam: a
// third-party human token must hold the capability mapped to relation, in
// addition to OpenFGA. Sessions, machine keys, and platform-marked clients
// that have not requested a granular capability are exempt — they keep their
// existing OpenFGA authority. An unknown relation fails closed.
func (id Identity) RequireCapability(relation string) error {
	if id.capabilityExempt() {
		return nil
	}
	want, ok := RequiredCapability(relation)
	if !ok {
		return NewInsufficientScopeError("")
	}
	if ContainsScope(id.CanonicalScopes, want) {
		return nil
	}
	return NewInsufficientScopeError(want)
}

// capabilityExempt reports whether this identity is outside the granular
// OAuth matrix: not a human OAuth delegation, or a platform-marked client
// that has not requested a granular capability (the documented rollout path
// for the official Render CLI and current mobile release).
func (id Identity) capabilityExempt() bool {
	if id.Method != "oauth2" || !id.Human {
		return true
	}
	if id.PlatformClient && !HasGranularCapability(id.CanonicalScopes) {
		return true
	}
	return false
}

// AttachOAuthProvenance copies the bounded OAuth grant facts onto an audit
// event. Session, machine, and system rows keep the fields empty.
func (id Identity) AttachOAuthProvenance(ev *AuditEvent) {
	if ev == nil || id.Method != "oauth2" {
		return
	}
	if !id.Human {
		return
	}
	ev.OAuthClientID = id.ClientID
	ev.OAuthAudience = id.AcceptedAudience
	ev.OAuthScopes = ScopeList(id.CanonicalScopes)
}
