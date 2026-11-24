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

	"github.com/modelcontextprotocol/go-sdk/mcp"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestSourcePatchRejectsBothRepoAndImage is w32/109: supplying both repo and
// image on PATCH /v1/services/{id}, ApplyServicePatch,
// SetSourceAndRegistryCredential, or MCP update_service must fail with 400
// (matching create, Blueprint validation, and UpdateBuildSource) and leave the
// service untouched, instead of letting image silently discard repo.
func TestSourcePatchRejectsBothRepoAndImage(t *testing.T) {
	const (
		origRepo  = "https://github.com/bex-co/bex"
		nextRepo  = "https://github.com/render-examples/go-web-server"
		origImage = "docker.io/library/alpine:3.20"
		nextImage = "docker.io/library/nginx:1.27-alpine"
		wantMsg   = "a service cannot declare both image and repo"
	)

	newFixture := func(kind string) *appv1alpha1.App {
		a := managedRepoApp("web")
		a.Spec.DisplayName = "Original Name"
		if kind == "git" {
			a.Spec.Repo = origRepo
			a.Spec.Branch = "main"
			a.Spec.Image = ""
			a.Spec.Runtime = "docker"
			a.Spec.CloneSecret = "web-clone"
			return a
		}
		a.Spec.Repo = ""
		a.Spec.Branch = appv1alpha1.DefaultBranch
		a.Spec.Image = origImage
		a.Spec.Runtime = "image"
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

	for _, startKind := range []string{"git", "image"} {
		t.Run("from_"+startKind, func(t *testing.T) {
			for _, surface := range []string{
				"REST PATCH",
				"REST PATCH dryRun",
				"REST PATCH multi-field",
				"ApplyServicePatch",
				"ApplyServicePatchDryRun",
				"SetSourceAndRegistryCredential",
				"MCP update_service",
				"MCP update_service dryRun",
			} {
				t.Run(surface, func(t *testing.T) {
					st := &recordingStore{}
					svc, cl := newService(st, newFixture(startKind))
					gh := &fakeCloneTokens{defaultBranch: "master", repoID: 707}
					svc.GitHub = gh
					before := getApp(t, cl, "web")

					switch surface {
					case "REST PATCH":
						mux := http.NewServeMux()
						svc.RegisterREST(mux)
						rec := httptest.NewRecorder()
						body := `{"repo":"` + nextRepo + `","image":{"imagePath":"` + nextImage + `","ownerId":""}}`
						validated(t, mux).ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", strings.NewReader(body)))
						if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), wantMsg) {
							t.Fatalf("PATCH %d: %s (want 400 containing %q)", rec.Code, rec.Body.String(), wantMsg)
						}
					case "REST PATCH dryRun":
						mux := http.NewServeMux()
						svc.RegisterREST(mux)
						rec := httptest.NewRecorder()
						body := `{"repo":"` + nextRepo + `","image":{"imagePath":"` + nextImage + `","ownerId":""}}`
						validated(t, mux).ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web?dryRun=true", strings.NewReader(body)))
						if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), wantMsg) {
							t.Fatalf("dryRun PATCH %d: %s (want 400 containing %q)", rec.Code, rec.Body.String(), wantMsg)
						}
					case "REST PATCH multi-field":
						mux := http.NewServeMux()
						svc.RegisterREST(mux)
						rec := httptest.NewRecorder()
						// name runs ahead of the source row in the op table; preflight must stop it too.
						body := `{"name":"Renamed","repo":"` + nextRepo + `","image":{"imagePath":"` + nextImage + `","ownerId":""}}`
						validated(t, mux).ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", strings.NewReader(body)))
						if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), wantMsg) {
							t.Fatalf("multi-field PATCH %d: %s (want 400 containing %q)", rec.Code, rec.Body.String(), wantMsg)
						}
					case "ApplyServicePatch":
						_, err := svc.ApplyServicePatch(context.Background(), "web", ServicePatch{
							DisplayName: str("Renamed"),
							Repo:        str(nextRepo),
							Image:       str(nextImage),
						})
						if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), wantMsg) {
							t.Fatalf("ApplyServicePatch err = %v, want ErrBadRequest containing %q", err, wantMsg)
						}
					case "ApplyServicePatchDryRun":
						_, err := svc.ApplyServicePatchDryRun(context.Background(), "web", ServicePatch{
							Repo:  str(nextRepo),
							Image: str(nextImage),
						})
						if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), wantMsg) {
							t.Fatalf("ApplyServicePatchDryRun err = %v, want ErrBadRequest containing %q", err, wantMsg)
						}
					case "SetSourceAndRegistryCredential":
						_, err := svc.SetSourceAndRegistryCredential(context.Background(), "web", sourcePatch{
							Repo:  str(nextRepo),
							Image: str(nextImage),
						})
						if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), wantMsg) {
							t.Fatalf("SetSourceAndRegistryCredential err = %v, want ErrBadRequest containing %q", err, wantMsg)
						}
					case "MCP update_service":
						res := callMCPUpdate(t, svc, map[string]any{
							"serviceId": "web",
							"repo":      nextRepo,
							"image":     nextImage,
						})
						if !res.IsError || !strings.Contains(mcpErrorText(res), wantMsg) {
							t.Fatalf("MCP update_service IsError=%v text=%q, want error containing %q", res.IsError, mcpErrorText(res), wantMsg)
						}
					case "MCP update_service dryRun":
						res := callMCPUpdate(t, svc, map[string]any{
							"serviceId": "web",
							"repo":      nextRepo,
							"image":     nextImage,
							"dryRun":    true,
						})
						if !res.IsError || !strings.Contains(mcpErrorText(res), wantMsg) {
							t.Fatalf("MCP update_service dryRun IsError=%v text=%q, want error containing %q", res.IsError, mcpErrorText(res), wantMsg)
						}
					}

					if !reflect.DeepEqual(before, getApp(t, cl, "web")) {
						t.Errorf("conflicting patch mutated App CR: got %+v, want %+v", getApp(t, cl, "web"), before)
					}
					if len(st.sourceWrites) != 0 {
						t.Errorf("conflicting patch wrote store rows: %+v", st.sourceWrites)
					}
					if len(st.deployCalls) != 0 {
						t.Errorf("conflicting patch opened deploys: %+v", st.deployCalls)
					}
					if gh.validateCalls != 0 {
						t.Errorf("conflicting patch called ValidateRepo %d times, want 0", gh.validateCalls)
					}
				})
			}
		})
	}
}

func TestSourcePatchRejectsBothRepoAndImageAllServiceTypesAndEmptyInputs(t *testing.T) {
	const (
		nextRepo  = "https://github.com/render-examples/go-web-server"
		nextImage = "docker.io/library/nginx:1.27-alpine"
		wantMsg   = "a service cannot declare both image and repo"
	)
	str := func(s string) *string { return &s }

	for _, svcType := range []string{
		appv1alpha1.TypeWebService,
		appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker,
		appv1alpha1.TypeCronJob,
		appv1alpha1.TypeStaticSite,
	} {
		t.Run("type_"+svcType, func(t *testing.T) {
			app := managedRepoApp("svc")
			app.Spec.Type = svcType
			if svcType == appv1alpha1.TypeCronJob {
				app.Spec.Schedule = "0 * * * *"
			}
			if svcType == appv1alpha1.TypeStaticSite {
				app.Spec.PublishPath = "dist"
			}
			svc, cl := newService(&recordingStore{}, app)
			before := getApp(t, cl, "svc")

			_, err := svc.SetSourceAndRegistryCredential(context.Background(), "svc", sourcePatch{
				Repo:  str(nextRepo),
				Image: str(nextImage),
			})
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), wantMsg) {
				t.Fatalf("err = %v, want ErrBadRequest containing %q", err, wantMsg)
			}
			if !reflect.DeepEqual(before, getApp(t, cl, "svc")) {
				t.Errorf("conflicting patch mutated %s CR", svcType)
			}
		})
	}

	for _, tc := range []struct {
		name  string
		patch sourcePatch
	}{
		{name: "empty repo with image", patch: sourcePatch{Repo: str(""), Image: str(nextImage)}},
		{name: "repo with empty image", patch: sourcePatch{Repo: str(nextRepo), Image: str("")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := managedRepoApp("web")
			svc, cl := newService(&recordingStore{}, app)
			before := getApp(t, cl, "web")
			_, err := svc.SetSourceAndRegistryCredential(context.Background(), "web", tc.patch)
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), wantMsg) {
				t.Fatalf("err = %v, want ErrBadRequest containing %q", err, wantMsg)
			}
			if !reflect.DeepEqual(before, getApp(t, cl, "web")) {
				t.Errorf("conflicting patch mutated CR")
			}
		})
	}
}

func TestSourcePatchSingleSourceSwitchControls(t *testing.T) {
	const (
		origRepo  = "https://github.com/bex-co/bex"
		nextRepo  = "https://github.com/render-examples/go-web-server"
		nextImage = "docker.io/library/nginx:1.27-alpine"
	)

	t.Run("REST PATCH image alone switches git service to image", func(t *testing.T) {
		st := &recordingStore{}
		app := managedRepoApp("web")
		app.Spec.Repo = origRepo
		app.Spec.Branch = "main"
		svc, cl := newService(st, app)
		mux := http.NewServeMux()
		svc.RegisterREST(mux)

		rec := httptest.NewRecorder()
		body := `{"repo":null,"image":{"imagePath":"` + nextImage + `","ownerId":""}}`
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH %d: %s", rec.Code, rec.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if out["imagePath"] != nextImage || out["repo"] != nil {
			t.Fatalf("response imagePath=%v repo=%v, want %q / nil", out["imagePath"], out["repo"], nextImage)
		}
		if spec := getApp(t, cl, "web").Spec; spec.Image != nextImage || spec.Repo != "" {
			t.Errorf("CR spec image=%q repo=%q, want %q / empty", spec.Image, spec.Repo, nextImage)
		}
	})

	t.Run("REST PATCH repo alone switches image service to git", func(t *testing.T) {
		st := &recordingStore{}
		app := managedRepoApp("web")
		app.Spec.Repo = ""
		app.Spec.Image = nextImage
		app.Spec.Runtime = "image"
		svc, cl := newService(st, app)
		svc.GitHub = &fakeCloneTokens{defaultBranch: "master", repoID: 707}
		mux := http.NewServeMux()
		svc.RegisterREST(mux)

		rec := httptest.NewRecorder()
		body := `{"repo":"` + nextRepo + `","image":null}`
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH %d: %s", rec.Code, rec.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if out["repo"] != nextRepo || out["imagePath"] != nil || out["branch"] != "master" {
			t.Fatalf("response repo=%v imagePath=%v branch=%v, want %q / nil / master", out["repo"], out["imagePath"], out["branch"], nextRepo)
		}
		if spec := getApp(t, cl, "web").Spec; spec.Repo != nextRepo || spec.Image != "" || spec.Branch != "master" {
			t.Errorf("CR spec repo=%q image=%q branch=%q, want %q / empty / master", spec.Repo, spec.Image, spec.Branch, nextRepo)
		}
	})
}
