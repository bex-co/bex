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

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"
)

// TestTheEnvironmentIsTheThreeCredentials is ADR112 D9 for the gateway binary:
// it reads the read-only Wasabi pair and the bearer it speaks to bex-api with,
// and nothing else. A fourth variable is configuration by the back door.
func TestTheEnvironmentIsTheThreeCredentials(t *testing.T) {
	want := []string{
		"BEX_OBJECT_STORAGE_GATEWAY_TOKEN", "BEX_OBJECT_STORAGE_READ_ACCESS_KEY", "BEX_OBJECT_STORAGE_READ_SECRET_KEY",
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "os" {
			switch sel.Sel.Name {
			case "Getenv", "LookupEnv":
				name, err := strconv.Unquote(call.Args[0].(*ast.BasicLit).Value)
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, name)
			case "Environ", "ExpandEnv", "Expand":
				t.Errorf("%s reads the environment wholesale with os.%s", fset.Position(sel.Pos()), sel.Sel.Name)
			}
		}
		return true
	})
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("main reads %v, want exactly %v", got, want)
	}
}
