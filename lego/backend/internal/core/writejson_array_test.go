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

package core

// writejson_array_test.go — w4/m116/t006. WriteJSON is the single serialization
// boundary every REST fragment shares, and the one place that can stop an empty
// list from printing as encoding/json's `null` against Render's
// `{"type":"array"}` contract. The cases below are the SHAPES the list routes
// actually hand it, plus the two shapes whose `null` is correct and must
// survive.

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

type writeJSONItem struct {
	ID string `json:"id"`
}

// marshalerList is a nil-able slice that decides its own JSON — json.RawMessage
// is the real instance of this shape, and its nil IS the JSON literal null.
type marshalerList []writeJSONItem

func (marshalerList) MarshalJSON() ([]byte, error) { return []byte(`null`), nil }

func TestWriteJSONNilSliceSerializesAsEmptyArray(t *testing.T) {
	var nilStructs []writeJSONItem
	var nilStrings []string
	var nilMaps []map[string]any
	var nilPointers []*writeJSONItem
	var nilAny []any

	for _, tc := range []struct {
		name string
		body any
		want string
	}{
		{"nil struct slice", nilStructs, `[]`},
		{"nil string slice", nilStrings, `[]`},
		{"nil map slice", nilMaps, `[]`},
		{"nil pointer slice", nilPointers, `[]`},
		{"nil any slice", nilAny, `[]`},
		{"already empty slice", []writeJSONItem{}, `[]`},
		{"populated slice is untouched", []string{"a"}, `["a"]`},
		// A nested nil slice is NOT normalized here — WriteJSON cannot rewrite a
		// field without re-deciding every response body's shape, so the envelope
		// routes guarantee their own arrays at the construction site (see
		// core.AllowListCIDRs and internal/api/emptylist_test.go).
		{"nested nil slice is left to its construction site", map[string]any{"cidrs": nilStrings}, `{"cidrs":null}`},
		// Exclusions: `null` is the RIGHT answer for both of these.
		{"nil json.RawMessage stays null", json.RawMessage(nil), `null`},
		{"nil byte slice stays null", []byte(nil), `null`},
		{"custom marshaler is obeyed", marshalerList(nil), `null`},
		{"nil body stays null", nil, `null`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteJSON(w, 200, tc.body)
			if got := trimJSON(w.Body.String()); got != tc.want {
				t.Errorf("WriteJSON(%#v) = %s, want %s", tc.body, got, tc.want)
			}
		})
	}
}

func trimJSON(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
