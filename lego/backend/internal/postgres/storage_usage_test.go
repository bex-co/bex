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
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w3/m656: Render's dashboard `database` query reads storageUsedPercent,
// storageUsed and storageTotal for its almost-full warning. bex answers them
// on GraphQL only, from the fullest instance volume over the logical size.

const gib = 1 << 30

func storageDatabase(name string, storageGB int32) *appv1alpha1.Database {
	return &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Name: name, Plan: "basic-1gb", StorageGB: storageGB},
	}
}

// denyRelationChecker allows every relation but one.
type denyRelationChecker string

func (d denyRelationChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return relation != string(d), nil
}

// countingStorage is a StorageUsedSource answering used for every database,
// counting its reads.
type countingStorage struct {
	used  float64
	ok    bool
	err   error
	block bool
	calls atomic.Int32
	asked []string
}

func (c *countingStorage) read(ctx context.Context, namespace, database string) (float64, bool, error) {
	c.calls.Add(1)
	c.asked = append(c.asked, namespace+"/"+database)
	if c.block {
		<-ctx.Done()
		return 0, false, ctx.Err()
	}
	return c.used, c.ok, c.err
}

func storageQuery(t *testing.T, svc *Service, query string) map[string]any {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctxAs("user-a"), RequestString: query})
	if len(res.Errors) > 0 {
		t.Fatalf("%s: %v", query, res.Errors)
	}
	return res.Data.(map[string]any)
}

const storageFields = `{ database(id: "dpg-a") { id name storageTotal storageUsed storageUsedPercent } }`

func TestDatabaseStorageFieldsReadTheFullestVolumeOverTheLogicalSize(t *testing.T) {
	svc, _ := newService(storageDatabase("dpg-a", 10))
	src := &countingStorage{used: 8.5 * gib, ok: true}
	svc.StorageUsed = src.read

	got := storageQuery(t, svc, storageFields)["database"].(map[string]any)
	if got["storageTotal"] != "10737418240" || got["storageUsed"] != "9126805504" || got["storageUsedPercent"] != 85.0 {
		t.Fatalf("storage fields = %v, want 10 GiB total, 8.5 GiB used, 85%%", got)
	}
	// One read serves both fields, against the database's own namespace.
	if src.calls.Load() != 1 || src.asked[0] != "default/dpg-a" {
		t.Errorf("usage reads = %d %v, want one for default/dpg-a", src.calls.Load(), src.asked)
	}
}

func TestDatabaseStorageTotalIsTheMetricsPagesCapacity(t *testing.T) {
	// A Free database's logical 1 GB, not its 10 GiB physical volume: a
	// database that wrote past it reads past 100%.
	db := storageDatabase("dpg-a", 0)
	db.Spec.Plan = "free"
	svc, _ := newService(db)
	svc.StorageUsed = (&countingStorage{used: 2 * gib, ok: true}).read

	got := storageQuery(t, svc, storageFields)["database"].(map[string]any)
	if got["storageTotal"] != "1073741824" || got["storageUsedPercent"] != 200.0 {
		t.Fatalf("storage fields = %v, want 1 GiB total and 200%%", got)
	}
}

func TestDatabaseStorageUsageIsNullWhenItCannotBeRead(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  *countingStorage
	}{
		{"no sample", &countingStorage{}},
		{"Prometheus error", &countingStorage{err: errors.New("prometheus: status 503")}},
		{"Prometheus too slow", &countingStorage{block: true}},
		{"no source", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func(timeout time.Duration) { storageReadTimeout = timeout }(storageReadTimeout)
			storageReadTimeout = 50 * time.Millisecond
			svc, _ := newService(storageDatabase("dpg-a", 10))
			if tc.src != nil {
				svc.StorageUsed = tc.src.read
			}
			start := time.Now()
			got := storageQuery(t, svc, storageFields)["database"].(map[string]any)
			if got["storageUsed"] != nil || got["storageUsedPercent"] != nil {
				t.Fatalf("usage = %v, want null", got)
			}
			// The rest of the read is unaffected.
			if got["name"] != "dpg-a" || got["storageTotal"] != "10737418240" {
				t.Errorf("database = %v, want its other fields", got)
			}
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Errorf("read took %v, want it bounded by %v", elapsed, storageReadTimeout)
			}
		})
	}
}

// The Metrics page reads the same series under can_view_activity; a member
// denied it (a Billing member) reads null, with no usage read and no error.
func TestDatabaseStorageUsageFollowsTheActivityPermission(t *testing.T) {
	svc, _ := newService(storageDatabase("dpg-a", 10))
	svc.Authz = denyRelationChecker(core.RelCanViewActivity)
	src := &countingStorage{used: 9 * gib, ok: true}
	svc.StorageUsed = src.read

	got := storageQuery(t, svc, storageFields)["database"].(map[string]any)
	if got["storageUsed"] != nil || got["storageUsedPercent"] != nil || got["storageTotal"] != "10737418240" {
		t.Fatalf("storage fields = %v, want usage null and the size kept", got)
	}
	if src.calls.Load() != 0 {
		t.Errorf("usage reads = %d, want none for a caller denied the activity", src.calls.Load())
	}
}

func TestDatabasesListReadsUsageOnlyWhenSelected(t *testing.T) {
	svc, _ := newService(storageDatabase("dpg-a", 10), storageDatabase("dpg-b", 10))
	src := &countingStorage{used: gib, ok: true}
	svc.StorageUsed = src.read

	storageQuery(t, svc, `{ databases { id name diskSizeGB storageTotal } }`)
	if src.calls.Load() != 0 {
		t.Fatalf("a list without the usage fields made %d usage reads", src.calls.Load())
	}
	storageQuery(t, svc, `{ databases { id storageUsed storageUsedPercent } }`)
	if src.calls.Load() != 2 {
		t.Fatalf("usage reads = %d, want one per database", src.calls.Load())
	}
}

// Render's public REST and MCP objects carry no usage field, and wiring the
// usage source changes neither byte for byte.
func TestRESTAndMCPPostgresObjectsCarryNoStorageUsage(t *testing.T) {
	bodies := func(src StorageUsedSource) (string, string) {
		svc, _ := newService(storageDatabase("dpg-a", 10))
		svc.StorageUsed = src
		rec := serveREST(svc, http.MethodGet, "/v1/postgres/dpg-a", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("REST get = %d %s", rec.Code, rec.Body)
		}
		res := pgMCPCall(t, svc)("get_postgres", map[string]any{"postgresId": "dpg-a"})
		return rec.Body.String(), rendertest.MCPText(t, res)
	}
	src := &countingStorage{used: 9 * gib, ok: true}
	restWired, mcpWired := bodies(src.read)
	restBare, mcpBare := bodies(nil)
	if restWired != restBare || mcpWired != mcpBare {
		t.Errorf("wiring the usage source changed REST or MCP:\nREST %s\nvs   %s\nMCP  %s\nvs   %s", restWired, restBare, mcpWired, mcpBare)
	}
	if src.calls.Load() != 0 {
		t.Errorf("REST and MCP made %d usage reads", src.calls.Load())
	}
}
