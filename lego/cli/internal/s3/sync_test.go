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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// sliceSource is entries in key order, counting how far the merge has read.
type sliceSource struct {
	entries []entry
	read    int
}

func (s *sliceSource) next() (entry, bool, error) {
	if s.read >= len(s.entries) {
		return entry{}, false, nil
	}
	s.read++
	return s.entries[s.read-1], true, nil
}

func at(sec int) time.Time { return time.Date(2026, 1, 1, 0, 0, sec, 0, time.UTC) }

// TestSyncDecisionTable is the rule of sync, key by key.
func TestSyncDecisionTable(t *testing.T) {
	src := []entry{
		{Key: "a-missing", Size: 1, Modified: at(5)},
		{Key: "b-same", Size: 2, Modified: at(5)},
		{Key: "c-size-differs", Size: 3, Modified: at(5)},
		{Key: "d-source-newer", Size: 4, Modified: at(9)},
		{Key: "e-destination-newer", Size: 5, Modified: at(5)},
		{Key: "f-same-second", Size: 6, Modified: at(5).Add(900 * time.Millisecond)},
		{Key: "h-newer-and-same-size", Size: 8, Modified: at(9)},
	}
	dst := []entry{
		{Key: "b-same", Size: 2, Modified: at(5)},
		{Key: "c-size-differs", Size: 99, Modified: at(50)},
		{Key: "d-source-newer", Size: 4, Modified: at(1)},
		{Key: "e-destination-newer", Size: 5, Modified: at(50)},
		{Key: "f-same-second", Size: 6, Modified: at(5)},
		{Key: "g-destination-only", Size: 7, Modified: at(5)},
		{Key: "h-newer-and-same-size", Size: 8, Modified: at(1)},
	}
	for _, tc := range []struct {
		name                 string
		sizeOnly, deleteMode bool
		want                 []string
	}{
		{"default", false, false, []string{"copy a-missing", "copy c-size-differs", "copy d-source-newer", "copy h-newer-and-same-size"}},
		{"--delete", false, true, []string{"copy a-missing", "copy c-size-differs", "copy d-source-newer", "delete g-destination-only", "copy h-newer-and-same-size"}},
		{"--size-only", true, false, []string{"copy a-missing", "copy c-size-differs"}},
	} {
		var got []string
		err := merge(&sliceSource{entries: src}, &sliceSource{entries: dst}, tc.sizeOnly, tc.deleteMode, func(s syncStep) bool {
			if s.copy {
				got = append(got, "copy "+s.src.Key)
			} else {
				got = append(got, "delete "+s.dst.Key)
			}
			return true
		})
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("%s: steps = %v, %v; want %v", tc.name, got, err, tc.want)
		}
	}
	// Either side empty.
	var got []string
	_ = merge(&sliceSource{}, &sliceSource{entries: dst}, false, true, func(s syncStep) bool { got = append(got, s.dst.Key); return true })
	if len(got) != len(dst) {
		t.Errorf("an empty source with --delete deleted %v, want all %d", got, len(dst))
	}
	got = nil
	_ = merge(&sliceSource{entries: src}, &sliceSource{}, false, false, func(s syncStep) bool { got = append(got, s.src.Key); return true })
	if len(got) != len(src) {
		t.Errorf("an empty destination copied %v, want all %d", got, len(src))
	}
}

// TestMergeReadsOneEntryAheadOfEachSide: it never holds more than what it is
// comparing, so a tree of a million keys is not in memory.
func TestMergeReadsOneEntryAheadOfEachSide(t *testing.T) {
	mk := func(n, step int) []entry {
		out := make([]entry, n)
		for i := range out {
			out[i] = entry{Key: fmt.Sprintf("k%08d", i*step), Size: 1, Modified: at(1)}
		}
		return out
	}
	src, dst := &sliceSource{entries: mk(1000, 2)}, &sliceSource{entries: mk(1000, 3)}
	steps := 0
	_ = merge(src, dst, false, true, func(syncStep) bool {
		steps++
		if steps == 100 && (src.read > 101 || dst.read > 101) {
			t.Errorf("after 100 steps the merge had read %d and %d entries", src.read, dst.read)
		}
		return true
	})
	if src.read != 1000 || dst.read != 1000 {
		t.Errorf("read %d and %d, want each side once, in full", src.read, dst.read)
	}
}

// TestListingSourceHoldsOnePageAtATime: the next page is asked for only when the
// merge has used up the last, so at most a page of a listing is in memory.
func TestListingSourceHoldsOnePageAtATime(t *testing.T) {
	g := newRig(t)
	g.s3.pageSize = 10
	for _, key := range keysOf(55, "k%03d") {
		g.s3.put("photos", key, []byte("x"), testNow)
	}
	g.must("mb", "s3://elsewhere")
	g.s3.reset()
	r, err := g.d.open(newTestCommand(), &globals{}, 4, true)
	if err != nil {
		t.Fatal(err)
	}
	defer r.close()
	src := r.listSource(context.Background(), "photos", "", nil)
	for i := 1; i <= 55; i++ {
		e, ok, err := src.next()
		if err != nil || !ok || e.Key != fmt.Sprintf("k%03d", i-1) {
			t.Fatalf("entry %d = %+v, %v, %v", i, e, ok, err)
		}
		if got, want := g.s3.count("list-type=2"), (i+9)/10; got != want {
			t.Fatalf("after %d entries %d list calls were made, want %d: the listing reads ahead", i, got, want)
		}
		if len(src.page) > 10 {
			t.Fatalf("a page of %d entries is held", len(src.page))
		}
	}
	if _, ok, _ := src.next(); ok {
		t.Error("more entries than the bucket holds")
	}
}

// TestLocalWalkYieldsKeysInTheOrderOfAListing: a directory sorts as its name and
// a slash, so the walk and a listing of the same tree can be merged in one pass.
func TestLocalWalkYieldsKeysInTheOrderOfAListing(t *testing.T) {
	dir := t.TempDir()
	var names []string
	for _, n := range []string{"a.txt", "a/b", "a-b", "a0", "a/b.c/d", "a/b2/c", "ä", "Z", "a b"} {
		writeFile(t, dir, n, 1)
		names = append(names, n)
	}
	var got []string
	if err := walkLocal(t.Context(), dir, func(string) {}, func(f localFile) error { got = append(got, f.Rel); return nil }); err != nil {
		t.Fatal(err)
	}
	want := slices.Clone(names)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("walk order  %q\nkey order   %q", got, want)
	}
}

func TestSyncTwiceCopiesNothingTheSecondTime(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	tree := map[string][]byte{}
	for name, size := range map[string]int{"a.txt": 10, "sub/b.bin": 9 << 20, "sub/deep/c.txt": 3, "empty": 0, "a b/é.txt": 4} {
		tree[name] = writeFile(t, dir, "src/"+name, size)
	}
	out := g.must("sync", filepath.Join(dir, "src"), "s3://photos/site")
	if got := strings.Count(out, "upload: "); got != 5 {
		t.Errorf("the first sync uploaded %d files, want 5:\n%s", got, out)
	}
	for name, data := range tree {
		if o, ok := g.s3.object("photos", "site/"+name); !ok || sum(o.data) != sum(data) {
			t.Errorf("site/%s is not the file", name)
		}
	}
	g.s3.reset()
	if out := g.must("sync", filepath.Join(dir, "src"), "s3://photos/site"); out != "" {
		t.Errorf("the second sync printed %q, want nothing", out)
	}
	for _, line := range g.s3.requests("") {
		if !strings.HasPrefix(line, "GET ") {
			t.Errorf("the second sync sent %q: it copied something", line)
		}
	}

	// And back down: a new directory gets everything, then nothing.
	back := filepath.Join(dir, "back")
	if out := g.must("sync", "s3://photos/site", back); strings.Count(out, "download: ") != 5 {
		t.Errorf("sync down printed:\n%s", out)
	}
	for name, data := range tree {
		sameBytes(t, filepath.Join(back, filepath.FromSlash(name)), data)
	}
	g.s3.reset()
	if out := g.must("sync", "s3://photos/site", back); out != "" {
		t.Errorf("the second sync down printed %q, want nothing", out)
	}
	if n := g.s3.count("GET /photos/site"); n != 0 {
		t.Errorf("the second sync down fetched %d objects", n)
	}
	// The round trip is stable the other way too: what was downloaded is not uploaded again.
	g.s3.reset()
	if out := g.must("sync", back, "s3://photos/site"); out != "" {
		t.Errorf("sync of the downloaded copy printed %q", out)
	}

	// Between two prefixes, through server-side copies, once and then never.
	if out := g.must("sync", "s3://photos/site", "s3://photos/mirror"); strings.Count(out, "copy: ") != 5 {
		t.Errorf("sync between prefixes printed:\n%s", out)
	}
	g.s3.reset()
	if out := g.must("sync", "s3://photos/site", "s3://photos/mirror"); out != "" || g.s3.count("x-amz-copy-source") != 0 {
		t.Errorf("the second prefix sync printed %q", out)
	}
	if keys := g.s3.keys("photos"); len(keys) != 10 {
		t.Errorf("keys = %v", keys)
	}
}

func TestSyncChangesAreCopiedAndDeleteRemovesExactlyTheExtras(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	writeBytes(t, dir, "src/keep", []byte("keep"))
	writeBytes(t, dir, "src/change", []byte("old"))
	writeBytes(t, dir, "src/skip.tmp", []byte("tmp"))
	g.must("sync", filepath.Join(dir, "src"), "s3://photos/p")
	g.s3.put("photos", "p/extra", []byte("extra"), testNow)
	g.s3.put("photos", "p/sub/extra2", []byte("extra"), testNow)
	g.s3.put("photos", "p/also.tmp", []byte("excluded, so never deleted"), testNow)
	g.s3.put("photos", "other/keep", []byte("not under the prefix"), testNow)
	writeBytes(t, dir, "src/change", []byte("a new and longer content"))

	// --dryrun prints the plan and changes nothing.
	g.s3.reset()
	out := g.must("sync", filepath.Join(dir, "src"), "s3://photos/p", "--delete", "--exclude", "*.tmp", "--dryrun")
	for _, want := range []string{"(dryrun) upload: " + filepath.Join(dir, "src", "change"), "(dryrun) delete: s3://photos/p/extra", "(dryrun) delete: s3://photos/p/sub/extra2"} {
		if !strings.Contains(out, want) {
			t.Errorf("dryrun lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "also.tmp") || strings.Contains(out, "skip.tmp") {
		t.Errorf("an excluded path is in the plan:\n%s", out)
	}
	if len(g.s3.requests("PUT")) != 0 || len(g.s3.requests("POST")) != 0 || len(g.s3.requests("DELETE")) != 0 {
		t.Errorf("--dryrun wrote: %v", g.s3.requests(""))
	}

	g.must("sync", filepath.Join(dir, "src"), "s3://photos/p", "--delete", "--exclude", "*.tmp")
	if got := g.s3.keys("photos"); !slices.Equal(got, []string{"other/keep", "p/also.tmp", "p/change", "p/keep", "p/skip.tmp"}) {
		t.Errorf("keys = %v, want the extras gone and the excluded and unrelated ones kept", got)
	}
	if o, _ := g.s3.object("photos", "p/change"); string(o.data) != "a new and longer content" {
		t.Errorf("a changed file was not copied: %q", o.data)
	}

	// Locally too: --delete removes the file the bucket does not have.
	back := filepath.Join(dir, "back")
	g.must("sync", "s3://photos/p", back, "--exclude", "*.tmp")
	writeBytes(t, dir, "back/local-only", []byte("x"))
	g.must("sync", "s3://photos/p", back, "--exclude", "*.tmp", "--delete")
	if got := entriesOf(t, back); !slices.Equal(got, []string{"change", "keep"}) {
		t.Errorf("local files = %v", got)
	}
}

func TestSyncSizeOnlyIgnoresTimes(t *testing.T) {
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	writeBytes(t, dir, "src/f", []byte("1234"))
	g.s3.put("photos", "p/f", []byte("abcd"), time.Now().Add(-48*time.Hour).UTC())
	if out := g.must("sync", filepath.Join(dir, "src"), "s3://photos/p", "--size-only"); out != "" {
		t.Errorf("--size-only copied a same-size file: %q", out)
	}
	if out := g.must("sync", filepath.Join(dir, "src"), "s3://photos/p"); !strings.Contains(out, "upload:") {
		t.Errorf("a newer local file was not copied: %q", out)
	}
}

// TestSyncDoesNotDeleteWhatItCouldNotRead: a directory it cannot read stops the
// sync before any delete, since its files would look gone.
func TestSyncDoesNotDeleteWhatItCouldNotRead(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads every directory")
	}
	g := newRig(t)
	g.must("mb", "s3://photos")
	dir := t.TempDir()
	writeBytes(t, dir, "src/ok", []byte("x"))
	writeBytes(t, dir, "src/locked/secret", []byte("x"))
	g.must("sync", filepath.Join(dir, "src"), "s3://photos/p")
	if err := os.Chmod(filepath.Join(dir, "src", "locked"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "src", "locked"), 0o755) })
	g.s3.reset()
	g.fails("sync", filepath.Join(dir, "src"), "s3://photos/p", "--delete")
	if _, ok := g.s3.object("photos", "p/locked/secret"); !ok {
		t.Error("a file under an unreadable directory was deleted from the bucket")
	}
	if len(g.s3.requests("POST")) != 0 {
		t.Errorf("a sync that could not read its source deleted: %v", g.s3.requests("POST"))
	}
}

func TestSyncNeedsAPrefixOnOneSide(t *testing.T) {
	g := newRig(t)
	dir := t.TempDir()
	writeFile(t, dir, "f", 1)
	if msg := g.fails("sync", filepath.Join(dir, "f"), "s3://photos/x"); !strings.Contains(msg, "not a directory") {
		t.Errorf("sync of a file says %q", msg)
	}
}
