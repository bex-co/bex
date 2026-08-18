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

package envgroups

import (
	"context"
	"testing"
)

func TestAuditNameClaimsDryRunApplyAndLegacyDuplicates(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	for gid, metadata := range map[string]map[string]string{
		"evg-d7a1g900000000000001": {"name": "unique", "workspace": "tea-a"},
		"evg-d7a1g900000000000002": {"name": "duplicate", "workspace": "tea-a"},
		"evg-d7a1g900000000000003": {"name": "duplicate", "workspace": "tea-a"},
	} {
		if err := store.Put(ctx, metaPath(gid), metadata); err != nil {
			t.Fatal(err)
		}
	}
	dry, err := AuditNameClaims(ctx, store, true)
	if err != nil {
		t.Fatal(err)
	}
	if dry.Scanned != 3 || dry.Missing != 1 || dry.Created != 0 || len(dry.Duplicates) != 1 || len(dry.Duplicates[0].IDs) != 2 {
		t.Fatalf("dry-run report = %+v", dry)
	}
	if claim, _ := store.Get(ctx, envGroupNameClaimPath("tea-a", "unique")); len(claim) != 0 {
		t.Fatalf("dry-run wrote claim: %v", claim)
	}
	applied, err := AuditNameClaims(ctx, store, false)
	if err != nil || applied.Created != 1 || applied.Missing != 1 || len(applied.Duplicates) != 1 {
		t.Fatalf("apply report = %+v err=%v", applied, err)
	}
	repeated, err := AuditNameClaims(ctx, store, false)
	if err != nil || repeated.Created != 0 || repeated.Existing != 1 || len(repeated.Duplicates) != 1 {
		t.Fatalf("idempotent report = %+v err=%v", repeated, err)
	}
	if claim, _ := store.Get(ctx, envGroupNameClaimPath("tea-a", "duplicate")); len(claim) != 0 {
		t.Fatalf("duplicate was assigned a winner: %v", claim)
	}
}
