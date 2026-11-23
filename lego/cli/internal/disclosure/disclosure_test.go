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

package disclosure

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/render-oss/cli/pkg/analytics"
	"github.com/spf13/cobra"
)

// cleanEnv points the state directory at a fresh temp dir and clears every
// input the gate reads, so a developer's or runner's CI/DO_NOT_TRACK cannot
// decide a case. It returns the analytics marker directory.
func cleanEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", dir)
	for _, key := range []string{"CI", "DO_NOT_TRACK", renderDisableAnalytics, "RENDER_CLI_ANALYTICS_SUBPROCESS"} {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "state", "analytics")
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}

func seed(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBeginGatesAnUndisclosedRunAndReleaseRestoresTheEnvironment(t *testing.T) {
	markers := cleanEnv(t)

	gate := Begin(nil)
	if got := os.Getenv(renderDisableAnalytics); got != "1" {
		t.Fatalf("%s = %q while undisclosed, want 1 so upstream's sender is built opted out", renderDisableAnalytics, got)
	}
	if analytics.ResolveConsent().Granted {
		t.Fatal("upstream consent granted on an undisclosed run")
	}
	if !exists(t, filepath.Join(markers, upstreamMarker)) {
		t.Error("upstream's marker was not written, so upstream would print Render's notice")
	}
	if exists(t, filepath.Join(markers, bexMarker)) {
		t.Error("Begin wrote the Bex marker; only a shown notice may")
	}
	if gate.optOut != "" {
		t.Errorf("optOut = %q for a consenting user, want empty", gate.optOut)
	}

	gate.Release()
	if value, set := os.LookupEnv(renderDisableAnalytics); set {
		t.Errorf("%s = %q after Release, want unset again for child processes", renderDisableAnalytics, value)
	}
	gate.Release() // idempotent
}

func TestReleaseRestoresTheUsersOwnNonOptOutValue(t *testing.T) {
	cleanEnv(t)
	// "0" is not an opt-out upstream, so the gate must override it, and hand
	// it back unchanged afterwards.
	t.Setenv(renderDisableAnalytics, "0")

	gate := Begin(nil)
	if got := os.Getenv(renderDisableAnalytics); got != "1" {
		t.Fatalf("%s = %q, want 1", renderDisableAnalytics, got)
	}
	gate.Release()
	if got := os.Getenv(renderDisableAnalytics); got != "0" {
		t.Errorf("%s = %q after Release, want the user's 0", renderDisableAnalytics, got)
	}
}

func TestBeginLeavesTheSenderAloneWhenTheGateIsOpen(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, markers string)
	}{
		{"CI=true bypasses the gate", func(t *testing.T, _ string) { t.Setenv("CI", "true") }},
		{"CI=1 bypasses the gate", func(t *testing.T, _ string) { t.Setenv("CI", "1") }},
		{"the Bex notice was shown", func(t *testing.T, markers string) { seed(t, markers, bexMarker) }},
		{"an opted-out user sends nothing anyway", func(t *testing.T, _ string) { t.Setenv("DO_NOT_TRACK", "1") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			markers := cleanEnv(t)
			tc.setup(t, markers)

			gate := Begin(nil)
			if value, set := os.LookupEnv(renderDisableAnalytics); set {
				t.Errorf("%s = %q, want untouched", renderDisableAnalytics, value)
			}
			if gate.restore != nil {
				t.Error("Begin recorded an override it did not need")
			}
			if !exists(t, filepath.Join(markers, upstreamMarker)) {
				t.Error("upstream's marker missing; it is written on every run")
			}
		})
	}
}

// The send subprocess is started by a parent that passed the gate and wrote
// upstream's marker, and it outlives that parent: a caller that removed its
// home once the command returned (a test's temp dir, a script's) must not see
// the child write the marker back and recreate the state directory.
func TestBeginInTheSendSubprocessWritesNothing(t *testing.T) {
	markers := cleanEnv(t)
	t.Setenv("RENDER_CLI_ANALYTICS_SUBPROCESS", "1")

	gate := Begin(nil)
	if value, set := os.LookupEnv(renderDisableAnalytics); set {
		t.Errorf("%s = %q, want untouched", renderDisableAnalytics, value)
	}
	if gate.restore != nil {
		t.Error("Begin recorded an override it did not need")
	}
	if exists(t, filepath.Dir(markers)) {
		t.Error("the send subprocess created the state directory")
	}
}

func TestCIWritesNoBexMarker(t *testing.T) {
	markers := cleanEnv(t)
	t.Setenv("CI", "true")

	gate := Begin(nil)
	var stderr bytes.Buffer
	gate.ShowIfNeeded(tree(), []string{"services"}, &stderr, true)
	if stderr.Len() != 0 {
		t.Errorf("CI printed the notice:\n%s", stderr.String())
	}
	if exists(t, filepath.Join(markers, bexMarker)) {
		t.Error("CI wrote the Bex marker")
	}
	if !CanSend() {
		t.Error("CanSend = false in CI")
	}
}

func TestOptOutNamesTheVariableTheUserSet(t *testing.T) {
	for _, tc := range []struct {
		name   string
		env    map[string]string
		filled map[string]string
		want   string
	}{
		{"consenting", nil, nil, ""},
		{"DO_NOT_TRACK", map[string]string{"DO_NOT_TRACK": "1"}, nil, "DO_NOT_TRACK"},
		{"DO_NOT_TRACK outranks the others, as upstream resolves it",
			map[string]string{"DO_NOT_TRACK": "true", renderDisableAnalytics: "1"},
			map[string]string{renderDisableAnalytics: "BEX_CLI_DISABLE_ANALYTICS"}, "DO_NOT_TRACK"},
		{"BEX_CLI_DISABLE_ANALYTICS, which the bridge mapped",
			map[string]string{renderDisableAnalytics: "1"},
			map[string]string{renderDisableAnalytics: "BEX_CLI_DISABLE_ANALYTICS"}, "BEX_CLI_DISABLE_ANALYTICS"},
		{"the user's own RENDER_CLI_DISABLE_ANALYTICS", map[string]string{renderDisableAnalytics: "1"}, nil, renderDisableAnalytics},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cleanEnv(t)
			for key, value := range tc.env {
				t.Setenv(key, value)
			}
			gate := Begin(tc.filled)
			defer gate.Release()
			if gate.optOut != tc.want {
				t.Errorf("optOut = %q, want %q", gate.optOut, tc.want)
			}
		})
	}
}

func TestNoticeText(t *testing.T) {
	consenting := strings.Join(lines(""), "\n")
	for _, want := range []string{
		"The bex CLI collects usage data",
		"The data goes to Bex, not to Render.",
		"never arguments or values",
		"No data has been sent yet.",
		"BEX_CLI_DISABLE_ANALYTICS=1 or DO_NOT_TRACK=1",
		docsURL,
	} {
		if !strings.Contains(consenting, want) {
			t.Errorf("notice missing %q:\n%s", want, consenting)
		}
	}
	optedOut := strings.Join(lines("BEX_CLI_DISABLE_ANALYTICS"), "\n")
	for _, want := range []string{
		"The data goes to Bex, not to Render.",
		"Telemetry is disabled, so nothing is sent.",
		"To enable telemetry, remove BEX_CLI_DISABLE_ANALYTICS from your environment.",
		docsURL,
	} {
		if !strings.Contains(optedOut, want) {
			t.Errorf("opted-out notice missing %q:\n%s", want, optedOut)
		}
	}
	for _, text := range []string{consenting, optedOut} {
		if strings.Contains(text, "render.com") || strings.Contains(text, "The Render CLI") {
			t.Errorf("notice still carries Render's copy:\n%s", text)
		}
	}
	if strings.Contains(optedOut, "No data has been sent yet.") {
		t.Error("the opted-out variant claims a pending send")
	}
}

// tree is a command tree shaped like the one upstream's setup leaves: the
// hidden `analytics` group carries upstream's notice-ineligible annotation.
func tree() *cobra.Command {
	run := func(*cobra.Command, []string) {}
	root := &cobra.Command{Use: "bex"}
	group := &cobra.Command{Use: "analytics", Hidden: true,
		Annotations: map[string]string{noticeEligibilityAnnotation: ineligible}}
	group.AddCommand(
		&cobra.Command{Use: "notice", RunE: func(*cobra.Command, []string) error { return nil }},
		&cobra.Command{Use: "send", Run: run},
	)
	services := &cobra.Command{Use: "services", Run: run}
	services.AddCommand(&cobra.Command{Use: "list", Run: run})
	completion := &cobra.Command{Use: "completion", Run: run}
	completion.AddCommand(&cobra.Command{Use: "zsh", Run: run})
	root.AddCommand(group, services, completion,
		&cobra.Command{Use: cobra.ShellCompRequestCmd, Aliases: []string{cobra.ShellCompNoDescRequestCmd}, Run: run},
		&cobra.Command{Use: "login", Run: run})
	return root
}

func TestShowIfNeededFollowsUpstreamsRule(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		terminal bool
		want     bool
	}{
		{"a command on a terminal", []string{"services", "list"}, true, true},
		{"login draws it though it never sends", []string{"login"}, true, true},
		{"an unknown command still ran the root", []string{"nope"}, true, true},
		{"no terminal on stderr", []string{"services"}, false, false},
		{"analytics send", []string{"analytics", "send", "file"}, true, false},
		{"analytics notice", []string{"analytics", "notice"}, true, false},
		{"completion script", []string{"completion", "zsh"}, true, false},
		{"__complete", []string{cobra.ShellCompRequestCmd, "services", ""}, true, false},
		{"__completeNoDesc", []string{cobra.ShellCompNoDescRequestCmd, "services", ""}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			markers := cleanEnv(t)
			gate := Begin(nil)
			var stderr bytes.Buffer
			gate.ShowIfNeeded(tree(), tc.args, &stderr, tc.terminal)
			if shown := strings.Contains(stderr.String(), recipient); shown != tc.want {
				t.Fatalf("shown = %v, want %v:\n%s", shown, tc.want, stderr.String())
			}
			if marked := exists(t, filepath.Join(markers, bexMarker)); marked != tc.want {
				t.Errorf("Bex marker written = %v, want %v", marked, tc.want)
			}
		})
	}
}

func TestShowIfNeededPrintsOncePerMachine(t *testing.T) {
	markers := cleanEnv(t)
	var first, second bytes.Buffer
	gate := Begin(nil)
	gate.Release()
	gate.ShowIfNeeded(tree(), []string{"services"}, &first, true)
	if strings.Count(first.String(), recipient) != 1 {
		t.Fatalf("first run printed:\n%s", first.String())
	}
	if !CanSend() || !exists(t, filepath.Join(markers, upstreamMarker)) {
		t.Fatal("the shown notice was not recorded, or upstream's marker is missing")
	}

	gate = Begin(nil)
	if _, set := os.LookupEnv(renderDisableAnalytics); set {
		t.Error("the run after the notice is still gated")
	}
	gate.ShowIfNeeded(tree(), []string{"services"}, &second, true)
	if second.Len() != 0 {
		t.Errorf("second run printed again:\n%s", second.String())
	}
}

func TestShowIfNeededSkipsAFailedSetup(t *testing.T) {
	cleanEnv(t)
	// Upstream's setup adds `analytics` once it has an API client; without it
	// upstream's post-run hook, where its notice lives, never runs.
	root := &cobra.Command{Use: "bex"}
	root.AddCommand(&cobra.Command{Use: "services", Run: func(*cobra.Command, []string) {}})
	var stderr bytes.Buffer
	Begin(nil).ShowIfNeeded(root, []string{"services"}, &stderr, true)
	if stderr.Len() != 0 || CanSend() {
		t.Errorf("a failed setup drew the notice:\n%s", stderr.String())
	}
}

func TestShowIfNeededPrintsTheOptedOutVariant(t *testing.T) {
	cleanEnv(t)
	t.Setenv(renderDisableAnalytics, "1")
	var stderr bytes.Buffer
	Begin(map[string]string{renderDisableAnalytics: "BEX_CLI_DISABLE_ANALYTICS"}).
		ShowIfNeeded(tree(), []string{"services"}, &stderr, true)
	if !strings.Contains(stderr.String(), "remove BEX_CLI_DISABLE_ANALYTICS") {
		t.Errorf("opted-out notice does not name BEX_CLI_DISABLE_ANALYTICS:\n%s", stderr.String())
	}
	if !CanSend() {
		t.Error("the opted-out notice was not recorded")
	}
}

func TestAdoptNoticeCommandPrintsOnceWithoutATerminal(t *testing.T) {
	markers := cleanEnv(t)
	root := tree()
	gate := Begin(nil)
	gate.AdoptNoticeCommand(root)

	var send *cobra.Command
	run := func() string {
		t.Helper()
		var stderr bytes.Buffer
		notice, _, err := root.Find([]string{"analytics", "notice"})
		if err != nil {
			t.Fatal(err)
		}
		notice.SetErr(&stderr)
		if err := notice.RunE(notice, nil); err != nil {
			t.Fatal(err)
		}
		send, _, _ = root.Find([]string{"analytics", "send"})
		return stderr.String()
	}

	if got, want := run(), strings.Join(lines(""), "\n")+"\n"; got != want {
		t.Fatalf("first call printed %q, want the plain notice %q", got, want)
	}
	for _, marker := range []string{upstreamMarker, bexMarker} {
		if !exists(t, filepath.Join(markers, marker)) {
			t.Errorf("%s not written", marker)
		}
	}
	if got := run(); got != "" {
		t.Errorf("second call printed %q, want nothing", got)
	}
	if send.RunE != nil || send.Run == nil {
		t.Error("analytics send was replaced; it must stay upstream's")
	}
}
