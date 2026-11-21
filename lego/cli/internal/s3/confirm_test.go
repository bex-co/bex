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
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/render-oss/cli/pkg/command"
	"github.com/spf13/cobra"
)

// bexRun runs `bex <args>` under a root that does what Render's root command
// does before every command: it reads the global --confirm flag and resolves
// the output format, which is interactive on a terminal and text off one. The
// answer is what a person would type at the prompt.
func (g *rig) bexRun(terminal bool, answer string, args ...string) (stdout, stderr string, err error) {
	g.t.Helper()
	g.stdin.Reset()
	g.stdin.WriteString(answer)
	root := &cobra.Command{Use: "bex", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().StringP("output", "o", "text", "")
	root.PersistentFlags().Bool(command.ConfirmFlag, false, "Skip all confirmation prompts")
	root.PersistentPreRunE = func(c *cobra.Command, _ []string) error {
		confirm, _ := c.Flags().GetBool(command.ConfirmFlag)
		format := command.TEXT
		if terminal {
			format = command.Interactive
		}
		c.SetContext(command.SetFormatInContext(command.SetConfirmInContext(commandContext(c), confirm), &format))
		return nil
	}
	root.AddCommand(newCommands(g.d)...)
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(append([]string{"s3"}, args...))
	err = root.Execute()
	return out.String(), errOut.String(), err
}

// bulkDelete is one of the three commands that delete in bulk.
type bulkDelete struct {
	name string
	// prepare stores what the command deletes and returns its arguments and the
	// question a terminal is asked.
	prepare func(t *testing.T, g *rig) (args []string, question string)
	// deleted says whether the command's deletion happened.
	deleted func(g *rig) bool
	// dryrun is the flag that deletes nothing, empty when the verb has none.
	dryrun string
}

func bulkDeletes() []bulkDelete {
	const ending = "? This cannot be undone. [y/N]: "
	return []bulkDelete{
		{
			name: "rm --recursive",
			prepare: func(t *testing.T, g *rig) ([]string, string) {
				for _, key := range []string{"t/a", "t/b", "keep"} {
					g.s3.put("photos", key, []byte("x"), testNow)
				}
				return []string{"rm", "s3://photos/t", "--recursive"}, "Are you sure you want to delete every object under 's3://photos/t/'" + ending
			},
			deleted: func(g *rig) bool { return !slices.Contains(g.s3.keys("photos"), "t/a") },
			dryrun:  "--dryrun",
		},
		{
			name: "rb --force",
			prepare: func(t *testing.T, g *rig) ([]string, string) {
				g.s3.put("photos", "a/1", []byte("x"), testNow)
				return []string{"rb", "s3://photos", "--force"}, "Are you sure you want to delete the bucket 's3://photos' and every object in it" + ending
			},
			deleted: func(g *rig) bool { _, ok := g.s3.buckets["photos"]; return !ok },
		},
		{
			name: "sync --delete",
			prepare: func(t *testing.T, g *rig) ([]string, string) {
				src := filepath.Join(t.TempDir(), "src")
				writeFile(t, src, "keep", 5)
				g.s3.put("photos", "p/extra", []byte("x"), testNow)
				return []string{"sync", src, "s3://photos/p", "--delete"}, "Are you sure you want to delete what is in 's3://photos/p/' and not in '" + src + "'" + ending
			},
			deleted: func(g *rig) bool { return !slices.Contains(g.s3.keys("photos"), "p/extra") },
			dryrun:  "--dryrun",
		},
	}
}

// TestABulkDeleteAsksOnATerminalAndWaitsForTheAnswer: on a terminal the three
// bulk deletes print `bex ea objects delete`'s question and do nothing, not
// even make a key, until it is answered; only y or yes goes on, and anything
// else, or the end of the input, cancels with exit 0.
func TestABulkDeleteAsksOnATerminalAndWaitsForTheAnswer(t *testing.T) {
	for _, verb := range bulkDeletes() {
		t.Run(verb.name, func(t *testing.T) {
			for _, answer := range []string{"y\n", "yes\n", " Y \n", "YES\n"} {
				g := newRig(t)
				args, question := verb.prepare(t, g)
				stdout, stderr, err := g.bexRun(true, answer, args...)
				if err != nil {
					t.Fatalf("answer %q: %v\n%s", answer, err, stderr)
				}
				if !strings.HasPrefix(stdout, question) {
					t.Errorf("answer %q: stdout = %q, want it to begin with the question %q", answer, stdout, question)
				}
				if strings.Contains(stdout, "canceled") || !verb.deleted(g) {
					t.Errorf("answer %q was not taken as a yes: stdout %q, deleted %v", answer, stdout, verb.deleted(g))
				}
			}

			for _, answer := range []string{"", "n\n", "no\n", "\n", "yep\n", "y", "sure\n"} {
				g := newRig(t)
				args, question := verb.prepare(t, g)
				stdout, stderr, err := g.bexRun(true, answer, args...)
				if err != nil {
					t.Fatalf("answer %q: a canceled delete must exit 0, got %v\n%s", answer, err, stderr)
				}
				if stdout != question+"Delete canceled.\n" {
					t.Errorf("answer %q: stdout = %q, want the question and %q", answer, stdout, "Delete canceled.")
				}
				if verb.deleted(g) {
					t.Errorf("answer %q deleted anyway", answer)
				}
				if n := g.api.mintCount(); n != 0 || len(g.s3.requests("")) != 0 {
					t.Errorf("answer %q: a declined delete made %d keys and %d requests, want none", answer, n, len(g.s3.requests("")))
				}
			}
		})
	}
}

// TestYesAndConfirmSkipTheQuestion: --yes, its short form -y and the global
// --confirm each delete without asking, even on a terminal with nothing typed.
func TestYesAndConfirmSkipTheQuestion(t *testing.T) {
	for _, verb := range bulkDeletes() {
		t.Run(verb.name, func(t *testing.T) {
			for _, flag := range []string{"--yes", "-y", "--confirm"} {
				g := newRig(t)
				args, _ := verb.prepare(t, g)
				stdout, stderr, err := g.bexRun(true, "", append(args, flag)...)
				if err != nil {
					t.Fatalf("%s: %v\n%s", flag, err, stderr)
				}
				if strings.Contains(stdout, "Are you sure") || strings.Contains(stdout, "canceled") {
					t.Errorf("%s still asked: %q", flag, stdout)
				}
				if !verb.deleted(g) {
					t.Errorf("%s did not delete", flag)
				}
			}
		})
	}
}

// TestABulkDeleteNeverAsksOffATerminal: a pipe, a script or CI, which the root
// resolves to a text or machine format, is never asked and deletes, as
// `bex ea objects delete` does.
func TestABulkDeleteNeverAsksOffATerminal(t *testing.T) {
	for _, verb := range bulkDeletes() {
		t.Run(verb.name, func(t *testing.T) {
			g := newRig(t)
			args, _ := verb.prepare(t, g)
			stdout, stderr, err := g.bexRun(false, "", args...)
			if err != nil {
				t.Fatalf("%v\n%s", err, stderr)
			}
			if strings.Contains(stdout, "Are you sure") || !verb.deleted(g) {
				t.Errorf("a run off a terminal asked or did not delete: stdout %q, deleted %v", stdout, verb.deleted(g))
			}
		})
	}
	// A bare command, with no root to say what the run is (the case of every other
	// test in this package), is not a terminal either.
	g := newRig(t)
	args, _ := bulkDeletes()[0].prepare(t, g)
	g.must(args...)
	if !bulkDeletes()[0].deleted(g) {
		t.Error("a command run without a root did not delete")
	}
}

// TestADryrunAndAnUnforcedRemovalNeverAsk: asking is for a delete that is about
// to happen in bulk. --dryrun deletes nothing, and `rb` without --force removes
// an empty bucket and nothing else.
func TestADryrunAndAnUnforcedRemovalNeverAsk(t *testing.T) {
	for _, verb := range bulkDeletes() {
		if verb.dryrun == "" {
			continue
		}
		t.Run(verb.name+" "+verb.dryrun, func(t *testing.T) {
			g := newRig(t)
			args, _ := verb.prepare(t, g)
			stdout, stderr, err := g.bexRun(true, "", append(args, verb.dryrun)...)
			if err != nil {
				t.Fatalf("%v\n%s", err, stderr)
			}
			if strings.Contains(stdout, "Are you sure") || verb.deleted(g) || !strings.Contains(stdout, "(dryrun) delete: ") {
				t.Errorf("a dry run asked or deleted: stdout %q, deleted %v", stdout, verb.deleted(g))
			}
		})
	}
	g := newRig(t)
	g.s3.bucket("empty")
	stdout, stderr, err := g.bexRun(true, "", "rb", "s3://empty")
	if err != nil || strings.Contains(stdout, "Are you sure") || !strings.Contains(stdout, "remove_bucket: empty") {
		t.Errorf("rb of an empty bucket: stdout %q, stderr %q, err %v", stdout, stderr, err)
	}
}

// TestTheQuestionOfARecursiveRemoveNamesItsFilters: with --include or --exclude
// the question says that only some of the objects under the prefix go.
func TestTheQuestionOfARecursiveRemoveNamesItsFilters(t *testing.T) {
	g := newRig(t)
	g.s3.put("photos", "t/a.tmp", []byte("x"), testNow)
	g.s3.put("photos", "t/b.jpg", []byte("x"), testNow)
	stdout, _, err := g.bexRun(true, "y\n", "rm", "s3://photos/t", "--recursive", "--exclude", "*", "--include", "*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	want := "Are you sure you want to delete the objects under 's3://photos/t/' that --include and --exclude pick? This cannot be undone. [y/N]: "
	if !strings.HasPrefix(stdout, want) {
		t.Errorf("stdout = %q, want it to begin with %q", stdout, want)
	}
	if got := g.s3.keys("photos"); !slices.Equal(got, []string{"t/b.jpg"}) {
		t.Errorf("keys left = %v, want only the excluded one kept", got)
	}
}
