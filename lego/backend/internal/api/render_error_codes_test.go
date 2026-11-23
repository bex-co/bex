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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestRenderOperationsAnswerOnlyRendersErrorCodes pins w8/411: an error on one
// of Render's REST operations carries a code only from the pinned errorCode
// vocabulary, as Render's own error body does, so the pinned CLI prints
// `received response code 404: not found`, not `… 404 (NOT_FOUND): not found`.
// That holds for a refusal a middleware outside the request validator writes
// too. A code in the vocabulary still answers, and a bex-native route keeps
// bex's code for the bex CLI's own commands.
func TestRenderOperationsAnswerOnlyRendersErrorCodes(t *testing.T) {
	base := &core.Base{Client: (&admission{}).client(), Namespace: "default", Authz: &fakeChecker{allow: true}}
	h, _ := serverWith(t, base, Deps{})
	readOnly, _, _ := scopedAPI(t, "identity-1", "dcr-client", "openid "+core.ScopeRead, []string{bexResource}, map[string]bool{"dcr-client": false})
	for _, c := range []struct {
		h                  http.Handler
		name, method, path string
		status             int
		code               string // "" when the body must carry none
	}{
		{h, "an unknown service", http.MethodGet, "/v1/services/srv-doesnotexist000000000", http.StatusNotFound, ""},
		{h, "an unknown custom domain", http.MethodGet, "/v1/services/srv-doesnotexist000000000/custom-domains/shop.example.com", http.StatusNotFound, ""},
		{h, "a restart of an unknown service", http.MethodPost, "/v1/services/srv-doesnotexist000000000/restart", http.StatusNotFound, ""},
		{h, "a NUL in a Render operation's query", http.MethodGet, "/v1/services?name=%00", http.StatusBadRequest, ""},
		{readOnly, "a code in Render's vocabulary", http.MethodPost, "/v1/services/web/suspend", http.StatusForbidden, core.InsufficientScopeCode},
		{h, "a bex-native route", http.MethodGet, "/v1/git/connections?ownerId=default", http.StatusServiceUnavailable, "GITHUB_UNAVAILABLE"},
		{h, "a NUL in a bex-native route's query", http.MethodGet, "/v1/git/connections?ownerId=%00", http.StatusBadRequest, core.InvalidTextCode},
	} {
		t.Run(c.name, func(t *testing.T) {
			res := do(t, c.h, c.method, c.path, testToken, "")
			var body map[string]any
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatalf("%s %s answered %d %s: %v", c.method, c.path, res.Code, res.Body, err)
			}
			code, hasCode := body["code"]
			if res.Code != c.status || hasCode != (c.code != "") || (hasCode && code != c.code) {
				t.Fatalf("%s %s = %d %s, want %d with code %q", c.method, c.path, res.Code, res.Body, c.status, c.code)
			}
			if id, _ := body["id"].(string); id == "" || body["message"] == "" {
				t.Fatalf("%s %s = %s, want Render's id and message", c.method, c.path, res.Body)
			}
			if _, ok := body["params"]; ok && body["params"] == nil {
				t.Fatalf("%s %s = %s, want no params: null", c.method, c.path, res.Body)
			}
		})
	}
}

type blueprintPlanFixtureStore struct {
	*blueprintFixtureStore
}

func (f *blueprintPlanFixtureStore) CreatePlannedBlueprint(_ context.Context, b store.Blueprint) (store.Blueprint, error) {
	return b, nil
}

func (f *blueprintPlanFixtureStore) UndoPlannedBlueprintCreate(context.Context, store.Blueprint) error {
	return nil
}

func (f *blueprintPlanFixtureStore) InsertPlannedBlueprintSync(_ context.Context, p store.PlannedBlueprintSync) (store.PlannedBlueprintSync, error) {
	return p, nil
}

func (f *blueprintPlanFixtureStore) LatestBlueprintPlan(context.Context, string, time.Time) (store.PlannedBlueprintSync, error) {
	return store.PlannedBlueprintSync{}, store.ErrNotFound
}

func (f *blueprintPlanFixtureStore) BlueprintPlannedSync(context.Context, string, string) (store.PlannedBlueprintSync, error) {
	return store.PlannedBlueprintSync{}, store.ErrNotFound
}

func (f *blueprintPlanFixtureStore) SettleNoChangesPlan(_ context.Context, id, tenantID, _ string) (store.Blueprint, error) {
	return f.GetBlueprint(context.Background(), id, tenantID)
}

type errBlueprintFetcher struct{ err error }

func (f errBlueprintFetcher) ResolveBlueprintCommit(context.Context, string, string, string) (string, error) {
	return "", f.err
}

func (f errBlueprintFetcher) FetchBlueprintFileAtCommit(context.Context, string, string, string, string) (string, error) {
	return "", f.err
}

type stubSandboxGroupStore struct {
	sandbox.GroupStore
	ws        string
	defaultID string
}

func (s *stubSandboxGroupStore) SandboxGroupWorkspace(_ context.Context, id string) (string, error) {
	if id == s.defaultID {
		return s.ws, nil
	}
	return "", store.ErrNotFound
}

func (s *stubSandboxGroupStore) MarkSandboxGroupDeleting(_ context.Context, _, id string) (store.SandboxGroup, error) {
	if id == s.defaultID {
		return store.SandboxGroup{}, store.ErrSandboxGroupIsDefault
	}
	return store.SandboxGroup{}, store.ErrNotFound
}

func (s *stubSandboxGroupStore) SetDefaultSandboxGroup(_ context.Context, _, _ string) (store.SandboxGroup, error) {
	return store.SandboxGroup{}, store.ErrNotFound
}

// TestRenderCLIOperationsOutsideSpecErrorCodes pins w32/100: Render CLI REST
// operations served outside the embedded render-public-api-1.json (EA
// Blueprints create/plan/approve, sandbox-groups delete/set-default, EA
// Objects, Postgres parameter-overrides, CLI telemetry, and ephemeral-shell)
// are filtered by withRenderErrorCodes just like spec-listed operations.
// Bex-internal codes leave the REST body, Render's own errorCode vocabulary
// values stay, and bex-native routes keep bex's codes.
func TestRenderCLIOperationsOutsideSpecErrorCodes(t *testing.T) {
	objSvc, ws := newObjectService(t)
	defaultGroupID := ids.New(ids.SandboxGroup)
	existingBP := store.Blueprint{ID: "blp-existing000000000000", TenantID: ws, Name: "taken-bp"}
	bpStore := &blueprintPlanFixtureStore{blueprintFixtureStore: &blueprintFixtureStore{rows: map[string]store.Blueprint{existingBP.ID: existingBP}}}

	web := sampleApp("web")
	web.Labels = map[string]string{core.LabelTenant: ws, core.LabelServiceName: "web", core.LabelAppID: "srv-web"}

	baseAllow := &core.Base{
		Client:    (&admission{}).client(web),
		Namespace: "default",
		Authz:     &fakeChecker{allow: true},
		Workspace: fakeWorkspace{"client-1": ws},
	}
	baseDeny := &core.Base{
		Client:    (&admission{}).client(web),
		Namespace: "default",
		Authz:     &fakeChecker{allow: false},
		Workspace: fakeWorkspace{"client-1": ws},
	}

	deps := Deps{
		BlueprintsStore: bpStore,
		SandboxClient:   &sandbox.Client{},
		SandboxGroups:   &stubSandboxGroupStore{ws: ws, defaultID: defaultGroupID},
		ObjectStorage:   objSvc,
	}
	h, srv := serverWith(t, baseAllow, deps)
	srv.Apps.GitFetcher = errBlueprintFetcher{err: fmt.Errorf("%w: %w", github.ErrRepoNotFoundOrNoAccess, &github.APIError{Status: http.StatusNotFound})}
	hDeny, _ := serverWith(t, baseDeny, deps)

	for _, c := range []struct {
		h                  http.Handler
		name, method, path string
		body               string
		status             int
		code               string // "" when the body must carry none
	}{
		// EA Blueprints create / plan / approve
		{h, "plan unknown blueprint", http.MethodPost, "/v1/blueprints/exs-00000000000000000000/syncs", `{}`, http.StatusNotFound, ""},
		{h, "approve unknown blueprint", http.MethodPost, "/v1/blueprints/exs-00000000000000000000/approve", `{}`, http.StatusNotFound, ""},
		{hDeny, "create blueprint forbidden", http.MethodPost, "/v1/blueprints",
			`{"ownerId":"` + ws + `","name":"x","source":{"repo":"https://github.com/acme/app","branch":"main","path":"render.yaml"}}`,
			http.StatusForbidden, ""},
		{h, "create blueprint name taken", http.MethodPost, "/v1/blueprints",
			`{"ownerId":"` + ws + `","name":"taken-bp","source":{"repo":"https://github.com/acme/app","branch":"main","path":"render.yaml"}}`,
			http.StatusConflict, ""},
		{h, "create blueprint repo not found drops bex preview reason", http.MethodPost, "/v1/blueprints",
			`{"ownerId":"` + ws + `","name":"new-bp","source":{"repo":"https://github.com/acme/nope","branch":"main","path":"render.yaml"}}`,
			http.StatusBadRequest, ""},

		// Sandbox groups delete / set-default
		{h, "delete unknown sandbox group", http.MethodDelete, "/v1/sandbox-groups/sbg-00000000000000000000", "", http.StatusNotFound, ""},
		{h, "delete malformed sandbox group keeps Render code", http.MethodDelete, "/v1/sandbox-groups/nope", "", http.StatusBadRequest, sandbox.CodeInvalidSandboxGroupID},
		{h, "delete default sandbox group keeps Render code", http.MethodDelete, "/v1/sandbox-groups/" + defaultGroupID, "", http.StatusConflict, sandbox.CodeDefaultSandboxGroupNotDeletable},
		{h, "set-default unknown sandbox group", http.MethodPost, "/v1/sandbox-groups/sbg-00000000000000000000/set-default", "", http.StatusNotFound, ""},
		{h, "set-default malformed sandbox group keeps Render code", http.MethodPost, "/v1/sandbox-groups/nope/set-default", "", http.StatusBadRequest, sandbox.CodeInvalidSandboxGroupID},

		// EA Objects list / put / get / delete
		{h, "list objects unoffered region", http.MethodGet, "/v1/objects/" + ws + "/ohio", "", http.StatusBadRequest, ""},
		{h, "list objects bad cursor keeps Render code", http.MethodGet, "/v1/objects/" + ws + "/oregon?cursor=!!!", "", http.StatusBadRequest, sandbox.CodeInvalidCursor},
		{h, "put object missing sizeBytes", http.MethodPut, "/v1/objects/" + ws + "/oregon/k", `{}`, http.StatusBadRequest, ""},
		{h, "get missing object", http.MethodGet, "/v1/objects/" + ws + "/oregon/missing", "", http.StatusNotFound, ""},
		{h, "delete missing object", http.MethodDelete, "/v1/objects/" + ws + "/oregon/missing", "", http.StatusNotFound, ""},

		// Other Render CLI operations outside the pinned spec
		{h, "postgres parameter-overrides unknown db", http.MethodGet, "/v1/postgres/dpg-00000000000000000000/parameter-overrides", "", http.StatusNotFound, ""},
		{h, "ephemeral-shell unknown service", http.MethodPost, "/v1/services/srv-doesnotexist000000000/ephemeral-shell", "", http.StatusNotFound, ""},
		{h, "ephemeral-shell unsupported drops bex code", http.MethodPost, "/v1/services/web/ephemeral-shell", "", http.StatusGone, ""},
		{hDeny, "cli-telemetry-events forbidden", http.MethodPost, "/v1/cli-telemetry-events", `{"command":"services"}`, http.StatusForbidden, ""},

		// Controls: bex-native REST routes keep bex's own codes
		{h, "bex-native object-storage bucket keeps code", http.MethodGet, "/v1/object-storage/" + ws + "/buckets/missing", "", http.StatusNotFound, "BUCKET_NOT_FOUND"},
		{h, "bex-native git connections keeps code", http.MethodGet, "/v1/git/connections?ownerId=" + ws, "", http.StatusServiceUnavailable, "GITHUB_UNAVAILABLE"},
		{hDeny, "bex-native blueprints preview keeps code", http.MethodPost, "/v1/blueprints/preview", `{"ownerId":"` + ws + `","repo":"https://github.com/acme/app"}`, http.StatusForbidden, "FORBIDDEN"},
	} {
		t.Run(c.name, func(t *testing.T) {
			res := do(t, c.h, c.method, c.path, testToken, c.body)
			var body map[string]any
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatalf("%s %s answered %d %s: %v", c.method, c.path, res.Code, res.Body, err)
			}
			code, hasCode := body["code"]
			if res.Code != c.status || hasCode != (c.code != "") || (hasCode && code != c.code) {
				t.Fatalf("%s %s = %d %s, want %d with code %q", c.method, c.path, res.Code, res.Body, c.status, c.code)
			}
			if id, _ := body["id"].(string); id == "" || body["message"] == "" {
				t.Fatalf("%s %s = %s, want Render's id and message", c.method, c.path, res.Body)
			}
		})
	}
}

// TestRenderCLIOperationsOutsideSpecAreLiveAndDisjoint guards that every
// pattern in renderCLIOperationsOutsideSpec matches a real route registered on
// the REST mux (preventing silent drift when a path parameter is renamed) and
// is not already covered by the embedded render-public-api-1.json (so future
// OpenAPI re-pins prune entries that enter the spec).
func TestRenderCLIOperationsOutsideSpecAreLiveAndDisjoint(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	live := map[string]bool{}
	for _, pattern := range serveMuxPatterns(matrixServer(t).restHandler()) {
		live[pattern] = true
	}
	paramRe := regexp.MustCompile(`\{[^}]+\}`)
	for pattern := range renderCLIOperationsOutsideSpec {
		if !live[pattern] {
			t.Errorf("renderCLIOperationsOutsideSpec pattern %q is not registered on the REST mux", pattern)
			continue
		}
		method, path, ok := strings.Cut(pattern, " ")
		if !ok {
			t.Errorf("renderCLIOperationsOutsideSpec pattern %q must be \"METHOD /v1/...\"", pattern)
			continue
		}
		probePath := paramRe.ReplaceAllString(path, "fixture")
		req := httptest.NewRequest(method, probePath, nil)
		if _, _, _, inSpec := contract.renderOperation(pattern, req); inSpec {
			t.Errorf("renderCLIOperationsOutsideSpec pattern %q is now in render-public-api-1.json; remove the redundant entry", pattern)
		}
	}
}

// TestBandwidthSourcesTooManyResourcesCode pins w5/m583 t001: through the
// composed server (with withRenderErrorCodes and the OpenAPI request gate),
// GET /v1/metrics/bandwidth-sources allows 30 resources and refuses 31 with
// HTTP 400 and structured `"code": "too_many_resources"`.
func TestBandwidthSourcesTooManyResourcesCode(t *testing.T) {
	ws := ids.New(ids.Workspace)
	srvID := ids.New(ids.Service)
	app := &appv1alpha1.App{}
	app.Name, app.Namespace = core.CRName(ws, "web"), "default"
	app.Labels = map[string]string{core.LabelTenant: ws, core.LabelServiceName: "web", core.LabelAppID: srvID}
	app.Spec = appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Image: "nginx:1"}

	base := &core.Base{
		Client:    fakeClient(app),
		Namespace: "default",
		Authz:     &fakeChecker{allow: true},
		Workspace: fakeWorkspace{"client-1": ws},
	}
	h, _ := serverWith(t, base, Deps{
		RequestMetrics: func(context.Context, metrics.RequestMetricsRequest) ([]metrics.MetricSeries, error) {
			return nil, nil
		},
	})

	query := func(n int) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = "resource=" + srvID
		}
		return "/v1/metrics/bandwidth-sources?" + strings.Join(parts, "&")
	}

	okRes := do(t, h, http.MethodGet, query(30), testToken, "")
	if okRes.Code != http.StatusOK {
		t.Fatalf("30 resources = %d %s, want 200", okRes.Code, okRes.Body.String())
	}

	res := do(t, h, http.MethodGet, query(31), testToken, "")
	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("31 resources answered %d %s: %v", res.Code, res.Body.String(), err)
	}
	if res.Code != http.StatusBadRequest || body["code"] != metrics.TooManyResourcesCode {
		t.Fatalf("31 resources = %d %s, want 400 with code %q", res.Code, res.Body.String(), metrics.TooManyResourcesCode)
	}
}

// TestBuildSourceRegionMismatchAcrossSurfaces pins w5/m583 t002: linking a
// git-backed build source in a different declared Render region returns
// `build_source_region_mismatch` across REST (POST /v1/services and
// PATCH /v1/services/{id}), GraphQL (linkBuildSource), and MCP
// (link_service_to_build_source), while matching regions and image-backed
// build sources succeed.
func TestBuildSourceRegionMismatchAcrossSurfaces(t *testing.T) {
	ws := ids.New(ids.Workspace)
	base := &core.Base{
		Client:    fakeClient(),
		Namespace: "default",
		Authz:     &fakeChecker{allow: true},
		Workspace: fakeWorkspace{"client-1": ws},
	}
	h, _ := serverWith(t, base, Deps{Region: "fsn1", DashboardURL: "https://dashboard.bex.co"})

	createBS := func(body string) string {
		t.Helper()
		res := do(t, h, http.MethodPost, "/v1/build-sources", testToken, body)
		if res.Code != http.StatusCreated {
			t.Fatalf("POST /v1/build-sources = %d %s", res.Code, res.Body.String())
		}
		var out struct{ ID string }
		if err := json.Unmarshal(res.Body.Bytes(), &out); err != nil || out.ID == "" {
			t.Fatalf("POST /v1/build-sources body = %s", res.Body.String())
		}
		return out.ID
	}

	oregonGit := createBS(`{"ownerId":"` + ws + `","name":"oregon-git","git":{"runtime":"node","repoUrl":"https://github.com/acme/mono","branch":"main","buildCommand":"npm ci","region":"oregon"}}`)
	frankfurtGit := createBS(`{"ownerId":"` + ws + `","name":"fra-git","git":{"runtime":"node","repoUrl":"https://github.com/acme/mono","branch":"main","buildCommand":"npm ci","region":"frankfurt"}}`)
	imageSrc := createBS(`{"ownerId":"` + ws + `","name":"img-src","image":{"ownerId":"` + ws + `","imageUrl":"docker.io/library/nginx:1"}}`)

	assertRESTErrorCode := func(label string, res *httptest.ResponseRecorder, wantStatus int, wantCode string) {
		t.Helper()
		var body map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s body = %s: %v", label, res.Body.String(), err)
		}
		if res.Code != wantStatus || body["code"] != wantCode {
			t.Fatalf("%s = %d %s, want %d %q", label, res.Code, res.Body.String(), wantStatus, wantCode)
		}
	}

	// REST POST /v1/services: frankfurt (explicit and default from fsn1) + oregon git source → 400 build_source_region_mismatch.
	assertRESTErrorCode("POST /v1/services explicit mismatch",
		do(t, h, http.MethodPost, "/v1/services", testToken,
			`{"type":"web_service","name":"web-mismatch","ownerId":"`+ws+`","serviceDetails":{"region":"frankfurt","buildSourceId":"`+oregonGit+`","envSpecificDetails":{"startCommand":"node server.js"}}}`),
		http.StatusBadRequest, apps.BuildSourceRegionMismatchCode)
	assertRESTErrorCode("POST /v1/services default-region mismatch",
		do(t, h, http.MethodPost, "/v1/services", testToken,
			`{"type":"web_service","name":"web-default-mismatch","ownerId":"`+ws+`","serviceDetails":{"buildSourceId":"`+oregonGit+`","envSpecificDetails":{"startCommand":"node server.js"}}}`),
		http.StatusBadRequest, apps.BuildSourceRegionMismatchCode)

	// REST POST /v1/services: matching region and image-backed build source succeed.
	okCreate := do(t, h, http.MethodPost, "/v1/services", testToken,
		`{"type":"web_service","name":"web-oregon","ownerId":"`+ws+`","serviceDetails":{"region":"oregon","buildSourceId":"`+oregonGit+`","envSpecificDetails":{"startCommand":"node server.js"}}}`)
	if okCreate.Code != http.StatusCreated {
		t.Fatalf("POST /v1/services same region = %d %s, want 201", okCreate.Code, okCreate.Body.String())
	}
	imgCreate := do(t, h, http.MethodPost, "/v1/services", testToken,
		`{"type":"web_service","name":"web-img","ownerId":"`+ws+`","serviceDetails":{"region":"singapore","buildSourceId":"`+imageSrc+`"}}`)
	if imgCreate.Code != http.StatusCreated {
		t.Fatalf("POST /v1/services image build source = %d %s, want 201", imgCreate.Code, imgCreate.Body.String())
	}

	// Create a frankfurt service for PATCH / GraphQL / MCP / CreateBuildSource link checks.
	svcRes := do(t, h, http.MethodPost, "/v1/services", testToken,
		`{"type":"web_service","name":"web-fra","ownerId":"`+ws+`","repo":"https://github.com/acme/web","serviceDetails":{"region":"frankfurt","runtime":"node","envSpecificDetails":{"buildCommand":"npm ci","startCommand":"node server.js"}}}`)
	if svcRes.Code != http.StatusCreated {
		t.Fatalf("create frankfurt service = %d %s", svcRes.Code, svcRes.Body.String())
	}
	var created struct {
		Service struct{ ID string } `json:"service"`
	}
	if err := json.Unmarshal(svcRes.Body.Bytes(), &created); err != nil || created.Service.ID == "" {
		t.Fatalf("create frankfurt service body = %s", svcRes.Body.String())
	}
	svcID := created.Service.ID

	// REST POST /v1/build-sources with serviceIds in another region → 400 build_source_region_mismatch.
	assertRESTErrorCode("POST /v1/build-sources serviceIds mismatch",
		do(t, h, http.MethodPost, "/v1/build-sources", testToken,
			`{"ownerId":"`+ws+`","name":"ohio-git","git":{"runtime":"node","repoUrl":"https://github.com/acme/mono","branch":"main","buildCommand":"npm ci","region":"ohio"},"serviceIds":["`+svcID+`"]}`),
		http.StatusBadRequest, apps.BuildSourceRegionMismatchCode)

	// REST PATCH /v1/services/{id}: mismatched region → 400 build_source_region_mismatch.
	assertRESTErrorCode("PATCH /v1/services/{id} mismatch",
		do(t, h, http.MethodPatch, "/v1/services/"+svcID, testToken,
			`{"buildSourceId":"`+oregonGit+`","serviceDetails":{"envSpecificDetails":{"startCommand":"node server.js"}}}`),
		http.StatusBadRequest, apps.BuildSourceRegionMismatchCode)

	// GraphQL linkBuildSource: mismatched region → extensions.code == build_source_region_mismatch.
	gqlBody := `{"query":"mutation { linkBuildSource(serviceId:\"` + svcID + `\", buildSourceId:\"` + oregonGit + `\", startCommand:\"node server.js\") { id } }"}`
	gqlRes := do(t, h, http.MethodPost, "/graphql", testToken, gqlBody)
	var gqlOut struct {
		Errors []struct {
			Message    string         `json:"message"`
			Extensions map[string]any `json:"extensions"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(gqlRes.Body.Bytes(), &gqlOut); err != nil || len(gqlOut.Errors) == 0 {
		t.Fatalf("GraphQL linkBuildSource mismatch = %d %s, want error", gqlRes.Code, gqlRes.Body.String())
	}
	if got := gqlOut.Errors[0].Extensions["code"]; got != apps.BuildSourceRegionMismatchCode {
		t.Fatalf("GraphQL linkBuildSource extensions.code = %v, want %q (body: %s)", got, apps.BuildSourceRegionMismatchCode, gqlRes.Body.String())
	}

	// MCP link_service_to_build_source: mismatched region → tool error prefixed with build_source_region_mismatch.
	mcpPayload := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"link_service_to_build_source","arguments":{"serviceId":"` + svcID + `","buildSourceId":"` + oregonGit + `","startCommand":"node server.js"}}}`
	mcpReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(mcpPayload))
	mcpReq.Header.Set("Authorization", "Bearer "+testToken)
	mcpReq.Header.Set("Content-Type", "application/json")
	mcpReq.Header.Set("Accept", "application/json, text/event-stream")
	mcpRec := httptest.NewRecorder()
	h.ServeHTTP(mcpRec, mcpReq)
	if !strings.Contains(mcpRec.Body.String(), `"isError":true`) || !strings.Contains(mcpRec.Body.String(), apps.BuildSourceRegionMismatchCode+": ") {
		t.Fatalf("MCP link_service_to_build_source mismatch = %d %s, want isError:true with %q prefix", mcpRec.Code, mcpRec.Body.String(), apps.BuildSourceRegionMismatchCode)
	}

	// REST PATCH /v1/services/{id}: same region and image-backed build source succeed.
	if patchOK := do(t, h, http.MethodPatch, "/v1/services/"+svcID, testToken,
		`{"buildSourceId":"`+frankfurtGit+`","serviceDetails":{"envSpecificDetails":{"startCommand":"node server.js"}}}`); patchOK.Code != http.StatusOK {
		t.Fatalf("PATCH same region = %d %s, want 200", patchOK.Code, patchOK.Body.String())
	}
	if patchImg := do(t, h, http.MethodPatch, "/v1/services/"+svcID, testToken,
		`{"buildSourceId":"`+imageSrc+`"}`); patchImg.Code != http.StatusOK {
		t.Fatalf("PATCH image build source = %d %s, want 200", patchImg.Code, patchImg.Body.String())
	}
}

type emittedErrorCodeRef struct {
	file   string // path relative to lego/backend/internal/
	symbol string // constant or helper emitting it
}

// emittedRenderErrorCodes accounts for every pinned Render errorCode value
// that bex emits in production Go code. TestEveryPinnedRenderErrorCodeIsClassified
// verifies that each entry's exact string literal appears in the named non-test
// Go file (excluding comments) and that no pinned code goes unclassified.
var emittedRenderErrorCodes = map[string]emittedErrorCodeRef{
	"insufficient_token_scope":            {file: "core/errors.go", symbol: "core.InsufficientScopeCode"},
	"duplicate_saved_search_name":         {file: "projects/savedsearches.go", symbol: "projects.DuplicateSavedSearchNameCode"},
	"too_many_resources":                  {file: "metrics/bandwidth_sources.go", symbol: "metrics.TooManyResourcesCode"},
	"sandbox_not_running":                 {file: "sandbox/records.go", symbol: "sandbox.CodeSandboxNotRunning"},
	"snapshot_creating":                   {file: "sandbox/snapshots.go", symbol: "sandbox.CodeSnapshotCreating"},
	"snapshot_not_available":              {file: "sandbox/snapshots.go", symbol: "sandbox.CodeSnapshotNotAvailable"},
	"snapshot_not_found":                  {file: "sandbox/snapshots.go", symbol: "sandbox.CodeSnapshotNotFound"},
	"build_not_found":                     {file: "sandbox/images.go", symbol: "sandbox.CodeBuildNotFound / deploys.buildNotFoundCode"},
	"build_not_succeeded":                 {file: "sandbox/images.go", symbol: "sandbox.CodeBuildNotSucceeded / deploys.buildNotSucceededCode"},
	"invalid_sandbox_group_id":            {file: "sandbox/groups.go", symbol: "sandbox.CodeInvalidSandboxGroupID"},
	"invalid_snapshot_id":                 {file: "sandbox/snapshots.go", symbol: "sandbox.CodeInvalidSnapshotID"},
	"invalid_owner_id":                    {file: "sandbox/groups.go", symbol: "sandbox.CodeInvalidOwnerID"},
	"invalid_status":                      {file: "sandbox/images.go", symbol: "sandbox.CodeInvalidBuildStatus / sandbox.CodeInvalidSnapshotStatus"},
	"invalid_cursor":                      {file: "sandbox/groups.go", symbol: "sandbox.CodeInvalidCursor"},
	"invalid_limit":                       {file: "sandbox/groups.go", symbol: "sandbox.CodeInvalidLimit"},
	"invalid_snapshot_name":               {file: "sandbox/snapshots.go", symbol: "sandbox.CodeInvalidSnapshotName"},
	"invalid_sandbox_name":                {file: "sandbox/service.go", symbol: "sandbox.CodeInvalidSandboxName"},
	"sandbox_group_limit_reached":         {file: "sandbox/groups.go", symbol: "sandbox.CodeSandboxGroupLimitReached"},
	"default_sandbox_group_not_deletable": {file: "sandbox/groups.go", symbol: "sandbox.CodeDefaultSandboxGroupNotDeletable"},
	"invalid_sandbox_plan":                {file: "sandbox/service.go", symbol: "sandbox.CodeInvalidSandboxPlan"},
	"invalid_timeout_seconds":             {file: "sandbox/service.go", symbol: "sandbox.CodeInvalidTimeoutSeconds"},
	"build_source_region_mismatch":        {file: "apps/build_sources.go", symbol: "apps.BuildSourceRegionMismatchCode"},
	"image_in_use":                        {file: "sandbox/images.go", symbol: "sandbox.CodeImageInUse"},
}

// notApplicableRenderErrorCodes accounts for pinned Render errorCode values
// that have no enforced bex counterpart today, with the exact rationale.
var notApplicableRenderErrorCodes = map[string]string{
	"multiple_regions":              "single-region Prometheus installation (BEX_REGION); multi-resource metrics queries succeed directly without cross-region fan-out restrictions, and no pinned OpenAPI path documents this code",
	"snapshot_plan_mismatch":        "applies only to runtime (memory+CPU) sandbox snapshots; bex currently serves filesystem snapshots only (runtime snapshots tracked in w2/m322)",
	"sandbox_group_suspended":       "bex has no suspended sandbox-group state (ADR104 D7, docs/render-artifacts/sandbox-groups.md)",
	"preauth_consent_required":      "Render card spend-preauthorization billing flow; bex uses Stripe SetupIntent + metered billing (ADR040)",
	"preauth_declined":              "Render card spend-preauthorization billing flow; bex uses Stripe SetupIntent + metered billing (ADR040)",
	"preauth_attempt_spent":         "Render card spend-preauthorization billing flow; bex uses Stripe SetupIntent + metered billing (ADR040)",
	"preauth_no_payment_method":     "Render card spend-preauthorization billing flow; bex enforces payment methods via Stripe SetupIntent / PAYMENT_REQUIRED (ADR040)",
	"preauth_unavailable":           "Render card spend-preauthorization billing flow; bex uses Stripe SetupIntent + metered billing (ADR040)",
	"cursor_origin_receipt_expired": "Render Cursor Origin partner-installation receipt flow; bex does not offer Cursor Origin integration",
	"cursor_origin_receipt_invalid": "Render Cursor Origin partner-installation receipt flow; bex does not offer Cursor Origin integration",
}

// TestEveryPinnedRenderErrorCodeIsClassified guards all values of Render's
// pinned components.schemas.errorCode enum (33 values at the current pin,
// w5/m583 t003). Future OpenAPI re-pins cannot silently add an unhandled code,
// and moving an N/A code to an emitted one is caught automatically.
func TestEveryPinnedRenderErrorCodeIsClassified(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}

	pinned := make([]string, 0, len(contract.errorCodes))
	for code := range contract.errorCodes {
		pinned = append(pinned, code)
	}
	sort.Strings(pinned)

	for _, code := range pinned {
		_, inEmitted := emittedRenderErrorCodes[code]
		_, inNA := notApplicableRenderErrorCodes[code]
		switch {
		case inEmitted && inNA:
			t.Errorf("pinned errorCode %q is listed in both emittedRenderErrorCodes and notApplicableRenderErrorCodes", code)
		case !inEmitted && !inNA:
			t.Errorf("pinned errorCode %q is unclassified — emit it in production code and add to emittedRenderErrorCodes, or document why it is N/A in notApplicableRenderErrorCodes", code)
		}
	}

	for code := range emittedRenderErrorCodes {
		if !contract.errorCodes[code] {
			t.Errorf("emittedRenderErrorCodes contains stale code %q not present in pinned components.schemas.errorCode", code)
		}
	}
	for code, reason := range notApplicableRenderErrorCodes {
		if !contract.errorCodes[code] {
			t.Errorf("notApplicableRenderErrorCodes contains stale code %q not present in pinned components.schemas.errorCode", code)
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("notApplicableRenderErrorCodes[%q] must state a non-empty reason", code)
		}
	}

	internalDir := filepath.Clean("..")
	literalsByFile := collectProductionStringLiterals(t, internalDir)

	for code, ref := range emittedRenderErrorCodes {
		if ref.file == "" || ref.symbol == "" {
			t.Errorf("emittedRenderErrorCodes[%q] must name both file and symbol", code)
			continue
		}
		if !literalsByFile[filepath.ToSlash(ref.file)][code] {
			t.Errorf("emittedRenderErrorCodes[%q] claims %s (%s), but no exact string literal %q was found in that non-test Go file", code, ref.file, ref.symbol, code)
		}
	}

	for code := range notApplicableRenderErrorCodes {
		for file, lits := range literalsByFile {
			if lits[code] {
				t.Errorf("notApplicableRenderErrorCodes[%q] now appears as a string literal in %s — move it to emittedRenderErrorCodes", code, file)
			}
		}
	}
}

// collectProductionStringLiterals parses every non-test .go file under root
// and indexes its unquoted string literals by slash-separated path relative to
// root.
func collectProductionStringLiterals(t *testing.T, root string) map[string]map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	out := map[string]map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		lits := map[string]bool{}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if s, unquoteErr := strconv.Unquote(lit.Value); unquoteErr == nil {
				lits[s] = true
			}
			return true
		})
		out[filepath.ToSlash(rel)] = lits
		return nil
	})
	if err != nil {
		t.Fatalf("scan production Go files under %s: %v", root, err)
	}
	return out
}
