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
	"net/http"

	"github.com/graphql-go/graphql"
)

// Server wires the adapters (REST + GraphQL + MCP) over one Core and one auth
// token. All surfaces mount on the same mux, share the bearer middleware, and
// call identical Core methods — so they cannot diverge in behavior.
type Server struct {
	Core       *Core
	Token      string // bearer token; empty => Handler refuses to build
	CORSOrigin string // exact allowed origin; empty => no CORS

	schema graphql.Schema
}

// Handler returns the fully wired http.Handler, or an error if misconfigured
// (empty token, or an invalid GraphQL schema). Routes (REST is Render-public-API
// compatible, served under both /v1/services and /v1/apps):
//
//	GET  /healthz                              (open)
//	GET  /v1/services, /v1/services/{id}       (bearer)   REST
//	POST /v1/services/{id}/{suspend|resume|restart}  (bearer)   REST
//	POST /graphql                              (bearer)   GraphQL
//	     /mcp                                  (bearer)   MCP (streamable-http)
func (s *Server) Handler() (http.Handler, error) {
	if s.Token == "" {
		return nil, errEmptyToken
	}
	schema, err := newSchema()
	if err != nil {
		return nil, err
	}
	s.schema = schema

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	// All three adapters sit behind the same bearer gate.
	mux.Handle("/v1/", bearerAuth(s.Token, s.restHandler()))
	mux.Handle("/graphql", bearerAuth(s.Token, s.graphqlHandler()))
	mux.Handle("/mcp", bearerAuth(s.Token, s.mcpHTTPHandler()))

	return withCORS(s.CORSOrigin, mux), nil
}

type apiError string

func (e apiError) Error() string { return string(e) }

const errEmptyToken = apiError("bex-api: BEX_API_TOKEN must be set (refusing to serve without auth)")
