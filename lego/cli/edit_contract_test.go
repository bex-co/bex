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

package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/render-oss/cli/cmd"

	"github.com/bex-co/bex/lego/cli/internal/desktop"
)

// The pinned Render CLI must not own `edit`: bex's command would silently
// replace it. A pin bump that adds one fails here.
func TestPinnedUpstreamHasNoRootEditCommand(t *testing.T) {
	for _, c := range cmd.RootCmd.Commands() {
		if c.Name() == "edit" || c.HasAlias("edit") {
			t.Fatalf("upstream %q now claims `edit`; reconcile with bex edit before bumping the pin", c.CommandPath())
		}
	}
}

// The shorthand must never take an upstream root command, even when the
// working directory holds a folder of the same name. No command name is
// path-shaped today; this guards the rule against a future loosening.
func TestShorthandNeverShadowsAnUpstreamRootCommand(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	for _, c := range cmd.RootCmd.Commands() {
		names := append([]string{c.Name()}, c.Aliases...)
		for _, name := range names {
			if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
				t.Fatal(err)
			}
			got, err := desktop.Shorthand([]string{name})
			if err != nil || !slices.Equal(got, []string{name}) {
				t.Fatalf("Shorthand(%q) = %q, %v; want the command untouched", name, got, err)
			}
		}
	}
}

func TestBexBinaryEditShorthand(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A launcher that would record any exec: none of these may reach it.
	bin := t.TempDir()
	marker := filepath.Join(dir, "launched")
	script := "#!/bin/sh\necho \"$@\" > " + marker + "\n"
	if err := os.WriteFile(filepath.Join(bin, desktop.LauncherName), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, int) {
		c := exec.Command(bexBinary, args...)
		c.Dir = dir
		c.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "BEX_NO_UPDATE_NOTIFIER=1", "CI=")
		out, _ := c.CombinedOutput()
		return string(out), c.ProcessState.ExitCode()
	}

	// Not a terminal: the shorthand reaches `bex edit`, which refuses.
	if out, code := run("."); code != 1 || !strings.Contains(out, "run `bex-desktop` directly") {
		t.Fatalf("bex . = %d %q", code, out)
	}
	if out, code := run("./missing"); code != 1 || !strings.Contains(out, "no such file or directory: ./missing") {
		t.Fatalf("bex ./missing = %d %q", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("a missing path was created")
	}
	// A bare word stays the upstream command beside a same-named folder.
	if out, code := run("docs", "--help"); code != 0 || strings.Contains(out, "bex desktop") {
		t.Fatalf("bex docs --help = %d %q", code, out)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the launcher ran")
	}
}
