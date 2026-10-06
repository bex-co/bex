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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func admissionReplicas(n int) []appv1alpha1.DatabaseReadReplica {
	result := make([]appv1alpha1.DatabaseReadReplica, n)
	for i := range result {
		result[i].Name = fmt.Sprintf("reader-%d", i+1)
	}
	return result
}

func TestDatabaseAdmissionCreateCapacity(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec appv1alpha1.DatabaseSpec
		code string
	}{
		{"default free", appv1alpha1.DatabaseSpec{}, ""},
		{"explicit free storage", appv1alpha1.DatabaseSpec{Plan: "free", StorageGB: 1}, ""},
		{"negative storage", appv1alpha1.DatabaseSpec{StorageGB: -1}, "POSTGRES_STORAGE_INVALID"},
		{"free storage expansion", appv1alpha1.DatabaseSpec{StorageGB: 5}, "POSTGRES_STORAGE_PLAN_UNSUPPORTED"},
		{"free replicas", appv1alpha1.DatabaseSpec{ReadReplicas: admissionReplicas(1)}, "POSTGRES_READ_REPLICA_PLAN_UNSUPPORTED"},
		{"small paid replicas", appv1alpha1.DatabaseSpec{Plan: "basic-256mb", StorageGB: 10, ReadReplicas: admissionReplicas(1)}, "POSTGRES_READ_REPLICA_PLAN_UNSUPPORTED"},
		{"replica storage floor", appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: 9, ReadReplicas: admissionReplicas(1)}, "POSTGRES_READ_REPLICA_STORAGE_UNSUPPORTED"},
		{"five replicas", appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: 10, ReadReplicas: admissionReplicas(5)}, ""},
		{"replica plan alias", appv1alpha1.DatabaseSpec{Plan: "0.5c-1g", StorageGB: 10, ReadReplicas: admissionReplicas(1)}, ""},
		{"six replicas", appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: 10, ReadReplicas: admissionReplicas(6)}, "POSTGRES_READ_REPLICA_LIMIT_EXCEEDED"},
		{"free autoscaling", appv1alpha1.DatabaseSpec{DiskAutoscaling: true}, "POSTGRES_DISK_AUTOSCALING_PLAN_UNSUPPORTED"},
		{"free pooler", appv1alpha1.DatabaseSpec{Pooler: true}, "POSTGRES_CONNECTION_POOL_PLAN_UNSUPPORTED"},
		{"paid features", appv1alpha1.DatabaseSpec{Plan: "basic-256mb", DiskAutoscaling: true, Pooler: true}, ""},
		{"paid maximum storage", appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: 16384}, ""},
		{"past paid maximum", appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: 16385}, "POSTGRES_STORAGE_LIMIT_EXCEEDED"},
		{"unknown plan cannot bypass", appv1alpha1.DatabaseSpec{Plan: "made-up", Pooler: true}, "POSTGRES_PLAN_UNSUPPORTED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckDatabaseAdmission(nil, tc.spec)
			if tc.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var coded *core.CodedError
			if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != tc.code {
				t.Fatalf("admission = %v, want coded %s with a field", err, tc.code)
			}
			if field, _ := coded.Params["field"].(string); field == "" {
				t.Fatalf("admission error has no input field: %+v", coded.Params)
			}
		})
	}
}

func TestDatabaseAdmissionPreservesLegacyStateAndStorageHighWater(t *testing.T) {
	legacy := &appv1alpha1.Database{
		Spec:   appv1alpha1.DatabaseSpec{Plan: "free", StorageGB: 15, DiskAutoscaling: true, Pooler: true, ReadReplicas: admissionReplicas(1)},
		Status: appv1alpha1.DatabaseStatus{AllocatedStorageGB: 20},
	}
	for _, change := range []func(*appv1alpha1.DatabaseSpec){
		func(s *appv1alpha1.DatabaseSpec) { s.Name = "renamed" },
		func(s *appv1alpha1.DatabaseSpec) { s.DiskAutoscaling, s.Pooler, s.ReadReplicas = false, false, nil },
		func(s *appv1alpha1.DatabaseSpec) { s.Plan = "basic-1gb" },
	} {
		before := legacy.DeepCopy()
		desired := legacy.DeepCopy().Spec
		change(&desired)
		if err := CheckDatabaseAdmission(legacy, desired); err != nil {
			t.Fatalf("legacy-preserving update refused: %v", err)
		}
		if !reflect.DeepEqual(legacy, before) || desired.StorageGB != 15 {
			t.Fatal("admission mutated existing data or silently shrank storage")
		}
	}
	// The operator's allocation can satisfy the replica storage floor even
	// when no explicit storage override has been written to the spec.
	allocated := &appv1alpha1.Database{
		Spec: appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, Status: appv1alpha1.DatabaseStatus{AllocatedStorageGB: 10},
	}
	desired := allocated.Spec
	desired.ReadReplicas = admissionReplicas(1)
	if err := CheckDatabaseAdmission(allocated, desired); err != nil {
		t.Fatalf("allocated replica storage ignored: %v", err)
	}
	desired.StorageGB = 5
	if err := CheckDatabaseAdmission(allocated, desired); err == nil || !strings.Contains(err.Error(), "grow-only") {
		t.Fatalf("allocated storage shrink = %v", err)
	}
}

func TestDatabaseAdmissionRefusesPlanChangesAcrossWriteAndPreviewPaths(t *testing.T) {
	for _, tc := range []struct {
		name, target, code string
		spec               appv1alpha1.DatabaseSpec
		allocated          int32
	}{
		{"replica CPU", "basic-256mb", "POSTGRES_READ_REPLICA_PLAN_UNSUPPORTED", appv1alpha1.DatabaseSpec{Plan: "basic-1gb", StorageGB: 10, ReadReplicas: admissionReplicas(1)}, 0},
		{"included storage", "free", "POSTGRES_STORAGE_PLAN_UNSUPPORTED", appv1alpha1.DatabaseSpec{Plan: "basic-1gb"}, 0},
		{"allocated storage", "free", "POSTGRES_STORAGE_PLAN_UNSUPPORTED", appv1alpha1.DatabaseSpec{Plan: "basic-256mb"}, 5},
		{"autoscaling", "free", "POSTGRES_DISK_AUTOSCALING_PLAN_UNSUPPORTED", appv1alpha1.DatabaseSpec{Plan: "basic-256mb", DiskAutoscaling: true}, 0},
		{"pooler", "free", "POSTGRES_CONNECTION_POOL_PLAN_UNSUPPORTED", appv1alpha1.DatabaseSpec{Plan: "basic-256mb", Pooler: true}, 0},
	} {
		for _, path := range []string{"set", "preview set", "patch", "preview patch"} {
			t.Run(tc.name+"/"+path, func(t *testing.T) {
				db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"}, Spec: tc.spec,
					Status: appv1alpha1.DatabaseStatus{AllocatedStorageGB: tc.allocated}}
				svc, cl := newService(db)
				sink := &webhookAuditSink{}
				svc.Audit = sink
				before := getDatabase(t, cl, "db")
				ctx := context.Background()
				var err error
				switch path {
				case "set":
					_, err = svc.SetPlan(ctx, "db", tc.target)
				case "preview set":
					_, err = svc.SetPlanDryRun(ctx, "db", tc.target)
				case "patch":
					_, err = svc.UpdatePostgres(ctx, "db", PostgresPatch{Plan: &tc.target})
				case "preview patch":
					_, err = svc.UpdatePostgresDryRun(ctx, "db", PostgresPatch{Plan: &tc.target})
				}
				var coded *core.CodedError
				if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != tc.code {
					t.Fatalf("plan change = %v, want %s", err, tc.code)
				}
				if !reflect.DeepEqual(before, getDatabase(t, cl, "db")) || len(sink.events) != 0 {
					t.Fatal("refused plan change changed the database or recorded success")
				}
			})
		}
	}
}

func TestDatabaseAdmissionAllowsCombinedFeatureDisableAndDowngrade(t *testing.T) {
	db := &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Spec: appv1alpha1.DatabaseSpec{Plan: "basic-256mb", StorageGB: 1, Pooler: true, DiskAutoscaling: true}}
	svc, cl := newService(db)
	plan, disabled := "free", false
	patch := PostgresPatch{Plan: &plan, EnableDiskAutoscaling: &disabled, Pooler: &disabled}
	if _, err := svc.UpdatePostgresDryRun(context.Background(), "db", patch); err != nil {
		t.Fatal(err)
	}
	if getDatabase(t, cl, "db").Spec.Plan != "basic-256mb" {
		t.Fatal("preview wrote the plan")
	}
	if _, err := svc.UpdatePostgres(context.Background(), "db", patch); err != nil {
		t.Fatal(err)
	}
	got := getDatabase(t, cl, "db").Spec
	if got.Plan != "free" || got.Pooler || got.DiskAutoscaling || got.StorageGB != 1 {
		t.Fatalf("combined update = %+v", got)
	}
}

func TestDatabaseAdmissionCreateAndDryRunAcrossSurfaces(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  CreatePostgresRequest
		code string
	}{
		{"free replica", CreatePostgresRequest{ReadReplicas: []ReadReplicaInput{{Name: "reader"}}}, "POSTGRES_READ_REPLICA_PLAN_UNSUPPORTED"},
		{"replica storage", CreatePostgresRequest{Plan: "basic-1gb", ReadReplicas: []ReadReplicaInput{{Name: "reader"}}}, "POSTGRES_READ_REPLICA_STORAGE_UNSUPPORTED"},
		{"free disk", CreatePostgresRequest{DiskSizeGB: 5}, "POSTGRES_STORAGE_PLAN_UNSUPPORTED"},
		{"free autoscaling", CreatePostgresRequest{EnableDiskAutoscaling: true}, "POSTGRES_DISK_AUTOSCALING_PLAN_UNSUPPORTED"},
		{"free pooler", CreatePostgresRequest{ConnectionPool: "pgbouncer"}, "POSTGRES_CONNECTION_POOL_PLAN_UNSUPPORTED"},
		{"eligible paid", CreatePostgresRequest{Plan: "0.5c-1g", DiskSizeGB: 10, ReadReplicas: []ReadReplicaInput{{Name: "reader"}}, EnableDiskAutoscaling: true, ConnectionPool: "pgbouncer"}, ""},
	} {
		for _, surface := range []string{"REST", "GraphQL", "MCP"} {
			for _, dryRun := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/dry=%v", tc.name, surface, dryRun), func(t *testing.T) {
					svc, cl := newService()
					sink := &webhookAuditSink{}
					svc.Audit = sink
					req := tc.req
					req.Name, req.DryRun = "candidate", dryRun
					id, code := admissionCreateCall(t, svc, surface, req)
					if code != tc.code {
						t.Fatalf("error code = %q, want %q", code, tc.code)
					}
					if tc.code != "" || dryRun {
						if countDatabases(t, cl) != 0 || len(sink.events) != 0 {
							t.Fatal("refused or preview create wrote a database or success event")
						}
						return
					}
					got := getDatabase(t, cl, id).Spec
					if got.Plan != "basic-1gb" || got.StorageGB != 10 || !got.DiskAutoscaling || !got.Pooler ||
						got.HighAvailability || len(got.ReadReplicas) != 1 || got.ReadReplicas[0].Name != "reader" {
						t.Fatalf("eligible replica create lost declared fields: %+v", got)
					}
				})
			}
		}
	}
}

func admissionCreateCall(t *testing.T, svc *Service, surface string, req CreatePostgresRequest) (string, string) {
	t.Helper()
	ctx := context.Background()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var args map[string]any
	if err := json.Unmarshal(body, &args); err != nil {
		t.Fatal(err)
	}
	switch surface {
	case "REST":
		res := serveREST(svc, http.MethodPost, "/v1/postgres", string(body))
		var out struct{ ID, Code string }
		if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if res.Code != http.StatusBadRequest && res.Code != http.StatusCreated && res.Code != http.StatusOK {
			t.Fatalf("create status = %d: %s", res.Code, res.Body)
		}
		return out.ID, out.Code
	case "GraphQL":
		schema, err := pgGQLSchema(svc)
		if err != nil {
			t.Fatal(err)
		}
		res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, VariableValues: args,
			RequestString: `mutation($name:String!, $plan:String, $diskSizeGB:Int, $readReplicas:[DatabaseReadReplicaInput!], $connectionPool:String, $enableDiskAutoscaling:Boolean, $dryRun:Boolean) {
				createDatabase(name:$name, plan:$plan, diskSizeGB:$diskSizeGB, readReplicas:$readReplicas, connectionPool:$connectionPool, enableDiskAutoscaling:$enableDiskAutoscaling, dryRun:$dryRun) { id }
			}`})
		if len(res.Errors) != 0 {
			if len(res.Errors) != 1 || res.Errors[0].Extensions["code"] == nil {
				t.Fatalf("GraphQL create errors = %+v", res.Errors)
			}
			return "", res.Errors[0].Extensions["code"].(string)
		}
		return res.Data.(map[string]any)["createDatabase"].(map[string]any)["id"].(string), ""
	case "MCP":
		session, cleanup := pgMCPSession(t, svc)
		defer cleanup()
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "create_postgres", Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			return "", admissionMCPErrorCode(t, res)
		}
		var view PostgresView
		body, _ := json.Marshal(res.StructuredContent)
		if err := json.Unmarshal(body, &view); err != nil {
			t.Fatal(err)
		}
		return view.ID, ""
	default:
		t.Fatalf("unknown surface %q", surface)
		return "", ""
	}
}

func admissionMCPErrorCode(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if !res.IsError || len(res.Content) != 1 {
		t.Fatalf("expected one named MCP error, got %+v", res)
	}
	content, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("MCP error content = %+v", res.Content)
	}
	code, _, _ := strings.Cut(content.Text, ": ")
	return code
}

func TestMCPPostgresCapacityPatchUsesAdmission(t *testing.T) {
	svc, cl := newService()
	seedDatabase(t, cl, "db")
	before := getDatabase(t, cl, "db")
	session, cleanup := pgMCPSession(t, svc)
	defer cleanup()
	for field, value := range map[string]any{"diskSizeGB": 5, "connectionPool": "pgbouncer", "enableDiskAutoscaling": true} {
		for _, dryRun := range []bool{false, true} {
			res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "update_postgres",
				Arguments: map[string]any{"postgresId": "db", field: value, "dryRun": dryRun}})
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"diskSizeGB": "POSTGRES_STORAGE_PLAN_UNSUPPORTED", "connectionPool": "POSTGRES_CONNECTION_POOL_PLAN_UNSUPPORTED", "enableDiskAutoscaling": "POSTGRES_DISK_AUTOSCALING_PLAN_UNSUPPORTED"}[field]
			if got := admissionMCPErrorCode(t, res); got != want || !reflect.DeepEqual(before, getDatabase(t, cl, "db")) {
				t.Fatalf("%s dry=%v: code=%s, want %s and unchanged database", field, dryRun, got, want)
			}
		}
	}
	call, closeClient := pgMCPClient(t, svc)
	defer closeClient()
	call("update_postgres", map[string]any{"postgresId": "db", "plan": "basic-256mb", "diskSizeGB": 5, "connectionPool": "pgbouncer", "enableDiskAutoscaling": true})
	got := getDatabase(t, cl, "db").Spec
	if got.Plan != "basic-256mb" || got.StorageGB != 5 || !got.Pooler || !got.DiskAutoscaling {
		t.Fatalf("combined paid upgrade = %+v", got)
	}
}

func TestDatabaseAdmissionPlanMetadataMatchesCatalog(t *testing.T) {
	svc, _ := newService()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()})})
	if err != nil {
		t.Fatal(err)
	}
	result := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ databaseInstanceTypes { id maxStorageGB supportsDiskAutoscaling supportsConnectionPooling maxReadReplicas readReplicaMinStorageGB } }`})
	if len(result.Errors) != 0 {
		t.Fatal(result.Errors)
	}
	var data struct {
		DatabaseInstanceTypes []DatabaseInstanceType
	}
	body, _ := json.Marshal(result.Data)
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.DatabaseInstanceTypes) != len(tiers.Postgres.IDs()) {
		t.Fatalf("plan metadata = %+v", data)
	}
	for _, got := range data.DatabaseInstanceTypes {
		tier, ok := tiers.Postgres.ByID(got.ID)
		if !ok || got.MaxStorageGB != tiers.Postgres.MaxStorageGB(got.ID) ||
			got.SupportsDiskAutoscaling != tier.SupportsDiskAutoscaling() || got.SupportsConnectionPooling != tier.SupportsConnectionPooling() ||
			got.MaxReadReplicas != tier.MaxReadReplicas() || got.ReadReplicaMinStorageGB != tiers.PostgresReadReplicaMinStorageGB {
			t.Fatalf("plan metadata disagrees with admission: %+v", got)
		}
	}
}
