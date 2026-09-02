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

// Package id is the one place bex mints and validates typed resource ids
// (docs/ADR020-identifiers.md — the ADR). A bex id is "<prefix>-<20-char xid>", e.g.
// "srv-c185th5c2rvvnhbfiltg": a short greppable type prefix, a hyphen, then an
// xid (k-sortable, non-guessable). The separator is a HYPHEN, never an
// underscore — ids must be safe to drop into DNS labels, hostnames, and
// Kubernetes object names, where "_" is illegal; the id_test guard enforces
// this, so the choice can't silently drift to Stripe's "tea_…" form.
//
// This package is a leaf: it imports only xid, so every feature and the store
// can depend on it without pulling in the control plane. Mint through New and
// validate through WellFormed/KindOf — never hand-concatenate a prefix.
package id

import (
	"crypto/sha256"
	"encoding/base32"
	"regexp"
	"slices"
	"strings"

	"github.com/rs/xid"
)

// Kind is a typed id namespace: the prefix that leads every id of that kind and
// a short description of what it identifies. Its fields are UNEXPORTED on
// purpose — a caller outside this package cannot fabricate a Kind with an
// arbitrary prefix, so the set of id kinds is closed at COMPILE TIME to the
// package-registered vars below. The only Kinds that exist are the ones this
// package declares; New/KindOf hand them out, callers can't invent new ones.
// Prefixes follow Render's public API so bex ids are drop-in for Render clients.
type Kind struct {
	prefix string
	desc   string
}

// Prefix and Desc read a Kind (e.g. for logs/errors). Read-only: there is no
// setter and no exported field, which is what makes the registry closed.
func (k Kind) Prefix() string { return k.prefix }
func (k Kind) Desc() string   { return k.desc }

// The registered kinds — the SINGLE source of truth. A new id-bearing resource
// adds its Kind here (and nowhere else); the guard test then holds it to the
// format + uniqueness + DNS-safety contract automatically.
var (
	Workspace    = Kind{prefix: "tea", desc: "workspace (tenant/team)"}   // Render: teams are tea-
	Service      = Kind{prefix: "srv", desc: "service (app)"}             // Render: services are srv-
	Postgres     = Kind{prefix: "dpg", desc: "managed Postgres database"} // Render: Postgres databases are dpg-
	KeyValue     = Kind{prefix: "red", desc: "managed key-value store"}   // Render: Key Value (Redis-compatible) stores are red-
	Domain       = Kind{prefix: "cdm", desc: "custom domain"}             // Render: custom domains are cdm-
	EnvGroup     = Kind{prefix: "evg", desc: "environment group"}         // Render: env groups are evg-
	Deploy       = Kind{prefix: "dep", desc: "deploy"}                    // Render: deploys are dep-
	Invite       = Kind{prefix: "inv", desc: "workspace member invite"}   // w4/m12 team invites
	Export       = Kind{prefix: "exp", desc: "managed-postgres logical export"}
	Audit        = Kind{prefix: "aud", desc: "audit log event"}                            // w4/m10 audit log
	Owner        = Kind{prefix: "own", desc: "user identity (Render own-)"}                // w6/m7: opaque per-subject user id
	Event        = Kind{prefix: "evt", desc: "service event (derived)"}                    // Render: events are evt-; w3/m7 — minted by Derive, never New
	CronRun      = Kind{prefix: "crr", desc: "cron job run (derived)"}                     // projection of App.status.runs; minted by Derive, never New
	Notification = Kind{prefix: "ntf", desc: "per-member deploy notification preferences"} // w3/m9
	Project      = Kind{prefix: "prj", desc: "project (service grouping)"}                 // w1/m31
	// RegistryCredential is a bex-chosen prefix — Render's own id spelling for
	// this resource isn't confirmed against a live capture (w2/m14).
	RegistryCredential = Kind{prefix: "rgc", desc: "external image registry credential"}
	Blueprint          = Kind{prefix: "blp", desc: "blueprint (render.yaml stack source)"} // w2/m15
	// Environment is a named subset of a Project's services (e.g. staging/
	// production) — the second half of w1/m31's grouping feature, layered on
	// afterward. "env" (not "evg" — that's the pre-existing, unrelated EnvGroup
	// env-var-grouping feature).
	Environment = Kind{prefix: "env", desc: "environment (named subset of a project's services)"}
	// Webhook / WebhookDelivery are bex-chosen prefixes (w3/m11 outbound event
	// webhooks) — Render's public docs don't expose its webhook-endpoint id
	// spelling, so these follow the RegistryCredential precedent.
	Webhook            = Kind{prefix: "whk", desc: "outbound webhook endpoint"}
	WebhookDelivery    = Kind{prefix: "whd", desc: "outbound webhook delivery"}
	WebhookReplayLease = Kind{prefix: "wrl", desc: "git webhook replay epoch lease"}
	// Job is a one-off job run in a service's container (Render's /services/{id}/jobs).
	// Prefix "job" matches Render's observed id prefix from the live API.
	Job        = Kind{prefix: "job", desc: "one-off job"}
	SSHKey     = Kind{prefix: "ssk", desc: "user SSH public key"}
	SSHSession = Kind{prefix: "ssn", desc: "audited SSH session"}
	// BlueprintSync is a recorded sync run (w2/m62 — Git-connected Blueprints).
	// Each manual or auto-triggered sync produces one row in blueprint_syncs.
	BlueprintSync = Kind{prefix: "bsr", desc: "blueprint sync run"}
	// AgentSession is bex-native (Render has no coding-agent session resource).
	// "ags" keeps the public id short, typed, and DNS-safe for sandbox metadata.
	AgentSession = Kind{prefix: "ags", desc: "cloud coding-agent session"}
	// Prefix "dsk" is Render's own spelling for a persistent service disk
	// (^dsk-[0-9a-z]{20}$ in its API), so a Render client's id parsing works
	// unchanged against bex. w1/m84, docs/ADR082-persistent-disks.md.
	Disk = Kind{prefix: "dsk", desc: "persistent service disk"}
	// WorkspaceCreationAttempt is an opaque, short-lived handle for the
	// pre-tenant billing flow. It is never a workspace id and never appears in
	// tenant/resource APIs.
	WorkspaceCreationAttempt = Kind{prefix: "wca", desc: "workspace creation attempt"}
)

// kinds lists every registered Kind; Kinds returns a copy. KindOf, New's
// membership guard, and the guard test enumerate it, so it must include every
// Kind declared above.
var kinds = []Kind{Workspace, Service, Postgres, KeyValue, Domain, EnvGroup, Deploy, Invite, Export, Audit, Owner, Event, CronRun, Notification, Project, RegistryCredential, Blueprint, Environment, Webhook, WebhookDelivery, WebhookReplayLease, Job, SSHKey, SSHSession, BlueprintSync, AgentSession, Disk, WorkspaceCreationAttempt}

// Kinds returns the registered id kinds (a copy — callers must not mutate it).
func Kinds() []Kind { return append([]Kind(nil), kinds...) }

// xidLen is the fixed length of an xid's base32-hex string (chars 0-9a-v).
const xidLen = 20

// wellFormed matches the canonical shape: a 2-4 char lowercase prefix, a hyphen,
// then the 20-char xid. Kept in one regex so WellFormed and the guard test agree.
var wellFormed = regexp.MustCompile(`^[a-z]{2,4}-[0-9a-v]{20}$`)

// New mints a fresh id for a kind: "<prefix>-<xid>". The only mint path — do not
// concatenate a prefix by hand (that bypasses the format the guard test pins).
// It panics on an unregistered Kind (the only one a caller can produce is the
// zero Kind{}, since the fields are unexported) — a programmer error surfaced
// fail-fast rather than as a silently malformed id.
func New(k Kind) string {
	if !slices.Contains(kinds, k) {
		panic("id.New: unregistered kind " + k.prefix + " — use a package-declared Kind (id.Workspace, …)")
	}
	return k.prefix + "-" + xid.New().String()
}

// Derive returns the id of a DERIVED resource: same "<prefix>-<20 chars>" shape
// as New, but a deterministic function of parts rather than a fresh xid. It is
// the mint path for a resource that is a PROJECTION of rows the store already
// holds — service events (internal/events), whose public identity is also
// materialized in an owner-scoped lookup index while their typed data remains
// in deploys, audit_events, and service_event_facts.
//
// Determinism is the whole point: an event's id must be identical on every read
// (a client pages with it, re-fetches it, dedupes on it), so New's fresh xid
// would be exactly wrong. Same parts in, same id out, forever.
//
// parts must uniquely identify the projected event (e.g. the source row id + the
// transition within it) — they are joined with a separator that cannot occur in
// an id, so ("dep-x", "started") and ("dep-xstarted") can never collide. The
// output is 100 bits of SHA-256 in base32hex, which keeps it inside the
// [0-9a-v]{20} alphabet WellFormed pins, so a derived id is indistinguishable in
// shape from a minted one and satisfies Render's ^evt-[0-9a-z]{20}$ pattern.
func Derive(k Kind, parts ...string) string {
	if !slices.Contains(kinds, k) {
		panic("id.Derive: unregistered kind " + k.prefix + " — use a package-declared Kind (id.Event, …)")
	}
	return k.prefix + "-" + deriveComponent(parts...)
}

// DeriveServiceInstance returns Render's compound service-instance id:
// "<service-id>-<stable-suffix>". A service instance is a projection of a
// Kubernetes Pod rather than an independently stored resource, so it extends
// the parent Service id instead of adding another top-level Kind. Callers pass
// the Pod UID as a derivation part; the opaque suffix keeps Kubernetes names
// and UIDs out of the public wire contract while remaining stable across reads.
//
// Legacy hand-applied Apps can still have a name-shaped service id. Preserve
// that parent verbatim for backwards compatibility; store-managed services use
// the canonical srv-<xid> parent and therefore match Render's
// ^srv-[a-z0-9]{20}-[a-z0-9]+$ instance-id shape.
func DeriveServiceInstance(serviceID string, parts ...string) string {
	return serviceID + "-" + deriveComponent(parts...)
}

func deriveComponent(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	// Encode only the 13 bytes the 20 output chars need (13×8 = 104 bits ≥ 20×5 =
	// 100), over a lowercase base32-hex alphabet — so this is one allocation, not
	// three (a 52-char encode of the full digest, a ToLower copy, then the concat).
	return derivedEncoding.EncodeToString(sum[:13])[:xidLen]
}

// derivedEncoding is base32-hex with the lowercase alphabet WellFormed pins
// ([0-9a-v]) — the standard HexEncoding is uppercase, which would need a second
// pass to fix.
var derivedEncoding = base32.NewEncoding("0123456789abcdefghijklmnopqrstuv").WithPadding(base32.NoPadding)

// WellFormed reports whether s has the canonical id shape (prefix-xid). It does
// NOT check that the prefix is a registered kind — use KindOf for that. Cheap
// input validation for API boundaries (a malformed id is a 400, not a lookup).
func WellFormed(s string) bool { return wellFormed.MatchString(s) }

// KindOf returns the registered Kind an id belongs to (ok=false if the shape is
// wrong or the prefix isn't a registered kind). Lets an adapter route or reject
// an id by type without string-slicing prefixes itself.
func KindOf(s string) (Kind, bool) {
	if !WellFormed(s) {
		return Kind{}, false
	}
	prefix := s[:len(s)-xidLen-1] // strip "-<xid>"
	for _, k := range kinds {
		if k.prefix == prefix {
			return k, true
		}
	}
	return Kind{}, false
}
