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

// Property tests over the OAuth capability matrix (ADR012). scope_test.go pins
// the enumerated cases; these pin the rules that must hold for EVERY input:
// the grant is a closed, sorted, de-duplicated vocabulary and normalizing it
// twice changes nothing; every bound is exact (accept at the bound, reject one
// over); the advertised set and the enforced set are the same set; and every
// RelCan… relation is satisfied by a granular capability a human token can
// actually request — never by the compatibility alias or a non-relation scope.

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestRequiredCapabilityIsAlwaysGranularAndNeverCompatibility(t *testing.T) {
	granular := map[string]bool{}
	for _, s := range AdvertisedScopes() {
		granular[s] = true
	}
	if len(granular) != len(AdvertisedScopes()) {
		t.Fatal("AdvertisedScopes has duplicates")
	}
	for _, rel := range RelCanRelations() {
		capability, ok := RequiredCapability(rel)
		if !ok {
			t.Errorf("relation %q is unmapped", rel)
			continue
		}
		if !granular[capability] {
			t.Errorf("relation %q requires %q, which a human token cannot request as a granular capability", rel, capability)
		}
		if capability == ScopeAPICompatibility {
			t.Errorf("relation %q is satisfied by the compatibility alias %q", rel, ScopeAPICompatibility)
		}
		if capability == ScopeCloudAgents {
			t.Errorf("relation %q is satisfied by %q, which must only cover the cloud-agents toolset", rel, ScopeCloudAgents)
		}
	}
}

func TestAdvertisedScopesIsACloneOfTheClosedVocabulary(t *testing.T) {
	got := AdvertisedScopes()
	want := []string{ScopeRead, ScopeWrite, ScopeSensitive, ScopeCloudAgents}
	if !slices.Equal(got, want) {
		t.Fatalf("AdvertisedScopes = %v, want %v", got, want)
	}
	closed := map[string]bool{}
	for _, s := range ClosedOAuthScopes() {
		closed[s] = true
	}
	for _, s := range got {
		if !closed[s] {
			t.Errorf("advertised %q is not admitted by the audit CHECK", s)
		}
	}
	if slices.Contains(got, ScopeAPICompatibility) {
		t.Error("the compatibility alias must not be advertised")
	}

	got[0] = "mutated"
	if again := AdvertisedScopes(); again[0] != ScopeRead {
		t.Fatal("AdvertisedScopes must return a clone, not its backing slice")
	}
}

func TestHasGranularCapabilityExhaustiveOverSubsets(t *testing.T) {
	closed := ClosedOAuthScopes()
	granular := map[string]bool{}
	for _, s := range AdvertisedScopes() {
		granular[s] = true
	}
	for mask := range 1 << len(closed) {
		var kept []string
		want := false
		for i, s := range closed {
			if mask&(1<<i) != 0 {
				kept = append(kept, s)
				want = want || granular[s]
			}
		}
		scope := strings.Join(kept, " ")
		if got := HasGranularCapability(scope); got != want {
			t.Fatalf("HasGranularCapability(%q) = %v, want %v", scope, got, want)
		}
	}
}

func TestNormalizeOAuthGrantIsClosedSortedAndIdempotent(t *testing.T) {
	closed := map[string]bool{}
	for _, s := range ClosedOAuthScopes() {
		closed[s] = true
	}
	cases := []struct {
		name      string
		scope     string
		audiences []string
		clientID  string
		resource  string
	}{
		{name: "no scope", clientID: "c", resource: "https://api.bex.co/mcp"},
		{name: "identity scopes only", scope: "openid profile email", clientID: "c"},
		{name: "duplicates", scope: "bex.write bex.read bex.write bex.read", clientID: "c"},
		{name: "unknown and near-matches dropped", scope: "bex.bogus bex.read-only bex bex.api bex.read", clientID: "c"},
		{name: "audience matched", scope: "bex.sensitive", audiences: []string{"https://other", "https://api.bex.co/mcp"}, clientID: "c", resource: "https://api.bex.co/mcp"},
		{name: "audience unmatched", scope: "bex.read", audiences: []string{"https://other"}, clientID: "c", resource: "https://api.bex.co/mcp"},
		{name: "whole closed vocabulary", scope: strings.Join(ClosedOAuthScopes(), " "), audiences: []string{"https://api.bex.co/mcp"}, clientID: "c", resource: "https://api.bex.co/mcp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, err := NormalizeOAuthGrant(tc.scope, tc.audiences, tc.clientID, tc.resource)
			if err != nil {
				t.Fatalf("NormalizeOAuthGrant: %v", err)
			}
			members := ScopeList(g.Scopes)
			if !slices.IsSorted(members) {
				t.Errorf("%q is not sorted", g.Scopes)
			}
			for i, m := range members {
				if !closed[m] {
					t.Errorf("%q escaped the closed vocabulary", m)
				}
				if i > 0 && members[i-1] == m {
					t.Errorf("%q is duplicated", m)
				}
			}
			for _, semantic := range []string{"openid", "profile", "email", "offline_access"} {
				if ContainsScope(g.Scopes, semantic) {
					t.Errorf("identity scope %q was retained", semantic)
				}
			}
			// Idempotent on its own output: a second normalization is a no-op,
			// so re-deriving provenance from a stored grant cannot drift.
			var audiences []string
			if g.AcceptedAudience != "" {
				audiences = []string{g.AcceptedAudience}
			}
			again, err := NormalizeOAuthGrant(g.Scopes, audiences, tc.clientID, tc.resource)
			if err != nil {
				t.Fatalf("re-normalize: %v", err)
			}
			if again != g {
				t.Fatalf("not idempotent: %+v -> %+v", g, again)
			}
		})
	}
}

func TestNormalizeOAuthGrantBoundsAreExact(t *testing.T) {
	// Each bound accepts exactly-at and rejects one-over; an off-by-one in
	// either direction either drops a legitimate grant or retains an unbounded
	// one.
	atScopeCap := strings.Repeat("bex.read ", MaxOAuthScopes)
	if _, err := NormalizeOAuthGrant(atScopeCap, nil, "c", ""); err != nil {
		t.Fatalf("scope set at the cap (%d members): %v", MaxOAuthScopes, err)
	}
	if _, err := NormalizeOAuthGrant(atScopeCap+"bex.read", nil, "c", ""); err == nil {
		t.Fatal("scope set one over the cap must fail closed")
	}

	atScopeLen := strings.Repeat("x", MaxOAuthScopeLen)
	if _, err := NormalizeOAuthGrant(atScopeLen, nil, "c", ""); err != nil {
		t.Fatalf("scope at the length cap: %v", err)
	}
	if _, err := NormalizeOAuthGrant(atScopeLen+"x", nil, "c", ""); err == nil {
		t.Fatal("scope one byte over the length cap must fail closed")
	}

	audiences := make([]string, MaxOAuthAudiences)
	for i := range audiences {
		audiences[i] = fmt.Sprintf("https://audience-%d", i)
	}
	if _, err := NormalizeOAuthGrant("bex.read", audiences, "c", audiences[0]); err != nil {
		t.Fatalf("audience list at the cap: %v", err)
	}
	if _, err := NormalizeOAuthGrant("bex.read", append(audiences, "https://extra"), "c", audiences[0]); err == nil {
		t.Fatal("audience list one over the cap must fail closed")
	}

	longAudience := strings.Repeat("a", MaxOAuthAudienceLen)
	if _, err := NormalizeOAuthGrant("bex.read", []string{longAudience}, "c", longAudience); err != nil {
		t.Fatalf("audience at the length cap: %v", err)
	}
	if _, err := NormalizeOAuthGrant("bex.read", []string{longAudience + "a"}, "c", longAudience+"a"); err == nil {
		t.Fatal("audience one byte over the length cap must fail closed")
	}

	atClientCap := strings.Repeat("c", MaxOAuthClientIDLen)
	if _, err := NormalizeOAuthGrant("bex.read", nil, atClientCap, ""); err != nil {
		t.Fatalf("client id at the cap: %v", err)
	}
	if _, err := NormalizeOAuthGrant("bex.read", nil, atClientCap+"c", ""); err == nil {
		t.Fatal("client id one byte over the cap must fail closed")
	}
}
