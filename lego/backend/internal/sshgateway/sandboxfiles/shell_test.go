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

package sandboxfiles

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestShellRejectsExistingSymlinkAncestorsAndTargets(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	actual, alias := filepath.Join(root, "actual"), filepath.Join(root, "alias")
	if err := os.Mkdir(actual, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(actual, "secret"), []byte("must remain private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(actual, alias); err != nil {
		t.Fatal(err)
	}
	for _, command := range [][]string{
		shell(downloadScript, alias, "secret"),
		shell(stageScript, alias, "new-file", ".bex-copy-test", "file", "0"),
		shell(downloadScript, root, "alias"),
		shell(stageScript, root, "alias", ".bex-copy-test", "directory", "0"),
	} {
		cmd := exec.Command(command[0], command[1:]...)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			t.Fatal("symlink path accepted")
		}
		if out.Len() != 0 {
			t.Fatalf("refused path leaked output: %q", out.Bytes())
		}
	}
	if _, err := os.Stat(filepath.Join(actual, ".bex-copy-test")); !os.IsNotExist(err) {
		t.Fatal("upload followed symlink ancestor")
	}
}

func TestStageUsesLiteralArgumentsAndNeverPublishes(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	name := "quotes' $(touch injected); file"
	command := shell(stageScript, root, name, ".bex-copy-test", "file", "3")
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin = bytes.NewReader([]byte{0, 255, 1})
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("stage: %v: %s", err, output)
	}
	got, err := os.ReadFile(filepath.Join(root, ".bex-copy-test", "data"))
	if err != nil || !bytes.Equal(got, []byte{0, 255, 1}) {
		t.Fatalf("staged = %v, %v", got, err)
	}
	for _, forbidden := range []string{name, "injected"} {
		if _, err := os.Stat(filepath.Join(root, forbidden)); !os.IsNotExist(err) {
			t.Fatalf("stage published or executed %q", forbidden)
		}
	}
}
