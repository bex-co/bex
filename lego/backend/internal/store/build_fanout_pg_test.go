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
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// w9/m180 t002: the fan-out ledger. A live carrier build is pending until its
// fan-out completes; a claim excludes it while its lease is live and from a
// second claimant; only each carrier's latest live build is pending; other
// apps' deploys never are.
func TestPGBuildFanoutLedger(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "build-fanout")
	ctx := context.Background()
	carrierID := ids.New(ids.BuildSource)
	carrier, err := s.CreateApp(ctx, App{ID: carrierID, TenantID: tenant.ID, Name: carrierID, DisplayName: "fanout", Type: TypeBuildSource, Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	web, err := s.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web-fanout", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	live := func(appID string, gen int64) Deploy {
		t.Helper()
		d, err := s.CreateDeploy(ctx, appID, TriggerNewCommit, "img:1", gen, CommitInfo{}, "")
		if err != nil {
			t.Fatal(err)
		}
		if ok, err := s.TransitionDeploy(ctx, d.ID, DeployLive, RanImage{Ref: "zot/src@sha256:" + d.ID}, "", "", "", nil); err != nil || !ok {
			t.Fatalf("transition %s live: ok=%v err=%v", d.ID, ok, err)
		}
		return d
	}
	pending := func(lease time.Duration) []string {
		t.Helper()
		ds, err := s.PendingBuildFanouts(ctx, lease)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, d := range ds {
			if d.AppID == carrier.ID || d.AppID == web.ID {
				ids = append(ids, d.ID)
			}
		}
		return ids
	}

	first := live(carrier.ID, 2)
	live(web.ID, 2)
	if got := pending(time.Hour); len(got) != 1 || got[0] != first.ID {
		t.Fatalf("pending = %v, want only the carrier build %s", got, first.ID)
	}
	if ds, _ := s.PendingBuildFanouts(ctx, time.Hour); ds[0].ResolvedImage != "zot/src@sha256:"+first.ID {
		t.Fatalf("pending build carries resolved image %q", ds[0].ResolvedImage)
	}

	// A claim takes it out of the queue, and a second claimant is refused.
	if ok, err := s.ClaimBuildFanout(ctx, first.ID, time.Hour); err != nil || !ok {
		t.Fatalf("first claim ok=%v err=%v", ok, err)
	}
	if ok, _ := s.ClaimBuildFanout(ctx, first.ID, time.Hour); ok {
		t.Fatal("a second claimant took a live lease")
	}
	if got := pending(time.Hour); len(got) != 0 {
		t.Fatalf("a claimed build is still pending: %v", got)
	}
	// An expired lease (an interrupted run) is pending and claimable again.
	time.Sleep(20 * time.Millisecond)
	if got := pending(10 * time.Millisecond); len(got) != 1 {
		t.Fatalf("an expired claim is not pending: %v", got)
	}
	if ok, _ := s.ClaimBuildFanout(ctx, first.ID, 10*time.Millisecond); !ok {
		t.Fatal("an expired lease could not be reclaimed")
	}

	// A newer live build supersedes an incomplete older one.
	second := live(carrier.ID, 3)
	if got := pending(time.Hour); len(got) != 1 || got[0] != second.ID {
		t.Fatalf("pending = %v, want only the newer build %s", got, second.ID)
	}

	// Completion is final: never pending, never claimable.
	if err := s.CompleteBuildFanout(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	if got := pending(0); len(got) != 0 {
		t.Fatalf("a completed build is pending: %v", got)
	}
	if ok, _ := s.ClaimBuildFanout(ctx, second.ID, 0); ok {
		t.Fatal("a completed build was claimed")
	}
}
