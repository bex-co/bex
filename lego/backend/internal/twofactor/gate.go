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

// Package twofactor is the policy behind a workspace's "require two-factor
// authentication" setting (docs/ADR012-auth.md §9a): who is checked,
// what counts as enrolled, and the caches that keep the check off the hot
// path. core.Base calls it through core.TwoFactorGate after every positive
// authorization decision, so REST, GraphQL and MCP share one enforcement
// point; this package holds no routes of its own.
package twofactor

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/singleflight"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TTL bounds how stale an answer may be: the enrollment fact per identity, the
// setting per workspace and a key's creator are each cached this long. It is
// the documented bound on "access returns after enrolling" and on "blocked
// after enabling, or after removing the last factor" (another replica can lag
// by up to the TTL; a local change invalidates this replica at once).
const TTL = 60 * time.Second

// Settings reads whether a workspace requires 2FA. *store.PGStore satisfies it
// through the thin adapter in cmd/api.
type Settings interface {
	RequireTwoFactor(ctx context.Context, workspaceID string) (bool, error)
}

// Enrollment reports whether a Kratos identity has an enrolled second factor
// (a TOTP credential, or a WebAuthn credential carrying a registered key).
// human=false means the subject is not a Kratos identity — a platform machine
// principal that can never enroll; an error means Kratos could not answer.
// workspaces.IdentityReader's MFAEnrolled has exactly this shape.
type Enrollment interface {
	MFAEnrolled(ctx context.Context, subject string) (enrolled, human bool, err error)
}

// KeyCreators resolves an OAuth client id to the identity that created it.
// isKey=false means the client is not a bex API key at all (an
// operator-provisioned platform client); isKey with an empty creator is a key
// minted before the created-by stamp existed.
type KeyCreators interface {
	KeyCreator(ctx context.Context, clientID string) (creator string, isKey bool, err error)
}

// Sessions names the workspace an agent session belongs to ("" when there is
// no such session). A session's workspace never changes.
type Sessions interface {
	AgentSessionWorkspace(ctx context.Context, sessionID string) (string, error)
}

// Config wires a Gate. Settings is required; a nil Enrollment (no
// BEX_KRATOS_ADMIN_URL) or nil Keys can answer no question, so a workspace
// that requires 2FA fails closed for the principals they would have decided.
type Config struct {
	Settings   Settings
	Enrollment Enrollment
	Keys       KeyCreators
	Sessions   Sessions
	// Now supplies the current time for the caches; injectable for tests.
	// nil => time.Now.
	Now func() time.Time
	// Metrics counts refusals; nil => not counted.
	Metrics *Metrics
}

// Metrics counts what the gate refuses, by a closed set of reasons. It never
// labels by workspace or identity: a tenant string would become permanent
// server memory (the httpmetrics.go rule).
type Metrics struct {
	refusals *prometheus.CounterVec
}

// The refusal reasons, the whole label vocabulary.
const (
	reasonNotEnrolled       = "not_enrolled"
	reasonUnattributedKey   = "unattributed_key"
	reasonLookupUnavailable = "lookup_unavailable"
)

// NewMetrics registers the refusal counter on registerer.
func NewMetrics(registerer prometheus.Registerer) *Metrics {
	m := &Metrics{refusals: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bex", Subsystem: "workspace_two_factor", Name: "refusals_total",
		Help: "Calls the workspace two-factor gate refused, by reason: a member or an API key's creator with no second factor (not_enrolled), an API key with no recorded creator (unattributed_key), or a lookup that could not answer for a workspace that requires 2FA (lookup_unavailable, failed closed).",
	}, []string{"reason"})}
	registerer.MustRegister(m.refusals)
	return m
}

func (m *Metrics) refused(reason string) {
	if m != nil {
		m.refusals.WithLabelValues(reason).Inc()
	}
}

// Gate is the core.TwoFactorGate (and core.TwoFactorObjectResolver) for
// bex-api. Safe for concurrent use. Concurrent misses for the same key share
// one load (core.TTLCache.Load); an error is never cached.
type Gate struct {
	cfg        Config
	now        func() time.Time
	settings   core.TTLCache[bool]
	enrollment core.TTLCache[enrollment]
	keys       core.TTLCache[keyCreator]
	sessions   core.TTLCache[string]
	// sessionFlight collapses concurrent session lookups. Sessions has its own
	// flight, not Load, because a session that does not exist ("") is not cached.
	sessionFlight singleflight.Group
}

type enrollment struct{ enrolled, human bool }

type keyCreator struct {
	creator string
	isKey   bool
}

// New builds a Gate over cfg.
func New(cfg Config) *Gate {
	g := &Gate{cfg: cfg, now: cfg.Now}
	if g.now == nil {
		g.now = time.Now
	}
	g.settings.Now, g.enrollment.Now, g.keys.Now, g.sessions.Now = g.now, g.now, g.now, g.now
	return g
}

// Invalidate drops this replica's cached setting for workspaceID — called the
// moment an admin changes it, so the change takes effect here at once and on
// other replicas within TTL.
func (g *Gate) Invalidate(workspaceID string) { g.settings.Delete(workspaceID) }

// Required reports whether workspaceID requires 2FA, through the same cache
// Check reads. The SSH gateway's verb (agentsession.TwoFactorHandler) uses it
// to answer "this workspace never asked" once for every member. An unreadable
// setting is core.ErrAuthzUnavailable, never "not required".
func (g *Gate) Required(ctx context.Context, workspaceID string) (bool, error) {
	if workspaceID == "" || workspaceID == core.DefaultTenant {
		return false, nil // the default workspace is the legacy/bootstrap tenant: it has no setting
	}
	required, err := g.required(ctx, workspaceID)
	if err != nil {
		log.Printf("twofactor: reading workspace setting: %v", err)
		g.cfg.Metrics.refused(reasonLookupUnavailable)
		return false, core.ErrAuthzUnavailable
	}
	return required, nil
}

// Check implements core.TwoFactorGate. A false answer from Required implies
// allowed, so a workspace that never asked costs no principal or enrollment
// lookup.
func (g *Gate) Check(ctx context.Context, workspaceID string, id core.Identity) error {
	required, err := g.Required(ctx, workspaceID)
	if err != nil || !required {
		return err
	}
	subject, viaKey, err := g.principal(ctx, id)
	if err != nil {
		log.Printf("twofactor: resolving principal: %v", err)
		g.cfg.Metrics.refused(reasonLookupUnavailable)
		return core.ErrAuthzUnavailable
	}
	if subject == "" {
		if viaKey {
			return g.unattributed(workspaceID)
		}
		return nil // nobody to check: a platform principal
	}
	e, err := g.enrollmentOf(ctx, subject)
	if err != nil {
		log.Printf("twofactor: reading enrollment: %v", err)
		g.cfg.Metrics.refused(reasonLookupUnavailable)
		return core.ErrAuthzUnavailable
	}
	switch {
	case e.enrolled:
		return nil
	case !e.human && viaKey:
		// The key's creator is no longer a Kratos identity (a deleted account),
		// so no second factor can ever be shown for it. Exempting it would let an
		// orphaned key bypass the setting; refuse it like a key with no creator.
		return g.unattributed(workspaceID)
	case !e.human:
		// Kratos does not know this subject: a platform principal that has nothing
		// to enroll, and blocking it would lock out the platform itself.
		return nil
	}
	g.cfg.Metrics.refused(reasonNotEnrolled)
	return core.NewWorkspaceTwoFactorRequiredError(workspaceID)
}

// unattributed refuses an API key whose creator's second factor cannot be
// checked. It would otherwise be a hole around the setting.
func (g *Gate) unattributed(workspaceID string) error {
	g.cfg.Metrics.refused(reasonUnattributedKey)
	return core.NewForbiddenError(core.CodeWorkspaceTwoFactorRequired,
		"this workspace requires two-factor authentication and this API key has no recorded creator whose second factor could be checked; create a new API key",
		map[string]any{"workspaceId": workspaceID})
}

// ObjectWorkspace implements core.TwoFactorObjectResolver: an agent session's
// object names no workspace, so it is looked up. Every other object kind is
// not resolved here ("" and no error). With no session reader wired the
// workspace cannot be named, and "" would skip the check, so it fails closed.
func (g *Gate) ObjectWorkspace(ctx context.Context, object string) (string, error) {
	sessionID, ok := strings.CutPrefix(object, core.AgentSessionObjectPrefix)
	if !ok {
		return "", nil
	}
	if ws, ok := g.sessions.Get(sessionID); ok {
		return ws, nil
	}
	return core.SingleFlight(ctx, &g.sessionFlight, sessionID, func(ctx context.Context) (string, error) {
		if g.cfg.Sessions == nil {
			return "", errNoLookup
		}
		ws, err := g.cfg.Sessions.AgentSessionWorkspace(ctx, sessionID)
		if err == nil && ws != "" { // a session that does not exist yet is not remembered
			g.sessions.Put(sessionID, ws, g.now().Add(TTL))
		}
		return ws, err
	})
}

func (g *Gate) required(ctx context.Context, workspaceID string) (bool, error) {
	if g.cfg.Settings == nil {
		return false, nil
	}
	return g.settings.Load(ctx, workspaceID, TTL, func(ctx context.Context) (bool, error) {
		return g.cfg.Settings.RequireTwoFactor(ctx, workspaceID)
	})
}

// principal is whose second factor decides for id. It keys on how the
// credential is held, never on Identity.Human or Method "session": background
// work (agentsessions/spawn.go, automation_scheduler.go) builds Method
// "session" identities and runs as the member who started it, the subject
// this check is meant to see.
//
//   - A machine API key (id.IsAPIKey) borrows its creator's: the creator, read
//     from the Hydra client's created-by stamp, and viaKey is true. A client
//     that is not a bex API key is a platform client and is exempt (subject
//     "", viaKey false). A bex key with no creator, or a token naming no client
//     at all, is unattributable, which would be a hole around the setting, so
//     it comes back as subject "" with viaKey true for refusal.
//   - Everything else — a Kratos session, a human OAuth token, a gateway or
//     ticket identity — is its own Subject.
func (g *Gate) principal(ctx context.Context, id core.Identity) (subject string, viaKey bool, err error) {
	if !id.IsAPIKey() {
		return id.Subject, false, nil
	}
	clientID := id.KeyClientID()
	if clientID == "" {
		return "", true, nil
	}
	k, err := g.keyCreatorOf(ctx, clientID)
	switch {
	case err != nil:
		return "", false, err
	case !k.isKey:
		return "", false, nil
	}
	return k.creator, true, nil
}

func (g *Gate) keyCreatorOf(ctx context.Context, clientID string) (keyCreator, error) {
	return g.keys.Load(ctx, clientID, TTL, func(ctx context.Context) (keyCreator, error) {
		if g.cfg.Keys == nil {
			return keyCreator{}, errNoLookup
		}
		creator, isKey, err := g.cfg.Keys.KeyCreator(ctx, clientID)
		return keyCreator{creator: creator, isKey: isKey}, err
	})
}

func (g *Gate) enrollmentOf(ctx context.Context, subject string) (enrollment, error) {
	return g.enrollment.Load(ctx, subject, TTL, func(ctx context.Context) (enrollment, error) {
		if g.cfg.Enrollment == nil {
			return enrollment{}, errNoLookup
		}
		enrolled, human, err := g.cfg.Enrollment.MFAEnrolled(ctx, subject)
		return enrollment{enrolled: enrolled, human: human}, err
	})
}

var errNoLookup = core.Err("no identity lookup is wired")
