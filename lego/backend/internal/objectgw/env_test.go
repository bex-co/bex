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

package objectgw

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPackageReadsNoEnvironmentVariable enforces ADR112 D9 here as it does for
// internal/objectstorage: the gateway's environment is its three credentials,
// read by cmd/objectgw, and its endpoints, ports, URLs and caps are constants.
func TestPackageReadsNoEnvironmentVariable(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no sources found: %v", err)
	}
	reads := map[string]bool{"Getenv": true, "LookupEnv": true, "Environ": true, "ExpandEnv": true, "Expand": true}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "os" && reads[sel.Sel.Name] {
				t.Errorf("%s reads the environment with os.%s; ADR112 D9 allows credentials only, read by cmd/objectgw", fset.Position(sel.Pos()), sel.Sel.Name)
			}
			return true
		})
	}
}
