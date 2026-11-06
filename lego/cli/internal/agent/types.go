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

package agent

import (
	"encoding/json"
	"time"
)

// The wire shapes below are the parts of bex-api's agent-session contract
// (lego/backend/internal/agentsessions, ADR094/ADR095) that `bex agent`
// reads. The CLI imports no sibling module, so testdata/agent-cli-contract.json
// pins the field names; JSON output prints the API's own bytes, never these.

// Session states (ADR094 D7). Every state but working ends the current turn.
const (
	stateWorking    = "working"
	stateNeedsInput = "needs_input"
	stateSucceeded  = "succeeded"
	stateFailed     = "failed"
	stateCanceled   = "canceled"
)

// sessionStates are the states `bex agent list --state` filters on.
var sessionStates = []string{stateWorking, stateNeedsInput, stateSucceeded, stateFailed, stateCanceled}

type session struct {
	ID                string      `json:"id"`
	OwnerID           string      `json:"ownerId"`
	Repo              string      `json:"repo"`
	Branch            string      `json:"branch"`
	Title             string      `json:"title"`
	Tags              []string    `json:"tags"`
	AgentConfig       agentConfig `json:"agentConfig"`
	State             string      `json:"state"`
	Turns             int         `json:"turns"`
	PRURL             string      `json:"prUrl"`
	FailureReason     string      `json:"failureReason"`
	FailureReasonCode string      `json:"failureReasonCode"`
	SSHAddress        string      `json:"sshAddress"`
	CreatedAt         time.Time   `json:"createdAt"`
	Outcome           *outcome    `json:"outcome"`
}

type agentConfig struct {
	Agent string `json:"agent"`
	Model string `json:"model,omitempty"`
	Task  string `json:"task"`
}

type outcome struct {
	Turn              int              `json:"turn"`
	Summary           string           `json:"summary"`
	PullRequest       *pullRequest     `json:"pullRequest"`
	DeliveryNote      string           `json:"deliveryNote"`
	FailureReasonCode string           `json:"failureReasonCode"`
	Files             []turnAttachment `json:"files"`
	Result            json.RawMessage  `json:"result"`
	Cost              *cost            `json:"cost"`
	// WaitingFor and PendingDecision say what a needs_input session waits for
	// (ADR100): approval or user, and the decision `bex agent answer` answers.
	WaitingFor      string    `json:"waitingFor"`
	PendingDecision *decision `json:"pendingDecision"`
}

// decision is the approval or question a turn paused on.
type decision struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
	State string `json:"state"`
}

// answerRequest is POST /v1/agent-sessions/{id}/decisions/{decisionId}.
type answerRequest struct {
	Allow  *bool   `json:"allow,omitempty"`
	Scope  string  `json:"scope,omitempty"`
	Answer *string `json:"answer,omitempty"`
}

type pullRequest struct {
	URL    string `json:"url"`
	Number int    `json:"number"`
}

// cost is outcome.cost: one total and where it went (ADR094 D7, w9/m178).
type cost struct {
	ModelUSD   string `json:"modelUSD"`
	ComputeUSD string `json:"computeUSD"`
	TotalUSD   string `json:"totalUSD"`
}

type turnAttachment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// attachment is an uploaded input or an agent-shared output (ADR095).
// DownloadURL is a 15-minute GET on the storage origin, never on bex-api.
type attachment struct {
	ID          string `json:"id"`
	Turn        int    `json:"turn"`
	Direction   string `json:"direction"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	DownloadURL string `json:"downloadUrl"`
}

// createRequest is POST /v1/agent-sessions; steerRequest is …/{id}/steer.
type createRequest struct {
	Repo           string            `json:"repo,omitempty"`
	Branch         string            `json:"branch,omitempty"`
	AgentConfig    agentConfig       `json:"agentConfig"`
	Title          string            `json:"title,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	AttachmentIDs  []string          `json:"attachmentIds,omitempty"`
	OutputSchema   json.RawMessage   `json:"outputSchema,omitempty"`
	MaxSpendUSD    *float64          `json:"maxSpendUSD,omitempty"`
	ApprovalMode   string            `json:"approvalMode,omitempty"`
	BypassApproval bool              `json:"bypassApproval,omitempty"`
	IdempotencyKey string            `json:"-"`
	Skills         []skillInvocation `json:"skills,omitempty"`
	sessionSecrets
}

type steerRequest struct {
	Prompt        string            `json:"prompt"`
	AttachmentIDs []string          `json:"attachmentIds,omitempty"`
	OutputSchema  json.RawMessage   `json:"outputSchema,omitempty"`
	Skills        []skillInvocation `json:"skills,omitempty"`
	sessionSecrets
}

// skillInvocation is ADR099 D4: a workspace skill to use this turn, with the
// text its $ARGUMENTS (and $1-$9) receive.
type skillInvocation struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}

// sessionSecrets is ADR098's attach shape, shared by create and steer:
// stored workspace secrets by name, and inline values for this session only.
type sessionSecrets struct {
	SecretNames []string       `json:"secretNames,omitempty"`
	Secrets     []inlineSecret `json:"secrets,omitempty"`
}

type inlineSecret struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type transcriptPage struct {
	Parts        []transcriptPart `json:"parts"`
	Turns        []transcriptTurn `json:"turns"`
	NextAfterSeq int64            `json:"nextAfterSeq"`
}

type transcriptPart struct {
	Seq       int64           `json:"seq"`
	PartIndex int64           `json:"partIndex"`
	Turn      int             `json:"turn"`
	Part      json.RawMessage `json:"part"`
	Partial   bool            `json:"partial"`
	// raw is the row as the API sent it, for --output json.
	raw json.RawMessage
}

func (p *transcriptPart) UnmarshalJSON(data []byte) error {
	type fields transcriptPart
	var f fields
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	*p = transcriptPart(f)
	p.raw = append(json.RawMessage(nil), data...)
	return nil
}

type transcriptTurn struct {
	Turn   int    `json:"turn"`
	Prompt string `json:"prompt"`
}
