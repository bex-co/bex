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

package agentsession

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The share-file verb (ADR095 D6) is how the agent hands the user a file that
// should not be committed. The sandbox POSTs the file to its own Pod-bound
// gateway URL; the gateway authenticates the source Pod exactly like the model
// proxy (session label only, so repo-less sessions can share too), then streams
// the bytes to bex-api under a signed envelope that names the verified session.
// bex-api re-checks that Pod against the session's current sandbox, then stores
// the file as an output attachment of the session's current turn. No credential
// ever enters the sandbox.

const (
	// ShareFileProxyPath prefixes the sandbox-facing route on the gateway's
	// agent listener: POST /agent-attachments/<b64 ns>/<b64 session>.
	ShareFileProxyPath = "/agent-attachments/"
	// InternalShareFilePath lives only on bex-api's cluster-internal listener.
	InternalShareFilePath = "/v1/agent-session-attachments"

	// MaxShareFileBytes is the per-file cap (ADR095 D8), the same 25 MiB as a
	// user upload; the attachment core takes its cap from here.
	MaxShareFileBytes = 25 << 20
	// MaxShareFileNameBytes bounds the name the sandbox sends; the driver
	// enforces the same number before posting.
	MaxShareFileNameBytes = 255

	// ShareFileKind tags the envelope, and its MAC domain, as this verb's.
	ShareFileKind = "share-file"

	// maxShareFileEnvelopeBytes bounds the decoded envelope. A maximal name
	// escapes to at most 6 bytes per byte in JSON (\u00XX), plus the fixed
	// fields; TestShareFileEnvelopeFitsItsCap pins the arithmetic.
	maxShareFileEnvelopeBytes = 4 << 10

	// Sandbox → gateway headers. The name is percent-encoded so any UTF-8
	// survives a header.
	ShareFileNameHeader   = "X-Bex-Share-Name"
	ShareFileSHA256Header = "X-Bex-Share-Sha256"
	// ShareFileEnvelopeHeader carries the signed gateway → bex-api envelope
	// (base64url JSON). The signature covers the envelope; the envelope binds
	// the body by size and SHA-256, so the body streams unbuffered.
	ShareFileEnvelopeHeader = "X-Bex-Share-Envelope"

	// AuditVerbShareFile records every share decision on both hops.
	AuditVerbShareFile = "agentsessions.ShareFile"
)

// shareFileMACDomain prefixes the MAC input of a share envelope, so no share
// signature can verify as another verb's (whose MAC covers the bare body), even
// though the envelope carries the same session fields as a model mint request.
// The original verbs keep their wire format unchanged across the separately
// deployed gateway and bex-api.
var shareFileMACDomain = []byte(ShareFileKind + "\n")

func shareFileMACInput(envelope []byte) []byte {
	return append(append([]byte{}, shareFileMACDomain...), envelope...)
}

// Share-file error codes returned to the sandbox, reported to the agent
// verbatim by the in-sandbox tool.
const (
	ShareFileInvalidRequest = "invalid_request"
	ShareFileTooLarge       = "too_large"
	ShareFileQuotaExceeded  = "quota_exceeded"
	ShareFileForbidden      = "forbidden"
	ShareFileRateLimited    = "rate_limited"
	ShareFileUnavailable    = "unavailable"
)

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ShareFileError is a typed refusal of the share verb, safe to relay to the
// sandbox: no upstream text, no credential.
type ShareFileError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (e *ShareFileError) Error() string { return e.Code + ": " + e.Message }

// NewShareFileError builds a refusal with the code's standard status and hint.
func NewShareFileError(code, message string) *ShareFileError {
	status, hint := http.StatusServiceUnavailable, "Retry once later; if it keeps failing, tell the user the file could not be shared."
	switch code {
	case ShareFileInvalidRequest:
		status, hint = http.StatusUnprocessableEntity, "Pass the path of a regular file under /workspace, /tmp or /attachments."
	case ShareFileTooLarge:
		status, hint = http.StatusRequestEntityTooLarge, fmt.Sprintf("Files are limited to %d MiB; share a smaller file or summarize it.", MaxShareFileBytes>>20)
	case ShareFileQuotaExceeded:
		status, hint = http.StatusConflict, "This session has reached its attachment limit; tell the user instead of sharing more files."
	case ShareFileForbidden:
		status, hint = http.StatusForbidden, "Only this session's sandbox may share files, while the session is running."
	case ShareFileRateLimited:
		status, hint = http.StatusTooManyRequests, "Wait and call this tool again later; do not call it in a loop."
	}
	return &ShareFileError{Status: status, Code: code, Message: message, Hint: hint}
}

// WriteShareFileError writes a refusal as {"error":{code,message,hint}}.
func WriteShareFileError(w http.ResponseWriter, err *ShareFileError) {
	WriteNoStoreJSON(w, err.Status, struct {
		Error *ShareFileError `json:"error"`
	}{err})
}

// ShareFileURL is the sandbox's share endpoint. It carries no credential.
func ShareFileURL(baseURL, namespace, sessionID string) (string, error) {
	return sessionScopedURL(baseURL, ShareFileProxyPath, namespace, sessionID)
}

// ParseShareFilePath extracts the claimed namespace and session; the gateway
// still authorizes both against the resolved source Pod.
func ParseShareFilePath(path string) (namespace, sessionID string, err error) {
	namespace, sessionID, _, hasRest, err := parseSessionScopedPath(path, ShareFileProxyPath)
	if err != nil || hasRest {
		return "", "", ErrInvalidRequest
	}
	return namespace, sessionID, nil
}

// ShareFileEnvelope is the signed gateway → bex-api statement: the session and
// Pod the gateway verified, and the file the sandbox declared.
type ShareFileEnvelope struct {
	Kind      string `json:"kind"`
	Workspace string `json:"workspace"`
	SessionID string `json:"sessionId"`
	PodName   string `json:"podName"`
	PodUID    string `json:"podUid"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	Nonce     string `json:"nonce"`
}

// Validate bounds the declared file before any byte is forwarded.
func (e ShareFileEnvelope) Validate() *ShareFileError {
	switch {
	case e.Size <= 0:
		return NewShareFileError(ShareFileInvalidRequest, "the file is empty or its size is missing")
	case e.Size > MaxShareFileBytes:
		return NewShareFileError(ShareFileTooLarge, fmt.Sprintf("the file is %d bytes", e.Size))
	case !sha256Hex.MatchString(e.SHA256):
		return NewShareFileError(ShareFileInvalidRequest, "the file's SHA-256 is missing or malformed")
	case strings.TrimSpace(e.Name) == "" || len(e.Name) > MaxShareFileNameBytes:
		return NewShareFileError(ShareFileInvalidRequest, "the file name is missing or too long")
	}
	return nil
}

// decodeShareFileEnvelope reads exactly a share envelope: its kind, no unknown
// field, and the gateway-verified session and Pod present.
func decodeShareFileEnvelope(raw []byte) (ShareFileEnvelope, bool) {
	var envelope ShareFileEnvelope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil || decoder.More() {
		return ShareFileEnvelope{}, false
	}
	ok := envelope.Kind == ShareFileKind && envelope.Workspace != "" && envelope.SessionID != "" &&
		envelope.PodName != "" && envelope.PodUID != ""
	return envelope, ok
}

// ShareFileResponse is what the agent gets back.
type ShareFileResponse struct {
	AttachmentID string `json:"attachmentId"`
	Name         string `json:"name"`
	MediaType    string `json:"mediaType"`
	Size         int64  `json:"size"`
}

// ShareFileSink stores a verified share as an output attachment. Every error it
// returns should be a *ShareFileError; anything else is reported as unavailable.
type ShareFileSink interface {
	ShareFile(ctx context.Context, envelope ShareFileEnvelope, body io.Reader) (ShareFileResponse, error)
}

// ShareFileHandler is bex-api's internal share verb.
type ShareFileHandler struct {
	Secret []byte
	Files  ShareFileSink
	Nonce  NonceClaimer
	Now    func() time.Time
}

func (h *ShareFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// The nonce store is not optional here: without it a captured envelope
	// could be replayed within the clock skew.
	if len(h.Secret) == 0 || h.Files == nil || h.Nonce == nil {
		WriteShareFileError(w, NewShareFileError(ShareFileUnavailable, "file sharing is not configured"))
		return
	}
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	raw, err := base64.RawURLEncoding.DecodeString(r.Header.Get(ShareFileEnvelopeHeader))
	if err != nil || len(raw) == 0 || len(raw) > maxShareFileEnvelopeBytes {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := Verify(h.Secret, shareFileMACInput(raw), r.Header.Get(TimestampHeader), r.Header.Get(SignatureHeader), now, mintClockSkew); err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	envelope, ok := decodeShareFileEnvelope(raw)
	if !ok {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if !claimNonce(r.Context(), h.Nonce, envelope.Nonce, now) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	if invalid := envelope.Validate(); invalid != nil {
		WriteShareFileError(w, invalid)
		return
	}
	response, err := h.Files.ShareFile(r.Context(), envelope, io.LimitReader(r.Body, envelope.Size+1))
	if err != nil {
		var typed *ShareFileError
		if !errors.As(err, &typed) {
			typed = NewShareFileError(ShareFileUnavailable, "the file could not be stored")
		}
		WriteShareFileError(w, typed)
		return
	}
	WriteNoStoreJSON(w, http.StatusOK, response)
}

// ShareFileClient is the gateway's HMAC-authenticated caller for the internal
// share verb.
type ShareFileClient struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	Now    func() time.Time
}

func (c *ShareFileClient) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// Share streams body to bex-api under a signed envelope. Every failure is a
// *ShareFileError safe to relay to the sandbox.
func (c *ShareFileClient) Share(ctx context.Context, envelope ShareFileEnvelope, body io.Reader) (ShareFileResponse, error) {
	envelope.Kind = ShareFileKind
	envelope.Nonce = newNonce()
	raw, err := json.Marshal(envelope)
	if err != nil {
		return ShareFileResponse{}, NewShareFileError(ShareFileUnavailable, "the share request could not be built")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, body)
	if err != nil {
		return ShareFileResponse{}, NewShareFileError(ShareFileUnavailable, "the share request could not be built")
	}
	req.ContentLength = envelope.Size
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set(ShareFileEnvelopeHeader, base64.RawURLEncoding.EncodeToString(raw))
	signHeaders(req.Header, c.Secret, shareFileMACInput(raw), c.now())
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	resp, err := client.Do(req)
	if err != nil {
		return ShareFileResponse{}, NewShareFileError(ShareFileUnavailable, "the attachment service is unreachable")
	}
	defer func() { _ = resp.Body.Close() }()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode == http.StatusOK {
		var out ShareFileResponse
		if err := json.Unmarshal(payload, &out); err != nil || out.AttachmentID == "" {
			return ShareFileResponse{}, NewShareFileError(ShareFileUnavailable, "the attachment service returned an invalid response")
		}
		return out, nil
	}
	var refusal struct {
		Error *ShareFileError `json:"error"`
	}
	if json.Unmarshal(payload, &refusal) == nil && refusal.Error != nil && refusal.Error.Code != "" {
		// Re-derive status and hint from the code: the gateway relays only its
		// own vocabulary.
		return ShareFileResponse{}, NewShareFileError(refusal.Error.Code, refusal.Error.Message)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ShareFileResponse{}, NewShareFileError(ShareFileForbidden, "the share request was not authorized")
	}
	return ShareFileResponse{}, NewShareFileError(ShareFileUnavailable, "the attachment service returned status "+strconv.Itoa(resp.StatusCode))
}
