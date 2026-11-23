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

// The confirmation of `bex s3`'s deletes (w21/115 for the bulk ones, w21/116 for
// one object), against the built binary and a real pseudo-terminal, so the root
// command's own reading of "this is a terminal" (stdin, stdout and stderr all
// terminals, TERM not dumb, no CI, no -o) decides, exactly as it does for
// `bex ea objects delete`, whose prompt is the reference.

import (
	"bytes"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

const promptEnding = "? This cannot be undone. [y/N]: "

// objectQuestion is `bex ea objects delete`'s question about one object, with
// the object it names filled in. The reference subtest below pins it to what the
// pinned `ea objects delete` prints, and `bex s3 rm` of one object asks it.
func objectQuestion(object string) string {
	return "Are you sure you want to delete object '" + object + "'" + promptEnding
}

// runAtPrompt runs command with a pseudo-terminal for each of stdin, stdout and
// stderr that the caller left nil, types answer when the prompt shows, and
// returns what the terminal displayed. A command that never prompts is not typed
// at and just finishes.
func runAtPrompt(t *testing.T, command *exec.Cmd, answer string) (transcript string, err error) {
	t.Helper()
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("open a pseudo-terminal: %v", err)
	}
	defer func() { _ = ptmx.Close() }()
	if command.Stdin == nil {
		command.Stdin = tty
	}
	if command.Stdout == nil {
		command.Stdout = tty
	}
	if command.Stderr == nil {
		command.Stderr = tty
	}
	if err := command.Start(); err != nil {
		_ = tty.Close()
		t.Fatal(err)
	}
	// Only the child holds the terminal now, so the reader below ends (EIO on
	// Linux) when it exits.
	_ = tty.Close()

	var (
		mu       sync.Mutex
		captured bytes.Buffer
	)
	copied := make(chan struct{})
	go func() {
		defer close(copied)
		chunk := make([]byte, 4096)
		for {
			n, readErr := ptmx.Read(chunk)
			mu.Lock()
			captured.Write(chunk[:n])
			mu.Unlock()
			if readErr != nil {
				return
			}
		}
	}()
	shown := func() string {
		mu.Lock()
		defer mu.Unlock()
		return captured.String()
	}

	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	deadline := time.After(20 * time.Second)
	typed := false
	for {
		select {
		case err = <-exited:
			select {
			case <-copied:
			case <-time.After(5 * time.Second):
				t.Fatal("terminal output did not drain")
			}
			return strings.ReplaceAll(shown(), "\r\n", "\n"), err
		case <-deadline:
			_ = command.Process.Kill()
			t.Fatalf("bex did not finish; the terminal showed %q", shown())
		case <-time.After(20 * time.Millisecond):
			if !typed && strings.Contains(shown(), "[y/N]: ") {
				typed = true
				if _, werr := io.WriteString(ptmx, answer); werr != nil {
					t.Fatal(werr)
				}
			}
		}
	}
}

// deleteRun is one `bex s3` command that deletes and asks first, against the
// fake S3 of s3_contract_test.go (an empty bucket), with its question.
type deleteRun struct {
	name     string
	args     func(dir string) []string
	question func(dir string) string
	// dryrun is the flag that makes it delete nothing; empty when it has none.
	dryrun string
}

var deleteRuns = []deleteRun{
	{
		name:     "rm",
		args:     func(string) []string { return []string{"s3", "rm", "s3://launch-contract-bucket/p/one.txt"} },
		question: func(string) string { return objectQuestion("s3://launch-contract-bucket/p/one.txt") },
		dryrun:   "--dryrun",
	},
	{
		name: "rm --recursive",
		args: func(string) []string { return []string{"s3", "rm", "s3://launch-contract-bucket/p", "--recursive"} },
		question: func(string) string {
			return "Are you sure you want to delete every object under 's3://launch-contract-bucket/p/'" + promptEnding
		},
		dryrun: "--dryrun",
	},
	{
		name: "rb --force",
		args: func(string) []string { return []string{"s3", "rb", "s3://launch-contract-bucket", "--force"} },
		question: func(string) string {
			return "Are you sure you want to delete the bucket 's3://launch-contract-bucket' and every object in it" + promptEnding
		},
	},
	{
		name: "sync --delete",
		args: func(dir string) []string {
			return []string{"s3", "sync", dir, "s3://launch-contract-bucket/p", "--delete"}
		},
		question: func(dir string) string {
			return "Are you sure you want to delete what is in 's3://launch-contract-bucket/p/' and not in '" + dir + "'" + promptEnding
		},
		dryrun: "--dryrun",
	},
}

// s3Terminal is a `bex s3` run ready to be put on a terminal, with the fake
// endpoint and bex-api it talks to.
type s3Terminal struct {
	command  *exec.Cmd
	endpoint *fakeS3Endpoint
	api      *telemetryStub
	dir      string
}

func newS3Terminal(t *testing.T, args func(dir string) []string, extraEnv ...string) *s3Terminal {
	t.Helper()
	stub := &telemetryStub{routes: s3MintRoutes()}
	api := httptest.NewServer(stub.handler())
	t.Cleanup(api.Close)
	endpoint := newFakeS3Endpoint(t)
	home := t.TempDir()
	seedNoticeMarker(t, home)
	dir := t.TempDir()
	command := exec.Command(buildBex(), args(dir)...)
	command.Env = s3QuietEnv(t, home, api.URL, endpoint.srv.URL, append([]string{"TERM=xterm"}, extraEnv...)...)
	return &s3Terminal{command: command, endpoint: endpoint, api: stub, dir: dir}
}

// touched is whether the command reached the endpoint or asked bex-api for a key.
func (s *s3Terminal) touched() bool {
	_, calls := s.endpoint.seen()
	return len(calls) > 0 || s.api.askedForAKey()
}

// askedForAKey is whether a request for an S3 key reached the stub bex-api.
func (s *telemetryStub) askedForAKey() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.ContainsFunc(s.headers, func(h string) bool { return strings.Contains(h, "/access-keys") })
}

// TestBexS3DeletesAskOnATerminalLikeEaObjectsDelete: the question has the words
// of `bex ea objects delete`, waits for the answer before any request is made,
// goes on only for y or yes, and a "no" ends with "Delete canceled." and exit 0.
func TestBexS3DeletesAskOnATerminalLikeEaObjectsDelete(t *testing.T) {
	t.Parallel()
	t.Run("the reference", func(t *testing.T) {
		t.Parallel()
		api := newObjectsAPI(t)
		command := bexAgainst(t, api.srv.URL, "ea", "objects", "delete", "a/b", "--region", "oregon")
		home := homeOf(command.Env)
		seedNoticeMarker(t, home)
		command.Env = append(command.Env, "TERM=xterm", "BEX_CLI_DISABLE_ANALYTICS=1")
		transcript, err := runAtPrompt(t, command, "n\n")
		if err != nil {
			t.Fatalf("ea objects delete answered n: %v\n%s", err, transcript)
		}
		if want := objectQuestion("a/b") + "n\nDelete canceled.\n"; transcript != want {
			t.Fatalf("ea objects delete showed %q, want %q (the reference these prompts follow changed)", transcript, want)
		}
	})
	for _, verb := range deleteRuns {
		t.Run(verb.name, func(t *testing.T) {
			t.Parallel()
			for answer, goesOn := range map[string]bool{"n\n": false, "\n": false, "y\n": true, "yes\n": true} {
				s := newS3Terminal(t, verb.args)
				transcript, err := runAtPrompt(t, s.command, answer)
				if err != nil {
					t.Fatalf("answer %q: %v\n%s", answer, err, transcript)
				}
				question := verb.question(s.dir)
				if !strings.HasPrefix(transcript, question+strings.TrimSuffix(answer, "\n")+"\n") {
					t.Errorf("answer %q: the terminal showed %q, want the question %q and the answer", answer, transcript, question)
				}
				if goesOn == strings.Contains(transcript, "Delete canceled.") {
					t.Errorf("answer %q: the terminal showed %q, want the delete to go on: %v", answer, transcript, goesOn)
				}
				if goesOn != s.touched() {
					t.Errorf("answer %q: touched the endpoint or asked for a key = %v, want %v", answer, s.touched(), goesOn)
				}
			}
		})
	}
}

// TestBexS3DeletesSkipTheQuestionWithYesOrConfirmAndNeverAskElsewhere: --yes,
// -y and the global --confirm skip it on a terminal, and so does everything the
// root does not read as a terminal: -o json, CI, and a pipe for any of the three
// streams. A dry run deletes nothing and is never asked.
func TestBexS3DeletesSkipTheQuestionWithYesOrConfirmAndNeverAskElsewhere(t *testing.T) {
	t.Parallel()
	for _, verb := range deleteRuns {
		t.Run(verb.name, func(t *testing.T) {
			t.Parallel()
			type variant struct {
				name  string
				extra []string // arguments added to the command
				env   []string
			}
			variants := []variant{
				{name: "--yes", extra: []string{"--yes"}},
				{name: "-y", extra: []string{"-y"}},
				{name: "--confirm", extra: []string{"--confirm"}},
				{name: "-o json", extra: []string{"-o", "json"}},
				{name: "CI", env: []string{"CI=true"}},
				{name: "RENDER_OUTPUT=text", env: []string{"RENDER_OUTPUT=text"}},
				{name: "TERM=dumb", env: []string{"TERM=dumb"}},
			}
			if verb.dryrun != "" {
				variants = append(variants, variant{name: verb.dryrun, extra: []string{verb.dryrun}})
			}
			for _, v := range variants {
				s := newS3Terminal(t, func(dir string) []string { return append(verb.args(dir), v.extra...) }, v.env...)
				transcript, err := runAtPrompt(t, s.command, "")
				if err != nil {
					t.Fatalf("%s: %v\n%s", v.name, err, transcript)
				}
				if strings.Contains(transcript, "Are you sure") || strings.Contains(transcript, "canceled") {
					t.Errorf("%s asked on a terminal: %q", v.name, transcript)
				}
				if !s.touched() {
					t.Errorf("%s did not go on", v.name)
				}
			}

			// Standard input, output or error not a terminal: the three streams all
			// have to be one, as for `ea objects delete`.
			for _, pipe := range []string{"stdin", "stdout", "stderr"} {
				s := newS3Terminal(t, verb.args)
				var out bytes.Buffer
				switch pipe {
				case "stdin":
					s.command.Stdin = strings.NewReader("")
					s.command.Stdout, s.command.Stderr = &out, &out
				case "stdout":
					s.command.Stdout = &out
				case "stderr":
					s.command.Stderr = &out
				}
				answer, err := runAtPrompt(t, s.command, "")
				if err != nil {
					t.Fatalf("%s piped: %v\n%s%s", pipe, err, answer, out.String())
				}
				if strings.Contains(answer+out.String(), "Are you sure") || !s.touched() {
					t.Errorf("a run with %s piped asked or did not go on: %q %q", pipe, answer, out.String())
				}
			}
		})
	}
}

// TestBexS3WithoutAnyTerminalNeverAsks is a script's run: no terminal on any
// stream, nothing typed, and the delete just happens.
func TestBexS3WithoutAnyTerminalNeverAsks(t *testing.T) {
	t.Parallel()
	for _, verb := range deleteRuns {
		t.Run(verb.name, func(t *testing.T) {
			t.Parallel()
			s := newS3Terminal(t, verb.args)
			s.command.Stdin = strings.NewReader("")
			output, err := s.command.CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s", err, output)
			}
			if strings.Contains(string(output), "Are you sure") || !s.touched() {
				t.Errorf("a script's run asked or did not go on: %q", output)
			}
		})
	}
}

// homeOf is the HOME of an environment.
func homeOf(env []string) string {
	for _, item := range slices.Backward(env) {
		if home, ok := strings.CutPrefix(item, "HOME="); ok {
			return home
		}
	}
	return filepath.Join(os.TempDir(), "no-home")
}
