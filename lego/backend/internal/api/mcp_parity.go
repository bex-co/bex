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
	"fmt"
	"slices"
)

// mcp_parity.go classifies bex's MCP tools against the pinned upstream surface
// (render_mcp.go). It is the MCP counterpart of conformance_allowlist_test.go,
// which does the same job for REST.
//
// The class is DERIVED from the pin, never hand-declared. A hand-maintained
// 213-row table would rot the moment a tool changed, which is exactly how the
// ten "checked live <date>" comments this replaces went stale in both
// directions. What IS hand-maintained is the small set of deliberate human
// decisions: which divergences from upstream bex accepts, and why. Everything
// else the guard test computes.

// mcpParityClass is where a bex tool sits relative to Render's official server.
type mcpParityClass string

const (
	// mcpParity1to1 — same name, same argument names as upstream. An agent
	// written against Render's MCP calls it unchanged. This is the contract;
	// renaming one of these breaks that agent.
	mcpParity1to1 mcpParityClass = "Parity1to1"

	// mcpParitySuperset — upstream's name and every upstream argument, plus
	// extra OPTIONAL bex arguments. Still call-compatible with an agent written
	// against Render, because the added arguments may be omitted.
	mcpParitySuperset mcpParityClass = "Superset"

	// mcpParityExtension — no upstream counterpart. bex owns the name and the
	// shape outright; no parity obligation attaches.
	mcpParityExtension mcpParityClass = "Extension"

	// mcpParityDivergent — shares an upstream tool's NAME but not its contract:
	// it drops an argument upstream accepts, or requires one upstream leaves
	// optional. A call an agent writes against Render's docs fails here.
	//
	// This fourth class is a deliberate addition to the three the milestone
	// specified. The pin found eight of these on its first run, and neither of
	// the other classes can express them honestly — "Superset" would claim
	// call-compatibility bex does not have, and "Extension" would claim bex owns
	// a name it shares. Every one needs an explicit, reasoned decision, which is
	// what mcpAcceptedDivergences records.
	mcpParityDivergent mcpParityClass = "Divergent"
)

// mcpDivergence describes exactly how a bex tool departs from upstream's
// contract for the same tool name.
type mcpDivergence struct {
	// MissingArgs are upstream arguments bex does not accept.
	MissingArgs []string
	// AddedRequired are arguments bex requires that upstream leaves optional.
	AddedRequired []string
	// ExtraArgs are bex-only additions; harmless on their own.
	ExtraArgs []string
}

func (d mcpDivergence) breaksCompatibility() bool {
	return len(d.MissingArgs) > 0 || len(d.AddedRequired) > 0
}

// mcpAcceptedDivergences records every tool that shares an upstream name while
// breaking its contract, with the reason bex accepts it. A divergence NOT in
// this map fails the guard — the point is that each one is a decision somebody
// made on purpose, not something that accumulated.
//
// Keep the reason specific enough to re-litigate later: "why is this fine" and
// "what would make it not fine".
var mcpAcceptedDivergences = map[string]string{
	// bex has no multi-region placement. BEX_REGION projects a single operator-
	// configured placement name onto REST metadata; there is nothing for a
	// caller to choose, so accepting `region` would be accepting a value bex
	// must then ignore. Revisit if bex ever runs more than one region.
	"create_web_service": "no multi-region placement (BEX_REGION is operator-set, not caller-chosen); `region` would be accepted and silently ignored",
	"create_cron_job":    "same as create_web_service — `region` is not a caller-chosen value in bex",
	"create_key_value":   "same as create_web_service — `region` is not a caller-chosen value in bex",

	// Two departures. `region` as above. `diskSizeGb` vs bex's `diskSizeGB` is a
	// genuine casing incompatibility with no defensible reason — an agent
	// following Render's schema silently fails to set disk size. Tracked for
	// repair rather than accepted on merit; see the milestone's follow-up notes.
	"create_postgres": "`region` as above; `diskSizeGb` vs bex's `diskSizeGB` is an unintended casing divergence — accepted only to keep this pin landable, filed for repair",

	// bex requires publishPath because a static site with no publish directory
	// has nothing to serve; upstream defaults it. Dropping buildCommand and
	// autoDeploy, though, is unintended.
	"create_static_site": "`publishPath` is genuinely required in bex (no default publish dir); missing `autoDeploy`/`buildCommand` is unintended and filed for repair",

	// bex's metrics surface predates this pin and uses `resource`/`quantile`/
	// `resolutionSeconds` where upstream uses `resourceId`/`httpLatencyQuantile`/
	// `resolution`. A rename, not a capability gap.
	"get_metrics": "argument names predate the pin (`resource` vs `resourceId`, `quantile` vs `httpLatencyQuantile`, `resolutionSeconds` vs `resolution`); a rename, not a capability gap — filed for repair",

	// bex has no preview environments at all (PR previews are a recorded
	// non-goal in .pm/DO_NOT_DO.md), so there is no set to include or exclude.
	"list_services": "`includePreviews` has no meaning in bex — PR preview environments are a recorded non-goal",
}

// mcpKnownUpstreamOnly records upstream tools bex deliberately does not ship.
// An upstream tool that is neither implemented nor listed here fails the guard,
// so a new Render tool cannot land unnoticed.
var mcpKnownUpstreamOnly = map[string]string{
	// w1/m55 (2026-07-27) adopted Render's request-scoped workspace contract and
	// removed the session-selection tools; every bex tool takes an optional
	// workspaceId instead. Upstream still ships both but treats selection as
	// transitional.
	"select_workspace":       "w1/m55 adopted the request-scoped workspaceId contract and removed session selection; upstream calls its own selection tools transitional",
	"get_selected_workspace": "w1/m55 — see select_workspace",

	// NOT a deliberate omission: bex covers the capability under a different
	// name (update_env_vars, plus set_env_var/delete_env_var). Listed here so
	// the guard passes, but the name divergence is real and is why
	// internal/secrets/mcp.go's "Render's official MCP has no env-var tools"
	// header comment was stale — upstream added this after that comment.
	"update_environment_variables": "bex covers the capability as `update_env_vars`; the NAME diverges and the stale header comment in internal/secrets/mcp.go was corrected in w1/m70",
}

// classifyMCPTool derives a tool's parity class from the pin.
func classifyMCPTool(name string, args, required []string, pin *renderMCPContract) (mcpParityClass, mcpDivergence) {
	up, ok := pin.Tool(name)
	if !ok {
		return mcpParityExtension, mcpDivergence{}
	}

	// upstreamOptional distinguishes "upstream accepts this" from "upstream
	// insists on this": requiring an argument upstream leaves optional rejects a
	// call an agent legitimately writes, while relaxing one upstream requires
	// does not.
	upstreamOptional := make(map[string]bool, len(up.Args))
	for _, a := range up.Args {
		upstreamOptional[a] = true
	}
	for _, r := range up.Required {
		upstreamOptional[r] = false
	}
	have := make(map[string]bool, len(args))
	for _, a := range args {
		have[a] = true
	}

	var d mcpDivergence
	for _, a := range up.Args {
		if !have[a] {
			d.MissingArgs = append(d.MissingArgs, a)
		}
	}
	for _, a := range args {
		if _, upstreamHas := upstreamOptional[a]; !upstreamHas {
			d.ExtraArgs = append(d.ExtraArgs, a)
		}
	}
	for _, r := range required {
		if upstreamOptional[r] {
			d.AddedRequired = append(d.AddedRequired, r)
		}
	}
	slices.Sort(d.MissingArgs)
	slices.Sort(d.ExtraArgs)
	slices.Sort(d.AddedRequired)

	switch {
	case d.breaksCompatibility():
		return mcpParityDivergent, d
	case len(d.ExtraArgs) > 0:
		return mcpParitySuperset, d
	default:
		return mcpParity1to1, d
	}
}

func (d mcpDivergence) String() string {
	return fmt.Sprintf("missing=%v addedRequired=%v extra=%v", d.MissingArgs, d.AddedRequired, d.ExtraArgs)
}
