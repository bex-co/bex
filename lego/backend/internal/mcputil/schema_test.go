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

package mcputil_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

type sampleSchemaArgs struct {
	Mode    string  `json:"mode,omitempty" jsonschema:"the mode"`
	OptMode *string `json:"optMode,omitempty" jsonschema:"optional pointer mode"`
	Union   any     `json:"union,omitempty" jsonschema:"number or string"`
}

func TestInputSchemaOptions(t *testing.T) {
	sch := mcputil.InputSchema[sampleSchemaArgs](
		mcputil.PropEnum("mode", "a", "b"),
		mcputil.PropEnum("optMode", "x", "y"),
		mcputil.PropTypes("union", "number", "string"),
	)
	raw, err := json.Marshal(sch)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc struct {
		Properties map[string]struct {
			Type        any      `json:"type"`
			Enum        []string `json:"enum"`
			Description string   `json:"description"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, raw)
	}
	if !slices.Equal(doc.Properties["mode"].Enum, []string{"a", "b"}) || doc.Properties["mode"].Type != "string" {
		t.Errorf("mode = %+v, want type=string enum=[a b]", doc.Properties["mode"])
	}
	// Pointer *string narrowed from ["null","string"] to "string" to agree with enum.
	if !slices.Equal(doc.Properties["optMode"].Enum, []string{"x", "y"}) || doc.Properties["optMode"].Type != "string" {
		t.Errorf("optMode = %+v, want type=string enum=[x y]", doc.Properties["optMode"])
	}
	unionTypes, ok := doc.Properties["union"].Type.([]any)
	if !ok || len(unionTypes) != 2 || unionTypes[0] != "number" || unionTypes[1] != "string" {
		t.Errorf("union = %+v, want type=[number string]", doc.Properties["union"])
	}
}

func TestMergeEnums(t *testing.T) {
	got := mcputil.MergeEnums([]string{"b", "a"}, []string{"c", "a"}, nil)
	if !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("MergeEnums = %v, want [a b c]", got)
	}
}

func TestInputSchemaPanicsOnBadConfig(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func()
	}{
		{"unknown property enum", func() { mcputil.InputSchema[sampleSchemaArgs](mcputil.PropEnum("missing", "a")) }},
		{"empty enum", func() { mcputil.InputSchema[sampleSchemaArgs](mcputil.PropEnum("mode")) }},
		{"unknown property types", func() { mcputil.InputSchema[sampleSchemaArgs](mcputil.PropTypes("missing", "string")) }},
		{"empty types", func() { mcputil.InputSchema[sampleSchemaArgs](mcputil.PropTypes("union")) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic")
				}
			}()
			tc.fn()
		})
	}
}
