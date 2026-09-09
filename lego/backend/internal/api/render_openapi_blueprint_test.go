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
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// blueprintFixtureStore is the smallest BlueprintStore that can answer the four
// by-id routes honestly: it scopes every read by tenant, exactly as the real
// Postgres store's queries do, so "belongs to another workspace" and "does not
// exist" reach the handler as the same not-found the real store produces.
type blueprintFixtureStore struct {
	rows  map[string]store.Blueprint
	syncs []store.BlueprintSync
}

func (f *blueprintFixtureStore) GetBlueprint(_ context.Context, id, tenantID string) (store.Blueprint, error) {
	b, ok := f.rows[id]
	if !ok || b.TenantID != tenantID {
		return store.Blueprint{}, fmt.Errorf("blueprint: %w", store.ErrNotFound)
	}
	return b, nil
}

func (f *blueprintFixtureStore) UpdateBlueprint(_ context.Context, id, tenantID string, name *string, autoSync *bool, path *string, _ *string, _ *time.Time) (store.Blueprint, error) {
	b, ok := f.rows[id]
	if !ok || b.TenantID != tenantID {
		return store.Blueprint{}, fmt.Errorf("blueprint: %w", store.ErrNotFound)
	}
	if name != nil {
		b.Name = *name
	}
	if autoSync != nil {
		b.AutoSync = *autoSync
	}
	if path != nil {
		b.Path = *path
	}
	f.rows[id] = b
	return b, nil
}

func (f *blueprintFixtureStore) DisconnectBlueprint(_ context.Context, id, tenantID string) error {
	b, ok := f.rows[id]
	if !ok || b.TenantID != tenantID {
		return fmt.Errorf("blueprint: %w", store.ErrNotFound)
	}
	delete(f.rows, id)
	return nil
}

func (f *blueprintFixtureStore) ListBlueprintSyncs(_ context.Context, blueprintID, _ string, _ int) ([]store.BlueprintSync, error) {
	var out []store.BlueprintSync
	for _, s := range f.syncs {
		if s.BlueprintID == blueprintID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *blueprintFixtureStore) UpsertBlueprint(_ context.Context, b store.Blueprint) (store.Blueprint, error) {
	return b, nil
}

func (f *blueprintFixtureStore) GetBlueprintByRepo(context.Context, string, string, string) (store.Blueprint, error) {
	return store.Blueprint{}, fmt.Errorf("blueprint: %w", store.ErrNotFound)
}

func (f *blueprintFixtureStore) ListBlueprints(context.Context, string) ([]store.Blueprint, error) {
	return nil, nil
}

func (f *blueprintFixtureStore) InsertBlueprintSync(_ context.Context, run store.BlueprintSync) (store.BlueprintSync, error) {
	return run, nil
}

func (f *blueprintFixtureStore) UpdateBlueprintSync(_ context.Context, id, state string, _ *time.Time, _ *string) (store.BlueprintSync, error) {
	return store.BlueprintSync{ID: id, State: state}, nil
}

// Lifecycle stubs (w8/m37): the by-id route tests never admit, stage,
// complete, or recover runs, so these stay unimplemented.
func (f *blueprintFixtureStore) AdmitBlueprintSyncRun(context.Context, string, string, store.BlueprintSync) (store.Blueprint, store.BlueprintSync, error) {
	return store.Blueprint{}, store.BlueprintSync{}, fmt.Errorf("blueprintFixtureStore: %w", store.ErrNotFound)
}

func (f *blueprintFixtureStore) AdmitBlueprintCreate(context.Context, store.Blueprint, store.BlueprintSync) (store.Blueprint, store.BlueprintSync, error) {
	return store.Blueprint{}, store.BlueprintSync{}, fmt.Errorf("blueprintFixtureStore: %w", store.ErrNotFound)
}

func (f *blueprintFixtureStore) StageBlueprintManifest(context.Context, string, string, int64, string, string) (store.Blueprint, error) {
	return store.Blueprint{}, fmt.Errorf("blueprintFixtureStore: %w", store.ErrNotFound)
}

func (f *blueprintFixtureStore) CompleteBlueprintSync(context.Context, string, string, string, int64, string, time.Time, *string) (store.Blueprint, error) {
	return store.Blueprint{}, fmt.Errorf("blueprintFixtureStore: %w", store.ErrNotFound)
}

func (f *blueprintFixtureStore) FailAdmittedSync(context.Context, string, string, string, int64, time.Time, *string) error {
	return fmt.Errorf("blueprintFixtureStore: %w", store.ErrNotFound)
}

func (f *blueprintFixtureStore) ListAbandonedBlueprintSyncs(context.Context, time.Time, int) ([]store.AbandonedBlueprintSync, error) {
	return nil, nil
}

func (f *blueprintFixtureStore) AbandonBlueprintSync(context.Context, string, time.Time, string) (bool, error) {
	return false, nil
}

// blueprintRESTHandler composes what the milestone's coverage gap was missing:
// the REAL apps.Service REST routes behind the REAL Render OpenAPI validator.
// The existing blueprint REST test builds a bare http.NewServeMux() and calls
// RegisterREST on it directly, so it never crosses this gate — which is why a
// validator that 400'd every one of these routes shipped as "done" under w2/m62
// and stayed green in CI.
func blueprintRESTHandler(t *testing.T, fixture *blueprintFixtureStore) http.Handler {
	t.Helper()
	svc := &apps.Service{
		Base: &core.Base{
			Client:    fakeClient(),
			Namespace: "default",
			Workspace: twoWorkspaceResolver{},
			Authz:     &fakeChecker{allow: true},
		},
		Blueprints: fixture,
	}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	h, err := newRenderRequestValidator(mux)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func blueprintRequest(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "user-a", Method: "oauth2"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestBlueprintIDRoutesPassTheRenderOpenAPIGate is w6/m96's regression. Every
// REST call to a blueprint-id route used to be rejected by the Render
// compatibility validator — before authz, before lookup — with
// `400 {"error":"invalid path parameter \"blueprintId\""}`, because Render's
// spec pins blueprintId to its own historical ^exs-… shape and bex mints blp-.
// No bex id has ever matched exs- and none can, so all four operations were
// unreachable to any REST-only client.
func TestBlueprintIDRoutesPassTheRenderOpenAPIGate(t *testing.T) {
	mine := ids.New(ids.Blueprint)
	theirs := ids.New(ids.Blueprint)
	fixture := &blueprintFixtureStore{
		rows: map[string]store.Blueprint{
			mine:   {ID: mine, TenantID: "tea-a", Name: "app", Repo: "https://github.com/a/app", Branch: "main", Status: "active"},
			theirs: {ID: theirs, TenantID: "tea-zzz", Name: "other", Repo: "https://github.com/z/other", Branch: "main", Status: "active"},
		},
		syncs: []store.BlueprintSync{{ID: ids.New(ids.BlueprintSync), BlueprintID: mine, State: store.BlueprintSyncStateSuccess}},
	}
	h := blueprintRESTHandler(t, fixture)

	t.Run("GET by id returns the blueprint, not a path-parameter 400", func(t *testing.T) {
		rec := blueprintRequest(t, h, http.MethodGet, "/v1/blueprints/"+mine+"?ownerId=tea-a", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /v1/blueprints/%s = %d %s, want 200", mine, rec.Code, rec.Body)
		}
		var view apps.BlueprintView
		if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if view.ID != mine || view.Name != "app" {
			t.Errorf("body = %+v, want the real blueprint row — a 200 with the wrong body is not a fix", view)
		}
	})

	t.Run("GET syncs returns the run history", func(t *testing.T) {
		rec := blueprintRequest(t, h, http.MethodGet, "/v1/blueprints/"+mine+"/syncs?ownerId=tea-a", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET syncs = %d %s, want 200", rec.Code, rec.Body)
		}
		var out []apps.BlueprintSyncView
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(out) != 1 {
			t.Errorf("syncs = %+v, want the one run recorded for this blueprint", out)
		}
	})

	t.Run("PATCH by id applies the change", func(t *testing.T) {
		rec := blueprintRequest(t, h, http.MethodPatch, "/v1/blueprints/"+mine, `{"name":"renamed","ownerId":"tea-a"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH = %d %s, want 200", rec.Code, rec.Body)
		}
		if got := fixture.rows[mine].Name; got != "renamed" {
			t.Errorf("stored name = %q, want the PATCH to have reached the store", got)
		}
	})

	t.Run("DELETE by id disconnects", func(t *testing.T) {
		gone := ids.New(ids.Blueprint)
		fixture.rows[gone] = store.Blueprint{ID: gone, TenantID: "tea-a", Name: "temp"}
		rec := blueprintRequest(t, h, http.MethodDelete, "/v1/blueprints/"+gone+"?ownerId=tea-a", "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("DELETE = %d %s, want 204", rec.Code, rec.Body)
		}
		if _, still := fixture.rows[gone]; still {
			t.Error("DELETE returned 204 without reaching the store")
		}
	})

	// The three outcomes that used to be one indistinguishable 400. Each asserts
	// its exact status, not merely "not 400" — a regression to a different wrong
	// status must not pass as a fix.
	t.Run("a well-formed id that does not exist is 404, not 400", func(t *testing.T) {
		absent := ids.New(ids.Blueprint)
		rec := blueprintRequest(t, h, http.MethodGet, "/v1/blueprints/"+absent+"?ownerId=tea-a", "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET absent id = %d %s, want 404", rec.Code, rec.Body)
		}
	})

	t.Run("a blueprint in another workspace is 404, not 400", func(t *testing.T) {
		// The caller is a member of tea-a and asks as tea-a; the row belongs to
		// tea-zzz, so the tenant-scoped store read simply does not find it.
		rec := blueprintRequest(t, h, http.MethodGet, "/v1/blueprints/"+theirs+"?ownerId=tea-a", "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET another workspace's blueprint = %d %s, want 404", rec.Code, rec.Body)
		}
	})

	t.Run("a workspace the caller is not a member of is 403, not 400", func(t *testing.T) {
		// Asking AS tea-zzz is refused by Authorize before any store read —
		// a different id AND a different caller-workspace from the case above,
		// so the two are genuinely distinct scenarios rather than one asserted twice.
		rec := blueprintRequest(t, h, http.MethodGet, "/v1/blueprints/"+theirs+"?ownerId=tea-zzz", "")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("GET as a non-member workspace = %d %s, want 403", rec.Code, rec.Body)
		}
	})

	// The widened pattern must still be a pattern: garbage is rejected at the
	// gate exactly as before, so this fix loosened the id shape rather than
	// disabling path-parameter validation for these routes.
	t.Run("garbage in the id position is still rejected by the validator", func(t *testing.T) {
		for _, bad := range []string{"not-an-id", "blp-TOOSHORT", "srv-" + strings.Repeat("a", 20)} {
			rec := blueprintRequest(t, h, http.MethodGet, "/v1/blueprints/"+bad+"?ownerId=tea-a", "")
			if rec.Code != http.StatusBadRequest {
				t.Errorf("GET /v1/blueprints/%s = %d, want 400 — the parameter must still be validated", bad, rec.Code)
				continue
			}
			// Asserted on the reason, not just the status: these requests also
			// carry ownerId, so a regression that re-broke the query extension
			// would produce a 400 for an unrelated cause and pass a bare
			// status check.
			if !strings.Contains(rec.Body.String(), "invalid path parameter") {
				t.Errorf("GET /v1/blueprints/%s rejected with %s, want the path-parameter reason", bad, rec.Body)
			}
		}
	})

	// Render's own documented id shape keeps working: this layer only ever
	// widens, so a client holding a Render-shaped id gets an honest not-found
	// from the store rather than a syntax error about Render's own spelling.
	t.Run("Render's own exs- shape still reaches the store and 404s", func(t *testing.T) {
		rec := blueprintRequest(t, h, http.MethodGet, "/v1/blueprints/exs-abcdefghij0123456789?ownerId=tea-a", "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET an exs- id = %d %s, want 404 (not 400: the compat layer must not narrow Render's contract)", rec.Code, rec.Body)
		}
	})
}

// renderIDPathParameters maps every pattern-constrained id path parameter in
// Render's spec to the bex id.Kind prefix that addresses it. It is deliberately
// hand-maintained and completeness-checked below: a Render spec refresh (or a
// bex route newly intersecting one of Render's) that introduces an id parameter
// nobody has classified fails CI rather than shipping a silently unreachable
// endpoint, which is exactly how blueprintId went unnoticed.
var renderIDPathParameters = map[string]ids.Kind{
	"diskId":      ids.Disk,
	"webhookId":   ids.Webhook,
	"eventId":     ids.Event,
	"jobId":       ids.Job,
	"blueprintId": ids.Blueprint,
}

// TestEveryConstrainedIDPathParameterAcceptsItsBexID is the guard that turns
// "blueprint was the only mismatch" from a one-time grep into a checked fact.
// For every pattern-constrained id path parameter reachable through a bex REST
// route, a REAL minted bex id of the corresponding kind must satisfy the
// (post-compatibility) pattern the validator will apply.
func TestEveryConstrainedIDPathParameterAcceptsItsBexID(t *testing.T) {
	contract, err := renderContractOnce()
	if err != nil {
		t.Fatal(err)
	}
	// The REAL REST mux, exactly as TestRenderRouteIntersectionInventory builds
	// it — a hand-listed route set would drift from what bex actually serves,
	// which is how this class of gap goes unnoticed in the first place.
	mux := NewServer(&core.Base{}, Deps{}).restHandler()

	found := map[string]string{}
	placeholder := regexp.MustCompile(`\{[^}]+\}`)
	for path, item := range contract.document.Paths.Map() {
		for method, operation := range item.Operations() {
			probe := httptest.NewRequest(strings.ToUpper(method), "/v1"+placeholder.ReplaceAllString(path, "fixture"), nil)
			if _, pattern := mux.Handler(probe); pattern == "" {
				continue // Render-only operation; bex serves no route here.
			}
			for _, params := range []openapi3.Parameters{item.Parameters, operation.Parameters} {
				for _, ref := range params {
					if ref == nil || ref.Value == nil || ref.Value.In != openapi3.ParameterInPath {
						continue
					}
					if ref.Value.Schema == nil || ref.Value.Schema.Value == nil || ref.Value.Schema.Value.Pattern == "" {
						continue
					}
					found[ref.Value.Name] = ref.Value.Schema.Value.Pattern
				}
			}
		}
	}

	for name, pattern := range found {
		kind, classified := renderIDPathParameters[name]
		if !classified {
			t.Errorf("path parameter %q is pattern-constrained (%s) on a bex-served route but is not classified in "+
				"renderIDPathParameters — add it with the id.Kind prefix that addresses it, or the route is "+
				"unreachable to REST clients the way blueprintId was (w6/m96)", name, pattern)
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			t.Errorf("parameter %q pattern %q does not compile: %v", name, pattern, err)
			continue
		}
		// Real minted ids, not hand-built strings: this asserts what bex
		// actually issues clears the gate, including its xid alphabet and
		// length. Several, because xid's suffix varies per call.
		for range 8 {
			minted := ids.New(kind)
			if !re.MatchString(minted) {
				t.Errorf("bex %s id %q does not satisfy Render's %q pattern %q — every REST call to this route 400s "+
					"at the OpenAPI gate before authz or lookup runs", kind.Prefix(), minted, name, pattern)
				break
			}
		}
	}
	for name := range renderIDPathParameters {
		if _, ok := found[name]; !ok {
			t.Errorf("renderIDPathParameters classifies %q, but no bex-served operation constrains it any more — "+
				"drop the entry so this map keeps meaning what it says", name)
		}
	}
}

// TestRenderPathParameterOverridesOnlyWiden pins the invariant that makes this
// file's compatibility layer safe to extend: every knob here RELAXES Render's
// contract and none narrows it. An override that merely swapped Render's shape
// for bex's would fix bex clients by breaking Render-shaped ones, and nothing
// else in the file would have caught it.
func TestRenderPathParameterOverridesOnlyWiden(t *testing.T) {
	pristine, err := openapi3.NewLoader().LoadFromData(renderOpenAPISource)
	if err != nil {
		t.Fatal(err)
	}
	for operationID, overrides := range renderPathParameterPatternCompatibility {
		item, operation := findRenderOperation(t, pristine, operationID)
		for name, replacement := range overrides {
			original := ""
			for _, params := range []openapi3.Parameters{item.Parameters, operation.Parameters} {
				for _, ref := range params {
					if ref != nil && ref.Value != nil && ref.Value.Name == name &&
						ref.Value.Schema != nil && ref.Value.Schema.Value != nil {
						original = ref.Value.Schema.Value.Pattern
					}
				}
			}
			if original == "" {
				t.Errorf("%s: parameter %q has no upstream pattern — the override is dead weight", operationID, name)
				continue
			}
			widened, err := regexp.Compile(replacement)
			if err != nil {
				t.Fatalf("%s: replacement pattern %q does not compile: %v", operationID, replacement, err)
			}
			// Anything the upstream pattern accepted, the replacement must too.
			// Sampled over the shape Render's own patterns describe rather than
			// proven symbolically — enough to catch a swap, which is the failure
			// this guards.
			for _, sample := range renderPatternSamples(original) {
				if !widened.MatchString(sample) {
					t.Errorf("%s: override for %q rejects %q, which Render's own pattern %q accepts — "+
						"this layer may only widen", operationID, name, sample, original)
				}
			}
		}
	}
}

// renderPatternSamples builds ids that satisfy a Render `^<prefix>-<class>{20}$`
// pattern, so a widening check has concrete strings to test with.
func renderPatternSamples(pattern string) []string {
	prefix, _, ok := strings.Cut(strings.TrimPrefix(pattern, "^"), "-")
	if !ok {
		return nil
	}
	return []string{
		prefix + "-" + strings.Repeat("a", 20),
		prefix + "-" + strings.Repeat("0", 20),
		prefix + "-" + "0123456789abcdefghij",
	}
}
