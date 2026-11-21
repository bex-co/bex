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

package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// The standby slot sweep's statements on a real PostgreSQL (17 in CI, a
// primary). On a standby they drop what slot synchronization cannot replace:
// that was checked against real 17.11 and 18.7 standbys (ADR009 § Logical
// replication › Lifecycle), which CI cannot run.

// On a primary the sweep drops nothing: every logical slot there, failover
// or not, belongs to a subscriber, although none of them is synchronized.
func TestStandbySlotSweepLeavesAPrimarysSlots(t *testing.T) {
	f := newLRFixture(t)
	ctx := context.Background()
	failover, plain := "lr_ha_"+f.n, "lr_nofo_"+f.n
	conn, err := pgx.Connect(ctx, f.superuser)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	var major int
	if err := conn.QueryRow(ctx, `SELECT current_setting('server_version_num')::int / 10000`).Scan(&major); err != nil {
		t.Fatal(err)
	}
	if major < 17 {
		testenv.Skip(t, "slot synchronization needs PostgreSQL 17 or later")
	}
	execAs(t, f.superuser,
		fmt.Sprintf(`SELECT pg_create_logical_replication_slot('%s', 'pgoutput', false, false, true)`, failover),
		fmt.Sprintf(`SELECT pg_create_logical_replication_slot('%s', 'pgoutput')`, plain))

	dropped, err := dropStaleSlots(ctx, conn)
	if err != nil || len(dropped) != 0 {
		t.Fatalf("sweep on a primary = %v, %v; want nothing dropped", dropped, err)
	}
	for _, slot := range []string{failover, plain} {
		if !queryBool(t, f.superuser, `SELECT EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name = $1 AND NOT synced)`, slot) {
			t.Errorf("slot %s is gone after the sweep", slot)
		}
	}
}
