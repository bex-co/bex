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

// Package s3 is `bex s3`, a native client of bex's S3-compatible object
// storage API (ADR112 D3): `ls`, `cp`, `mv`, `rm`, `sync`, `mb`, `rb` and
// `presign`, bex's own `cors`, `lifecycle` and `info`, and the commands around
// them, `env`, `keys` and `public`. It signs and sends its own requests with
// the AWS SDK for Go v2, with a key minted for the signed-in person, and runs
// no other program. It is a client of the API alone: quota, metering and
// authorization stay with the server. It imports nothing from the other
// modules, so the REST shapes of the bucket and key routes are written from
// the routes' contract, not shared with them.
package s3

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/config"
	"github.com/spf13/cobra"

	"github.com/bex-co/bex/lego/cli/internal/usage"
)

// deps is the process boundary, injectable for tests.
type deps struct {
	api       func() (*api, error)
	workspace func() (string, error)
	host      func() string
	lookupEnv func(string) (string, bool)
	state     func() (string, error)
	now       func() time.Time
	hostname  func() string
	// httpClient is what S3 requests are sent with; nil is the SDK's default.
	httpClient func() aws.HTTPClient
	stdin      io.Reader
	// isTerminal says whether the progress line may be drawn on a stream.
	isTerminal func(io.Writer) bool
}

func osDeps() deps {
	return deps{
		api:       defaultAPI,
		workspace: config.WorkspaceID,
		host:      cfg.GetHost,
		lookupEnv: os.LookupEnv,
		state:     config.StateDir,
		now:       time.Now,
		hostname: func() string {
			name, _ := os.Hostname()
			return name
		},
		stdin:      os.Stdin,
		isTerminal: isTerminal,
	}
}

// globals are the flags every `bex s3` command takes.
type globals struct {
	// region is the signing region, and the location `mb` creates a bucket in.
	region string
}

// Commands is `bex s3`.
func Commands() []*cobra.Command {
	return newCommands(osDeps())
}

func newCommands(d deps) []*cobra.Command {
	g := &globals{}
	s3 := &cobra.Command{
		Use:   "s3",
		Short: "Use bex object storage: buckets, objects, sync, presigned URLs, CORS and lifecycle rules",
		Long: "Object storage on bex's S3-compatible API, from your terminal, with nothing else\n" +
			"to install. Sign in with `bex login`: a key is made for you, lasts twelve hours\n" +
			"and is replaced when under one hour is left.\n\n" +
			"Objects:\n" +
			"  ls [s3://bucket[/prefix]]    list buckets, or the objects under a prefix\n" +
			"  cp <from> <to>               copy a file, a directory (--recursive) or an object\n" +
			"  mv <from> <to>               copy, then delete the source\n" +
			"  rm s3://bucket/key           delete an object, or a prefix with --recursive\n" +
			"  sync <from> <to>             copy what is missing or changed between a directory and a prefix\n" +
			"  presign s3://bucket/key      a URL that reads an object without a key\n\n" +
			"Buckets:\n" +
			"  mb s3://bucket               make a bucket (--region oregon|frankfurt)\n" +
			"  rb s3://bucket               remove an empty bucket (--force removes its objects first)\n" +
			"  cors get|set|delete          a bucket's CORS rules\n" +
			"  lifecycle get|set|delete     a bucket's expiry rules\n" +
			"  public <bucket> enable|disable|status   publish a bucket read-only (workspace admin)\n" +
			"  info <bucket>|s3://bucket/key   a bucket's region and size, or an object's metadata\n\n" +
			"Keys and other tools:\n" +
			"  env                          the endpoint and a key, as variables for another S3 client\n" +
			"  keys create|list|revoke      persistent keys for CI and long transfers\n\n" +
			"A delete asks first. `rm`, `rb --force` and `sync --delete` ask \"Are you sure?\" in a\n" +
			"terminal, as `bex ea objects delete` does; --yes (-y) or the global --confirm skips\n" +
			"the question, and a pipe, a script or CI is never asked. `mv` does not ask.\n\n" +
			"A command given an access key in BEX_S3_ACCESS_KEY_ID and BEX_S3_SECRET_ACCESS_KEY\n" +
			"uses it instead of making one, which is how CI, with no sign-in, uses `bex s3`.\n" +
			"An object is at most 5 GiB. A key that expires during a transfer fails the transfer:\n" +
			"use a persistent key (`bex s3 keys create`) for a transfer over hours.",
		Example: "  bex s3 mb s3://photos --region frankfurt\n  bex s3 cp cat.jpg s3://photos/cat.jpg\n  bex s3 sync ./site s3://photos/site\n  bex s3 ls s3://photos/",
		Args: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				return nil
			}
			return usageError("unknown command %q for %q: `bex s3 --help` lists the commands, and `bex s3 env` prints the endpoint and a key for another S3 client", args[0], c.CommandPath())
		},
		SilenceUsage: true,
		RunE: func(c *cobra.Command, _ []string) error {
			return c.Help()
		},
	}
	s3.PersistentFlags().StringVar(&g.region, "region", "", "the region requests are signed for, and where mb makes a bucket (default "+defaultRegion+")")
	s3.AddCommand(
		lsCommand(d, g), cpCommand(d, g), mvCommand(d, g), rmCommand(d, g), syncCommand(d, g), presignCommand(d, g),
		mbCommand(d, g), rbCommand(d, g), corsCommand(d, g), lifecycleCommand(d, g),
		envCommand(d, g), keysCommand(d), publicCommand(d), infoCommand(d, g),
	)
	return []*cobra.Command{s3}
}

// usageError is a command line that is wrong: exit code 2.
func usageError(format string, args ...any) error {
	return usage.Errorf(format, args...)
}

// commandContext is the command's context, never nil.
func commandContext(c *cobra.Command) context.Context {
	if ctx := c.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

// run is one command's connection to object storage: a key, an S3 client that
// holds at most `--concurrency` requests in flight, and where results go.
type run struct {
	d      deps
	ctx    context.Context
	cancel context.CancelFunc
	sess   session
	client *sdk.Client
	format format
	out    io.Writer
	errOut io.Writer
	rep    *reporter
	prog   *progress
	// concurrency is the most requests in flight, and the most files and parts
	// a transfer works on at once.
	concurrency int
}

// open resolves the output format and a key and builds the client. Callers
// check their arguments first, so a malformed command line never costs a key.
// An interrupt cancels the command's context, which unwinds its workers and
// removes the temporary files of a download.
func (d deps) open(c *cobra.Command, g *globals, concurrency int, noProgress bool) (*run, error) {
	f, err := resolveFormat(c)
	if err != nil {
		return nil, err
	}
	ctx, cancel := signal.NotifyContext(commandContext(c), os.Interrupt, syscall.SIGTERM)
	sess, err := d.session(ctx, g.region)
	if err != nil {
		cancel()
		return nil, err
	}
	out, errOut := c.OutOrStdout(), c.ErrOrStderr()
	prog := newProgress(errOut, f == formatText && !noProgress && d.isTerminal != nil && d.isTerminal(errOut))
	r := &run{
		d: d, ctx: ctx, cancel: cancel, sess: sess, client: d.newClient(sess, concurrency), concurrency: concurrency,
		format: f, out: out, errOut: errOut, prog: prog,
		rep: &reporter{out: out, format: f, prog: prog},
	}
	prog.start()
	return r, nil
}

// close ends the command's progress line and its context.
func (r *run) close() {
	r.prog.stop()
	r.cancel()
}

// fail is err in bex's words. A key the server no longer knows is forgotten,
// so the next command makes a new one instead of meeting the same refusal.
func (r *run) fail(err error) error {
	if err == nil {
		return nil
	}
	if f, ok := faultOf(err); ok && keyGone(f) && r.sess.CachePath != "" {
		_ = os.Remove(r.sess.CachePath)
	}
	return explain(err)
}

// concurrencyFlag adds --concurrency to a command.
func concurrencyFlag(c *cobra.Command, n *int) {
	c.Flags().IntVar(n, "concurrency", defaultConcurrency, fmt.Sprintf("requests in flight at once, at most %d; the gateway admits 16 streams per workspace", maxConcurrency))
}

// checkConcurrency refuses a --concurrency outside 1 to the maximum.
func checkConcurrency(n int) error {
	if n < 1 || n > maxConcurrency {
		return usageError("--concurrency must be between 1 and %d, not %d", maxConcurrency, n)
	}
	return nil
}
