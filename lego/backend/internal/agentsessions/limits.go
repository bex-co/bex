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

package agentsessions

import "time"

// Attachment limits (ADR095 D8). The per-session caps live with the store
// binding that enforces them (store.MaxAgentSessionAttachments*).
const (
	maxAttachmentBytes    = 25 << 20
	maxAttachmentsPerTurn = 10
	attachmentUnboundTTL  = 24 * time.Hour
	attachmentURLTTL      = 15 * time.Minute
	attachmentSweepBatch  = 200
	attachmentSweepBudget = 2 * time.Minute

	// An MCP upload carries the bytes base64 in the tool call, so it is bounded
	// well below a streamed upload; reads inline a small image or text file so
	// an LLM caller sees it without a second fetch (ADR095 D11).
	maxMCPUploadBytes      = 4 << 20
	maxInlineImageBytes    = 5 << 20
	maxInlineTextBytes     = 256 << 10
)
