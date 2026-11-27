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
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// upgradeRunDatabase is a database whose last upgrade, 16 → 18, bex-api
// accepted as run pgu-run, with the operator's report left to each case.
func upgradeRunDatabase(name string) *appv1alpha1.Database {
	db := versionDatabase(name, "basic-1gb", "16", true)
	db.Generation = 4
	db.Spec.Version = "18"
	db.Annotations = map[string]string{
		annotationUpgradeRun: `{"runId":"pgu-run","fromVersion":"16","toVersion":"18","startedAt":"2026-11-26T10:00:00Z"}`,
	}
	return db
}

func failedUpgradeCondition(generation int64) metav1.Condition {
	return metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: appv1alpha1.ReasonMajorVersionUpgradeFailed, ObservedGeneration: generation,
	}
}

// A run's state is read from the operator's report of the upgrade, never
// stored: each edge of the operator's upgrade lifecycle reads as Render's
// state for it.
func TestUpgradeRunStateFollowsTheOperator(t *testing.T) {
	tests := []struct {
		name   string
		report func(db *appv1alpha1.Database)
		want   string
	}{
		{name: "accepted, the operator not there yet", report: func(*appv1alpha1.Database) {}, want: UpgradeRunInProgress},
		{name: "CNPG running pg_upgrade", report: func(db *appv1alpha1.Database) {
			db.Status.Phase = appv1alpha1.DBPhaseUpgrading
		}, want: UpgradeRunInProgress},
		{name: "the new major healthy", report: func(db *appv1alpha1.Database) {
			db.Status.CurrentVersion = "18"
		}, want: UpgradeRunSucceeded},
		{name: "succeeded, then its post-upgrade backup failed", report: func(db *appv1alpha1.Database) {
			db.Status.CurrentVersion = "18"
			db.Status.Phase = appv1alpha1.DBPhaseFailed
		}, want: UpgradeRunSucceeded},
		{name: "pg_upgrade failed, spec not yet returned", report: func(db *appv1alpha1.Database) {
			db.Status.Phase = appv1alpha1.DBPhaseFailed
			db.Status.Conditions = []metav1.Condition{failedUpgradeCondition(db.Generation)}
		}, want: UpgradeRunFailed},
		{name: "pg_upgrade failed, spec returned to the source", report: func(db *appv1alpha1.Database) {
			db.Spec.Version = "16"
		}, want: UpgradeRunFailed},
		{name: "an earlier attempt's failure, this one just accepted", report: func(db *appv1alpha1.Database) {
			db.Status.Conditions = []metav1.Condition{failedUpgradeCondition(db.Generation - 1)}
		}, want: UpgradeRunInProgress},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := upgradeRunDatabase("state-db")
			tt.report(db)
			run := upgradeRunOf(db)
			if run == nil {
				t.Fatal("no run read from the record")
			}
			if run.State != tt.want {
				t.Errorf("state = %s, want %s", run.State, tt.want)
			}
			if run.RunID != "pgu-run" || run.ToVersion != "18" || run.FromVersion != "16" || run.StartedAt != "2026-11-26T10:00:00Z" {
				t.Errorf("run = %+v", run)
			}
		})
	}
}

func TestUpgradeRunOfIgnoresAMissingOrBrokenRecord(t *testing.T) {
	for name, raw := range map[string]string{"none": "", "not JSON": "{", "no id": `{"toVersion":"18"}`, "no target": `{"runId":"pgu-x"}`} {
		t.Run(name, func(t *testing.T) {
			db := versionDatabase("db", "free", "16", false)
			db.Annotations = map[string]string{annotationUpgradeRun: raw}
			if run := upgradeRunOf(db); run != nil {
				t.Errorf("run = %+v, want none", run)
			}
		})
	}
}

func storedDatabase(t *testing.T, cl client.Client, name string) *appv1alpha1.Database {
	t.Helper()
	var db appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &db); err != nil {
		t.Fatal(err)
	}
	return &db
}

func storedRun(t *testing.T, cl client.Client, name string) upgradeRunRecord {
	t.Helper()
	var record upgradeRunRecord
	raw := storedDatabase(t, cl, name).Annotations[annotationUpgradeRun]
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		t.Fatalf("upgrade run record %q: %v", raw, err)
	}
	return record
}

func graphQLSchema(t *testing.T, svc *Service) graphql.Schema {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

// Every surface that accepts an upgrade records its run, with the source the
// database was observed on and a fresh run id; GraphQL reads it back as
// Render's lastUpgradeRun, and REST keeps its public shape.
func TestAcceptedUpgradeRecordsItsRunOnEverySurface(t *testing.T) {
	ctx := context.Background()
	start := time.Now().UTC().Add(-time.Second)
	assertRecorded := func(t *testing.T, cl client.Client, name string) upgradeRunRecord {
		t.Helper()
		record := storedRun(t, cl, name)
		if !strings.HasPrefix(record.RunID, "pgu-") || record.FromVersion != "16" || record.ToVersion != "17" {
			t.Errorf("record = %+v", record)
		}
		at, err := time.Parse(time.RFC3339, record.StartedAt)
		if err != nil || at.Before(start.Truncate(time.Second)) {
			t.Errorf("startedAt = %q (%v), want the acceptance time", record.StartedAt, err)
		}
		return record
	}

	t.Run("graphql updateDatabaseVersion and upgradeDatabase", func(t *testing.T) {
		for _, mutation := range []string{
			`mutation { updateDatabaseVersion(id:"gql-run", version:"17") { id } }`,
			`mutation { upgradeDatabase(id:"gql-run", newVersion:"17") { id } }`,
		} {
			svc, cl := newService(versionDatabase("gql-run", "free", "16", false))
			schema := graphQLSchema(t, svc)
			if res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: mutation}); len(res.Errors) > 0 {
				t.Fatalf("%s: %v", mutation, res.Errors)
			}
			record := assertRecorded(t, cl, "gql-run")
			res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{ database(id:"gql-run") {
				versionUpgradeAvailable lastUpgradeRun { runId startedAt state toVersion fromVersion acknowledged } } }`})
			if len(res.Errors) > 0 {
				t.Fatal(res.Errors)
			}
			got := res.Data.(map[string]any)["database"].(map[string]any)
			if got["versionUpgradeAvailable"] != "18" {
				t.Errorf("versionUpgradeAvailable = %v, want 18", got["versionUpgradeAvailable"])
			}
			run := got["lastUpgradeRun"].(map[string]any)
			if run["runId"] != record.RunID || run["state"] != UpgradeRunInProgress || run["toVersion"] != "17" ||
				run["fromVersion"] != "16" || run["acknowledged"] != false || run["startedAt"] != record.StartedAt {
				t.Errorf("lastUpgradeRun = %v", run)
			}
		}
	})

	t.Run("rest PATCH", func(t *testing.T) {
		svc, cl := newService(versionDatabase("rest-run", "free", "16", false))
		if rec := serveREST(svc, "PATCH", "/v1/postgres/rest-run", `{"version":"17"}`); rec.Code != 200 {
			t.Fatalf("PATCH = %d %s", rec.Code, rec.Body.String())
		}
		assertRecorded(t, cl, "rest-run")
		rec := serveREST(svc, "GET", "/v1/postgres/rest-run", "")
		if strings.Contains(rec.Body.String(), "UpgradeRun") || strings.Contains(rec.Body.String(), "versionUpgradeAvailable") {
			t.Errorf("REST GET carries the attempt record: %s", rec.Body.String())
		}
	})

	t.Run("mcp update_postgres", func(t *testing.T) {
		svc, cl := newService(versionDatabase("mcp-run", "free", "16", false))
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
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "update_postgres", Arguments: map[string]any{"postgresId": "mcp-run", "version": "17"}})
		if err != nil || result.IsError {
			t.Fatalf("tool: err=%v result=%+v", err, result)
		}
		assertRecorded(t, cl, "mcp-run")
	})
}

// Only a write that moves spec.version starts a run: a refused upgrade, a dry
// run and an unrelated update leave the record as it was.
func TestOnlyAnAcceptedUpgradeStartsARun(t *testing.T) {
	ctx := context.Background()
	t.Run("refused", func(t *testing.T) {
		svc, cl := newService(versionDatabase("refused-run", "basic-1gb", "16", false))
		if _, err := svc.SetVersion(ctx, "refused-run", "17"); codedErrorCode(err) != "POSTGRES_UPGRADE_BACKUP_REQUIRED" {
			t.Fatalf("err = %v", err)
		}
		if raw := storedDatabase(t, cl, "refused-run").Annotations[annotationUpgradeRun]; raw != "" {
			t.Errorf("a refused upgrade recorded %s", raw)
		}
	})
	t.Run("dry run", func(t *testing.T) {
		svc, cl := newService(versionDatabase("dry-run", "free", "16", false))
		if rec := serveREST(svc, "PATCH", "/v1/postgres/dry-run", `{"version":"17","dryRun":true}`); rec.Code != 200 {
			t.Fatalf("PATCH = %d %s", rec.Code, rec.Body.String())
		}
		if raw := storedDatabase(t, cl, "dry-run").Annotations[annotationUpgradeRun]; raw != "" {
			t.Errorf("a dry run recorded %s", raw)
		}
	})
	t.Run("an unrelated update keeps the last run", func(t *testing.T) {
		db := upgradeRunDatabase("kept-run")
		db.Spec.Version, db.Status.CurrentVersion = "18", "18"
		svc, cl := newService(db)
		renamed := "renamed"
		if _, err := svc.UpdatePostgres(ctx, "kept-run", PostgresPatch{Name: &renamed}); err != nil {
			t.Fatal(err)
		}
		if got := storedRun(t, cl, "kept-run"); got.RunID != "pgu-run" {
			t.Errorf("record = %+v, want pgu-run kept", got)
		}
	})
}

// acknowledgeChecker denies one relation and allows the rest.
type acknowledgeChecker struct{ deny string }

func (c acknowledgeChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return relation != c.deny, nil
}

func TestAcknowledgeUpgrade(t *testing.T) {
	ctx := context.Background()
	failed := func() *appv1alpha1.Database {
		db := upgradeRunDatabase("ack-db")
		db.Spec.Version = "16"
		return db
	}

	t.Run("a finished run, through GraphQL, once and again", func(t *testing.T) {
		svc, cl := newService(failed())
		schema := graphQLSchema(t, svc)
		for range 2 {
			res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `mutation {
				acknowledgeDatabaseUpgrade(id:"ack-db", runId:"pgu-run") { id lastUpgradeRun { state acknowledged } } }`})
			if len(res.Errors) > 0 {
				t.Fatal(res.Errors)
			}
			run := res.Data.(map[string]any)["acknowledgeDatabaseUpgrade"].(map[string]any)["lastUpgradeRun"].(map[string]any)
			if run["state"] != UpgradeRunFailed || run["acknowledged"] != true {
				t.Errorf("lastUpgradeRun = %v", run)
			}
		}
		if got := storedDatabase(t, cl, "ack-db").Annotations[annotationUpgradeRunAcknowledged]; got != "pgu-run" {
			t.Errorf("acknowledged = %q", got)
		}
	})

	t.Run("a running upgrade", func(t *testing.T) {
		svc, cl := newService(upgradeRunDatabase("ack-db"))
		_, err := svc.AcknowledgeUpgrade(ctx, "ack-db", "pgu-run")
		if codedErrorCode(err) != PostgresUpgradeRunInProgressCode {
			t.Fatalf("err = %v", err)
		}
		if got := storedDatabase(t, cl, "ack-db").Annotations[annotationUpgradeRunAcknowledged]; got != "" {
			t.Errorf("acknowledged = %q", got)
		}
	})

	t.Run("another run's id", func(t *testing.T) {
		svc, _ := newService(failed())
		if _, err := svc.AcknowledgeUpgrade(ctx, "ack-db", "pgu-older"); codedErrorCode(err) != PostgresUpgradeRunNotFoundCode {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("no run at all", func(t *testing.T) {
		svc, _ := newService(versionDatabase("ack-db", "free", "16", false))
		if _, err := svc.AcknowledgeUpgrade(ctx, "ack-db", "pgu-run"); codedErrorCode(err) != PostgresUpgradeRunNotFoundCode {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("a member who may not upgrade", func(t *testing.T) {
		svc, cl := newService(failed())
		svc.Authz = acknowledgeChecker{deny: core.RelCanCreate}
		_, err := svc.AcknowledgeUpgrade(ctxAs("viewer"), "ack-db", "pgu-run")
		if !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("err = %v, want forbidden", err)
		}
		if got := storedDatabase(t, cl, "ack-db").Annotations[annotationUpgradeRunAcknowledged]; got != "" {
			t.Errorf("acknowledged = %q", got)
		}
	})

	t.Run("a new run is not acknowledged by the last one's acknowledgement", func(t *testing.T) {
		db := failed()
		db.Annotations[annotationUpgradeRunAcknowledged] = "pgu-run"
		before := db.DeepCopy()
		db.Spec.Version = "18"
		stampUpgradeRun(before, db, time.Now())
		if run := upgradeRunOf(db); run == nil || run.RunID == "pgu-run" || run.Acknowledged || run.State != UpgradeRunInProgress {
			t.Errorf("run = %+v, want a new unacknowledged run in progress", run)
		}
	})
}

// extensionsService serves a database whose application database has the
// given extensions installed (nil: unreadable).
func extensionsService(t *testing.T, db *appv1alpha1.Database, installed []string) (*Service, client.Client) {
	t.Helper()
	db.Status.SecretName = db.Name + "-app"
	svc, cl := newService(db, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: db.Name + "-app", Namespace: "default"},
		Data:       map[string][]byte{"uri": []byte("postgresql://u:p@" + db.Name + "-rw:5432/app")},
	})
	svc.queryExecutor = func(_ context.Context, _, sql string, _ queryLimits, readOnly bool) (QueryResult, error) {
		if sql != installedExtensionsSQL || !readOnly {
			t.Errorf("query %q readOnly=%v, want the read-only extension list", sql, readOnly)
		}
		if installed == nil {
			return QueryResult{}, errQueryConnClose
		}
		rows := make([][]any, len(installed))
		for i, name := range installed {
			rows[i] = []any{name}
		}
		return QueryResult{Columns: []string{"extname"}, Rows: rows}, nil
	}
	return svc, cl
}

func TestDatabaseExtensionsNamesTheOnesThatBlockTheLatestMajor(t *testing.T) {
	svc, _ := extensionsService(t, versionDatabase("ext-db", "free", "15", false),
		[]string{"adminpack", "pg_similarity", "pg_stat_statements", "plpgsql", "vector"})
	res := graphql.Do(graphql.Params{Schema: graphQLSchema(t, svc), Context: context.Background(),
		RequestString: `{ databaseExtensions(id:"ext-db") { name blocksUpgradeDatabase } }`})
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	blocked := map[string]bool{}
	for _, e := range res.Data.(map[string]any)["databaseExtensions"].([]any) {
		ext := e.(map[string]any)
		blocked[ext["name"].(string)] = ext["blocksUpgradeDatabase"].(bool)
	}
	want := map[string]bool{"adminpack": true, "pg_similarity": true, "pg_stat_statements": false, "plpgsql": false, "vector": false}
	if len(blocked) != len(want) {
		t.Fatalf("extensions = %v, want %v", blocked, want)
	}
	for name, blocks := range want {
		if blocked[name] != blocks {
			t.Errorf("%s blocks = %v, want %v", name, blocked[name], blocks)
		}
	}
}

func TestDatabaseExtensionsUnreadableIsUnavailable(t *testing.T) {
	svc, _ := extensionsService(t, versionDatabase("ext-db", "free", "16", false), nil)
	_, err := svc.DatabaseExtensions(context.Background(), "ext-db")
	if codedErrorCode(err) != PostgresExtensionsUnreadableCode || !errors.Is(err, core.ErrUnavailable) {
		t.Fatalf("err = %v, want %s (unavailable)", err, PostgresExtensionsUnreadableCode)
	}
	if strings.Contains(err.Error(), "ext-db-rw") {
		t.Errorf("the answer names the internal host: %v", err)
	}
}

// pg_upgrade refuses a cluster with an extension the target lacks, after the
// downtime began; bex refuses it first, on every surface, naming it.
func TestUpgradeRefusedWhileABlockingExtensionIsInstalled(t *testing.T) {
	ctx := context.Background()
	t.Run("old_snapshot to 17", func(t *testing.T) {
		svc, cl := extensionsService(t, versionDatabase("ext-db", "free", "16", false), []string{"old_snapshot", "plpgsql"})
		_, err := svc.SetVersion(ctx, "ext-db", "17")
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != PostgresUpgradeExtensionsBlockedCode {
			t.Fatalf("err = %v", err)
		}
		if !strings.Contains(coded.Error(), "old_snapshot") || !slices.Equal(coded.Params["extensions"].([]string), []string{"old_snapshot"}) {
			t.Errorf("refusal = %q %v", coded.Error(), coded.Params)
		}
		if got := storedDatabase(t, cl, "ext-db"); got.Spec.Version != "16" || got.Annotations[annotationUpgradeRun] != "" {
			t.Errorf("spec.version = %q, record %q", got.Spec.Version, got.Annotations[annotationUpgradeRun])
		}
		rec := serveREST(svc, "PATCH", "/v1/postgres/ext-db", `{"version":"18"}`)
		if rec.Code != 409 || !strings.Contains(rec.Body.String(), PostgresUpgradeExtensionsBlockedCode) {
			t.Errorf("REST PATCH = %d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("adminpack to 16, which still carries it", func(t *testing.T) {
		svc, cl := extensionsService(t, versionDatabase("ext-db", "free", "15", false), []string{"adminpack"})
		if _, err := svc.SetVersion(ctx, "ext-db", "16"); err != nil {
			t.Fatal(err)
		}
		if got := storedDatabase(t, cl, "ext-db").Spec.Version; got != "16" {
			t.Errorf("spec.version = %q", got)
		}
	})
	t.Run("extensions unreadable", func(t *testing.T) {
		svc, cl := extensionsService(t, versionDatabase("ext-db", "free", "16", false), nil)
		if _, err := svc.SetVersion(ctx, "ext-db", "18"); err != nil {
			t.Fatal(err)
		}
		if got := storedDatabase(t, cl, "ext-db").Spec.Version; got != "18" {
			t.Errorf("spec.version = %q", got)
		}
	})
}

// The blockers are exactly the extensions some bex operand carries that the
// target major's does not, for every major an upgrade may target.
func TestUpgradeBlockingExtensionsMatchTheOperands(t *testing.T) {
	raw, err := os.ReadFile("../../../../deploy/pg-operand/matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	var matrix map[string]struct {
		Extensions []string `json:"extensions"`
	}
	if err := json.Unmarshal(raw, &matrix); err != nil {
		t.Fatal(err)
	}
	carried := map[string]bool{}
	for _, major := range matrix {
		for _, e := range major.Extensions {
			carried[e] = true
		}
	}
	for _, version := range supportedPostgresVersions {
		target, _ := strconv.Atoi(version)
		if target < minPostgresUpgradeTarget {
			continue
		}
		var want []string
		for e := range carried {
			if !slices.Contains(matrix[version].Extensions, e) {
				want = append(want, e)
			}
		}
		slices.Sort(want)
		if got := upgradeBlockingExtensions(target); !slices.Equal(got, want) {
			t.Errorf("blocking extensions for %d = %v, want %v (matrix.json)", target, got, want)
		}
	}
}
