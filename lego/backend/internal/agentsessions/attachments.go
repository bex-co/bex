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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"path"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// AttachmentStore is the agent_session_attachments table (ADR095 D1).
type AttachmentStore interface {
	BeginAgentAttachment(context.Context, store.AgentAttachment) error
	ReadyAgentAttachment(ctx context.Context, id, sha256 string, unboundExpiry time.Time) (store.AgentAttachment, error)
	GetAgentAttachment(context.Context, string) (store.AgentAttachment, error)
	ListAgentSessionAttachments(context.Context, string, store.AgentAttachmentListQuery) ([]store.AgentAttachment, error)
	ListAgentSessionInputs(context.Context, string) ([]store.AgentAttachment, error)
	RetireAgentAttachment(ctx context.Context, id string, unboundOnly bool) error
	ExpireAgentAttachments(context.Context, time.Time) (int, error)
	ClaimRetiredAgentAttachments(ctx context.Context, limit int, now time.Time) ([]store.AgentAttachment, error)
	PurgeAgentAttachments(context.Context, []string) error
}

// Attachment is the public view of an attachment on every surface.
type Attachment struct {
	ID        string     `json:"id"`
	SessionID string     `json:"sessionId,omitempty"`
	Turn      int        `json:"turn,omitempty"`
	Direction string     `json:"direction"`
	Name      string     `json:"name"`
	MediaType string     `json:"mediaType"`
	Size      int64      `json:"size"`
	SHA256    string     `json:"sha256"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	// DownloadURL is a GET on the storage origin, valid for 15 minutes (ADR095
	// D7). Signing is local, so every read carries a fresh one.
	DownloadURL string `json:"downloadUrl,omitempty"`
}

// AttachmentLimits is what a client needs to validate before uploading;
// Capabilities carries it so no client hardcodes a number.
type AttachmentLimits struct {
	Enabled         bool  `json:"enabled"`
	MaxBytes        int64 `json:"maxBytes"`
	MaxPerTurn      int   `json:"maxPerTurn"`
	MaxPerSession   int   `json:"maxPerSession"`
	MaxSessionBytes int64 `json:"maxSessionBytes"`
}

// UploadAttachmentRequest is one upload: Size is the declared Content-Length.
type UploadAttachmentRequest struct {
	OwnerID string
	Name    string
	Size    int64
	Body    io.Reader
}

// AttachmentListQuery filters a session's attachments; a nil Turn is every
// turn.
type AttachmentListQuery struct {
	Direction string
	Turn      *int
	Cursor    string
	Limit     int
}

var errAttachmentDigestMismatch = core.NewBadRequestError("AGENT_ATTACHMENT_INVALID",
	"the file's bytes do not match its declared SHA-256", nil)

var errAttachmentsUnavailable = core.NewUnavailableError("AGENT_ATTACHMENTS_UNAVAILABLE",
	"agent attachments are not configured on this deployment", nil)

func (s *Service) attachmentsEnabled() bool {
	return s.Attachments != nil && s.AttachmentRows != nil && s.AttachmentCopier != nil
}

func (s *Service) attachmentLimits() AttachmentLimits {
	if !s.attachmentsEnabled() {
		return AttachmentLimits{}
	}
	return AttachmentLimits{Enabled: true, MaxBytes: maxAttachmentBytes, MaxPerTurn: maxAttachmentsPerTurn,
		MaxPerSession: store.MaxAgentSessionAttachments, MaxSessionBytes: store.MaxAgentSessionAttachmentBytes}
}

func attachmentView(a store.AgentAttachment) Attachment {
	return Attachment{ID: a.ID, SessionID: a.SessionID, Turn: a.Turn, Direction: a.Direction, Name: a.Name,
		MediaType: a.MediaType, Size: a.SizeBytes, SHA256: a.SHA256, CreatedAt: a.CreatedAt, ExpiresAt: a.ExpiresAt}
}

func turnAttachments(in []store.TurnAttachment) []TurnAttachment {
	out := make([]TurnAttachment, len(in))
	for i, a := range in {
		out[i] = TurnAttachment(a)
	}
	return out
}

func isAttachmentID(id string) bool {
	kind, ok := ids.KindOf(id)
	return ok && kind == ids.AgentAttachment
}

func attachmentNotFound(id string) error {
	return core.NewNotFoundError("AGENT_ATTACHMENT_NOT_FOUND", "agent attachment not found", map[string]any{"id": id})
}

// UploadAttachment streams one file into the attachments bucket and records it
// as an unbound input (ADR095 D2). The type is sniffed from the bytes and the
// size is enforced on what is read.
func (s *Service) UploadAttachment(ctx context.Context, req UploadAttachmentRequest) (Attachment, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return Attachment{}, err
	}
	if !s.attachmentsEnabled() {
		return Attachment{}, errAttachmentsUnavailable
	}
	workspaceID, ok := s.Tenant(ctx)
	if !ok {
		return Attachment{}, core.ErrForbidden
	}
	if req.Size <= 0 {
		return Attachment{}, core.NewBadRequestError("AGENT_ATTACHMENT_INVALID",
			"an attachment upload needs a non-empty body with Content-Length", nil)
	}
	if req.Size > maxAttachmentBytes {
		return Attachment{}, core.NewTooLargeError("AGENT_ATTACHMENT_TOO_LARGE",
			fmt.Sprintf("attachments are limited to %d MiB", maxAttachmentBytes>>20),
			map[string]any{"maxBytes": maxAttachmentBytes})
	}
	return s.storeAttachment(ctx, store.AgentAttachment{
		WorkspaceID: workspaceID,
		Direction:   store.AgentAttachmentInput,
		Name:        sanitizeAttachmentName(req.Name),
		CreatedBy:   core.SubjectFrom(ctx),
	}, req.Size, req.Body, "")
}

// storeAttachment records the row as pending, stores the object, then marks
// the row ready. Any failure retires the row, so the sweep removes whatever
// object may exist; a row left pending by a crash expires into the same sweep.
// The caller has authorized and bounded size; row.SessionID set binds the row
// at insert (an agent output), and a non-empty wantSHA256 must match the body.
func (s *Service) storeAttachment(ctx context.Context, row store.AgentAttachment, size int64, body io.Reader, wantSHA256 string) (Attachment, error) {
	head := make([]byte, 512)
	n, err := io.ReadFull(body, head[:min(int64(len(head)), size)])
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return Attachment{}, attachmentShortBody(err)
	}
	head = head[:n]
	row.ID = ids.New(ids.AgentAttachment)
	row.ObjectKey = row.WorkspaceID + "/" + row.ID
	row.MediaType = sniffMediaType(head, row.Name)
	row.SizeBytes = size
	if err := s.AttachmentRows.BeginAgentAttachment(ctx, row); err != nil {
		return Attachment{}, mapAttachmentError(err)
	}

	hash := sha256.New()
	read := &bodyReader{r: io.TeeReader(io.MultiReader(bytes.NewReader(head), io.LimitReader(body, size-int64(n))), hash), want: size}
	err = s.Attachments.Put(ctx, row.ObjectKey, read, size, row.MediaType)
	switch {
	case read.err != nil:
		s.retireAttachment(ctx, row.ID)
		return Attachment{}, attachmentShortBody(read.err)
	case err != nil:
		s.retireAttachment(ctx, row.ID)
		return Attachment{}, core.HideCause(errAttachmentsUnavailable, err)
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	// A caller that knows the digest (the agent's share envelope) is checked
	// before the row turns ready, so bytes that do not match are never visible.
	if wantSHA256 != "" && sum != wantSHA256 {
		s.retireAttachment(ctx, row.ID)
		return Attachment{}, errAttachmentDigestMismatch
	}
	stored, err := s.AttachmentRows.ReadyAgentAttachment(ctx, row.ID, sum, time.Now().Add(attachmentUnboundTTL))
	if err != nil {
		s.retireAttachment(ctx, row.ID)
		return Attachment{}, err
	}
	return attachmentView(stored), nil
}

func attachmentShortBody(err error) error {
	log.Printf("agent attachment: upload body failed: %v", err)
	return core.NewBadRequestError("AGENT_ATTACHMENT_INVALID",
		"the upload ended before its declared Content-Length; retry it", nil)
}

// retireAttachment hands a failed upload to the sweep. If even that write
// fails, the pending row expires into the sweep within the hour.
func (s *Service) retireAttachment(ctx context.Context, id string) {
	if err := s.AttachmentRows.RetireAgentAttachment(context.WithoutCancel(ctx), id, false); err != nil {
		log.Printf("agent attachment: retire %s failed (it expires into the sweep): %v", id, err)
	}
}

// bodyReader keeps the body's own failure, a read error or an end before want
// bytes, so a client that stops sending is told apart from a storage failure.
type bodyReader struct {
	r    io.Reader
	n    int64
	want int64
	err  error
}

func (b *bodyReader) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.n += int64(n)
	switch {
	case errors.Is(err, io.EOF) && b.n < b.want:
		b.err = io.ErrUnexpectedEOF
	case err != nil && !errors.Is(err, io.EOF):
		b.err = err
	}
	return n, err
}

// scopeAttachment routes a by-id verb to the attachment's own workspace (the
// ADR072 by-id matrix), authorizes relation there, and returns the row it read
// for routing.
func (s *Service) scopeAttachment(ctx context.Context, id, relation string) (context.Context, store.AgentAttachment, error) {
	var row store.AgentAttachment
	var owner core.ResourceOwner
	if s.AttachmentRows != nil && isAttachmentID(id) {
		owner = func(ctx context.Context) ([]string, error) {
			var err error
			row, err = s.AttachmentRows.GetAgentAttachment(ctx, id)
			return store.OwnerWorkspaces(row.WorkspaceID, err)
		}
	}
	ctx, err := s.ScopeByID(ctx, "", owner)
	if err != nil {
		return ctx, row, err
	}
	return ctx, row, s.Authorize(ctx, relation)
}

// readableAttachment loads an attachment the caller may act on: a bound one
// also needs can_operate on its session, an unbound one belongs to its
// uploader alone. Those row-level denials answer not-found, so ids cannot be
// probed.
func (s *Service) readableAttachment(ctx context.Context, id, relation string) (store.AgentAttachment, error) {
	ctx, row, err := s.scopeAttachment(ctx, id, relation)
	if err != nil {
		return store.AgentAttachment{}, err
	}
	if !s.attachmentsEnabled() {
		return store.AgentAttachment{}, errAttachmentsUnavailable
	}
	// ScopeByID skips the routing read when the request already names a
	// workspace; read the row here then.
	if row.ID == "" && isAttachmentID(id) {
		if row, err = s.AttachmentRows.GetAgentAttachment(ctx, id); err != nil && !errors.Is(err, store.ErrNotFound) {
			return store.AgentAttachment{}, err
		}
	}
	if workspace, ok := s.Tenant(ctx); row.ID == "" || !ok || workspace != row.WorkspaceID {
		return store.AgentAttachment{}, attachmentNotFound(id)
	}
	if row.SessionID != "" {
		if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(row.SessionID)); err != nil {
			if errors.Is(err, core.ErrForbidden) {
				return store.AgentAttachment{}, attachmentNotFound(id)
			}
			return store.AgentAttachment{}, err
		}
		return row, nil
	}
	if subject := core.SubjectFrom(ctx); subject == "" || subject != row.CreatedBy {
		return store.AgentAttachment{}, attachmentNotFound(id)
	}
	return row, nil
}

// GetAttachment returns one attachment's metadata and a download URL.
func (s *Service) GetAttachment(ctx context.Context, id string) (Attachment, error) {
	row, err := s.readableAttachment(ctx, id, core.RelCanOperate)
	if err != nil {
		return Attachment{}, err
	}
	return s.signedView(ctx, row)
}

func (s *Service) signedView(ctx context.Context, row store.AgentAttachment) (Attachment, error) {
	view := attachmentView(row)
	signed, err := s.presign(ctx, row)
	if err != nil {
		return Attachment{}, err
	}
	view.DownloadURL = signed
	return view, nil
}

// AttachmentInline returns the attachment with a download URL and, when it is
// a raster image or a text file small enough to hand an MCP caller in the tool
// result, its bytes. Larger or other files return nil bytes: the URL is the way
// to them.
func (s *Service) AttachmentInline(ctx context.Context, id string) (Attachment, []byte, error) {
	row, err := s.readableAttachment(ctx, id, core.RelCanOperate)
	if err != nil {
		return Attachment{}, nil, err
	}
	view, err := s.signedView(ctx, row)
	if err != nil || !inlineEligible(row.MediaType, row.SizeBytes) {
		return view, nil, err
	}
	body, err := s.Attachments.Get(ctx, row.ObjectKey)
	if err != nil {
		return Attachment{}, nil, core.HideCause(errAttachmentsUnavailable, err)
	}
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(io.LimitReader(body, row.SizeBytes+1))
	if err != nil || int64(len(data)) != row.SizeBytes {
		return Attachment{}, nil, core.HideCause(errAttachmentsUnavailable, fmt.Errorf("read %s: %d of %d bytes: %w", row.ID, len(data), row.SizeBytes, err))
	}
	return view, data, nil
}

// inlineEligible is the D11 rule: a raster image up to 5 MiB, or text up to
// 256 KiB.
func inlineEligible(mediaType string, size int64) bool {
	if slices.Contains(inlineImageTypes, mediaType) {
		return size <= maxInlineImageBytes
	}
	return isTextMediaType(mediaType) && size <= maxInlineTextBytes
}

func isTextMediaType(mediaType string) bool {
	return strings.HasPrefix(mediaType, "text/") || mediaType == "application/json" || mediaType == "application/yaml"
}

// AttachmentURL returns a short-lived GET URL on the storage origin (ADR095
// D7). Bytes are never served from a bex origin.
func (s *Service) AttachmentURL(ctx context.Context, id string) (string, error) {
	row, err := s.readableAttachment(ctx, id, core.RelCanOperate)
	if err != nil {
		return "", err
	}
	return s.presign(ctx, row)
}

func (s *Service) presign(ctx context.Context, row store.AgentAttachment) (string, error) {
	signed, err := s.Attachments.PresignGet(ctx, row.ObjectKey, row.MediaType,
		attachmentDisposition(row.Name, row.MediaType), attachmentURLTTL)
	if err != nil {
		return "", core.HideCause(errAttachmentsUnavailable, err)
	}
	return signed, nil
}

// ListAttachments pages a session's attachments, oldest first.
func (s *Service) ListAttachments(ctx context.Context, sessionID string, q AttachmentListQuery) ([]Attachment, error) {
	if err := s.AuthorizeOn(ctx, core.RelCanOperate, sessionObject(sessionID)); err != nil {
		return nil, err
	}
	if !s.attachmentsEnabled() {
		return nil, errAttachmentsUnavailable
	}
	if err := validateSessionID(sessionID); err != nil {
		return nil, err
	}
	switch q.Direction {
	case "", store.AgentAttachmentInput, store.AgentAttachmentOutput:
	default:
		return nil, core.NewBadRequestError("AGENT_ATTACHMENT_INVALID", "direction must be input or output", map[string]any{"field": "direction"})
	}
	turn := 0
	if q.Turn != nil {
		if turn = *q.Turn; turn < 1 {
			return nil, core.NewBadRequestError("AGENT_ATTACHMENT_INVALID", "turn must be a positive integer", map[string]any{"field": "turn"})
		}
	}
	if q.Cursor != "" && !isAttachmentID(q.Cursor) {
		return nil, core.NewBadRequestError("AGENT_ATTACHMENT_INVALID", "cursor must be an attachment id", map[string]any{"field": "cursor"})
	}
	rows, err := s.AttachmentRows.ListAgentSessionAttachments(ctx, sessionID, store.AgentAttachmentListQuery{
		Direction: q.Direction, Turn: turn, Cursor: q.Cursor, Limit: q.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Attachment, len(rows))
	for i, row := range rows {
		if out[i], err = s.signedView(ctx, row); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// DeleteAttachment removes the caller's unbound upload. Attached files belong
// to the session's record and go with it (ADR065 delete).
func (s *Service) DeleteAttachment(ctx context.Context, id string) error {
	row, err := s.readableAttachment(ctx, id, core.RelCanCreate)
	if err != nil {
		return err
	}
	if row.SessionID != "" {
		return core.NewConflictError("AGENT_ATTACHMENT_BOUND",
			"this file is attached to a session and is deleted with it", map[string]any{"sessionId": row.SessionID})
	}
	if err := s.AttachmentRows.RetireAgentAttachment(ctx, id, true); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return attachmentNotFound(id)
		}
		return err
	}
	return nil
}

// validateAttachmentIDs checks a turn's attachment list before the store binds
// it: well-formed, at most maxAttachmentsPerTurn, no duplicates.
func (s *Service) validateAttachmentIDs(list []string) error {
	if len(list) == 0 {
		return nil
	}
	if !s.attachmentsEnabled() {
		return errAttachmentsUnavailable
	}
	if len(list) > maxAttachmentsPerTurn {
		return core.NewBadRequestError("AGENT_ATTACHMENT_LIMIT",
			fmt.Sprintf("a turn can attach at most %d files", maxAttachmentsPerTurn), map[string]any{"maxPerTurn": maxAttachmentsPerTurn})
	}
	for i, id := range list {
		if !isAttachmentID(id) || slices.Contains(list[:i], id) {
			return core.NewBadRequestError("AGENT_ATTACHMENT_INVALID", "attachmentIds must be distinct attachment ids", map[string]any{"id": id})
		}
	}
	return nil
}

// mapAttachmentError turns the store's refusals into coded errors.
func mapAttachmentError(err error) error {
	var invalid *store.AgentAttachmentInvalidError
	switch {
	case errors.As(err, &invalid):
		return core.NewBadRequestError("AGENT_ATTACHMENT_INVALID",
			"attachment "+invalid.ID+" cannot be attached: it must be your own upload or a file of this session, in this workspace",
			map[string]any{"id": invalid.ID})
	case errors.Is(err, store.ErrAgentSessionAttachmentQuota):
		return core.NewConflictError("AGENT_SESSION_ATTACHMENT_QUOTA",
			fmt.Sprintf("a session holds at most %d files and %d MiB", store.MaxAgentSessionAttachments, store.MaxAgentSessionAttachmentBytes>>20),
			map[string]any{"maxPerSession": store.MaxAgentSessionAttachments, "maxSessionBytes": store.MaxAgentSessionAttachmentBytes})
	case errors.Is(err, store.ErrUnboundAttachmentQuota):
		return core.NewConflictError("AGENT_ATTACHMENT_UNBOUND_QUOTA",
			fmt.Sprintf("you have %d unattached files or %d MiB waiting; attach or delete some first", store.MaxUnboundAttachmentsPerUser, store.MaxUnboundAttachmentBytes>>20),
			map[string]any{"maxUnbound": store.MaxUnboundAttachmentsPerUser, "maxUnboundBytes": store.MaxUnboundAttachmentBytes})
	}
	return err
}

// inlineImageTypes are the only types a browser may render in place (ADR095
// D9); SVG is an image to the eye and a script host to the browser.
var inlineImageTypes = []string{"image/png", "image/jpeg", "image/gif", "image/webp"}

// textByExtension refines a sniffed text/plain within the same family.
var textByExtension = map[string]string{
	".csv": "text/csv", ".json": "application/json", ".md": "text/markdown",
	".yaml": "application/yaml", ".yml": "application/yaml", ".diff": "text/x-diff", ".patch": "text/x-diff",
}

// sniffMediaType decides the type from the bytes, never from the client's
// Content-Type; the extension may only refine plain text.
func sniffMediaType(head []byte, name string) string {
	detected, _, err := mime.ParseMediaType(http.DetectContentType(head))
	if err != nil {
		return "application/octet-stream"
	}
	if detected == "text/plain" {
		if refined, ok := textByExtension[strings.ToLower(path.Ext(name))]; ok {
			return refined
		}
	}
	return detected
}

func attachmentDisposition(name, mediaType string) string {
	kind := "attachment"
	if slices.Contains(inlineImageTypes, mediaType) {
		kind = "inline"
	}
	if disposition := mime.FormatMediaType(kind, map[string]string{"filename": name}); disposition != "" {
		return disposition
	}
	return kind
}

// maxAttachmentNameBytes keeps `<att-id>-<name>`, the file name a sandbox
// receives, within the 255-byte name limit of every Linux filesystem.
const maxAttachmentNameBytes = 255 - attachmentIDBytes - 1

// attachmentIDBytes is the length of an `att-<xid>` id.
const attachmentIDBytes = len("att-") + 20

// sanitizeAttachmentName keeps a display name safe to show, to offer as a
// download name and to use in a sandbox file name. It is never a storage key.
func sanitizeAttachmentName(name string) string {
	name = norm.NFC.String(name)
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		switch {
		case unicode.Is(unicode.Cf, r):
			return -1 // format characters, bidi overrides among them, would disguise the name
		case unicode.IsControl(r) || strings.ContainsRune(`:*?"<>|`, r):
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	for len(name) > maxAttachmentNameBytes {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	if name == "" {
		return "attachment"
	}
	return name
}
