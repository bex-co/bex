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
	"log"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A major-version upgrade of a logical-replication publisher (w34/m543;
// docs/ADR009 § Major-version upgrades). CNPG upgrades in place with
// pg_upgrade, which carries the instance's logical slots only from
// PostgreSQL 17 or later, and there refuses, failing the whole upgrade, while
// a slot is invalidated or has not consumed all WAL ("The slot "s" has not
// consumed the WAL yet", checked on 17 -> 18 on 2026-11-21). From 13-16 it
// drops every slot without a word. So bex refuses the first by name, and
// accepts the second while telling the caller.

// The upgrade's logical-slot answers: a refusal, and a read that failed.
const (
	PostgresUpgradeLogicalSlotsBehindCode     = "POSTGRES_UPGRADE_LOGICAL_SLOTS_BEHIND"
	PostgresUpgradeLogicalSlotsUnreadableCode = "POSTGRES_UPGRADE_LOGICAL_SLOTS_UNREADABLE"
)

// slotCatchUpWait bounds how long the upgrade check waits for the slots a
// subscriber streams from to confirm the WAL written before it began: a
// caught-up subscriber confirms within a second or two. A variable so tests
// can shorten it.
var slotCatchUpWait = 10 * time.Second

const slotCatchUpPoll = 250 * time.Millisecond

// logicalSlot is one logical replication slot of the instance, as
// pg_replication_slots shows it to every role.
type logicalSlot struct {
	name string
	// active: a walsender (a subscriber) holds the slot.
	active bool
	// invalid: PostgreSQL invalidated the slot; its subscriber has to be
	// re-created.
	invalid bool
	// caughtUp: the slot has confirmed every WAL position written before the
	// check began.
	caughtUp bool
}

// describe says why the slot stops an upgrade.
func (sl logicalSlot) describe() string {
	switch {
	case sl.invalid:
		return sl.name + " (invalidated: drop it and re-create its subscription)"
	case sl.active:
		return sl.name + " (its subscriber has not caught up)"
	default:
		return sl.name + " (no subscriber is connected to it)"
	}
}

// readLogicalSlots lists the instance's logical slots, every database's,
// since pg_upgrade checks and carries them all. target is the WAL position a
// slot must have confirmed to count as caught up; the invalidation reason is
// PostgreSQL 17's column, so major says whether to read it.
func readLogicalSlots(ctx context.Context, conn *pgx.Conn, major int, target string) ([]logicalSlot, error) {
	invalid := `wal_status = 'lost'`
	if major >= appv1alpha1.LogicalSlotsCarriedMajor {
		invalid += ` OR invalidation_reason IS NOT NULL`
	}
	rows, err := conn.Query(ctx, `SELECT slot_name::text, active, coalesce(`+invalid+`, false),
		coalesce(confirmed_flush_lsn >= $1::pg_lsn, false)
		FROM pg_catalog.pg_replication_slots WHERE slot_type = 'logical' AND NOT temporary ORDER BY slot_name`, target)
	if err != nil {
		return nil, mapPGError(err)
	}
	slots, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (logicalSlot, error) {
		var sl logicalSlot
		err := row.Scan(&sl.name, &sl.active, &sl.invalid, &sl.caughtUp)
		return sl, err
	})
	return slots, mapPGError(err)
}

// logicalSlotsBehind returns the instance's logical slots that would fail a
// pg_upgrade from 17 or later: the invalidated ones, and those that have not
// confirmed the WAL written before the check began. A slot a subscriber is
// streaming from gets slotCatchUpWait to confirm; one nobody holds cannot
// move, so the wait ends once only those are left. A slot whose only WAL
// since is a checkpoint would pass pg_upgrade, which decodes it to tell, but
// not this check: its subscriber has to connect once.
func logicalSlotsBehind(ctx context.Context, conn *pgx.Conn, major int) ([]logicalSlot, error) {
	var target string
	if err := conn.QueryRow(ctx, `SELECT pg_catalog.pg_current_wal_lsn()::text`).Scan(&target); err != nil {
		return nil, mapPGError(err)
	}
	deadline := time.Now().Add(slotCatchUpWait)
	for {
		slots, err := readLogicalSlots(ctx, conn, major, target)
		if err != nil {
			return nil, err
		}
		behind := slices.DeleteFunc(slots, func(sl logicalSlot) bool { return sl.caughtUp && !sl.invalid })
		moving := slices.ContainsFunc(behind, func(sl logicalSlot) bool { return sl.active && !sl.invalid })
		if !moving || time.Now().After(deadline) {
			return behind, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(slotCatchUpPoll):
		}
	}
}

// upgradeLogicalSlotsNotice applies pg_upgrade's logical-slot rules to an
// upgrade of d from currentMajor: from 17 or later it refuses while a slot
// is behind; before 17 it returns the notice that the slots will not
// survive, empty when there are none. A database that is not a publisher is
// not read: no role of it can create a logical slot.
func (s *Service) upgradeLogicalSlotsNotice(ctx context.Context, d *appv1alpha1.Database, currentMajor int, targetVersion string) (string, error) {
	if !d.Spec.LogicalReplication.Publishes() {
		return "", nil
	}
	current := strconv.Itoa(currentMajor)
	params := map[string]any{"currentVersion": current, "targetVersion": targetVersion}
	// The cause is logged, not answered: it can name the instance's
	// internal host.
	unreadable := func(err error) error {
		log.Printf("postgres upgrade %s: reading the logical replication slots: %v", d.Name, err)
		return core.NewUnavailableError(PostgresUpgradeLogicalSlotsUnreadableCode,
			"could not read the database's logical replication slots, which a major-version upgrade checks first; try again once the database is available", params)
	}
	uri, err := s.databaseURI(ctx, d)
	if err != nil {
		return "", unreadable(err)
	}
	conn, err := connectForLogicalReplication(ctx, uri)
	if err != nil {
		return "", unreadable(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	if currentMajor < appv1alpha1.LogicalSlotsCarriedMajor {
		// Every slot goes, caught up or not: no position to compare with.
		slots, err := readLogicalSlots(ctx, conn, currentMajor, "0/0")
		if err != nil {
			return "", unreadable(err)
		}
		return droppedSlotsNotice(current, slotNames(slots)), nil
	}
	behind, err := logicalSlotsBehind(ctx, conn, currentMajor)
	if err != nil {
		return "", unreadable(err)
	}
	if len(behind) == 0 {
		return "", nil
	}
	described := make([]string, len(behind))
	for i, sl := range behind {
		described[i] = sl.describe()
	}
	params["slots"] = slotNames(behind)
	return "", core.NewConflictError(PostgresUpgradeLogicalSlotsBehindCode, fmt.Sprintf(
		"PostgreSQL's pg_upgrade refuses to upgrade PostgreSQL %s while a logical replication slot has not consumed all WAL: %s. Let each subscriber connect and catch up, or drop its slot, then upgrade again",
		current, strings.Join(described, ", ")), params)
}

// droppedSlotsNotice is the answer to an accepted upgrade from a major
// before 17 that has logical slots: they will not survive it.
func droppedSlotsNotice(current string, slots []string) string {
	if len(slots) == 0 {
		return ""
	}
	return fmt.Sprintf("PostgreSQL %s's pg_upgrade does not carry logical replication slots, so this upgrade drops %s: re-create each of their subscriptions once it finishes, which copies the tables again. Slots survive an upgrade from PostgreSQL 17 or later",
		current, strings.Join(slots, ", "))
}

func slotNames(slots []logicalSlot) []string {
	names := make([]string, len(slots))
	for i, sl := range slots {
		names[i] = sl.name
	}
	return names
}
