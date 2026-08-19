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
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func versionDatabase(name, plan, version string, backupsEnabled bool) *appv1alpha1.Database {
	return &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       appv1alpha1.DatabaseSpec{Plan: plan, Version: version},
		Status: appv1alpha1.DatabaseStatus{
			Phase: appv1alpha1.DBPhaseReady, CurrentVersion: version,
			BackupsEnabled: backupsEnabled,
		},
	}
}

func completedVersionBackup(name, serverName string) *unstructured.Unstructured {
	backup := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "postgresql.cnpg.io/v1",
		"kind":       "Backup",
		"metadata": map[string]any{
			"name": name + "-backup-1", "namespace": "default",
			"labels": map[string]any{labelCNPGCluster: name},
		},
		"status": map[string]any{"phase": "completed", "serverName": serverName},
	}}
	backup.SetGroupVersionKind(cnpgBackupGVK)
	return backup
}

func codedErrorCode(err error) string {
	var coded *core.CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

func TestSetVersionValidationAndBackupGuard(t *testing.T) {
	tests := []struct {
		name               string
		plan               string
		current            string
		target             string
		backupsEnabled     bool
		backupServerName   string
		backupObjectServer string
		unobserved         bool
		wantCode           string
		wantVersion        string
	}{
		{name: "free upward", plan: "free", target: "17", wantVersion: "17"},
		{name: "version not observed", plan: "free", target: "17", unobserved: true, wantCode: "POSTGRES_VERSION_NOT_OBSERVED", wantVersion: "16"},
		{name: "unknown", plan: "free", target: "19", wantCode: "POSTGRES_VERSION_UNKNOWN", wantVersion: "16"},
		{name: "same", plan: "free", target: "16", wantCode: "POSTGRES_VERSION_NOT_NEWER", wantVersion: "16"},
		{name: "downgrade", plan: "free", target: "15", wantCode: "POSTGRES_VERSION_NOT_NEWER", wantVersion: "16"},
		{name: "durable backup disabled", plan: "basic-1gb", target: "17", wantCode: "POSTGRES_UPGRADE_BACKUP_REQUIRED", wantVersion: "16"},
		{name: "durable backup pending", plan: "basic-1gb", target: "17", backupsEnabled: true, wantCode: "POSTGRES_UPGRADE_BACKUP_REQUIRED", wantVersion: "16"},
		{name: "durable completed backup", plan: "basic-1gb", target: "17", backupsEnabled: true, backupObjectServer: "upgrade-db", wantVersion: "17"},
		{name: "durable stale generation", plan: "basic-1gb", current: "17", target: "18", backupsEnabled: true, backupServerName: "upgrade-db-pg17", backupObjectServer: "upgrade-db", wantCode: "POSTGRES_UPGRADE_BACKUP_REQUIRED", wantVersion: "17"},
		{name: "durable current generation", plan: "basic-1gb", current: "17", target: "18", backupsEnabled: true, backupServerName: "upgrade-db-pg17", backupObjectServer: "upgrade-db-pg17", wantVersion: "18"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := tt.current
			if current == "" {
				current = "16"
			}
			db := versionDatabase("upgrade-db", tt.plan, current, tt.backupsEnabled)
			if tt.unobserved {
				db.Status.CurrentVersion = ""
			}
			db.Status.BackupServerName = tt.backupServerName
			objects := []client.Object{db}
			if tt.backupObjectServer != "" {
				objects = append(objects, completedVersionBackup("upgrade-db", tt.backupObjectServer))
			}
			svc, cl := newService(objects...)
			_, err := svc.SetVersion(context.Background(), "upgrade-db", tt.target)
			if got := codedErrorCode(err); got != tt.wantCode {
				t.Fatalf("error code = %q (err %v), want %q", got, err, tt.wantCode)
			}
			var got appv1alpha1.Database
			if getErr := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "upgrade-db"}, &got); getErr != nil {
				t.Fatal(getErr)
			}
			if got.Spec.Version != tt.wantVersion {
				t.Errorf("spec.version = %q, want %q", got.Spec.Version, tt.wantVersion)
			}
		})
	}
}

func TestRESTVersionUpgradeNamedErrorsAndPatch(t *testing.T) {
	svc, cl := newService(versionDatabase("rest-upgrade", "free", "16", false))

	bad := serveREST(svc, "PATCH", "/v1/postgres/rest-upgrade", `{"version":"19"}`)
	if bad.Code != 400 {
		t.Fatalf("unknown version = %d: %s", bad.Code, bad.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(bad.Body.Bytes(), &body)
	if body["code"] != "POSTGRES_VERSION_UNKNOWN" {
		t.Errorf("error code = %v", body["code"])
	}

	ok := serveREST(svc, "PATCH", "/v1/postgres/rest-upgrade", `{"version":"17"}`)
	if ok.Code != 200 {
		t.Fatalf("upgrade = %d: %s", ok.Code, ok.Body.String())
	}
	var got appv1alpha1.Database
	_ = cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "rest-upgrade"}, &got)
	if got.Spec.Version != "17" {
		t.Errorf("spec.version = %q", got.Spec.Version)
	}
}

func TestGraphQLAndMCPVersionUpgradeParity(t *testing.T) {
	ctx := context.Background()

	t.Run("graphql", func(t *testing.T) {
		svc, cl := newService(versionDatabase("gql-upgrade", "free", "16", false))
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `mutation { updateDatabaseVersion(id:"gql-upgrade", version:"17") { id version } }`})
		if len(res.Errors) > 0 {
			t.Fatalf("mutation errors: %v", res.Errors)
		}
		var got appv1alpha1.Database
		_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "gql-upgrade"}, &got)
		if got.Spec.Version != "17" {
			t.Errorf("spec.version = %q", got.Spec.Version)
		}
	})

	t.Run("mcp", func(t *testing.T) {
		svc, cl := newService(versionDatabase("mcp-upgrade", "free", "16", false))
		server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
		svc.RegisterMCP(server)
		serverTransport, clientTransport := mcp.NewInMemoryTransports()
		if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
			t.Fatal(err)
		}
		session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		// w1/m74 folded update_postgres_version into the resource's patch tool.
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "update_postgres", Arguments: map[string]any{"postgresId": "mcp-upgrade", "version": "17"}})
		if err != nil || result.IsError {
			t.Fatalf("tool: err=%v result=%+v", err, result)
		}
		var got appv1alpha1.Database
		_ = cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "mcp-upgrade"}, &got)
		if got.Spec.Version != "17" {
			t.Errorf("spec.version = %q", got.Spec.Version)
		}
	})
}
