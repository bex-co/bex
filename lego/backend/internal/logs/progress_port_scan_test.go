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

// at is scanSince plus d, as a line's timestamp.
func at(d time.Duration) string { return scanSince.Add(d).Format(time.RFC3339Nano) }

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

// TestPortScanLinesKeepRendersCadence is w8/385's cadence: the scan
// line when the stall first says no port is open, then one a minute, five at
// most, the docs line after the first only; it stops when a stall says the
// scan is over or the deploy ends, and a deploy that failed scanning ends with
// Render's timeout line before its failure line.
func TestPortScanLinesKeepRendersCadence(t *testing.T) {
	const failed = "the container is running but its startup health check never succeeded within the rollout window, so the deploy failed: a TCP connect to any open port on the instance."
	scan, docs, timeout := portScanLine, portScanDocsLine, portScanTimeoutLine
	for _, c := range []struct {
		name string
		row  DeployProgress
		want []string
	}{
		{"open, read 2.5 minutes in", scanRow("update_in_progress", time.Time{}, time.Time{}, scanSince.Add(150*time.Second)),
			[]string{at(0) + " " + scan, at(0) + " " + docs, at(time.Minute) + " " + scan, at(2*time.Minute) + " " + scan}},
		{"open, read on a minute", scanRow("update_in_progress", time.Time{}, time.Time{}, scanSince.Add(time.Minute)),
			[]string{at(0) + " " + scan, at(0) + " " + docs, at(time.Minute) + " " + scan}},
		{"open, read before the first line", scanRow("update_in_progress", time.Time{}, time.Time{}, scanSince.Add(-time.Second)), nil},
		{"open past five minutes stops at five", scanRow("update_in_progress", time.Time{}, time.Time{}, scanSince.Add(14*time.Minute)),
			[]string{at(0) + " " + scan, at(0) + " " + docs, at(time.Minute) + " " + scan, at(2*time.Minute) + " " + scan,
				at(3*time.Minute) + " " + scan, at(4*time.Minute) + " " + scan}},
		{"the port appeared", scanRow("live", scanSince.Add(3*time.Minute), scanSince.Add(90*time.Second), scanSince.Add(time.Hour)),
			[]string{at(0) + " " + scan, at(0) + " " + docs, at(time.Minute) + " " + scan, at(3*time.Minute) + " ==> Your service is live 🎉"}},
		{"canceled while scanning", scanRow("canceled", scanSince.Add(2*time.Minute), time.Time{}, scanSince.Add(time.Hour)),
			[]string{at(0) + " " + scan, at(0) + " " + docs, at(time.Minute) + " " + scan, at(2*time.Minute) + " ==> Deploy canceled"}},
		{"failed while scanning", func() DeployProgress {
			d := scanRow("update_failed", scanSince.Add(14*time.Minute+30*time.Second), time.Time{}, scanSince.Add(time.Hour))
			d.FailureReason = failed
			return d
		}(), []string{at(0) + " " + scan, at(0) + " " + docs, at(time.Minute) + " " + scan, at(2*time.Minute) + " " + scan,
			at(3*time.Minute) + " " + scan, at(4*time.Minute) + " " + scan,
			at(14*time.Minute+30*time.Second) + " " + timeout, at(14*time.Minute+30*time.Second) + " " + docs,
			at(14*time.Minute+30*time.Second) + " ==> Deploy failed: " + failed}},
		{"failed after the scan was over", scanRow("update_failed", scanSince.Add(5*time.Minute), scanSince.Add(30*time.Second), scanSince.Add(time.Hour)),
			[]string{at(0) + " " + scan, at(0) + " " + docs, at(5*time.Minute) + " ==> Deploy failed"}},
		{"no scan", func() DeployProgress {
			d := scanRow("update_in_progress", time.Time{}, time.Time{}, scanSince.Add(time.Hour))
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
// build tail reads the row every few seconds, and each scan line reaches it
// once, when its minute comes, never ahead of time.
func TestPortScanTailSendsEachLineOnceAsItFallsDue(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	read := scanSince.Add(-10 * time.Second)
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		return []DeployProgress{scanRow("update_in_progress", time.Time{}, time.Time{}, read)}, nil
	}
	follower := svc.newProgressFollower(LogQuery{App: "web", Types: []string{LogTypeBuild}, Since: scanSince.Add(-time.Second)}, "web", progressContext{})
	var sent []string
	emit := func(e LogEntry) error { sent = append(sent, e.Timestamp+" "+e.Message); return nil }
	for ; read.Before(scanSince.Add(130 * time.Second)); read = read.Add(5 * time.Second) {
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
	want := []string{at(0) + " " + portScanLine, at(0) + " " + portScanDocsLine, at(time.Minute) + " " + portScanLine, at(2*time.Minute) + " " + portScanLine}
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
