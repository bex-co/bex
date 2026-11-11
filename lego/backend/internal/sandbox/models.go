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

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Plan is the Render-compatible sandbox compute size (SandboxPlan in the CLI's
// OpenAPI client). Each plan fixes the sandbox container's CPU and memory limit
// (planResources); billing follows those resources through the weighted
// compute meter, so a plan needs no pricing entry of its own.
type Plan string

const (
	// Render's spec-named plans (CLI v2.35 help: `--plan 1c-2g|2c-4g|4c-8g|8c-16g`).
	Plan1C2G  Plan = "1c-2g"
	Plan2C4G  Plan = "2c-4g"
	Plan4C8G  Plan = "4c-8g"
	Plan8C16G Plan = "8c-16g"
	// Render's deprecated names, kept as accepted aliases. They size like the
	// Workflow plans of the same name: starter 0.5 CPU / 512 MB (no spec-named
	// equivalent), standard ≡ 1c-2g, pro ≡ 2c-4g.
	PlanStarter  Plan = "starter"
	PlanStandard Plan = "standard"
	PlanPro      Plan = "pro"

	// DefaultPlan is the plan of a sandbox created without one: Render's
	// default, 1 CPU and 2 GB (ADR104 § Groups, plans and limits). bex used
	// to default to the deprecated starter, half a CPU and 512 MB (w8/247).
	DefaultPlan = Plan1C2G
)

// planSize is the CPU and memory limit a plan gives the sandbox container.
type planSize struct {
	CPU    string
	Memory string
}

// planResources maps every accepted plan onto its compute size. The
// spec-named plans are exactly their names; the deprecated names keep the
// sizes Render documents for them.
var planResources = map[Plan]planSize{
	Plan1C2G:     {CPU: "1", Memory: "2Gi"},
	Plan2C4G:     {CPU: "2", Memory: "4Gi"},
	Plan4C8G:     {CPU: "4", Memory: "8Gi"},
	Plan8C16G:    {CPU: "8", Memory: "16Gi"},
	PlanStarter:  {CPU: "500m", Memory: "512Mi"},
	PlanStandard: {CPU: "1", Memory: "2Gi"},
	PlanPro:      {CPU: "2", Memory: "4Gi"},
}

// planNames lists the accepted plans in Render's order (sandboxPlan enum):
// the current spec-named plans, then the deprecated aliases.
func planNames() []string {
	return []string{
		string(Plan1C2G), string(Plan2C4G), string(Plan4C8G), string(Plan8C16G),
		string(PlanStarter), string(PlanStandard), string(PlanPro),
	}
}

// PlanLimits returns the CPU and memory limit of a known plan.
func PlanLimits(p Plan) (cpu, memory string, ok bool) {
	size, ok := planResources[p]
	return size.CPU, size.Memory, ok
}

// ValidPlan reports whether p is a known Render sandbox plan.
func ValidPlan(p Plan) bool {
	_, ok := planResources[p]
	return ok
}

// Status is the Render-compatible sandbox lifecycle status (SandboxStatus).
// bex maps its OpenSandbox states onto these: Pausing⇄suspending,
// Paused⇄suspended, Resuming⇄resuming, delete⇄terminated
// (docs/render-artifacts/ea-sandbox.md, w2/m278).
type Status string

const (
	StatusCreating   Status = "creating"
	StatusRunning    Status = "running"
	StatusSuspended  Status = "suspended"
	StatusResuming   Status = "resuming"
	StatusErrored    Status = "errored"
	StatusTerminated Status = "terminated"
	// StatusSuspending is a sandbox whose idle (or explicit) pause is
	// committing its root filesystem: OpenSandbox's Pausing phase.
	StatusSuspending Status = "suspending"
)

// Sandbox is the Render-compatible resource returned on the REST/GraphQL/MCP
// surfaces. Id is a bex typed id (sbx-<xid>); Owner is the resolved caller
// identity (ADR014 D7 ownership); Workspace scopes list/get to the tenant. The
// Render CLI's Sandbox model also carries region/timeout/networkPolicy/timestamps
// (docs/render-artifacts/ea-sandbox.md); bex fills what it knows and reflects the
// rest so `render ea sandbox` renders a faithful record.
type Sandbox struct {
	ID string `json:"id"`
	// Name is Render's optional, non-unique sandbox label (w5/m287); null
	// when the sandbox has none, as Render answers.
	Name   *string `json:"name"`
	Plan   Plan    `json:"plan"`
	Status Status  `json:"status"`
	Region string  `json:"region,omitempty"`
	// TimeoutSeconds is the effective lifetime in seconds; 0 means no timeout,
	// as on Render (w2/m263 D3=A). Legacy metadata without a bound reads as
	// 86400 (w5/m99); see timeoutFromMetadata.
	TimeoutSeconds int `json:"timeoutSeconds"`
	// IdleTimeoutSeconds is the inactivity window after which the sandbox
	// suspends; 0 disables idle suspension, as on Render (w2/m278). See
	// resolveIdleTimeout.
	IdleTimeoutSeconds int            `json:"idleTimeoutSeconds"`
	NetworkPolicy      *NetworkPolicy `json:"networkPolicy,omitempty"`
	Owner              string         `json:"owner,omitempty"`
	Workspace          string         `json:"workspace,omitempty"`
	CreatedAt          *time.Time     `json:"createdAt,omitempty"`
	// TerminatedAt is when an ended sandbox ended (Render's nullable
	// terminatedAt, w3/m318); null while it lives.
	TerminatedAt *time.Time `json:"terminatedAt"`
	// Image is the resolved template image (bex Create is template-based,
	// ADR014 D2) — informational; the request never picks an arbitrary image.
	Image string `json:"image,omitempty"`
	// SandboxGroupID is the group the sandbox belongs to (Render's
	// `sandboxGroupId`, w22/m311). Empty, so omitted, for an agent-session
	// sandbox, which has none, and when the deployment has no group store.
	SandboxGroupID string `json:"sandboxGroupId,omitempty"`
	// ungrouped marks an agent-session sandbox, which belongs to no group. It is
	// not part of any wire shape.
	ungrouped bool
}

// NetworkPolicy mirrors Render's per-sandbox outbound posture (w32/m283,
// ADR042 § Sandbox network policies): `allow-all` (Render's default) reaches
// any public destination on any protocol, `allow-list` reaches only the listed
// hostnames over HTTPS, and `deny-all` reaches nothing. Under every policy the
// platform floor stays denied (cluster, node, metadata, private ranges, other
// tenants); Cilium enforces it outside the gVisor guest.
//
// Render renamed the field to `type` and kept `default` as a deprecated alias
// that requests may send and responses always echo (Render CLI v2.29 sends
// `type`). Requests may carry either; normalizeNetworkPolicy folds them into
// Default, and MarshalJSON writes both.
type NetworkPolicy struct {
	Default NetworkPolicyDefault `json:"default,omitempty"`
	Type    NetworkPolicyDefault `json:"type,omitempty"`
	// Rules are the allow-list destinations: required for allow-list and
	// refused for any other policy, as on Render.
	Rules []NetworkPolicyRule `json:"rules,omitempty"`
}

// NetworkPolicyRule is one Render allow-list destination. Protocol is always
// `https` (Render accepts no other value and defaults to it).
type NetworkPolicyRule struct {
	Domain   string `json:"domain"`
	Protocol string `json:"protocol,omitempty"`
}

// MarshalJSON writes Render's response shape: `type`, its `default` alias and,
// for an allow-list, its rules.
func (p NetworkPolicy) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type    NetworkPolicyDefault `json:"type"`
		Default NetworkPolicyDefault `json:"default"`
		Rules   []NetworkPolicyRule  `json:"rules,omitempty"`
	}{p.Default, p.Default, p.Rules})
}

// NetworkPolicyProtocolHTTPS is the only allow-list rule protocol Render accepts.
const NetworkPolicyProtocolHTTPS = "https"

// NetworkPolicyDefault is Render's exact sandboxNetworkPolicy.type enum.
type NetworkPolicyDefault string

const (
	NetworkPolicyAllowAll  NetworkPolicyDefault = "allow-all"
	NetworkPolicyDenyAll   NetworkPolicyDefault = "deny-all"
	NetworkPolicyAllowList NetworkPolicyDefault = "allow-list"
)

// SandboxEnvelope wraps a Sandbox for the Render CLI's list surface, which reads
// a cursor-paginated array of `{ sandbox, cursor }` items, not a bare array
// (docs/render-artifacts/ea-sandbox.md — `ea sandbox list`).
type SandboxEnvelope struct {
	Sandbox Sandbox `json:"sandbox"`
	Cursor  string  `json:"cursor"`
}

// mapOpenSandboxStatus maps an OpenSandbox lifecycle state string onto the
// Render-compatible Status vocabulary. Unknown states report errored rather
// than inventing a healthy status.
func mapOpenSandboxStatus(s string) Status {
	switch s {
	case "Running", "running", "Ready", "ready":
		return StatusRunning
	case "Creating", "creating", "Pending", "pending", "Allocated", "allocated":
		return StatusCreating
	case "Pausing", "pausing", "Suspending", "suspending":
		return StatusSuspending
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

// renderListStatuses is Render's list-sandboxes `status` enum.
var renderListStatuses = []string{
	string(StatusCreating), string(StatusRunning), string(StatusSuspending), string(StatusSuspended),
	string(StatusResuming), string(StatusErrored), string(StatusTerminated),
}

// parseStatusFilter checks Render's list `status` filter values (REST splits
// its comma/repeated form with core.QueryList). Empty means Render's default —
// every status except terminated. An unknown status is refused by name, never
// dropped, so a filter is never silently widened (w2/m263).
func parseStatusFilter(values []string) (map[Status]bool, error) {
	want := make(map[Status]bool, len(values))
	for _, status := range values {
		if !slices.Contains(renderListStatuses, status) {
			return nil, core.NewBadRequestError("SANDBOX_STATUS_INVALID",
				fmt.Sprintf("unknown sandbox status %q; use one of %s", status, strings.Join(renderListStatuses, ", ")),
				map[string]any{"field": "status", "supported": renderListStatuses})
		}
		want[Status(status)] = true
	}
	return want, nil
}

// filterStatuses keeps the sandboxes whose status is wanted, or every
// non-terminated one when no status was named.
func filterStatuses(in []Sandbox, want map[Status]bool) []Sandbox {
	return core.Filter(in, func(sb Sandbox) bool {
		if len(want) == 0 {
			return sb.Status != StatusTerminated
		}
		return want[sb.Status]
	})
}
