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
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestRoundTripsAreByteIdentical: an object of 0 bytes, 1 byte and 20 MiB goes
// up and comes back whole; the large one is a multipart upload of three 8 MiB
// parts and a ranged download, and the small ones are one request each.
func TestRoundTripsAreByteIdentical(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir, back := t.TempDir(), t.TempDir()
	for _, tc := range []struct {
		name  string
		size  int
		parts int
	}{{"zero", 0, 0}, {"one", 1, 0}, {"big", 20 << 20, 3}} {
		want := writeFile(t, dir, tc.name, tc.size)
		g.s3.reset()
		out := g.must("cp", filepath.Join(dir, tc.name), "s3://photos/up/"+tc.name)
		if want := "upload: " + filepath.Join(dir, tc.name) + " to s3://photos/up/" + tc.name; strings.TrimSpace(out) != want {
			t.Errorf("cp printed %q, want %q", out, want)
		}
		if got := g.s3.count("partNumber="); got != tc.parts {
			t.Errorf("%s: %d part uploads, want %d", tc.name, got, tc.parts)
		}
		if multipart := g.s3.count("POST /photos/up/" + tc.name + "?uploads"); (multipart == 1) != (tc.parts > 0) {
			t.Errorf("%s: %d multipart starts for %d parts", tc.name, multipart, tc.parts)
		}
		stored, ok := g.s3.object("photos", "up/"+tc.name)
		if !ok || sum(stored.data) != sum(want) {
			t.Fatalf("%s: the stored object differs from the file", tc.name)
		}
		g.must("cp", "s3://photos/up/"+tc.name, filepath.Join(back, tc.name))
		sameBytes(t, filepath.Join(back, tc.name), want)
	}
	if names := entriesOf(t, back); len(names) != 3 {
		t.Errorf("downloads left %v, want three files and no temporary ones", names)
	}
}

func TestDownloadGivesTheFileTheObjectsTimeAndNamesIntoADirectory(t *testing.T) {
	g := newRig(t)
	when := time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC)
	g.s3.put("photos", "a/b/cat.jpg", []byte("meow"), when)
	dir := t.TempDir()
	g.must("cp", "s3://photos/a/b/cat.jpg", dir+"/")
	info, err := os.Stat(filepath.Join(dir, "cat.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(when) {
		t.Errorf("mtime = %v, want the object's %v", info.ModTime(), when)
	}
	// An existing directory without a slash too, and an explicit name.
	g.must("cp", "s3://photos/a/b/cat.jpg", dir)
	g.must("cp", "s3://photos/a/b/cat.jpg", filepath.Join(dir, "sub", "renamed.jpg"))
	if got := entriesOf(t, dir); !slices.Equal(got, []string{"cat.jpg", "sub/renamed.jpg"}) {
		t.Errorf("files = %v", got)
	}
	if msg := g.fails("cp", "s3://photos/missing", dir); !strings.Contains(msg, "there is no object s3://photos/missing") {
		t.Errorf("a missing object says %q", msg)
	}
	if got := entriesOf(t, dir); len(got) != 2 {
		t.Errorf("a failed download left %v", got)
	}
}

// TestNoMoreRequestsThanConcurrencyAreInFlight, files and parts together.
func TestNoMoreRequestsThanConcurrencyAreInFlight(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	g.s3.hold = 15 * time.Millisecond
	dir := t.TempDir()
	for i := range 6 {
		writeFile(t, dir, "tree/f"+string(rune('a'+i)), 1<<10)
	}
	writeFile(t, dir, "tree/large", 40<<20) // five parts
	for _, n := range []string{"1", "3", "10"} {
		g.s3.reset()
		g.s3.mu.Lock()
		g.s3.buckets["photos"].objects = map[string]*fakeObject{}
		g.s3.mu.Unlock()
		g.must("cp", filepath.Join(dir, "tree"), "s3://photos/t", "--recursive", "--concurrency", n)
		bound := map[string]int32{"1": 1, "3": 3, "10": 10}[n]
		if peak := g.s3.peak.Load(); peak > bound {
			t.Errorf("--concurrency %s: %d requests were in flight at once", n, peak)
		} else if bound > 1 && peak < 2 {
			t.Errorf("--concurrency %s: peak %d, the work was not done concurrently", n, peak)
		}
		if len(g.s3.keys("photos")) != 7 {
			t.Errorf("--concurrency %s: %d objects", n, len(g.s3.keys("photos")))
		}
	}
	// The same bound holds for downloads.
	back := t.TempDir()
	g.s3.reset()
	g.must("cp", "s3://photos/t", back, "--recursive", "--concurrency", "2")
	if peak := g.s3.peak.Load(); peak > 2 {
		t.Errorf("a download at --concurrency 2 had %d requests in flight", peak)
	}
	sameBytes(t, filepath.Join(back, "large"), mustRead(t, filepath.Join(dir, "tree", "large")))
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestAFailedDownloadLeavesNoFileAndKeepsTheOldOne: the part that fails is in
// the middle of the object, and the temporary file is gone with the transfer.
func TestAFailedDownloadLeavesNoFileAndKeepsTheOldOne(t *testing.T) {
	g := newRig(t)
	data := make([]byte, 20<<20)
	g.s3.put("photos", "big", data, testNow)
	dir := t.TempDir()
	old := writeBytes(t, dir, "big", []byte("the older file"))
	g.s3.intercept = func(r *http.Request, _ int) (int, string, string, bool) {
		return http.StatusBadRequest, "InvalidRequest", "this part cannot be read", strings.HasPrefix(r.Header.Get("Range"), "bytes=8388608-")
	}
	msg := g.fails("cp", "s3://photos/big", filepath.Join(dir, "big"))
	if !strings.Contains(msg, "InvalidRequest") {
		t.Errorf("error %q does not carry the part's failure", msg)
	}
	if names := entriesOf(t, dir); !slices.Equal(names, []string{"big"}) {
		t.Errorf("files after the failure = %v, want only the older one and no temporary file", names)
	}
	sameBytes(t, filepath.Join(dir, "big"), old)

	// A recursive download reports each failure, finishes the rest and exits non-zero.
	g.s3.put("photos", "tree/ok", []byte("fine"), testNow)
	g.s3.put("photos", "tree/big", data, testNow)
	out := t.TempDir()
	stdout, stderr, err := g.bex("cp", "s3://photos/tree", out, "--recursive")
	if err == nil || !strings.Contains(err.Error(), "1 of 2 operations failed") {
		t.Errorf("err = %v, want one of two failed", err)
	}
	if !strings.Contains(stderr, "failed: download s3://photos/tree/big") || !strings.Contains(stdout, "download: s3://photos/tree/ok") {
		t.Errorf("stdout %q, stderr %q", stdout, stderr)
	}
	if names := entriesOf(t, out); !slices.Equal(names, []string{"ok"}) {
		t.Errorf("files = %v, want only the one that arrived", names)
	}
}

func TestStandardInputAndOutput(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	payload := strings.Repeat("stream ", 3<<20) // over one part
	g.stdin.WriteString(payload)
	out := g.must("cp", "-", "s3://photos/piped.txt")
	if strings.TrimSpace(out) != "upload: - to s3://photos/piped.txt" {
		t.Errorf("cp - printed %q", out)
	}
	if o, ok := g.s3.object("photos", "piped.txt"); !ok || string(o.data) != payload {
		t.Fatalf("the stored object is %d bytes, want the %d of the stream", len(o.data), len(payload))
	}
	if g.s3.count("partNumber=") < 2 {
		t.Error("a stream larger than a part was not a multipart upload")
	}
	// Standard output carries the bytes and nothing else, not even progress or a result line.
	got, stderr, err := g.bex("cp", "s3://photos/piped.txt", "-")
	if err != nil || got != payload || stderr != "" {
		t.Errorf("cp to - = %d bytes, stderr %q, err %v", len(got), stderr, err)
	}
	// An empty stream is an empty object.
	g.stdin.Reset()
	g.must("cp", "-", "s3://photos/empty")
	if o, ok := g.s3.object("photos", "empty"); !ok || len(o.data) != 0 {
		t.Errorf("empty stream stored %v", ok)
	}
}

// TestStandardInputMayBeAPipeOrAFile: in production standard input is an
// *os.File, a Seeker even when it is a pipe, whose Seek and ReadAt fail. A
// pipe carrying more than one part and an empty pipe go up whole, and so does a
// redirected regular file (w32/062: `seek /dev/stdin: illegal seek`).
func TestStandardInputMayBeAPipeOrAFile(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	piped := func(payload []byte) *os.File {
		pr, pw, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = pr.Close() })
		go func() {
			_, _ = pw.Write(payload)
			_ = pw.Close()
		}()
		return pr
	}
	redirected := func(payload []byte) *os.File {
		dir := t.TempDir()
		writeBytes(t, dir, "in", payload)
		f, err := os.Open(filepath.Join(dir, "in"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = f.Close() })
		return f
	}
	big := []byte(strings.Repeat("pipe ", 2<<20)) // 10 MiB, over one part
	for _, tc := range []struct {
		name    string
		stdin   func([]byte) *os.File
		payload []byte
		parts   int
	}{
		{"pipe-big", piped, big, 2},
		{"pipe-empty", piped, nil, 0},
		{"file-big", redirected, big, 2},
	} {
		g.s3.reset()
		g.d.stdin = tc.stdin(tc.payload)
		out := g.must("cp", "-", "s3://photos/"+tc.name)
		if want := "upload: - to s3://photos/" + tc.name; strings.TrimSpace(out) != want {
			t.Errorf("%s: cp - printed %q, want %q", tc.name, out, want)
		}
		o, ok := g.s3.object("photos", tc.name)
		if !ok || sum(o.data) != sum(tc.payload) {
			t.Fatalf("%s: the stored object differs from the %d bytes of standard input", tc.name, len(tc.payload))
		}
		if got := g.s3.count("partNumber="); got != tc.parts {
			t.Errorf("%s: %d part uploads, want %d", tc.name, got, tc.parts)
		}
	}
}

func TestDryrunMakesNoRequestThatChangesAnything(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	g.s3.put("photos", "src/a", []byte("a"), testNow)
	dir := t.TempDir()
	writeFile(t, dir, "tree/x", 10)
	g.s3.reset()
	for _, args := range [][]string{
		{"cp", filepath.Join(dir, "tree"), "s3://photos/dst", "--recursive", "--dryrun"},
		{"cp", filepath.Join(dir, "tree", "x"), "s3://photos/x", "--dryrun"},
		{"mv", filepath.Join(dir, "tree", "x"), "s3://photos/x", "--dryrun"},
		{"cp", "s3://photos/src/a", filepath.Join(dir, "got"), "--dryrun"},
		{"mv", "s3://photos/src/a", "s3://photos/moved/a", "--dryrun"},
		{"cp", "s3://photos/src", "s3://photos/copy", "--recursive", "--dryrun"},
		{"rm", "s3://photos/src/a", "--dryrun"},
		{"sync", filepath.Join(dir, "tree"), "s3://photos/sync", "--dryrun", "--delete"},
	} {
		out := g.must(args...)
		if !strings.Contains(out, "(dryrun) ") {
			t.Errorf("bex s3 %v printed %q, want a (dryrun) line", args, out)
		}
	}
	for _, line := range g.s3.requests("") {
		if !strings.HasPrefix(line, "GET ") && !strings.HasPrefix(line, "HEAD ") {
			t.Errorf("--dryrun sent %q", line)
		}
	}
	if keys := g.s3.keys("photos"); !slices.Equal(keys, []string{"src/a"}) {
		t.Errorf("keys = %v after dry runs", keys)
	}
	if _, err := os.Stat(filepath.Join(dir, "got")); err == nil {
		t.Error("a dry-run download wrote a file")
	}
	if _, err := os.Stat(filepath.Join(dir, "tree", "x")); err != nil {
		t.Error("a dry-run mv removed the source")
	}
}

// TestMvDeletesOnlyAfterTheCopySucceeded in each direction.
func TestMvDeletesOnlyAfterTheCopySucceeded(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	fail := func(match func(*http.Request) bool) {
		g.s3.intercept = func(r *http.Request, _ int) (int, string, string, bool) {
			return http.StatusBadRequest, "InvalidRequest", "refused", match(r)
		}
	}

	// bucket to bucket: the copy fails, the source stays.
	g.s3.put("photos", "src/a", []byte("a"), testNow)
	fail(func(r *http.Request) bool { return r.Header.Get("x-amz-copy-source") != "" })
	g.fails("mv", "s3://photos/src/a", "s3://photos/dst/a")
	if keys := g.s3.keys("photos"); !slices.Equal(keys, []string{"src/a"}) {
		t.Errorf("a failed copy left %v", keys)
	}
	if g.s3.count("DELETE ") != 0 {
		t.Error("mv deleted after a copy that failed")
	}
	g.s3.intercept = nil
	if out := g.must("mv", "s3://photos/src/a", "s3://photos/dst/a"); !strings.Contains(out, "move: s3://photos/src/a to s3://photos/dst/a") {
		t.Errorf("mv printed %q", out)
	}
	if keys := g.s3.keys("photos"); !slices.Equal(keys, []string{"dst/a"}) {
		t.Errorf("after a successful move: %v", keys)
	}

	// local to bucket: the upload fails, the file stays.
	data := writeFile(t, dir, "report.pdf", 100)
	fail(func(r *http.Request) bool { return r.Method == http.MethodPut })
	g.fails("mv", filepath.Join(dir, "report.pdf"), "s3://photos/report.pdf")
	sameBytes(t, filepath.Join(dir, "report.pdf"), data)
	g.s3.intercept = nil
	g.must("mv", filepath.Join(dir, "report.pdf"), "s3://photos/report.pdf")
	if _, err := os.Stat(filepath.Join(dir, "report.pdf")); err == nil {
		t.Error("the file is still there after a successful move")
	}
	if o, ok := g.s3.object("photos", "report.pdf"); !ok || sum(o.data) != sum(data) {
		t.Error("the moved file is not in the bucket")
	}

	// bucket to local: the download fails, the object stays.
	fail(func(r *http.Request) bool { return r.Method == http.MethodGet && r.URL.Path == "/photos/report.pdf" })
	g.fails("mv", "s3://photos/report.pdf", filepath.Join(dir, "back.pdf"))
	if _, ok := g.s3.object("photos", "report.pdf"); !ok {
		t.Error("mv deleted an object it failed to download")
	}
	g.s3.intercept = nil
	g.must("mv", "s3://photos/report.pdf", filepath.Join(dir, "back.pdf"))
	sameBytes(t, filepath.Join(dir, "back.pdf"), data)
	if _, ok := g.s3.object("photos", "report.pdf"); ok {
		t.Error("the object is still in the bucket after a successful move")
	}

	// A move onto itself is refused before anything is touched.
	g.s3.put("photos", "same", []byte("x"), testNow)
	g.s3.reset()
	g.fails("mv", "s3://photos/same", "s3://photos/same")
	if len(g.s3.requests("")) != 0 {
		t.Error("mv onto itself made requests")
	}
}

// TestRecursiveMvMovesFileByFile: each source goes only after its own copy.
func TestRecursiveMvMovesFileByFile(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	good := writeFile(t, dir, "tree/ok.txt", 10)
	writeFile(t, dir, "tree/bad.txt", 10)
	g.s3.intercept = func(r *http.Request, _ int) (int, string, string, bool) {
		return http.StatusBadRequest, "InvalidRequest", "refused", r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "bad.txt")
	}
	_, _, err := g.bex("mv", filepath.Join(dir, "tree"), "s3://photos/t", "--recursive")
	if err == nil {
		t.Fatal("a failed transfer exited 0")
	}
	if got := entriesOf(t, dir); !slices.Equal(got, []string{"tree/bad.txt"}) {
		t.Errorf("local files left = %v, want only the one whose upload failed", got)
	}
	if o, ok := g.s3.object("photos", "t/ok.txt"); !ok || sum(o.data) != sum(good) {
		t.Error("the file that was moved is not in the bucket")
	}
}

func TestCopyBetweenBuckets(t *testing.T) {
	g := newRig(t)
	g.s3.put("photos", "a dir/cat+dog é.jpg", []byte("meow"), testNow)
	g.s3.bucket("backup")
	// A key with a space, a plus and a non-ASCII letter reaches the other bucket whole.
	g.must("cp", "s3://photos/a dir/cat+dog é.jpg", "s3://backup/")
	if o, ok := g.s3.object("backup", "cat+dog é.jpg"); !ok || string(o.data) != "meow" {
		t.Errorf("backup has %v", g.s3.keys("backup"))
	}
	// A recursive copy keeps what is below the prefix.
	g.s3.put("photos", "tree/x/1", []byte("1"), testNow)
	g.s3.put("photos", "tree/2", []byte("2"), testNow)
	g.s3.put("photos", "tree2/3", []byte("3"), testNow)
	g.must("cp", "s3://photos/tree", "s3://backup/copy", "--recursive")
	if keys := g.s3.keys("backup"); !slices.Equal(keys, []string{"cat+dog é.jpg", "copy/2", "copy/x/1"}) {
		t.Errorf("backup keys = %v, want only what is under tree/", keys)
	}
	// Prefixes that overlap would make a copy read what it writes.
	if msg := g.fails("cp", "s3://photos/tree", "s3://photos/tree/again", "--recursive"); !strings.Contains(msg, "overlap") {
		t.Errorf("an overlapping copy says %q", msg)
	}
}

func TestMetadataAndContentTypeAreSetAndNamesAreLowercase(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	writeFile(t, dir, "page.html", 20)
	writeFile(t, dir, "blob", 20)
	g.must("cp", filepath.Join(dir, "page.html"), "s3://photos/page.html", "--metadata", "Mtime=1", "--metadata", "Owner=ada")
	o, _ := g.s3.object("photos", "page.html")
	if !strings.HasPrefix(o.contentType, "text/html") || o.metadata["mtime"] != "1" || o.metadata["owner"] != "ada" {
		t.Errorf("stored content type %q and metadata %v", o.contentType, o.metadata)
	}
	g.must("cp", filepath.Join(dir, "blob"), "s3://photos/blob", "--content-type", "application/x-thing")
	if o, _ := g.s3.object("photos", "blob"); o.contentType != "application/x-thing" {
		t.Errorf("content type = %q", o.contentType)
	}
	// The name reads back as the server stores it: lowercase.
	out := g.must("info", "s3://photos/page.html")
	if !strings.Contains(out, "Metadata mtime") || strings.Contains(out, "Mtime") {
		t.Errorf("info =\n%s\nwant the metadata name in lowercase", out)
	}
	// A copy that sets headers replaces them; one that does not keeps them.
	g.must("cp", "s3://photos/page.html", "s3://photos/kept.html")
	if o, _ := g.s3.object("photos", "kept.html"); o.metadata["owner"] != "ada" || !strings.HasPrefix(o.contentType, "text/html") {
		t.Errorf("a plain copy has %q %v", o.contentType, o.metadata)
	}
	g.must("cp", "s3://photos/page.html", "s3://photos/replaced.html", "--metadata", "k=v")
	if o, _ := g.s3.object("photos", "replaced.html"); o.metadata["k"] != "v" || o.metadata["owner"] != "" || !strings.HasPrefix(o.contentType, "text/html") {
		t.Errorf("a copy with --metadata has %q %v, want the new metadata and the old type", o.contentType, o.metadata)
	}
}

func TestUploadOfMoreThanFiveGiBIsRefusedBeforeAnyRequest(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	path := filepath.Join(t.TempDir(), "huge")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxObject + 1); err != nil {
		t.Skipf("no sparse files here: %v", err)
	}
	_ = f.Close()
	g.s3.reset()
	msg := g.fails("cp", path, "s3://photos/huge")
	if !strings.Contains(msg, "at most 5.0 GiB") {
		t.Errorf("error %q does not name the 5 GiB cap", msg)
	}
	if n := len(g.s3.requests("")); n != 0 {
		t.Errorf("%d requests were made for an object that cannot be stored", n)
	}
}

func TestRecursiveUploadSkipsWhatIsNotARegularFileWithOneWarningEach(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	writeFile(t, dir, "tree/a.txt", 5)
	writeFile(t, dir, "tree/sub/b.txt", 5)
	if err := os.Symlink(filepath.Join(dir, "tree", "a.txt"), filepath.Join(dir, "tree", "link")); err != nil {
		t.Skip(err)
	}
	_, stderr, err := g.bex("cp", filepath.Join(dir, "tree"), "s3://photos/t", "--recursive", "--exclude", "*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(stderr, "warning: skipping") != 1 || !strings.Contains(stderr, "a symbolic link") {
		t.Errorf("stderr = %q, want one warning for the link", stderr)
	}
	if keys := g.s3.keys("photos"); !slices.Equal(keys, []string{"t/a.txt", "t/sub/b.txt"}) {
		t.Errorf("keys = %v", keys)
	}
	var doc struct{ Results []result }
	g.s3.reset()
	if err := json.Unmarshal([]byte(g.must("cp", filepath.Join(dir, "tree"), "s3://photos/u", "--recursive", "-o", "json")), &doc); err != nil || len(doc.Results) != 2 {
		t.Errorf("-o json = %+v, %v", doc, err)
	}
}

func TestRecursiveDownloadRefusesNamesThatEscapeTheDirectory(t *testing.T) {
	g := newRig(t)
	g.s3.put("photos", "t/ok", []byte("ok"), testNow)
	g.s3.put("photos", "t/../evil", []byte("evil"), testNow)
	g.s3.put("photos", "t/dir/", nil, testNow) // a folder marker
	out := t.TempDir()
	dest := filepath.Join(out, "dest")
	_, stderr, err := g.bex("cp", "s3://photos/t", dest, "--recursive")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "stays inside") {
		t.Errorf("stderr = %q, want a warning for the name that escapes", stderr)
	}
	if _, err := os.Stat(filepath.Join(out, "evil")); err == nil {
		t.Error("an object wrote outside the destination")
	}
	if got := entriesOf(t, dest); !slices.Equal(got, []string{"ok"}) {
		t.Errorf("files = %v", got)
	}
	if !isDir(filepath.Join(dest, "dir")) {
		t.Error("a folder marker did not make its directory")
	}
}
