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

package keyvalue

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

// staticKVPodLogs returns a PodLogSource that maps pod names to fixed log
// content, mirroring staticPodLogs in postgres/logs_test.go.
func staticKVPodLogs(podLines map[string]string) core.PodLogSource {
	return func(_ context.Context, _, pod, _ string, _ int64) (io.ReadCloser, error) {
		lines, ok := podLines[pod]
		if !ok {
			return nil, errors.New("no such pod")
		}
		return io.NopCloser(strings.NewReader(lines)), nil
	}
}

func seedKVPods(t *testing.T, svc *Service, kvName string, podNames ...string) {
	t.Helper()
	for _, name := range podNames {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				Labels:    map[string]string{core.PodLabelKeyValue: kvName},
			},
		}
		if err := svc.Client.Create(context.Background(), pod); err != nil {
			t.Fatalf("seed pod %s: %v", name, err)
		}
	}
}

func TestQueryKeyValueLogsNoSource(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "my-kv")
	// PodLogs intentionally not wired.
	_, err := svc.QueryKeyValueLogs(context.Background(), "my-kv", KeyValueLogQuery{})
	if !errors.Is(err, core.ErrLogsUnavailable) {
		t.Errorf("QueryKeyValueLogs without PodLogs = %v, want ErrLogsUnavailable", err)
	}
}

func TestQueryKeyValueLogsDelegatesTypedResource(t *testing.T) {
	id := ids.New(ids.KeyValue)
	want := KeyValueLogEntry{Message: "durable", Labels: map[string]string{"service": id}}
	var gotName string
	var gotQuery KeyValueLogQuery
	svc := &Service{KeyValueLogs: func(_ context.Context, name string, q KeyValueLogQuery) ([]KeyValueLogEntry, error) {
		gotName, gotQuery = name, q
		return []KeyValueLogEntry{want}, nil
	}}
	query := KeyValueLogQuery{Search: "startup", Limit: 5, Instance: []string{"kv-1"}}
	entries, err := svc.QueryKeyValueLogs(context.Background(), id, query)
	if err != nil {
		t.Fatalf("QueryKeyValueLogs: %v", err)
	}
	if gotName != id || gotQuery.Search != query.Search || gotQuery.Limit != query.Limit || gotQuery.Instance[0] != query.Instance[0] {
		t.Fatalf("delegate got name=%q query=%+v, want name=%q query=%+v", gotName, gotQuery, id, query)
	}
	if len(entries) != 1 || entries[0].Message != want.Message || entries[0].Labels["service"] != id {
		t.Fatalf("delegate entries = %+v, want %+v", entries, want)
	}
}

func TestQueryKeyValueLogsUnknownKeyValue(t *testing.T) {
	svc, _ := newService()
	svc.PodLogs = staticKVPodLogs(nil)
	_, err := svc.QueryKeyValueLogs(context.Background(), "no-such-kv", KeyValueLogQuery{})
	if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("QueryKeyValueLogs unknown kv = %v, want ErrNotFound", err)
	}
}

func TestQueryKeyValueLogsReturnsParsedLines(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "my-kv")
	seedKVPods(t, svc, "my-kv", "my-kv-1", "my-kv-2")

	podLines := map[string]string{
		"my-kv-1": "2026-07-15T10:00:00.000000000Z * Ready to accept connections\n2026-07-15T10:00:01.000000000Z # Server is now ready\n",
		"my-kv-2": "2026-07-15T09:59:00.000000000Z * DB loaded from disk\n",
	}
	svc.PodLogs = staticKVPodLogs(podLines)

	entries, err := svc.QueryKeyValueLogs(context.Background(), "my-kv", KeyValueLogQuery{Limit: 10})
	if err != nil {
		t.Fatalf("QueryKeyValueLogs: %v", err)
	}
	// 3 lines from both pods, sorted by timestamp.
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(entries), entries)
	}
	// Oldest first: my-kv-2's 09:59 line.
	if !strings.Contains(entries[0].Timestamp, "09:59") {
		t.Errorf("entries[0] should be the oldest: %+v", entries[0])
	}
	if entries[0].Labels["instance"] != "my-kv-2" {
		t.Errorf("entries[0].Labels[instance] = %q, want my-kv-2", entries[0].Labels["instance"])
	}
	if entries[0].Labels["type"] != "keyvalue" {
		t.Errorf("entries[0].Labels[type] = %q, want keyvalue", entries[0].Labels["type"])
	}
}

func TestQueryKeyValueLogsInstanceFilter(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "my-kv")
	seedKVPods(t, svc, "my-kv", "my-kv-1", "my-kv-2")

	podLines := map[string]string{
		"my-kv-1": "2026-07-15T10:00:00.000000000Z * from-1\n",
		"my-kv-2": "2026-07-15T10:00:01.000000000Z * from-2\n",
	}
	svc.PodLogs = staticKVPodLogs(podLines)

	entries, err := svc.QueryKeyValueLogs(context.Background(), "my-kv", KeyValueLogQuery{Instance: []string{"my-kv-1"}})
	if err != nil {
		t.Fatalf("QueryKeyValueLogs: %v", err)
	}
	if len(entries) != 1 || entries[0].Labels["instance"] != "my-kv-1" {
		t.Errorf("instance filter = %+v, want one entry from my-kv-1", entries)
	}
}

func TestQueryKeyValueLogsTextFilter(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "my-kv")
	seedKVPods(t, svc, "my-kv", "my-kv-1")

	podLines := map[string]string{
		"my-kv-1": "2026-07-15T10:00:00.000000000Z * Ready to accept connections\n2026-07-15T10:00:01.000000000Z # Config reloaded\n",
	}
	svc.PodLogs = staticKVPodLogs(podLines)

	entries, err := svc.QueryKeyValueLogs(context.Background(), "my-kv", KeyValueLogQuery{Search: "ready"})
	if err != nil {
		t.Fatalf("QueryKeyValueLogs: %v", err)
	}
	if len(entries) != 1 || !strings.Contains(strings.ToLower(entries[0].Message), "ready") {
		t.Errorf("text filter = %+v, want only the ready line", entries)
	}
}

func TestGQLKeyValueLogEntryExposesInstanceAndType(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "my-kv")
	seedKVPods(t, svc, "my-kv", "my-kv-1")
	svc.PodLogs = staticKVPodLogs(map[string]string{
		"my-kv-1": "2026-07-16T12:00:00.000000000Z * Ready to accept connections\n",
	})
	schema, err := kvGQLSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "any", Method: "session"})
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ keyValueLogs(id:"my-kv") { timestamp message instance type } }`,
		Context:       ctx,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("gql keyValueLogs: %v", res.Errors)
	}
	rows, _ := res.Data.(map[string]any)["keyValueLogs"].([]any)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	row := rows[0].(map[string]any)
	if row["instance"] != "my-kv-1" {
		t.Errorf("instance = %v, want my-kv-1", row["instance"])
	}
	if row["type"] != "keyvalue" {
		t.Errorf("type = %v, want keyvalue", row["type"])
	}
}

func TestQueryKeyValueLogsCrossWorkspaceIsNotFound(t *testing.T) {
	svc, cl := newService()
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	seedKeyValue(t, cl, "my-kv")
	// Label the KV as belonging to tea-a.
	kv := &appv1alpha1.KeyValue{}
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "my-kv"}, kv); err != nil {
		t.Fatalf("get kv: %v", err)
	}
	kv.Labels = map[string]string{core.LabelTenant: "tea-a"}
	if err := cl.Update(context.Background(), kv); err != nil {
		t.Fatalf("update kv labels: %v", err)
	}

	svcOther := &Service{Base: &core.Base{Client: svc.Client, Namespace: "default", Workspace: fakeWorkspace{"user-b": "tea-b"}}}
	svcOther.PodLogs = staticKVPodLogs(nil)

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-b", Method: "session"})
	_, err := svcOther.QueryKeyValueLogs(ctx, "my-kv", KeyValueLogQuery{})
	if !errors.Is(err, core.ErrForbidden) {
		t.Errorf("cross-workspace QueryKeyValueLogs = %v, want ErrForbidden", err)
	}
}
