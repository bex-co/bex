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

package agentcred

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// ShareFileForwarder is bex-api's internal share verb as the gateway sees it
// (*agentsession.ShareFileClient). Every error it returns is a
// *agentsession.ShareFileError safe to relay to the sandbox.
type ShareFileForwarder interface {
	Share(context.Context, agentsession.ShareFileEnvelope, io.Reader) (agentsession.ShareFileResponse, error)
}

// serveShareFile is the share-side twin of servePullRequest (ADR095 D6): the
// same admit gate, with the session-only Pod rule so repo-less sessions share
// too, then one signed, streamed call to bex-api naming the verified session
// and Pod. The sandbox supplies only the bytes, their name and their SHA-256.
func (b *Broker) serveShareFile(w http.ResponseWriter, r *http.Request) {
	fail := func(code, message string) {
		agentsession.WriteShareFileError(w, agentsession.NewShareFileError(code, message))
	}
	if !b.Enabled() || b.Files == nil {
		fail(agentsession.ShareFileUnavailable, "file sharing is not available on this gateway")
		return
	}
	if r.Method != http.MethodPost || r.URL.RawQuery != "" {
		fail(agentsession.ShareFileInvalidRequest, "POST the file's bytes to this URL")
		return
	}
	namespace, sessionID, err := agentsession.ParseShareFilePath(r.URL.Path)
	if err != nil {
		fail(agentsession.ShareFileInvalidRequest, "this is not a share URL")
		return
	}
	envelope, invalid := shareFileDeclaration(r)
	if invalid != nil {
		agentsession.WriteShareFileError(w, invalid)
		return
	}
	admitted, ok := b.admit(r, namespace, authorizeSession(sessionID), func(refusal admissionRefusal) {
		switch refusal {
		case refusedSource:
			fail(agentsession.ShareFileForbidden, "unidentified source")
		case refusedConcurrency:
			fail(agentsession.ShareFileRateLimited, "too many concurrent requests from this sandbox")
		case refusedPod:
			fail(agentsession.ShareFileForbidden, "this request does not come from the session's sandbox")
		case refusedTarget:
			fail(agentsession.ShareFileForbidden, "this sandbox may only share files for its own session")
		case refusedSessionBudget:
			fail(agentsession.ShareFileRateLimited, "request budget exhausted for this session")
		case refusedWorkspaceBudget:
			fail(agentsession.ShareFileRateLimited, "request budget exhausted for this workspace")
		}
	})
	if !ok {
		return
	}
	defer admitted.release()
	r, pod, verified := admitted.r, admitted.pod, admitted.verified
	envelope.Workspace, envelope.SessionID = verified.Workspace, verified.SessionID
	envelope.PodName, envelope.PodUID = verified.PodName, verified.PodUID
	response, err := b.Files.Share(r.Context(), envelope, http.MaxBytesReader(w, r.Body, envelope.Size))
	outcome := core.AuditAllowed
	if err != nil {
		outcome = core.AuditDenied
	}
	core.RecordAuditEvent(r.Context(), b.Audit, core.AuditEvent{
		Caller: pod.Name, CallerMethod: "sandbox", Verb: agentsession.AuditVerbShareFile,
		Resource: core.WorkspaceObject(verified.Workspace), Target: "agent-session:" + verified.SessionID,
		TargetName: envelope.Name, Outcome: outcome, At: b.now(),
	})
	if err != nil {
		var typed *agentsession.ShareFileError
		if !errors.As(err, &typed) {
			typed = agentsession.NewShareFileError(agentsession.ShareFileUnavailable, "the file could not be shared")
		}
		log.Printf("agent share file: refused (session=%s): %s", verified.SessionID, typed.Code)
		agentsession.WriteShareFileError(w, typed)
		return
	}
	b.Metrics.Authentication("accepted")
	agentsession.WriteNoStoreJSON(w, http.StatusOK, response)
}

// shareFileDeclaration reads the sandbox's declaration of the file — size from
// Content-Length, name and SHA-256 from headers — and validates it before the
// Pod lookup, so a malformed request costs nothing.
func shareFileDeclaration(r *http.Request) (agentsession.ShareFileEnvelope, *agentsession.ShareFileError) {
	name, err := url.PathUnescape(r.Header.Get(agentsession.ShareFileNameHeader))
	if err != nil {
		return agentsession.ShareFileEnvelope{}, agentsession.NewShareFileError(agentsession.ShareFileInvalidRequest, "the name is not percent-encoded")
	}
	envelope := agentsession.ShareFileEnvelope{
		Name: name, Size: r.ContentLength,
		SHA256: strings.ToLower(strings.TrimSpace(r.Header.Get(agentsession.ShareFileSHA256Header))),
	}
	if invalid := envelope.Validate(); invalid != nil {
		return agentsession.ShareFileEnvelope{}, invalid
	}
	return envelope, nil
}
