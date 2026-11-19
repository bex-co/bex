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
	"fmt"
	"slices"
	"strings"
	"sync"
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
	c := lineCap{}
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
	c := lineCap{}
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
	c := lineCap{}
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
	tail := func(q LogQuery) map[string][]string {
		logs := newScriptedStream(map[string][]string{
			"web-1": {stream(floodLines("web-1", 40000, 2*time.Minute))},
			"web-2": {stream(floodLines("web-2", 120, 2*time.Minute))},
		})
		svc := tailService(logs.follow, sampleApp("web"),
			appPodIn("web", "web-1", corev1.PodRunning, containerRunning, 0),
			appPodIn("web", "web-2", corev1.PodRunning, containerRunning, 0))
		var mu sync.Mutex
		got := map[string][]string{}
		var stamps []LogEntry
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- svc.followLogs(ctx, q, func(e LogEntry) error {
				mu.Lock()
				defer mu.Unlock()
				// The tail names an instance by its public id; the fixture's
				// lines carry their pod's name.
				pod, _, _ := strings.Cut(e.Message, " ")
				got[pod] = append(got[pod], e.Message)
				if pod == "web-1" {
					stamps = append(stamps, e)
				}
				return nil
			}, nil)
		}()
		// Each scripted log is read once and ends: wait until the emitted
		// count stops growing.
		deadline := time.Now().Add(30 * time.Second)
		for last, settled := -1, 0; settled < 5; {
			if time.Now().After(deadline) {
				t.Fatalf("the tail never settled: %d lines of web-1, %d of web-2", len(got["web-1"]), len(got["web-2"]))
			}
			time.Sleep(20 * time.Millisecond)
			mu.Lock()
			n := len(got["web-1"]) + len(got["web-2"])
			mu.Unlock()
			if n == last && n > 0 {
				settled++
			} else {
				last, settled = n, 0
			}
		}
		cancel()
		<-done
		if q.Search == nil {
			for minute, n := range perMinute(t, stamps) {
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
	for _, m := range all["web-1"] {
		if strings.Contains(m, "web-1 line 1") {
			want = append(want, m)
		}
	}
	if got := tail(LogQuery{App: "web", Search: []string{"web-1 line 1"}})["web-1"]; !slices.Equal(got, want) {
		t.Errorf("a text filter kept %d lines, want the %d capped lines that match", len(got), len(want))
	}
}

// A deployment without the durable store reads history from the pods. It keeps
// the same cap, so it shows what the store would hold.
func TestStorelessHistoryKeepsTheCap(t *testing.T) {
	var flood, quiet []string
	for _, e := range floodLines("web-1", 20000, 0) {
		flood = append(flood, e.Timestamp+" "+e.Message)
	}
	for _, e := range floodLines("web-2", 30, 0) {
		quiet = append(quiet, e.Timestamp+" "+e.Message)
	}
	svc := newService(map[string][]string{"web-1": flood, "web-2": quiet},
		sampleApp("web"), podFor("web", "web-1"), podFor("web", "web-2"))
	entries, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Limit: 100000})
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	got := map[string]int{}
	for _, e := range entries {
		pod, _, _ := strings.Cut(e.Message, " ")
		got[pod]++
	}
	if got["web-1"] != appLineBurst || got["web-2"] != 30 {
		t.Fatalf("store-less history kept %v, want the flood's %d-line burst and every neighbor line", got, appLineBurst)
	}
}
