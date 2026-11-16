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

package build

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

func TestLoadToolchainInventory(t *testing.T) {
	inv, err := LoadToolchainInventory()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := inv.ClusterBuilderResolvedAt()
	if err != nil {
		t.Fatal(err)
	}
	if resolved.IsZero() {
		t.Fatal("cnb-builder resolved_at must be set")
	}
	if resolved.After(time.Now().UTC().Add(time.Minute)) {
		t.Fatalf("cnb-builder resolved_at %s is in the future", resolved)
	}
}

func TestParseToolchainInventoryFailsClosed(t *testing.T) {
	cases := map[string]string{
		"not json":        `{`,
		"wrong schema":    `{"schema":"nope","images":[{"id":"cnb-builder","kind":"builder","upstream":"docker.io/x:y","committed":"x@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","resolved_at":"2026-08-18T00:00:00Z","source":"s","files":["a.go"]}]}`,
		"empty images":    `{"schema":"bex.build-toolchain-freshness/v1","images":[]}`,
		"missing builder": `{"schema":"bex.build-toolchain-freshness/v1","images":[{"id":"native-node","kind":"native-base","upstream":"docker.io/library/node:24","committed":"node@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","resolved_at":"2026-08-18T00:00:00Z","source":"s","files":["a.go"]}]}`,
		"short digest":    `{"schema":"bex.build-toolchain-freshness/v1","images":[{"id":"cnb-builder","kind":"builder","upstream":"docker.io/x:y","committed":"x@sha256:dead","resolved_at":"2026-08-18T00:00:00Z","source":"s","files":["a.go"]}]}`,
		"bad timestamp":   `{"schema":"bex.build-toolchain-freshness/v1","images":[{"id":"cnb-builder","kind":"builder","upstream":"docker.io/x:y","committed":"x@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","resolved_at":"yesterday","source":"s","files":["a.go"]}]}`,
		"digest upstream": `{"schema":"bex.build-toolchain-freshness/v1","images":[{"id":"cnb-builder","kind":"builder","upstream":"docker.io/x@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","committed":"x@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","resolved_at":"2026-08-18T00:00:00Z","source":"s","files":["a.go"]}]}`,
		"unknown kind":    `{"schema":"bex.build-toolchain-freshness/v1","images":[{"id":"cnb-builder","kind":"other","upstream":"docker.io/x:y","committed":"x@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","resolved_at":"2026-08-18T00:00:00Z","source":"s","files":["a.go"]}]}`,
		"empty files":     `{"schema":"bex.build-toolchain-freshness/v1","images":[{"id":"cnb-builder","kind":"builder","upstream":"docker.io/x:y","committed":"x@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","resolved_at":"2026-08-18T00:00:00Z","source":"s","files":[]}]}`,
		"duplicate id":    `{"schema":"bex.build-toolchain-freshness/v1","images":[{"id":"cnb-builder","kind":"builder","upstream":"docker.io/x:y","committed":"x@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","resolved_at":"2026-08-18T00:00:00Z","source":"s","files":["a.go"]},{"id":"cnb-builder","kind":"builder","upstream":"docker.io/x:y","committed":"x@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","resolved_at":"2026-08-18T00:00:00Z","source":"s","files":["a.go"]}]}`,
	}
	for name, raw := range cases {
		if _, err := ParseToolchainInventory([]byte(raw)); err == nil {
			t.Errorf("%s: accepted malformed inventory", name)
		}
	}
}

// repoRoot is the repository checkout this test file sits in.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".."))
}

func TestToolchainInventoryMatchesCommittedPins(t *testing.T) {
	inv, err := LoadToolchainInventory()
	if err != nil {
		t.Fatal(err)
	}
	root := repoRoot(t)
	for _, img := range inv.Images {
		digest := img.Digest()
		if digest == "" {
			t.Errorf("%s: committed has no digest", img.ID)
			continue
		}
		for _, rel := range img.Files {
			body, err := os.ReadFile(filepath.Join(root, rel))
			if err != nil {
				t.Errorf("%s: %s: %v", img.ID, rel, err)
				continue
			}
			if !strings.Contains(string(body), digest) {
				t.Errorf("%s: digest %s missing from %s", img.ID, digest, rel)
			}
		}
	}
}

// TestToolchainPinSitesRunThisSuite: the check above reads every file the
// inventory lists, so a digest edited in one listed file and not another fails
// it only if editing that file runs this suite. operator-test.yml's paths must
// therefore cover each one, for pushes and pull requests alike. The paths are
// read from the inventory at run time, which scripts/ci_test_inputs.py cannot
// see (w6/m291: deploy/cnb-run, the kpack chart and deploy/hugo were missing).
func TestToolchainPinSitesRunThisSuite(t *testing.T) {
	inv, err := LoadToolchainInventory()
	if err != nil {
		t.Fatal(err)
	}
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "operator-test.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On map[string]struct {
			Paths []string `yaml:"paths"`
		} `yaml:"on"`
	}
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		t.Fatal(err)
	}
	for _, event := range []string{"push", "pull_request"} {
		paths := workflow.On[event].Paths
		if len(paths) == 0 {
			t.Fatalf("operator-test.yml %s has no paths", event)
		}
		for _, img := range inv.Images {
			for _, file := range img.Files {
				if !slices.ContainsFunc(paths, func(pattern string) bool { return workflowPathMatches(pattern, file) }) {
					t.Errorf("%s: editing %s does not run operator-test.yml on %s; add it to both paths lists", img.ID, file, event)
				}
			}
		}
	}
}

// workflowPathMatches covers the two pattern shapes operator-test.yml uses: an
// exact file and a directory's `/**`.
func workflowPathMatches(pattern, file string) bool {
	if dir, ok := strings.CutSuffix(pattern, "/**"); ok {
		return strings.HasPrefix(file, dir+"/")
	}
	return pattern == file
}

func TestWorkflowPathMatches(t *testing.T) {
	for _, tc := range []struct {
		pattern, file string
		want          bool
	}{
		{"lego/operator/**", "lego/operator/cmd/manager/main.go", true},
		{"deploy/hugo/**", "deploy/hugo/Dockerfile", true},
		{"deploy/hugo/**", "deploy/hugo-other/Dockerfile", false},
		{"lego/Dockerfile", "lego/Dockerfile", true},
		{"lego/Dockerfile", "lego/Dockerfile.dev", false},
	} {
		if got := workflowPathMatches(tc.pattern, tc.file); got != tc.want {
			t.Errorf("workflowPathMatches(%q, %q) = %v, want %v", tc.pattern, tc.file, got, tc.want)
		}
	}
}
