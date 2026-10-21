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
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// deliberateBlueprintDivergences are the refusals bex makes on purpose
// (ADR049, ADR018) or that Render makes too, matched on the refusal text. A
// corpus file may fail only for these or for a gap already filed
// (knownBlueprintGaps); any other refusal is a new class to triage (w5/m143).
var deliberateBlueprintDivergences = []string{
	"preview environments are not available", // incl. previewValue
	"per-resource region placement is not available on bex",
	"Render Workflows",                              // type: workflow and fromService workflow refs
	"sizeGB must be at least 10",                    // bex's 10 GB disk minimum (w1/078)
	"sync:false is not allowed inside an env group", // Render forbids it too (blueprint-spec)
	"a private_service requires a paid plan",        // Render has no free private service
	"'dockerFilePath' not allowed",                  // Render's key is dockerfilePath
	"is not a supported Render Postgres plan",       // retired Render plans (e.g. pro-4gb)
}

// knownBlueprintGaps are Render-accepted inputs bex still refuses, each owned
// by a board item; remove an entry when its item ships.
var knownBlueprintGaps = []string{
	"which has no such plainly-defined variable", // w5/170: copying a sibling's sync:false variable
	"is not trusted", // w5/171: public registries outside the allowlist (cgr.dev)
}

// TestRenderExamplesCorpus runs every render.yaml from github.com/render-examples
// through ValidateBlueprint (fetch them with scripts/render-examples-corpus.sh;
// they are third-party files, so they are not vendored). Opt-in:
// BEX_RENDER_EXAMPLES_DIR=<dir of *.yaml>.
func TestRenderExamplesCorpus(t *testing.T) {
	dir := os.Getenv("BEX_RENDER_EXAMPLES_DIR")
	if dir == "" {
		t.Skip("set BEX_RENDER_EXAMPLES_DIR to a directory of render-examples render.yaml files")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no corpus files in %s: %v", dir, err)
	}
	sort.Strings(files)
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	valid := 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(filepath.Base(f), ".yaml")
		v, err := svc.ValidateBlueprint(context.Background(), "", string(raw), "")
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if v.Valid {
			valid++
			continue
		}
		for _, e := range v.Errors {
			deliberate := false
			for _, d := range append(append([]string(nil), deliberateBlueprintDivergences...), knownBlueprintGaps...) {
				if strings.Contains(e.Error, d) {
					deliberate = true
					break
				}
			}
			if !deliberate {
				path := ""
				if e.Path != nil {
					path = *e.Path
				}
				t.Errorf("%s: %s %s: %s", name, e.Code, path, e.Error)
			}
		}
	}
	t.Logf("valid %d / %d", valid, len(files))
}
