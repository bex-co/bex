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
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// bexTool is one registered bex tool reduced to the same contractual surface
// the pin records for upstream.
type bexTool struct {
	Name     string
	Args     []string
	Required []string
}

// enumerateBexTools asks the fully-wired MCP server for its own tools/list —
// the same authoritative method scripts/render-mcp-capture.py uses upstream, so
// both sides of the comparison are measured the same way rather than one being
// measured and the other assumed.
func enumerateBexTools(t *testing.T) []bexTool {
	t.Helper()

	// Populate every feature-service field so features() surfaces all of them.
	// The inner *core.Base staying nil is fine: registration only reads the
	// pointer. Mirrors the reflection walk in TestAuthzGuardsEveryVerb.
	baseType := reflect.TypeOf(&core.Base{})
	embedsBase := func(st reflect.Type) bool {
		for i := 0; i < st.NumField(); i++ {
			if f := st.Field(i); f.Anonymous && f.Type == baseType {
				return true
			}
		}
		return false
	}
	srv := &Server{}
	v := reflect.ValueOf(srv).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() != reflect.Ptr || f.Type().Elem().Kind() != reflect.Struct || !f.CanSet() {
			continue
		}
		if embedsBase(f.Type().Elem()) {
			f.Set(reflect.New(f.Type().Elem()))
		}
	}

	ctx := context.Background()
	serverT, clientT := mcp.NewInMemoryTransports()
	go func() { _ = srv.MCPServer().Run(ctx, serverT) }()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "bex-parity-guard", Version: "0"}, nil).
		Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("connect to in-process MCP server: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) == 0 {
		t.Fatal("no tools registered — the guard would be vacuous")
	}

	out := make([]bexTool, 0, len(res.Tools))
	for _, x := range res.Tools {
		bt := bexTool{Name: x.Name}
		if x.InputSchema != nil {
			raw, err := json.Marshal(x.InputSchema)
			if err != nil {
				t.Fatalf("marshal input schema for %s: %v", x.Name, err)
			}
			var sch struct {
				Properties map[string]json.RawMessage `json:"properties"`
				Required   []string                   `json:"required"`
			}
			if err := json.Unmarshal(raw, &sch); err != nil {
				t.Fatalf("parse input schema for %s: %v", x.Name, err)
			}
			for k := range sch.Properties {
				bt.Args = append(bt.Args, k)
			}
			bt.Required = append(bt.Required, sch.Required...)
		}
		slices.Sort(bt.Args)
		slices.Sort(bt.Required)
		out = append(out, bt)
	}
	slices.SortFunc(out, func(a, b bexTool) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// TestMCPParityEveryDivergenceIsAccepted is the guard the milestone exists for:
// a bex tool may share an upstream tool's name only if it honours that tool's
// contract, or if somebody deliberately recorded why it does not.
func TestMCPParityEveryDivergenceIsAccepted(t *testing.T) {
	pin, err := loadRenderMCPContract()
	if err != nil {
		t.Fatalf("load pin: %v", err)
	}

	for _, bt := range enumerateBexTools(t) {
		class, d := classifyMCPTool(bt.Name, bt.Args, bt.Required, pin)
		if class != mcpParityDivergent {
			continue
		}
		if _, accepted := mcpAcceptedDivergences[bt.Name]; !accepted {
			t.Errorf("tool %q shares an upstream name but breaks its contract (%s).\n"+
				"An agent written against Render's MCP will fail calling it. Either fix the "+
				"argument surface, or record the reason in mcpAcceptedDivergences.",
				bt.Name, d)
		}
	}
}

// TestMCPParityNoStaleAcceptedDivergences keeps the accepted list honest: an
// entry that no longer describes a real divergence must be deleted, or it
// silently licenses a future one.
func TestMCPParityNoStaleAcceptedDivergences(t *testing.T) {
	pin, err := loadRenderMCPContract()
	if err != nil {
		t.Fatalf("load pin: %v", err)
	}

	live := map[string]bexTool{}
	for _, bt := range enumerateBexTools(t) {
		live[bt.Name] = bt
	}

	for name := range mcpAcceptedDivergences {
		bt, ok := live[name]
		if !ok {
			t.Errorf("mcpAcceptedDivergences lists %q, which bex no longer registers — delete the entry", name)
			continue
		}
		if class, _ := classifyMCPTool(bt.Name, bt.Args, bt.Required, pin); class != mcpParityDivergent {
			t.Errorf("mcpAcceptedDivergences lists %q, but it is now %s — the divergence was fixed, delete the entry",
				name, class)
		}
	}
}

// TestMCPParityUpstreamToolsAreImplementedOrAcknowledged fails when Render adds
// a tool bex neither implements nor deliberately declines, so a new upstream
// capability cannot land unnoticed.
func TestMCPParityUpstreamToolsAreImplementedOrAcknowledged(t *testing.T) {
	pin, err := loadRenderMCPContract()
	if err != nil {
		t.Fatalf("load pin: %v", err)
	}

	have := map[string]bool{}
	for _, bt := range enumerateBexTools(t) {
		have[bt.Name] = true
	}

	for _, name := range pin.Names() {
		if have[name] {
			continue
		}
		if _, known := mcpKnownUpstreamOnly[name]; !known {
			t.Errorf("upstream registers %q but bex does not, and it is not in mcpKnownUpstreamOnly.\n"+
				"Either implement it or record why bex declines it.", name)
		}
	}

	// And the reverse: an acknowledged upstream-only tool that upstream dropped,
	// or that bex has since implemented, is a stale entry.
	for name := range mcpKnownUpstreamOnly {
		if _, ok := pin.Tool(name); !ok {
			t.Errorf("mcpKnownUpstreamOnly lists %q, which upstream no longer registers — delete the entry", name)
		}
		if have[name] {
			t.Errorf("mcpKnownUpstreamOnly lists %q, but bex now implements it — delete the entry", name)
		}
	}
}

// TestMCPParityClassification covers the derivation itself against fixtures, so
// a bug in classifyMCPTool cannot quietly make every other guard vacuous.
func TestMCPParityClassification(t *testing.T) {
	pin := &renderMCPContract{byName: map[string]renderMCPTool{
		"shared": {Name: "shared", Args: []string{"a", "b"}, Required: []string{"a"}},
	}}

	tests := []struct {
		name      string
		tool      string
		args      []string
		required  []string
		wantClass mcpParityClass
	}{
		{"identical args is 1:1", "shared", []string{"a", "b"}, []string{"a"}, mcpParity1to1},
		{"extra optional arg is superset", "shared", []string{"a", "b", "c"}, []string{"a"}, mcpParitySuperset},
		{"unknown name is extension", "bex_only", []string{"z"}, nil, mcpParityExtension},
		{"dropping an upstream arg diverges", "shared", []string{"a"}, []string{"a"}, mcpParityDivergent},
		{"requiring an upstream-optional arg diverges", "shared", []string{"a", "b"}, []string{"a", "b"}, mcpParityDivergent},
		{"dropping an arg while adding one still diverges", "shared", []string{"a", "c"}, []string{"a"}, mcpParityDivergent},
		// Relaxing a requirement upstream imposes is safe: every call an agent
		// writes against Render still succeeds here.
		{"relaxing an upstream requirement stays 1:1", "shared", []string{"a", "b"}, nil, mcpParity1to1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, d := classifyMCPTool(tc.tool, tc.args, tc.required, pin)
			if got != tc.wantClass {
				t.Errorf("class = %s, want %s (%s)", got, tc.wantClass, d)
			}
		})
	}
}

// TestMCPParityInventory records the measured class counts. It is the number
// ADR018 cites, and it fails if the surface moves without the ledger moving —
// so "213 tools" can never again be a figure nobody checked.
func TestMCPParityInventory(t *testing.T) {
	pin, err := loadRenderMCPContract()
	if err != nil {
		t.Fatalf("load pin: %v", err)
	}

	counts := map[mcpParityClass]int{}
	tools := enumerateBexTools(t)
	for _, bt := range tools {
		class, _ := classifyMCPTool(bt.Name, bt.Args, bt.Required, pin)
		counts[class]++
	}

	// Measured at w1/m70 against the pinned upstream commit, then re-measured
	// twice as the per-field grammar folded away: w1/m71 took 30 Extension
	// `set_*` tools into five patch tools (213 → 187), and w1/m74 took the 12
	// remaining per-field `update_*`/`rename_*` tools into those same tools
	// (187 → 175), keeping only what REST puts behind its own route. w3/m46
	// wired `clearCache` into trigger_deploy, moving it from Divergent (8 → 7)
	// to Superset (1 → 2). Update this table and ADR018's MCP inventory together
	// — that pairing is the point.
	want := map[mcpParityClass]int{
		mcpParity1to1:      10,
		mcpParitySuperset:  2, // +trigger_deploy (clearCache wired, w3/m46)
		mcpParityDivergent: 7, // -trigger_deploy (clearCache wired, w3/m46)
		mcpParityExtension: 157, // +list_git_connections (ADR075 w5/m74)
	}
	const wantTotal = 176

	if len(tools) != wantTotal {
		t.Errorf("bex registers %d MCP tools, expected %d — update this test AND ADR018's MCP inventory together",
			len(tools), wantTotal)
	}
	for class, n := range want {
		if counts[class] != n {
			t.Errorf("class %s: got %d tools, want %d — update this test AND ADR018's MCP inventory together",
				class, counts[class], n)
		}
	}
	t.Logf("MCP parity inventory vs %s@%s: 1:1=%d superset=%d divergent=%d extension=%d (total %d; upstream %d)",
		pin.Source.Ref, pin.Source.Commit[:12],
		counts[mcpParity1to1], counts[mcpParitySuperset],
		counts[mcpParityDivergent], counts[mcpParityExtension],
		len(tools), len(pin.Tools))
}

// TestMCPDivergenceMessage covers the diagnostic the guards print. It is the
// only thing a developer sees when a divergence appears, so it must name the
// arguments rather than just asserting that something is wrong.
func TestMCPDivergenceMessage(t *testing.T) {
	pin := &renderMCPContract{byName: map[string]renderMCPTool{
		"shared": {Name: "shared", Args: []string{"keep", "dropped"}, Required: []string{"keep"}},
	}}
	_, d := classifyMCPTool("shared", []string{"keep", "added"}, []string{"keep", "added"}, pin)

	msg := d.String()
	for _, want := range []string{"dropped", "added", "missing", "extra"} {
		if !strings.Contains(msg, want) {
			t.Errorf("divergence message %q does not mention %q", msg, want)
		}
	}
	if !d.breaksCompatibility() {
		t.Error("dropping an upstream argument must break compatibility")
	}

	// An extras-only difference is reported but is not a break.
	_, safe := classifyMCPTool("shared", []string{"keep", "dropped", "extra"}, []string{"keep"}, pin)
	if safe.breaksCompatibility() {
		t.Errorf("extra optional args must not break compatibility: %s", safe)
	}
}
