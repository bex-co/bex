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

package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestLokiPathFilterWildcardsAndEscaping(t *testing.T) {
	t.Run("LogQL rendering", func(t *testing.T) {
		cases := []struct {
			name  string
			paths []string
			want  string
		}{
			{"empty", nil, ""},
			{"blank only", []string{""}, ""},
			{"single literal keeps exact equality", []string{"/api/v1"}, `request_path="/api/v1"`},
			{"multiple literals OR exact equality", []string{"/a", "/b", "/a"}, `request_path="/a" or request_path="/b"`},
			{"prefix wildcard", []string{"/api/*"}, `request_path=~"^/api/.*$"`},
			{"interior wildcard", []string{"/api/*/items"}, `request_path=~"^/api/.*/items$"`},
			{"collapsed consecutive wildcards", []string{"/api/**"}, `request_path=~"^/api/.*$"`},
			{"bare wildcard", []string{"*"}, `request_path=~"^.*$"`},
			{"mixed literal and wildcard", []string{"/health", "/api/*"}, `request_path="/health" or request_path=~"^/api/.*$"`},
			{"metacharacters escaped literally", []string{"/v1.0/(a+b)?[x]*"}, `request_path=~"^/v1\\.0/\\(a\\+b\\)\\?\\[x\\].*$"`},
			{"LogQL quote injection stays quoted", []string{`/api/*" or 1==1`}, `request_path=~"^/api/.*\" or 1==1$"`},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := lokiPathFilter("request_path", tc.paths); got != tc.want {
					t.Errorf("lokiPathFilter(%v) =\n  %s\nwant\n  %s", tc.paths, got, tc.want)
				}
			})
		}
	})

	t.Run("compiled RE2 matching semantics", func(t *testing.T) {
		cases := []struct {
			pattern string
			match   []string
			noMatch []string
		}{
			{
				pattern: "/api/*",
				match:   []string{"/api/", "/api/health", "/api/v1/users"},
				noMatch: []string{"/api", "/v2/api/health", "/apiv1/users"},
			},
			{
				pattern: "/api/*/items",
				match:   []string{"/api/v1/items", "/api/123/sub/items"},
				noMatch: []string{"/api/v1/items/extra", "/v2/api/v1/items"},
			},
			{
				pattern: "/v1.0/(a+b)?*",
				match:   []string{"/v1.0/(a+b)?", "/v1.0/(a+b)?id=1"},
				noMatch: []string{"/v1X0/(a+b)?", "/v1.0/ab", "/v1.0/a+b"},
			},
		}
		for _, tc := range cases {
			re, err := regexp.Compile(wildcardPathRegex(tc.pattern))
			if err != nil {
				t.Fatalf("wildcardPathRegex(%q) failed RE2 compile: %v", tc.pattern, err)
			}
			for _, p := range tc.match {
				if !re.MatchString(p) {
					t.Errorf("pattern %q (%s) should match %q", tc.pattern, re.String(), p)
				}
			}
			for _, p := range tc.noMatch {
				if re.MatchString(p) {
					t.Errorf("pattern %q (%s) must not match %q", tc.pattern, re.String(), p)
				}
			}
		}
	})

	t.Run("lokiRequestQueryFor uses wildcard matcher for http_requests and http_latency", func(t *testing.T) {
		baseReq := RequestMetricsRequest{
			Namespace: "tea-a", App: "web", Resolution: time.Minute,
			Paths: []string{"/api/*"},
		}
		reqQ := lokiRequestQueryFor(func(r RequestMetricsRequest) RequestMetricsRequest {
			r.Metric = MetricHTTPRequests
			return r
		}(baseReq))
		if !strings.Contains(reqQ, `| request_path=~"^/api/.*$"`) {
			t.Errorf("http_requests query missing wildcard path matcher: %s", reqQ)
		}
		latQ := lokiRequestQueryFor(func(r RequestMetricsRequest) RequestMetricsRequest {
			r.Metric = MetricHTTPLatency
			r.Quantile = 0.95
			return r
		}(baseReq))
		if !strings.Contains(latQ, `| request_path=~"^/api/.*$"`) {
			t.Errorf("http_latency query missing wildcard path matcher: %s", latQ)
		}
	})
}

func TestLokiPathSuggestionsSourceRoundTrip(t *testing.T) {
	start := time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
	end := time.Date(2026, 11, 23, 11, 0, 0, 0, time.UTC)

	t.Run("query shape, frequency ordering, and tie-breaking", func(t *testing.T) {
		var gotPath, gotQuery, gotDir, gotLimit, gotStart, gotEnd string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			q := r.URL.Query()
			gotQuery = q.Get("query")
			gotDir = q.Get("direction")
			gotLimit = q.Get("limit")
			gotStart = q.Get("start")
			gotEnd = q.Get("end")
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"streams","result":[` +
				`{"stream":{"status":"200"},"values":[["1000","/api/users"],["1001","/api/health"],["1002","/api/users"],["1003","/api/alpha"]]},` +
				`{"stream":{"status":"404"},"values":[["1004","/api/users"],["1005","/api/alpha"],["1006","/api/beta"],["1007",""]]}` +
				`]}}`))
		}))
		defer ts.Close()

		src := NewLokiPathFilterSuggestionsSource(ts.URL, ts.Client())
		got, err := src(context.Background(), PathFilterSuggestionsRequest{
			Namespace:   "tea-a",
			App:         "web",
			Start:       start,
			End:         end,
			Hosts:       []string{"api.example.com"},
			StatusCodes: []string{"2xx", "404"},
			Paths:       []string{"/api/*"},
		})
		if err != nil {
			t.Fatalf("source: %v", err)
		}
		if gotPath != "/loki/api/v1/query_range" {
			t.Errorf("path = %q, want /loki/api/v1/query_range", gotPath)
		}
		if gotDir != "backward" || gotLimit != "5000" {
			t.Errorf("direction=%q limit=%q, want backward and 5000", gotDir, gotLimit)
		}
		if gotStart != fmt.Sprintf("%d", start.UnixNano()) || gotEnd != fmt.Sprintf("%d", end.Add(time.Nanosecond).UnixNano()) {
			t.Errorf("start=%s end=%s, want %d and %d", gotStart, gotEnd, start.UnixNano(), end.Add(time.Nanosecond).UnixNano())
		}
		wantQuery := `{namespace="tea-a", app="web", type="request", status=~"^(2..|404)$"} | json request_host="RequestHost", request_path="RequestPath" | request_host="api.example.com" | request_path=~"^/api/.*$" | request_path!="" | line_format "{{.request_path}}"`
		if gotQuery != wantQuery {
			t.Errorf("LogQL =\n  %s\nwant\n  %s", gotQuery, wantQuery)
		}
		// /api/users (3), /api/alpha (2), then /api/beta (1) before /api/health (1) lexicographically.
		want := []string{"/api/users", "/api/alpha", "/api/beta", "/api/health"}
		if !slices.Equal(got, want) {
			t.Errorf("suggestions = %v, want %v", got, want)
		}
	})

	t.Run("caps distinct suggestions to maxPathFilterSuggestions", func(t *testing.T) {
		values := make([]string, 0, maxPathFilterSuggestions+30)
		for i := range maxPathFilterSuggestions + 30 {
			values = append(values, fmt.Sprintf(`["%d","/p/%03d"]`, 1000+i, i))
		}
		body := `{"status":"success","data":{"resultType":"streams","result":[{"stream":{},"values":[` + strings.Join(values, ",") + `]}]}}`
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		defer ts.Close()

		got, err := NewLokiPathFilterSuggestionsSource(ts.URL, ts.Client())(context.Background(), PathFilterSuggestionsRequest{
			Namespace: "tea-a", App: "web", Start: start, End: end,
		})
		if err != nil {
			t.Fatalf("source: %v", err)
		}
		if len(got) != maxPathFilterSuggestions {
			t.Fatalf("len = %d, want %d", len(got), maxPathFilterSuggestions)
		}
		if got[0] != "/p/000" || got[len(got)-1] != fmt.Sprintf("/p/%03d", maxPathFilterSuggestions-1) {
			t.Errorf("unexpected bounded order: first=%q last=%q", got[0], got[len(got)-1])
		}
	})

	t.Run("empty, inverted, and error responses", func(t *testing.T) {
		called := false
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			http.Error(w, "boom", http.StatusBadGateway)
		}))
		defer ts.Close()
		src := NewLokiPathFilterSuggestionsSource(ts.URL, ts.Client())

		// Inverted range skips HTTP entirely.
		got, err := src(context.Background(), PathFilterSuggestionsRequest{
			Namespace: "tea-a", App: "web", Start: end, End: start,
		})
		if err != nil || got == nil || len(got) != 0 || called {
			t.Errorf("inverted range: got=%v err=%v called=%v, want empty non-nil with no HTTP call", got, err, called)
		}

		// Non-200 surfaces as an error.
		if _, err := src(context.Background(), PathFilterSuggestionsRequest{
			Namespace: "tea-a", App: "web", Start: start, End: end,
		}); err == nil {
			t.Error("HTTP 502 must return an error")
		}
	})
}

type selectiveAuthChecker struct {
	allowed map[string]bool
}

func (c selectiveAuthChecker) Check(_ context.Context, _, _, object string) (bool, error) {
	return c.allowed[object], nil
}

type multiWorkspaceResolver struct {
	defaultTenant string
	members       map[string]bool
}

func (r multiWorkspaceResolver) Tenant(context.Context, core.Identity) (string, bool) {
	return r.defaultTenant, true
}

func (r multiWorkspaceResolver) IsMember(_ context.Context, _ core.Identity, tenantID string) (bool, error) {
	return r.members[tenantID], nil
}

func TestPathFilterSuggestionsServiceContract(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)

	t.Run("multi-resource union, deduplication, and rank interleaving", func(t *testing.T) {
		appA := sampleApp("web")
		appA.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelAppID: "srv-web"}
		appB := sampleApp("api")
		appB.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelAppID: "srv-api"}

		svc := newService(nil, nil, appA, appB)
		svc.Clock = func() time.Time { return now }
		var reqs []PathFilterSuggestionsRequest
		svc.PathFilterSuggestionsSource = func(_ context.Context, req PathFilterSuggestionsRequest) ([]string, error) {
			reqs = append(reqs, req)
			if req.App == "web" {
				return []string{"/shared", "/web-1", "/web-2"}, nil
			}
			return []string{"/api-1", "/shared", "/api-2"}, nil
		}

		// Duplicate resource "web" must only query Loki once.
		got, err := svc.PathFilterSuggestions(context.Background(), PathFilterSuggestionsQuery{
			Apps:        []string{"web", "api", "web"},
			Hosts:       []string{"example.com"},
			StatusCodes: []string{"200", "4xx"},
			Paths:       []string{"/*"},
		})
		if err != nil {
			t.Fatalf("PathFilterSuggestions: %v", err)
		}
		if len(reqs) != 2 {
			t.Fatalf("source called %d times, want 2 (deduplicated)", len(reqs))
		}
		if !reqs[0].Start.Equal(now.Add(-time.Hour)) || !reqs[0].End.Equal(now) || reqs[0].Limit != lokiPathSuggestionsLimit {
			t.Errorf("default window/limit = %+v, want [now-1h, now] limit %d", reqs[0], lokiPathSuggestionsLimit)
		}
		want := []string{"/shared", "/api-1", "/web-1", "/web-2", "/api-2"}
		if !slices.Equal(got, want) {
			t.Errorf("merged = %v, want %v", got, want)
		}
	})

	t.Run("plan retention clamping and app creation floor", func(t *testing.T) {
		app := sampleApp("web")
		app.Labels = map[string]string{core.LabelTenant: "tea-hobby"}
		app.CreationTimestamp = metav1.NewTime(now.Add(-10 * retentionDay))

		svc := newService(nil, nil, app)
		svc.Clock = func() time.Time { return now }
		svc.Plans = &planReader{plan: "hobby"} // 7 days retention

		var gotReqs []PathFilterSuggestionsRequest
		svc.PathFilterSuggestionsSource = func(_ context.Context, req PathFilterSuggestionsRequest) ([]string, error) {
			gotReqs = append(gotReqs, req)
			return []string{"/ok"}, nil
		}

		// 20-day range clamps to Hobby's 7-day floor.
		got, err := svc.PathFilterSuggestions(context.Background(), PathFilterSuggestionsQuery{
			Apps:  []string{"web"},
			Start: now.Add(-20 * retentionDay),
			End:   now,
		})
		if err != nil || !slices.Equal(got, []string{"/ok"}) || len(gotReqs) != 1 {
			t.Fatalf("clamped read: got=%v err=%v reqs=%d", got, err, len(gotReqs))
		}
		if wantStart := now.Add(-7 * retentionDay); !gotReqs[0].Start.Equal(wantStart) {
			t.Errorf("clamped start = %v, want %v (Hobby 7d floor)", gotReqs[0].Start, wantStart)
		}

		// Range wholly before Hobby's 7-day floor returns [] without calling Loki.
		gotReqs = nil
		got, err = svc.PathFilterSuggestions(context.Background(), PathFilterSuggestionsQuery{
			Apps:  []string{"web"},
			Start: now.Add(-15 * retentionDay),
			End:   now.Add(-8 * retentionDay),
		})
		if err != nil || got == nil || len(got) != 0 || len(gotReqs) != 0 {
			t.Errorf("before-retention read: got=%v err=%v reqs=%d, want empty without querying Loki", got, err, len(gotReqs))
		}

		// App created 30 minutes ago clamps a 2-hour window to creationTimestamp (w8/m124).
		app.CreationTimestamp = metav1.NewTime(now.Add(-30 * time.Minute))
		svc2 := newService(nil, nil, app)
		svc2.Clock = func() time.Time { return now }
		gotReqs = nil
		svc2.PathFilterSuggestionsSource = func(_ context.Context, req PathFilterSuggestionsRequest) ([]string, error) {
			gotReqs = append(gotReqs, req)
			return []string{"/fresh"}, nil
		}
		if _, err := svc2.PathFilterSuggestions(context.Background(), PathFilterSuggestionsQuery{
			Apps:  []string{"web"},
			Start: now.Add(-2 * time.Hour),
			End:   now,
		}); err != nil {
			t.Fatalf("incarnation clamp: %v", err)
		}
		if len(gotReqs) != 1 || !gotReqs[0].Start.Equal(now.Add(-30*time.Minute)) {
			t.Errorf("incarnation start = %+v, want %v", gotReqs, now.Add(-30*time.Minute))
		}
	})

	t.Run("authz checks every requested resource", func(t *testing.T) {
		own := sampleApp("own")
		own.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelAppID: "srv-own"}
		denied := sampleApp("denied")
		denied.Labels = map[string]string{core.LabelTenant: "tea-b", core.LabelAppID: "srv-denied"}
		foreign := sampleApp("foreign")
		foreign.Labels = map[string]string{core.LabelTenant: "tea-c", core.LabelAppID: "srv-foreign"}

		svc := newService(nil, nil, own, denied, foreign)
		svc.Clock = func() time.Time { return now }
		svc.Authz = selectiveAuthChecker{allowed: map[string]bool{core.WorkspaceObject("tea-a"): true}}
		// Member of tea-a (allowed) and tea-b (denied by Authz => 403), not a member of tea-c (also 403 via by-id membership gate).
		svc.Workspace = multiWorkspaceResolver{defaultTenant: "tea-a", members: map[string]bool{"tea-a": true, "tea-b": true}}
		called := false
		svc.PathFilterSuggestionsSource = func(_ context.Context, _ PathFilterSuggestionsRequest) ([]string, error) {
			called = true
			return []string{"/secret"}, nil
		}

		ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "u1", Method: "session"})
		// Even when the first resource is authorized, an unauthorized second resource must fail 403 before Loki is queried.
		if _, err := svc.PathFilterSuggestions(ctx, PathFilterSuggestionsQuery{
			Apps: []string{"srv-own", "srv-denied"},
		}); !errors.Is(err, core.ErrForbidden) {
			t.Errorf("mixed own+denied = %v, want ErrForbidden", err)
		}
		// A non-member cross-workspace by-id lookup is also refused (never leaks).
		if _, err := svc.PathFilterSuggestions(ctx, PathFilterSuggestionsQuery{
			Apps: []string{"srv-own", "srv-foreign"},
		}); !errors.Is(err, core.ErrForbidden) && !errors.Is(err, core.ErrNotFound) {
			t.Errorf("mixed own+foreign = %v, want ErrForbidden or ErrNotFound", err)
		}
		if _, err := svc.PathFilterSuggestions(ctx, PathFilterSuggestionsQuery{
			Apps: []string{"srv-own", "srv-missing"},
		}); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("mixed own+missing = %v, want ErrNotFound", err)
		}
		if called {
			t.Error("source must not be called when any requested resource is unauthorized or unknown")
		}
	})

	t.Run("bounds, validation, and Loki unwired 503", func(t *testing.T) {
		svc := newService(nil, nil, sampleApp("web"))
		svc.Clock = func() time.Time { return now }
		svc.MaxQueryHours = 24

		// Unwired Loki => 503.
		if _, err := svc.PathFilterSuggestions(context.Background(), PathFilterSuggestionsQuery{
			Apps: []string{"web"},
		}); !errors.Is(err, core.ErrLogStoreUnavailable) {
			t.Errorf("unwired Loki = %v, want ErrLogStoreUnavailable", err)
		}

		called := false
		svc.PathFilterSuggestionsSource = func(_ context.Context, _ PathFilterSuggestionsRequest) ([]string, error) {
			called = true
			return []string{"/ok"}, nil
		}

		tooManyResources := make([]string, maxMetricsFanOut+1)
		for i := range tooManyResources {
			tooManyResources[i] = "web"
		}
		tooManyPaths := make([]string, maxPathFilters+1)
		for i := range tooManyPaths {
			tooManyPaths[i] = "/a"
		}
		badCases := []struct {
			name string
			q    PathFilterSuggestionsQuery
		}{
			{"empty resources", PathFilterSuggestionsQuery{}},
			{"blank resources", PathFilterSuggestionsQuery{Apps: []string{"", "   "}}},
			{"over MaxQueryHours", PathFilterSuggestionsQuery{Apps: []string{"web"}, Start: now.Add(-25 * time.Hour), End: now}},
			{"inverted range", PathFilterSuggestionsQuery{Apps: []string{"web"}, Start: now, End: now.Add(-time.Minute)}},
			{"over fan-out", PathFilterSuggestionsQuery{Apps: tooManyResources}},
			{"invalid statusCode", PathFilterSuggestionsQuery{Apps: []string{"web"}, StatusCodes: []string{"999"}}},
			{"too many paths", PathFilterSuggestionsQuery{Apps: []string{"web"}, Paths: tooManyPaths}},
			{"oversized path pattern", PathFilterSuggestionsQuery{Apps: []string{"web"}, Paths: []string{"/" + strings.Repeat("a", maxPathPatternLen)}}},
			{"too many wildcards", PathFilterSuggestionsQuery{Apps: []string{"web"}, Paths: []string{strings.Repeat("/*", maxPathWildcards+1)}}},
			{"oversized host", PathFilterSuggestionsQuery{Apps: []string{"web"}, Hosts: []string{strings.Repeat("h", maxHostPatternLen+1)}}},
		}
		for _, tc := range badCases {
			t.Run(tc.name, func(t *testing.T) {
				called = false
				if _, err := svc.PathFilterSuggestions(context.Background(), tc.q); !errors.Is(err, core.ErrBadRequest) {
					t.Errorf("%s = %v, want ErrBadRequest", tc.name, err)
				}
				if called {
					t.Errorf("%s reached Loki source", tc.name)
				}
			})
		}
	})
}

func TestPathFilterSuggestionsRESTAndGraphQLParity(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	ownA := sampleApp("web")
	ownA.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelAppID: "srv-web"}
	ownB := sampleApp("api")
	ownB.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelAppID: "srv-api"}
	foreign := sampleApp("secret")
	foreign.Labels = map[string]string{core.LabelTenant: "tea-b", core.LabelAppID: "srv-secret"}

	svc := newService(nil, nil, ownA, ownB, foreign)
	svc.Clock = func() time.Time { return now }
	svc.Authz = selectiveAuthChecker{allowed: map[string]bool{core.WorkspaceObject("tea-a"): true}}
	svc.Workspace = multiWorkspaceResolver{defaultTenant: "tea-a", members: map[string]bool{"tea-a": true, "tea-b": true}}
	svc.PathFilterSuggestionsSource = func(_ context.Context, req PathFilterSuggestionsRequest) ([]string, error) {
		if req.App == "web" {
			return []string{"/api/v1/users", "/api/health"}, nil
		}
		return []string{"/api/orders", "/api/health"}, nil
	}

	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "u1", Method: "session"})

	// REST with multiple resources + path wildcard.
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodGet, "/v1/metrics/filters/path?resource=srv-web&service=srv-api&path=/api/*", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("REST = %d %s, want 200", rec.Code, rec.Body)
	}
	var restPaths []string
	if err := json.Unmarshal(rec.Body.Bytes(), &restPaths); err != nil {
		t.Fatalf("decode REST: %v", err)
	}

	// GraphQL with identical serviceIDs + paths must return identical suggestions.
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("gqlSchema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       ctx,
		RequestString: `{ metricsPathFilterSuggestions(query: {serviceIDs: ["srv-web", "srv-api"], paths: ["/api/*"]}) { paths } }`,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL errors: %v", res.Errors)
	}
	data := res.Data.(map[string]any)["metricsPathFilterSuggestions"].(map[string]any)
	rawGQLPaths := data["paths"].([]any)
	gqlPaths := make([]string, len(rawGQLPaths))
	for i, v := range rawGQLPaths {
		gqlPaths[i] = v.(string)
	}
	if !slices.Equal(restPaths, gqlPaths) {
		t.Errorf("REST %v != GraphQL %v", restPaths, gqlPaths)
	}
	want := []string{"/api/v1/users", "/api/orders", "/api/health"}
	if !slices.Equal(restPaths, want) {
		t.Errorf("suggestions = %v, want %v", restPaths, want)
	}

	// GraphQL must reject unauthorized and unknown serviceIDs (previously an unauthed stub).
	for _, tc := range []struct {
		name string
		ids  string
	}{
		{"unauthorized single", `["srv-secret"]`},
		{"unauthorized second", `["srv-web", "srv-secret"]`},
		{"unknown service", `["srv-nope"]`},
		{"empty serviceIDs", `[]`},
		{"blank serviceID", `[""]`},
	} {
		t.Run("GraphQL rejects "+tc.name, func(t *testing.T) {
			r := graphql.Do(graphql.Params{
				Schema:        schema,
				Context:       ctx,
				RequestString: `{ metricsPathFilterSuggestions(query: {serviceIDs: ` + tc.ids + `}) { paths } }`,
			})
			if len(r.Errors) == 0 {
				t.Errorf("%s: expected GraphQL error, got data=%v", tc.name, r.Data)
			}
		})
	}

	// REST unauthorized -> 403, missing resource -> 400, unknown -> 404.
	for _, tc := range []struct {
		target string
		want   int
	}{
		{"/v1/metrics/filters/path?resource=srv-secret", http.StatusForbidden},
		{"/v1/metrics/filters/path?resource=srv-web&resource=srv-secret", http.StatusForbidden},
		{"/v1/metrics/filters/path?resource=srv-nope", http.StatusNotFound},
		{"/v1/metrics/filters/path", http.StatusBadRequest},
		{"/v1/metrics/filters/path?resource=", http.StatusBadRequest},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.target, nil).WithContext(ctx)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("GET %s = %d %s, want %d", tc.target, rec.Code, rec.Body, tc.want)
		}
	}
}

func TestWildcardPathCrossSurfaceParity(t *testing.T) {
	const pattern = "/api/*"
	tooManyWildcards := strings.Repeat("/*", maxPathWildcards+1)

	newSurfaceSvc := func() (*Service, *[]RequestMetricsRequest) {
		got := &[]RequestMetricsRequest{}
		svc := newService(nil, nil, sampleApp("web"), podFor("web", webInst))
		svc.RequestLogMetrics = func(_ context.Context, r RequestMetricsRequest) ([]MetricSeries, error) {
			*got = append(*got, r)
			return []MetricSeries{{Points: []MetricPoint{{Timestamp: "2026-11-23T12:00:00Z", Value: 2}}}}, nil
		}
		return svc, got
	}

	// Valid wildcard pattern across REST, GraphQL, and MCP for both http_requests and http_latency.
	restSvc, restGot := newSurfaceSvc()
	for _, seg := range []string{"http-requests", "http-latency"} {
		if rec := serveREST(restSvc, "/v1/metrics/"+seg+"?resource=web&path=%2Fapi%2F*"); rec.Code != http.StatusOK {
			t.Fatalf("REST %s: %d %s", seg, rec.Code, rec.Body)
		}
	}

	gqlSvc, gqlGot := newSurfaceSvc()
	schema, _ := gqlSchema(gqlSvc)
	for _, name := range []string{"HTTP_REQUESTS", "HTTP_LATENCY"} {
		res := graphql.Do(graphql.Params{
			Schema:        schema,
			Context:       context.Background(),
			RequestString: `{ metrics(query: {filters: [{field:"RESOURCE",values:["web"]},{field:"PATH",values:["` + pattern + `"]}], name:"` + name + `"}) { unit } }`,
		})
		if len(res.Errors) > 0 {
			t.Fatalf("GraphQL %s: %v", name, res.Errors)
		}
	}

	mcpSvc, mcpGot := newSurfaceSvc()
	cs := mcpSession(t, mcpSvc)
	if _, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "get_metrics",
		Arguments: map[string]any{
			"resourceId":  "web",
			"metricTypes": []string{"http_request_count", "http_latency"},
			"httpPath":    pattern,
		},
	}); err != nil {
		t.Fatalf("MCP: %v", err)
	}

	for surface, got := range map[string]*[]RequestMetricsRequest{"REST": restGot, "GraphQL": gqlGot, "MCP": mcpGot} {
		if len(*got) != 2 {
			t.Fatalf("%s made %d Loki reads, want 2", surface, len(*got))
		}
		for _, r := range *got {
			if !slices.Equal(r.Paths, []string{pattern}) {
				t.Errorf("%s %s paths = %v, want [%q]", surface, r.Metric, r.Paths, pattern)
			}
			if q := lokiRequestQueryFor(r); !strings.Contains(q, `request_path=~"^/api/.*$"`) {
				t.Errorf("%s %s LogQL = %s, want anchored wildcard regex", surface, r.Metric, q)
			}
		}
	}

	// Over-limit wildcard pattern rejected on every surface before reaching Loki.
	badSvc, badGot := newSurfaceSvc()
	if rec := serveREST(badSvc, "/v1/metrics/http-requests?resource=web&path="+tooManyWildcards); rec.Code != http.StatusBadRequest {
		t.Errorf("REST over-limit wildcards = %d, want 400", rec.Code)
	}
	badSchema, _ := gqlSchema(badSvc)
	if res := graphql.Do(graphql.Params{
		Schema:        badSchema,
		Context:       context.Background(),
		RequestString: `{ metrics(query: {filters: [{field:"RESOURCE",values:["web"]},{field:"PATH",values:["` + tooManyWildcards + `"]}], name:"HTTP_REQUESTS"}) { unit } }`,
	}); len(res.Errors) == 0 {
		t.Error("GraphQL over-limit wildcards must fail")
	}
	badCS := mcpSession(t, badSvc)
	if res, err := badCS.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "get_metrics",
		Arguments: map[string]any{
			"resourceId":  "web",
			"metricTypes": []string{"http_request_count"},
			"httpPath":    tooManyWildcards,
		},
	}); err == nil && (res == nil || !res.IsError) {
		t.Error("MCP over-limit wildcards must fail")
	}
	if len(*badGot) != 0 {
		t.Errorf("over-limit wildcard pattern reached Loki source %d times", len(*badGot))
	}
}
