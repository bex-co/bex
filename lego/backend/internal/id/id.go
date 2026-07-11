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
// (docs/identifiers.md — the ADR). A bex id is "<prefix>-<20-char xid>", e.g.
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
	"regexp"
	"slices"

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
	Workspace = Kind{prefix: "tea", desc: "workspace (tenant/team)"} // Render: teams are tea-
	Service   = Kind{prefix: "srv", desc: "service (app)"}           // Render: services are srv-
	Domain    = Kind{prefix: "cdm", desc: "custom domain"}           // Render: custom domains are cdm-
	EnvGroup  = Kind{prefix: "evg", desc: "environment group"}       // Render: env groups are evg-
	Deploy    = Kind{prefix: "dep", desc: "deploy"}                  // Render: deploys are dep-
	Invite    = Kind{prefix: "inv", desc: "workspace member invite"} // w4/m12 team invites
	Export    = Kind{prefix: "exp", desc: "managed-postgres export (on-demand snapshot)"}
	Audit     = Kind{prefix: "aud", desc: "audit log event"} // w4/m10 audit log
)

// kinds lists every registered Kind; Kinds returns a copy. KindOf, New's
// membership guard, and the guard test enumerate it, so it must include every
// Kind declared above.
var kinds = []Kind{Workspace, Service, Domain, EnvGroup, Deploy, Invite, Export, Audit}

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
