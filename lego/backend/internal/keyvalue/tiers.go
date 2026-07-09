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

package keyvalue

import (
	"context"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
)

// KeyValueInstanceType is the display-shaped projection of one lego/types/tiers
// Valkey tier — the create dialog's plan-picker source, the managed key-value
// sibling of postgres.DatabaseInstanceType. Render's dashboard hardcodes its own
// Key Value plan list (no public REST/MCP equivalent to mirror byte-for-byte),
// so this is a bex extension, documented as such — never a fourth hardcoded copy
// of the ladder the tiers catalog collapsed. ID is the KeyValue CRD's spec.plan
// spelling (what createKeyValue accepts), matching the other fields.
type KeyValueInstanceType struct {
	ID        string
	Name      string
	CPU       string
	Memory    string
	StorageGB int32
}

// InstanceTypes lists every plan in the shared Valkey catalog, in ladder order —
// read from lego/types/tiers, never a hardcoded copy here (the same rule
// apps.InstanceTypes and postgres.InstanceTypes follow).
func (s *Service) InstanceTypes(ctx context.Context) ([]KeyValueInstanceType, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	ids := tiers.Valkey.IDs()
	out := make([]KeyValueInstanceType, len(ids))
	for i, id := range ids {
		t, _ := tiers.Valkey.ByID(id)
		out[i] = KeyValueInstanceType{
			ID:        t.ID,
			Name:      kvTierDisplayName(id),
			CPU:       t.CPU,
			Memory:    t.Memory,
			StorageGB: t.StorageGB,
		}
	}
	return out, nil
}

// kvTierDisplayName title-cases a Valkey plan id for display, e.g. "free" ->
// "Free", "standard" -> "Standard". Valkey plan ids are single lowercase words
// (Render's Key Value vocabulary), so a plain title-case suffices — no
// byte-size-unit handling like the Postgres family needs.
func kvTierDisplayName(id string) string {
	if id == "" {
		return id
	}
	return strings.ToUpper(id[:1]) + id[1:]
}
