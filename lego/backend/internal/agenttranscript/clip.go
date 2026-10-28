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
	"regexp"
	"strconv"
	"unicode/utf8"
)

const (
	// MaxToolOutputChars is the longest tool output text stored inline. Longer
	// text keeps its first and last clipKeepChars characters around a marker
	// (OpenHands' head+tail clip); the complete text goes to the full-output
	// side table (FullOutput, ADR051 amendment 2026-10-27, w10/m18).
	MaxToolOutputChars = 8000
	clipKeepChars      = MaxToolOutputChars / 2

	// MaxFullOutputBytes caps one stored full output. Longer text keeps its
	// first MaxFullOutputBytes bytes (on a character boundary) followed by
	// "\n\n[bex: full output capped at 1 MiB, N chars omitted]".
	MaxFullOutputBytes = 1 << 20
)

// clippedMarker matches a marker ClipText wrote. Text already clipped (its
// length is the cap plus one marker) is not clipped again, so re-coalescing a
// replayed item is the identity.
var clippedMarker = regexp.MustCompile(`\n\n\[bex: \d+ chars clipped\]\n\n`)

const maxMarkerChars = 48

// ClipText returns s unchanged when it has at most MaxToolOutputChars
// characters; otherwise its head and tail around
// "\n\n[bex: N chars clipped]\n\n", N being the removed character count. The
// dashboard matches the marker with /\[bex: (\d+) chars clipped\]/.
func ClipText(s string) (string, bool) {
	out, clipped, _ := clipText(s)
	return out, clipped
}

// clipText is ClipText that also reports text a previous clip already
// shortened (already: the marker is there and nothing is removed now).
func clipText(s string) (out string, clipped, already bool) {
	if len(s) <= MaxToolOutputChars {
		return s, false, false
	}
	total := utf8.RuneCountInString(s)
	if total <= MaxToolOutputChars {
		return s, false, false
	}
	if total <= MaxToolOutputChars+maxMarkerChars && clippedMarker.MatchString(s) {
		return s, false, true
	}
	head := byteOffsetOfRune(s, clipKeepChars)
	tail := byteOffsetOfRune(s, total-clipKeepChars)
	removed := total - 2*clipKeepChars
	return s[:head] + "\n\n[bex: " + strconv.Itoa(removed) + " chars clipped]\n\n" + s[tail:], true, false
}

// capFullOutput bounds one full output at MaxFullOutputBytes, marking the cut.
func capFullOutput(s string) string {
	if len(s) <= MaxFullOutputBytes {
		return s
	}
	cut := MaxFullOutputBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	omitted := utf8.RuneCountInString(s[cut:])
	return s[:cut] + "\n\n[bex: full output capped at 1 MiB, " + strconv.Itoa(omitted) + " chars omitted]"
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

// clipResult is what clipJSON did to one output value.
type clipResult struct {
	// full is the complete text of what was clipped: the original string when
	// exactly one string was clipped, else the original JSON value. Empty when
	// nothing was clipped now.
	full string
	// truncated: something was clipped now or by an earlier clip.
	truncated bool
}

// clipJSON clips a tool output: a JSON string is clipped as text, and every
// string inside a structured (object/array) output is clipped the same way.
// The input is returned byte-for-byte when nothing needs clipping.
func clipJSON(raw json.RawMessage) (json.RawMessage, clipResult) {
	// A string longer than the cap needs at least that many bytes plus quotes.
	if len(raw) <= MaxToolOutputChars+2 {
		return raw, clipResult{}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if dec.Decode(&value) != nil {
		return raw, clipResult{}
	}
	var st clipState
	value = st.clip(value)
	res := clipResult{truncated: len(st.originals) > 0 || st.already}
	if len(st.originals) == 0 {
		return raw, res
	}
	out, err := marshalNoEscape(value)
	if err != nil {
		return raw, clipResult{}
	}
	res.full = string(raw)
	if len(st.originals) == 1 {
		res.full = st.originals[0]
	}
	return out, res
}

type clipState struct {
	originals []string
	already   bool
}

func (st *clipState) clip(v any) any {
	switch x := v.(type) {
	case string:
		out, clipped, already := clipText(x)
		if clipped {
			st.originals = append(st.originals, x)
		}
		st.already = st.already || already
		return out
	case []any:
		for i, e := range x {
			x[i] = st.clip(e)
		}
		return x
	case map[string]any:
		for k, e := range x {
			x[k] = st.clip(e)
		}
		return x
	default:
		return v
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
