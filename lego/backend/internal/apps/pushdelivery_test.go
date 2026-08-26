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

package apps

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/graphql-go/graphql"
)

// TestPushDeliveryMethodFollowsTheRepoGrant is w6/m99's core regression. Before
// it, every surface reported the stored autoDeploy setting and nothing else, so
// a github.com repo the workspace's installation does not grant — the live case
// this milestone was filed from — was indistinguishable from one it does, even
// though GitHub never delivers a push event for the former.
//
// It also pins the two "not a no" cases: an image-backed App has no push to
// deliver, and a deployment with the GitHub App unwired still redeploys through
// the manual HMAC webhook.
func TestPushDeliveryMethodFollowsTheRepoGrant(t *testing.T) {
	for _, tc := range []struct {
		name string
		repo string
		gh   *fakeCloneTokens
		want string
	}{
		{
			name: "granted-repo-keeps-the-github-app-claim", // the w2/m9 control case
			repo: "https://github.com/acme/mono",
			gh:   &fakeCloneTokens{token: "tok", ok: true},
			want: PushDeliveryGitHubApp,
		},
		{
			name: "ungranted-github-repo-is-manual-webhook",
			repo: "https://github.com/someone-else/side-project",
			gh:   &fakeCloneTokens{ok: false},
			want: PushDeliveryManualWebhook,
		},
		{
			name: "github-integration-off-is-manual-webhook-not-none",
			repo: "https://github.com/acme/mono",
			gh:   nil, // s.GitHub unwired
			want: PushDeliveryManualWebhook,
		},
		{
			name: "image-backed-service-has-no-push-to-deliver",
			repo: "",
			gh:   &fakeCloneTokens{ok: true},
			want: PushDeliveryNone,
		},
		{
			name: "github-failure-is-unknown-never-a-guess",
			repo: "https://github.com/acme/mono",
			gh:   &fakeCloneTokens{err: errors.New("github down")},
			want: PushDeliveryUnknown,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := repoApp("web", tc.repo, "master")
			svc, _ := newService(nil, app)
			if tc.gh != nil {
				svc.GitHub = tc.gh
			}
			view, err := svc.Get(context.Background(), "web")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if view.PushDeliveryMethod != tc.want {
				t.Errorf("pushDeliveryMethod = %q, want %q", view.PushDeliveryMethod, tc.want)
			}
			// The stored on/off SETTING is untouched by this milestone: it stays
			// default-on for a repo-backed create even when nothing can deliver
			// a push (Render's own behavior — the defect was the mechanism claim
			// layered on top, never the default).
			if tc.repo != "" && !view.AutoDeploy {
				t.Errorf("autoDeploy = false, want the stored default-on setting preserved")
			}
		})
	}
}

// TestPushDeliveryReadPathMintsNoCloneToken keeps the read and deploy paths
// distinct: reporting deliverability must go through RepoGranted, never mint a
// clone credential (and never write a Secret) just to render a hint.
func TestPushDeliveryReadPathMintsNoCloneToken(t *testing.T) {
	gh := &fakeCloneTokens{token: "tok", ok: true}
	svc, _ := newService(nil, repoApp("web", "https://github.com/acme/mono", "master"))
	svc.GitHub = gh
	if _, err := svc.Get(context.Background(), "web"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gh.calls != 0 {
		t.Errorf("read path called CloneToken %d times, want 0 (RepoGranted only)", gh.calls)
	}
	if gh.grantCalls != 1 {
		t.Errorf("read path called RepoGranted %d times, want 1", gh.grantCalls)
	}
}

// TestPushDeliveryMemoizesTheGrantLookup guards the cost of the check: it is
// GitHub round-trips, and the dashboard re-reads a service repeatedly, so
// repeated reads of the same workspace+repo must reuse one answer.
func TestPushDeliveryMemoizesTheGrantLookup(t *testing.T) {
	gh := &fakeCloneTokens{ok: true}
	svc, _ := newService(nil,
		repoApp("web", "https://github.com/acme/mono", "master"),
		repoApp("worker", "https://github.com/acme/mono", "master"),
	)
	svc.GitHub = gh
	for _, name := range []string{"web", "worker", "web"} {
		v, err := svc.Get(context.Background(), name)
		if err != nil {
			t.Fatalf("Get(%s): %v", name, err)
		}
		if v.PushDeliveryMethod != PushDeliveryGitHubApp {
			t.Errorf("%s pushDeliveryMethod = %q, want %q", name, v.PushDeliveryMethod, PushDeliveryGitHubApp)
		}
	}
	if gh.grantCalls != 1 {
		t.Errorf("three reads of one repo asked GitHub %d times, want 1 (memoized)", gh.grantCalls)
	}
}

// TestPushDeliveryIsNotComputedByList pins the deliberate scope of the check:
// only Get pays for it (the verb REST's by-id GET, GraphQL server(id)/service(id)
// and MCP get_service all route through). Computing it in List would put one
// GitHub round-trip per distinct repo on the hottest read path — a poll the
// dashboard's list query does not even select the field for. Absent there means
// "not computed on this projection", never "no".
func TestPushDeliveryIsNotComputedByList(t *testing.T) {
	gh := &fakeCloneTokens{ok: true}
	svc, _ := newService(nil,
		repoApp("web", "https://github.com/acme/mono", "master"),
		repoApp("worker", "https://github.com/other/repo", "master"),
	)
	svc.GitHub = gh
	views, err := svc.List(context.Background(), "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("List returned %d services, want 2", len(views))
	}
	for _, v := range views {
		if v.PushDeliveryMethod != "" {
			t.Errorf("%s pushDeliveryMethod = %q on a list projection, want empty", v.Name, v.PushDeliveryMethod)
		}
	}
	if gh.grantCalls != 0 {
		t.Errorf("List made %d grant lookups, want 0", gh.grantCalls)
	}
}

// TestPushDeliveryAgreesAcrossRESTGraphQLAndMCP is the parity assertion t003
// asks for in ONE place rather than three drifting per-surface tests: the same
// service, read three ways, must carry the identical value — for the granted
// case and the ungranted case alike. It also pins that autoDeploy/
// autoDeployTrigger keep reporting the stored setting on both.
func TestPushDeliveryAgreesAcrossRESTGraphQLAndMCP(t *testing.T) {
	for _, tc := range []struct {
		name    string
		granted bool
		want    string
	}{
		{name: "granted", granted: true, want: PushDeliveryGitHubApp},
		{name: "ungranted", granted: false, want: PushDeliveryManualWebhook},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newService(nil, repoApp("web", "https://github.com/acme/mono", "master"))
			svc.GitHub = &fakeCloneTokens{ok: tc.granted}
			ctx := context.Background()

			// REST
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("REST GET: %d %s", rec.Code, rec.Body)
			}
			var restBody struct {
				PushDeliveryMethod string `json:"pushDeliveryMethod"`
				AutoDeploy         string `json:"autoDeploy"`
				AutoDeployTrigger  string `json:"autoDeployTrigger"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &restBody); err != nil {
				t.Fatalf("decode REST body: %v", err)
			}
			if restBody.PushDeliveryMethod != tc.want {
				t.Errorf("REST pushDeliveryMethod = %q, want %q", restBody.PushDeliveryMethod, tc.want)
			}
			// Unchanged by this milestone on BOTH branches — the setting is on,
			// whether or not anything can deliver a push for the repo.
			if restBody.AutoDeploy != "yes" || restBody.AutoDeployTrigger != "commit" {
				t.Errorf("REST autoDeploy/%s trigger = %q/%q, want yes/commit",
					tc.name, restBody.AutoDeploy, restBody.AutoDeployTrigger)
			}

			// GraphQL
			schema, err := graphql.NewSchema(graphql.SchemaConfig{
				Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			})
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: ctx,
				RequestString: `{ server(id: "web") { pushDeliveryMethod autoDeploy autoDeployTrigger } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("gql: %v", res.Errors)
			}
			server := res.Data.(map[string]any)["server"].(map[string]any)
			if server["pushDeliveryMethod"] != tc.want {
				t.Errorf("GraphQL pushDeliveryMethod = %v, want %q", server["pushDeliveryMethod"], tc.want)
			}
			if server["autoDeploy"] != true || server["autoDeployTrigger"] != "commit" {
				t.Errorf("GraphQL autoDeploy/trigger = %v/%v, want true/commit",
					server["autoDeploy"], server["autoDeployTrigger"])
			}

			// MCP get_service — the same toRenderService projection its
			// structured content is built from.
			handler := svc.serviceTool(svc.Get)
			_, mcpService, err := handler(ctx, nil, serviceArgs{ServiceID: "web"})
			if err != nil {
				t.Fatalf("MCP get_service: %v", err)
			}
			if mcpService.PushDeliveryMethod != tc.want {
				t.Errorf("MCP pushDeliveryMethod = %q, want %q", mcpService.PushDeliveryMethod, tc.want)
			}
			if mcpService.AutoDeploy != "yes" || mcpService.AutoDeployTrigger != "commit" {
				t.Errorf("MCP autoDeploy/trigger = %q/%q, want yes/commit",
					mcpService.AutoDeploy, mcpService.AutoDeployTrigger)
			}
		})
	}
}
