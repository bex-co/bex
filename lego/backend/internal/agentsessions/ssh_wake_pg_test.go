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
	"net/http/httptest"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w10/m597 against the real store: the SSH gateway's wake verb starts a
// closed session's sandbox through Service.Resume as the registered key's
// owner, with every check that owner's own resume gets, and starts it once.

// wakeVerb serves the verb over the env's store and service and returns its
// client, with an SSH key registered for each subject.
func (e *retainEnv) wakeVerb(t *testing.T, subjects ...string) (*agentsession.SessionWakeClient, map[string]string) {
	t.Helper()
	fingerprints := map[string]string{}
	for _, subject := range subjects {
		id := ids.New(ids.SSHKey)
		if _, err := e.pg.CreateSSHKey(context.Background(), store.SSHKey{ID: id, Subject: subject, Name: "laptop", PublicKey: "ssh-ed25519 AAAA" + id, Fingerprint: "SHA256:" + id}); err != nil {
			t.Fatal(err)
		}
		fingerprints[subject] = "SHA256:" + id
	}
	resume := func(ctx context.Context, id string) (string, error) {
		view, err := e.svc.Resume(ctx, id)
		return view.Phase, err
	}
	srv := httptest.NewServer(&agentsession.SessionWakeHandler{Secret: []byte("exec-secret"), Resume: resume, Keys: e.pg, Grants: e.pg, Nonce: e.pg})
	t.Cleanup(srv.Close)
	return &agentsession.SessionWakeClient{URL: srv.URL, Secret: []byte("exec-secret")}, fingerprints
}

func (e *retainEnv) wake(t *testing.T, client *agentsession.SessionWakeClient, subject, fingerprint string) agentsession.SessionWakeResponse {
	t.Helper()
	out, err := client.Wake(context.Background(), agentsession.SessionWakeRequest{SessionID: e.id, Subject: subject, Fingerprint: fingerprint, SourceIP: "203.0.113.7"})
	if err != nil {
		t.Fatalf("wake as %s: %v", subject, err)
	}
	return out
}

func TestSSHWakeVerbStartsAClosedSessionAsTheKeysOwnerPG(t *testing.T) {
	e := editorEnv(t, gitStatus(statusGit{}))
	e.noImage(t)
	other := e.create(t, "") // another live sandbox in the workspace
	var starts []func()
	e.svc.dispatchRunner = func(fn func()) { starts = append(starts, fn) }
	client, keys := e.wakeVerb(t, "alice", "contrib", "viewer")

	// A viewer (no can_operate) and a contributor (no can_create) may not
	// start it; nothing starts and the session is untouched.
	for _, subject := range []string{"viewer", "contrib"} {
		if out := e.wake(t, client, subject, keys[subject]); out.Refusal != agentsession.SessionWakeForbidden {
			t.Fatalf("wake as %s = %+v, want forbidden", subject, out)
		}
	}
	// A key presented for another subject is refused before resume.
	if out := e.wake(t, client, "alice", keys["viewer"]); out.Refusal != agentsession.SessionWakeForbidden {
		t.Fatalf("alice's wake with the viewer's key = %+v, want forbidden", out)
	}
	// Admission refuses as it refuses a turn: the live-sandbox cap.
	if e.phase(t, other.ID) != PhaseRunning {
		t.Fatalf("the other session is %s, want running", e.phase(t, other.ID))
	}
	e.svc.MaxLiveSandboxes = 1
	if out := e.wake(t, client, "alice", keys["alice"]); out.Refusal != agentsession.SessionWakeRefused || out.Code != codeLiveLimit || out.Message == "" {
		t.Fatalf("wake past the live-sandbox cap = %+v, want %s", out, codeLiveLimit)
	}
	e.svc.MaxLiveSandboxes = 0
	if len(starts) != 0 || e.session(t).Phase != PhaseCompleted {
		t.Fatalf("after the refusals: %d starts, phase %s; want none and completed", len(starts), e.session(t).Phase)
	}

	// The owner starts it: resuming, one start; asking again (a second
	// login, another key) returns it unchanged and starts nothing more.
	if out := e.wake(t, client, "alice", keys["alice"]); out.Phase != PhaseResuming || out.Refusal != "" {
		t.Fatalf("wake as alice = %+v, want resuming", out)
	}
	if out := e.wake(t, client, "alice", keys["alice"]); out.Phase != PhaseResuming {
		t.Fatalf("a second wake = %+v, want resuming unchanged", out)
	}
	if len(starts) != 1 || e.session(t).Phase != PhaseResuming {
		t.Fatalf("starts = %d phase %s, want one start while resuming", len(starts), e.session(t).Phase)
	}
}

// An archived session is refused with resume's own code: unarchive first.
func TestSSHWakeVerbRefusesAnArchivedSessionPG(t *testing.T) {
	e := editorEnv(t, gitStatus(statusGit{}))
	client, keys := e.wakeVerb(t, "alice")
	if _, err := e.svc.Archive(caller("alice"), e.id); err != nil {
		t.Fatal(err)
	}
	if out := e.wake(t, client, "alice", keys["alice"]); out.Refusal != agentsession.SessionWakeRefused || out.Code != codeArchived {
		t.Fatalf("wake of an archived session = %+v, want AGENT_SESSION_ARCHIVED", out)
	}
}
