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

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// instanceTypeWording matches the plan name Render retired on 2026-08-26:
// "'Compute plan' replaces 'instance type' across the dashboard, API, CLI, and
// docs" (docs/render-artifacts/compute-page.md, w6/m483). Identifiers such as
// InstanceType and codes such as INSTANCE_TYPE_UNAVAILABLE do not match.
var instanceTypeWording = regexp.MustCompile(`(?i)\binstance[ -]types?\b`)

// instanceTypeWordingAllowed are the literals that keep the old words because
// Render itself still publishes them: its dashboard calls the plan_changed
// event "Instance Type Changed" (render.com/docs/webhooks).
var instanceTypeWordingAllowed = map[string]bool{
	`"Instance Type Changed"`: true,
}

// TestNoInstanceTypeWordingInMessages fails when a non-test string literal in
// bex-api (an error, an email, an MCP description) calls a plan an "instance
// type" again.
func TestNoInstanceTypeWordingInMessages(t *testing.T) {
	fset := token.NewFileSet()
	for _, root := range []string{"..", "../../cmd"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if ok && lit.Kind == token.STRING && instanceTypeWording.MatchString(lit.Value) && !instanceTypeWordingAllowed[lit.Value] {
					t.Errorf("%s: %s calls a plan an instance type; Render calls it a compute plan", fset.Position(lit.Pos()), lit.Value)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
