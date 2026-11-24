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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/deploys"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func signGitWebhook(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// TestPGBuildSourcePushToBuildAndFanoutCrossSurface exercises w2/m610 end to
// end against real PostgreSQL and the composed REST, GraphQL, and MCP surfaces:
//   - a signed GitHub push matching buildFilter triggers a carrier build (bld-…)
//     with commitId/commitUrl readable identically on REST, GraphQL, and MCP;
//   - buildFilter ignores, skip phrases, and branch deletes leave no carrier
//     builds and leak zero service_event_facts rows;
//   - when the carrier build goes live, deploys.BuildSucceeded fans out one
//     deploy pinned to bld-… to linked autoDeploy:true and first-build services,
//     skipping autoDeploy:false and suspended services;
//   - image-backed Build Sources ignore git pushes.
func TestPGBuildSourcePushToBuildAndFanoutCrossSurface(t *testing.T) {
	ctx, pool, st := ownerGuardPG(t)
	owner, err := st.CreateWorkspace(ctx, "bds-push-"+ids.New(ids.Workspace), store.PlanHobby, "client-1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, owner.ID)
		_ = st.DeleteTenant(context.Background(), owner.ID)
	})

	const (
		ghSecret       = "pg-bds-gh-secret"
		installationID = int64(884201)
		repoURL        = "https://github.com/acme/monorepo"
		repoID         = int64(771234)
		commitSHA      = "1111111111111111111111111111111111111111"
		commitSHA2     = "2222222222222222222222222222222222222222"
		oldImage       = "zot.test/acme/bds@sha256:oldbuild"
		newImage       = "zot.test/acme/bds@sha256:newbuild"
	)

	if _, err := st.MaintainGitWebhookReplayEpochs(ctx, store.GitWebhookReplayEpochs("", ghSecret), time.Now()); err != nil {
		t.Fatalf("MaintainGitWebhookReplayEpochs: %v", err)
	}
	if _, err := st.BindGitConnection(ctx, store.GitConnection{
		WorkspaceID:    owner.ID,
		InstallationID: installationID,
		AccountLogin:   "acme",
	}, 10, 10); err != nil {
		t.Fatalf("BindGitConnection: %v", err)
	}

	// Create git-backed and image-backed Build Source carriers in Postgres + fake k8s.
	gitCarrierID := ids.New(ids.BuildSource)
	imgCarrierID := ids.New(ids.BuildSource)
	if _, err := st.CreateApp(ctx, store.App{
		ID:           gitCarrierID,
		DisplayName:  "MonoGit",
		TenantID:     owner.ID,
		Name:         gitCarrierID,
		Type:         store.TypeBuildSource,
		Repo:         repoURL,
		GitHubRepoID: repoID,
		Branch:       "main",
		Port:         8080,
		Replicas:     1,
		Tier:         "free",
	}); err != nil {
		t.Fatalf("CreateApp git carrier: %v", err)
	}
	if _, err := st.CreateApp(ctx, store.App{
		ID:          imgCarrierID,
		DisplayName: "MonoImg",
		TenantID:    owner.ID,
		Name:        imgCarrierID,
		Type:        store.TypeBuildSource,
		Image:       "ghcr.io/acme/prebuilt:v1",
		Port:        8080,
		Replicas:    1,
		Tier:        "free",
	}); err != nil {
		t.Fatalf("CreateApp img carrier: %v", err)
	}

	makeCarrierCR := func(id, display, repo, image string, bf *appv1alpha1.BuildFilterSpec) *appv1alpha1.App {
		a, _, _ := metadataResources(owner.ID)
		a.Name = store.CRName(owner.ID, id)
		a.Generation = 2
		a.Labels[store.LabelManagedBy] = store.ManagedByValue
		a.Labels[store.LabelAppID] = id
		a.Spec = appv1alpha1.AppSpec{
			Type:        appv1alpha1.TypeBuildSource,
			DisplayName: display,
			Repo:        repo,
			Image:       image,
			Branch:      "main",
			Runtime:     "node",
			BuildFilter: bf,
			AutoDeploy:  repo != "",
		}
		if repo != "" {
			core.SetGitHubRepoID(a, repoID)
		}
		return a
	}

	gitCarrierCR := makeCarrierCR(gitCarrierID, "MonoGit", repoURL, "", &appv1alpha1.BuildFilterSpec{
		Paths:        []string{"services/**"},
		IgnoredPaths: []string{"**/*.md"},
	})
	imgCarrierCR := makeCarrierCR(imgCarrierID, "MonoImg", "", "ghcr.io/acme/prebuilt:v1", nil)

	// Create 4 linked services: autoDeploy:true, autoDeploy:false, suspended, and waiting-for-first-build.
	type svcSpec struct {
		name       string
		autoDeploy bool
		suspended  bool
		image      string
	}
	linkedSpecs := []svcSpec{
		{name: "auto-api", autoDeploy: true, image: oldImage},
		{name: "manual-api", autoDeploy: false, image: oldImage},
		{name: "paused-api", autoDeploy: true, suspended: true, image: oldImage},
		{name: "first-api", autoDeploy: false, image: ""},
	}
	svcIDs := map[string]string{}
	objects := []client.Object{gitCarrierCR, imgCarrierCR}
	for _, sp := range linkedSpecs {
		row, err := st.CreateApp(ctx, store.App{
			TenantID:         owner.ID,
			Name:             sp.name,
			Type:             appv1alpha1.TypeWebService,
			BuildSourceID:    gitCarrierID,
			BuildSourceImage: sp.image,
			Port:             8080,
			Replicas:         1,
			Tier:             "starter",
		})
		if err != nil {
			t.Fatalf("CreateApp %s: %v", sp.name, err)
		}
		if sp.suspended {
			if err := st.SetAppSuspended(ctx, row.ID, true); err != nil {
				t.Fatalf("SetAppSuspended %s: %v", sp.name, err)
			}
		}
		svcIDs[sp.name] = row.ID
		a, _, _ := metadataResources(owner.ID)
		a.Name = store.CRName(owner.ID, sp.name)
		a.Labels[store.LabelManagedBy] = store.ManagedByValue
		a.Labels[store.LabelAppID] = row.ID
		a.Labels[core.LabelServiceName] = sp.name
		a.Spec = appv1alpha1.AppSpec{
			Type:       appv1alpha1.TypeWebService,
			AutoDeploy: sp.autoDeploy,
			Suspended:  sp.suspended,
			BuildSource: &appv1alpha1.BuildSourceLink{
				Name:  gitCarrierCR.Name,
				Image: sp.image,
			},
		}
		objects = append(objects, a)
	}

	base := serverBase(t, st)
	base.Client = fakeClient(objects...)
	base.Authz = &fakeMembershipChecker{memberOf: map[string]string{owner.ID: "user:client-1"}}
	srv := NewServer(base, Deps{
		WorkspaceStore: st,
		Store:          st,
		DeployStore:    st,
		GitHubStore:    st,
		EventFacts:     st,
	})
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.GitHubWebhookSecret = ghSecret
	srv.MultitenantWebhook = true
	srv.WebhookReplays = st
	handler := buildHandler(t, srv)
	session := mcpSessionAs(t, srv, "client-1")

	sendGitWebhook := func(event, delivery string, payload map[string]any) {
		t.Helper()
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/git", strings.NewReader(string(raw)))
		req.Header.Set("X-Hub-Signature-256", signGitWebhook(ghSecret, raw))
		req.Header.Set("X-GitHub-Event", event)
		req.Header.Set("X-GitHub-Delivery", delivery)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("webhook %s/%s => %d: %s", event, delivery, rec.Code, rec.Body.String())
		}
	}

	countFacts := func(appID string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM service_event_facts WHERE app_id = $1`, appID).Scan(&n); err != nil {
			t.Fatalf("count service_event_facts: %v", err)
		}
		return n
	}

	// Mark the carrier's initial deploy (and the already-running services'
	// initial deploys) live so subsequent pushes and fan-out deploys run
	// immediately instead of deferring behind unadopted initial rows.
	markInitialLive := func(appID, image string) {
		t.Helper()
		rows, err := st.ListDeploys(ctx, appID, store.DeployFilter{Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if _, err := st.TransitionDeploy(ctx, r.ID, store.DeployLive, store.RanImage{Ref: image}, "", "", "", nil); err != nil {
				t.Fatalf("TransitionDeploy %s: %v", r.ID, err)
			}
		}
	}
	markInitialLive(gitCarrierID, oldImage)
	for _, name := range []string{"auto-api", "manual-api", "paused-api"} {
		markInitialLive(svcIDs[name], oldImage)
	}

	// Count initial deploy rows opened at CreateApp time.
	beforeBuilds, err := st.ListDeploys(ctx, gitCarrierID, store.DeployFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Ignored by buildFilter.ignoredPaths and skip phrase -> no new build, 0 facts.
	sendGitWebhook("push", "pg-del-ignored", map[string]any{
		"ref":          "refs/heads/main",
		"after":        commitSHA,
		"installation": map[string]int64{"id": installationID},
		"repository":   map[string]any{"id": repoID, "clone_url": repoURL + ".git", "html_url": repoURL},
		"commits":      []map[string]any{{"modified": []string{"services/README.md"}}},
		"head_commit":  map[string]string{"id": commitSHA, "message": "docs only", "timestamp": "2026-11-23T12:00:00Z"},
	})
	sendGitWebhook("push", "pg-del-skip", map[string]any{
		"ref":          "refs/heads/main",
		"after":        commitSHA,
		"installation": map[string]int64{"id": installationID},
		"repository":   map[string]any{"id": repoID, "clone_url": repoURL + ".git", "html_url": repoURL},
		"commits":      []map[string]any{{"modified": []string{"services/api/main.go"}}},
		"head_commit":  map[string]string{"id": commitSHA, "message": "chore [skip render]", "timestamp": "2026-11-23T12:01:00Z"},
	})
	afterSkipped, err := st.ListDeploys(ctx, gitCarrierID, store.DeployFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(afterSkipped) != len(beforeBuilds) {
		t.Fatalf("skipped pushes opened carrier deploys: before=%d after=%d", len(beforeBuilds), len(afterSkipped))
	}
	if n := countFacts(gitCarrierID); n != 0 {
		t.Fatalf("skipped carrier pushes leaked %d service_event_facts rows", n)
	}

	// 2. Matching push triggers a carrier build with commit provenance & empty build_deploy_scope.
	sendGitWebhook("push", "pg-del-match", map[string]any{
		"ref":          "refs/heads/main",
		"after":        commitSHA,
		"installation": map[string]int64{"id": installationID},
		"repository":   map[string]any{"id": repoID, "clone_url": repoURL + ".git", "html_url": repoURL},
		"commits":      []map[string]any{{"modified": []string{"services/api/main.go"}}},
		"head_commit":  map[string]string{"id": commitSHA, "message": "feat: monorepo build", "timestamp": "2026-11-23T12:02:00Z"},
	})
	carrierDeploys, err := st.ListDeploys(ctx, gitCarrierID, store.DeployFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(carrierDeploys) != len(beforeBuilds)+1 {
		t.Fatalf("carrier deploys = %d, want %d", len(carrierDeploys), len(beforeBuilds)+1)
	}
	pushedDeploy := carrierDeploys[0]
	if pushedDeploy.Trigger != store.TriggerNewCommit || pushedDeploy.Commit != commitSHA || pushedDeploy.BuildDeployScope != "" {
		t.Fatalf("pushed carrier deploy = %+v, want trigger=new_commit commit=%s empty scope", pushedDeploy, commitSHA)
	}
	wantBuildID := deploys.BuildID(pushedDeploy.ID)
	wantCommitURL := repoURL + "/commit/" + commitSHA

	// Verify identical build fields across REST, GraphQL, and MCP.
	restListRec := do(t, handler, http.MethodGet, "/v1/build-sources/"+gitCarrierID+"/builds", testToken, "")
	if restListRec.Code != http.StatusOK {
		t.Fatalf("GET builds => %d: %s", restListRec.Code, restListRec.Body.String())
	}
	var restList []struct {
		Build struct {
			ID       string `json:"id"`
			BuildRun struct {
				CommitID  string `json:"commitId"`
				CommitURL string `json:"commitUrl"`
				Status    string `json:"status"`
			} `json:"buildRun"`
		} `json:"build"`
	}
	if err := json.Unmarshal(restListRec.Body.Bytes(), &restList); err != nil || len(restList) == 0 {
		t.Fatalf("decode REST builds: %v body=%s", err, restListRec.Body.String())
	}
	if restList[0].Build.ID != wantBuildID || restList[0].Build.BuildRun.CommitID != commitSHA || restList[0].Build.BuildRun.CommitURL != wantCommitURL {
		t.Fatalf("REST build = %+v, want id=%s commit=%s url=%s", restList[0].Build, wantBuildID, commitSHA, wantCommitURL)
	}

	gqlBuild := gql(t, handler, fmt.Sprintf(`{build(buildSourceId:%q,id:%q){id buildRun{commitId commitUrl status}}}`, gitCarrierID, wantBuildID))["build"].(map[string]any)
	gqlRun := gqlBuild["buildRun"].(map[string]any)
	if gqlBuild["id"] != wantBuildID || gqlRun["commitId"] != commitSHA || gqlRun["commitUrl"] != wantCommitURL {
		t.Fatalf("GraphQL build = %+v, want id=%s commit=%s url=%s", gqlBuild, wantBuildID, commitSHA, wantCommitURL)
	}

	mcpBuild := callTool[map[string]any](t, session, "get_build_source_build", map[string]any{"buildSourceId": gitCarrierID, "buildId": wantBuildID})
	mcpRun := mcpBuild["buildRun"].(map[string]any)
	if mcpBuild["id"] != wantBuildID || mcpRun["commitId"] != commitSHA || mcpRun["commitUrl"] != wantCommitURL {
		t.Fatalf("MCP build = %+v, want id=%s commit=%s url=%s", mcpBuild, wantBuildID, commitSHA, wantCommitURL)
	}

	// 3. Complete the carrier build and run BuildSucceeded fan-out.
	if _, err := st.TransitionDeploy(ctx, pushedDeploy.ID, store.DeployLive, store.RanImage{Ref: newImage}, "", "", "", nil); err != nil {
		t.Fatalf("TransitionDeploy pushed carrier deploy: %v", err)
	}
	if err := srv.Deploys.BuildSucceeded(ctx, store.BuiltSource{
		TenantID:         owner.ID,
		CarrierID:        gitCarrierID,
		DeployID:         pushedDeploy.ID,
		BuildDeployScope: pushedDeploy.BuildDeployScope,
		Commit:           pushedDeploy.CommitInfo(),
		Namespace:        gitCarrierCR.Namespace,
		Carrier:          gitCarrierCR.Name,
		Image:            newImage,
	}); err != nil {
		t.Fatalf("BuildSucceeded: %v", err)
	}

	// auto-api and first-api must each have a new_commit deploy pinned to wantBuildID.
	for _, name := range []string{"auto-api", "first-api"} {
		rec := do(t, handler, http.MethodGet, "/v1/services/"+svcIDs[name]+"/deploys", testToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s deploys => %d: %s", name, rec.Code, rec.Body.String())
		}
		var rows []struct {
			Deploy struct {
				Trigger string `json:"trigger"`
				BuildID string `json:"buildId"`
				Commit  struct {
					ID string `json:"id"`
				} `json:"commit"`
			} `json:"deploy"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, r := range rows {
			if r.Deploy.BuildID == wantBuildID && r.Deploy.Trigger == store.TriggerNewCommit && r.Deploy.Commit.ID == commitSHA {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s deploys = %+v, want a new_commit deploy with buildId=%s commit=%s", name, rows, wantBuildID, commitSHA)
		}
	}

	// manual-api (autoDeploy: false) and paused-api (suspended) must receive NO fan-out deploy for wantBuildID.
	for _, name := range []string{"manual-api", "paused-api"} {
		rows, err := st.ListDeploys(ctx, svcIDs[name], store.DeployFilter{Limit: 20})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.BuildID == wantBuildID {
				t.Fatalf("%s must not be deployed by push fan-out, found deploy %+v", name, r)
			}
		}
	}

	// Image-backed carrier must have received no new deploys.
	imgDeploys, err := st.ListDeploys(ctx, imgCarrierID, store.DeployFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range imgDeploys {
		if r.Trigger == store.TriggerNewCommit {
			t.Fatalf("image-backed build source got push deploy: %+v", r)
		}
	}

	// 4. Branch delete on carrier neither leaks branch_deleted facts nor bricks later pushes.
	sendGitWebhook("delete", "pg-del-branch", map[string]any{
		"ref":          "main",
		"ref_type":     "branch",
		"installation": map[string]int64{"id": installationID},
		"repository":   map[string]any{"id": repoID, "clone_url": repoURL + ".git", "html_url": repoURL},
	})
	if n := countFacts(gitCarrierID); n != 0 {
		t.Fatalf("branch delete leaked %d service_event_facts rows for carrier", n)
	}
	sendGitWebhook("push", "pg-del-after-recreate", map[string]any{
		"ref":          "refs/heads/main",
		"after":        commitSHA2,
		"installation": map[string]int64{"id": installationID},
		"repository":   map[string]any{"id": repoID, "clone_url": repoURL + ".git", "html_url": repoURL},
		"commits":      []map[string]any{{"modified": []string{"services/api/handler.go"}}},
		"head_commit":  map[string]string{"id": commitSHA2, "message": "feat: recreated branch", "timestamp": "2026-11-23T12:05:00Z"},
	})
	afterRecreate, err := st.ListDeploys(ctx, gitCarrierID, store.DeployFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(afterRecreate) != len(carrierDeploys)+1 || afterRecreate[0].Commit != commitSHA2 {
		t.Fatalf("push after branch delete/recreate: deploys=%+v, want new build for %s", afterRecreate, commitSHA2)
	}
}
