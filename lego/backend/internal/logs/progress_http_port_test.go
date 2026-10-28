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
	"strings"
	"testing"
	"time"
)

func TestHTTPPortNarrationUsesTheClosedDeployRecord(t *testing.T) {
	finished := time.Date(2026, 10, 28, 16, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, status string
		port         int32
		finished     time.Time
		want         bool
	}{
		{"live detected", "live", 15672, finished, true},
		{"historical detected", "deactivated", 80, finished, true},
		{"older unrecorded deploy", "deactivated", 0, finished, false},
		{"failed candidate", "update_failed", 15672, finished, false},
		{"pending candidate", "update_in_progress", 15672, time.Time{}, false},
		{"invalid port", "live", 65536, finished, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := DeployProgress{ID: "dep-record", Status: tc.status, CreatedAt: finished.Add(-time.Minute), FinishedAt: tc.finished, DetectedHTTPPort: tc.port}
			entries := progressLines(row, progressContext{})
			found := -1
			for i, entry := range entries {
				if !strings.Contains(entry.Message, "Detected HTTP") {
					continue
				}
				found = i
				if entry.Timestamp != finished.Format(time.RFC3339Nano) || entry.Labels[LabelInstance] != row.ID || entry.Labels[LabelType] != LogTypeBuild || entry.Labels["container"] != progressContainer {
					t.Fatalf("wrong narration identity: %+v", entry)
				}
			}
			if (found >= 0) != tc.want {
				t.Fatalf("narration=%+v, wantedHTTP=%v", entries, tc.want)
			}
			if tc.want && (found+1 >= len(entries) || entries[found+1].Message != "==> Your service is live 🎉") {
				t.Fatalf("HTTP selection must precede live: %+v", entries)
			}
		})
	}
}

func TestHTTPPortProgressFollowerEmitsOnce(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	at := time.Date(2026, 10, 28, 16, 0, 0, 0, time.UTC)
	row := DeployProgress{ID: "dep-http", Status: "live", CreatedAt: at.Add(-time.Minute), FinishedAt: at, DetectedHTTPPort: 15672}
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return []DeployProgress{row}, nil }
	follower := svc.newProgressFollower(LogQuery{App: "web", Types: []string{LogTypeBuild}}, "web", progressContext{})
	var entries []LogEntry
	for range 3 {
		if err := follower.emitReached(t.Context(), func(e LogEntry) error { entries = append(entries, e); return nil }); err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for _, entry := range entries {
		if entry.Message == "==> Detected HTTP on port 15672; serving it" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("HTTP narration emitted%d times: %+v", count, entries)
	}
}
