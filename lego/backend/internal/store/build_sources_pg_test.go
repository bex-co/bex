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

package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestBuildSourceCarrierRowsPG pins w2/m254's storage (ADR097): a build source
// is an apps row of type build_source that no service listing or service count
// includes; a linked service projects its carrier and selected build; a carrier
// with links cannot be deleted alone, while a workspace deletion that cascades
// through both still succeeds; and a carrier is never grouped or itself linked.
func TestBuildSourceCarrierRowsPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := store.Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	ws, err := st.CreateWorkspace(ctx, "bsrc-"+stamp, store.PlanHobby, "bsrc-"+stamp)
	if err != nil {
		t.Fatal(err)
	}

	carrierID := ids.New(ids.BuildSource)
	carrier, err := st.CreateApp(ctx, store.App{ID: carrierID, TenantID: ws.ID, Name: carrierID, Type: store.TypeBuildSource,
		Repo: "https://github.com/acme/mono", Branch: "main", Port: 10000, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	web, err := st.CreateApp(ctx, store.App{TenantID: ws.ID, Name: "web-" + stamp, Type: "web_service", Port: 10000, Replicas: 1, Tier: "starter",
		BuildSourceID: carrier.ID, BuildSourceImage: "zot.test/tea/src@sha256:abc"})
	if err != nil {
		t.Fatal(err)
	}

	if n, err := st.CountBuildSourcesForTenant(ctx, ws.ID); err != nil || n != 1 {
		t.Errorf("CountBuildSourcesForTenant = %d (err %v), want the one carrier", n, err)
	}
	if links, err := st.ListBuildSourceLinks(ctx, carrier.ID); err != nil || len(links) != 1 || links[0].ID != web.ID {
		t.Errorf("ListBuildSourceLinks = %+v (err %v), want the linked service", links, err)
	}
	if n, err := st.CountAppsForTenant(ctx, ws.ID); err != nil || n != 1 {
		t.Errorf("CountAppsForTenant = %d (err %v), want the one service", n, err)
	}
	all, err := st.ListApps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range all {
		if a.ID == carrier.ID {
			t.Error("ListApps returned the build source carrier")
		}
	}
	got, err := st.GetApp(ctx, web.ID)
	if err != nil || got.BuildSourceID != carrier.ID || got.BuildSourceImage != "zot.test/tea/src@sha256:abc" {
		t.Fatalf("linked service = %+v err=%v", got, err)
	}
	desired, err := st.ListDesiredApps(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var sawCarrier, sawLink bool
	for _, d := range desired {
		sawCarrier = sawCarrier || d.ID == carrier.ID
		sawLink = sawLink || (d.ID == web.ID && d.BuildSourceID == carrier.ID && d.BuildSourceImage != "")
	}
	if !sawCarrier || !sawLink {
		t.Fatalf("ListDesiredApps carrier %v link %v, want both (a carrier still needs its CR)", sawCarrier, sawLink)
	}

	// Linking an existing service clears its own source in the same write.
	api, err := st.CreateApp(ctx, store.App{TenantID: ws.ID, Name: "api-" + stamp, Type: "web_service", Image: "nginx:1", Port: 10000, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetAppBuildSource(ctx, api.ID, carrier.ID, ""); err != nil {
		t.Fatal(err)
	}
	if got, err := st.GetApp(ctx, api.ID); err != nil || got.BuildSourceID != carrier.ID || got.Image != "" || got.Repo != "" {
		t.Fatalf("linked row = %+v err=%v, want the link and no source of its own", got, err)
	}
	if err := st.SetAppBuildSource(ctx, carrier.ID, carrier.ID, ""); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("linking a carrier = %v, want not found", err)
	}
	if err := st.DeleteApp(ctx, carrier.ID); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("deleting a carrier with a linked service = %v, want a conflict", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE apps SET build_source_id = $1 WHERE id = $1`, carrier.ID); err == nil {
		t.Error("a carrier was linked to a build source")
	}
	if err := st.DeleteTenant(ctx, ws.ID); err != nil {
		t.Fatalf("deleting the workspace with a carrier and its linked service: %v", err)
	}
}
