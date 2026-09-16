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
	"fmt"
	"hash/fnv"
	"time"
)

// render.go maps LogEntry onto Render's public-API log object: a required id,
// and labels as a [{name,value}] array. The MCP list_logs tool returns LogEntry
// verbatim (matching Render's MCP server); the REST logs API uses this shape.

// renderLabels is the order labels appear on a REST log line (Render's `name`) and
// the LogEntry key each reads from — they differ only for `resource`, which Core
// carries as `service`. A line carries only the labels its stream actually had: an
// app line has no method/statusCode, a request line no instance/container.
var renderLabels = []struct{ name, key string }{
	{LabelType, LabelType},
	{"resource", "service"},
	{LabelInstance, LabelInstance},
	{"container", "container"},
	{LabelLevel, LabelLevel},
	{LabelMethod, LabelMethod},
	{LabelStatusCode, LabelStatusCode},
}

// renderLabel is Render's logLabel ({name, value}); the REST logs API returns
// labels as an ordered array rather than LogEntry's map.
type renderLabel struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// renderLog is Render's public-API log object (id/message/timestamp/labels all
// required by the spec).
type renderLog struct {
	ID        string        `json:"id"`
	Message   string        `json:"message"`
	Timestamp string        `json:"timestamp"`
	Labels    []renderLabel `json:"labels"`
}

// renderLogList is the logs envelope; Render marks all four fields required.
// nextStartTime/nextEndTime name the *next page's* window for the query's
// direction — feed them straight back as startTime/endTime (Render's contract,
// the official CLI's scroll-to-load path). They are not the current page's
// newest/oldest bounds.
type renderLogList struct {
	HasMore       bool        `json:"hasMore"`
	NextStartTime string      `json:"nextStartTime"`
	NextEndTime   string      `json:"nextEndTime"`
	Logs          []renderLog `json:"logs"`
}

// logID synthesizes a stable, unique id (Render ids are opaque; bex derives one
// from instance + timestamp + a message hash so the same line is always the same
// id). Request logs carry no instance label (they come from the edge, not a
// replica), so fall back to the service name to avoid a leading "-".
func logID(e LogEntry) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(e.Message))
	instance := e.Labels[LabelInstance]
	if instance == "" {
		instance = e.Labels["service"]
	}
	return fmt.Sprintf("%s-%s-%08x", instance, e.Timestamp, h.Sum32())
}

func toRenderLog(e LogEntry) renderLog {
	labels := make([]renderLabel, 0, len(e.Labels))
	for _, l := range renderLabels {
		if v := e.Labels[l.key]; v != "" {
			labels = append(labels, renderLabel{Name: l.name, Value: v})
		}
	}
	return renderLog{ID: logID(e), Message: e.Message, Timestamp: e.Timestamp, Labels: labels}
}

// toRenderLogList builds the logs envelope. since/end are the query's own
// resolved time bounds and direction picks which end of the window limit kept.
// Render marks nextStartTime/nextEndTime as REQUIRED timestamps (never omitted
// or empty, verified against the render-oss/cli generated client's
// Logs200Response: both are plain time.Time, not pointers), so an empty-result
// query still needs valid cursors; the query's own window is the only bound
// available when there are no entries to derive one from.
func toRenderLogList(entries []LogEntry, limit int64, since, end time.Time, direction string) renderLogList {
	out := renderLogList{Logs: make([]renderLog, 0, len(entries))}
	for _, e := range entries {
		out.Logs = append(out.Logs, toRenderLog(e))
	}
	out.HasMore, out.NextStartTime, out.NextEndTime = pageCursors(entries, limit, since, end, direction)
	return out
}

// pageCursors computes the Render paging envelope fields shared by REST,
// GraphQL, and MCP. Callers feed nextStartTime/nextEndTime back as
// startTime/endTime to fetch the next page.
//
// Boundary rule (start and end are inclusive on the pod-log path; Loki's
// query_range end is exclusive — both stay correct with the adjustments
// below):
//
//   - backward (default): nextStartTime = query start, nextEndTime = one
//     nanosecond before the page's oldest timestamp — so an inclusive end
//     does not re-include the boundary group. capToLimit keeps a shared-
//     timestamp group whole first, so every sibling at that instant is
//     already on this page.
//   - forward: nextStartTime = one nanosecond past the page's newest
//     timestamp, nextEndTime = query end — so an inclusive start does not
//     repeat the newest line (same whole-group guarantee from capToLimit).
func pageCursors(entries []LogEntry, limit int64, since, end time.Time, direction string) (hasMore bool, nextStart, nextEnd string) {
	since, end = resolveCursorWindow(since, end)
	hasMore = limit > 0 && int64(len(entries)) >= limit
	if len(entries) == 0 {
		// Second precision matches Render's empty-page cursors and keeps two
		// identical empty reads byte-identical within the same second.
		return hasMore, since.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339)
	}
	oldest, newest := entries[0].Timestamp, entries[len(entries)-1].Timestamp
	if direction == DirectionForward {
		return hasMore, advancePast(newest), end.UTC().Format(time.RFC3339Nano)
	}
	return hasMore, since.UTC().Format(time.RFC3339Nano), retreatBefore(oldest)
}

// resolveCursorWindow fills Render's documented defaults so REQUIRED cursors
// are never empty when the caller omitted a bound.
func resolveCursorWindow(since, end time.Time) (time.Time, time.Time) {
	if end.IsZero() {
		end = time.Now().UTC()
	}
	if since.IsZero() {
		since = end.Add(-time.Hour)
	}
	return since, end
}

// advancePast returns an RFC3339Nano instant one nanosecond after stamp, so a
// forward page's inclusive startTime does not re-include the previous page's
// newest line. An unparseable stamp is returned unchanged (the next query's
// own validation will surface it).
func advancePast(stamp string) string {
	t, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return stamp
	}
	return t.Add(time.Nanosecond).UTC().Format(time.RFC3339Nano)
}

// retreatBefore returns an RFC3339Nano instant one nanosecond before stamp, so
// a backward page's inclusive endTime does not re-include the previous page's
// oldest line (and its shared-timestamp siblings, already delivered).
func retreatBefore(stamp string) string {
	t, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return stamp
	}
	return t.Add(-time.Nanosecond).UTC().Format(time.RFC3339Nano)
}
