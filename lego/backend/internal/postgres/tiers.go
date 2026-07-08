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

package postgres

import (
	"context"
	"regexp"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
)

// DatabaseInstanceType is the display-shaped projection of one lego/types/tiers
// Postgres tier — the create dialog's plan picker source, the managed-Postgres
// sibling of apps.InstanceType. Render's dashboard hardcodes its own Postgres
// instance-type list (no public REST/MCP equivalent to mirror byte-for-byte),
// so this is a bex extension, documented as such — never a fourth hardcoded
// copy of the ladder the w1/m8 catalog collapsed. ID is the Database CRD's
// spec.plan spelling (what createDatabase accepts), matching the other fields.
type DatabaseInstanceType struct {
	ID        string
	Name      string
	CPU       string
	Memory    string
	StorageGB int32
}

// InstanceTypes lists every plan in the shared Postgres catalog, in ladder
// order — read from lego/types/tiers, never a hardcoded copy here (the same
// rule apps.InstanceTypes follows for the compute family).
func (s *Service) InstanceTypes(ctx context.Context) ([]DatabaseInstanceType, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	ids := tiers.Postgres.IDs()
	out := make([]DatabaseInstanceType, len(ids))
	for i, id := range ids {
		t, _ := tiers.Postgres.ByID(id)
		out[i] = DatabaseInstanceType{
			ID:        t.ID,
			Name:      pgTierDisplayName(id),
			CPU:       t.CPU,
			Memory:    t.Memory,
			StorageGB: t.StorageGB,
		}
	}
	return out, nil
}

// sizeToken matches a trailing size token (256mb, 1gb, 2tb) whose unit should
// be upper-cased in the display name rather than title-cased ("256mb" reads as
// "256MB", not "256mb").
var sizeToken = regexp.MustCompile(`^\d+(mb|gb|tb)$`)

// pgTierDisplayName turns a Postgres plan id into its display spelling, e.g.
// "basic-256mb" -> "Basic 256MB", "free" -> "Free" (Render's Postgres
// family-size naming). The compute sibling (apps.tierDisplayName) title-cases
// hyphen-words; this additionally upper-cases the byte-size unit.
func pgTierDisplayName(id string) string {
	words := strings.Split(id, "-")
	for i, w := range words {
		switch {
		case w == "":
			continue
		case sizeToken.MatchString(w):
			words[i] = strings.ToUpper(w)
		default:
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
