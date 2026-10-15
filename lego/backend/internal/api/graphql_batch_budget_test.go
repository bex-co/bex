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
