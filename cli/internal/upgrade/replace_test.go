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

package upgrade

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceBinaryInstallsAtomically(t *testing.T) {
	dir := t.TempDir()
	execPath := filepath.Join(dir, "bex")
	if err := os.WriteFile(execPath, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replaceBinary(execPath, []byte("NEW-BYTES")); err != nil {
		t.Fatalf("replaceBinary: %v", err)
	}
	got, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "NEW-BYTES" {
		t.Errorf("content = %q, want NEW-BYTES", got)
	}
	fi, _ := os.Stat(execPath)
	if fi.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want 0755 (preserved from original)", fi.Mode().Perm())
	}
	// No staged temp files left behind.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("stray files left in target dir: %v", entries)
	}
}

func TestReplaceBinaryRollsBackOnStagingFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory write permissions")
	}
	dir := t.TempDir()
	execPath := filepath.Join(dir, "bex")
	if err := os.WriteFile(execPath, []byte("ORIGINAL"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Make the directory unwritable so staging the temp file fails before the
	// original is ever touched.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err := replaceBinary(execPath, []byte("NEW"))
	if err == nil {
		t.Fatal("expected staging failure in a read-only directory")
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ORIGINAL" {
		t.Errorf("original binary was corrupted on failure: %q", got)
	}
}

func TestPackageManagerHint(t *testing.T) {
	owned := []string{
		"/opt/homebrew/Cellar/bex/1.2.0/bin/bex",
		"/usr/local/Cellar/bex/1.2.0/bin/bex",
		"/opt/homebrew/bin/bex",
		"/home/linuxbrew/.linuxbrew/bin/bex",
	}
	for _, p := range owned {
		hint, isOwned := packageManagerHint(p)
		if !isOwned {
			t.Errorf("%s should be package-manager owned", p)
		}
		if hint == "" {
			t.Errorf("%s: expected a non-empty hint", p)
		}
	}
	raw := []string{"/usr/local/bin/bex", "/home/alice/.local/bin/bex", "/root/bex"}
	for _, p := range raw {
		if _, isOwned := packageManagerHint(p); isOwned {
			t.Errorf("%s should not be package-manager owned", p)
		}
	}
}
