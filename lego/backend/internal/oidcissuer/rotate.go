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

package oidcissuer

import (
	"context"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// RotationStore is KeyStore plus the rotation write.
type RotationStore interface {
	KeyStore
	RotateOIDCSigningKey(ctx context.Context, kid string, privateKey []byte, lead time.Duration, replace bool) (store.OIDCSigningKey, error)
}

// KeyStatus is one key as the rotation command reports it.
type KeyStatus struct {
	KID         string    `json:"kid"`
	State       State     `json:"state"`
	CreatedAt   time.Time `json:"createdAt,omitzero"`
	ActivatesAt time.Time `json:"activatesAt"`
}

// RotationReport is what `/api oidc-rotate-key` prints.
type RotationReport struct {
	Applied   bool        `json:"applied"`
	Emergency bool        `json:"emergency"`
	Keys      []KeyStatus `json:"keys"`             // the key set before the rotation
	NewKey    *KeyStatus  `json:"newKey,omitempty"` // the key the rotation adds (or would add)
	// SupersededRemovedAt is when the key signing now stops being published:
	// RetireAfter after the new key starts signing. Zero for an emergency
	// rotation, which removes every other key at once.
	SupersededRemovedAt time.Time `json:"supersededRemovedAt,omitzero"`
}

// Rotate adds a signing key (ADR116 D6). A normal rotation publishes it now
// and lets it sign PublishLead later; the key it supersedes stays published
// until RetireAfter after that. An emergency rotation signs at once and
// deletes every other key, so nothing signed before verifies. Without apply
// it reports the plan and writes nothing. A rotation refuses while an earlier
// one's key has not started signing.
func Rotate(ctx context.Context, st RotationStore, now time.Time, emergency, apply bool) (RotationReport, error) {
	rows, err := st.OIDCSigningKeys(ctx)
	if err != nil {
		return RotationReport{}, err
	}
	report := RotationReport{Applied: apply, Emergency: emergency, Keys: []KeyStatus{}}
	keys := make([]Key, 0, len(rows))
	for _, row := range rows {
		k, err := parseKey(row)
		if err != nil {
			return RotationReport{}, err
		}
		keys = append(keys, k)
	}
	pending := false
	for _, c := range Classify(keys, now) {
		report.Keys = append(report.Keys, KeyStatus{KID: c.ID, State: c.State, CreatedAt: c.CreatedAt, ActivatesAt: c.ActivatesAt})
		pending = pending || c.State == StateNext
	}
	if pending && !emergency {
		return report, store.ErrOIDCRotationPending
	}
	lead := PublishLead
	if emergency {
		lead = 0
	}
	kid, der, err := NewKey()
	if err != nil {
		return RotationReport{}, err
	}
	report.NewKey = &KeyStatus{KID: kid, State: StateNext, ActivatesAt: now.Add(lead)}
	if apply {
		row, err := st.RotateOIDCSigningKey(ctx, kid, der, lead, emergency)
		if err != nil {
			return report, err
		}
		report.NewKey.CreatedAt, report.NewKey.ActivatesAt = row.CreatedAt, row.ActivatesAt
		if emergency {
			report.NewKey.State = StateCurrent
		}
	}
	if !emergency {
		report.SupersededRemovedAt = report.NewKey.ActivatesAt.Add(RetireAfter)
	}
	return report, nil
}
