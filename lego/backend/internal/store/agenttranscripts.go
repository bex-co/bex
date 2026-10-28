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
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/agenttranscript"
)

// LegacyPartLimitReason is the truncation reason the pre-w10/m16 harvest wrote
// when a turn's driver log exceeded 5,000 raw chunks (or held an unparseable
// line). The compaction clears it on a turn whose stored rows prove complete.
const LegacyPartLimitReason = "driver log parse/part limit reached"

// TranscriptItemMerge is one Completer harvest of a turn: the items folded
// from the driver log (agenttranscript.Coalesce), ascending by index.
type TranscriptItemMerge struct {
	Turn  int
	Items []AgentSessionTranscriptPart
	// FirstChunk/LastChunk are the lowest and highest driver ordinals the items
	// were folded from. A fold that starts at ordinal 0 and reaches at least
	// the turn's last stored legacy chunk supersedes those raw rows entirely.
	FirstChunk, LastChunk int64
	// Quota is the session's cumulative transcript byte cap.
	Quota int64
}

// TranscriptMergeResult reports what one harvest merge wrote.
type TranscriptMergeResult struct {
	Inserted, Upgraded, Replaced int
	QuotaReached                 bool
}

type transcriptRowInfo struct {
	state int16
	bytes int64
}

// MergeAgentSessionTranscriptItems idempotently merges a harvested turn under
// the session's append lock: it inserts every missing item index, replaces a
// partial item the live tee stored at disconnect with its complete copy, and —
// for a turn still stored as legacy raw chunks (an attach that spanned the
// w10/m16 deploy) — swaps the raw rows for the items when the fold covers them.
// Every write is charged against Quota cumulatively; the first item that would
// exceed it stops the merge (QuotaReached).
//
// bex-api only: the gateway's database role may INSERT, never UPDATE or
// DELETE, transcript rows.
func (s *PGStore) MergeAgentSessionTranscriptItems(ctx context.Context, sessionID string, m TranscriptItemMerge) (TranscriptMergeResult, error) {
	var res TranscriptMergeResult
	if len(m.Items) == 0 {
		return res, nil
	}
	items := append([]AgentSessionTranscriptPart(nil), m.Items...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].PartIndex < items[j].PartIndex })
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		res = TranscriptMergeResult{}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, sessionID); err != nil {
			return err
		}
		var total, seq int64
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(sum(octet_length(part)), 0), COALESCE(max(seq), -1)
			FROM agent_session_transcripts WHERE session_id=$1`, sessionID).Scan(&total, &seq); err != nil {
			return err
		}
		existing, err := turnRowInfo(ctx, tx, sessionID, m.Turn)
		if err != nil {
			return err
		}
		legacyMax, turnBytes, legacy := int64(-1), int64(0), false
		for index, row := range existing {
			if row.state == transcriptRowLegacyChunk {
				legacy = true
				legacyMax = max(legacyMax, index)
			}
			turnBytes += row.bytes
		}
		if legacy && m.FirstChunk == 0 && m.LastChunk >= legacyMax {
			tag, err := tx.Exec(ctx, `DELETE FROM agent_session_transcripts WHERE session_id=$1 AND turn=$2`, sessionID, m.Turn)
			if err != nil {
				return err
			}
			res.Replaced = int(tag.RowsAffected())
			total -= turnBytes
			existing = map[int64]transcriptRowInfo{}
		}
		for _, it := range items {
			size := int64(len(it.Part))
			if row, ok := existing[it.PartIndex]; ok {
				if row.state != transcriptRowPartialItem || it.Partial {
					continue
				}
				if total-row.bytes+size > m.Quota {
					res.QuotaReached = true
					return nil
				}
				if _, err := tx.Exec(ctx, `
					UPDATE agent_session_transcripts SET part=$4, item_state=$5
					WHERE session_id=$1 AND turn=$2 AND part_index=$3`,
					sessionID, m.Turn, it.PartIndex, string(it.Part), int16(transcriptRowItem)); err != nil {
					return err
				}
				total += size - row.bytes
				res.Upgraded++
				continue
			}
			if total+size > m.Quota {
				res.QuotaReached = true
				return nil
			}
			it.Item = true
			if _, err := tx.Exec(ctx, `
				INSERT INTO agent_session_transcripts (session_id, seq, turn, part_index, part, item_state)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				sessionID, seq+1, m.Turn, it.PartIndex, string(it.Part), transcriptRowState(it)); err != nil {
				return err
			}
			seq++
			total += size
			res.Inserted++
		}
		return nil
	})
	return res, classify("agent session transcript", err)
}

func turnRowInfo(ctx context.Context, tx pgx.Tx, sessionID string, turn int) (map[int64]transcriptRowInfo, error) {
	rows, err := tx.Query(ctx, `
		SELECT part_index, item_state, octet_length(part)
		FROM agent_session_transcripts WHERE session_id=$1 AND turn=$2`, sessionID, turn)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]transcriptRowInfo{}
	for rows.Next() {
		var index int64
		var row transcriptRowInfo
		if err := rows.Scan(&index, &row.state, &row.bytes); err != nil {
			return nil, err
		}
		out[index] = row
	}
	return out, rows.Err()
}

// TranscriptCompactionOptions tunes CompactAgentSessionTranscripts.
type TranscriptCompactionOptions struct {
	// DryRun computes and reports the rewrite without writing anything.
	DryRun bool
	// BatchSessions is how many candidate sessions one listing query returns
	// (default 50). Each session is rewritten in its own transaction.
	BatchSessions int
	// Logf receives one line per rewritten turn (nil: silent).
	Logf func(format string, args ...any)
}

// TranscriptCompactionStats is the compaction's tally.
type TranscriptCompactionStats struct {
	Sessions, Turns, SkippedOpenTurns, FlagsCleared int
	RowsBefore, RowsAfter                           int
	BytesBefore, BytesAfter                         int64
}

func (s *TranscriptCompactionStats) add(o TranscriptCompactionStats) {
	s.Sessions += o.Sessions
	s.Turns += o.Turns
	s.SkippedOpenTurns += o.SkippedOpenTurns
	s.FlagsCleared += o.FlagsCleared
	s.RowsBefore += o.RowsBefore
	s.RowsAfter += o.RowsAfter
	s.BytesBefore += o.BytesBefore
	s.BytesAfter += o.BytesAfter
}

var errCompactionDryRun = errors.New("transcript compaction dry run")

// CompactAgentSessionTranscripts is the one-shot w10/m16 backfill: it rewrites
// every settled turn still stored as legacy raw UI-message chunks
// (item_state = 0) into completed items with the same coalescer the live paths
// use, keeping each item's replay position (the seq of its first chunk). The
// legacy rows themselves are the marker — the partial index over them is empty
// once the backfill has drained, so a later run costs one index probe. It is
// idempotent and resumable (a crash leaves whole sessions either rewritten or
// untouched), batched per session under the transcript append lock, and skips
// a turn that may still be running (the Completer harvest rewrites that one).
// A turn the old 5,000-part harvest cap marked truncated whose rows prove
// complete (contiguous ordinals ending in `finish`) has the flag cleared.
func (s *PGStore) CompactAgentSessionTranscripts(ctx context.Context, opts TranscriptCompactionOptions) (TranscriptCompactionStats, error) {
	batch := opts.BatchSessions
	if batch <= 0 {
		batch = 50
	}
	var stats TranscriptCompactionStats
	after := ""
	for {
		rows, err := s.Pool.Query(ctx, `
			SELECT DISTINCT session_id FROM agent_session_transcripts
			WHERE item_state = 0 AND session_id > $1
			ORDER BY session_id LIMIT $2`, after, batch)
		if err != nil {
			return stats, err
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return stats, err
		}
		if len(ids) == 0 {
			return stats, nil
		}
		for _, id := range ids {
			one, err := s.compactSessionTranscript(ctx, id, opts)
			if err != nil {
				return stats, fmt.Errorf("compact transcript of %s: %w", id, err)
			}
			stats.add(one)
			after = id
		}
	}
}

func (s *PGStore) compactSessionTranscript(ctx context.Context, sessionID string, opts TranscriptCompactionOptions) (TranscriptCompactionStats, error) {
	var stats TranscriptCompactionStats
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		stats = TranscriptCompactionStats{}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, sessionID); err != nil {
			return err
		}
		var currentTurn int
		if err := tx.QueryRow(ctx, `SELECT turns FROM agent_sessions WHERE id=$1`, sessionID).Scan(&currentTurn); err != nil {
			return err
		}
		type turnFacts struct {
			settled bool
			reason  string
		}
		facts := map[int]turnFacts{}
		rows, err := tx.Query(ctx, `
			SELECT turn, completed_at IS NOT NULL, truncation_reason
			FROM agent_session_turns WHERE session_id=$1`, sessionID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var turn int
			var f turnFacts
			if err := rows.Scan(&turn, &f.settled, &f.reason); err != nil {
				rows.Close()
				return err
			}
			facts[turn] = f
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		rows, err = tx.Query(ctx, `
			SELECT DISTINCT turn FROM agent_session_transcripts
			WHERE session_id=$1 AND item_state = 0 ORDER BY turn`, sessionID)
		if err != nil {
			return err
		}
		turns, err := pgx.CollectRows(rows, pgx.RowTo[int])
		if err != nil {
			return err
		}
		for _, turn := range turns {
			f, known := facts[turn]
			if known && !f.settled && turn >= currentTurn {
				stats.SkippedOpenTurns++
				continue
			}
			one, err := compactTurnTranscript(ctx, tx, sessionID, turn, f.reason, opts)
			if err != nil {
				return err
			}
			stats.add(one)
		}
		if stats.Turns > 0 {
			stats.Sessions = 1
		}
		if opts.DryRun {
			return errCompactionDryRun
		}
		return nil
	})
	if errors.Is(err, errCompactionDryRun) {
		err = nil
	}
	return stats, err
}

func compactTurnTranscript(ctx context.Context, tx pgx.Tx, sessionID string, turn int, reason string, opts TranscriptCompactionOptions) (TranscriptCompactionStats, error) {
	stats := TranscriptCompactionStats{Turns: 1}
	rows, err := tx.Query(ctx, `
		SELECT seq, part_index, part, item_state, created_at FROM agent_session_transcripts
		WHERE session_id=$1 AND turn=$2 ORDER BY part_index`, sessionID, turn)
	if err != nil {
		return stats, err
	}
	var chunks []agenttranscript.Chunk
	type source struct {
		seq     int64
		created time.Time
	}
	sourceOf := map[int64]source{}
	contiguous, allLegacy := true, true
	for rows.Next() {
		var seq, index int64
		var part string
		var state int16
		var created time.Time
		if err := rows.Scan(&seq, &index, &part, &state, &created); err != nil {
			rows.Close()
			return stats, err
		}
		if index != int64(stats.RowsBefore) {
			contiguous = false
		}
		stats.RowsBefore++
		stats.BytesBefore += int64(len(part))
		sourceOf[index] = source{seq, created}
		if state == transcriptRowLegacyChunk {
			chunks = append(chunks, agenttranscript.Chunk{Index: index, Part: json.RawMessage(part)})
			continue
		}
		allLegacy = false
		for _, c := range agenttranscript.Expand(json.RawMessage(part)) {
			chunks = append(chunks, agenttranscript.Chunk{Index: index, Part: c})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return stats, err
	}
	items := agenttranscript.Coalesce(chunks)
	finished := false
	for _, it := range items {
		stats.RowsAfter++
		stats.BytesAfter += int64(len(it.Part))
		finished = finished || agenttranscript.ChunkType(it.Part) == "finish"
	}
	clearFlag := reason == LegacyPartLimitReason && finished && contiguous && allLegacy
	if clearFlag {
		stats.FlagsCleared = 1
	}
	if opts.Logf != nil {
		opts.Logf("agent transcript compaction: session=%s turn=%d rows %d→%d bytes %d→%d clear_truncation=%v dry_run=%v",
			sessionID, turn, stats.RowsBefore, stats.RowsAfter, stats.BytesBefore, stats.BytesAfter, clearFlag, opts.DryRun)
	}
	if opts.DryRun {
		return stats, nil
	}
	if _, err := tx.Exec(ctx, `DELETE FROM agent_session_transcripts WHERE session_id=$1 AND turn=$2`, sessionID, turn); err != nil {
		return stats, err
	}
	for _, it := range items {
		src, ok := sourceOf[it.Index]
		if !ok {
			return stats, fmt.Errorf("turn %d item %d has no source row", turn, it.Index)
		}
		row := AgentSessionTranscriptPart{Item: true, Partial: it.Partial}
		if _, err := tx.Exec(ctx, `
			INSERT INTO agent_session_transcripts (session_id, seq, turn, part_index, part, item_state, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			sessionID, src.seq, turn, it.Index, string(it.Part), transcriptRowState(row), src.created); err != nil {
			return stats, err
		}
	}
	if clearFlag {
		if _, err := tx.Exec(ctx, `
			UPDATE agent_session_turns
			SET transcript_complete=true, transcript_truncated=false, truncation_reason=''
			WHERE session_id=$1 AND turn=$2 AND truncation_reason=$3`, sessionID, turn, LegacyPartLimitReason); err != nil {
			return stats, err
		}
	}
	return stats, nil
}
