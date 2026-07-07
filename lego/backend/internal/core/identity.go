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
