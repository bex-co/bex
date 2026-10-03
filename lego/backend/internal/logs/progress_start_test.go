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
	"slices"
	"testing"
	"time"
)

// TestImageRestartWithUnknownStartNarratesNoStartBanner is the w4/m156 log
// half: the image Restart whose container logged "Starting up" at 03:14:55
// used to be narrated "==> Deploying image" at 03:15:08 — the live observation
// stamped as its start, after the app had already started. A deploy whose
// start is unknown now gets no start banner at all, while its queued and live
// lines stay (the application's own log is a separate type=app stream this
// narration never touches). An observed start keeps its banner at exactly
// that instant.
func TestImageRestartWithUnknownStartNarratesNoStartBanner(t *testing.T) {
	const image = "traefik/whoami:v1.10.1"
	created := time.Date(2026, 10, 3, 3, 14, 52, 339929000, time.UTC)
	finished := time.Date(2026, 10, 3, 3, 15, 8, 748639000, time.UTC)
	restart := DeployProgress{ID: "dep-restart", Status: "live", Image: image, CreatedAt: created, FinishedAt: finished}

	read := func(row DeployProgress) []LogEntry {
		t.Helper()
		svc := newService(nil, sampleApp("web"))
		svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
		svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
			return []DeployProgress{row}, nil
		}
		entries, err := svc.QueryLogs(context.Background(), LogQuery{
			App: "web", Types: []string{LogTypeBuild},
			Since: created.Add(-time.Minute), End: finished.Add(time.Minute),
		})
		if err != nil {
			t.Fatalf("QueryLogs: %v", err)
		}
		return entries
	}
	messages := func(entries []LogEntry) []string {
		out := make([]string, 0, len(entries))
		for _, e := range entries {
			out = append(out, e.Message)
		}
		return out
	}

	got := read(restart)
	want := []string{"==> Deploy queued", "==> Your service is live 🎉"}
	if !slices.Equal(messages(got), want) {
		t.Fatalf("unknown-start narration = %v, want %v", messages(got), want)
	}

	observed := time.Date(2026, 10, 3, 3, 14, 53, 0, time.UTC)
	restart.StartedAt = observed
	got = read(restart)
	i := slices.Index(messages(got), "==> Deploying image "+image)
	if i < 0 || got[i].Timestamp != observed.Format(time.RFC3339Nano) {
		t.Fatalf("observed-start narration = %v, want the image banner at %s", got, observed)
	}
}
