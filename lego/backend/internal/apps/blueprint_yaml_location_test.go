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

package apps

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// w4/632: a YAML syntax error's line and column are Render's 1-indexed
// position of the construct the message names, not yaml.v3's 0-based parser
// line at column 1. The first five rows are the QA probe's files; the
// expected values are PyYAML's marks for them.
func TestBlueprintYAMLSyntaxErrorPosition(t *testing.T) {
	for _, tc := range []struct {
		name, manifest string
		line, column   int
	}{
		// The opening `[` of the unclosed flow sequence.
		{"unclosed flow sequence", "services:\n  - type: web\n    name: [unclosed\n    runtime: image\n", 3, 11},
		{"unclosed flow sequence mid-file", "a: 1\nb: 2\nc: 3\nd: [1, 2\ne: 5\n", 4, 4},
		// An under-indented key breaks the block collection it should belong
		// to; the position is that collection's start (PyYAML's context mark),
		// the `-` on line 2, not the bad line 4.
		{"mis-indented key", "services:\n  - type: web\n    name: ok\n   runtime: image\n", 2, 3},
		{"tab indentation", "services:\n  - type: web\n\tname: tab\n", 3, 1},
		{"unterminated double quote", "services:\n  - type: web\n    name: \"unterminated\n    runtime: image\n", 3, 11},

		// The construct on line 1, where yaml.v3 reports the problem's line.
		{"flow mapping on line 1", "a: {b: 1\nc: 2\n", 1, 4},
		{"flow sequence at column 1", "[1, 2\n", 1, 1},
		{"single quote on line 1", "a: 'x\n", 1, 4},
		// Nested and multi-line flow: the innermost collection left open.
		{"inner flow sequence", "a: [[1, 2], [3\nb: 1\n", 1, 13},
		{"multi-line flow sequence", "x: 1\na: [1,\n  2,\n  3\nb: 1\n", 2, 4},
		// Quoting, comments and block scalars do not open a construct.
		{"bracket inside quotes", "x: \"[\"\na: [1, 2\nb: 1\n", 2, 4},
		{"bracket in a comment", "x: 1 # [\na: [1, 2\nb: 1\n", 2, 4},
		{"quote in a block scalar", "s: |\n  it's \"quoted\n  [\nq: 'open\n", 4, 4},
		{"anchored flow sequence", "a: 1\nb: &x [1, 2\nc: 3\n", 2, 7},
		{"tab before a sequence", "services:\n\t- type: web\n", 2, 1},
		// No construct to recover: the offending line's first token.
		{"under-indented mapping key", "a:\n  b: 1\n c: 2\n", 3, 2},
		// Columns count characters, not bytes.
		{"multi-byte key before a flow sequence", "é: [1, 2\nb: 1\n", 1, 4},
		{"multi-byte scalar before an inner flow sequence", "a: ['é', [3\nb: 1\n", 1, 10},
		{"multi-byte key before a quote", "ü: 'x\n", 1, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, problems := CompileBlueprintIR(tc.manifest)
			if len(problems) != 1 || problems[0].Code != "BLUEPRINT_YAML_SYNTAX" {
				t.Fatalf("problems = %+v, want one syntax refusal", problems)
			}
			if got := problems[0]; got.Line != tc.line || got.Column != tc.column {
				t.Errorf("%s: position = %d:%d, want %d:%d", got.Message, got.Line, got.Column, tc.line, tc.column)
			}
		})
	}
}

// w4/632: an unknown property points at its key, which the message names, not
// at its value.
func TestBlueprintUnknownPropertyPointsAtItsKey(t *testing.T) {
	manifest := "services:\n  - type: web\n    name: qa-web\n    runtime: image\n    image:\n      url: docker.io/library/busybox:1.37\n    colour: blue\n"
	_, _, problems := CompileBlueprintIR(manifest)
	if len(problems) != 1 || problems[0].Path != "#/services/0/colour" {
		t.Fatalf("problems = %+v, want the unknown colour property", problems)
	}
	if got := problems[0]; got.Line != 7 || got.Column != 5 {
		t.Errorf("position = %d:%d, want the key at 7:5", got.Line, got.Column)
	}
}

// w8/365: several unknown keys in one mapping are one problem each, at the
// key the message names, as a lone key is (w4/632). The validator lists them
// in Go map order, so repeated runs must still compare equal.
func TestBlueprintUnknownPropertiesPointAtEachKey(t *testing.T) {
	const web = "services:\n  - type: web\n    name: qa-web\n    runtime: node\n    plan: free\n" +
		"    repo: https://github.com/render-examples/express-hello-world\n" +
		"    buildCommand: yarn\n    startCommand: node app.js\n"
	type want struct {
		path, key    string
		line, column int
	}
	for _, tc := range []struct {
		name, manifest string
		want           []want
	}{
		{"two keys in a service and one at the root", web + "    bogusField: 1\n    healthCheck: /healthz\ntopLevelBogus: true\n", []want{
			{"#/services/0/bogusField", "bogusField", 9, 5},
			{"#/services/0/healthCheck", "healthCheck", 10, 5},
			{"#/topLevelBogus", "topLevelBogus", 11, 1},
		}},
		{"three keys in a service", web + "    zeta: 1\n    alpha: 2\n    mid: 3\n", []want{
			{"#/services/0/alpha", "alpha", 10, 5},
			{"#/services/0/mid", "mid", 11, 5},
			{"#/services/0/zeta", "zeta", 9, 5},
		}},
		{"two keys in an env var", web + "    envVars:\n      - key: GREETING\n        value: hi\n        colour: blue\n        shade: dark\n", []want{
			{"#/services/0/envVars/0/colour", "colour", 12, 9},
			{"#/services/0/envVars/0/shade", "shade", 13, 9},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, first := CompileBlueprintIR(tc.manifest)
			if len(first) != len(tc.want) {
				t.Fatalf("problems = %+v, want %d, one per unknown key", first, len(tc.want))
			}
			for i, w := range tc.want {
				got := first[i]
				parent := strings.TrimPrefix(w.path[:strings.LastIndex(w.path, "/")], "#")
				message := "at '" + parent + "': additional properties '" + w.key + "' not allowed"
				if got.Code != "BLUEPRINT_SCHEMA_INVALID" || got.Path != w.path || got.Message != message || got.Line != w.line || got.Column != w.column {
					t.Errorf("problem %d = %+v, want %s %q at %d:%d", i, got, w.path, message, w.line, w.column)
				}
			}
			for run := 0; run < 20; run++ {
				if _, _, again := CompileBlueprintIR(tc.manifest); !reflect.DeepEqual(again, first) {
					t.Fatalf("run %d: problems = %+v, want %+v", run, again, first)
				}
			}
		})
	}
}

// Placing a syntax error costs time linear in the manifest: a one-line
// manifest at the size cap, packed with flow collections or quoted scalars
// that open before the error, is located well inside a second and at the same
// position as a short one.
func TestBlueprintYAMLSyntaxErrorLocationIsLinear(t *testing.T) {
	for _, tc := range []struct{ name, item string }{
		{"flow sequences", "[]"},
		{"quoted scalars", "''"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tail := tc.item + " x y z"
			n := (blueprintMaxManifestBytes - len("a: [") - len(tail)) / len(tc.item+",")
			manifest := "a: [" + strings.Repeat(tc.item+",", n) + tail
			if len(manifest) > blueprintMaxManifestBytes || len(manifest) < blueprintMaxManifestBytes-len(tc.item+",") {
				t.Fatalf("manifest is %d bytes, want just under the %d cap", len(manifest), blueprintMaxManifestBytes)
			}
			start := time.Now()
			_, problems := CompileBlueprintSource(manifest)
			elapsed := time.Since(start)
			if len(problems) != 1 || problems[0].Code != "BLUEPRINT_YAML_SYNTAX" {
				t.Fatalf("problems = %+v, want one syntax refusal", problems)
			}
			if got := problems[0]; got.Line != 1 || got.Column != 4 {
				t.Errorf("%s: position = %d:%d, want the unclosed [ at 1:4", got.Message, got.Line, got.Column)
			}
			if elapsed > time.Second {
				t.Errorf("locating the error took %s, want under 1s", elapsed)
			}
		})
	}
}
