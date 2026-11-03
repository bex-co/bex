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
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeObjects struct {
	mu      sync.Mutex
	data    map[string][]byte
	putErr  error
	delErr  error
	deleted []string
}

func (f *fakeObjects) Put(_ context.Context, key string, body io.Reader, size int64, _ string) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if f.putErr != nil {
		return f.putErr
	}
	if int64(len(b)) != size {
		return io.ErrUnexpectedEOF // S3 refuses a body shorter than Content-Length
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[key] = b
	return nil
}

func (f *fakeObjects) Get(_ context.Context, key string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.data[key]
	if !ok {
		return nil, errors.New("no such key")
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (f *fakeObjects) DeleteMany(_ context.Context, keys []string) ([]string, error) {
	if f.delErr != nil {
		return nil, f.delErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, key := range keys {
		f.deleted = append(f.deleted, key)
		delete(f.data, key)
	}
	return keys, nil
}

func (f *fakeObjects) PresignGet(_ context.Context, key, mediaType, disposition string, _ time.Duration) (string, error) {
	return "https://storage.example/" + key + "?type=" + mediaType + "&disposition=" + disposition, nil
}

// fakeAttachmentRows keeps the rows with the real store's states and rules.
type fakeAttachmentRows struct {
	rows  map[string]store.AgentAttachment
	state map[string]string
	// readied records every id that was ever made visible.
	readied []string
}

func (f *fakeAttachmentRows) BeginAgentAttachment(_ context.Context, a store.AgentAttachment) error {
	a.CreatedAt = time.Now()
	f.rows[a.ID], f.state[a.ID] = a, "pending"
	return nil
}
func (f *fakeAttachmentRows) ReadyAgentAttachment(_ context.Context, id, sum string, expiry time.Time) (store.AgentAttachment, error) {
	if f.state[id] != "pending" {
		return store.AgentAttachment{}, store.ErrNotFound
	}
	a := f.rows[id]
	a.SHA256 = sum
	if a.SessionID == "" {
		a.ExpiresAt = &expiry
	}
	f.rows[id], f.state[id] = a, "ready"
	f.readied = append(f.readied, id)
	return a, nil
}
func (f *fakeAttachmentRows) GetAgentAttachment(_ context.Context, id string) (store.AgentAttachment, error) {
	if f.state[id] != "ready" {
		return store.AgentAttachment{}, store.ErrNotFound
	}
	return f.rows[id], nil
}
func (f *fakeAttachmentRows) ListAgentSessionAttachments(_ context.Context, sessionID string, q store.AgentAttachmentListQuery) ([]store.AgentAttachment, error) {
	var out []store.AgentAttachment
	for id, a := range f.rows {
		if a.SessionID == sessionID && f.state[id] == "ready" && (q.Turn == 0 || a.Turn == q.Turn) && (q.Direction == "" || a.Direction == q.Direction) {
			out = append(out, a)
		}
	}
	slices.SortFunc(out, func(a, b store.AgentAttachment) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}
func (f *fakeAttachmentRows) ListAgentSessionInputs(ctx context.Context, sessionID string) ([]store.AgentAttachment, error) {
	return f.ListAgentSessionAttachments(ctx, sessionID, store.AgentAttachmentListQuery{Direction: store.AgentAttachmentInput})
}
func (f *fakeAttachmentRows) RetireAgentAttachment(_ context.Context, id string, unboundOnly bool) error {
	if f.state[id] == "" || f.state[id] == "deleted" || (unboundOnly && f.rows[id].SessionID != "") {
		return store.ErrNotFound
	}
	f.state[id] = "deleted"
	return nil
}
func (f *fakeAttachmentRows) ExpireAgentAttachments(_ context.Context, now time.Time) (int, error) {
	n := 0
	for id, a := range f.rows {
		if f.state[id] != "deleted" && a.ExpiresAt != nil && !a.ExpiresAt.After(now) && (f.state[id] == "pending" || a.SessionID == "") {
			f.state[id] = "deleted"
			n++
		}
	}
	return n, nil
}
func (f *fakeAttachmentRows) ClaimRetiredAgentAttachments(_ context.Context, limit int, _ time.Time) ([]store.AgentAttachment, error) {
	var out []store.AgentAttachment
	for id, a := range f.rows {
		if f.state[id] == "deleted" && len(out) < limit {
			out = append(out, a)
		}
	}
	return out, nil
}
func (f *fakeAttachmentRows) PurgeAgentAttachments(_ context.Context, list []string) error {
	for _, id := range list {
		delete(f.rows, id)
		delete(f.state, id)
	}
	return nil
}

// bind applies the real store's binding rule: ready inputs of the workspace,
// either the caller's unbound uploads or this session's own files.
func (f *fakeAttachmentRows) bind(workspaceID, sessionID string, turn int, b store.AgentAttachmentBinding, apply bool) ([]store.TurnAttachment, error) {
	var snapshot []store.TurnAttachment
	for _, id := range b.IDs {
		a, ok := f.rows[id]
		own := (a.SessionID != "" && a.SessionID == sessionID) || (a.SessionID == "" && a.CreatedBy == b.Caller)
		if !ok || f.state[id] != "ready" || a.Direction != store.AgentAttachmentInput || a.WorkspaceID != workspaceID || !own {
			return nil, &store.AgentAttachmentInvalidError{ID: id}
		}
	}
	if !apply {
		return nil, nil
	}
	for _, id := range b.IDs {
		a := f.rows[id]
		if a.SessionID == "" {
			a.SessionID, a.Turn, a.ExpiresAt = sessionID, turn, nil
			f.rows[id] = a
		}
		snapshot = append(snapshot, store.TurnAttachment{ID: a.ID, Name: a.Name, MediaType: a.MediaType, Size: a.SizeBytes})
	}
	return snapshot, nil
}

// bindingStore is the agent-session fake with the PG store's attachment
// behavior: binding inside the turn write, retiring inside the delete.
type bindingStore struct {
	*fakeStore
	rows *fakeAttachmentRows
}

func (b bindingStore) CreateAgentSession(ctx context.Context, in store.AgentSession) (store.AgentSession, error) {
	if _, err := b.rows.bind(in.WorkspaceID, "", 1, in.InitialAttachments, false); err != nil {
		return store.AgentSession{}, err
	}
	out, err := b.fakeStore.CreateAgentSession(ctx, in)
	if err != nil {
		return out, err
	}
	snapshot, _ := b.rows.bind(in.WorkspaceID, out.ID, 1, in.InitialAttachments, true)
	b.setSnapshot(out.ID, 1, snapshot)
	return out, nil
}

func (b bindingStore) BeginAgentSessionTurn(ctx context.Context, id, prompt, delivery, phase, status string, binding store.AgentAttachmentBinding) (store.AgentSession, error) {
	row := b.fakeStore.rows[id]
	snapshot, err := b.rows.bind(row.WorkspaceID, id, row.Turns+1, binding, true)
	if err != nil {
		return store.AgentSession{}, err
	}
	out, err := b.fakeStore.BeginAgentSessionTurn(ctx, id, prompt, delivery, phase, status, binding)
	if err == nil {
		b.setSnapshot(id, out.Turns, snapshot)
	}
	return out, err
}

func (b bindingStore) DeleteAgentSession(ctx context.Context, id string) error {
	for aid, a := range b.rows.rows {
		if a.SessionID == id && b.rows.state[aid] != "deleted" {
			b.rows.state[aid] = "deleted"
		}
	}
	return b.fakeStore.DeleteAgentSession(ctx, id)
}

func (b bindingStore) setSnapshot(id string, turn int, snapshot []store.TurnAttachment) {
	t := b.fakeStore.turns[id][turn]
	t.Attachments = snapshot
	b.fakeStore.turns[id][turn] = t
}

type fakeCopier struct {
	calls    []map[string]string
	modes    map[string]int64
	err      error
	attempts int
}

func (f *fakeCopier) UploadAgentSessionFiles(_ context.Context, _, _, _ string, open func() io.ReadCloser) error {
	f.attempts++
	archive := open()
	defer archive.Close()
	entries := map[string]string{}
	f.modes = map[string]int64{}
	reader := tar.NewReader(archive)
	for {
		h, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(reader)
		entries[h.Name] = string(body)
		f.modes[h.Name] = h.Mode
	}
	f.calls = append(f.calls, entries)
	return f.err
}

func attachmentFixture() (*Service, *fakeStore, *fakeLifecycle, *fakeObjects, *fakeAttachmentRows, *fakeCopier) {
	svc, st, fga, lifecycle := fixture()
	fga.members["carol"] = "tea-a"
	objects := &fakeObjects{data: map[string][]byte{}}
	rows := &fakeAttachmentRows{rows: map[string]store.AgentAttachment{}, state: map[string]string{}}
	copier := &fakeCopier{}
	svc.Attachments, svc.AttachmentRows, svc.AttachmentCopier = objects, rows, copier
	svc.Store = bindingStore{fakeStore: st, rows: rows}
	return svc, st, lifecycle, objects, rows, copier
}

var pngBytes = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 32)...)

func upload(t *testing.T, svc *Service, who, name string, body []byte) Attachment {
	t.Helper()
	a, err := svc.UploadAttachment(caller(who), UploadAttachmentRequest{
		OwnerID: "tea-a", Name: name, Size: int64(len(body)), Body: bytes.NewReader(body),
	})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func codeOf(err error) string {
	var coded *core.CodedError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return ""
}

func manifestOf(t *testing.T, lc *fakeLifecycle) attachmentManifest {
	t.Helper()
	var m attachmentManifest
	if err := json.Unmarshal([]byte(lc.driverEnv["BEX_AGENT_ATTACHMENTS"]), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestUploadSniffsTheTypeFromTheBytes(t *testing.T) {
	svc, _, _, objects, rows, _ := attachmentFixture()
	// A PNG named like text, and HTML named like an image: the bytes decide.
	png := upload(t, svc, "alice", "notes.txt", pngBytes)
	html := upload(t, svc, "alice", "../../shot.png", []byte("<html><script>alert(1)</script></html>"))
	if png.MediaType != "image/png" || html.MediaType != "text/html" {
		t.Fatalf("types = %q, %q", png.MediaType, html.MediaType)
	}
	if html.Name != "shot.png" {
		t.Fatalf("name = %q, want the base name only", html.Name)
	}
	sum := sha256.Sum256(pngBytes)
	if png.SHA256 != hex.EncodeToString(sum[:]) || png.Size != int64(len(pngBytes)) {
		t.Fatalf("hash/size = %s/%d", png.SHA256, png.Size)
	}
	row := rows.rows[png.ID]
	if row.ObjectKey != "tea-a/"+png.ID || !bytes.Equal(objects.data[row.ObjectKey], pngBytes) || rows.state[png.ID] != "ready" {
		t.Fatalf("row = %+v state %s", row, rows.state[png.ID])
	}
	if row.ExpiresAt == nil || row.SessionID != "" || row.CreatedBy != "alice" {
		t.Fatalf("row = %+v, want an unbound upload owned by alice that expires", row)
	}
}

func TestUploadRefusalsAndFailures(t *testing.T) {
	svc, _, _, objects, rows, _ := attachmentFixture()
	_, err := svc.UploadAttachment(caller("alice"), UploadAttachmentRequest{OwnerID: "tea-a", Name: "big", Size: maxAttachmentBytes + 1, Body: strings.NewReader("")})
	if !errors.Is(err, core.ErrTooLarge) || codeOf(err) != "AGENT_ATTACHMENT_TOO_LARGE" {
		t.Fatalf("oversize err = %v", err)
	}
	// Declared longer than what arrives: a client fault, the row retired.
	_, err = svc.UploadAttachment(caller("alice"), UploadAttachmentRequest{OwnerID: "tea-a", Name: "short", Size: 10, Body: strings.NewReader("abc")})
	if codeOf(err) != "AGENT_ATTACHMENT_INVALID" || !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("short err = %v", err)
	}
	// Storage failing is not the client's fault, and its row is retired too.
	objects.putErr = errors.New("wasabi 500")
	_, err = svc.UploadAttachment(caller("alice"), UploadAttachmentRequest{OwnerID: "tea-a", Name: "x", Size: 1, Body: strings.NewReader("x")})
	if codeOf(err) != "AGENT_ATTACHMENTS_UNAVAILABLE" || !errors.Is(err, core.ErrUnavailable) {
		t.Fatalf("storage err = %v", err)
	}
	objects.putErr = nil
	for id, state := range rows.state {
		if state != "deleted" {
			t.Fatalf("failed upload %s left state %s, want it queued for the sweep", id, state)
		}
	}
	_, err = svc.UploadAttachment(caller("bob"), UploadAttachmentRequest{OwnerID: "tea-a", Name: "x", Size: 1, Body: strings.NewReader("x")})
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("non-member err = %v", err)
	}
	svc.Attachments = nil
	_, err = svc.UploadAttachment(caller("alice"), UploadAttachmentRequest{OwnerID: "tea-a", Name: "x", Size: 1, Body: strings.NewReader("x")})
	if codeOf(err) != "AGENT_ATTACHMENTS_UNAVAILABLE" {
		t.Fatalf("unconfigured err = %v", err)
	}
}

func TestUnboundUploadsBelongToTheirUploader(t *testing.T) {
	svc, _, _, _, _, _ := attachmentFixture()
	a := upload(t, svc, "alice", "log.txt", []byte("hello"))
	if _, err := svc.GetAttachment(caller("carol"), a.ID); codeOf(err) != "AGENT_ATTACHMENT_NOT_FOUND" {
		t.Fatalf("another member read an unbound upload: %v", err)
	}
	got, err := svc.GetAttachment(caller("alice"), a.ID)
	if err != nil || !strings.Contains(got.DownloadURL, "disposition=attachment") {
		t.Fatalf("owner read = %+v, %v", got, err)
	}
	if err := svc.DeleteAttachment(caller("carol"), a.ID); codeOf(err) != "AGENT_ATTACHMENT_NOT_FOUND" {
		t.Fatalf("another member deleted an unbound upload: %v", err)
	}
	if err := svc.DeleteAttachment(caller("alice"), a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetAttachment(caller("alice"), a.ID); codeOf(err) != "AGENT_ATTACHMENT_NOT_FOUND" {
		t.Fatalf("deleted upload still readable: %v", err)
	}
}

func TestBoundAttachmentsFollowSessionAccess(t *testing.T) {
	svc, _, _, _, _, _ := attachmentFixture()
	a := upload(t, svc, "alice", "log.txt", []byte("hello"))
	view, err := svc.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Task: "t"}, AttachmentIDs: []string{a.ID}})
	if err != nil {
		t.Fatal(err)
	}
	// Another operator of the session's workspace reads it; an outsider cannot.
	if got, err := svc.GetAttachment(caller("carol"), a.ID); err != nil || got.SessionID != view.ID || got.Turn != 1 {
		t.Fatalf("operator read = %+v, %v", got, err)
	}
	if _, err := svc.GetAttachment(caller("bob"), a.ID); err == nil {
		t.Fatal("a non-member read a bound attachment")
	}
	if err := svc.DeleteAttachment(caller("alice"), a.ID); codeOf(err) != "AGENT_ATTACHMENT_BOUND" {
		t.Fatalf("deleting a bound file = %v", err)
	}
	if _, err := svc.ListAttachments(caller("bob"), view.ID, AttachmentListQuery{}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("outsider list = %v", err)
	}
}

func TestAnotherMembersUploadCannotBeAttached(t *testing.T) {
	svc, st, _, _, _, _ := attachmentFixture()
	theirs := upload(t, svc, "carol", "x", []byte("x"))
	_, err := svc.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Task: "t"}, AttachmentIDs: []string{theirs.ID}})
	if codeOf(err) != "AGENT_ATTACHMENT_INVALID" {
		t.Fatalf("err = %v", err)
	}
	if len(st.rows) != 0 {
		t.Fatal("a refused binding still created the session")
	}
}

func TestCreateCopiesAttachmentsIntoTheSandbox(t *testing.T) {
	svc, _, lifecycle, _, _, copier := attachmentFixture()
	shot := upload(t, svc, "alice", "shot.png", pngBytes)
	log := upload(t, svc, "alice", "build.log", []byte("error: boom\n"))
	view, err := svc.Create(caller("alice"), CreateRequest{
		OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Task: "fix the build"},
		AttachmentIDs: []string{shot.ID, log.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.FailureReasonCode != "" {
		t.Fatalf("session failed: %s", view.FailureReason)
	}
	if len(copier.calls) != 1 {
		t.Fatalf("copies = %d, want one archive", len(copier.calls))
	}
	entries := copier.calls[0]
	if entries["1/"+log.ID+"-build.log"] != "error: boom\n" || entries["1/"+shot.ID+"-shot.png"] != string(pngBytes) {
		t.Fatalf("archive entries = %v", entries)
	}
	if copier.modes["1/"+log.ID+"-build.log"] != 0o444 {
		t.Fatalf("mode = %o, want read-only", copier.modes["1/"+log.ID+"-build.log"])
	}
	m := manifestOf(t, lifecycle)
	if m.WaitMs <= 0 || len(m.Files) != 2 {
		t.Fatalf("manifest = %+v", m)
	}
	for _, f := range m.Files {
		if !strings.HasPrefix(f.Path, "/attachments/1/att-") || f.Turn != 1 || !f.Current {
			t.Fatalf("manifest file = %+v", f)
		}
	}
}

func TestSteerCopiesEveryTurnsFilesAndMarksItsOwn(t *testing.T) {
	svc, _, lifecycle, _, _, copier := attachmentFixture()
	first := upload(t, svc, "alice", "first.log", []byte("1"))
	view, err := svc.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Task: "t"}, AttachmentIDs: []string{first.ID}})
	if err != nil {
		t.Fatal(err)
	}
	st := svc.Store.(bindingStore).fakeStore
	row := st.rows[view.ID]
	row.Phase = PhaseCompleted
	st.rows[view.ID] = row

	second := upload(t, svc, "alice", "second.log", []byte("22"))
	// Turn 2 attaches a new file and refers to turn 1's file again.
	if _, err := svc.Steer(caller("alice"), SteerRequest{SessionID: view.ID, Prompt: "and this", AttachmentIDs: []string{second.ID, first.ID}}); err != nil {
		t.Fatal(err)
	}
	last := copier.calls[len(copier.calls)-1]
	if last["1/"+first.ID+"-first.log"] != "1" || last["2/"+second.ID+"-second.log"] != "22" {
		t.Fatalf("turn 2 archive = %v, want both turns' files", last)
	}
	for _, f := range manifestOf(t, lifecycle).Files {
		if !f.Current {
			t.Fatalf("%s not marked current, though turn 2 attached it", f.Name)
		}
	}
	if got, _ := svc.GetAttachment(caller("alice"), first.ID); got.Turn != 1 {
		t.Fatalf("re-attached file moved to turn %d, want it to keep turn 1", got.Turn)
	}
}

func TestAttachmentCopyFailureFailsTheTurnWithACode(t *testing.T) {
	svc, st, _, _, _, copier := attachmentFixture()
	copier.err = errors.New("gateway down")
	a := upload(t, svc, "alice", "log.txt", []byte("x"))
	view, err := svc.Create(caller("alice"), CreateRequest{
		OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Task: "t"}, AttachmentIDs: []string{a.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	row := st.rows[view.ID]
	if row.Phase != PhaseFailed || row.FailureReasonCode != "ATTACHMENT_COPY_FAILED" {
		t.Fatalf("row = phase %s code %q", row.Phase, row.FailureReasonCode)
	}
}

func TestCreateWithoutAttachmentsSetsNoManifest(t *testing.T) {
	svc, _, lifecycle, _, _, copier := attachmentFixture()
	if _, err := svc.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Task: "t"}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := lifecycle.driverEnv["BEX_AGENT_ATTACHMENTS"]; ok || len(copier.calls) != 0 {
		t.Fatalf("a session without files got a manifest or a copy")
	}
}

func TestDeletingASessionRetiresItsFiles(t *testing.T) {
	svc, st, _, objects, rows, _ := attachmentFixture()
	a := upload(t, svc, "alice", "log.txt", []byte("x"))
	view, err := svc.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Task: "t"}, AttachmentIDs: []string{a.ID}})
	if err != nil {
		t.Fatal(err)
	}
	row := st.rows[view.ID]
	row.Phase, row.SandboxID = PhaseCompleted, ""
	st.rows[view.ID] = row
	if err := svc.Delete(caller("alice"), view.ID); err != nil {
		t.Fatal(err)
	}
	if rows.state[a.ID] != "deleted" {
		t.Fatalf("state = %s, want the file retired with its session", rows.state[a.ID])
	}
	if err := sweepAttachments(context.Background(), svc, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, ok := objects.data["tea-a/"+a.ID]; ok {
		t.Fatal("the sweep left a deleted session's object")
	}
}

func TestValidateAttachmentIDs(t *testing.T) {
	svc, _, _, _, _, _ := attachmentFixture()
	id := upload(t, svc, "alice", "a", []byte("a")).ID
	many := make([]string, maxAttachmentsPerTurn+1)
	for i := range many {
		many[i] = ids.New(ids.AgentAttachment)
	}
	for name, tc := range map[string]struct {
		ids  []string
		code string
	}{
		"too many":   {many, "AGENT_ATTACHMENT_LIMIT"},
		"duplicate":  {[]string{id, id}, "AGENT_ATTACHMENT_INVALID"},
		"wrong kind": {[]string{"ags-d0abc"}, "AGENT_ATTACHMENT_INVALID"},
	} {
		if err := svc.validateAttachmentIDs(tc.ids); codeOf(err) != tc.code {
			t.Errorf("%s: err = %v, want %s", name, err, tc.code)
		}
	}
	if err := svc.validateAttachmentIDs([]string{id}); err != nil {
		t.Fatal(err)
	}
	svc.AttachmentCopier = nil
	if err := svc.validateAttachmentIDs([]string{id}); codeOf(err) != "AGENT_ATTACHMENTS_UNAVAILABLE" {
		t.Fatalf("unconfigured err = %v", err)
	}
}

func TestStoreRefusalsAreCoded(t *testing.T) {
	for err, want := range map[error]string{
		&store.AgentAttachmentInvalidError{ID: "att-x"}: "AGENT_ATTACHMENT_INVALID",
		store.ErrAgentSessionAttachmentQuota:            "AGENT_SESSION_ATTACHMENT_QUOTA",
		store.ErrUnboundAttachmentQuota:                 "AGENT_ATTACHMENT_UNBOUND_QUOTA",
	} {
		if got := codeOf(mapAttachmentError(err)); got != want {
			t.Errorf("%v → %s, want %s", err, got, want)
		}
	}
}

func TestSweepPurgesExpiredFilesAndKeepsWhatItCannotDelete(t *testing.T) {
	svc, _, _, objects, rows, _ := attachmentFixture()
	a := upload(t, svc, "alice", "old", []byte("x"))
	past := time.Now().Add(-time.Minute)
	row := rows.rows[a.ID]
	row.ExpiresAt = &past
	rows.rows[a.ID] = row

	objects.delErr = errors.New("storage down")
	if err := sweepAttachments(context.Background(), svc, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, kept := rows.rows[a.ID]; !kept {
		t.Fatal("a failed object delete dropped the row that tracks it")
	}
	objects.delErr = nil
	if err := sweepAttachments(context.Background(), svc, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, kept := rows.rows[a.ID]; kept || len(objects.data) != 0 {
		t.Fatalf("expired upload not purged: rows=%d objects=%d", len(rows.rows), len(objects.data))
	}
}

func TestListAttachmentsValidatesTurn(t *testing.T) {
	svc, _, _, _, _, _ := attachmentFixture()
	a := upload(t, svc, "alice", "log.txt", []byte("x"))
	view, err := svc.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Task: "t"}, AttachmentIDs: []string{a.ID}})
	if err != nil {
		t.Fatal(err)
	}
	zero, one, two := 0, 1, 2
	if _, err := svc.ListAttachments(caller("alice"), view.ID, AttachmentListQuery{Turn: &zero}); codeOf(err) != "AGENT_ATTACHMENT_INVALID" {
		t.Fatalf("turn 0 = %v", err)
	}
	if got, err := svc.ListAttachments(caller("alice"), view.ID, AttachmentListQuery{Turn: &one}); err != nil || len(got) != 1 || got[0].DownloadURL == "" {
		t.Fatalf("turn 1 = %+v, %v", got, err)
	}
	if got, _ := svc.ListAttachments(caller("alice"), view.ID, AttachmentListQuery{Turn: &two}); len(got) != 0 {
		t.Fatalf("turn 2 = %+v", got)
	}
}

func TestRESTContentRedirectsAndTurnParsing(t *testing.T) {
	svc, _, _, _, _, _ := attachmentFixture()
	a := upload(t, svc, "alice", "shot.png", pngBytes)
	view, err := svc.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Task: "t"}, AttachmentIDs: []string{a.ID}})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	svc.registerAttachmentREST(mux)
	serve := func(target string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, target, nil).WithContext(caller("alice"))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	rec := serve("/v1/agent-attachments/" + a.ID + "/content")
	if rec.Code != http.StatusFound || !strings.HasPrefix(rec.Header().Get("Location"), "https://storage.example/tea-a/"+a.ID) ||
		!strings.Contains(rec.Header().Get("Location"), "disposition=inline") || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("content = %d %v", rec.Code, rec.Header())
	}
	if rec := serve("/v1/agent-sessions/" + view.ID + "/attachments?turn=x"); rec.Code != http.StatusBadRequest {
		t.Fatalf("turn=x = %d", rec.Code)
	}
	if rec := serve("/v1/agent-sessions/" + view.ID + "/attachments?turn=0"); rec.Code != http.StatusBadRequest {
		t.Fatalf("turn=0 = %d", rec.Code)
	}
	rec = serve("/v1/agent-sessions/" + view.ID + "/attachments?turn=1")
	var list []Attachment
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list) != 1 || list[0].ID != a.ID {
		t.Fatalf("list = %d %s", rec.Code, rec.Body)
	}
}

func TestAttachmentNamesTypesAndDisposition(t *testing.T) {
	for in, want := range map[string]string{
		"report.csv":            "report.csv",
		`C:\Users\me\log.txt`:   "log.txt",
		"a/b/../c.txt":          "c.txt",
		"bad\x00\x07:name?.txt": "bad___name_.txt",
		" .. ":                  "attachment",
		"evil\u202egnp.exe":     "evilgnp.exe", // a right-to-left override would render as "exe.png"
	} {
		if got := sanitizeAttachmentName(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
	// The sandbox file name `<att-id>-<name>` fits the 255-byte limit even for
	// the longest name the sanitizer keeps, multibyte runes included.
	for _, long := range []string{strings.Repeat("a", 400), strings.Repeat("é", 400)} {
		name := sanitizeAttachmentName(long)
		if n := len(ids.New(ids.AgentAttachment)) + 1 + len(name); n > 255 || !utf8Valid(name) {
			t.Fatalf("sandbox name is %d bytes for %d-byte input", n, len(long))
		}
	}
	if got := sniffMediaType([]byte("a,b\n1,2\n"), "data.csv"); got != "text/csv" {
		t.Errorf("csv = %q", got)
	}
	// An extension may refine plain text only: a .csv that is really a PNG stays a PNG.
	if got := sniffMediaType(pngBytes, "data.csv"); got != "image/png" {
		t.Errorf("png named csv = %q", got)
	}
	if got := attachmentDisposition("a b.png", "image/png"); got != `inline; filename="a b.png"` {
		t.Errorf("png disposition = %q", got)
	}
	if got := attachmentDisposition("x.svg", "image/svg+xml"); !strings.HasPrefix(got, "attachment;") {
		t.Errorf("svg disposition = %q, want download-only", got)
	}
	if got := attachmentDisposition("résumé.pdf", "application/pdf"); got != "attachment; filename*=utf-8''r%C3%A9sum%C3%A9.pdf" {
		t.Errorf("unicode disposition = %q", got)
	}
}

func utf8Valid(s string) bool { return strings.ToValidUTF8(s, "\uFFFD") == s }
