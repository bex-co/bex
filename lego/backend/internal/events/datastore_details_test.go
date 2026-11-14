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

package events

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// TestDatastoreEventDetailsOnRetrieve is w4/m379: GET /v1/events/{eventId}
// answers every Postgres and Key Value event bex emits with the details
// Render's member for its type names, filled from what the row records. A
// caller-less row names no user, and a row recorded before a from-value was
// carries the to-side alone rather than an invented one.
func TestDatastoreEventDetailsOnRetrieve(t *testing.T) {
	ten, fifteen, zero, two := int32(10), int32(15), int32(0), int32(2)
	on, off := true, false
	persistence, policy := "off", "allkeys_lru"
	const user = `"user":{"id":"user-x","email":"x@example.com"}`
	audit := func(verb, caller string, row store.ServiceEventRow) store.ServiceEventRow {
		row.Source, row.Verb, row.Caller = store.EventSourceAudit, verb, caller
		return row
	}
	fact := func(typ string, row store.ServiceEventRow) store.ServiceEventRow {
		row.Source, row.FactType = store.EventSourceFact, typ
		return row
	}
	versions := store.ServiceEventRow{VersionFrom: "16", VersionTo: "17"}
	for _, tc := range []struct {
		name string
		row  store.ServiceEventRow
		want string
	}{
		{"created", audit(core.AuditVerbPostgresCreated, "user-x", store.ServiceEventRow{}), `{` + user + `}`},
		{"created by no caller", audit(core.AuditVerbPostgresCreated, "", store.ServiceEventRow{}), `{}`},
		{"restarted", audit(core.AuditVerbPostgresRestarted, "user-x", store.ServiceEventRow{}), `{` + user + `}`},
		{"pool enabled", audit(core.AuditVerbPostgresPoolerChanged, "user-x", store.ServiceEventRow{ConnectionPoolEnabled: &on}), `{` + user + `,"enabled":true}`},
		{"pool disabled by no caller", audit(core.AuditVerbPostgresPoolerChanged, "", store.ServiceEventRow{ConnectionPoolEnabled: &off}), `{"enabled":false}`},
		{"disk size", audit(core.AuditVerbPostgresDiskSizeChanged, "user-x", store.ServiceEventRow{DiskSizeGB: &fifteen, AuditChanges: store.AuditChanges{DiskSizeFromGB: &ten}}),
			`{` + user + `,"fromDiskSize":10,"toDiskSize":15}`},
		{"disk size recorded before its from-value", audit(core.AuditVerbPostgresDiskSizeChanged, "user-x", store.ServiceEventRow{DiskSizeGB: &fifteen}),
			`{` + user + `,"toDiskSize":15}`},
		{"ha on", audit(core.AuditVerbPostgresHAChanged, "user-x", store.ServiceEventRow{HighAvailabilityEnabled: &on}), `{"fromStatus":"disabled","toStatus":"enabled"}`},
		{"ha off", audit(core.AuditVerbPostgresHAChanged, "user-x", store.ServiceEventRow{HighAvailabilityEnabled: &off}), `{"fromStatus":"enabled","toStatus":"disabled"}`},
		{"read replicas", audit(core.AuditVerbPostgresReadReplicasChanged, "user-x", store.ServiceEventRow{AuditChanges: store.AuditChanges{ReadReplicasFrom: &zero, ReadReplicasTo: &two}}),
			`{"fromReplicas":0,"toReplicas":2}`},
		{"read replicas recorded before their counts", audit(core.AuditVerbPostgresReadReplicasChanged, "user-x", store.ServiceEventRow{}), `{}`},
		{"disk autoscaling", audit(core.AuditVerbPostgresDiskAutoscalingChanged, "user-x", store.ServiceEventRow{}), `{}`},
		{"upgrade started", fact(TypePostgresUpgradeStarted, versions), `{"fromVersion":"16","toVersion":"17"}`},
		// w4/654: the caller of the version write the store joins.
		{"upgrade started by its caller", fact(TypePostgresUpgradeStarted, store.ServiceEventRow{VersionFrom: "16", VersionTo: "17", Caller: "user-x"}),
			`{` + user + `,"fromVersion":"16","toVersion":"17"}`},
		{"upgrade succeeded names no user", fact(TypePostgresUpgradeSucceeded, store.ServiceEventRow{VersionFrom: "16", VersionTo: "17", Caller: "user-x"}),
			`{"fromVersion":"16","toVersion":"17"}`},
		{"upgrade succeeded", fact(TypePostgresUpgradeSucceeded, versions), `{"fromVersion":"16","toVersion":"17"}`},
		{"upgrade failed", fact(TypePostgresUpgradeFailed, versions), `{"fromVersion":"16","toVersion":"17"}`},
		{"key value config restart", audit(core.AuditVerbKeyValueConfigChanged, "user-x", store.ServiceEventRow{MaxmemoryPolicy: &policy, PersistenceMode: &persistence}),
			`{"message":"Persistence mode changed to off; the instance restarted to apply it.","reason":"config_change"}`},
		{"key value config restart with no recorded mode", audit(core.AuditVerbKeyValueConfigChanged, "user-x", store.ServiceEventRow{MaxmemoryPolicy: &policy}),
			`{"message":"The configuration changed; the instance restarted to apply it.","reason":"config_change"}`},
		// Unchanged: Render's empty variants, the recorded reasonCode extra
		// and the leader.
		{"available", fact(TypePostgresAvailable, store.ServiceEventRow{}), `{}`},
		{"unavailable", fact(TypePostgresUnavailable, store.ServiceEventRow{ReasonCode: store.EventReasonReadinessFailed}), `{"reasonCode":"readiness_failed"}`},
		{"leader changed", fact(TypePostgresClusterLeaderChanged, store.ServiceEventRow{LeaderID: "dpg-1-2"}), `{"leaderId":"dpg-1-2"}`},
		{"backup started", audit(core.AuditVerbPostgresBackupStarted, "user-x", store.ServiceEventRow{}), `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := tc.row
			row.Key, row.At = "datastore:"+tc.name, now
			eventID := ids.Derive(ids.Event, row.Key)
			svc := newService(&hydrationStore{fakeStore: &fakeStore{lookup: store.ServiceEventLookup{Event: row, ServiceID: ids.New(ids.Postgres)}}})
			svc.Workspace = hydrationWorkspaces{}
			svc.Identities = emailLookup{"user-x": "x@example.com"}
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			ctx := core.WithIdentity(t.Context(), core.Identity{Subject: "user", Method: "session"})
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/events/"+eventID, nil).WithContext(ctx))
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			if body := rec.Body.String(); !strings.HasSuffix(strings.TrimSpace(body), `"details":`+tc.want+`}`) {
				t.Errorf("event = %s, want details %s", body, tc.want)
			}

			// w4/655: GraphQL serviceEvent.details carries every member REST
			// sent, with the same value (reason is restartReason there).
			var rest map[string]any
			if err := json.Unmarshal([]byte(tc.want), &rest); err != nil {
				t.Fatal(err)
			}
			schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()})})
			if err != nil {
				t.Fatal(err)
			}
			result := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: fmt.Sprintf(
				`{serviceEvent(id:%q){details{user{id email} enabled fromDiskSize toDiskSize fromStatus toStatus fromReplicas toReplicas fromVersion toVersion restartReason message leaderId reasonCode}}}`, eventID)})
			if len(result.Errors) > 0 {
				t.Fatal(result.Errors)
			}
			gql := result.Data.(map[string]any)["serviceEvent"].(map[string]any)["details"].(map[string]any)
			for key, want := range rest {
				gqlKey := key
				if key == "reason" {
					gqlKey = "restartReason"
				}
				if got := gql[gqlKey]; fmt.Sprint(got) != fmt.Sprint(want) {
					t.Errorf("graphql details.%s = %v, want REST's %v", gqlKey, got, want)
				}
			}
		})
	}
}
