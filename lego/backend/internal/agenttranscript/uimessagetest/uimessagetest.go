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

// Package uimessagetest is a test-only model of the Vercel AI SDK v6
// processUIMessageStream merge (ai@6 dist, `processUIMessageStream`): it folds
// UI-message chunks into the assistant message's parts the way useChat does, so
// tests can prove a compacted transcript replays to the same rendered content
// as the raw chunks. It also loads the recorded production turn fixture.
package uimessagetest

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Part is one assembled UIMessage part.
type Part = map[string]any

// Merge folds chunks into UIMessage parts. Like the AI SDK it fails on a
// text/reasoning delta or end without its start and on tool output for an
// unknown call.
func Merge(chunks []json.RawMessage) ([]Part, error) {
	m := &merger{activeText: map[string]Part{}, activeReasoning: map[string]Part{}, partialTools: map[string]Part{}}
	for i, raw := range chunks {
		var chunk map[string]any
		if err := json.Unmarshal(raw, &chunk); err != nil {
			return nil, fmt.Errorf("chunk %d: %w", i, err)
		}
		if err := m.apply(chunk); err != nil {
			return nil, fmt.Errorf("chunk %d (%v): %w", i, chunk["type"], err)
		}
	}
	return m.parts, nil
}

type merger struct {
	parts           []Part
	activeText      map[string]Part
	activeReasoning map[string]Part
	partialTools    map[string]Part
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func (m *merger) findTool(id string) Part {
	for i := len(m.parts) - 1; i >= 0; i-- {
		p := m.parts[i]
		t := str(p["type"])
		if (t == "dynamic-tool" || strings.HasPrefix(t, "tool-")) && str(p["toolCallId"]) == id {
			return p
		}
	}
	return nil
}

func setOpt(p Part, key string, v any) {
	if v == nil {
		delete(p, key)
		return
	}
	p[key] = v
}

// updateTool mirrors updateDynamicToolPart / updateToolPart.
func (m *merger) updateTool(chunk map[string]any, state string, input, output, errorText, preliminary any, existing Part) {
	dynamic, _ := chunk["dynamic"].(bool)
	id := str(chunk["toolCallId"])
	p := existing
	if p == nil {
		p = m.findTool(id)
	}
	pm := chunk["providerMetadata"]
	result := state == "output-available" || state == "output-error"
	if p == nil {
		p = Part{"toolCallId": id, "state": state}
		if dynamic {
			p["type"] = "dynamic-tool"
			p["toolName"] = chunk["toolName"]
		} else {
			p["type"] = "tool-" + str(chunk["toolName"])
		}
		m.parts = append(m.parts, p)
	} else if str(p["type"]) == "dynamic-tool" {
		if name, ok := chunk["toolName"]; ok {
			p["toolName"] = name
		}
	}
	p["state"] = state
	setOpt(p, "input", input)
	setOpt(p, "output", output)
	setOpt(p, "errorText", errorText)
	setOpt(p, "preliminary", preliminary)
	if title, ok := chunk["title"]; ok {
		p["title"] = title
	}
	if v, ok := chunk["providerExecuted"]; ok {
		p["providerExecuted"] = v
	}
	if pm != nil {
		if result {
			p["resultProviderMetadata"] = pm
		} else {
			p["callProviderMetadata"] = pm
		}
	}
}

func (m *merger) apply(chunk map[string]any) error {
	t := str(chunk["type"])
	id := str(chunk["id"])
	switch t {
	case "text-start", "reasoning-start":
		kind := strings.TrimSuffix(t, "-start")
		p := Part{"type": kind, "text": "", "state": "streaming"}
		setOpt(p, "providerMetadata", chunk["providerMetadata"])
		if kind == "text" {
			m.activeText[id] = p
		} else {
			m.activeReasoning[id] = p
		}
		m.parts = append(m.parts, p)
	case "text-delta", "reasoning-delta", "text-end", "reasoning-end":
		active := m.activeText
		if strings.HasPrefix(t, "reasoning-") {
			active = m.activeReasoning
		}
		p := active[id]
		if p == nil {
			return fmt.Errorf("missing part %q", id)
		}
		if strings.HasSuffix(t, "-delta") {
			p["text"] = str(p["text"]) + str(chunk["delta"])
		} else {
			p["state"] = "done"
			delete(active, id)
		}
		if pm := chunk["providerMetadata"]; pm != nil {
			p["providerMetadata"] = pm
		}
	case "tool-input-start":
		m.partialTools[str(chunk["toolCallId"])] = Part{"text": "", "chunk": chunk}
		m.updateTool(chunk, "input-streaming", nil, nil, nil, nil, nil)
	case "tool-input-delta":
		pt := m.partialTools[str(chunk["toolCallId"])]
		if pt == nil {
			return fmt.Errorf("tool-input-delta for unknown call")
		}
		pt["text"] = str(pt["text"]) + str(chunk["inputTextDelta"])
		var partial any
		_ = json.Unmarshal([]byte(str(pt["text"])), &partial)
		start := pt["chunk"].(map[string]any)
		m.updateTool(map[string]any{"toolCallId": start["toolCallId"], "toolName": start["toolName"], "dynamic": start["dynamic"], "title": start["title"]},
			"input-streaming", partial, nil, nil, nil, nil)
	case "tool-input-available":
		m.updateTool(chunk, "input-available", chunk["input"], nil, nil, nil, nil)
	case "tool-input-error":
		m.updateTool(chunk, "output-error", chunk["input"], nil, chunk["errorText"], nil, nil)
	case "tool-output-available", "tool-output-error":
		p := m.findTool(str(chunk["toolCallId"]))
		if p == nil {
			return fmt.Errorf("no tool invocation %q", chunk["toolCallId"])
		}
		c := map[string]any{"toolCallId": chunk["toolCallId"], "providerMetadata": chunk["providerMetadata"], "title": p["title"]}
		if v, ok := chunk["providerExecuted"]; ok {
			c["providerExecuted"] = v
		}
		if t == "tool-output-available" {
			m.updateTool(c, "output-available", p["input"], chunk["output"], nil, chunk["preliminary"], p)
		} else {
			m.updateTool(c, "output-error", p["input"], nil, chunk["errorText"], nil, p)
		}
	case "tool-output-denied":
		p := m.findTool(str(chunk["toolCallId"]))
		if p == nil {
			return fmt.Errorf("no tool invocation %q", chunk["toolCallId"])
		}
		p["state"] = "output-denied"
	case "start-step":
		m.parts = append(m.parts, Part{"type": "step-start"})
	case "finish-step":
		m.activeText, m.activeReasoning = map[string]Part{}, map[string]Part{}
	case "start", "finish", "error", "message-metadata", "abort":
	default:
		if !strings.HasPrefix(t, "data-") {
			m.parts = append(m.parts, chunk)
			return nil
		}
		if transient, _ := chunk["transient"].(bool); transient {
			return nil
		}
		if dataID, ok := chunk["id"]; ok && dataID != nil {
			for _, p := range m.parts {
				if str(p["type"]) == t && p["id"] == dataID {
					p["data"] = chunk["data"]
					return nil
				}
			}
		}
		m.parts = append(m.parts, chunk)
	}
	return nil
}

// FixturePath is the recorded production turn: every stored UI-message chunk of
// ags-dbf8nggkbcjs73a5r3u0 turn 1 (5,602 rows, 2.9 MB), one JSON per line, in
// seq order (w10/m16).
func FixturePath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", "ags-dbf8nggkbcjs73a5r3u0-turn1.jsonl.gz")
}

// LoadFixture reads the recorded production turn's raw chunks.
func LoadFixture() ([]json.RawMessage, error) {
	f, err := os.Open(FixturePath())
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	var out []json.RawMessage
	scanner := bufio.NewScanner(gz)
	scanner.Buffer(make([]byte, 0, 1<<20), 4<<20)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		if len(line) > 0 {
			out = append(out, line)
		}
	}
	return out, scanner.Err()
}
