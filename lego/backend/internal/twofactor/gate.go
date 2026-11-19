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

// Package twofactor is the policy behind a workspace's login requirements:
// "require two-factor authentication" (docs/ADR012-auth.md §9a) and "require
// Google login" (§9b, w7/m465), Render's "Enforce 2FA & Google login". It
// decides who is checked, what counts as enrolled or as signed in with Google,
// and holds the caches that keep the check off the hot path. core.Base calls
// it through core.TwoFactorGate after every positive authorization decision,
// so REST, GraphQL and MCP share one enforcement point; this package holds no
// routes of its own.
package twofactor

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/singleflight"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TTL bounds how stale an answer may be: the enrollment fact and the Google
// account per identity, the settings per workspace and a key's creator are
// each cached this long. It is the documented bound on "access returns after
// enrolling" and on "blocked after enabling, or after removing the last
// factor" (another replica can lag by up to the TTL; a local change
// invalidates this replica at once).
const TTL = 60 * time.Second

// Settings reads a workspace's login requirements (its own ORed with its
// org's) and an org's own (w7/m466). *store.PGStore satisfies it.
type Settings interface {
	LoginRequirements(ctx context.Context, workspaceID string) (core.LoginRequirements, error)
	OrganizationLoginRequirements(ctx context.Context, orgID string) (core.LoginRequirements, error)
}

// Enrollment reports whether a Kratos identity has an enrolled second factor
// (a TOTP credential, or a WebAuthn credential carrying a registered key).
// human=false means the subject is not a Kratos identity — a platform machine
// principal that can never enroll; an error means Kratos could not answer.
// workspaces.IdentityReader's MFAEnrolled has exactly this shape.
type Enrollment interface {
	MFAEnrolled(ctx context.Context, subject string) (enrolled, human bool, err error)
}

// GoogleAccounts reports whether a Kratos identity has a linked Google account
// whose email is the identity's own email trait (Render: "Team members must
// sign in with the Google account that matches the invited email address").
// human=false means the subject is not a Kratos identity; an error means
// Kratos could not answer. workspaces.KratosIdentities satisfies it.
type GoogleAccounts interface {
	GoogleAccountMatches(ctx context.Context, subject string) (matches, human bool, err error)
}

// KeyCreators resolves an OAuth client id to the identity that created it and
// how that identity was signed in when it did. isKey=false means the client is
// not a bex API key at all (an operator-provisioned platform client); isKey
// with an empty creator is a key minted before the created-by stamp existed,
// and an unrecorded signIn one minted before the sign-in stamp (w7/m465).
type KeyCreators interface {
	KeyCreator(ctx context.Context, clientID string) (creator string, signIn core.SignIn, isKey bool, err error)
}

// Sessions names the workspace an agent session belongs to ("" when there is
// no such session). A session's workspace never changes.
type Sessions interface {
	AgentSessionWorkspace(ctx context.Context, sessionID string) (string, error)
}

// Config wires a Gate. Settings is required; a nil Enrollment or Google (no
// BEX_KRATOS_ADMIN_URL) or nil Keys can answer no question, so a workspace
// that requires 2FA or Google login fails closed for the principals they would
// have decided.
type Config struct {
	Settings   Settings
	Enrollment Enrollment
	Google     GoogleAccounts
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
	reasonNotGoogle         = "not_google"
	reasonKeyNotGoogle      = "key_not_google"
)

// NewMetrics registers the refusal counter on registerer.
func NewMetrics(registerer prometheus.Registerer) *Metrics {
	m := &Metrics{refusals: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bex", Subsystem: "workspace_two_factor", Name: "refusals_total",
		Help: "Calls the workspace login-requirements gate refused, by reason: a member or an API key's creator with no second factor (not_enrolled), an API key with no recorded creator (unattributed_key), a member not signed in with the Google account of their email where Google login is required (not_google), an API key or SSH key not created in such a sign-in (key_not_google), or a lookup that could not answer for a workspace that requires either (lookup_unavailable, failed closed).",
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
	settings   core.TTLCache[core.LoginRequirements]
	enrollment core.TTLCache[enrollment]
	google     core.TTLCache[googleAccount]
	keys       core.TTLCache[keyCreator]
	sessions   core.TTLCache[string]
	// sessionFlight collapses concurrent session lookups. Sessions has its own
	// flight, not Load, because a session that does not exist ("") is not cached.
	sessionFlight singleflight.Group
}

type enrollment struct{ enrolled, human bool }

type googleAccount struct{ matches, human bool }

type keyCreator struct {
	creator string
	signIn  core.SignIn
	isKey   bool
}

// New builds a Gate over cfg.
func New(cfg Config) *Gate {
	g := &Gate{cfg: cfg, now: cfg.Now}
	if g.now == nil {
		g.now = time.Now
	}
	g.settings.Now, g.enrollment.Now, g.google.Now, g.keys.Now, g.sessions.Now = g.now, g.now, g.now, g.now, g.now
	return g
}

// Invalidate drops this replica's cached setting under key — a workspace id,
// or core.OrganizationObject(orgID) for an org's own setting — called the
// moment an admin or Owner changes it, so the change takes effect here at once
// and on other replicas within TTL. An org change invalidates the org and each
// of its workspaces.
func (g *Gate) Invalidate(key string) { g.settings.Delete(key) }

// Required reports workspaceID's login requirements, through the same cache
// Check reads. The SSH gateway's verb (agentsession.TwoFactorHandler) uses it
// to answer "this workspace requires nothing" once for every member. An
// unreadable setting is core.ErrAuthzUnavailable, never "not required".
func (g *Gate) Required(ctx context.Context, workspaceID string) (core.LoginRequirements, error) {
	if workspaceID == "" || workspaceID == core.DefaultTenant {
		return core.LoginRequirements{}, nil // the default workspace is the legacy/bootstrap tenant: it has no setting
	}
	return g.load(ctx, workspaceID, func(ctx context.Context, s Settings) (core.LoginRequirements, error) {
		return s.LoginRequirements(ctx, workspaceID)
	})
}

// load reads the requirements cached under key (a workspace id or an org's
// object) through read. No Settings wired requires nothing; an unreadable
// setting is core.ErrAuthzUnavailable, never "not required", and not cached.
func (g *Gate) load(ctx context.Context, key string, read func(context.Context, Settings) (core.LoginRequirements, error)) (core.LoginRequirements, error) {
	if g.cfg.Settings == nil {
		return core.LoginRequirements{}, nil
	}
	required, err := g.settings.Load(ctx, key, TTL, func(ctx context.Context) (core.LoginRequirements, error) {
		return read(ctx, g.cfg.Settings)
	})
	if err != nil {
		log.Printf("twofactor: reading the login setting of %s: %v", key, err)
		g.cfg.Metrics.refused(reasonLookupUnavailable)
		return core.LoginRequirements{}, core.ErrAuthzUnavailable
	}
	return required, nil
}

// Check implements core.TwoFactorGate. A workspace that requires nothing costs
// no principal, enrollment or account lookup. A principal that fails both
// requirements gets the Google-login refusal: signing in again is the step
// that comes first (docs/ADR012-auth.md §9b).
func (g *Gate) Check(ctx context.Context, workspaceID string, id core.Identity) error {
	required, err := g.Required(ctx, workspaceID)
	if err != nil {
		return err
	}
	return g.check(ctx, scope{workspaceID: workspaceID, required: required, metrics: g.cfg.Metrics}, id)
}

// CheckOrganization implements core.OrganizationTwoFactorGate: an org's own
// objects are held to the org's own requirements (w7/m466, §9c), with Check's
// rules and refusals, which name the org.
func (g *Gate) CheckOrganization(ctx context.Context, orgID string, id core.Identity) error {
	if orgID == "" {
		return nil
	}
	required, err := g.load(ctx, core.OrganizationObject(orgID), func(ctx context.Context, s Settings) (core.LoginRequirements, error) {
		return s.OrganizationLoginRequirements(ctx, orgID)
	})
	if err != nil {
		return err
	}
	return g.check(ctx, scope{required: required, metrics: g.cfg.Metrics}, id)
}

// scope is what a check enforces: the workspace's id ("" on an org's own
// objects) and what it requires, and where its refusals are counted. Only a
// request's check counts them; an assessment or the lock-out guard leaves
// metrics nil, because nobody was refused.
type scope struct {
	workspaceID string
	required    core.LoginRequirements
	metrics     *Metrics
}

func (g *Gate) check(ctx context.Context, sc scope, id core.Identity) error {
	if !sc.required.Any() {
		return nil
	}
	p, err := g.principal(ctx, id)
	if err != nil {
		return g.unavailable(sc, "resolving principal", err)
	}
	return g.decide(ctx, sc, id, p)
}

// decide is the rules of both requirements for a principal already resolved:
// the Google-login refusal first, then 2FA.
func (g *Gate) decide(ctx context.Context, sc scope, id core.Identity, p principal) error {
	if sc.required.GoogleLogin {
		if err := g.checkGoogle(ctx, sc, id, p); err != nil {
			return err
		}
	}
	if sc.required.TwoFactor {
		return g.checkTwoFactor(ctx, sc, p)
	}
	return nil
}

// SignedInWithGoogle reports whether id would meet a Google-login requirement
// right now: the lock-out guard of the setting's own mutation, so an admin
// cannot turn on a requirement that refuses them. It is the same check Check
// runs, and like an assessment it counts no refusal. An error means the answer
// could not be established.
func (g *Gate) SignedInWithGoogle(ctx context.Context, id core.Identity) (bool, error) {
	p, err := g.principal(ctx, id)
	if err != nil {
		return false, err
	}
	return meets(g.checkGoogle(ctx, scope{}, id, p))
}

// AssessMember reports whether subject, a member who is not making a request,
// would meet required: Check's rules for a principal with no sign-in of its
// own (background work acting as the member), so their enrolled factor for
// 2FA and, for Google login, a linked Google account matching their email. A
// subject Kratos does not know is a platform principal and meets it. How the
// member is signed in right now is not read: one signed in another way is
// asked to sign in with Google at their next request. It counts no refusal
// (docs/ADR012-auth.md §9c, "The enable confirmation", w7/m471). An error
// means a lookup could not answer.
func (g *Gate) AssessMember(ctx context.Context, required core.LoginRequirements, subject string) (bool, error) {
	return g.assess(ctx, required, core.Identity{Subject: subject, Method: core.MethodSession}, principal{subject: subject})
}

// AssessAPIKey reports whether an API key created by creator in the sign-in
// signIn would meet required, by Check's rules for a key: it borrows its
// creator's factor, a key with no creator or one Kratos no longer knows is
// refused, and for Google login an unstamped key is exempt while any other
// stamp needs Google and a matching account. It counts no refusal.
func (g *Gate) AssessAPIKey(ctx context.Context, required core.LoginRequirements, creator string, signIn core.SignIn) (bool, error) {
	return g.assess(ctx, required, core.Identity{}, principal{subject: creator, viaKey: true, keySignIn: signIn})
}

// AssessSSHKey reports whether subject's native SSH key, stamped signIn when it
// was added, would meet required, by Check's rules for the gateway's identity:
// the owner's own factor for 2FA, the key's stamp and the owner's account for
// Google login. It counts no refusal.
func (g *Gate) AssessSSHKey(ctx context.Context, required core.LoginRequirements, subject string, signIn core.SignIn) (bool, error) {
	id := core.Identity{Subject: subject, Method: core.MethodSSH, SignIn: signIn}
	return g.assess(ctx, required, id, principal{subject: subject})
}

func (g *Gate) assess(ctx context.Context, required core.LoginRequirements, id core.Identity, p principal) (bool, error) {
	return meets(g.decide(ctx, scope{required: required}, id, p))
}

// meets turns a decision into an answer: a refusal is false, an outage an
// error.
func meets(err error) (bool, error) {
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, core.ErrForbidden):
		return false, nil
	}
	return false, err
}

// checkTwoFactor is the 2FA requirement (§9a): the principal's subject has an
// enrolled second factor.
func (g *Gate) checkTwoFactor(ctx context.Context, sc scope, p principal) error {
	if p.subject == "" {
		if p.viaKey {
			return g.unattributed(sc)
		}
		return nil // nobody to check: a platform principal
	}
	e, err := g.enrollmentOf(ctx, p.subject)
	if err != nil {
		return g.unavailable(sc, "reading enrollment", err)
	}
	switch {
	case e.enrolled:
		return nil
	case !e.human && p.viaKey:
		// The key's creator is no longer a Kratos identity (a deleted account),
		// so no second factor can ever be shown for it. Exempting it would let an
		// orphaned key bypass the setting; refuse it like a key with no creator.
		return g.unattributed(sc)
	case !e.human:
		// Kratos does not know this subject: a platform principal that has nothing
		// to enroll, and blocking it would lock out the platform itself.
		return nil
	}
	sc.metrics.refused(reasonNotEnrolled)
	return core.NewLoginTwoFactorRequiredError(sc.workspaceID, sc.required)
}

// checkGoogle is the Google-login requirement (§9b). What decides depends on
// how the credential is held:
//
//   - A key — an API key (its Hydra client's sign-in stamp) or a native SSH
//     key (the stamp the gateway read off ssh_keys) — counts when it was
//     created in a Google sign-in and its creator's Google account still
//     matches their email. A key with no stamp predates the setting and is
//     exempt, as Render exempts API keys created before it shipped.
//   - An interactive sign-in — a Kratos session, or a human OAuth token
//     carrying the browser session it was consented from — counts when that
//     sign-in was Google and the Google account matches the email. A human
//     token with no recorded sign-in (minted before the stamp) does not count:
//     signing in again is cheap.
//   - Anything else — background work acting as a member, a gateway ticket —
//     has no sign-in of its own. Each ticket was minted by a call that passed
//     this check, so what is left to ask is the member's account: a linked
//     Google account that matches their email. A subject Kratos does not know
//     is a platform principal and is exempt.
func (g *Gate) checkGoogle(ctx context.Context, sc scope, id core.Identity, p principal) error {
	refuse, platformExempt := g.refuseMember, false
	switch {
	case p.viaKey || id.Method == core.MethodSSH || id.Method == core.MethodS3Key:
		stamp := p.keySignIn
		if !p.viaKey {
			stamp = id.SignIn
		}
		if stamp == core.SignInUnrecorded {
			return nil
		}
		refuse = g.refuseKey
		if stamp != core.SignInGoogle || p.subject == "" {
			return refuse(sc)
		}
	case id.Method == core.MethodSession && id.SignIn != core.SignInUnrecorded,
		id.Method == core.MethodOAuth2 && id.Human:
		if id.SignIn != core.SignInGoogle {
			return refuse(sc)
		}
	default:
		if p.subject == "" {
			return nil // a platform principal: nobody to check
		}
		platformExempt = true
	}
	a, err := g.googleAccountOf(ctx, p.subject)
	switch {
	case err != nil:
		return g.unavailable(sc, "reading the Google account", err)
	case !a.human && platformExempt:
		return nil // Kratos does not know this subject: a platform principal
	case !a.human || !a.matches:
		return refuse(sc)
	}
	return nil
}

func (g *Gate) refuseMember(sc scope) error {
	sc.metrics.refused(reasonNotGoogle)
	return core.NewLoginGoogleLoginRequiredError(sc.workspaceID, sc.required, false)
}

func (g *Gate) refuseKey(sc scope) error {
	sc.metrics.refused(reasonKeyNotGoogle)
	return core.NewLoginGoogleLoginRequiredError(sc.workspaceID, sc.required, true)
}

// unavailable fails closed: an answer that cannot be had is never an allow.
func (g *Gate) unavailable(sc scope, what string, err error) error {
	log.Printf("twofactor: %s: %v", what, err)
	sc.metrics.refused(reasonLookupUnavailable)
	return core.ErrAuthzUnavailable
}

// unattributed refuses an API key whose creator's second factor cannot be
// checked. It would otherwise be a hole around the setting.
func (g *Gate) unattributed(sc scope) error {
	sc.metrics.refused(reasonUnattributedKey)
	return core.NewLoginUnattributedKeyError(sc.workspaceID, sc.required)
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

// principal is whose account decides for id. It keys on how the credential
// is held, never on Identity.Human or Method "session": background work
// (agentsessions/spawn.go, automation_scheduler.go) builds Method "session"
// identities and runs as the member who started it, the subject this check is
// meant to see.
//
//   - A machine API key (id.IsAPIKey) borrows its creator's: the creator, read
//     from the Hydra client's created-by stamp, with the sign-in stamp beside
//     it, and viaKey true. A client that is not a bex API key is a platform
//     client and is exempt (subject "", viaKey false). A bex key with no
//     creator, or a token naming no client at all, is unattributable, which
//     would be a hole around the 2FA setting, so it comes back as subject ""
//     with viaKey true for refusal.
//   - Everything else — a Kratos session, a human OAuth token, a gateway or
//     ticket identity — is its own Subject.
type principal struct {
	subject   string
	viaKey    bool
	keySignIn core.SignIn
}

func (g *Gate) principal(ctx context.Context, id core.Identity) (principal, error) {
	if !id.IsAPIKey() {
		return principal{subject: id.Subject}, nil
	}
	clientID := id.KeyClientID()
	if clientID == "" {
		return principal{viaKey: true}, nil
	}
	k, err := g.keyCreatorOf(ctx, clientID)
	switch {
	case err != nil:
		return principal{}, err
	case !k.isKey:
		return principal{}, nil
	}
	return principal{subject: k.creator, viaKey: true, keySignIn: k.signIn}, nil
}

// APIKeyCreator implements core.KeyCreatorResolver over the same cached lookup
// the login requirements read, so the request-time key ceiling (w9/m481) costs
// no Hydra call beyond the first per key and TTL. A key's creator stamp never
// changes, so the cache cannot go stale; a revoked key loses its own tuple.
func (g *Gate) APIKeyCreator(ctx context.Context, clientID string) (creator string, isKey bool, err error) {
	k, err := g.keyCreatorOf(ctx, clientID)
	return k.creator, k.isKey, err
}

var _ core.KeyCreatorResolver = (*Gate)(nil)

func (g *Gate) keyCreatorOf(ctx context.Context, clientID string) (keyCreator, error) {
	return g.keys.Load(ctx, clientID, TTL, func(ctx context.Context) (keyCreator, error) {
		if g.cfg.Keys == nil {
			return keyCreator{}, errNoLookup
		}
		creator, signIn, isKey, err := g.cfg.Keys.KeyCreator(ctx, clientID)
		return keyCreator{creator: creator, signIn: signIn, isKey: isKey}, err
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

func (g *Gate) googleAccountOf(ctx context.Context, subject string) (googleAccount, error) {
	return g.google.Load(ctx, subject, TTL, func(ctx context.Context) (googleAccount, error) {
		if g.cfg.Google == nil {
			return googleAccount{}, errNoLookup
		}
		matches, human, err := g.cfg.Google.GoogleAccountMatches(ctx, subject)
		return googleAccount{matches: matches, human: human}, err
	})
}

var errNoLookup = core.Err("no identity lookup is wired")
