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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestPGTerminalCloseSettlesARunningPreDeploy pins w4/187 against real
// Postgres: SetDeployPreDeployStatus only writes open rows, so a deploy
// superseded mid-pre-deploy kept `running` forever. The terminal close itself
// settles it — succeeded when the release went live, canceled otherwise — and
// leaves an already-settled or absent step alone.
func TestPGTerminalCloseSettlesARunningPreDeploy(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	s := NewPGStore(pool)
	run := time.Now().UnixNano()
	ten, err := s.CreateTenant(ctx, fmt.Sprintf("predeploy-close-%d", run), PlanPro)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	defer func() { _ = s.DeleteTenant(ctx, ten.ID) }()

	for i, tc := range []struct {
		name, preDeploy, close, cancelReason, want string
	}{
		{"superseded mid-step", PreDeployRunning, DeployCanceled, "Superseded by dep-next", PreDeployCanceled},
		{"went live", PreDeployRunning, DeployLive, "", PreDeploySucceeded},
		{"already failed", PreDeployFailed, DeployUpdateFailed, "", PreDeployFailed},
		{"no pre-deploy step", "", DeployCanceled, "user", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, err := s.CreateApp(ctx, App{TenantID: ten.ID, Name: fmt.Sprintf("pd-close-%d-%d", run, i),
				Image: "registry.example/app:v1", Branch: "main", Port: 8080, Replicas: 1, Tier: "starter"})
			if err != nil {
				t.Fatalf("create app: %v", err)
			}
			deploys, err := s.ListDeploys(ctx, app.ID, DeployFilter{})
			if err != nil || len(deploys) != 1 {
				t.Fatalf("deploy fixture = %+v (err %v)", deploys, err)
			}
			d := deploys[0]
			if _, err := s.TransitionDeploy(ctx, d.ID, DeployUpdateInProgress, "", "", "", "", nil); err != nil {
				t.Fatalf("dispatch: %v", err)
			}
			if tc.preDeploy != "" {
				if _, err := s.SetDeployPreDeployStatus(ctx, d.ID, tc.preDeploy); err != nil {
					t.Fatalf("set pre-deploy: %v", err)
				}
			}
			if won, err := s.TransitionDeploy(ctx, d.ID, tc.close, "", "", "", tc.cancelReason, nil); err != nil || !won {
				t.Fatalf("close %s = (%v, %v)", tc.close, won, err)
			}
			got, err := s.GetDeploy(ctx, app.ID, d.ID)
			if err != nil {
				t.Fatalf("get deploy: %v", err)
			}
			if got.Status != tc.close || got.PreDeployStatus != tc.want {
				t.Fatalf("closed deploy = status %q preDeployStatus %q, want %q / %q", got.Status, got.PreDeployStatus, tc.close, tc.want)
			}
		})
	}
}
