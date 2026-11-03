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

import (
	"context"
	"errors"
	"io"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// agentCreator is created_by on an agent's output attachment: no member uploaded it.
const agentCreator = "agent"

// ShareFile stores a file the agent shared (ADR095 D6) as an output attachment
// of the session's current turn. The gateway has already bound the request to
// a Pod carrying the session's labels and signed the envelope, so there is no
// caller identity to authorize. The session row is re-read and the Pod held to
// the same current-sandbox rule as every other gateway verb: an active phase,
// the session's workspace, and the current sandbox generation's Pod — so a
// terminal session, a canceled one in its idle grace, or a stale Pod from a
// failed teardown cannot share.
func (s *Service) ShareFile(ctx context.Context, envelope agentsession.ShareFileEnvelope, body io.Reader) (agentsession.ShareFileResponse, error) {
	if !s.attachmentsEnabled() {
		return agentsession.ShareFileResponse{}, agentsession.NewShareFileError(agentsession.ShareFileUnavailable, "file sharing is not configured on this deployment")
	}
	record, err := s.Store.GetAgentSession(ctx, envelope.SessionID)
	if err != nil || !agentsession.CurrentSandboxCaller(record, envelope.Workspace, envelope.PodName, envelope.PodUID) {
		return agentsession.ShareFileResponse{}, agentsession.NewShareFileError(agentsession.ShareFileForbidden, "this session is not running a sandbox")
	}
	view, err := s.storeAttachment(ctx, store.AgentAttachment{
		WorkspaceID: record.WorkspaceID,
		SessionID:   record.ID,
		Turn:        max(1, record.Turns),
		Direction:   store.AgentAttachmentOutput,
		Name:        sanitizeAttachmentName(envelope.Name),
		CreatedBy:   agentCreator,
	}, envelope.Size, body, envelope.SHA256)
	if err != nil {
		return agentsession.ShareFileResponse{}, shareFileRefusal(err)
	}
	return agentsession.ShareFileResponse{AttachmentID: view.ID, Name: view.Name, MediaType: view.MediaType, Size: view.Size}, nil
}

// shareFileRefusal maps the upload core's coded errors onto the share verb's
// vocabulary, which the in-sandbox tool reports to the agent verbatim.
func shareFileRefusal(err error) *agentsession.ShareFileError {
	var coded *core.CodedError
	if errors.As(err, &coded) {
		switch coded.Code {
		case "AGENT_SESSION_ATTACHMENT_QUOTA":
			return agentsession.NewShareFileError(agentsession.ShareFileQuotaExceeded, coded.Error())
		case "AGENT_ATTACHMENT_INVALID":
			return agentsession.NewShareFileError(agentsession.ShareFileInvalidRequest, coded.Error())
		case "AGENT_ATTACHMENT_TOO_LARGE":
			return agentsession.NewShareFileError(agentsession.ShareFileTooLarge, coded.Error())
		}
	}
	return agentsession.NewShareFileError(agentsession.ShareFileUnavailable, "the file could not be stored")
}
