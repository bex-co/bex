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
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestPrebuiltImageRefusesBranchAcrossSurfaces is w32/110: a prebuilt-image
// service has no Git repository and reads branch as null on every surface, so
// supplying a non-empty branch on REST PATCH, GraphQL setBranch, MCP
// update_service, or direct service verbs must return 400 (matching create's
// errPrebuiltImageField("branch")), leave spec.Branch untouched, stamp no
// AnnotationPendingSourceGeneration, and record zero branch_changed /
// source_changed events.
func TestPrebuiltImageRefusesBranchAcrossSurfaces(t *testing.T) {
	const (
		origImage = "docker.io/library/nginx:1.27-alpine"
		wantMsg   = "prebuilt image services cannot declare branch"
	)

	newImageFixture := func(svcType, storedBranch string) *appv1alpha1.App {
		a := managedImageApp("web")
		a.Spec.Type = svcType
		a.Spec.Image = origImage
		a.Spec.Branch = storedBranch
		a.Spec.DisplayName = "Original"
		if svcType == appv1alpha1.TypeCronJob {
			a.Spec.Schedule = "0 * * * *"
			a.Spec.Command = "echo hi"
		}
		return a
	}

	callMCPUpdate := func(t *testing.T, svc *Service, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		ctx := context.Background()
		server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
		svc.RegisterMCP(server)
		serverT, clientT := mcp.NewInMemoryTransports()
		ss, err := server.Connect(ctx, serverT, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = ss.Close() })
		cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cs.Close() })
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "update_service", Arguments: args})
		if err != nil {
			t.Fatalf("CallTool protocol err: %v", err)
		}
		return res
	}

	mcpErrorText := func(res *mcp.CallToolResult) string {
		var b strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				b.WriteString(tc.Text)
			}
		}
		return b.String()
	}

	str := func(s string) *string { return &s }

	for _, svcType := range []string{
		appv1alpha1.TypeWebService,
		appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker,
		appv1alpha1.TypeCronJob,
	} {
		for _, reqBranch := range []string{"dev", "main"} {
			t.Run(svcType+"/branch_"+reqBranch, func(t *testing.T) {
				for _, surface := range []string{
					"REST PATCH",
					"REST PATCH dryRun",
					"REST PATCH multi-field",
					"GraphQL setBranch",
					"MCP update_service",
					"MCP update_service dryRun",
					"SetSourceAndRegistryCredential",
					"ApplyServicePatch",
				} {
					t.Run(surface, func(t *testing.T) {
						st := &recordingStore{}
						sink := &captureAuditSink{}
						facts := &branchFacts{}
						svc, cl := newService(st, newImageFixture(svcType, appv1alpha1.DefaultBranch))
						svc.Audit = sink
						svc.EventFacts = facts
						before := getApp(t, cl, "web")

						switch surface {
						case "REST PATCH":
							mux := http.NewServeMux()
							svc.RegisterREST(mux)
							rec := httptest.NewRecorder()
							body := `{"branch":"` + reqBranch + `"}`
							validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(body)))
							if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), wantMsg) {
								t.Fatalf("PATCH %d: %s (want 400 containing %q)", rec.Code, rec.Body.String(), wantMsg)
							}
						case "REST PATCH dryRun":
							mux := http.NewServeMux()
							svc.RegisterREST(mux)
							rec := httptest.NewRecorder()
							body := `{"branch":"` + reqBranch + `"}`
							validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web?dryRun=true", strings.NewReader(body)))
							if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), wantMsg) {
								t.Fatalf("dryRun PATCH %d: %s (want 400 containing %q)", rec.Code, rec.Body.String(), wantMsg)
							}
						case "REST PATCH multi-field":
							mux := http.NewServeMux()
							svc.RegisterREST(mux)
							rec := httptest.NewRecorder()
							body := `{"name":"Renamed","branch":"` + reqBranch + `"}`
							validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(body)))
							if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), wantMsg) {
								t.Fatalf("multi-field PATCH %d: %s (want 400 containing %q)", rec.Code, rec.Body.String(), wantMsg)
							}
						case "GraphQL setBranch":
							res := graphql.Do(graphql.Params{
								Schema:        gqlSchema(t, svc),
								Context:       context.Background(),
								RequestString: `mutation { setBranch(id: "web", branch: "` + reqBranch + `") { id branch } }`,
							})
							if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Message, wantMsg) {
								t.Fatalf("GraphQL setBranch errors = %v, want error containing %q", res.Errors, wantMsg)
							}
						case "MCP update_service":
							res := callMCPUpdate(t, svc, map[string]any{
								"serviceId": "web",
								"branch":    reqBranch,
							})
							if !res.IsError || !strings.Contains(mcpErrorText(res), wantMsg) {
								t.Fatalf("MCP update_service IsError=%v text=%q, want error containing %q", res.IsError, mcpErrorText(res), wantMsg)
							}
						case "MCP update_service dryRun":
							res := callMCPUpdate(t, svc, map[string]any{
								"serviceId": "web",
								"branch":    reqBranch,
								"dryRun":    true,
							})
							if !res.IsError || !strings.Contains(mcpErrorText(res), wantMsg) {
								t.Fatalf("MCP update_service dryRun IsError=%v text=%q, want error containing %q", res.IsError, mcpErrorText(res), wantMsg)
							}
						case "SetSourceAndRegistryCredential":
							_, err := svc.SetSourceAndRegistryCredential(context.Background(), "web", sourcePatch{Branch: str(reqBranch)})
							if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), wantMsg) {
								t.Fatalf("SetSourceAndRegistryCredential err = %v, want ErrBadRequest containing %q", err, wantMsg)
							}
						case "ApplyServicePatch":
							_, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
								DisplayName: str("Renamed"),
								Branch:      str(reqBranch),
							})
							if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), wantMsg) {
								t.Fatalf("ApplyServicePatch err = %v, want ErrBadRequest containing %q", err, wantMsg)
							}
						}

						after := getApp(t, cl, "web")
						if !reflect.DeepEqual(after, before) {
							t.Errorf("refused branch patch mutated App CR:\ngot:  %+v\nwant: %+v", after, before)
						}
						if _, pending := after.Annotations[appv1alpha1.AnnotationPendingSourceGeneration]; pending {
							t.Errorf("refused branch patch stamped AnnotationPendingSourceGeneration")
						}
						if len(st.sourceWrites) != 0 {
							t.Errorf("refused branch patch wrote store rows: %+v", st.sourceWrites)
						}
						if len(facts.facts) != 0 {
							t.Errorf("refused branch patch emitted event facts: %+v", facts.facts)
						}
						if n := sink.countVerb(core.AuditVerbSetSourceAndRegistryCredential); n != 0 {
							t.Errorf("refused branch patch emitted %d source_changed audit rows, want 0", n)
						}
					})
				}
			})
		}
	}

	t.Run("switching git service to image with non-empty branch is refused", func(t *testing.T) {
		st := &recordingStore{}
		sink := &captureAuditSink{}
		facts := &branchFacts{}
		app := managedRepoApp("web")
		app.Spec.Branch = "release"
		svc, cl := newService(st, app)
		svc.Audit = sink
		svc.EventFacts = facts
		before := getApp(t, cl, "web")

		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		rec := httptest.NewRecorder()
		body := `{"image":{"imagePath":"` + origImage + `","ownerId":""},"branch":"dev"}`
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), wantMsg) {
			t.Fatalf("PATCH image+branch = %d: %s (want 400 containing %q)", rec.Code, rec.Body.String(), wantMsg)
		}
		if !reflect.DeepEqual(getApp(t, cl, "web"), before) {
			t.Errorf("refused image+branch patch mutated App CR")
		}
		if len(st.sourceWrites) != 0 || len(facts.facts) != 0 || sink.countVerb(core.AuditVerbSetSourceAndRegistryCredential) != 0 {
			t.Errorf("refused image+branch patch recorded side effects: writes=%d facts=%d audit=%d",
				len(st.sourceWrites), len(facts.facts), sink.countVerb(core.AuditVerbSetSourceAndRegistryCredential))
		}
	})

	t.Run("build-source-linked service refuses non-empty branch and accepts empty echo", func(t *testing.T) {
		svc, _, ctx := newBuildSourceService(t)
		created, err := svc.CreateBuildSource(ctx, monorepoSource)
		if err != nil {
			t.Fatalf("CreateBuildSource: %v", err)
		}
		web, err := svc.Create(ctx, CreateRequest{
			Name: "web", Type: appv1alpha1.TypeWebService, Runtime: "docker",
			Repo: "https://github.com/bex-co/bex", Branch: "main",
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		startCmd := "node server.js"
		if _, err := svc.LinkBuildSource(ctx, web.ID, created.ID, &startCmd, false); err != nil {
			t.Fatalf("LinkBuildSource: %v", err)
		}
		sink := &captureAuditSink{}
		facts := &branchFacts{}
		svc.Audit = sink
		svc.EventFacts = facts
		before := getTenantApp(t, svc.Client, "tea-a", "web")

		if _, err := svc.SetSourceAndRegistryCredential(ctx, web.ID, sourcePatch{Branch: str("dev")}); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "has no repo to build") {
			t.Fatalf("linked service branch patch = %v, want ErrBadRequest mentioning has no repo to build", err)
		}
		if _, err := svc.SetSourceAndRegistryCredential(ctx, web.ID, sourcePatch{Branch: str("")}); err != nil {
			t.Fatalf("linked service empty branch echo = %v, want no-op", err)
		}
		after := getTenantApp(t, svc.Client, "tea-a", "web")
		if !reflect.DeepEqual(after, before) {
			t.Errorf("linked service branch calls mutated CR")
		}
		if len(facts.facts) != 0 || sink.countVerb(core.AuditVerbSetSourceAndRegistryCredential) != 0 {
			t.Errorf("linked service branch calls emitted events: facts=%+v audit=%d",
				facts.facts, sink.countVerb(core.AuditVerbSetSourceAndRegistryCredential))
		}
	})

	t.Run("empty branch echo on prebuilt-image service is a no-op that preserves retained branch", func(t *testing.T) {
		for _, storedBranch := range []string{"", appv1alpha1.DefaultBranch, "release"} {
			t.Run("stored_"+storedBranch, func(t *testing.T) {
				st := &recordingStore{}
				sink := &captureAuditSink{}
				facts := &branchFacts{}
				svc, cl := newService(st, newImageFixture(appv1alpha1.TypeWebService, storedBranch))
				svc.Audit = sink
				svc.EventFacts = facts
				before := getApp(t, cl, "web")

				mux := http.NewServeMux()
				svc.RegisterREST(mux)
				rec := httptest.NewRecorder()
				validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(`{"branch":""}`)))
				if rec.Code != http.StatusOK {
					t.Fatalf("PATCH branch \"\" = %d: %s, want 200 no-op", rec.Code, rec.Body.String())
				}
				after := getApp(t, cl, "web")
				if !reflect.DeepEqual(after, before) {
					t.Errorf("empty branch echo mutated CR:\ngot:  %+v\nwant: %+v", after, before)
				}
				if _, pending := after.Annotations[appv1alpha1.AnnotationPendingSourceGeneration]; pending {
					t.Errorf("empty branch echo stamped AnnotationPendingSourceGeneration")
				}
				if len(st.sourceWrites) != 0 || len(facts.facts) != 0 || sink.countVerb(core.AuditVerbSetSourceAndRegistryCredential) != 0 {
					t.Errorf("empty branch echo had side effects: writes=%d facts=%d audit=%d",
						len(st.sourceWrites), len(facts.facts), sink.countVerb(core.AuditVerbSetSourceAndRegistryCredential))
				}
			})
		}
	})
}

// TestSourceChangeBranchEventControls pins w32/110's event-fact contract for
// both direct source patches and Blueprint syncs: branch_changed is emitted
// only when both before and after are repo-backed and the tracked branch
// actually moved; switching Git -> image or image -> Git emits source_changed
// and zero branch_changed facts.
func TestSourceChangeBranchEventControls(t *testing.T) {
	const (
		repoA = "https://github.com/bex-co/bex"
		repoB = "https://github.com/render-examples/go-web-server"
		image = "docker.io/library/nginx:1.27-alpine"
	)
	str := func(s string) *string { return &s }

	t.Run("git service branch update emits branch_changed and source_changed", func(t *testing.T) {
		st := &recordingStore{}
		sink := &captureAuditSink{}
		facts := &branchFacts{}
		app := managedRepoApp("web")
		app.Spec.Repo = repoA
		app.Spec.Branch = "main"
		svc, cl := newService(st, app)
		svc.Audit = sink
		svc.EventFacts = facts

		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(`{"branch":"dev"}`)))
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH branch dev = %d: %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body["branch"] != "dev" {
			t.Errorf("response branch = %v, want dev", body["branch"])
		}
		got := getApp(t, cl, "web")
		if got.Spec.Branch != "dev" {
			t.Errorf("CR spec.Branch = %q, want dev", got.Spec.Branch)
		}
		if _, pending := got.Annotations[appv1alpha1.AnnotationPendingSourceGeneration]; !pending {
			t.Errorf("git branch update did not stamp AnnotationPendingSourceGeneration")
		}
		if len(facts.facts) != 1 || facts.facts[0].Type != store.EventFactBranchChanged ||
			facts.facts[0].BranchFrom != "main" || facts.facts[0].BranchTo != "dev" {
			t.Fatalf("facts = %+v, want one branch_changed main->dev", facts.facts)
		}
		if n := sink.countVerb(core.AuditVerbSetSourceAndRegistryCredential); n != 1 {
			t.Errorf("source_changed audit count = %d, want 1", n)
		}
	})

	t.Run("image service switched to repo+branch emits source_changed and zero branch_changed", func(t *testing.T) {
		st := &recordingStore{}
		sink := &captureAuditSink{}
		facts := &branchFacts{}
		app := managedImageApp("web")
		app.Spec.Image = image
		app.Spec.Branch = appv1alpha1.DefaultBranch
		svc, cl := newService(st, app)
		svc.Audit = sink
		svc.EventFacts = facts

		v, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
			Repo:   str(repoA),
			Branch: str("dev"),
		})
		if err != nil {
			t.Fatalf("image -> repo+branch: %v", err)
		}
		if v.Repo != repoA || v.Branch != "dev" || v.SourceImage != "" {
			t.Fatalf("view = repo %q branch %q sourceImage %q, want %q / dev / empty", v.Repo, v.Branch, v.SourceImage, repoA)
		}
		if spec := getApp(t, cl, "web").Spec; spec.Repo != repoA || spec.Branch != "dev" {
			t.Errorf("CR spec = repo %q branch %q, want %q / dev", spec.Repo, spec.Branch, repoA)
		}
		if len(facts.facts) != 0 {
			t.Errorf("image -> repo+branch emitted spurious branch_changed facts: %+v", facts.facts)
		}
		if n := sink.countVerb(core.AuditVerbSetSourceAndRegistryCredential); n != 1 {
			t.Errorf("source_changed audit count = %d, want 1", n)
		}
	})

	t.Run("image service switched to master-default repo without branch emits zero branch_changed", func(t *testing.T) {
		st := &recordingStore{}
		sink := &captureAuditSink{}
		facts := &branchFacts{}
		app := managedImageApp("web")
		app.Spec.Image = image
		app.Spec.Branch = appv1alpha1.DefaultBranch
		svc, cl := newService(st, app)
		svc.GitHub = &fakeCloneTokens{defaultBranch: "master", repoID: 707}
		svc.Audit = sink
		svc.EventFacts = facts

		v, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{Repo: str(repoB)})
		if err != nil {
			t.Fatalf("image -> repo: %v", err)
		}
		if v.Repo != repoB || v.Branch != "master" {
			t.Fatalf("view = repo %q branch %q, want %q / master", v.Repo, v.Branch, repoB)
		}
		if spec := getApp(t, cl, "web").Spec; spec.Branch != "master" {
			t.Errorf("CR spec.Branch = %q, want master", spec.Branch)
		}
		if len(facts.facts) != 0 {
			t.Errorf("image -> repo emitted spurious branch_changed facts: %+v", facts.facts)
		}
		if n := sink.countVerb(core.AuditVerbSetSourceAndRegistryCredential); n != 1 {
			t.Errorf("source_changed audit count = %d, want 1", n)
		}
	})

	t.Run("git service on release switched to image emits source_changed and zero branch_changed", func(t *testing.T) {
		st := &recordingStore{}
		sink := &captureAuditSink{}
		facts := &branchFacts{}
		app := managedRepoApp("web")
		app.Spec.Repo = repoA
		app.Spec.Branch = "release"
		svc, cl := newService(st, app)
		svc.Audit = sink
		svc.EventFacts = facts

		v, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{Image: str(image)})
		if err != nil {
			t.Fatalf("git -> image: %v", err)
		}
		if v.SourceImage != image || v.Repo != "" || v.Branch != "" {
			t.Fatalf("view = sourceImage %q repo %q branch %q, want %q / empty / empty", v.SourceImage, v.Repo, v.Branch, image)
		}
		if spec := getApp(t, cl, "web").Spec; spec.Image != image || spec.Repo != "" {
			t.Errorf("CR spec = image %q repo %q, want %q / empty", spec.Image, spec.Repo, image)
		}
		if len(facts.facts) != 0 {
			t.Errorf("git -> image emitted spurious branch_changed facts: %+v", facts.facts)
		}
		if n := sink.countVerb(core.AuditVerbSetSourceAndRegistryCredential); n != 1 {
			t.Errorf("source_changed audit count = %d, want 1", n)
		}
	})

	t.Run("blueprint sync git(release) to image emits source_changed and zero branch_changed", func(t *testing.T) {
		rec := &recordingStore{}
		svc, _, sink, ctx := managedStackService(t, rec)
		facts := &branchFacts{}
		svc.EventFacts = facts

		releaseGit := strings.Replace(nodeService, "branch: main", "branch: release", 1)
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: releaseGit}); err != nil {
			t.Fatalf("create git(release): %v", err)
		}
		sink.events = nil
		facts.facts = nil

		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: imageService}); err != nil {
			t.Fatalf("switch to image: %v", err)
		}
		if len(facts.facts) != 0 {
			t.Errorf("blueprint git(release) -> image emitted spurious branch_changed facts: %+v", facts.facts)
		}
		if n := sink.countVerb(core.AuditVerbSetSourceAndRegistryCredential); n != 1 {
			t.Errorf("source_changed audit count = %d, want 1", n)
		}
	})

	t.Run("blueprint sync image to git(dev) emits source_changed and zero branch_changed", func(t *testing.T) {
		rec := &recordingStore{}
		svc, _, sink, ctx := managedStackService(t, rec)
		facts := &branchFacts{}
		svc.EventFacts = facts

		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: imageService}); err != nil {
			t.Fatalf("create image: %v", err)
		}
		sink.events = nil
		facts.facts = nil

		devGit := strings.Replace(nodeService, "branch: main", "branch: dev", 1)
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: devGit}); err != nil {
			t.Fatalf("switch to git(dev): %v", err)
		}
		if len(facts.facts) != 0 {
			t.Errorf("blueprint image -> git(dev) emitted spurious branch_changed facts: %+v", facts.facts)
		}
		if n := sink.countVerb(core.AuditVerbSetSourceAndRegistryCredential); n != 1 {
			t.Errorf("source_changed audit count = %d, want 1", n)
		}
	})
}
