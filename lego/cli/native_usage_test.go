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
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/render-oss/cli/pkg/command"
	"github.com/spf13/cobra"
)

// nativePaths is every command of bex's own tree, by its path below the root.
func nativePaths() map[string]*cobra.Command {
	paths := map[string]*cobra.Command{}
	var walk func(prefix []string, c *cobra.Command)
	walk = func(prefix []string, c *cobra.Command) {
		path := append(slices.Clone(prefix), c.Name())
		paths[strings.Join(path, " ")] = c
		for _, sub := range c.Commands() {
			walk(path, sub)
		}
	}
	for _, c := range nativeCommands("dev") {
		walk(nil, c)
	}
	return paths
}

// runNative runs args in a fresh copy of bex's own tree. Usage is silenced
// at the root so stdout holds only what a command printed itself.
func runNative(args ...string) (string, error) {
	root := &cobra.Command{Use: "bex", SilenceErrors: true, SilenceUsage: true}
	root.AddCommand(nativeCommands("dev")...)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := root.Execute()
	return stdout.String(), err
}

func exitCodeOf(err error) int {
	var coded command.ExitCoder
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	if err != nil {
		return 1
	}
	return 0
}

// w8/368: every bex-native group refuses a word that names none of its
// subcommands with exit 2, where cobra would print the group's help (or run
// it) and exit 0. A new group anywhere in nativeCommands is walked here.
func TestEveryNativeGroupRefusesAnUnknownWord(t *testing.T) {
	var groups []string
	for path, c := range nativePaths() {
		if !c.HasSubCommands() {
			continue
		}
		groups = append(groups, path)
		stdout, err := runNative(append(strings.Fields(path), "qa-nosuchsub")...)
		if code := exitCodeOf(err); code != 2 {
			t.Errorf("bex %s qa-nosuchsub: exit %d (%v), want 2", path, code, err)
			continue
		}
		if want := `unknown command "qa-nosuchsub" for "bex ` + path + `"`; !strings.HasPrefix(err.Error(), want) {
			t.Errorf("bex %s qa-nosuchsub: %q, want it to start %q", path, err, want)
		}
		if stdout != "" {
			t.Errorf("bex %s qa-nosuchsub printed on stdout:\n%s", path, stdout)
		}
	}
	for _, known := range []string{"agent", "agent secrets", "agent skills", "agent automations", "agent env", "code", "code keys", "s3", "s3 cors", "s3 lifecycle", "s3 keys"} {
		if !slices.Contains(groups, known) {
			t.Errorf("the walk did not reach the group %q (it found %v)", known, groups)
		}
	}
}

// Cobra's own flag errors exit 1; every bex-native command's exit 2. The
// launchers that hand every argument to another program parse no flags.
func TestEveryNativeCommandRefusesAnUnknownFlagWithExit2(t *testing.T) {
	checked := 0
	for path, c := range nativePaths() {
		if c.DisableFlagParsing {
			continue
		}
		checked++
		_, err := runNative(append(strings.Fields(path), "--qa-no-such-flag")...)
		if code := exitCodeOf(err); code != 2 || !strings.Contains(err.Error(), "unknown flag: --qa-no-such-flag") {
			t.Errorf("bex %s --qa-no-such-flag: exit %d (%v), want 2 and the unknown flag", path, code, err)
		}
	}
	if checked < 50 {
		t.Errorf("checked %d commands, want the whole native tree", checked)
	}
}
