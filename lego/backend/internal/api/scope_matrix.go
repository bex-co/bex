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
	"errors"
	"net/http"
	"strings"

	"github.com/graphql-go/graphql/language/ast"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Operation ids are stable, checked-in names:
//
//	REST <ServeMux pattern>     e.g. "REST GET /v1/services/{id}"
//	GQL Query.<field> / GQL Mutation.<field>
//	MCP <tool name>
//
// classifiedOps (scope_matrix_ops.go) is the exhaustive table. An unclassified
// live operation fails the guard test; at runtime it fails closed as write.

const (
	maxScopeAuditOpLen = 256
	scopeAuditUnscoped = "workspace:unscoped"
)

// lookupScopeClass returns the classified class for op. ok is false when the
// operation is missing from the table — callers must fail closed.
func lookupScopeClass(op string) (string, bool) {
	class, ok := classifiedOps[op]
	return class, ok
}

// defaultScopeClass is the mechanical default: REST GET/HEAD, GraphQL Query,
// and MCP list_/get_ tools are read; everything else is write. Mint and
// sensitive reveals are listed in scopeClassOverrides.
func defaultScopeClass(op string) string {
	switch {
	case strings.HasPrefix(op, "REST GET "), strings.HasPrefix(op, "REST HEAD "):
		return core.OpClassRead
	case strings.HasPrefix(op, "GQL Query."):
		return core.OpClassRead
	case strings.HasPrefix(op, "MCP "):
		name := strings.TrimPrefix(op, "MCP ")
		if strings.HasPrefix(name, "list_") || strings.HasPrefix(name, "get_") {
			return core.OpClassRead
		}
		return core.OpClassWrite
	default:
		return core.OpClassWrite
	}
}

func classifiedClass(op string) string {
	if class, ok := scopeClassOverrides[op]; ok {
		return class
	}
	return defaultScopeClass(op)
}

// requireScopeClass is the one shared checker the three surfaces call. Exempt
// identities (sessions, API keys, platform clients without a granular grant)
// pass. Refusals are audited; allows are not (volume).
func (s *Server) requireScopeClass(ctx context.Context, op string) error {
	if op == "" {
		return nil
	}
	id, ok := core.IdentityFrom(ctx)
	if !ok || id.CapabilityExempt() {
		return nil
	}
	class, found := lookupScopeClass(op)
	if !found {
		class = core.OpClassWrite
	}
	err := id.RequireOpClass(class)
	if err != nil {
		s.recordScopeClassDenial(ctx, op, class)
		return err
	}
	return nil
}

func (s *Server) recordScopeClassDenial(ctx context.Context, op, class string) {
	base := s.scopeBase()
	if base == nil {
		return
	}
	resource := scopeAuditUnscoped
	if tenantID, ok := base.Tenant(ctx); ok && tenantID != "" {
		resource = core.WorkspaceObject(tenantID)
	}
	ev := core.AuditEvent{
		Verb:     core.AuditVerbScopeClass,
		Resource: resource,
		Target:   boundScopeOp(op + " " + class),
		Outcome:  core.AuditDenied,
		At:       base.Now(),
	}
	if id, ok := core.IdentityFrom(ctx); ok {
		ev.Caller, ev.CallerMethod = id.Subject, id.Method
		id.AttachOAuthProvenance(&ev)
	}
	core.RecordAuditEvent(ctx, base.Audit, ev)
}

func (s *Server) scopeBase() *core.Base {
	if s == nil {
		return nil
	}
	if s.Apps != nil {
		return s.Apps.Base
	}
	if s.APIKeys != nil {
		return s.APIKeys.Base
	}
	return nil
}

func boundScopeOp(op string) string {
	op = strings.TrimSpace(op)
	if len(op) > maxScopeAuditOpLen {
		return op[:maxScopeAuditOpLen]
	}
	return op
}

func (s *Server) withScopeClassREST(mux *http.ServeMux, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id, ok := core.IdentityFrom(r.Context()); ok && id.CapabilityExempt() {
			next.ServeHTTP(w, r)
			return
		}
		_, pattern := mux.Handler(r)
		if pattern != "" {
			if err := s.requireScopeClass(r.Context(), "REST "+pattern); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func writeGraphQLErrors(w http.ResponseWriter, err error) {
	entry := map[string]any{"message": err.Error()}
	var coded *core.CodedError
	if errors.As(err, &coded) {
		if ext := coded.Extensions(); len(ext) > 0 {
			entry["extensions"] = ext
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"errors": []map[string]any{entry},
	})
}

// graphqlTopLevelOps returns GQL Query.x / GQL Mutation.x for each top-level
// field of the selected operation. Parse failures return nil so graphql.Do
// reports the canonical parse error. Introspection meta-fields (__schema) are
// omitted — they are always read-class and not in the table.
func graphqlTopLevelOps(query, operationName string) []string {
	fragments, ops, ok := parseGraphQLDocument(query)
	if !ok {
		return nil
	}
	if operationName != "" {
		filtered := ops[:0]
		for _, op := range ops {
			if op.Name != nil && op.Name.Value == operationName {
				filtered = append(filtered, op)
			}
		}
		ops = filtered
	}
	var out []string
	for _, op := range ops {
		kind := "Query"
		if op.Operation == "mutation" {
			kind = "Mutation"
		}
		collectGraphQLTopLevel(op.SelectionSet, fragments, kind, map[string]bool{}, &out)
	}
	return out
}

func collectGraphQLTopLevel(sel *ast.SelectionSet, fragments map[string]*ast.FragmentDefinition, kind string, visiting map[string]bool, out *[]string) {
	if sel == nil {
		return
	}
	for _, s := range sel.Selections {
		switch n := s.(type) {
		case *ast.Field:
			if n.Name == nil || strings.HasPrefix(n.Name.Value, "__") {
				continue
			}
			*out = append(*out, "GQL "+kind+"."+n.Name.Value)
		case *ast.InlineFragment:
			collectGraphQLTopLevel(n.SelectionSet, fragments, kind, visiting, out)
		case *ast.FragmentSpread:
			if n.Name == nil {
				continue
			}
			name := n.Name.Value
			if visiting[name] {
				continue
			}
			frag, ok := fragments[name]
			if !ok {
				continue
			}
			visiting[name] = true
			collectGraphQLTopLevel(frag.SelectionSet, fragments, kind, visiting, out)
			delete(visiting, name)
		}
	}
}

func (s *Server) requireGraphQLScope(ctx context.Context, query, operationName string) error {
	if id, ok := core.IdentityFrom(ctx); !ok || id.CapabilityExempt() {
		return nil
	}
	for _, op := range graphqlTopLevelOps(query, operationName) {
		if err := s.requireScopeClass(ctx, op); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) requireMCPScope(ctx context.Context, tool string) error {
	if tool == "" {
		return nil
	}
	return s.requireScopeClass(ctx, "MCP "+tool)
}
