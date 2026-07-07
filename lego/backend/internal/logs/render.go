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
)

// render.go maps LogEntry onto Render's public-API log object: a required id,
// and labels as a [{name,value}] array. The MCP list_logs tool returns LogEntry
// verbatim (matching Render's MCP server); the REST logs API uses this shape.

// renderLogTypeApp is Render's `type` label value for application logs. bex only
// sources application logs, so every REST log line is tagged with it.
const renderLogTypeApp = "app"

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

// renderLogList is the logs envelope; Render marks all four fields required. The
// cursors bound the returned batch: nextStartTime = newest line, nextEndTime =
// oldest (the backward-page cursor).
type renderLogList struct {
	HasMore       bool        `json:"hasMore"`
	NextStartTime string      `json:"nextStartTime"`
	NextEndTime   string      `json:"nextEndTime"`
	Logs          []renderLog `json:"logs"`
}

// logID synthesizes a stable, unique id (Render ids are opaque; bex derives one
// from instance + timestamp + a message hash so the same line is always the same
// id).
func logID(e LogEntry) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(e.Message))
	return fmt.Sprintf("%s-%s-%08x", e.Labels["instance"], e.Timestamp, h.Sum32())
}

func toRenderLog(e LogEntry) renderLog {
	// Fixed label order (Render's names): type, then resource (LogEntry's
	// "service"), instance, container — deterministic output.
	labels := []renderLabel{{Name: "type", Value: renderLogTypeApp}}
	if v := e.Labels["service"]; v != "" {
		labels = append(labels, renderLabel{Name: "resource", Value: v})
	}
	if v := e.Labels["instance"]; v != "" {
		labels = append(labels, renderLabel{Name: "instance", Value: v})
	}
	if v := e.Labels["container"]; v != "" {
		labels = append(labels, renderLabel{Name: "container", Value: v})
	}
	return renderLog{ID: logID(e), Message: e.Message, Timestamp: e.Timestamp, Labels: labels}
}

func toRenderLogList(entries []LogEntry, limit int64) renderLogList {
	out := renderLogList{Logs: make([]renderLog, 0, len(entries))}
	for _, e := range entries {
		out.Logs = append(out.Logs, toRenderLog(e))
	}
	// entries are timestamp-sorted (oldest-first); cursors bound the batch.
	if n := len(entries); n > 0 {
		out.NextStartTime = entries[n-1].Timestamp // newest
		out.NextEndTime = entries[0].Timestamp     // oldest
	}
	out.HasMore = limit > 0 && int64(len(entries)) >= limit
	return out
}
