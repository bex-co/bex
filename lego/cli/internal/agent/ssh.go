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

package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/bex-co/bex/lego/cli/internal/telemetry"
)

// ADR106 D5: a shell on a session's sandbox through the user's own ssh, the
// target `bex edit ags-…` opens in the editor (ADR054).

func sshCommand(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "ssh <ags-id> [-- <ssh args…>]",
		Short: "Open a shell on an agent session's sandbox",
		Long: "Runs your ssh against the session's sandbox (ags-…@ssh.bex.co), with your\n" +
			"registered SSH key, like `bex edit ags-…` does for the editor. Arguments\n" +
			"after -- go to ssh unchanged. The sandbox runs only while a turn does.",
		Args: func(c *cobra.Command, args []string) error {
			if dash := c.ArgsLenAtDash(); dash > 1 || (dash < 0 && len(args) > 1) {
				return usageError("put ssh's own arguments after --")
			}
			return sessionArgs(-1)(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			inv, cancel, err := d.start(c)
			if err != nil {
				return err
			}
			defer cancel()
			address, err := sandboxAddress(inv.ctx, inv.api, args[0])
			if err != nil {
				return err
			}
			startedAt := time.Now()
			return d.execSSH(sshArgv(args[1:], address), func() {
				telemetry.EmitLaunch(c.CommandPath(), startedAt, c.ErrOrStderr())
			})
		},
	}
}

// sandboxAddress is the SSH address of id's running sandbox.
func sandboxAddress(ctx context.Context, a *api, id string) (string, error) {
	s, _, err := a.session(ctx, id)
	if err != nil {
		return "", err
	}
	if s.SSHAddress == "" {
		return "", fmt.Errorf("%s has no running sandbox (it runs while a turn does); send it a message to start one", id)
	}
	return s.SSHAddress, nil
}

// execSystemSSH replaces this process with ssh, so the shell owns the
// terminal (and ssh's exit status is the command's).
func execSystemSSH(args []string, beforeExec func()) error {
	path, err := exec.LookPath("ssh")
	if err != nil {
		return fmt.Errorf("ssh is not installed or not on PATH: %w", err)
	}
	beforeExec()
	if err := syscall.Exec(path, append([]string{"ssh"}, args...), os.Environ()); err != nil {
		return fmt.Errorf("exec %s: %w", path, err)
	}
	return nil
}

// sshValueOptions are ssh's options that take a value (ssh(1)).
const sshValueOptions = "BbcDEeFIiJLlmOoPpQRSWw"

// sshArgv places dest where ssh expects it, `ssh [options] destination
// [command]`: after the leading options (and their values), before a remote
// command.
func sshArgv(passthrough []string, dest string) []string {
	i := 0
	for i < len(passthrough) {
		arg := passthrough[i]
		if arg == "--" || !strings.HasPrefix(arg, "-") || arg == "-" {
			break
		}
		i++
		if len(arg) == 2 && strings.ContainsRune(sshValueOptions, rune(arg[1])) && i < len(passthrough) {
			i++ // the option's value
		}
	}
	argv := append(append([]string{}, passthrough[:i]...), dest)
	return append(argv, passthrough[i:]...)
}
