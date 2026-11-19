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

// Package s3 is `bex s3`: the AWS CLI pointed at bex's S3-compatible API with
// a key minted for the signed-in person (ADR112 D3), and the bex-owned
// commands around it: `env`, `keys`, `public` and `info`. It is a thin client
// of bex-api's object-storage routes; quota, metering and authorization stay
// with the server. It imports nothing from the other modules, so the REST
// shapes below are written from the routes' contract, not shared with them.
package s3

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/spf13/cobra"
)

// exitUsage is the exit code of a command line that is wrong, as `bex agent`
// uses it.
const exitUsage = 2

// deps is the process boundary, injectable for tests.
type deps struct {
	api       func() (*api, error)
	workspace func() (string, error)
	host      func() string
	lookupEnv func(string) (string, bool)
	environ   func() []string
	state     func() (string, error)
	now       func() time.Time
	hostname  func() string
	lookPath  func(string) (string, error)
	// exec replaces the process with path, after beforeExec (the launch
	// telemetry).
	exec func(path string, argv, env []string, beforeExec func()) error
}

func osDeps() deps {
	return deps{
		api:       defaultAPI,
		workspace: config.WorkspaceID,
		host:      cfg.GetHost,
		lookupEnv: os.LookupEnv,
		environ:   os.Environ,
		state:     config.StateDir,
		now:       time.Now,
		hostname: func() string {
			name, _ := os.Hostname()
			return name
		},
		lookPath: exec.LookPath,
		exec:     execAWS,
	}
}

// Commands are `bex s3` and `bex s3api`.
func Commands() []*cobra.Command {
	return newCommands(osDeps())
}

func newCommands(d deps) []*cobra.Command {
	s3 := &cobra.Command{
		Use:   "s3 <aws s3 command> [arguments…] | env | keys | public | info",
		Short: "Use bex object storage with the AWS CLI: buckets, cp, ls, mv, rm, sync, presign",
		Long: "Runs your AWS CLI (`aws s3 …`) against bex object storage, with the endpoint\n" +
			"and a key already set: sign in with `bex login` and use any `aws s3` command.\n" +
			"A key is made for you, lasts twelve hours and is replaced when under one hour\n" +
			"is left; your own AWS profile and credentials are never used or sent, and bex's\n" +
			"key never lands in ~/.aws.\n\n" +
			"Everything except the four names below is handed to `aws s3` as typed:\n" +
			"  bex s3 mb s3://photos --region frankfurt\n" +
			"  bex s3 sync ./site s3://photos/site\n" +
			"  bex s3 presign s3://photos/cat.jpg\n\n" +
			"bex's own commands:\n" +
			"  env                                  the endpoint and key as variables for rclone, boto3, s5cmd or Terraform\n" +
			"  keys create|list|revoke              persistent keys for CI and long transfers\n" +
			"  public <bucket> enable|disable|status  publish a bucket read-only (workspace admin)\n" +
			"  info <bucket>                        a bucket's region, public host, objects and bytes\n\n" +
			"`bex s3api <call>` runs `aws s3api` the same way, for the calls `aws s3` has\n" +
			"no command for (bucket CORS and lifecycle among them). An upload larger than\n" +
			"5 GiB is refused. A key that expires during a transfer fails the transfer with\n" +
			"a 403: use a persistent key (`bex s3 keys create`) for a transfer over hours.",
		Example:            "  bex s3 ls\n  bex s3 cp cat.jpg s3://photos/cat.jpg\n  bex s3 keys create --name ci\n  bex s3 public photos enable",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		RunE: func(c *cobra.Command, args []string) error {
			return d.launch(c, "s3", args)
		},
	}
	s3.AddCommand(envCommand(d), keysCommand(d), publicCommand(d), infoCommand(d))

	s3api := &cobra.Command{
		Use:   "s3api <aws s3api call> [arguments…]",
		Short: "Call bex's S3 API directly with the AWS CLI (put-bucket-cors, list-objects-v2, …)",
		Long: "Runs your AWS CLI (`aws s3api …`) against bex object storage with the endpoint\n" +
			"and a key already set, exactly as `bex s3` does. Every argument goes to\n" +
			"`aws s3api` as typed.",
		Example:            "  bex s3api list-objects-v2 --bucket photos --prefix site/\n  bex s3api put-bucket-cors --bucket photos --cors-configuration file://cors.json",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		RunE: func(c *cobra.Command, args []string) error {
			return d.launch(c, "s3api", args)
		},
	}
	return []*cobra.Command{s3, s3api}
}

// usageError is a command line that is wrong: exit code 2.
func usageError(format string, args ...any) error {
	return command.NewExitError(exitUsage, fmt.Errorf(format, args...))
}

// commandContext is the command's context, never nil.
func commandContext(c *cobra.Command) context.Context {
	if ctx := c.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

// helpRequested is `bex s3` with nothing to run, or only -h/--help: bex's own
// help. Anything longer, `bex s3 ls --help` included, is the AWS CLI's.
func helpRequested(args []string) bool {
	return len(args) == 0 || (len(args) == 1 && (args[0] == "-h" || args[0] == "--help"))
}
