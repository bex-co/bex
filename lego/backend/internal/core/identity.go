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
	// usually have Subject == ClientID.
	ClientID string
	// Human distinguishes an OAuth authorization/device token carrying an end-user
	// `sub` (distinct from client_id) from a client_credentials machine token. It
	// lets onboarding and revocation use explicit human semantics.
	Human bool
	// Email is the caller's verified email, populated for session (human)
	// callers from their Kratos identity traits; empty for machine (API-key)
	// callers, which have no email. It is the key a pending workspace invite is
	// redeemed against on first login (internal/api/tenancy.go), not an
	// authorization input — the subject remains the tenant-scoping hook.
	Email string
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
