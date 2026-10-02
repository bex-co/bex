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
	"strconv"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestTextSourcesPreDeployUnionRespectsTimeAndScope(t *testing.T) {
	t.Parallel()
	const migration = "predeploy-web-gen-3"
	svc := newService(map[string][]string{
		migration: {
			"2026-07-05T00:00:00Z qa_text_A too early",
			"2026-07-05T00:00:01Z qa_text_A",
			"2026-07-05T00:00:01Z unrelated control",
			"2026-07-05T00:00:01Z qa_text_B",
			"2026-07-05T00:00:02Z qa_text_B too late",
		},
		"web-1":                 {"2026-07-05T00:00:01Z qa_text_A app stdout"},
		"predeploy-other-gen-1": {"2026-07-05T00:00:01Z qa_text_B other service"},
	}, sampleApp("web"), podFor("web", "web-1"), preDeployPod("web", migration), preDeployPod("other", "predeploy-other-gen-1"))
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) {
		t.Error("pre-deploy reads must use their Job pod, even with history wired")
		return nil, nil
	}
	instant := time.Date(2026, 7, 5, 0, 0, 1, 0, time.UTC)
	for _, terms := range [][]string{{"qa_text_A", "qa_text_B"}, {"qa_text_B", "qa_text_A"}} {
		entries, err := svc.QueryLogs(context.Background(), LogQuery{
			App: "web", Types: []string{LogTypePreDeploy}, Search: terms,
			Since: instant.Add(-time.Millisecond), End: instant.Add(time.Millisecond), Limit: 10,
		})
		if err != nil {
			t.Fatal(err)
		}
		assertTextSourceMessages(t, entries, []string{"qa_text_A", "qa_text_B"})
		for _, entry := range entries {
			if entry.Labels[LabelType] != LogTypePreDeploy || entry.Labels["container"] != core.PreDeployContainer || entry.Labels[LabelInstance] != ids.ServiceInstanceID("web", migration) {
				t.Errorf("pre-deploy source attribution = %+v", entry.Labels)
			}
		}
	}
}

func TestTextSourcesBuildTailAndProgressFollowerKeepUnion(t *testing.T) {
	t.Parallel()
	instant := time.Date(2026, 7, 17, 20, 16, 16, 0, time.UTC)
	app := sampleApp("web")
	app.Spec.Repo = "https://github.com/qa_text_A/repo"
	pod := buildPodFor("web", "bld-web-gen-2", "builds", "buildkit", instant)
	old := buildPodFor("web", "bld-web-gen-1", "builds", "buildkit", instant.Add(-time.Minute))
	svc := newService(map[string][]string{
		pod.Name: {
			"2026-07-17T20:16:15Z qa_text_A too early",
			"2026-07-17T20:16:16Z qa_text_A container",
			"2026-07-17T20:16:16Z unrelated control",
			"2026-07-17T20:16:16Z qa_text_B container",
			"2026-07-17T20:16:17Z qa_text_B too late",
		},
		old.Name: {"2026-07-17T20:16:16Z qa_text_B old build"},
	}, app, old, pod)
	svc.BuildNamespace = "builds"
	deploy := inFlightDeploy()
	deploy.Status, deploy.Built = "build_failed", true
	deploy.StartedAt, deploy.FinishedAt = instant, instant
	deploy.FailureReason = "qa_text_B unavailable"
	svc.DeployProgress = func(_ context.Context, resource string, _ time.Time) ([]DeployProgress, error) {
		if resource != "web" {
			t.Errorf("progress lookup escaped its resource: %q", resource)
		}
		return []DeployProgress{deploy}, nil
	}
	for _, terms := range [][]string{{"qa_text_A", "qa_text_B"}, {"qa_text_B", "qa_text_A"}} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		var entries []LogEntry
		err := svc.FollowLogs(ctx, LogQuery{
			App: "web", Types: []string{LogTypeBuild}, Search: terms,
			Since: instant.Add(-time.Millisecond), End: instant.Add(time.Millisecond),
		}, func(entry LogEntry) error {
			entries = append(entries, entry)
			return nil
		})
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		assertTextSourceMessages(t, entries, []string{
			"==> Building from https://github.com/qa_text_A/repo@abc1234",
			"==> Build failed: qa_text_B unavailable",
			"qa_text_A container", "qa_text_B container",
		})
		for i, entry := range entries {
			container, instance := "buildkit", pod.Name
			if i < 2 {
				container, instance = progressContainer, deploy.ID
			}
			if entry.Labels[LabelType] != LogTypeBuild || entry.Labels["container"] != container || entry.Labels[LabelInstance] != ids.ServiceInstanceID("web", instance) {
				t.Errorf("build source attribution = %+v", entry.Labels)
			}
		}
	}
}

func TestTextSourcesDatastoreHistoryKeepsScopedUnion(t *testing.T) {
	t.Parallel()
	instant := time.Date(2026, 7, 5, 0, 0, 1, 0, time.UTC)
	for _, tc := range []struct {
		resource, selector, kind string
	}{
		{postgresID, "database", "postgres"},
		{keyValueID, "keyvalue", "keyvalue"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			f := newFakeLoki(lokiResp(map[string]any{
				"stream": fmt.Sprintf(`{%q:%q,"pod":"datastore-1","type":%q}`, tc.selector, tc.resource, tc.kind),
				"values": fmt.Sprintf(`[[%q,"qa_text_A"],[%q,"qa_text_B"]]`, strconv.FormatInt(instant.UnixNano(), 10), strconv.FormatInt(instant.UnixNano(), 10)),
			}))
			defer f.srv.Close()
			db := sampleDatabase(postgresID)
			db.Namespace = "tea-logs"
			kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: keyValueID, Namespace: "tea-logs"}}
			svc := newService(nil, db, kv)
			svc.History = NewLokiSource(f.srv.URL, f.srv.Client())
			for _, terms := range [][]string{{"", "QA_TEXT_B", "qa_text_A", "qa_text_b"}, {"qa_text_A", "qa_text_B"}} {
				entries, err := svc.QueryLogs(context.Background(), LogQuery{
					App: tc.resource, Search: terms, Since: instant.Add(-time.Second), End: instant.Add(time.Second), Limit: 2,
				})
				if err != nil {
					t.Fatal(err)
				}
				assertTextSourceMessages(t, entries, []string{"qa_text_A", "qa_text_B"})
				wantQuery := fmt.Sprintf(`{namespace="tea-logs", %s=%q} |~ "(?i)qa_text_a|qa_text_b"`, tc.selector, tc.resource)
				if got := f.lastValues.Get("query"); got != wantQuery {
					t.Fatalf("datastore history lost terms or scope: got %q, want %q", got, wantQuery)
				}
				for _, entry := range entries {
					if entry.Labels["service"] != tc.resource || entry.Labels[LabelType] != tc.kind {
						t.Errorf("datastore source attribution = %+v", entry.Labels)
					}
				}
			}
		})
	}
}

func assertTextSourceMessages(t *testing.T, entries []LogEntry, want []string) {
	t.Helper()
	messages := make([]string, 0, len(entries))
	seen := map[string]bool{}
	for _, entry := range entries {
		messages = append(messages, entry.Message)
		if key := logID(entry); seen[key] {
			t.Errorf("duplicate log row: %+v", entry)
		} else {
			seen[key] = true
		}
	}
	if !slices.Equal(messages, want) {
		t.Fatalf("messages = %q, want %q", messages, want)
	}
}
