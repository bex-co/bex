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

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/events"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func tenantWebApp(tenant, id, name string) *appv1alpha1.App {
	app := sampleApp(core.CRName(tenant, name))
	app.Namespace = tenant
	app.Labels = map[string]string{
		core.LabelTenant:      tenant,
		core.LabelAppID:       id,
		core.LabelServiceName: name,
	}
	return app
}

func tenantBuildSource(tenant, id, displayName string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      core.CRName(tenant, id),
			Namespace: tenant,
			Labels: map[string]string{
				core.LabelTenant:      tenant,
				core.LabelAppID:       id,
				core.LabelServiceName: id,
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type:        appv1alpha1.TypeBuildSource,
			DisplayName: displayName,
			Image:       "docker.io/library/nginx:1.27",
		},
	}
}

type listQueryJobStore struct {
	jobs       []store.Job
	lastFilter store.JobListFilter
}

func (s *listQueryJobStore) ListJobs(_ context.Context, _, _ string, filter store.JobListFilter) ([]store.Job, error) {
	s.lastFilter = filter
	var out []store.Job
	for _, j := range s.jobs {
		if len(filter.Statuses) == 0 || slices.Contains(filter.Statuses, j.Status) {
			out = append(out, j)
		}
	}
	return out, nil
}

func (s *listQueryJobStore) GetJob(context.Context, string, string, string) (store.Job, error) {
	return store.Job{}, store.ErrNotFound
}

func (s *listQueryJobStore) UpdateJobStatus(context.Context, string, string) (store.Job, error) {
	return store.Job{}, store.ErrNotFound
}

type listQueryEventStore struct {
	rows []store.ServiceEventRow
}

func (s *listQueryEventStore) ListServiceEvents(_ context.Context, _, _ string, f store.ServiceEventFilter) ([]store.ServiceEventRow, error) {
	var out []store.ServiceEventRow
	for _, row := range s.rows {
		switch row.Source {
		case store.EventSourceDeploy:
			if slices.Contains(f.Phases, row.Phase) {
				out = append(out, row)
			}
		case store.EventSourceFact:
			if slices.Contains(f.FactTypes, row.FactType) {
				out = append(out, row)
			}
		}
	}
	return out, nil
}

func (s *listQueryEventStore) GetServiceEvent(context.Context, string, string) (store.ServiceEventLookup, error) {
	return store.ServiceEventLookup{}, store.ErrNotFound
}

func (s *listQueryEventStore) ServiceEventWorkspaces(context.Context, string) ([]string, error) {
	return nil, nil
}

// w5/m580 t004: composed-server behavioral tests proving comma-separated
// (explode=false) and repeated query parameters pass the OpenAPI request gate,
// filter accurately in the handlers, and pass response conformance.
func TestListNotificationOverridesMultiOwnerComposed(t *testing.T) {
	a := tenantWebApp("tea-aaaaaaaaaaaaaaaaaaaa", "srv-aaaaaaaaaaaaaaaaaaaa", "web")
	b := tenantWebApp("tea-bbbbbbbbbbbbbbbbbbbb", "srv-bbbbbbbbbbbbbbbbbbbb", "api")
	base := &core.Base{
		Client:    fakeClient(a, b),
		Namespace: "default",
		Workspace: coretest.Workspaces{"tea-aaaaaaaaaaaaaaaaaaaa", "tea-bbbbbbbbbbbbbbbbbbbb"},
		MemberWorkspaceIDs: func(context.Context, core.Identity) ([]string, error) {
			return []string{"tea-aaaaaaaaaaaaaaaaaaaa", "tea-bbbbbbbbbbbbbbbbbbbb"}, nil
		},
	}
	h, _ := serverWith(t, base, Deps{})

	listIDs := func(query string) []string {
		t.Helper()
		rec := do(t, h, http.MethodGet, "/v1/notification-settings/overrides"+query, testToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET overrides%s = %d %s", query, rec.Code, rec.Body.String())
		}
		var rows []struct {
			Override struct {
				ServiceID string `json:"serviceId"`
			} `json:"override"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, r.Override.ServiceID)
		}
		slices.Sort(out)
		return out
	}

	wantBoth := []string{"srv-aaaaaaaaaaaaaaaaaaaa", "srv-bbbbbbbbbbbbbbbbbbbb"}
	if got := listIDs("?ownerId=tea-aaaaaaaaaaaaaaaaaaaa,tea-bbbbbbbbbbbbbbbbbbbb"); !slices.Equal(got, wantBoth) {
		t.Errorf("comma-separated ownerId = %v, want %v", got, wantBoth)
	}
	if got := listIDs("?ownerId=tea-aaaaaaaaaaaaaaaaaaaa&ownerId=tea-bbbbbbbbbbbbbbbbbbbb"); !slices.Equal(got, wantBoth) {
		t.Errorf("repeated ownerId = %v, want %v", got, wantBoth)
	}
	if got := listIDs("?ownerId=tea-aaaaaaaaaaaaaaaaaaaa"); !slices.Equal(got, []string{"srv-aaaaaaaaaaaaaaaaaaaa"}) {
		t.Errorf("single ownerId = %v, want [srv-aaaaaaaaaaaaaaaaaaaa]", got)
	}
	if rec := do(t, h, http.MethodGet, "/v1/notification-settings/overrides?ownerId=tea-aaaaaaaaaaaaaaaaaaaa,tea-cccccccccccccccccccc", testToken, ""); rec.Code != http.StatusForbidden {
		t.Errorf("unauthorized ownerId in comma list = %d %s, want 403", rec.Code, rec.Body.String())
	}
}

func TestListBuildSourcesNameFilterComposed(t *testing.T) {
	bsA := tenantBuildSource("tea-aaaaaaaaaaaaaaaaaaaa", "bds-aaaaaaaaaaaaaaaaaaaa", "alpha")
	bsB := tenantBuildSource("tea-aaaaaaaaaaaaaaaaaaaa", "bds-bbbbbbbbbbbbbbbbbbbb", "beta")
	bsComma := tenantBuildSource("tea-bbbbbbbbbbbbbbbbbbbb", "bds-cccccccccccccccccccc", "gamma,delta")
	base := &core.Base{
		Client:    fakeClient(bsA, bsB, bsComma),
		Namespace: "default",
		Workspace: coretest.Workspaces{"tea-aaaaaaaaaaaaaaaaaaaa", "tea-bbbbbbbbbbbbbbbbbbbb"},
		MemberWorkspaceIDs: func(context.Context, core.Identity) ([]string, error) {
			return []string{"tea-aaaaaaaaaaaaaaaaaaaa", "tea-bbbbbbbbbbbbbbbbbbbb"}, nil
		},
	}
	h, _ := serverWith(t, base, Deps{})

	listNames := func(query string) []string {
		t.Helper()
		rec := do(t, h, http.MethodGet, "/v1/build-sources?"+query, testToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /v1/build-sources?%s = %d %s", query, rec.Code, rec.Body.String())
		}
		var rows []struct {
			BuildSource struct {
				Name string `json:"name"`
			} `json:"buildSource"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, r.BuildSource.Name)
		}
		slices.Sort(out)
		return out
	}

	for _, tc := range []struct {
		query string
		want  []string
	}{
		{"name=alpha,beta", []string{"alpha", "beta"}},
		{"name=alpha&name=beta", []string{"alpha", "beta"}},
		{"name=gamma%2Cdelta", []string{"gamma,delta"}},
		{"name=alpha,gamma%2Cdelta", []string{"alpha", "gamma,delta"}},
		{"ownerId=tea-aaaaaaaaaaaaaaaaaaaa,tea-bbbbbbbbbbbbbbbbbbbb&name=beta,gamma%2Cdelta", []string{"beta", "gamma,delta"}},
	} {
		t.Run(tc.query, func(t *testing.T) {
			if got := listNames(tc.query); !slices.Equal(got, tc.want) {
				t.Fatalf("names = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestListJobsStatusMultiValueComposed(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	app := tenantWebApp("tea-aaaaaaaaaaaaaaaaaaaa", "srv-aaaaaaaaaaaaaaaaaaaa", "web")
	jobStore := &listQueryJobStore{
		jobs: []store.Job{
			{ID: "job-11111111111111111111", ServiceName: "srv-aaaaaaaaaaaaaaaaaaaa", StartCommand: "echo a", PlanID: "starter", Status: store.JobPending, CreatedAt: now},
			{ID: "job-22222222222222222222", ServiceName: "srv-aaaaaaaaaaaaaaaaaaaa", StartCommand: "echo b", PlanID: "starter", Status: store.JobRunning, CreatedAt: now},
			{ID: "job-33333333333333333333", ServiceName: "srv-aaaaaaaaaaaaaaaaaaaa", StartCommand: "echo c", PlanID: "starter", Status: store.JobSucceeded, CreatedAt: now},
		},
	}
	base := &core.Base{
		Client:    fakeClient(app),
		Namespace: "default",
		Workspace: coretest.Workspaces{"tea-aaaaaaaaaaaaaaaaaaaa"},
	}
	h, _ := serverWith(t, base, Deps{JobStore: jobStore})

	for _, query := range []string{"status=pending,running", "status=pending&status=running"} {
		t.Run(query, func(t *testing.T) {
			rec := do(t, h, http.MethodGet, "/v1/services/srv-aaaaaaaaaaaaaaaaaaaa/jobs?"+query, testToken, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("GET jobs?%s = %d %s", query, rec.Code, rec.Body.String())
			}
			var rows []struct {
				Job struct {
					Status string `json:"status"`
				} `json:"job"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
				t.Fatal(err)
			}
			var statuses []string
			for _, r := range rows {
				statuses = append(statuses, r.Job.Status)
			}
			slices.Sort(statuses)
			if !slices.Equal(statuses, []string{"pending", "running"}) {
				t.Fatalf("statuses = %v, want [pending running]", statuses)
			}
			if !slices.Equal(jobStore.lastFilter.Statuses, []string{"pending", "running"}) {
				t.Fatalf("store filter statuses = %v, want [pending running]", jobStore.lastFilter.Statuses)
			}
		})
	}

	if rec := do(t, h, http.MethodGet, "/v1/services/srv-aaaaaaaaaaaaaaaaaaaa/jobs?status=pending,bogus", testToken, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status member = %d %s, want 400", rec.Code, rec.Body.String())
	}
}

func TestListEventsTypeMultiValueComposed(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	app := tenantWebApp("tea-aaaaaaaaaaaaaaaaaaaa", "srv-aaaaaaaaaaaaaaaaaaaa", "web")
	eventStore := &listQueryEventStore{
		rows: []store.ServiceEventRow{
			{
				Key:      "dep:dep-11111111111111111111:ended",
				At:       now,
				Source:   store.EventSourceDeploy,
				DeployID: "dep-11111111111111111111",
				Phase:    store.EventPhaseEnded,
				Status:   store.DeployLive,
			},
			{
				Key:      "fact:evt-22222222222222222222",
				At:       now.Add(-time.Minute),
				Source:   store.EventSourceFact,
				FactType: events.TypeServerFailed,
			},
		},
	}
	base := &core.Base{
		Client:    fakeClient(app),
		Namespace: "default",
		Workspace: coretest.Workspaces{"tea-aaaaaaaaaaaaaaaaaaaa"},
	}
	h, _ := serverWith(t, base, Deps{EventStore: eventStore})

	for _, query := range []string{"type=deploy_ended,server_failed", "type=deploy_ended&type=server_failed"} {
		t.Run(query, func(t *testing.T) {
			rec := do(t, h, http.MethodGet, "/v1/services/srv-aaaaaaaaaaaaaaaaaaaa/events?"+query, testToken, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("GET events?%s = %d %s", query, rec.Code, rec.Body.String())
			}
			var rows []struct {
				Event struct {
					Type string `json:"type"`
				} `json:"event"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, r := range rows {
				got = append(got, r.Event.Type)
			}
			slices.Sort(got)
			if !slices.Equal(got, []string{events.TypeDeployEnded, events.TypeServerFailed}) {
				t.Fatalf("types = %v, want [deploy_ended server_failed]", got)
			}
		})
	}
}

func TestListLogsOwnerIDComposed(t *testing.T) {
	appA := tenantWebApp("tea-aaaaaaaaaaaaaaaaaaaa", "srv-aaaaaaaaaaaaaaaaaaaa", "web")
	appB := tenantWebApp("tea-bbbbbbbbbbbbbbbbbbbb", "srv-bbbbbbbbbbbbbbbbbbbb", "web")
	podA := podFor(appA.Name, "web-a-1")
	podA.Namespace = appA.Namespace
	podB := podFor(appB.Name, "web-b-1")
	podB.Namespace = appB.Namespace

	base := &core.Base{
		Client:    fakeClient(appA, appB, podA, podB),
		Namespace: "default",
		Workspace: coretest.Workspaces{"tea-aaaaaaaaaaaaaaaaaaaa", "tea-bbbbbbbbbbbbbbbbbbbb"},
	}
	h, _ := serverWith(t, base, Deps{
		PodLogs: staticLogs(map[string][]string{
			"web-a-1": {"2026-11-20T12:00:01Z alpha-only"},
			"web-b-1": {"2026-11-20T12:00:01Z beta-only"},
		}),
		LogLabelValues: func(_ context.Context, app, _ string, _ logs.LogQuery) ([]string, error) {
			return []string{app}, nil
		},
	})

	const start = "2026-11-20T12:00:00Z"
	if rec := do(t, h, http.MethodGet, "/v1/logs?ownerId=tea-cccccccccccccccccccc&resource=srv-aaaaaaaaaaaaaaaaaaaa&startTime="+start, testToken, ""); rec.Code != http.StatusForbidden {
		t.Errorf("foreign ownerId on /v1/logs = %d %s, want 403", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodGet, "/v1/logs/values?ownerId=tea-cccccccccccccccccccc&resource=srv-aaaaaaaaaaaaaaaaaaaa&label=instance&startTime="+start, testToken, ""); rec.Code != http.StatusForbidden {
		t.Errorf("foreign ownerId on /v1/logs/values = %d %s, want 403", rec.Code, rec.Body.String())
	}
	if rec := do(t, h, http.MethodGet, "/v1/logs?ownerId=tea-bbbbbbbbbbbbbbbbbbbb&resource=srv-aaaaaaaaaaaaaaaaaaaa&startTime="+start, testToken, ""); rec.Code != http.StatusNotFound {
		t.Errorf("mismatched ownerId on /v1/logs = %d %s, want 404", rec.Code, rec.Body.String())
	}
	rec := do(t, h, http.MethodGet, "/v1/logs?ownerId=tea-bbbbbbbbbbbbbbbbbbbb&resource=web&startTime="+start, testToken, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "beta-only") || strings.Contains(rec.Body.String(), "alpha-only") {
		t.Fatalf("GET /v1/logs?ownerId=tea-b&resource=web = %d %s, want beta-only", rec.Code, rec.Body.String())
	}
	rec = do(t, h, http.MethodGet, "/v1/logs/values?ownerId=tea-bbbbbbbbbbbbbbbbbbbb&resource=web&label=instance&startTime="+start, testToken, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "srv-bbbbbbbbbbbbbbbbbbbb") || strings.Contains(rec.Body.String(), "srv-aaaaaaaaaaaaaaaaaaaa") {
		t.Fatalf("GET /v1/logs/values?ownerId=tea-b&resource=web = %d %s, want srv-b only", rec.Code, rec.Body.String())
	}
}

// w5/199: kin-openapi's explode=false query decoder validates only the first
// repeated key. Verify the composed REST OpenAPI gate joins repeated values for
// every non-`name` explode=false array query parameter across served lists so
// an invalid second value returns HTTP 400 (instead of slipping through as 200
// or reaching the handler), while literal percent-encoded commas in `name`
// filters remain intact.
func TestRepeatedExplodeFalseEnumQueriesRejectedComposed(t *testing.T) {
	commaSvc := tenantWebApp("tea-aaaaaaaaaaaaaaaaaaaa", "srv-aaaaaaaaaaaaaaaaaaaa", "comma")
	commaSvc.Spec.DisplayName = "a,b"
	commaSvc.Spec.Repo, commaSvc.Spec.Image = "https://github.com/bex-co/example", ""
	commaSvc.Spec.Runtime = "go"

	fragA := tenantWebApp("tea-aaaaaaaaaaaaaaaaaaaa", "srv-bbbbbbbbbbbbbbbbbbbb", "frag-a")
	fragA.Spec.DisplayName = "a"
	fragA.Spec.Repo, fragA.Spec.Image = "https://github.com/bex-co/example", ""
	fragA.Spec.Runtime = "go"

	fragB := tenantWebApp("tea-aaaaaaaaaaaaaaaaaaaa", "srv-cccccccccccccccccccc", "frag-b")
	fragB.Spec.DisplayName = "b"
	fragB.Spec.Repo, fragB.Spec.Image = "https://github.com/bex-co/example", ""
	fragB.Spec.Runtime = "go"

	site := tenantWebApp("tea-aaaaaaaaaaaaaaaaaaaa", "srv-dddddddddddddddddddd", "site")
	site.Spec.Type = appv1alpha1.TypeStaticSite
	site.Spec.Repo, site.Spec.Image = "https://github.com/bex-co/example", ""
	site.Spec.PublishPath = "dist"
	site.Spec.Routes = []appv1alpha1.StaticRoute{
		{Type: "redirect", Source: "/old", Destination: "/"},
		{Type: "rewrite", Source: "/app/*", Destination: "/index.html"},
	}

	jobStore := &listQueryJobStore{}
	eventStore := &listQueryEventStore{}
	sandboxUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("refused sandbox list request reached upstream: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(sandboxUpstream.Close)
	base := &core.Base{
		Client:    fakeClient(commaSvc, fragA, fragB, site),
		Namespace: "default",
		Workspace: coretest.Workspaces{"tea-aaaaaaaaaaaaaaaaaaaa"},
	}
	h, _ := serverWith(t, base, Deps{
		Region:                 "fsn1",
		JobStore:               jobStore,
		EventStore:             eventStore,
		SandboxClient:          sandbox.NewClient(sandboxUpstream.URL),
		SandboxTemplates:       map[string]sandbox.Template{"base": {Image: "alpine:3", CPU: "500m", Memory: "512Mi"}},
		SandboxDefaultTemplate: "base",
		SandboxDefaultPlan:     sandbox.PlanStarter,
	})

	for _, tc := range []struct {
		name     string
		path     string
		wantText string
		wantCode string
	}{
		{
			name:     "list-services region",
			path:     "/v1/services?region=frankfurt&region=BOGUS",
			wantText: "region must be one of",
		},
		{
			name:     "list-services type",
			path:     "/v1/services?type=web_service&type=BOGUS",
			wantText: "type must be one of",
		},
		{
			name:     "list-services env",
			path:     "/v1/services?env=go&env=BOGUS",
			wantText: "env must be one of",
		},
		{
			name:     "list-services suspended",
			path:     "/v1/services?suspended=suspended&suspended=BOGUS",
			wantText: "suspended must be one of",
		},
		{
			name:     "list-events type",
			path:     "/v1/services/srv-aaaaaaaaaaaaaaaaaaaa/events?type=deploy_ended&type=BOGUS",
			wantText: "type must be one of",
		},
		{
			name:     "list-routes type",
			path:     "/v1/services/srv-dddddddddddddddddddd/routes?type=redirect&type=BOGUS",
			wantText: "type must be one of",
		},
		{
			name:     "list-job status",
			path:     "/v1/services/srv-aaaaaaaaaaaaaaaaaaaa/jobs?status=pending&status=BOGUS",
			wantText: "status must be one of",
		},
		{
			name:     "list-postgres suspended",
			path:     "/v1/postgres?suspended=suspended&suspended=BOGUS",
			wantText: "suspended must be one of",
		},
		{
			name:     "list-registry-credentials type",
			path:     "/v1/registrycredentials?type=DOCKER&type=BOGUS",
			wantText: "type must be one of",
		},
		{
			name:     "list-sandboxes status",
			path:     "/v1/sandboxes?status=running&status=BOGUS",
			wantText: "status must be one of",
		},
		{
			name:     "list-sandbox-builds status",
			path:     "/v1/sandbox-groups/sbg-aaaaaaaaaaaaaaaaaaaa/builds?status=succeeded&status=BOGUS",
			wantText: "status must be one of",
			wantCode: "invalid_status",
		},
		{
			name:     "list-sandbox-snapshots status",
			path:     "/v1/sandbox-groups/sbg-aaaaaaaaaaaaaaaaaaaa/snapshots?ownerId=tea-aaaaaaaaaaaaaaaaaaaa&status=available&status=BOGUS",
			wantText: "status must be one of",
			wantCode: "invalid_status",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, h, http.MethodGet, tc.path, testToken, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("GET %s = %d %s, want 400", tc.path, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.wantText) {
				t.Errorf("GET %s body = %s, want %q", tc.path, rec.Body.String(), tc.wantText)
			}
			if tc.wantCode != "" {
				var errBody struct {
					Code string `json:"code"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
					t.Fatalf("unmarshal error body: %v", err)
				}
				if errBody.Code != tc.wantCode {
					t.Errorf("GET %s error code = %q, want %q", tc.path, errBody.Code, tc.wantCode)
				}
			}
		})
	}

	// Valid repeated enum values continue to pass the gate and return 200.
	for _, path := range []string{
		"/v1/services?region=frankfurt&region=oregon",
		"/v1/services?type=web_service&type=static_site",
		"/v1/services?env=go&env=node",
		"/v1/services/srv-dddddddddddddddddddd/routes?type=redirect&type=rewrite",
	} {
		if rec := do(t, h, http.MethodGet, path, testToken, ""); rec.Code != http.StatusOK {
			t.Errorf("valid repeated query GET %s = %d %s, want 200", path, rec.Code, rec.Body.String())
		}
	}

	// Literal percent-encoded commas in `name` remain intact, both standalone
	// and alongside repeated non-`name` explode=false parameters.
	for _, query := range []string{
		"name=a%2Cb",
		"name=a%2Cb&region=frankfurt&region=oregon",
	} {
		rec := do(t, h, http.MethodGet, "/v1/services?"+query, testToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /v1/services?%s = %d %s, want 200", query, rec.Code, rec.Body.String())
		}
		var rows []struct {
			Service struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"service"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].Service.ID != "srv-aaaaaaaaaaaaaaaaaaaa" || rows[0].Service.Name != "a,b" {
			t.Errorf("GET /v1/services?%s = %+v, want only srv-aaaaaaaaaaaaaaaaaaaa (a,b)", query, rows)
		}
	}
}

func TestExplodeFalseQueryIndexExcludesName(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	byOpID := map[string][]string{}
	for op, names := range contract.explodeFalseQuery {
		if slices.Contains(names, "name") {
			t.Errorf("operation %s included \"name\" in explodeFalseQuery: %v", op.OperationID, names)
		}
		if op.OperationID != "" {
			byOpID[op.OperationID] = names
		}
	}
	for opID, want := range map[string][]string{
		"list-services":             {"env", "region", "suspended", "type"},
		"list-postgres":             {"region", "suspended"},
		"list-key-value":            {"region"},
		"list-redis":                {"region"},
		"list-events":               {"type"},
		"list-routes":               {"type"},
		"list-job":                  {"status"},
		"list-registry-credentials": {"type"},
		"list-sandboxes":            {"status"},
		"list-sandbox-builds":       {"status"},
		"list-sandbox-snapshots":    {"status"},
	} {
		got := byOpID[opID]
		for _, param := range want {
			if !slices.Contains(got, param) {
				t.Errorf("%s missing %q in explodeFalseQuery (got %v)", opID, param, got)
			}
		}
	}
}
