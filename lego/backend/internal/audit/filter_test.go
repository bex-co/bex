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

package audit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// filter_test.go pins the audit-log list filter on BOTH surfaces.
//
// REST and GraphQL each parsed the time bounds inline with the permissive
// `if t, err := time.Parse(...); err == nil { assign }` shape, so a bound the
// caller spelled wrong was dropped on the floor and the query ran with NO TIME
// FILTER — `?startTime=yesterday` answered with the store's default page of the
// caller's whole history instead of the window asked for. The row count was
// never unbounded (the store substitutes store.DefaultAuditPageSize for a zero
// limit); what was lost is the filter, which is worse to miss precisely because
// the response looks exactly like a correct one. Nothing surfaced it: no error,
// a plausible page, and the two copies were identical so neither could
// contradict the other.
//
// The same struct's Direction field had carried the opposite rule since w4/013
// ("nothing accepted is ignored"), and every other caller-supplied time filter
// in the module already went through core.ParseTime. Audit was the outlier.
//
// Limit is deliberately NOT part of that change and is pinned here so it stays
// deliberate: this list clamps because the store substitutes a real default
// page, which is the discriminator core.ErrLimitNotPositive documents.

func filterHarness(t *testing.T) (*fakeStore, *http.ServeMux, graphql.Schema) {
	t.Helper()
	fs := &fakeStore{}
	svc := &Service{Base: &core.Base{Authz: fakeAllowChecker{}}, Store: fs}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return fs, mux, schema
}

func auditCtx() context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: "admin-1", Method: "session"})
}

func getAuditREST(t *testing.T, mux *http.ServeMux, query string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/owners/tea-owner/audit-logs?"+query, nil)
	mux.ServeHTTP(rec, req.WithContext(auditCtx()))
	return rec
}

func queryAuditGraphQL(t *testing.T, schema graphql.Schema, args string) error {
	t.Helper()
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ auditLogs(ownerId:"tea-owner"` + args + `) { id } }`,
		Context:       auditCtx(),
	})
	if len(res.Errors) > 0 {
		return errors.New(res.Errors[0].Message)
	}
	return nil
}

// TestMalformedTimeBoundIsRejectedOnBothSurfaces is the regression proper. The
// assertion has to be on the ERROR, because a leaked bad bound is invisible in
// the response: it comes back as a full, plausible page. The store-was-never-
// called check is the other half — an unbounded query must not be issued.
func TestMalformedTimeBoundIsRejectedOnBothSurfaces(t *testing.T) {
	for _, bound := range []string{"startTime", "endTime"} {
		t.Run("REST/"+bound, func(t *testing.T) {
			fs, mux, _ := filterHarness(t)
			rec := getAuditREST(t, mux, bound+"=yesterday")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), bound) {
				t.Errorf("body = %s, want the offending field named", rec.Body.String())
			}
			assertStoreNotQueried(t, fs)
		})

		t.Run("GraphQL/"+bound, func(t *testing.T) {
			fs, _, schema := filterHarness(t)
			err := queryAuditGraphQL(t, schema, `, `+bound+`: "yesterday"`)
			if err == nil {
				t.Fatal("malformed bound accepted; the query would run over the whole audit history")
			}
			if !strings.Contains(err.Error(), bound) {
				t.Errorf("error = %v, want the offending field named", err)
			}
			assertStoreNotQueried(t, fs)
		})
	}
}

// TestValidTimeBoundsReachTheStore is the other half of the guard: one that
// rejected everything would satisfy the test above. Both surfaces must still
// deliver the exact instants the caller asked for.
func TestValidTimeBoundsReachTheStore(t *testing.T) {
	since := "2026-01-02T03:04:05Z"
	until := "2026-02-03T04:05:06Z"
	wantSince, _ := time.Parse(time.RFC3339, since)
	wantUntil, _ := time.Parse(time.RFC3339, until)

	t.Run("REST", func(t *testing.T) {
		fs, mux, _ := filterHarness(t)
		if rec := getAuditREST(t, mux, "startTime="+since+"&endTime="+until); rec.Code != http.StatusOK {
			t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body.String())
		}
		assertWindow(t, fs, wantSince, wantUntil)
	})

	t.Run("GraphQL", func(t *testing.T) {
		fs, _, schema := filterHarness(t)
		if err := queryAuditGraphQL(t, schema, `, startTime: "`+since+`", endTime: "`+until+`"`); err != nil {
			t.Fatalf("valid bounds rejected: %v", err)
		}
		assertWindow(t, fs, wantSince, wantUntil)
	})
}

// TestAbsentTimeBoundsStayUnbounded pins that "" is still absence, not a
// malformed value — the whole audit history remains the default answer when no
// window is named, which is the pre-existing contract.
func TestAbsentTimeBoundsStayUnbounded(t *testing.T) {
	fs, mux, _ := filterHarness(t)
	if rec := getAuditREST(t, mux, ""); rec.Code != http.StatusOK {
		t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body.String())
	}
	got := fs.filter()
	if !got.Since.IsZero() || !got.Until.IsZero() {
		t.Fatalf("filter = %+v, want both bounds zero", got)
	}
}

// TestUnparseableLimitStillClampsRatherThanRejecting pins the deliberate
// asymmetry against the change above, so nobody "completes" it by mistake.
// deploys and jobs reject a bad limit because their stores read <=0 as
// "absent" and substitute the MAX page; audit's store substitutes
// store.DefaultAuditPageSize, a real default, so falling through to it is a
// bounded, defined answer rather than an unbounded one.
func TestUnparseableLimitStillClampsRatherThanRejecting(t *testing.T) {
	fs, mux, _ := filterHarness(t)
	if rec := getAuditREST(t, mux, "limit=abc"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — audit clamps its limit, it does not reject it", rec.Code)
	}
	if got := fs.filter().Limit; got != 0 {
		t.Errorf("Limit = %d, want 0 so the store applies DefaultAuditPageSize", got)
	}
}

// TestUnknownDirectionStillRejected guards the rule that was already right,
// since FilterOf now sits between the adapters and Service.List and could
// swallow it.
func TestUnknownDirectionStillRejected(t *testing.T) {
	_, mux, schema := filterHarness(t)
	if rec := getAuditREST(t, mux, "direction=sideways"); rec.Code != http.StatusBadRequest {
		t.Errorf("REST direction=sideways = %d, want 400", rec.Code)
	}
	if err := queryAuditGraphQL(t, schema, `, direction: "sideways"`); err == nil {
		t.Error("GraphQL direction=sideways accepted, want a named 400")
	}
}

// TestFilterOfIsTheOneTranslator drives the seam directly for the property the
// two surface tests cannot show on their own: that they agree. A field added to
// one adapter but not the other is the drift this feature already had.
func TestFilterOfIsTheOneTranslator(t *testing.T) {
	start, end := "2026-01-02T03:04:05Z", "2026-02-03T04:05:06Z"
	wantSince, _ := time.Parse(time.RFC3339, start)
	wantUntil, _ := time.Parse(time.RFC3339, end)

	f, err := FilterOf(start, end, core.DirectionForward, "aud-7", 42)
	if err != nil {
		t.Fatalf("FilterOf: %v", err)
	}
	// Asserted as exact instants, not merely non-zero: with five positional
	// parameters, four of them strings, a transposed startTime/endTime is the
	// most plausible defect in this signature — and it is precisely what a
	// non-zero check cannot see.
	if !f.Since.Equal(wantSince) || !f.Until.Equal(wantUntil) {
		t.Errorf("window = [%v, %v], want [%v, %v]", f.Since, f.Until, wantSince, wantUntil)
	}
	if f.Direction != core.DirectionForward || f.Cursor != "aud-7" || f.Limit != 42 {
		t.Errorf("filter = %+v, want direction/cursor/limit carried through", f)
	}
}

func assertStoreNotQueried(t *testing.T, fs *fakeStore) {
	t.Helper()
	if got := fs.workspace(); got != "" {
		t.Errorf("store was queried (workspace %q) despite a malformed filter — "+
			"the read ran with the bad bound silently dropped", got)
	}
}

func assertWindow(t *testing.T, fs *fakeStore, since, until time.Time) {
	t.Helper()
	got := fs.filter()
	if !got.Since.Equal(since) || !got.Until.Equal(until) {
		t.Errorf("store window = [%v, %v], want [%v, %v]", got.Since, got.Until, since, until)
	}
}

// filter/workspace read the spy's fields under its mutex, matching the
// accessors the sweep-loop tests use.
func (f *fakeStore) filter() store.AuditFilter {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.gotFilter
}

func (f *fakeStore) workspace() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.gotWorkspace
}
