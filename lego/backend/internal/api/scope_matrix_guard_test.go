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
	"bytes"
	"context"
	"fmt"
	"go/format"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/audit"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/usage"
)

type revokeRegistrar struct{}

func (revokeRegistrar) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/oauth/revoke", func(http.ResponseWriter, *http.Request) {})
}

// matrixServer wires every optional feature that registers a surface so the
// classification table covers sandbox routes and store-gated GraphQL fields.
func matrixServer(t *testing.T) *Server {
	t.Helper()
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default"}
	srv := NewServer(base, Deps{
		Usage:         &usage.Service{Base: base},
		Audit:         &audit.Service{Base: base},
		SandboxClient: &sandbox.Client{},
	})
	srv.HydraAdminURL = fakeHydraURL(t)
	return srv
}

func liveScopeOperations(t *testing.T) []string {
	t.Helper()
	srv := matrixServer(t)
	set := map[string]struct{}{}

	for _, pattern := range serveMuxPatterns(srv.restHandler(revokeRegistrar{})) {
		if pattern == "" {
			continue
		}
		set["REST "+pattern] = struct{}{}
	}

	schema, err := srv.newSchema()
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	if q := schema.QueryType(); q != nil {
		for name := range q.Fields() {
			if strings.HasPrefix(name, "__") {
				continue
			}
			set["GQL Query."+name] = struct{}{}
		}
	}
	if m := schema.MutationType(); m != nil {
		for name := range m.Fields() {
			if strings.HasPrefix(name, "__") {
				continue
			}
			set["GQL Mutation."+name] = struct{}{}
		}
	}

	cs := mcpSession(t, srv)
	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range tools.Tools {
		if tool == nil || tool.Name == "" {
			continue
		}
		set["MCP "+tool.Name] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for op := range set {
		out = append(out, op)
	}
	slices.Sort(out)
	if len(out) < 50 {
		t.Fatalf("live operations = %d; enumerator looks broken", len(out))
	}
	return out
}

func TestScopeMatrixCoversLiveOperations(t *testing.T) {
	live := liveScopeOperations(t)
	missing := make([]string, 0)
	for _, op := range live {
		if _, ok := classifiedOps[op]; !ok {
			missing = append(missing, op)
		}
	}
	liveSet := make(map[string]struct{}, len(live))
	for _, op := range live {
		liveSet[op] = struct{}{}
	}
	extra := make([]string, 0)
	for op := range classifiedOps {
		if _, ok := liveSet[op]; !ok {
			extra = append(extra, op)
		}
	}
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("classification table drifted from live operations\nmissing (%d): %s\nstale (%d): %s\nregenerate with GENERATE_SCOPE_MATRIX=1 go test ./internal/api -run TestGenerateScopeMatrix",
			len(missing), strings.Join(missing, ", "), len(extra), strings.Join(extra, ", "))
	}
	for _, op := range live {
		got := classifiedOps[op]
		want := classifiedClass(op)
		if got != want {
			t.Errorf("%s: classified %q, classifiedClass %q", op, got, want)
		}
	}
}

func TestScopeClassOverridesAreLive(t *testing.T) {
	live := liveScopeOperations(t)
	liveSet := make(map[string]struct{}, len(live))
	for _, op := range live {
		liveSet[op] = struct{}{}
	}
	for op := range scopeClassOverrides {
		if _, ok := liveSet[op]; !ok {
			t.Errorf("override %q is not a live operation (rename or delete it)", op)
		}
	}
}

func TestMintAndSensitiveHeuristics(t *testing.T) {
	for op, class := range classifiedOps {
		switch {
		case strings.Contains(op, "deploy-hook") && !strings.Contains(op, "deploy-hooks"):
			if class != core.OpClassMint {
				t.Errorf("%s: deploy-hook must be mint, got %s", op, class)
			}
		case strings.Contains(op, "createApiKey") || strings.Contains(op, "create_api_key") || op == "REST POST /v1/api-keys":
			if class != core.OpClassMint {
				t.Errorf("%s: API-key create must be mint, got %s", op, class)
			}
		case strings.Contains(op, "createSSHKey") || op == "MCP add_ssh_key" || op == "REST POST /v1/ssh-keys":
			if class != core.OpClassMint {
				t.Errorf("%s: SSH-key enroll must be mint, got %s", op, class)
			}
		case strings.Contains(op, "connection-info") || strings.Contains(op, "ConnectionInfo"):
			if class != core.OpClassSensitive {
				t.Errorf("%s: connection-info must be sensitive, got %s", op, class)
			}
		case strings.Contains(op, "/env-vars") && strings.HasPrefix(op, "REST GET "):
			if class != core.OpClassSensitive {
				t.Errorf("%s: env-var value read must be sensitive, got %s", op, class)
			}
		case strings.Contains(op, "/secret-files") && strings.HasPrefix(op, "REST GET "):
			if class != core.OpClassSensitive {
				t.Errorf("%s: secret-file value read must be sensitive, got %s", op, class)
			}
		case strings.Contains(op, "list_postgres_processes") || strings.Contains(op, "list_postgres_top_queries"):
			if class != core.OpClassSensitive {
				t.Errorf("%s: live SQL text must be sensitive, got %s", op, class)
			}
		case strings.Contains(op, "get_env_group_var") || strings.Contains(op, "get_env_group_secret_file") || strings.Contains(op, "envGroupVar") || strings.Contains(op, "envGroupSecretFile"):
			if class != core.OpClassSensitive {
				t.Errorf("%s: env-group value read must be sensitive, got %s", op, class)
			}
		case op == "REST POST /v1/env-groups/{id}/services/{serviceId}" || op == "GQL Mutation.linkEnvGroup" || op == "MCP link_env_group":
			if class != core.OpClassSensitive {
				t.Errorf("%s: env-group workload materialization must be sensitive, got %s", op, class)
			}
		}
	}
}

func TestGenerateScopeMatrix(t *testing.T) {
	if os.Getenv("GENERATE_SCOPE_MATRIX") != "1" {
		t.Skip("set GENERATE_SCOPE_MATRIX=1 to rewrite scope_matrix_ops.go")
	}
	live := liveScopeOperations(t)
	var b bytes.Buffer
	b.WriteString(`/*
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

import "github.com/bex-co/bex/lego/backend/internal/core"

// classifiedOps is the exhaustive operation-class table. Regenerated by
// GENERATE_SCOPE_MATRIX=1 go test ./internal/api -run TestGenerateScopeMatrix.
var classifiedOps = map[string]string{
`)
	for _, op := range live {
		fmt.Fprintf(&b, "\t%q: %s,\n", op, opClassConst(classifiedClass(op)))
	}
	b.WriteString("}\n")
	formatted, err := format.Source(b.Bytes())
	if err != nil {
		t.Fatalf("gofmt: %v\n%s", err, b.String())
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	path := filepath.Join(filepath.Dir(thisFile), "scope_matrix_ops.go")
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s (%d operations)", path, len(live))
}

func opClassConst(class string) string {
	switch class {
	case core.OpClassRead:
		return "core.OpClassRead"
	case core.OpClassWrite:
		return "core.OpClassWrite"
	case core.OpClassSensitive:
		return "core.OpClassSensitive"
	case core.OpClassMint:
		return "core.OpClassMint"
	default:
		return fmt.Sprintf("%q", class)
	}
}
