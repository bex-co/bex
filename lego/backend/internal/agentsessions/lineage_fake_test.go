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

package agentsessions

import (
	"context"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The in-memory fake's side of the ADR102 lineage store methods. The real
// rules (limits, locks, spend) are tested against Postgres; the fake only
// answers what the non-lineage tests ask of it.

func (f *fakeStore) ListAgentSessionChildren(_ context.Context, _, parentID string) ([]store.AgentSession, error) {
	f.rowMu.Lock()
	defer f.rowMu.Unlock()
	var out []store.AgentSession
	for _, r := range f.rows {
		if r.ParentSessionID == parentID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeStore) CountLiveAgentSessionChildren(ctx context.Context, ws, parentID string) (int, error) {
	children, _ := f.ListAgentSessionChildren(ctx, ws, parentID)
	n := 0
	for _, c := range children {
		for _, p := range store.ActiveAgentSessionPhases {
			if c.Phase == p {
				n++
			}
		}
	}
	return n, nil
}

func (f *fakeStore) AgentSessionSubtree(context.Context, string, string) ([]store.AgentSession, error) {
	return nil, nil
}

func (f *fakeStore) ListUnfinishedAgentSessionDescendants(context.Context, string, string) ([]store.AgentSession, error) {
	return nil, nil
}

func (f *fakeStore) SetAgentSessionCancelCause(context.Context, string, string, string) error {
	return nil
}

func (f *fakeStore) AgentSessionCeilingHeadroom(context.Context, string, string) (int64, bool, error) {
	return 0, false, nil
}

func (f *fakeStore) IsAgentSessionDescendant(_ context.Context, _, ancestorID, id string) (bool, error) {
	f.rowMu.Lock()
	defer f.rowMu.Unlock()
	for hops := 0; hops < 8; hops++ {
		row, ok := f.rows[id]
		if !ok || row.ParentSessionID == "" {
			return false, nil
		}
		if row.ParentSessionID == ancestorID {
			return true, nil
		}
		id = row.ParentSessionID
	}
	return false, nil
}
