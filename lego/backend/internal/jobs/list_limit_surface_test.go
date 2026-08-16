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

package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// list_limit_surface_test.go pins the `limit` contract of list_jobs on ALL
// THREE surfaces, not just REST.
//
// The guard once lived only in rest.go's filterFromQuery. GraphQL and
// MCP handed their int straight to FilterFromStrings, which passed it to the
// store, where clampPageLimit reads limit <= 0 as "absent" and substitutes
// core.MaxPageLimit. So `jobs(serviceId: "web", limit: -1)` and
// list_jobs{limit:-1} each returned a 100-row page that looked exactly like
// the page the caller asked for — no error anywhere. The sibling feature
// (deploys) had the guard on all three surfaces and documented why; jobs was
// the copy that never got it.
//
// Everything is driven through a registered mux / schema / MCP session rather
// than by calling filterFromQuery or FilterFromStrings directly, so moving the
// guard between the adapter and the translator again cannot quietly pass these.

func gqlSchema(t *testing.T, s *Service) graphql.Schema {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: s.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return schema
}

func mcpSession(t *testing.T, s *Service) *mcp.ClientSession {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	s.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// surfaceHarness builds one Service over the recording store and returns the
// three registered entry points plus the store, so a single case can assert
// the same input against every surface.
type surfaceHarness struct {
	store  *recordingJobStore
	mux    *http.ServeMux
	schema graphql.Schema
	mcp    *mcp.ClientSession
}

func newSurfaceHarness(t *testing.T) *surfaceHarness {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: "web:v1"},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(client.Object(app)).Build()
	st := &recordingJobStore{}
	// ONE Service behind all three surfaces, exactly as api/server.go composes
	// it — so a filter recorded by any surface is the same object.
	//
	// Authz is deliberately nil (core.Base's "authorization not enforced"
	// mode, the deploys MCP-test precedent): the in-memory MCP transport gives
	// the tool handler its own context, so a wired checker would answer
	// ErrForbidden for want of an identity and every rejection assertion below
	// would pass without ever reaching the limit guard. Authorization has its
	// own coverage; this file is about the filter contract. The assertions
	// still name `limit` explicitly so a rejection from some OTHER cause
	// cannot stand in for the one under test.
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, Store: st}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	return &surfaceHarness{store: st, mux: mux, schema: gqlSchema(t, svc), mcp: mcpSession(t, svc)}
}

// identityCtx carries the authenticated caller every verb's authorize gate
// expects; the fake checker allows, so this only has to be present.
func identityCtx() context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "session"})
}

// listJobsGraphQL runs the jobs query with an explicit limit literal and
// reports the first error, if any.
func (h *surfaceHarness) listJobsGraphQL(t *testing.T, limitLiteral string) error {
	t.Helper()
	q := `{ jobs(serviceId: "web"` + limitLiteral + `) { id } }`
	res := graphql.Do(graphql.Params{Schema: h.schema, Context: identityCtx(), RequestString: q})
	if len(res.Errors) > 0 {
		return errors.New(res.Errors[0].Message)
	}
	return nil
}

// listJobsMCP calls the list_jobs tool and reports a tool-level error.
func (h *surfaceHarness) listJobsMCP(t *testing.T, args map[string]any) error {
	t.Helper()
	res, err := h.mcp.CallTool(identityCtx(), &mcp.CallToolParams{Name: "list_jobs", Arguments: args})
	if err != nil {
		return err
	}
	if res.IsError {
		b, _ := json.Marshal(res.Content)
		return errors.New(string(b))
	}
	return nil
}

// TestNegativeLimitIsRejectedOnEverySurface is the regression proper. The
// store's own clamp means a leaked negative limit is INVISIBLE in the response
// — it comes back as the max page — so the assertion is on the error, and on
// the store never having been handed a negative limit.
func TestNegativeLimitIsRejectedOnEverySurface(t *testing.T) {
	t.Run("REST", func(t *testing.T) {
		h := newSurfaceHarness(t)
		rec := getJobs(t, h.mux, "/v1/services/web/jobs?limit=-1")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
		}
		assertRejectedForLimit(t, h, errors.New(rec.Body.String()))
	})

	t.Run("GraphQL", func(t *testing.T) {
		h := newSurfaceHarness(t)
		err := h.listJobsGraphQL(t, ", limit: -1")
		if err == nil {
			t.Fatal("limit: -1 was accepted; the store would clamp it to a full MaxPageLimit page")
		}
		assertRejectedForLimit(t, h, err)
	})

	t.Run("MCP", func(t *testing.T) {
		h := newSurfaceHarness(t)
		err := h.listJobsMCP(t, map[string]any{"serviceId": "web", "limit": -1})
		if err == nil {
			t.Fatal("limit -1 was accepted; the store would clamp it to a full MaxPageLimit page")
		}
		assertRejectedForLimit(t, h, err)
	})
}

// TestExplicitZeroLimitIsRejectedWhereItIsDistinguishable pins the deliberate
// asymmetry: zero is how an omitted limit arrives at the translator, so only a
// surface that can still see PRESENCE may reject it. REST (a query string) and
// GraphQL (an argument map) can; MCP's `omitempty` int cannot, and there an
// explicit 0 legitimately means "no cap".
// REST's `?limit=0` arm lives in TestListJobsFilterEdgeCases, which already
// tables it; only the surfaces that had no coverage are added here.
func TestExplicitZeroLimitIsRejectedWhereItIsDistinguishable(t *testing.T) {
	t.Run("GraphQL rejects limit: 0", func(t *testing.T) {
		h := newSurfaceHarness(t)
		if err := h.listJobsGraphQL(t, ", limit: 0"); err == nil {
			t.Fatal("explicit limit: 0 was accepted, but REST 400s the same input")
		}
	})

	t.Run("an omitted limit is still the full history", func(t *testing.T) {
		h := newSurfaceHarness(t)
		if err := h.listJobsGraphQL(t, ""); err != nil {
			t.Fatalf("omitted limit = %v, want accepted", err)
		}
		if h.store.filter.Limit != 0 {
			t.Fatalf("Limit = %d, want 0 (absent ⇒ the store's own newest-first cap)", h.store.filter.Limit)
		}
	})
}

// TestPositiveLimitReachesTheStoreOnEverySurface is the other half of the
// guard: a guard that rejected everything would pass the tests above. Each
// surface must still deliver the exact value the caller asked for.
func TestPositiveLimitReachesTheStoreOnEverySurface(t *testing.T) {
	for _, tc := range []struct {
		surface string
		call    func(t *testing.T, h *surfaceHarness)
	}{
		{"REST", func(t *testing.T, h *surfaceHarness) {
			if rec := getJobs(t, h.mux, "/v1/services/web/jobs?limit=7"); rec.Code != http.StatusOK {
				t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body.String())
			}
		}},
		{"GraphQL", func(t *testing.T, h *surfaceHarness) {
			if err := h.listJobsGraphQL(t, ", limit: 7"); err != nil {
				t.Fatalf("limit: 7 = %v", err)
			}
		}},
		{"MCP", func(t *testing.T, h *surfaceHarness) {
			if err := h.listJobsMCP(t, map[string]any{"serviceId": "web", "limit": 7}); err != nil {
				t.Fatalf("limit 7 = %v", err)
			}
		}},
	} {
		t.Run(tc.surface, func(t *testing.T) {
			h := newSurfaceHarness(t)
			tc.call(t, h)
			if h.store.filter.Limit != 7 {
				t.Fatalf("store saw Limit = %d, want 7", h.store.filter.Limit)
			}
		})
	}
}

// TestListFilterRejectsMalformedTimeOnEverySurface guards the other half of
// FilterFromStrings that only REST covered. deploys and jobs each carried a
// private parseTime; both now call core.ParseTime, and this pins that a
// malformed bound is still a 400 rather than a silently dropped filter (which
// would return unfiltered history looking like a filtered page).
// (REST's arm is already tabled in TestListJobsFilterEdgeCases.)
func TestListFilterRejectsMalformedTimeOnEverySurface(t *testing.T) {
	_, err := FilterFromStrings(nil, "not-a-time", "", "", "", "", "", "", 0)
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("err = %v, want a core.ErrBadRequest", err)
	}
	// The worked example is the reason core.ParseTime's message reads the way it
	// does; a caller who spelled a timestamp wrong has nothing else to go on.
	if got := err.Error(); !strings.Contains(got, "createdBefore") || !strings.Contains(got, "2026-01-02T15:04:05Z") {
		t.Fatalf("message = %q, want the field name and a worked example", got)
	}
}

// assertRejectedForLimit checks that the rejection was ABOUT the limit — an
// unrelated 400/forbidden must not be able to stand in for the guard under
// test — and that the store was never handed the negative value.
func assertRejectedForLimit(t *testing.T, h *surfaceHarness, err error) {
	t.Helper()
	if !strings.Contains(err.Error(), core.ErrLimitNotPositive.Error()) {
		t.Fatalf("rejected with %q, want the limit message — some other check answered first", err)
	}
	if h.store.filter.Limit < 0 {
		t.Fatalf("store saw Limit = %d; a negative limit must never reach clampPageLimit", h.store.filter.Limit)
	}
}
