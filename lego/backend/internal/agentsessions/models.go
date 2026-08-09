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

// Package agentsessions owns the tenant-facing lifecycle of cloud coding-agent
// sessions from ADR047 D3. It records intent in the control plane, delegates
// sandbox mechanism to internal/sandbox, and only mints an attach ticket: the
// isolated gateway remains the sole session ingress.
package agentsessions

import "time"

const (
	PhaseCreating      = "creating"
	PhaseRunning       = "running"
	PhaseResuming      = "resuming"
	PhaseRedispatching = "redispatching"
	PhaseCompleted     = "completed"
	PhaseFailed        = "failed"
	PhaseCanceling     = "canceling"
	PhaseCanceled      = "canceled"
)

// Delivery mode records how a turn's sandbox was obtained (ADR047 D8 phase 1):
// resume of the existing sandbox, or a re-dispatch onto a fresh sandbox+clone
// (the working path until w3/m42 hibernation lands).
const (
	DeliveryResume     = "resume"
	DeliveryRedispatch = "redispatch"
)

// Driver turn states written to the sandbox status file
// (lego/agent-image/driver/src/session.mjs); the Completer switches on these.
const (
	driverStateRunning   = "running"
	driverStateSucceeded = "succeeded"
	driverStateFailed    = "failed"
)

// Evidence is the bounded, Codex-style verifiable extract the driver captures
// from the session log at turn completion (ADR047 D4). It is a size-capped
// subset of the (wave-2) full transcript: the command log, test output tails,
// and a trailing output window. Truncated is set whenever any cap dropped
// content, so truncation is explicit, never silent.
type Evidence struct {
	CommandLog   []string `json:"commandLog,omitempty"`
	TestOutput   []string `json:"testOutput,omitempty"`
	OutputTail   string   `json:"outputTail,omitempty"`
	ChangedFiles []string `json:"changedFiles,omitempty"`
	Commits      int      `json:"commits,omitempty"`
	Truncated    bool     `json:"truncated,omitempty"`
}

// AgentConfig is the stable cross-surface driver contract. Agent selects the
// installed ACP adapter, Model is passed to that driver, Task is the initial
// fire-and-forget prompt, and Template selects a platform-registered agent
// sandbox image (never an arbitrary image supplied by a tenant).
type AgentConfig struct {
	Agent         string `json:"agent"`
	Model         string `json:"model,omitempty"`
	ModelEndpoint string `json:"modelEndpoint,omitempty"`
	Task          string `json:"task"`
	Template      string `json:"template,omitempty"`
}

// AgentProfile is a selectable, secret-free agent option for the phone composer
// (w11/m6 t001): the short ACP-adapter selector plus a human label. Model
// endpoints, templates, and egress destinations never appear here — those stay
// desktop configuration.
type AgentProfile struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// GitHubReadinessView is the mobile-safe slice of a workspace's GitHub App
// connection: whether a draft PR can be opened, the connected account login for
// display, and — when NOT connected — the desktop install URL to remediate.
// Installation ids and tokens never cross this projection.
type GitHubReadinessView struct {
	Connected    bool   `json:"connected"`
	AccountLogin string `json:"accountLogin,omitempty"`
	InstallURL   string `json:"installUrl,omitempty"`
}

// Capabilities is the mobile-safe composer/readiness projection (w11/m6 t001).
// It reveals only what a phone needs to compose and gate a fire-and-forget
// session — selectable agent profiles and readiness booleans — and deliberately
// carries none of the raw AgentConfig secret surface (model endpoint, template,
// egress) nor any credential. Enabled is false when the feature is not wired;
// Ready folds the two provisioning gates (GitHub App + BYO model key) so the
// composer can present a single submit affordance with desktop-configuration
// callouts for whatever is missing.
type Capabilities struct {
	Enabled       bool                `json:"enabled"`
	GitHub        GitHubReadinessView `json:"github"`
	ModelKeyReady bool                `json:"modelKeyReady"`
	Agents        []AgentProfile      `json:"agents"`
	Ready         bool                `json:"ready"`
}

// agentProfiles is the fixed, secret-free set of selectable agents, mirroring
// the ACP adapters agentCommand installs (claude/codex/gemini). Adding a profile
// here is the only place a new phone-selectable agent is exposed.
func agentProfiles() []AgentProfile {
	return []AgentProfile{
		{ID: "claude", Label: "Claude Code"},
		{ID: "codex", Label: "Codex"},
		{ID: "gemini", Label: "Gemini"},
	}
}

type CreateRequest struct {
	OwnerID         string      `json:"ownerId,omitempty"`
	ResumeSessionID string      `json:"resumeSessionId,omitempty"`
	Repo            string      `json:"repo,omitempty"`
	Branch          string      `json:"branch,omitempty"`
	AgentConfig     AgentConfig `json:"agentConfig,omitempty"`
	EgressAllowlist []string    `json:"egressAllowlist,omitempty"`
}

// SteerRequest is a follow-up prompt turn on an existing session (ADR047 D8
// phase 1). The BYO model key is sourced from OpenBao (ADR047 D7), never the
// request, so no secret rides this input.
type SteerRequest struct {
	SessionID       string   `json:"-"`
	Prompt          string   `json:"prompt,omitempty"`
	EgressAllowlist []string `json:"egressAllowlist,omitempty"`
}

// View is byte-for-byte the same domain shape projected by REST, GraphQL, and
// MCP. Ticket/URL/ExpiresAt are present only on create/resume/steer; list/get
// never mint ambient attach authority.
type View struct {
	ID            string      `json:"id"`
	OwnerID       string      `json:"ownerId"`
	Repo          string      `json:"repo"`
	Branch        string      `json:"branch"`
	AgentConfig   AgentConfig `json:"agentConfig"`
	SandboxID     string      `json:"sandboxId,omitempty"`
	Phase         string      `json:"phase"`
	Status        string      `json:"status"`
	HeadSHA       string      `json:"headSha,omitempty"`
	PRURL         string      `json:"prUrl,omitempty"`
	PRNumber      int         `json:"prNumber,omitempty"`
	Evidence      *Evidence   `json:"evidence,omitempty"`
	Turns         int         `json:"turns,omitempty"`
	DeliveryMode  string      `json:"deliveryMode,omitempty"`
	FailureReason string      `json:"failureReason,omitempty"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
	CanceledAt    *time.Time  `json:"canceledAt,omitempty"`
	Ticket        string      `json:"ticket,omitempty"`
	URL           string      `json:"url,omitempty"`
	ExpiresAt     *time.Time  `json:"expiresAt,omitempty"`
}
