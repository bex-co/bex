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

// Package signins records a member's sign-ins and sign-outs in the workspace
// audit log (w9/m452): Render's LoginEvent and LogoutEvent, once in each
// workspace the member belongs to.
//
// Signing in and out happens in Kratos, which calls nothing on bex-api. A
// sign-in is therefore the first time bex-api sees a new Kratos session
// (ObserveSession, from the auth gate), which covers every way a session is
// minted: password, social sign-in, a second factor and account recovery
// alike. A sign-out is the dashboard's report just before it ends the Kratos
// session (the recordSignOut mutation) and the CLI's logout (RecordSignOut).
// A session that simply expires records none: Kratos has no hook to say so.
package signins

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
)

// SignInWindow is how recently a session must have authenticated for its
// first sight to count as a sign-in. The dashboard calls bex-api as soon as
// a sign-in lands, so a session first seen any later was signed in before
// this replica, or this code, was running (a long-lived session after a
// rollout): recording it would date a sign-in nobody just made.
const SignInWindow = time.Hour

// clockSkew tolerates a Kratos clock slightly ahead of bex-api's.
const clockSkew = time.Minute

// recordTimeout bounds one sign-in's workspace read; the write itself is
// bounded by core.RecordAuditBatch.
const recordTimeout = 5 * time.Second

// WorkspaceLister lists the workspaces a subject belongs to.
// *store.PGStore is the production implementation.
type WorkspaceLister interface {
	SubjectWorkspaceIDs(ctx context.Context, subject string) ([]string, error)
}

// Session is what a sign-in row needs of a Kratos session: its id (the dedupe
// key), when it authenticated (the row's time) and where from.
type Session struct {
	ID              string
	AuthenticatedAt time.Time
	SourceIP        string
	UserAgent       string
}

// Recorder writes the sign-in and sign-out rows. Both are fail-open: a
// sign-in or a sign-out never waits on, or fails because of, its audit row.
type Recorder struct {
	Workspaces WorkspaceLister
	Audit      core.AuditBatchSink
	// Now is the clock of the sign-in window and the sign-out rows; nil means
	// time.Now.
	Now func() time.Time

	// seen holds the sessions this replica has recorded, so later requests on
	// one cost a map lookup. Its Load also collapses one session's concurrent
	// first requests into one write. Another replica seeing the same session
	// writes the same derived row ids, which the store keeps once.
	once sync.Once
	seen *core.TTLCache[struct{}]
	// pending counts sign-ins still being written (tests wait on it).
	pending sync.WaitGroup
}

func (r *Recorder) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Recorder) cache() *core.TTLCache[struct{}] {
	r.once.Do(func() {
		r.seen = core.NewTTLCache[struct{}]()
		r.seen.Now = r.now
	})
	return r.seen
}

// ObserveSession records subject's sign-in when s is a session this replica
// has not recorded and it authenticated within SignInWindow. It returns at
// once: the row is written in the background, and a write that fails is
// logged and tried again on the session's next request.
func (r *Recorder) ObserveSession(ctx context.Context, subject string, s Session) {
	if subject == "" || s.ID == "" || s.AuthenticatedAt.IsZero() {
		return
	}
	now := r.now()
	if now.Sub(s.AuthenticatedAt) > SignInWindow || s.AuthenticatedAt.Sub(now) > clockSkew {
		return
	}
	seen := r.cache()
	if _, ok := seen.Get(s.ID); ok {
		return
	}
	ttl := s.AuthenticatedAt.Add(SignInWindow + clockSkew).Sub(now)
	ctx = context.WithoutCancel(ctx)
	r.pending.Add(1)
	go func() {
		defer r.pending.Done()
		_, err := seen.Load(ctx, s.ID, ttl, func(ctx context.Context) (struct{}, error) {
			// A request that missed the cache just before an earlier flight
			// stored the session starts a flight of its own after it ended.
			if _, ok := seen.Get(s.ID); ok {
				return struct{}{}, nil
			}
			return struct{}{}, r.recordSignIn(ctx, subject, s)
		})
		if err != nil {
			log.Printf("signins: record the sign-in of %s: %v", subject, err)
		}
	}()
}

// recordSignIn writes subject's LoginEvent in each of their workspaces. Each
// row's id derives from the session and the workspace, so writing it again
// (a retry, or another replica) records nothing more.
func (r *Recorder) recordSignIn(ctx context.Context, subject string, s Session) error {
	ctx, cancel := context.WithTimeout(ctx, recordTimeout)
	defer cancel()
	workspaces, err := r.Workspaces.SubjectWorkspaceIDs(ctx, subject)
	if err != nil {
		return err
	}
	at := s.AuthenticatedAt.UTC()
	evs := make([]core.AuditEvent, 0, len(workspaces))
	for _, workspace := range workspaces {
		ev := event(core.AuditVerbLogin, subject, core.MethodSession, workspace, at)
		ev.ID = id.Derive(id.Audit, core.AuditVerbLogin, subject, s.ID, workspace)
		ev.SourceIP, ev.UserAgent = s.SourceIP, core.AuditUserAgent(s.UserAgent)
		evs = append(evs, ev)
	}
	return core.RecordAuditBatch(ctx, r.Audit, evs...)
}

type sessionKey struct{}

// WithSessionID records on ctx the Kratos session the request authenticated
// with. The auth gate sets it; a dashboard sign-out reads it.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionKey{}, sessionID)
}

func sessionIDFrom(ctx context.Context) string {
	sessionID, _ := ctx.Value(sessionKey{}).(string)
	return sessionID
}

// RecordSignOut writes subject's LogoutEvent in each of their workspaces, by
// the credential method that reported it, from the request in ctx. A sign-out
// of a Kratos session (WithSessionID) derives its row ids from the session, so
// a repeated report, a retried sign-out, records it once.
func (r *Recorder) RecordSignOut(ctx context.Context, subject, method string) error {
	if subject == "" {
		return errors.New("signins: no subject to sign out")
	}
	workspaces, err := r.Workspaces.SubjectWorkspaceIDs(ctx, subject)
	if err != nil {
		return err
	}
	at := r.now().UTC()
	evs := make([]core.AuditEvent, 0, len(workspaces))
	sessionID := sessionIDFrom(ctx)
	for _, workspace := range workspaces {
		ev := event(core.AuditVerbLogout, subject, method, workspace, at)
		if sessionID != "" {
			ev.ID = id.Derive(id.Audit, core.AuditVerbLogout, subject, sessionID, workspace)
		}
		evs = append(evs, ev)
	}
	return core.RecordAuditBatch(ctx, r.Audit, evs...)
}

func event(verb, subject, method, workspace string, at time.Time) core.AuditEvent {
	return core.AuditEvent{
		Caller:       subject,
		CallerMethod: method,
		Verb:         verb,
		Resource:     core.WorkspaceObject(workspace),
		Outcome:      core.AuditAllowed,
		At:           at,
	}
}
