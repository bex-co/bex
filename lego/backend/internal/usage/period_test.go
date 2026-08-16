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

package usage

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// period_test.go pins the contract in ResolvePeriodEnd's doc across all three
// surfaces: a malformed period is a 400, a well-formed one resolves and is
// echoed back, and a non-past month clamps rather than errors.
//
// The assertions are on the ERROR and on the ECHOED month, never on "it
// returned something": the swallowed-parse bug this replaced produced a
// perfectly valid-looking response for the wrong month.

// periodSurfaces holds all three registered entry points over ONE Service, so a
// test can drive the same input at each. Built once per top-level test: the MCP
// session alone (server + transport pair + handshake) is ~89% of a call's cost
// and ~98% of its allocations, and nothing here is mutated by a read.
type periodSurfaces struct {
	mux    *http.ServeMux
	schema graphql.Schema
	mcp    *mcp.ClientSession
}

func periodHarness(t *testing.T) *periodSurfaces {
	t.Helper()
	svc := svcWithTenant(seedStore(), "tea-001")

	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	schema, err := buildTestSchema(svc)
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	ctx := periodCtx()
	srv := mcp.NewServer(&mcp.Implementation{Name: "usage-test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("MCP server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("MCP client connect: %v", err)
	}
	// Closing the client tears down the pipe; the server loop then sees EOF and
	// exits. Without this each session leaks four goroutines — measured.
	t.Cleanup(func() { _ = cs.Close(); _ = ss.Wait() })

	return &periodSurfaces{mux: mux, schema: schema, mcp: cs}
}

func periodCtx() context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: "user:alice"})
}

func getUsageREST(t *testing.T, mux *http.ServeMux, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/v1/usage?"+query, nil).WithContext(periodCtx())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// getUsageGraphQL returns the period the response echoed, so a caller can
// assert the month actually used and not merely that no error came back.
func getUsageGraphQL(schema graphql.Schema, period string) (string, error) {
	res := graphql.Do(graphql.Params{
		Schema:         schema,
		RequestString:  `query($period: String) { usage(period: $period) { workspaceId period } }`,
		VariableValues: map[string]any{"period": period},
		Context:        periodCtx(),
	})
	if len(res.Errors) > 0 {
		return "", errors.New(res.Errors[0].Message)
	}
	data, _ := res.Data.(map[string]any)
	usage, _ := data["usage"].(map[string]any)
	echoed, _ := usage["period"].(string)
	return echoed, nil
}

// mcpPeriod calls the real registered get_usage tool over the shared session and
// returns the period the response echoed.
func (p *periodSurfaces) mcpPeriod(period string) (string, error) {
	res, err := p.mcp.CallTool(periodCtx(), &mcp.CallToolParams{
		Name: "get_usage", Arguments: map[string]any{"period": period},
	})
	if err != nil {
		return "", err
	}
	if res.IsError {
		b, _ := json.Marshal(res.Content)
		return "", errors.New(string(b))
	}
	var out usageResponse
	b, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	return out.Period, nil
}

// TestMalformedPeriodIsRejectedOnEverySurface is the regression proper.
func TestMalformedPeriodIsRejectedOnEverySurface(t *testing.T) {
	h := periodHarness(t)

	for _, bad := range []string{"july", "2026-1x", "2026", "2026-13-01"} {
		t.Run("REST/"+bad, func(t *testing.T) {
			rec := getUsageREST(t, h.mux, "period="+bad)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("period=%s = %d, want 400 (body: %s)", bad, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "period") {
				t.Errorf("body = %s, want the offending field named", rec.Body.String())
			}
		})

		t.Run("GraphQL/"+bad, func(t *testing.T) {
			if _, err := getUsageGraphQL(h.schema, bad); err == nil {
				t.Fatalf("period=%s accepted; the caller silently gets a different month's numbers", bad)
			}
		})

		t.Run("MCP/"+bad, func(t *testing.T) {
			if _, err := h.mcpPeriod(bad); err == nil {
				t.Fatalf("period=%s accepted; the caller silently gets a different month's numbers", bad)
			}
		})
	}
}

// TestValidPeriodStillResolves is the other half: a guard that rejected
// everything would satisfy the test above. A well-formed past month must still
// reach the store and echo itself back.
func TestValidPeriodStillResolves(t *testing.T) {
	h := periodHarness(t)

	t.Run("REST", func(t *testing.T) {
		rec := getUsageREST(t, h.mux, "period=2026-06")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body.String())
		}
		var resp usageResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatal(err)
		}
		if resp.Period != "2026-06" {
			t.Errorf("period = %q, want 2026-06", resp.Period)
		}
	})

	t.Run("GraphQL", func(t *testing.T) {
		echoed, err := getUsageGraphQL(h.schema, "2026-06")
		if err != nil {
			t.Fatalf("valid period rejected: %v", err)
		}
		if echoed != "2026-06" {
			t.Errorf("echoed period = %q, want 2026-06", echoed)
		}
	})

	t.Run("MCP", func(t *testing.T) {
		echoed, err := h.mcpPeriod("2026-06")
		if err != nil {
			t.Fatalf("valid period rejected: %v", err)
		}
		if echoed != "2026-06" {
			t.Errorf("echoed period = %q, want 2026-06", echoed)
		}
	})

	t.Run("absent period is still the current month", func(t *testing.T) {
		if rec := getUsageREST(t, h.mux, ""); rec.Code != http.StatusOK {
			t.Fatalf("status = %d (body: %s)", rec.Code, rec.Body.String())
		}
	})
}

// TestResolvePeriodEndClampsRatherThanRejectsANonPastMonth pins the deliberate
// asymmetry so it is not "completed" into a rejection later. A period that
// PARSES is a value the caller spelled correctly; the current month is answered
// month-to-date and a future month has no data to bound, so both collapse to
// "as of now". Only an UNPARSEABLE period is an ignored input.
func TestResolvePeriodEndClampsRatherThanRejectsANonPastMonth(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		name   string
		period string
		want   time.Time
	}{
		{"absent", "", now},
		{"the current month", "2026-08", now},
		{"a future month", "2027-01", now},
		{"a past month ends at its last instant", "2026-06",
			time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolvePeriodEnd(tc.period, now)
			if err != nil {
				t.Fatalf("ResolvePeriodEnd(%q) = %v, want no error", tc.period, err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("ResolvePeriodEnd(%q) = %v, want %v", tc.period, got, tc.want)
			}
		})
	}

	if _, err := ResolvePeriodEnd("nonsense", now); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("malformed period = %v, want a core.ErrBadRequest", err)
	}
}
