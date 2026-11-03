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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// runningSession creates a session whose current sandbox is live on turn.
func runningSession(t *testing.T, svc *Service, st *fakeStore, turn int) string {
	t.Helper()
	view, err := svc.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Task: "t"}})
	if err != nil {
		t.Fatal(err)
	}
	setSession(st, view.ID, func(row *store.AgentSession) {
		row.SandboxID, row.Turns, row.Phase = "sbx-live", turn, PhaseRunning
	})
	return view.ID
}

func setSession(st *fakeStore, id string, mutate func(*store.AgentSession)) {
	st.rowMu.Lock()
	defer st.rowMu.Unlock()
	row := st.rows[id]
	mutate(&row)
	st.rows[id] = row
}

// shareEnvelope is what the gateway signs for the session's current Pod.
func shareEnvelope(sessionID, name string, body []byte) agentsession.ShareFileEnvelope {
	sum := sha256.Sum256(body)
	return agentsession.ShareFileEnvelope{Kind: agentsession.ShareFileKind, Workspace: "tea-a", SessionID: sessionID,
		PodName: "sbx-live-0", PodUID: "uid-live", Name: name, Size: int64(len(body)), SHA256: hex.EncodeToString(sum[:])}
}

func shareCode(err error) string {
	var typed *agentsession.ShareFileError
	if errors.As(err, &typed) {
		return typed.Code
	}
	return ""
}

func TestShareFileStoresAnOutputOfTheCurrentTurn(t *testing.T) {
	svc, st, _, objects, rows, _ := attachmentFixture()
	sessionID := runningSession(t, svc, st, 3)
	body := []byte("total,3\n")

	got, err := svc.ShareFile(context.Background(), shareEnvelope(sessionID, "../report.csv", body), bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	row := rows.rows[got.AttachmentID]
	if row.Direction != store.AgentAttachmentOutput || row.SessionID != sessionID || row.Turn != 3 ||
		row.CreatedBy != agentCreator || rows.state[got.AttachmentID] != "ready" || row.ExpiresAt != nil {
		t.Fatalf("row = %+v state=%s, want a ready output of turn 3", row, rows.state[got.AttachmentID])
	}
	if got.Name != "report.csv" || got.MediaType != "text/csv" || !bytes.Equal(objects.data[row.ObjectKey], body) {
		t.Fatalf("share = %+v", got)
	}
	// Members read it as a session file, by direction and turn.
	three := 3
	listed, err := svc.ListAttachments(caller("alice"), sessionID, AttachmentListQuery{Direction: store.AgentAttachmentOutput, Turn: &three})
	if err != nil || len(listed) != 1 || listed[0].ID != got.AttachmentID || listed[0].DownloadURL == "" {
		t.Fatalf("outputs = %+v, %v", listed, err)
	}
	if inputs, _ := svc.ListAttachments(caller("alice"), sessionID, AttachmentListQuery{Direction: store.AgentAttachmentInput}); len(inputs) != 0 {
		t.Fatalf("an output listed as an input: %+v", inputs)
	}
}

func TestShareFileRefusesBytesThatDoNotMatchTheEnvelope(t *testing.T) {
	svc, st, _, _, rows, _ := attachmentFixture()
	sessionID := runningSession(t, svc, st, 1)
	envelope := shareEnvelope(sessionID, "a.txt", []byte("declared"))
	_, err := svc.ShareFile(context.Background(), envelope, strings.NewReader("swapped!"))
	if shareCode(err) != agentsession.ShareFileInvalidRequest {
		t.Fatalf("err = %v, want invalid_request", err)
	}
	for id, state := range rows.state {
		if state != "deleted" {
			t.Fatalf("attachment %s left %s after a digest mismatch", id, state)
		}
	}
	if len(rows.readied) != 0 {
		t.Fatalf("mismatched bytes were visible before retirement: %v", rows.readied)
	}
}

// The share verb holds the gateway's Pod to the same current-sandbox rule as
// the Git, model and PR verbs.
func TestShareFileRefusesAnythingButTheCurrentSandbox(t *testing.T) {
	body := []byte("x")
	cases := map[string]struct {
		session  func(*store.AgentSession)
		envelope func(*agentsession.ShareFileEnvelope)
	}{
		"terminal session":       {session: func(r *store.AgentSession) { r.Phase = PhaseCompleted }},
		"canceled, in its grace": {session: func(r *store.AgentSession) { r.Phase = PhaseCanceling }},
		"no sandbox":             {session: func(r *store.AgentSession) { r.SandboxID = "" }},
		"stale pod generation":   {envelope: func(e *agentsession.ShareFileEnvelope) { e.PodName = "sbx-old-0" }},
		"no pod uid":             {envelope: func(e *agentsession.ShareFileEnvelope) { e.PodUID = "" }},
		"other workspace":        {envelope: func(e *agentsession.ShareFileEnvelope) { e.Workspace = "tea-b" }},
		"unknown session":        {envelope: func(e *agentsession.ShareFileEnvelope) { e.SessionID = "ags-missing" }},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, st, _, _, rows, _ := attachmentFixture()
			sessionID := runningSession(t, svc, st, 1)
			if tc.session != nil {
				setSession(st, sessionID, tc.session)
			}
			envelope := shareEnvelope(sessionID, "a.txt", body)
			if tc.envelope != nil {
				tc.envelope(&envelope)
			}
			if _, err := svc.ShareFile(context.Background(), envelope, bytes.NewReader(body)); shareCode(err) != agentsession.ShareFileForbidden {
				t.Fatalf("err = %v, want forbidden", err)
			}
			if len(rows.rows) != 0 {
				t.Fatalf("a refused share wrote rows: %+v", rows.rows)
			}
		})
	}
}

func TestShareFileWithoutAnAttachmentsStore(t *testing.T) {
	svc, _, _, _ := fixture()
	_, err := svc.ShareFile(context.Background(), shareEnvelope("ags-x", "a.txt", []byte("x")), strings.NewReader("x"))
	if shareCode(err) != agentsession.ShareFileUnavailable {
		t.Fatalf("err = %v, want unavailable", err)
	}
}

func TestShareFileQuotaMapsToItsCode(t *testing.T) {
	err := shareFileRefusal(mapAttachmentError(store.ErrAgentSessionAttachmentQuota))
	if err.Code != agentsession.ShareFileQuotaExceeded {
		t.Fatalf("quota refusal = %+v", err)
	}
}

// Every session gets a share URL that names it, once attachments exist.
func TestDriverEnvCarriesTheShareURL(t *testing.T) {
	svc, st, lifecycle, _, _, _ := attachmentFixture()
	sessionID := runningSession(t, svc, st, 1)
	shareURL := lifecycle.driverEnv["BEX_AGENT_SHARE_URL"]
	ns, sid, err := agentsession.ParseShareFilePath(shareURL[strings.Index(shareURL, agentsession.ShareFileProxyPath):])
	if err != nil || ns != "tea-a-sandbox" || sid != sessionID {
		t.Fatalf("share URL %q parsed to %q %q %v", shareURL, ns, sid, err)
	}

	plain, _, _, plainLifecycle := fixture()
	if _, err := plain.Create(caller("alice"), CreateRequest{OwnerID: "tea-a", AgentConfig: AgentConfig{Agent: "claude", Task: "t"}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := plainLifecycle.driverEnv["BEX_AGENT_SHARE_URL"]; ok {
		t.Fatal("a deployment without attachments advertised a share URL")
	}
}
