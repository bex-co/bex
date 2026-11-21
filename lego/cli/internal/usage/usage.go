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

// Package usage gives a wrong command line of a bex-native command one exit
// code, 2, the code `bex agent` documents for a usage error (w8/368).
//
// Cobra alone answers 0 or 1. Below the root it takes a word that names no
// subcommand for an argument: a group with no run function then prints its
// help and exits 0, and one with a run function runs and ignores the word.
// Its own argument and flag checks return plain errors, which exit 1. The
// imported Render commands keep that, as the pinned Render CLI has it.
package usage

import (
	"errors"
	"fmt"
	"strings"

	"github.com/render-oss/cli/pkg/command"
	"github.com/spf13/cobra"
)

// ExitCode is the exit code of a command line that is wrong.
const ExitCode = 2

// helpOnlyAnnotation marks a group Guard made runnable only so that cobra
// checks its arguments.
const helpOnlyAnnotation = "bex.co/usage-help-only"

// Errorf is a command line that is wrong: exit code 2.
func Errorf(format string, args ...any) error {
	return command.NewExitError(ExitCode, fmt.Errorf(format, args...))
}

// Guard makes a wrong command line anywhere in root's tree exit 2: a flag
// cobra cannot parse, arguments a command's check refuses, and a word after a
// group that names none of its subcommands. Call it on each bex-native
// command before it joins the CLI's root. Without a word, a group still prints
// its help, or runs its own function.
func Guard(root *cobra.Command) {
	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		// The CLI's root records a flag error for upstream's telemetry.
		if parent := root.Parent(); parent != nil {
			err = parent.FlagErrorFunc()(c, err)
		}
		return withExitCode(err)
	})
	guard(root)
}

func guard(c *cobra.Command) {
	if c.HasSubCommands() {
		if c.Args == nil {
			c.Args = SubcommandArgs
			c.SilenceUsage = true
		}
		// Cobra checks the arguments of a runnable command only.
		if c.Run == nil && c.RunE == nil {
			c.RunE = func(c *cobra.Command, _ []string) error { return c.Help() }
			if c.Annotations == nil {
				c.Annotations = map[string]string{}
			}
			c.Annotations[helpOnlyAnnotation] = "true"
		}
	}
	if args := c.Args; args != nil {
		c.Args = func(c *cobra.Command, a []string) error { return withExitCode(args(c, a)) }
	}
	for _, sub := range c.Commands() {
		guard(sub)
	}
}

// HelpOnly reports whether c is a group that only prints its help when run,
// the way a group cobra cannot run does: its run function is Guard's.
func HelpOnly(c *cobra.Command) bool {
	return c.Annotations[helpOnlyAnnotation] != ""
}

// SubcommandArgs refuses a word after a group, with exit code 2, the way
// cobra refuses an unknown command at the root: naming the subcommands it is
// close to. Guard gives it to every group that has no check of its own.
func SubcommandArgs(c *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return Errorf("unknown command %q for %q%s", args[0], c.CommandPath(), suggestions(c, args[0]))
}

// suggestions is cobra's own "Did you mean this?" block, which it only
// writes for the root.
func suggestions(c *cobra.Command, typed string) string {
	if c.DisableSuggestions {
		return ""
	}
	if c.SuggestionsMinimumDistance <= 0 {
		c.SuggestionsMinimumDistance = 2
	}
	names := c.SuggestionsFor(typed)
	if len(names) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\nDid you mean this?\n")
	for _, name := range names {
		_, _ = fmt.Fprintf(&b, "\t%v\n", name)
	}
	return b.String()
}

// withExitCode gives err exit code 2 unless it already carries one.
func withExitCode(err error) error {
	var coded command.ExitCoder
	if err == nil || errors.As(err, &coded) {
		return err
	}
	return command.NewExitError(ExitCode, err)
}
