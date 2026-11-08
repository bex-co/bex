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
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestPGAgentApprovalModeWithdrawn pins migration 0207 (approval mode withdrawn,
// DO_NOT_DO #APPROVAL-MODE): after every migration the decision table, its
// mirror columns and the approval settings are gone, the cancel trigger is
// dropped, and the session state notification fires on phase changes only,
// as it did before 0205.
func TestPGAgentApprovalModeWithdrawn(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var tables int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_name = 'agent_session_decisions'`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Error("agent_session_decisions still exists after 0207")
	}
	for table, column := range map[string][]string{
		"agent_sessions":     {"pending_decision_id", "pending_decision_kind", "pending_decision_count", "approval_mode"},
		"agent_environments": {"approval_mode", "approval_bypass_allowed"},
	} {
		for _, name := range column {
			var n int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns
				WHERE table_schema = current_schema() AND table_name = $1 AND column_name = $2`, table, name).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Errorf("%s.%s still exists after 0207", table, name)
			}
		}
	}
	var triggers int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_trigger WHERE tgname = 'agent_sessions_cancel_decisions'`).Scan(&triggers); err != nil {
		t.Fatal(err)
	}
	if triggers != 0 {
		t.Error("the decision cancel trigger outlived 0207")
	}
	var body string
	if err := pool.QueryRow(ctx, `SELECT pg_get_functiondef('notify_agent_session_state'::regproc)`).Scan(&body); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "pending_decision") || !strings.Contains(body, "OLD.phase IS DISTINCT FROM NEW.phase") {
		t.Errorf("notify_agent_session_state is not the phase-only version:\n%s", body)
	}
}
