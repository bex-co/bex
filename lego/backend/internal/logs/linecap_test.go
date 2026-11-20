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

package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w3/m506: a live tail drops what the shipper drops for history — an
// instance's application lines beyond Render's 6,000 a minute.

var capEpoch = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

func appLine(instance string, at time.Time, msg string) LogEntry {
	return newLogEntry("web", instance, core.AppContainer, LogTypeApp, at.Format(time.RFC3339Nano), msg)
}

// floodLines is n lines of instance, spread evenly over d from capEpoch.
func floodLines(instance string, n int, d time.Duration) []LogEntry {
	out := make([]LogEntry, n)
	for i := range out {
		out[i] = appLine(instance, capEpoch.Add(d*time.Duration(i)/time.Duration(n)), fmt.Sprintf("%s line %d", instance, i))
	}
	return out
}

// perMinute counts the admitted lines of each minute after capEpoch.
func perMinute(t *testing.T, entries []LogEntry) map[int]int {
	t.Helper()
	counts := map[int]int{}
	for _, e := range entries {
		at, err := time.Parse(time.RFC3339Nano, e.Timestamp)
		if err != nil {
			t.Fatalf("unstamped admitted line %+v", e)
		}
		counts[int(at.Sub(capEpoch)/time.Minute)]++
	}
	return counts
}

func TestLineCapKeepsOneBurstThenRendersRateAndNothingOfANeighbor(t *testing.T) {
	// Three minutes at 20,000 lines a minute, beside a neighbor printing a
	// line a second.
	flood := floodLines("web-1", 60000, 3*time.Minute)
	quiet := floodLines("web-2", 180, 3*time.Minute)
	// The tail sees both instances' lines interleaved in time.
	lines := slices.SortedStableFunc(slices.Values(slices.Concat(flood, quiet)), func(a, b LogEntry) int {
		return strings.Compare(a.Timestamp, b.Timestamp)
	})
	c := newLineCap()
	var kept, keptQuiet []LogEntry
	for _, e := range lines {
		switch {
		case !c.admit(e):
		case e.Labels[LabelInstance] == "web-1":
			kept = append(kept, e)
		default:
			keptQuiet = append(keptQuiet, e)
		}
	}
	got := perMinute(t, kept)
	// The first minute: the 6,000-line burst plus a minute of refill; every
	// later minute: the refill alone, Render's 6,000.
	if got[0] < 11990 || got[0] > appLineBurst+60*appLineRate {
		t.Errorf("first minute kept %d lines, want one burst plus a minute's refill (≈12,000)", got[0])
	}
	for minute := 1; minute < 3; minute++ {
		if got[minute] < 5990 || got[minute] > 6001 {
			t.Errorf("minute %d kept %d lines, want Render's 6,000", minute, got[minute])
		}
	}
	if len(keptQuiet) != len(quiet) {
		t.Errorf("the neighbor kept %d of %d lines; one instance's flood must cost only its own lines", len(keptQuiet), len(quiet))
	}
}

func TestLineCapCountsByTheLinesOwnTime(t *testing.T) {
	c := newLineCap()
	// 20,000 lines written in the same instant keep exactly the burst,
	// however long the tail takes to read them.
	burst := floodLines("web-1", 20000, 0)
	kept := 0
	for _, e := range burst {
		if c.admit(e) {
			kept++
		}
	}
	if kept != appLineBurst {
		t.Fatalf("an instant flood kept %d lines, want the %d-line burst", kept, appLineBurst)
	}
	// A line stamped earlier (stderr behind stdout) refills nothing; the rest
	// of a split line, which carries no stamp, is counted at the last time.
	for _, e := range []LogEntry{appLine("web-1", capEpoch.Add(-time.Second), "older"),
		newLogEntry("web", "web-1", core.AppContainer, LogTypeApp, "", "unstamped rest")} {
		if c.admit(e) {
			t.Errorf("%q passed an empty bucket", e.Message)
		}
	}
	// One second later the bucket holds a second's refill.
	later := floodLines("web-1", 200, 0)
	kept = 0
	for _, e := range later {
		e.Timestamp = capEpoch.Add(time.Second).Format(time.RFC3339Nano)
		if c.admit(e) {
			kept++
		}
	}
	if kept != appLineRate {
		t.Fatalf("a second later %d lines passed, want the %d-line refill", kept, appLineRate)
	}
}

func TestLineCapLeavesPlatformGeneratedLinesUncapped(t *testing.T) {
	c := newLineCap()
	for _, e := range floodLines("web-1", appLineBurst, 0) {
		c.admit(e)
	}
	at := capEpoch.Format(time.RFC3339Nano)
	for name, e := range map[string]LogEntry{
		"request line":    {Timestamp: at, Message: "GET /", Labels: map[string]string{LabelType: LogTypeRequest, LabelInstance: "web-1"}},
		"postgres line":   newLogEntry("dpg", "web-1", "postgres", "postgres", at, "checkpoint"),
		"key value line":  newLogEntry("red", "web-1", "valkey", "keyvalue", at, "saved"),
		"build line":      newLogEntry("web", "web-1", "build", LogTypeBuild, at, "step 1"),
		"bex's own line":  newLogEntry("web", "web-1", progressContainer, LogTypeApp, at, logsGoneMessage),
		"other instance":  appLine("web-2", capEpoch, "neighbor"),
		"capped instance": appLine("web-1", capEpoch, "over the cap"),
	} {
		if want := name != "capped instance"; c.admit(e) != want {
			t.Errorf("%s: admit = %v, want %v", name, !want, want)
		}
	}
}

// settledTail runs a tail of q on svc until the lines it emits stop growing,
// then closes it and returns them. A scripted pod log is read once and ends.
// A non-nil read reports whether every pod's log has been read to its end: a
// follow replaying its warm-up emits nothing for a while.
func settledTail(t *testing.T, svc *Service, q LogQuery, read func() bool) []LogEntry {
	t.Helper()
	var mu sync.Mutex
	var got []LogEntry
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- svc.followLogs(ctx, q, func(e LogEntry) error {
			mu.Lock()
			defer mu.Unlock()
			got = append(got, e)
			return nil
		}, nil)
	}()
	deadline := time.Now().Add(30 * time.Second)
	for last, settled := -1, 0; settled < 5; {
		if time.Now().After(deadline) {
			t.Fatalf("the tail never settled: %d lines", last)
		}
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n == last && n > 0 && (read == nil || read()) {
			settled++
		} else {
			last, settled = n, 0
		}
	}
	cancel()
	<-done
	mu.Lock()
	defer mu.Unlock()
	return slices.Clone(got)
}

// byPod groups emitted lines by the pod that wrote them: a tail names an
// instance by its public id, and the fixture's lines start with their pod's
// name.
func byPod(entries []LogEntry) map[string][]LogEntry {
	out := map[string][]LogEntry{}
	for _, e := range entries {
		pod, _, _ := strings.Cut(e.Message, " ")
		out[pod] = append(out[pod], e)
	}
	return out
}

// The cap applies on the tail path itself: a follow of an instance printing
// 20,000 lines a minute passes at most the cap and its neighbor everything,
// and a text filter selects among the capped lines, as a history read does.
func TestAppTailDropsAnInstancesExcessAsHistoryDoes(t *testing.T) {
	stream := func(lines []LogEntry) string {
		var b strings.Builder
		for _, e := range lines {
			fmt.Fprintf(&b, "%s %s\n", e.Timestamp, e.Message)
		}
		return b.String()
	}
	tail := func(q LogQuery) map[string][]LogEntry {
		logs := newScriptedStream(map[string][]string{
			"web-1": {stream(floodLines("web-1", 40000, 2*time.Minute))},
			"web-2": {stream(floodLines("web-2", 120, 2*time.Minute))},
		})
		svc := tailService(logs.follow, sampleApp("web"),
			appPodIn("web", "web-1", corev1.PodRunning, containerRunning, 0),
			appPodIn("web", "web-2", corev1.PodRunning, containerRunning, 0))
		got := byPod(settledTail(t, svc, q, nil))
		if q.Search == nil {
			for minute, n := range perMinute(t, got["web-1"]) {
				if minute == 0 && n > appLineBurst+60*appLineRate || minute > 0 && n > 6001 {
					t.Errorf("minute %d of the flood passed %d lines", minute, n)
				}
			}
		}
		return got
	}

	all := tail(LogQuery{App: "web"})
	// One burst, then two minutes of refill.
	if n := len(all["web-1"]); n < appLineBurst+120*appLineRate-1 || n > appLineBurst+120*appLineRate {
		t.Errorf("the flooding instance's tail passed %d of 40,000 lines, want %d", n, appLineBurst+120*appLineRate)
	}
	if n := len(all["web-2"]); n != 120 {
		t.Errorf("the neighbor's tail passed %d of 120 lines", n)
	}
	// A filter narrows the capped lines; it never brings back a dropped one.
	var want []string
	for _, m := range messages(all["web-1"]) {
		if strings.Contains(m, "web-1 line 1") {
			want = append(want, m)
		}
	}
	if got := messages(tail(LogQuery{App: "web", Search: []string{"web-1 line 1"}})["web-1"]); !slices.Equal(got, want) {
		t.Errorf("a text filter kept %d lines, want the %d capped lines that match", len(got), len(want))
	}
}

// sinceStream serves each pod's lines from since on, as the kubelet's
// SinceTime does. read reports whether every pod's log has been read to its
// end.
func sinceStream(lines map[string][]LogEntry) (stream PodLogStream, read func() bool) {
	var ended atomic.Int32
	stream = func(_ context.Context, _, pod, _ string, since time.Time) (io.ReadCloser, error) {
		var b strings.Builder
		for _, e := range lines[pod] {
			if at, _ := time.Parse(time.RFC3339Nano, e.Timestamp); !at.Before(since) {
				fmt.Fprintf(&b, "%s %s\n", e.Timestamp, e.Message)
			}
		}
		return io.NopCloser(&endReader{Reader: strings.NewReader(b.String()), ended: &ended}), nil
	}
	return stream, func() bool { return int(ended.Load()) >= len(lines) }
}

// endReader counts itself in ended once it has been read to its end.
type endReader struct {
	io.Reader
	ended *atomic.Int32
	once  sync.Once
}

func (r *endReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF {
		r.once.Do(func() { r.ended.Add(1) })
	}
	return n, err
}

// keptFrom is what the shipper keeps of one instance's whole log, from its
// first line, stamped at or after from: the lines' messages, in order.
func keptFrom(lines []LogEntry, from time.Time) []string {
	c := newLineCap()
	var kept []string
	for _, e := range lines {
		if at, _ := time.Parse(time.RFC3339Nano, e.Timestamp); c.admit(e) && !at.Before(from) {
			kept = append(kept, e.Message)
		}
	}
	return kept
}

// burstLines is instance's flood in the w8/341 replay's shape: bursts of size
// lines from a shell loop that sleeps 0.1 s between them, so one every
// 105.3 ms (about 190 lines a second at 20 a burst), the lines of a burst
// 23.7 µs apart, off the 10 ms grid.
func burstLines(instance string, bursts, size int) []LogEntry {
	out := make([]LogEntry, 0, bursts*size)
	for k := range bursts {
		for j := range size {
			at := capEpoch.Add(3700*time.Microsecond + time.Duration(k)*105300*time.Microsecond + time.Duration(j)*23700*time.Nanosecond)
			out = append(out, appLine(instance, at, fmt.Sprintf("%s seq=%d", instance, k*size+j)))
		}
	}
	return out
}

// w8/341: a tail opened while an instance floods starts with the bucket the
// shipper has for it, not a full one. A pod that has printed 20-line bursts
// every 105 ms for five minutes passes the lines history keeps from the
// moment the tail opens, the same lines and not only as many (w8/m525); it
// used to pass a fresh 6,000-line burst on top, and then 37.7% other lines.
// A quiet pod still passes its whole burst, and nothing written before the
// tail's bound is shown.
func TestATailOpenedMidFloodShowsWhatHistoryKeeps(t *testing.T) {
	opened := capEpoch.Add(5 * time.Minute)
	flood := burstLines("web-1", 3420, 20)           // 190 a second for 6 minutes
	quiet := floodLines("web-2", 300, 5*time.Minute) // one a second
	for i := range 20000 {
		quiet = append(quiet, appLine("web-2", opened, fmt.Sprintf("web-2 burst %d", i)))
	}
	stream, read := sinceStream(map[string][]LogEntry{"web-1": flood, "web-2": quiet})
	svc := tailService(stream, sampleApp("web"),
		appPodIn("web", "web-1", corev1.PodRunning, containerRunning, 0),
		appPodIn("web", "web-2", corev1.PodRunning, containerRunning, 0))
	got := byPod(settledTail(t, svc, LogQuery{App: "web", Since: opened}, read))

	for pod, lines := range got {
		for _, e := range lines {
			if at, err := time.Parse(time.RFC3339Nano, e.Timestamp); err != nil || at.Before(opened) {
				t.Fatalf("%s: the tail showed %q stamped %s, before it opened", pod, e.Message, e.Timestamp)
			}
		}
	}
	// History keeps about 100 of each second's 190 lines: the first 10 or 11
	// of each burst.
	if want, kept := keptFrom(flood, opened), messages(got["web-1"]); !slices.Equal(kept, want) {
		t.Errorf("the flooding instance's tail passed %d lines, %d of them missing from the %d history keeps",
			len(kept), notIn(kept, want), len(want))
	}
	if n := len(got["web-2"]); n != appLineBurst {
		t.Errorf("the quiet instance's tail passed %d of its 20,000-line burst, want the %d-line burst", n, appLineBurst)
	}
}

// A deployment without the durable store reads history from the pods. It keeps
// the same cap, so it shows what the store would hold: of 20,000 lines written
// at once the store keeps the first 6,000, so the last 100 are gone, while a
// neighbor keeps every line.
func TestStorelessHistoryKeepsTheCap(t *testing.T) {
	svc := &Service{
		Base: &core.Base{Client: fakeClientWith(sampleApp("web"), podFor("web", "web-1"), podFor("web", "web-2")), Namespace: "default"},
		PodLogs: tailedLogs(map[string][]LogEntry{
			"web-1": floodLines("web-1", 20000, 0),
			"web-2": floodLines("web-2", 30, 0),
		}),
	}
	entries, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Limit: 100})
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	got := map[string]int{}
	for pod, lines := range byPod(entries) {
		got[pod] = len(lines)
	}
	if got["web-1"] != 0 || got["web-2"] != 30 {
		t.Fatalf("store-less history kept %v, want none of the flood's last lines and every neighbor line", got)
	}
}

// tailedLogs serves each pod's last tail lines, as the kubelet's TailLines
// does; tail <= 0 serves them all.
func tailedLogs(lines map[string][]LogEntry) PodLogSource {
	return func(_ context.Context, _, pod, _ string, tail int64) (io.ReadCloser, error) {
		from := 0
		if n := len(lines[pod]); tail > 0 && int(tail) < n {
			from = n - int(tail)
		}
		var b strings.Builder
		for _, e := range lines[pod][from:] {
			fmt.Fprintf(&b, "%s %s\n", e.Timestamp, e.Message)
		}
		return io.NopCloser(strings.NewReader(b.String())), nil
	}
}

// w8/341: the store-less history read asks the kubelet for an instance's last
// lines, and those lines' bucket is the one the flood before them emptied. Of
// the last 100 lines of a pod printing 200 a second, it keeps the half the
// store would hold; it used to keep all 100.
func TestStorelessHistoryCountsTheFloodBeforeItsLines(t *testing.T) {
	flood := floodLines("web-1", 60000, 5*time.Minute) // 200 a second
	svc := &Service{
		Base:    &core.Base{Client: fakeClientWith(sampleApp("web"), podFor("web", "web-1")), Namespace: "default"},
		PodLogs: tailedLogs(map[string][]LogEntry{"web-1": flood}),
	}
	entries, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Limit: 100})
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	last100 := flood[len(flood)-100:]
	first, _ := time.Parse(time.RFC3339Nano, last100[0].Timestamp)
	if want := keptFrom(flood, first); !slices.Equal(messages(entries), want) {
		t.Fatalf("store-less history kept %d of the last 100 lines, want the %d the store holds: %q", len(entries), len(want), messages(entries))
	}
	for _, e := range entries {
		if at, _ := time.Parse(time.RFC3339Nano, e.Timestamp); at.Before(first) {
			t.Fatalf("store-less history showed %q from the warm-up", e.Message)
		}
	}
}

// notIn counts the lines of a that b lacks.
func notIn(a, b []string) int {
	have := make(map[string]bool, len(b))
	for _, m := range b {
		have[m] = true
	}
	n := 0
	for _, m := range a {
		if !have[m] {
			n++
		}
	}
	return n
}

// lineCapTable is testdata/linecap-shared.json: stamped lines in file order
// and whether their pod's bucket keeps each one. The log shipper's patched
// stage.limit is tested against the same table in its image build
// (deploy/log-shipper/Dockerfile), and scripts/test_log_shipper.py replays
// it, so the tail and history keep the same lines (w8/m525).
type lineCapTable struct {
	Rate  int64 `json:"rate"`
	Burst int64 `json:"burst"`
	Lines []struct {
		Pod   string `json:"pod"`
		TS    string `json:"ts"`
		Admit bool   `json:"admit"`
	} `json:"lines"`
}

func TestLineCapKeepsTheShippersLines(t *testing.T) {
	raw, err := os.ReadFile("testdata/linecap-shared.json")
	if err != nil {
		t.Fatal(err)
	}
	var table lineCapTable
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatal(err)
	}
	c := lineCapOf(table.Rate, table.Burst)
	kept := 0
	for i, l := range table.Lines {
		if got := c.admit(appLine(l.Pod, mustTime(t, l.TS), fmt.Sprintf("%s %d", l.Pod, i))); got != l.Admit {
			t.Errorf("line %d (%s at %s): admit = %v, want %v", i, l.Pod, l.TS, got, l.Admit)
		}
		if l.Admit {
			kept++
		}
	}
	// The table must tell a full bucket from an empty one.
	if kept == 0 || kept == len(table.Lines) {
		t.Fatalf("the table keeps %d of %d lines; it must keep some and drop some", kept, len(table.Lines))
	}
}

func mustTime(t *testing.T, stamp string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

// The tail's bucket for an instance starts full wherever its read starts,
// while the shipper's has been counting since the pod's first line. Once a
// flood has drained the tail's, both make the same decision on every line:
// whole tokens on one grid leave nothing between them, where fractional
// tokens refilled by elapsed time stayed out of step for good.
func TestLineCapBucketsConvergeOnceAFloodDrainsThem(t *testing.T) {
	flood := burstLines("web-1", 1710, 20) // 190 a second for 3 minutes
	shipper, tail := newLineCap(), newLineCap()
	tailFrom := len(flood) / 3 // the tail's bucket starts full a minute in
	drained, differed := -1, false
	for i, e := range flood {
		kept := shipper.admit(e)
		if i < tailFrom {
			continue
		}
		tailKept := tail.admit(e)
		switch {
		case drained >= 0 && tailKept != kept:
			t.Fatalf("line %d (%s): the tail kept %v and the shipper %v after both drained at line %d", i, e.Message, tailKept, kept, drained)
		case drained < 0 && !tailKept:
			drained = i
		case tailKept != kept:
			differed = true
		}
	}
	if drained < 0 || !differed {
		t.Fatalf("the tail's bucket never drained (%d) or never differed from the shipper's (%v): the flood proves nothing", drained, differed)
	}
}

// The kubelet stamps a split line once, with its first chunk's time, as the
// shipper does (cri-first-chunk-timestamp.patch). A follow that skipped the
// first chunk as older gets the rest unstamped: it is counted at its
// bucket's last step and refills nothing.
func TestLineCapCountsAnUnstampedRestAtTheLastStep(t *testing.T) {
	c := lineCapOf(appLineRate, 2)
	rest := newLogEntry("web", "web-1", core.AppContainer, LogTypeApp, "", "rest of a split line")
	for i, tc := range []struct {
		e    LogEntry
		want bool
	}{
		{appLine("web-1", capEpoch, "first chunk"), true},
		{rest, true},
		{rest, false},
		// One step later, one token.
		{appLine("web-1", capEpoch.Add(10*time.Millisecond), "next line"), true},
		{rest, false},
	} {
		if got := c.admit(tc.e); got != tc.want {
			t.Errorf("line %d (%q): admit = %v, want %v", i, tc.e.Message, got, tc.want)
		}
	}
}
