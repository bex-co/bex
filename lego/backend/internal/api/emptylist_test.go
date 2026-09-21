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

// emptylist_test.go — w4/m116/t006. Render's public API declares every list
// response as `{"type":"array"}` with no `nullable`
// (internal/api/openapi/render-public-api-1.json: list-services,
// list-projects, list-postgres, listWorkflows, …), so an empty list is `[]`.
// Go's encoding/json prints a NIL slice as `null`, and a list handler reaches a
// nil slice by accident — `var out []T`, a store read with no rows,
// core.Page over a nil input — so the wart is one careless list route away at
// all times.
//
// TestEmptyListRoutesSerializeAsJSONArray is the standing gate: every GET route
// on the single REST mux must be classified here, and every classified ARRAY
// route whose empty case this DB-less harness can produce is asserted to answer
// `[]`. A new GET route fails the completeness check until it is classified, so
// the table cannot silently fall behind the router.

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// emptyListCase is one array-valued GET whose empty answer this harness can
// produce. field is "" when the whole body is the array, or the response
// object's key when the array is nested (bex-native envelope routes).
type emptyListCase struct {
	route   string // the mux pattern — the key the completeness check matches
	path    string // the concrete request path
	field   string // "" => the body itself is the array
	withApp bool   // serve from the fixture that has an App/Database/KeyValue
}

// emptyListCases: array routes proven empty end-to-end, through the real auth
// gate and the real REST mux.
var emptyListCases = []emptyListCase{
	// --- top-level arrays (Render's list shape) ---
	{route: "GET /v1/services", path: "/v1/services"},
	{route: "GET /v1/projects", path: "/v1/projects?ownerId=tea-empty"},
	{route: "GET /v1/postgres", path: "/v1/postgres"},
	{route: "GET /v1/key-value", path: "/v1/key-value"},
	{route: "GET /v1/env-groups", path: "/v1/env-groups"},
	{route: "GET /v1/api-keys", path: "/v1/api-keys"},
	{route: "GET /v1/registrycredentials", path: "/v1/registrycredentials"},
	{route: "GET /v1/owners", path: "/v1/owners"},
	{route: "GET /v1/owners/{ownerId}/audit-logs", path: "/v1/owners/tea-empty/audit-logs"},
	{route: "GET /v1/notification-settings/overrides", path: "/v1/notification-settings/overrides"},
	{route: "GET /v1/workflows", path: "/v1/workflows"},
	{route: "GET /v1/services/{id}/deploys", path: "/v1/services/web/deploys", withApp: true},
	{route: "GET /v1/services/{id}/events", path: "/v1/services/web/events", withApp: true},
	{route: "GET /v1/services/{id}/custom-domains", path: "/v1/services/web/custom-domains", withApp: true},
	{route: "GET /v1/services/{id}/env-vars", path: "/v1/services/web/env-vars", withApp: true},
	{route: "GET /v1/services/{id}/secret-files", path: "/v1/services/web/secret-files", withApp: true},
	{route: "GET /v1/services/{id}/instances", path: "/v1/services/web/instances", withApp: true},
	{route: "GET /v1/postgres/{id}/export", path: "/v1/postgres/pg1/export", withApp: true},
	{route: "GET /v1/postgres/{id}/users", path: "/v1/postgres/pg1/users", withApp: true},
	{route: "GET /v1/postgres/{id}/parameters", path: "/v1/postgres/pg1/parameters", withApp: true},
	{route: "GET /v1/postgres/{id}/top-queries", path: "/v1/postgres/pg1/top-queries", withApp: true},
	// Every /v1/metrics/… series route shares metrics.toRenderMetrics; each one
	// is probed with a source that answers no series at all.
	{route: "GET /v1/metrics/cpu", path: "/v1/metrics/cpu?resource=web", withApp: true},
	{route: "GET /v1/metrics/cpu-limit", path: "/v1/metrics/cpu-limit?resource=web", withApp: true},
	{route: "GET /v1/metrics/cpu-target", path: "/v1/metrics/cpu-target?resource=web", withApp: true},
	{route: "GET /v1/metrics/memory", path: "/v1/metrics/memory?resource=web", withApp: true},
	{route: "GET /v1/metrics/memory-limit", path: "/v1/metrics/memory-limit?resource=web", withApp: true},
	{route: "GET /v1/metrics/memory-target", path: "/v1/metrics/memory-target?resource=web", withApp: true},
	{route: "GET /v1/metrics/instance-count", path: "/v1/metrics/instance-count?resource=web", withApp: true},
	{route: "GET /v1/metrics/http-requests", path: "/v1/metrics/http-requests?resource=web", withApp: true},
	{route: "GET /v1/metrics/http-latency", path: "/v1/metrics/http-latency?resource=web", withApp: true},
	{route: "GET /v1/metrics/bandwidth", path: "/v1/metrics/bandwidth?resource=web", withApp: true},
	{route: "GET /v1/metrics/active-connections", path: "/v1/metrics/active-connections?resource=pg1", withApp: true},
	{route: "GET /v1/metrics/db-connections", path: "/v1/metrics/db-connections?resource=pg1", withApp: true},
	{route: "GET /v1/metrics/replication-lag", path: "/v1/metrics/replication-lag?resource=pg1", withApp: true},
	{route: "GET /v1/metrics/disk-usage", path: "/v1/metrics/disk-usage?resource=pg1", withApp: true},
	{route: "GET /v1/metrics/disk", path: "/v1/metrics/disk?resource=pg1", withApp: true},
	{route: "GET /v1/metrics/filters/path", path: "/v1/metrics/filters/path?resource=web", withApp: true},

	// --- arrays NESTED in a response object. These are past core.WriteJSON's
	// nil-slice guard, so each one is a construction-site guarantee and the
	// only reason this table asserts on fields at all. ---
	{route: "GET /v1/services/{id}/outbound-ips", path: "/v1/services/web/outbound-ips", field: "ips", withApp: true},
	{route: "GET /v1/postgres/{id}/ip-allow-list", path: "/v1/postgres/pg1/ip-allow-list", field: "cidrs", withApp: true},
	{route: "GET /v1/key-value/{id}/ip-allow-list", path: "/v1/key-value/kv1/ip-allow-list", field: "cidrs", withApp: true},
	{route: "GET /v1/agent-sessions/capabilities", path: "/v1/agent-sessions/capabilities", field: "agents"},
}

// neverEmptyArrayRoutes are array routes whose contents are a build-time
// constant, so there is no empty case to assert — they cannot regress to null.
var neverEmptyArrayRoutes = map[string]string{
	"GET /v1/metrics/disk-capacity":                   "the provisioned capacity of a disk — one constant series, never an empty list",
	"GET /v1/webhooks/event-types":                    "webhooks.EventTypes, a sorted package var over a non-empty set (webhooks/service.go)",
	"GET /v1/metrics/filters/http":                    "one descriptor per supported HTTP filter; inner values[] via filterValuesOrEmpty (metrics/service.go)",
	"GET /v1/metrics/filters/application":             "one descriptor per supported application filter, same helper",
	"GET /v1/viewer/capabilities":                     "grants is append onto make([]CapabilityGrant, 0, 8) and is never empty (members/service.go)",
	"GET /v1/notification-settings/push/availability": "booleans only; no array",
}

// unreachableArrayRoutes are array routes this DB-less harness cannot drive to
// a 200 (they need the control-plane Postgres, Loki, GitHub, or a browser
// session). Each entry cites the construction that makes its empty answer
// non-nil, so the claim is checkable by reading rather than by running.
var unreachableArrayRoutes = map[string]string{
	"GET /v1/blueprints":                         "BEX_CP_DB_URI; make([]BlueprintView, len(bs)) (apps/blueprint.go)",
	"GET /v1/blueprints/{id}/syncs":              "BEX_CP_DB_URI; make([]BlueprintSyncView, len(runs)) (apps/blueprint.go)",
	"GET /v1/disks":                              "BEX_CP_DB_URI; toDiskList make (apps/disks.go)",
	"GET /v1/disks/{diskId}/snapshots":           "BEX_CP_DB_URI; make([]DiskSnapshotView, 0, len(objects)) (apps/disk_snapshots.go)",
	"GET /v1/services/{id}/jobs":                 "BEX_CP_DB_URI; toJobList make (jobs/rest.go)",
	"GET /v1/services/{id}/runs":                 "needs a cron App; toCronJobRunList make (apps/render.go)",
	"GET /v1/cron-jobs/{id}/runs":                "needs a cron App; same toCronJobRunList",
	"GET /v1/services/{id}/routes":               "needs a static_site App; toRenderRoutes maps nil to [] (apps/render.go)",
	"GET /v1/services/{id}/headers":              "needs a static_site App; toRenderHeaders maps nil to [] (apps/render.go)",
	"GET /v1/environments":                       "needs the environments store; explicit `if out == nil { out = []environmentWithCursor{} }` (environments/rest.go)",
	"GET /v1/webhooks":                           "needs the webhook store; toWirePage make (webhooks/rest.go)",
	"GET /v1/webhooks/{id}/events":               "needs the webhook store; toWebhookEventList make (webhooks/rest.go)",
	"GET /v1/ssh-keys":                           "needs the ssh-key store; PGStore.ListSSHKeys make([]SSHKey, 0) (store/sshkeys.go)",
	"GET /v1/notifications":                      "needs the notification store; make([]PushNotificationView, 0, len(rows)) (notifications/inbox.go)",
	"GET /v1/notification-device-subscriptions":  "needs the notification store; make (notifications/subscriptions.go)",
	"GET /v1/notification-webpush-subscriptions": "needs the notification store; make (notifications/subscriptions.go)",
	"GET /v1/notification-settings/push":         "needs the notification store; normalizePushSettings fills events/workingHours/quietHours/serviceOverrides (notifications/service.go)",
	"GET /v1/repos":                              "needs the GitHub integration; make([]Repo, 0, total) (github/service.go)",
	"GET /v1/git/connections":                    "needs the GitHub integration; make([]Connection, 0, len(rows)) (github/service.go)",
	"GET /v1/git/claim/selections/{id}":          "needs the GitHub integration; candidates is make([]ClaimCandidate, 0, …) (github/service.go)",
	"GET /v1/agent-sessions":                     "needs the agent-session stack; make([]View, 0, len(rows)) (agentsessions/service.go)",
	"GET /v1/agent-sessions/{id}/transcript":     "needs the agent-session stack; TranscriptPage parts/turns are make (agentsessions/service.go)",
	"GET /v1/owners/{ownerId}/members":           "needs a real workspace; toRenderTeamMembers make (workspaces/render.go)",
	"GET /v1/workspaces/{workspaceId}/members":   "needs the members store; make([]MemberView, 0, len(ms)) (members/service.go)",
	"GET /v1/workspaces/{workspaceId}/invites":   "needs the members store; make([]InviteView, 0, len(invs)) (members/service.go)",
	"GET /v1/workspaces/{workspaceId}/billing":   "needs Stripe/Metronome; invoices is make([]Invoice, 0) (billing/read.go)",
	"GET /v1/logs":                               "needs a log source; renderLogList.Logs is make (logs/render.go)",
	"GET /v1/logs/values":                        "needs the durable log store; `all := []string{}` (logs/rest.go)",
	"GET /v1/logs/subscribe":                     "SSE stream, not a JSON body",
	"GET /v1/postgres/{id}/logs":                 "needs a log source; datastorelogs.Collect starts at []Entry{}",
	"GET /v1/key-value/{id}/logs":                "needs a log source; same datastorelogs.Collect",
	"GET /v1/users/deletion-preview":             "needs a direct browser session; Preview's delete/leave/blocked start as empty slices (accounts/service.go)",
	"GET /v1/postgres/{id}/processes":            "opens a live connection to the datastore; processViews make (postgres/insights.go)",
	"GET /v1/postgres/{id}/table-scans":          "live datastore connection; make([]TableScanView, 0, …) (postgres/insights.go)",
	"GET /v1/postgres/{id}/sizes":                "live datastore connection; tables is make([]TableSizeView, 0, …) (postgres/insights.go)",
	"GET /v1/postgres/{id}/parameter-overrides":  "live datastore connection; make([]ParameterOverrideView, 0, …) (postgres/insights.go)",
	"GET /v1/metrics/kv-memory":                  "resolves a Key Value through the live datastore path this fixture cannot satisfy; shares metrics.toRenderMetrics with the proven series routes",
	"GET /v1/metrics/kv-connections":             "same Key Value resolution; same toRenderMetrics",
	"GET /v1/metrics/bandwidth-sources":          "deliberately 501 — bex serves month-to-date totals, not a series",
}

// scalarRoutes are the GETs whose 200 body contains no JSON array at all.
var scalarRoutes = map[string]bool{
	"GET /v1/services/{id}":                                 true,
	"GET /v1/services/{id}/autoscaling":                     true,
	"GET /v1/services/{id}/deploy-hook":                     true,
	"GET /v1/services/{id}/deploys/{deployId}":              true,
	"GET /v1/services/{id}/env-vars/{key}":                  true,
	"GET /v1/services/{id}/secret-files/{name}":             true,
	"GET /v1/services/{id}/custom-domains/{name}":           true,
	"GET /v1/services/{id}/jobs/{jobId}":                    true,
	"GET /v1/services/{id}/runs/{runId}":                    true,
	"GET /v1/cron-jobs/{id}/runs/{runId}":                   true,
	"GET /v1/events/{eventId}":                              true,
	"GET /v1/env-groups/{id}":                               true,
	"GET /v1/env-groups/{id}/env-vars/{key}":                true,
	"GET /v1/env-groups/{id}/secret-files/{name}":           true,
	"GET /v1/environments/{id}":                             true,
	"GET /v1/projects/{id}":                                 true,
	"GET /v1/postgres/{id}":                                 true,
	"GET /v1/postgres/{id}/connection-info":                 true,
	"GET /v1/key-value/{id}":                                true,
	"GET /v1/key-value/{id}/connection-info":                true,
	"GET /v1/registrycredentials/{id}":                      true,
	"GET /v1/blueprints/{id}":                               true,
	"GET /v1/disks/{diskId}":                                true,
	"GET /v1/webhooks/{id}":                                 true,
	"GET /v1/owners/{ownerId}":                              true,
	"GET /v1/owners/{ownerId}/limits":                       true,
	"GET /v1/workspaces/{workspaceId}/seat-usage":           true,
	"GET /v1/notification-settings":                         true,
	"GET /v1/notification-settings/overrides/services/{id}": true,
	"GET /v1/agent-sessions/{id}":                           true,
	"GET /v1/git/callback":                                  true,
	"GET /v1/git/connection":                                true,
	"GET /v1/users":                                         true,
}

// emptyListFixture builds the DB-less server the table drives. withApp seeds
// one App, one Database and one KeyValue so the per-resource sub-routes
// resolve; every store and metrics source answers EMPTY, which is precisely
// the case Render spells `[]`.
func emptyListFixture(t *testing.T, withApp bool) (http.Handler, *Server) {
	t.Helper()
	objs := []client.Object{}
	if withApp {
		objs = append(objs,
			sampleApp("web"),
			conformDatabase("pg1"),
			conformKeyValue("kv1"),
			conformDatastoreSecret("pg1-app", map[string]string{
				"username": "u", "password": "p", "dbname": "d",
				"uri": "postgresql://u:p@pg1-rw.default:5432/d",
			}),
			conformDatastoreSecret("kv1", map[string]string{"uri": "redis://default:p@kv1.default:6379"}),
		)
	}
	base := &core.Base{Client: fakeClient(objs...), Namespace: "default"}
	noSeries := func(context.Context, metrics.ResourceMetricsRangeRequest) ([]metrics.MetricSeries, error) {
		return nil, nil
	}
	h, srv := serverWith(t, base, Deps{
		Secrets:            &fakeAuditKV{m: map[string]map[string]string{}},
		APIKeys:            newFakeKeyStore(),
		DeployStore:        &conformDeployStore{byApp: map[string][]store.Deploy{}},
		EventStore:         &fakeEventStore{},
		ProjectsStore:      newConformProjectStore(),
		WorkspaceStore:     newFakeWSStore(),
		RegistryCredsStore: newFakeRCStore(),
		Audit:              &audit.Service{Base: base, Store: &fakeAuditStore{}},

		ResourceMetrics: func(context.Context, string, string) ([]metrics.PodResourceUsage, error) {
			return nil, nil
		},
		ResourceMetricsRange: noSeries,
		ResourceLimitRange: func(context.Context, metrics.ResourceLimitRangeRequest) ([]metrics.MetricSeries, error) {
			return nil, nil
		},
		RequestMetrics: func(context.Context, metrics.RequestMetricsRequest) ([]metrics.MetricSeries, error) {
			return nil, nil
		},
		DiskUsage: func(context.Context, metrics.DiskUsageRequest) ([]metrics.MetricSeries, error) {
			return nil, nil
		},
		DBConnections: func(context.Context, metrics.DBConnectionsRequest) ([]metrics.MetricSeries, error) {
			return nil, nil
		},
		ReplicationLag: func(context.Context, metrics.ReplicationLagRequest) ([]metrics.MetricSeries, error) {
			return nil, nil
		},
		KeyValueStats: func(context.Context, metrics.KeyValueStatsRequest) ([]metrics.MetricSeries, error) {
			return nil, nil
		},
		MetricsFilterValues: func(context.Context, metrics.MetricsFilterValuesRequest) ([]string, error) {
			return nil, nil
		},
		MonthToDateBandwidth: func(context.Context, string, []string, bool, time.Time, time.Time) (metrics.BandwidthBytes, []string, error) {
			return metrics.BandwidthBytes{}, nil, nil
		},
	})
	return h, srv
}

// TestEmptyListRoutesSerializeAsJSONArray is the table: every array route whose
// empty case is reachable here must answer `[]`, never `null`.
func TestEmptyListRoutesSerializeAsJSONArray(t *testing.T) {
	bare, _ := emptyListFixture(t, false)
	seeded, _ := emptyListFixture(t, true)
	handlers := map[bool]http.Handler{false: bare, true: seeded}
	for _, tc := range emptyListCases {
		t.Run(tc.route+which(tc.field), func(t *testing.T) {
			w := do(t, handlers[tc.withApp], "GET", tc.path, testToken, "")
			if w.Code != http.StatusOK {
				t.Fatalf("GET %s => %d (want 200): %s", tc.path, w.Code, w.Body.String())
			}
			raw := w.Body.Bytes()
			if tc.field != "" {
				var obj map[string]json.RawMessage
				if err := json.Unmarshal(raw, &obj); err != nil {
					t.Fatalf("GET %s: body is not a JSON object: %s", tc.path, w.Body.String())
				}
				got, ok := obj[tc.field]
				if !ok {
					t.Fatalf("GET %s: response has no %q field: %s", tc.path, tc.field, w.Body.String())
				}
				raw = got
			}
			if got := strings.TrimSpace(string(raw)); got != "[]" {
				where := "body"
				if tc.field != "" {
					where = tc.field
				}
				t.Errorf("GET %s: empty %s = %s, want []\n"+
					"Render declares this as {\"type\":\"array\"} with no nullable; `null` is off contract.\n"+
					"Fix at the construction site (make([]T, 0, n)) — core.WriteJSON only normalizes a TOP-LEVEL nil slice.",
					tc.path, where, got)
			}
		})
	}
}

// TestEmptyListRouteTableCoversEveryGETRoute is the drift guard: a GET route
// added to any feature's REST fragment must be classified above — asserted
// empty, declared never-empty, declared unreachable with its non-nil
// construction cited, or declared scalar. Without this, "every list route" is
// a claim about the day the table was written.
func TestEmptyListRouteTableCoversEveryGETRoute(t *testing.T) {
	classified := map[string]bool{}
	for _, tc := range emptyListCases {
		classified[tc.route] = true
	}
	for route := range neverEmptyArrayRoutes {
		classified[route] = true
	}
	for route := range unreachableArrayRoutes {
		classified[route] = true
	}
	for route := range scalarRoutes {
		classified[route] = true
	}

	_, srv := emptyListFixture(t, true)
	var missing, stale []string
	live := map[string]bool{}
	for _, pattern := range serveMuxPatterns(srv.restHandler()) {
		if !strings.HasPrefix(pattern, "GET ") {
			continue
		}
		live[pattern] = true
		if !classified[pattern] {
			missing = append(missing, pattern)
		}
	}
	for route := range classified {
		if !live[route] {
			stale = append(stale, route)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 {
		t.Errorf("unclassified GET route(s) — add each to emptyListCases (preferred), "+
			"neverEmptyArrayRoutes, unreachableArrayRoutes, or scalarRoutes:\n  %s",
			strings.Join(missing, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("classified route(s) no longer served — drop them from the table:\n  %s",
			strings.Join(stale, "\n  "))
	}
}

func which(field string) string {
	if field == "" {
		return ""
	}
	return "#" + field
}
