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

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// runBexCode runs the launcher against an API that is not there and returns
// its exit code. None of these command lines reaches the API.
func runBexCode(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	stdout, stderr, err := runBexAgainst(t, "http://127.0.0.1:1", args...)
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), stdout, stderr
	}
	if err != nil {
		t.Fatalf("bex %v: %v", args, err)
	}
	return 0, stdout, stderr
}

// w8/368: a wrong command line of a bex-native command exits 2 through
// upstream's executor, as docs/bex-cli.md says, with nothing on stdout.
func TestBexNativeUsageErrorsExit2(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args string
		want []string
	}{
		{"agent wiat ags-x", []string{`unknown command "wiat" for "bex agent"`, "Did you mean this?", "\twait\n"}},
		{"agents frob", []string{`unknown command "frob" for "bex agent"`}},
		{"agent show ags-d0000000000000000000", []string{`unknown command "show" for "bex agent"`}},
		{"agent secrets frob", []string{`unknown command "frob" for "bex agent secrets"`}},
		{"agent skills frob", []string{`unknown command "frob" for "bex agent skills"`}},
		{"agent automations frob", []string{`unknown command "frob" for "bex agent automations"`}},
		{"code frob", []string{`unknown command "frob" for "bex code"`}},
		{"code keys frob", []string{`unknown command "frob" for "bex code keys"`}},
		{"s3 keys frob", []string{`unknown command "frob" for "bex s3 keys"`}},
		// cobra's own checks, which exit 1 by themselves
		{"agent env frob", []string{`unknown command "frob" for "bex agent env"`}},
		{"s3 mb", []string{"accepts 1 arg(s), received 0"}},
		{"s3 mb a b c", []string{"accepts 1 arg(s), received 3"}},
		{"s3 ls --qa-bogus", []string{"unknown flag: --qa-bogus"}},
		// a key bex object storage cannot serve (w32/063)
		{"s3 cp a.txt s3://photos/x/../y", []string{`"s3://photos/x/../y" cannot name an object or a prefix: the key "x/../y" has a ".." segment`}},
		{"s3 cp a.txt s3://photos//dbl", []string{`the key "/dbl" starts with a slash`, "no leading or doubled slash"}},
		{"upgrade frob", []string{`unknown command "frob" for "bex upgrade"`}},
	} {
		t.Run(tc.args, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := runBexCode(t, strings.Fields(tc.args)...)
			if code != 2 || stdout != "" {
				t.Fatalf("exit %d, want 2 with an empty stdout\nstdout %s\nstderr %s", code, stdout, stderr)
			}
			for _, want := range tc.want {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr lacks %q:\n%s", want, stderr)
				}
			}
		})
	}
}

// Without a word a group prints its help as before, still a group that
// needs a subcommand; `bex code` still prints the providers' status.
func TestBexNativeGroupsWithoutAWordAreUnchanged(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args string
		want []string
	}{
		{"agent", []string{"bex agent <subcommand> [flags]", `Use "bex agent <subcommand> --help"`}},
		{"agents", []string{"bex agent <subcommand> [flags]"}},
		{"agent secrets", []string{"bex agent secrets <subcommand> [flags]"}},
		{"s3 keys", []string{"bex s3 keys <subcommand> [flags]"}},
		{"code", []string{"bex glm", "bex code keys set glm"}},
	} {
		t.Run(tc.args, func(t *testing.T) {
			t.Parallel()
			code, stdout, stderr := runBexCode(t, strings.Fields(tc.args)...)
			if code != 0 || stderr != "" {
				t.Fatalf("exit %d, want 0 with an empty stderr\nstderr %s", code, stderr)
			}
			for _, want := range tc.want {
				if !strings.Contains(stdout, want) {
					t.Errorf("stdout lacks %q:\n%s", want, stdout)
				}
			}
		})
	}
}

// The imported Render commands, and `bex skills`, which takes the place of
// Render's, keep the pinned Render CLI's answers: 0 with the group's help for
// an unknown word after a group cobra cannot run, 1 for a refused argument or
// flag.
func TestImportedCommandsKeepRenderUsageExitCodes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args string
		code int
	}{
		{"workspace frob", 0},
		{"pg frob", 0},
		{"ea frob", 1},
		{"services --qa-bogus", 1},
		{"pg get", 1},
		{"skills frob", 1},
		{"skills install --qa-bogus", 1},
	} {
		t.Run(tc.args, func(t *testing.T) {
			t.Parallel()
			if code, stdout, stderr := runBexCode(t, strings.Fields(tc.args)...); code != tc.code {
				t.Errorf("exit %d, want %d\nstdout %s\nstderr %s", code, tc.code, stdout, stderr)
			}
		})
	}
}
