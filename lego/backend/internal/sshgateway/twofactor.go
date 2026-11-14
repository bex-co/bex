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

package sshgateway

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Workspace-enforced two-factor authentication at the gateway
// (docs/ADR012-auth.md §9a). Native public-key SSH never passes bex-api's
// middleware and the gateway's tickets are redeemed and revalidated here, so
// the gateway's own core.Base carries a core.TwoFactorGate. The gateway has no
// Kratos client and its Postgres role cannot read the workspace setting, so the
// gate asks bex-api's HMAC verb (agentsession.InternalTwoFactorPath), which runs
// the real twofactor.Gate, and only caches the answer. Hanging the gate on
// core.Base means native SSH authentication and reauthorization, the web
// shell, agent attach and sandbox exec/files are all covered by the one funnel
// hook, with no per-transport check to forget.

// TwoFactorCacheTTL bounds how stale a cached answer may be. It is the
// gateway's share of the bound on a live session: a member who stops being
// compliant is cut off within bex-api's own 60 s cache (twofactor.TTL), plus
// this, plus one revalidation interval (BEX_SSH_REVALIDATE_INTERVAL, 1 m).
const TwoFactorCacheTTL = 30 * time.Second

// TwoFactorAsker asks bex-api whether the Kratos subject, authenticated by the
// gateway as method, may act in a workspace. required=false means the workspace
// has no requirement and the answer holds for every member. An error means the
// answer could not be had. *agentsession.TwoFactorClient satisfies it.
type TwoFactorAsker interface {
	Ask(ctx context.Context, workspaceID, subject, method string) (required, allowed bool, err error)
}

// TwoFactorSessions names the workspace an agent session belongs to ("" when
// there is no such session), for the workspace an agent_session:<id> object
// belongs to. One column of the table the gateway already reads for the ags-…
// resolver, so the check adds nothing to its database surface. *store.PGStore
// satisfies it.
type TwoFactorSessions interface {
	AgentSessionWorkspace(ctx context.Context, id string) (string, error)
}

// TwoFactorGate is the gateway's core.TwoFactorGate and
// core.TwoFactorObjectResolver. Safe for concurrent use; the zero value with an
// Asker is ready. Nothing it cannot establish is an allow: a transport or
// upstream failure is core.ErrAuthzUnavailable, like an unreachable OpenFGA,
// and is never cached.
type TwoFactorGate struct {
	Asker    TwoFactorAsker
	Sessions TwoFactorSessions
	// Metrics counts refusals; nil => not counted.
	Metrics *Metrics
	// Now supplies the clock for the caches; nil => time.Now. Injectable so a
	// test can cross the TTL without sleeping.
	Now func() time.Time

	once sync.Once
	// off holds workspaces that require nothing: one answer for every member
	// and method. member holds a member's verdict (true = refused) on a
	// workspace that does require 2FA, keyed by workspace and subject. session
	// holds a session's workspace.
	off     core.TTLCache[struct{}]
	member  core.TTLCache[bool]
	session core.TTLCache[string]
	// Concurrent misses for one key share one question or lookup.
	askFlight, sessionFlight singleflight.Group
}

var (
	_ core.TwoFactorGate           = (*TwoFactorGate)(nil)
	_ core.TwoFactorObjectResolver = (*TwoFactorGate)(nil)
)

// twoFactorAnswer is bex-api's reply to one question.
type twoFactorAnswer struct{ required, allowed bool }

// Check implements core.TwoFactorGate. A setting-off workspace is answered once
// per TTL for every member and method, so it costs the hot path no round trip
// beyond that one.
func (g *TwoFactorGate) Check(ctx context.Context, workspaceID string, id core.Identity) error {
	g.init()
	if _, ok := g.off.Get(workspaceID); ok {
		return nil
	}
	memberKey := workspaceID + "\x00" + id.Subject
	if refused, ok := g.member.Get(memberKey); ok {
		return g.verdict(workspaceID, refused)
	}
	if g.Asker == nil {
		return g.unavailable(errors.New("no way to ask bex-api"))
	}
	answer, err := core.SingleFlight(ctx, &g.askFlight, memberKey, func(ctx context.Context) (twoFactorAnswer, error) {
		required, allowed, err := g.Asker.Ask(ctx, workspaceID, id.Subject, id.Method)
		if err != nil {
			return twoFactorAnswer{}, err
		}
		expires := g.now().Add(TwoFactorCacheTTL)
		if required {
			g.member.Put(memberKey, !allowed, expires)
		} else {
			g.off.Put(workspaceID, struct{}{}, expires)
		}
		return twoFactorAnswer{required: required, allowed: allowed}, nil
	})
	if err != nil {
		return g.unavailable(err)
	}
	return g.verdict(workspaceID, answer.required && !answer.allowed)
}

// ObjectWorkspace implements core.TwoFactorObjectResolver: an agent session's
// object names no workspace, so it comes from the session row. Any other object
// kind is not resolved here. A session that does not exist resolves to no
// workspace, as the bex-api gate does; any other failure fails the check closed.
func (g *TwoFactorGate) ObjectWorkspace(ctx context.Context, object string) (string, error) {
	sessionID, ok := strings.CutPrefix(object, core.AgentSessionObjectPrefix)
	if !ok {
		return "", nil
	}
	g.init()
	if ws, ok := g.session.Get(sessionID); ok {
		return ws, nil
	}
	if g.Sessions == nil {
		return "", errors.New("no session reader to resolve an agent session's workspace")
	}
	return core.SingleFlight(ctx, &g.sessionFlight, sessionID, func(ctx context.Context) (string, error) {
		ws, err := g.Sessions.AgentSessionWorkspace(ctx, sessionID)
		if err == nil && ws != "" { // a session that does not exist yet is not remembered
			g.session.Put(sessionID, ws, g.now().Add(TwoFactorCacheTTL))
		}
		return ws, err
	})
}

func (g *TwoFactorGate) verdict(workspaceID string, refused bool) error {
	if !refused {
		return nil
	}
	g.Metrics.TwoFactorRefusal(twoFactorNotEnrolled)
	return core.NewWorkspaceTwoFactorRequiredError(workspaceID)
}

func (g *TwoFactorGate) unavailable(err error) error {
	log.Printf("ssh gateway: two-factor check unavailable: %v", err)
	g.Metrics.TwoFactorRefusal(twoFactorUnavailable)
	return core.ErrAuthzUnavailable
}

// init points the caches at Now once, so a gate built as a plain struct literal
// is usable.
func (g *TwoFactorGate) init() {
	g.once.Do(func() { g.off.Now, g.member.Now, g.session.Now = g.Now, g.Now, g.Now })
}

func (g *TwoFactorGate) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now()
}
