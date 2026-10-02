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
	"testing"
)

func TestPGInitialAppCreationBarrier(t *testing.T) {
	st := openLifecyclePG(t)
	tenant := lifecycleTenant(t, st, "initial-app")
	ctx := context.Background()
	row, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "pending", CreationPending: true, Image: "nginx:alpine", Branch: "main", Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	assertPending := func(want bool) {
		t.Helper()
		got, err := st.GetApp(ctx, row.ID)
		if err != nil || got.CreationPending != want {
			t.Fatalf("GetApp pending=%v err=%v", got.CreationPending, err)
		}
		desired, err := st.ListDesiredApps(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range desired {
			if d.ID == row.ID {
				if d.CreationPending != want {
					t.Fatalf("projector pending=%v want %v", d.CreationPending, want)
				}
				return
			}
		}
		t.Fatal("pending row omitted from projector deletion fence")
	}
	assertPending(true)
	for range 2 {
		if err := st.CompleteAppCreation(ctx, row.ID); err != nil {
			t.Fatal(err)
		}
		assertPending(false)
	}
	deploys, err := st.ListDeploys(ctx, row.ID, DeployFilter{})
	if err != nil || len(deploys) != 1 || deploys[0].ID != row.FirstDeployID || deploys[0].Generation != FirstDeployGeneration {
		t.Fatalf("completion changed first deploy: %+v %v", deploys, err)
	}
	if err := st.DeleteApp(ctx, row.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.CompleteAppCreation(ctx, row.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("complete deleted row: %v", err)
	}
	legacy, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "ordinary", Image: "nginx:alpine", Branch: "main", Tier: "free"})
	if err != nil || legacy.CreationPending {
		t.Fatalf("ordinary create changed: %+v %v", legacy, err)
	}
}
