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
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/types/renderlog"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// scanSince is when the fixtures' stall first said no port was open.
var scanSince = time.Date(2026, 11, 21, 21, 23, 0, 0, time.UTC)

// scanRow is an image web deploy whose rollout scans for a port from
// scanSince.
func scanRow(status string, finished, until, read time.Time) DeployProgress {
	return DeployProgress{
		ID: "dep-scan", Status: status, Image: "docker.io/library/busybox:1.36",
		CreatedAt: scanSince.Add(-40 * time.Second), StartedAt: scanSince.Add(-30 * time.Second),
		FinishedAt: finished, PortScanSince: scanSince, PortScanUntil: until, ReadAt: read,
	}
}

// rendersTimeout is how Render's log closes a deploy that ran out its rollout
// budget (w8/391), in the bytes Render's log API returns (w32/m576).
var rendersTimeout = []string{rendersTimedOut, rendersTroubleshoot}

// at is scanSince plus d, as a line's timestamp.
func at(d time.Duration) string { return scanSince.Add(d).Format(time.RFC3339Nano) }

// Render's three narrations, restated here so the tests pin which lines and
// which docs line each finding carries, and the form the timeout takes: red
// for the plain wording, as Render's log API returns it (kush252/Sentry-AI),
// plain for the two no capture shows (w32/m576).
var (
	plainScan  = portScanWording{portScanLine, portScanTimeoutLine, portScanDocsLine, renderlog.PlatformFailure}
	localScan  = portScanWording{portScanLocalhostLine, portScanLocalhostTimeoutLine, portScanDocsLine, renderlog.Plain}
	noHTTPScan = portScanWording{portScanNoHTTPLine, portScanNoHTTPTimeoutLine, "", renderlog.Plain}
)

// said is msgs stamped at scanSince plus d, as scanStory lists them.
func said(d time.Duration, msgs ...string) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, at(d)+" "+m)
	}
	return out
}

// withDocs is line, then w's docs line when it has one, both in form f.
func (w portScanWording) withDocs(line string, f renderlog.Form) []string {
	if w.docs == "" {
		return []string{styled(f, line)}
	}
	return []string{styled(f, line), styled(f, w.docs)}
}

// scanPairs is w's scan line, with its docs line, at each of the given
// minutes into the scan.
func scanPairs(w portScanWording, minutes ...int) []string {
	var out []string
	for _, m := range minutes {
		out = append(out, said(time.Duration(m)*time.Minute, w.withDocs(w.scan, renderlog.Plain)...)...)
	}
	return out
}

// timedOut is w's timeout line, with its docs line, at d into the scan.
func timedOut(w portScanWording, d time.Duration) []string {
	return said(d, w.withDocs(w.timeout, w.timeoutForm)...)
}

// scanStory is the platform lines of a row from its first scan line on.
func scanStory(d DeployProgress) []string {
	var out []string
	for _, e := range progressLines(d, progressContext{}) {
		if e.Timestamp >= at(0) {
			out = append(out, e.Timestamp+" "+e.Message)
		}
	}
	return out
}

// TestPortScanLinesKeepRendersCadence is Render's cadence (w8/385, w8/389):
// the scan line when the stall first says no port is open, then one a
// minute, five at most, each followed by the docs line, then the timeout
// line and its docs line a minute after the fifth, and nothing more until
// the deploy ends. It stops when a stall says the scan is over or the deploy
// ends; a port found after the timeout line still goes live, and a deploy
// that failed scanning before the timeout line was due gets it as it fails.
func TestPortScanLinesKeepRendersCadence(t *testing.T) {
	const failed = "the container is running but its startup health check never succeeded within the rollout window, so the deploy failed: a TCP connect to any open port on the instance."
	failedScanning := func(after time.Duration) DeployProgress {
		d := scanRow("update_failed", scanSince.Add(after), time.Time{}, scanSince.Add(time.Hour))
		d.FailureReason = failed
		return d
	}
	open := func(read time.Duration) DeployProgress {
		return scanRow("update_in_progress", time.Time{}, time.Time{}, scanSince.Add(read))
	}
	// atBudget is d closed by its rollout budget.
	atBudget := func(d DeployProgress) DeployProgress {
		d.TimedOut = true
		return d
	}
	for _, c := range []struct {
		name string
		row  DeployProgress
		want []string
	}{
		{"open, read 2.5 minutes in", open(150 * time.Second), scanPairs(plainScan, 0, 1, 2)},
		{"open, read on a minute", open(time.Minute), scanPairs(plainScan, 0, 1)},
		{"open, read before the first line", open(-time.Second), nil},
		{"open, read just before the timeout", open(5*time.Minute - time.Second), scanPairs(plainScan, 0, 1, 2, 3, 4)},
		{"open, read on the timeout", open(5 * time.Minute),
			slices.Concat(scanPairs(plainScan, 0, 1, 2, 3, 4), timedOut(plainScan, 5*time.Minute))},
		{"open, read 14 minutes in, nothing after the timeout", open(14 * time.Minute),
			slices.Concat(scanPairs(plainScan, 0, 1, 2, 3, 4), timedOut(plainScan, 5*time.Minute))},
		{"the port appeared", scanRow("live", scanSince.Add(3*time.Minute), scanSince.Add(90*time.Second), scanSince.Add(time.Hour)),
			slices.Concat(scanPairs(plainScan, 0, 1), said(3*time.Minute, rendersLive))},
		{"the port appeared after the timeout line", scanRow("live", scanSince.Add(7*time.Minute+10*time.Second), scanSince.Add(7*time.Minute), scanSince.Add(time.Hour)),
			slices.Concat(scanPairs(plainScan, 0, 1, 2, 3, 4), timedOut(plainScan, 5*time.Minute),
				said(7*time.Minute+10*time.Second, rendersLive))},
		{"canceled while scanning", scanRow("canceled", scanSince.Add(2*time.Minute), time.Time{}, scanSince.Add(time.Hour)),
			slices.Concat(scanPairs(plainScan, 0, 1), said(2*time.Minute, "==> Deploy canceled"))},
		{"failed at the budget, scanning", atBudget(failedScanning(14*time.Minute + 30*time.Second)),
			slices.Concat(scanPairs(plainScan, 0, 1, 2, 3, 4), timedOut(plainScan, 5*time.Minute),
				said(14*time.Minute+30*time.Second, rendersTimeout...))},
		{"failed at the budget after the scan was over", atBudget(scanRow("update_failed", scanSince.Add(14*time.Minute+30*time.Second), scanSince.Add(30*time.Second), scanSince.Add(time.Hour))),
			slices.Concat(scanPairs(plainScan, 0), said(14*time.Minute+30*time.Second, rendersTimeout...))},
		{"failed scanning before the timeout was due", failedScanning(3*time.Minute + 30*time.Second),
			slices.Concat(scanPairs(plainScan, 0, 1, 2, 3), timedOut(plainScan, 3*time.Minute+30*time.Second),
				said(3*time.Minute+30*time.Second, "==> Deploy failed: "+failed))},
		{"failed on the timeout's moment", failedScanning(5 * time.Minute),
			slices.Concat(scanPairs(plainScan, 0, 1, 2, 3, 4), timedOut(plainScan, 5*time.Minute),
				said(5*time.Minute, "==> Deploy failed: "+failed))},
		{"failed after the scan was over", scanRow("update_failed", scanSince.Add(5*time.Minute), scanSince.Add(30*time.Second), scanSince.Add(time.Hour)),
			slices.Concat(scanPairs(plainScan, 0), said(5*time.Minute, "==> Deploy failed"))},
		{"no scan", func() DeployProgress {
			d := open(time.Hour)
			d.PortScanSince = time.Time{}
			return d
		}(), nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := scanStory(c.row); !slices.Equal(got, c.want) {
				t.Fatalf("narration:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(c.want, "\n"))
			}
		})
	}
}

// TestPortScanTailSendsEachLineOnceAsItFallsDue is the subscribe stream: a
// build tail reads the row every few seconds, and each scan line and its docs
// line reach it once, when their minute comes, never ahead of time; after the
// timeout pair nothing more arrives.
func TestPortScanTailSendsEachLineOnceAsItFallsDue(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	read := scanSince.Add(-10 * time.Second)
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{scanRow("update_in_progress", time.Time{}, time.Time{}, read)}, nil
	}
	follower := svc.newProgressFollower(LogQuery{App: "web", Types: []string{LogTypeBuild}, Since: scanSince.Add(-time.Second)}, "web", progressContext{})
	var sent []string
	emit := func(e LogEntry) error { sent = append(sent, e.Timestamp+" "+e.Message); return nil }
	for ; read.Before(scanSince.Add(8 * time.Minute)); read = read.Add(5 * time.Second) {
		if err := follower.emitReached(t.Context(), emit); err != nil {
			t.Fatal(err)
		}
		for _, line := range sent {
			stamp, _, _ := strings.Cut(line, " ")
			if sentAt, err := time.Parse(time.RFC3339Nano, stamp); err != nil || sentAt.After(read) {
				t.Fatalf("line sent before it was due (read %s): %s", read.Format(time.RFC3339), line)
			}
		}
	}
	want := slices.Concat(scanPairs(plainScan, 0, 1, 2, 3, 4), timedOut(plainScan, 5*time.Minute))
	if !slices.Equal(sent, want) {
		t.Fatalf("tail sent:\n%s\nwant:\n%s", strings.Join(sent, "\n"), strings.Join(want, "\n"))
	}
}

// TestPortScanLineReadsTheSameOnRESTAndGraphQL is the history read: REST
// /v1/logs (the CLI's `bex logs --type build`) and GraphQL logs (the
// dashboard's deploy log panel) carry the same scan lines with the same ids.
func TestPortScanLineReadsTheSameOnRESTAndGraphQL(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{scanRow("update_in_progress", time.Time{}, time.Time{}, scanSince.Add(70*time.Second))}, nil
	}
	window := "startTime=" + scanSince.Add(-time.Minute).Format(time.RFC3339) + "&endTime=" + scanSince.Add(2*time.Minute).Format(time.RFC3339)

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/logs?resource=web&type=build&"+window, nil))
	var env renderLogList
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode REST logs: %v (body %s)", err, rec.Body.String())
	}
	var rest []string
	for _, l := range env.Logs {
		if l.Message == portScanLine {
			rest = append(rest, l.ID)
		}
	}

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	gqlWindow := `startTime:"` + scanSince.Add(-time.Minute).Format(time.RFC3339) + `", endTime:"` + scanSince.Add(2*time.Minute).Format(time.RFC3339) + `"`
	data := runQuery(t, schema, `{ logs(resource:"web", type:"build", `+gqlWindow+`) { logs { id message } } }`)
	var gql []string
	for _, row := range data["logs"].(map[string]any)["logs"].([]any) {
		if m := row.(map[string]any); m["message"] == portScanLine {
			gql = append(gql, m["id"].(string))
		}
	}
	if len(rest) != 2 {
		t.Fatalf("REST carried %d scan lines, want the two due 70 s in: %s", len(rest), rec.Body.String())
	}
	slices.Sort(rest)
	slices.Sort(gql)
	if !slices.Equal(rest, gql) {
		t.Fatalf("GraphQL scan line ids %v, REST %v", gql, rest)
	}
}

// TestPortScanLinesNameWhatTheScanFound is w8/388: Render words its scan for
// a listener only on localhost and for open ports that answer no HTTP. Each
// line carries the wording of what the scan had found by its moment, so the
// lines before the finding keep the plain text, the timeout line included;
// the no-HTTP lines carry no docs line, as Render's do.
func TestPortScanLinesNameWhatTheScanFound(t *testing.T) {
	const end = 14*time.Minute + 30*time.Second
	found := func(d DeployProgress, what appv1alpha1.PortScanFinding, foundAt time.Duration) DeployProgress {
		d.PortScanFound, d.PortScanFoundAt = what, scanSince.Add(foundAt)
		return d
	}
	failed := scanRow("update_failed", scanSince.Add(end), time.Time{}, scanSince.Add(time.Hour))
	for _, c := range []struct {
		name string
		row  DeployProgress
		want []string
	}{
		{"localhost from the start, failed", found(failed, appv1alpha1.PortScanFoundLocalhost, 0),
			slices.Concat(scanPairs(localScan, 0, 1, 2, 3, 4), timedOut(localScan, 5*time.Minute), said(end, "==> Deploy failed"))},
		{"no HTTP found 90 s in, failed", found(failed, appv1alpha1.PortScanFoundNoHTTP, 90*time.Second),
			slices.Concat(scanPairs(plainScan, 0, 1), scanPairs(noHTTPScan, 2, 3, 4), timedOut(noHTTPScan, 5*time.Minute),
				said(end, "==> Deploy failed"))},
		{"localhost found after the timeout line, failed", found(failed, appv1alpha1.PortScanFoundLocalhost, 6*time.Minute),
			slices.Concat(scanPairs(plainScan, 0, 1, 2, 3, 4), timedOut(plainScan, 5*time.Minute), said(end, "==> Deploy failed"))},
		{"no HTTP from the start, then a port answered", found(scanRow("live", scanSince.Add(3*time.Minute), scanSince.Add(170*time.Second), scanSince.Add(time.Hour)),
			appv1alpha1.PortScanFoundNoHTTP, 0),
			slices.Concat(scanPairs(noHTTPScan, 0, 1, 2), said(3*time.Minute, rendersLive))},
		{"open, read before the finding's moment", found(scanRow("update_in_progress", time.Time{}, time.Time{}, scanSince.Add(70*time.Second)),
			appv1alpha1.PortScanFoundLocalhost, 2*time.Minute), scanPairs(plainScan, 0, 1)},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := scanStory(c.row); !slices.Equal(got, c.want) {
				t.Fatalf("narration:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(c.want, "\n"))
			}
		})
	}
}

// A tail that read the plain lines before the scan found a localhost listener
// keeps them, and the lines after carry Render's localhost wording: no line
// is sent twice or reworded.
func TestPortScanTailWordsTheLinesAfterTheFinding(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	read := scanSince.Add(-10 * time.Second)
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		d := scanRow("update_in_progress", time.Time{}, time.Time{}, read)
		if foundAt := scanSince.Add(70 * time.Second); !read.Before(foundAt) {
			d.PortScanFound, d.PortScanFoundAt = appv1alpha1.PortScanFoundLocalhost, foundAt
		}
		return []DeployProgress{d}, nil
	}
	follower := svc.newProgressFollower(LogQuery{App: "web", Types: []string{LogTypeBuild}, Since: scanSince.Add(-time.Second)}, "web", progressContext{})
	var sent []string
	emit := func(e LogEntry) error { sent = append(sent, e.Timestamp+" "+e.Message); return nil }
	for ; read.Before(scanSince.Add(190 * time.Second)); read = read.Add(5 * time.Second) {
		if err := follower.emitReached(t.Context(), emit); err != nil {
			t.Fatal(err)
		}
	}
	want := slices.Concat(scanPairs(plainScan, 0, 1), scanPairs(localScan, 2, 3))
	if !slices.Equal(sent, want) {
		t.Fatalf("tail sent:\n%s\nwant:\n%s", strings.Join(sent, "\n"), strings.Join(want, "\n"))
	}
}

// TestPortScanLinesNameThePortPORTSets is a web service whose PORT is set and
// closed (w8/416). Render scans that port alone and says so: its scan line a
// minute apart, five times, with no docs line, then its timeout line naming
// the port and its fix (community.render.com/t/21740;
// htdio-stg/deploy-templates#1). The text is restated so the test pins it.
func TestPortScanLinesNameThePortPORTSets(t *testing.T) {
	envScan := portScanWording{
		scan: "==> Continuing to scan for open port 8080 (from PORT environment variable)...",
		timeout: "==> Port scan timeout reached, failed to detect open port 8080 from PORT environment variable. " +
			"Bind your service to port 8080 or update the PORT environment variable to the correct port.",
		// No capture shows Render's codes on the PORT wording (w32/m576).
		timeoutForm: renderlog.Plain,
	}
	const end = 14*time.Minute + 30*time.Second
	onPORT := func(d DeployProgress) DeployProgress {
		d.PortScanFound, d.PortScanFoundAt = appv1alpha1.PortScanFoundEnvPort(8080), scanSince
		return d
	}
	atBudget := onPORT(scanRow("update_failed", scanSince.Add(end), time.Time{}, scanSince.Add(time.Hour)))
	atBudget.TimedOut = true
	for _, c := range []struct {
		name string
		row  DeployProgress
		want []string
	}{
		{"open, read 2.5 minutes in", onPORT(scanRow("update_in_progress", time.Time{}, time.Time{}, scanSince.Add(150*time.Second))),
			scanPairs(envScan, 0, 1, 2)},
		{"failed at the budget", atBudget,
			slices.Concat(scanPairs(envScan, 0, 1, 2, 3, 4), timedOut(envScan, 5*time.Minute), said(end, rendersTimeout...))},
		{"PORT was bound 3 minutes in", onPORT(scanRow("live", scanSince.Add(3*time.Minute+5*time.Second), scanSince.Add(3*time.Minute), scanSince.Add(time.Hour))),
			slices.Concat(scanPairs(envScan, 0, 1, 2), said(3*time.Minute+5*time.Second, rendersLive))},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := scanStory(c.row); !slices.Equal(got, c.want) {
				t.Fatalf("narration:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(c.want, "\n"))
			}
		})
	}
}
