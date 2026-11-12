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
	"strings"
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
	carrier, err := st.CreateApp(ctx, store.App{ID: carrierID, DisplayName: "Mono", TenantID: ws.ID, Name: carrierID, Type: store.TypeBuildSource,
		Repo: "https://github.com/acme/mono", Branch: "main", Port: 10000, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	// The carrier keeps the bds- id the API minted: the CR and every
	// /v1/build-sources route name it by that id.
	if got, err := st.GetApp(ctx, carrierID); err != nil || got.DisplayName != "Mono" {
		t.Fatalf("carrier row = %+v, %v; want it under the minted %s, named Mono in the same create", got, err, carrierID)
	}
	svc, err := st.CreateApp(ctx, store.App{ID: carrierID + "x", DisplayName: "Svc", TenantID: ws.ID, Name: "svc-" + stamp, Type: "web_service", Image: "nginx:1", Port: 10000, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(svc.ID, "srv-") || svc.DisplayName != "" {
		t.Fatalf("service row id %q display name %q; want a fresh srv- id and no display name, whatever the caller passed", svc.ID, svc.DisplayName)
	}
	if err := st.DeleteApp(ctx, svc.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", ids.New(ids.Service)} {
		if _, err := st.CreateApp(ctx, store.App{ID: id, TenantID: ws.ID, Name: "bad-" + stamp, Type: store.TypeBuildSource, Repo: "https://github.com/acme/mono", Port: 10000, Replicas: 1, Tier: "starter"}); err == nil {
			t.Errorf("a carrier with id %q was created; it needs its minted bds- id", id)
		}
	}
	web, err := st.CreateApp(ctx, store.App{TenantID: ws.ID, Name: "web-" + stamp, Type: "web_service", Port: 10000, Replicas: 1, Tier: "starter",
		BuildSourceID: carrier.ID, BuildSourceImage: "zot.test/tea/src@sha256:abc"})
	if err != nil {
		t.Fatal(err)
	}

	carriers, err := st.ListBuildSourceCarriers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	listedCarrier, listedService := false, false
	for _, c := range carriers {
		listedCarrier = listedCarrier || c.ID == carrier.ID
		listedService = listedService || c.ID == web.ID
	}
	if !listedCarrier || listedService {
		t.Errorf("ListBuildSourceCarriers carrier %v service %v, want only carriers", listedCarrier, listedService)
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
	if changed, err := st.SetAppBuildSource(ctx, api.ID, carrier.ID, ""); err != nil || !changed {
		t.Fatalf("link = changed %t, err %v; want a change", changed, err)
	}
	if got, err := st.GetApp(ctx, api.ID); err != nil || got.BuildSourceID != carrier.ID || got.Image != "" || got.Repo != "" {
		t.Fatalf("linked row = %+v err=%v, want the link and no source of its own", got, err)
	}
	// A repeat of the link writes nothing, and says so (w9/092).
	if changed, err := st.SetAppBuildSource(ctx, api.ID, carrier.ID, ""); err != nil || changed {
		t.Fatalf("repeated link = changed %t, err %v; want no change", changed, err)
	}
	// A deploy records the build it shipped, in its own insert; other
	// deploys carry none.
	buildID := ids.New(ids.Build)
	shipped, err := st.CreateDeploy(ctx, web.ID, store.TriggerAPI, "", 2, store.CommitInfo{BuildID: buildID}, "")
	if err != nil {
		t.Fatal(err)
	}
	if d, err := st.GetDeploy(ctx, web.ID, shipped.ID); err != nil || d.BuildID != buildID {
		t.Fatalf("deploy build = %q, %v; want %s", d.BuildID, err, buildID)
	}
	if d, err := st.GetDeploy(ctx, web.ID, web.FirstDeployID); err != nil || d.BuildID != "" {
		t.Fatalf("an unbuilt deploy's build = %q, %v; want none", d.BuildID, err)
	}
	// A deploy selects the source's next build on the linked row only.
	if err := st.SetAppBuildSourceImage(ctx, api.ID, carrier.ID, "zot/src@sha256:next"); err != nil {
		t.Fatal(err)
	}
	if got, err := st.GetApp(ctx, api.ID); err != nil || got.BuildSourceImage != "zot/src@sha256:next" || got.BuildSourceID != carrier.ID {
		t.Fatalf("selected build = %+v err=%v", got, err)
	}
	if err := st.SetAppBuildSourceImage(ctx, carrier.ID, carrier.ID, "zot/src@sha256:next"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("selecting a build on an unlinked row = %v, want not found", err)
	}
	if err := st.SetAppBuildSourceImage(ctx, api.ID, "bds-other", "zot/src@sha256:stale"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("selecting a build of a source the row no longer links = %v, want not found", err)
	}
	if _, err := st.SetAppBuildSource(ctx, carrier.ID, carrier.ID, ""); !errors.Is(err, store.ErrNotFound) {
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
