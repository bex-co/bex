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

package api

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestHTTPPortProgressHistoryKeepsEachDeploysListener(t *testing.T) {
	at := time.Date(2026, 10, 28, 16, 0, 0, 0, time.UTC)
	oldEnd, newEnd := at.Add(-time.Minute), at
	old := store.Deploy{ID: "dep-rabbitmq", Status: store.DeployDeactivated, DetectedHTTPPort: 15672, CreatedAt: at.Add(-2 * time.Minute), FinishedAt: &oldEnd}
	latest := store.Deploy{ID: "dep-nginx", Status: store.DeployLive, DetectedHTTPPort: 80, CreatedAt: at.Add(-time.Minute), FinishedAt: &newEnd}
	app := sampleApp("web")
	// The App has already moved to a third image's candidate. Historical lines
	// must still come from their own deploy records rather than this port.
	app.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "third@sha256:c", Revision: "rev-3", PrimaryPort: 9999, HTTPSelection: "http"}
	server := NewServer(&core.Base{Client: fakeClient(app), Namespace: "default"}, Deps{
		DeployStore: &conformDeployStore{byApp: map[string][]store.Deploy{"web": {latest, old}}},
		LogHistory:  func(context.Context, string, logs.LogQuery) ([]logs.LogEntry, error) { return nil, nil },
	})
	entries, err := server.Logs.QueryLogs(context.Background(), logs.LogQuery{App: "web", Types: []string{logs.LogTypeBuild}, Since: at.Add(-3 * time.Minute), End: at.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]logs.LogEntry{}
	for _, entry := range entries {
		if entry.Message == "==> Detected HTTP on port 15672; serving it" || entry.Message == "==> Detected HTTP on port 80; serving it" {
			found[entry.Message] = entry
		}
		if entry.Message == "==> Detected HTTP on port 9999; serving it" {
			t.Fatal("history borrowed the current candidate's listener")
		}
	}
	oldLine, newLine := found["==> Detected HTTP on port 15672; serving it"], found["==> Detected HTTP on port 80; serving it"]
	if oldLine.Timestamp != oldEnd.Format(time.RFC3339Nano) || newLine.Timestamp != newEnd.Format(time.RFC3339Nano) || oldLine.Labels[logs.LabelInstance] == newLine.Labels[logs.LabelInstance] {
		t.Fatalf("historical narration=%v entries=%+v", found, entries)
	}
}

// w32/m567: the composition root hands the narration each deploy row's own
// record of where it went live, so a deploy's log names the URL it went live
// at, and a row with no record (an older deploy) ends at its live line.
func TestPrimaryURLBlockReadsEachDeploysOwnRecord(t *testing.T) {
	at := time.Date(2026, 11, 22, 16, 0, 0, 0, time.UTC)
	oldEnd, newEnd := at.Add(-time.Minute), at
	old := store.Deploy{ID: "dep-old", Status: store.DeployDeactivated, CreatedAt: at.Add(-2 * time.Minute), FinishedAt: &oldEnd}
	latest := store.Deploy{ID: "dep-new", Status: store.DeployLive, CreatedAt: at.Add(-time.Minute), FinishedAt: &newEnd,
		LiveURLs: []string{"https://shop.example", "https://web.onbex.co"}}
	server := NewServer(&core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}, Deps{
		DeployStore: &conformDeployStore{byApp: map[string][]store.Deploy{"web": {latest, old}}},
		LogHistory:  func(context.Context, string, logs.LogQuery) ([]logs.LogEntry, error) { return nil, nil },
	})
	entries, err := server.Logs.QueryLogs(context.Background(), logs.LogQuery{App: "web", Types: []string{logs.LogTypeBuild}, Since: at.Add(-3 * time.Minute), End: at.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	urls := map[string][]string{}
	for _, e := range entries {
		if strings.HasPrefix(e.Message, "==> Available at your primary URL ") {
			urls[e.Labels[logs.LabelBuild]] = append(urls[e.Labels[logs.LabelBuild]], e.Message)
		}
	}
	if want := []string{"==> Available at your primary URL https://shop.example + 1 more domain"}; len(urls) != 1 || !slices.Equal(urls["dep-new"], want) {
		t.Fatalf("primary URL lines by deploy = %q, want only dep-new's %q", urls, want)
	}
}

// w32/m572: the composition root hands the narration each deploy row's own
// record of its pre-deploy step, so a failed step's log names its command and
// closes with its exit status, and a row with no record (an older deploy)
// keeps bex's line.
func TestPreDeployLinesReadEachDeploysOwnRecord(t *testing.T) {
	at := time.Date(2026, 11, 22, 11, 29, 32, 470000000, time.UTC)
	launched, container, settled := at.Add(-17*time.Second), at.Add(-5*time.Second).Truncate(time.Second), at.Add(-time.Second)
	code := int32(127)
	oldEnd := at.Add(-time.Hour)
	old := store.Deploy{ID: "dep-old", Status: store.DeployPreDeployFailed, PreDeployStatus: store.PreDeployFailed, CreatedAt: oldEnd.Add(-time.Minute),
		FinishedAt: &oldEnd, FailureReason: "the pre-deploy command exited with code 1; check the pre-deploy logs"}
	latest := store.Deploy{ID: "dep-new", Status: store.DeployPreDeployFailed, PreDeployStatus: store.PreDeployFailed, CreatedAt: at.Add(-time.Minute),
		FinishedAt: &at, FailureReason: "the pre-deploy command exited with code 127; check the pre-deploy logs",
		PreDeployRun: store.DeployPreDeployRun{StartedAt: &launched, ContainerStartedAt: &container, FinishedAt: &settled,
			Command: "alembic upgrade head", Native: true, ExitCode: &code}}
	server := NewServer(&core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}, Deps{
		DeployStore: &conformDeployStore{byApp: map[string][]store.Deploy{"web": {latest, old}}},
		LogHistory:  func(context.Context, string, logs.LogQuery) ([]logs.LogEntry, error) { return nil, nil },
	})
	entries, err := server.Logs.QueryLogs(context.Background(), logs.LogQuery{App: "web", Types: []string{logs.LogTypeBuild}, Since: oldEnd.Add(-time.Hour), End: at.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	lines := map[string][]string{}
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Message), "pre-deploy") || strings.HasPrefix(e.Message, "==> Running") || strings.HasPrefix(e.Message, "==> Exited") {
			lines[e.Labels[logs.LabelBuild]] = append(lines[e.Labels[logs.LabelBuild]], e.Message)
		}
	}
	want := []string{"==> Starting pre-deploy: alembic upgrade head", "==> Running 'alembic upgrade head'", "==> Pre-deploy has failed", "==> Exited with status 127"}
	if !slices.Equal(lines["dep-new"], want) {
		t.Fatalf("dep-new's step lines = %q, want %q", lines["dep-new"], want)
	}
	if want := []string{"==> Pre-deploy failed: the pre-deploy command exited with code 1; check the pre-deploy logs"}; !slices.Equal(lines["dep-old"], want) {
		t.Fatalf("dep-old's step lines = %q, want bex's %q", lines["dep-old"], want)
	}
}
