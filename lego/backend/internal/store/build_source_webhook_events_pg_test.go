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

// w9/m180: a build source's builds are its carrier's deploy rows (ADR097
// implementation notes), but a carrier is not a service, so its builds must
// not reach workspace webhooks as service deploy events. A linked service's
// own deploy still does.
func TestPGCarrierBuildsAreNotServiceDeployWebhookEvents(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "carrier-webhooks")
	ctx := context.Background()
	carrierID := ids.New(ids.BuildSource)
	carrier, err := s.CreateApp(ctx, App{ID: carrierID, TenantID: tenant.ID, Name: carrierID, DisplayName: "mono", Type: TypeBuildSource, Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	web, err := s.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web-carrier-webhooks", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	finished := func(appID string) Deploy {
		t.Helper()
		d, err := s.CreateDeploy(ctx, appID, TriggerNewCommit, "img:1", 2, CommitInfo{}, "")
		if err != nil {
			t.Fatal(err)
		}
		if ok, err := s.TransitionDeploy(ctx, d.ID, DeployLive, RanImage{Ref: "zot/src@sha256:" + d.ID}, "", "", "", nil); err != nil || !ok {
			t.Fatalf("transition %s live: ok=%v err=%v", d.ID, ok, err)
		}
		return d
	}
	build := finished(carrier.ID)
	deploy := finished(web.ID)

	rows, err := s.ListWebhookEvents(ctx, time.Time{}, "", time.Now().Add(time.Minute), []string{}, []string{tenant.ID}, 100)
	if err != nil {
		t.Fatal(err)
	}
	var sawService bool
	for _, r := range rows {
		if r.Source != EventSourceDeploy {
			continue
		}
		if r.DeployID == build.ID {
			t.Fatalf("carrier build %s surfaced as a service deploy event: %+v", build.ID, r)
		}
		if r.DeployID == deploy.ID {
			sawService = true
		}
	}
	if !sawService {
		t.Fatalf("the linked service's own deploy %s is missing from the webhook events", deploy.ID)
	}
}

// w9/m180: a carrier is never a project or environment member (ADR097; its
// project scope is scope_project_id). Naming one in a project's service list,
// by id or by name, must leave it untouched while the named service moves.
func TestPGProjectServicesNeverMoveACarrier(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "carrier-placement")
	ctx := context.Background()
	carrierID := ids.New(ids.BuildSource)
	if _, err := s.CreateApp(ctx, App{ID: carrierID, TenantID: tenant.ID, Name: carrierID, DisplayName: "mono", Type: TypeBuildSource, Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"}); err != nil {
		t.Fatal(err)
	}
	web, err := s.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web-carrier-placement", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	project, err := s.CreateProject(ctx, tenant.ID, "placement")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetProjectServices(ctx, project.ID, tenant.ID, []string{carrierID, web.ID}, nil); err != nil {
		t.Fatal(err)
	}
	carrier, err := s.GetApp(ctx, carrierID)
	if err != nil {
		t.Fatal(err)
	}
	if carrier.ProjectID != "" {
		t.Fatalf("carrier %s joined project %s", carrierID, carrier.ProjectID)
	}
	moved, err := s.GetApp(ctx, web.ID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.ProjectID != project.ID {
		t.Fatalf("service project = %q, want %q", moved.ProjectID, project.ID)
	}
}
