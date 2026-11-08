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
	"sort"
	"time"
)

// memStore implements BuildFanoutLedger so reconciler tests exercise the
// durable fan-out; the SQL itself is pinned by TestPGBuildFanoutLedger.
type memFanout struct {
	claimedAt time.Time
	done      bool
}

// fanoutState is called with m.mu held.
func (m *memStore) fanoutState(id string) *memFanout {
	if m.fanouts == nil {
		m.fanouts = map[string]*memFanout{}
	}
	f := m.fanouts[id]
	if f == nil {
		f = &memFanout{}
		m.fanouts[id] = f
	}
	return f
}

func (m *memStore) ClaimBuildFanout(_ context.Context, deployID string, lease time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f := m.fanoutState(deployID)
	if f.done || (!f.claimedAt.IsZero() && time.Since(f.claimedAt) < lease) {
		return false, nil
	}
	f.claimedAt = time.Now()
	return true, nil
}

func (m *memStore) CompleteBuildFanout(_ context.Context, deployID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fanoutState(deployID).done = true
	return nil
}

func (m *memStore) PendingBuildFanouts(_ context.Context, lease time.Duration) ([]Deploy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	latest := map[string]Deploy{}
	for _, d := range m.deploys {
		if d.Status != DeployLive || m.apps[d.AppID].Type != TypeBuildSource {
			continue
		}
		cur, ok := latest[d.AppID]
		if !ok || finishedAfter(d, cur) {
			latest[d.AppID] = d
		}
	}
	var out []Deploy
	for _, d := range latest {
		f := m.fanoutState(d.ID)
		if f.done || (!f.claimedAt.IsZero() && time.Since(f.claimedAt) < lease) {
			continue
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AppID < out[j].AppID })
	return out, nil
}

func finishedAfter(a, b Deploy) bool {
	switch {
	case a.FinishedAt != nil && b.FinishedAt != nil && !a.FinishedAt.Equal(*b.FinishedAt):
		return a.FinishedAt.After(*b.FinishedAt)
	case a.Generation != b.Generation:
		return a.Generation > b.Generation
	}
	return a.ID > b.ID
}
