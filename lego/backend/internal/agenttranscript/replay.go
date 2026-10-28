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

package agenttranscript

import (
	"encoding/json"
	"strings"
)

// ChunkType returns a payload's `type` field ("" when it has none or is not a
// JSON object).
func ChunkType(part json.RawMessage) string {
	var head struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(part, &head) != nil {
		return ""
	}
	return head.Type
}

// storedPart is the union of the item fields replay reads.
type storedPart struct {
	Type             string          `json:"type"`
	ID               string          `json:"id"`
	Text             string          `json:"text"`
	ProviderMetadata json.RawMessage `json:"providerMetadata"`
	toolItem
}

// isItem reports whether a stored payload is a coalesced UIMessage part (text,
// reasoning or tool) rather than a single UI-message chunk. No chunk of the v6
// vocabulary has these shapes: chunk types are text-start/-delta/-end, and tool
// chunks never carry a `state`.
func isItem(p storedPart) bool {
	switch {
	case p.Type == "text" || p.Type == "reasoning":
		return true
	case p.Type == "dynamic-tool" || strings.HasPrefix(p.Type, "tool-"):
		return p.ToolCallID != "" && p.State != ""
	}
	return false
}

// Expand returns the standard Vercel AI SDK v6 UI-message chunks that rebuild a
// stored payload: text → text-start + one text-delta + text-end, reasoning →
// the reasoning-* equivalent, tool → tool-input-available plus its
// tool-output-available / tool-output-error / tool-output-denied. Any other
// payload (a legacy raw chunk, start/finish, a data part) is already a chunk
// and is returned as-is.
func Expand(part json.RawMessage) []json.RawMessage {
	out := expand(part)
	for _, chunk := range out {
		if chunk == nil {
			return []json.RawMessage{part}
		}
	}
	return out
}

func expand(part json.RawMessage) []json.RawMessage {
	var p storedPart
	if json.Unmarshal(part, &p) != nil || !isItem(p) {
		return []json.RawMessage{part}
	}
	pm := nonNull(p.ProviderMetadata)
	if p.Type == "text" || p.Type == "reasoning" {
		type textChunk struct {
			Type             string          `json:"type"`
			ID               string          `json:"id"`
			Delta            *string         `json:"delta,omitempty"`
			ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
		}
		out := []json.RawMessage{mustMarshal(textChunk{Type: p.Type + "-start", ID: p.ID, ProviderMetadata: pm})}
		if p.Text != "" {
			out = append(out, mustMarshal(textChunk{Type: p.Type + "-delta", ID: p.ID, Delta: &p.Text}))
		}
		return append(out, mustMarshal(textChunk{Type: p.Type + "-end", ID: p.ID, ProviderMetadata: pm}))
	}
	t := p.toolItem
	dynamic := p.Type == "dynamic-tool"
	var dyn *bool
	if dynamic {
		dyn = &dynamic
	}
	out := []json.RawMessage{mustMarshal(struct {
		Type             string          `json:"type"`
		ToolCallID       string          `json:"toolCallId"`
		ToolName         string          `json:"toolName"`
		Input            json.RawMessage `json:"input,omitempty"`
		Dynamic          *bool           `json:"dynamic,omitempty"`
		Title            *string         `json:"title,omitempty"`
		ProviderExecuted *bool           `json:"providerExecuted,omitempty"`
		ToolMetadata     json.RawMessage `json:"toolMetadata,omitempty"`
		ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
	}{"tool-input-available", t.ToolCallID, t.ToolName, t.Input, dyn, t.Title, t.ProviderExecuted, t.ToolMetadata, nonNull(t.CallPM)})}
	type outcome struct {
		Type             string          `json:"type"`
		ToolCallID       string          `json:"toolCallId"`
		Output           json.RawMessage `json:"output,omitempty"`
		ErrorText        *string         `json:"errorText,omitempty"`
		Dynamic          *bool           `json:"dynamic,omitempty"`
		Preliminary      *bool           `json:"preliminary,omitempty"`
		ProviderExecuted *bool           `json:"providerExecuted,omitempty"`
		ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
	}
	switch t.State {
	case "output-available":
		output := t.Output
		if len(output) == 0 {
			output = json.RawMessage("null")
		}
		out = append(out, mustMarshal(outcome{Type: "tool-output-available", ToolCallID: t.ToolCallID, Output: output,
			Dynamic: dyn, Preliminary: t.Preliminary, ProviderExecuted: t.ProviderExecuted, ProviderMetadata: nonNull(t.ResultPM)}))
	case "output-error":
		errorText := ""
		if t.ErrorText != nil {
			errorText = *t.ErrorText
		}
		out = append(out, mustMarshal(outcome{Type: "tool-output-error", ToolCallID: t.ToolCallID, ErrorText: &errorText,
			Dynamic: dyn, ProviderExecuted: t.ProviderExecuted, ProviderMetadata: nonNull(t.ResultPM)}))
	case "output-denied":
		out = append(out, mustMarshal(outcome{Type: "tool-output-denied", ToolCallID: t.ToolCallID}))
	}
	return out
}

// mustMarshal returns nil when v cannot be encoded; Expand then falls back to
// the stored payload. Every fragment comes from already-parsed JSON.
func mustMarshal(v any) json.RawMessage {
	out, err := marshalNoEscape(v)
	if err != nil {
		return nil
	}
	return out
}
