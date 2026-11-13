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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w2/m365: a re-created service's own App object name survives the round trip
// through Postgres on every read the projector and the API use, and its
// fresh slug never takes the bare name its predecessor's host still serves.
// A row without one reads as CRName.
func TestPGAppObjectNameRoundTrips(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	ten, err := st.CreateTenant(ctx, "object-name", "hobby")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(ctx, ten.ID) })
	objectName := core.CRName(ten.ID, "web") + "-ab12"
	row, err := st.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", ObjectName: objectName, Image: "nginx:1", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	plain, err := st.CreateApp(ctx, App{TenantID: ten.ID, Name: "api", Image: "nginx:1", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(row.Slug, "web-") || len(row.Slug) != len("web-")+4 {
		t.Fatalf("slug = %q, want a fresh web-xxxx", row.Slug)
	}
	got, err := st.GetApp(ctx, row.ID)
	if err != nil || got.ObjectName != objectName || got.AppObjectName() != objectName {
		t.Fatalf("GetApp = %q, %v; want %q", got.ObjectName, err, objectName)
	}
	desired, err := st.ListDesiredApps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, d := range desired {
		switch d.ID {
		case row.ID:
			seen++
			if d.AppObjectName() != objectName {
				t.Errorf("desired %s projects as %q, want %q", d.ID, d.AppObjectName(), objectName)
			}
		case plain.ID:
			seen++
			if d.ObjectName != "" || d.AppObjectName() != core.CRName(ten.ID, "api") {
				t.Errorf("desired %s = %q/%q, want no object name, so CRName", d.ID, d.ObjectName, d.AppObjectName())
			}
		}
	}
	if seen != 2 {
		t.Fatalf("ListDesiredApps returned %d of the 2 rows", seen)
	}
}
