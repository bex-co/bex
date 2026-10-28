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

// Package agenttranscript turns an agent turn's Vercel AI SDK v6 UI-message
// chunk stream into the durable transcript's completed items (ADR051 § Item
// model, w10/m16) and expands a stored item back into the minimal standard
// chunk sequence for replay.
//
// One Coalescer serves both writers — the gateway's live tee and the
// Completer's driver-log harvest — so they produce byte-identical items for the
// same turn. An item's index is the turn-local ordinal of its FIRST chunk: it
// does not depend on where a reader started counting, so the
// (session, turn, part_index) idempotent merge keeps working when the tee and
// the harvest each see the turn.
//
// Items, by chunk family:
//
//   - text-* → one UIMessage `text` part; reasoning-* → one `reasoning` part.
//   - tool-input-* / tool-output-* for one toolCallId → one `dynamic-tool` (or
//     `tool-<name>`) part with input, output or error, title and metadata.
//   - data-acp-diff / data-acp-terminal → latest snapshot per tool and
//     path/terminal, completed with its tool.
//   - non-transient data-* with an id (data-acp-plan) → latest snapshot,
//     completed at the turn's finish.
//   - data-acp-usage-update → only the last value of the turn.
//   - every other transient chunk (data-acp-info, available-commands,
//     session-info-update, …) → never stored.
//   - anything else (start, finish, error, data-bex-*, …) → stored as-is.
//
// Tool output text above MaxToolOutputChars is clipped head+tail with the
// `[bex: N chars clipped]` marker the dashboard recognizes.
package agenttranscript

import (
	"encoding/json"
	"sort"
	"strings"
)

// Chunk is one UI-message chunk with its turn-local emission ordinal.
type Chunk struct {
	Index int64
	Part  json.RawMessage
}

// Item is one completed (or, at stream end, partial) transcript item. Index is
// the ordinal of its first chunk. Partial marks an item the stream ended inside
// of; a later, complete copy of the same item may replace it.
type Item struct {
	Index   int64
	Part    json.RawMessage
	Partial bool
}

// usageUpdateType is the one transient chunk type the transcript keeps (its
// final value per turn).
const usageUpdateType = "data-acp-usage-update"

type itemKind int

const (
	kindStandalone itemKind = iota
	kindText
	kindTool
	kindSnapshot
)

type pending struct {
	index int64
	kind  itemKind
	done  bool
	size  int64

	// text / reasoning
	partType string
	id       string
	text     strings.Builder
	pm       json.RawMessage

	// tool
	tool toolItem

	// standalone / snapshot: the chunk to store (latest snapshot wins)
	chunk json.RawMessage
	// snapshot attached to a tool (diff/terminal): completes with that tool
	toolCallID string
	key        string
}

type toolItem struct {
	ToolCallID       string          `json:"toolCallId"`
	ToolName         string          `json:"toolName"`
	Title            *string         `json:"title,omitempty"`
	State            string          `json:"state"`
	Input            json.RawMessage `json:"input,omitempty"`
	Output           json.RawMessage `json:"output,omitempty"`
	ErrorText        *string         `json:"errorText,omitempty"`
	Preliminary      *bool           `json:"preliminary,omitempty"`
	ProviderExecuted *bool           `json:"providerExecuted,omitempty"`
	ToolMetadata     json.RawMessage `json:"toolMetadata,omitempty"`
	CallPM           json.RawMessage `json:"callProviderMetadata,omitempty"`
	ResultPM         json.RawMessage `json:"resultProviderMetadata,omitempty"`
	Truncated        bool            `json:"truncated,omitempty"`

	dynamic   bool
	inputText strings.Builder
}

// chunkFields is the union of the chunk fields the coalescer reads.
type chunkFields struct {
	Type             string          `json:"type"`
	ID               *string         `json:"id"`
	Delta            string          `json:"delta"`
	ProviderMetadata json.RawMessage `json:"providerMetadata"`
	ToolCallID       string          `json:"toolCallId"`
	ToolName         string          `json:"toolName"`
	Title            *string         `json:"title"`
	Input            json.RawMessage `json:"input"`
	InputTextDelta   string          `json:"inputTextDelta"`
	Dynamic          *bool           `json:"dynamic"`
	ProviderExecuted *bool           `json:"providerExecuted"`
	ToolMetadata     json.RawMessage `json:"toolMetadata"`
	Output           json.RawMessage `json:"output"`
	ErrorText        *string         `json:"errorText"`
	Preliminary      *bool           `json:"preliminary"`
	Transient        bool            `json:"transient"`
	Data             json.RawMessage `json:"data"`
}

// Coalescer folds one turn's chunk stream into items. It is not safe for
// concurrent use; each stream owns one.
type Coalescer struct {
	order      []*pending // unreleased items, ascending index
	texts      map[string]*pending
	reasonings map[string]*pending
	tools      map[string]*pending // every tool seen this turn (late chunks are absorbed)
	snapshots  map[string]*pending // open keyed snapshots
	usage      *pending
	bytes      int64
}

// NewCoalescer returns an empty per-turn coalescer.
func NewCoalescer() *Coalescer {
	return &Coalescer{
		texts:      map[string]*pending{},
		reasonings: map[string]*pending{},
		tools:      map[string]*pending{},
		snapshots:  map[string]*pending{},
	}
}

// PendingBytes approximates the bytes held by unreleased items.
func (c *Coalescer) PendingBytes() int64 { return c.bytes }

// Push folds one chunk in. owner is the index of the item the chunk belongs to
// (-1 for a chunk that is never stored). released lists items now final, in
// ascending index order: an item completed after a later-started one is held
// until every lower-indexed item is final too, so writers append items in index
// order and the store's seq order stays the UI's part order.
func (c *Coalescer) Push(index int64, part json.RawMessage) (owner int64, released []Item) {
	owner = c.push(index, part)
	return owner, c.release()
}

// Flush releases every remaining item, marking unfinished ones Partial. Call it
// once at stream end.
func (c *Coalescer) Flush() []Item {
	out := make([]Item, 0, len(c.order))
	for _, p := range c.order {
		out = append(out, p.item(!p.done))
	}
	c.order = nil
	c.bytes = 0
	c.texts = map[string]*pending{}
	c.reasonings = map[string]*pending{}
	c.snapshots = map[string]*pending{}
	c.usage = nil
	return out
}

// Coalesce folds a whole chunk sequence (e.g. a parsed driver log) into its
// items. Chunks are processed in ascending index order.
func Coalesce(chunks []Chunk) []Item {
	sorted := append([]Chunk(nil), chunks...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Index < sorted[j].Index })
	c := NewCoalescer()
	var out []Item
	for _, ch := range sorted {
		_, released := c.Push(ch.Index, ch.Part)
		out = append(out, released...)
	}
	return append(out, c.Flush()...)
}

func (c *Coalescer) release() []Item {
	n := 0
	for n < len(c.order) && c.order[n].done {
		n++
	}
	if n == 0 {
		return nil
	}
	out := make([]Item, n)
	for i, p := range c.order[:n] {
		out[i] = p.item(false)
		c.bytes -= p.size
	}
	c.order = append(c.order[:0], c.order[n:]...)
	return out
}

func (c *Coalescer) open(p *pending) *pending {
	c.order = append(c.order, p)
	c.resize(p)
	return p
}

func (c *Coalescer) resize(p *pending) {
	var size int64
	switch p.kind {
	case kindText:
		size = int64(p.text.Len() + len(p.pm))
	case kindTool:
		size = int64(len(p.tool.Input) + len(p.tool.Output) + p.tool.inputText.Len())
		if p.tool.ErrorText != nil {
			size += int64(len(*p.tool.ErrorText))
		}
	default:
		size = int64(len(p.chunk))
	}
	c.bytes += size - p.size
	p.size = size
}

func (c *Coalescer) standalone(index int64, part json.RawMessage) int64 {
	c.open(&pending{index: index, kind: kindStandalone, done: true, chunk: part})
	return index
}

func (c *Coalescer) push(index int64, part json.RawMessage) int64 {
	var ch chunkFields
	if json.Unmarshal(part, &ch) != nil || ch.Type == "" {
		return c.standalone(index, part)
	}
	switch ch.Type {
	case "text-start", "reasoning-start":
		return c.textStart(index, ch)
	case "text-delta", "reasoning-delta":
		return c.textDelta(ch)
	case "text-end", "reasoning-end":
		return c.textEnd(ch)
	case "tool-input-start", "tool-input-delta", "tool-input-available", "tool-input-error",
		"tool-output-available", "tool-output-error", "tool-output-denied":
		return c.toolChunk(index, ch)
	case "finish":
		c.finishTurn()
		return c.standalone(index, part)
	}
	if strings.HasPrefix(ch.Type, "data-") {
		return c.dataChunk(index, part, ch)
	}
	return c.standalone(index, part)
}

func (c *Coalescer) textMap(chunkType string) (map[string]*pending, string) {
	if strings.HasPrefix(chunkType, "reasoning-") {
		return c.reasonings, "reasoning"
	}
	return c.texts, "text"
}

func (c *Coalescer) textStart(index int64, ch chunkFields) int64 {
	active, partType := c.textMap(ch.Type)
	id := deref(ch.ID)
	if prev := active[id]; prev != nil {
		// A repeated start opens a new part in the AI SDK; the old one is over.
		prev.done = true
	}
	p := c.open(&pending{index: index, kind: kindText, partType: partType, id: id, pm: ch.ProviderMetadata})
	active[id] = p
	return index
}

func (c *Coalescer) textDelta(ch chunkFields) int64 {
	active, _ := c.textMap(ch.Type)
	p := active[deref(ch.ID)]
	if p == nil {
		return -1 // the AI SDK rejects a delta without its start; so does replay
	}
	p.text.WriteString(ch.Delta)
	if pm := nonNull(ch.ProviderMetadata); pm != nil {
		p.pm = pm
	}
	c.resize(p)
	return p.index
}

func (c *Coalescer) textEnd(ch chunkFields) int64 {
	active, _ := c.textMap(ch.Type)
	id := deref(ch.ID)
	p := active[id]
	if p == nil {
		return -1
	}
	if pm := nonNull(ch.ProviderMetadata); pm != nil {
		p.pm = pm
	}
	p.done = true
	delete(active, id)
	c.resize(p)
	return p.index
}

func (c *Coalescer) toolChunk(index int64, ch chunkFields) int64 {
	p := c.tools[ch.ToolCallID]
	if p != nil && p.done {
		return p.index // a late update for a finished call is not replayed
	}
	opens := ch.Type == "tool-input-start" || ch.Type == "tool-input-available" || ch.Type == "tool-input-error"
	if p == nil {
		if !opens {
			return -1 // output for an unknown call: the AI SDK rejects it too
		}
		p = c.open(&pending{index: index, kind: kindTool, tool: toolItem{ToolCallID: ch.ToolCallID}})
		c.tools[ch.ToolCallID] = p
	}
	t := &p.tool
	hasPM := nonNull(ch.ProviderMetadata) != nil
	switch ch.Type {
	case "tool-input-start":
		t.ToolName = ch.ToolName
		t.State = "input-streaming"
		t.Input = nil
		t.dynamic = ch.Dynamic != nil && *ch.Dynamic
		t.applyCommon(ch)
		if hasPM {
			t.CallPM = ch.ProviderMetadata
		}
	case "tool-input-delta":
		t.inputText.WriteString(ch.InputTextDelta)
	case "tool-input-available":
		t.ToolName = ch.ToolName
		t.State = "input-available"
		t.Input = ch.Input
		t.inputText.Reset()
		if ch.Dynamic != nil {
			t.dynamic = *ch.Dynamic
		}
		t.applyCommon(ch)
		if hasPM {
			t.CallPM = ch.ProviderMetadata
		}
	case "tool-input-error":
		if ch.ToolName != "" {
			t.ToolName = ch.ToolName
		}
		if ch.Dynamic != nil {
			t.dynamic = *ch.Dynamic
		}
		t.Input = ch.Input
		t.setError(ch.ErrorText)
		t.applyCommon(ch)
		if hasPM {
			t.ResultPM = ch.ProviderMetadata
		}
		c.completeTool(p)
	case "tool-output-available":
		output, clipped := clipJSON(ch.Output)
		t.Output = output
		t.Truncated = t.Truncated || clipped
		t.ErrorText = nil
		t.State = "output-available"
		t.Preliminary = ch.Preliminary
		if ch.ProviderExecuted != nil {
			t.ProviderExecuted = ch.ProviderExecuted
		}
		if hasPM {
			t.ResultPM = ch.ProviderMetadata
		}
		if ch.Preliminary == nil || !*ch.Preliminary {
			c.completeTool(p)
		}
	case "tool-output-error":
		t.setError(ch.ErrorText)
		t.Output = nil
		t.Preliminary = nil
		if ch.ProviderExecuted != nil {
			t.ProviderExecuted = ch.ProviderExecuted
		}
		if hasPM {
			t.ResultPM = ch.ProviderMetadata
		}
		c.completeTool(p)
	case "tool-output-denied":
		t.State = "output-denied"
		c.completeTool(p)
	}
	c.resize(p)
	return p.index
}

func (t *toolItem) applyCommon(ch chunkFields) {
	if ch.Title != nil {
		t.Title = ch.Title
	}
	if ch.ProviderExecuted != nil {
		t.ProviderExecuted = ch.ProviderExecuted
	}
	if len(ch.ToolMetadata) > 0 {
		t.ToolMetadata = ch.ToolMetadata
	}
}

func (t *toolItem) setError(text *string) {
	t.State = "output-error"
	value := ""
	if text != nil {
		value = *text
	}
	clipped, ok := ClipText(value)
	t.ErrorText = &clipped
	t.Truncated = t.Truncated || ok
}

// completeTool finishes a tool and every diff/terminal snapshot attached to it.
func (c *Coalescer) completeTool(p *pending) {
	p.done = true
	for key, snap := range c.snapshots {
		if snap.toolCallID != "" && snap.toolCallID == p.tool.ToolCallID {
			snap.done = true
			delete(c.snapshots, key)
		}
	}
}

// finishTurn closes every open item: nothing after the turn's finish can
// extend them, so they are final (not partial) as they stand.
func (c *Coalescer) finishTurn() {
	for _, p := range c.order {
		p.done = true
	}
	c.texts = map[string]*pending{}
	c.reasonings = map[string]*pending{}
	c.snapshots = map[string]*pending{}
	c.usage = nil
}

func (c *Coalescer) dataChunk(index int64, part json.RawMessage, ch chunkFields) int64 {
	if ch.Transient {
		if ch.Type != usageUpdateType {
			return -1
		}
		if c.usage == nil {
			c.usage = c.open(&pending{index: index, kind: kindSnapshot, chunk: part})
		} else {
			c.usage.chunk = part
			c.resize(c.usage)
		}
		return c.usage.index
	}
	var data struct {
		ToolCallID string `json:"toolCallId"`
		Path       string `json:"path"`
		TerminalID string `json:"terminalId"`
	}
	_ = json.Unmarshal(ch.Data, &data)
	var key, toolCallID string
	switch {
	case (ch.Type == "data-acp-diff" || ch.Type == "data-acp-terminal") && data.ToolCallID != "":
		toolCallID = data.ToolCallID
		key = ch.Type + "\x00" + data.ToolCallID + "\x00" + data.Path + data.TerminalID
		if ch.Type == "data-acp-terminal" {
			part = clipTerminal(part)
		}
	case ch.ID != nil:
		key = ch.Type + "\x00id\x00" + *ch.ID
	default:
		return c.standalone(index, part)
	}
	if snap := c.snapshots[key]; snap != nil {
		snap.chunk = part
		c.resize(snap)
		return snap.index
	}
	snap := c.open(&pending{index: index, kind: kindSnapshot, chunk: part, toolCallID: toolCallID, key: key})
	if toolCallID != "" {
		if tool := c.tools[toolCallID]; tool == nil || tool.done {
			snap.done = true // no open call to wait for
			return index
		}
	}
	c.snapshots[key] = snap
	return index
}

// clipTerminal clips the terminal output strings of a data-acp-terminal chunk.
func clipTerminal(part json.RawMessage) json.RawMessage {
	var chunk map[string]json.RawMessage
	if json.Unmarshal(part, &chunk) != nil {
		return part
	}
	data, clipped := clipJSON(chunk["data"])
	if !clipped {
		return part
	}
	chunk["data"] = data
	out, err := marshalNoEscape(chunk)
	if err != nil {
		return part
	}
	return out
}

func (p *pending) item(partial bool) Item {
	return Item{Index: p.index, Part: p.render(), Partial: partial}
}

func (p *pending) render() json.RawMessage {
	switch p.kind {
	case kindText:
		state := "streaming"
		if p.done {
			state = "done"
		}
		out, _ := marshalNoEscape(struct {
			Type             string          `json:"type"`
			ID               string          `json:"id"`
			Text             string          `json:"text"`
			State            string          `json:"state"`
			ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
		}{p.partType, p.id, p.text.String(), state, nonNull(p.pm)})
		return out
	case kindTool:
		t := p.tool
		if len(t.Input) == 0 && t.inputText.Len() > 0 && json.Valid([]byte(t.inputText.String())) {
			t.Input = json.RawMessage(t.inputText.String())
		}
		partType := "dynamic-tool"
		if !t.dynamic {
			partType = "tool-" + t.ToolName
		}
		if t.State == "" {
			t.State = "input-streaming"
		}
		out, _ := marshalNoEscape(struct {
			Type string `json:"type"`
			toolItem
		}{partType, t})
		return out
	default:
		return p.chunk
	}
}

func nonNull(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return raw
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
