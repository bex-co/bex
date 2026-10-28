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
