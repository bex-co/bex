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
	"net/http/httptest"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const lrWorkspace = "tea-logical"

// publisherDatabase is a Ready paid 10 GB Postgres with a dedicated reader,
// the shape Render's prerequisites admit.
func publisherDatabase(name string) *appv1alpha1.Database {
	return &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: map[string]string{core.LabelTenant: lrWorkspace}},
		Spec: appv1alpha1.DatabaseSpec{
			Name: "orders", Plan: "basic-1gb", StorageGB: 10,
			Users:        []appv1alpha1.DatabaseUser{{Name: "cdc_reader", SecretName: name + "-cdc-reader"}, {Name: "app_rotated", OwnerPrivileges: true}},
			DeletedUsers: []string{"old_reader"},
		},
		Status: appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady, SecretName: name + "-app", ExternalHost: name + ".db.example.test"},
	}
}

func newLogicalReplicationService(t *testing.T, plan string, objs ...client.Object) (*Service, *webhookAuditSink) {
	t.Helper()
	svc, _ := newService(objs...)
	sink := &webhookAuditSink{}
	svc.Audit = sink
	if plan != "" {
		svc.Plans = core.WorkspacePlanFunc(func(_ context.Context, id string) (string, error) {
			if id != lrWorkspace {
				return "", errors.New("unexpected workspace " + id)
			}
			return plan, nil
		})
	}
	return svc, sink
}

func codeOf(err error) string {
	var coded *core.CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

// Render's prerequisites, and the request's own validity, refuse by name
// before anything is written: no spec change, no audit row (DoD 2).
func TestLogicalReplicationRefusesBeforeWriting(t *testing.T) {
	enable := LogicalReplicationRequest{Database: "dpg-lr", Enable: true, PublisherRole: "cdc_reader", Schemas: []string{"public"}, AllTables: true}
	now := metav1.Now()
	for _, tc := range []struct {
		name   string
		plan   string // "" leaves the workspace plan unreadable
		mutate func(*appv1alpha1.Database)
		req    func(LogicalReplicationRequest) LogicalReplicationRequest
		code   string
		status int
	}{
		{name: "a Hobby workspace", plan: "hobby", code: LogicalReplicationPlanRequiredCode, status: 400},
		{name: "an unreadable workspace plan", plan: "", code: LogicalReplicationPlanRequiredCode, status: 400},
		{name: "a free database", plan: "pro", mutate: func(d *appv1alpha1.Database) { d.Spec.Plan, d.Spec.StorageGB = "free", 0 }, code: LogicalReplicationFreePlanCode, status: 400},
		{name: "5 GB of storage", plan: "pro", mutate: func(d *appv1alpha1.Database) { d.Spec.StorageGB = 5 }, code: LogicalReplicationStorageTooSmallCode, status: 400},
		{name: "a role the database lacks", plan: "scale", req: func(r LogicalReplicationRequest) LogicalReplicationRequest { r.PublisherRole = "nobody"; return r }, code: LogicalReplicationRoleUnknownCode, status: 400},
		{name: "a retired login", plan: "pro", req: func(r LogicalReplicationRequest) LogicalReplicationRequest { r.PublisherRole = "old_reader"; return r }, code: LogicalReplicationRoleUnknownCode, status: 400},
		{name: "a reserved role", plan: "pro", req: func(r LogicalReplicationRequest) LogicalReplicationRequest { r.PublisherRole = "postgres"; return r }, code: LogicalReplicationRoleUnknownCode, status: 400},
		{name: "no schema", plan: "pro", req: func(r LogicalReplicationRequest) LogicalReplicationRequest { r.Schemas = nil; return r }, code: logicalReplicationInvalidRequestCode, status: 400},
		{name: "a suspended database", plan: "pro", mutate: func(d *appv1alpha1.Database) { d.Spec.Suspended = true }, code: LogicalReplicationUnavailableCode, status: 503},
		{name: "a database being created", plan: "pro", mutate: func(d *appv1alpha1.Database) { d.Status.Phase = appv1alpha1.DBPhaseProvisioning }, code: LogicalReplicationUnavailableCode, status: 503},
		{name: "a database being deleted", plan: "pro", mutate: func(d *appv1alpha1.Database) {
			d.DeletionTimestamp, d.Finalizers = &now, []string{"app.bex.co/test"}
		}, code: LogicalReplicationUnavailableCode, status: 503},
		{name: "disabling a suspended database", plan: "pro", mutate: func(d *appv1alpha1.Database) {
			d.Spec.Suspended = true
			d.Spec.LogicalReplication = &appv1alpha1.DatabaseLogicalReplication{PublisherRole: "cdc_reader", Schemas: []string{"public"}}
		}, req: func(r LogicalReplicationRequest) LogicalReplicationRequest {
			return LogicalReplicationRequest{Database: r.Database}
		}, code: LogicalReplicationUnavailableCode, status: 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := publisherDatabase("dpg-lr")
			if tc.mutate != nil {
				tc.mutate(d)
			}
			before := d.Spec.LogicalReplication
			svc, sink := newLogicalReplicationService(t, tc.plan, d)
			req := enable
			if tc.req != nil {
				req = tc.req(req)
			}
			rec := httptest.NewRecorder()
			body := `{"database":"` + req.Database + `","enable":` + map[bool]string{true: "true", false: "false"}[req.Enable] +
				`,"publisherRole":"` + req.PublisherRole + `","schemas":[` + quoteJoin(req.Schemas) + `],"allTables":true}`
			r := httptest.NewRequest(http.MethodPost, LogicalReplicationPath, strings.NewReader(body))
			r.Header.Set("Authorization", "Bearer cp-token")
			LogicalReplicationHandler(svc, "cp-token").ServeHTTP(rec, r)
			if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.code) {
				t.Fatalf("answer = %d %s, want %d naming %s", rec.Code, rec.Body, tc.status, tc.code)
			}
			if _, err := svc.SetLogicalReplication(context.Background(), req); codeOf(err) != tc.code {
				t.Fatalf("SetLogicalReplication = %v, want %s", err, tc.code)
			}
			var got appv1alpha1.Database
			if err := svc.Client.Get(context.Background(), client.ObjectKeyFromObject(d), &got); err != nil {
				t.Fatal(err)
			}
			if (got.Spec.LogicalReplication == nil) != (before == nil) {
				t.Errorf("spec.logicalReplication = %+v after a refusal, want it unchanged", got.Spec.LogicalReplication)
			}
			if len(sink.events) != 0 {
				t.Errorf("a refusal recorded %d audit rows", len(sink.events))
			}
		})
	}
}

func quoteJoin(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = `"` + s + `"`
	}
	return strings.Join(quoted, ",")
}

// The verb's whole authentication is the control-plane bearer, checked
// before anything else: no token, a wrong one, or an unconfigured one never
// reaches the database lookup.
func TestLogicalReplicationHandlerBearer(t *testing.T) {
	svc, _ := newLogicalReplicationService(t, "pro")
	for _, tc := range []struct {
		name, configured, header string
		want                     int
	}{
		{"no token", "cp-token", "", http.StatusUnauthorized},
		{"a wrong token", "cp-token", "Bearer other", http.StatusUnauthorized},
		{"no token configured", "", "Bearer ", http.StatusUnauthorized},
		{"the token, an unknown database", "cp-token", "Bearer cp-token", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, LogicalReplicationPath, strings.NewReader(`{"database":"dpg-missing","enable":false}`))
			if tc.header != "" {
				r.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			LogicalReplicationHandler(svc, tc.configured).ServeHTTP(rec, r)
			if rec.Code != tc.want {
				t.Errorf("status = %d %s, want %d", rec.Code, rec.Body, tc.want)
			}
		})
	}
}

// The verb has no REST route: Render has no API field, so the tenant mux
// never serves it.
func TestLogicalReplicationIsNoRESTRoute(t *testing.T) {
	d := publisherDatabase("dpg-lr")
	svc, _ := newLogicalReplicationService(t, "pro", d)
	rec := serveREST(svc, http.MethodPost, LogicalReplicationPath, `{"database":"dpg-lr","enable":true,"publisherRole":"cdc_reader","schemas":["public"]}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("the tenant mux answered %d for the verb's path, want 404", rec.Code)
	}
}

func TestLiveCredential(t *testing.T) {
	d := publisherDatabase("dpg-lr")
	for role, want := range map[string]bool{
		d.Spec.EffectiveDatabaseUser(d.Name): true,
		"cdc_reader":                         true,
		"app_rotated":                        true,
		"old_reader":                         false,
		"nobody":                             false,
		"pg_monitor":                         false,
		"Cdc_Reader":                         false,
		"":                                   false,
	} {
		if got := liveCredential(d, role); got != want {
			t.Errorf("liveCredential(%q) = %v, want %v", role, got, want)
		}
	}
	// A retired owner is no publisher either.
	d.Spec.DeletedUsers = append(d.Spec.DeletedUsers, d.Spec.EffectiveDatabaseUser(d.Name))
	if liveCredential(d, d.Spec.EffectiveDatabaseUser(d.Name)) {
		t.Error("a retired owner was accepted as a live credential")
	}
}

func TestParseRequestSchemasAreNormalized(t *testing.T) {
	d := publisherDatabase("dpg-lr")
	svc, _ := newLogicalReplicationService(t, "pro", d)
	change, err := svc.checkLogicalReplicationRequest(context.Background(), d,
		LogicalReplicationRequest{Database: d.Name, Enable: true, PublisherRole: "cdc_reader", Schemas: []string{"sales", "public", "sales"}})
	if err != nil {
		t.Fatal(err)
	}
	if lr := change.next; strings.Join(lr.Schemas, ",") != "public,sales" || lr.AllTables {
		t.Errorf("spec = %+v, want the sorted unique schemas and no all-tables publication", lr)
	}
	if _, err := svc.checkLogicalReplicationRequest(context.Background(), d,
		LogicalReplicationRequest{Database: d.Name, Enable: true, PublisherRole: "cdc_reader", Schemas: []string{strings.Repeat("s", 64)}}); codeOf(err) != logicalReplicationInvalidRequestCode {
		t.Errorf("a 64-byte schema name = %v, want %s", err, logicalReplicationInvalidRequestCode)
	}
}
