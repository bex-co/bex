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
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
)

// overlapStore records the most metadata reads it ever had in flight at once.
type overlapStore struct {
	*fakeStore
	mu             sync.Mutex
	slow           bool // only once the fixture is built
	inFlight, peak int
}

func (o *overlapStore) Get(ctx context.Context, path string) (map[string]string, error) {
	o.mu.Lock()
	slow := o.slow
	o.mu.Unlock()
	if slow && strings.HasSuffix(path, "/meta") {
		o.mu.Lock()
		o.inFlight++
		o.peak = max(o.peak, o.inFlight)
		o.mu.Unlock()
		time.Sleep(2 * time.Millisecond) // a round trip, so reads can overlap
		defer func() {
			o.mu.Lock()
			o.inFlight--
			o.mu.Unlock()
		}()
	}
	return o.fakeStore.Get(ctx, path)
}

// A workspace's group list reads the metadata of every id it lists, and a
// deleted group's id stays listed behind its resurrection fence, so the scan
// grows with every group the workspace ever deleted — the bex workspace's
// list took 14 s, which the CLI paid once per project (w8/m113). The reads
// overlap, within listMetaReadConcurrency, and the list still returns exactly
// the live groups in id order.
func TestListEnvGroupsReadsMetadataConcurrently(t *testing.T) {
	// Workspace-scoped, as in production: a deleted group's id is listed under
	// its workspace tenant and costs a legacy and a workspace read.
	store := &overlapStore{fakeStore: newFakeStore()}
	svc := &Service{
		Base:  &core.Base{Client: fakeClient(), Namespace: "default", Workspace: coretest.Members{"dana": {"tea-a"}}},
		Store: store,
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "dana", Method: "session"})
	var live []string
	for i := range 40 {
		g, err := svc.CreateEnvGroup(ctx, CreateEnvGroupRequest{Name: "g" + strconv.Itoa(i)})
		if err != nil {
			t.Fatalf("create g%d: %v", i, err)
		}
		if i%10 == 0 {
			live = append(live, g.ID)
			continue
		}
		if err := svc.DeleteEnvGroup(ctx, g.ID); err != nil {
			t.Fatalf("delete g%d: %v", i, err)
		}
	}
	listed, err := svc.listGroupIDs(ctx, "tea-a", true)
	if err != nil || len(listed) != 40 {
		t.Fatalf("listed %d ids (%v), want all 40: deleted groups stay behind their fences", len(listed), err)
	}
	store.mu.Lock()
	store.slow = true
	store.mu.Unlock()
	groups, err := svc.ListEnvGroups(ctx, "tea-a")
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(groups))
	for _, g := range groups {
		got = append(got, g.ID)
	}
	slices.Sort(live)
	if !slices.Equal(got, live) {
		t.Fatalf("listed groups %v, want the live %v in id order", got, live)
	}
	if store.peak < 2 || store.peak > listMetaReadConcurrency {
		t.Fatalf("peak concurrent metadata reads = %d, want 2..%d", store.peak, listMetaReadConcurrency)
	}
}
