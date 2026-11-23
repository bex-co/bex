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
	// Types is each argument's JSON type set (a Go pointer field is
	// ["null","integer"], say), for the w5/m145 type check.
	Types map[string][]string
	// Enums is each argument's JSON Schema "enum" string values, for the
	// w5/m600 enum parity check.
	Enums map[string][]string
}

// enumerateBexTools asks the fully-wired MCP server for its own tools/list —
// the same authoritative method scripts/render-mcp-capture.py uses upstream, so
// both sides of the comparison are measured the same way rather than one being
// measured and the other assumed.
func enumerateBexTools(t *testing.T) []bexTool {
	t.Helper()

	res := listBexTools(t, fullyWiredServer())

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
			bt.Types = make(map[string][]string, len(sch.Properties))
			bt.Enums = make(map[string][]string, len(sch.Properties))
			for k, prop := range sch.Properties {
				bt.Args = append(bt.Args, k)
				bt.Types[k] = jsonSchemaTypes(prop)
				bt.Enums[k] = jsonSchemaEnum(prop)
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
	// to Superset (1 → 2). w2/023 added get_service_outbound_ips (Extension
	// 164 → 165), the Render-shaped outbound-IPs read Render's own MCP does
	// not ship. w2/m91 repaired get_metrics' arg contract (Divergent 7 → 6,
	// Superset 2 → 3); create_postgres/create_static_site stay Divergent for
	// their accepted genuine differences (region / required publishPath).
	// w1/m165 implemented upstream's list_events (1:1 10 → 11, total 188 →
	// 189) against the d9a8abd re-pin; list_service_events stays as its alias.
	// w4/m189 added the optional textMode extension to list_logs and
	// list_log_label_values (1:1 11 → 9, Superset 3 → 5): Render-shaped calls
	// are unchanged, omitted textMode keeps raw text matching.
	// w2/046 implemented upstream's update_environment_variables to its exact
	// contract (1:1 9 → 10, total 189 → 190).
	// w8/152 added Render's region to the four create tools (Divergent 6 → 2,
	// Superset 5 → 9).
	// w5/m283 re-pinned at 16fe2e9: upstream's five Blueprint tools are now
	// served to its contract (1:1 10 → 15); bex's create/get/update/approve
	// extensions took those names, and create_planned_blueprint and
	// plan_blueprint_sync were removed for create_blueprint and plan_blueprint
	// (Extension 204 → 198, total 225 → 224).
	// w3/068 re-pinned at a18ef3e: upstream added run_cron_job and
	// cancel_cron_job_run, which bex already shipped as extensions. Both now
	// take upstream's contract (Extension 238 → 236): run_cron_job is 1:1
	// (15 → 16), and cancel_cron_job_run keeps runId as an optional extra, a
	// Superset (9 → 10).
	// Update this table and ADR018's MCP inventory together — that pairing is
	// the point.
	want := map[mcpParityClass]int{
		mcpParity1to1:      16,  // +run_cron_job (w3/068 cancelActiveRun); +list_services (w4/660 includePreviews); -approve_blueprint (w5/m384 optional syncId, now Superset); +create_blueprint +plan_blueprint +approve_blueprint +get_blueprint +update_blueprint (w5/m283); +list_events (w1/m165); -list_logs -list_log_label_values (w4/m189); +update_environment_variables (w2/046)
		mcpParitySuperset:  10,  // +cancel_cron_job_run (w3/068, optional bex runId); +approve_blueprint (w5/m384 syncId); -list_logs (w3/m367: upstream added Workflows task/taskRun filters, an accepted divergence); +get_metrics (w2/m91 Render arg names); +list_logs +list_log_label_values (w4/m189 textMode); +4 create tools (w8/152 region)
		mcpParityDivergent: 2,   // -list_services (w4/660 includePreviews); +list_logs (w3/m367 Workflows filters, mcpAcceptedDivergences); -get_metrics (w2/m91); -create_web_service -create_cron_job -create_postgres -create_key_value (w8/152 region)
		mcpParityExtension: 261, // +update_agent_session (w10/m599); +suspend_sandbox (w4/m596); +list_sandbox_group_credentials +get_sandbox_group_credential +list_sandbox_group_credential_versions +create_sandbox_group_credential +update_sandbox_group_credential +delete_sandbox_group_credential +restore_sandbox_group_credential (ADR122, w2/m533); +update_api_key_permissions +list_api_key_permissions (w9/m496); +preview_organization_invite +accept_organization_invite (w7/083); +list_organizations +get_organization +create_organization_workspace +set_workspace_privacy +list_organization_members +list_organization_invites +invite_organization_member +revoke_organization_invite +change_organization_member_role +remove_organization_member +join_workspace +join_workspace_as_admin (ADR115, w7/m464); -run_cron_job -cancel_cron_job_run (w3/068 upstream names); +create_sandbox_snapshot +get_sandbox_snapshot +list_sandbox_snapshots +delete_sandbox_snapshot (w32/m285); +list_agent_repo_environment_builds +get_agent_repo_environment_build +build_agent_repo_environment (ADR111, w9/m407); +list_agent_session_queued_messages +edit_agent_session_queued_message +delete_agent_session_queued_message +move_agent_session_queued_message +interrupt_agent_session (ADR047 D12, w22/m403); +list_agent_repo_environments +set_agent_repo_environment +delete_agent_repo_environment (ADR109, w9/m387); +delete_sandbox_image (w3/m367); +set_default_sandbox_group +delete_sandbox_group (w2/m359); +import_sandbox_image +get_sandbox_image +list_sandbox_images (w2/m358); +list_sandbox_groups +get_sandbox_group +create_sandbox_group +update_sandbox_group (w22/m311); +set_agent_session_tags (w7/m296 t003); +list_sandbox_executions (w32/m298); +get_agent_session_tree (ADR102 D8, w21/m279 t005); +list_agent_skill_versions +get_agent_skill_version (ADR099 D3, w21/m280 t003); +list_agent_automations +get_agent_automation +create_agent_automation +update_agent_automation +delete_agent_automation +pause_agent_automation +resume_agent_automation +run_agent_automation +take_over_agent_automation +list_agent_automation_runs (ADR101 D8, w21/m278 t006); -create_blueprint -get_blueprint -update_blueprint -approve_blueprint -create_planned_blueprint -plan_blueprint_sync (w5/m283 Render names); +list_agent_skills +get_agent_skill +create_agent_skill +update_agent_skill +delete_agent_skill (ADR099, w9/m175); +list_agent_secrets +set_agent_secret +delete_agent_secret (ADR098, w9/m174); +get_agent_session_model_cost (ADR091 D6, w10/m14 t013); +enable_keyvalue_internal_auth (w5/m268); +get_owner_notification_settings +update_owner_notification_settings (w3/m271); +list_postgres_credentials +create_postgres_credential +delete_postgres_credential (w5/m265); +link_service_to_build_source (w2/m254 t008); +8 build source tools (w2/m254 t007); +upload_agent_attachment +get_agent_attachment +list_agent_session_attachments +delete_agent_attachment (ADR095 D11, w9/m171); +create_planned_blueprint +plan_blueprint_sync +approve_blueprint (w2/m256); +wait_agent_sessions (ADR094 D6, w21/m258); +create_agent_session +send_agent_session_message (ADR094 D2 canonical names; spawn_/steer_ stay as aliases, w21/m257); +get_agent_session_tool_output (w10/m18); +get_agent_environment +update_agent_environment (w10/m17); -pin_agent_session -unpin_agent_session (w10/m15); +leave_workspace (w5/m102); +5 disk tools (ADR082 w1/m84) +2 snapshot tools (w1/m85); +list_git_connections (ADR075 w5/m74); +get_service_outbound_ips (w2/023); +list_postgres_parameters (w6/m133); +resume_keyvalue (w4/083)
	}
	const wantTotal = 289

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

// TestMCPParityCreateToolsAcceptRegion pins w8/152: the four create tools
// take Render's `region` (normalized to BEX_REGION like every other surface),
// so none of them breaks upstream's contract. Before, `region` was their one
// accepted divergence; if a tool drops it again, or upstream adds another
// argument bex lacks, this fails rather than silently re-opening a break.
func TestMCPParityCreateToolsAcceptRegion(t *testing.T) {
	pin, err := loadRenderMCPContract()
	if err != nil {
		t.Fatalf("load pin: %v", err)
	}
	live := map[string]bexTool{}
	for _, bt := range enumerateBexTools(t) {
		live[bt.Name] = bt
	}
	for _, name := range []string{"create_web_service", "create_cron_job", "create_postgres", "create_key_value"} {
		bt, ok := live[name]
		if !ok {
			t.Fatalf("bex no longer registers %s", name)
		}
		if !slices.Contains(bt.Args, "region") {
			t.Errorf("%s does not accept region", name)
		}
		if class, d := classifyMCPTool(bt.Name, bt.Args, bt.Required, pin); d.breaksCompatibility() {
			t.Errorf("%s: class=%s divergence=%s; want no break", name, class, d)
		}
	}
}

// listBexTools runs the fully-wired server in process and returns its own
// tools/list. Shared with the tool-identity guard (w4/113), which needs the
// descriptions and raw schemas enumerateBexTools deliberately drops.
func listBexTools(t *testing.T, srv *Server) *mcp.ListToolsResult {
	t.Helper()
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
	return res
}

// fullyWiredServer populates every feature-service field so features() surfaces
// all of them. The inner *core.Base staying nil is fine: registration only
// reads the pointer. Mirrors the reflection walk in TestAuthzGuardsEveryVerb.
func fullyWiredServer() *Server {
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
	return srv
}

// jsonSchemaTypes reads a property's JSON types from "type" (string or array
// of strings) as well as any "anyOf" / "oneOf" union branches.
func jsonSchemaTypes(prop json.RawMessage) []string {
	var p struct {
		Type  json.RawMessage   `json:"type"`
		AnyOf []json.RawMessage `json:"anyOf"`
		OneOf []json.RawMessage `json:"oneOf"`
	}
	if json.Unmarshal(prop, &p) != nil {
		return nil
	}
	var out []string
	if len(p.Type) > 0 {
		var one string
		if json.Unmarshal(p.Type, &one) == nil && one != "" {
			out = append(out, one)
		} else {
			var many []string
			if json.Unmarshal(p.Type, &many) == nil {
				out = append(out, many...)
			}
		}
	}
	for _, branch := range append(p.AnyOf, p.OneOf...) {
		out = append(out, jsonSchemaTypes(branch)...)
	}
	if len(out) == 0 {
		return nil
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// jsonSchemaEnum reads a property's string values from JSON Schema "enum".
func jsonSchemaEnum(prop json.RawMessage) []string {
	var p struct {
		Enum []any `json:"enum"`
	}
	if json.Unmarshal(prop, &p) != nil || len(p.Enum) == 0 {
		return nil
	}
	out := make([]string, 0, len(p.Enum))
	for _, v := range p.Enum {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// TestMCPArgumentTypesMatchUpstream is w5/m145's type half of the parity
// guard: a bex tool sharing an upstream name must accept each upstream
// argument in upstream's JSON type (an integer is a number), or record why in
// mcpAcceptedTypeDivergences. Argument names alone let clearCache:true and
// version:16 ship broken.
func TestMCPArgumentTypesMatchUpstream(t *testing.T) {
	pin, err := loadRenderMCPContract()
	if err != nil {
		t.Fatalf("load pin: %v", err)
	}
	for _, bt := range enumerateBexTools(t) {
		up, ok := pin.Tool(bt.Name)
		if !ok {
			continue
		}
		for _, arg := range mcpArgTypeMismatches(bt.Types, up) {
			if _, accepted := mcpAcceptedTypeDivergences[bt.Name+"."+arg]; accepted {
				continue
			}
			t.Errorf("%s.%s: bex accepts %v, upstream sends %q — fix the type or record the reason in mcpAcceptedTypeDivergences",
				bt.Name, arg, bt.Types[arg], up.ArgTypes[arg].Type)
		}
	}
	// Every accepted divergence must still be one, so the list cannot rot.
	for key := range mcpAcceptedTypeDivergences {
		name, arg, _ := strings.Cut(key, ".")
		up, _ := pin.Tool(name)
		var still bool
		for _, bt := range enumerateBexTools(t) {
			if bt.Name == name && slices.Contains(mcpArgTypeMismatches(bt.Types, up), arg) {
				still = true
			}
		}
		if !still {
			t.Errorf("mcpAcceptedTypeDivergences[%q] no longer diverges; remove it", key)
		}
	}
}

func TestMCPArgTypeCompatibility(t *testing.T) {
	up := renderMCPTool{ArgTypes: map[string]renderMCPArgType{
		"flag": {Type: "boolean"}, "n": {Type: "number"}, "s": {Type: "string"}, "untyped": {}, "empty": {Type: "number"},
	}}
	got := mcpArgTypeMismatches(map[string][]string{
		"flag": {"string"}, "n": {"null", "integer"}, "s": {"string"}, "untyped": {"object"}, "empty": nil,
	}, up)
	if !slices.Equal(got, []string{"empty", "flag"}) {
		t.Errorf("mismatches = %v, want [empty flag] (an integer is a number; an untyped upstream arg is unconstrained; an empty bex type mismatches a typed upstream arg)", got)
	}
	// Union schemas using type array, anyOf, or oneOf all extract their member types.
	if types := jsonSchemaTypes(json.RawMessage(`{"type":["boolean","string"]}`)); !slices.Equal(types, []string{"boolean", "string"}) {
		t.Errorf("jsonSchemaTypes(array) = %v, want [boolean string]", types)
	}
	if types := jsonSchemaTypes(json.RawMessage(`{"anyOf":[{"type":"number"},{"type":"string"}]}`)); !slices.Equal(types, []string{"number", "string"}) {
		t.Errorf("jsonSchemaTypes(anyOf) = %v, want [number string]", types)
	}
	if types := jsonSchemaTypes(json.RawMessage(`{"oneOf":[{"type":"boolean"},{"type":"string"}]}`)); !slices.Equal(types, []string{"boolean", "string"}) {
		t.Errorf("jsonSchemaTypes(oneOf) = %v, want [boolean string]", types)
	}
	if types := jsonSchemaTypes(json.RawMessage(`{}`)); len(types) != 0 {
		t.Errorf("jsonSchemaTypes({}) = %v, want nil", types)
	}
}

// TestMCPArgumentEnumsMatchUpstream is w5/m600's enum parity guard: every
// shared Render tool argument whose upstream pin declares a JSON Schema "enum"
// must advertise a JSON Schema "enum" containing every pinned upstream value
// (bexEnum ⊇ upstreamEnum), or record why in mcpAcceptedEnumDivergences.
func TestMCPArgumentEnumsMatchUpstream(t *testing.T) {
	pin, err := loadRenderMCPContract()
	if err != nil {
		t.Fatalf("load pin: %v", err)
	}
	tools := enumerateBexTools(t)
	checked := 0
	for _, bt := range tools {
		up, ok := pin.Tool(bt.Name)
		if !ok {
			continue
		}
		for arg, want := range up.ArgTypes {
			if len(want.Enum) > 0 {
				if _, exists := bt.Enums[arg]; exists {
					checked++
				}
			}
		}
		for _, arg := range mcpArgEnumMismatches(bt.Enums, up) {
			if _, accepted := mcpAcceptedEnumDivergences[bt.Name+"."+arg]; accepted {
				continue
			}
			t.Errorf("%s.%s: bex enum %v does not cover upstream enum %v — emit a superset enum or record the reason in mcpAcceptedEnumDivergences",
				bt.Name, arg, bt.Enums[arg], up.ArgTypes[arg].Enum)
		}
	}
	if checked != 21 {
		t.Fatalf("checked %d shared enum-constrained arguments, want 21 (check the upstream pin)", checked)
	}
	// Every accepted enum divergence must still be one, so the list cannot rot.
	for key := range mcpAcceptedEnumDivergences {
		name, arg, _ := strings.Cut(key, ".")
		up, _ := pin.Tool(name)
		var still bool
		for _, bt := range tools {
			if bt.Name == name && slices.Contains(mcpArgEnumMismatches(bt.Enums, up), arg) {
				still = true
			}
		}
		if !still {
			t.Errorf("mcpAcceptedEnumDivergences[%q] no longer diverges; remove it", key)
		}
	}
}

func TestMCPArgEnumCompatibility(t *testing.T) {
	up := renderMCPTool{ArgTypes: map[string]renderMCPArgType{
		"exact":    {Type: "string", Enum: []string{"a", "b"}},
		"superset": {Type: "string", Enum: []string{"a", "b"}},
		"missing":  {Type: "string", Enum: []string{"a", "b"}},
		"narrow":   {Type: "string", Enum: []string{"a", "b"}},
		"absent":   {Type: "string", Enum: []string{"a", "b"}},
		"plain":    {Type: "string"},
	}}
	got := mcpArgEnumMismatches(map[string][]string{
		"exact":    {"a", "b"},
		"superset": {"a", "b", "c"},
		"missing":  nil,
		"narrow":   {"a"},
		"plain":    nil,
	}, up)
	if !slices.Equal(got, []string{"missing", "narrow"}) {
		t.Errorf("enum mismatches = %v, want [missing narrow]", got)
	}
}

// mcpAcceptedAnnotationDivergences records a shared tool whose behavior hints
// deliberately differ from upstream's, with the reason (w5/m145).
var mcpAcceptedAnnotationDivergences = map[string]string{
	// Upstream marks list_workspaces readOnlyHint:false (its session-selection
	// lineage); listing workspaces modifies nothing, and bex has no selection.
	"list_workspaces": "a pure read in bex (no session selection); upstream's readOnlyHint:false is its select_workspace lineage",
}

// TestMCPAnnotationsMatchUpstream is w5/m145's annotation half of the parity
// guard: every bex tool carries behavior hints (without them a client treats
// list_services as a destructive write and prompts), and a tool sharing an
// upstream name agrees with upstream's readOnly and destructive hints.
func TestMCPAnnotationsMatchUpstream(t *testing.T) {
	pin, err := loadRenderMCPContract()
	if err != nil {
		t.Fatalf("load pin: %v", err)
	}
	res := listBexTools(t, fullyWiredServer())
	for _, x := range res.Tools {
		if x.Annotations == nil {
			t.Errorf("%s has no annotations", x.Name)
			continue
		}
		up, ok := pin.Tool(x.Name)
		if !ok || len(up.Annotations) == 0 {
			continue
		}
		if _, accepted := mcpAcceptedAnnotationDivergences[x.Name]; accepted {
			continue
		}
		destructive := x.Annotations.DestructiveHint == nil || *x.Annotations.DestructiveHint
		if want, set := up.Annotations["readOnlyHint"]; set && x.Annotations.ReadOnlyHint != want {
			t.Errorf("%s readOnlyHint = %v, upstream %v", x.Name, x.Annotations.ReadOnlyHint, want)
		}
		if want, set := up.Annotations["destructiveHint"]; set && !x.Annotations.ReadOnlyHint && destructive != want {
			t.Errorf("%s destructiveHint = %v, upstream %v", x.Name, destructive, want)
		}
	}
}
