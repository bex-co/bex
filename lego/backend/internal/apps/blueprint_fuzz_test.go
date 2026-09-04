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

import "testing"

// blueprintFuzzSeeds are the corpus every Blueprint fuzz target starts from:
// the real render.yaml fixtures the package's own tests exercise, plus a few
// hand-picked malformed shapes that steer the mutator toward the compiler's
// scalar, reference, and structural edges. Seeds live as source so a reviewer
// can see exactly what the untrusted-input surface is being probed with; the
// on-disk testdata/fuzz cache is never committed.
func blueprintFuzzSeeds(f *testing.F) {
	f.Helper()
	seeds := []string{
		"",
		"{}",
		"null",
		"[]",
		compilerValidBlueprint,
		fiveFieldManifest,
		envScopedGroupManifest,
		foreignMatchManifest,
		// key-value flavor (both spellings are accepted service types).
		"services:\n  - name: cache\n    type: keyvalue\n    ipAllowList: []\n",
		"services:\n  - name: cache\n    type: redis\n    ipAllowList: []\n",
		// a disk-bearing service and a numeric-heavy database.
		"services:\n  - name: web\n    type: web\n    runtime: image\n    image: {url: web:1}\n    numInstances: 3\ndatabases:\n  - name: db\n    plan: basic-256mb\n    diskSizeGB: 10\n    postgresMajorVersion: '16'\n",
		// scaling + fromService host/port references across siblings.
		"services:\n  - name: web\n    type: web\n    runtime: image\n    image: {url: web:1}\n    scaling: {minInstances: 1, maxInstances: 5, targetCPUPercent: 70}\n    envVars:\n      - {key: API, fromService: {name: api, type: web, property: host}}\n  - name: api\n    type: web\n    runtime: image\n    image: {url: api:1}\n",
		// malformed / edge shapes.
		"services:\n  - {",
		"services: [",
		"\x00",
		":",
		"- - - - -",
		"a: &x\n  b: *x\n",
		"services:\n  - name: web\n    type: web\n    numInstances: 99999999999999999999\n",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
}

// FuzzCompileBlueprintIR drives the strict untrusted-render.yaml intake — the
// YAML source walk, JSON-schema validation, capability gate, and IR
// normalization — asserting it can never panic on any input. Every discovered
// crasher is promoted to a named f.Add seed (or a plain regression test) with a
// root-cause fix, never a blanket recover.
func FuzzCompileBlueprintIR(f *testing.F) {
	blueprintFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, manifest string) {
		source, ir, problems := CompileBlueprintIR(manifest)
		// A clean compile must hand every later stage a usable source tree; a
		// nil source with no reported problem is itself a compiler defect.
		if len(problems) == 0 && source == nil {
			t.Fatalf("CompileBlueprintIR returned no problems and a nil source for %q", manifest)
		}
		_ = ir
	})
}

// FuzzParseCompiledStack drives the full deploy-apply preparation for an
// untrusted manifest: compile, then the typed adapter parse that classifies
// services, databases, env groups, and cross-resource references into a
// parsedStack. This is the deepest stage reachable from a Blueprint create/sync
// body, so it exercises the scalar-to-typed conversions the compiler itself
// does not. It must never panic; a rejected manifest returns an error.
func FuzzParseCompiledStack(f *testing.F) {
	blueprintFuzzSeeds(f)
	f.Fuzz(func(t *testing.T, manifest string) {
		_, _ = parseStack(DeployRequest{Manifest: manifest})
	})
}
