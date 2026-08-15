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
	"fmt"
	"time"
)

// feedtail.go is the one watermark-tailing pager over the composed event feed
// (ListWebhookEvents). It lives with the query for the same reason FeedCommitLag
// does: the rules below are properties of THIS feed's cursor contract, not of
// either consumer.
//
// Two workers tail the feed — internal/webhooks (deliveries) and
// internal/notifications (push) — and until this seam existed they ran the same
// algorithm from two copies. Every one of its rules fails SILENTLY when broken:
// events are skipped, or redelivered forever, and nothing returns an error.
//
//   - The batch and the cursor advance in ONE Commit call. Splitting them either
//     drops events (advance first, then crash) or redelivers them (insert first,
//     then crash).
//   - A quiet window parks the cursor forward only once it lags by Park —
//     per-tick parking is a Postgres write every poll interval forever, and never
//     parking makes a restart re-read the whole empty window.
//   - A full page loops immediately; a short page ends the pass. "Always stop"
//     drains a backlog one page per tick; "always loop" never returns the tick.
//   - The cursor must be Loaded before it is read. A zero cursor means the
//     beginning of time, so tailing from one would re-deliver every event the
//     feed has ever held; TailFeed refuses instead.
//
// What stays with each consumer: its page size, its park interval, its verb
// filter, and its projection — the per-worker parts a reader must see.

// FeedCursor is one tailer's cached position in the composed feed: the durable
// watermark, read once per process and advanced in memory by each committed
// page. The cache is a pure optimization under multiple replicas — a stale
// cursor re-reads rows the consumer's own idempotency key then dedupes, so it
// converges rather than duplicating.
//
// The zero value is UNLOADED and is not usable until Load runs.
type FeedCursor struct {
	loaded bool
	at     time.Time
	key    string
}

// EnsureWatermark seeds a tailer's durable watermark at `at` if none exists yet
// and returns the current one — EnsureWebhookWatermark / EnsurePushWatermark.
type EnsureWatermark func(ctx context.Context, at time.Time) (time.Time, string, error)

// Load reads the durable watermark once per process, seeding it at `at` on first
// start so a newly enabled feature does not replay the feed's whole history.
// It is idempotent: once loaded, TailFeed's own commits keep the cache current,
// so later calls are a no-op and cost no round trip.
func (c *FeedCursor) Load(ctx context.Context, ensure EnsureWatermark, at time.Time) error {
	if c.loaded {
		return nil
	}
	loadedAt, key, err := ensure(ctx, at)
	if err != nil {
		return err
	}
	c.at, c.key, c.loaded = loadedAt, key, true
	return nil
}

// Loaded reports whether the durable watermark has been read yet. A consumer
// that projects rows OUTSIDE the feed (the agent-session push scan) uses it to
// stay behind the feed dispatch that anchors the watermark.
func (c *FeedCursor) Loaded() bool { return c.loaded }

// Position is the cached watermark — the (at, key) a consumer passes back to
// Commit when it wants the cursor left exactly where it is.
func (c *FeedCursor) Position() (time.Time, string) { return c.at, c.key }

// ErrFeedCursorUnloaded is returned when a pass is asked to tail from a cursor
// whose durable watermark was never read. Failing here is the point: the zero
// cursor is the beginning of time, so the alternative is a silent replay of
// every event in the feed.
var ErrFeedCursorUnloaded = errors.New("feed cursor read before Load")

// DefaultFeedPark bounds how far the durable watermark may lag the read window
// before an otherwise-quiet pass persists it forward.
//
// It lives here, like FeedCommitLag, because it is a property of the shared
// watermark row rather than a per-consumer tunable: parking per tick would be a
// Postgres write transaction every poll interval forever on a platform where
// nothing is happening, and parking never makes a restart re-read the whole
// empty window. A minute makes a quiet-but-subscribed platform cost one small
// write a minute and re-read at most a minute of already-empty window — the
// same trade for every tailer. Page size, by contrast, is genuinely
// per-consumer (the push worker does per-row lookups that the webhook worker
// does not) and stays in each package.
const DefaultFeedPark = time.Minute

// FeedPass is one pass over the feed: the window to read, the per-consumer
// knobs, and the three seams (read, project, commit) that make a tailer what it
// is. T is the consumer's own batch item — a webhook delivery, a push
// notification — which only its own Project and Commit ever see.
type FeedPass[T any] struct {
	// Until is the read window's exclusive end, already held back by
	// FeedCommitLag.
	Until time.Time
	// Floor optionally starts the READ later than the durable watermark, when the
	// consumer can prove nothing before it is deliverable anyway (webhooks: the
	// oldest enabled endpoint's creation). A read-side skip only — nothing is
	// persisted, and the park check below still measures the durable cursor, so a
	// floor can never park the watermark forward over unread rows.
	Floor time.Time
	// Verbs is the audit arm's verb allow-list. Both audit arms filter with
	// `e.verb = ANY($4)`, so an empty Verbs matches NO verb and drops the audit
	// arm from the union entirely — it is "exclude audit rows", not "every verb".
	// That is what the push worker wants (it projects no audit source at all);
	// a consumer that does want audit events must name them.
	Verbs []string
	// Tenants scopes the read to the workspaces this consumer has recipients in.
	// Required: every arm filters with `= ANY($5)`, so an empty Tenants returns
	// zero rows — which this pager cannot tell from a quiet feed, and would answer
	// by PARKING the durable watermark past events it never read. A consumer with
	// no tenants must not run a pass at all.
	Tenants []string
	// Limit is one page's size. A full page means more may be waiting, so the
	// pass reads again immediately instead of costing a whole tick.
	Limit int
	// Park is how far the durable cursor may lag the read window before an
	// otherwise-quiet pass persists it forward; 0 => DefaultFeedPark.
	Park time.Duration

	// List reads one page after (afterAt, afterKey) — ListWebhookEvents.
	List func(ctx context.Context, afterAt time.Time, afterKey string, until time.Time, verbs, tenants []string, limit int) ([]WebhookEventRow, error)
	// Project turns one page into the consumer's batch. It runs per page, not per
	// row, so each consumer keeps its own fan-out loop (and its own clock read)
	// intact. Returning an empty batch is normal — the cursor still advances past
	// rows this consumer does not deliver.
	Project func(ctx context.Context, rows []WebhookEventRow) ([]T, error)
	// Commit inserts the batch AND advances the durable watermark to (at, key) in
	// ONE transaction — EnqueueWebhookDeliveries / EnqueuePushNotifications.
	Commit func(ctx context.Context, batch []T, at time.Time, key string) error
}

// validate rejects a pass before it touches the store. Every field named here
// has a zero value that fails SILENTLY — the pass returns nil having delivered
// nothing, or panics only once real events arrive — so a mis-wired tailer would
// otherwise look healthy in tests and in production logs alike. Floor and Verbs
// are absent on purpose: their zero values are meaningful.
//
// The nil-seam checks earn their place even though a nil func would panic:
// core.Poll has no recover, so the panic takes bex-api down, and a nil Project
// or Commit is reached only on the first page that HAS rows — it survives every
// quiet-feed test and fires in production on the first real event.
func (pass FeedPass[T]) validate() error {
	switch {
	case pass.Until.IsZero():
		return errors.New("feed pass needs a read window (Until)")
	case len(pass.Tenants) == 0:
		// The dangerous one: an empty tenant set reads zero rows from a busy feed,
		// which this pager answers by parking the watermark past them.
		return errors.New("feed pass needs at least one tenant; an empty set reads a false quiet window")
	case pass.Limit < 1:
		return fmt.Errorf("feed pass limit must be positive, got %d", pass.Limit)
	case pass.List == nil, pass.Project == nil, pass.Commit == nil:
		return errors.New("feed pass needs List, Project and Commit")
	}
	return nil
}

// TailFeed advances cur through the feed, projecting and committing a page at a
// time until a page comes back short or empty. The cursor is advanced only after
// its page's Commit returns, so a failed write is re-read rather than skipped.
func TailFeed[T any](ctx context.Context, cur *FeedCursor, pass FeedPass[T]) error {
	if cur == nil || !cur.loaded {
		return ErrFeedCursorUnloaded
	}
	if err := pass.validate(); err != nil {
		return err
	}
	if pass.Park <= 0 {
		pass.Park = DefaultFeedPark
	}
	// The read may start at the floor; the durable cursor stays where it is.
	readAt, readKey := cur.at, cur.key
	if pass.Floor.After(readAt) {
		readAt, readKey = pass.Floor, ""
	}
	for {
		rows, err := pass.List(ctx, readAt, readKey, pass.Until, pass.Verbs, pass.Tenants, pass.Limit)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			// Quiet window. Park against the DURABLE cursor, never the floor: the
			// floor skipped rows that are merely undeliverable to this consumer, and
			// persisting past them would strand any tailer state behind it.
			if pass.Until.Sub(cur.at) > pass.Park {
				if err := pass.Commit(ctx, nil, pass.Until, ""); err != nil {
					return err
				}
				cur.at, cur.key = pass.Until, ""
			}
			return nil
		}
		batch, err := pass.Project(ctx, rows)
		if err != nil {
			return err
		}
		last := rows[len(rows)-1]
		if err := pass.Commit(ctx, batch, last.CursorAt, last.Key); err != nil {
			return err
		}
		cur.at, cur.key = last.CursorAt, last.Key
		readAt, readKey = last.CursorAt, last.Key
		if len(rows) < pass.Limit {
			return nil
		}
	}
}
