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
	"log"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// Bandwidth served to a DDoS attack is not billed, as on Render (w4/m709,
// docs/ADR023-usage-metering.md § Bandwidth served to a DDoS attack is not
// billed). A bex operator declares an attack window over the control-plane
// API; every egress_bytes hour a live window overlaps is left off the bill.

// The bounds a window must keep. They catch a typo (a wrong year, a window
// meant to last hours that lasts weeks), not a real attack: a longer one is
// declared as consecutive windows.
const (
	// EgressAttackWindowMaxSpan is the longest one window may last.
	EgressAttackWindowMaxSpan = 7 * 24 * time.Hour
	// EgressAttackWindowLookback is how far back a window may start: the
	// emitter's backfill horizon plus a day, well inside the hourly rows'
	// three-month retention, so the hours it covers still have rows.
	EgressAttackWindowLookback = 35 * 24 * time.Hour
	// EgressAttackWindowLead is how far ahead a window may end, so an attack
	// still under way is declared up to now and extended as it goes on.
	EgressAttackWindowLead = 24 * time.Hour
	// egressAttackReasonMax caps a reason, as a billing override's is.
	egressAttackReasonMax = 200
)

// EgressAttackWindow is one operator-declared attack window. ServiceIDs empty
// covers every service of the workspace.
type EgressAttackWindow struct {
	ID             string                     `json:"id"`
	WorkspaceID    string                     `json:"workspaceId"`
	ServiceIDs     []string                   `json:"serviceIds"`
	StartsAt       time.Time                  `json:"startsAt"`
	EndsAt         time.Time                  `json:"endsAt"`
	Actor          string                     `json:"actor"`
	Reason         string                     `json:"reason"`
	CreatedAt      time.Time                  `json:"createdAt"`
	UpdatedAt      time.Time                  `json:"updatedAt"`
	WithdrawnAt    *time.Time                 `json:"withdrawnAt"`
	WithdrawnBy    string                     `json:"withdrawnBy,omitempty"`
	WithdrawReason string                     `json:"withdrawReason,omitempty"`
	Changes        []EgressAttackWindowChange `json:"changes,omitempty"`
}

// Covers reports whether the window applies to serviceID's rows.
func (w EgressAttackWindow) Covers(serviceID string) bool {
	return len(w.ServiceIDs) == 0 || slices.Contains(w.ServiceIDs, serviceID)
}

// EgressAttackWindowChange is one declaration, extension or withdrawal.
type EgressAttackWindowChange struct {
	Action   string    `json:"action"`
	StartsAt time.Time `json:"startsAt"`
	EndsAt   time.Time `json:"endsAt"`
	Actor    string    `json:"actor"`
	Reason   string    `json:"reason"`
	At       time.Time `json:"at"`
}

// EgressAttackWindowDeclaration is what an operator declares.
type EgressAttackWindowDeclaration struct {
	WorkspaceID string
	ServiceIDs  []string
	StartsAt    time.Time
	EndsAt      time.Time
	Actor       string
	Reason      string
}

// egressAttackHour is the SQL predicate "the usage row alias is an
// egress_bytes hour a live attack window overlaps": the one place the rule is
// computed. Every read that turns egress into money, allowance or a spin-down
// uses it (the outbox, UsageBefore, the month sums, compaction and the usage
// read), so they cannot disagree. A window overlaps the hour [h, h+1h) when it
// starts before h+1h and ends after h, so a window that starts or ends inside
// an hour takes that whole hour. alias needs workspace_id, service_id, kind
// and window_start.
func egressAttackHour(alias string) string {
	return `(` + alias + `.kind = '` + UsageKindEgressBytes + `' AND EXISTS (
		SELECT 1 FROM egress_attack_windows eaw
		WHERE eaw.workspace_id = ` + alias + `.workspace_id
		  AND eaw.withdrawn_at IS NULL
		  AND eaw.starts_at < ` + alias + `.window_start + interval '1 hour'
		  AND eaw.ends_at > ` + alias + `.window_start
		  AND (eaw.service_ids = '{}' OR ` + alias + `.service_id = ANY (eaw.service_ids))))`
}

// HourSpan is a run of whole UTC hours [Start, End).
type HourSpan struct {
	Start, End time.Time
}

// EgressAttackHourSpans is the whole hours windows take off the bill inside
// [from, to), merged and in order: each window widened to the hours it
// overlaps, as egressAttackHour counts them, then clipped to [from, to).
func EgressAttackHourSpans(windows []EgressAttackWindow, from, to time.Time) []HourSpan {
	spans := make([]HourSpan, 0, len(windows))
	for _, w := range windows {
		start := w.StartsAt.UTC().Truncate(time.Hour)
		end := w.EndsAt.UTC().Truncate(time.Hour)
		if end.Before(w.EndsAt) {
			end = end.Add(time.Hour)
		}
		start, end = laterOf(start, from), earlierOf(end, to)
		if start.Before(end) {
			spans = append(spans, HourSpan{Start: start, End: end})
		}
	}
	slices.SortFunc(spans, func(a, b HourSpan) int { return a.Start.Compare(b.Start) })
	merged := spans[:0]
	for _, s := range spans {
		if n := len(merged); n > 0 && !s.Start.After(merged[n-1].End) {
			merged[n-1].End = laterOf(merged[n-1].End, s.End)
			continue
		}
		merged = append(merged, s)
	}
	return merged
}

func earlierOf(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

// validateEgressAttackSpan refuses a span that is empty, longer than
// EgressAttackWindowMaxSpan or ends more than EgressAttackWindowLead from now.
// A start earlier than EgressAttackWindowLookback is refused only when
// checkStart is set: an extension that keeps its start keeps it.
func validateEgressAttackSpan(start, end, now time.Time, checkStart bool) error {
	switch {
	case start.IsZero() || end.IsZero():
		return fmt.Errorf("%w: egress attack window: startsAt and endsAt are required", ErrInvalid)
	case !start.Before(end):
		return fmt.Errorf("%w: egress attack window: startsAt must be before endsAt", ErrInvalid)
	case end.Sub(start) > EgressAttackWindowMaxSpan:
		return fmt.Errorf("%w: egress attack window: a window lasts at most %d days; declare a longer attack as consecutive windows", ErrInvalid, EgressAttackWindowMaxSpan/(24*time.Hour))
	case end.After(now.Add(EgressAttackWindowLead)):
		return fmt.Errorf("%w: egress attack window: endsAt is more than %d hours from now; declare up to now and extend the window while the attack goes on", ErrInvalid, EgressAttackWindowLead/time.Hour)
	case checkStart && start.Before(now.Add(-EgressAttackWindowLookback)):
		return fmt.Errorf("%w: egress attack window: startsAt is more than %d days ago", ErrInvalid, EgressAttackWindowLookback/(24*time.Hour))
	}
	return nil
}

// normalizeEgressAttackActor trims and requires the actor and reason every
// change records, as a billing override requires them.
func normalizeEgressAttackActor(actor, reason string) (string, string, error) {
	actor, reason = strings.TrimSpace(actor), strings.TrimSpace(reason)
	if actor == "" || reason == "" {
		return "", "", fmt.Errorf("%w: egress attack window: actor and reason are required", ErrInvalid)
	}
	if len(reason) > egressAttackReasonMax {
		return "", "", fmt.Errorf("%w: egress attack window: reason is longer than %d characters", ErrInvalid, egressAttackReasonMax)
	}
	return actor, reason, nil
}

// normalizeServiceIDs trims, drops empties and duplicates, and sorts.
func normalizeServiceIDs(in []string) []string {
	out := make([]string, 0, len(in))
	for _, id := range in {
		if id = strings.TrimSpace(id); id != "" && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// DeclareEgressAttackWindow records a window over d's span. It refuses an
// unknown workspace (ErrNotFound), a span outside the bounds, a missing actor
// or reason, and a service that is not one of the workspace's metered
// resources (ErrInvalid each). now is the clock the bounds are read against.
func (s *PGStore) DeclareEgressAttackWindow(ctx context.Context, d EgressAttackWindowDeclaration, now time.Time) (EgressAttackWindow, error) {
	actor, reason, err := normalizeEgressAttackActor(d.Actor, d.Reason)
	if err != nil {
		return EgressAttackWindow{}, err
	}
	start, end := d.StartsAt.UTC(), d.EndsAt.UTC()
	if err := validateEgressAttackSpan(start, end, now.UTC(), true); err != nil {
		return EgressAttackWindow{}, err
	}
	services := normalizeServiceIDs(d.ServiceIDs)
	windowID := ids.New(ids.EgressAttackWindow)
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tenants WHERE id = $1)`, d.WorkspaceID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("egress attack window: workspace %s: %w", d.WorkspaceID, ErrNotFound)
		}
		if err := refuseForeignServices(ctx, tx, d.WorkspaceID, services); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO egress_attack_windows (id, workspace_id, service_ids, starts_at, ends_at, actor, reason, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)`,
			windowID, d.WorkspaceID, services, start, end, actor, reason, now.UTC()); err != nil {
			return classify("egress attack window", err)
		}
		return insertEgressAttackChange(ctx, tx, windowID, "declare", start, end, actor, reason, now)
	})
	if err != nil {
		return EgressAttackWindow{}, err
	}
	log.Printf("billing: egress attack window %s declared for %s over %s..%s (services %v) by %s: %s",
		windowID, d.WorkspaceID, start.Format(time.RFC3339), end.Format(time.RFC3339), services, actor, reason)
	return s.getEgressAttackWindow(ctx, d.WorkspaceID, windowID)
}

// refuseForeignServices refuses any id that is neither one of the workspace's
// services nor a resource whose egress bex has metered for it (a public
// Postgres or Key Value, or a service since deleted).
func refuseForeignServices(ctx context.Context, tx pgx.Tx, workspaceID string, services []string) error {
	if len(services) == 0 {
		return nil
	}
	rows, err := tx.Query(ctx, `
		SELECT s.id FROM unnest($2::text[]) AS s(id)
		WHERE NOT EXISTS (SELECT 1 FROM apps a WHERE a.id = s.id AND a.tenant_id = $1)
		  AND NOT EXISTS (SELECT 1 FROM usage_source_streams u
		                  WHERE u.workspace_id = $1 AND u.service_id = s.id AND u.kind = $3)
		  AND NOT EXISTS (SELECT 1 FROM usage_hourly h
		                  WHERE h.workspace_id = $1 AND h.service_id = s.id AND h.kind = $3)
		ORDER BY s.id`, workspaceID, services, UsageKindEgressBytes)
	if err != nil {
		return err
	}
	foreign, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	if len(foreign) > 0 {
		return fmt.Errorf("%w: egress attack window: %s is not a service of workspace %s", ErrInvalid, strings.Join(foreign, ", "), workspaceID)
	}
	return nil
}

func insertEgressAttackChange(ctx context.Context, tx pgx.Tx, windowID, action string, start, end time.Time, actor, reason string, at time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO egress_attack_window_changes (window_id, action, starts_at, ends_at, actor, reason, at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, windowID, action, start, end, actor, reason, at.UTC())
	return err
}

// ExtendEgressAttackWindow widens a live window: startsAt earlier, endsAt
// later, or both (a zero value keeps that end). It never narrows one; withdraw
// it and declare the narrower span instead. A withdrawn window is ErrConflict.
func (s *PGStore) ExtendEgressAttackWindow(ctx context.Context, workspaceID, windowID string, startsAt, endsAt time.Time, actor, reason string, now time.Time) (EgressAttackWindow, error) {
	actor, reason, err := normalizeEgressAttackActor(actor, reason)
	if err != nil {
		return EgressAttackWindow{}, err
	}
	var start, end time.Time
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		oldStart, oldEnd, withdrawn, err := lockEgressAttackWindow(ctx, tx, workspaceID, windowID)
		if err != nil {
			return err
		}
		if withdrawn {
			return fmt.Errorf("egress attack window %s was withdrawn; declare a new one: %w", windowID, ErrConflict)
		}
		start, end = oldStart, oldEnd
		if !startsAt.IsZero() {
			start = startsAt.UTC()
		}
		if !endsAt.IsZero() {
			end = endsAt.UTC()
		}
		switch {
		case start.After(oldStart) || end.Before(oldEnd):
			return fmt.Errorf("%w: egress attack window: an extension only widens a window; withdraw it and declare the narrower span", ErrInvalid)
		case start.Equal(oldStart) && end.Equal(oldEnd):
			return fmt.Errorf("%w: egress attack window: the extension changes neither startsAt nor endsAt", ErrInvalid)
		}
		if err := validateEgressAttackSpan(start, end, now.UTC(), !start.Equal(oldStart)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE egress_attack_windows SET starts_at = $2, ends_at = $3, updated_at = $4 WHERE id = $1`,
			windowID, start, end, now.UTC()); err != nil {
			return err
		}
		return insertEgressAttackChange(ctx, tx, windowID, "extend", start, end, actor, reason, now)
	})
	if err != nil {
		return EgressAttackWindow{}, err
	}
	log.Printf("billing: egress attack window %s of %s extended to %s..%s by %s: %s",
		windowID, workspaceID, start.Format(time.RFC3339), end.Format(time.RFC3339), actor, reason)
	return s.getEgressAttackWindow(ctx, workspaceID, windowID)
}

// WithdrawEgressAttackWindow ends a window's effect: its hours not yet sent to
// Stripe bill again. The row stays, with who withdrew it and why. Hours
// already sent as 0, and credits already issued, are not reversed. Withdrawing
// a withdrawn window changes nothing and reports changed false.
func (s *PGStore) WithdrawEgressAttackWindow(ctx context.Context, workspaceID, windowID, actor, reason string, now time.Time) (EgressAttackWindow, bool, error) {
	actor, reason, err := normalizeEgressAttackActor(actor, reason)
	if err != nil {
		return EgressAttackWindow{}, false, err
	}
	changed := false
	err = s.withTx(ctx, func(tx pgx.Tx) error {
		start, end, withdrawn, err := lockEgressAttackWindow(ctx, tx, workspaceID, windowID)
		if err != nil || withdrawn {
			return err
		}
		changed = true
		if _, err := tx.Exec(ctx, `
			UPDATE egress_attack_windows
			SET withdrawn_at = $2, withdrawn_by = $3, withdraw_reason = $4, updated_at = $2
			WHERE id = $1`, windowID, now.UTC(), actor, reason); err != nil {
			return err
		}
		return insertEgressAttackChange(ctx, tx, windowID, "withdraw", start, end, actor, reason, now)
	})
	if err != nil {
		return EgressAttackWindow{}, false, err
	}
	if changed {
		log.Printf("billing: egress attack window %s of %s withdrawn by %s: %s", windowID, workspaceID, actor, reason)
	}
	w, err := s.getEgressAttackWindow(ctx, workspaceID, windowID)
	return w, changed, err
}

// lockEgressAttackWindow reads one window of the workspace FOR UPDATE: its
// span and whether it was withdrawn. Another workspace's window is ErrNotFound.
func lockEgressAttackWindow(ctx context.Context, tx pgx.Tx, workspaceID, windowID string) (start, end time.Time, withdrawn bool, err error) {
	var withdrawnAt *time.Time
	if err = tx.QueryRow(ctx, `
		SELECT starts_at, ends_at, withdrawn_at FROM egress_attack_windows
		WHERE id = $1 AND workspace_id = $2 FOR UPDATE`, windowID, workspaceID).Scan(&start, &end, &withdrawnAt); err != nil {
		return time.Time{}, time.Time{}, false, classify("egress attack window", err)
	}
	return start.UTC(), end.UTC(), withdrawnAt != nil, nil
}

const egressAttackWindowColumns = `id, workspace_id, service_ids, starts_at, ends_at, actor, reason,
	created_at, updated_at, withdrawn_at, withdrawn_by, withdraw_reason`

func scanEgressAttackWindow(row pgx.Row) (EgressAttackWindow, error) {
	var w EgressAttackWindow
	err := row.Scan(&w.ID, &w.WorkspaceID, &w.ServiceIDs, &w.StartsAt, &w.EndsAt, &w.Actor, &w.Reason,
		&w.CreatedAt, &w.UpdatedAt, &w.WithdrawnAt, &w.WithdrawnBy, &w.WithdrawReason)
	if w.ServiceIDs == nil {
		w.ServiceIDs = []string{}
	}
	w.StartsAt, w.EndsAt, w.CreatedAt, w.UpdatedAt = w.StartsAt.UTC(), w.EndsAt.UTC(), w.CreatedAt.UTC(), w.UpdatedAt.UTC()
	if w.WithdrawnAt != nil {
		at := w.WithdrawnAt.UTC()
		w.WithdrawnAt = &at
	}
	return w, err
}

// ListEgressAttackWindows is every window of the workspace, withdrawn ones
// included, newest first, each with its changes in order. An unknown
// workspace is ErrNotFound.
func (s *PGStore) ListEgressAttackWindows(ctx context.Context, workspaceID string) ([]EgressAttackWindow, error) {
	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tenants WHERE id = $1)`, workspaceID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("egress attack windows: workspace %s: %w", workspaceID, ErrNotFound)
	}
	return s.egressAttackWindows(ctx, `workspace_id = $1 ORDER BY starts_at DESC, id`, workspaceID)
}

func (s *PGStore) getEgressAttackWindow(ctx context.Context, workspaceID, windowID string) (EgressAttackWindow, error) {
	windows, err := s.egressAttackWindows(ctx, `workspace_id = $1 AND id = $2`, workspaceID, windowID)
	if err != nil {
		return EgressAttackWindow{}, err
	}
	if len(windows) == 0 {
		return EgressAttackWindow{}, fmt.Errorf("egress attack window: %w", ErrNotFound)
	}
	return windows[0], nil
}

// egressAttackWindows reads the windows where holds, then their changes.
func (s *PGStore) egressAttackWindows(ctx context.Context, where string, args ...any) ([]EgressAttackWindow, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+egressAttackWindowColumns+` FROM egress_attack_windows WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	windows, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (EgressAttackWindow, error) {
		return scanEgressAttackWindow(row)
	})
	if err != nil || len(windows) == 0 {
		return windows, err
	}
	windowIDs := make([]string, len(windows))
	byID := make(map[string]int, len(windows))
	for i, w := range windows {
		windowIDs[i] = w.ID
		byID[w.ID] = i
	}
	changes, err := s.Pool.Query(ctx, `
		SELECT window_id, action, starts_at, ends_at, actor, reason, at
		FROM egress_attack_window_changes WHERE window_id = ANY ($1) ORDER BY at, action`, windowIDs)
	if err != nil {
		return nil, err
	}
	defer changes.Close()
	for changes.Next() {
		var windowID string
		var c EgressAttackWindowChange
		if err := changes.Scan(&windowID, &c.Action, &c.StartsAt, &c.EndsAt, &c.Actor, &c.Reason, &c.At); err != nil {
			return nil, err
		}
		c.StartsAt, c.EndsAt, c.At = c.StartsAt.UTC(), c.EndsAt.UTC(), c.At.UTC()
		i := byID[windowID]
		windows[i].Changes = append(windows[i].Changes, c)
	}
	return windows, changes.Err()
}

// EgressAttackWindowsOverlapping is the live windows of workspaceID that cover
// serviceID and overlap an hour of [from, to): exactly those egressAttackHour
// leaves serviceID's rows of those hours out for.
func (s *PGStore) EgressAttackWindowsOverlapping(ctx context.Context, workspaceID, serviceID string, from, to time.Time) ([]EgressAttackWindow, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("%w: egress attack windows: workspace id required", ErrInvalid)
	}
	// The hours of [from, to) run from from's hour to the end of the hour
	// holding the last instant before to.
	first := from.UTC().Truncate(time.Hour)
	last := to.UTC().Add(-time.Nanosecond).Truncate(time.Hour).Add(time.Hour)
	return s.egressAttackWindows(ctx, `workspace_id = $1 AND withdrawn_at IS NULL
		  AND starts_at < $4 AND ends_at > $3
		  AND (service_ids = '{}' OR $2 = ANY (service_ids))
		ORDER BY starts_at, id`, workspaceID, serviceID, first, last)
}

// EgressAttackCredit is one negative invoice item that gives back what a
// workspace-month's egress hours already sent to Stripe billed because of an
// attack window declared after they shipped (w4/m709 t004). Bytes and
// AmountCents are what it adds to the month's earlier credits.
type EgressAttackCredit struct {
	WorkspaceID   string
	Month         time.Time // first day of the UTC month
	Seq           int
	Bytes         int64
	AmountCents   int64
	CreatedAt     time.Time
	InvoiceItemID string
	SentAt        *time.Time
}

// EgressAttackCreditMonth is a workspace-month that a live window covers some
// of the exported egress hours of: where a credit may be owed.
type EgressAttackCreditMonth struct {
	WorkspaceID string
	Month       time.Time
}

// EgressAttackCreditMonths lists the workspace-months a live window covers an
// exported egress hour of, for workspaces that are billed (neither excluded
// nor comped).
func (s *PGStore) EgressAttackCreditMonths(ctx context.Context) ([]EgressAttackCreditMonth, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT DISTINCT w.workspace_id, (date_trunc('month', u.window_start AT TIME ZONE 'UTC'))::date
		FROM egress_attack_windows w
		JOIN tenants t ON t.id = w.workspace_id
		JOIN usage_hourly u ON u.workspace_id = w.workspace_id AND u.kind = $1
		WHERE w.withdrawn_at IS NULL
		  AND NOT t.billing_excluded AND NOT t.billing_comped
		  AND u.billing_export_state = 'emitted'
		  AND u.window_start > w.starts_at - interval '1 hour' AND u.window_start < w.ends_at
		  AND (w.service_ids = '{}' OR u.service_id = ANY (w.service_ids))
		ORDER BY 1, 2`, UsageKindEgressBytes)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (EgressAttackCreditMonth, error) {
		var m EgressAttackCreditMonth
		err := row.Scan(&m.WorkspaceID, &m.Month)
		m.Month = MonthStartUTC(m.Month)
		return m, err
	})
}

// EgressMonthRow is one egress_bytes hour of a workspace-month, as the credit
// reads it: Billed is what the export sent for it (its frozen billed quantity,
// or the whole quantity), meaningful only when Emitted.
type EgressMonthRow struct {
	WindowStart time.Time
	Quantity    int64
	Emitted     bool
	Billed      int64
	AttackHour  bool
}

// EgressAttackLedger is what the credit needs to know about one
// workspace-month: its egress hours in UsageBefore's order, the credits
// already recorded and the next credit's seq. Compacted says the month's
// hours were folded into usage_monthly, so its rows are gone and no credit
// can be worked out.
type EgressAttackLedger struct {
	Rows          []EgressMonthRow
	CreditedBytes int64
	CreditedCents int64
	NextSeq       int
	Compacted     bool
}

// EgressAttackLedger reads workspaceID's ledger for the month holding month.
func (s *PGStore) EgressAttackLedger(ctx context.Context, workspaceID string, month time.Time) (EgressAttackLedger, error) {
	start := MonthStartUTC(month)
	var l EgressAttackLedger
	if err := s.Pool.QueryRow(ctx, `
		SELECT COALESCE(sum(bytes), 0)::bigint, COALESCE(sum(amount_cents), 0)::bigint, COALESCE(max(seq), 0) + 1,
		       EXISTS (SELECT 1 FROM usage_monthly WHERE workspace_id = $1 AND kind = $3 AND month = $2::date)
		FROM egress_attack_credits WHERE workspace_id = $1 AND month = $2::date`,
		workspaceID, start.Format(time.DateOnly), UsageKindEgressBytes).Scan(&l.CreditedBytes, &l.CreditedCents, &l.NextSeq, &l.Compacted); err != nil {
		return EgressAttackLedger{}, err
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT u.window_start, u.quantity, u.billing_export_state = 'emitted',
		       COALESCE(u.billing_export_quantity, u.quantity), `+egressAttackHour("u")+`
		FROM usage_hourly u
		WHERE u.workspace_id = $1 AND u.kind = $4 AND u.quantity > 0
		  AND u.window_start >= $2 AND u.window_start < $3
		ORDER BY u.window_start, u.resource_kind, u.service_id`,
		workspaceID, start, start.AddDate(0, 1, 0), UsageKindEgressBytes)
	if err != nil {
		return EgressAttackLedger{}, err
	}
	l.Rows, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (EgressMonthRow, error) {
		var r EgressMonthRow
		err := row.Scan(&r.WindowStart, &r.Quantity, &r.Emitted, &r.Billed, &r.AttackHour)
		return r, err
	})
	return l, err
}

// RecordEgressAttackCredit inserts credit before Stripe hears of it and
// reports whether this caller inserted it. A replica that read the same
// ledger inserts the same seq, which does nothing, so a credit is recorded
// once however many replicas run.
func (s *PGStore) RecordEgressAttackCredit(ctx context.Context, credit EgressAttackCredit) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		INSERT INTO egress_attack_credits (workspace_id, month, seq, bytes, amount_cents)
		VALUES ($1, $2::date, $3, $4, $5)
		ON CONFLICT DO NOTHING`,
		credit.WorkspaceID, MonthStartUTC(credit.Month).Format(time.DateOnly), credit.Seq, credit.Bytes, credit.AmountCents)
	if err != nil {
		return false, classify("egress attack credit", err)
	}
	return tag.RowsAffected() == 1, nil
}

// UnsentEgressAttackCredits is up to limit recorded credits Stripe has not
// accepted yet, oldest first.
func (s *PGStore) UnsentEgressAttackCredits(ctx context.Context, limit int) ([]EgressAttackCredit, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT workspace_id, month, seq, bytes, amount_cents, created_at, invoice_item_id, sent_at
		FROM egress_attack_credits WHERE sent_at IS NULL
		ORDER BY created_at, workspace_id, month, seq LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (EgressAttackCredit, error) {
		var c EgressAttackCredit
		err := row.Scan(&c.WorkspaceID, &c.Month, &c.Seq, &c.Bytes, &c.AmountCents, &c.CreatedAt, &c.InvoiceItemID, &c.SentAt)
		c.Month = MonthStartUTC(c.Month)
		return c, err
	})
}

// MarkEgressAttackCreditSent records the invoice item Stripe created for
// credit. A credit already marked keeps its first item.
func (s *PGStore) MarkEgressAttackCreditSent(ctx context.Context, credit EgressAttackCredit, invoiceItemID string, at time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE egress_attack_credits SET invoice_item_id = $4, sent_at = $5
		WHERE workspace_id = $1 AND month = $2::date AND seq = $3 AND sent_at IS NULL`,
		credit.WorkspaceID, MonthStartUTC(credit.Month).Format(time.DateOnly), credit.Seq, invoiceItemID, at.UTC())
	return err
}
