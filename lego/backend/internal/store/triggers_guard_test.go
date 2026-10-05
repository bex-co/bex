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

package store

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"
)

// TestAllTriggersListsEveryTriggerConst keeps AllTriggers — what REST
// conformance proves maps onto Render's deploy trigger enum (w8/058) — in
// step with every Trigger* constant this package declares.
func TestAllTriggersListsEveryTriggerConst(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "store.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var declared []string
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range spec.Names {
			if len(name.Name) > len("Trigger") && name.Name[:len("Trigger")] == "Trigger" && i < len(spec.Values) {
				if lit, ok := spec.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					value, _ := strconv.Unquote(lit.Value)
					declared = append(declared, value)
				}
			}
		}
		return true
	})
	if len(declared) < 6 {
		t.Fatalf("found %d Trigger* consts; the guard's parse looks broken", len(declared))
	}
	for _, v := range declared {
		if !slices.Contains(AllTriggers, v) {
			t.Errorf("trigger %q is declared but missing from AllTriggers", v)
		}
	}
	if len(AllTriggers) != len(declared) {
		t.Errorf("AllTriggers has %d entries, %d Trigger* consts declared", len(AllTriggers), len(declared))
	}
}
