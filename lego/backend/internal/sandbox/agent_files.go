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

package sandbox

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/sandboxexec"
	"github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
)

// UploadAgentSessionFiles extracts one tar archive at path, which must be
// sandboxfiles.AgentAttachmentsPath (ADR095 D4) or sandboxfiles.AgentSkillsPath
// (ADR099 D5), in an agent session's own sandbox over the gateway's
// /sandbox-files path. It is platform-initiated
// like the Completer's status read: the ticket carries
// sandboxexec.SystemSubject, whose authority is the HMAC only bex-api holds,
// and the gateway admits that subject for this one destination only.
//
// The archive is read once, so a gateway that sheds the transfer as busy
// (429) before reading it gets it again; the caller supplies a fresh archive
// per attempt through open.
func (l *AgentSessionLifecycle) UploadAgentSessionFiles(ctx context.Context, workspaceID, sessionID, sandboxID, path string, open func() io.ReadCloser) error {
	s := l.service
	if err := s.fileAvailabilityError(); err != nil {
		return err
	}
	key, err := s.agentSessionKey(ctx, workspaceID)
	if err != nil {
		return err
	}
	raw, err := s.agentSessionSandbox(ctx, key, workspaceID, sessionID, sandboxID)
	if err != nil {
		return err
	}
	const attempts = 4
	for attempt := 1; ; attempt++ {
		status, err := s.putAgentSessionArchive(ctx, workspaceID, raw, path, open)
		if err != nil {
			return err
		}
		switch {
		case status == http.StatusNoContent:
			return nil
		case status == http.StatusTooManyRequests && attempt < attempts:
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		default:
			log.Printf("agent session files: sandbox file gateway answered %d (session=%s path=%s)", status, sessionID, path)
			return unreachable(fmt.Sprintf("the file gateway answered %d", status))
		}
	}
}

func (s *Service) putAgentSessionArchive(ctx context.Context, workspaceID string, raw osSandbox, path string, open func() io.ReadCloser) (int, error) {
	ticket, gateway, err := s.gatewayFileTicket(sandboxexec.SystemSubject, workspaceID, raw,
		sandboxfiles.OperationUpload, path)
	if err != nil {
		return 0, err
	}
	archive := open()
	defer func() { _ = archive.Close() }()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, gateway.String(), archive)
	if err != nil {
		return 0, err
	}
	req.Header.Set(sandboxfiles.TicketHeader, ticket)
	req.Header.Set("Content-Type", "application/x-tar")
	resp, err := s.gatewayClient().Do(req)
	if err != nil {
		log.Printf("agent session files: sandbox file gateway unreachable: %v", err)
		return 0, unreachable("the file gateway did not answer")
	}
	_ = resp.Body.Close()
	return resp.StatusCode, nil
}
