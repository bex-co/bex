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
	"errors"
	"testing"
	"time"
)

// TailFeed is the one pager both feed tailers run on, so every rule it enforces
// is now enforced in exactly one place — and breaking any of them is silent at
// runtime (events skipped, or redelivered forever, with no error anywhere).
// internal/webhooks and internal/notifications each keep their own
// watermark_test.go pinning how THEY wire this seam; these tests pin the seam.

// tailProbe records how a pass drove the store: every page it asked for and
// every commit it made.
type tailProbe struct {
	pages   [][]WebhookEventRow // handed out in order, one per List call
	reads   []tailRead
	commits []tailCommit
	listErr error
	failAt  int // 1-based commit call that returns an error; 0 => none
}

type tailRead struct {
	at    time.Time
	key   string
	until time.Time
	limit int
}

type tailCommit struct {
	items int
	at    time.Time
	key   string
}

func (p *tailProbe) list(_ context.Context, afterAt time.Time, afterKey string, until time.Time, _, _ []string, limit int) ([]WebhookEventRow, error) {
	p.reads = append(p.reads, tailRead{at: afterAt, key: afterKey, until: until, limit: limit})
	if p.listErr != nil {
		return nil, p.listErr
	}
	if len(p.reads) > len(p.pages) {
		return nil, nil
	}
	return p.pages[len(p.reads)-1], nil
}

func (p *tailProbe) commit(_ context.Context, batch []WebhookEventRow, at time.Time, key string) error {
	p.commits = append(p.commits, tailCommit{items: len(batch), at: at, key: key})
	if p.failAt == len(p.commits) {
		return errors.New("commit failed")
	}
	return nil
}

// passthrough projects every row, so these tests exercise the pager rather than
// any consumer's fan-out.
func passthrough(_ context.Context, rows []WebhookEventRow) ([]WebhookEventRow, error) {
	return rows, nil
}

func feedRow(key string, at time.Time) WebhookEventRow {
	return WebhookEventRow{Key: key, At: at, CursorAt: at, TenantID: "tea-a", Source: EventSourceDeploy}
}

func loadedCursor(t *testing.T, at time.Time) *FeedCursor {
	t.Helper()
	cur := &FeedCursor{}
	ensure := func(_ context.Context, _ time.Time) (time.Time, string, error) { return at, "", nil }
	if err := cur.Load(context.Background(), ensure, at); err != nil {
		t.Fatalf("load cursor: %v", err)
	}
	return cur
}

func basePass(p *tailProbe, until time.Time) FeedPass[WebhookEventRow] {
	return FeedPass[WebhookEventRow]{
		Until:   until,
		Tenants: []string{"tea-a"},
		Limit:   3,
		Park:    time.Minute,
		List:    p.list,
		Project: passthrough,
		Commit:  p.commit,
	}
}

// The zero cursor is the beginning of time. Tailing from one would re-deliver
// every event the feed has ever held — to every subscriber at once — so the pass
// must refuse rather than read.
func TestTailFeedRefusesAnUnloadedCursor(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	p := &tailProbe{pages: [][]WebhookEventRow{{feedRow("dep-1", now.Add(-time.Hour))}}}

	err := TailFeed(context.Background(), &FeedCursor{}, basePass(p, now))
	if !errors.Is(err, ErrFeedCursorUnloaded) {
		t.Errorf("error = %v, want ErrFeedCursorUnloaded", err)
	}
	if len(p.reads) != 0 || len(p.commits) != 0 {
		t.Errorf("touched the store before refusing: %d reads, %d commits", len(p.reads), len(p.commits))
	}
	if err := TailFeed(context.Background(), nil, basePass(p, now)); !errors.Is(err, ErrFeedCursorUnloaded) {
		t.Errorf("nil cursor: error = %v, want ErrFeedCursorUnloaded", err)
	}
}

// Every field whose zero value fails SILENTLY is rejected before the pass reads
// anything. The table is the point: each of these compiles, and each produces a
// pager that looks healthy — no error, no log line — while doing the wrong thing
// forever. Floor and Verbs are absent deliberately; their zero values are
// meaningful (see the "genuinely optional" case below).
func TestTailFeedRefusesAPassWhoseZeroValueWouldFailSilently(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	rows := [][]WebhookEventRow{{feedRow("dep-1", now.Add(-time.Minute))}}

	cases := []struct {
		name   string
		break_ func(*FeedPass[WebhookEventRow])
		why    string
	}{
		{"zero Until", func(p *FeedPass[WebhookEventRow]) { p.Until = time.Time{} },
			"reads an empty window forever and never parks"},
		{"nil Tenants", func(p *FeedPass[WebhookEventRow]) { p.Tenants = nil },
			"every arm's tenant filter matches nothing, so the pass reads a false quiet window and PARKS the durable watermark past unread events"},
		{"empty Tenants", func(p *FeedPass[WebhookEventRow]) { p.Tenants = []string{} },
			"same as nil — an empty SQL array matches nothing"},
		{"zero Limit", func(p *FeedPass[WebhookEventRow]) { p.Limit = 0 },
			"len(rows) < Limit is the loop's only exit on a busy feed"},
		{"negative Limit", func(p *FeedPass[WebhookEventRow]) { p.Limit = -1 }, "same"},
		{"nil List", func(p *FeedPass[WebhookEventRow]) { p.List = nil },
			"panics on the first tick, taking bex-api down"},
		{"nil Project", func(p *FeedPass[WebhookEventRow]) { p.Project = nil },
			"panics on the first page that HAS rows — so it survives every quiet-feed test"},
		{"nil Commit", func(p *FeedPass[WebhookEventRow]) { p.Commit = nil },
			"panics on the first commit or park"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &tailProbe{pages: rows}
			pass := basePass(p, now)
			tc.break_(&pass)
			cur := loadedCursor(t, now.Add(-2*time.Hour)) // lagging enough to park
			err := TailFeed(context.Background(), cur, pass)
			if err == nil {
				t.Fatalf("accepted a pass with %s — %s", tc.name, tc.why)
			}
			if len(p.reads) != 0 || len(p.commits) != 0 {
				t.Errorf("touched the store before refusing: %d reads, %d commits", len(p.reads), len(p.commits))
			}
			if at, _ := cur.Position(); !at.Equal(now.Add(-2 * time.Hour)) {
				t.Errorf("cursor moved to %v — a refused pass must not touch the watermark", at)
			}
		})
	}

	t.Run("Floor and Verbs are genuinely optional", func(t *testing.T) {
		p := &tailProbe{pages: rows}
		pass := basePass(p, now) // neither Floor nor Verbs set
		if err := TailFeed(context.Background(), loadedCursor(t, now.Add(-time.Hour)), pass); err != nil {
			t.Errorf("rejected a pass with no Floor/Verbs: %v", err)
		}
	})
}

// Park is the one knob whose right value is a property of the feed rather than
// the consumer, so an unset Park takes the shared default instead of degrading
// to "write the watermark on every quiet tick, forever".
func TestUnsetParkTakesTheSharedDefault(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)

	p := &tailProbe{}
	pass := basePass(p, now)
	pass.Park = 0
	// Lagging by less than the default: an unguarded zero Park would write here.
	if err := TailFeed(context.Background(), loadedCursor(t, now.Add(-DefaultFeedPark/2)), pass); err != nil {
		t.Fatalf("tail: %v", err)
	}
	if len(p.commits) != 0 {
		t.Errorf("an unset Park wrote the watermark %d times inside the default interval", len(p.commits))
	}

	p = &tailProbe{}
	pass = basePass(p, now)
	pass.Park = 0
	if err := TailFeed(context.Background(), loadedCursor(t, now.Add(-2*DefaultFeedPark)), pass); err != nil {
		t.Fatalf("tail: %v", err)
	}
	if len(p.commits) != 1 {
		t.Errorf("an unset Park did not park past the default interval: %d writes", len(p.commits))
	}
}

// Load reads the durable watermark once per process: it is the tick-rate I/O
// this cache exists to remove, and after the first page the cache — not the
// database — is the position the next pass reads from.
func TestLoadReadsTheDurableWatermarkOnceThenTracksCommittedPages(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	seeded := now.Add(-time.Hour)
	ensures := 0
	ensure := func(_ context.Context, _ time.Time) (time.Time, string, error) {
		ensures++
		return seeded, "dep-seed", nil
	}

	cur := &FeedCursor{}
	if cur.Loaded() {
		t.Error("the zero cursor reports itself loaded")
	}
	for range 3 {
		if err := cur.Load(context.Background(), ensure, now); err != nil {
			t.Fatalf("load: %v", err)
		}
	}
	if ensures != 1 {
		t.Errorf("ensured the watermark %d times, want 1 — later ticks must cost no round trip", ensures)
	}
	if !cur.Loaded() {
		t.Error("cursor is not loaded after Load")
	}
	if at, key := cur.Position(); !at.Equal(seeded) || key != "dep-seed" {
		t.Errorf("position = (%v,%q), want the durable watermark (%v,\"dep-seed\")", at, key, seeded)
	}

	// One short page, then the cursor must track its last row.
	last := now.Add(-30 * time.Minute)
	p := &tailProbe{pages: [][]WebhookEventRow{{feedRow("dep-1", last.Add(-time.Second)), feedRow("dep-2", last)}}}
	if err := TailFeed(context.Background(), cur, basePass(p, now)); err != nil {
		t.Fatalf("tail: %v", err)
	}
	if at, key := cur.Position(); !at.Equal(last) || key != "dep-2" {
		t.Errorf("position = (%v,%q), want the page's last row (%v,\"dep-2\")", at, key, last)
	}
	if ensures != 1 {
		t.Errorf("tailing re-read the durable watermark (%d ensures)", ensures)
	}
	// The read starts from the cursor it was given, not from zero.
	if got := p.reads[0]; !got.at.Equal(seeded) || got.key != "dep-seed" {
		t.Errorf("first read started at (%v,%q), want the loaded watermark", got.at, got.key)
	}
}

// The batch and the cursor advance in ONE call. Splitting them either drops
// events (advance first, then crash) or redelivers them (insert first, then
// crash) — and a crash is the only way to observe the difference.
func TestBatchAndCursorAdvanceInOneCommit(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	last := now.Add(-time.Minute)
	p := &tailProbe{pages: [][]WebhookEventRow{{feedRow("dep-1", last.Add(-time.Second)), feedRow("dep-2", last)}}}

	if err := TailFeed(context.Background(), loadedCursor(t, now.Add(-time.Hour)), basePass(p, now)); err != nil {
		t.Fatalf("tail: %v", err)
	}
	if len(p.commits) != 1 {
		t.Fatalf("commits = %d, want exactly 1 carrying both the batch and the cursor: %+v", len(p.commits), p.commits)
	}
	if got := p.commits[0]; got.items != 2 || !got.at.Equal(last) || got.key != "dep-2" {
		t.Errorf("commit = %+v, want 2 items at the page's last row (%v,\"dep-2\")", got, last)
	}
}

// A failed commit must leave the cursor where it was, so the next tick re-reads
// the page instead of stepping over it.
func TestAFailedCommitDoesNotAdvanceTheCursor(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	seeded := now.Add(-time.Hour)
	p := &tailProbe{
		pages:  [][]WebhookEventRow{{feedRow("dep-1", now.Add(-time.Minute))}},
		failAt: 1,
	}
	cur := loadedCursor(t, seeded)

	if err := TailFeed(context.Background(), cur, basePass(p, now)); err == nil {
		t.Fatal("tail returned nil after a failed commit")
	}
	if at, _ := cur.Position(); !at.Equal(seeded) {
		t.Errorf("cursor moved to %v after a failed commit, want it left at %v", at, seeded)
	}
}

// A full page means more may be waiting, so the pass reads again immediately; a
// short page ends it. "Always stop" drains a backlog one page per tick forever;
// "always loop" never returns the tick.
func TestFullPageLoopsAndShortPageEndsThePass(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	at := now.Add(-time.Hour)
	full := []WebhookEventRow{feedRow("dep-1", at), feedRow("dep-2", at.Add(time.Second)), feedRow("dep-3", at.Add(2*time.Second))}

	t.Run("short page reads once", func(t *testing.T) {
		p := &tailProbe{pages: [][]WebhookEventRow{full[:2]}}
		if err := TailFeed(context.Background(), loadedCursor(t, at.Add(-time.Hour)), basePass(p, now)); err != nil {
			t.Fatalf("tail: %v", err)
		}
		if len(p.reads) != 1 {
			t.Errorf("reads = %d, want 1 for a short page", len(p.reads))
		}
	})

	t.Run("full page reads again from its last row", func(t *testing.T) {
		p := &tailProbe{pages: [][]WebhookEventRow{full}} // the 2nd read falls off the end => empty
		if err := TailFeed(context.Background(), loadedCursor(t, at.Add(-time.Hour)), basePass(p, now)); err != nil {
			t.Fatalf("tail: %v", err)
		}
		if len(p.reads) != 2 {
			t.Fatalf("reads = %d, want 2: a full page must be followed by another read", len(p.reads))
		}
		if got := p.reads[1]; !got.at.Equal(at.Add(2*time.Second)) || got.key != "dep-3" {
			t.Errorf("second read started at (%v,%q), want the first page's last row", got.at, got.key)
		}
	})
}

// A quiet window must not write the watermark on every tick — that is a Postgres
// write every poll interval, forever, on a platform where nothing is happening.
func TestQuietWindowParksOnlyOncePastTheParkInterval(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)

	t.Run("inside the park interval writes nothing", func(t *testing.T) {
		p := &tailProbe{}
		cur := loadedCursor(t, now.Add(-30*time.Second))
		if err := TailFeed(context.Background(), cur, basePass(p, now)); err != nil {
			t.Fatalf("tail: %v", err)
		}
		if len(p.commits) != 0 {
			t.Errorf("wrote the watermark %d times inside the park interval, want 0", len(p.commits))
		}
	})

	t.Run("past the park interval writes once, empty, and moves the cursor", func(t *testing.T) {
		p := &tailProbe{}
		cur := loadedCursor(t, now.Add(-2*time.Minute))
		if err := TailFeed(context.Background(), cur, basePass(p, now)); err != nil {
			t.Fatalf("tail: %v", err)
		}
		if len(p.commits) != 1 {
			t.Fatalf("watermark writes = %d, want exactly 1: %+v", len(p.commits), p.commits)
		}
		if got := p.commits[0]; got.items != 0 || !got.at.Equal(now) || got.key != "" {
			t.Errorf("park write = %+v, want an empty batch parked at %v with no key", got, now)
		}
		if at, key := cur.Position(); !at.Equal(now) || key != "" {
			t.Errorf("cursor = (%v,%q), want (%v,\"\") — a restart must not re-read the parked window", at, key, now)
		}
	})
}

// Floor moves the READ forward when the consumer can prove nothing older is
// deliverable to it; the durable cursor stays put. The two must not be confused:
// the floor is a per-consumer optimization, the cursor is what a restart resumes
// from.
func TestFloorMovesTheReadForwardButNotTheCursor(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	seeded := now.Add(-2 * time.Hour)
	floor := now.Add(-10 * time.Minute)

	t.Run("a floor ahead of the cursor starts the read there", func(t *testing.T) {
		p := &tailProbe{}
		cur := loadedCursor(t, seeded)
		pass := basePass(p, now)
		pass.Floor = floor
		if err := TailFeed(context.Background(), cur, pass); err != nil {
			t.Fatalf("tail: %v", err)
		}
		if got := p.reads[0]; !got.at.Equal(floor) || got.key != "" {
			t.Errorf("read started at (%v,%q), want the floor (%v,\"\")", got.at, got.key, floor)
		}
	})

	t.Run("a floor behind the cursor is ignored", func(t *testing.T) {
		p := &tailProbe{}
		cur := loadedCursor(t, floor)
		pass := basePass(p, now)
		pass.Floor = seeded // older than the cursor
		if err := TailFeed(context.Background(), cur, pass); err != nil {
			t.Fatalf("tail: %v", err)
		}
		if got := p.reads[0]; !got.at.Equal(floor) {
			t.Errorf("read started at %v, want the cursor %v — a floor must never rewind the read", got.at, floor)
		}
	})

	// The park check measures the DURABLE cursor. Measuring the floor instead
	// looks identical on a busy feed and on a consumer with no floor, but it
	// strands the durable watermark: a consumer whose floor tracks now() would
	// never park, so every restart re-reads the whole window back to the seed.
	t.Run("park measures the cursor, not the floor", func(t *testing.T) {
		p := &tailProbe{}
		cur := loadedCursor(t, seeded)
		pass := basePass(p, now)
		pass.Floor = now.Add(-time.Second) // well inside the park interval
		if err := TailFeed(context.Background(), cur, pass); err != nil {
			t.Fatalf("tail: %v", err)
		}
		if len(p.commits) != 1 {
			t.Fatalf("watermark writes = %d, want 1: the cursor lags 2h and must park: %+v", len(p.commits), p.commits)
		}
		if at, _ := cur.Position(); !at.Equal(now) {
			t.Errorf("cursor = %v, want it parked forward to %v", at, now)
		}
	})
}

// Project is the consumer's own seam and its error must abort the pass with the
// cursor untouched — a page whose projection failed has not been delivered.
func TestProjectErrorAbortsThePassWithoutCommitting(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	seeded := now.Add(-time.Hour)
	p := &tailProbe{pages: [][]WebhookEventRow{{feedRow("dep-1", now.Add(-time.Minute))}}}
	cur := loadedCursor(t, seeded)
	pass := basePass(p, now)
	boom := errors.New("project failed")
	pass.Project = func(context.Context, []WebhookEventRow) ([]WebhookEventRow, error) { return nil, boom }

	if err := TailFeed(context.Background(), cur, pass); !errors.Is(err, boom) {
		t.Errorf("error = %v, want the projection's own error", err)
	}
	if len(p.commits) != 0 {
		t.Errorf("committed %d times despite a failed projection", len(p.commits))
	}
	if at, _ := cur.Position(); !at.Equal(seeded) {
		t.Errorf("cursor moved to %v, want it left at %v", at, seeded)
	}
}
