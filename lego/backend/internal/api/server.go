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

// Package api is the composition root of bex-api: it wires the feature services
// (apps, logs, metrics, apikeys, postgres) behind one auth gate and assembles
// the three transports as SINGLE artifacts — one REST router, one GraphQL
// schema, one MCP registry. Each feature contributes registration fragments
// (RegisterREST / GraphQLQuery+GraphQLMutation / RegisterMCP); the root merges
// them, so a verb reachable over one surface is reachable over all three and the
// surfaces cannot drift. The root imports the features + core; features never
// import the root (no cycle).
package api

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/url"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/bex-co/bex/lego/backend/internal/postgres"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
)

const (
	mcpServerName = "bex"
	mcpVersion    = "0.1.0"
)

const errNoHydraURL = "bex-api: BEX_HYDRA_ADMIN_URL must be set (refusing to serve without a token validator)"

// Server wires the feature services over one auth gate and assembles the three
// surfaces. All surfaces mount on the same mux, share the auth middleware, and
// call identical Service methods — so they cannot diverge in behavior.
type Server struct {
	Apps     *apps.Service
	Logs     *logs.Service
	Metrics  *metrics.Service
	APIKeys  *apikeys.Service
	Postgres *postgres.Service
	Secrets  *secrets.Service

	CORSOrigin string // comma-separated allowed origins; empty => no CORS

	HydraAdminURL string // Hydra admin base URL (introspection); required
	KratosURL     string // Kratos public base URL (whoami); empty disables sessions

	// OAuth 2.1 resource-server discovery (w4/m9, MCP authorization spec).
	// OAuthIssuer is Hydra's public issuer (e.g. https://oauth.bex.co);
	// OAuthResource is this API's canonical resource URI (e.g.
	// https://api.bex.co/mcp) — advertised via RFC 9728 protected-resource
	// metadata and enforced as the expected token audience. Both unset =>
	// behavior is byte-identical to before (no metadata endpoint, bare
	// WWW-Authenticate, no audience check).
	OAuthIssuer   string
	OAuthResource string

	// WebhookSecret is the shared HMAC-SHA256 key the git push webhook verifies
	// signatures against; empty disables the endpoint (it 503s). The webhook sits
	// OUTSIDE the OAuth gate — its signature is its authentication.
	WebhookSecret string

	schema graphql.Schema
}

// Deps bundles the injected backends the feature services need — the seams that
// keep the domain layer clientset/HTTP-free (nil leaves a verb reporting its
// "…Unavailable" sentinel). NewServer wires them onto the services in one place.
type Deps struct {
	PodLogs              logs.PodLogSource
	PodLogsFollow        logs.PodLogStream
	ResourceMetrics      metrics.ResourceMetricsSource
	ResourceMetricsRange metrics.ResourceMetricsRangeSource
	RequestMetrics       metrics.RequestMetricsSource
	MonthToDateBandwidth metrics.MonthToDateBandwidthSource
	MetricsFilterValues  metrics.MetricsFilterValuesSource
	APIKeys              apikeys.APIKeyStore
	Store                apps.IntentStore
	Secrets              secrets.SecretStore
}

// NewServer wires the five feature services over one core.Base + deps. Callers
// set the HTTP config fields (CORSOrigin/HydraAdminURL/KratosURL) on the result.
func NewServer(base *core.Base, d Deps) *Server {
	return &Server{
		Apps: &apps.Service{Base: base, Store: d.Store},
		Logs: &logs.Service{Base: base, PodLogs: d.PodLogs, PodLogsFollow: d.PodLogsFollow},
		Metrics: &metrics.Service{
			Base:                       base,
			ResourceMetrics:            d.ResourceMetrics,
			ResourceMetricsRange:       d.ResourceMetricsRange,
			RequestMetrics:             d.RequestMetrics,
			MonthToDateBandwidthSource: d.MonthToDateBandwidth,
			MetricsFilterValuesSource:  d.MetricsFilterValues,
		},
		APIKeys:  &apikeys.Service{Base: base, APIKeys: d.APIKeys},
		Postgres: &postgres.Service{Base: base},
		Secrets:  &secrets.Service{Base: base, Store: d.Secrets},
	}
}

// Feature registration contracts. A feature implements the fragments it has; the
// root type-asserts each service against these when assembling the surfaces, so a
// feature with no mutations (logs, metrics) simply omits GraphQLMutation.
type (
	restRegistrar       interface{ RegisterREST(*http.ServeMux) }
	gqlQueryProvider    interface{ GraphQLQuery() graphql.Fields }
	gqlMutationProvider interface{ GraphQLMutation() graphql.Fields }
	mcpRegistrar        interface{ RegisterMCP(*mcp.Server) }
)

// features lists the wired (non-nil) feature services in a stable order. A typed
// nil stored in an interface is not == nil, so each is checked explicitly.
func (s *Server) features() []any {
	var out []any
	if s.Apps != nil {
		out = append(out, s.Apps)
	}
	if s.Logs != nil {
		out = append(out, s.Logs)
	}
	if s.Metrics != nil {
		out = append(out, s.Metrics)
	}
	if s.APIKeys != nil {
		out = append(out, s.APIKeys)
	}
	if s.Postgres != nil {
		out = append(out, s.Postgres)
	}
	if s.Secrets != nil {
		out = append(out, s.Secrets)
	}
	return out
}

// Handler returns the fully wired http.Handler, or an error if misconfigured
// (missing Hydra URL, or an invalid GraphQL schema). Routes:
//
//	GET  /healthz                              (open)
//	GET  /v1/services, /v1/services/{id}       (auth)   REST
//	POST /v1/services/{id}/{suspend|resume|restart}  (auth)   REST
//	POST /graphql                              (auth)   GraphQL
//	     /mcp                                  (auth)   MCP (streamable-http)
func (s *Server) Handler() (http.Handler, error) {
	auth, err := s.authMiddleware()
	if err != nil {
		return nil, err
	}
	schema, err := s.newSchema()
	if err != nil {
		return nil, err
	}
	s.schema = schema

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// The git push webhook authenticates by HMAC signature, not the OAuth gate,
	// so it mounts directly (ahead of the /v1/ wildcard — a more specific pattern
	// wins in net/http's mux). A git host can't present a bearer token.
	if s.Apps != nil {
		mux.Handle("POST /v1/webhooks/git", &apps.GitWebhook{Svc: s.Apps, Secret: s.WebhookSecret})
	}
	// All three adapters sit behind the same auth gate.
	mux.Handle("/v1/", auth(s.restHandler()))
	mux.Handle("/graphql", auth(s.graphqlHandler()))
	mux.Handle("/mcp", auth(s.mcpHTTPHandler()))

	// RFC 9728 protected-resource metadata (w4/m9): open by design — it's how an
	// unauthenticated MCP client discovers the authorization server (the MCP
	// authorization spec requires it). One predicate decides both this mount and
	// the 401 WWW-Authenticate enrichment (resourceMetadataURL), so the hint and
	// the endpoint can't drift.
	if s.resourceMetadataURL() != "" {
		mux.HandleFunc("GET /.well-known/oauth-protected-resource", func(w http.ResponseWriter, _ *http.Request) {
			core.WriteJSON(w, http.StatusOK, map[string]any{
				"resource":                 s.OAuthResource,
				"authorization_servers":    []string{s.OAuthIssuer},
				"bearer_methods_supported": []string{"header"},
			})
		})
	}

	return withCORS(s.CORSOrigin, mux), nil
}

// authMiddleware builds the auth gate, validating its configuration up front so a
// misconfigured binary refuses to start.
func (s *Server) authMiddleware() (func(http.Handler) http.Handler, error) {
	if s.HydraAdminURL == "" {
		return nil, core.Err(errNoHydraURL)
	}
	return newOryAuth(s.HydraAdminURL, s.KratosURL, s.OAuthResource, s.resourceMetadataURL()).middleware, nil
}

// resourceMetadataURL derives the public URL of this API's RFC 9728 metadata
// endpoint from the resource URI (same scheme+host, well-known path) — what the
// 401 WWW-Authenticate header advertises. Empty when discovery isn't configured
// or the resource URI doesn't parse.
func (s *Server) resourceMetadataURL() string {
	if s.OAuthIssuer == "" || s.OAuthResource == "" {
		return ""
	}
	u, err := url.Parse(s.OAuthResource)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host + "/.well-known/oauth-protected-resource"
}

// restHandler mounts every feature's REST fragment on one mux — the single REST
// router (Render-public-API compatible), served under /v1.
func (s *Server) restHandler() http.Handler {
	mux := http.NewServeMux()
	for _, f := range s.features() {
		if r, ok := f.(restRegistrar); ok {
			r.RegisterREST(mux)
		}
	}
	return mux
}

// newSchema merges every feature's GraphQL fragments into the single root Query
// and Mutation objects — the one schema the /graphql handler serves.
func (s *Server) newSchema() (graphql.Schema, error) {
	query := graphql.Fields{}
	mutation := graphql.Fields{}
	for _, f := range s.features() {
		if p, ok := f.(gqlQueryProvider); ok {
			maps.Copy(query, p.GraphQLQuery())
		}
		if p, ok := f.(gqlMutationProvider); ok {
			maps.Copy(mutation, p.GraphQLMutation())
		}
	}
	return graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: query}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: mutation}),
	})
}

// graphqlHandler serves POST /graphql over the compiled schema. The request
// context already carries the caller Identity (attached by the auth middleware),
// which the feature resolvers' authorize gate reads.
func (s *Server) graphqlHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query         string         `json:"query"`
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}
		// Env-var reads nest under the apps Service type but live in the secrets
		// feature; inject the reader so those resolvers reach it via context (the
		// shared Service GraphQL type stays stateless — no per-server closure).
		ctx := r.Context()
		if s.Secrets != nil {
			ctx = core.WithEnvVars(ctx, s.Secrets)
		}
		result := graphql.Do(graphql.Params{
			Schema:         s.schema,
			RequestString:  body.Query,
			OperationName:  body.OperationName,
			VariableValues: body.Variables,
			Context:        ctx,
		})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})
}

// MCPServer builds the MCP server with every feature's tools registered. The
// returned server is stateless w.r.t. sessions, so one instance is reused for
// stdio and across HTTP sessions.
func (s *Server) MCPServer() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: mcpServerName, Version: mcpVersion}, nil)
	for _, f := range s.features() {
		if r, ok := f.(mcpRegistrar); ok {
			r.RegisterMCP(srv)
		}
	}
	return srv
}

// mcpHTTPHandler serves the MCP streamable-HTTP transport (mounted at /mcp behind
// the same auth gate as REST/GraphQL).
func (s *Server) mcpHTTPHandler() http.Handler {
	srv := s.MCPServer()
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
}

// RunStdio serves the MCP adapter over stdio — the transport a local agent
// launches bex as a subprocess with. The trust boundary is the process itself
// (no bearer applies); the HTTP transport keeps the gate. Blocks until the
// client disconnects or ctx is cancelled.
func (s *Server) RunStdio(ctx context.Context) error {
	return s.MCPServer().Run(ctx, &mcp.StdioTransport{})
}
