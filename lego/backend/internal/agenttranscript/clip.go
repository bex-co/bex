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
	"bytes"
	"encoding/json"
	"strconv"
	"unicode/utf8"
)

const (
	// MaxToolOutputChars is the longest tool output text stored inline. Longer
	// text keeps its first and last clipKeepChars characters around a marker
	// (Claude Code's BASH_MAX_OUTPUT_LENGTH; OpenHands' head+tail clip).
	MaxToolOutputChars = 30000
	clipKeepChars      = MaxToolOutputChars / 2
)

// ClipText returns s unchanged when it has at most MaxToolOutputChars
// characters; otherwise its head and tail around
// "\n\n[bex: N chars clipped]\n\n", N being the removed character count. The
// dashboard matches the marker with /\[bex: (\d+) chars clipped\]/.
func ClipText(s string) (string, bool) {
	if len(s) <= MaxToolOutputChars {
		return s, false
	}
	total := utf8.RuneCountInString(s)
	if total <= MaxToolOutputChars {
		return s, false
	}
	head := byteOffsetOfRune(s, clipKeepChars)
	tail := byteOffsetOfRune(s, total-clipKeepChars)
	clipped := total - 2*clipKeepChars
	return s[:head] + "\n\n[bex: " + strconv.Itoa(clipped) + " chars clipped]\n\n" + s[tail:], true
}

func byteOffsetOfRune(s string, n int) int {
	for i := range s {
		if n == 0 {
			return i
		}
		n--
	}
	return len(s)
}

// clipJSON clips a tool output: a JSON string is clipped as text, and every
// string inside a structured (object/array) output is clipped the same way.
// The input is returned byte-for-byte when nothing needs clipping.
func clipJSON(raw json.RawMessage) (json.RawMessage, bool) {
	// A string longer than the cap needs at least that many bytes plus quotes.
	if len(raw) <= MaxToolOutputChars+2 {
		return raw, false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if dec.Decode(&value) != nil {
		return raw, false
	}
	value, clipped := clipValue(value)
	if !clipped {
		return raw, false
	}
	out, err := marshalNoEscape(value)
	if err != nil {
		return raw, false
	}
	return out, true
}

func clipValue(v any) (any, bool) {
	switch x := v.(type) {
	case string:
		return ClipText(x)
	case []any:
		changed := false
		for i, e := range x {
			var c bool
			x[i], c = clipValue(e)
			changed = changed || c
		}
		return x, changed
	case map[string]any:
		changed := false
		for k, e := range x {
			var c bool
			x[k], c = clipValue(e)
			changed = changed || c
		}
		return x, changed
	default:
		return v, false
	}
}

// marshalNoEscape is json.Marshal without HTML escaping (tool output is
// shell/markdown text; `<`/`>`/`&` stay readable and byte-stable) and without
// the encoder's trailing newline.
func marshalNoEscape(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}
