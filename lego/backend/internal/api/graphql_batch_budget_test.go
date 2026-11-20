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
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// budgetServer's schema has one mutation, step(name, ms), that records its start and
// then works for ms milliseconds without watching ctx — the shape of a write
// that cannot be interrupted once dispatched — and one query, ping.
type budgetProbe struct {
	mu      sync.Mutex
	started []string
}

func (p *budgetProbe) record(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.started = append(p.started, name)
}

func (p *budgetProbe) names() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.started...)
}

func budgetServer(t *testing.T, budget time.Duration) (*Server, *budgetProbe) {
	t.Helper()
	probe := &budgetProbe{}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: graphql.Fields{
			"ping": {Type: graphql.String, Resolve: func(graphql.ResolveParams) (any, error) { return "pong", nil }},
		}}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: graphql.Fields{
			"step": {
				Type: graphql.String,
				Args: graphql.FieldConfigArgument{
					"name": {Type: graphql.NewNonNull(graphql.String)},
					"ms":   {Type: graphql.NewNonNull(graphql.Int)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					name := p.Args["name"].(string)
					probe.record(name)
					time.Sleep(time.Duration(p.Args["ms"].(int)) * time.Millisecond)
					return name, nil
				},
			},
		}}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return &Server{schema: schema, gqlBudget: budget}, probe
}

func stepOp(name string, ms int) string {
	return `{"query":"mutation { step(name: \"` + name + `\", ms: ` + strconv.Itoa(ms) + `) }"}`
}

type wireResult struct {
	Data   map[string]any `json:"data"`
	Errors []struct {
		Message    string         `json:"message"`
		Extensions map[string]any `json:"extensions"`
	} `json:"errors"`
}

func errCode(r wireResult) string {
	if len(r.Errors) == 0 {
		return ""
	}
	code, _ := r.Errors[0].Extensions["code"].(string)
	return code
}

// The a570 failure, at test scale: three mutations, each well inside the
// per-document budget, sent as one Apollo batch to a server whose write
// deadline sits just above that budget. Per-member budgets summed past the
// write deadline and the client read nothing. With one shared deadline the
// client reads an ordered result per member: the completed one keeps its data,
// the one running at the deadline is WRITE_OUTCOME_UNKNOWN, and the one never
// dispatched is OPERATION_NOT_STARTED and never ran (w4/m211).
func TestGraphQLBatchSharesOneBudgetInsideTheWriteDeadline(t *testing.T) {
	const budget = 300 * time.Millisecond
	srv, probe := budgetServer(t, budget)
	ts := httptest.NewUnstartedServer(srv.graphqlHandler())
	ts.Config.WriteTimeout = budget + 400*time.Millisecond
	ts.Start()
	defer ts.Close()

	body := "[" + stepOp("first", 250) + "," + stepOp("second", 250) + "," + stepOp("third", 250) + "]"
	start := time.Now()
	resp, err := ts.Client().Post(ts.URL, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("batch lost its response: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("batch response cut off after %v: %v", time.Since(start), err)
	}
	var results []wireResult
	if err := json.Unmarshal(raw, &results); err != nil || len(results) != 3 {
		t.Fatalf("results = %s (%v), want three ordered slots", raw, err)
	}
	if results[0].Data["step"] != "first" || len(results[0].Errors) != 0 {
		t.Errorf("completed member = %+v, want its data", results[0])
	}
	if got := errCode(results[1]); got != core.CodeWriteOutcomeUnknown {
		t.Errorf("interrupted member code = %q (%+v), want %s", got, results[1], core.CodeWriteOutcomeUnknown)
	}
	if got := errCode(results[2]); got != core.CodeOperationNotStarted || !strings.Contains(results[2].Errors[0].Message, "did not start") {
		t.Errorf("unstarted member = %+v, want %s saying it did not start", results[2], core.CodeOperationNotStarted)
	}
	// The second resolver finishes late, after its slot was answered; the
	// third must never have been entered.
	time.Sleep(250 * time.Millisecond)
	if got := probe.names(); strings.Join(got, ",") != "first,second" {
		t.Errorf("resolvers started = %v, want first,second only", got)
	}
}

// Members that finish inside the shared budget keep their normal results, in
// order, and an already-expired request dispatches nothing.
func TestGraphQLBatchControls(t *testing.T) {
	t.Run("fast mixed batch", func(t *testing.T) {
		srv, probe := budgetServer(t, time.Second)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(
			"["+stepOp("a", 1)+`,{"query":"{ ping }"},`+stepOp("b", 1)+"]"))
		srv.graphqlHandler().ServeHTTP(rec, req)
		var results []wireResult
		if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil || len(results) != 3 {
			t.Fatalf("results = %s (%v)", rec.Body, err)
		}
		if results[0].Data["step"] != "a" || results[1].Data["ping"] != "pong" || results[2].Data["step"] != "b" {
			t.Errorf("results = %+v, want a, pong, b", results)
		}
		if got := probe.names(); strings.Join(got, ",") != "a,b" {
			t.Errorf("started = %v", got)
		}
	})

	t.Run("expired before dispatch", func(t *testing.T) {
		srv, probe := budgetServer(t, time.Second)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader("["+stepOp("a", 1)+"]")).WithContext(ctx)
		_, _, outcome := srv.serveGraphQL(rec, req)
		var results []wireResult
		if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil || len(results) != 1 {
			t.Fatalf("results = %s (%v)", rec.Body, err)
		}
		if errCode(results[0]) != core.CodeOperationNotStarted || outcome != gqlOutcomeUnavailable {
			t.Errorf("result = %+v outcome %q, want %s / %s", results[0], outcome, core.CodeOperationNotStarted, gqlOutcomeUnavailable)
		}
		if got := probe.names(); len(got) != 0 {
			t.Errorf("started = %v, want none", got)
		}
	})

	t.Run("single mutation keeps its own budget and outcome", func(t *testing.T) {
		srv, _ := budgetServer(t, 50*time.Millisecond)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(stepOp("solo", 200)))
		srv.graphqlHandler().ServeHTTP(rec, req)
		var result wireResult
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("result = %s (%v)", rec.Body, err)
		}
		if errCode(result) != core.CodeWriteOutcomeUnknown {
			t.Errorf("single timed-out mutation = %+v, want %s", result, core.CodeWriteOutcomeUnknown)
		}
	})
}

// graphqlBatch is a JSON array of one GraphQL request body per document.
func graphqlBatch(docs ...string) string {
	parts := make([]string, len(docs))
	for i, doc := range docs {
		b, _ := json.Marshal(graphqlRequestBody{Query: doc})
		parts[i] = string(b)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// A batch costs one rate-limit token per member, as its members would sent
// one by one: one 32-member POST bought 32 documents' work for one token.
func TestGraphQLBatchCostsATokenPerMember(t *testing.T) {
	srv, _ := budgetServer(t, time.Second)
	srv.RateLimiter = NewRateLimiter(1, 40) // one a minute: nothing refills mid-test
	h := srv.RateLimiter.Middleware(srv.graphqlHandler())
	id := core.Identity{Subject: "batcher", Method: "oauth2"}
	post := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, r.WithContext(core.WithIdentity(r.Context(), id)))
		return rec
	}
	ping := `{"query":"{ ping }"}`
	batch := "[" + strings.TrimSuffix(strings.Repeat(ping+",", 32), ",") + "]"

	rec := post(batch)
	var results []wireResult
	if err := json.Unmarshal(rec.Body.Bytes(), &results); rec.Code != http.StatusOK || err != nil || len(results) != 32 || results[31].Data["ping"] != "pong" {
		t.Fatalf("first batch = %d %.200s, want 32 answers", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("RateLimit-Remaining"); got != "8" {
		t.Errorf("RateLimit-Remaining after a 32-member batch = %q, want 8 of 40", got)
	}
	// 8 tokens left: the next 32-member batch is refused whole, before any
	// member runs. It keeps only the token every request is charged.
	rec = post(batch)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" || !strings.Contains(rec.Body.String(), "RATE_LIMITED") {
		t.Fatalf("second batch = %d %v %s, want 429 RATE_LIMITED with Retry-After", rec.Code, rec.Header(), rec.Body)
	}
	// A single document and a one-member batch still cost one token each.
	for _, body := range []string{ping, "[" + ping + "]"} {
		if rec = post(body); rec.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", body, rec.Code, rec.Body)
		}
	}
	if got := rec.Header().Get("RateLimit-Remaining"); got != "5" {
		t.Errorf("RateLimit-Remaining = %q, want 5: the refused batch took one token", got)
	}
}

// The batches here kept graphql-go's validation busy for half a minute or
// more per request: their members' costs added up, one token each. A
// batch's members now draw on one validation budget; the members past it
// are refused before graphql-go reads them, in their own result slots.
func TestGraphQLBatchValidationIsBounded(t *testing.T) {
	srv, _ := scopedKeyServer(t)
	// An invalid argument value nested 9 deep, of about n bytes: graphql-go
	// prints it twice to name it, at about 16 µs a byte.
	invalidValue := func(n int) string {
		obj := strings.Repeat("{a:", 9) + "1" + strings.Repeat("}", 9) + ","
		return "{ __type(name: [" + strings.Repeat(obj, n/len(obj)) + "]) { name } }"
	}
	repeat := func(n int, doc string) []string {
		docs := make([]string, n)
		for i := range docs {
			docs[i] = doc
		}
		return docs
	}
	// 30 KB and 10,000 errors.
	manyErrors := "{ __typename\n" + strings.Repeat("@a\n", 10000) + "}"
	// A response name repeated 10 times at each of three levels, the leaves
	// with a variable argument: 2.8 s alone.
	overlap := "query($v: Int) { " + strings.Repeat("__schema { "+strings.Repeat("types { "+strings.Repeat("name(a: $v) ", 10)+"} ", 10)+"} ", 10) + "}"
	for _, tc := range []struct {
		name string
		docs []string
		// refused is how many members the budget refuses, with this message.
		refused int
		want    string
	}{{
		name:    "32 invalid 4 KB values",
		docs:    repeat(32, invalidValue(4000)),
		refused: 30,
		want:    "query rejected: the request's argument and default values are larger than 8192 bytes together; pass large inputs as variables",
	}, {
		name:    "an 8 KB invalid value and 30 KB documents of errors",
		docs:    append([]string{invalidValue(8000)}, repeat(5, manyErrors)...),
		refused: 1,
		want:    "query rejected: the request's documents are larger than 131072 bytes together",
	}, {
		name:    "overlapping documents",
		docs:    repeat(9, overlap),
		refused: 9,
		want:    "query rejected: the request's fields that share a response name compare more than 1024 bytes of argument values",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(graphqlBatch(tc.docs...)))
			start := time.Now()
			finishesWithin(t, tc.name, func() { srv.serveGraphQL(rec, req) })
			// About 0.4 s on a laptop for the slowest; the slack is for CI.
			if took := time.Since(start); took > 2*time.Second {
				t.Errorf("took %v, want about a second at most", took)
			}
			var results []wireResult
			if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil || len(results) != len(tc.docs) {
				t.Fatalf("%d results (%v), want one per member", len(results), err)
			}
			refused := 0
			for _, r := range results {
				if len(r.Errors) > 0 && r.Errors[0].Message == tc.want {
					refused++
				}
			}
			if refused != tc.refused {
				t.Errorf("%d members refused with %q, want %d", refused, tc.want, tc.refused)
			}
		})
	}
}
