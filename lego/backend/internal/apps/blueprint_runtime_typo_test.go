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
	"slices"
	"testing"
)

// w8/238: a misspelled runtime is the one problem the schema reports; the
// build-field rules no longer add a false "Dockerfile or prebuilt-image
// service" error for it, and still judge every valid runtime.
func TestBlueprintRuntimeTypoReportsOnlyTheRuntime(t *testing.T) {
	service := func(fields string) string {
		return "services:\n  - type: web\n    name: qa-bp-rt\n    plan: free\n" + fields
	}
	for _, tc := range []struct {
		name, manifest string
		want           []string
	}{
		{"misspelled native runtime", service("    runtime: nodee\n    buildCommand: npm i\n    startCommand: node x\n"),
			[]string{"#/services/0/runtime"}},
		{"docker runtime with a buildCommand", service("    runtime: docker\n    repo: https://github.com/x/y\n    buildCommand: npm i\n"),
			[]string{"#/services/0/buildCommand"}},
		{"image runtime with a dockerfilePath", service("    runtime: image\n    image:\n      url: docker.io/library/nginx:1\n    dockerfilePath: ./Dockerfile\n"),
			[]string{"#/services/0/dockerfilePath"}},
		{"node runtime with a buildCommand", service("    runtime: node\n    buildCommand: npm i\n    startCommand: node x\n"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, problems := CompileBlueprintSource(tc.manifest)
			var paths []string
			for _, p := range problems {
				paths = append(paths, p.Path)
			}
			slices.Sort(paths)
			if !slices.Equal(paths, tc.want) {
				t.Fatalf("problems at %v, want %v: %+v", paths, tc.want, problems)
			}
		})
	}
}
