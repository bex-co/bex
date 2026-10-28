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

package agenttranscript_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agenttranscript"
	"github.com/bex-co/bex/lego/backend/internal/agenttranscript/uimessagetest"
)

func chunks(parts ...string) []agenttranscript.Chunk {
	out := make([]agenttranscript.Chunk, len(parts))
	for i, p := range parts {
		out[i] = agenttranscript.Chunk{Index: int64(i), Part: json.RawMessage(p)}
	}
	return out
}

func replay(items []agenttranscript.Item) []json.RawMessage {
	var out []json.RawMessage
	for _, it := range items {
		out = append(out, agenttranscript.Expand(it.Part)...)
	}
	return out
}

func raw(cs []agenttranscript.Chunk) []json.RawMessage {
	out := make([]json.RawMessage, len(cs))
	for i, c := range cs {
		out[i] = c.Part
	}
	return out
}

func mustMerge(t *testing.T, cs []json.RawMessage) []uimessagetest.Part {
	t.Helper()
	parts, err := uimessagetest.Merge(cs)
	if err != nil {
		t.Fatal(err)
	}
	return parts
}

func asJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func indexes(items []agenttranscript.Item) []int64 {
	out := make([]int64, len(items))
	for i, it := range items {
		out[i] = it.Index
	}
	return out
}

// The recorded production turn (5,602 chunks) compacts to a few hundred items
// whose replay rebuilds the same UIMessage parts the raw chunks did.
func TestRecordedTurnCompactsAndReplaysTheSameParts(t *testing.T) {
	fixture, err := uimessagetest.LoadFixture()
	if err != nil {
		t.Fatal(err)
	}
	cs := make([]agenttranscript.Chunk, len(fixture))
	var rawBytes int
	for i, p := range fixture {
		cs[i] = agenttranscript.Chunk{Index: int64(i), Part: p}
		rawBytes += len(p)
	}
	items := agenttranscript.Coalesce(cs)
	var itemBytes, fullBytes, fullCount int
	full := map[string]string{}
	for _, it := range items {
		itemBytes += len(it.Part)
		for _, f := range it.FullOutputs {
			fullBytes += len(f.Text)
			fullCount++
			full[uimessagetest.FullKey(f.ToolCallID, f.Field)] = f.Text
		}
	}
	t.Logf("raw %d rows / %d bytes → %d items / %d bytes inline + %d full outputs / %d bytes out of line",
		len(fixture), rawBytes, len(items), itemBytes, fullCount, fullBytes)
	if fullCount == 0 {
		t.Fatal("the recorded turn has outputs over the inline cap; none were stored out of line")
	}
	if len(items) > 600 {
		t.Fatalf("items = %d, want ≤ 600", len(items))
	}
	if itemBytes*100 > rawBytes*60 {
		t.Fatalf("item bytes %d not a real reduction of %d", itemBytes, rawBytes)
	}
	for i := 1; i < len(items); i++ {
		if items[i].Index <= items[i-1].Index {
			t.Fatalf("items not in ascending index order at %d", i)
		}
	}
	want := mustMerge(t, fixture)
	got := mustMerge(t, replay(items))
	if len(got) != len(want) {
		t.Fatalf("replayed %d parts, raw merge %d", len(got), len(want))
	}
	var texts, reasonings, tools int
	for i := range want {
		w, g := want[i], got[i]
		// A clipped output is flagged, and its full copy restores the raw part.
		clipped := clipMarker.MatchString(asJSON(g["output"]) + asJSON(g["errorText"]))
		flagged, err := uimessagetest.RestoreFullOutput(g, full)
		if err != nil || flagged != clipped {
			t.Fatalf("part %d: clipped=%v flagged=%v err=%v", i, clipped, flagged, err)
		}
		// The recorded turn ends inside a reasoning block: replay closes the
		// partial item, the live stream had left it streaming.
		if w["state"] == "streaming" {
			g["state"] = "streaming"
		}
		if !reflect.DeepEqual(w, g) {
			t.Fatalf("part %d differs:\nraw    %s\nreplay %s", i, asJSON(w), asJSON(g))
		}
		switch w["type"] {
		case "text":
			texts++
		case "reasoning":
			reasonings++
		case "dynamic-tool":
			tools++
		}
	}
	t.Logf("parts: %d text, %d reasoning, %d tool", texts, reasonings, tools)
	if texts != 13 || reasonings != 96 || tools < 290 {
		t.Fatalf("unexpected part census text=%d reasoning=%d tool=%d", texts, reasonings, tools)
	}
	// Re-coalescing the replayed chunks of each item is the identity (the
	// backfill relies on it for rows that are already items).
	var again []agenttranscript.Chunk
	for _, it := range items {
		for _, c := range agenttranscript.Expand(it.Part) {
			again = append(again, agenttranscript.Chunk{Index: it.Index, Part: c})
		}
	}
	recoalesced := agenttranscript.Coalesce(again)
	if len(recoalesced) != len(items) {
		t.Fatalf("re-coalesced %d items, want %d", len(recoalesced), len(items))
	}
	for i, it := range items {
		// Replay closes a partial item, so only its state may change.
		if it.Partial {
			continue
		}
		if string(recoalesced[i].Part) != string(it.Part) || recoalesced[i].Partial {
			t.Fatalf("item %d does not round-trip:\n%s\n%s", i, it.Part, recoalesced[i].Part)
		}
	}
}

func TestInterleavedTextReasoningAndTools(t *testing.T) {
	cs := chunks(
		`{"type":"start"}`,
		`{"type":"reasoning-start","id":"r1"}`,
		`{"type":"reasoning-delta","id":"r1","delta":"think "}`,
		`{"type":"data-acp-info","data":{"x":1},"transient":true}`,
		`{"type":"reasoning-delta","id":"r1","delta":"hard"}`,
		`{"type":"reasoning-end","id":"r1","providerMetadata":{"bex":{"endAt":"t"}}}`,
		`{"type":"tool-input-start","toolCallId":"a","toolName":"Terminal","dynamic":true,"title":"Terminal"}`,
		`{"type":"tool-input-available","toolCallId":"a","toolName":"Terminal","input":{"command":"ls"},"dynamic":true,"title":"ls"}`,
		`{"type":"text-start","id":"t1"}`,
		`{"type":"text-delta","id":"t1","delta":"hello "}`,
		`{"type":"data-acp-usage-update","data":{"used":1},"transient":true}`,
		`{"type":"text-delta","id":"t1","delta":"world"}`,
		`{"type":"text-end","id":"t1"}`,
		`{"type":"tool-output-available","toolCallId":"a","output":"file","dynamic":true,"preliminary":false}`,
		`{"type":"data-acp-usage-update","data":{"used":2},"transient":true}`,
		`{"type":"finish"}`,
	)
	items := agenttranscript.Coalesce(cs)
	if got, want := fmt.Sprint(indexes(items)), "[0 1 6 8 10 15]"; got != want {
		t.Fatalf("item indexes = %s, want %s", got, want)
	}
	if !strings.Contains(string(items[1].Part), `"text":"think hard"`) || !strings.Contains(string(items[3].Part), `"text":"hello world"`) {
		t.Fatalf("text not concatenated: %s / %s", items[1].Part, items[3].Part)
	}
	if !strings.Contains(string(items[2].Part), `"title":"ls"`) || !strings.Contains(string(items[2].Part), `"output":"file"`) {
		t.Fatalf("tool item = %s", items[2].Part)
	}
	if !strings.Contains(string(items[4].Part), `"used":2`) {
		t.Fatalf("usage item kept %s, want the last value", items[4].Part)
	}
	for _, it := range items {
		if it.Partial {
			t.Fatalf("finished turn left a partial item: %+v", it)
		}
		if strings.Contains(string(it.Part), "data-acp-info") {
			t.Fatal("transient data-acp-info was stored")
		}
	}
	want := mustMerge(t, raw(cs))
	got := mustMerge(t, replay(items))
	if asJSON(want) != asJSON(got) {
		t.Fatalf("replay differs:\nraw    %s\nreplay %s", asJSON(want), asJSON(got))
	}
}

// A tool whose output arrives after later items started keeps its position
// (index of its first chunk), and the items after it are released only once it
// is final so writers append in index order.
func TestLateToolOutputKeepsPositionAndReleaseOrder(t *testing.T) {
	c := agenttranscript.NewCoalescer()
	push := func(i int64, p string) []agenttranscript.Item {
		_, released := c.Push(i, json.RawMessage(p))
		return released
	}
	if r := push(0, `{"type":"tool-input-available","toolCallId":"slow","toolName":"Task","input":{},"dynamic":true}`); len(r) != 0 {
		t.Fatalf("open tool released early: %v", r)
	}
	push(1, `{"type":"text-start","id":"t"}`)
	push(2, `{"type":"text-delta","id":"t","delta":"meanwhile"}`)
	if r := push(3, `{"type":"text-end","id":"t"}`); len(r) != 0 {
		t.Fatalf("text released ahead of the open lower-index tool: %v", indexes(r))
	}
	r := push(4, `{"type":"tool-output-available","toolCallId":"slow","output":"done","dynamic":true}`)
	if fmt.Sprint(indexes(r)) != "[0 1]" {
		t.Fatalf("released = %v, want [0 1]", indexes(r))
	}
	// A late update for the finished call is absorbed, not stored again.
	owner, r := c.Push(5, json.RawMessage(`{"type":"tool-output-available","toolCallId":"slow","output":"again","dynamic":true}`))
	if owner != 0 || len(r) != 0 || len(c.Flush()) != 0 {
		t.Fatalf("late output owner=%d released=%v", owner, r)
	}
}

func TestPartialItemsAtDisconnect(t *testing.T) {
	c := agenttranscript.NewCoalescer()
	for i, p := range []string{
		`{"type":"text-start","id":"t"}`,
		`{"type":"text-delta","id":"t","delta":"half"}`,
		`{"type":"tool-input-start","toolCallId":"x","toolName":"Read","dynamic":true}`,
	} {
		c.Push(int64(i), json.RawMessage(p))
	}
	items := c.Flush()
	if len(items) != 2 || !items[0].Partial || !items[1].Partial {
		t.Fatalf("flush = %+v, want two partial items", items)
	}
	if !strings.Contains(string(items[0].Part), `"state":"streaming"`) {
		t.Fatalf("partial text = %s", items[0].Part)
	}
	got := mustMerge(t, replay(items))
	if len(got) != 2 || got[0]["text"] != "half" || got[1]["toolCallId"] != "x" {
		t.Fatalf("partial replay = %s", asJSON(got))
	}
}

func TestSnapshotsKeepTheLatestPerPlanAndPerToolDiff(t *testing.T) {
	cs := chunks(
		`{"type":"data-acp-plan","id":"acp-plan","data":{"entries":[{"content":"a","status":"pending"}]}}`,
		`{"type":"tool-input-start","toolCallId":"e","toolName":"Edit","dynamic":true}`,
		`{"type":"data-acp-diff","data":{"path":"f.go","oldText":"1","newText":"2","toolCallId":"e"}}`,
		`{"type":"data-acp-diff","data":{"path":"f.go","oldText":"1","newText":"3","toolCallId":"e"}}`,
		`{"type":"tool-output-available","toolCallId":"e","output":{},"dynamic":true}`,
		`{"type":"data-acp-plan","id":"acp-plan","data":{"entries":[{"content":"a","status":"completed"}]}}`,
		`{"type":"data-acp-terminal","data":{"terminalId":"t1","output":"x","toolCallId":"gone"}}`,
		`{"type":"finish"}`,
	)
	items := agenttranscript.Coalesce(cs)
	if fmt.Sprint(indexes(items)) != "[0 1 2 6 7]" {
		t.Fatalf("indexes = %v", indexes(items))
	}
	if !strings.Contains(string(items[0].Part), `"completed"`) || !strings.Contains(string(items[2].Part), `"newText":"3"`) {
		t.Fatalf("snapshots not latest: %s / %s", items[0].Part, items[2].Part)
	}
	got := mustMerge(t, replay(items))
	want := mustMerge(t, raw(cs))
	// The live stream appended both diffs; the durable item keeps the latest.
	if len(want) != 5 || len(got) != 4 || asJSON(got[0]) != asJSON(want[0]) || asJSON(got[2]) != asJSON(want[3]) {
		t.Fatalf("replay = %s\nraw = %s", asJSON(got), asJSON(want))
	}
}

var clipMarker = regexp.MustCompile(`\[bex: (\d+) chars clipped\]`)

func TestToolOutputIsClippedHeadAndTail(t *testing.T) {
	big := strings.Repeat("a", 100_000) + "MIDDLE" + strings.Repeat("z", 100_000)
	output, _ := json.Marshal(big)
	structured, _ := json.Marshal(map[string]any{"stdout": big, "exitCode": 0})
	for name, out := range map[string]string{"string": string(output), "structured": string(structured)} {
		t.Run(name, func(t *testing.T) {
			items := agenttranscript.Coalesce(chunks(
				`{"type":"tool-input-available","toolCallId":"b","toolName":"Bash","input":{},"dynamic":true}`,
				`{"type":"tool-output-available","toolCallId":"b","output":`+out+`,"dynamic":true}`,
			))
			if len(items) != 1 {
				t.Fatalf("items = %d", len(items))
			}
			part := string(items[0].Part)
			m := clipMarker.FindStringSubmatch(part)
			if m == nil || m[1] != fmt.Sprint(len(big)-8_000) {
				t.Fatalf("marker = %v", m)
			}
			if strings.Contains(part, "MIDDLE") || !strings.Contains(part, `"truncated":true`) ||
				!strings.Contains(part, `"fullOutput":true`) {
				t.Fatal("middle kept or truncated/fullOutput flag missing")
			}
			fulls := items[0].FullOutputs
			if len(fulls) != 1 || fulls[0].ToolCallID != "b" || fulls[0].Field != agenttranscript.FieldOutput || fulls[0].Text != big {
				t.Fatalf("full outputs = %d, want the complete original string", len(fulls))
			}
			if len(part) > 9_000+200 {
				t.Fatalf("clipped item is %d bytes", len(part))
			}
			got := mustMerge(t, replay(items))
			text := asJSON(got[0]["output"])
			if !strings.Contains(text, strings.Repeat("a", 4_000)) || !strings.Contains(text, strings.Repeat("z", 4_000)) {
				t.Fatal("head or tail lost")
			}
			if !strings.Contains(asJSON(got[0]["resultProviderMetadata"]), `"bex-transcript":{"fullOutput":true}`) {
				t.Fatalf("replay does not flag the full copy: %v", got[0]["resultProviderMetadata"])
			}
		})
	}
	if s, clipped := agenttranscript.ClipText(strings.Repeat("é", 8_000)); clipped || s != strings.Repeat("é", 8_000) {
		t.Fatal("text at the cap must not be clipped")
	}
	s, clipped := agenttranscript.ClipText(strings.Repeat("é", 8_001))
	if !clipped || !strings.Contains(s, "\n\n[bex: 1 chars clipped]\n\n") || !strings.HasPrefix(s, strings.Repeat("é", 4_000)+"\n") {
		t.Fatalf("rune-aware clip wrong: %q…", s[:40])
	}
	if again, clipped := agenttranscript.ClipText(s); clipped || again != s {
		t.Fatal("already-clipped text was clipped again")
	}
}

func TestOrphansAndUnparseableChunks(t *testing.T) {
	c := agenttranscript.NewCoalescer()
	if owner, _ := c.Push(0, json.RawMessage(`{"type":"text-delta","id":"nope","delta":"x"}`)); owner != -1 {
		t.Fatalf("orphan delta owner = %d", owner)
	}
	if owner, _ := c.Push(1, json.RawMessage(`{"type":"tool-output-available","toolCallId":"nope","output":1}`)); owner != -1 {
		t.Fatalf("orphan output owner = %d", owner)
	}
	owner, released := c.Push(2, json.RawMessage(`not json`))
	if owner != 2 || len(released) != 1 || string(released[0].Part) != "not json" {
		t.Fatalf("unparseable chunk = %d %+v", owner, released)
	}
	// A tool chunk (no state) is not mistaken for a stored tool item.
	chunk := json.RawMessage(`{"type":"tool-output-available","toolCallId":"x"}`)
	if got := agenttranscript.Expand(chunk); len(got) != 1 || string(got[0]) != string(chunk) {
		t.Fatalf("chunk expanded as an item: %s", got)
	}
}

// A preliminary output keeps its call open; a later error replaces the output,
// is clipped like output (with the truncated flag), and replays as
// output-error. The turn's finish closes a still-running call as final.
func TestToolErrorAfterPreliminaryOutputAndFinish(t *testing.T) {
	errText, _ := json.Marshal(strings.Repeat("e", 40_000))
	cs := chunks(
		`{"type":"tool-input-available","toolCallId":"a","toolName":"Bash","input":{"command":"make"},"dynamic":true}`,
		`{"type":"tool-output-available","toolCallId":"a","output":"building…","dynamic":true,"preliminary":true}`,
		`{"type":"tool-input-available","toolCallId":"b","toolName":"Read","input":{"path":"x"},"dynamic":true}`,
		`{"type":"tool-output-error","toolCallId":"a","errorText":`+string(errText)+`,"dynamic":true}`,
		`{"type":"finish"}`,
	)
	c := agenttranscript.NewCoalescer()
	c.Push(cs[0].Index, cs[0].Part)
	if _, released := c.Push(cs[1].Index, cs[1].Part); len(released) != 0 {
		t.Fatalf("a preliminary output completed the call: %v", indexes(released))
	}
	items := agenttranscript.Coalesce(cs)
	if fmt.Sprint(indexes(items)) != "[0 2 4]" {
		t.Fatalf("indexes = %v", indexes(items))
	}
	for _, it := range items {
		if it.Partial {
			t.Fatalf("finish left item %d partial", it.Index)
		}
	}
	failed := string(items[0].Part)
	if !strings.Contains(failed, `"state":"output-error"`) || !strings.Contains(failed, `"truncated":true`) ||
		!strings.Contains(failed, "[bex: 32000 chars clipped]") || strings.Contains(failed, `"output"`) ||
		!strings.Contains(failed, `"fullOutput":true`) {
		t.Fatalf("failed call item = %.300s", failed)
	}
	if f := items[0].FullOutputs; len(f) != 1 || f[0].Field != agenttranscript.FieldErrorText || len(f[0].Text) != 40_000 {
		t.Fatalf("errorText full outputs = %+v", len(f))
	}
	got := mustMerge(t, replay(items))
	if got[0]["state"] != "output-error" || got[1]["state"] != "input-available" {
		t.Fatalf("replayed states = %v / %v", got[0]["state"], got[1]["state"])
	}
}

