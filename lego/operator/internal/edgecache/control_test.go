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

package edgecache

import (
	"fmt"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// kubeletFormat renders annotations the way the kubelet writes a downward-API
// metadata.annotations file (k8s.io/kubernetes/pkg/fieldpath.FormatMap).
func kubeletFormat(annotations map[string]string) []byte {
	lines := make([]string, 0, len(annotations))
	for _, k := range slices.Sorted(maps.Keys(annotations)) {
		lines = append(lines, fmt.Sprintf("%v=%q", k, annotations[k]))
	}
	return []byte(strings.Join(lines, "\n"))
}

func status(h http.Handler, target string) string {
	return get(h, "GET", "http://app.test"+target, nil).Header().Get(StatusHeader)
}

// A purge drops every stored entry; the next request for one is a MISS that
// reaches the app.
func TestPurgeEmptiesTheCache(t *testing.T) {
	o := &origin{}
	h := newCache(t, ProfileStatic, o, nil)
	status(h, "/app.css")
	if s := status(h, "/app.css"); s != "HIT" {
		t.Fatalf("before the purge = %s, want HIT", s)
	}
	h.Purge()
	if s := status(h, "/app.css"); s != "MISS" {
		t.Fatalf("after the purge = %s, want MISS", s)
	}
	if s := status(h, "/app.css"); s != "HIT" {
		t.Fatalf("refilled = %s, want HIT", s)
	}
	if o.hits.Load() != 2 {
		t.Fatalf("origin hits = %d, want 2 (one fill on each side of the purge)", o.hits.Load())
	}
}

// A fill that began before a purge is served to its own client but neither
// stored nor handed to the requests waiting on it: they were answered after
// the purge, so each goes to the app.
func TestAFillInFlightAcrossAPurgeIsNotStored(t *testing.T) {
	o := &origin{delay: 200 * time.Millisecond}
	h := newCache(t, ProfileStatic, o, nil)
	var wg sync.WaitGroup
	statuses := make([]string, 3)
	wg.Go(func() { statuses[0] = status(h, "/app.css") })
	deadline := time.Now().Add(5 * time.Second)
	for o.hits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	wg.Go(func() { statuses[1] = status(h, "/app.css") }) // joins the fill
	time.Sleep(20 * time.Millisecond)
	h.Purge()
	wg.Wait()
	if statuses[0] != "MISS" || statuses[1] != "MISS" {
		t.Fatalf("the fill and its waiter = %v, want both MISS", statuses[:2])
	}
	if s := status(h, "/app.css"); s != "MISS" {
		t.Fatalf("after a purge during the fill = %s, want MISS: a pre-purge response was stored", s)
	}
	if s := status(h, "/app.css"); s != "HIT" {
		t.Fatalf("the post-purge fill = %s, want HIT", s)
	}
}

// A profile switch applies from the next request and drops what the old
// profile stored; any value but the two caching profiles passes through.
func TestSetProfileSwitchesAtOnce(t *testing.T) {
	o := &origin{}
	h := newCache(t, ProfileStatic, o, nil)
	status(h, "/app.css")
	if s := status(h, "/page"); s != "DYNAMIC" {
		t.Fatalf("a page on the static profile = %s, want DYNAMIC", s)
	}
	h.SetProfile(ProfileAll)
	if s := status(h, "/app.css"); s != "MISS" {
		t.Fatalf("a .css after the switch = %s, want MISS (the switch purges)", s)
	}
	status(h, "/page")
	if s := status(h, "/page"); s != "HIT" {
		t.Fatalf("a page on the all profile = %s, want HIT", s)
	}
	h.SetProfile(ProfileNone)
	for _, path := range []string{"/app.css", "/page"} {
		if s := status(h, path); s != "DYNAMIC" {
			t.Fatalf("%s with caching off = %s, want DYNAMIC", path, s)
		}
	}
	before := o.hits.Load()
	status(h, "/page")
	if o.hits.Load() != before+1 {
		t.Fatal("with caching off a request must reach the app")
	}
	h.SetProfile(ProfileStatic)
	if s := status(h, "/page"); s != "DYNAMIC" {
		t.Fatalf("a page back on the static profile = %s, want DYNAMIC", s)
	}
}

func writeControl(t *testing.T, path string, annotations map[string]string) {
	t.Helper()
	// The kubelet swaps the whole file in; a rename is the same for a reader.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, kubeletFormat(annotations), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}
}

// The control file drives the running proxy: the first read records the
// purge stamp a new pod starts with, a later stamp purges, and the profile
// annotation replaces the start-up profile until it is removed.
func TestTheControlFilePurgesAndSwitchesARunningProxy(t *testing.T) {
	o := &origin{}
	h := newCache(t, ProfileStatic, o, nil)
	path := filepath.Join(t.TempDir(), "annotations")
	annotations := map[string]string{"kubernetes.io/config.seen": "2026-11-18T10:00:00Z", PurgedAtAnnotation: "2026-11-18T09:00:00Z"}
	writeControl(t, path, annotations)
	var logged []string
	c := NewController(h, path)
	c.Logf = func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }
	status(h, "/app.css")
	if err := c.Sync(); err != nil {
		t.Fatal(err)
	}
	if s := status(h, "/app.css"); s != "HIT" {
		t.Fatalf("after the first read = %s, want HIT: the stamp a pod starts with is no purge", s)
	}

	annotations[PurgedAtAnnotation] = "2026-11-18T10:30:00.123456789Z"
	writeControl(t, path, annotations)
	if err := c.Sync(); err != nil {
		t.Fatal(err)
	}
	if s := status(h, "/app.css"); s != "MISS" {
		t.Fatalf("after a new purge stamp = %s, want MISS", s)
	}
	if err := c.Sync(); err != nil {
		t.Fatal(err)
	}
	if s := status(h, "/app.css"); s != "HIT" {
		t.Fatalf("an unchanged file purged again: %s, want HIT", s)
	}

	annotations[ProfileAnnotation] = ProfileAll
	writeControl(t, path, annotations)
	if err := c.Sync(); err != nil {
		t.Fatal(err)
	}
	if h.Profile() != ProfileAll {
		t.Fatalf("profile = %q, want the annotation's %q", h.Profile(), ProfileAll)
	}
	annotations[ProfileAnnotation] = ProfileNone
	writeControl(t, path, annotations)
	if err := c.Sync(); err != nil {
		t.Fatal(err)
	}
	if s := status(h, "/app.css"); s != "DYNAMIC" {
		t.Fatalf("with the annotation at no-cache = %s, want DYNAMIC", s)
	}
	delete(annotations, ProfileAnnotation)
	writeControl(t, path, annotations)
	if err := c.Sync(); err != nil {
		t.Fatal(err)
	}
	if h.Profile() != ProfileStatic {
		t.Fatalf("profile without the annotation = %q, want the start-up %q", h.Profile(), ProfileStatic)
	}
	want := []string{
		`edge-cache: purged (app.bex.co/cache-purged-at=2026-11-18T10:30:00.123456789Z)`,
		`edge-cache: profile "origin-controlled" -> "origin-controlled-all"`,
		`edge-cache: profile "origin-controlled-all" -> "no-cache"`,
		`edge-cache: profile "no-cache" -> "origin-controlled"`,
	}
	if !slices.Equal(logged, want) {
		t.Fatalf("logged\n  %q\nwant\n  %q", logged, want)
	}
}

// A file that cannot be read changes nothing: the proxy keeps caching by what
// it last read.
func TestAnUnreadableControlFileChangesNothing(t *testing.T) {
	o := &origin{}
	h := newCache(t, ProfileStatic, o, nil)
	c := NewController(h, filepath.Join(t.TempDir(), "missing"))
	status(h, "/app.css")
	if err := c.Sync(); err == nil {
		t.Fatal("a missing control file read without an error")
	}
	if s := status(h, "/app.css"); s != "HIT" || h.Profile() != ProfileStatic {
		t.Fatalf("after a failed read: %s on %q, want a HIT on the start-up profile", s, h.Profile())
	}
}

// Values the kubelet escapes, and lines that are not key="value", parse as
// the map the pod carries.
func TestParseAnnotationsReadsTheKubeletFormat(t *testing.T) {
	want := map[string]string{
		PurgedAtAnnotation:  "2026-11-18T10:30:00Z",
		ProfileAnnotation:   ProfileAll,
		"example.com/multi": "line one\nline \"two\"=x",
		"example.com/empty": "",
	}
	raw := append(kubeletFormat(want), []byte("\nnot a line\nbroken=\"unterminated")...)
	if got := parseAnnotations(raw); !maps.Equal(got, want) {
		t.Fatalf("parsed %q, want %q", got, want)
	}
}

// A tier pod outlives the app's instances, so a deploy empties it through its
// cache-revision annotation: a new revision after the first read purges, the
// revision the pod starts with does not, and a purge stamp and a revision
// arriving together purge once.
func TestANewCacheRevisionPurgesARunningProxy(t *testing.T) {
	o := &origin{}
	h := newCache(t, ProfileStatic, o, nil)
	path := filepath.Join(t.TempDir(), "annotations")
	annotations := map[string]string{RevisionAnnotation: "rev-1", PurgedAtAnnotation: "2026-11-18T09:00:00Z"}
	writeControl(t, path, annotations)
	var logged []string
	c := NewController(h, path)
	c.Logf = func(format string, args ...any) { logged = append(logged, fmt.Sprintf(format, args...)) }
	status(h, "/app.css")
	if err := c.Sync(); err != nil {
		t.Fatal(err)
	}
	if s := status(h, "/app.css"); s != "HIT" {
		t.Fatalf("after the first read = %s, want HIT: the revision a pod starts with is no purge", s)
	}
	annotations[RevisionAnnotation] = "rev-2"
	writeControl(t, path, annotations)
	if err := c.Sync(); err != nil {
		t.Fatal(err)
	}
	if s := status(h, "/app.css"); s != "MISS" {
		t.Fatalf("after a new revision = %s, want MISS", s)
	}
	if err := c.Sync(); err != nil {
		t.Fatal(err)
	}
	if s := status(h, "/app.css"); s != "HIT" {
		t.Fatalf("an unchanged revision purged again: %s, want HIT", s)
	}
	annotations[RevisionAnnotation] = "rev-3"
	annotations[PurgedAtAnnotation] = "2026-11-18T11:00:00Z"
	writeControl(t, path, annotations)
	if err := c.Sync(); err != nil {
		t.Fatal(err)
	}
	if s := status(h, "/app.css"); s != "MISS" {
		t.Fatalf("after a new revision and stamp = %s, want MISS", s)
	}
	want := []string{
		`edge-cache: purged (app.bex.co/cache-revision=rev-2)`,
		`edge-cache: purged (app.bex.co/cache-purged-at=2026-11-18T11:00:00Z, app.bex.co/cache-revision=rev-3)`,
	}
	if !slices.Equal(logged, want) {
		t.Fatalf("logged\n  %q\nwant\n  %q", logged, want)
	}
	if o.hits.Load() != 3 {
		t.Fatalf("origin hits = %d, want 3: the fill and one refill after each purge", o.hits.Load())
	}
}
