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

// Package sandbox is the bex-api feature for hosted agent sandboxes (pillar 5,
// re-opened ADR042/w3/m32). It owns an in-package OpenSandbox lifecycle client
// (the operator's runtime/opensandbox client is unimportable — Go internal +
// the operator→types←backend arrow), an authorized service that stamps caller
// ownership and per-workspace scoping, and the REST/GraphQL/MCP adapters that
// make `render ea sandbox create/exec/list/stop` work unmodified
// (docs/render-artifacts/ea-sandbox.md). Env-gated by BEX_OPENSANDBOX_URL —
// unset means the verbs 503 and no OpenSandbox client is constructed, so the
// binary is byte-identical to before this feature.
package sandbox

// Plan is the Render-compatible sandbox compute size (SandboxPlan in the CLI's
// OpenAPI client). bex maps these onto its own sizing at Create time.
type Plan string

const (
	PlanStarter  Plan = "starter"
	PlanStandard Plan = "standard"
	PlanPro      Plan = "pro"
)

// ValidPlan reports whether p is a known Render sandbox plan.
func ValidPlan(p Plan) bool {
	switch p {
	case PlanStarter, PlanStandard, PlanPro:
		return true
	}
	return false
}

// Status is the Render-compatible sandbox lifecycle status (SandboxStatus).
// bex maps its OpenSandbox states onto these: pause⇄Suspended, resume⇄Resuming,
// delete⇄Terminated (docs/render-artifacts/ea-sandbox.md).
type Status string

const (
	StatusCreating   Status = "creating"
	StatusRunning    Status = "running"
	StatusSuspended  Status = "suspended"
	StatusResuming   Status = "resuming"
	StatusErrored    Status = "errored"
	StatusTerminated Status = "terminated"
)

// Sandbox is the Render-compatible resource returned on the REST/GraphQL/MCP
// surfaces. Id is a bex typed id (sbx-<xid>); Owner is the resolved caller
// identity (ADR014 D7 ownership); Workspace scopes list/get to the tenant. The
// Render CLI's Sandbox model also carries region/timeout/networkPolicy/timestamps
// (docs/render-artifacts/ea-sandbox.md); bex fills what it knows and reflects the
// rest so `render ea sandbox` renders a faithful record.
type Sandbox struct {
	ID             string         `json:"id"`
	Plan           Plan           `json:"plan"`
	Status         Status         `json:"status"`
	Region         string         `json:"region,omitempty"`
	TimeoutSeconds int            `json:"timeoutSeconds,omitempty"`
	NetworkPolicy  *NetworkPolicy `json:"networkPolicy,omitempty"`
	Owner          string         `json:"owner,omitempty"`
	Workspace      string         `json:"workspace,omitempty"`
	// Image is the resolved template image (bex Create is template-based,
	// ADR014 D2) — informational; the request never picks an arbitrary image.
	Image string `json:"image,omitempty"`
}

// NetworkPolicy mirrors Render's per-sandbox network posture. bex accepts and
// reflects it, but the effective policy is the `<ws>-sandbox` namespace boundary
// (ADR042 D3 / ADR043 D2), not a per-sandbox setting (ea-sandbox.md §Divergences).
type NetworkPolicy struct {
	Default string `json:"default,omitempty"`
}

// SandboxEnvelope wraps a Sandbox for the Render CLI's list surface, which reads
// a cursor-paginated array of `{ sandbox, cursor }` items, not a bare array
// (docs/render-artifacts/ea-sandbox.md — `ea sandbox list`).
type SandboxEnvelope struct {
	Sandbox Sandbox `json:"sandbox"`
	Cursor  string  `json:"cursor,omitempty"`
}

// mapOpenSandboxStatus maps an OpenSandbox lifecycle state string onto the
// Render-compatible Status vocabulary. Unknown states report errored rather
// than inventing a healthy status.
func mapOpenSandboxStatus(s string) Status {
	switch s {
	case "Running", "running", "Ready", "ready":
		return StatusRunning
	case "Creating", "creating", "Pending", "pending":
		return StatusCreating
	case "Paused", "paused", "Stopped", "stopped", "Suspended", "suspended":
		return StatusSuspended
	case "Resuming", "resuming":
		return StatusResuming
	case "Terminated", "terminated", "Deleted", "deleted":
		return StatusTerminated
	default:
		return StatusErrored
	}
}
