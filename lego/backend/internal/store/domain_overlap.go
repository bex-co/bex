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

package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"golang.org/x/net/publicsuffix"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ErrDomainCovered refuses to promote a custom domain that a verified domain
// of another workspace overlaps through a wildcard (w4/m326, ADR005 §
// Wildcard domains). The claim stays pending.
var ErrDomainCovered = errors.New("domain overlaps a verified domain of another workspace")

// domainOverlapDomain is the name shared by every host that can overlap host
// through a wildcard: the shortest suffix of the name host serves (a
// wildcard's base) that is a registrable domain. For an ordinary host that is
// its registrable domain (eTLD+1). Taking the shortest keeps it shared where
// the public-suffix list puts a suffix inside a registrable domain, so a
// wildcard and every name under it, at any depth, have the same one.
func domainOverlapDomain(host string) string {
	name := strings.TrimPrefix(host, appv1alpha1.WildcardHostPrefix)
	labels := strings.Split(name, ".")
	for i := len(labels) - 2; i >= 0; i-- {
		suffix := strings.Join(labels[i:], ".")
		if registrable, err := publicsuffix.EffectiveTLDPlusOne(suffix); err == nil && registrable == suffix {
			return suffix
		}
	}
	return name
}

// wildcardsOver lists the wildcards that cover name at any depth:
// "*.<parent>" for each parent of name above its top-level domain.
func wildcardsOver(name string) []string {
	out := []string{}
	for _, parent, ok := strings.Cut(name, "."); ok && strings.Contains(parent, "."); _, parent, ok = strings.Cut(parent, ".") {
		out = append(out, appv1alpha1.WildcardHostPrefix+parent)
	}
	return out
}

// verifiedOverlapElsewhere reports whether a verified domain of a workspace
// other than appID's overlaps host: a wildcard above it at any depth or, when
// host is a wildcard, any name under its base. Within one workspace a name
// under its own wildcard routes to another of its services, and is allowed.
func verifiedOverlapElsewhere(ctx context.Context, tx pgx.Tx, appID, host string) (bool, error) {
	name, wild := strings.CutPrefix(host, appv1alpha1.WildcardHostPrefix)
	under := ""
	if wild {
		under = "." + name
	}
	var covered bool
	err := tx.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM domains d JOIN apps a ON a.id = d.app_id
		   WHERE d.claim_state = 'verified'
		     AND a.tenant_id <> (SELECT tenant_id FROM apps WHERE id = $1)
		     AND (d.host = ANY($2::text[]) OR ($3 <> '' AND right(d.host, char_length($3)) = $3)))`,
		appID, wildcardsOver(name), under,
	).Scan(&covered)
	return covered, err
}

// ForeignWildcardCovers reports whether a wildcard claim of a workspace
// other than workspaceID, pending or verified, covers the concrete host at
// any depth. A non-apex name under one verifies only through a CNAME to its
// own service (ADR005 § Wildcard domains).
func (s *PGStore) ForeignWildcardCovers(ctx context.Context, workspaceID, host string) (bool, error) {
	var covered bool
	err := s.Pool.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM domains d JOIN apps a ON a.id = d.app_id
		   WHERE d.host = ANY($2::text[]) AND a.tenant_id <> $1)`,
		workspaceID, wildcardsOver(host),
	).Scan(&covered)
	return covered, err
}
