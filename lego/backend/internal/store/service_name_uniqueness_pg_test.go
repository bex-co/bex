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
	"sync"
	"testing"
)

// w8/m47: renames skipped the workspace-uniqueness rule creates keep — two
// services could be listed as "hello-go", sequentially or concurrently.
func TestServiceNamesStayUniqueAcrossCreateAndRename(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	tenant, err := st.CreateTenant(ctx, "names-"+uniqueMachineRun(), PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	create := func(name string) App {
		t.Helper()
		app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: name, Type: "web_service", Image: "test/image", Branch: "main", Port: 8080, Replicas: 1, Tier: "free"})
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		return app
	}
	a, b, c := create("svc-a"), create("svc-b"), create("svc-c")

	for _, taken := range []string{"svc-b" /* a creation name */} {
		if err := st.SetAppDisplayName(ctx, a.ID, taken); !errors.Is(err, ErrConflict) {
			t.Errorf("rename A to %q = %v, want ErrConflict", taken, err)
		}
	}
	if err := st.SetAppDisplayName(ctx, c.ID, "shown"); err != nil {
		t.Fatalf("rename C: %v", err)
	}
	if err := st.SetAppDisplayName(ctx, a.ID, "shown"); !errors.Is(err, ErrConflict) {
		t.Errorf("rename A to C's displayed name = %v, want ErrConflict", err)
	}
	if _, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "shown", Type: "web_service", Image: "test/image", Branch: "main", Port: 8080, Replicas: 1, Tier: "free"}); !errors.Is(err, ErrConflict) {
		t.Errorf("create as C's displayed name = %v, want ErrConflict", err)
	}
	// Your own creation name, a clear, and a re-set are always allowed.
	for _, own := range []string{"svc-a", "", "fresh", "fresh"} {
		if err := st.SetAppDisplayName(ctx, a.ID, own); err != nil {
			t.Errorf("rename A to %q: %v", own, err)
		}
	}

	// Concurrent renames of two services to one new name: exactly one lands.
	for round := range 5 {
		name := "race-" + string(rune('a'+round))
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i, id := range []string{b.ID, c.ID} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = st.SetAppDisplayName(ctx, id, name)
			}()
		}
		wg.Wait()
		ok := 0
		for _, err := range errs {
			switch {
			case err == nil:
				ok++
			case !errors.Is(err, ErrConflict):
				t.Fatalf("round %d: %v", round, err)
			}
		}
		if ok != 1 {
			t.Fatalf("round %d: %d renames to %q landed, want exactly one (%v)", round, ok, name, errs)
		}
	}
}
