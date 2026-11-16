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
	"encoding/json"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// ADR047 D12 (w22/m413 t004): `bex agent` says what became of the unpushed
// work of a turn a person stopped, in the words the dashboard uses.

var keptUntil = time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)

// keptUntilText is the time as the CLI writes it: the viewer's own zone.
func keptUntilText() string { return keptUntil.Local().Format(minuteLayout + " MST") }

func TestRetentionSentencesAreTheDashboardsWords(t *testing.T) {
	until := keptUntil
	for _, tc := range []struct {
		name string
		turn transcriptTurn
		want string
	}{
		{"kept", transcriptTurn{Retention: "kept", RetainedUntil: &until}, "Your changes are kept until " + keptUntilText() + "."},
		{"saving", transcriptTurn{Retention: "saving"}, "Saving your changes so the next message can continue from them."},
		{"continued", transcriptTurn{Retention: "continued"}, "The next message continued from these changes."},
		{"lost, expired", transcriptTurn{Retention: "lost", RetentionLostReason: "expired"}, "Your unpushed changes were lost when the retained sandbox expired."},
		{"lost, storage full", transcriptTurn{Retention: "lost", RetentionLostReason: "storage_full"}, "Your unpushed changes were not kept because the workspace's sandbox storage was full."},
		{"lost, save failed", transcriptTurn{Retention: "lost", RetentionLostReason: "save_failed"}, "Your unpushed changes could not be saved and were lost."},
		{"lost, editor open", transcriptTurn{Retention: "lost", RetentionLostReason: "editor_open"}, "Your unpushed changes were not kept because an editor was open on the sandbox."},
		{"lost, not restored", transcriptTurn{Retention: "lost", RetentionLostReason: "not_restored"}, "Your unpushed changes could not be restored, so the next message started from a fresh checkout."},
		{"lost, canceled", transcriptTurn{Retention: "lost", RetentionLostReason: "canceled"}, "Your unpushed changes were lost."},
		{"lost, no reason", transcriptTurn{Retention: "lost"}, "Your unpushed changes were lost."},
		{"lost, a newer reason", transcriptTurn{Retention: "lost", RetentionLostReason: "something-new"}, "Your unpushed changes were lost."},
		{"none", transcriptTurn{Retention: "none"}, ""},
		{"a newer state", transcriptTurn{Retention: "frozen"}, ""},
		{"kept with no time", transcriptTurn{Retention: "kept"}, ""},
	} {
		if got := retentionSentence(tc.turn); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := stoppedNote(transcriptTurn{Turn: 3, Retention: "none"}); got != "Turn 3 was interrupted before it finished." {
		t.Errorf("a stopped turn that kept nothing: %q", got)
	}
}

// stoppedFixture is a session whose first turn a person stopped, with a second
// turn after it, and the retention its first turn reports.
func stoppedFixture(f *fakeAPI, id string, retention map[string]any) {
	f.script[id] = []string{stateSucceeded}
	f.transcript[id] = []string{
		row(1, 1, 1, `{"type":"text","text":"half a reply"}`, false),
		row(2, 1, 2, `{"type":"text","text":"carrying on"}`, false),
	}
	first := map[string]any{"turn": 1, "prompt": "fix it", "interrupted": true}
	for k, v := range retention {
		first[k] = v
	}
	f.turns[id] = []map[string]any{first, {"turn": 2, "prompt": "again", "interrupted": false, "retention": "none"}}
}

func TestLogsEndsAStoppedTurnWithWhatBecameOfItsWork(t *testing.T) {
	for _, tc := range []struct {
		name      string
		retention map[string]any
		sentence  string
	}{
		{"kept", map[string]any{"retention": "kept", "retainedUntil": "2026-11-16T10:00:00Z"}, " Your changes are kept until " + keptUntilText() + "."},
		{"lost", map[string]any{"retention": "lost", "retentionLostReason": "expired"}, " Your unpushed changes were lost when the retained sandbox expired."},
		{"saving", map[string]any{"retention": "saving"}, " Saving your changes so the next message can continue from them."},
		{"nothing kept", map[string]any{"retention": "none"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeAPI(t)
			stoppedFixture(f, "ags-s1", tc.retention)
			stdout, stderr, code := run(t, f.deps(t).deps, "logs", "ags-s1")
			if code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			// The ending comes after the turn's own lines and before the next turn.
			want := "── turn 1 ──\n> fix it\nhalf a reply\n" +
				"■ Turn 1 was interrupted before it finished." + tc.sentence + "\n" +
				"── turn 2 ──\n> again\ncarrying on\n"
			if stdout != want {
				t.Fatalf("logs\n got %q\nwant %q", stdout, want)
			}
		})
	}
}

func TestLogsEndsTheLastTurnToo(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-s2"] = []string{stateSucceeded}
	f.transcript["ags-s2"] = []string{row(1, 1, 1, `{"type":"text","text":"half a reply"}`, false)}
	f.turns["ags-s2"] = []map[string]any{{"turn": 1, "prompt": "fix it", "interrupted": true, "retention": "lost", "retentionLostReason": "storage_full"}}
	stdout, _, code := run(t, f.deps(t).deps, "logs", "ags-s2")
	want := "── turn 1 ──\n> fix it\nhalf a reply\n■ Turn 1 was interrupted before it finished. " +
		"Your unpushed changes were not kept because the workspace's sandbox storage was full.\n"
	if code != 0 || stdout != want {
		t.Fatalf("exit %d\n got %q\nwant %q", code, stdout, want)
	}
}

// A turn nobody stopped has no ending, whatever else the answer says.
func TestLogsSaysNothingOfATurnNobodyStopped(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-s3"] = []string{stateSucceeded}
	f.transcript["ags-s3"] = []string{row(1, 1, 1, `{"type":"text","text":"all done"}`, false)}
	f.turns["ags-s3"] = []map[string]any{{"turn": 1, "prompt": "fix it", "interrupted": false, "retention": "kept", "retainedUntil": "2026-11-16T10:00:00Z"}}
	stdout, _, _ := run(t, f.deps(t).deps, "logs", "ags-s3")
	if stdout != "── turn 1 ──\n> fix it\nall done\n" {
		t.Fatalf("logs printed %q", stdout)
	}
}

// -o json prints one transcript row per line, and, after the rows of a stopped
// turn, that turn as the API sent it: its retention is in the output too.
func TestLogsJSONCarriesTheStoppedTurnAsTheAPISentIt(t *testing.T) {
	f := newFakeAPI(t)
	stoppedFixture(f, "ags-s4", map[string]any{"retention": "kept", "retainedUntil": "2026-11-16T10:00:00Z"})
	stdout, stderr, code := run(t, f.deps(t).deps, "logs", "ags-s4", "-o", "json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 || !strings.Contains(lines[0], `"seq":1`) || !strings.Contains(lines[2], `"seq":2`) {
		t.Fatalf("json lines = %q", lines)
	}
	for _, want := range []string{`"turn":1`, `"interrupted":true`, `"retention":"kept"`, `"retainedUntil":"2026-11-16T10:00:00Z"`} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("the stopped turn's line %q lacks %s", lines[1], want)
		}
	}
}

func TestGetShowsWhatBecameOfAStoppedTurnsWork(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-g1"] = []string{stateSucceeded}
	f.turns["ags-g1"] = []map[string]any{{"turn": 1, "prompt": "fix it", "interrupted": true, "retention": "kept", "retainedUntil": "2026-11-16T10:00:00Z"}}
	stdout, stderr, code := run(t, f.deps(t).deps, "get", "ags-g1")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	line := ""
	for _, l := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(l, "Stopped") {
			line = l
		}
	}
	want := "Turn 1 was interrupted before it finished. Your changes are kept until " + keptUntilText() + "."
	if !strings.HasSuffix(line, want) {
		t.Fatalf("get printed no Stopped row saying %q:\n%s", want, stdout)
	}
	// The query asks for no transcript part, only the turns.
	if !strings.Contains(f.transcrQuery, "limit=1") || !strings.Contains(f.transcrQuery, "afterSeq=2147483647") {
		t.Fatalf("the transcript was read with %q", f.transcrQuery)
	}
}

func TestGetSaysNothingExtraWhenThereIsNothingToSay(t *testing.T) {
	cases := map[string]func(f *fakeAPI){
		"the latest turn was not stopped": func(f *fakeAPI) {
			f.turns["ags-g2"] = []map[string]any{{"turn": 1, "interrupted": true, "retention": "kept", "retainedUntil": "2026-11-16T10:00:00Z"}, {"turn": 2, "interrupted": false}}
		},
		"no turn": func(f *fakeAPI) { f.turns["ags-g2"] = []map[string]any{} },
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeAPI(t)
			f.script["ags-g2"] = []string{stateSucceeded}
			arrange(f)
			stdout, _, code := run(t, f.deps(t).deps, "get", "ags-g2")
			if code != 0 || strings.Contains(stdout, "Stopped") || strings.Contains(stdout, "interrupted") {
				t.Fatalf("exit %d, get printed %q", code, stdout)
			}
		})
	}
}

// A working session has no settled turn to ask about, and the structured
// answers are the API's own: neither reads the transcript.
func TestGetReadsNoTranscriptWhereItHasNothingToShow(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-g3"] = []string{stateWorking}
	f.script["ags-g4"] = []string{stateSucceeded}
	f.turns["ags-g4"] = []map[string]any{{"turn": 1, "interrupted": true, "retention": "lost"}}
	for _, args := range [][]string{{"get", "ags-g3"}, {"get", "ags-g4", "-o", "json"}, {"get", "ags-g4", "-o", "yaml"}} {
		if _, _, code := run(t, f.deps(t).deps, args...); code != 0 {
			t.Fatalf("%v exit %d", args, code)
		}
		if f.transcrGet != 0 {
			t.Fatalf("%v read the transcript %d times", args, f.transcrGet)
		}
	}
}

// The extra read is best effort: when it fails the session is still shown.
func TestGetStillShowsTheSessionWhenTheTurnReadFails(t *testing.T) {
	f := newFakeAPI(t)
	f.script["ags-g5"] = []string{stateSucceeded}
	f.turns["ags-g5"] = []map[string]any{{"turn": 1, "interrupted": true, "retention": "lost"}}
	f.settled["ags-g5"] = true
	f.failTail = []int{http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusServiceUnavailable}
	stdout, stderr, code := run(t, f.deps(t).deps, "get", "ags-g5")
	if code != 0 || !strings.Contains(stdout, "ags-g5") || strings.Contains(stdout, "Stopped") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if f.transcrGet == 0 {
		t.Fatal("the turn was never read, so the failure was not exercised")
	}
}

// The states and loss reasons bex-api's transcript names are the fixture's
// lists, which the backend twin pins to its own constants. The CLI declares the
// same lists and says every entry: a state or reason added to the server
// reaches the fixture, and so this test, where the CLI is made to decide how to
// word it (a cause with nothing to say is a plain loss, on purpose).
func TestEveryRetentionStateAndLossReasonTheContractNamesIsSaid(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/agent-cli-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		States  []string `json:"retentionStates"`
		Reasons []string `json:"retentionLostReasons"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	if want := []string{retentionNone, retentionSaving, retentionKept, retentionContinued, retentionLost}; !slices.Equal(golden.States, want) {
		t.Fatalf("the contract's states %v are not the CLI's %v", golden.States, want)
	}
	if want := []string{lostExpired, lostStorageFull, lostSaveFailed, lostEditorOpen, lostCanceled, lostArchived, lostNotRestored}; !slices.Equal(golden.Reasons, want) {
		t.Fatalf("the contract's loss reasons %v are not the CLI's %v", golden.Reasons, want)
	}

	until := keptUntil
	for _, state := range golden.States {
		got := retentionSentence(transcriptTurn{Retention: state, RetainedUntil: &until})
		if (state == retentionNone) != (got == "") {
			t.Errorf("state %q says %q: only none says nothing", state, got)
		}
	}
	plainLoss := lostSentence("")
	for _, reason := range golden.Reasons {
		got := lostSentence(reason)
		ended := reason == lostCanceled || reason == lostArchived
		if got == "" || (got == plainLoss) != ended {
			t.Errorf("loss reason %q says %q: only a canceled or archived session is a plain loss (%q)", reason, got, plainLoss)
		}
	}
	// A reason this CLI does not know is a plain loss, never a guess.
	if got := lostSentence("something-new"); got != plainLoss {
		t.Errorf("an unknown loss reason says %q, want %q", got, plainLoss)
	}
}

// Only a stopped turn is ever printed as the API sent it, so only it keeps its
// bytes: a transcript of many turns is not copied turn by turn.
func TestOnlyAStoppedTurnKeepsItsRawBytes(t *testing.T) {
	var page transcriptPage
	const body = `{"parts":[],"turns":[{"turn":1,"prompt":"a","interrupted":false},{"turn":2,"prompt":"b","interrupted":true,"retention":"lost"}],"nextAfterSeq":0}`
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Turns) != 2 || page.Turns[0].raw != nil || !strings.Contains(string(page.Turns[1].raw), `"retention":"lost"`) {
		t.Fatalf("raw bytes kept: turn 1 %q, turn 2 %q", page.Turns[0].raw, page.Turns[1].raw)
	}
}
