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

package s3

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/render-oss/cli/pkg/command"
	"github.com/spf13/cobra"
)

// yesFlag adds --yes (-y) to a command that asks before it deletes, with the
// name and help text of the flag on `bex ea objects delete`.
func yesFlag(c *cobra.Command, yes *bool) {
	c.Flags().BoolVarP(yes, "yes", "y", false, "Skip confirmation prompt")
}

// confirmDelete asks before a delete (one object, or many), in the words and
// with the rules of `bex ea objects delete`, and says whether to go on. `what`
// completes "Are you sure you want to delete ...?".
//
// It asks only when the command is interactive, which is the root command's
// own reading of the run (every one of stdin, stdout and stderr is a terminal,
// TERM is not dumb, CI is not set, and neither `-o` nor RENDER_OUTPUT chose a
// format): a pipe, a script or CI is never asked. --yes and the root --confirm
// flag ("Skip all confirmation prompts") skip the question. An answer that is
// not y or yes, and an end of input, cancel: "Delete canceled." is printed and
// the command succeeds having done nothing. A command run with no root (the
// unit tests') has no format and never asks.
func (d deps) confirmDelete(c *cobra.Command, yes bool, what string) bool {
	ctx := commandContext(c)
	if yes || command.GetConfirmFromContext(ctx) {
		return true
	}
	if format := command.GetFormatFromContext(ctx); format == nil || !format.Interactive() {
		return true
	}
	out := c.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Are you sure you want to delete %s? This cannot be undone. [y/N]: ", what)
	if d.stdin != nil {
		if line, err := bufio.NewReader(d.stdin).ReadString('\n'); err == nil {
			if answer := strings.ToLower(strings.TrimSpace(line)); answer == "y" || answer == "yes" {
				return true
			}
		}
	}
	_, _ = fmt.Fprintln(out, "Delete canceled.")
	return false
}
