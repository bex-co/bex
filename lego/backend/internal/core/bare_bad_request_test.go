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
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// bareBadRequestAllowed lists the non-test returns that may answer the bare
// core.ErrBadRequest sentinel, as file -> count. Every other client-input
// rejection wraps it with the field and the rule (w4/383), so a REST,
// GraphQL or MCP caller is told what to fix. An allowed site is not client
// input: an upstream 4xx, or a malformed SSH username the gateway reports
// itself.
var bareBadRequestAllowed = map[string]int{
	"github/service.go": 2,
	"apps/service.go":   2,
}

// TestNoBareBadRequestReturnsInValidation fails when a new `return …,
// core.ErrBadRequest` appears without a detail.
func TestNoBareBadRequestReturnsInValidation(t *testing.T) {
	bare := regexp.MustCompile(`^\s*return .*\bcore\.ErrBadRequest$`)
	found := map[string]int{}
	root := ".."
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		for _, line := range strings.Split(string(raw), "\n") {
			if bare.MatchString(line) {
				found[filepath.ToSlash(rel)]++
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for file, n := range found {
		if n != bareBadRequestAllowed[file] {
			t.Errorf("%s has %d bare core.ErrBadRequest returns, allowed %d: wrap it with fmt.Errorf(\"%%w: <field>: <rule>\", core.ErrBadRequest)", file, n, bareBadRequestAllowed[file])
		}
	}
	for file, n := range bareBadRequestAllowed {
		if found[file] != n {
			t.Errorf("%s allowlist says %d bare returns but found %d; update the allowlist", file, n, found[file])
		}
	}
}
