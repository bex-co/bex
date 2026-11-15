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

package agentsessions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agenttranscript"
	"github.com/bex-co/bex/lego/backend/internal/agenttranscript/uimessagetest"
)

// driverLogOf builds a driver session log of about size bytes for turn 1 from
// a real turn's chunks, repeated with fresh text and tool ids so each repeat
// folds into its own items, as a long session's would.
func driverLogOf(t testing.TB, size int) []byte {
	t.Helper()
	parts, err := uimessagetest.LoadFixture()
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	log.Grow(size)
	index := 0
	for rep := 0; ; rep++ {
		for _, part := range parts {
			var chunk map[string]any
			if err := json.Unmarshal(part, &chunk); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"id", "toolCallId"} {
				if s, ok := chunk[key].(string); ok {
					chunk[key] = fmt.Sprintf("%s-%d", s, rep)
				}
			}
			body, err := json.Marshal(chunk)
			if err != nil {
				t.Fatal(err)
			}
			line := fmt.Sprintf(`{"at":"2026-11-15T05:13:00.000Z","type":"ui-message","turn":1,"partIndex":%d,"part":%s}`+"\n", index, body)
			if log.Len()+len(line) > size {
				return log.Bytes()
			}
			log.WriteString(line)
			index++
		}
	}
}

// liveHeap is the heap still reachable right now.
func liveHeap() int64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.HeapAlloc)
}

// A harvest of a full 16 MiB driver log holds less than the log's own size at
// any point (w4/677): the fold keeps the items and the line being read, never
// the log or its chunks. Before, a harvest held the log as one string, a copy
// of every line, every chunk and then the items, about 5x the log, on every
// replica once a minute per running session.
func TestTranscriptFoldHoldsLessThanTheLog(t *testing.T) {
	log := driverLogOf(t, driverSessionLogMaxBytes)
	base := liveHeap()
	peak := int64(0)
	fold := newTranscriptFold(1)
	lines := 0
	for line := range bytes.SplitSeq(log, []byte("\n")) {
		fold.line(line)
		if lines++; lines%500 == 0 {
			peak = max(peak, liveHeap()-base)
		}
	}
	items := fold.finish()
	peak = max(peak, liveHeap()-base)
	if len(items) < 1000 || fold.parseTruncated {
		t.Fatalf("folded %d items (parse truncated %t) from %d chunks, want the whole log", len(items), fold.parseTruncated, fold.chunks)
	}
	if peak > int64(len(log)) {
		t.Fatalf("harvest of a %d-byte log held %d bytes at its peak, want under the log's size", len(log), peak)
	}
	runtime.KeepAlive(items)
	runtime.KeepAlive(log)
}

// The fold is the coalescer over the log's chunks: the same items as
// agenttranscript.Coalesce and the turn's index range. A part the log repeats
// is folded once, and the turn reads as driver-truncated.
func TestTranscriptFoldMatchesCoalesce(t *testing.T) {
	log := driverLogOf(t, 1<<20)
	fold := newTranscriptFold(1)
	var chunks []agenttranscript.Chunk
	for line := range bytes.SplitSeq(log, []byte("\n")) {
		fold.line(line)
		if len(line) > 0 {
			var rec transcriptRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				t.Fatal(err)
			}
			chunks = append(chunks, agenttranscript.Chunk{Index: *rec.PartIndex, Part: rec.Part})
		}
	}
	if fold.driverTruncated(int64(len(log))) {
		t.Fatal("a gapless log under the cap reads as driver-truncated")
	}
	fold.line(bytes.SplitN(log, []byte("\n"), 2)[0]) // part 0 again
	got, want := fold.finish(), agenttranscript.Coalesce(chunks)
	if len(got) != len(want) || fold.chunks != len(chunks) || fold.first != 0 || fold.last != int64(len(chunks)-1) {
		t.Fatalf("fold = %d items over %d chunks [%d, %d], want %d items over %d chunks", len(got), fold.chunks, fold.first, fold.last, len(want), len(chunks))
	}
	for i := range got {
		if got[i].Index != want[i].Index || !bytes.Equal(got[i].Part, want[i].Part) || got[i].Partial != want[i].Partial {
			t.Fatalf("item %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if !fold.driverTruncated(int64(len(log))) {
		t.Fatal("a log that goes back reads as one write of the turn")
	}
}
