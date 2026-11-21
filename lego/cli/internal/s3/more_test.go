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
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestGlobFiltersMatchPathsBelowTheSourceAndTheLastRuleDecides(t *testing.T) {
	for _, tc := range []struct {
		rules []string // "-" excludes, "+" includes, in command-line order
		path  string
		want  bool
	}{
		{nil, "a/b.txt", true},
		{[]string{"-*.tmp"}, "a/b.tmp", false}, // * crosses slashes
		{[]string{"-*.tmp"}, "a/b.txt", true},
		{[]string{"-a?c"}, "abc", false}, // ? is one character
		{[]string{"-a?c"}, "a/c", false}, // ... any, a slash too
		{[]string{"-a?c"}, "ac", true},
		{[]string{"-[ab]*"}, "b.txt", false}, // a class
		{[]string{"-[!ab]*"}, "b.txt", true}, // a negated class
		{[]string{"-[!ab]*"}, "z.txt", false},
		{[]string{"-v1.0"}, "v1x0", true}, // a dot is a dot, not any character
		{[]string{"-v1.0"}, "v1.0", false},
		{[]string{"-(x)+[y]"}, "(x)+y", false},      // regexp characters are plain
		{[]string{"-é*"}, "éclair", false},          // multibyte
		{[]string{"-*", "+*.jpg"}, "a/b.jpg", true}, // exclude everything, then include
		{[]string{"-*", "+*.jpg"}, "a/b.png", false},
		{[]string{"+*.jpg", "-*"}, "a/b.jpg", false}, // the last matching rule decides
		{[]string{"-*", "+logs/*", "-logs/old/*"}, "logs/new/a", true},
		{[]string{"-*", "+logs/*", "-logs/old/*"}, "logs/old/a", false},
	} {
		var rules filters
		for _, r := range tc.rules {
			if err := (filterFlag{include: r[0] == '+', rules: &rules}).Set(r[1:]); err != nil {
				t.Fatal(err)
			}
		}
		if got := rules.allows(tc.path); got != tc.want {
			t.Errorf("rules %v on %q = %v, want %v", tc.rules, tc.path, got, tc.want)
		}
	}
	var rules filters
	if err := (filterFlag{rules: &rules}).Set("[abc"); err == nil || !strings.Contains(err.Error(), "no closing ]") {
		t.Errorf("an unterminated class: %v", err)
	}
}

func TestParseLocationAndDestinationNaming(t *testing.T) {
	for _, tc := range []struct {
		arg    string
		bucket string
		key    string
		remote bool
		bad    bool
	}{
		{"s3://photos", "photos", "", true, false},
		{"s3://photos/", "photos", "", true, false},
		{"s3://photos/a/b c.txt", "photos", "a/b c.txt", true, false},
		{"./local/file", "", "", false, false},
		{"-", "", "", false, false},
		{"s3://", "", "", false, true},
		{"s3://Photos/k", "", "", false, true},
		{"s3://ab/k", "", "", false, true},
		{"s3://-photos/k", "", "", false, true},
		{"s3://../k", "", "", false, true},
		{"", "", "", false, true},
	} {
		loc, err := parseLocation(tc.arg)
		if (err != nil) != tc.bad {
			t.Errorf("parseLocation(%q) error = %v, want bad=%v", tc.arg, err, tc.bad)
			continue
		}
		if err == nil && (loc.Remote != tc.remote || loc.Bucket != tc.bucket || loc.Key != tc.key) {
			t.Errorf("parseLocation(%q) = %+v", tc.arg, loc)
		}
	}
	for _, tc := range []struct{ key, name, want string }{
		{"", "a.txt", "a.txt"}, {"dir/", "a.txt", "dir/a.txt"}, {"dir/new.txt", "a.txt", "dir/new.txt"},
	} {
		if got := destinationKey(tc.key, tc.name); got != tc.want {
			t.Errorf("destinationKey(%q, %q) = %q, want %q", tc.key, tc.name, got, tc.want)
		}
	}
	for _, tc := range []struct{ in, want string }{{"", ""}, {"a", "a/"}, {"a/", "a/"}, {"a/b", "a/b/"}} {
		if got := asDir(tc.in); got != tc.want {
			t.Errorf("asDir(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	for rel, ok := range map[string]bool{
		"a": true, "a/b.txt": true, "dir/": true, ".hidden": true, "a..b": true,
		"": false, "../x": false, "a/../x": false, "a/./b": false, "/abs": false, "a//b": false, "a\x00b": false,
	} {
		if got := safeRel(rel); got != ok {
			t.Errorf("safeRel(%q) = %v, want %v", rel, got, ok)
		}
	}
}

// TestAnErrorEveryFileWouldMeetStopsTheCommandOnce: a quota refusal on the first
// upload of a tree is one error, not one failure line per file.
func TestAnErrorEveryFileWouldMeetStopsTheCommandOnce(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	for i := range 40 {
		writeFile(t, dir, "tree/f"+string(rune('a'+i%26))+string(rune('a'+i/26)), 100)
	}
	g.s3.intercept = func(r *http.Request, _ int) (int, string, string, bool) {
		return http.StatusForbidden, "QuotaExceeded", "this workspace's object storage in this region would exceed its storage limit of 107374182400", r.Method == http.MethodPut
	}
	g.s3.reset()
	_, stderr, err := g.bex("cp", filepath.Join(dir, "tree"), "s3://photos/t", "--recursive", "--concurrency", "2")
	if err == nil || !strings.Contains(err.Error(), "QuotaExceeded") || !strings.Contains(err.Error(), "107374182400") {
		t.Fatalf("err = %v, want the quota refusal with its cap", err)
	}
	if strings.Contains(stderr, "failed:") {
		t.Errorf("a fatal error was reported once per file:\n%s", stderr)
	}
	if puts := len(g.s3.requests("PUT")); puts > 6 {
		t.Errorf("%d uploads were attempted after the first refusal, want the command to stop", puts)
	}
}

// TestADeleteTheServerRefusesForSomeKeysIsReportedKeyByKey: the batch goes, the
// keys it kept are named, and the command fails without printing them as deleted.
func TestADeleteTheServerRefusesForSomeKeysIsReportedKeyByKey(t *testing.T) {
	g := newRig(t)
	for _, key := range []string{"t/a", "t/b", "t/keep"} {
		g.s3.put("photos", key, []byte("x"), testNow)
	}
	g.s3.protected = map[string]bool{"t/keep": true}
	stdout, stderr, err := g.bex("rm", "s3://photos/t", "--recursive")
	if err == nil || !strings.Contains(err.Error(), "1 of 1 operations failed") {
		t.Errorf("err = %v", err)
	}
	if !strings.Contains(stderr, "failed: delete s3://photos/t/keep: AccessDenied: this key is protected") || strings.Count(stderr, "failed:") != 1 {
		t.Errorf("stderr = %q, want the one key named with the server's reason", stderr)
	}
	if strings.Contains(stdout, "t/keep") || !strings.Contains(stdout, "delete: s3://photos/t/a") || !strings.Contains(stdout, "delete: s3://photos/t/b") {
		t.Errorf("stdout = %q, want the two deleted and not the refused one", stdout)
	}
	if got := g.s3.keys("photos"); !slices.Equal(got, []string{"t/keep"}) {
		t.Errorf("keys left = %v", got)
	}
}

// TestProgressIsDrawnOnATerminalAndNeverElse: a transfer long enough for the
// ticker draws a status line on stderr only when stderr is a terminal and
// --no-progress is not given, and clears it before the result line prints.
func TestProgressIsDrawnOnATerminalAndNeverElse(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	writeFile(t, dir, "f", 3<<20)
	g.s3.hold = 600 * time.Millisecond
	run := func(terminal bool, extra ...string) string {
		g.d.isTerminal = func(io.Writer) bool { return terminal }
		_, stderr, err := g.bex(append([]string{"cp", filepath.Join(dir, "f"), "s3://photos/f"}, extra...)...)
		if err != nil {
			t.Fatal(err)
		}
		return stderr
	}
	if got := run(true); !strings.Contains(got, "Completed ") || !strings.Contains(got, "/3.0 MiB") || !strings.HasSuffix(got, "\r\x1b[2K") {
		t.Errorf("a terminal run drew %q, want a Completed line of 3.0 MiB cleared at the end", got)
	}
	if got := run(true, "--no-progress"); got != "" {
		t.Errorf("--no-progress drew %q", got)
	}
	if got := run(false); got != "" {
		t.Errorf("a run that is not on a terminal drew %q", got)
	}
	g.d.isTerminal = func(io.Writer) bool { return true }
	if _, stderr, _ := g.bex("cp", filepath.Join(dir, "f"), "s3://photos/f", "-o", "json"); stderr != "" {
		t.Errorf("a JSON run drew %q", stderr)
	}
}

func TestANamedPipeIsSkippedWithAWarning(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	writeFile(t, dir, "tree/ok", 5)
	if err := syscall.Mkfifo(filepath.Join(dir, "tree", "pipe"), 0o600); err != nil {
		t.Skip(err)
	}
	_, stderr, err := g.bex("sync", filepath.Join(dir, "tree"), "s3://photos/t")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(stderr, "warning: skipping") != 1 || !strings.Contains(stderr, "a named pipe") {
		t.Errorf("stderr = %q", stderr)
	}
	if got := g.s3.keys("photos"); !slices.Equal(got, []string{"t/ok"}) {
		t.Errorf("keys = %v", got)
	}
}

// TestARecursiveCopyThatSetsMetadataKeepsEachObjectsContentType: the listing does
// not carry a content type, so the copy reads it from the object first.
func TestARecursiveCopyThatSetsMetadataKeepsEachObjectsContentType(t *testing.T) {
	g := newRig(t)
	g.s3.put("photos", "t/a.html", []byte("<p>"), testNow)
	g.s3.put("photos", "t/b.json", []byte("{}"), testNow)
	g.s3.mu.Lock()
	g.s3.buckets["photos"].objects["t/a.html"].contentType = "text/html"
	g.s3.buckets["photos"].objects["t/b.json"].contentType = "application/json"
	g.s3.mu.Unlock()
	g.s3.bucket("backup")
	g.must("cp", "s3://photos/t", "s3://backup/c", "--recursive", "--metadata", "k=v")
	for key, want := range map[string]string{"c/a.html": "text/html", "c/b.json": "application/json"} {
		o, ok := g.s3.object("backup", key)
		if !ok || o.contentType != want || o.metadata["k"] != "v" {
			t.Errorf("%s: type %q metadata %v, want %q and k=v", key, o.contentType, o.metadata, want)
		}
	}
}

func TestFatalErrorsAreTheOnesEveryLaterRequestWouldMeet(t *testing.T) {
	g := newRig(t)
	g.s3.bucket("photos")
	for code, want := range map[string]bool{
		"InvalidAccessKeyId": true, "ExpiredToken": true, "SignatureDoesNotMatch": true, "RequestTimeTooSkewed": true,
		"QuotaExceeded": true, "AccessDenied": true, "NotImplemented": true,
		"NoSuchKey": false, "InvalidRequest": false, "EntityTooSmall": false,
	} {
		g.s3.intercept = func(r *http.Request, _ int) (int, string, string, bool) { return 400, code, "m", true }
		_, _, err := g.bex("ls", "s3://photos")
		if err == nil {
			t.Fatalf("%s: no error", code)
		}
		if got := fatal(err); got != want {
			t.Errorf("fatal(%s) = %v, want %v", code, got, want)
		}
	}
}
