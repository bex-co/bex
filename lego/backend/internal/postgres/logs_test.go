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

package postgres

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// staticPodLogs returns a PodLogSource that maps pod names to fixed log lines.
func staticPodLogs(podLines map[string]string) core.PodLogSource {
	return func(_ context.Context, _, pod, _ string, _ int64) (io.ReadCloser, error) {
		lines, ok := podLines[pod]
		if !ok {
			return nil, errors.New("no such pod")
		}
		return io.NopCloser(strings.NewReader(lines)), nil
	}
}

func seedPods(t *testing.T, svc *Service, dbName string, podNames ...string) {
	t.Helper()
	for _, name := range podNames {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				Labels:    map[string]string{core.PodLabelCNPGCluster: dbName},
			},
		}
		if err := svc.Client.Create(context.Background(), pod); err != nil {
			t.Fatalf("seed pod %s: %v", name, err)
		}
	}
}

func TestQueryDatabaseLogsNoSource(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "my-pg")
	// PodLogs intentionally not wired.
	_, err := svc.QueryDatabaseLogs(context.Background(), "my-pg", DatabaseLogQuery{})
	if !errors.Is(err, core.ErrLogsUnavailable) {
		t.Errorf("QueryDatabaseLogs without PodLogs = %v, want ErrLogsUnavailable", err)
	}
}

func TestQueryDatabaseLogsDelegatesTypedResource(t *testing.T) {
	id := ids.New(ids.Postgres)
	want := DatabaseLogEntry{Message: "durable", Labels: map[string]string{"service": id}}
	var gotName string
	var gotQuery DatabaseLogQuery
	svc := &Service{DatabaseLogs: func(_ context.Context, name string, q DatabaseLogQuery) ([]DatabaseLogEntry, error) {
		gotName, gotQuery = name, q
		return []DatabaseLogEntry{want}, nil
	}}
	query := DatabaseLogQuery{Search: "checkpoint", Limit: 7, Instance: []string{"db-1"}}
	entries, err := svc.QueryDatabaseLogs(context.Background(), id, query)
	if err != nil {
		t.Fatalf("QueryDatabaseLogs: %v", err)
	}
	if gotName != id || gotQuery.Search != query.Search || gotQuery.Limit != query.Limit || gotQuery.Instance[0] != query.Instance[0] {
		t.Fatalf("delegate got name=%q query=%+v, want name=%q query=%+v", gotName, gotQuery, id, query)
	}
	if len(entries) != 1 || entries[0].Message != want.Message || entries[0].Labels["service"] != id {
		t.Fatalf("delegate entries = %+v, want %+v", entries, want)
	}
}

func TestQueryDatabaseLogsUnknownDatabase(t *testing.T) {
	svc, _ := newService()
	svc.PodLogs = staticPodLogs(nil)
	_, err := svc.QueryDatabaseLogs(context.Background(), "no-such-db", DatabaseLogQuery{})
	if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("QueryDatabaseLogs unknown db = %v, want ErrNotFound", err)
	}
}

func TestQueryDatabaseLogsReturnsParsedLines(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "my-pg")
	seedPods(t, svc, "my-pg", "my-pg-1", "my-pg-2")

	podLines := map[string]string{
		"my-pg-1": "2026-07-15T10:00:00.000000000Z LOG: checkpoint starting: time\n2026-07-15T10:00:01.000000000Z LOG: checkpoint complete\n",
		"my-pg-2": "2026-07-15T09:59:00.000000000Z LOG: autovacuum: VACUUM my_table\n",
	}
	svc.PodLogs = staticPodLogs(podLines)

	entries, err := svc.QueryDatabaseLogs(context.Background(), "my-pg", DatabaseLogQuery{Limit: 10})
	if err != nil {
		t.Fatalf("QueryDatabaseLogs: %v", err)
	}
	// All 3 lines from both pods, sorted by timestamp.
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(entries), entries)
	}
	// Oldest first: my-pg-2's 09:59 line, then my-pg-1's two 10:00 lines.
	if !strings.Contains(entries[0].Timestamp, "09:59") {
		t.Errorf("entries[0] should be the oldest: %+v", entries[0])
	}
	// Labels set correctly.
	if entries[0].Labels["instance"] != "my-pg-2" {
		t.Errorf("entries[0].Labels[instance] = %q, want my-pg-2", entries[0].Labels["instance"])
	}
	if entries[0].Labels["type"] != "postgres" {
		t.Errorf("entries[0].Labels[type] = %q, want postgres", entries[0].Labels["type"])
	}
}

func TestQueryDatabaseLogsInstanceFilter(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "my-pg")
	seedPods(t, svc, "my-pg", "my-pg-1", "my-pg-2")

	podLines := map[string]string{
		"my-pg-1": "2026-07-15T10:00:00.000000000Z LOG: from-1\n",
		"my-pg-2": "2026-07-15T10:00:01.000000000Z LOG: from-2\n",
	}
	svc.PodLogs = staticPodLogs(podLines)

	entries, err := svc.QueryDatabaseLogs(context.Background(), "my-pg", DatabaseLogQuery{Instance: []string{"my-pg-1"}})
	if err != nil {
		t.Fatalf("QueryDatabaseLogs: %v", err)
	}
	if len(entries) != 1 || entries[0].Labels["instance"] != "my-pg-1" {
		t.Errorf("instance filter = %+v, want one entry from my-pg-1", entries)
	}
}

func TestQueryDatabaseLogsTextFilter(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "my-pg")
	seedPods(t, svc, "my-pg", "my-pg-1")

	podLines := map[string]string{
		"my-pg-1": "2026-07-15T10:00:00.000000000Z LOG: checkpoint complete\n2026-07-15T10:00:01.000000000Z LOG: autovacuum ran\n",
	}
	svc.PodLogs = staticPodLogs(podLines)

	entries, err := svc.QueryDatabaseLogs(context.Background(), "my-pg", DatabaseLogQuery{Search: "checkpoint"})
	if err != nil {
		t.Fatalf("QueryDatabaseLogs: %v", err)
	}
	if len(entries) != 1 || !strings.Contains(entries[0].Message, "checkpoint") {
		t.Errorf("text filter = %+v, want only the checkpoint line", entries)
	}
}

func TestGQLDatabaseLogEntryExposesInstanceAndType(t *testing.T) {
	// Regression for w2/m50: databaseLogGQLType previously omitted "instance"
	// and "type" even though the service layer populated Labels with both. This
	// test drives the real GraphQL schema to confirm the fields are present and
	// correctly resolved.
	svc, cl := newService()
	seedDatabase(t, cl, "my-pg")
	seedPods(t, svc, "my-pg", "my-pg-1")
	svc.PodLogs = staticPodLogs(map[string]string{
		"my-pg-1": "2026-07-16T12:00:00.000000000Z LOG: checkpoint complete\n",
	})
	schema, err := pgGQLSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "any", Method: "session"})
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ databaseLogs(id:"my-pg") { timestamp message instance type } }`,
		Context:       ctx,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql databaseLogs: %v", res.Errors)
	}
	rows, _ := res.Data.(map[string]any)["databaseLogs"].([]any)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	row := rows[0].(map[string]any)
	if row["instance"] != "my-pg-1" {
		t.Errorf("instance = %v, want my-pg-1", row["instance"])
	}
	if row["type"] != "postgres" {
		t.Errorf("type = %v, want postgres", row["type"])
	}
}

func TestQueryDatabaseLogsCrossWorkspaceIsNotFound(t *testing.T) {
	svc, cl := newService()
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	seedDatabase(t, cl, "my-pg")
	// Label the DB as belonging to tea-a.
	db := &appv1alpha1.Database{}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "my-pg"}, db); err != nil {
		t.Fatalf("get db: %v", err)
	}
	db.Labels = map[string]string{core.LabelTenant: "tea-a"}
	if err := cl.Update(context.Background(), db); err != nil {
		t.Fatalf("update db labels: %v", err)
	}

	svcOther := &Service{Base: &core.Base{Client: svc.Client, Namespace: "default", Workspace: fakeWorkspace{"user-b": "tea-b"}}}
	svcOther.PodLogs = staticPodLogs(nil)

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-b", Method: "session"})
	_, err := svcOther.QueryDatabaseLogs(ctx, "my-pg", DatabaseLogQuery{})
	if !errors.Is(err, core.ErrForbidden) {
		t.Errorf("cross-workspace QueryDatabaseLogs = %v, want ErrForbidden", err)
	}
}
