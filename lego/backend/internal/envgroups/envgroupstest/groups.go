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

// Package envgroupstest is the in-memory environment-group store tests use in
// place of the control-plane Postgres (w8/m114).
package envgroupstest

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Groups is an in-memory envgroups.GroupStore with env_groups' semantics: a unique
// (tenant, name), compare-and-set on meta_version and content_version, and a
// claim that records its operation in the same write. The store package's
// Postgres tests pin the real implementation to the same contract.
type Groups struct {
	mu           sync.Mutex
	rows         map[string]store.EnvGroup
	contentLocks map[string]bool
	// CutoverPending answers EnvGroupCutoverComplete false while set.
	CutoverPending bool
	// InsertErr / UpdateErr fail the next insert / metadata update.
	InsertErr error
	UpdateErr func(next store.EnvGroup) error
	// AfterGet runs (unlocked) after each GetEnvGroup, so a test can commit a
	// concurrent write between a writer's read and its compare-and-set.
	AfterGet func(id string)
	// OnRelease runs (unlocked) before each content release: the point where
	// a save has written its maps but not yet committed.
	OnRelease func(id string)
	calls     int
}

func New() *Groups { return &Groups{rows: map[string]store.EnvGroup{}} }

type contentLockContext struct {
	groups *Groups
	id     string
}

type contentLockKey struct{}

func (f *Groups) WithEnvGroupContentLock(ctx context.Context, id string, fn func(context.Context) error) error {
	if held, ok := ctx.Value(contentLockKey{}).(contentLockContext); ok && held.groups == f && held.id == id {
		return fn(ctx)
	}
	f.mu.Lock()
	if f.contentLocks == nil {
		f.contentLocks = map[string]bool{}
	}
	if f.contentLocks[id] {
		f.mu.Unlock()
		return store.ErrEnvGroupChanged
	}
	f.contentLocks[id] = true
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		delete(f.contentLocks, id)
		f.mu.Unlock()
	}()
	return fn(context.WithValue(ctx, contentLockKey{}, contentLockContext{groups: f, id: id}))
}

func cloneRow(g store.EnvGroup) store.EnvGroup {
	g.ServiceLinks = slices.Clone(g.ServiceLinks)
	g.EnvKeys = slices.Clone(g.EnvKeys)
	g.FileNames = slices.Clone(g.FileNames)
	if g.Content.Op != nil {
		op := *g.Content.Op
		g.Content.Op = &op
	}
	return g
}

func (f *Groups) nameTaken(tenant, name, exclude string) bool {
	for _, g := range f.rows {
		if g.TenantID == tenant && g.Name == name && g.ID != exclude {
			return true
		}
	}
	return false
}

func (f *Groups) InsertEnvGroup(_ context.Context, g store.EnvGroup) (store.EnvGroup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if err := f.InsertErr; err != nil {
		f.InsertErr = nil
		return store.EnvGroup{}, err
	}
	if _, ok := f.rows[g.ID]; ok {
		return store.EnvGroup{}, fmt.Errorf("environment group: %w", store.ErrConflict)
	}
	if f.nameTaken(g.TenantID, g.Name, "") {
		return store.EnvGroup{}, store.ErrEnvGroupNameTaken
	}
	g = cloneRow(g)
	g.Content.Generation = max(g.Content.Generation, 1)
	g.MetaVersion, g.Content.State, g.Content.Version, g.Content.Op = 1, store.EnvGroupIdle, 1, nil
	if g.ServiceLinks == nil {
		g.ServiceLinks = []string{}
	}
	f.rows[g.ID] = g
	return cloneRow(g), nil
}

func (f *Groups) GetEnvGroup(_ context.Context, id string) (store.EnvGroup, error) {
	f.mu.Lock()
	f.calls++
	g, ok := f.rows[id]
	hook := f.AfterGet
	f.mu.Unlock()
	if hook != nil {
		hook(id)
	}
	if !ok {
		return store.EnvGroup{}, fmt.Errorf("environment group: %w", store.ErrNotFound)
	}
	return cloneRow(g), nil
}

func (f *Groups) GetEnvGroupByName(_ context.Context, tenantID, name string) (store.EnvGroup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	for _, g := range f.rows {
		if g.TenantID == tenantID && g.Name == name {
			return cloneRow(g), nil
		}
	}
	return store.EnvGroup{}, fmt.Errorf("environment group: %w", store.ErrNotFound)
}

func (f *Groups) ListEnvGroups(_ context.Context, tenantID string) ([]store.EnvGroup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	var out []store.EnvGroup
	for _, g := range f.rows {
		if g.TenantID == tenantID {
			out = append(out, cloneRow(g))
		}
	}
	slices.SortFunc(out, func(a, b store.EnvGroup) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	return out, nil
}

func (f *Groups) CountEnvGroups(ctx context.Context, tenantID string) (int, error) {
	rows, err := f.ListEnvGroups(ctx, tenantID)
	return len(rows), err
}

func (f *Groups) EnvGroupNameTaken(_ context.Context, tenantID, name string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.nameTaken(tenantID, name, ""), nil
}

func (f *Groups) UpdateEnvGroupMeta(ctx context.Context, g store.EnvGroup) (store.EnvGroup, error) {
	if err := ctx.Err(); err != nil {
		return store.EnvGroup{}, err // as a database call on a canceled context fails
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if hook := f.UpdateErr; hook != nil {
		f.mu.Unlock()
		err := hook(g)
		f.mu.Lock()
		if err != nil {
			return store.EnvGroup{}, err
		}
	}
	cur, ok := f.rows[g.ID]
	if !ok {
		return store.EnvGroup{}, fmt.Errorf("environment group: %w", store.ErrNotFound)
	}
	if cur.MetaVersion != g.MetaVersion {
		return store.EnvGroup{}, store.ErrEnvGroupChanged
	}
	if f.nameTaken(cur.TenantID, g.Name, g.ID) {
		return store.EnvGroup{}, store.ErrEnvGroupNameTaken
	}
	cur.Name, cur.EnvironmentID, cur.UpdatedAt = g.Name, g.EnvironmentID, g.UpdatedAt
	cur.ServiceLinks = slices.Clone(g.ServiceLinks)
	if cur.ServiceLinks == nil {
		cur.ServiceLinks = []string{}
	}
	cur.MetaVersion++
	f.rows[g.ID] = cur
	return cloneRow(cur), nil
}

func (f *Groups) DeleteEnvGroup(_ context.Context, id string) (store.EnvGroup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	g, ok := f.rows[id]
	if !ok {
		return store.EnvGroup{}, fmt.Errorf("environment group: %w", store.ErrNotFound)
	}
	delete(f.rows, id)
	return g, nil
}

// lostRace mirrors PGStore.envGroupLostRace. Callers hold f.mu.
func (f *Groups) lostRace(id string) error {
	if _, ok := f.rows[id]; ok {
		return store.ErrEnvGroupChanged
	}
	return fmt.Errorf("environment group: %w", store.ErrNotFound)
}

func (f *Groups) ClaimEnvGroupContent(_ context.Context, id string, expectedVersion int64, op store.EnvGroupOp) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	g, ok := f.rows[id]
	if !ok || g.Content.Version != expectedVersion || g.Content.State != store.EnvGroupIdle || g.Content.Op != nil {
		return 0, f.lostRace(id)
	}
	g.Content.State, g.Content.Op = store.EnvGroupBusy, &op
	g.Content.Version++
	f.rows[id] = g
	return g.Content.Version, nil
}

func (f *Groups) AdvanceEnvGroupOp(_ context.Context, id, opID, phase string, leaseUntil time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	g, ok := f.rows[id]
	if !ok || g.Content.Op == nil || g.Content.Op.ID != opID {
		return f.lostRace(id)
	}
	op := *g.Content.Op
	op.Phase, op.LeaseUntil = phase, leaseUntil
	g.Content.Op = &op
	f.rows[id] = g
	return nil
}

func (f *Groups) ReleaseEnvGroupContent(_ context.Context, id string, expectedVersion int64, rel store.EnvGroupRelease) error {
	f.mu.Lock()
	hook := f.OnRelease
	f.mu.Unlock()
	if hook != nil {
		hook(id)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	g, ok := f.rows[id]
	if !ok || g.Content.Version != expectedVersion {
		return f.lostRace(id)
	}
	if rel.RepairRequired {
		g.Content.State = store.EnvGroupRepairRequired
	} else if rel.RetainOperation {
		g.Content.State = store.EnvGroupIdle
	} else {
		g.Content.State, g.Content.Op = store.EnvGroupIdle, nil
	}
	if c := rel.Commit; c != nil {
		g.Content.Generation++
		g.EnvKeys, g.FileNames, g.UpdatedAt = slices.Clone(c.EnvKeys), slices.Clone(c.FileNames), c.UpdatedAt
	}
	g.Content.Version++
	f.rows[id] = g
	return nil
}

func (f *Groups) EnvGroupCutoverComplete(context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return !f.CutoverPending, nil
}

// Seed sets a group row as a test's starting state, idle at generation 1.
func (f *Groups) Seed(g store.EnvGroup) store.EnvGroup {
	if g.Content.State == "" {
		g.Content.State = store.EnvGroupIdle
	}
	if g.Content.Generation == 0 {
		g.Content.Generation = 1
	}
	if g.MetaVersion == 0 {
		g.MetaVersion = 1
	}
	if g.Content.Version == 0 {
		g.Content.Version = 1
	}
	if g.ServiceLinks == nil {
		g.ServiceLinks = []string{}
	}
	f.Set(g)
	return g
}

// Commit applies a concurrent writer's metadata change to a row directly,
// moving its version as a real write would.
func (f *Groups) Commit(id string, mutate func(*store.EnvGroup)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	g := cloneRow(f.rows[id])
	mutate(&g)
	g.MetaVersion++
	f.rows[id] = g
}

// Row returns a group's current row for assertions (zero when absent).
func (f *Groups) Row(id string) (store.EnvGroup, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.rows[id]
	return cloneRow(g), ok
}

// Set overwrites a row for a test's setup.
func (f *Groups) Set(g store.EnvGroup) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[g.ID] = cloneRow(g)
}

// Calls counts every store call so far, for a test that bounds them.
func (f *Groups) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// CompleteEnvGroupCutover records the cutover marker.
func (f *Groups) CompleteEnvGroupCutover(context.Context, time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.CutoverPending = false
	return nil
}
