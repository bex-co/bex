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
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Render's public API rate limits (w8/410): "Rate limits are shared for a set
// of endpoints … All but one of these rate limits apply for your user except
// for deploys which is per service." (api-docs.render.com/reference/rate-limiting,
// read 2026-11-22.) A Render-contract REST operation, and a Render MCP tool,
// spends one token of exactly one row in place of bex-api's general budget
// (BEX_RATE_LIMIT). Sandbox operations keep their own table (w22/m443), asked
// first. GraphQL is the dashboard's transport, not a Render surface, so it
// keeps the general budget; so does a Kratos session on REST, which is the
// dashboard's browser (Render's public API takes API keys only).

// renderRateRow is one row of Render's table.
type renderRateRow string

const (
	renderRowServiceCreate renderRateRow = "service_create" // POST /v1/services
	renderRowDeploy        renderRateRow = "deploy"         // update, deploy, resume, suspend, deploy hooks: per service
	renderRowCustomDomain  renderRateRow = "custom_domain"  // create and verify a custom domain
	renderRowJobs          renderRateRow = "jobs"           // POST /v1/services/{serviceId}/jobs
	renderRowLogs          renderRateRow = "logs"           // GET /v1/logs, /logs/subscribe, /logs/values
	renderRowWrite         renderRateRow = "write"          // every other POST, PATCH, PUT and DELETE
	renderRowRead          renderRateRow = "read"           // every other GET (and HEAD)
)

// renderRowRates is Render's table: requests per window.
var renderRowRates = map[renderRateRow]struct {
	requests int
	per      time.Duration
}{
	renderRowServiceCreate: {20, time.Hour},
	renderRowDeploy:        {10, time.Minute},
	renderRowCustomDomain:  {50, time.Hour},
	renderRowJobs:          {100, time.Minute},
	renderRowLogs:          {30, time.Minute},
	renderRowWrite:         {30, time.Minute},
	renderRowRead:          {400, time.Minute},
}

// renderOperationRows names the pinned spec's operations that have a row of
// their own. Every other operation takes the read row for a GET and the write
// row for anything else. Render's page lists POST, PATCH and DELETE as writes;
// a PUT (the env var and secret file writes) is one too.
var renderOperationRows = map[string]renderRateRow{
	"create-service":        renderRowServiceCreate,
	"update-service":        renderRowDeploy,
	"create-deploy":         renderRowDeploy,
	"resume-service":        renderRowDeploy,
	"suspend-service":       renderRowDeploy,
	"create-custom-domain":  renderRowCustomDomain,
	"refresh-custom-domain": renderRowCustomDomain,
	"post-job":              renderRowJobs,
	"list-logs":             renderRowLogs,
	"subscribe-logs":        renderRowLogs,
	"list-logs-values":      renderRowLogs,
}

func renderOperationRow(operationID, method string) renderRateRow {
	if row, ok := renderOperationRows[operationID]; ok {
		return row
	}
	if method == http.MethodGet {
		return renderRowRead
	}
	return renderRowWrite
}

// renderMCPToolRows gives each Render MCP tool bex serves the row of the REST
// request render-oss/render-mcp-server makes for it, one call one request:
// its tools call Render's public API with the caller's API key. The tools bex
// does not serve (mcpKnownUpstreamOnly: the workflow and workspace-selection
// tools) have none. TestEveryRenderMCPToolHasARow
// holds this against the pin.
var renderMCPToolRows = map[string]renderRateRow{
	"create_web_service":           renderRowServiceCreate,
	"create_static_site":           renderRowServiceCreate,
	"create_cron_job":              renderRowServiceCreate,
	"trigger_deploy":               renderRowDeploy,
	"list_logs":                    renderRowLogs,
	"list_log_label_values":        renderRowLogs,
	"create_postgres":              renderRowWrite,
	"create_key_value":             renderRowWrite,
	"update_environment_variables": renderRowWrite,
	"run_cron_job":                 renderRowWrite,
	"cancel_cron_job_run":          renderRowWrite,
	"create_blueprint":             renderRowWrite,
	"plan_blueprint":               renderRowWrite,
	"approve_blueprint":            renderRowWrite,
	"update_blueprint":             renderRowWrite,
	"get_blueprint":                renderRowRead,
	"get_deploy":                   renderRowRead,
	"get_key_value":                renderRowRead,
	"get_metrics":                  renderRowRead,
	"get_postgres":                 renderRowRead,
	"get_service":                  renderRowRead,
	"list_deploys":                 renderRowRead,
	"list_events":                  renderRowRead,
	"list_key_value":               renderRowRead,
	"list_postgres_instances":      renderRowRead,
	"list_services":                renderRowRead,
	"list_workspaces":              renderRowRead,
	"query_render_postgres":        renderRowRead,
}

// renderWorkspaceRatePerMinute bounds every Render row of one workspace
// together, whoever sends it. It is bex's abuse bound, as the sandbox one is
// (ADR104 D11), for API keys with no recorded creator, which are callers of
// their own: one user spending every per-user row in full sends under 600
// requests a minute.
const renderWorkspaceRatePerMinute = 10000

// RenderRateLimits holds Render's public API buckets: one keyed limiter per
// row, and the workspace bound over all of them. They are core's token
// buckets: each row admits its figure as a burst and refills continuously, and
// when SetSharedStore is wired (w8/m579) every replica shares one Postgres store.
type RenderRateLimits struct {
	// Keys resolves an API key to its creator, so a user's keys share that
	// user's rows (core.RateLimitUser).
	Keys core.KeyCreatorResolver

	rows      map[renderRateRow]*core.KeyedRateLimiter[string]
	workspace *core.KeyedRateLimiter[string]
}

// NewRenderRateLimits returns the buckets at Render's rates. A bucket is kept
// at least until it would be full again, so an hourly row cannot be reset by
// going idle for the general limiter's ten minutes.
func NewRenderRateLimits(keys core.KeyCreatorResolver) *RenderRateLimits {
	rows := make(map[renderRateRow]*core.KeyedRateLimiter[string], len(renderRowRates))
	for row, r := range renderRowRates {
		perMinute := float64(r.requests) * float64(time.Minute) / float64(r.per)
		rows[row] = core.NewKeyedRateLimiter[string](perMinute, r.requests, max(limiterIdle, r.per), limiterSweepEvery)
	}
	return &RenderRateLimits{
		Keys:      keys,
		rows:      rows,
		workspace: core.NewKeyedRateLimiter[string](renderWorkspaceRatePerMinute, 0, limiterIdle, limiterSweepEvery),
	}
}

// SetSharedStore backs every Render row and the Render workspace bound with a
// cross-replica RateLimitStore.
func (l *RenderRateLimits) SetSharedStore(store core.RateLimitStore) {
	if l == nil {
		return
	}
	for row, limiter := range l.rows {
		limiter.SetSharedStore(store, "render:"+string(row), nil)
	}
	l.workspace.SetSharedStore(store, "render:workspace", nil)
}

// ServiceDeploys is the deploy row's limiter, keyed by service id. A deploy
// hook spends it too: Render counts hooks in the same ten a minute.
func (l *RenderRateLimits) ServiceDeploys() *core.KeyedRateLimiter[string] {
	return l.rows[renderRowDeploy]
}

// renderLimited reports whether a caller's requests take Render's rows: every
// authenticated caller but a Kratos session, the dashboard's browser.
func renderLimited(ctx context.Context) bool {
	id, ok := core.IdentityFrom(ctx)
	return ok && id.Subject != "" && id.Method != core.MethodSession
}

// buckets are what one request of row spends. The deploy row is keyed by
// service, every other row by the caller's user; an operation of the deploy
// row with no service id falls back to the write row.
func (l *RenderRateLimits) buckets(ctx context.Context, row renderRateRow, serviceID, workspace string) []core.RateLimitBucket {
	key := serviceID
	if row != renderRowDeploy || serviceID == "" {
		if row == renderRowDeploy {
			row = renderRowWrite
		}
		key = core.RateLimitUser(ctx, l.Keys)
	}
	out := []core.RateLimitBucket{l.rows[row].Handle(key)}
	if workspace != "" {
		out = append(out, l.workspace.Handle(workspace))
	}
	return out
}

// renderRESTOperations is the REST mount's hook into the general limiter
// (OperationLimits): a request bex's REST mux routes to a Render operation
// spends that operation's row. A bex-native route, an unknown path and a
// session caller spend the general budget.
type renderRESTOperations struct {
	limits   *RenderRateLimits
	rest     *http.ServeMux
	contract *renderOpenAPIContract
}

func (o renderRESTOperations) RequestBuckets(r *http.Request, workspace string) []core.RateLimitBucket {
	if !renderLimited(r.Context()) {
		return nil
	}
	probe := r
	if r.Method == http.MethodHead {
		// A HEAD is a GET without its body, on the mux and on Render's table.
		get := *r
		get.Method = http.MethodGet
		probe = &get
	}
	_, pattern := o.rest.Handler(probe)
	if pattern == "" {
		return nil
	}
	route, params, _, ok := o.contract.renderOperation(pattern, probe)
	if !ok {
		return nil
	}
	row := renderOperationRow(compatOperationID(route.Operation.OperationID), probe.Method)
	return o.limits.buckets(r.Context(), row, params["serviceId"], workspace)
}

// restRateLimitMiddleware is the general limiter for the REST mount, which
// also classifies Render's operations into their rows (w8/410).
func (s *Server) restRateLimitMiddleware(restMux *http.ServeMux, contract *renderOpenAPIContract) func(http.Handler) http.Handler {
	if s.RateLimiter == nil || s.RenderRateLimits == nil {
		return s.rateLimitMiddleware()
	}
	ops := operationChain{s.RateLimiter.Operations, renderRESTOperations{limits: s.RenderRateLimits, rest: restMux, contract: contract}}
	return func(next http.Handler) http.Handler { return s.RateLimiter.middlewareWith(ops, next) }
}

// LimitRenderOperations gives the Render-contract REST operations and MCP
// tools Render's per-endpoint rates in place of the general budget, and deploy
// hooks the deploy row's per-service bucket (w8/410). It does nothing without
// a general limiter (BEX_RATE_LIMIT=0 turns both off), and must run before
// Handler().
func (s *Server) LimitRenderOperations() {
	if s.RateLimiter == nil {
		return
	}
	var keys core.KeyCreatorResolver
	if s.Apps != nil && s.Apps.Base != nil {
		keys = s.Apps.KeyCreators
	}
	s.RenderRateLimits = NewRenderRateLimits(keys)
	if s.RateLimitStore != nil {
		s.RenderRateLimits.SetSharedStore(s.RateLimitStore)
	}
	if s.Deploys != nil {
		s.Deploys.ServiceDeployRate = s.RenderRateLimits.ServiceDeploys()
	}
}

// mcpRenderRateLimits charges a Render MCP tool call to its row and gives the
// request's general token back, so the call is counted once, as its REST
// request is. A bex-only tool, a call no HTTP limiter admitted (stdio) and a
// session caller keep the general budget.
func (s *Server) mcpRenderRateLimits(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		call, ok := req.(*mcp.CallToolRequest)
		if method != "tools/call" || !ok || call.Params == nil || s.RenderRateLimits == nil || s.RateLimiter == nil {
			return next(ctx, method, req)
		}
		row, ok := renderMCPToolRows[call.Params.Name]
		scope := core.RateLimitScopeFrom(ctx)
		if !ok || scope == nil || !renderLimited(ctx) {
			return next(ctx, method, req)
		}
		var args struct {
			ServiceID string `json:"serviceId"`
		}
		if row == renderRowDeploy {
			_ = json.Unmarshal(call.Params.Arguments, &args)
		}
		now := s.RateLimiter.clock()
		scope.Refund(ctx, now)
		buckets := s.RenderRateLimits.buckets(ctx, row, args.ServiceID, scope.Workspace())
		states, err := core.ChargeRateLimit(ctx, scope.Header(), now, "rate_limited", buckets...)
		if err != nil {
			return mcpCallError(ctx, core.MCPError(err)), nil
		}
		if h := scope.Header(); h != nil {
			core.SetRateLimitHeaders(h, false, states...)
		}
		return next(ctx, method, req)
	}
}
