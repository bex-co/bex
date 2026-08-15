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

import "context"

// Identity is the authenticated caller: an OAuth2 client (API key) validated by
// Hydra introspection or a Kratos identity validated by its session. Subject is
// Hydra's client_id/sub or the Kratos identity id — the tenant-scoping hook.
type Identity struct {
	Subject string
	Method  string // "oauth2" | "session"
	// ClientID is Hydra's OAuth2 client_id. It stays empty for Kratos-session
	// callers. Human OAuth tokens keep Subject as their Kratos identity while
	// ClientID identifies the public app that minted the token; machine tokens
	// usually have Subject == ClientID — which is how CurrentUser knows to
	// resolve a key's minting human instead of probing Kratos with a client id
	// that can never be there (w4/m25).
	ClientID string
	// Human distinguishes an OAuth authorization/device token carrying an end-user
	// `sub` (distinct from client_id) from a client_credentials machine token. It
	// lets onboarding and revocation use explicit human semantics.
	Human bool
	// Email is the caller's email, populated for session (human) callers from
	// their Kratos identity traits; empty for machine (API-key) callers, which
	// have no email. It is the key a pending workspace invite is redeemed against
	// on first login (internal/api/tenancy.go), not an authorization input — the
	// subject remains the tenant-scoping hook. NOTE: this is the raw trait, which
	// Kratos does not require to be verified before a session exists — consult
	// EmailVerified before trusting it as proof of email ownership (w1/m53).
	Email string
	// EmailVerified reports whether Kratos has verified the trait Email for this
	// identity (a matching verifiable_addresses entry with verified=true). False
	// for machine callers and for session callers whose email is not yet verified.
	// Invite redemption can be gated on this so an attacker who registers with a
	// victim's not-yet-signed-up invited address can't claim the invite (w1/m53).
	EmailVerified bool
	// Name is the caller's display name (the optional Kratos `name` trait,
	// w4/m25), populated for session callers alongside Email; empty for machine
	// callers and for identities that never set the trait. Presentation only —
	// never an authorization or invite-redemption input.
	Name string
}

type identityCtxKey struct{}

// IdentityFrom returns the Identity the auth middleware attached to the request
// context (in the composition root), read here by Base.Authorize.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityCtxKey{}).(Identity)
	return id, ok
}

// WithIdentity returns ctx carrying id — the auth middleware's setter, kept here
// so the context key stays private to core (the only reader is Authorize).
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityCtxKey{}, id)
}

// PlatformClientResolver reports whether an OAuth client id is one the
// platform provisioned itself (`bex.co/platform-client` metadata, stamped by
// scripts/auth-bootstrap-client.sh — the official Render CLI, bex-mobile).
// Implemented by the composition root against Hydra's admin API; consumed by
// Base.AuthorizeMintClass so a delegated human token can prove it comes from
// a bex-issued client. Errors mean "cannot establish trust", never "false".
type PlatformClientResolver interface {
	IsPlatformClient(ctx context.Context, clientID string) (bool, error)
}
