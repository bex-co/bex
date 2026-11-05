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
	"context"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// UserIDLookup resolves identity subjects to the own- ids already minted for
// them, minting none (*store.PGStore.LookupOwnerIDs).
type UserIDLookup interface {
	LookupOwnerIDs(ctx context.Context, subjects []string) (map[string]string, error)
}

// PublicUserIDs maps each subject that has a user id to that id's Render
// usr- spelling (w5/m277), in one read. A subject without one — an API-key
// client, a removed person never listed — is absent, and callers keep what
// they showed before. A nil lookup resolves nothing.
func PublicUserIDs(ctx context.Context, lookup UserIDLookup, subjects []string) (map[string]string, error) {
	if lookup == nil || len(subjects) == 0 {
		return nil, nil
	}
	ownIDs, err := lookup.LookupOwnerIDs(ctx, subjects)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(ownIDs))
	for subject, own := range ownIDs {
		out[subject] = ids.UserPublicID(own)
	}
	return out, nil
}
