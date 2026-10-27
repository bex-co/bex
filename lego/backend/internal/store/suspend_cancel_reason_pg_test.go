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

// A rollout a suspend ended is canceled, not superseded (w5/m114 live replay):
// its deploy_ended keeps the suspend line as cancelReason and carries no
// reason code, while a supersede cancel still reads superseded.
func TestPGSuspendCanceledDeployIsNotReportedSuperseded(t *testing.T) {
	ctx := context.Background()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)
	ten, err := st.CreateWorkspace(ctx, fmt.Sprintf("suspend-cancel-%d", time.Now().UnixNano()), PlanHobby, "test-suspend-cancel")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.DeleteTenant(ctx, ten.ID) }()
	row, err := st.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "nginx:1", Port: 80, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	reasons := map[int64]string{1: suspendCancelReason, 2: "Superseded by a newer release"}
	ids := map[string]int64{}
	for gen, reason := range reasons {
		d, err := st.CreateDeploy(ctx, row.ID, "api", "nginx:1", gen, CommitInfo{}, "dana")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE deploys SET status = $2, cancel_reason = $3, overlap_pending = false, finished_at = now() WHERE id = $1`,
			d.ID, DeployCanceled, reason); err != nil {
			t.Fatal(err)
		}
		ids[d.ID] = gen
	}
	events, err := st.ListServiceEvents(ctx, row.ID, ten.ID, ServiceEventFilter{Limit: 100, Phases: []string{EventPhaseEnded}})
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, e := range events {
		gen, ok := ids[e.DeployID]
		if !ok || e.Source != EventSourceDeploy {
			continue
		}
		seen++
		wantCode := string(EventReasonSuperseded)
		if gen == 1 {
			wantCode = ""
		}
		if e.CancelReason != reasons[gen] || e.ReasonCode != wantCode {
			t.Errorf("deploy_ended for %q: cancelReason=%q reasonCode=%q, want %q / %q", reasons[gen], e.CancelReason, e.ReasonCode, reasons[gen], wantCode)
		}
	}
	if seen != 2 {
		t.Fatalf("saw %d deploy_ended rows, want 2: %+v", seen, events)
	}
}
