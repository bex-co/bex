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

package core

import (
	"context"
	"errors"
	"testing"
)

// recorded_test.go pins AuthorizeWorkspaceOrRecorded (w9/121): the store is
// asked only when OpenFGA plainly refused a person signed in directly, and
// its yes is a positive decision in every other respect — the OAuth
// capability, the workspace's login requirement and the audit row.

// deleteLikeVerb stands in for workspaces.Service.Delete.
type deleteLikeVerb struct{ b *Base }

func (v deleteLikeVerb) Delete(ctx context.Context, workspaceID string, recorded RecordedWorkspaceDecision) error {
	_, err := v.b.AuthorizeWorkspaceOrRecorded(ctx, RelCanManage, workspaceID, recorded)
	return err
}

func TestAuthorizeWorkspaceOrRecorded(t *testing.T) {
	session := Identity{Subject: "alice", Method: MethodSession}
	humanOAuth := Identity{Subject: "alice", Method: MethodOAuth2, ClientID: "cli", Human: true, CanonicalScopes: ScopeWrite}
	readOnlyOAuth := Identity{Subject: "alice", Method: MethodOAuth2, ClientID: "cli", Human: true, CanonicalScopes: ScopeRead}
	apiKey := Identity{Subject: "key-1", Method: MethodOAuth2, ClientID: "key-1"}
	s3Key := Identity{Subject: "alice", Method: MethodS3Key, AuditSubject: "s3k-1"}
	sshKey := Identity{Subject: "alice", Method: MethodSSH}
	actingFor := Identity{Subject: "alice", Method: MethodSession, AuditMethod: MethodShell}

	cases := []struct {
		name      string
		authz     Checker
		id        Identity
		workspace string
		admit     bool
		admitErr  error
		gate      TwoFactorGate
		wantErr   error
		wantAsked bool
		wantRow   AuditOutcome
	}{
		{name: "OpenFGA admits, so the store is not asked", authz: &fakeAllowChecker{}, id: session, workspace: "tea-a", wantRow: AuditAllowed},
		{name: "a refused person the store records is admitted", authz: fakeDenyChecker{}, id: session, workspace: "tea-a", admit: true, wantAsked: true, wantRow: AuditAllowed},
		{name: "a refused human OAuth token the store records is admitted", authz: fakeDenyChecker{}, id: humanOAuth, workspace: "tea-a", admit: true, wantAsked: true, wantRow: AuditAllowed},
		{name: "a refused person the store does not record stays refused", authz: fakeDenyChecker{}, id: session, workspace: "tea-a", wantErr: ErrForbidden, wantAsked: true, wantRow: AuditDenied},
		{name: "a token without the write capability is refused before anyone is asked", authz: fakeDenyChecker{}, id: readOnlyOAuth, workspace: "tea-a", admit: true, wantErr: ErrForbidden, wantRow: AuditDenied},
		{name: "an API key is never asked about", authz: fakeDenyChecker{}, id: apiKey, workspace: "tea-a", admit: true, wantErr: ErrForbidden, wantRow: AuditDenied},
		{name: "an S3 key acting as its creator is never asked about", authz: fakeDenyChecker{}, id: s3Key, workspace: "tea-a", admit: true, wantErr: ErrForbidden, wantRow: AuditDenied},
		{name: "an SSH key is never asked about", authz: fakeDenyChecker{}, id: sshKey, workspace: "tea-a", admit: true, wantErr: ErrForbidden, wantRow: AuditDenied},
		{name: "a credential acting for someone is never asked about", authz: fakeDenyChecker{}, id: actingFor, workspace: "tea-a", admit: true, wantErr: ErrForbidden, wantRow: AuditDenied},
		{name: "an id OpenFGA cannot hold is never asked about", authz: fakeDenyChecker{}, id: session, workspace: "tea a", admit: true, wantErr: ErrForbidden, wantRow: AuditDenied},
		{name: "an unreachable OpenFGA is no refusal to override", authz: &unavailableChecker{}, id: session, workspace: "tea-a", admit: true, wantErr: ErrAuthzUnavailable, wantRow: AuditDenied},
		{name: "a store that cannot answer fails closed", authz: fakeDenyChecker{}, id: session, workspace: "tea-a", admitErr: errors.New("postgres: connection refused"), wantErr: ErrAuthzUnavailable, wantAsked: true, wantRow: AuditDenied},
		{name: "the workspace's login requirement still applies", authz: fakeDenyChecker{}, id: session, workspace: "tea-a", admit: true, gate: &recordingGate{refuse: map[string]bool{"tea-a": true}}, wantErr: ErrForbidden, wantAsked: true, wantRow: AuditDenied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sink := &fakeAuditSink{}
			b := &Base{Authz: tc.authz, Audit: sink}
			if tc.gate != nil {
				b.TwoFactor = tc.gate
			}
			var asked []string
			recorded := func(_ context.Context, workspaceID, subject string) (bool, error) {
				asked = append(asked, workspaceID+"|"+subject)
				return tc.admit, tc.admitErr
			}
			err := deleteLikeVerb{b}.Delete(WithIdentity(context.Background(), tc.id), tc.workspace, recorded)
			switch {
			case tc.wantErr == nil && err != nil:
				t.Fatalf("err = %v, want admitted", err)
			case tc.wantErr != nil && !errors.Is(err, tc.wantErr):
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantAsked != (len(asked) > 0) {
				t.Fatalf("store asked %v, want asked=%v", asked, tc.wantAsked)
			}
			if tc.wantAsked && asked[0] != tc.workspace+"|"+tc.id.Subject {
				t.Fatalf("store asked about %v, want %s|%s", asked, tc.workspace, tc.id.Subject)
			}
			if sink.len() != 1 {
				t.Fatalf("audit rows = %+v, want exactly one", sink.events)
			}
			if ev := sink.events[0]; ev.Outcome != tc.wantRow || ev.Resource != WorkspaceObject(tc.workspace) || ev.Verb == "" {
				t.Fatalf("audit row = %+v, want %s on %s with its verb", ev, tc.wantRow, WorkspaceObject(tc.workspace))
			}
		})
	}
}
