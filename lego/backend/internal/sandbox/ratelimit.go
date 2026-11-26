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

package sandbox

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

// Render's sandbox API rate limits (w22/m443, ADR104 D7): "These limits apply
// per user, shared across that user's API keys. Requests over a limit fail
// with HTTP 429." (render.com/docs/sandboxes-limits § API rate limits, read
// 2026-11-17.) A sandbox operation spends one token of exactly one class and
// none of bex-api's general per-subject budget (BEX_RATE_LIMIT), whichever of
// REST, GraphQL and MCP carries it.

// RateClass is one row of Render's table.
type RateClass string

const (
	// RateClassCreate is "Create a sandbox". A resume starts a sandbox and
	// takes a create's admission, so it is counted here (bex's assignment:
	// Render's table has no resume row).
	RateClassCreate RateClass = "create"
	// RateClassTerminate is "Terminate a sandbox", and bex's pause.
	RateClassTerminate RateClass = "terminate"
	// RateClassExec is "Execute a command": one unit per command, charged at
	// the single-step exec or at the run connect-token mint, whose token runs
	// one command once.
	RateClassExec RateClass = "exec"
	// RateClassFiles is "Upload or download a file or directory", charged at
	// the file connect-token mint (one token, one transfer).
	RateClassFiles RateClass = "files"
	// RateClassRead is "Read sandbox and sandbox group resources": sandboxes,
	// their execution history, groups, and a group's images and snapshots.
	RateClassRead RateClass = "read"
	// RateClassGroupWrite is "Write sandbox group resources": groups, and the
	// images and snapshots a group holds (import, delete).
	RateClassGroupWrite RateClass = "group_write"
	// RateClassCapture is "Capture a snapshot". The store's count of a
	// requester's accepted captures in the last minute
	// (DefaultSnapshotCapturesPerMinute, w32/m285) is the same figure and
	// stays: it holds across replicas and restarts, where this bucket counts
	// requests in one process.
	RateClassCapture RateClass = "capture"
)

// rateClassPerMinute is Render's table: requests per minute, per user.
var rateClassPerMinute = map[RateClass]int{
	RateClassCreate:     100,
	RateClassTerminate:  1000,
	RateClassExec:       1000,
	RateClassFiles:      1000,
	RateClassRead:       400,
	RateClassGroupWrite: 100,
	RateClassCapture:    DefaultSnapshotCapturesPerMinute,
}

// sandboxWorkspaceRatePerMinute bounds every sandbox operation of one
// workspace together, whoever sends it. It is bex's abuse bound (ADR104 D11),
// not a Render limit: one user spending every class in full sends 3,610
// requests a minute, so the bound never refuses a caller Render's table
// admits unless three members run every class at its limit at once.
const sandboxWorkspaceRatePerMinute = 10000

const (
	rateLimiterIdle       = 10 * time.Minute
	rateLimiterSweepEvery = 5 * time.Minute
)

// The refusal's code is the general limiter's on each surface: GraphQL's
// extensions.code, and the id of the 429 body REST and MCP share.
const (
	gqlRateLimitedCode = "RATE_LIMITED"
	mcpRateLimitedCode = "rate_limited"
)

// rateClasses assigns every sandbox operation bex serves to one class, named
// as the API's scope matrix names it. An operation missing here cannot be
// mounted: registration panics (handle, limitedFields, addTool).
//
// A group's brokered credentials (ADR122) are sandbox group resources: their
// reads and writes take those two rows.
var rateClasses = map[string]RateClass{
	"REST POST /v1/sandboxes":                              RateClassCreate,
	"REST POST /v1/sandboxes/{id}/resume":                  RateClassCreate,
	"REST POST /v1/sandboxes/{id}/terminate":               RateClassTerminate,
	"REST POST /v1/sandboxes/{id}/suspend":                 RateClassTerminate,
	"REST POST /v1/sandboxes/{id}/exec":                    RateClassExec,
	"REST POST /v1/sandboxes/{id}/runs/{operation}/token":  RateClassExec,
	"REST POST /v1/sandboxes/{id}/files/{operation}/token": RateClassFiles,
	"REST POST /v1/sandboxes/{id}/snapshots":               RateClassCapture,

	"REST GET /v1/sandboxes":                                                           RateClassRead,
	"REST GET /v1/sandboxes/{id}":                                                      RateClassRead,
	"REST GET /v1/sandboxes/{id}/execs":                                                RateClassRead,
	"REST GET /v1/sandboxes/{id}/execs/{execId}":                                       RateClassRead,
	"REST GET /v1/sandbox-groups":                                                      RateClassRead,
	"REST GET /v1/sandbox-groups/{sandboxGroupId}":                                     RateClassRead,
	"REST GET /v1/sandbox-groups/{sandboxGroupId}/builds":                              RateClassRead,
	"REST GET /v1/sandbox-groups/{sandboxGroupId}/builds/{buildId}":                    RateClassRead,
	"REST GET /v1/sandbox-groups/{sandboxGroupId}/snapshots":                           RateClassRead,
	"REST GET /v1/sandbox-groups/{sandboxGroupId}/snapshots/{snapshotId}":              RateClassRead,
	"REST GET /v1/sandbox-groups/{sandboxGroupId}/credentials":                         RateClassRead,
	"REST GET /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}":          RateClassRead,
	"REST GET /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}/versions": RateClassRead,

	"REST POST /v1/sandbox-groups":                                                     RateClassGroupWrite,
	"REST PATCH /v1/sandbox-groups/{sandboxGroupId}":                                   RateClassGroupWrite,
	"REST DELETE /v1/sandbox-groups/{sandboxGroupId}":                                  RateClassGroupWrite,
	"REST POST /v1/sandbox-groups/{sandboxGroupId}/set-default":                        RateClassGroupWrite,
	"REST POST /v1/sandbox-groups/{sandboxGroupId}/builds":                             RateClassGroupWrite,
	"REST DELETE /v1/sandbox-groups/{sandboxGroupId}/builds/{buildId}":                 RateClassGroupWrite,
	"REST DELETE /v1/sandbox-groups/{sandboxGroupId}/snapshots/{snapshotId}":           RateClassGroupWrite,
	"REST POST /v1/sandbox-groups/{sandboxGroupId}/credentials":                        RateClassGroupWrite,
	"REST PATCH /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}":        RateClassGroupWrite,
	"REST DELETE /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}":       RateClassGroupWrite,
	"REST POST /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}/restore": RateClassGroupWrite,

	"GQL Mutation.createSandbox":                 RateClassCreate,
	"GQL Mutation.suspendSandbox":                RateClassTerminate,
	"GQL Mutation.terminateSandbox":              RateClassTerminate,
	"GQL Mutation.createSandboxSnapshot":         RateClassCapture,
	"GQL Query.sandboxes":                        RateClassRead,
	"GQL Query.sandbox":                          RateClassRead,
	"GQL Query.sandboxExecutions":                RateClassRead,
	"GQL Query.sandboxExecution":                 RateClassRead,
	"GQL Query.sandboxGroups":                    RateClassRead,
	"GQL Query.sandboxGroup":                     RateClassRead,
	"GQL Query.sandboxGroupCredentials":          RateClassRead,
	"GQL Query.sandboxGroupCredential":           RateClassRead,
	"GQL Query.sandboxGroupCredentialVersions":   RateClassRead,
	"GQL Query.sandboxImages":                    RateClassRead,
	"GQL Query.sandboxImage":                     RateClassRead,
	"GQL Query.sandboxSnapshots":                 RateClassRead,
	"GQL Query.sandboxesForGroup":                RateClassRead,
	"GQL Query.sandboxGroupRuns":                 RateClassRead,
	"GQL Query.sandboxEvents":                    RateClassRead,
	"GQL Query.sandboxSnapshotsForGroup":         RateClassRead,
	"GQL Query.sandboxBuildSources":              RateClassRead,
	"GQL Query.sandboxSnapshot":                  RateClassRead,
	"GQL Mutation.createSandboxGroup":            RateClassGroupWrite,
	"GQL Mutation.updateSandboxGroup":            RateClassGroupWrite,
	"GQL Mutation.deleteSandboxGroup":            RateClassGroupWrite,
	"GQL Mutation.setDefaultSandboxGroup":        RateClassGroupWrite,
	"GQL Mutation.setSandboxGroupEnvironment":    RateClassGroupWrite,
	"GQL Mutation.createSandboxGroupCredential":  RateClassGroupWrite,
	"GQL Mutation.updateSandboxGroupCredential":  RateClassGroupWrite,
	"GQL Mutation.deleteSandboxGroupCredential":  RateClassGroupWrite,
	"GQL Mutation.restoreSandboxGroupCredential": RateClassGroupWrite,
	"GQL Mutation.importSandboxImage":            RateClassGroupWrite,
	"GQL Mutation.deleteSandboxImage":            RateClassGroupWrite,
	"GQL Mutation.deleteSandboxSnapshot":         RateClassGroupWrite,

	"MCP spawn_sandbox":                          RateClassCreate,
	"MCP stop_sandbox":                           RateClassTerminate,
	"MCP suspend_sandbox":                        RateClassTerminate,
	"MCP sandbox_exec":                           RateClassExec,
	"MCP create_sandbox_snapshot":                RateClassCapture,
	"MCP list_sandboxes":                         RateClassRead,
	"MCP list_sandbox_executions":                RateClassRead,
	"MCP list_sandbox_groups":                    RateClassRead,
	"MCP get_sandbox_group":                      RateClassRead,
	"MCP list_sandbox_group_credentials":         RateClassRead,
	"MCP get_sandbox_group_credential":           RateClassRead,
	"MCP list_sandbox_group_credential_versions": RateClassRead,
	"MCP list_sandbox_images":                    RateClassRead,
	"MCP get_sandbox_image":                      RateClassRead,
	"MCP list_sandbox_snapshots":                 RateClassRead,
	"MCP get_sandbox_snapshot":                   RateClassRead,
	"MCP create_sandbox_group":                   RateClassGroupWrite,
	"MCP update_sandbox_group":                   RateClassGroupWrite,
	"MCP delete_sandbox_group":                   RateClassGroupWrite,
	"MCP set_default_sandbox_group":              RateClassGroupWrite,
	"MCP set_sandbox_group_environment":          RateClassGroupWrite,
	"MCP create_sandbox_group_credential":        RateClassGroupWrite,
	"MCP update_sandbox_group_credential":        RateClassGroupWrite,
	"MCP delete_sandbox_group_credential":        RateClassGroupWrite,
	"MCP restore_sandbox_group_credential":       RateClassGroupWrite,
	"MCP import_sandbox_image":                   RateClassGroupWrite,
	"MCP delete_sandbox_image":                   RateClassGroupWrite,
	"MCP delete_sandbox_snapshot":                RateClassGroupWrite,
}

// RateClasses is the class of every sandbox operation, named as the API's
// scope matrix names it ("REST POST /v1/sandboxes", "GQL Query.sandbox",
// "MCP spawn_sandbox"). The API's guard test holds it against the operations
// the server really mounts.
func RateClasses() map[string]RateClass { return maps.Clone(rateClasses) }

// mustRateClass refuses to mount an operation that has no class.
func mustRateClass(operation string) RateClass {
	class, ok := rateClasses[operation]
	if !ok {
		panic(fmt.Sprintf("sandbox: %q has no rate class; add it to rateClasses (ADR104 D7)", operation))
	}
	return class
}

// restRateClasses matches a request to its REST route's class. It serves
// nothing: only its pattern matching is used.
var restRateClasses = func() *http.ServeMux {
	mux := http.NewServeMux()
	for operation := range rateClasses {
		if pattern, ok := strings.CutPrefix(operation, "REST "); ok {
			mux.Handle(pattern, http.NotFoundHandler())
		}
	}
	return mux
}()

// RateLimits holds Render's per-user sandbox buckets, one per class, and the
// workspace bound over all of them. Each is a core.KeyedRateLimiter: a class
// admits its rate as a burst and then refills continuously, and when
// SetSharedStore is wired (w8/m579) every replica shares one Postgres store.
// A nil *RateLimits limits nothing, which is what BEX_RATE_LIMIT=0 leaves.
type RateLimits struct {
	// Keys resolves an API key to its creator, so a user's keys share that
	// user's buckets: the cached lookup that caps a key at its creator's role
	// (core.Base.KeyCreators, w9/m481). nil, or a key with no recorded
	// creator, leaves the key a caller of its own.
	Keys core.KeyCreatorResolver
	// Now supplies the current time; nil => time.Now. Injectable so a test
	// can cross a minute without sleeping.
	Now func() time.Time

	classes   map[RateClass]*core.KeyedRateLimiter[string]
	workspace *core.KeyedRateLimiter[string]
}

// NewRateLimits returns the buckets at Render's rates.
func NewRateLimits(keys core.KeyCreatorResolver) *RateLimits {
	classes := make(map[RateClass]*core.KeyedRateLimiter[string], len(rateClassPerMinute))
	for class, perMinute := range rateClassPerMinute {
		classes[class] = core.NewKeyedRateLimiter[string](float64(perMinute), 0, rateLimiterIdle, rateLimiterSweepEvery)
	}
	return &RateLimits{
		Keys:      keys,
		classes:   classes,
		workspace: core.NewKeyedRateLimiter[string](sandboxWorkspaceRatePerMinute, 0, rateLimiterIdle, rateLimiterSweepEvery),
	}
}

// SetSharedStore backs every sandbox rate class and the sandbox workspace bound
// with a cross-replica RateLimitStore.
func (l *RateLimits) SetSharedStore(store core.RateLimitStore) {
	if l == nil {
		return
	}
	for class, limiter := range l.classes {
		limiter.SetSharedStore(store, "sandbox:"+string(class), nil)
	}
	l.workspace.SetSharedStore(store, "sandbox:workspace", nil)
}

func (l *RateLimits) now() time.Time {
	if l.Now != nil {
		return l.Now()
	}
	return time.Now()
}

// buckets are what one operation of class spends: the caller's class bucket
// and, when the caller has a workspace bucket key, the workspace bound.
func (l *RateLimits) buckets(ctx context.Context, class RateClass, workspace string) []core.RateLimitBucket {
	out := []core.RateLimitBucket{l.classes[class].Handle(core.RateLimitUser(ctx, l.Keys))}
	if workspace != "" {
		out = append(out, l.workspace.Handle(workspace))
	}
	return out
}

// RequestBuckets is the general limiter's hook (api.OperationLimits): a REST
// sandbox route spends its class's buckets in place of the general ones, and
// the limiter answers its own 429 when one is empty. nil for any other
// request, GraphQL and MCP included: their operations are charged where they
// are known (charge).
func (l *RateLimits) RequestBuckets(r *http.Request, workspace string) []core.RateLimitBucket {
	if l == nil {
		return nil
	}
	_, pattern := restRateClasses.Handler(r)
	class, ok := rateClasses["REST "+pattern]
	if !ok {
		return nil
	}
	return l.buckets(r.Context(), class, workspace)
}

// charge spends one operation of class for a GraphQL field or an MCP tool,
// from the same buckets a REST route spends. With refund the request's
// general tokens are given back, so the operation is counted once; a refusal
// carries the surface's rate-limit code and, where the request still has a
// response header, the general limiter's Retry-After and RateLimit headers.
func (l *RateLimits) charge(ctx context.Context, class RateClass, code string, refund bool) error {
	if l == nil {
		return nil
	}
	scope := core.RateLimitScopeFrom(ctx)
	now := l.now()
	if refund {
		scope.Refund(ctx, now)
	}
	_, err := core.ChargeRateLimit(ctx, scope.Header(), now, code, l.buckets(ctx, class, scope.Workspace())...)
	return err
}

// handle mounts a REST sandbox route. The general limiter charges the route's
// class before the handler runs (RequestBuckets); a pattern with no class is
// refused here, at startup.
func (s *Service) handle(mux *http.ServeMux, pattern string, handler http.HandlerFunc) {
	mustRateClass("REST " + pattern)
	mux.HandleFunc(pattern, handler)
}

// limitedFields charges each of a root type's sandbox fields to its class
// before the field resolves. The request's general tokens are given back only
// when every top-level field of the operation is a sandbox one: a document
// that mixes in another field keeps paying the general budget for it.
func (s *Service) limitedFields(root string, fields graphql.Fields) graphql.Fields {
	for name, field := range fields {
		class := mustRateClass("GQL " + root + "." + name)
		resolve := field.Resolve
		field.Resolve = func(p graphql.ResolveParams) (any, error) {
			if err := s.RateLimits.charge(p.Context, class, gqlRateLimitedCode, onlySandboxFields(root, p.Info)); err != nil {
				return nil, err
			}
			return resolve(p)
		}
	}
	return fields
}

// onlySandboxFields reports whether every top-level selection of the
// operation is a sandbox field. A fragment at the top level is not looked
// into, and counts as another field.
func onlySandboxFields(root string, info graphql.ResolveInfo) bool {
	if info.Operation == nil || info.Operation.GetSelectionSet() == nil {
		return false
	}
	for _, selection := range info.Operation.GetSelectionSet().Selections {
		field, ok := selection.(*ast.Field)
		if !ok || field.Name == nil {
			return false
		}
		if _, ok := rateClasses["GQL "+root+"."+field.Name.Value]; !ok {
			return false
		}
	}
	return true
}

// addTool registers a sandbox MCP tool that spends its class before it runs.
// One tools/call is one operation, so its general tokens are given back.
func addTool[In, Out any](s *Service, server *mcp.Server, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) {
	class := mustRateClass("MCP " + tool.Name)
	mcputil.AddTool(server, tool, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		if err := s.RateLimits.charge(ctx, class, mcpRateLimitedCode, true); err != nil {
			var zero Out
			return nil, zero, err
		}
		return handler(ctx, req, in)
	})
}
