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

package oauthgrant

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
)

// GrantWindow is how recently Hydra must have handled a consent for
// RecordGrant to record it. The dashboard reports an approval right after
// Hydra accepts it, so a consent any older is not the one being reported.
const GrantWindow = 5 * time.Minute

// grantClockSkew tolerates a Hydra clock slightly ahead of bex-api's.
const grantClockSkew = time.Minute

// clientLookupTimeout bounds the revocation's client-name lookup: the member's
// revoke waits on it, and a row without the name is still recorded.
const clientLookupTimeout = 2 * time.Second

// WorkspaceLister lists the workspaces a subject belongs to.
// *store.PGStore is the production implementation.
type WorkspaceLister interface {
	SubjectWorkspaceIDs(ctx context.Context, subject string) ([]string, error)
}

// Access records a member's own decisions about an OAuth client's access to
// their account in the audit log (w4/m454): approving it on the dashboard's
// consent page as Render's GrantOAuthAccessEvent, revoking it under Connected
// agents as RevokeOAuthAccessEvent. A grant covers every workspace the member
// belongs to, so each is recorded once in each of them, with the member as
// the actor and the client as the target. Both write fail-open: a consent or a
// revocation never fails because its audit row did not land.
type Access struct {
	// HydraAdminURL is Hydra's admin API (BEX_HYDRA_ADMIN_URL).
	HydraAdminURL string
	// Client calls Hydra; nil means a client with a 10 s timeout on the shared
	// Ory transport.
	Client     *http.Client
	Workspaces WorkspaceLister
	Audit      core.AuditSink
	// Now is the clock of the window check and the revocation rows; nil means
	// time.Now.
	Now func() time.Time
}

func (a *Access) hydra() hydraAdmin { return newHydraAdmin(a.HydraAdminURL, a.Client) }

func (a *Access) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// RecordGrant records subject's approval of clientID, once in each of the
// subject's workspaces. The dashboard reports it, but Hydra is the evidence:
// the newest consent session Hydra holds for the pair must have been handled
// within GrantWindow, so a caller cannot record a grant they never gave. It
// reports false, and records nothing, when there is no such session or the
// client is one of the platform's own (skip_consent: the bex CLI, the desktop
// and mobile apps, the ops portals), whose sign-in is not an agent being given
// access; true means Hydra vouched for the grant and its rows went to the
// fail-open audit sink. The row ids derive from the consent's handled_at, so a
// repeated report of the same consent records nothing more.
func (a *Access) RecordGrant(ctx context.Context, subject, clientID string) (bool, error) {
	sessions, err := a.hydra().consentSessions(ctx, subject)
	if err != nil {
		return false, err
	}
	var newest *consentSession
	for i, s := range sessions {
		if s.ConsentRequest.Client.ClientID != clientID || s.HandledAt == nil {
			continue
		}
		if newest == nil || s.HandledAt.After(*newest.HandledAt) {
			newest = &sessions[i]
		}
	}
	now := a.now()
	if newest == nil || newest.ConsentRequest.Client.SkipConsent ||
		now.Sub(*newest.HandledAt) > GrantWindow || newest.HandledAt.Sub(now) > grantClockSkew {
		return false, nil
	}
	workspaces, err := a.Workspaces.SubjectWorkspaceIDs(ctx, subject)
	if err != nil {
		return false, err
	}
	handledAt := newest.HandledAt.UTC()
	evs := make([]core.AuditEvent, 0, len(workspaces))
	for _, workspace := range workspaces {
		ev := accessEvent(core.AuditVerbGrantOAuthAccess, subject, clientID, newest.ConsentRequest.Client.ClientName, workspace, handledAt)
		ev.ID = id.Derive(id.Audit, core.AuditVerbGrantOAuthAccess, subject, clientID, handledAt.Format(time.RFC3339Nano), workspace)
		ev.OAuthGrantScopes = newest.GrantScope
		evs = append(evs, ev)
	}
	// The dashboard gives up on the report after 3 s; the rows still land.
	core.RecordAuditEvents(context.WithoutCancel(ctx), a.Audit, evs...)
	return true, nil
}

// RecordRevoke records subject's revocation of clientID, once in each of the
// subject's workspaces. The dashboard has already revoked the grant at Hydra,
// so Hydra is asked only for the client's name. A client Hydra does not know
// records nothing: there was no grant to revoke. One Hydra cannot answer for
// is still recorded, without its name, because the revocation happened; the
// lookup's failure is then returned for the log.
func (a *Access) RecordRevoke(ctx context.Context, subject, clientID string) error {
	lookupCtx, cancel := context.WithTimeout(ctx, clientLookupTimeout)
	name, known, lookupErr := a.hydra().clientName(lookupCtx, clientID)
	cancel()
	if lookupErr == nil && !known {
		return nil
	}
	workspaces, err := a.Workspaces.SubjectWorkspaceIDs(ctx, subject)
	if err != nil {
		return err
	}
	at := a.now().UTC()
	evs := make([]core.AuditEvent, 0, len(workspaces))
	for _, workspace := range workspaces {
		evs = append(evs, accessEvent(core.AuditVerbRevokeOAuthAccess, subject, clientID, name, workspace, at))
	}
	core.RecordAuditEvents(context.WithoutCancel(ctx), a.Audit, evs...)
	if lookupErr != nil {
		return fmt.Errorf("recorded without the client's name: %w", lookupErr)
	}
	return nil
}

func accessEvent(verb, subject, clientID, clientName, workspace string, at time.Time) core.AuditEvent {
	return core.AuditEvent{
		Caller:       subject,
		CallerMethod: core.MethodSession,
		Verb:         verb,
		Resource:     core.WorkspaceObject(workspace),
		Target:       core.OAuthClientTarget(clientID),
		TargetName:   clientName,
		Outcome:      core.AuditAllowed,
		At:           at,
	}
}
