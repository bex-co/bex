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

// w8/367: a renamed service kept its creation name reserved, so no other
// service could be created or renamed to a name no service showed. Names are
// the ones services are shown as, as on Render; the creation name stays the
// renamed service's resource name, which the unique index still holds.
func TestARenamedServicesCreationNameIsFreeToShow(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	tenant, err := st.CreateTenant(ctx, "renamed-"+uniqueMachineRun(), PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	create := func(name string) (App, error) {
		return st.CreateApp(ctx, App{TenantID: tenant.ID, Name: name, Type: "web_service", Image: "test/image", Branch: "main", Port: 8080, Replicas: 1, Tier: "free"})
	}
	renamed, err := create("api")
	if err != nil {
		t.Fatal(err)
	}
	third, err := create("worker")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetAppDisplayName(ctx, renamed.ID, "api-old"); err != nil {
		t.Fatalf("rename api: %v", err)
	}
	if taken, err := st.ServiceNameTaken(ctx, tenant.ID, "", "api"); err != nil || taken {
		t.Fatalf("ServiceNameTaken(api) after its rename = %v, %v; want free", taken, err)
	}
	if taken, err := st.ServiceNameTaken(ctx, tenant.ID, "", "api-old"); err != nil || !taken {
		t.Fatalf("ServiceNameTaken(api-old) = %v, %v; want taken", taken, err)
	}
	if _, err := create("api"); !errors.Is(err, ErrConflict) {
		t.Fatalf("create as the resource name api = %v, want the unique index's ErrConflict", err)
	}
	newcomer, err := create("api-x7k2")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetAppDisplayName(ctx, newcomer.ID, "api"); err != nil {
		t.Fatalf("label the newcomer api: %v", err)
	}
	// While the newcomer shows "api", nobody else may: not a third service,
	// not the renamed one taking it back, not a clear that would show it.
	for _, tc := range []struct {
		id, name string
	}{{third.ID, "api"}, {renamed.ID, "api"}, {renamed.ID, ""}} {
		if err := st.SetAppDisplayName(ctx, tc.id, tc.name); !errors.Is(err, ErrConflict) {
			t.Errorf("SetAppDisplayName(%s, %q) while the newcomer shows api = %v, want ErrConflict", tc.id, tc.name, err)
		}
	}
	if err := st.DeleteApp(ctx, newcomer.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.SetAppDisplayName(ctx, third.ID, "api"); err != nil {
		t.Fatalf("rename a third service to api once the newcomer is gone: %v", err)
	}
}
