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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/events"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestBuildEndedEventBuildIDDrivesBuildLogFilter is w4/710's end-to-end
// acceptance check: GET /v1/events/{id} for a build_ended returns details.buildId
// equal to its deploy id, and passing that exact buildId to
// GET /v1/logs?resource=<svc>&build=<buildId> queries Loki for that build's
// Job and returns its build-labeled lines (excluding another build's lines).
// The same test checks build_started, pre_deploy_started, pre_deploy_ended's
// deployCommandExecutionId and a build-source deploy_ended's buildId.
func TestBuildEndedEventBuildIDDrivesBuildLogFilter(t *testing.T) {
	at := time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)
	dep1 := ids.New(ids.Deploy)
	dep2 := ids.New(ids.Deploy)
	depBS := ids.New(ids.Deploy)
	shippedBuild := ids.New(ids.Build)

	app := eventsApp()
	appID := app.Labels[store.LabelAppID]

	buildStartedRow := store.ServiceEventRow{
		Key: "fact:deploy:" + dep2 + ":build_started", At: at.Add(-time.Second), Source: store.EventSourceFact,
		FactType: events.TypeBuildStarted, DeployID: dep2, Trigger: store.TriggerManual,
	}
	buildEndedRow := store.ServiceEventRow{
		Key: "fact:deploy:" + dep2 + ":build_ended", At: at, Source: store.EventSourceFact,
		FactType: events.TypeBuildEnded, DeployID: dep2, FactStatus: store.EventStatusSucceeded,
	}
	preDeployStartedRow := store.ServiceEventRow{
		Key: "fact:deploy:" + dep2 + ":pre_deploy_started", At: at.Add(time.Second), Source: store.EventSourceFact,
		FactType: events.TypePreDeployStarted, DeployID: dep2,
	}
	preDeployEndedRow := store.ServiceEventRow{
		Key: "fact:deploy:" + dep2 + ":pre_deploy_ended", At: at.Add(2 * time.Second), Source: store.EventSourceFact,
		FactType: events.TypePreDeployEnded, DeployID: dep2, FactStatus: store.EventStatusSucceeded,
	}
	bsDeployEndedRow := store.ServiceEventRow{
		Key: depBS + ":ended", At: at.Add(3 * time.Second), Source: store.EventSourceDeploy, Phase: store.EventPhaseEnded,
		DeployID: depBS, DeployBuildID: shippedBuild, Status: store.DeployLive,
	}

	evStore := &fakeEventStore{rows: []store.ServiceEventRow{
		buildStartedRow, buildEndedRow, preDeployStartedRow, preDeployEndedRow, bsDeployEndedRow,
	}}
	t1, t2 := at.Add(-2*time.Minute), at.Add(-time.Minute)
	deployStore := &conformDeployStore{byApp: map[string][]store.Deploy{
		appID: {
			{ID: dep2, AppID: appID, Generation: 3, Status: store.DeployLive, CreatedAt: t2, FinishedAt: &at},
			{ID: dep1, AppID: appID, Generation: 2, Status: store.DeployBuildFailed, FailureReason: "dep1 build failed", CreatedAt: t1, FinishedAt: &t2},
		},
	}}

	job1 := appv1alpha1.BuildJobName("web", appv1alpha1.BuildRevision(2))
	job2 := appv1alpha1.BuildJobName("web", appv1alpha1.BuildRevision(3))
	pod2 := job2 + "-bbbbb"
	var lokiQuery string
	lokiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lokiQuery = r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		ts := fmt.Sprintf("%d", at.Add(-30*time.Second).UnixNano())
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"streams","result":[{"stream":{"type":"build","pod":%q,"app":"web"},"values":[[%q,"second build line"]]}]}}`, pod2, ts)
	}))
	t.Cleanup(lokiSrv.Close)

	base := &core.Base{Client: fakeClient(app), Namespace: "default", Authz: &fakeChecker{allow: true}}
	// serverWith validates every response against Render's pinned OpenAPI spec.
	// srv.Handler() serves the exact same server for retrieve-event on the
	// ended members whose only documented omission is Render's deprecated
	// integer status (pinned per member by TestRenderEventDetailsMatchTheirMember).
	h, srv := serverWith(t, base, Deps{
		EventStore:  evStore,
		DeployStore: deployStore,
		LogHistory:  logs.NewLokiSource(lokiSrv.URL, lokiSrv.Client()),
	})
	raw, err := srv.Handler()
	if err != nil {
		t.Fatal(err)
	}

	// 1. GET /v1/events/{id} for build_started and pre_deploy_started conform
	//    with zero OpenAPI violations under rendertest.Wrap.
	for _, tc := range []struct {
		key, field, want string
	}{
		{buildStartedRow.Key, "buildId", dep2},
		{preDeployStartedRow.Key, "deployCommandExecutionId", dep2},
	} {
		res := do(t, h, http.MethodGet, "/v1/events/"+ids.Derive(ids.Event, tc.key), testToken, "")
		if res.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", tc.key, res.Code, res.Body.String())
		}
		var ev wireEvent
		if err := json.Unmarshal(res.Body.Bytes(), &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Details[tc.field] != tc.want {
			t.Errorf("%s details.%s = %v, want %q", tc.key, tc.field, ev.Details[tc.field], tc.want)
		}
	}

	// 2. GET /v1/events/{id} for build_ended returns buildId == dep2.
	buildEventID := ids.Derive(ids.Event, buildEndedRow.Key)
	res := do(t, raw, http.MethodGet, "/v1/events/"+buildEventID, testToken, "")
	if res.Code != http.StatusOK {
		t.Fatalf("GET build_ended event: %d %s", res.Code, res.Body.String())
	}
	var buildEv wireEvent
	if err := json.Unmarshal(res.Body.Bytes(), &buildEv); err != nil {
		t.Fatal(err)
	}
	gotBuildID, _ := buildEv.Details["buildId"].(string)
	if gotBuildID != dep2 {
		t.Fatalf("build_ended details.buildId = %q, want %q (body: %s)", gotBuildID, dep2, res.Body.String())
	}

	// 3. Follow buildId to GET /v1/logs?resource=<svc>&build=<that id> through
	//    the Render-validated handler and the real Loki query builder.
	q := url.Values{
		"resource":  {appID},
		"build":     {gotBuildID},
		"startTime": {at.Add(-5 * time.Minute).Format(time.RFC3339)},
		"endTime":   {at.Add(time.Minute).Format(time.RFC3339)},
	}
	logRes := do(t, h, http.MethodGet, "/v1/logs?"+q.Encode(), testToken, "")
	if logRes.Code != http.StatusOK {
		t.Fatalf("GET /v1/logs?build=%s: %d %s", gotBuildID, logRes.Code, logRes.Body.String())
	}
	if !strings.Contains(lokiQuery, job2) || strings.Contains(lokiQuery, job1) {
		t.Errorf("Loki query = %q, want it scoped to %s (dep2) and not %s (dep1)", lokiQuery, job2, job1)
	}
	var logBody struct {
		Logs []struct {
			Message string `json:"message"`
			Labels  []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"labels"`
		} `json:"logs"`
	}
	if err := json.Unmarshal(logRes.Body.Bytes(), &logBody); err != nil {
		t.Fatal(err)
	}
	var messages []string
	for _, l := range logBody.Logs {
		messages = append(messages, l.Message)
		if l.Message == "second build line" {
			var buildLabel string
			for _, lbl := range l.Labels {
				if lbl.Name == logs.LabelBuild {
					buildLabel = lbl.Value
				}
			}
			if buildLabel != dep2 {
				t.Errorf("build log line label build = %q, want %q", buildLabel, dep2)
			}
		}
	}
	if !slices.Contains(messages, "second build line") {
		t.Errorf("logs for build %s missing dep2's line: %v", gotBuildID, messages)
	}
	if slices.ContainsFunc(messages, func(m string) bool { return strings.Contains(m, "dep1 build failed") }) {
		t.Errorf("logs for build %s leaked dep1's platform line: %v", gotBuildID, messages)
	}

	// 4. GET /v1/events/{id} for pre_deploy_ended returns deployCommandExecutionId == dep2.
	preDeployEventID := ids.Derive(ids.Event, preDeployEndedRow.Key)
	preRes := do(t, raw, http.MethodGet, "/v1/events/"+preDeployEventID, testToken, "")
	if preRes.Code != http.StatusOK {
		t.Fatalf("GET pre_deploy_ended event: %d %s", preRes.Code, preRes.Body.String())
	}
	var preEv wireEvent
	if err := json.Unmarshal(preRes.Body.Bytes(), &preEv); err != nil {
		t.Fatal(err)
	}
	if preEv.Details["deployCommandExecutionId"] != dep2 {
		t.Errorf("pre_deploy_ended details.deployCommandExecutionId = %v, want %q", preEv.Details["deployCommandExecutionId"], dep2)
	}

	// 5. GET /v1/events/{id} for a build-source deploy_ended returns the deploy's buildId.
	bsEventID := ids.Derive(ids.Event, bsDeployEndedRow.Key)
	bsRes := do(t, raw, http.MethodGet, "/v1/events/"+bsEventID, testToken, "")
	if bsRes.Code != http.StatusOK {
		t.Fatalf("GET deploy_ended event: %d %s", bsRes.Code, bsRes.Body.String())
	}
	var bsEv wireEvent
	if err := json.Unmarshal(bsRes.Body.Bytes(), &bsEv); err != nil {
		t.Fatal(err)
	}
	if bsEv.Details["buildId"] != shippedBuild {
		t.Errorf("build-source deploy_ended details.buildId = %v, want %q", bsEv.Details["buildId"], shippedBuild)
	}
}
